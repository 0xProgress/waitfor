# Security policy

## Supported versions

The latest release on the `main` branch is supported. Older releases do
not receive security fixes. If you're pinning a version, pin the most
recent minor.

## Reporting a vulnerability

**Do not open a public issue for a security problem.**

Report privately via [GitHub Security Advisories](https://github.com/0xProgress/waitfor/security/advisories/new).
Include:

- What you found, in enough detail to reproduce
- Affected version (`waitfor --version`)
- Operating system
- Whether you intend to publish the finding, and on what timeline

You'll get an acknowledgement within **72 hours** and a fix or a
disclosure plan within **14 days** for issues we can reproduce. We'll
credit you in the release notes unless you ask otherwise.

If GitHub advisories aren't an option, email the maintainer listed in
`go.mod`. PGP is not currently available.

---

## What counts as a security issue

`waitfor` is a single-binary CLI that runs with the privileges of the
invoking user. It has no network listeners, no persistent state, no
credentials store, and no elevation path. Given that, the reportable
issues are narrow.

**In scope:**

- **Command injection outside the documented contract.** `waitfor command`
  runs `sh -c "<user-supplied string>"`. That is by design — the user's
  string is their string. A vulnerability would be a way for a string to
  escape that contract: shell metacharacters interpreted somewhere they
  shouldn't be, or arguments re-parsed after the fact.
- **Process-group escape.** `waitfor command` puts the child in its own
  process group and kills the group on cancellation. A way to spawn a
  child that survives cancellation, or to signal a process outside our
  own group, is a bug we want to hear about.
- **Symlink / path traversal in `waitfor file`.** The condition uses
  `os.Stat`, which follows symlinks. That's intentional. An unexpected
  way to make `waitfor file` report success for a path the invoking user
  cannot read, or to leak file contents through the failure output, is
  in scope.
- **Crash or panic on untrusted input.** Any input that causes
  `waitfor` to panic rather than return an error. The CLI should never
  crash on a URL, path, process name, header, or command string.
- **Resource exhaustion from a hostile endpoint.** `waitfor http`
  currently caps response bodies at 4 KiB and redirect chains at 10
  hops. A way to bypass either cap, or to otherwise make a single
  `Check` consume unbounded CPU or memory, is in scope.
- **TLS verification bypass.** `--insecure` is opt-in and clearly
  documented. A code path that skips verification _without_ the flag,
  or a way to make a cert failure be reported as success, is a bug.

**Out of scope:**

- **Running arbitrary commands.** `waitfor command "rm -rf /"` will try
  to `rm -rf /`. That is the point of the subcommand. It runs as the
  invoking user and never elevates. If you want to sandbox it, sandbox
  the whole invocation — do not file a bug.
- **Reading files the invoking user can read.** `waitfor file /etc/shadow`
  will report permission-denied if you can't read it, and success if you
  can. Both outcomes are correct.
- **Dialing arbitrary hosts or ports.** `waitfor port 1` on a remote
  host behind a firewall is a legitimate diagnostic. There is no SSRF
  surface because there is no privileged network context — the user's
  network access _is_ the tool's network access.
- **Verbose output leaking secrets.** `--verbose` prints the state string
  on every attempt. If you pass `--header "Authorization: Bearer xyz"`,
  the header itself is not printed, but a server response containing it
  could be surfaced via `Body (last)`. Consider what you feed into a
  command line before running it with `--verbose` in a shared log.
- **DNS resolution behavior.** `waitfor` uses the Go standard resolver.
  If you need a specific resolver, set `GODEBUG=netdns=...` or the
  appropriate `resolv.conf` — that's an environment concern, not a
  `waitfor` bug.
- **Windows process scanning.** The process condition returns an error
  on platforms other than Linux and macOS. That's a documented
  limitation, not a vulnerability.
- **Denial of service by the user against themselves.** `waitfor port 1`
  for 300 seconds hammers a closed port at 500 ms intervals. That's
  600 dials to localhost. If you find that objectionable, set
  `--interval` higher.

---

## Threat model in one paragraph

`waitfor` is a diagnostic tool. It assumes the invoking user is
trustworthy and the environment is untrusted. It does not assume any
privilege, does not persist state, and does not open listening sockets.
Its security-relevant behavior is limited to: spawning the shell the user
asked for, dialing the host the user named, reading the path the user
pointed at, and inspecting the processes the user can already see. The
threats it cares about are the ones that break those contracts.

---

## Hardening recommendations for CI

`waitfor` runs in build pipelines, which is a more adversarial
environment than a developer laptop. Recommended practices:

- **Pin a version.** `go install github.com/0xProgress/waitfor@v0.1.0`,
  not `@latest`. Verify the checksum from the release page.
- **Don't pass untrusted input to `command`.** If a script constructs a
  command string from user-controlled data, that's your injection
  surface, not waitfor's. Quote and validate before passing.
- **`--json` over `--verbose` in shared logs.** The verbose output
  includes state strings that may echo service responses. The JSON
  output is structured and easier to redact downstream.
- **Avoid `--insecure` unless the pipeline is already trusted.** If your
  job pulls from a public registry and dials a private service, a
  compromised registry entry could redirect the check to an
  attacker-controlled endpoint. Use certificates.

---

## Known limitations

These are documented in the README and are not vulnerabilities:

- Linux process names are truncated to 15 characters by the kernel
  (`TASK_COMM_LEN - 1`). `--exact` against a longer name will not match.
- `waitfor port` dials localhost only. Reaching a service by hostname
  requires `waitfor command "nc -z host port"` or `waitfor http`.
- `waitfor http` follows redirects up to 10 hops and then reports
  `redirect loop detected`. It does not expose the redirect chain in
  output.
- `waitfor file` follows symlinks; it does not detect loops itself
  (`os.Stat` returns ELOOP, which surfaces as a fatal error).

---

## Changelog

Security fixes are called out in the release notes with a `security:`
prefix. Subscribe to the repo's releases if you deploy `waitfor` in a
pipeline.
