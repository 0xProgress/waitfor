// Package poller implements the core polling loop shared by every waitfor
// condition: interval, timeout, verbose attempt callbacks, and clean
// cancellation via context.
package poller

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// MinInterval is the smallest allowed polling interval. Anything smaller is
// clamped to this to avoid hammering the system.
const MinInterval = 100 * time.Millisecond

// Condition is a single thing waitfor can wait for.
type Condition interface {
	Kind() string
	Target() string
	Describe() string
	SuccessMessage() string

	// Check performs one evaluation. It returns a human-readable state
	// string describing what was observed, whether the condition is met,
	// and a non-nil error only for fatal failures (permission denied,
	// invalid input). Transient "not yet" states use state + ok=false +
	// err=nil.
	Check(ctx context.Context) (state string, ok bool, err error)
}

// Warner is an optional interface a Condition may implement to inject
// pre-flight warnings into the Result.
type Warner interface {
	Warnings() []string
}

// Detailer is an optional interface a Condition may implement to expose a
// labeled, per-attempt detail (e.g. "Body (last)" for HTTP, "Stderr" for
// command) that the output layer renders alongside the state.
type Detailer interface {
	LastDetail() (label, value string)
}

// Result captures the outcome of polling a Condition.
type Result struct {
	Kind           string
	Target         string
	Describe       string
	SuccessMessage string
	Success        bool
	Elapsed        time.Duration
	Timeout        time.Duration
	LastState      string
	DetailLabel    string
	DetailValue    string
	Attempts       int
	Err            error
	Warnings       []string
	Interrupted    bool
}

// Poller drives the polling loop.
type Poller struct {
	Timeout  time.Duration
	Interval time.Duration
	Verbose  bool

	// OnAttempt, if non-nil and Verbose is true, is called after each Check.
	OnAttempt func(attempt int, state string, elapsed time.Duration, ok bool)
}

// Run polls c until it succeeds, times out, fails fatally, or ctx is
// cancelled. It always returns a Result describing what happened.
func (p *Poller) Run(ctx context.Context, c Condition) Result {
	// Wrap ctx with our own timeout so a single long-running Check can't
	// overshoot the overall budget. For Timeout <= 0 we don't wrap; the
	// single check runs unbounded.
	if p.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, p.Timeout)
		defer cancel()
	}

	interval := max(p.Interval, MinInterval)
	start := time.Now()

	r := Result{
		Kind:           c.Kind(),
		Target:         c.Target(),
		Describe:       c.Describe(),
		SuccessMessage: c.SuccessMessage(),
		Timeout:        p.Timeout,
	}

	if p.Timeout <= 0 {
		p.checkOnce(ctx, c, &r, start)
		r.Elapsed = time.Since(start)
		return r
	}

	if interval > p.Timeout {
		r.Warnings = append(r.Warnings, fmt.Sprintf(
			"interval (%s) is longer than timeout (%s); condition will only be checked once",
			interval, p.Timeout,
		))
		p.checkOnce(ctx, c, &r, start)
		r.Elapsed = time.Since(start)
		return r
	}

	deadline := start.Add(p.Timeout)
	for {
		// Deadline fired between polls: preserve the previous attempt's
		// LastState and return as a timeout.
		if p.handleCtxDone(ctx, &r, start) {
			return r
		}

		state, ok, err := c.Check(ctx)
		r.Attempts++

		// If the poller's deadline fired *during* Check, the state we just
		// observed may reflect the cancellation rather than the condition.
		// If a previous attempt already produced a real state, keep that
		// (it's the more informative diagnostic). Otherwise, record what
		// this attempt observed so the user has something to go on.
		if p.handleCtxDone(ctx, &r, start) {
			if r.LastState == "" && state != "" {
				r.LastState = state
			}
			return r
		}

		r.LastState = state
		if d, ok := c.(Detailer); ok {
			r.DetailLabel, r.DetailValue = d.LastDetail()
		}
		if p.Verbose && p.OnAttempt != nil {
			p.OnAttempt(r.Attempts, state, time.Since(start), ok)
		}

		if err != nil {
			r.Err = err
			r.Elapsed = time.Since(start)
			return r
		}
		if ok {
			r.Success = true
			r.Elapsed = time.Since(start)
			return r
		}

		remaining := time.Until(deadline)
		if remaining <= 0 {
			r.Elapsed = time.Since(start)
			return r
		}

		wait := min(interval, remaining)

		select {
		case <-ctx.Done():
			p.handleCtxDone(ctx, &r, start)
			return r
		case <-time.After(wait):
		}
	}
}

// checkOnce performs a single Check and records the outcome in r.
//
// Same ctx-expiry discipline as the main loop: if the deadline fires during
// Check, discard the cancellation-flavored state rather than reporting it
// as the condition's state.
func (p *Poller) checkOnce(ctx context.Context, c Condition, r *Result, start time.Time) {
	if p.handleCtxDone(ctx, r, start) {
		return
	}

	state, ok, err := c.Check(ctx)
	r.Attempts = 1

	if p.handleCtxDone(ctx, r, start) {
		if r.LastState == "" && state != "" {
			r.LastState = state
		}
		return
	}

	r.LastState = state
	if d, ok := c.(Detailer); ok {
		r.DetailLabel, r.DetailValue = d.LastDetail()
	}
	if p.Verbose && p.OnAttempt != nil {
		p.OnAttempt(1, state, time.Since(start), ok)
	}
	if err != nil {
		r.Err = err
		return
	}
	r.Success = ok
}

// handleCtxDone inspects ctx; if cancelled, records the appropriate state
// in r and returns true. DeadlineExceeded means our own timeout fired
// (a plain timeout). Canceled means the caller interrupted us.
func (p *Poller) handleCtxDone(ctx context.Context, r *Result, start time.Time) bool {
	err := ctx.Err()
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		r.Elapsed = time.Since(start)
		return true
	}
	r.Interrupted = true
	r.Elapsed = time.Since(start)
	return true
}
