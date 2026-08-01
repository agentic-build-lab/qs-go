package qsgo

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

type normalizedStringifyOptions struct {
	addQueryPrefix     bool
	allowDots          bool
	allowEmptyArrays   bool
	arrayFormat        ArrayFormat
	charset            Charset
	charsetSentinel    bool
	commaRoundTrip     bool
	delimiter          string
	depthLimited       bool
	depth              int
	encode             bool
	encodeDotInKeys    bool
	encodeValuesOnly   bool
	format             Format
	skipNulls          bool
	strictNullHandling bool
}

// Stringify serializes the closed Value algebra using the observable defaults
// and core options of the frozen qs implementation.
func Stringify(value Value, options *StringifyOptions) (string, error) {
	normalized, err := normalizeStringifyOptions(options)
	if err != nil {
		return "", err
	}

	var pairs []string
	switch value.Kind() {
	case KindObject:
		members, _ := value.Members()
		for _, member := range members {
			if normalized.skipNulls && member.Value.Kind() == KindNull {
				continue
			}
			prefix := encodeLiteralDots(member.Key, normalized.encodeDotInKeys)
			encoded, encodeErr := stringifyValue(member.Value, prefix, normalized, 0)
			if encodeErr != nil {
				return "", encodeErr
			}
			pairs = append(pairs, encoded...)
		}
	case KindArray:
		elements, _ := value.Elements()
		for index, element := range elements {
			if !element.Present {
				continue
			}
			if normalized.skipNulls && element.Value.Kind() == KindNull {
				continue
			}
			encoded, encodeErr := stringifyValue(element.Value, strconv.Itoa(index), normalized, 0)
			if encodeErr != nil {
				return "", encodeErr
			}
			pairs = append(pairs, encoded...)
		}
	case KindUndefined, KindNull, KindString, KindBool, KindNumber, KindBigInt, KindBytes, KindTime:
		return "", nil
	default:
		return "", fmt.Errorf("qsgo: unsupported value kind %d", value.Kind())
	}

	if len(pairs) == 0 {
		return "", nil
	}

	var result strings.Builder
	if normalized.addQueryPrefix {
		result.WriteByte('?')
	}
	if normalized.charsetSentinel {
		if normalized.charset == CharsetISO88591 {
			result.WriteString("utf8=%26%2310003%3B")
		} else {
			result.WriteString("utf8=%E2%9C%93")
		}
		result.WriteString(normalized.delimiter)
	}
	result.WriteString(strings.Join(pairs, normalized.delimiter))
	return result.String(), nil
}

func normalizeStringifyOptions(options *StringifyOptions) (normalizedStringifyOptions, error) {
	defaults := DefaultStringifyOptions()
	if options == nil {
		options = &defaults
	}

	if options.ArrayFormat > ArrayFormatComma {
		return normalizedStringifyOptions{}, fmt.Errorf("qsgo: unsupported array format %d", options.ArrayFormat)
	}
	if options.Charset > CharsetISO88591 {
		return normalizedStringifyOptions{}, fmt.Errorf("qsgo: unsupported charset %d", options.Charset)
	}
	if options.Format > FormatRFC1738 {
		return normalizedStringifyOptions{}, fmt.Errorf("qsgo: unsupported format %d", options.Format)
	}

	encodeDotInKeys := normalizeBool(options.EncodeDotInKeys, false)
	allowDots := normalizeBool(options.AllowDots, false)
	if !options.AllowDots.Set && encodeDotInKeys {
		allowDots = true
	}
	delimiter := options.Delimiter
	if delimiter == "" {
		// StringifyOptions predates a StringOption wrapper. Treat its zero value
		// as omitted so &StringifyOptions{} behaves like an empty JS options object.
		delimiter = defaults.Delimiter
	}

	normalized := normalizedStringifyOptions{
		addQueryPrefix:     normalizeBool(options.AddQueryPrefix, false),
		allowDots:          allowDots,
		allowEmptyArrays:   normalizeBool(options.AllowEmptyArrays, false),
		arrayFormat:        options.ArrayFormat,
		charset:            options.Charset,
		charsetSentinel:    normalizeBool(options.CharsetSentinel, false),
		commaRoundTrip:     normalizeBool(options.CommaRoundTrip, false),
		delimiter:          delimiter,
		encode:             normalizeBool(options.Encode, true),
		encodeDotInKeys:    encodeDotInKeys,
		encodeValuesOnly:   normalizeBool(options.EncodeValuesOnly, false),
		format:             options.Format,
		skipNulls:          normalizeBool(options.SkipNulls, false),
		strictNullHandling: normalizeBool(options.StrictNullHandling, false),
	}
	if options.Depth.Set && !options.Depth.Unlimited {
		normalized.depthLimited = true
		normalized.depth = options.Depth.Value
	}
	return normalized, nil
}

func stringifyValue(
	value Value,
	prefix string,
	options normalizedStringifyOptions,
	currentDepth int,
) ([]string, error) {
	if options.depthLimited && currentDepth > options.depth {
		return nil, fmt.Errorf("Input depth exceeded depth option of %d", options.depth)
	}

	switch value.Kind() {
	case KindUndefined:
		return nil, nil
	case KindNull:
		if options.strictNullHandling {
			key, err := stringifyKey(prefix, options)
			if err != nil {
				return nil, err
			}
			return []string{key}, nil
		}
		return stringifyScalar("", prefix, options, false)
	case KindString, KindBool, KindNumber, KindBigInt, KindBytes, KindTime:
		scalar, err := valueString(value)
		if err != nil {
			return nil, err
		}
		return stringifyScalar(scalar, prefix, options, false)
	case KindArray:
		return stringifyArray(value, prefix, options, currentDepth)
	case KindObject:
		return stringifyObject(value, prefix, options, currentDepth)
	default:
		return nil, fmt.Errorf("qsgo: unsupported value kind %d", value.Kind())
	}
}

func stringifyObject(
	value Value,
	prefix string,
	options normalizedStringifyOptions,
	currentDepth int,
) ([]string, error) {
	members, _ := value.Members()
	if len(members) == 0 {
		return nil, nil
	}

	adjustedPrefix := prefix
	if options.encodeDotInKeys {
		adjustedPrefix = strings.ReplaceAll(adjustedPrefix, ".", "%2E")
	}

	var pairs []string
	for _, member := range members {
		if options.skipNulls && member.Value.Kind() == KindNull {
			continue
		}
		encodedKey := member.Key
		if options.allowDots && options.encodeDotInKeys {
			encodedKey = strings.ReplaceAll(encodedKey, ".", "%2E")
		}
		childPrefix := adjustedPrefix + "[" + encodedKey + "]"
		if options.allowDots {
			childPrefix = adjustedPrefix + "." + encodedKey
		}
		encoded, err := stringifyValue(member.Value, childPrefix, options, currentDepth+1)
		if err != nil {
			return nil, err
		}
		pairs = append(pairs, encoded...)
	}
	return pairs, nil
}

func stringifyArray(
	value Value,
	prefix string,
	options normalizedStringifyOptions,
	currentDepth int,
) ([]string, error) {
	elements, _ := value.Elements()
	adjustedPrefix := prefix
	if options.encodeDotInKeys {
		adjustedPrefix = strings.ReplaceAll(adjustedPrefix, ".", "%2E")
	}
	if options.arrayFormat == ArrayFormatComma && options.commaRoundTrip && len(elements) == 1 {
		adjustedPrefix += "[]"
	}
	if options.allowEmptyArrays && len(elements) == 0 {
		// Upstream intentionally bypasses both encoder and formatter here.
		return []string{adjustedPrefix + "[]"}, nil
	}
	if options.arrayFormat == ArrayFormatComma {
		return stringifyCommaArray(elements, adjustedPrefix, options)
	}

	var pairs []string
	for index, element := range elements {
		if !element.Present {
			continue
		}
		if options.skipNulls && element.Value.Kind() == KindNull {
			continue
		}

		var childPrefix string
		switch options.arrayFormat {
		case ArrayFormatIndices:
			childPrefix = adjustedPrefix + "[" + strconv.Itoa(index) + "]"
		case ArrayFormatBrackets:
			childPrefix = adjustedPrefix + "[]"
		case ArrayFormatRepeat:
			childPrefix = adjustedPrefix
		default:
			return nil, fmt.Errorf("qsgo: unsupported array format %d", options.arrayFormat)
		}
		encoded, err := stringifyValue(element.Value, childPrefix, options, currentDepth+1)
		if err != nil {
			return nil, err
		}
		pairs = append(pairs, encoded...)
	}
	return pairs, nil
}

func stringifyCommaArray(
	elements []Element,
	prefix string,
	options normalizedStringifyOptions,
) ([]string, error) {
	if len(elements) == 0 {
		return nil, nil
	}

	parts := make([]string, len(elements))
	for index, element := range elements {
		if !element.Present || element.Value.Kind() == KindUndefined || element.Value.Kind() == KindNull {
			continue
		}
		part, err := arrayJoinString(element.Value)
		if err != nil {
			return nil, err
		}
		if options.encode && options.encodeValuesOnly {
			// qs calls the encoder with only the value in this branch. For the
			// built-in encoder that means UTF-8/RFC3986 defaults, regardless of
			// the outer charset and format.
			part, err = encodeComponent(part, CharsetUTF8, FormatRFC3986)
			if err != nil {
				return nil, err
			}
		}
		parts[index] = part
	}

	joined := strings.Join(parts, ",")
	if joined == "" {
		if options.skipNulls {
			return nil, nil
		}
		if options.strictNullHandling {
			key, err := stringifyKey(prefix, options)
			if err != nil {
				return nil, err
			}
			return []string{key}, nil
		}
	}
	return stringifyScalar(joined, prefix, options, options.encode && options.encodeValuesOnly)
}

func stringifyScalar(
	value string,
	prefix string,
	options normalizedStringifyOptions,
	valueAlreadyEncoded bool,
) ([]string, error) {
	key, err := stringifyKey(prefix, options)
	if err != nil {
		return nil, err
	}

	var encodedValue string
	if valueAlreadyEncoded || !options.encode {
		encodedValue, err = applyFormat(value, options.format)
	} else {
		encodedValue, err = encodeComponent(value, options.charset, options.format)
	}
	if err != nil {
		return nil, err
	}
	return []string{key + "=" + encodedValue}, nil
}

func stringifyKey(prefix string, options normalizedStringifyOptions) (string, error) {
	if options.encode && !options.encodeValuesOnly {
		return encodeComponent(prefix, options.charset, options.format)
	}
	return applyFormat(prefix, options.format)
}

func encodeLiteralDots(value string, enabled bool) string {
	if !enabled {
		return value
	}
	return strings.ReplaceAll(value, ".", "%2E")
}

func valueString(value Value) (string, error) {
	switch value.Kind() {
	case KindString:
		result, _ := value.AsString()
		return result, nil
	case KindBool:
		result, _ := value.AsBool()
		return strconv.FormatBool(result), nil
	case KindNumber:
		result, _ := value.AsNumber()
		return formatJavaScriptNumber(result), nil
	case KindBigInt:
		result, _ := value.AsBigInt()
		return result.String(), nil
	case KindBytes:
		result, _ := value.AsBytes()
		return strings.ToValidUTF8(string(result), "\uFFFD"), nil
	case KindTime:
		result, _ := value.AsTime()
		return formatJavaScriptDate(result), nil
	default:
		return "", fmt.Errorf("qsgo: value kind %d is not scalar", value.Kind())
	}
}

func arrayJoinString(value Value) (string, error) {
	switch value.Kind() {
	case KindUndefined, KindNull:
		return "", nil
	case KindString, KindBool, KindNumber, KindBigInt, KindBytes, KindTime:
		return valueString(value)
	case KindObject:
		return "[object Object]", nil
	case KindArray:
		elements, _ := value.Elements()
		parts := make([]string, len(elements))
		for index, element := range elements {
			if !element.Present {
				continue
			}
			part, err := arrayJoinString(element.Value)
			if err != nil {
				return "", err
			}
			parts[index] = part
		}
		return strings.Join(parts, ","), nil
	default:
		return "", fmt.Errorf("qsgo: unsupported value kind %d", value.Kind())
	}
}

func formatJavaScriptNumber(value float64) string {
	switch {
	case math.IsNaN(value):
		return "NaN"
	case math.IsInf(value, 1):
		return "Infinity"
	case math.IsInf(value, -1):
		return "-Infinity"
	case value == 0:
		return "0"
	}

	absolute := math.Abs(value)
	if absolute >= 1e-6 && absolute < 1e21 {
		return strconv.FormatFloat(value, 'f', -1, 64)
	}
	return normalizeExponent(strconv.FormatFloat(value, 'e', -1, 64))
}

func normalizeExponent(value string) string {
	marker := strings.LastIndexByte(value, 'e')
	if marker < 0 || marker+2 >= len(value) {
		return value
	}
	prefix := value[:marker+1]
	exponent := value[marker+1:]
	sign := ""
	if exponent[0] == '+' || exponent[0] == '-' {
		sign = exponent[:1]
		exponent = exponent[1:]
	}
	exponent = strings.TrimLeft(exponent, "0")
	if exponent == "" {
		exponent = "0"
	}
	return prefix + sign + exponent
}

func formatJavaScriptDate(value time.Time) string {
	utc := value.UTC()
	year := utc.Year()
	var yearString string
	if year >= 0 && year <= 9999 {
		yearString = fmt.Sprintf("%04d", year)
	} else if year < 0 {
		yearString = fmt.Sprintf("-%06d", -year)
	} else {
		yearString = fmt.Sprintf("+%06d", year)
	}
	return fmt.Sprintf(
		"%s-%02d-%02dT%02d:%02d:%02d.%03dZ",
		yearString,
		int(utc.Month()),
		utc.Day(),
		utc.Hour(),
		utc.Minute(),
		utc.Second(),
		utc.Nanosecond()/int(time.Millisecond),
	)
}
