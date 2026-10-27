//go:build !unix

package command

import "runtime"

// processCPU is not measured on this platform.
func processCPU() (sys, user float64) { return 0, 0 }

func infoCPU(w *infoWriter) {
	w.section("CPU")
	w.field("used_cpu_sys", "0.000000")
	w.field("used_cpu_user", "0.000000")
	w.int("goroutines", int64(runtime.NumGoroutine()))
	w.end()
}
