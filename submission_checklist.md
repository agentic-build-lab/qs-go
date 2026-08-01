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

- [ ] `Parse` P0/P1 behavior implemented and tested.
- [ ] `Stringify` P0/P1 behavior implemented and tested.
- [ ] CLI builds into a standalone executable in one command.
- [ ] Original-suite adapter reports an honest per-file parity rate.
- [ ] `go test ./... -count=1` passes.
- [ ] `go test -race ./... -count=1` passes.

## Behavioral evidence

- [ ] Frozen Node oracle verifies the commit, test tree, and file hashes.
- [ ] Differential harness runs for at least 60 continuous seconds.
- [ ] Public fuzz log records seed, case count, duration, and zero divergences or
      lists every known divergence honestly.
- [ ] Benchmark raw samples and methodology committed.

## Quality and presentation

- [x] Zero `any` / `interface{}` in the public value model.
- [x] BSD-3-Clause project license and upstream attribution included.
- [x] At least ten non-trivial architectural decisions documented.
- [ ] README contains API examples, compatibility table, and exact limitations.
- [ ] Demo script and under-five-minute video completed.
- [ ] Devfolio/project submission completed before 2026-08-04 02:00 Beijing.
- [ ] Technical write-up submitted before 2026-08-11 02:00 Beijing.
