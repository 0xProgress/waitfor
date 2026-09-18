// Package file implements waitfor conditions that watch the filesystem.
package file

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// FileCondition waits for a file or directory to exist. Optionally, a
// regular file must also meet a minimum size.
type FileCondition struct {
	path    string
	minSize int64
}

// FileOptions configures NewFileCondition.
type FileOptions struct {
	// MinSize requires a regular file to be at least this many bytes.
	// Ignored for directories.
	MinSize int64

	// NonEmpty is shorthand for MinSize >= 1. If both are set, the larger
	// value wins.
	NonEmpty bool
}

// NewFileCondition validates options and builds a condition.
func NewFileCondition(path string, opts FileOptions) (*FileCondition, error) {
	if path == "" {
		return nil, fmt.Errorf("file: empty path")
	}
	min := opts.MinSize
	if opts.NonEmpty && min < 1 {
		min = 1
	}
	if min < 0 {
		return nil, fmt.Errorf("file: --min-size must be >= 0")
	}
	return &FileCondition{path: path, minSize: min}, nil
}

func (c *FileCondition) Kind() string   { return "file" }
func (c *FileCondition) Target() string { return c.path }
func (c *FileCondition) Describe() string {
	return fmt.Sprintf("file exists %s", c.path)
}

func (c *FileCondition) SuccessMessage() string {
	return fmt.Sprintf("%s exists", c.path)
}

// Check stats the path and evaluates existence / size / permissions.
//
// Failure states produced:
//   - "not found"
//   - "parent directory missing: <dir>"
//   - "empty"                        (regular file below --min-size)
//   - "permission denied: <path>"
//
// This is a fatal error path only for unexpected stat failures that aren't
// not-exist or permission — everything else is a normal "not yet".
func (c *FileCondition) Check(_ context.Context) (string, bool, error) {
	info, err := os.Stat(c.path)
	if err != nil {
		switch {
		case os.IsNotExist(err):
			parent := filepath.Dir(c.path)
			if _, perr := os.Stat(parent); perr != nil {
				if os.IsNotExist(perr) {
					return "parent directory missing: " + parent, false, nil
				}
				if os.IsPermission(perr) {
					return "permission denied: " + parent, false, nil
				}
			}
			return "not found", false, nil
		case os.IsPermission(err):
			return "permission denied: " + c.path, false, nil
		default:
			return err.Error(), false, err
		}
	}

	// Directories satisfy existence regardless of size.
	if info.IsDir() {
		return "directory exists", true, nil
	}

	if c.minSize > 0 && info.Size() < c.minSize {
		if info.Size() == 0 {
			return "empty", false, nil
		}
		return fmt.Sprintf("size %d < %d", info.Size(), c.minSize), false, nil
	}

	return fmt.Sprintf("size %d", info.Size()), true, nil
}
