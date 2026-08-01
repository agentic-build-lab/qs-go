# Provenance

## Competition work

- Event: Port Mortem 2026 / Code Resurrection
- Entrant: Chongxin Liu
- Implementation language: Go
- Work start: 2026-08-01 during the official competition window
- Implementation repository: created from an empty directory; no Go port was
  imported

## Behavioral reference

- Upstream project: `ljharb/qs`
- Upstream URL: <https://github.com/ljharb/qs>
- Commit: `3a890d4ecd3deb72a45d90be36f4f8c5970467c7`
- Describe: `v6.15.3-8-g3a890d4`
- Package version field: `6.15.3`
- Upstream license: BSD-3-Clause
- Original baseline: `1045/1045` Tape assertions passed locally

The upstream source is used as a behavioral oracle and specification. It is not
copied into the Go implementation. Required attribution and the upstream
license are retained in `third_party/qs/LICENSE.md`.

## Local toolchain

- Go: `go1.26.5 windows/amd64`
- Node.js used for the oracle: `24.11.1`
- npm used to install oracle-only dependencies: `11.6.2`

The verified-complete Go toolchain is stored outside this implementation
directory at `../toolchain_complete/go` and is not part of a release artifact.
An earlier interrupted extraction was removed after its failure mode was
recorded in `../toolchain_extraction_diagnostic.md`; it is never used by the
build.
