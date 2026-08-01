# Clean build evidence

Verified on 2026-08-01 from Git commit
`dd534049a033fef4a7a3c4a0c0ab0baa9ca2d7a3` and release tree
`f087271542a32287847e0575b6affbbe9e2afda0`.

1. `git archive --format=zip --mtime 2026-08-01T00:00:00Z HEAD^{tree}` created
   a source ZIP from the committed release tree, excluding every untracked file
   and this evidence file via `.gitattributes export-ignore`. Archiving the tree
   object avoids embedding a commit-ID comment; fixing the entry time makes the
   ZIP byte-for-byte reproducible across later runs.
2. The ZIP was expanded into a new empty directory.
3. Go built `./cmd/qsgo` from that extracted module using `-trimpath`.
4. The resulting executable reported the frozen upstream identity, parsed the
   shared smoke query, and normalized it to:

   `a%5Bb%5D=c&list%5B0%5D=1&list%5B1%5D=2`

Evidence hashes:

- Source archive SHA-256:
  `4D28659DA3C11DDB57676F3B774F7312220990AAE6EC0EC1993B6BED130DDB95`
- Windows amd64 executable SHA-256:
  `DA317AFB2398E1C2EC05681BC6E30F69BEC80F17CC7BCD6AF31EC83D92FF9446`

The clean extraction also passed `go test ./... -count=1` and `go vet ./...`.
A public-artifact scan of the extracted tree found zero secret-pattern files,
zero personal-email files, and zero Go files containing the prohibited dynamic
escape-hatch patterns used by this project audit.
