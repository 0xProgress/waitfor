package poller

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeState struct {
	state string
	ok    bool
	err   error
	delay time.Duration
}

// fakeCondition scripts a sequence of states. The last state repeats once
// the sequence is exhausted.
type fakeCondition struct {
	kind   string
	target string
	states []fakeState
	idx    int
}

func (f *fakeCondition) Kind() string           { return f.kind }
func (f *fakeCondition) Target() string         { return f.target }
func (f *fakeCondition) Describe() string       { return "fake " + f.target }
func (f *fakeCondition) SuccessMessage() string { return "fake " + f.target + " ready" }

func (f *fakeCondition) Check(ctx context.Context) (string, bool, error) {
	i := f.idx
	f.idx++

	var s fakeState
	if len(f.states) == 0 {
		return "no states", false, nil
	}
	if i < len(f.states) {
		s = f.states[i]
	} else {
		s = f.states[len(f.states)-1]
	}
	if s.delay > 0 {
		select {
		case <-time.After(s.delay):
		case <-ctx.Done():
			return "interrupted", false, ctx.Err()
		}
	}
	return s.state, s.ok, s.err
}

func TestRunImmediateSuccess(t *testing.T) {
	c := &fakeCondition{kind: "test", target: "x", states: []fakeState{
		{state: "ready", ok: true},
	}}
	p := &Poller{Timeout: time.Second, Interval: 10 * time.Millisecond}
	r := p.Run(context.Background(), c)
	if !r.Success {
		t.Fatalf("expected success, got %+v", r)
	}
	if r.Attempts != 1 {
		t.Errorf("attempts = %d, want 1", r.Attempts)
	}
	if r.LastState != "ready" {
		t.Errorf("last state = %q", r.LastState)
	}
}

func TestRunSucceedsAfterRetries(t *testing.T) {
	c := &fakeCondition{kind: "test", target: "x", states: []fakeState{
		{state: "starting"},
		{state: "starting"},
		{state: "ready", ok: true},
	}}
	p := &Poller{Timeout: time.Second, Interval: 10 * time.Millisecond}
	r := p.Run(context.Background(), c)
	if !r.Success {
		t.Fatalf("expected success, got %+v", r)
	}
	if r.Attempts != 3 {
		t.Errorf("attempts = %d, want 3", r.Attempts)
	}
}

func TestRunTimesOut(t *testing.T) {
	c := &fakeCondition{kind: "test", target: "x", states: []fakeState{
		{state: "not ready"},
	}}
	p := &Poller{Timeout: 150 * time.Millisecond, Interval: 40 * time.Millisecond}
	r := p.Run(context.Background(), c)
	if r.Success {
		t.Fatalf("expected timeout, got success")
	}
	if r.Err != nil {
		t.Errorf("unexpected fatal err: %v", r.Err)
	}
	if r.Attempts < 2 {
		t.Errorf("expected multiple attempts, got %d", r.Attempts)
	}
	if r.Elapsed < 150*time.Millisecond {
		t.Errorf("elapsed %s < timeout", r.Elapsed)
	}
}

func TestRunTimeoutZeroSingleCheck(t *testing.T) {
	c := &fakeCondition{kind: "test", target: "x", states: []fakeState{
		{state: "not ready"},
	}}
	p := &Poller{Timeout: 0, Interval: 500 * time.Millisecond}
	r := p.Run(context.Background(), c)
	if r.Attempts != 1 {
		t.Errorf("attempts = %d, want 1 (timeout 0 = single check)", r.Attempts)
	}
	if r.Success {
		t.Errorf("expected failure")
	}
}

func TestRunIntervalClampedToMin(t *testing.T) {
	// Interval of 1ms is clamped to 100ms. With a 250ms timeout, that
	// yields ~3 attempts.
	c := &fakeCondition{kind: "test", target: "x", states: []fakeState{
		{state: "not ready"},
	}}
	p := &Poller{Timeout: 250 * time.Millisecond, Interval: 1 * time.Millisecond}
	r := p.Run(context.Background(), c)
	if r.Attempts > 4 {
		t.Errorf("attempts = %d; interval not clamped (would be ~250)", r.Attempts)
	}
	if r.Attempts < 2 {
		t.Errorf("attempts = %d; expected at least 2", r.Attempts)
	}
}

func TestRunIntervalGreaterThanTimeoutWarns(t *testing.T) {
	c := &fakeCondition{kind: "test", target: "x", states: []fakeState{
		{state: "not ready"},
	}}
	p := &Poller{Timeout: 200 * time.Millisecond, Interval: 5 * time.Second}
	r := p.Run(context.Background(), c)
	if len(r.Warnings) == 0 {
		t.Fatalf("expected warning about interval > timeout")
	}
	if r.Attempts != 1 {
		t.Errorf("attempts = %d, want 1", r.Attempts)
	}
}

func TestRunFatalErrorStopsImmediately(t *testing.T) {
	fatal := errors.New("permission denied")
	c := &fakeCondition{kind: "test", target: "x", states: []fakeState{
		{state: "denied", err: fatal},
	}}
	p := &Poller{Timeout: time.Second, Interval: 10 * time.Millisecond}
	r := p.Run(context.Background(), c)
	if r.Success {
		t.Fatalf("expected failure")
	}
	if !errors.Is(r.Err, fatal) {
		t.Errorf("err = %v, want %v", r.Err, fatal)
	}
	if r.Attempts != 1 {
		t.Errorf("attempts = %d, want 1", r.Attempts)
	}
}

func TestRunCancelContextInterrupted(t *testing.T) {
	// Delay inside Check so cancellation lands while a check is in flight,
	// not between checks. This makes the assertion deterministic.
	c := &fakeCondition{kind: "test", target: "x", states: []fakeState{
		{state: "waiting", delay: 40 * time.Millisecond},
	}}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(60 * time.Millisecond)
		cancel()
	}()
	p := &Poller{Timeout: 5 * time.Second, Interval: 50 * time.Millisecond}
	r := p.Run(ctx, c)
	if r.Success {
		t.Fatalf("expected interruption, got success")
	}
	if !r.Interrupted {
		t.Errorf("Interrupted = false")
	}
}

func TestRunVerboseCallbackFires(t *testing.T) {
	c := &fakeCondition{kind: "test", target: "x", states: []fakeState{
		{state: "a"}, {state: "b"}, {state: "ready", ok: true},
	}}
	var calls []string
	p := &Poller{
		Timeout:  time.Second,
		Interval: 10 * time.Millisecond,
		Verbose:  true,
		OnAttempt: func(n int, state string, _ time.Duration, _ bool) {
			calls = append(calls, state)
		},
	}
	_ = p.Run(context.Background(), c)
	if len(calls) != 3 {
		t.Errorf("OnAttempt calls = %d, want 3", len(calls))
	}
}
