# Evaluation guide

This page is the shortest path through the evidence for the Port Mortem 2026
Track H submission. Every headline number below points to a command or retained
artifact, and every equivalence claim is scoped.

## Scoring map

| Criterion | Evidence |
| --- | --- |
| Functionality and reliability | [No-install live demo](https://agentic-build-lab.github.io/qs-go/), `go test ./... -count=1`, `go vet ./...`, the standalone CLI, clean-archive verification, and Linux/Windows CI |
| Behavioral equivalence | Frozen `1045/1045` JavaScript baseline; commit, test-tree, and source-hash handshake; 672,321-case differential record; regression test for the discovered integer-property ordering mismatch; explicit compatibility ledger |
| Code quality | Closed typed `Value` algebra, ordered objects, explicit resource budgets, standard-library core, documented decisions, and no `any`, `interface{}`, or `unsafe` escape hatch in Go source |
| Innovation | One CLI delivered as a native binary and a live Go/WebAssembly demo, plus an evidence chain that is itself tested: runner-level corpus reachability, hash-frozen oracle boundaries, raw-sample benchmark tooling, and visible performance regressions rather than a one-sided scorecard |

## Five-minute verification path

For the fastest no-install smoke test, open the [live
demo](https://agentic-build-lab.github.io/qs-go/) and run both `parse` and
`normalize`. GitHub Actions compiles the same `cmd/qsgo` entry point used by the
standalone release to WebAssembly; the page does not contain a second parser.

Run the default release-path checks from a fresh clone with Go 1.26 or later:

```bash
go test ./... -count=1
go vet ./...
go build -trimpath ./...
go test . -cover -count=1
```

The expected root-package statement coverage is `81.5%`. The default test path
is pure Go and does not require Node or an untracked fixture.

Then inspect:

1. `fuzz/report.json` and `fuzz/log.txt` — final 60-second record: 672,321
   comparisons, zero observed mismatches, zero oracle errors, and zero Go
   errors, plus the scope and scheduler-audit notes.
2. `testdata/oracle/oracle_manifest.json` — frozen upstream identity and the
   four original-test SHA-256 values.
3. `docs/upstream_test_ledger.json` — exact, adapted, and deferred surfaces.
4. `DECISIONS.md` — the engineering tradeoffs and the scheduler audit.
5. `bench/recorded_v2/summary.json` and `bench/methodology.md` - the completed
   raw-sample record, benchmark generations, and interpretation limits.

## Frozen-oracle verification

The upstream JavaScript result is a baseline, not a claim that 1,045 assertions
run directly against Go. With the exact frozen `ljharb/qs` checkout and its
installed test dependencies at `../upstream_qs`, run:

```powershell
$env:QSGO_RUN_ORACLE_TESTS = '1'
go test ./internal/differential `
  -run '^TestFrozenOracleHandshakeAndBasicCases$' -count=1
Remove-Item Env:QSGO_RUN_ORACLE_TESTS
```

The final differential command is:

```bash
go run ./cmd/differential_fuzz \
  -duration 60s -min-cases 10000 -seed 0x5153474f
```

Runner tests prove every one of the 32 scheduled parse and 24 scheduled
stringify templates is reachable. The recorded result covers ordinary-JSON
dense values. Sparse-array semantics are covered by Go unit tests but remain
outside this oracle wire; tagged values, callbacks, cycles, and exhaustive
one-to-one mapping of all 311 upstream source blocks are deferred.

## Track H rationale

The official [Port Mortem repository
pool](https://coderesurrection.com/2026/repo-pool/) lists `ljharb/qs` under
Track H for a JavaScript-to-Go port. Go is not used only to reproduce a
command-line wrapper:
the dynamic value surface is redesigned as an embeddable, closed, type-safe API
with deterministic ordering and explicit limits. The release library and CLI
have no Node, cgo, subprocess, or third-party runtime dependency.

## Important limits

- `1045/1045` describes the frozen upstream JavaScript baseline.
- `672,321 zero differences` means zero observed differences within the
  declared differential corpus; it is not a blanket 100% parity claim.
- Benchmark observations are host- and session-specific. Historical v1
  aggregates and the completed v2 raw record are kept distinct. The v2 record
  retains parser regressions and a first-start Go outlier instead of filtering
  them away.
- Race instrumentation was unavailable in the recorded portable Windows
  toolchain because cgo was disabled; this is not reported as a passing check.
- The discovered mismatch was in the developing Go port, not an upstream bug,
  and is not submitted for the separate Bug Catcher prize.
- The interactive demo exposes the CLI's default `parse` and `normalize`
  commands only. Its per-run browser timing is presentation feedback, not part
  of the recorded cross-runtime benchmark.
