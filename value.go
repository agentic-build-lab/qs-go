package qsgo

import (
	"errors"
	"math/big"
	"time"
)

// Kind identifies one member of the closed Value algebra.
type Kind uint8

const (
	KindUndefined Kind = iota
	KindNull
	KindString
	KindBool
	KindNumber
	KindBigInt
	KindBytes
	KindTime
	KindObject
	KindArray
)

var errInvalidBigInt = errors.New("qsgo: invalid base-10 bigint")

// Member is one ordered object property.
type Member struct {
	Key   string
	Value Value
}

// Element is one array slot. Present=false represents a sparse hole; its Value
// is ignored. Explicit undefined and null use Present=true with their own Kind.
type Element struct {
	Present bool
	Value   Value
}

// Value is a closed, typed representation of JavaScript values supported by
// qs. Its fields are private so invalid cross-kind states cannot be constructed.
type Value struct {
	kind        Kind
	stringValue string
	boolValue   bool
	numberValue float64
	bigIntValue *big.Int
	bytesValue  []byte
	timeValue   time.Time
	objectValue []Member
	objectIndex map[string]int
	arrayValue  []Element
}

func NewUndefined() Value { return Value{kind: KindUndefined} }
func NewNull() Value      { return Value{kind: KindNull} }
func NewString(value string) Value {
	return Value{kind: KindString, stringValue: value}
}
func NewBool(value bool) Value { return Value{kind: KindBool, boolValue: value} }
func NewNumber(value float64) Value {
	return Value{kind: KindNumber, numberValue: value}
}

func NewBigInt(value *big.Int) Value {
	if value == nil {
		return Value{kind: KindBigInt, bigIntValue: new(big.Int)}
	}
	return Value{kind: KindBigInt, bigIntValue: new(big.Int).Set(value)}
}

func NewBigIntDecimal(value string) (Value, error) {
	parsed, ok := new(big.Int).SetString(value, 10)
	if !ok {
		return Value{}, errInvalidBigInt
	}
	return NewBigInt(parsed), nil
}

func NewBytes(value []byte) Value {
	return Value{kind: KindBytes, bytesValue: append([]byte(nil), value...)}
}

func NewTime(value time.Time) Value {
	return Value{kind: KindTime, timeValue: value}
}

// NewObject retains first-insertion order. Repeated keys replace the earlier
// value in place, matching assignment to an existing JavaScript property.
func NewObject(members ...Member) Value {
	result := Value{kind: KindObject, objectIndex: make(map[string]int, len(members))}
	for _, member := range members {
		result.setMember(member.Key, member.Value)
	}
	return result
}

func NewArray(values ...Value) Value {
	elements := make([]Element, len(values))
	for index, value := range values {
		elements[index] = Element{Present: true, Value: value}
	}
	return Value{kind: KindArray, arrayValue: elements}
}

func NewSparseArray(elements ...Element) Value {
	return Value{kind: KindArray, arrayValue: append([]Element(nil), elements...)}
}

func (value Value) Kind() Kind { return value.kind }

func (value Value) AsString() (string, bool) {
	return value.stringValue, value.kind == KindString
}

func (value Value) AsBool() (bool, bool) {
	return value.boolValue, value.kind == KindBool
}

func (value Value) AsNumber() (float64, bool) {
	return value.numberValue, value.kind == KindNumber
}

func (value Value) AsBigInt() (*big.Int, bool) {
	if value.kind != KindBigInt {
		return nil, false
	}
	if value.bigIntValue == nil {
		return new(big.Int), true
	}
	return new(big.Int).Set(value.bigIntValue), true
}

func (value Value) AsBytes() ([]byte, bool) {
	if value.kind != KindBytes {
		return nil, false
	}
	return append([]byte(nil), value.bytesValue...), true
}

func (value Value) AsTime() (time.Time, bool) {
	return value.timeValue, value.kind == KindTime
}

func (value Value) Members() ([]Member, bool) {
	if value.kind != KindObject {
		return nil, false
	}
	return append([]Member(nil), value.objectValue...), true
}

func (value Value) Elements() ([]Element, bool) {
	if value.kind != KindArray {
		return nil, false
	}
	return append([]Element(nil), value.arrayValue...), true
}

func (value *Value) setMember(key string, memberValue Value) {
	if value.kind != KindObject {
		*value = NewObject()
	}
	if value.objectIndex == nil {
		value.rebuildObjectIndex()
	}
	if index, exists := value.objectIndex[key]; exists {
		value.objectValue[index].Value = memberValue
		return
	}
	value.objectIndex[key] = len(value.objectValue)
	value.objectValue = append(value.objectValue, Member{Key: key, Value: memberValue})
}

func (value *Value) member(key string) (*Value, bool) {
	if value.kind != KindObject {
		return nil, false
	}
	if value.objectIndex == nil {
		value.rebuildObjectIndex()
	}
	index, exists := value.objectIndex[key]
	if !exists {
		return nil, false
	}
	return &value.objectValue[index].Value, true
}

func (value *Value) rebuildObjectIndex() {
	value.objectIndex = make(map[string]int, len(value.objectValue))
	for index, member := range value.objectValue {
		value.objectIndex[member.Key] = index
	}
}
