# Demo script — target 4:40

The final demo is a designed evidence walkthrough. Its command and output
panels are faithful representations of the recorded artifacts, not a claim of
live terminal capture. Do not imply that all 1,045 JavaScript assertions run
directly against Go; state the exact tested scope on screen.

## 0:00–0:25 — The port and the proof target

Show the repository root, `.port-mortem.toml`, and `PROVENANCE.md`.

Narration:

> This is qs-go, a typed Go port of ljharb/qs at the exact frozen commit shown
> here. The original four test files are hash-pinned, and the original baseline
> is 1,045 passing assertions. My goal is behavioral evidence, not a compile-only
> translation.

## 0:25–0:55 — Standalone build

In a clean source-archive extraction, run:

```powershell
go build -trimpath -o qsgo.exe ./cmd/qsgo
.\qsgo.exe version
.\qsgo.exe normalize 'a%5Bb%5D=c&list%5B%5D=1&list%5B%5D=2'
```

Show that Node is not invoked by the release binary.

## 0:55–1:35 — Observable behavior

Run:

```powershell
.\qsgo.exe parse 'a%5Bb%5D=c&list%5B%5D=1&list%5B%5D=2'
go test ./... -count=1
go vet ./...
```

Briefly open `value.go`: point to the closed `Value` algebra, ordered `Member`,
and sparse `Element`. Search Go source for the prohibited escape-hatch terms and
show zero results.

## 1:35–2:20 — Frozen oracle and differential proof

Open `testdata/oracle/oracle_manifest.json` and the handshake assertions. Then
show `fuzz/log.txt`:

- exact upstream commit and test-tree hash;
- 60,000 ms;
- 672,321 total cases: 336,161 parse and 336,160 stringify;
- 32 scheduled parse and 24 scheduled stringify templates, with runner-level
  reachability tests;
- zero mismatches and zero execution errors.

Narration:

> This is a scoped JSON-compatible differential result, not a blanket 100%
> parity claim. JavaScript-only callbacks and tagged values remain listed in the
> compatibility ledger.

## 2:20–3:00 — The bug the fuzzer found

Show the recorded counterexample:

```text
a[3]=4zf&a[1]=ui_ir  with arrayLimit=2
```

Explain that JavaScript enumerates canonical integer properties in numeric
order, while the first Go version retained insertion order. Show
`TestObjectUsesJavaScriptIntegerPropertyOrder` and the final zero-difference run.

## 3:00–3:45 — Honest benchmark

Open `bench/recorded_v2/summary.json` and a compact chart/table:

- parse median is 11.6% to 20.0% slower;
- stringify median is 40.3% to 60.7% faster;
- cold-start median is 14.91 ms versus 73.29 ms, about 4.9 times faster;
- polled peak Working Set is 35.85 MiB versus 65.91 MiB, 45.6% lower; and
- a retained 384.82 ms first-start Go outlier makes p99 worse.

State that the 40-sample p99 is the maximum, runs were sequential and
host-specific, and peak memory was sampled at 10 ms intervals.

## 3:45–4:20 — Engineering decisions

Scroll `DECISIONS.md` and call out three choices:

1. balanced bracket scanner instead of a regular expression;
2. `arrayLimit` as a representation boundary, not silent truncation;
3. Node isolated to the development oracle, never the release path.

## 4:20–4:40 — Close

Show `submission_checklist.md`.

> qs-go is a standalone Go port with a typed API, clean build, frozen oracle,
> 672,321 zero-difference comparisons, a fuzzer-discovered fix, and
> an honest benchmark. Every important claim has a command, hash, or ledger
> entry behind it. Reproduce it.
