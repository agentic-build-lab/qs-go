package differential

import "testing"

func TestGeneratorIsDeterministic(t *testing.T) {
	first := newGenerator(0x5153474F)
	second := newGenerator(0x5153474F)
	for index := 0; index < 100; index++ {
		if first.next() != second.next() {
			t.Fatalf("generator diverged at step %d", index)
		}
	}
}

func TestGeneratorCoversEveryProfile(t *testing.T) {
	generator := newGenerator(0x5153474F)
	for index := 0; index < 64; index++ {
		parse := generator.parseCase(index)
		if parse.query == "" || len(parse.oracleOptions) == 0 {
			t.Fatalf("invalid parse case %d", index)
		}
		stringify := generator.stringifyCase(index)
		if len(stringify.jsonInput) == 0 || len(stringify.oracleOptions) == 0 {
			t.Fatalf("invalid stringify case %d", index)
		}
	}
}
