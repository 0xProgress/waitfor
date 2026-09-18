//go:build !linux && !darwin

package process

import "fmt"

func listProcesses() ([]ProcInfo, error) {
	return nil, fmt.Errorf("process: unsupported platform")
}
