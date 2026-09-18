// Package output renders poller results as human text or JSON.
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/0xProgress/waitfor/internal/poller"
	"github.com/0xProgress/waitfor/internal/tips"
)

// Options controls rendering.
type Options struct {
	Quiet   bool
	JSON    bool
	Verbose bool
}

// jsonResult is the wire shape for --json.
type jsonResult struct {
	Condition   string `json:"condition"`
	Target      string `json:"target"`
	Success     bool   `json:"success"`
	ElapsedMS   int64  `json:"elapsed_ms"`
	TimeoutMS   int64  `json:"timeout_ms"`
	LastState   string `json:"last_state"`
	DetailLabel string `json:"detail_label,omitempty"`
	DetailValue string `json:"detail_value,omitempty"`
	Attempts    int    `json:"attempts"`
}

// Render writes r to w according to opts.
func Render(w io.Writer, r poller.Result, opts Options) error {
	if opts.Quiet {
		return nil
	}
	if opts.JSON {
		return writeJSON(w, r)
	}
	return writeHuman(w, r)
}

func writeJSON(w io.Writer, r poller.Result) error {
	out := jsonResult{
		Condition:   r.Kind,
		Target:      r.Target,
		Success:     r.Success,
		ElapsedMS:   r.Elapsed.Milliseconds(),
		TimeoutMS:   r.Timeout.Milliseconds(),
		LastState:   r.LastState,
		DetailLabel: r.DetailLabel,
		DetailValue: r.DetailValue,
		Attempts:    r.Attempts,
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func writeHuman(w io.Writer, r poller.Result) error {
	for _, warn := range r.Warnings {
		fmt.Fprintf(w, "warning: %s\n", warn)
	}

	if r.Success {
		fmt.Fprintf(w, "✓ %s (waited %s)\n", r.SuccessMessage, FormatDuration(r.Elapsed))
		return nil
	}

	if r.Interrupted {
		fmt.Fprintf(w, "✗ Interrupted\n\n")
		writeDetailBlock(w, r)
		return nil
	}

	if r.Timeout == 0 {
		fmt.Fprintf(w, "✗ Condition not met (single check, no timeout set)\n\n")
	} else {
		fmt.Fprintf(w, "✗ Timed out after %s\n\n", FormatDuration(r.Timeout))
	}
	writeDetailBlock(w, r)

	if tip := tips.For(r.Kind, r.Target, r.LastState); tip != "" {
		fmt.Fprintf(w, "\n  Tip: %s\n", tip)
	}
	return nil
}

// FormatDuration renders a duration in a compact, human-friendly form.
//
//	0             -> "0ms"
//	250ms         -> "250ms"
//	1500ms        -> "1.5s"
//	30s           -> "30s"
//	3200ms        -> "3.2s"
func FormatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d%time.Second == 0 {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}

// AttemptLine writes a single verbose poll-attempt line, e.g.
//
//	✗ attempt 3: connection refused (1.2s)
func AttemptLine(w io.Writer, n int, state string, elapsed time.Duration, ok bool) {
	mark := "✗"
	if ok {
		mark = "✓"
	}
	fmt.Fprintf(w, "  %s attempt %d: %s (%s)\n", mark, n, state, FormatDuration(elapsed))
}

// writeDetailBlock emits the Condition / Last state / optional detail lines,
// aligned so values start at column 14 (labels padded to 11, plus a space).
func writeDetailBlock(w io.Writer, r poller.Result) {
	fmt.Fprintf(w, "  %-11s %s\n", "Condition:", r.Describe)
	fmt.Fprintf(w, "  %-11s %s\n", "Last state:", r.LastState)
	if r.DetailValue != "" {
		label := r.DetailLabel
		if label == "" {
			label = "Detail"
		}
		fmt.Fprintf(w, "  %-11s %s\n", label+":", r.DetailValue)
	}
}

// AllResult captures the outcome of polling multiple conditions in parallel.
type AllResult struct {
	Children    []poller.Result
	Success     bool
	Elapsed     time.Duration
	Timeout     time.Duration
	Interrupted bool
}

// RenderAll writes a multi-condition result.
func RenderAll(w io.Writer, r AllResult, opts Options) error {
	if opts.Quiet {
		return nil
	}
	if opts.JSON {
		return writeJSONAll(w, r)
	}
	return writeHumanAll(w, r)
}

type jsonAllResult struct {
	Condition   string       `json:"condition"`
	Success     bool         `json:"success"`
	ElapsedMS   int64        `json:"elapsed_ms"`
	TimeoutMS   int64        `json:"timeout_ms"`
	Interrupted bool         `json:"interrupted,omitempty"`
	Children    []jsonResult `json:"children"`
}

func writeJSONAll(w io.Writer, r AllResult) error {
	children := make([]jsonResult, len(r.Children))
	for i, c := range r.Children {
		children[i] = jsonResult{
			Condition:   c.Kind,
			Target:      c.Target,
			Success:     c.Success,
			ElapsedMS:   c.Elapsed.Milliseconds(),
			TimeoutMS:   c.Timeout.Milliseconds(),
			LastState:   c.LastState,
			DetailLabel: c.DetailLabel,
			DetailValue: c.DetailValue,
			Attempts:    c.Attempts,
		}
	}
	out := jsonAllResult{
		Condition:   "all",
		Success:     r.Success,
		ElapsedMS:   r.Elapsed.Milliseconds(),
		TimeoutMS:   r.Timeout.Milliseconds(),
		Interrupted: r.Interrupted,
		Children:    children,
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func writeHumanAll(w io.Writer, r AllResult) error {
	if r.Success {
		fmt.Fprintf(w, "✓ all conditions met (waited %s)\n", FormatDuration(r.Elapsed))
		return nil
	}

	switch {
	case r.Interrupted:
		fmt.Fprintf(w, "✗ Interrupted\n\n")
	case r.Timeout == 0:
		fmt.Fprintf(w, "✗ Condition not met (single check, no timeout set)\n\n")
	default:
		fmt.Fprintf(w, "✗ Timed out after %s\n\n", FormatDuration(r.Timeout))
	}

	var failed []poller.Result
	for _, c := range r.Children {
		if !c.Success {
			failed = append(failed, c)
		}
	}

	fmt.Fprintf(w, "  %d of %d conditions failed:\n\n", len(failed), len(r.Children))

	for i, c := range failed {
		if i > 0 {
			fmt.Fprintln(w)
		}
		fmt.Fprintf(w, "  %s\n", c.Describe)
		fmt.Fprintf(w, "    Last state: %s\n", c.LastState)
		if c.DetailValue != "" {
			label := c.DetailLabel
			if label == "" {
				label = "Detail"
			}
			fmt.Fprintf(w, "    %s: %s\n", label, c.DetailValue)
		}
	}
	return nil
}
