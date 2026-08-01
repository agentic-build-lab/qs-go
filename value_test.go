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
