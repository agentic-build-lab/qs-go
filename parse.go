package qsgo

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	utf8CharsetSentinel = "utf8=%E2%9C%93"
	isoCharsetSentinel  = "utf8=%26%2310003%3B"
)

var (
	errInvalidCharset        = errors.New("The charset option must be either utf-8, iso-8859-1, or undefined")
	errInvalidDuplicates     = errors.New("The duplicates option must be either combine, first, or last")
	errInvalidParameterLimit = errors.New("`parameterLimit` must be nonnegative or unlimited")
)

type parseConfig struct {
	allowDots                bool
	allowEmptyArrays         bool
	allowPrototypes          bool
	allowSparse              bool
	arrayLimit               int
	charset                  Charset
	charsetSentinel          bool
	comma                    bool
	decodeDotInKeys          bool
	delimiter                string
	depth                    int
	duplicates               DuplicatePolicy
	ignoreQueryPrefix        bool
	interpretNumericEntities bool
	parameterLimit           int
	parameterUnlimited       bool
	parseArrays              bool
	plainObjects             bool
	strictDepth              bool
	strictMerge              bool
	strictNullHandling       bool
	throwOnLimitExceeded     bool
}

// parseNode is the parser's mutable representation. In particular, overflow
// metadata must travel with nested objects even though it is not observable in
// the public Value returned by Parse.
type parseNode struct {
	kind Kind

	scalar Value

	object      []parseMember
	objectIndex map[string]int

	array []parseElement

	overflow bool
	maxIndex int
}

type parseMember struct {
	key   string
	value *parseNode
}

type parseElement struct {
	present bool
	value   *parseNode
}

type rawValues struct {
	members []parseMember
	index   map[string]int
}

// Parse decodes query using the observable parsing semantics of the frozen qs
// snapshot. A nil options pointer uses DefaultParseOptions.
func Parse(query string, options *ParseOptions) (Value, error) {
	config, err := normalizeParserOptions(options)
	if err != nil {
		return Value{}, err
	}

	if query == "" {
		return NewObject(), nil
	}

	values, err := parseQueryValues(query, config)
	if err != nil {
		return Value{}, err
	}

	result := newObjectNode()
	for _, member := range values.members {
		parsed, parseErr := parseKey(member.key, member.value, config)
		if parseErr != nil {
			return Value{}, parseErr
		}
		if parsed == nil {
			continue
		}

		result, err = mergeNodes(result, parsed, config)
		if err != nil {
			return Value{}, err
		}
	}

	if !config.allowSparse {
		compactNode(result)
	}

	return nodeToValue(result), nil
}

func normalizeParserOptions(options *ParseOptions) (parseConfig, error) {
	config := parseConfig{
		allowDots:                false,
		allowEmptyArrays:         false,
		allowPrototypes:          false,
		allowSparse:              false,
		arrayLimit:               20,
		charset:                  CharsetUTF8,
		charsetSentinel:          false,
		comma:                    false,
		decodeDotInKeys:          false,
		delimiter:                "&",
		depth:                    5,
		duplicates:               DuplicatesCombine,
		ignoreQueryPrefix:        false,
		interpretNumericEntities: false,
		parameterLimit:           1000,
		parseArrays:              true,
		plainObjects:             false,
		strictDepth:              false,
		strictMerge:              true,
		strictNullHandling:       false,
		throwOnLimitExceeded:     false,
	}
	if options == nil {
		return config, nil
	}

	config.allowDots = normalizeBool(options.AllowDots, config.allowDots)
	config.allowEmptyArrays = normalizeBool(options.AllowEmptyArrays, config.allowEmptyArrays)
	config.allowPrototypes = normalizeBool(options.AllowPrototypes, config.allowPrototypes)
	config.allowSparse = normalizeBool(options.AllowSparse, config.allowSparse)
	if options.ArrayLimit.Set {
		config.arrayLimit = options.ArrayLimit.Value
	}
	if options.Charset != CharsetUTF8 && options.Charset != CharsetISO88591 {
		return parseConfig{}, errInvalidCharset
	}
	config.charset = options.Charset
	config.charsetSentinel = normalizeBool(options.CharsetSentinel, config.charsetSentinel)
	config.comma = normalizeBool(options.Comma, config.comma)
	config.decodeDotInKeys = normalizeBool(options.DecodeDotInKeys, config.decodeDotInKeys)
	if options.Delimiter != "" {
		config.delimiter = options.Delimiter
	}
	if options.Depth.Set {
		config.depth = options.Depth.Value
	}
	if options.Duplicates > DuplicatesLast {
		return parseConfig{}, errInvalidDuplicates
	}
	config.duplicates = options.Duplicates
	config.ignoreQueryPrefix = normalizeBool(options.IgnoreQueryPrefix, config.ignoreQueryPrefix)
	config.interpretNumericEntities = normalizeBool(options.InterpretNumericEntities, config.interpretNumericEntities)
	if options.ParameterLimit.Set {
		if options.ParameterLimit.Unlimited {
			config.parameterUnlimited = true
		} else {
			if options.ParameterLimit.Value < 0 {
				return parseConfig{}, errInvalidParameterLimit
			}
			config.parameterLimit = options.ParameterLimit.Value
		}
	}
	config.parseArrays = normalizeBool(options.ParseArrays, config.parseArrays)
	config.plainObjects = normalizeBool(options.PlainObjects, config.plainObjects)
	config.strictDepth = normalizeBool(options.StrictDepth, config.strictDepth)
	config.strictMerge = normalizeBool(options.StrictMerge, config.strictMerge)
	config.strictNullHandling = normalizeBool(options.StrictNullHandling, config.strictNullHandling)
	config.throwOnLimitExceeded = normalizeBool(options.ThrowOnLimitExceeded, config.throwOnLimitExceeded)

	// Upstream implicitly enables dot parsing only when AllowDots was omitted.
	if config.decodeDotInKeys && !options.AllowDots.Set {
		config.allowDots = true
	}

	return config, nil
}

func parseQueryValues(query string, config parseConfig) (rawValues, error) {
	clean := query
	if config.ignoreQueryPrefix && strings.HasPrefix(clean, "?") {
		clean = clean[1:]
	}
	clean = decodeEncodedBrackets(clean)

	parts, err := splitParameters(clean, config)
	if err != nil {
		return rawValues{}, err
	}

	charset := config.charset
	skipIndex := -1
	if config.charsetSentinel {
		for index, part := range parts {
			if !strings.HasPrefix(part, "utf8=") {
				continue
			}
			switch part {
			case utf8CharsetSentinel:
				charset = CharsetUTF8
			case isoCharsetSentinel:
				charset = CharsetISO88591
			}
			skipIndex = index
			break
		}
	}

	values := rawValues{index: make(map[string]int)}
	for index, part := range parts {
		if index == skipIndex {
			continue
		}

		bracketEquals := strings.Index(part, "]=")
		equals := strings.IndexByte(part, '=')
		if bracketEquals >= 0 {
			equals = bracketEquals + 1
		}

		var key string
		var value *parseNode
		if equals < 0 {
			key = decodeToken(part, charset)
			if config.strictNullHandling {
				value = newScalarNode(NewNull())
			} else {
				value = newStringNode("")
			}
		} else {
			key = decodeToken(part[:equals], charset)
			existingLength := 0
			if existing, found := values.get(key); found && existing.kind == KindArray {
				existingLength = len(existing.array)
			}

			value, err = parseRawValue(
				part[equals+1:],
				existingLength,
				!strings.Contains(part, "[]="),
				charset,
				config,
			)
			if err != nil {
				return rawValues{}, err
			}
		}

		if config.interpretNumericEntities && charset == CharsetISO88591 && nodeTruthy(value) {
			value = newStringNode(interpretEntities(nodeToJSString(value)))
		}

		bracketArray := strings.Contains(part, "[]=")
		if bracketArray && value.kind == KindArray {
			value = newArrayNode(parseElement{present: true, value: value})
		}

		if config.comma && value.kind == KindArray && len(value.array) > config.arrayLimit {
			value, err = combineNodes(newArrayNode(), value, config)
			if err != nil {
				return rawValues{}, err
			}
		}

		if existing, found := values.get(key); found && (config.duplicates == DuplicatesCombine || bracketArray) {
			combined, combineErr := combineNodes(existing, value, config)
			if combineErr != nil {
				return rawValues{}, combineErr
			}
			values.replace(key, combined)
		} else if !found || config.duplicates == DuplicatesLast {
			values.replace(key, value)
		}
	}

	return values, nil
}

func splitParameters(query string, config parseConfig) ([]string, error) {
	if config.parameterUnlimited {
		return strings.Split(query, config.delimiter), nil
	}

	if config.throwOnLimitExceeded {
		parts := strings.Split(query, config.delimiter)
		if len(parts) > config.parameterLimit {
			return nil, parameterLimitError(config.parameterLimit)
		}
		return parts, nil
	}

	if config.parameterLimit == 0 {
		return nil, nil
	}
	parts := strings.Split(query, config.delimiter)
	if len(parts) > config.parameterLimit {
		parts = parts[:config.parameterLimit]
	}
	return parts, nil
}

func parseRawValue(raw string, currentArrayLength int, flatComma bool, charset Charset, config parseConfig) (*parseNode, error) {
	if raw != "" && config.comma && strings.Contains(raw, ",") {
		parts := strings.Split(raw, ",")
		if flatComma && config.throwOnLimitExceeded && len(parts) > config.arrayLimit {
			return nil, arrayLimitError(config.arrayLimit)
		}
		elements := make([]parseElement, len(parts))
		for index, part := range parts {
			elements[index] = parseElement{present: true, value: newStringNode(decodeToken(part, charset))}
		}
		return newArrayNode(elements...), nil
	}

	if config.throwOnLimitExceeded && currentArrayLength >= config.arrayLimit {
		return nil, arrayLimitError(config.arrayLimit)
	}
	return newStringNode(decodeToken(raw, charset)), nil
}

func parseKey(key string, value *parseNode, config parseConfig) (*parseNode, error) {
	if key == "" {
		return nil, nil
	}
	segments, allowed, err := splitKeySegments(key, config)
	if err != nil || !allowed {
		return nil, err
	}
	if len(segments) == 0 {
		return nil, nil
	}
	return parseObject(segments, value, config)
}

func splitKeySegments(original string, config parseConfig) ([]string, bool, error) {
	key := original
	if config.allowDots {
		key = expandDots(key)
	}

	if config.depth <= 0 {
		if prototypeBlocked(key, config) {
			return nil, false, nil
		}
		return []string{key}, true, nil
	}

	segments := make([]string, 0, config.depth+1)
	first := strings.IndexByte(key, '[')
	parent := key
	if first >= 0 {
		parent = key[:first]
	}
	if parent != "" {
		if prototypeBlocked(parent, config) {
			return nil, false, nil
		}
		segments = append(segments, parent)
	}

	open := first
	collected := 0
	for open >= 0 && collected < config.depth {
		level := 1
		closeIndex := -1
		for index := open + 1; index < len(key); index++ {
			switch key[index] {
			case '[':
				level++
			case ']':
				level--
				if level == 0 {
					closeIndex = index
				}
			}
			if closeIndex >= 0 {
				break
			}
		}

		if closeIndex < 0 {
			segments = append(segments, "["+key[open:]+"]")
			return segments, true, nil
		}

		segment := key[open : closeIndex+1]
		content := segment[1 : len(segment)-1]
		if prototypeBlocked(content, config) {
			return nil, false, nil
		}
		segments = append(segments, segment)
		collected++

		next := strings.IndexByte(key[closeIndex+1:], '[')
		if next < 0 {
			open = -1
		} else {
			open = closeIndex + 1 + next
		}
	}

	if open >= 0 {
		if config.strictDepth {
			return nil, false, fmt.Errorf("Input depth exceeded depth option of %d and strictDepth is true", config.depth)
		}
		segments = append(segments, "["+key[open:]+"]")
	}

	return segments, true, nil
}

func parseObject(segments []string, leaf *parseNode, config parseConfig) (*parseNode, error) {
	for index := len(segments) - 1; index >= 0; index-- {
		root := segments[index]
		if root == "[]" && config.parseArrays {
			if leaf.overflow {
				continue
			}
			if config.allowEmptyArrays && (isEmptyString(leaf) || (config.strictNullHandling && leaf.kind == KindNull)) {
				leaf = newArrayNode()
				continue
			}
			combined, err := combineNodes(newArrayNode(), leaf, config)
			if err != nil {
				return nil, err
			}
			leaf = combined
			continue
		}

		object := newObjectNode()
		cleanRoot := root
		if len(root) >= 2 && root[0] == '[' && root[len(root)-1] == ']' {
			cleanRoot = root[1 : len(root)-1]
		}
		decodedRoot := cleanRoot
		if config.decodeDotInKeys {
			decodedRoot = strings.ReplaceAll(decodedRoot, "%2E", ".")
		}

		arrayIndex, validIndex := canonicalArrayIndex(decodedRoot)
		validIndex = validIndex && root != decodedRoot && config.parseArrays
		switch {
		case !config.parseArrays && decodedRoot == "":
			object.set("0", leaf)
		case validIndex && arrayIndex < config.arrayLimit:
			elements := make([]parseElement, arrayIndex+1)
			elements[arrayIndex] = parseElement{present: true, value: leaf}
			leaf = newArrayNode(elements...)
			continue
		case validIndex && config.throwOnLimitExceeded:
			return nil, arrayLimitError(config.arrayLimit)
		case validIndex:
			object.set(strconv.Itoa(arrayIndex), leaf)
			object.overflow = true
			object.maxIndex = arrayIndex
		case decodedRoot != "__proto__":
			object.set(decodedRoot, leaf)
		}
		leaf = object
	}
	return leaf, nil
}

func combineNodes(left, right *parseNode, config parseConfig) (*parseNode, error) {
	if left.kind == KindObject && left.overflow {
		if config.throwOnLimitExceeded {
			return nil, arrayLimitError(config.arrayLimit)
		}
		left.maxIndex++
		left.set(strconv.Itoa(left.maxIndex), right)
		return left, nil
	}

	elements := make([]parseElement, 0, nodeConcatLength(left)+nodeConcatLength(right))
	elements = appendConcatOperand(elements, left)
	elements = appendConcatOperand(elements, right)
	if len(elements) > config.arrayLimit {
		if config.throwOnLimitExceeded {
			return nil, arrayLimitError(config.arrayLimit)
		}
		return arrayToObjectWithOverflow(elements, len(elements)-1), nil
	}
	return newArrayNode(elements...), nil
}

func mergeNodes(target, source *parseNode, config parseConfig) (*parseNode, error) {
	if source == nil {
		return target, nil
	}
	if target == nil {
		return source, nil
	}

	if !source.objectLike() {
		switch target.kind {
		case KindArray:
			nextIndex := len(target.array)
			if nextIndex >= config.arrayLimit {
				if config.throwOnLimitExceeded {
					return nil, arrayLimitError(config.arrayLimit)
				}
				elements := append(append([]parseElement(nil), target.array...), parseElement{present: true, value: source})
				return arrayToObjectWithOverflow(elements, nextIndex), nil
			}
			target.array = append(target.array, parseElement{present: true, value: source})
			return target, nil
		case KindObject:
			if target.overflow {
				target.maxIndex++
				target.set(strconv.Itoa(target.maxIndex), source)
				return target, nil
			}
			if config.strictMerge {
				return newArrayNode(
					parseElement{present: true, value: target},
					parseElement{present: true, value: source},
				), nil
			}
			property := nodePropertyString(source)
			if config.plainObjects || config.allowPrototypes || !isPrototypeProperty(property) {
				target.set(property, newScalarNode(NewBool(true)))
			}
			return target, nil
		default:
			return newArrayNode(
				parseElement{present: true, value: target},
				parseElement{present: true, value: source},
			), nil
		}
	}

	if !target.objectLike() {
		if source.kind == KindObject && source.overflow {
			result := newObjectNode()
			result.set("0", target)
			for _, member := range source.object {
				index, ok := canonicalArrayIndex(member.key)
				if !ok {
					continue
				}
				result.set(strconv.Itoa(index+1), member.value)
			}
			result.overflow = true
			result.maxIndex = source.maxIndex + 1
			return result, nil
		}

		elements := []parseElement{{present: true, value: target}}
		elements = appendConcatOperand(elements, source)
		if len(elements) > config.arrayLimit {
			if config.throwOnLimitExceeded {
				return nil, arrayLimitError(config.arrayLimit)
			}
			return arrayToObjectWithOverflow(elements, len(elements)-1), nil
		}
		return newArrayNode(elements...), nil
	}

	if target.kind == KindArray && source.kind == KindObject {
		target = arrayToObject(target.array)
	}

	if target.kind == KindArray && source.kind == KindArray {
		for index, sourceElement := range source.array {
			if !sourceElement.present {
				continue
			}
			if index < len(target.array) && target.array[index].present {
				targetValue := target.array[index].value
				if targetValue.objectLike() && sourceElement.value.objectLike() {
					merged, err := mergeNodes(targetValue, sourceElement.value, config)
					if err != nil {
						return nil, err
					}
					target.array[index].value = merged
				} else {
					target.array = append(target.array, sourceElement)
				}
			} else {
				for len(target.array) <= index {
					target.array = append(target.array, parseElement{})
				}
				target.array[index] = sourceElement
			}
		}
		if len(target.array) > config.arrayLimit {
			if config.throwOnLimitExceeded {
				return nil, arrayLimitError(config.arrayLimit)
			}
			return arrayToObjectWithOverflow(target.array, len(target.array)-1), nil
		}
		return target, nil
	}

	if source.kind == KindArray {
		for index, element := range source.array {
			if !element.present {
				continue
			}
			var err error
			target, err = mergeObjectProperty(target, strconv.Itoa(index), element.value, config)
			if err != nil {
				return nil, err
			}
		}
	} else {
		for _, member := range source.object {
			var err error
			target, err = mergeObjectProperty(target, member.key, member.value, config)
			if err != nil {
				return nil, err
			}
		}
	}

	if source.kind == KindObject && source.overflow && len(source.object) > 0 {
		if !target.overflow || source.maxIndex > target.maxIndex {
			target.maxIndex = source.maxIndex
		}
		target.overflow = true
	}
	return target, nil
}

func mergeObjectProperty(target *parseNode, key string, value *parseNode, config parseConfig) (*parseNode, error) {
	if existing, found := target.get(key); found {
		merged, err := mergeNodes(existing, value, config)
		if err != nil {
			return nil, err
		}
		target.set(key, merged)
	} else {
		target.set(key, value)
	}
	if target.overflow {
		if index, ok := canonicalArrayIndex(key); ok && index > target.maxIndex {
			target.maxIndex = index
		}
	}
	return target, nil
}

func newScalarNode(value Value) *parseNode {
	return &parseNode{kind: value.Kind(), scalar: value}
}

func newStringNode(value string) *parseNode {
	return newScalarNode(NewString(value))
}

func newObjectNode() *parseNode {
	return &parseNode{kind: KindObject, objectIndex: make(map[string]int)}
}

func newArrayNode(elements ...parseElement) *parseNode {
	return &parseNode{kind: KindArray, array: append([]parseElement(nil), elements...)}
}

func (node *parseNode) objectLike() bool {
	return node != nil && (node.kind == KindObject || node.kind == KindArray)
}

func (node *parseNode) set(key string, value *parseNode) {
	if node.objectIndex == nil {
		node.objectIndex = make(map[string]int)
	}
	if index, found := node.objectIndex[key]; found {
		node.object[index].value = value
		return
	}
	node.objectIndex[key] = len(node.object)
	node.object = append(node.object, parseMember{key: key, value: value})
}

func (node *parseNode) get(key string) (*parseNode, bool) {
	if node == nil || node.kind != KindObject {
		return nil, false
	}
	index, found := node.objectIndex[key]
	if !found {
		return nil, false
	}
	return node.object[index].value, true
}

func (values *rawValues) get(key string) (*parseNode, bool) {
	index, found := values.index[key]
	if !found {
		return nil, false
	}
	return values.members[index].value, true
}

func (values *rawValues) replace(key string, value *parseNode) {
	if index, found := values.index[key]; found {
		values.members[index].value = value
		return
	}
	values.index[key] = len(values.members)
	values.members = append(values.members, parseMember{key: key, value: value})
}

func nodeConcatLength(node *parseNode) int {
	if node.kind == KindArray {
		return len(node.array)
	}
	return 1
}

func appendConcatOperand(elements []parseElement, node *parseNode) []parseElement {
	if node.kind == KindArray {
		return append(elements, node.array...)
	}
	return append(elements, parseElement{present: true, value: node})
}

func arrayToObject(elements []parseElement) *parseNode {
	object := newObjectNode()
	for index, element := range elements {
		if element.present {
			object.set(strconv.Itoa(index), element.value)
		}
	}
	return object
}

func arrayToObjectWithOverflow(elements []parseElement, maxIndex int) *parseNode {
	object := arrayToObject(elements)
	object.overflow = true
	object.maxIndex = maxIndex
	return object
}

func compactNode(node *parseNode) {
	if node == nil {
		return
	}
	switch node.kind {
	case KindObject:
		for _, member := range node.object {
			compactNode(member.value)
		}
	case KindArray:
		compacted := make([]parseElement, 0, len(node.array))
		for _, element := range node.array {
			if !element.present {
				continue
			}
			compactNode(element.value)
			compacted = append(compacted, element)
		}
		node.array = compacted
	}
}

func nodeToValue(node *parseNode) Value {
	if node == nil {
		return NewUndefined()
	}
	switch node.kind {
	case KindObject:
		members := make([]Member, len(node.object))
		for index, member := range node.object {
			members[index] = Member{Key: member.key, Value: nodeToValue(member.value)}
		}
		return NewObject(members...)
	case KindArray:
		if len(node.array) == 0 {
			return NewArray()
		}
		elements := make([]Element, len(node.array))
		for index, element := range node.array {
			elements[index].Present = element.present
			if element.present {
				elements[index].Value = nodeToValue(element.value)
			}
		}
		return NewSparseArray(elements...)
	default:
		return node.scalar
	}
}

func decodeEncodedBrackets(value string) string {
	var builder strings.Builder
	builder.Grow(len(value))
	for index := 0; index < len(value); {
		if index+2 < len(value) && value[index] == '%' {
			first := value[index+1]
			second := value[index+2]
			if first == '5' && (second == 'B' || second == 'b') {
				builder.WriteByte('[')
				index += 3
				continue
			}
			if first == '5' && (second == 'D' || second == 'd') {
				builder.WriteByte(']')
				index += 3
				continue
			}
		}
		builder.WriteByte(value[index])
		index++
	}
	return builder.String()
}

func decodeToken(value string, charset Charset) string {
	value = strings.ReplaceAll(value, "+", " ")
	if charset == CharsetISO88591 {
		var builder strings.Builder
		builder.Grow(len(value))
		for index := 0; index < len(value); {
			if index+2 < len(value) && value[index] == '%' {
				if decoded, ok := decodeHexByte(value[index+1], value[index+2]); ok {
					builder.WriteRune(rune(decoded))
					index += 3
					continue
				}
			}
			r, size := utf8.DecodeRuneInString(value[index:])
			if r == utf8.RuneError && size == 1 {
				builder.WriteByte(value[index])
				index++
			} else {
				builder.WriteRune(r)
				index += size
			}
		}
		return builder.String()
	}

	decoded := make([]byte, 0, len(value))
	for index := 0; index < len(value); {
		if value[index] == '%' {
			if index+2 >= len(value) {
				return value
			}
			byteValue, ok := decodeHexByte(value[index+1], value[index+2])
			if !ok {
				return value
			}
			decoded = append(decoded, byteValue)
			index += 3
			continue
		}
		decoded = append(decoded, value[index])
		index++
	}
	if !utf8.Valid(decoded) {
		return value
	}
	return string(decoded)
}

func decodeHexByte(first, second byte) (byte, bool) {
	high, ok := hexNibble(first)
	if !ok {
		return 0, false
	}
	low, ok := hexNibble(second)
	if !ok {
		return 0, false
	}
	return high<<4 | low, true
}

func hexNibble(value byte) (byte, bool) {
	switch {
	case value >= '0' && value <= '9':
		return value - '0', true
	case value >= 'A' && value <= 'F':
		return value - 'A' + 10, true
	case value >= 'a' && value <= 'f':
		return value - 'a' + 10, true
	default:
		return 0, false
	}
}

func expandDots(key string) string {
	var builder strings.Builder
	builder.Grow(len(key))
	for index := 0; index < len(key); {
		if key[index] == '.' && index+1 < len(key) && key[index+1] != '.' && key[index+1] != '[' {
			end := index + 1
			for end < len(key) && key[end] != '.' && key[end] != '[' {
				end++
			}
			builder.WriteByte('[')
			builder.WriteString(key[index+1 : end])
			builder.WriteByte(']')
			index = end
			continue
		}
		builder.WriteByte(key[index])
		index++
	}
	return builder.String()
}

func canonicalArrayIndex(value string) (int, bool) {
	if value == "" {
		return 0, false
	}
	index, err := strconv.Atoi(value)
	if err != nil || index < 0 || strconv.Itoa(index) != value {
		return 0, false
	}
	return index, true
}

func prototypeBlocked(value string, config parseConfig) bool {
	return !config.plainObjects && !config.allowPrototypes && isPrototypeProperty(value)
}

func isPrototypeProperty(value string) bool {
	switch value {
	case "__defineGetter__", "__defineSetter__", "hasOwnProperty", "__lookupGetter__", "__lookupSetter__",
		"isPrototypeOf", "propertyIsEnumerable", "toLocaleString", "toString", "valueOf", "constructor", "__proto__":
		return true
	default:
		return false
	}
}

func nodeTruthy(node *parseNode) bool {
	if node == nil {
		return false
	}
	switch node.kind {
	case KindNull, KindUndefined:
		return false
	case KindString:
		value, _ := node.scalar.AsString()
		return value != ""
	case KindBool:
		value, _ := node.scalar.AsBool()
		return value
	case KindNumber:
		value, _ := node.scalar.AsNumber()
		return value != 0
	default:
		return true
	}
}

func nodeToJSString(node *parseNode) string {
	switch node.kind {
	case KindNull, KindUndefined:
		return ""
	case KindString:
		value, _ := node.scalar.AsString()
		return value
	case KindBool:
		value, _ := node.scalar.AsBool()
		return strconv.FormatBool(value)
	case KindNumber:
		value, _ := node.scalar.AsNumber()
		return strconv.FormatFloat(value, 'g', -1, 64)
	case KindArray:
		parts := make([]string, len(node.array))
		for index, element := range node.array {
			if element.present {
				parts[index] = nodeToJSString(element.value)
			}
		}
		return strings.Join(parts, ",")
	default:
		return "[object Object]"
	}
}

func nodePropertyString(node *parseNode) string {
	if node.kind == KindNull {
		return "null"
	}
	return nodeToJSString(node)
}

func interpretEntities(value string) string {
	var builder strings.Builder
	for index := 0; index < len(value); {
		if index+3 < len(value) && value[index] == '&' && value[index+1] == '#' {
			end := index + 2
			codeUnit := 0
			digits := 0
			for end < len(value) && value[end] >= '0' && value[end] <= '9' {
				codeUnit = (codeUnit*10 + int(value[end]-'0')) & 0xFFFF
				end++
				digits++
			}
			if digits > 0 && end < len(value) && value[end] == ';' {
				builder.WriteRune(rune(codeUnit))
				index = end + 1
				continue
			}
		}
		builder.WriteByte(value[index])
		index++
	}
	return builder.String()
}

func isEmptyString(node *parseNode) bool {
	if node == nil || node.kind != KindString {
		return false
	}
	value, _ := node.scalar.AsString()
	return value == ""
}

func arrayLimitError(limit int) error {
	word := "elements"
	if limit == 1 {
		word = "element"
	}
	return fmt.Errorf("Array limit exceeded. Only %d %s allowed in an array.", limit, word)
}

func parameterLimitError(limit int) error {
	word := "parameters"
	if limit == 1 {
		word = "parameter"
	}
	return fmt.Errorf("Parameter limit exceeded. Only %d %s allowed.", limit, word)
}
