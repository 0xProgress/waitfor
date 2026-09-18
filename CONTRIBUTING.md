# Contributing to waitfor

Thanks for considering a contribution. This document covers the local
setup, the shape of the codebase, how to add a condition type, and what
we're looking for in a pull request.

By participating you agree to the [Code of Conduct](CODE_OF_CONDUCT.md).

---

## Development setup

Requirements:

- Go **1.22** or newer (the project uses `errors.AsType`, `for range int`, and other post-1.22 idioms)
- `make` (optional but recommended)
- `goreleaser` (only if you're cutting a release)

```bash
git clone https://github.com/0xProgress/waitfor
cd waitfor
make build          # bin/waitfor
make test           # go test ./...
make race           # go test -race ./...
make lint           # go vet + gofmt clean
```

`make help` lists every target.

---

## Project structure

```
waitfor/
├── main.go                  ← entry point; signal wiring + exit-code mapping
├── cmd/                     ← cobra commands and shared helpers
│   ├── root.go              ← global flags, PersistentPreRunE, Normalize
│   ├── poll.go              ← buildPoller / finish, shared by every subcommand
│   ├── port.go http.go file.go process.go command.go all.go
│   └── *_test.go
├── internal/
│   ├── poller/              ← the polling loop (Condition, Result, Poller)
│   ├── output/              ← human and JSON renderers
│   ├── tips/                ← per-condition hint strings
│   ├── exitcode/            ← codes 0/1/2/3 and the coded *Error type
│   ├── network/             ← port and http conditions
│   ├── file/                ← filesystem condition
│   ├── process/             ← process condition (Linux + macOS)
│   ├── command/             ← shell command condition
│   └── version/             ← build-time metadata injected via ldflags
├── docs/
│   └── ci-cd.md
└── .github/
```

The dividing line: `cmd/` is presentation and wiring. Everything under
`internal/` is library code that could be tested without a CLI.

---

## Local testing

Tests are the contract. Every bug fix starts with a failing test; every
new condition starts with tests for the condition package.

- `make test` — the default suite. Should always pass.
- `make race` — required before opening a PR that touches `internal/poller`
  or `cmd/all.go`. The parallel `all` mode and the poller's `OnAttempt`
  callback are the only places where concurrency matters.
- `make lint` — `go vet` and a `gofmt` cleanliness check. CI runs this.

There is no integration test harness. Most conditions are tested against
real OS primitives on localhost: `httptest` servers, `net.Listen` on
`127.0.0.1:0`, `t.TempDir()` for files, the test binary itself for
`process`.

Two test conventions worth following:

1. **Skip visibly, don't pass silently.** If a test can't run in a given
   environment (IPv6 unavailable, running as root, firewall drops SYN),
   `t.Skip` with a reason. Silence hides coverage gaps.
2. **No `time.Sleep` in tests unless there's no alternative.** Prefer
   channels, `t.Cleanup`, and context deadlines. The poller tests use a
   scripting fake condition rather than real timers.

---

## Adding a new condition type

A condition is anything that implements `poller.Condition`:

```go
type Condition interface {
	Kind() string
	Target() string
	Describe() string
	SuccessMessage() string
	Check(ctx context.Context) (state string, ok bool, err error)
}
```

Checklist for a new condition (e.g. `waitfor grpc`):

1. **Package:** `internal/<name>/<name>.go` with the condition, plus
   `<name>_test.go`. Keep the package free of CLI concerns.
2. **States are strings.** `Check` returns a short, human-readable state
   like `"connection refused"` or `"503 Service Unavailable"`. Transient
   failures use `(state, false, nil)`. Only fatal failures return
   `err != nil` — those map to exit code 2 or 3.
3. **Respect `ctx`.** A long-running `Check` must cancel when the poller's
   deadline fires. If your condition spawns processes, put them in their
   own process group and kill the group on cancel (see `internal/command`).
4. **Pre-flight validation.** Invalid input (missing host, port 0,
   malformed header) should be rejected in `New<Name>Condition`, not
   inside `Check`. That gives the user exit 2 immediately, not after a
   30-second poll.
5. **Optional interfaces.** Implement `poller.Warner` (`Warnings() []string`)
   for advisories shown before polling starts, and `poller.Detailer`
   (`LastDetail() (label, value string)`) for a per-attempt detail line
   like HTTP's `Body (last)`.
6. **Tip.** Add a tip function in `internal/tips/tips.go` for each state
   your condition can return, keyed on `Kind`. Return `""` when no useful
   hint exists — the output layer skips empty tips.
7. **CLI.** `cmd/<name>.go` should construct the condition, call
   `buildPoller` and `finish`. Do not duplicate the exit-code mapping.
8. **`all` dispatch.** Add the keyword to `isConditionKeyword` and a
   case to `buildSubCondition` in `cmd/all.go`. `buildXCondition` should
   use the same `newXxxCmd()` constructor as the standalone subcommand so
   flags stay in sync.
9. **Tests.** Unit tests for the condition (state classification, ctx
   cancellation, bad-args rejection) and CLI tests for success / timeout /
   exit-2. Include an `all` test that mixes the new kind with an existing
   one.
10. **Docs.** Add the subcommand to `README.md` with its flags and one
    failure-output example, and to `docs/ci-cd.md` if it's useful in CI.

Use the existing subcommands as templates. `internal/command` is the
smallest complete example; `internal/network` shows how to handle a
variety of failure states cleanly.

---

## Commit conventions

We follow [Conventional Commits](https://www.conventionalcommits.org/):

```
feat: add waitfor grpc subcommand
fix: preserve last state through deadline in poller
docs: document --json shape for all subcommand
test: cover IPv6 fallback in port condition
refactor: extract finish() helper for subcommands
chore: bump cobra to 1.8.1
ci: run race detector on Linux and macOS
```

The type prefix matters — `.goreleaser.yml` groups changelog entries by it.
Commits without a recognized prefix land under "Others".

Keep commits small and focused. A PR that adds a condition _and_
refactors the poller is two PRs. Squashing on merge is fine.

---

## Pull request process

1. Open an issue first for anything beyond a typo or a one-line fix. It's
   cheap and prevents working on something that won't be merged.
2. Fill out the PR template. The "How I tested it" section is not optional —
   include commands and observed output.
3. CI must pass (`make lint`, `make test`, `make race`).
4. One approval from a maintainer. No self-merges.
5. Keep the PR small. If it's over ~400 lines of diff, split it.

Reviewers care about, in order:

1. **Correctness.** Does it do what the description says?
2. **Tests.** Do they exercise the interesting cases, including failure
   states and cancellation?
3. **Diagnostic value.** Does the failure output tell the user something
   useful? This is the whole point of the tool.
4. **Consistency.** Does it match the shape of the existing conditions?

Style nits are welcome but never block a merge. Behavioral nits do.

---

## What we won't merge

- **Dependencies for their own sake.** The tool currently depends only on
  `cobra` + `pflag`. If you can do it with the standard library, do that.
  A new dependency needs a reason in the PR description.
- **"Failures" that aren't.** If `waitfor` correctly timed out because
  your service isn't running, that's the tool working. It's not a bug.
- **Skipping the `Check` contract.** Conditions that block indefinitely,
  ignore `ctx`, or return fatal errors for transient failures will be
  sent back.
- **Color output without `NO_COLOR` support.** If we ever add color, it
  must respect `NO_COLOR` and non-TTY writers. Not in scope today.
- **Anything that requires elevated privileges.** `waitfor` runs as the
  invoking user, full stop. No `sudo`, no setuid, no capabilities.

---

## Reporting bugs

Use the [bug report template](.github/ISSUE_TEMPLATE/bug_report.yml). The
single most useful thing you can include is the **full failure block** —
the `Condition:` / `Last state:` / `Tip:` output. That block is the
diagnostic; without it we're guessing.

For security issues, see [SECURITY.md](SECURITY.md). Do not open a public
issue.

---

## License

By contributing, you agree your contributions are licensed under the MIT
License (see [LICENSE](LICENSE)).
