package exitcode

import (
	"errors"
	"fmt"
	"testing"
)

func TestConstants(t *testing.T) {
	cases := []struct {
		name string
		got  int
		want int
	}{
		{"Success", Success, 0},
		{"Timeout", Timeout, 1},
		{"BadArgs", BadArgs, 2},
		{"Permission", Permission, 3},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s = %d, want %d", c.name, c.got, c.want)
		}
	}
}

func TestErrorEmptyHasFallbackMessage(t *testing.T) {
	e := &Error{Code: BadArgs}
	if e.Error() != "error" {
		t.Errorf("Error() = %q, want %q", e.Error(), "error")
	}
}

func TestErrorMsgOnly(t *testing.T) {
	e := &Error{Code: BadArgs, Msg: "boom"}
	if e.Error() != "boom" {
		t.Errorf("Error() = %q", e.Error())
	}
}

func TestErrorErrOnly(t *testing.T) {
	e := &Error{Code: BadArgs, Err: errors.New("inner")}
	if e.Error() != "inner" {
		t.Errorf("Error() = %q", e.Error())
	}
}

func TestErrorMsgAndErr(t *testing.T) {
	e := &Error{Code: BadArgs, Msg: "outer", Err: errors.New("inner")}
	if e.Error() != "outer: inner" {
		t.Errorf("Error() = %q", e.Error())
	}
}

func TestErrorUnwrap(t *testing.T) {
	inner := errors.New("inner")
	e := &Error{Code: BadArgs, Err: inner}
	if !errors.Is(e, inner) {
		t.Error("Unwrap should expose inner through errors.Is")
	}
}

func TestNew(t *testing.T) {
	e := New(Permission, "denied")
	if e.Code != Permission {
		t.Errorf("Code = %d, want %d", e.Code, Permission)
	}
	if e.Msg != "denied" {
		t.Errorf("Msg = %q", e.Msg)
	}
	if e.Err != nil || e.Silent {
		t.Errorf("unexpected fields: Err=%v Silent=%v", e.Err, e.Silent)
	}
}

func TestBadArgsf(t *testing.T) {
	e := BadArgsf("port %d invalid", 70000)
	if e.Code != BadArgs {
		t.Errorf("Code = %d, want %d", e.Code, BadArgs)
	}
	if e.Msg != "port 70000 invalid" {
		t.Errorf("Msg = %q", e.Msg)
	}
}

func TestPermissionf(t *testing.T) {
	e := Permissionf("cannot read %s", "/etc/shadow")
	if e.Code != Permission {
		t.Errorf("Code = %d, want %d", e.Code, Permission)
	}
	if e.Msg != "cannot read /etc/shadow" {
		t.Errorf("Msg = %q", e.Msg)
	}
}

func TestSilentTimeout(t *testing.T) {
	e := SilentTimeout()
	if e.Code != Timeout {
		t.Errorf("Code = %d, want %d", e.Code, Timeout)
	}
	if !e.Silent {
		t.Error("Silent should be true")
	}
	if e.Msg != "" || e.Err != nil {
		t.Errorf("SilentTimeout should have empty Msg and nil Err, got %+v", e)
	}
}

func TestCodeOfNil(t *testing.T) {
	if got := CodeOf(nil); got != Success {
		t.Errorf("CodeOf(nil) = %d, want %d", got, Success)
	}
}

func TestCodeOfCoded(t *testing.T) {
	e := New(Permission, "denied")
	if got := CodeOf(e); got != Permission {
		t.Errorf("CodeOf(coded) = %d, want %d", got, Permission)
	}
}

func TestCodeOfUncoded(t *testing.T) {
	if got := CodeOf(errors.New("plain")); got != BadArgs {
		t.Errorf("CodeOf(plain) = %d, want %d", got, BadArgs)
	}
}

func TestCodeOfWrappedCoded(t *testing.T) {
	inner := New(Permission, "denied")
	wrapped := fmt.Errorf("context: %w", inner)
	if got := CodeOf(wrapped); got != Permission {
		t.Errorf("CodeOf(wrapped) = %d, want %d", got, Permission)
	}
}

func TestFromErrNil(t *testing.T) {
	if got := FromErr(nil, BadArgs); got != nil {
		t.Errorf("FromErr(nil) = %+v, want nil", got)
	}
}

func TestFromErrAlreadyCodedReturnsSame(t *testing.T) {
	inner := New(Permission, "denied")
	got := FromErr(inner, BadArgs)
	if got != inner {
		t.Errorf("FromErr returned a different *Error: %+v", got)
	}
}

func TestFromErrUncoded(t *testing.T) {
	inner := errors.New("plain")
	got := FromErr(inner, Permission)
	if got == nil {
		t.Fatal("FromErr returned nil")
	}
	if got.Code != Permission {
		t.Errorf("Code = %d, want %d", got.Code, Permission)
	}
	if got.Err != inner {
		t.Errorf("Err = %v, want %v", got.Err, inner)
	}
}

func TestFromErrWrappedCodedUnwraps(t *testing.T) {
	inner := New(Permission, "denied")
	wrapped := fmt.Errorf("outer: %w", inner)
	got := FromErr(wrapped, BadArgs)
	// FromErr uses errors.AsType, so wrapping should still resolve to inner.
	if got != inner {
		t.Errorf("FromErr should unwrap to inner, got %+v", got)
	}
}
