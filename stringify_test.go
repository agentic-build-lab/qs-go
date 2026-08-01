package qsgo

import (
	"math"
	"math/big"
	"strings"
	"testing"
	"time"
)

func TestStringifyDefaultsPreserveOrderAndTypes(t *testing.T) {
	value := NewObject(
		Member{Key: "b", Value: NewString("first")},
		Member{Key: "a", Value: NewNumber(2)},
		Member{Key: "flag", Value: NewBool(false)},
	)
	assertStringify(t, value, nil, "b=first&a=2&flag=false")

	assertStringify(t, NewUndefined(), nil, "")
	assertStringify(t, NewNull(), nil, "")
	assertStringify(t, NewString("root"), nil, "")
}

func TestStringifyScalarRepresentations(t *testing.T) {
	large, ok := new(big.Int).SetString("123456789012345678901234567890", 10)
	if !ok {
		t.Fatal("failed to build bigint fixture")
	}
	value := NewObject(
		Member{Key: "unicode", Value: NewString("€")},
		Member{Key: "negative_zero", Value: NewNumber(math.Copysign(0, -1))},
		Member{Key: "nan", Value: NewNumber(math.NaN())},
		Member{Key: "positive_infinity", Value: NewNumber(math.Inf(1))},
		Member{Key: "negative_infinity", Value: NewNumber(math.Inf(-1))},
		Member{Key: "exponent", Value: NewNumber(1e22)},
		Member{Key: "big", Value: NewBigInt(large)},
		Member{Key: "bytes", Value: NewBytes([]byte("a b"))},
	)
	assertStringify(
		t,
		value,
		nil,
		"unicode=%E2%82%AC&negative_zero=0&nan=NaN&positive_infinity=Infinity&negative_infinity=-Infinity&exponent=1e%2B22&big=123456789012345678901234567890&bytes=a%20b",
	)
}

func TestJavaScriptNumberFormattingThresholds(t *testing.T) {
	tests := []struct {
		value    float64
		expected string
	}{
		{value: 1e-7, expected: "1e-7"},
		{value: 1e-6, expected: "0.000001"},
		{value: 1e20, expected: "100000000000000000000"},
		{value: 1e21, expected: "1e+21"},
		{value: 1000000000000000100, expected: "1000000000000000100"},
		{value: math.SmallestNonzeroFloat64, expected: "5e-324"},
		{value: 2.2250738585072014e-308, expected: "2.2250738585072014e-308"},
		{value: 1.2345678901234567, expected: "1.2345678901234567"},
		{value: -1e-7, expected: "-1e-7"},
	}
	for _, test := range tests {
		if actual := formatJavaScriptNumber(test.value); actual != test.expected {
			t.Errorf("formatJavaScriptNumber(%g) = %q, want %q", test.value, actual, test.expected)
		}
	}
}

func TestStringifyNullUndefinedAndEmptyString(t *testing.T) {
	value := NewObject(
		Member{Key: "undefined", Value: NewUndefined()},
		Member{Key: "null", Value: NewNull()},
		Member{Key: "empty", Value: NewString("")},
		Member{Key: "nested", Value: NewObject(
			Member{Key: "missing", Value: NewUndefined()},
			Member{Key: "null", Value: NewNull()},
		)},
	)
	assertStringify(t, value, nil, "null=&empty=&nested%5Bnull%5D=")

	strict := DefaultStringifyOptions()
	strict.StrictNullHandling = Bool(true)
	assertStringify(t, value, &strict, "null&empty=&nested%5Bnull%5D")

	skip := DefaultStringifyOptions()
	skip.SkipNulls = Bool(true)
	assertStringify(t, value, &skip, "empty=")
}

func TestStringifyArrayFormats(t *testing.T) {
	value := NewObject(Member{Key: "a", Value: NewArray(NewString("b"), NewString("c"))})
	tests := []struct {
		name     string
		format   ArrayFormat
		expected string
	}{
		{name: "indices", format: ArrayFormatIndices, expected: "a%5B0%5D=b&a%5B1%5D=c"},
		{name: "brackets", format: ArrayFormatBrackets, expected: "a%5B%5D=b&a%5B%5D=c"},
		{name: "repeat", format: ArrayFormatRepeat, expected: "a=b&a=c"},
		{name: "comma", format: ArrayFormatComma, expected: "a=b%2Cc"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			options := DefaultStringifyOptions()
			options.ArrayFormat = test.format
			assertStringify(t, value, &options, test.expected)
		})
	}
}

func TestStringifyCommaRoundTripAndElementEncoding(t *testing.T) {
	comma := DefaultStringifyOptions()
	comma.ArrayFormat = ArrayFormatComma
	assertStringify(
		t,
		NewObject(Member{Key: "a", Value: NewArray(NewString("c,d"), NewString("e"))}),
		&comma,
		"a=c%2Cd%2Ce",
	)

	comma.EncodeValuesOnly = Bool(true)
	assertStringify(
		t,
		NewObject(Member{Key: "a", Value: NewArray(NewString("c,d"), NewString("e"))}),
		&comma,
		"a=c%2Cd,e",
	)

	comma.CommaRoundTrip = Bool(true)
	assertStringify(
		t,
		NewObject(Member{Key: "a", Value: NewArray(NewString("c"))}),
		&comma,
		"a[]=c",
	)

	comma.EncodeValuesOnly = Bool(false)
	assertStringify(
		t,
		NewObject(Member{Key: "a", Value: NewArray(NewString("c"))}),
		&comma,
		"a%5B%5D=c",
	)
}

func TestStringifyCommaNullAndSparseSlots(t *testing.T) {
	comma := DefaultStringifyOptions()
	comma.ArrayFormat = ArrayFormatComma
	comma.EncodeValuesOnly = Bool(true)
	value := NewObject(Member{Key: "a", Value: NewSparseArray(
		Element{},
		Element{Present: true, Value: NewNull()},
		Element{Present: true, Value: NewString("b")},
	)})
	assertStringify(t, value, &comma, "a=,,b")

	strict := comma
	assertStringify(
		t,
		NewObject(Member{Key: "a", Value: NewArray(NewNull())}),
		&strict,
		"a=",
	)
	strict.StrictNullHandling = Bool(true)
	assertStringify(
		t,
		NewObject(Member{Key: "a", Value: NewArray(NewNull())}),
		&strict,
		"a",
	)
	strict.SkipNulls = Bool(true)
	assertStringify(
		t,
		NewObject(Member{Key: "a", Value: NewArray(NewNull())}),
		&strict,
		"",
	)
}

func TestStringifySparseArrays(t *testing.T) {
	value := NewObject(Member{Key: "a", Value: NewSparseArray(
		Element{},
		Element{Present: true, Value: NewString("2")},
		Element{},
		Element{},
		Element{Present: true, Value: NewString("1")},
	)})

	indices := DefaultStringifyOptions()
	indices.EncodeValuesOnly = Bool(true)
	assertStringify(t, value, &indices, "a[1]=2&a[4]=1")

	brackets := indices
	brackets.ArrayFormat = ArrayFormatBrackets
	assertStringify(t, value, &brackets, "a[]=2&a[]=1")

	repeat := indices
	repeat.ArrayFormat = ArrayFormatRepeat
	assertStringify(t, value, &repeat, "a=2&a=1")
}

func TestStringifyEmptyArrays(t *testing.T) {
	value := NewObject(
		Member{Key: "a", Value: NewArray()},
		Member{Key: "b", Value: NewString("value")},
	)
	assertStringify(t, value, nil, "b=value")

	options := DefaultStringifyOptions()
	options.AllowEmptyArrays = Bool(true)
	assertStringify(t, value, &options, "a[]&b=value")
	assertStringify(
		t,
		NewObject(Member{Key: "a b", Value: NewArray()}),
		&options,
		"a b[]",
	)
}

func TestStringifyNestedObjectsAndDots(t *testing.T) {
	value := NewObject(Member{Key: "a", Value: NewObject(
		Member{Key: "b", Value: NewObject(Member{Key: "c", Value: NewString("d")})},
	)})
	assertStringify(t, value, nil, "a%5Bb%5D%5Bc%5D=d")

	dots := DefaultStringifyOptions()
	dots.AllowDots = Bool(true)
	assertStringify(t, value, &dots, "a.b.c=d")

	dots.EncodeDotInKeys = Bool(true)
	assertStringify(t, value, &dots, "a%252Eb.c=d")

	literal := NewObject(Member{Key: "name.obj", Value: NewString("John")})
	assertStringify(t, literal, &dots, "name%252Eobj=John")

	dots.EncodeValuesOnly = Bool(true)
	assertStringify(t, literal, &dots, "name%2Eobj=John")
}

func TestStringifyEncodeDotInKeysAutoEnablesDotsOnlyWhenOmitted(t *testing.T) {
	value := NewObject(Member{Key: "name.obj", Value: NewObject(
		Member{Key: "first", Value: NewString("John")},
	)})

	automatic := StringifyOptions{EncodeDotInKeys: Bool(true)}
	assertStringify(t, value, &automatic, "name%252Eobj.first=John")

	explicit := StringifyOptions{
		AllowDots:       Bool(false),
		EncodeDotInKeys: Bool(true),
	}
	assertStringify(t, value, &explicit, "name%252Eobj%5Bfirst%5D=John")
}

func TestStringifyEncodeValuesOnlyAndEncodeFalse(t *testing.T) {
	value := NewObject(Member{Key: "a b", Value: NewObject(
		Member{Key: "c", Value: NewString("d=e")},
	)})

	valuesOnly := DefaultStringifyOptions()
	valuesOnly.EncodeValuesOnly = Bool(true)
	assertStringify(t, value, &valuesOnly, "a b[c]=d%3De")

	plain := DefaultStringifyOptions()
	plain.Encode = Bool(false)
	assertStringify(t, value, &plain, "a b[c]=d=e")
}

func TestStringifyRFCFormats(t *testing.T) {
	value := NewObject(Member{Key: "a b", Value: NewString("c d")})
	assertStringify(t, value, nil, "a%20b=c%20d")

	options := DefaultStringifyOptions()
	options.Format = FormatRFC1738
	assertStringify(t, value, &options, "a+b=c+d")
	assertStringify(
		t,
		NewObject(Member{Key: "foo(ref)", Value: NewString("bar")}),
		&options,
		"foo(ref)=bar",
	)
}

func TestStringifyCharsetsAndSentinels(t *testing.T) {
	latin := DefaultStringifyOptions()
	latin.Charset = CharsetISO88591
	assertStringify(
		t,
		NewObject(Member{Key: "æ", Value: NewString("æ")}),
		&latin,
		"%E6=%E6",
	)
	assertStringify(
		t,
		NewObject(Member{Key: "a", Value: NewString("☺")}),
		&latin,
		"a=%26%239786%3B",
	)
	assertStringify(
		t,
		NewObject(Member{Key: "a", Value: NewString("𐐷")}),
		&latin,
		"a=%26%2355297%3B%26%2356375%3B",
	)
	assertStringify(
		t,
		NewObject(
			Member{Key: "plus", Value: NewString("+")},
			Member{Key: "tilde", Value: NewString("~")},
		),
		&latin,
		"plus=+&tilde=%7E",
	)

	latin.CharsetSentinel = Bool(true)
	assertStringify(
		t,
		NewObject(Member{Key: "a", Value: NewString("æ")}),
		&latin,
		"utf8=%26%2310003%3B&a=%E6",
	)

	utf8Options := DefaultStringifyOptions()
	utf8Options.CharsetSentinel = Bool(true)
	utf8Options.Delimiter = ";"
	assertStringify(
		t,
		NewObject(
			Member{Key: "a", Value: NewNumber(1)},
			Member{Key: "b", Value: NewNumber(2)},
		),
		&utf8Options,
		"utf8=%E2%9C%93;a=1;b=2",
	)
}

func TestStringifyQueryPrefixAndDelimiter(t *testing.T) {
	options := DefaultStringifyOptions()
	options.AddQueryPrefix = Bool(true)
	options.Delimiter = ";"
	assertStringify(
		t,
		NewObject(
			Member{Key: "a", Value: NewString("b")},
			Member{Key: "c", Value: NewString("d")},
		),
		&options,
		"?a=b;c=d",
	)
	assertStringify(t, NewObject(), &options, "")
}

func TestStringifyDateUsesUTCISOWithMilliseconds(t *testing.T) {
	location := time.FixedZone("fixture", 8*60*60)
	date := time.Date(2026, time.August, 1, 10, 3, 4, 5*int(time.Millisecond), location)
	assertStringify(
		t,
		NewObject(Member{Key: "at", Value: NewTime(date)}),
		nil,
		"at=2026-08-01T02%3A03%3A04.005Z",
	)
}

func TestStringifyDepth(t *testing.T) {
	flat := NewObject(Member{Key: "a", Value: NewString("b")})
	nested := NewObject(Member{Key: "a", Value: NewObject(
		Member{Key: "b", Value: NewObject(Member{Key: "c", Value: NewString("d")})},
	)})

	zero := DefaultStringifyOptions()
	zero.Depth = FiniteLimit(0)
	assertStringify(t, flat, &zero, "a=b")
	assertStringifyError(t, NewObject(Member{Key: "a", Value: NewObject(
		Member{Key: "b", Value: NewString("c")},
	)}), &zero, "Input depth exceeded depth option of 0")

	two := DefaultStringifyOptions()
	two.Depth = FiniteLimit(2)
	assertStringify(t, nested, &two, "a%5Bb%5D%5Bc%5D=d")

	one := DefaultStringifyOptions()
	one.Depth = FiniteLimit(1)
	assertStringifyError(t, nested, &one, "Input depth exceeded depth option of 1")
}

func TestStringifyRootSparseArray(t *testing.T) {
	value := NewSparseArray(
		Element{},
		Element{Present: true, Value: NewString("two")},
		Element{Present: true, Value: NewNull()},
	)
	assertStringify(t, value, nil, "1=two&2=")
}

func TestStringifyNestedArraysAndObjects(t *testing.T) {
	value := NewObject(Member{Key: "a", Value: NewArray(NewObject(
		Member{Key: "b", Value: NewObject(
			Member{Key: "c", Value: NewArray(NewNumber(1))},
		)},
	))})
	options := DefaultStringifyOptions()
	options.EncodeValuesOnly = Bool(true)
	assertStringify(t, value, &options, "a[0][b][c][0]=1")

	options.ArrayFormat = ArrayFormatBrackets
	assertStringify(t, value, &options, "a[][b][c][]=1")

	options.ArrayFormat = ArrayFormatRepeat
	assertStringify(t, value, &options, "a[b][c]=1")
}

func TestStringifyLongInput(t *testing.T) {
	input := strings.Repeat(" x", 5000)
	actual, err := Stringify(NewObject(Member{Key: "foo", Value: NewString(input)}), nil)
	if err != nil {
		t.Fatalf("Stringify returned error: %v", err)
	}
	if !strings.HasPrefix(actual, "foo=%20x%20x") {
		t.Fatalf("unexpected prefix: %.30q", actual)
	}
	if strings.Count(actual, "%20") != 5000 {
		t.Fatalf("encoded spaces = %d, want 5000", strings.Count(actual, "%20"))
	}
}

func TestStringifyRejectsInvalidEnums(t *testing.T) {
	value := NewObject(Member{Key: "a", Value: NewString("b")})

	invalidArray := DefaultStringifyOptions()
	invalidArray.ArrayFormat = ArrayFormat(99)
	assertStringifyError(t, value, &invalidArray, "qsgo: unsupported array format 99")

	invalidCharset := DefaultStringifyOptions()
	invalidCharset.Charset = Charset(99)
	assertStringifyError(t, value, &invalidCharset, "qsgo: unsupported charset 99")

	invalidFormat := DefaultStringifyOptions()
	invalidFormat.Format = Format(99)
	assertStringifyError(t, value, &invalidFormat, "qsgo: unsupported format 99")
}

func assertStringify(t *testing.T, value Value, options *StringifyOptions, expected string) {
	t.Helper()
	actual, err := Stringify(value, options)
	if err != nil {
		t.Fatalf("Stringify returned error: %v", err)
	}
	if actual != expected {
		t.Fatalf("Stringify = %q, want %q", actual, expected)
	}
}

func assertStringifyError(t *testing.T, value Value, options *StringifyOptions, expected string) {
	t.Helper()
	actual, err := Stringify(value, options)
	if err == nil {
		t.Fatalf("Stringify = %q, want error %q", actual, expected)
	}
	if err.Error() != expected {
		t.Fatalf("Stringify error = %q, want %q", err.Error(), expected)
	}
}
