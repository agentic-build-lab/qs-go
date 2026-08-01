# Clean build evidence

Verified on 2026-08-01 from Git commit `ef558ce`.

1. `git archive` created a source ZIP from `HEAD`, excluding every untracked and
   ignored file.
2. The ZIP was expanded into a new empty directory.
3. Go built `./cmd/qsgo` from that extracted module using `-trimpath`.
4. The resulting executable normalized the shared smoke query to:

   `a%5Bb%5D=c&list%5B0%5D=1&list%5B1%5D=2`

Evidence hashes:

- Source archive SHA-256:
  `1FE0CB5E274AEC0EDC8951C41711F947EAE907B7DA47503184935D47E5628472`
- Windows amd64 executable SHA-256:
  `39C2DF056CF2EE36B1E6DCEEC2CCCF938BBC8F8685B0EE30F57B7DFD79B0A71F`

The first clean-build command attempted to pass an absolute package directory
while the shell remained in the original module. Go correctly rejected that as
an out-of-module package path. Re-running in the extracted module with Go's
`-C` option succeeded; no source or test change was required.
