# Recorded comparative benchmark v2

This directory is the publishable portion of the benchmark record completed on
2026-08-03. The run is bound to qs-go commit
`e81c85b2e6cd71bba26ade520ebc27e110d483cd` and tree
`1700a993a37530400f1b517a449abd2ea394c995`.

Before measurement, the runner passed the Go test and vet gates, the opt-in
frozen-oracle integration, and the frozen upstream JavaScript baseline of
1,045/1,045 assertions. It also verified the 672,321-case differential report
by SHA-256. The benchmark completed with a clean repository before and after
measurement, and its final publishable-text privacy scan passed.

## Headline observations

Compared with Node.js v24.11.1 on the recorded Windows host:

- Go stringify median latency was 40.3% lower for the flat workload and 60.7%
  lower for the nested workload.
- Go parse median latency was 20.0% higher for the flat workload and 11.6%
  higher for the nested workload.
- Go's externally polled peak Working Set was 35.85 MiB versus 65.91 MiB, a
  45.6% reduction.
- Go cold-start median was 14.91 ms versus 73.29 ms, about 4.9 times faster.
- The first Go cold-start sample was a 384.82 ms outlier. With 40 samples and
  the declared percentile rule, p99 is the maximum, so Go p99 was 4.83 times
  the Node p99. This observation is retained, not filtered or rerun away.

These are host-specific microbenchmark observations. Correctness evidence is
authoritative over speed, and no result is generalized beyond the recorded
workloads and environment.

## Contents

- `summary.json`: source identities, environment, method, aggregates,
  limitations, and artifact manifest.
- `raw/`: 160 latency samples per runtime, 40 cold starts per runtime, and the
  timestamped 10 ms Working Set polling series.
- `correctness/`: captured stdout and stderr for tests, vet, builds, the frozen
  oracle integration, and the upstream Tape suite.
- `evidence/fuzz_report.json`: the hash-verified differential report copied
  before timing.
- `runtime/`: sanitized record context and three host snapshots.

The two generated benchmark executables are omitted from Git history. Their
hashes remain in `summary.json`; the complete evidence bundle, including those
executables, is published as the release asset
`qs_go_benchmark_evidence_v2.zip`.

See `../methodology.md` and `../run_benchmark.ps1` for the complete method and
reproduction command.
