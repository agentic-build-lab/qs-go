# Demo script — target 4:40

The recording must show live commands and legible evidence. Do not imply that
all 1,045 JavaScript assertions run directly against Go; state the exact tested
scope on screen.

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
- 564,651 total cases;
- 32 parse and 24 stringify profiles;
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

Open `bench/results.json` and a compact chart/table:

- parse is broadly at parity, with p99 regressions of 4.9%–6.3%;
- stringify median improves 39.1%–53.1%;
- cold-start median is 20.6 ms versus 106.7 ms;
- polled peak Working Set is 29.9 MB versus 78.1 MB.

State that runs were sequential, host-specific, and peak memory was sampled at
10 ms intervals.

## 3:45–4:20 — Engineering decisions

Scroll `DECISIONS.md` and call out three choices:

1. balanced bracket scanner instead of a regular expression;
2. `arrayLimit` as a representation boundary, not silent truncation;
3. Node isolated to the development oracle, never the release path.

## 4:20–4:40 — Close

Show `submission_checklist.md`.

> qs-go is a standalone Go port with a typed API, clean build, frozen oracle,
> over half a million zero-difference comparisons, a fuzzer-discovered fix, and
> an honest benchmark. Every important claim has a command, hash, or ledger
> entry behind it. Reproduce it.
