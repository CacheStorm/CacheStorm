package server

import (
	"math"
	"os"
	"runtime/debug"
	"sync/atomic"
)

// ManageGoMemoryLimit lets the server derive the Go runtime's soft memory
// limit from maxmemory. The binary turns it on; embedders and tests keep the
// runtime's own settings. A GOMEMLIMIT in the environment always wins.
var ManageGoMemoryLimit atomic.Bool

// applyGoMemoryLimit makes the garbage collector work harder as the heap
// nears maxmemory instead of letting it grow to twice the live data, which
// is GOGC's default and what made RSS several times used_memory. The limit
// leaves room for buffers and the runtime above the dataset.
func applyGoMemoryLimit(maxMemory int64) {
	if !ManageGoMemoryLimit.Load() || os.Getenv("GOMEMLIMIT") != "" {
		return
	}
	if maxMemory <= 0 {
		debug.SetMemoryLimit(math.MaxInt64)
		return
	}
	debug.SetMemoryLimit(maxMemory + maxMemory/4 + 64<<20)
}
