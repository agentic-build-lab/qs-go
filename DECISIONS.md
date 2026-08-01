# Decision log

## 2026-08-01 — Freeze an exact upstream development snapshot

The target is commit `3a890d4e`, not the npm version label alone. The checkout
is eight commits after `v6.15.3`; its stringify tests differ from that tag.
Pinning the commit and test hashes makes every compatibility claim reproducible.

## 2026-08-01 — Use a closed typed value algebra

The public API represents values with a `Value` discriminated union, ordered
`Member` values, and sparse `Element` slots. It does not accept `any` or
`interface{}`. This prevents ambiguous conversions and preserves distinctions
that ordinary Go maps and slices lose.

## 2026-08-01 — Preserve order explicitly

JavaScript object key order is observable in stringify output. Go map iteration
is intentionally unordered, so objects are stored as ordered members with an
internal index for lookup and merge operations.

## 2026-08-01 — Separate safety limits from compatibility thresholds

The upstream `arrayLimit` changes container representation; it is not a hard
memory limit. `parameterLimit` constrains pair count. The Go port models those
semantics and will expose separate byte, node, and nesting safety limits for
untrusted input.

## 2026-08-01 — Differential testing uses tagged NDJSON

Plain JSON cannot preserve sparse holes, `undefined`, negative zero, invalid
UTF-16 surrogates, or reference identity. The oracle protocol therefore uses
tagged values and an explicit handshake that rejects an altered upstream tree.
