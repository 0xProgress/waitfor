package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/0xProgress/waitfor/internal/exitcode"
)

func TestHTTPCmdSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	out, err := runRoot(t, "--timeout", "2s", "http", srv.URL)
	if code := codeOf(t, err); code != 0 {
		t.Fatalf("exit = %d, err = %v\n%s", code, err, out)
	}
	if !strings.Contains(out, "✓ GET "+srv.URL) {
		t.Errorf("missing success line:\n%s", out)
	}
}

func TestHTTPCmdTimeout503(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"status":"starting","ready":false}`))
	}))
	defer srv.Close()

	out, err := runRoot(t,
		"--timeout", "300ms",
		"--interval", "80ms",
		"http", srv.URL,
	)
	if code := codeOf(t, err); code != exitcode.Timeout {
		t.Fatalf("exit = %d, want %d", code, exitcode.Timeout)
	}
	for _, want := range []string{
		"✗ Timed out after",
		"503 Service Unavailable",
		`Body (last): {"status":"starting","ready":false}`,
		"Tip:",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestHTTPCmdSpecificStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	// Match.
	_, err := runRoot(t, "--timeout", "2s", "http", srv.URL, "--status", "204")
	if code := codeOf(t, err); code != 0 {
		t.Errorf("exit = %d, want 0", code)
	}
	// Mismatch — will time out.
	_, err = runRoot(t,
		"--timeout", "200ms",
		"--interval", "50ms",
		"http", srv.URL, "--status", "200",
	)
	if code := codeOf(t, err); code != exitcode.Timeout {
		t.Errorf("exit = %d, want %d", code, exitcode.Timeout)
	}
}

func TestHTTPCmdInvalidURLExit2(t *testing.T) {
	for _, u := range []string{"http://", "ftp://example.com"} {
		_, err := runRoot(t, "http", u)
		if code := codeOf(t, err); code != exitcode.BadArgs {
			t.Errorf("url %q: exit = %d, want %d", u, code, exitcode.BadArgs)
		}
	}
}

func TestHTTPCmdInvalidHeaderExit2(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()

	_, err := runRoot(t, "http", srv.URL, "--header", "no-colon")
	if code := codeOf(t, err); code != exitcode.BadArgs {
		t.Errorf("exit = %d, want %d", code, exitcode.BadArgs)
	}
}

func TestHTTPCmdSchemeWarning(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// Strip "http://" to trigger the warning path.
	bare := strings.TrimPrefix(srv.URL, "http://")
	out, err := runRoot(t, "--timeout", "2s", "http", bare)
	if code := codeOf(t, err); code != 0 {
		t.Fatalf("exit = %d, err = %v", code, err)
	}
	if !strings.Contains(out, "warning:") || !strings.Contains(out, "http://") {
		t.Errorf("missing scheme warning:\n%s", out)
	}
}

func TestHTTPCmdJSON(t *testing.T) {
	var n int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&n, 1) < 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"starting":true}`))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	out, err := runRoot(t,
		"--json",
		"--timeout", "3s",
		"--interval", "100ms",
		"http", srv.URL,
	)
	if code := codeOf(t, err); code != 0 {
		t.Fatalf("exit = %d, err = %v\n%s", code, err, out)
	}

	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if got["condition"] != "http" {
		t.Errorf("condition = %v", got["condition"])
	}
	if got["success"] != true {
		t.Errorf("success = %v", got["success"])
	}
	if got["target"] != srv.URL {
		t.Errorf("target = %v, want %v", got["target"], srv.URL)
	}
	if _, present := got["detail_value"]; present {
		t.Errorf("detail_value should be omitted on success, got %v", got["detail_value"])
	}
}

func TestHTTPCmdJSONIncludesBodyOnFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"ready":false}`))
	}))
	defer srv.Close()

	out, err := runRoot(t,
		"--json",
		"--timeout", "200ms",
		"--interval", "50ms",
		"http", srv.URL,
	)
	if code := codeOf(t, err); code != exitcode.Timeout {
		t.Fatalf("exit = %d", code)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if got["detail_label"] != "Body (last)" {
		t.Errorf("detail_label = %v", got["detail_label"])
	}
	if got["detail_value"] != `{"ready":false}` {
		t.Errorf("detail_value = %v", got["detail_value"])
	}
}

func TestHTTPCmdQuiet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	out, err := runRoot(t, "--quiet", "--timeout", "2s", "http", srv.URL)
	if code := codeOf(t, err); code != 0 {
		t.Errorf("exit = %d", code)
	}
	if out != "" {
		t.Errorf("expected empty output, got %q", out)
	}
}
