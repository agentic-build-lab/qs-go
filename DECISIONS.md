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

## 2026-08-01 — Bound the first oracle wire to versioned NDJSON

Plain JSON cannot preserve sparse holes, `undefined`, negative zero, invalid
UTF-16 surrogates, or reference identity. The recorded stage-one oracle
therefore accepts only dense JSON-compatible trees and built-in serializable
options. Its handshake rejects an altered upstream tree and declares this
wire boundary explicitly; tagged-value transport remains a documented future
expansion rather than an inflated current claim.

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

## 2026-08-02 — Prove corpus reachability from the runner

Alternating parse and stringify through one global case index introduced a
parity bias: the generators defined 32 and 24 scheduled templates, but the
recorded runner reached only half of each schedule. The runner now advances an
operation-local index. Tests assert full 32/24 reachability and distinct
stringify input/option templates, and the superseded run is not used as final
evidence. Evidence breadth is measured from the execution schedule, not
inferred from switch-case count.

## 2026-08-02 — Keep benchmark evidence generations immutable

The first benchmark record retained aggregates but not its individual latency,
cold-start, or Working Set samples. Those observations cannot be reconstructed
honestly. The v2 runner therefore creates a new, non-overwriting evidence
directory, verifies source identity before and after timing, and retains raw
samples in acquisition order. Historical gaps remain disclosed instead of
being backfilled from summary statistics.
