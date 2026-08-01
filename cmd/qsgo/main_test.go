package main

import (
	"testing"

	qsgo "github.com/agentic-build-lab/qs-go"
)

func TestEncodeJSONPreservesObjectOrderAndSparseLength(t *testing.T) {
	value := qsgo.NewObject(
		qsgo.Member{Key: "b", Value: qsgo.NewString("two")},
		qsgo.Member{Key: "a", Value: qsgo.NewSparseArray(
			qsgo.Element{},
			qsgo.Element{Present: true, Value: qsgo.NewNull()},
		)},
	)
	actual, err := encodeJSON(value)
	if err != nil {
		t.Fatal(err)
	}
	if actual != `{"b":"two","a":[null,null]}` {
		t.Fatalf("encodeJSON = %s", actual)
	}
}

func TestNormalizeUsesTypedParserAndStringifier(t *testing.T) {
	value, err := qsgo.Parse("a%5Bb%5D=c&list%5B%5D=1&list%5B%5D=2", nil)
	if err != nil {
		t.Fatal(err)
	}
	actual, err := qsgo.Stringify(value, nil)
	if err != nil {
		t.Fatal(err)
	}
	if actual != "a%5Bb%5D=c&list%5B0%5D=1&list%5B1%5D=2" {
		t.Fatalf("normalize = %q", actual)
	}
}
