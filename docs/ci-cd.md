# Using waitfor in CI/CD

`waitfor` is built for even CI. It exits fast on success, explains failures
clearly, and works as a static binary with no runtime dependencies.

---

## GitHub Actions

### Wait for a service container

```yaml
services:
  postgres:
    image: postgres:16
    env:
      POSTGRES_PASSWORD: postgres
    ports:
      - 5432:5432

steps:
  - name: Install waitfor
    run: go install github.com/0xProgress/waitfor@latest

  - name: Wait for Postgres
    run: waitfor port 5432 --timeout 30s

  - name: Wait for Redis
    run: waitfor port 6379 --timeout 30s

  - name: Wait for all services
    run: waitfor all port 5432 port 6379 --timeout 30s
```

### Wait for an HTTP health endpoint

```yaml
- name: Start server
  run: ./bin/server &

- name: Wait for server to be ready
  run: waitfor http localhost:8080/health --timeout 30s
```

### Wait for a file written by another step

```yaml
- name: Generate config
  run: some-tool --output /tmp/config.json

- name: Wait for config
  run: waitfor file /tmp/config.json --nonempty --timeout 10s
```

### Dogfooding — waitfor waiting for itself

```yaml
- name: Build waitfor
  run: go build -o /usr/local/bin/waitfor .

- name: Verify binary works
  run: waitfor command "waitfor --help" --timeout 5s
```

---

## GitLab CI

```yaml
test:
  image: golang:1.22
  services:
    - postgres:16
  before_script:
    - go install github.com/0xProgress/waitfor@latest
    - waitfor port 5432 --timeout 30s
  script:
    - go test ./...
```

---

## Docker Compose

In a `docker-compose.yml`, use `waitfor` in your app's entrypoint instead
of `depends_on` (which only waits for the container to start, not for the
service inside to be ready):

```dockerfile
# entrypoint.sh
#!/bin/sh
set -e
waitfor port 5432 --timeout 60s
waitfor port 6379 --timeout 60s
exec "$@"
```

```yaml
# docker-compose.yml
services:
  app:
    build: .
    entrypoint: ["/entrypoint.sh"]
    command: ["./bin/myapp"]
    depends_on:
      - postgres
      - redis
```

---

## Makefile

```makefile
.PHONY: dev-up
dev-up:
	docker compose up -d
	waitfor port 5432 --timeout 30s
	waitfor port 6379 --timeout 30s
	@echo "All services ready."
```

---

## Why waitfor beats `sleep N`

|                       | `sleep 10`                   | `waitfor`                                |
| --------------------- | ---------------------------- | ---------------------------------------- |
| Fast on success       | ✗ Always waits full duration | ✓ Returns the instant condition is met   |
| Diagnostic on failure | ✗ Silent                     | ✓ Reports last state + hint              |
| Tunable               | ✗ Hardcoded                  | ✓ `--timeout`, `--interval`              |
| Exit codes            | ✗ Always 0                   | ✓ 0 = success, 1 = timeout, 2 = bad args |
| Multiple conditions   | ✗ Chain sleeps               | ✓ `waitfor all` runs in parallel         |

---

## Exit codes reference

| Code | Meaning                         |
| ---- | ------------------------------- |
| `0`  | Condition met                   |
| `1`  | Timed out — condition never met |
| `2`  | Bad arguments / invalid input   |
| `3`  | Permission error                |
