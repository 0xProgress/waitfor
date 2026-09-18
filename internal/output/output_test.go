package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/0xProgress/waitfor/internal/poller"
)

func sampleSuccess() poller.Result {
	return poller.Result{
		Kind:           "port",
		Target:         "5432",
		Describe:       "TCP localhost:5432",
		SuccessMessage: "Port 5432 is open",
		Success:        true,
		Elapsed:        3200 * time.Millisecond,
		Timeout:        30 * time.Second,
		LastState:      "connected",
		Attempts:       7,
	}
}

func sampleTimeout() poller.Result {
	return poller.Result{
		Kind:      "port",
		Target:    "5432",
		Describe:  "TCP localhost:5432",
		Timeout:   30 * time.Second,
		LastState: "connection refused",
		Attempts:  60,
	}
}

func TestRenderSuccessHuman(t *testing.T) {
	var buf bytes.Buffer
	if err := Render(&buf, sampleSuccess(), Options{}); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if !strings.Contains(got, "✓ Port 5432 is open (waited 3.2s)") {
		t.Errorf("unexpected success line:\n%s", got)
	}
}

func TestRenderTimeoutHumanIncludesTip(t *testing.T) {
	var buf bytes.Buffer
	if err := Render(&buf, sampleTimeout(), Options{}); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	want := []string{
		"✗ Timed out after 30s",
		"Condition:  TCP localhost:5432",
		"Last state: connection refused",
		"Tip:",
	}
	for _, w := range want {
		if !strings.Contains(got, w) {
			t.Errorf("missing %q in:\n%s", w, got)
		}
	}
}

func TestRenderQuietProducesNothing(t *testing.T) {
	var buf bytes.Buffer
	if err := Render(&buf, sampleSuccess(), Options{Quiet: true}); err != nil {
		t.Fatal(err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected empty output, got %q", buf.String())
	}
}

func TestRenderJSONShape(t *testing.T) {
	var buf bytes.Buffer
	if err := Render(&buf, sampleTimeout(), Options{JSON: true}); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	want := map[string]any{
		"condition":  "port",
		"target":     "5432",
		"success":    false,
		"elapsed_ms": float64(0),
		"timeout_ms": float64(30000),
		"last_state": "connection refused",
		"attempts":   float64(60),
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %v (%T), want %v (%T)", k, got[k], got[k], v, v)
		}
	}
}

func TestRenderJSONSuccess(t *testing.T) {
	var buf bytes.Buffer
	if err := Render(&buf, sampleSuccess(), Options{JSON: true}); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["success"] != true {
		t.Errorf("success = %v", got["success"])
	}
	if got["elapsed_ms"] != float64(3200) {
		t.Errorf("elapsed_ms = %v, want 3200", got["elapsed_ms"])
	}
}

func TestRenderInterrupted(t *testing.T) {
	r := sampleTimeout()
	r.Interrupted = true
	var buf bytes.Buffer
	if err := Render(&buf, r, Options{}); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if !strings.Contains(got, "✗ Interrupted") {
		t.Errorf("missing interrupted banner:\n%s", got)
	}
}

func TestFormatDuration(t *testing.T) {
	cases := []struct {
		in   time.Duration
		want string
	}{
		{0, "0ms"},
		{250 * time.Millisecond, "250ms"},
		{1500 * time.Millisecond, "1.5s"},
		{3200 * time.Millisecond, "3.2s"},
		{30 * time.Second, "30s"},
		{60 * time.Second, "60s"},
	}
	for _, c := range cases {
		if got := FormatDuration(c.in); got != c.want {
			t.Errorf("FormatDuration(%s) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestRenderTimeoutHumanIncludesBody(t *testing.T) {
	r := sampleTimeout()
	r.DetailLabel = "Body (last)"
	r.DetailValue = `{"status":"starting","ready":false}`
	var buf bytes.Buffer
	if err := Render(&buf, r, Options{}); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if !strings.Contains(got, `Body (last): {"status":"starting","ready":false}`) {
		t.Errorf("missing body line:\n%s", got)
	}
}

func TestRenderTimeoutHumanGenericLabel(t *testing.T) {
	r := sampleTimeout()
	r.DetailLabel = "Stderr"
	r.DetailValue = "boom"
	var buf bytes.Buffer
	if err := Render(&buf, r, Options{}); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	// 12-column alignment: "  Stderr:     boom"
	if !strings.Contains(got, "Stderr:     boom") {
		t.Errorf("expected 12-col-aligned Stderr line:\n%s", got)
	}
}

func TestRenderJSONIncludesDetail(t *testing.T) {
	r := sampleTimeout()
	r.DetailLabel = "Stderr"
	r.DetailValue = "no such container"
	var buf bytes.Buffer
	if err := Render(&buf, r, Options{JSON: true}); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	if got["detail_label"] != "Stderr" {
		t.Errorf("detail_label = %v", got["detail_label"])
	}
	if got["detail_value"] != "no such container" {
		t.Errorf("detail_value = %v", got["detail_value"])
	}
}

func TestRenderJSONOmitsEmptyDetail(t *testing.T) {
	var buf bytes.Buffer
	if err := Render(&buf, sampleTimeout(), Options{JSON: true}); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if _, present := got["detail_label"]; present {
		t.Errorf("detail_label should be omitted when empty")
	}
	if _, present := got["detail_value"]; present {
		t.Errorf("detail_value should be omitted when empty")
	}
}

func TestRenderTimeoutZeroMessage(t *testing.T) {
	r := sampleTimeout()
	r.Timeout = 0
	var buf bytes.Buffer
	if err := Render(&buf, r, Options{}); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if !strings.Contains(got, "Condition not met (single check, no timeout set)") {
		t.Errorf("expected single-check message:\n%s", got)
	}
	if strings.Contains(got, "Timed out after 0") {
		t.Errorf("should not print 'Timed out after 0':\n%s", got)
	}
}
