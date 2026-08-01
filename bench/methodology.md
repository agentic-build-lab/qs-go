# Benchmark methodology

This report compares the frozen JavaScript `qs` oracle with `qs-go` on the same
Windows host. Results are not populated until the benchmark commands, machine
metadata, and raw samples have been committed together.

## Environment controls

- Pin the original to commit `3a890d4ecd3deb72a45d90be36f4f8c5970467c7`.
- Record Node, Go, OS, CPU, logical-core count, and available memory.
- Use the project-local Go toolchain and the existing frozen Node dependency
  tree; do not download packages during measurement.
- Run correctness tests immediately before timing.
- Warm up each implementation outside the measured sample set.
- Execute at least 30 independent samples per workload and publish every raw
  duration, not only the best result.
- Report median, p95, p99, throughput, cold-start time, and peak resident memory.

## Shared workloads

1. Flat parse: 100 ordinary key/value pairs.
2. Nested parse: objects and indexed arrays through five levels.
3. Adversarial parse: malformed percent escapes, repeated keys, sparse indices,
   and prototype-like names.
4. Flat stringify: 100 ordered scalar members.
5. Nested stringify: ordered objects, arrays, nulls, Unicode, and dates.
6. Round trip: parse then stringify a fixed corpus of realistic URLs.
7. Process startup: one parse through the Node CLI and the compiled Go binary.

Inputs must be byte-identical. Output is checked before a timing sample counts;
a fast mismatching result is recorded as a correctness failure, not a benchmark.

## Planned commands

```powershell
& '..\toolchain_complete\go\bin\go.exe' test ./... -count=1
& '..\toolchain_complete\go\bin\go.exe' test -run '^$' -bench . -benchmem -count 10
```

The final `results.json` will contain machine-readable raw samples and summary
statistics. This file intentionally makes no performance claim before that run.
