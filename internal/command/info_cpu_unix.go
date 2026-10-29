//go:build unix

package command

import (
	"runtime"
	"strconv"
	"syscall"
)

// processCPU returns the system and user CPU seconds the process used.
func processCPU() (sys, user float64) {
	return rusageCPU(syscall.RUSAGE_SELF)
}

func rusageCPU(who int) (sys, user float64) {
	var ru syscall.Rusage
	if err := syscall.Getrusage(who, &ru); err == nil {
		sys = float64(ru.Stime.Sec) + float64(ru.Stime.Usec)/1e6
		user = float64(ru.Utime.Sec) + float64(ru.Utime.Usec)/1e6
	}
	return sys, user
}

func infoCPU(w *infoWriter) {
	sys, user := processCPU()
	csys, cuser := rusageCPU(syscall.RUSAGE_CHILDREN)
	f := func(v float64) string { return strconv.FormatFloat(v, 'f', 6, 64) }
	w.section("CPU")
	w.field("used_cpu_sys", f(sys))
	w.field("used_cpu_user", f(user))
	w.field("used_cpu_sys_children", f(csys))
	w.field("used_cpu_user_children", f(cuser))
	// The server has no main thread; the process figures stand in for it.
	w.field("used_cpu_sys_main_thread", f(sys))
	w.field("used_cpu_user_main_thread", f(user))
	w.int("goroutines", int64(runtime.NumGoroutine()))
	w.end()
}
