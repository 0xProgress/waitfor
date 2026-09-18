//go:build darwin

package process

import (
	"bufio"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// listProcesses shells out to ps. comm= gives the full path to the
// executable; we take the basename so it matches Linux's comm semantics.
func listProcesses() ([]ProcInfo, error) {
	cmd := exec.Command("ps", "-axo", "pid=,user=,comm=")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ps: %w", err)
	}

	var procs []ProcInfo
	sc := bufio.NewScanner(strings.NewReader(string(out)))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		pid, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}
		user := fields[1]
		// comm may contain spaces on macOS; join the remainder.
		name := filepath.Base(strings.Join(fields[2:], " "))
		procs = append(procs, ProcInfo{PID: pid, Name: name, User: user})
	}
	return procs, sc.Err()
}
