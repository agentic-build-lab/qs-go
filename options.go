package qsgo

// BoolOption distinguishes an omitted option from an explicit false value.
type BoolOption struct {
	Set   bool
	Value bool
}

func Bool(value bool) BoolOption { return BoolOption{Set: true, Value: value} }

// IntOption distinguishes an omitted option from an explicit zero value.
type IntOption struct {
	Set   bool
	Value int
}

func Int(value int) IntOption { return IntOption{Set: true, Value: value} }

// Limit represents either a finite nonnegative limit or infinity.
type Limit struct {
	Set       bool
	Unlimited bool
	Value     int
}

func FiniteLimit(value int) Limit { return Limit{Set: true, Value: value} }
func UnlimitedLimit() Limit       { return Limit{Set: true, Unlimited: true} }

type Charset uint8

const (
	CharsetUTF8 Charset = iota
	CharsetISO88591
)

type DuplicatePolicy uint8

const (
	DuplicatesCombine DuplicatePolicy = iota
	DuplicatesFirst
	DuplicatesLast
)

type ArrayFormat uint8

const (
	ArrayFormatIndices ArrayFormat = iota
	ArrayFormatBrackets
	ArrayFormatRepeat
	ArrayFormatComma
)

type Format uint8

const (
	FormatRFC3986 Format = iota
	FormatRFC1738
)

// ParseOptions models options with cross-runtime behavior. Nil options use the
// upstream defaults documented in DefaultParseOptions.
type ParseOptions struct {
	AllowDots                BoolOption
	AllowEmptyArrays         BoolOption
	AllowPrototypes          BoolOption
	AllowSparse              BoolOption
	ArrayLimit               IntOption
	Charset                  Charset
	CharsetSentinel          BoolOption
	Comma                    BoolOption
	DecodeDotInKeys          BoolOption
	Delimiter                string
	Depth                    IntOption
	Duplicates               DuplicatePolicy
	IgnoreQueryPrefix        BoolOption
	InterpretNumericEntities BoolOption
	ParameterLimit           Limit
	ParseArrays              BoolOption
	PlainObjects             BoolOption
	StrictDepth              BoolOption
	StrictMerge              BoolOption
	StrictNullHandling       BoolOption
	ThrowOnLimitExceeded     BoolOption
}

func DefaultParseOptions() ParseOptions {
	return ParseOptions{
		AllowDots:                Bool(false),
		AllowEmptyArrays:         Bool(false),
		AllowPrototypes:          Bool(false),
		AllowSparse:              Bool(false),
		ArrayLimit:               Int(20),
		Charset:                  CharsetUTF8,
		CharsetSentinel:          Bool(false),
		Comma:                    Bool(false),
		DecodeDotInKeys:          Bool(false),
		Delimiter:                "&",
		Depth:                    Int(5),
		Duplicates:               DuplicatesCombine,
		IgnoreQueryPrefix:        Bool(false),
		InterpretNumericEntities: Bool(false),
		ParameterLimit:           FiniteLimit(1000),
		ParseArrays:              Bool(true),
		PlainObjects:             Bool(false),
		StrictDepth:              Bool(false),
		StrictMerge:              Bool(true),
		StrictNullHandling:       Bool(false),
		ThrowOnLimitExceeded:     Bool(false),
	}
}

// StringifyOptions models deterministic qs output without dynamic escape-hatch hooks.
// Typed callback extension points will be added with the differential harness.
type StringifyOptions struct {
	AddQueryPrefix     BoolOption
	AllowDots          BoolOption
	AllowEmptyArrays   BoolOption
	ArrayFormat        ArrayFormat
	Charset            Charset
	CharsetSentinel    BoolOption
	CommaRoundTrip     BoolOption
	Delimiter          string
	Depth              Limit
	Encode             BoolOption
	EncodeDotInKeys    BoolOption
	EncodeValuesOnly   BoolOption
	Format             Format
	SkipNulls          BoolOption
	StrictNullHandling BoolOption
}

func DefaultStringifyOptions() StringifyOptions {
	return StringifyOptions{
		AddQueryPrefix:     Bool(false),
		AllowDots:          Bool(false),
		AllowEmptyArrays:   Bool(false),
		ArrayFormat:        ArrayFormatIndices,
		Charset:            CharsetUTF8,
		CharsetSentinel:    Bool(false),
		CommaRoundTrip:     Bool(false),
		Delimiter:          "&",
		Depth:              UnlimitedLimit(),
		Encode:             Bool(true),
		EncodeDotInKeys:    Bool(false),
		EncodeValuesOnly:   Bool(false),
		Format:             FormatRFC3986,
		SkipNulls:          Bool(false),
		StrictNullHandling: Bool(false),
	}
}

func normalizeBool(option BoolOption, fallback bool) bool {
	if option.Set {
		return option.Value
	}
	return fallback
}
