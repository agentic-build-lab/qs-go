# qs-go

`qs-go` is a clean-room Go port of the observable query-string behavior in
[`ljharb/qs`](https://github.com/ljharb/qs), frozen at commit
`3a890d4ecd3deb72a45d90be36f4f8c5970467c7` (`v6.15.3-8-g3a890d4`).

The project is being built for Port Mortem 2026. Its priorities are behavioral
equivalence, deterministic output, explicit resource limits, differential
testing against the frozen JavaScript oracle, and an API that never requires
`interface{}` or `any`.

## Status

Work began during the official competition window. The frozen upstream suite
passes `1045/1045` assertions locally. The typed value model and option surface
are in place; parser, stringifier, oracle harness, benchmarks, and compatibility
ledger are being implemented incrementally.

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
& '..\toolchain_complete\go\bin\go.exe' test -race ./... -count=1
```

See `PROVENANCE.md`, `DECISIONS.md`, and `testdata/oracle/oracle_manifest.json`
for the evidence trail.
