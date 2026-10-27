//go:build unix

package command

import (
	"runtime"
	"strconv"
	"syscall"
)

func infoCPU(w *infoWriter) {
	var ru syscall.Rusage
	sys, user := 0.0, 0.0
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru); err == nil {
		sys = float64(ru.Stime.Sec) + float64(ru.Stime.Usec)/1e6
		user = float64(ru.Utime.Sec) + float64(ru.Utime.Usec)/1e6
	}
	w.section("CPU")
	w.field("used_cpu_sys", strconv.FormatFloat(sys, 'f', 6, 64))
	w.field("used_cpu_user", strconv.FormatFloat(user, 'f', 6, 64))
	w.int("goroutines", int64(runtime.NumGoroutine()))
	w.end()
}
