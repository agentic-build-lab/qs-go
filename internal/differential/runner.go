package differential

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"strconv"
	"time"

	qsgo "github.com/agentic-build-lab/qs-go"
	"github.com/agentic-build-lab/qs-go/internal/oracle"
)

const ReportSchema = "qs_go_differential_report/v1"

type Config struct {
	ModuleRoot string
	NodeBinary string
	Duration   time.Duration
	Seed       uint32
}

type Failure struct {
	Index     int    `json:"index"`
	Operation string `json:"operation"`
	Input     string `json:"input"`
	Options   string `json:"options"`
	Expected  string `json:"expected,omitempty"`
	Actual    string `json:"actual,omitempty"`
	Error     string `json:"error,omitempty"`
}

type Report struct {
	Schema           string   `json:"schema"`
	SeedHex          string   `json:"seed_hex"`
	StartedAt        string   `json:"started_at"`
	FinishedAt       string   `json:"finished_at"`
	DurationMillis   int64    `json:"duration_ms"`
	TotalCases       int      `json:"total_cases"`
	ParseCases       int      `json:"parse_cases"`
	StringifyCases   int      `json:"stringify_cases"`
	Mismatches       int      `json:"mismatches"`
	OracleErrors     int      `json:"oracle_errors"`
	GoErrors         int      `json:"go_errors"`
	UpstreamCommit   string   `json:"upstream_commit"`
	UpstreamTestTree string   `json:"upstream_test_tree"`
	FirstFailure     *Failure `json:"first_failure,omitempty"`
}

func Run(parent context.Context, config Config) (Report, error) {
	if config.ModuleRoot == "" {
		return Report{}, errors.New("differential: module root is required")
	}
	if config.Duration <= 0 {
		config.Duration = 60 * time.Second
	}
	if config.Seed == 0 {
		config.Seed = 0x5153474F
	}

	client, err := oracle.Start(parent, oracle.Config{
		NodeBinary:       config.NodeBinary,
		ScriptPath:       filepath.Join(config.ModuleRoot, "internal", "oracle", "node_oracle.cjs"),
		ManifestPath:     filepath.Join(config.ModuleRoot, "testdata", "oracle", "oracle_manifest.json"),
		WorkingDirectory: config.ModuleRoot,
		StartupTimeout:   10 * time.Second,
		RequestTimeout:   3 * time.Second,
	})
	if err != nil {
		return Report{}, err
	}
	defer client.Close()

	handshake := client.Handshake()
	started := time.Now().UTC()
	report := Report{
		Schema:           ReportSchema,
		SeedHex:          fmt.Sprintf("0x%08x", config.Seed),
		StartedAt:        started.Format(time.RFC3339Nano),
		UpstreamCommit:   handshake.Upstream.Commit,
		UpstreamTestTree: handshake.Upstream.TestTreeSHA1,
	}
	deadline := started.Add(config.Duration)
	generator := newGenerator(config.Seed)

	for time.Now().UTC().Before(deadline) {
		index := report.TotalCases
		if index%2 == 0 {
			parseCase := generator.parseCase(index)
			report.ParseCases++
			if failure := compareParse(parent, client, index, parseCase, &report); failure != nil {
				report.FirstFailure = failure
				report.Mismatches++
				break
			}
		} else {
			stringifyCase := generator.stringifyCase(index)
			report.StringifyCases++
			if failure := compareStringify(parent, client, index, stringifyCase, &report); failure != nil {
				report.FirstFailure = failure
				report.Mismatches++
				break
			}
		}
		report.TotalCases++
	}

	finished := time.Now().UTC()
	report.FinishedAt = finished.Format(time.RFC3339Nano)
	report.DurationMillis = finished.Sub(started).Milliseconds()
	return report, nil
}

type parseCase struct {
	query         string
	goOptions     *qsgo.ParseOptions
	oracleOptions json.RawMessage
}

type stringifyCase struct {
	value         qsgo.Value
	jsonInput     json.RawMessage
	goOptions     *qsgo.StringifyOptions
	oracleOptions json.RawMessage
}

func compareParse(ctx context.Context, client *oracle.Client, index int, testCase parseCase, report *Report) *Failure {
	expected, err := client.Parse(ctx, testCase.query, testCase.oracleOptions)
	if err != nil {
		report.OracleErrors++
		return &Failure{Index: index, Operation: "parse", Input: testCase.query, Options: string(testCase.oracleOptions), Error: "oracle: " + err.Error()}
	}
	actualValue, err := qsgo.Parse(testCase.query, testCase.goOptions)
	if err != nil {
		report.GoErrors++
		return &Failure{Index: index, Operation: "parse", Input: testCase.query, Options: string(testCase.oracleOptions), Expected: string(expected), Error: "go: " + err.Error()}
	}
	actual, err := valueJSON(actualValue)
	if err != nil {
		report.GoErrors++
		return &Failure{Index: index, Operation: "parse", Input: testCase.query, Options: string(testCase.oracleOptions), Expected: string(expected), Error: err.Error()}
	}
	if !jsonEqual(expected, actual) {
		return &Failure{Index: index, Operation: "parse", Input: testCase.query, Options: string(testCase.oracleOptions), Expected: string(expected), Actual: string(actual)}
	}
	return nil
}

func compareStringify(ctx context.Context, client *oracle.Client, index int, testCase stringifyCase, report *Report) *Failure {
	expected, err := client.Stringify(ctx, testCase.jsonInput, testCase.oracleOptions)
	if err != nil {
		report.OracleErrors++
		return &Failure{Index: index, Operation: "stringify", Input: string(testCase.jsonInput), Options: string(testCase.oracleOptions), Error: "oracle: " + err.Error()}
	}
	actual, err := qsgo.Stringify(testCase.value, testCase.goOptions)
	if err != nil {
		report.GoErrors++
		return &Failure{Index: index, Operation: "stringify", Input: string(testCase.jsonInput), Options: string(testCase.oracleOptions), Expected: expected, Error: "go: " + err.Error()}
	}
	if expected != actual {
		return &Failure{Index: index, Operation: "stringify", Input: string(testCase.jsonInput), Options: string(testCase.oracleOptions), Expected: expected, Actual: actual}
	}
	return nil
}

type generator struct {
	state uint32
}

func newGenerator(seed uint32) *generator { return &generator{state: seed} }

func (generator *generator) next() uint32 {
	generator.state += 0x6D2B79F5
	value := generator.state
	value = (value ^ (value >> 15)) * (value | 1)
	value ^= value + (value^(value>>7))*(value|61)
	return value ^ (value >> 14)
}

func (generator *generator) word() string {
	length := 1 + int(generator.next()%8)
	result := make([]byte, length)
	const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789-_~"
	for index := range result {
		result[index] = alphabet[int(generator.next()%uint32(len(alphabet)))]
	}
	return string(result)
}

func (generator *generator) parseCase(index int) parseCase {
	left, middle, right := generator.word(), generator.word(), generator.word()
	switch index % 16 {
	case 0:
		return parseCase{query: "a=" + left + "&b=" + right, oracleOptions: json.RawMessage(`{}`)}
	case 1:
		return parseCase{query: "a[b]=" + left + "&a[c]=" + right, oracleOptions: json.RawMessage(`{}`)}
	case 2:
		return parseCase{query: "list[]=" + left + "&list[]=" + right, oracleOptions: json.RawMessage(`{}`)}
	case 3:
		return parseCase{query: "list[0]=" + left + "&list[2]=" + right, oracleOptions: json.RawMessage(`{}`)}
	case 4:
		return parseCase{query: "a=" + left + "&a=" + right, oracleOptions: json.RawMessage(`{}`)}
	case 5:
		options := qsgo.ParseOptions{AllowDots: qsgo.Bool(true)}
		return parseCase{query: "a.b=" + left + "&a.c=" + right, goOptions: &options, oracleOptions: json.RawMessage(`{"allowDots":true}`)}
	case 6:
		options := qsgo.ParseOptions{StrictNullHandling: qsgo.Bool(true)}
		return parseCase{query: "a&b=" + right, goOptions: &options, oracleOptions: json.RawMessage(`{"strictNullHandling":true}`)}
	case 7:
		options := qsgo.ParseOptions{Comma: qsgo.Bool(true)}
		return parseCase{query: "a=" + left + "," + middle + "," + right, goOptions: &options, oracleOptions: json.RawMessage(`{"comma":true}`)}
	case 8:
		options := qsgo.ParseOptions{IgnoreQueryPrefix: qsgo.Bool(true)}
		return parseCase{query: "?a=" + left + "&b=" + right, goOptions: &options, oracleOptions: json.RawMessage(`{"ignoreQueryPrefix":true}`)}
	case 9:
		options := qsgo.ParseOptions{Delimiter: ";"}
		return parseCase{query: "a=" + left + ";b=" + right, goOptions: &options, oracleOptions: json.RawMessage(`{"delimiter":";"}`)}
	case 10:
		options := qsgo.ParseOptions{AllowEmptyArrays: qsgo.Bool(true)}
		return parseCase{query: "a[]&b=" + right, goOptions: &options, oracleOptions: json.RawMessage(`{"allowEmptyArrays":true}`)}
	case 11:
		options := qsgo.ParseOptions{Depth: qsgo.Int(1)}
		return parseCase{query: "a[b][c][d]=" + left, goOptions: &options, oracleOptions: json.RawMessage(`{"depth":1}`)}
	case 12:
		return parseCase{query: "a%5Bb%5D=" + left + "&encoded%3Dkey=" + right, oracleOptions: json.RawMessage(`{}`)}
	case 13:
		return parseCase{query: "a=" + left + "%ZZ" + right + "&b=%E0%A4%A", oracleOptions: json.RawMessage(`{}`)}
	case 14:
		return parseCase{query: "toString=" + left + "&safe=" + right + "&a[__proto__]=x", oracleOptions: json.RawMessage(`{}`)}
	default:
		policy := qsgo.DuplicatesLast
		return parseCase{query: "a=" + left + "&a=" + middle + "&a=" + right, goOptions: &qsgo.ParseOptions{Duplicates: policy}, oracleOptions: json.RawMessage(`{"duplicates":"last"}`)}
	}
}

func (generator *generator) stringifyCase(index int) stringifyCase {
	left, right := generator.word(), generator.word()
	value := qsgo.NewObject(
		qsgo.Member{Key: "a", Value: qsgo.NewString(left)},
		qsgo.Member{Key: "b", Value: qsgo.NewString(right)},
	)
	options := (*qsgo.StringifyOptions)(nil)
	oracleOptions := json.RawMessage(`{}`)

	switch index % 12 {
	case 1:
		value = qsgo.NewObject(qsgo.Member{Key: "a", Value: qsgo.NewObject(qsgo.Member{Key: "b", Value: qsgo.NewString(left)})})
	case 3:
		value = qsgo.NewObject(qsgo.Member{Key: "list", Value: qsgo.NewArray(qsgo.NewString(left), qsgo.NewString(right))})
		configured := qsgo.StringifyOptions{ArrayFormat: qsgo.ArrayFormatBrackets}
		options = &configured
		oracleOptions = json.RawMessage(`{"arrayFormat":"brackets"}`)
	case 5:
		value = qsgo.NewObject(qsgo.Member{Key: "list", Value: qsgo.NewArray(qsgo.NewString(left), qsgo.NewString(right))})
		configured := qsgo.StringifyOptions{ArrayFormat: qsgo.ArrayFormatRepeat}
		options = &configured
		oracleOptions = json.RawMessage(`{"arrayFormat":"repeat"}`)
	case 7:
		value = qsgo.NewObject(qsgo.Member{Key: "list", Value: qsgo.NewArray(qsgo.NewString(left), qsgo.NewString(right))})
		configured := qsgo.StringifyOptions{ArrayFormat: qsgo.ArrayFormatComma}
		options = &configured
		oracleOptions = json.RawMessage(`{"arrayFormat":"comma"}`)
	case 9:
		value = qsgo.NewObject(qsgo.Member{Key: "a", Value: qsgo.NewObject(qsgo.Member{Key: "b", Value: qsgo.NewString(left)})})
		configured := qsgo.StringifyOptions{AllowDots: qsgo.Bool(true)}
		options = &configured
		oracleOptions = json.RawMessage(`{"allowDots":true}`)
	case 11:
		value = qsgo.NewObject(qsgo.Member{Key: "a", Value: qsgo.NewNull()}, qsgo.Member{Key: "b", Value: qsgo.NewString(right)})
		configured := qsgo.StringifyOptions{StrictNullHandling: qsgo.Bool(true)}
		options = &configured
		oracleOptions = json.RawMessage(`{"strictNullHandling":true}`)
	}

	input, err := valueJSON(value)
	if err != nil {
		panic(err)
	}
	return stringifyCase{value: value, jsonInput: input, goOptions: options, oracleOptions: oracleOptions}
}

func valueJSON(value qsgo.Value) (json.RawMessage, error) {
	var output bytes.Buffer
	if err := appendValueJSON(&output, value); err != nil {
		return nil, err
	}
	return append(json.RawMessage(nil), output.Bytes()...), nil
}

func appendValueJSON(output *bytes.Buffer, value qsgo.Value) error {
	switch value.Kind() {
	case qsgo.KindNull, qsgo.KindUndefined:
		output.WriteString("null")
	case qsgo.KindString:
		text, _ := value.AsString()
		output.WriteString(strconv.Quote(text))
	case qsgo.KindBool:
		boolean, _ := value.AsBool()
		output.WriteString(strconv.FormatBool(boolean))
	case qsgo.KindNumber:
		number, _ := value.AsNumber()
		if math.IsNaN(number) || math.IsInf(number, 0) {
			return errors.New("differential: non-finite number requires tagged protocol")
		}
		output.WriteString(strconv.FormatFloat(number, 'g', -1, 64))
	case qsgo.KindObject:
		members, _ := value.Members()
		output.WriteByte('{')
		for index, member := range members {
			if index > 0 {
				output.WriteByte(',')
			}
			output.WriteString(strconv.Quote(member.Key))
			output.WriteByte(':')
			if err := appendValueJSON(output, member.Value); err != nil {
				return err
			}
		}
		output.WriteByte('}')
	case qsgo.KindArray:
		elements, _ := value.Elements()
		output.WriteByte('[')
		for index, element := range elements {
			if index > 0 {
				output.WriteByte(',')
			}
			if !element.Present {
				return errors.New("differential: sparse array requires tagged protocol")
			}
			if err := appendValueJSON(output, element.Value); err != nil {
				return err
			}
		}
		output.WriteByte(']')
	default:
		return fmt.Errorf("differential: value kind %d requires tagged protocol", value.Kind())
	}
	return nil
}

func jsonEqual(left, right json.RawMessage) bool {
	var compactLeft bytes.Buffer
	if err := json.Compact(&compactLeft, left); err != nil {
		return false
	}
	var compactRight bytes.Buffer
	if err := json.Compact(&compactRight, right); err != nil {
		return false
	}
	return bytes.Equal(compactLeft.Bytes(), compactRight.Bytes())
}
