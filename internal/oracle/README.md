# Frozen Node oracle

This directory contains the first differential-testing stage for the frozen
`ljharb/qs` snapshot. `node_oracle.cjs` is a long-lived NDJSON process. It uses
only local files, built-in Node modules, and the fixed upstream directory at
`../../../upstream_qs`; it performs no network access and never evaluates code
received from a request.

The first successful request must be `handshake`. Before accepting it, Node
verifies all of the following against `testdata/oracle/oracle_manifest.json`:

- Git commit and `git describe` identity;
- the Git tree hash for `HEAD:test`;
- the SHA-256 digest of all four upstream test files.

One request is limited to 1 MiB and one response to 2 MiB. The Go client applies
the same limits before writing or while reading, uses a 2-second default request
timeout, and kills the process after a timeout or malformed transport response.
Stderr capture is bounded to 64 KiB.

## Stage-one subset

The runnable first stage intentionally supports only values that round-trip
through ordinary JSON:

- `parse` accepts a string and serializable built-in options. Dense arrays,
  plain objects, strings, booleans, finite numbers, and null are returned.
- `stringify` accepts an ordinary JSON tree and serializable built-in options.
- callbacks, regular-expression delimiters, sparse arrays, cycles, undefined,
  bigint, byte buffers, dates, non-finite numbers, UTF-16 lone-surrogate tagging,
  and null-prototype identity are not represented yet.

Unknown options are rejected. In particular, callback-bearing options such as
`decoder`, `encoder`, `filter`, `sort`, and `serializeDate` cannot cross the
protocol and cannot be used to inject executable code.
