package command

import (
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

// registryCleanup removes the entries this test created from a package-global
// registry, so the test leaves no state behind for the rest of the package's
// suite.
func registryCleanup[T any](t *testing.T, mu *sync.RWMutex, reg map[string]*T, names ...string) {
	t.Helper()
	t.Cleanup(func() {
		mu.Lock()
		for _, name := range names {
			delete(reg, name)
		}
		mu.Unlock()
	})
}

// TestSlidingStatsIsSafeWhileSlidingCheckWrites reads a sliding window's
// statistics while another client records entries in it.
//
// The read path took the map lock only to look the window up and then released
// it, reading the window's Entries slice with no lock at all while
// SLIDING.CHECK replaced that slice under the write lock.
func TestSlidingStatsIsSafeWhileSlidingCheckWrites(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMoreCommands(router)

	const iterations = 2000

	registryCleanup(t, &slidingWindowsMu, slidingWindows, "sw-race")

	if reply := registryRun(t, router, s, "SLIDING.CREATE", "sw-race", "100000", "60000"); reply != "+OK\r\n" {
		t.Fatalf("SLIDING.CREATE reply = %q, want +OK", reply)
	}

	var wg sync.WaitGroup
	start := make(chan struct{})

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < iterations; i++ {
			ctx, _ := bufCtx("SLIDING.CHECK", bytesArgs("sw-race"), s)
			if err := router.Execute(ctx); err != nil {
				t.Errorf("SLIDING.CHECK: unexpected error: %v", err)
				return
			}
		}
	}()

	for r := 0; r < 2; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for i := 0; i < iterations; i++ {
				ctx, buf := bufCtx("SLIDING.STATS", bytesArgs("sw-race"), s)
				if err := router.Execute(ctx); err != nil {
					t.Errorf("SLIDING.STATS: unexpected error: %v", err)
					return
				}
				if reply := buf.String(); !strings.HasPrefix(reply, "*") {
					t.Errorf("SLIDING.STATS: expected a RESP array reply, got %q", reply)
					return
				}
			}
		}()
	}

	close(start)
	wg.Wait()
}

// TestGaugeGetIsSafeWhileGaugeIncrWrites reads a gauge while another client
// increments it.
//
// GAUGE.GET released the map lock right after the lookup and then read the
// gauge's Value with no lock at all, while GAUGE.INCR updated that value
// under the write lock.
func TestGaugeGetIsSafeWhileGaugeIncrWrites(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMoreCommands(router)

	const iterations = 2000

	registryCleanup(t, &gaugesMu, gauges, "g-race")

	if reply := registryRun(t, router, s, "GAUGE.CREATE", "g-race", "0"); reply != "+OK\r\n" {
		t.Fatalf("GAUGE.CREATE reply = %q, want +OK", reply)
	}

	var wg sync.WaitGroup
	start := make(chan struct{})

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < iterations; i++ {
			ctx, _ := bufCtx("GAUGE.INCR", bytesArgs("g-race"), s)
			if err := router.Execute(ctx); err != nil {
				t.Errorf("GAUGE.INCR: unexpected error: %v", err)
				return
			}
		}
	}()

	for r := 0; r < 2; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for i := 0; i < iterations; i++ {
				ctx, buf := bufCtx("GAUGE.GET", bytesArgs("g-race"), s)
				if err := router.Execute(ctx); err != nil {
					t.Errorf("GAUGE.GET: unexpected error: %v", err)
					return
				}
				if reply := buf.String(); !strings.HasPrefix(reply, "$") {
					t.Errorf("GAUGE.GET: expected a bulk string reply, got %q", reply)
					return
				}
			}
		}()
	}

	close(start)
	wg.Wait()
}

// TestMeterGetIsSafeWhileMeterRecordWrites reads a meter's usage while
// another client records values in it.
//
// METER.GET released the map lock right after the lookup and then summed the
// meter's Records slice with no lock at all, while METER.RECORD appended to
// that slice under the write lock.
func TestMeterGetIsSafeWhileMeterRecordWrites(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMoreCommands(router)

	const iterations = 2000

	registryCleanup(t, &metersMu, meters, "m-race")

	if reply := registryRun(t, router, s, "METER.CREATE", "m-race", "ops"); reply != "+OK\r\n" {
		t.Fatalf("METER.CREATE reply = %q, want +OK", reply)
	}

	var wg sync.WaitGroup
	start := make(chan struct{})

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < iterations; i++ {
			ctx, _ := bufCtx("METER.RECORD", bytesArgs("m-race", "1"), s)
			if err := router.Execute(ctx); err != nil {
				t.Errorf("METER.RECORD: unexpected error: %v", err)
				return
			}
		}
	}()

	for r := 0; r < 2; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for i := 0; i < iterations; i++ {
				ctx, buf := bufCtx("METER.GET", bytesArgs("m-race"), s)
				if err := router.Execute(ctx); err != nil {
					t.Errorf("METER.GET: unexpected error: %v", err)
					return
				}
				if reply := buf.String(); !strings.HasPrefix(reply, "*") {
					t.Errorf("METER.GET: expected a RESP array reply, got %q", reply)
					return
				}
			}
		}()
	}

	close(start)
	wg.Wait()
}

// TestMeterBillingIsSafeWhileMeterRecordWrites reads a meter's billing usage
// while another client records values in it.
//
// METER.BILLING released the map lock right after the lookup and then summed
// the meter's Records slice with no lock at all, while METER.RECORD appended
// to that slice under the write lock.
func TestMeterBillingIsSafeWhileMeterRecordWrites(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMoreCommands(router)

	const iterations = 2000

	registryCleanup(t, &metersMu, meters, "mb-race")

	if reply := registryRun(t, router, s, "METER.CREATE", "mb-race", "ops"); reply != "+OK\r\n" {
		t.Fatalf("METER.CREATE reply = %q, want +OK", reply)
	}

	var wg sync.WaitGroup
	start := make(chan struct{})

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < iterations; i++ {
			ctx, _ := bufCtx("METER.RECORD", bytesArgs("mb-race", "1"), s)
			if err := router.Execute(ctx); err != nil {
				t.Errorf("METER.RECORD: unexpected error: %v", err)
				return
			}
		}
	}()

	for r := 0; r < 2; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for i := 0; i < iterations; i++ {
				ctx, buf := bufCtx("METER.BILLING", bytesArgs("mb-race"), s)
				if err := router.Execute(ctx); err != nil {
					t.Errorf("METER.BILLING: unexpected error: %v", err)
					return
				}
				if reply := buf.String(); !strings.HasPrefix(reply, "*") {
					t.Errorf("METER.BILLING: expected a RESP array reply, got %q", reply)
					return
				}
			}
		}()
	}

	close(start)
	wg.Wait()
}

// TestPipelineStatusIsSafeWhilePipelineMutatorsWrite reads a pipeline's status
// while another client adds stages to and executes it.
//
// PIPELINE.STATUS released the map lock right after the lookup and then read
// the pipeline's Status string and its Stages slice with no lock at all, while
// PIPELINE.ADDSTAGE and PIPELINE.EXECUTE wrote them under the write lock.
func TestPipelineStatusIsSafeWhilePipelineMutatorsWrite(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMoreCommands(router)

	const iterations = 2000

	registryCleanup(t, &pipelinesMu, pipelines, "p-race")

	if reply := registryRun(t, router, s, "PIPELINE.CREATE", "p-race"); reply != "+OK\r\n" {
		t.Fatalf("PIPELINE.CREATE reply = %q, want +OK", reply)
	}

	var wg sync.WaitGroup
	start := make(chan struct{})

	// Two writers: one grows the stage list, one rewrites the status string.
	for w := 0; w < 2; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for i := 0; i < iterations; i++ {
				cmd, args := "PIPELINE.ADDSTAGE", []string{"p-race", "stage"}
				if w == 1 {
					cmd, args = "PIPELINE.EXECUTE", []string{"p-race"}
				}
				ctx, _ := bufCtx(cmd, bytesArgs(args...), s)
				if err := router.Execute(ctx); err != nil {
					t.Errorf("%s: unexpected error: %v", cmd, err)
					return
				}
			}
		}()
	}

	for r := 0; r < 2; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for i := 0; i < iterations; i++ {
				ctx, buf := bufCtx("PIPELINE.STATUS", bytesArgs("p-race"), s)
				if err := router.Execute(ctx); err != nil {
					t.Errorf("PIPELINE.STATUS: unexpected error: %v", err)
					return
				}
				if reply := buf.String(); !strings.HasPrefix(reply, "*") {
					t.Errorf("PIPELINE.STATUS: expected a RESP array reply, got %q", reply)
					return
				}
			}
		}()
	}

	close(start)
	wg.Wait()
}

// TestTraceGetIsSafeWhileTraceMutatorsWrite reads a trace while another client
// adds spans to and closes it.
//
// TRACE.GET released the map lock right after the lookup and then read the
// trace's Status string and its Spans slice with no lock at all, while
// TRACE.SPAN and TRACE.END wrote them under the write lock.
func TestTraceGetIsSafeWhileTraceMutatorsWrite(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMoreCommands(router)

	const iterations = 2000

	registryCleanup(t, &tracesMu, traces, "t-race")

	if reply := registryRun(t, router, s, "TRACE.START", "t-race"); reply != "+OK\r\n" {
		t.Fatalf("TRACE.START reply = %q, want +OK", reply)
	}

	var wg sync.WaitGroup
	start := make(chan struct{})

	for w := 0; w < 2; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for i := 0; i < iterations; i++ {
				cmd, args := "TRACE.SPAN", []string{"t-race", "span", "s1"}
				if w == 1 {
					cmd, args = "TRACE.END", []string{"t-race"}
				}
				ctx, _ := bufCtx(cmd, bytesArgs(args...), s)
				if err := router.Execute(ctx); err != nil {
					t.Errorf("%s: unexpected error: %v", cmd, err)
					return
				}
			}
		}()
	}

	for r := 0; r < 2; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for i := 0; i < iterations; i++ {
				ctx, buf := bufCtx("TRACE.GET", bytesArgs("t-race"), s)
				if err := router.Execute(ctx); err != nil {
					t.Errorf("TRACE.GET: unexpected error: %v", err)
					return
				}
				if reply := buf.String(); !strings.HasPrefix(reply, "*") {
					t.Errorf("TRACE.GET: expected a RESP array reply, got %q", reply)
					return
				}
			}
		}()
	}

	close(start)
	wg.Wait()
}

// TestSketchQueryIsSafeWhileSketchUpdateWrites queries a sketch while another
// client updates it.
//
// SKETCH.QUERY released the map lock right after the lookup and then read the
// sketch's counter cells with no lock at all, while SKETCH.UPDATE incremented
// those cells under the write lock.
func TestSketchQueryIsSafeWhileSketchUpdateWrites(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMoreCommands(router)

	const iterations = 2000

	registryCleanup(t, &sketchesMu, sketches, "s-race")

	if reply := registryRun(t, router, s, "SKETCH.CREATE", "s-race", "16", "4"); reply != "+OK\r\n" {
		t.Fatalf("SKETCH.CREATE reply = %q, want +OK", reply)
	}

	var wg sync.WaitGroup
	start := make(chan struct{})

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < iterations; i++ {
			ctx, _ := bufCtx("SKETCH.UPDATE", bytesArgs("s-race", "item", "1"), s)
			if err := router.Execute(ctx); err != nil {
				t.Errorf("SKETCH.UPDATE: unexpected error: %v", err)
				return
			}
		}
	}()

	for r := 0; r < 2; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for i := 0; i < iterations; i++ {
				ctx, buf := bufCtx("SKETCH.QUERY", bytesArgs("s-race", "item"), s)
				if err := router.Execute(ctx); err != nil {
					t.Errorf("SKETCH.QUERY: unexpected error: %v", err)
					return
				}
				if reply := buf.String(); !strings.HasPrefix(reply, ":") {
					t.Errorf("SKETCH.QUERY: expected an integer reply, got %q", reply)
					return
				}
			}
		}()
	}

	close(start)
	wg.Wait()
}

// TestRingBufferSizeIsSafeWhileRingBufferWriteWrites reads a ring buffer's
// size while another client writes to it.
//
// RINGBUFFER.SIZE released the map lock right after the lookup and then read
// the buffer's Size and Count fields with no lock at all, while
// RINGBUFFER.WRITE mutated them under the write lock.
func TestRingBufferSizeIsSafeWhileRingBufferWriteWrites(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMoreCommands(router)

	const iterations = 2000

	registryCleanup(t, &ringBuffersMu, ringBuffers, "rb-size-race")

	if reply := registryRun(t, router, s, "RINGBUFFER.CREATE", "rb-size-race", "16"); reply != "+OK\r\n" {
		t.Fatalf("RINGBUFFER.CREATE reply = %q, want +OK", reply)
	}

	var wg sync.WaitGroup
	start := make(chan struct{})

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < iterations; i++ {
			ctx, _ := bufCtx("RINGBUFFER.WRITE", bytesArgs("rb-size-race", "payload"), s)
			if err := router.Execute(ctx); err != nil {
				t.Errorf("RINGBUFFER.WRITE: unexpected error: %v", err)
				return
			}
		}
	}()

	for r := 0; r < 2; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for i := 0; i < iterations; i++ {
				ctx, buf := bufCtx("RINGBUFFER.SIZE", bytesArgs("rb-size-race"), s)
				if err := router.Execute(ctx); err != nil {
					t.Errorf("RINGBUFFER.SIZE: unexpected error: %v", err)
					return
				}
				if reply := buf.String(); !strings.HasPrefix(reply, "*") {
					t.Errorf("RINGBUFFER.SIZE: expected a RESP array reply, got %q", reply)
					return
				}
			}
		}()
	}

	close(start)
	wg.Wait()
}

// TestPartitionGetIsSafeWhilePartitionAddWrites reads a partition while
// another client adds values to it.
//
// PARTITION.GET released the map lock right after the lookup and then walked
// the partition's slice with no lock at all, while PARTITION.ADD appended to
// that slice under the write lock.
func TestPartitionGetIsSafeWhilePartitionAddWrites(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMoreCommands(router)

	const iterations = 2000
	const partitionCount = 4

	registryCleanup(t, &partitionsMu, partitions, "pg-race")

	if reply := registryRun(t, router, s, "PARTITION.CREATE", "pg-race", strconv.Itoa(partitionCount)); reply != "+OK\r\n" {
		t.Fatalf("PARTITION.CREATE reply = %q, want +OK", reply)
	}

	// Read the partition the writer appends to, so the two touch the same
	// slice.
	key := "partition-key"
	idx := int(hashString(key) % partitionCount)

	var wg sync.WaitGroup
	start := make(chan struct{})

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < iterations; i++ {
			ctx, _ := bufCtx("PARTITION.ADD", bytesArgs("pg-race", key, "value"), s)
			if err := router.Execute(ctx); err != nil {
				t.Errorf("PARTITION.ADD: unexpected error: %v", err)
				return
			}
		}
	}()

	for r := 0; r < 2; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for i := 0; i < iterations; i++ {
				ctx, buf := bufCtx("PARTITION.GET", bytesArgs("pg-race", strconv.Itoa(idx)), s)
				if err := router.Execute(ctx); err != nil {
					t.Errorf("PARTITION.GET: unexpected error: %v", err)
					return
				}
				reply := buf.String()
				if !strings.HasPrefix(reply, "*") {
					t.Errorf("PARTITION.GET: expected a RESP array reply, got %q", reply)
					return
				}
				n, err := ringBufferArrayLenX(reply)
				if err != nil {
					t.Errorf("PARTITION.GET: %v", err)
					return
				}
				if n < 0 || n > iterations {
					t.Errorf("PARTITION.GET returned %d elements, want between 0 and %d: %q",
						n, iterations, reply)
					return
				}
			}
		}()
	}

	close(start)
	wg.Wait()
}

// TestPartitionListIsSafeWhilePartitionAddWrites lists a partition's counts
// while another client adds values to it.
//
// PARTITION.LIST released the map lock right after the lookup and then read
// every partition's slice length with no lock at all, while PARTITION.ADD
// appended to those slices under the write lock.
func TestPartitionListIsSafeWhilePartitionAddWrites(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMoreCommands(router)

	const iterations = 2000
	const partitionCount = 4

	registryCleanup(t, &partitionsMu, partitions, "pl-race")

	if reply := registryRun(t, router, s, "PARTITION.CREATE", "pl-race", strconv.Itoa(partitionCount)); reply != "+OK\r\n" {
		t.Fatalf("PARTITION.CREATE reply = %q, want +OK", reply)
	}

	var wg sync.WaitGroup
	start := make(chan struct{})

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < iterations; i++ {
			ctx, _ := bufCtx("PARTITION.ADD", bytesArgs("pl-race", "partition-key", "value"), s)
			if err := router.Execute(ctx); err != nil {
				t.Errorf("PARTITION.ADD: unexpected error: %v", err)
				return
			}
		}
	}()

	for r := 0; r < 2; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for i := 0; i < iterations; i++ {
				ctx, buf := bufCtx("PARTITION.LIST", bytesArgs("pl-race"), s)
				if err := router.Execute(ctx); err != nil {
					t.Errorf("PARTITION.LIST: unexpected error: %v", err)
					return
				}
				if reply := buf.String(); !strings.HasPrefix(reply, "*") {
					t.Errorf("PARTITION.LIST: expected a RESP array reply, got %q", reply)
					return
				}
			}
		}()
	}

	close(start)
	wg.Wait()
}

// TestRegistryReadersAreExactSequentially is the control case: with no
// concurrency the readers must report exactly what the writers recorded. It
// must pass both before and after the fixes, so a broken harness cannot
// masquerade as the defect.
func TestRegistryReadersAreExactSequentially(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMoreCommands(router)

	registryCleanup(t, &gaugesMu, gauges, "g-ctl")
	registryCleanup(t, &metersMu, meters, "m-ctl")
	registryCleanup(t, &sketchesMu, sketches, "s-ctl")
	registryCleanup(t, &partitionsMu, partitions, "p-ctl")

	if reply := registryRun(t, router, s, "GAUGE.CREATE", "g-ctl", "0"); reply != "+OK\r\n" {
		t.Fatalf("GAUGE.CREATE reply = %q, want +OK", reply)
	}
	for i := 0; i < 3; i++ {
		registryRun(t, router, s, "GAUGE.INCR", "g-ctl", "2")
	}
	if reply := registryRun(t, router, s, "GAUGE.GET", "g-ctl"); reply != "$8\r\n6.000000\r\n" {
		t.Fatalf("GAUGE.GET reply = %q, want 6.000000", reply)
	}

	if reply := registryRun(t, router, s, "METER.CREATE", "m-ctl", "ops"); reply != "+OK\r\n" {
		t.Fatalf("METER.CREATE reply = %q, want +OK", reply)
	}
	for _, v := range []string{"1", "2", "3"} {
		registryRun(t, router, s, "METER.RECORD", "m-ctl", v)
	}
	reply := registryRun(t, router, s, "METER.GET", "m-ctl")
	if !strings.Contains(reply, ":6\r\n") {
		t.Fatalf("METER.GET reply = %q, want the total 6", reply)
	}
	if !strings.Contains(reply, ":3\r\n") {
		t.Fatalf("METER.GET reply = %q, want the count 3", reply)
	}

	if reply := registryRun(t, router, s, "SKETCH.CREATE", "s-ctl", "16", "4"); reply != "+OK\r\n" {
		t.Fatalf("SKETCH.CREATE reply = %q, want +OK", reply)
	}
	registryRun(t, router, s, "SKETCH.UPDATE", "s-ctl", "item", "5")
	if reply := registryRun(t, router, s, "SKETCH.QUERY", "s-ctl", "item"); reply != ":5\r\n" {
		t.Fatalf("SKETCH.QUERY reply = %q, want :5", reply)
	}

	if reply := registryRun(t, router, s, "PARTITION.CREATE", "p-ctl", "2"); reply != "+OK\r\n" {
		t.Fatalf("PARTITION.CREATE reply = %q, want +OK", reply)
	}
	registryRun(t, router, s, "PARTITION.ADD", "p-ctl", "k", "v")
	idx := int(hashString("k") % 2)
	reply = registryRun(t, router, s, "PARTITION.GET", "p-ctl", strconv.Itoa(idx))
	if !strings.Contains(reply, "v") {
		t.Fatalf("PARTITION.GET reply = %q, want it to contain the value", reply)
	}
}
