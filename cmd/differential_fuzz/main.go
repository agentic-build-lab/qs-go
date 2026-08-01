package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/agentic-build-lab/qs-go/internal/differential"
)

func main() {
	duration := flag.Duration("duration", 60*time.Second, "continuous differential run duration")
	seedText := flag.String("seed", "0x5153474f", "uint32 seed in decimal or 0x-prefixed hex")
	minimumCases := flag.Int("min-cases", 10000, "minimum successful cases")
	moduleRoot := flag.String("module-root", ".", "qs-go module root")
	flag.Parse()

	seed, err := parseSeed(*seedText)
	if err != nil {
		fail("parse seed", err)
	}
	absoluteRoot, err := filepath.Abs(*moduleRoot)
	if err != nil {
		fail("resolve module root", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), *duration+20*time.Second)
	defer cancel()
	report, err := differential.Run(ctx, differential.Config{ModuleRoot: absoluteRoot, Duration: *duration, Seed: seed})
	if err != nil {
		fail("run differential harness", err)
	}
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fail("encode report", err)
	}
	fmt.Println(string(encoded))
	if report.Mismatches != 0 || report.OracleErrors != 0 || report.GoErrors != 0 || report.TotalCases < *minimumCases {
		os.Exit(1)
	}
}

func parseSeed(input string) (uint32, error) {
	base := 10
	value := input
	if strings.HasPrefix(input, "0x") || strings.HasPrefix(input, "0X") {
		base = 16
		value = input[2:]
	}
	parsed, err := strconv.ParseUint(value, base, 32)
	return uint32(parsed), err
}

func fail(action string, err error) {
	fmt.Fprintln(os.Stderr, "differential_fuzz:", action+":", err)
	os.Exit(2)
}
