# Submission checklist

## Eligibility and provenance

- [x] Registered as Chongxin Liu, solo entrant, China, age 22.
- [x] Track H (Open Pair): JavaScript to Go; public-source choice justified in
      the README under the event's Open Pair rule.
- [x] Source is a real BSD-3-Clause public repository.
- [x] Port code started after the official kickoff.
- [x] Exact upstream commit and four original-test SHA-256 values recorded.
- [x] Original upstream baseline passes 1045/1045 assertions.
- [x] Public repository published at
      `https://github.com/agentic-build-lab/qs-go`.

## Functionality and reliability

- [x] `Parse` P0/P1 behavior implemented and tested.
- [x] `Stringify` P0/P1 behavior implemented and tested.
- [x] CLI builds into a standalone executable in one command.
- [x] Fresh `git archive` extraction builds and runs without untracked files.
- [x] Default tests are pure-Go and fresh-clone safe; frozen Node oracle
      verification is an explicit, documented opt-in.
- [ ] Original-suite adapter reports an honest per-file parity rate.
- [x] `go test ./... -count=1` passes.
- [ ] `go test -race ./... -count=1` passes (unavailable in the portable
      Windows toolchain because cgo is disabled; limitation documented).

## Behavioral evidence

- [x] Frozen Node oracle verifies the commit, test tree, and file hashes.
- [x] Differential harness runs for at least 60 continuous seconds.
- [x] Public fuzz log records seed, case count, duration, and zero divergences or
      lists every known divergence honestly.
- [x] Historical aggregate benchmark, comparative summary, methodology, and
      missing raw observations are documented without reconstruction.
- [x] Fresh v2 benchmark record retains raw latency, cold-start, and Working Set
      samples with host and source metadata.

## Quality and presentation

- [x] Zero `any` / `interface{}` in the public value model.
- [x] BSD-3-Clause project license and upstream attribution included.
- [x] At least ten non-trivial architectural decisions documented.
- [x] README contains API examples, compatibility table, and exact limitations.
- [x] Under-five-minute live-evidence demo script completed.
- [x] Demo video recorded and fully decoded at 1920x1080, constant 30 fps,
      H.264/AAC, and exactly 4:40; representative frames across the complete
      timeline were visually reviewed. The anonymously downloaded release asset
      matches SHA-256
      `FA2B9D9D0E0AC6237D5B6F84ACB9CBD122D68C3ADEA55C56DBB063793E574490`.
- [ ] Official organizer submission form completed before 2026-08-04 02:00
      Beijing (registration is through Tally; Devfolio is not required).
- [ ] Technical write-up submitted before 2026-08-11 02:00 Beijing.
