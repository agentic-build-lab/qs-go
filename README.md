# qs-go

`qs-go` is a clean-room Go port of the observable query-string behavior in
[`ljharb/qs`](https://github.com/ljharb/qs), frozen at commit
`3a890d4ecd3deb72a45d90be36f4f8c5970467c7` (`v6.15.3-8-g3a890d4`).

The project was built for Port Mortem 2026. Its priorities are behavioral
equivalence, deterministic output, explicit resource limits, differential
testing against the frozen JavaScript oracle, and an API that never requires
`interface{}` or `any`.

## Competition track and source choice

This submission uses **Track H (Open Pair): JavaScript to Go**. The event FAQ
allows Track H entrants to choose any defensible public repository when the
choice is justified in the README. `ljharb/qs` is a strong migration target:
it is a mature, BSD-3-Clause query-string codec with a large executable test
baseline, deeply observable edge-case behavior, and meaningful cross-runtime
representation problems. Those properties make behavioral equivalence
measurable through frozen-source hashes, differential fuzzing, regression
tests, and comparative benchmarks rather than a compile-only claim.

## Status

Work began during the official competition window. The frozen upstream suite
passes `1045/1045` assertions locally. The typed parser and stringifier, a
standalone CLI, a hash-verifying Node oracle, and deterministic differential
harness are implemented. The final recorded stage-one run completed 564,651
comparisons with zero differences after first finding and fixing a real integer
property-ordering mismatch. Comparative benchmarks and an explicit
compatibility ledger are included. Tagged cross-runtime values and exhaustive
one-to-one upstream block mapping remain deferred; no current claim implies
100% parity.

## Design constraints

- Ordered objects preserve observable query-parameter order.
- Sparse arrays distinguish a missing slot from explicit `null` and
  `undefined`.
- Parse limits are explicit and do not silently allocate giant sparse arrays.
- JavaScript-specific behavior is documented as exact, adapted, or
  runtime-only rather than silently omitted.
- The upstream snapshot and its test hashes are verified before differential
  tests execute.

## Local commands

The competition toolchain is project-local. From this directory:

```powershell
& '..\toolchain_complete\go\bin\go.exe' test ./... -count=1
& '..\toolchain_complete\go\bin\go.exe' vet ./...
```

The portable Windows toolchain has cgo disabled, so `go test -race` is not
available in this environment. This is recorded as an evidence limitation,
not presented as a passing check.

See `PROVENANCE.md`, `DECISIONS.md`, and `testdata/oracle/oracle_manifest.json`
for the evidence trail.

## CLI smoke test

The release artifact is a standalone Go executable; Node is used only by the
development-time oracle. On any machine with Go installed:

```bash
make verify
```

Without `make`:

```bash
go build -trimpath -o bin/qsgo ./cmd/qsgo
./bin/qsgo parse 'a%5Bb%5D=c&list%5B%5D=1&list%5B%5D=2'
./bin/qsgo normalize 'a%5Bb%5D=c&list%5B%5D=1&list%5B%5D=2'
```

Windows one-command build (the policy override applies only to this process):

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\build.ps1
```

Expected parse output:

```json
{"a":{"b":"c"},"list":["1","2"]}
```

## Library API

```go
package main

import (
	"fmt"
	"log"

	qsgo "github.com/agentic-build-lab/qs-go"
)

func main() {
	parsed, err := qsgo.Parse("a%5Bb%5D=c&list[]=1&list[]=2", nil)
	if err != nil {
		log.Fatal(err)
	}

	normalized, err := qsgo.Stringify(parsed, nil)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(normalized)
}
```

Construct typed values directly when query parsing is not the input boundary:

```go
value := qsgo.NewObject(
	qsgo.Member{Key: "user", Value: qsgo.NewString("Ada")},
	qsgo.Member{Key: "roles", Value: qsgo.NewArray(
		qsgo.NewString("admin"),
		qsgo.NewString("reviewer"),
	)},
)

options := qsgo.DefaultStringifyOptions()
options.ArrayFormat = qsgo.ArrayFormatBrackets
query, err := qsgo.Stringify(value, &options)
```

## Compatibility status

| Surface | Status | Evidence |
| --- | --- | --- |
| Frozen upstream identity | Exact | Commit, test-tree hash, and four source hashes verified before oracle handshake |
| JSON-compatible parsing | High-risk P0/P1 implemented | 57 named parser scenarios plus differential profiles |
| JSON-compatible stringifying | High-risk P0/P1 implemented | 21 test functions, four array-format subtests, and 81.7% selected-file coverage |
| Declared dense-value differential subset | Exact within scope | 564,651 deterministic comparisons; zero final mismatches or execution errors |
| Standalone release path | Implemented | Clean `git archive` build; no Node, cgo, subprocess, or third-party runtime in the binary path |
| Sparse holes and explicit `undefined` in the typed API | Represented | Closed `Value` algebra distinguishes holes, undefined, and null |
| Tagged UTF-16, non-finite values, cycles, and reference identity over the oracle wire | Deferred | Requires the tagged-value protocol expansion |
| Executable callbacks and RegExp delimiters | Deferred by design | No `eval`; a future fixed callback registry would be required |
| One-to-one mapping of all 311 upstream source blocks | Deferred | Current ledger is a risk-prioritized matrix, not a blanket parity claim |

## Exact limitations

- The original `1045/1045` result is the frozen JavaScript baseline. Those
  assertions do not run directly against the Go package.
- The zero-difference fuzz result covers the declared JSON-compatible dense
  subset and 32 parse / 24 stringify option profiles, not every JavaScript-only
  value or callback behavior.
- The compatibility ledger is risk-prioritized and does not yet map every one
  of the 311 upstream source test blocks.
- Benchmark results are host-specific, sequential, and subordinate to
  correctness. Peak Working Set was sampled every 10 ms, so very short spikes
  may be missed.
- Race instrumentation could not be run with the portable Windows toolchain
  because cgo is disabled.

## Differential evidence

The stage-one deterministic harness compares this port with the hash-verified
JavaScript oracle through one long-lived NDJSON process:

```bash
go run ./cmd/differential_fuzz -duration 60s -min-cases 10000 -seed 0x5153474f
```

The final recorded run completed 564,651 cases in 60,000 ms with zero
mismatches and zero execution errors across 32 parse and 24 stringify profiles.
See `fuzz/log.txt`. The expanded pre-run first found and fixed a real JavaScript
integer-property ordering mismatch. Scope remains explicitly limited to
JSON-compatible dense values; the compatibility ledger lists JavaScript-only
tagged values and callbacks that are not yet covered.

## Benchmark summary

The paired run in `bench/results.json` used 40 latency samples of 500
iterations and 40 independent cold starts per runtime. Compared with Node
v24.11.1 on the recorded Windows host:

- parse median was broadly at parity; parse p99 was 4.9% to 6.3% slower;
- stringify median was 39.1% to 53.1% faster;
- cold-start median was 20.6 ms versus 106.7 ms;
- externally polled peak Working Set was 29.9 MB versus 78.1 MB.

See `bench/methodology.md` and `bench/results.json` for the complete method,
distributions, and limitations.
