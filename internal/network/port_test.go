package network

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"
)

func TestNewPortConditionValid(t *testing.T) {
	for _, in := range []string{"1", "80", "5432", "65535"} {
		c, err := NewPortCondition(in)
		if err != nil {
			t.Errorf("NewPortCondition(%q) error: %v", in, err)
			continue
		}
		if c.Target() != in {
			t.Errorf("Target() = %q, want %q", c.Target(), in)
		}
	}
}

func TestNewPortConditionInvalid(t *testing.T) {
	cases := []string{"0", "-1", "65536", "abc", "12.5", ""}
	for _, in := range cases {
		if _, err := NewPortCondition(in); err == nil {
			t.Errorf("NewPortCondition(%q) = nil error, want failure", in)
		}
	}
}

func TestPortConditionKindAndDescription(t *testing.T) {
	c, _ := NewPortCondition("5432")
	if c.Kind() != "port" {
		t.Errorf("Kind = %q", c.Kind())
	}
	if c.Describe() != "TCP localhost:5432" {
		t.Errorf("Describe = %q", c.Describe())
	}
	if c.SuccessMessage() != "Port 5432 is open" {
		t.Errorf("SuccessMessage = %q", c.SuccessMessage())
	}
}

// freePort opens a listener on an ephemeral port and returns it plus the
// port number. Caller must close the listener.
func freePort(t *testing.T) (net.Listener, string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	_, port, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		ln.Close()
		t.Fatalf("split: %v", err)
	}
	return ln, port
}

func TestPortConditionSuccess(t *testing.T) {
	ln, port := freePort(t)
	defer ln.Close()

	c, err := NewPortCondition(port)
	if err != nil {
		t.Fatal(err)
	}
	state, ok, err := c.Check(context.Background())
	if err != nil {
		t.Fatalf("Check err: %v", err)
	}
	if !ok {
		t.Fatalf("expected success, state = %q", state)
	}
	if state != "connected" {
		t.Errorf("state = %q, want connected", state)
	}
}

func TestPortConditionRefused(t *testing.T) {
	// Bind then close to get a port that's very likely free.
	ln, port := freePort(t)
	ln.Close()

	c, _ := NewPortCondition(port)
	// Try a few times — nothing is listening, but the OS may hold the port
	// briefly. State should be "connection refused" once things settle.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		state, ok, err := c.Check(context.Background())
		if err != nil {
			t.Fatalf("Check err: %v", err)
		}
		if !ok && state == "connection refused" {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("never observed connection refused")
}

func TestPortConditionContextCancel(t *testing.T) {
	// Use a port that is not listening on localhost. Cancellation should
	// propagate through DialContext.
	c, _ := NewPortCondition("1") // privileged, refused fast
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	state, ok, err := c.Check(ctx)
	if ok {
		t.Fatalf("expected failure, state = %q", state)
	}
	if err != nil && !strings.Contains(err.Error(), "context canceled") {
		// Some kernels return refused before honoring ctx; that's fine,
		// but we should never return a bogus success.
		t.Logf("Check returned err on canceled ctx: %v", err)
	}
}

// TestPortConditionIPv6Fallback binds only on IPv6 loopback and confirms
// the condition still reports success via the [::1] fallback path.
func TestPortConditionIPv6Fallback(t *testing.T) {
	ln, err := net.Listen("tcp6", "[::1]:0")
	if err != nil {
		t.Skipf("IPv6 loopback unavailable: %v", err)
	}
	defer ln.Close()

	_, port, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}

	c, err := NewPortCondition(port)
	if err != nil {
		t.Fatal(err)
	}
	state, ok, err := c.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatalf("expected IPv6 fallback to succeed, state = %q", state)
	}
	if state != "connected" {
		t.Errorf("state = %q, want connected", state)
	}
}
