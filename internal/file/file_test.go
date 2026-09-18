package file

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestNewFileConditionRejectsEmptyPath(t *testing.T) {
	if _, err := NewFileCondition("", FileOptions{}); err == nil {
		t.Fatal("expected error for empty path")
	}
}

func TestNewFileConditionNonEmptyImpliesMinSize1(t *testing.T) {
	c, _ := NewFileCondition("/x", FileOptions{NonEmpty: true})
	if c.minSize != 1 {
		t.Errorf("minSize = %d, want 1", c.minSize)
	}
}

func TestNewFileConditionNonEmptyRespectsLargerMinSize(t *testing.T) {
	c, _ := NewFileCondition("/x", FileOptions{NonEmpty: true, MinSize: 10})
	if c.minSize != 10 {
		t.Errorf("minSize = %d, want 10", c.minSize)
	}
}

func TestFileConditionKindAndDescription(t *testing.T) {
	c, _ := NewFileCondition("./tmp/ready", FileOptions{})
	if c.Kind() != "file" {
		t.Errorf("Kind = %q", c.Kind())
	}
	if c.Describe() != "file exists ./tmp/ready" {
		t.Errorf("Describe = %q", c.Describe())
	}
	if c.SuccessMessage() != "./tmp/ready exists" {
		t.Errorf("SuccessMessage = %q", c.SuccessMessage())
	}
}

func TestFileConditionExists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ready")
	writeFile(t, path, []byte("hello"))

	c, _ := NewFileCondition(path, FileOptions{})
	state, ok, err := c.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatalf("expected success, state = %q", state)
	}
}

func TestFileConditionNotFound(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "missing")

	c, _ := NewFileCondition(path, FileOptions{})
	state, ok, _ := c.Check(context.Background())
	if ok {
		t.Fatal("expected failure")
	}
	if state != "not found" {
		t.Errorf("state = %q, want not found", state)
	}
}

func TestFileConditionParentMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nope", "ready")

	c, _ := NewFileCondition(path, FileOptions{})
	state, ok, _ := c.Check(context.Background())
	if ok {
		t.Fatal("expected failure")
	}
	if !strings.HasPrefix(state, "parent directory missing:") {
		t.Errorf("state = %q, want parent directory missing", state)
	}
}

func TestFileConditionEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ready")
	writeFile(t, path, nil)

	c, _ := NewFileCondition(path, FileOptions{NonEmpty: true})
	state, ok, _ := c.Check(context.Background())
	if ok {
		t.Fatal("expected failure")
	}
	if state != "empty" {
		t.Errorf("state = %q, want empty", state)
	}
}

func TestFileConditionBelowMinSize(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data")
	writeFile(t, path, []byte("abc")) // 3 bytes

	c, _ := NewFileCondition(path, FileOptions{MinSize: 10})
	state, ok, _ := c.Check(context.Background())
	if ok {
		t.Fatal("expected failure")
	}
	if !strings.Contains(state, "size 3") {
		t.Errorf("state = %q, want size 3 < 10", state)
	}
}

func TestFileConditionGrowsPastMinSize(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "growing")
	writeFile(t, path, nil)

	c, _ := NewFileCondition(path, FileOptions{NonEmpty: true})

	if _, ok, _ := c.Check(context.Background()); ok {
		t.Fatal("empty file should fail --nonempty")
	}

	writeFile(t, path, []byte("data"))
	if _, ok, _ := c.Check(context.Background()); !ok {
		t.Fatal("non-empty file should pass --nonempty")
	}
}

func TestFileConditionDirectoryAlwaysSucceeds(t *testing.T) {
	dir := t.TempDir()
	c, _ := NewFileCondition(dir, FileOptions{MinSize: 9999})

	state, ok, _ := c.Check(context.Background())
	if !ok {
		t.Fatalf("directory should succeed regardless of min-size, state = %q", state)
	}
	if state != "directory exists" {
		t.Errorf("state = %q", state)
	}
}

func TestFileConditionPermissionDenied(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root; permission bits are ignored")
	}
	dir := t.TempDir()
	locked := filepath.Join(dir, "locked")
	if err := os.Mkdir(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(locked, 0o755)

	path := filepath.Join(locked, "ready")
	c, _ := NewFileCondition(path, FileOptions{})
	state, ok, err := c.Check(context.Background())
	if err != nil {
		t.Fatalf("unexpected fatal err: %v", err)
	}
	if ok {
		t.Fatal("expected failure")
	}
	if !strings.Contains(state, "permission denied") {
		t.Errorf("state = %q, want permission denied", state)
	}
}

func TestFileConditionPollsUntilFileAppears(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sentinel")

	c, _ := NewFileCondition(path, FileOptions{})

	done := make(chan struct{})
	go func() {
		time.Sleep(80 * time.Millisecond)
		writeFile(t, path, []byte("here"))
		close(done)
	}()

	// Simulate a few polls; the file should appear by the third or so.
	for range 20 {
		if _, ok, _ := c.Check(context.Background()); ok {
			<-done
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("never observed the file")
}
