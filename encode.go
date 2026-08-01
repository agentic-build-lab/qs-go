package qsgo

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

const upperHex = "0123456789ABCDEF"

func encodeComponent(input string, charset Charset, format Format) (string, error) {
	if input == "" {
		return "", nil
	}

	var encoded string
	switch charset {
	case CharsetUTF8:
		encoded = encodeUTF8(input, format)
	case CharsetISO88591:
		encoded = encodeISO88591(input)
	default:
		return "", fmt.Errorf("qsgo: unsupported charset %d", charset)
	}
	return applyFormat(encoded, format)
}

func applyFormat(value string, format Format) (string, error) {
	switch format {
	case FormatRFC3986:
		return value, nil
	case FormatRFC1738:
		return strings.ReplaceAll(value, "%20", "+"), nil
	default:
		return "", fmt.Errorf("qsgo: unsupported format %d", format)
	}
}

func encodeUTF8(input string, format Format) string {
	var output strings.Builder
	output.Grow(len(input))

	for len(input) > 0 {
		r, size := utf8.DecodeRuneInString(input)
		if r == utf8.RuneError && size == 1 {
			r = utf8.RuneError
		}
		input = input[size:]

		if r < utf8.RuneSelf && isSafeASCII(byte(r), format) {
			output.WriteByte(byte(r))
			continue
		}

		var bytes [utf8.UTFMax]byte
		count := utf8.EncodeRune(bytes[:], r)
		for index := 0; index < count; index++ {
			writePercentByte(&output, bytes[index])
		}
	}
	return output.String()
}

func encodeISO88591(input string) string {
	var output strings.Builder
	output.Grow(len(input))

	for len(input) > 0 {
		r, size := utf8.DecodeRuneInString(input)
		if r == utf8.RuneError && size == 1 {
			r = utf8.RuneError
		}
		input = input[size:]

		switch {
		case r <= 0xFF:
			value := byte(r)
			if isJSEscapeSafe(value) {
				output.WriteByte(value)
			} else {
				writePercentByte(&output, value)
			}
		case r <= 0xFFFF:
			writeNumericEntity(&output, uint16(r))
		default:
			high, low := utf16.EncodeRune(r)
			writeNumericEntity(&output, uint16(high))
			writeNumericEntity(&output, uint16(low))
		}
	}
	return output.String()
}

func isSafeASCII(value byte, format Format) bool {
	if (value >= 'a' && value <= 'z') ||
		(value >= 'A' && value <= 'Z') ||
		(value >= '0' && value <= '9') {
		return true
	}
	switch value {
	case '-', '.', '_', '~':
		return true
	case '(', ')':
		return format == FormatRFC1738
	default:
		return false
	}
}

// JavaScript's legacy escape function is the observable ISO-8859-1 encoder
// used by qs. Its safe set differs from both RFC 3986 and encodeURIComponent.
func isJSEscapeSafe(value byte) bool {
	if (value >= 'a' && value <= 'z') ||
		(value >= 'A' && value <= 'Z') ||
		(value >= '0' && value <= '9') {
		return true
	}
	switch value {
	case '@', '*', '_', '+', '-', '.', '/':
		return true
	default:
		return false
	}
}

func writePercentByte(output *strings.Builder, value byte) {
	output.WriteByte('%')
	output.WriteByte(upperHex[value>>4])
	output.WriteByte(upperHex[value&0x0F])
}

func writeNumericEntity(output *strings.Builder, value uint16) {
	output.WriteString("%26%23")
	output.WriteString(strconv.FormatUint(uint64(value), 10))
	output.WriteString("%3B")
}
