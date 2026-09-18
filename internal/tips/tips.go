// Package tips provides per-condition hint strings shown on failure.
package tips

import (
	"fmt"
	"strings"
)

// For returns a hint for the given condition kind, target, and last state.
// Returns "" if no hint applies.
func For(kind, target, lastState string) string {
	switch kind {
	case "port":
		return portTip(target, lastState)
	case "http":
		return httpTip(target, lastState)
	case "file":
		return fileTip(target, lastState)
	case "process":
		return processTip(target, lastState)
	case "command":
		return commandTip(target, lastState)
	}
	return ""
}

func portTip(port, state string) string {
	switch {
	case strings.Contains(state, "connection refused"):
		return fmt.Sprintf(
			"Port %s is reachable but nothing is listening. Is the service running? Try: ss -ltn | grep %s",
			port, port,
		)
	case strings.Contains(state, "no route to host"):
		return fmt.Sprintf(
			"Host for port %s is unreachable. Check network connectivity and firewall rules.",
			port,
		)
	case strings.Contains(state, "connection timeout"):
		return fmt.Sprintf(
			"Connection to port %s timed out. A firewall may be dropping packets.",
			port,
		)
	}
	return ""
}

func httpTip(url, state string) string {
	switch {
	case strings.Contains(state, "connection refused"):
		return fmt.Sprintf(
			"%s refused the connection. Is the service running? Try: curl -v %s",
			url, url,
		)
	case strings.Contains(state, "DNS"):
		return fmt.Sprintf(
			"DNS lookup for %s failed. Check the hostname and your resolver.",
			url,
		)
	case strings.Contains(state, "TLS"):
		return fmt.Sprintf(
			"TLS handshake with %s failed. Check the certificate, or retry with --insecure.",
			url,
		)
	case strings.Contains(state, "redirect"):
		return fmt.Sprintf(
			"%s is stuck in a redirect loop. Inspect the Location headers with: curl -vI %s",
			url, url,
		)
	case len(state) > 0 && state[0] >= '0' && state[0] <= '9':
		return fmt.Sprintf(
			"%s is responding but returned an unexpected status. Try: curl -v %s",
			url, url,
		)
	}
	return ""
}

func fileTip(path, state string) string {
	switch {
	case strings.Contains(state, "not found"):
		return fmt.Sprintf(
			"%s does not exist. Check that the process writing it has started.",
			path,
		)
	case strings.Contains(state, "empty"):
		return fmt.Sprintf(
			"%s exists but is empty. The writer may not have flushed yet.",
			path,
		)
	case strings.Contains(state, "permission"):
		return fmt.Sprintf(
			"Permission denied reading %s. Check ownership and mode with: ls -l %s",
			path, path,
		)
	case strings.Contains(state, "parent"):
		return fmt.Sprintf(
			"Parent directory of %s does not exist. Create it or point at a different path.",
			path,
		)
	}
	return ""
}

func processTip(name, _ string) string {
	return fmt.Sprintf(
		"No process matching %q is running. Check with: ps aux | grep %s",
		name, name,
	)
}

// commandTip returns no hint. The spec shows the failure block for a
// command ending after Stderr:, with no Tip line.
func commandTip(_, _ string) string {
	return ""
}
