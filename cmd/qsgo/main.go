package main

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	"os"
	"strconv"
	"time"

	qsgo "github.com/agentic-build-lab/qs-go"
)

const usage = `qsgo — typed Go port of ljharb/qs

Usage:
  qsgo parse <query>
  qsgo normalize <query>
  qsgo version

parse prints ordered JSON. normalize parses with upstream defaults and then
stringifies the typed result, providing a quick compatibility smoke test.`

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "qsgo:", err)
		os.Exit(1)
	}
}

func run(arguments []string, output *os.File) error {
	if len(arguments) == 0 {
		fmt.Fprintln(output, usage)
		return nil
	}

	switch arguments[0] {
	case "version":
		fmt.Fprintln(output, "qsgo dev (oracle ljharb/qs v6.15.3-8-g3a890d4)")
		return nil
	case "parse", "normalize":
		if len(arguments) != 2 {
			return errors.New("parse and normalize require exactly one quoted query argument")
		}
		value, err := qsgo.Parse(arguments[1], nil)
		if err != nil {
			return err
		}
		if arguments[0] == "normalize" {
			normalized, err := qsgo.Stringify(value, nil)
			if err != nil {
				return err
			}
			fmt.Fprintln(output, normalized)
			return nil
		}
		encoded, err := encodeJSON(value)
		if err != nil {
			return err
		}
		fmt.Fprintln(output, encoded)
		return nil
	default:
		return fmt.Errorf("unknown command %q\n\n%s", arguments[0], usage)
	}
}

func encodeJSON(value qsgo.Value) (string, error) {
	var output bytes.Buffer
	if err := appendJSON(&output, value); err != nil {
		return "", err
	}
	return output.String(), nil
}

func appendJSON(output *bytes.Buffer, value qsgo.Value) error {
	switch value.Kind() {
	case qsgo.KindUndefined, qsgo.KindNull:
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
			output.WriteString("null")
		} else {
			output.WriteString(strconv.FormatFloat(number, 'g', -1, 64))
		}
	case qsgo.KindBigInt:
		integer, _ := value.AsBigInt()
		output.WriteString(strconv.Quote(integer.String()))
	case qsgo.KindBytes:
		data, _ := value.AsBytes()
		output.WriteString(strconv.Quote(base64.StdEncoding.EncodeToString(data)))
	case qsgo.KindTime:
		instant, _ := value.AsTime()
		output.WriteString(strconv.Quote(instant.UTC().Format(time.RFC3339Nano)))
	case qsgo.KindObject:
		members, _ := value.Members()
		output.WriteByte('{')
		for index, member := range members {
			if index > 0 {
				output.WriteByte(',')
			}
			output.WriteString(strconv.Quote(member.Key))
			output.WriteByte(':')
			if err := appendJSON(output, member.Value); err != nil {
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
				output.WriteString("null")
				continue
			}
			if err := appendJSON(output, element.Value); err != nil {
				return err
			}
		}
		output.WriteByte(']')
	default:
		return fmt.Errorf("cannot encode unknown value kind %d", value.Kind())
	}
	return nil
}
