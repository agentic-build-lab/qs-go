package qsgo

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseBasicValuesAndDecoding(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		options  *ParseOptions
		expected Value
	}{
		{name: "numeric object key", query: "0=foo", expected: NewObject(Member{Key: "0", Value: NewString("foo")})},
		{name: "plus becomes space", query: "foo=c++", expected: NewObject(Member{Key: "foo", Value: NewString("c  ")})},
		{name: "bracket key containing equals", query: "a[<=>]==23", expected: NewObject(Member{Key: "a", Value: NewObject(Member{Key: "<=>", Value: NewString("=23")})})},
		{name: "first equals divides value", query: "foo=bar=baz", expected: NewObject(Member{Key: "foo", Value: NewString("bar=baz")})},
		{name: "missing equals is empty by default", query: "foo", expected: NewObject(Member{Key: "foo", Value: NewString("")})},
		{name: "missing equals is null in strict mode", query: "foo", options: withParseOptions(func(options *ParseOptions) { options.StrictNullHandling = Bool(true) }), expected: NewObject(Member{Key: "foo", Value: NewNull()})},
		{name: "encoded equals", query: "he%3Dllo=th%3Dere", expected: NewObject(Member{Key: "he=llo", Value: NewString("th=ere")})},
		{name: "encoded utf8", query: "a[b%20c]=d&a[b]=c%20d", expected: NewObject(Member{Key: "a", Value: NewObject(Member{Key: "b c", Value: NewString("d")}, Member{Key: "b", Value: NewString("c d")})})},
		{name: "malformed percent is literal", query: "foo=%:%}", expected: NewObject(Member{Key: "foo", Value: NewString("%:%}")})},
		{name: "empty input", query: "", expected: NewObject()},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertParseValue(t, test.query, test.options, test.expected)
		})
	}
}

func TestParseBracketDotAndDepthSemantics(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		options  *ParseOptions
		expected Value
	}{
		{
			name:  "nested brackets",
			query: "a[b][c]=d",
			expected: NewObject(Member{Key: "a", Value: NewObject(
				Member{Key: "b", Value: NewObject(Member{Key: "c", Value: NewString("d")})},
			)}),
		},
		{
			name:  "default depth preserves remainder",
			query: "a[b][c][d][e][f][g][h]=i",
			expected: NewObject(Member{Key: "a", Value: NewObject(
				Member{Key: "b", Value: NewObject(
					Member{Key: "c", Value: NewObject(
						Member{Key: "d", Value: NewObject(
							Member{Key: "e", Value: NewObject(
								Member{Key: "f", Value: NewObject(
									Member{Key: "[g][h]", Value: NewString("i")},
								)},
							)},
						)},
					)},
				)},
			)}),
		},
		{
			name:    "depth one",
			query:   "a[b][c][d]=e",
			options: withParseOptions(func(options *ParseOptions) { options.Depth = Int(1) }),
			expected: NewObject(Member{Key: "a", Value: NewObject(
				Member{Key: "b", Value: NewObject(
					Member{Key: "[c][d]", Value: NewString("e")},
				)},
			)}),
		},
		{
			name:     "depth zero keeps whole key",
			query:    "a[0]=b&a[1]=c",
			options:  withParseOptions(func(options *ParseOptions) { options.Depth = Int(0) }),
			expected: NewObject(Member{Key: "a[0]", Value: NewString("b")}, Member{Key: "a[1]", Value: NewString("c")}),
		},
		{
			name:    "dot notation",
			query:   "foo[0].baz=bar&fool.bad=baz",
			options: withParseOptions(func(options *ParseOptions) { options.AllowDots = Bool(true) }),
			expected: NewObject(
				Member{Key: "foo", Value: NewArray(NewObject(Member{Key: "baz", Value: NewString("bar")}))},
				Member{Key: "fool", Value: NewObject(Member{Key: "bad", Value: NewString("baz")})},
			),
		},
		{
			name:    "decoded dot remains literal segment",
			query:   "name%252Eobj.first=John&name%252Eobj.last=Doe",
			options: withParseOptions(func(options *ParseOptions) { options.AllowDots = Bool(true); options.DecodeDotInKeys = Bool(true) }),
			expected: NewObject(Member{Key: "name.obj", Value: NewObject(
				Member{Key: "first", Value: NewString("John")},
				Member{Key: "last", Value: NewString("Doe")},
			)}),
		},
		{
			name:  "inner brackets are literal",
			query: "search[withbracket[]]=foobar",
			expected: NewObject(Member{Key: "search", Value: NewObject(
				Member{Key: "withbracket[]", Value: NewString("foobar")},
			)}),
		},
		{
			name:  "unclosed group after parent",
			query: "a[b][c=v",
			expected: NewObject(Member{Key: "a", Value: NewObject(
				Member{Key: "b", Value: NewObject(
					Member{Key: "[c", Value: NewString("v")},
				)},
			)}),
		},
		{
			name:  "text after balanced group is ignored",
			query: "a[b]extra=v",
			expected: NewObject(Member{Key: "a", Value: NewObject(
				Member{Key: "b", Value: NewString("v")},
			)}),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertParseValue(t, test.query, test.options, test.expected)
		})
	}
}

func TestParseStrictDepth(t *testing.T) {
	options := withParseOptions(func(options *ParseOptions) {
		options.Depth = Int(1)
		options.StrictDepth = Bool(true)
	})
	_, err := Parse("a[b][c]=d", options)
	if err == nil || err.Error() != "Input depth exceeded depth option of 1 and strictDepth is true" {
		t.Fatalf("strict depth error = %v", err)
	}

	options.Depth = Int(0)
	if _, err = Parse("a[b][c]=d", options); err != nil {
		t.Fatalf("depth zero must remain lenient: %v", err)
	}
}

func TestParseArraysIndicesSparseAndMixedNotation(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		options  *ParseOptions
		expected Value
	}{
		{name: "explicit array", query: "a[]=b&a[]=c", expected: NewObject(Member{Key: "a", Value: NewArray(NewString("b"), NewString("c"))})},
		{name: "ordered indices", query: "a[1]=c&a[0]=b&a[2]=d", expected: NewObject(Member{Key: "a", Value: NewArray(NewString("b"), NewString("c"), NewString("d"))})},
		{name: "sparse compacts by default", query: "a[10]=1&a[2]=2", expected: NewObject(Member{Key: "a", Value: NewArray(NewString("2"), NewString("1"))})},
		{
			name:    "sparse remains sparse when allowed",
			query:   "a[4]=1&a[1]=2",
			options: withParseOptions(func(options *ParseOptions) { options.AllowSparse = Bool(true) }),
			expected: NewObject(Member{Key: "a", Value: NewSparseArray(
				Element{},
				Element{Present: true, Value: NewString("2")},
				Element{},
				Element{},
				Element{Present: true, Value: NewString("1")},
			)}),
		},
		{
			name:  "array and object key becomes object",
			query: "foo[]=bar&foo[bad]=baz",
			expected: NewObject(Member{Key: "foo", Value: NewObject(
				Member{Key: "0", Value: NewString("bar")},
				Member{Key: "bad", Value: NewString("baz")},
			)}),
		},
		{
			name:  "array of objects",
			query: "a[0][b]=c&a[1][b]=d",
			expected: NewObject(Member{Key: "a", Value: NewArray(
				NewObject(Member{Key: "b", Value: NewString("c")}),
				NewObject(Member{Key: "b", Value: NewString("d")}),
			)}),
		},
		{
			name:    "array parsing disabled",
			query:   "a[0]=b&a[1]=c&a[]=d",
			options: withParseOptions(func(options *ParseOptions) { options.ParseArrays = Bool(false) }),
			expected: NewObject(Member{Key: "a", Value: NewObject(
				Member{Key: "0", Value: NewArray(NewString("b"), NewString("d"))},
				Member{Key: "1", Value: NewString("c")},
			)}),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertParseValue(t, test.query, test.options, test.expected)
		})
	}
}

func TestParseEmptyArraysAndNullSlots(t *testing.T) {
	assertParseValue(
		t,
		"foo[]&bar=baz",
		withParseOptions(func(options *ParseOptions) { options.AllowEmptyArrays = Bool(true) }),
		NewObject(Member{Key: "foo", Value: NewArray()}, Member{Key: "bar", Value: NewString("baz")}),
	)
	assertParseValue(
		t,
		"a[]=b&a[]&a[]=c&a[]=",
		withParseOptions(func(options *ParseOptions) { options.StrictNullHandling = Bool(true) }),
		NewObject(Member{Key: "a", Value: NewArray(NewString("b"), NewNull(), NewString("c"), NewString(""))}),
	)
}

func TestParseDuplicatePoliciesAndStrictMerge(t *testing.T) {
	assertParseValue(t, "foo=bar&foo=baz", nil, NewObject(Member{Key: "foo", Value: NewArray(NewString("bar"), NewString("baz"))}))
	assertParseValue(
		t,
		"foo=bar&foo=baz",
		withParseOptions(func(options *ParseOptions) { options.Duplicates = DuplicatesFirst }),
		NewObject(Member{Key: "foo", Value: NewString("bar")}),
	)
	assertParseValue(
		t,
		"foo=bar&foo=baz",
		withParseOptions(func(options *ParseOptions) { options.Duplicates = DuplicatesLast }),
		NewObject(Member{Key: "foo", Value: NewString("baz")}),
	)
	assertParseValue(
		t,
		"a=1&a=2&b[]=1&b[]=2",
		withParseOptions(func(options *ParseOptions) { options.Duplicates = DuplicatesLast }),
		NewObject(Member{Key: "a", Value: NewString("2")}, Member{Key: "b", Value: NewArray(NewString("1"), NewString("2"))}),
	)
	assertParseValue(
		t,
		"a[b]=c&a=d",
		nil,
		NewObject(Member{Key: "a", Value: NewArray(NewObject(Member{Key: "b", Value: NewString("c")}), NewString("d"))}),
	)
	assertParseValue(
		t,
		"a[b]=c&a=d",
		withParseOptions(func(options *ParseOptions) { options.StrictMerge = Bool(false) }),
		NewObject(Member{Key: "a", Value: NewObject(
			Member{Key: "b", Value: NewString("c")},
			Member{Key: "d", Value: NewBool(true)},
		)}),
	)
}

func TestParseParameterLimits(t *testing.T) {
	assertParseValue(
		t,
		"a=1&b=2&c=3",
		withParseOptions(func(options *ParseOptions) { options.ParameterLimit = FiniteLimit(2) }),
		NewObject(Member{Key: "a", Value: NewString("1")}, Member{Key: "b", Value: NewString("2")}),
	)

	_, err := Parse("a=1&b=2", withParseOptions(func(options *ParseOptions) {
		options.ParameterLimit = FiniteLimit(1)
		options.ThrowOnLimitExceeded = Bool(true)
	}))
	if err == nil || err.Error() != "Parameter limit exceeded. Only 1 parameter allowed." {
		t.Fatalf("parameter limit error = %v", err)
	}

	assertParseValue(
		t,
		"a=1&b=2&c=3",
		withParseOptions(func(options *ParseOptions) { options.ParameterLimit = UnlimitedLimit() }),
		NewObject(Member{Key: "a", Value: NewString("1")}, Member{Key: "b", Value: NewString("2")}, Member{Key: "c", Value: NewString("3")}),
	)
}

func TestParseArrayLimitsAndOverflowObjects(t *testing.T) {
	assertParseValue(
		t,
		"a[]=1&a[]=2&a[]=3",
		withParseOptions(func(options *ParseOptions) { options.ArrayLimit = Int(3) }),
		NewObject(Member{Key: "a", Value: NewArray(NewString("1"), NewString("2"), NewString("3"))}),
	)
	assertParseValue(
		t,
		"a[]=1&a[]=2&a[]=3&a[]=4",
		withParseOptions(func(options *ParseOptions) { options.ArrayLimit = Int(3) }),
		NewObject(Member{Key: "a", Value: NewObject(
			Member{Key: "0", Value: NewString("1")},
			Member{Key: "1", Value: NewString("2")},
			Member{Key: "2", Value: NewString("3")},
			Member{Key: "3", Value: NewString("4")},
		)}),
	)
	assertParseValue(
		t,
		"a[19]=x&a[20]=y",
		withParseOptions(func(options *ParseOptions) { options.ArrayLimit = Int(20) }),
		NewObject(Member{Key: "a", Value: NewObject(
			Member{Key: "19", Value: NewString("x")},
			Member{Key: "20", Value: NewString("y")},
		)}),
	)

	_, err := Parse("a[]=1&a[]=2", withParseOptions(func(options *ParseOptions) {
		options.ArrayLimit = Int(1)
		options.ThrowOnLimitExceeded = Bool(true)
	}))
	if err == nil || err.Error() != "Array limit exceeded. Only 1 element allowed in an array." {
		t.Fatalf("array limit error = %v", err)
	}
}

func TestParseCommaArraysAndEncodedCommas(t *testing.T) {
	options := withParseOptions(func(options *ParseOptions) { options.Comma = Bool(true) })
	assertParseValue(t, "a=1,2,3", options, NewObject(Member{Key: "a", Value: NewArray(NewString("1"), NewString("2"), NewString("3"))}))
	assertParseValue(t, "foo=a%2Cb,d", options, NewObject(Member{Key: "foo", Value: NewArray(NewString("a,b"), NewString("d"))}))
	assertParseValue(
		t,
		"foo[]=1,2,3&foo[]=4,5,6",
		options,
		NewObject(Member{Key: "foo", Value: NewArray(
			NewArray(NewString("1"), NewString("2"), NewString("3")),
			NewArray(NewString("4"), NewString("5"), NewString("6")),
		)}),
	)

	assertParseValue(
		t,
		"a=1,2,3&a=4,5,6",
		withParseOptions(func(options *ParseOptions) { options.Comma = Bool(true); options.ArrayLimit = Int(5) }),
		NewObject(Member{Key: "a", Value: NewObject(
			Member{Key: "0", Value: NewString("1")},
			Member{Key: "1", Value: NewString("2")},
			Member{Key: "2", Value: NewString("3")},
			Member{Key: "3", Value: NewString("4")},
			Member{Key: "4", Value: NewString("5")},
			Member{Key: "5", Value: NewString("6")},
		)}),
	)
}

func TestParsePrototypeProtection(t *testing.T) {
	assertParseValue(t, "toString=foo&a[hasOwnProperty]=b&safe=c", nil, NewObject(Member{Key: "safe", Value: NewString("c")}))
	assertParseValue(
		t,
		"toString=foo&a[hasOwnProperty]=b",
		withParseOptions(func(options *ParseOptions) { options.AllowPrototypes = Bool(true) }),
		NewObject(
			Member{Key: "toString", Value: NewString("foo")},
			Member{Key: "a", Value: NewObject(
				Member{Key: "hasOwnProperty", Value: NewString("b")},
			)},
		),
	)
	assertParseValue(
		t,
		"foo[__proto__][hidden]=value&foo[bar]=stuffs",
		withParseOptions(func(options *ParseOptions) { options.AllowPrototypes = Bool(true) }),
		NewObject(Member{Key: "foo", Value: NewObject(
			Member{Key: "bar", Value: NewString("stuffs")},
		)}),
	)
}

func TestParseCharsetsSentinelsAndNumericEntities(t *testing.T) {
	assertParseValue(
		t,
		"%A2=%BD",
		withParseOptions(func(options *ParseOptions) { options.Charset = CharsetISO88591 }),
		NewObject(Member{Key: "\u00A2", Value: NewString("\u00BD")}),
	)
	assertParseValue(
		t,
		"a=%C3%B8&utf8=%26%2310003%3B",
		withParseOptions(func(options *ParseOptions) { options.CharsetSentinel = Bool(true) }),
		NewObject(Member{Key: "a", Value: NewString("\u00C3\u00B8")}),
	)
	assertParseValue(
		t,
		"utf8=%E2%9C%93&a=%C3%B8",
		withParseOptions(func(options *ParseOptions) { options.Charset = CharsetISO88591; options.CharsetSentinel = Bool(true) }),
		NewObject(Member{Key: "a", Value: NewString("\u00F8")}),
	)
	assertParseValue(
		t,
		"foo=%26%239786%3B",
		withParseOptions(func(options *ParseOptions) {
			options.Charset = CharsetISO88591
			options.InterpretNumericEntities = Bool(true)
		}),
		NewObject(Member{Key: "foo", Value: NewString("\u263A")}),
	)
}

func TestParsePrefixAndDelimiter(t *testing.T) {
	assertParseValue(
		t,
		"?foo=bar",
		withParseOptions(func(options *ParseOptions) { options.IgnoreQueryPrefix = Bool(true) }),
		NewObject(Member{Key: "foo", Value: NewString("bar")}),
	)
	assertParseValue(
		t,
		"a=b;c=d",
		withParseOptions(func(options *ParseOptions) { options.Delimiter = ";" }),
		NewObject(Member{Key: "a", Value: NewString("b")}, Member{Key: "c", Value: NewString("d")}),
	)
}

func TestParseVeryDeepInputDoesNotPanic(t *testing.T) {
	var builder strings.Builder
	builder.WriteString("root")
	for range 5000 {
		builder.WriteString("[p]")
	}
	builder.WriteString("=value")

	options := withParseOptions(func(options *ParseOptions) { options.Depth = Int(5000) })
	if _, err := Parse(builder.String(), options); err != nil {
		t.Fatalf("deep parse failed: %v", err)
	}
}

func withParseOptions(change func(*ParseOptions)) *ParseOptions {
	options := DefaultParseOptions()
	change(&options)
	return &options
}

func assertParseValue(t *testing.T, query string, options *ParseOptions, expected Value) {
	t.Helper()
	actual, err := Parse(query, options)
	if err != nil {
		t.Fatalf("Parse(%q) error: %v", query, err)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("Parse(%q)\nactual:   %#v\nexpected: %#v", query, actual, expected)
	}
}
