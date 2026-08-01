# Submission checklist

## Eligibility and provenance

- [x] Registered as Chongxin Liu, solo entrant, China, age 22.
- [x] Track F: JavaScript to Go.
- [x] Source is a real BSD-3-Clause public repository.
- [x] Port code started after the official kickoff.
- [x] Exact upstream commit and four original-test SHA-256 values recorded.
- [x] Original upstream baseline passes 1045/1045 assertions.
- [ ] Public repository URL added before submission.

## Functionality and reliability

- [x] `Parse` P0/P1 behavior implemented and tested.
- [x] `Stringify` P0/P1 behavior implemented and tested.
- [x] CLI builds into a standalone executable in one command.
- [x] Fresh `git archive` extraction builds and runs without untracked files.
- [ ] Original-suite adapter reports an honest per-file parity rate.
- [x] `go test ./... -count=1` passes.
- [ ] `go test -race ./... -count=1` passes (unavailable in the portable
      Windows toolchain because cgo is disabled; limitation documented).

## Behavioral evidence

- [x] Frozen Node oracle verifies the commit, test tree, and file hashes.
- [x] Differential harness runs for at least 60 continuous seconds.
- [x] Public fuzz log records seed, case count, duration, and zero divergences or
      lists every known divergence honestly.
- [x] Benchmark distributions, comparative summary, and methodology committed.

## Quality and presentation

- [x] Zero `any` / `interface{}` in the public value model.
- [x] BSD-3-Clause project license and upstream attribution included.
- [x] At least ten non-trivial architectural decisions documented.
- [x] README contains API examples, compatibility table, and exact limitations.
- [x] Under-five-minute live-evidence demo script completed.
- [ ] Demo video recorded, rendered, and verified under five minutes.
- [ ] Devfolio/project submission completed before 2026-08-04 02:00 Beijing.
- [ ] Technical write-up submitted before 2026-08-11 02:00 Beijing.
