# qs-go

[![verify](https://github.com/agentic-build-lab/qs-go/actions/workflows/verify.yml/badge.svg)](https://github.com/agentic-build-lab/qs-go/actions/workflows/verify.yml)

Judges and reviewers can start with [`EVALUATION.md`](EVALUATION.md) for a
one-page map from the scoring criteria to commands, evidence, and limits.

`qs-go` is a clean-room Go port of the observable query-string behavior in
[`ljharb/qs`](https://github.com/ljharb/qs), frozen at commit
`3a890d4ecd3deb72a45d90be36f4f8c5970467c7` (`v6.15.3-8-g3a890d4`).

The project was built for Port Mortem 2026. Its priorities are behavioral
equivalence, deterministic output, explicit resource limits, differential
testing against the frozen JavaScript oracle, and an API that never requires
`interface{}` or `any`.

## Competition track and source choice

This submission uses **Track H (Open Pair): JavaScript to Go**. The official
[repository pool](https://coderesurrection.com/2026/repo-pool/) explicitly
lists `ljharb/qs` for Track H. This project intentionally uses the Open Pair
rather than treating the work as a CLI-only rewrite: a dynamic JavaScript data
surface becomes a closed, type-safe Go API that can be embedded in Go services
and shipped as a standalone binary.

`ljharb/qs` is a strong migration target because it is a mature, BSD-3-Clause
codec with a large executable baseline, deeply observable edge-case behavior,
and representation problems that ordinary Go maps and slices erase. Frozen
source hashes, differential fuzzing, regression tests, startup measurements,
and memory evidence make that cross-runtime migration measurable rather than a
compile-only claim.

## Status

Work began during the official competition window. The frozen upstream suite
passes `1045/1045` assertions locally. The typed parser and stringifier, a
standalone CLI, a hash-verifying Node oracle, and deterministic differential
harness are implemented. The final recorded stage-one run completed 672,321
comparisons with zero differences after first finding and fixing a real integer
property-ordering mismatch. A final evidence audit also found and removed a
parity bias in the corpus scheduler; runner-level tests now prove all 32 parse
and 24 stringify templates are reachable. Comparative benchmarks and an
explicit compatibility ledger are included. Tagged cross-runtime values and
exhaustive one-to-one upstream block mapping remain deferred; no current claim
implies 100% parity.

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

With Go 1.26 or later installed, run from this directory:

```powershell
go test ./... -count=1
go vet ./...
```

The portable Windows toolchain has cgo disabled, so `go test -race` is not
available in this environment. This is recorded as an evidence limitation,
not presented as a passing check.

The default `go test ./...` path is pure Go and is safe for a fresh clone. The
development-only Node oracle integration is opt-in because its hash-verified
upstream fixture is intentionally excluded from the release. To run it, place
the frozen `ljharb/qs` checkout at `../upstream_qs`, including its installed
test dependencies, and set `QSGO_RUN_ORACLE_TESTS=1`:

```powershell
$env:QSGO_RUN_ORACLE_TESTS = '1'
go test ./internal/differential -run '^TestFrozenOracleHandshakeAndBasicCases$' -count=1
Remove-Item Env:QSGO_RUN_ORACLE_TESTS
```

On shells supported by Make, the equivalent command is `make oracle-test`.
Opting in without the required fixture fails with its expected absolute path;
the test never downloads source or dependencies.

See `PROVENANCE.md`, `DECISIONS.md`, `internal/oracle/README.md`, and
`testdata/oracle/oracle_manifest.json` for the evidence trail.

## CLI smoke test

The release artifact is a standalone Go executable; Node is used only by the
explicitly enabled development-time oracle. On any machine with Go installed,
the default verification path remains pure Go:

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
| JSON-compatible parsing | High-risk P0/P1 implemented | 57 named parser scenarios plus 32 scheduled differential templates |
| JSON-compatible stringifying | High-risk P0/P1 implemented | 21 test functions, four array-format subtests, and 24 scheduled differential templates |
| Current root package coverage | Measured | 81.5% statement coverage via `go test . -cover -count=1` |
| Declared dense-value differential subset | Exact within scope | 672,321 deterministic comparisons; zero final mismatches or execution errors |
| Standalone release path | Implemented | Clean `git archive` build; no Node, cgo, subprocess, or third-party runtime in the binary path |
| Sparse holes and explicit `undefined` in the typed API | Represented | Closed `Value` algebra distinguishes holes, undefined, and null |
| Tagged UTF-16, non-finite values, cycles, and reference identity over the oracle wire | Deferred | Requires the tagged-value protocol expansion |
| Executable callbacks and RegExp delimiters | Deferred by design | No `eval`; a future fixed callback registry would be required |
| One-to-one mapping of all 311 upstream source blocks | Deferred | Current ledger is a risk-prioritized matrix, not a blanket parity claim |

## Exact limitations

- The original `1045/1045` result is the frozen JavaScript baseline. Those
  assertions do not run directly against the Go package.
- The zero-difference fuzz result covers the declared JSON-compatible dense
  subset and 32 scheduled parse / 24 scheduled stringify templates, not every
  JavaScript-only value or callback behavior. Sparse-array semantics are tested
  in Go but remain outside the ordinary-JSON oracle wire.
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

The final recorded run completed 672,321 cases in 60,000 ms: 336,161 parse and
336,160 stringify comparisons, with zero mismatches and zero execution errors.
Runner-level tests prove all 32 scheduled parse and 24 scheduled stringify
templates are reachable. See `fuzz/report.json` for the machine-readable result
and `fuzz/log.txt` for scope and audit notes. The expanded pre-run first found
and fixed a real JavaScript integer-property ordering mismatch. Scope remains
explicitly limited to JSON-compatible dense values; the compatibility ledger
lists JavaScript-only tagged values and callbacks that are not yet covered.

## Benchmark summary

The publishable [v2 record](bench/recorded_v2/README.md) retains 40 raw latency
samples for each of four workloads and 40 independent cold starts per runtime.
Compared with Node v24.11.1 on the recorded Windows host:

- stringify median latency was 40.3% to 60.7% lower;
- parse median latency was 11.6% to 20.0% higher;
- cold-start median was 14.91 ms versus 73.29 ms, about 4.9 times faster;
- externally polled peak Working Set was 35.85 MiB versus 65.91 MiB, 45.6%
  lower; and
- the first Go cold start was a retained 384.82 ms outlier, making Go p99 4.83
  times the Node p99 under the declared p99-equals-maximum rule.

The record is bound to implementation commit `e81c85b`, the verified upstream
identity, and the SHA-256 of the 672,321-case differential report. It includes
the raw sample series, correctness logs, host snapshots, source and artifact
hashes, normalized child-process environment, and explicit limitations.

The earlier `bench/results.json` aggregate record remains immutable. It did not
retain individual samples, so none were reconstructed. Run the read-only v2
prerequisite and provenance check on Windows with:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass `
  -File .\bench\run_benchmark.ps1 -Mode validate
```

A new full record uses `-Mode record`, writes a separate artifact below the
ignored `work/` directory, and never overwrites either recorded generation.
See `bench/methodology.md`, `bench/recorded_v2/summary.json`, and
`bench/results.json` for the exact method, data, and limitations.
