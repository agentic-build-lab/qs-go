# Benchmark methodology

## Evidence generations

This repository contains two distinct benchmark evidence generations.

`bench/results.json` is the frozen aggregate record created on 2026-08-01. It
preserves runtime versions, method counts, percentile summaries, cold-start
summaries, selected runtime counters, and the reported peak Working Set values.
The individual latency samples, individual cold-start durations, Working Set
polling series, CPU model, and measurement-time available-memory reading were
not preserved for that historical run. Those values cannot be reconstructed
from aggregate percentiles and have not been inferred after the fact.

`bench/run_benchmark.ps1` is the reproducible evidence runner added after the
historical record. New runs use schema `qs_go_comparative_benchmark/v2`, retain
raw evidence, and are written to a new ignored directory under `work/`. The
runner never modifies or replaces `bench/results.json`. A v2 result must be
reviewed as a separate measurement rather than presented as recovered evidence
for the historical run.

## Integrity gates

Record mode requires a clean implementation repository and verifies its Git
root, commit, tree, and branch before measurement. It verifies the frozen
`ljharb/qs` checkout against `testdata/oracle/oracle_manifest.json`, including:

- commit `3a890d4ecd3deb72a45d90be36f4f8c5970467c7`;
- the frozen upstream test-tree SHA-1;
- every test-file SHA-256 listed in the manifest;
- an unmodified tracked upstream worktree; and
- the recorded 1045-passing, zero-failing assertion baseline.

The runner performs no package download. Go uses the selected local toolchain
with `GOTOOLCHAIN=local`, `GOPROXY=off`, and project-local build caches and
temporary storage. Child processes also disable persistent Go environment and
workspace configuration, fix `GOOS=windows`, `GOARCH=amd64`, and `GOAMD64=v1`,
and clear Go experiment flags. Node coverage and compile-cache overrides are
removed before measurement.

Before timing, the runner executes:

```powershell
go test ./... -count=1
go vet ./...
node node_modules/tape/bin/tape 'test/**/*.js'
```

The exact commands, stdout, stderr, exit status, source hashes, and binary
hashes are retained. Source hashes, repository identity, upstream identity, and
the frozen historical-result hash are checked again after measurement. A
change causes the run to fail.

## Recorded environment

A v2 run records:

- Windows caption, version, build number, and architecture;
- CPU model, physical-core count, and logical-core count;
- installed and OS-visible physical memory;
- available physical memory before correctness checks, immediately before
  measurement, and after measurement;
- exact Go and Node runtime versions;
- implementation commit and tree identifiers;
- upstream commit and test-tree identifiers; and
- SHA-256 values for the runner, workload sources, binaries, logs, and raw
  result files.

Host name, user name, serial numbers, unrelated inherited environment values,
credentials, and other personal identifiers are not recorded. The normalized
benchmark environment policy is recorded. The local checkout path and its
path-derived identity hash are used only for in-process verification and are
rejected by the publishable-evidence privacy guard.

## Latency workloads

Both implementations construct equivalent deterministic inputs for four
workloads:

1. `parse_flat_100`: parse 100 ordinary scalar key/value pairs.
2. `parse_nested_20`: parse 20 scalar members below `root[group]`.
3. `stringify_flat_100`: stringify 100 ordered scalar members.
4. `stringify_nested_20`: stringify 20 ordered scalar members below
   `root.group`.

Each workload performs 2,000 warm-up operations outside the measured sample
set. It then records 40 sequential timing batches in one warmed process, with
500 operations per batch. Every raw batch duration is retained as nanoseconds
per operation in acquisition order.

These latency batches are repeated observations within one process; they are
not independent process runs. The checksum prevents the measured work from
being discarded, but semantic equivalence is established by the separately
recorded tests and differential corpus rather than by comparing only the
checksum.

For a sorted sample set, percentiles use:

```text
index = floor((sample_count - 1) * quantile + 0.5)
```

With 40 samples, p99 selects the maximum sample. The report states this
explicitly and retains the raw values so alternative statistical summaries can
be calculated without rerunning the benchmark.

## Cold start and Working Set

Cold start is measured with 40 new processes per runtime. The Node process
loads the frozen source and parses `a[b]=c`; the compiled Go CLI performs the
same parse. Every sample retains elapsed milliseconds, exit code, stdout, and
stderr. A nonzero exit or unexpected Go output fails the run.

Latency processes are measured sequentially. While each process is alive, its
Working Set is polled externally every 10 ms. The complete timestamped polling
series, the maximum observed sample, and the operating system's reported peak
Working Set are retained separately. The polled maximum remains subject to the
10 ms sampling interval and can miss shorter spikes.

## Commands

Run the read-only prerequisite and provenance check from the repository root:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass `
  -File .\bench\run_benchmark.ps1 -Mode validate
```

Create a new evidence record only when the machine is otherwise idle and the
repository is clean:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass `
  -File .\bench\run_benchmark.ps1 -Mode record
```

The default destination is:

```text
work/benchmark_run_<utc>/
```

A custom destination is accepted only when it remains below this repository's
`work/` directory. Existing destinations are never overwritten.

The publishable files from the completed 2026-08-03 record are retained under
`bench/recorded_v2/`. The record binds implementation commit `e81c85b`, the
verified upstream identity, and the SHA-256 of `fuzz/report.json`. The complete
release evidence bundle also retains the two generated executables listed in
the summary artifact manifest.

## Interpretation limits

Microbenchmarks are host-, runtime-, scheduler-, and session-specific. The
runner does not pin CPU affinity, disable frequency scaling, or claim that a
single sequential session generalizes to other machines. Sequential order can
also introduce thermal or cache effects.

Go allocation counters and Node heap counters describe different garbage
collectors and are not directly comparable. Working Set is the cross-process
memory observation, with the polling limitation stated above.

Benchmark speed never overrides correctness. Compatibility claims remain
bounded by the documented Go tests, frozen-oracle verification, differential
templates, and exact limitations.
