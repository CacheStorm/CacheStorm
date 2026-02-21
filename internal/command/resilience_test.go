package command

import (
	"bytes"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func newTestContextX(cmd string, args [][]byte, s *store.Store) *Context {
	buf := &bytes.Buffer{}
	w := resp.NewWriter(buf)
	return &Context{
		Command: cmd,
		Args:    args,
		Store:   s,
		Writer:  w,
	}
}

func TestAllCommandsRegistered(t *testing.T) {
	router := NewRouter()
	s := store.NewStore()

	RegisterStringCommands(router)
	RegisterHashCommands(router)
	RegisterListCommands(router)
	RegisterSetCommands(router)
	RegisterSortedSetCommands(router)
	RegisterKeyCommands(router)
	RegisterServerCommands(router)
	RegisterBitmapCommands(router)
	RegisterHyperLogLogCommands(router)
	RegisterGeoCommands(router)
	RegisterStreamCommands(router)
	RegisterJSONCommands(router)
	RegisterTSCommands(router)
	RegisterSearchCommands(router)
	RegisterProbabilisticCommands(router)
	RegisterGraphCommands(router)
	RegisterDigestCommands(router)
	RegisterUtilityCommands(router)
	RegisterMonitoringCommands(router)
	RegisterCacheWarmingCommands(router)
	RegisterStatsCommands(router)
	RegisterSchedulerCommands(router)
	RegisterEventCommands(router)
	RegisterUtilityExtCommands(router)
	RegisterTemplateCommands(router)
	RegisterWorkflowCommands(router)
	RegisterDataStructuresCommands(router)
	RegisterEncodingCommands(router)
	RegisterActorCommands(router)
	RegisterMVCCCommands(router)
	RegisterIntegrationCommands(router)
	RegisterExtendedCommands(router)
	RegisterMoreCommands(router)
	RegisterExtraCommands(router)
	RegisterAdvancedCommands2(router)
	RegisterResilienceCommands(router)

	count := len(router.Commands())

	t.Logf("Total commands registered: %d", count)

	if count < 1400 {
		t.Errorf("Expected at least 1400 commands, got %d", count)
	}

	_ = s
}

func TestCircuitBreakerCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterResilienceCommands(router)

	t.Run("CIRCUITX.CREATE", func(t *testing.T) {
		ctx := newTestContextX("CIRCUITX.CREATE", [][]byte{[]byte("test"), []byte("5"), []byte("1000")}, s)
		handler, ok := router.Get("CIRCUITX.CREATE")
		if !ok {
			t.Fatal("CIRCUITX.CREATE not found")
		}
		if err := handler.Handler(ctx); err != nil {
			t.Errorf("CIRCUITX.CREATE error: %v", err)
		}
	})

	t.Run("CIRCUITX.STATUS", func(t *testing.T) {
		ctx := newTestContextX("CIRCUITX.STATUS", [][]byte{[]byte("test")}, s)
		handler, ok := router.Get("CIRCUITX.STATUS")
		if !ok {
			t.Fatal("CIRCUITX.STATUS not found")
		}
		if err := handler.Handler(ctx); err != nil {
			t.Errorf("CIRCUITX.STATUS error: %v", err)
		}
	})
}

func TestRateLimiterCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterResilienceCommands(router)

	t.Run("RATELIMITER.CREATE", func(t *testing.T) {
		ctx := newTestContextX("RATELIMITER.CREATE", [][]byte{[]byte("api"), []byte("100"), []byte("60000")}, s)
		handler, ok := router.Get("RATELIMITER.CREATE")
		if !ok {
			t.Fatal("RATELIMITER.CREATE not found")
		}
		if err := handler.Handler(ctx); err != nil {
			t.Errorf("RATELIMITER.CREATE error: %v", err)
		}
	})

	t.Run("RATELIMITER.TRY", func(t *testing.T) {
		ctx := newTestContextX("RATELIMITER.TRY", [][]byte{[]byte("api")}, s)
		handler, ok := router.Get("RATELIMITER.TRY")
		if !ok {
			t.Fatal("RATELIMITER.TRY not found")
		}
		if err := handler.Handler(ctx); err != nil {
			t.Errorf("RATELIMITER.TRY error: %v", err)
		}
	})
}

func TestPromiseCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterResilienceCommands(router)

	t.Run("PROMISE.CREATE", func(t *testing.T) {
		ctx := newTestContextX("PROMISE.CREATE", [][]byte{}, s)
		handler, ok := router.Get("PROMISE.CREATE")
		if !ok {
			t.Fatal("PROMISE.CREATE not found")
		}
		if err := handler.Handler(ctx); err != nil {
			t.Errorf("PROMISE.CREATE error: %v", err)
		}
	})
}

func TestFutureCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterResilienceCommands(router)

	t.Run("FUTURE.CREATE", func(t *testing.T) {
		ctx := newTestContextX("FUTURE.CREATE", [][]byte{}, s)
		handler, ok := router.Get("FUTURE.CREATE")
		if !ok {
			t.Fatal("FUTURE.CREATE not found")
		}
		if err := handler.Handler(ctx); err != nil {
			t.Errorf("FUTURE.CREATE error: %v", err)
		}
	})
}

func TestAggregatorCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterResilienceCommands(router)

	t.Run("AGGREGATOR.CREATE", func(t *testing.T) {
		ctx := newTestContextX("AGGREGATOR.CREATE", [][]byte{[]byte("stats"), []byte("sum")}, s)
		handler, ok := router.Get("AGGREGATOR.CREATE")
		if !ok {
			t.Fatal("AGGREGATOR.CREATE not found")
		}
		if err := handler.Handler(ctx); err != nil {
			t.Errorf("AGGREGATOR.CREATE error: %v", err)
		}
	})

	t.Run("AGGREGATOR.ADD", func(t *testing.T) {
		ctx := newTestContextX("AGGREGATOR.ADD", [][]byte{[]byte("stats"), []byte("10")}, s)
		handler, ok := router.Get("AGGREGATOR.ADD")
		if !ok {
			t.Fatal("AGGREGATOR.ADD not found")
		}
		if err := handler.Handler(ctx); err != nil {
			t.Errorf("AGGREGATOR.ADD error: %v", err)
		}
	})

	t.Run("AGGREGATOR.GET", func(t *testing.T) {
		ctx := newTestContextX("AGGREGATOR.GET", [][]byte{[]byte("stats")}, s)
		handler, ok := router.Get("AGGREGATOR.GET")
		if !ok {
			t.Fatal("AGGREGATOR.GET not found")
		}
		if err := handler.Handler(ctx); err != nil {
			t.Errorf("AGGREGATOR.GET error: %v", err)
		}
	})
}

func TestWindowCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterResilienceCommands(router)

	t.Run("WINDOWX.CREATE", func(t *testing.T) {
		ctx := newTestContextX("WINDOWX.CREATE", [][]byte{[]byte("metrics"), []byte("100")}, s)
		handler, ok := router.Get("WINDOWX.CREATE")
		if !ok {
			t.Fatal("WINDOWX.CREATE not found")
		}
		if err := handler.Handler(ctx); err != nil {
			t.Errorf("WINDOWX.CREATE error: %v", err)
		}
	})
}

func TestAsyncCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterResilienceCommands(router)

	t.Run("ASYNC.SUBMIT", func(t *testing.T) {
		ctx := newTestContextX("ASYNC.SUBMIT", [][]byte{[]byte("task")}, s)
		handler, ok := router.Get("ASYNC.SUBMIT")
		if !ok {
			t.Fatal("ASYNC.SUBMIT not found")
		}
		if err := handler.Handler(ctx); err != nil {
			t.Errorf("ASYNC.SUBMIT error: %v", err)
		}
	})
}

func TestLockCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterResilienceCommands(router)

	t.Run("LOCKX.ACQUIRE", func(t *testing.T) {
		ctx := newTestContextX("LOCKX.ACQUIRE", [][]byte{[]byte("resource1"), []byte("client1"), []byte("5000")}, s)
		handler, ok := router.Get("LOCKX.ACQUIRE")
		if !ok {
			t.Fatal("LOCKX.ACQUIRE not found")
		}
		if err := handler.Handler(ctx); err != nil {
			t.Errorf("LOCKX.ACQUIRE error: %v", err)
		}
	})
}

func TestSemaphoreCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterResilienceCommands(router)

	t.Run("SEMAPHOREX.CREATE", func(t *testing.T) {
		ctx := newTestContextX("SEMAPHOREX.CREATE", [][]byte{[]byte("pool"), []byte("10")}, s)
		handler, ok := router.Get("SEMAPHOREX.CREATE")
		if !ok {
			t.Fatal("SEMAPHOREX.CREATE not found")
		}
		if err := handler.Handler(ctx); err != nil {
			t.Errorf("SEMAPHOREX.CREATE error: %v", err)
		}
	})
}

func TestEventSourcingCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterResilienceCommands(router)

	t.Run("EVENTSOURCING.APPEND", func(t *testing.T) {
		ctx := newTestContextX("EVENTSOURCING.APPEND", [][]byte{[]byte("stream1"), []byte("created"), []byte("data")}, s)
		handler, ok := router.Get("EVENTSOURCING.APPEND")
		if !ok {
			t.Fatal("EVENTSOURCING.APPEND not found")
		}
		if err := handler.Handler(ctx); err != nil {
			t.Errorf("EVENTSOURCING.APPEND error: %v", err)
		}
	})

	t.Run("EVENTSOURCING.REPLAY", func(t *testing.T) {
		ctx := newTestContextX("EVENTSOURCING.REPLAY", [][]byte{[]byte("stream1")}, s)
		handler, ok := router.Get("EVENTSOURCING.REPLAY")
		if !ok {
			t.Fatal("EVENTSOURCING.REPLAY not found")
		}
		if err := handler.Handler(ctx); err != nil {
			t.Errorf("EVENTSOURCING.REPLAY error: %v", err)
		}
	})
}

func TestBackpressureCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterResilienceCommands(router)

	t.Run("BACKPRESSURE.CREATE", func(t *testing.T) {
		ctx := newTestContextX("BACKPRESSURE.CREATE", [][]byte{[]byte("pipe"), []byte("1000"), []byte("100")}, s)
		handler, ok := router.Get("BACKPRESSURE.CREATE")
		if !ok {
			t.Fatal("BACKPRESSURE.CREATE not found")
		}
		if err := handler.Handler(ctx); err != nil {
			t.Errorf("BACKPRESSURE.CREATE error: %v", err)
		}
	})
}

func TestBatchCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterResilienceCommands(router)

	t.Run("BATCHX.CREATE", func(t *testing.T) {
		ctx := newTestContextX("BATCHX.CREATE", [][]byte{[]byte("batch1")}, s)
		handler, ok := router.Get("BATCHX.CREATE")
		if !ok {
			t.Fatal("BATCHX.CREATE not found")
		}
		if err := handler.Handler(ctx); err != nil {
			t.Errorf("BATCHX.CREATE error: %v", err)
		}
	})
}

func TestPipelineCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterResilienceCommands(router)

	t.Run("PIPELINEX.START", func(t *testing.T) {
		ctx := newTestContextX("PIPELINEX.START", [][]byte{}, s)
		handler, ok := router.Get("PIPELINEX.START")
		if !ok {
			t.Fatal("PIPELINEX.START not found")
		}
		if err := handler.Handler(ctx); err != nil {
			t.Errorf("PIPELINEX.START error: %v", err)
		}
	})
}

func TestTransactionCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterResilienceCommands(router)

	t.Run("TRANSX.BEGIN", func(t *testing.T) {
		ctx := newTestContextX("TRANSX.BEGIN", [][]byte{}, s)
		handler, ok := router.Get("TRANSX.BEGIN")
		if !ok {
			t.Fatal("TRANSX.BEGIN not found")
		}
		if err := handler.Handler(ctx); err != nil {
			t.Errorf("TRANSX.BEGIN error: %v", err)
		}
	})
}

func TestConpoolCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterResilienceCommands(router)

	t.Run("CONPOOL.CREATE", func(t *testing.T) {
		ctx := newTestContextX("CONPOOL.CREATE", [][]byte{[]byte("dbpool"), []byte("10")}, s)
		handler, ok := router.Get("CONPOOL.CREATE")
		if !ok {
			t.Fatal("CONPOOL.CREATE not found")
		}
		if err := handler.Handler(ctx); err != nil {
			t.Errorf("CONPOOL.CREATE error: %v", err)
		}
	})
}

func TestBulkheadCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterResilienceCommands(router)

	t.Run("BULKHEAD.CREATE", func(t *testing.T) {
		ctx := newTestContextX("BULKHEAD.CREATE", [][]byte{[]byte("api"), []byte("5")}, s)
		handler, ok := router.Get("BULKHEAD.CREATE")
		if !ok {
			t.Fatal("BULKHEAD.CREATE not found")
		}
		if err := handler.Handler(ctx); err != nil {
			t.Errorf("BULKHEAD.CREATE error: %v", err)
		}
	})
}

func TestTimeoutCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterResilienceCommands(router)

	t.Run("TIMEOUT.CREATE", func(t *testing.T) {
		ctx := newTestContextX("TIMEOUT.CREATE", [][]byte{[]byte("request"), []byte("5000")}, s)
		handler, ok := router.Get("TIMEOUT.CREATE")
		if !ok {
			t.Fatal("TIMEOUT.CREATE not found")
		}
		if err := handler.Handler(ctx); err != nil {
			t.Errorf("TIMEOUT.CREATE error: %v", err)
		}
	})
}

func TestRetryCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterResilienceCommands(router)

	t.Run("RETRY.CREATE", func(t *testing.T) {
		ctx := newTestContextX("RETRY.CREATE", [][]byte{[]byte("operation"), []byte("3"), []byte("100")}, s)
		handler, ok := router.Get("RETRY.CREATE")
		if !ok {
			t.Fatal("RETRY.CREATE not found")
		}
		if err := handler.Handler(ctx); err != nil {
			t.Errorf("RETRY.CREATE error: %v", err)
		}
	})
}

func TestTelemetryCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterResilienceCommands(router)

	t.Run("TELEMETRY.RECORD", func(t *testing.T) {
		ctx := newTestContextX("TELEMETRY.RECORD", [][]byte{[]byte("cpu"), []byte("1000"), []byte("45.5")}, s)
		handler, ok := router.Get("TELEMETRY.RECORD")
		if !ok {
			t.Fatal("TELEMETRY.RECORD not found")
		}
		if err := handler.Handler(ctx); err != nil {
			t.Errorf("TELEMETRY.RECORD error: %v", err)
		}
	})
}

func TestObservableCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterResilienceCommands(router)

	t.Run("OBSERVABLE.CREATE", func(t *testing.T) {
		ctx := newTestContextX("OBSERVABLE.CREATE", [][]byte{}, s)
		handler, ok := router.Get("OBSERVABLE.CREATE")
		if !ok {
			t.Fatal("OBSERVABLE.CREATE not found")
		}
		if err := handler.Handler(ctx); err != nil {
			t.Errorf("OBSERVABLE.CREATE error: %v", err)
		}
	})
}

func TestStreamProcCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterResilienceCommands(router)

	t.Run("STREAMPROC.CREATE", func(t *testing.T) {
		ctx := newTestContextX("STREAMPROC.CREATE", [][]byte{[]byte("process")}, s)
		handler, ok := router.Get("STREAMPROC.CREATE")
		if !ok {
			t.Fatal("STREAMPROC.CREATE not found")
		}
		if err := handler.Handler(ctx); err != nil {
			t.Errorf("STREAMPROC.CREATE error: %v", err)
		}
	})
}

func TestThrottleCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterResilienceCommands(router)

	t.Run("THROTTLEX.CREATE", func(t *testing.T) {
		ctx := newTestContextX("THROTTLEX.CREATE", [][]byte{[]byte("api"), []byte("100")}, s)
		handler, ok := router.Get("THROTTLEX.CREATE")
		if !ok {
			t.Fatal("THROTTLEX.CREATE not found")
		}
		if err := handler.Handler(ctx); err != nil {
			t.Errorf("THROTTLEX.CREATE error: %v", err)
		}
	})
}

func TestDebounceCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterResilienceCommands(router)

	t.Run("DEBOUNCEX.CREATE", func(t *testing.T) {
		ctx := newTestContextX("DEBOUNCEX.CREATE", [][]byte{[]byte("search"), []byte("300")}, s)
		handler, ok := router.Get("DEBOUNCEX.CREATE")
		if !ok {
			t.Fatal("DEBOUNCEX.CREATE not found")
		}
		if err := handler.Handler(ctx); err != nil {
			t.Errorf("DEBOUNCEX.CREATE error: %v", err)
		}
	})
}

func TestCoalesceCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterResilienceCommands(router)

	t.Run("COALESCE.CREATE", func(t *testing.T) {
		ctx := newTestContextX("COALESCE.CREATE", [][]byte{[]byte("batch")}, s)
		handler, ok := router.Get("COALESCE.CREATE")
		if !ok {
			t.Fatal("COALESCE.CREATE not found")
		}
		if err := handler.Handler(ctx); err != nil {
			t.Errorf("COALESCE.CREATE error: %v", err)
		}
	})
}

func TestJoinCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterResilienceCommands(router)

	t.Run("JOINX.CREATE", func(t *testing.T) {
		ctx := newTestContextX("JOINX.CREATE", [][]byte{[]byte("join1")}, s)
		handler, ok := router.Get("JOINX.CREATE")
		if !ok {
			t.Fatal("JOINX.CREATE not found")
		}
		if err := handler.Handler(ctx); err != nil {
			t.Errorf("JOINX.CREATE error: %v", err)
		}
	})
}

func TestShuffleCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterResilienceCommands(router)

	t.Run("SHUFFLE.CREATE", func(t *testing.T) {
		ctx := newTestContextX("SHUFFLE.CREATE", [][]byte{[]byte("deck")}, s)
		handler, ok := router.Get("SHUFFLE.CREATE")
		if !ok {
			t.Fatal("SHUFFLE.CREATE not found")
		}
		if err := handler.Handler(ctx); err != nil {
			t.Errorf("SHUFFLE.CREATE error: %v", err)
		}
	})
}

func TestPartitionCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterResilienceCommands(router)

	t.Run("PARTITIONX.CREATE", func(t *testing.T) {
		ctx := newTestContextX("PARTITIONX.CREATE", [][]byte{[]byte("data"), []byte("10")}, s)
		handler, ok := router.Get("PARTITIONX.CREATE")
		if !ok {
			t.Fatal("PARTITIONX.CREATE not found")
		}
		if err := handler.Handler(ctx); err != nil {
			t.Errorf("PARTITIONX.CREATE error: %v", err)
		}
	})
}
