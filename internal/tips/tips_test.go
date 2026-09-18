package tips

import (
	"strings"
	"testing"
)

func TestPortTipConnectionRefused(t *testing.T) {
	got := For("port", "5432", "connection refused")
	if !strings.Contains(got, "5432") {
		t.Errorf("tip should mention port: %q", got)
	}
	if !strings.Contains(got, "listening") {
		t.Errorf("unexpected tip: %q", got)
	}
}

func TestPortTipNoRoute(t *testing.T) {
	got := For("port", "5432", "no route to host")
	if !strings.Contains(got, "5432") {
		t.Errorf("tip should mention port: %q", got)
	}
}

func TestPortTipTimeout(t *testing.T) {
	got := For("port", "5432", "connection timeout")
	if !strings.Contains(got, "5432") {
		t.Errorf("tip should mention port: %q", got)
	}
}

func TestHTTPTipConnectionRefused(t *testing.T) {
	url := "localhost:8080/health"
	got := For("http", url, "connection refused")
	if !strings.Contains(got, url) {
		t.Errorf("tip should mention URL: %q", got)
	}
}

func TestHTTPTipNon2xx(t *testing.T) {
	url := "localhost:8080/health"
	got := For("http", url, "503 Service Unavailable")
	if !strings.Contains(got, url) {
		t.Errorf("tip should mention URL: %q", got)
	}
}

func TestHTTPTipDNS(t *testing.T) {
	got := For("http", "nope.invalid/x", "DNS resolution failed")
	if !strings.Contains(got, "nope.invalid/x") {
		t.Errorf("tip should mention URL: %q", got)
	}
}

func TestHTTPTipTLS(t *testing.T) {
	got := For("http", "https://example.test", "TLS handshake failed")
	if !strings.Contains(got, "https://example.test") {
		t.Errorf("tip should mention URL: %q", got)
	}
	if !strings.Contains(got, "--insecure") {
		t.Errorf("tip should mention --insecure: %q", got)
	}
}

func TestHTTPTipRedirectLoop(t *testing.T) {
	got := For("http", "localhost:8080", "redirect loop detected")
	if !strings.Contains(got, "localhost:8080") {
		t.Errorf("tip should mention URL: %q", got)
	}
}

func TestFileTipNotFound(t *testing.T) {
	got := For("file", "./tmp/ready", "not found")
	if !strings.Contains(got, "./tmp/ready") {
		t.Errorf("tip should mention path: %q", got)
	}
}

func TestFileTipEmpty(t *testing.T) {
	got := For("file", "./tmp/ready", "empty")
	if !strings.Contains(got, "./tmp/ready") {
		t.Errorf("tip should mention path: %q", got)
	}
}

func TestFileTipPermission(t *testing.T) {
	got := For("file", "/var/lib/data", "permission denied")
	if !strings.Contains(got, "/var/lib/data") {
		t.Errorf("tip should mention path: %q", got)
	}
}

func TestProcessTip(t *testing.T) {
	got := For("process", "postgres", "no matching process found")
	if !strings.Contains(got, "postgres") {
		t.Errorf("tip should mention name: %q", got)
	}
	if !strings.Contains(got, "ps aux") {
		t.Errorf("tip should mention ps aux: %q", got)
	}
}

func TestCommandTipIsEmpty(t *testing.T) {
	// The spec shows no Tip line for command failures.
	got := For("command", "docker inspect mycontainer", "exit code 1")
	if got != "" {
		t.Errorf("expected empty tip, got %q", got)
	}
}

func TestUnknownKindReturnsEmpty(t *testing.T) {
	if got := For("nope", "x", "y"); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}
