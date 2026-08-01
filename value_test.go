package qsgo

import (
	"math/big"
	"testing"
)

func TestObjectPreservesInsertionOrderAndReplacesInPlace(t *testing.T) {
	value := NewObject(
		Member{Key: "b", Value: NewString("first")},
		Member{Key: "a", Value: NewNumber(2)},
		Member{Key: "b", Value: NewString("last")},
	)

	members, ok := value.Members()
	if !ok {
		t.Fatal("expected object")
	}
	if len(members) != 2 || members[0].Key != "b" || members[1].Key != "a" {
		t.Fatalf("unexpected ordered members: %#v", members)
	}
	if actual, _ := members[0].Value.AsString(); actual != "last" {
		t.Fatalf("replacement = %q, want last", actual)
	}
}

func TestObjectUsesJavaScriptIntegerPropertyOrder(t *testing.T) {
	value := NewObject(
		Member{Key: "later", Value: NewString("ordinary-first")},
		Member{Key: "3", Value: NewString("three")},
		Member{Key: "1", Value: NewString("one")},
		Member{Key: "01", Value: NewString("not-an-index")},
		Member{Key: "4294967295", Value: NewString("outside-index-range")},
	)
	members, _ := value.Members()
	keys := make([]string, len(members))
	for index, member := range members {
		keys[index] = member.Key
	}
	expected := []string{"1", "3", "later", "01", "4294967295"}
	for index := range expected {
		if keys[index] != expected[index] {
			t.Fatalf("keys = %#v, want %#v", keys, expected)
		}
	}
}

func TestSparseArraySeparatesHoleUndefinedAndNull(t *testing.T) {
	value := NewSparseArray(
		Element{},
		Element{Present: true, Value: NewUndefined()},
		Element{Present: true, Value: NewNull()},
	)
	elements, ok := value.Elements()
	if !ok || len(elements) != 3 {
		t.Fatalf("unexpected elements: %#v", elements)
	}
	if elements[0].Present || !elements[1].Present || !elements[2].Present {
		t.Fatalf("presence flags lost: %#v", elements)
	}
	if elements[1].Value.Kind() != KindUndefined || elements[2].Value.Kind() != KindNull {
		t.Fatalf("value kinds lost: %#v", elements)
	}
}

func TestBigIntConstructorCopiesInput(t *testing.T) {
	source := big.NewInt(42)
	value := NewBigInt(source)
	source.SetInt64(9)
	actual, ok := value.AsBigInt()
	if !ok || actual.String() != "42" {
		t.Fatalf("bigint = %v, %v", actual, ok)
	}
}
