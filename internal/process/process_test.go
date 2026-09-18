package process

import (
	"strings"
	"testing"
)

func TestNewProcessConditionRejectsEmpty(t *testing.T) {
	if _, err := NewProcessCondition("", ProcessOptions{}); err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestNewProcessConditionRejectsNumericPID(t *testing.T) {
	_, err := NewProcessCondition("1234", ProcessOptions{})
	if err == nil {
		t.Fatal("expected error for numeric name")
	}
	if !strings.Contains(err.Error(), "PID") {
		t.Errorf("error should mention PID: %v", err)
	}
	if !strings.Contains(err.Error(), "kill -0 1234") {
		t.Errorf("error should suggest kill -0 1234: %v", err)
	}
}

func TestNewProcessConditionRejectsUnknownUser(t *testing.T) {
	_, err := NewProcessCondition("bash", ProcessOptions{User: "definitely-no-such-user-xyz"})
	if err == nil {
		t.Fatal("expected error for unknown user")
	}
}

func TestProcMatchesSubstring(t *testing.T) {
	p := ProcInfo{PID: 1, Name: "postgres", User: "alice"}
	if !procMatches(p, "post", false, "") {
		t.Error("substring match should succeed")
	}
	if procMatches(p, "mysql", false, "") {
		t.Error("non-match should fail")
	}
}

func TestProcMatchesExact(t *testing.T) {
	p := ProcInfo{Name: "postgres"}
	if !procMatches(p, "postgres", true, "") {
		t.Error("exact match should succeed")
	}
	if procMatches(p, "post", true, "") {
		t.Error("partial should fail with exact=true")
	}
}

func TestProcMatchesUser(t *testing.T) {
	p := ProcInfo{Name: "postgres", User: "alice"}
	if !procMatches(p, "postgres", false, "alice") {
		t.Error("matching user should succeed")
	}
	if procMatches(p, "postgres", false, "bob") {
		t.Error("non-matching user should fail")
	}
}

func TestProcMatchesUnknownOwnerFailsUserFilter(t *testing.T) {
	p := ProcInfo{Name: "postgres", User: ""}
	if procMatches(p, "postgres", false, "alice") {
		t.Error("unknown owner should not match a user filter")
	}
}

func TestProcessConditionMeta(t *testing.T) {
	c, _ := NewProcessCondition("postgres", ProcessOptions{})
	if c.Kind() != "process" {
		t.Errorf("Kind = %q", c.Kind())
	}
	if c.Describe() != `process "postgres"` {
		t.Errorf("Describe = %q", c.Describe())
	}
	if c.SuccessMessage() != `Process "postgres" is running` {
		t.Errorf("SuccessMessage = %q", c.SuccessMessage())
	}

	c2, _ := NewProcessCondition("postgres", ProcessOptions{User: "root"})
	if !strings.Contains(c2.Describe(), "user root") {
		t.Errorf("Describe missing user: %q", c2.Describe())
	}
}
