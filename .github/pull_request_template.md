## What this PR does

<!-- One paragraph. What changed and why. Be specific. -->

## Related issue

Closes #

## Type

- [ ] New condition type (new subcommand)
- [ ] Bug fix
- [ ] Documentation
- [ ] Refactor
- [ ] Chore / CI
- [ ] Release / build tooling

## How I tested it

<!--
Commands you ran and the observed output.
Include the exact invocation and, where relevant, the full success or
failure block, e.g.

  $ waitfor --timeout 2s port 5432
  ✓ Port 5432 is open (waited 0ms)

  $ waitfor --timeout 1s port 9999
  ✗ Timed out after 1s

    Condition:  TCP localhost:9999
    Last state: connection refused
    Tip: ...
-->

---

## New condition type PRs only

Skip this section if you're not adding a subcommand.

- [ ] An open issue existed before I started work
- [ ] Condition implements `poller.Condition` (`Kind`, `Target`, `Describe`, `SuccessMessage`, `Check`)
- [ ] `Check` returns transient "not yet" states as `(state, false, nil)`; `err != nil` is reserved for fatal failures
- [ ] `Check` respects `ctx` — a long-running evaluation cancels when the poller's deadline fires, without orphaning child processes
- [ ] State strings returned by `Check` are documented in a comment and match the tip logic
- [ ] A tip function exists for the new `Kind` in `internal/tips` — or is deliberately empty, per spec
- [ ] `cmd/<name>.go` wires the condition through `buildPoller` + `finish`
- [ ] `newAllCmd` dispatch (`isConditionKeyword`, `buildSubCondition`) includes the new keyword
- [ ] Human output matches the standard failure block shape: `Condition:` / `Last state:` / optional detail / `Tip:`
- [ ] `--json` output carries `condition`, `target`, `success`, `elapsed_ms`, `timeout_ms`, `last_state`, `attempts`
- [ ] Unit tests for the condition package
- [ ] CLI tests in `cmd/` covering success, timeout, and bad-args (exit 2)
- [ ] `README.md` and `docs/ci-cd.md` updated with the new subcommand

---

## General checklist

- [ ] `make lint` passes (vet + gofmt clean)
- [ ] `make test` passes
- [ ] `make race` passes, required if the PR touches `internal/poller` or `cmd/all.go`
- [ ] PR title follows Conventional Commits (`feat:`, `fix:`, `docs:`, `test:`, `refactor:`, `chore:`, `ci:`)
- [ ] No new dependency unless the PR explains why the standard library is insufficient
- [ ] No changes to exit-code semantics (0/1/2/3) without updating `README.md` and `internal/exitcode`
- [ ] No secrets, tokens, or private hostnames in tests, examples, or fixtures
