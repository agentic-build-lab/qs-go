# Clean build evidence

Verified on 2026-08-03 from implementation-and-evidence commit
`4adef55a81b26de77439131f02ee318a77e5a298` and release tree
`cc14e2d00fe465ee9358a6fe16ff52591042643a`.

1. `git archive --format=zip --mtime 2026-08-03T00:00:00Z 4adef55^{tree}`
   created a source ZIP from the committed release tree, excluding every
   untracked file and this evidence file via `.gitattributes export-ignore`.
   Archiving the tree object avoids embedding a commit-ID comment; fixing the
   entry time makes the ZIP byte-for-byte reproducible across later runs.
2. The ZIP was expanded under an unrelated parent with no sibling
   `upstream_qs` fixture.
3. The extraction passed `go test ./... -count=1` and `go vet ./...`.
4. Go built `./cmd/qsgo` from that extraction with `-trimpath`.
5. The executable reported the frozen upstream identity and normalized the
   shared smoke query to:

   `a%5Bb%5D=c&list%5B0%5D=1&list%5B1%5D=2`

Evidence hashes:

- Source archive SHA-256:
  `2F7FB9CC5DCE65DF1C9E5EABC4CD3D4733F7DAF4A6171130080F84930AE8FCC8`
- Windows amd64 executable SHA-256:
  `DA317AFB2398E1C2EC05681BC6E30F69BEC80F17CC7BCD6AF31EC83D92FF9446`

This proves that the default release path is pure Go. The hash-verified Node
oracle remains an explicit development-only opt-in and is not shipped.

The final ZIP contains 91 entries, including the committed v2 raw benchmark
record. It contains no `.git` directory, generated `work/` directory, or this
export-ignored evidence file. A public-artifact scan found zero secret-pattern
files, zero personal paths or email addresses, and zero Go files containing
the prohibited dynamic escape-hatch patterns used by this project audit.
