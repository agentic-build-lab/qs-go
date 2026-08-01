# Clean build evidence

Verified on 2026-08-01 from Git commit
`6a387db55f944bde3793ea1dc02be329778c0ec8` and release tree
`91843cbe4d32ee5bfb3f564028841353b95fff02`.

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
  `EF56E2A23575CDD4BB1E64D627560AA75AA5AEAD59051025358297C844DD1578`
- Windows amd64 executable SHA-256:
  `DA317AFB2398E1C2EC05681BC6E30F69BEC80F17CC7BCD6AF31EC83D92FF9446`

The clean extraction also passed `go test ./... -count=1` and `go vet ./...`.
A public-artifact scan of the extracted tree found zero secret-pattern files,
zero personal-email files, and zero Go files containing the prohibited dynamic
escape-hatch patterns used by this project audit.
