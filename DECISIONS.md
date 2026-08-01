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

## 2026-08-01 — Keep the Node oracle outside the release path

Node.js is permitted only as a development-time differential oracle. The Go
library and CLI build and run without Node, JavaScript, cgo, subprocess calls,
or a source-language runtime. Oracle tests are isolated so a release artifact
cannot silently fall back to the original implementation.

## 2026-08-01 — Parse bracket paths with a balanced scanner

The upstream accepts nested brackets inside a single key segment and preserves
unclosed suffixes in specific ways. A regular expression split is shorter but
cannot reproduce those cases. The parser therefore uses an explicit balanced
scanner and records depth overflow as data unless strict-depth mode requests an
error.

## 2026-08-01 — Treat arrayLimit as a representation boundary

An index at or beyond the upstream threshold changes an array into an ordered
numeric-key object; it does not truncate the value. The port models that
conversion directly and keeps a separate resource budget for denial-of-service
protection. This avoids a tempting but incompatible hard-limit interpretation.

## 2026-08-01 — Reproduce prototype-key filtering in Go

Go objects cannot suffer JavaScript prototype mutation, but callers can observe
whether keys such as `constructor` and `toString` survive parsing. The port keeps
the upstream filtering policy for behavioral equivalence and always rejects
`__proto__` path segments even when prototype names are otherwise enabled.

## 2026-08-01 — Prefer standard-library core code

The parser and stringifier use the Go standard library only. This reduces the
supply-chain surface, keeps the one-command build offline, and makes benchmark
results attributable to the port rather than a third-party URL codec. Test-only
oracle dependencies remain frozen with hashes and never enter the binary.

## 2026-08-01 — Reproduce JavaScript integer-property enumeration

The expanded differential corpus found that an `arrayLimit` overflow inserted
numeric keys as `3` then `1`, while JavaScript enumerated them as `1` then `3`.
Ordered objects therefore use the ECMAScript-observable rule: canonical uint32
indices below `4294967295` sort numerically before ordinary string keys, whose
insertion order remains stable. Keys such as `01` and `4294967295` stay ordinary.
