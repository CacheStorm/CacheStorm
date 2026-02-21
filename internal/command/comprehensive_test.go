package command

import (
	"bytes"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func newTestCtx(cmd string, args [][]byte, s *store.Store) *Context {
	buf := &bytes.Buffer{}
	w := resp.NewWriter(buf)
	return NewContext(cmd, args, s, w)
}

func TestAllCommandsExist(t *testing.T) {
	router := NewRouter()
	s := store.NewStore()

	RegisterStringCommands(router)
	RegisterHashCommands(router)
	RegisterListCommands(router)
	RegisterSetCommands(router)
	RegisterSortedSetCommands(router)
	RegisterBitmapCommands(router)
	RegisterHyperLogLogCommands(router)
	RegisterGeoCommands(router)
	RegisterStreamCommands(router)
	RegisterJSONCommands(router)
	RegisterTSCommands(router)
	RegisterSearchCommands(router)
	RegisterProbabilisticCommands(router)
	RegisterGraphCommands(router)
	RegisterPubSubCommands(router)
	RegisterServerCommands(router)
	RegisterClusterCommands(router)
	RegisterScriptCommands(router)
	RegisterTagCommands(router)
	RegisterNamespaceCommands(router)
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
	RegisterMLCommands(router)
	RegisterKeyCommands(router)

	count := len(router.Commands())
	t.Logf("Total registered commands: %d", count)

	if count < 1500 {
		t.Errorf("Expected at least 1500 commands, got %d", count)
	}

	_ = s
}

func TestStringCommandsComprehensive(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)

	tests := []struct {
		name  string
		cmd   string
		args  [][]byte
		check func(t *testing.T, ctx *Context)
	}{
		{"SET", "SET", [][]byte{[]byte("key"), []byte("value")}, nil},
		{"GET", "GET", [][]byte{[]byte("key")}, nil},
		{"INCR", "INCR", [][]byte{[]byte("counter")}, nil},
		{"DECR", "DECR", [][]byte{[]byte("counter")}, nil},
		{"INCRBY", "INCRBY", [][]byte{[]byte("counter"), []byte("5")}, nil},
		{"DECRBY", "DECRBY", [][]byte{[]byte("counter"), []byte("3")}, nil},
		{"APPEND", "APPEND", [][]byte{[]byte("key"), []byte("suffix")}, nil},
		{"STRLEN", "STRLEN", [][]byte{[]byte("key")}, nil},
		{"GETRANGE", "GETRANGE", [][]byte{[]byte("key"), []byte("0"), []byte("3")}, nil},
		{"SETRANGE", "SETRANGE", [][]byte{[]byte("key"), []byte("0"), []byte("new")}, nil},
		{"MSET", "MSET", [][]byte{[]byte("k1"), []byte("v1"), []byte("k2"), []byte("v2")}, nil},
		{"MGET", "MGET", [][]byte{[]byte("k1"), []byte("k2")}, nil},
		{"SETNX", "SETNX", [][]byte{[]byte("newkey"), []byte("value")}, nil},
		{"SETEX", "SETEX", [][]byte{[]byte("expkey"), []byte("60"), []byte("value")}, nil},
		{"INCRBYFLOAT", "INCRBYFLOAT", [][]byte{[]byte("floatkey"), []byte("2.5")}, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newTestCtx(tt.cmd, tt.args, s)
			handler, ok := router.Get(tt.cmd)
			if !ok {
				t.Fatalf("Command %s not found", tt.cmd)
			}
			if err := handler.Handler(ctx); err != nil {
				t.Errorf("Command %s failed: %v", tt.cmd, err)
			}
		})
	}
}

func TestHashCommandsComprehensive(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterHashCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"HSET", "HSET", [][]byte{[]byte("hash"), []byte("field"), []byte("value")}},
		{"HGET", "HGET", [][]byte{[]byte("hash"), []byte("field")}},
		{"HGETALL", "HGETALL", [][]byte{[]byte("hash")}},
		{"HDEL", "HDEL", [][]byte{[]byte("hash"), []byte("field")}},
		{"HEXISTS", "HEXISTS", [][]byte{[]byte("hash"), []byte("field")}},
		{"HINCRBY", "HINCRBY", [][]byte{[]byte("hash"), []byte("counter"), []byte("1")}},
		{"HKEYS", "HKEYS", [][]byte{[]byte("hash")}},
		{"HVALS", "HVALS", [][]byte{[]byte("hash")}},
		{"HLEN", "HLEN", [][]byte{[]byte("hash")}},
		{"HMSET", "HMSET", [][]byte{[]byte("hash"), []byte("f1"), []byte("v1"), []byte("f2"), []byte("v2")}},
		{"HMGET", "HMGET", [][]byte{[]byte("hash"), []byte("f1"), []byte("f2")}},
		{"HSETNX", "HSETNX", [][]byte{[]byte("hash"), []byte("newfield"), []byte("value")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newTestCtx(tt.cmd, tt.args, s)
			handler, ok := router.Get(tt.cmd)
			if !ok {
				t.Fatalf("Command %s not found", tt.cmd)
			}
			if err := handler.Handler(ctx); err != nil {
				t.Errorf("Command %s failed: %v", tt.cmd, err)
			}
		})
	}
}

func TestListCommandsComprehensive(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterListCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"LPUSH", "LPUSH", [][]byte{[]byte("list"), []byte("value")}},
		{"RPUSH", "RPUSH", [][]byte{[]byte("list"), []byte("value")}},
		{"LPOP", "LPOP", [][]byte{[]byte("list")}},
		{"RPOP", "RPOP", [][]byte{[]byte("list")}},
		{"LLEN", "LLEN", [][]byte{[]byte("list")}},
		{"LRANGE", "LRANGE", [][]byte{[]byte("list"), []byte("0"), []byte("-1")}},
		{"LINDEX", "LINDEX", [][]byte{[]byte("list"), []byte("0")}},
		{"LPOS", "LPOS", [][]byte{[]byte("list"), []byte("value")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newTestCtx(tt.cmd, tt.args, s)
			handler, ok := router.Get(tt.cmd)
			if !ok {
				t.Fatalf("Command %s not found", tt.cmd)
			}
			if err := handler.Handler(ctx); err != nil {
				t.Errorf("Command %s failed: %v", tt.cmd, err)
			}
		})
	}
}

func TestSetCommandsComprehensive(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterSetCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"SADD", "SADD", [][]byte{[]byte("set"), []byte("member")}},
		{"SREM", "SREM", [][]byte{[]byte("set"), []byte("member")}},
		{"SISMEMBER", "SISMEMBER", [][]byte{[]byte("set"), []byte("member")}},
		{"SMEMBERS", "SMEMBERS", [][]byte{[]byte("set")}},
		{"SCARD", "SCARD", [][]byte{[]byte("set")}},
		{"SPOP", "SPOP", [][]byte{[]byte("set")}},
		{"SRANDMEMBER", "SRANDMEMBER", [][]byte{[]byte("set")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newTestCtx(tt.cmd, tt.args, s)
			handler, ok := router.Get(tt.cmd)
			if !ok {
				t.Fatalf("Command %s not found", tt.cmd)
			}
			if err := handler.Handler(ctx); err != nil {
				t.Errorf("Command %s failed: %v", tt.cmd, err)
			}
		})
	}
}

func TestSortedSetCommandsComprehensive(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterSortedSetCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"ZADD", "ZADD", [][]byte{[]byte("zset"), []byte("1"), []byte("member")}},
		{"ZREM", "ZREM", [][]byte{[]byte("zset"), []byte("member")}},
		{"ZSCORE", "ZSCORE", [][]byte{[]byte("zset"), []byte("member")}},
		{"ZCARD", "ZCARD", [][]byte{[]byte("zset")}},
		{"ZRANK", "ZRANK", [][]byte{[]byte("zset"), []byte("member")}},
		{"ZREVRANK", "ZREVRANK", [][]byte{[]byte("zset"), []byte("member")}},
		{"ZRANGE", "ZRANGE", [][]byte{[]byte("zset"), []byte("0"), []byte("-1")}},
		{"ZREVRANGE", "ZREVRANGE", [][]byte{[]byte("zset"), []byte("0"), []byte("-1")}},
		{"ZCOUNT", "ZCOUNT", [][]byte{[]byte("zset"), []byte("0"), []byte("10")}},
		{"ZINCRBY", "ZINCRBY", [][]byte{[]byte("zset"), []byte("1"), []byte("member")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newTestCtx(tt.cmd, tt.args, s)
			handler, ok := router.Get(tt.cmd)
			if !ok {
				t.Fatalf("Command %s not found", tt.cmd)
			}
			if err := handler.Handler(ctx); err != nil {
				t.Errorf("Command %s failed: %v", tt.cmd, err)
			}
		})
	}
}

func TestBitmapCommandsComprehensive(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterBitmapCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"SETBIT", "SETBIT", [][]byte{[]byte("bitmap"), []byte("0"), []byte("1")}},
		{"GETBIT", "GETBIT", [][]byte{[]byte("bitmap"), []byte("0")}},
		{"BITCOUNT", "BITCOUNT", [][]byte{[]byte("bitmap")}},
		{"BITPOS", "BITPOS", [][]byte{[]byte("bitmap"), []byte("1")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newTestCtx(tt.cmd, tt.args, s)
			handler, ok := router.Get(tt.cmd)
			if !ok {
				t.Fatalf("Command %s not found", tt.cmd)
			}
			if err := handler.Handler(ctx); err != nil {
				t.Errorf("Command %s failed: %v", tt.cmd, err)
			}
		})
	}
}

func TestGeoCommandsComprehensive(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterGeoCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"GEOADD", "GEOADD", [][]byte{[]byte("geo"), []byte("13.361389"), []byte("38.115556"), []byte("Palermo")}},
		{"GEODIST", "GEODIST", [][]byte{[]byte("geo"), []byte("Palermo"), []byte("Catania")}},
		{"GEOHASH", "GEOHASH", [][]byte{[]byte("geo"), []byte("Palermo")}},
		{"GEOPOS", "GEOPOS", [][]byte{[]byte("geo"), []byte("Palermo")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newTestCtx(tt.cmd, tt.args, s)
			handler, ok := router.Get(tt.cmd)
			if !ok {
				t.Fatalf("Command %s not found", tt.cmd)
			}
			if err := handler.Handler(ctx); err != nil {
				t.Errorf("Command %s failed: %v", tt.cmd, err)
			}
		})
	}
}

func TestStreamCommandsComprehensive(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStreamCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"XADD", "XADD", [][]byte{[]byte("stream"), []byte("*"), []byte("field"), []byte("value")}},
		{"XLEN", "XLEN", [][]byte{[]byte("stream")}},
		{"XRANGE", "XRANGE", [][]byte{[]byte("stream"), []byte("-"), []byte("+")}},
		{"XREVRANGE", "XREVRANGE", [][]byte{[]byte("stream"), []byte("+"), []byte("-")}},
		{"XREAD", "XREAD", [][]byte{[]byte("STREAMS"), []byte("stream"), []byte("0")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newTestCtx(tt.cmd, tt.args, s)
			handler, ok := router.Get(tt.cmd)
			if !ok {
				t.Fatalf("Command %s not found", tt.cmd)
			}
			if err := handler.Handler(ctx); err != nil {
				t.Errorf("Command %s failed: %v", tt.cmd, err)
			}
		})
	}
}

func TestServerCommandsComprehensive(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterServerCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"PING", "PING", nil},
		{"ECHO", "ECHO", [][]byte{[]byte("hello")}},
		{"DBSIZE", "DBSIZE", nil},
		{"TIME", "TIME", nil},
		{"FLUSHALL", "FLUSHALL", nil},
		{"FLUSHDB", "FLUSHDB", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newTestCtx(tt.cmd, tt.args, s)
			handler, ok := router.Get(tt.cmd)
			if !ok {
				t.Fatalf("Command %s not found", tt.cmd)
			}
			if err := handler.Handler(ctx); err != nil {
				t.Errorf("Command %s failed: %v", tt.cmd, err)
			}
		})
	}
}

func TestKeyCommandsComprehensive(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterKeyCommands(router)
	RegisterStringCommands(router)

	s.Set("exists", &store.StringValue{Data: []byte("value")}, store.SetOptions{})
	s.Set("expireme", &store.StringValue{Data: []byte("value")}, store.SetOptions{})

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"EXISTS yes", "EXISTS", [][]byte{[]byte("exists")}},
		{"EXISTS no", "EXISTS", [][]byte{[]byte("notexists")}},
		{"DEL", "DEL", [][]byte{[]byte("exists")}},
		{"TYPE", "TYPE", [][]byte{[]byte("exists")}},
		{"KEYS", "KEYS", [][]byte{[]byte("*")}},
		{"RANDOMKEY", "RANDOMKEY", nil},
		{"TTL", "TTL", [][]byte{[]byte("exists")}},
		{"PTTL", "PTTL", [][]byte{[]byte("exists")}},
		{"EXPIRE", "EXPIRE", [][]byte{[]byte("expireme"), []byte("60")}},
		{"PERSIST", "PERSIST", [][]byte{[]byte("expireme")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newTestCtx(tt.cmd, tt.args, s)
			handler, ok := router.Get(tt.cmd)
			if !ok {
				t.Fatalf("Command %s not found", tt.cmd)
			}
			if err := handler.Handler(ctx); err != nil {
				t.Errorf("Command %s failed: %v", tt.cmd, err)
			}
		})
	}
}

func TestTransactionCommandsComprehensive(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterTransactionCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"MULTI", "MULTI", nil},
		{"EXEC", "EXEC", nil},
		{"DISCARD", "DISCARD", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newTestCtx(tt.cmd, tt.args, s)
			handler, ok := router.Get(tt.cmd)
			if !ok {
				t.Fatalf("Command %s not found", tt.cmd)
			}
			if err := handler.Handler(ctx); err != nil {
				t.Errorf("Command %s failed: %v", tt.cmd, err)
			}
		})
	}
}

func TestScriptCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterScriptCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"EVAL", "EVAL", [][]byte{[]byte("return 1"), []byte("0")}},
		{"SCRIPT EXISTS", "SCRIPT", [][]byte{[]byte("EXISTS"), []byte("abc123")}},
		{"SCRIPT LOAD", "SCRIPT", [][]byte{[]byte("LOAD"), []byte("return 1")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newTestCtx(tt.cmd, tt.args, s)
			handler, ok := router.Get(tt.cmd)
			if !ok {
				t.Fatalf("Command %s not found", tt.cmd)
			}
			if err := handler.Handler(ctx); err != nil {
				t.Errorf("Command %s failed: %v", tt.cmd, err)
			}
		})
	}
}

func TestPubSubCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterPubSubCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"PUBLISH", "PUBLISH", [][]byte{[]byte("channel"), []byte("message")}},
		{"PUBSUB CHANNELS", "PUBSUB", [][]byte{[]byte("CHANNELS")}},
		{"PUBSUB NUMSUB", "PUBSUB", [][]byte{[]byte("NUMSUB"), []byte("channel")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newTestCtx(tt.cmd, tt.args, s)
			handler, ok := router.Get(tt.cmd)
			if !ok {
				t.Fatalf("Command %s not found", tt.cmd)
			}
			if err := handler.Handler(ctx); err != nil {
				t.Errorf("Command %s failed: %v", tt.cmd, err)
			}
		})
	}
}

func TestHyperLogLogCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterHyperLogLogCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"PFADD", "PFADD", [][]byte{[]byte("hll"), []byte("a"), []byte("b"), []byte("c")}},
		{"PFCOUNT", "PFCOUNT", [][]byte{[]byte("hll")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newTestCtx(tt.cmd, tt.args, s)
			handler, ok := router.Get(tt.cmd)
			if !ok {
				t.Fatalf("Command %s not found", tt.cmd)
			}
			if err := handler.Handler(ctx); err != nil {
				t.Errorf("Command %s failed: %v", tt.cmd, err)
			}
		})
	}
}

func TestClusterCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterClusterCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"CLUSTER INFO", "CLUSTER", [][]byte{[]byte("INFO")}},
		{"CLUSTER NODES", "CLUSTER", [][]byte{[]byte("NODES")}},
		{"CLUSTER SLOTS", "CLUSTER", [][]byte{[]byte("SLOTS")}},
		{"ASKING", "ASKING", nil},
		{"READONLY", "READONLY", nil},
		{"READWRITE", "READWRITE", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newTestCtx(tt.cmd, tt.args, s)
			handler, ok := router.Get(tt.cmd)
			if !ok {
				t.Fatalf("Command %s not found", tt.cmd)
			}
			if err := handler.Handler(ctx); err != nil {
				t.Errorf("Command %s failed: %v", tt.cmd, err)
			}
		})
	}
}

func TestMLCommandsComprehensive(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMLCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"MODEL.CREATE", "MODEL.CREATE", [][]byte{[]byte("model1"), []byte("classifier")}},
		{"MODEL.STATUS", "MODEL.STATUS", [][]byte{[]byte("model1")}},
		{"FEATURE.SET", "FEATURE.SET", [][]byte{[]byte("entity1"), []byte("f1"), []byte("1.5")}},
		{"FEATURE.GET", "FEATURE.GET", [][]byte{[]byte("entity1"), []byte("f1")}},
		{"CLASSIFIER.CREATE", "CLASSIFIER.CREATE", [][]byte{[]byte("clf"), []byte("spam"), []byte("ham")}},
		{"REGRESSOR.CREATE", "REGRESSOR.CREATE", [][]byte{[]byte("reg")}},
		{"DATASET.CREATE", "DATASET.CREATE", [][]byte{[]byte("data1")}},
		{"EMBEDDING.CREATE", "EMBEDDING.CREATE", [][]byte{[]byte("emb1"), []byte("0.1"), []byte("0.2"), []byte("0.3")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newTestCtx(tt.cmd, tt.args, s)
			handler, ok := router.Get(tt.cmd)
			if !ok {
				t.Fatalf("Command %s not found", tt.cmd)
			}
			if err := handler.Handler(ctx); err != nil {
				t.Errorf("Command %s failed: %v", tt.cmd, err)
			}
		})
	}
}

func TestResilienceCommandsComprehensive(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterResilienceCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"CIRCUITX.CREATE", "CIRCUITX.CREATE", [][]byte{[]byte("cb1"), []byte("5"), []byte("1000")}},
		{"CIRCUITX.STATUS", "CIRCUITX.STATUS", [][]byte{[]byte("cb1")}},
		{"RATELIMITER.CREATE", "RATELIMITER.CREATE", [][]byte{[]byte("rl1"), []byte("10"), []byte("60000")}},
		{"RATELIMITER.TRY", "RATELIMITER.TRY", [][]byte{[]byte("rl1")}},
		{"BULKHEAD.CREATE", "BULKHEAD.CREATE", [][]byte{[]byte("bh1"), []byte("5")}},
		{"RETRY.CREATE", "RETRY.CREATE", [][]byte{[]byte("rt1"), []byte("3"), []byte("100")}},
		{"LOCKX.ACQUIRE", "LOCKX.ACQUIRE", [][]byte{[]byte("lock1"), []byte("owner1"), []byte("5000")}},
		{"SEMAPHOREX.CREATE", "SEMAPHOREX.CREATE", [][]byte{[]byte("sem1"), []byte("10")}},
		{"ASYNC.SUBMIT", "ASYNC.SUBMIT", [][]byte{[]byte("task1")}},
		{"PROMISE.CREATE", "PROMISE.CREATE", nil},
		{"FUTURE.CREATE", "FUTURE.CREATE", nil},
		{"AGGREGATOR.CREATE", "AGGREGATOR.CREATE", [][]byte{[]byte("agg1"), []byte("sum")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newTestCtx(tt.cmd, tt.args, s)
			handler, ok := router.Get(tt.cmd)
			if !ok {
				t.Fatalf("Command %s not found", tt.cmd)
			}
			if err := handler.Handler(ctx); err != nil {
				t.Errorf("Command %s failed: %v", tt.cmd, err)
			}
		})
	}
}
