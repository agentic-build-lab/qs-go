# Clean build evidence

Verified on 2026-08-01 from Git commit
`32f7ec41f69fbfa5a8e5e648ef7aa5f7a80cfca1`.

1. `git archive` created a source ZIP from `HEAD`, excluding every untracked and
   ignored file.
2. The ZIP was expanded into a new empty directory.
3. Go built `./cmd/qsgo` from that extracted module using `-trimpath`.
4. The resulting executable reported the frozen upstream identity, parsed the
   shared smoke query, and normalized it to:

   `a%5Bb%5D=c&list%5B0%5D=1&list%5B1%5D=2`

Evidence hashes:

- Source archive SHA-256:
  `7ADB9E4937FED3802F49086760687D7C695F423F0DB20C733E416FB50B069484`
- Windows amd64 executable SHA-256:
  `DA317AFB2398E1C2EC05681BC6E30F69BEC80F17CC7BCD6AF31EC83D92FF9446`

The clean extraction also passed `go test ./... -count=1` and `go vet ./...`.
A public-artifact scan of the extracted tree found zero secret-pattern files,
zero personal-email files, and zero Go files containing the prohibited dynamic
escape-hatch patterns used by this project audit.
