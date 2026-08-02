package differential

import (
	"fmt"
	"testing"
)

func TestGeneratorIsDeterministic(t *testing.T) {
	first := newGenerator(0x5153474F)
	second := newGenerator(0x5153474F)
	for index := 0; index < 100; index++ {
		if first.next() != second.next() {
			t.Fatalf("generator diverged at step %d", index)
		}
	}
}

func TestGeneratorProducesEveryScheduledTemplate(t *testing.T) {
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

func TestOperationScheduleReachesEveryTemplate(t *testing.T) {
	parseTemplates := make(map[int]bool)
	stringifyTemplates := make(map[int]bool)
	for totalCaseIndex := 0; totalCaseIndex < 64; totalCaseIndex++ {
		templateIndex := operationCaseIndex(totalCaseIndex)
		if totalCaseIndex%2 == 0 {
			parseTemplates[templateIndex%32] = true
		} else {
			stringifyTemplates[templateIndex%24] = true
		}
	}
	if len(parseTemplates) != 32 {
		t.Fatalf("scheduled parse templates = %d, want 32", len(parseTemplates))
	}
	if len(stringifyTemplates) != 24 {
		t.Fatalf("scheduled stringify templates = %d, want 24", len(stringifyTemplates))
	}
}

func TestParseTemplatesHaveUniqueSerializedFixtures(t *testing.T) {
	const templateCount = 32
	seen := make(map[string]int, templateCount)
	for templateIndex := 0; templateIndex < templateCount; templateIndex++ {
		// Reset the seed so generated words cannot make duplicate fixtures look
		// unique merely because they received different random values.
		testCase := newGenerator(0x5153474F).parseCase(templateIndex)
		signature := fmt.Sprintf("%s\x00%s", testCase.query, testCase.oracleOptions)
		if previousIndex, exists := seen[signature]; exists {
			t.Fatalf("parse templates %d and %d share query/options fixture %q", previousIndex, templateIndex, signature)
		}
		seen[signature] = templateIndex
	}
}

func TestStringifyTemplatesHaveUniqueSerializedFixtures(t *testing.T) {
	const templateCount = 24
	seen := make(map[string]int, templateCount)
	for templateIndex := 0; templateIndex < templateCount; templateIndex++ {
		// Reset the seed so generated words cannot make duplicate fixtures look
		// unique merely because they received different random values.
		testCase := newGenerator(0x5153474F).stringifyCase(templateIndex)
		signature := fmt.Sprintf("%s\x00%s", testCase.jsonInput, testCase.oracleOptions)
		if previousIndex, exists := seen[signature]; exists {
			t.Fatalf("stringify templates %d and %d share input/options fixture %q", previousIndex, templateIndex, signature)
		}
		seen[signature] = templateIndex
	}
}
