// Package network implements waitfor conditions that probe the network:
// TCP ports and HTTP endpoints.
package network

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"syscall"
	"time"
)

// dialTimeout bounds a single TCP dial attempt so one Check can't consume
// the whole polling budget. The poller's context can still cancel earlier.
const dialTimeout = 5 * time.Second

// PortCondition waits for a TCP port on localhost to accept connections.
type PortCondition struct {
	port int
}

// NewPortCondition parses and validates a port string.
//
// Returns an error for non-numeric input, or a value outside [1, 65535].
// Port 0 is rejected immediately per the spec — it must never enter the
// polling loop.
func NewPortCondition(portStr string) (*PortCondition, error) {
	n, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, fmt.Errorf("invalid port %q: must be an integer", portStr)
	}
	if n < 1 || n > 65535 {
		return nil, fmt.Errorf("invalid port %d: must be between 1 and 65535", n)
	}
	return &PortCondition{port: n}, nil
}

func (c *PortCondition) Kind() string   { return "port" }
func (c *PortCondition) Target() string { return strconv.Itoa(c.port) }
func (c *PortCondition) Describe() string {
	return fmt.Sprintf("TCP localhost:%d", c.port)
}
func (c *PortCondition) SuccessMessage() string {
	return fmt.Sprintf("Port %d is open", c.port)
}

// Check dials 127.0.0.1 first, then [::1]. Reports the IPv4 state if both
// fail, since that's the address the user most likely meant.
func (c *PortCondition) Check(ctx context.Context) (string, bool, error) {
	v4State, v4OK := c.tryDial(ctx, "127.0.0.1")
	if v4OK {
		return "connected", true, nil
	}
	v6State, v6OK := c.tryDial(ctx, "::1")
	if v6OK {
		return "connected", true, nil
	}
	if v4State != "" {
		return v4State, false, nil
	}
	return v6State, false, nil
}

func (c *PortCondition) tryDial(ctx context.Context, host string) (string, bool) {
	addr := net.JoinHostPort(host, strconv.Itoa(c.port))
	d := net.Dialer{Timeout: dialTimeout}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err == nil {
		_ = conn.Close()
		return "connected", true
	}
	return classifyDialError(err), false
}

// classifyDialError maps syscall-level dial failures to the state strings
// the tips layer looks for.
func classifyDialError(err error) string {
	if err == nil {
		return ""
	}
	switch {
	case errors.Is(err, syscall.ECONNREFUSED):
		return "connection refused"
	case errors.Is(err, syscall.ETIMEDOUT),
		errors.Is(err, context.DeadlineExceeded):
		return "connection timeout"
	case errors.Is(err, syscall.EHOSTUNREACH),
		errors.Is(err, syscall.ENETUNREACH):
		return "no route to host"
	}

	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return "connection timeout"
	}
	return err.Error()
}
