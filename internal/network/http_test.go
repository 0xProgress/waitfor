package network

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewHTTPConditionPrependsScheme(t *testing.T) {
	c, err := NewHTTPCondition("localhost:8080/health", HTTPOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if c.Target() != "localhost:8080/health" {
		t.Errorf("Target = %q", c.Target())
	}
	warnings := c.Warnings()
	if len(warnings) != 1 || !strings.Contains(warnings[0], "http://localhost:8080/health") {
		t.Errorf("warnings = %v", warnings)
	}
}

func TestNewHTTPConditionKeepsExplicitScheme(t *testing.T) {
	c, err := NewHTTPCondition("https://example.com", HTTPOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got := c.Warnings(); len(got) != 0 {
		t.Errorf("unexpected warnings: %v", got)
	}
}

func TestNewHTTPConditionInvalidURL(t *testing.T) {
	cases := []string{"", "http://", "ftp://example.com", "://nope"}
	for _, in := range cases {
		if _, err := NewHTTPCondition(in, HTTPOptions{}); err == nil {
			t.Errorf("NewHTTPCondition(%q) = nil error", in)
		}
	}
}

func TestNewHTTPConditionInvalidHeader(t *testing.T) {
	_, err := NewHTTPCondition("http://example.com", HTTPOptions{
		Headers: []string{"no-colon-here"},
	})
	if err == nil {
		t.Fatal("expected error for header without colon")
	}
}

func TestHTTPConditionSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c, _ := NewHTTPCondition(srv.URL, HTTPOptions{})
	state, ok, err := c.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatalf("expected success, state = %q", state)
	}
	if !strings.HasPrefix(state, "200") {
		t.Errorf("state = %q", state)
	}
}

func TestHTTPConditionSpecificStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	c, _ := NewHTTPCondition(srv.URL, HTTPOptions{WantCode: 201})
	if _, ok, _ := c.Check(context.Background()); !ok {
		t.Fatal("expected 201 to match")
	}

	c2, _ := NewHTTPCondition(srv.URL, HTTPOptions{WantCode: 200})
	state, ok, _ := c2.Check(context.Background())
	if ok {
		t.Fatal("expected 200 not to match 201")
	}
	if !strings.HasPrefix(state, "201") {
		t.Errorf("state = %q, want 201...", state)
	}
}

func TestHTTPConditionNon2xxCapturesBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"status":"starting","ready":false}`))
	}))
	defer srv.Close()

	c, _ := NewHTTPCondition(srv.URL, HTTPOptions{})
	state, ok, err := c.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected failure")
	}
	if !strings.HasPrefix(state, "503") {
		t.Errorf("state = %q", state)
	}
	if _, body := c.LastDetail(); body != `{"status":"starting","ready":false}` {
		t.Errorf("body = %q", body)
	}
}

func TestHTTPConditionSucceedsAfterRetries(t *testing.T) {
	var n int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&n, 1) < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c, _ := NewHTTPCondition(srv.URL, HTTPOptions{})

	if _, ok, _ := c.Check(context.Background()); ok {
		t.Fatal("first check should fail")
	}
	if _, ok, _ := c.Check(context.Background()); ok {
		t.Fatal("second check should fail")
	}
	if _, ok, _ := c.Check(context.Background()); !ok {
		t.Fatal("third check should succeed")
	}
	// After success, body should be cleared.
	if _, body := c.LastDetail(); body != "" {
		t.Errorf("body after success = %q, want empty", body)
	}
}

func TestHTTPConditionConnectionRefused(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close()

	c, _ := NewHTTPCondition(url, HTTPOptions{})
	state, ok, _ := c.Check(context.Background())
	if ok {
		t.Fatal("expected failure")
	}
	if !strings.Contains(state, "connection refused") {
		t.Errorf("state = %q, want connection refused", state)
	}
}

func TestHTTPConditionDNSFailure(t *testing.T) {
	c, _ := NewHTTPCondition("http://this-host-does-not-exist.invalid/x", HTTPOptions{})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	state, ok, _ := c.Check(ctx)
	if ok {
		t.Fatal("expected failure")
	}
	if !strings.Contains(state, "DNS") {
		t.Errorf("state = %q, want DNS error", state)
	}
}

func TestHTTPConditionRedirectLoop(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, srv.URL+"/next", http.StatusFound)
	}))
	defer srv.Close()

	c, _ := NewHTTPCondition(srv.URL, HTTPOptions{})
	state, ok, _ := c.Check(context.Background())
	if ok {
		t.Fatal("expected failure")
	}
	if !strings.Contains(state, "redirect") {
		t.Errorf("state = %q, want redirect", state)
	}
}

func TestHTTPConditionHeadersSent(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("X-Token")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c, _ := NewHTTPCondition(srv.URL, HTTPOptions{
		Headers: []string{"X-Token: abc123"},
	})
	if _, ok, _ := c.Check(context.Background()); !ok {
		t.Fatal("expected success")
	}
	if got != "abc123" {
		t.Errorf("header = %q, want abc123", got)
	}
}

func TestHTTPConditionMethodSent(t *testing.T) {
	var method string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c, _ := NewHTTPCondition(srv.URL, HTTPOptions{Method: "HEAD"})
	if _, ok, _ := c.Check(context.Background()); !ok {
		t.Fatal("expected success")
	}
	if method != "HEAD" {
		t.Errorf("method = %q, want HEAD", method)
	}
}

func TestHTTPConditionInsecureTLS(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// Without --insecure: TLS error.
	c1, _ := NewHTTPCondition(srv.URL, HTTPOptions{})
	state, ok, _ := c1.Check(context.Background())
	if ok {
		t.Fatal("expected TLS failure without --insecure")
	}
	if !strings.Contains(state, "TLS") {
		t.Errorf("state = %q, want TLS error", state)
	}

	// With --insecure: success.
	c2, _ := NewHTTPCondition(srv.URL, HTTPOptions{Insecure: true})
	if _, ok, _ := c2.Check(context.Background()); !ok {
		t.Fatal("expected success with --insecure")
	}
}

func TestTruncateBody(t *testing.T) {
	long := strings.Repeat("x", 500)
	got := truncateBody([]byte(long))
	if len(got) > maxBodyDisplay+3 {
		t.Errorf("len = %d, want <= %d", len(got), maxBodyDisplay+3)
	}
	if !strings.HasSuffix(got, "…") {
		t.Errorf("missing ellipsis: %q", got)
	}
	// Whitespace collapsing.
	got = truncateBody([]byte("  a\n\n  b\tc  "))
	if got != "a b c" {
		t.Errorf("got %q, want %q", got, "a b c")
	}
}

func TestHTTPConditionRepeatedHeaders(t *testing.T) {
	var got []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = append([]string(nil), r.Header.Values("X-Tag")...)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c, _ := NewHTTPCondition(srv.URL, HTTPOptions{
		Headers: []string{"X-Tag: a", "X-Tag: b"},
	})
	if _, ok, _ := c.Check(context.Background()); !ok {
		t.Fatal("expected success")
	}
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("headers = %v, want [a b]", got)
	}
}
