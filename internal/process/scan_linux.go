//go:build linux

package process

import (
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

func listProcesses() ([]ProcInfo, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}

	userCache := map[uint32]string{}
	var out []ProcInfo

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}

		commBytes, err := os.ReadFile(filepath.Join("/proc", e.Name(), "comm"))
		if err != nil {
			continue
		}
		name := strings.TrimRight(string(commBytes), "\n")
		if name == "" {
			continue
		}

		info := ProcInfo{PID: pid, Name: name}

		if fi, err := os.Stat(filepath.Join("/proc", e.Name())); err == nil {
			if st, ok := fi.Sys().(*syscall.Stat_t); ok {
				uid := st.Uid
				if cached, ok := userCache[uid]; ok {
					info.User = cached
				} else {
					uname := ""
					if u, err := user.LookupId(strconv.FormatUint(uint64(uid), 10)); err == nil {
						uname = u.Username
					}
					userCache[uid] = uname
					info.User = uname
				}
			}
		}

		out = append(out, info)
	}
	return out, nil
}
