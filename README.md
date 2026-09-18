# waitfor

> _Block until a condition is true, or tell you exactly why it wasn't._

`waitfor` polls a condition on a configurable interval until it becomes true
or a timeout expires. The killer feature isn't the polling — it's the
**failure output**. Every other tool says `timed out`. `waitfor` tells you
_why_.

```
$ waitfor port 5432 --timeout 5s
✗ Timed out after 5s

  Condition:  TCP localhost:5432
  Last state: connection refused

  Tip: Port 5432 is reachable but nothing is listening. Is the service running? Try: ss -ltn | grep 5432
```

---

## Install

### From source

```bash
go install github.com/0xProgress/waitfor@latest
```

### Prebuilt binaries

Download the latest release from the [Releases page](https://github.com/0xProgress/waitfor/releases).
Checksums and cross-platform archives are published for every tag.

### Build locally

```bash
git clone https://github.com/0xProgress/waitfor
cd waitfor
make build          # produces bin/waitfor
make install        # installs to $GOBIN
```

Requires Go 1.22 or newer.

---

## Quickstart

```bash
# Wait for Postgres on localhost
waitfor port 5432

# Wait for a service container to become healthy
waitfor http localhost:8080/health --timeout 60s

# Wait for a file to appear
waitfor file ./tmp/ready --nonempty

# Wait for a process
waitfor process postgres

# Wait for an arbitrary shell command
waitfor command "docker inspect mycontainer | jq -e '.State.Running'"

# Wait for several conditions at once
waitfor all port 5432 port 6379 http localhost:8080/health
```

---

## Subcommands

### `waitfor port <number>`

Waits for a TCP port on localhost to accept connections. Tries IPv4
(`127.0.0.1`) first, then IPv6 (`[::1]`).

| Failure state        | Meaning                                    |
| -------------------- | ------------------------------------------ |
| `connection refused` | Port is reachable but nothing is listening |
| `connection timeout` | Firewall or host unreachable               |
| `no route to host`   | Network-level failure                      |

```bash
waitfor port 5432
waitfor port 5432 --timeout 30s
```

### `waitfor http <url>`

Waits for an HTTP endpoint to return the expected status (default: any 2xx).

| Flag              | Default | Description                       |
| ----------------- | ------- | --------------------------------- |
| `--status <code>` | `0`     | Expect a specific status code     |
| `--method <verb>` | `GET`   | HTTP method                       |
| `--insecure`      | `false` | Skip TLS verification             |
| `--header "K: V"` | —       | Add a request header (repeatable) |

URLs without a scheme are auto-prepended with `http://` and a warning is
printed. On non-2xx responses, the last body is captured and shown.

```bash
waitfor http localhost:8080/health
waitfor http localhost:8080/health --timeout 60s --interval 2s
waitfor http https://api.example.com/ready --status 200 --header "X-Token: abc"
```

```
$ waitfor http localhost:8080/health --timeout 30s
✗ Timed out after 30s

  Condition:  HTTP GET localhost:8080/health
  Last state: 503 Service Unavailable
  Body (last): {"status":"starting","ready":false}

  Tip: localhost:8080/health is responding but returned an unexpected status. Try: curl -v localhost:8080/health
```

### `waitfor file <path>`

Waits for a file or directory to exist.

| Flag                 | Default | Description                   |
| -------------------- | ------- | ----------------------------- |
| `--min-size <bytes>` | `0`     | File must be at least N bytes |
| `--nonempty`         | `false` | Shorthand for `--min-size 1`  |

Distinguishes `not found`, `empty`, `size N < M`, `permission denied`, and
`parent directory missing`.

```bash
waitfor file ./tmp/ready
waitfor file ./dump.sql --min-size 1048576     # at least 1 MiB
waitfor file ./tmp/ready --nonempty
```

### `waitfor process <name>`

Waits for a process matching a name to be running (substring match by default).

| Flag                | Default | Description                             |
| ------------------- | ------- | --------------------------------------- |
| `--exact`           | `false` | Exact name match instead of substring   |
| `--user <username>` | —       | Only match processes owned by this user |

Runs on Linux (`/proc`) and macOS (`ps`). On other platforms, returns an
error immediately. Numeric PIDs are rejected with a hint to use
`waitfor command 'kill -0 <pid>'`.

```bash
waitfor process postgres
waitfor process postgres --exact
waitfor process postgres --user deploy
```

**Limitation:** Linux `comm` is capped at 15 characters by the kernel. Names
longer than 15 chars won't match on Linux. This is a kernel constraint, not
a `waitfor` bug.

### `waitfor command "<shell command>"`

Waits for a shell command to exit with the expected code.

| Flag              | Default | Description        |
| ----------------- | ------- | ------------------ |
| `--exit-code <n>` | `0`     | Expected exit code |

The command is passed to `sh -c` unmodified, so pipes, redirection, and
command substitution all work. It runs as the current user; `waitfor` never
elevates.

```bash
waitfor command "docker inspect mycontainer | jq -e '.State.Running'"
waitfor command "test -f /var/run/app.pid"
waitfor command "systemctl is-active --quiet nginx" --exit-code 0
```

```
$ waitfor command 'docker inspect mycontainer | jq ".State.Running"' --timeout 30s
✗ Timed out after 30s

  Condition:  command "docker inspect mycontainer | jq \".State.Running\""
  Last state: exit code 1
  Stderr:     Error: No such container: mycontainer
```

### `waitfor all <condition> [condition...]`

Waits for multiple conditions simultaneously. All must pass. Runs each
condition in its own goroutine.

Global flags may appear **anywhere** in the argument list. Sub-condition
flags (like `--status` or `--exit-code`) apply only to the condition they
follow.

```bash
waitfor all port 5432 port 6379
waitfor all --timeout 60s port 5432 process postgres
waitfor all http localhost:8080/health --status 200 file ./ready
```

```
$ waitfor all port 5432 port 6379 http localhost:8080/health
✗ Timed out after 30s

  2 of 3 conditions failed:

  TCP localhost:6379
    Last state: connection refused

  HTTP GET localhost:8080/health
    Last state: 503 Service Unavailable
```

---

## Global flags

| Flag         | Default | Description                |
| ------------ | ------- | -------------------------- |
| `--timeout`  | `30s`   | How long to wait total     |
| `--interval` | `500ms` | How often to poll          |
| `--quiet`    | `false` | No output, just exit codes |
| `--json`     | `false` | Machine-readable output    |
| `--verbose`  | `false` | Show each poll attempt     |

---

## Exit codes

| Code | Meaning                                                          |
| ---- | ---------------------------------------------------------------- |
| `0`  | Condition met                                                    |
| `1`  | Timeout — condition never met                                    |
| `2`  | Bad arguments / invalid input                                    |
| `3`  | Permission error _(reserved; no condition currently emits this)_ |

---

## JSON output

Every subcommand supports `--json`:

```json
{
  "condition": "port",
  "target": "5432",
  "success": false,
  "elapsed_ms": 30041,
  "timeout_ms": 30000,
  "last_state": "connection refused",
  "attempts": 60
}
```

Optional fields (`detail_label`, `detail_value`) are omitted when empty. For
`all`, a `children` array carries one entry per sub-condition.

---

## Use in CI/CD

See **[docs/ci-cd.md](docs/ci-cd.md)** for copy-paste recipes covering GitHub
Actions, GitLab CI, Docker Compose, and Kubernetes. The short version:

```yaml
# GitHub Actions
- name: Wait for Postgres
  run: waitfor port 5432 --timeout 30s
```

Why `waitfor` beats `sleep 10` in CI:

- **Fast on success.** Returns the instant the condition is met, not after a fixed delay.
- **Diagnostic on failure.** The timeout output tells you _which_ thing failed and _why_, so you don't have to re-run with more logging.
- **Deterministic in flaky environments.** Configurable timeout and interval let you tune per-job.

---

## Edge cases

`waitfor` handles these per spec:

- `--timeout 0s` performs a single check and reports `Condition not met (single check, no timeout set)`.
- `--interval` shorter than 100ms is clamped to 100ms.
- `--interval` longer than `--timeout` warns and performs a single check.
- `--timeout` or `--interval` < 0 is rejected with exit 2.
- URLs without a scheme are auto-prepended with `http://` and a warning is printed _before_ polling begins.
- SIGINT/SIGTERM cancel polling cleanly and exit 1 with the last observed state.
- `waitfor all` with zero conditions is an error, not a silent success.

---

## Development

```bash
make test        # go test ./...
make race        # go test -race ./...
make lint        # go vet ./...
make build       # build bin/waitfor
make install     # install to $GOBIN
make release     # goreleaser release --snapshot --clean
```

Package layout:

```
waitfor/
├── main.go
├── cmd/                  ← cobra commands
└── internal/
    ├── command/          ← shell command condition
    ├── exitcode/         ← exit-code constants and coded errors
    ├── file/             ← filesystem condition
    ├── network/          ← port and http conditions
    ├── output/           ← human and JSON renderers
    ├── poller/           ← the polling loop
    ├── process/          ← process condition (Linux + macOS)
    └── tips/             ← per-condition hint strings
```

---

## License

MIT — see [LICENSE](LICENSE).
