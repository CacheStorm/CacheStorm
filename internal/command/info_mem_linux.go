//go:build linux

package command

import (
	"bytes"
	"os"
	"strconv"
)

// processRSS returns the resident set size from /proc/self/statm.
func processRSS() (int64, bool) {
	data, err := os.ReadFile("/proc/self/statm")
	if err != nil {
		return 0, false
	}
	fields := bytes.Fields(data)
	if len(fields) < 2 {
		return 0, false
	}
	pages, err := strconv.ParseInt(string(fields[1]), 10, 64)
	if err != nil {
		return 0, false
	}
	return pages * int64(os.Getpagesize()), true
}

// totalSystemMemory returns MemTotal from /proc/meminfo, or 0.
func totalSystemMemory() int64 {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0
	}
	for _, line := range bytes.Split(data, []byte("\n")) {
		if rest, ok := bytes.CutPrefix(line, []byte("MemTotal:")); ok {
			fields := bytes.Fields(rest)
			if len(fields) == 0 {
				return 0
			}
			kb, err := strconv.ParseInt(string(fields[0]), 10, 64)
			if err != nil {
				return 0
			}
			return kb * 1024
		}
	}
	return 0
}
