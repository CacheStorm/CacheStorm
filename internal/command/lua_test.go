package command

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestScriptEngine_Eval(t *testing.T) {
	s := store.NewStore()
	engine := NewScriptEngine(s)

	tests := []struct {
		name   string
		script string
		keys   []string
		args   []string
		want   interface{}
	}{
		{
			name:   "simple return",
			script: "return 42",
			keys:   []string{},
			args:   []string{},
			want:   float64(42),
		},
		{
			name:   "return string",
			script: "return 'hello'",
			keys:   []string{},
			args:   []string{},
			want:   "hello",
		},
		{
			name:   "return true",
			script: "return true",
			keys:   []string{},
			args:   []string{},
			want:   true,
		},
		{
			name:   "return false",
			script: "return false",
			keys:   []string{},
			args:   []string{},
			want:   false,
		},
		{
			name:   "return table",
			script: "return {1, 2, 3}",
			keys:   []string{},
			args:   []string{},
			want:   nil,
		},
		{
			name:   "access KEYS",
			script: "return redis.KEYS[1]",
			keys:   []string{"mykey"},
			args:   []string{},
			want:   "mykey",
		},
		{
			name:   "access ARGV",
			script: "return redis.ARGV[1]",
			keys:   []string{},
			args:   []string{"myarg"},
			want:   "myarg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := engine.Eval(tt.script, tt.keys, tt.args)
			if err != nil {
				t.Errorf("Eval() error = %v", err)
				return
			}
			if tt.name == "return table" {
				slice, ok := got.([]interface{})
				if !ok || len(slice) != 3 {
					t.Errorf("Eval() = %v, want []interface{} with 3 elements", got)
					return
				}
				if slice[0] != float64(1) || slice[1] != float64(2) || slice[2] != float64(3) {
					t.Errorf("Eval() = %v, want [1, 2, 3]", got)
				}
				return
			}
			if got != tt.want {
				t.Errorf("Eval() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestScriptEngine_RedisCall(t *testing.T) {
	s := store.NewStore()
	engine := NewScriptEngine(s)

	t.Run("SET and GET", func(t *testing.T) {
		script := `
			redis.call('SET', 'testkey', 'testvalue')
			return redis.call('GET', 'testkey')
		`
		got, err := engine.Eval(script, []string{}, []string{})
		if err != nil {
			t.Errorf("Eval() error = %v", err)
			return
		}
		if got != "testvalue" {
			t.Errorf("Eval() = %v, want testvalue", got)
		}
	})

	t.Run("INCR", func(t *testing.T) {
		script := `
			redis.call('SET', 'counter', '0')
			redis.call('INCR', 'counter')
			redis.call('INCR', 'counter')
			return redis.call('GET', 'counter')
		`
		got, err := engine.Eval(script, []string{}, []string{})
		if err != nil {
			t.Errorf("Eval() error = %v", err)
			return
		}
		if got != "2" {
			t.Errorf("Eval() = %v, want 2", got)
		}
	})

	t.Run("DEL", func(t *testing.T) {
		script := `
			redis.call('SET', 'todel', 'value')
			local result = redis.call('DEL', 'todel')
			return result
		`
		got, err := engine.Eval(script, []string{}, []string{})
		if err != nil {
			t.Errorf("Eval() error = %v", err)
			return
		}
		if got != float64(1) {
			t.Errorf("Eval() = %v, want 1", got)
		}
	})

	t.Run("EXISTS", func(t *testing.T) {
		script := `
			redis.call('SET', 'existskey', 'value')
			local exists = redis.call('EXISTS', 'existskey')
			local notexists = redis.call('EXISTS', 'nothere')
			return {exists, notexists}
		`
		got, err := engine.Eval(script, []string{}, []string{})
		if err != nil {
			t.Errorf("Eval() error = %v", err)
			return
		}
		expected := []interface{}{float64(1), float64(0)}
		if !compareSlices(got, expected) {
			t.Errorf("Eval() = %v, want %v", got, expected)
		}
	})

	t.Run("HSET and HGET", func(t *testing.T) {
		script := `
			redis.call('HSET', 'myhash', 'field1', 'value1')
			return redis.call('HGET', 'myhash', 'field1')
		`
		got, err := engine.Eval(script, []string{}, []string{})
		if err != nil {
			t.Errorf("Eval() error = %v", err)
			return
		}
		if got != "value1" {
			t.Errorf("Eval() = %v, want value1", got)
		}
	})

	t.Run("LPUSH and LPOP", func(t *testing.T) {
		script := `
			redis.call('LPUSH', 'mylist', 'a')
			redis.call('LPUSH', 'mylist', 'b')
			return redis.call('LPOP', 'mylist')
		`
		got, err := engine.Eval(script, []string{}, []string{})
		if err != nil {
			t.Errorf("Eval() error = %v", err)
			return
		}
		if got != "b" {
			t.Errorf("Eval() = %v, want b", got)
		}
	})

	t.Run("SADD and SISMEMBER", func(t *testing.T) {
		script := `
			redis.call('SADD', 'myset', 'member1')
			return redis.call('SISMEMBER', 'myset', 'member1')
		`
		got, err := engine.Eval(script, []string{}, []string{})
		if err != nil {
			t.Errorf("Eval() error = %v", err)
			return
		}
		if got != float64(1) {
			t.Errorf("Eval() = %v, want 1", got)
		}
	})

	t.Run("TYPE", func(t *testing.T) {
		script := `
			redis.call('SET', 'strkey', 'value')
			return redis.call('TYPE', 'strkey')
		`
		got, err := engine.Eval(script, []string{}, []string{})
		if err != nil {
			t.Errorf("Eval() error = %v", err)
			return
		}
		if got != "string" {
			t.Errorf("Eval() = %v, want string", got)
		}
	})

	t.Run("EXPIRE and TTL", func(t *testing.T) {
		script := `
			redis.call('SET', 'expirekey', 'value')
			redis.call('EXPIRE', 'expirekey', 100)
			return redis.call('TTL', 'expirekey')
		`
		got, err := engine.Eval(script, []string{}, []string{})
		if err != nil {
			t.Errorf("Eval() error = %v", err)
			return
		}
		ttl, ok := got.(float64)
		if !ok || ttl < 95 || ttl > 100 {
			t.Errorf("Eval() = %v, want ~100", got)
		}
	})

	t.Run("MSET and MGET", func(t *testing.T) {
		script := `
			redis.call('MSET', 'k1', 'v1', 'k2', 'v2')
			return redis.call('MGET', 'k1', 'k2')
		`
		got, err := engine.Eval(script, []string{}, []string{})
		if err != nil {
			t.Errorf("Eval() error = %v", err)
			return
		}
		slice, ok := got.([]interface{})
		if !ok || len(slice) != 2 {
			t.Errorf("Eval() = %v, want []interface{} with 2 elements", got)
			return
		}
		if slice[0] != "v1" || slice[1] != "v2" {
			t.Errorf("Eval() = %v, want [v1, v2]", got)
		}
	})

	t.Run("HEXISTS and HDEL", func(t *testing.T) {
		script := `
			redis.call('HSET', 'myhash', 'field1', 'value1')
			local exists = redis.call('HEXISTS', 'myhash', 'field1')
			redis.call('HDEL', 'myhash', 'field1')
			local after = redis.call('HEXISTS', 'myhash', 'field1')
			return {exists, after}
		`
		got, err := engine.Eval(script, []string{}, []string{})
		if err != nil {
			t.Errorf("Eval() error = %v", err)
			return
		}
		slice, ok := got.([]interface{})
		if !ok || len(slice) != 2 {
			t.Errorf("Eval() = %v, want []interface{} with 2 elements", got)
			return
		}
		if slice[0] != float64(1) || slice[1] != float64(0) {
			t.Errorf("Eval() = %v, want [1, 0]", got)
		}
	})

	t.Run("HLEN", func(t *testing.T) {
		script := `
			redis.call('HSET', 'hashlen', 'f1', 'v1')
			redis.call('HSET', 'hashlen', 'f2', 'v2')
			return redis.call('HLEN', 'hashlen')
		`
		got, err := engine.Eval(script, []string{}, []string{})
		if err != nil {
			t.Errorf("Eval() error = %v", err)
			return
		}
		if got != float64(2) {
			t.Errorf("Eval() = %v, want 2", got)
		}
	})

	t.Run("LLEN and LRANGE", func(t *testing.T) {
		script := `
			redis.call('RPUSH', 'mylist', 'a')
			redis.call('RPUSH', 'mylist', 'b')
			redis.call('RPUSH', 'mylist', 'c')
			local len = redis.call('LLEN', 'mylist')
			local range_result = redis.call('LRANGE', 'mylist', 0, -1)
			return {len, range_result}
		`
		got, err := engine.Eval(script, []string{}, []string{})
		if err != nil {
			t.Errorf("Eval() error = %v", err)
			return
		}
		slice, ok := got.([]interface{})
		if !ok || len(slice) != 2 {
			t.Errorf("Eval() = %v, want []interface{} with 2 elements", got)
			return
		}
		lenVal, ok := slice[0].(float64)
		if !ok || lenVal < 3 {
			t.Errorf("LLEN = %v, want >= 3", slice[0])
		}
	})

	t.Run("ZADD, ZSCORE, ZCARD", func(t *testing.T) {
		script := `
			redis.call('ZADD', 'myzset', 1.0, 'one')
			redis.call('ZADD', 'myzset', 2.0, 'two')
			local score = redis.call('ZSCORE', 'myzset', 'one')
			local card = redis.call('ZCARD', 'myzset')
			return {score, card}
		`
		got, err := engine.Eval(script, []string{}, []string{})
		if err != nil {
			t.Errorf("Eval() error = %v", err)
			return
		}
		slice, ok := got.([]interface{})
		if !ok || len(slice) != 2 {
			t.Errorf("Eval() = %v, want []interface{} with 2 elements", got)
			return
		}
		if slice[0] != float64(1.0) {
			t.Errorf("ZSCORE = %v, want 1.0", slice[0])
		}
		if slice[1] != float64(2) {
			t.Errorf("ZCARD = %v, want 2", slice[1])
		}
	})

	t.Run("ZREM", func(t *testing.T) {
		script := `
			redis.call('ZADD', 'zremtest', 1.0, 'member1')
			redis.call('ZADD', 'zremtest', 2.0, 'member2')
			local removed = redis.call('ZREM', 'zremtest', 'member1')
			local card = redis.call('ZCARD', 'zremtest')
			return {removed, card}
		`
		got, err := engine.Eval(script, []string{}, []string{})
		if err != nil {
			t.Errorf("Eval() error = %v", err)
			return
		}
		slice, ok := got.([]interface{})
		if !ok || len(slice) != 2 {
			t.Errorf("Eval() = %v, want []interface{} with 2 elements", got)
			return
		}
		if slice[0] != float64(1) {
			t.Errorf("ZREM = %v, want 1", slice[0])
		}
		if slice[1] != float64(1) {
			t.Errorf("ZCARD after ZREM = %v, want 1", slice[1])
		}
	})

	t.Run("DBSIZE", func(t *testing.T) {
		script := `
			redis.call('SET', 'key1', 'value1')
			redis.call('SET', 'key2', 'value2')
			return redis.call('DBSIZE')
		`
		got, err := engine.Eval(script, []string{}, []string{})
		if err != nil {
			t.Errorf("Eval() error = %v", err)
			return
		}
		size, ok := got.(float64)
		if !ok || size < 2 {
			t.Errorf("Eval() = %v, want >= 2", got)
		}
	})

	t.Run("FLUSHDB", func(t *testing.T) {
		script := `
			redis.call('SET', 'flushkey', 'value')
			redis.call('FLUSHDB')
			return redis.call('DBSIZE')
		`
		got, err := engine.Eval(script, []string{}, []string{})
		if err != nil {
			t.Errorf("Eval() error = %v", err)
			return
		}
		if got != float64(0) {
			t.Errorf("Eval() = %v, want 0", got)
		}
	})
}

func TestScriptEngine_ScriptSHA(t *testing.T) {
	script := "return 42"
	sha := ScriptSHA(script)

	if len(sha) != 40 {
		t.Errorf("ScriptSHA() length = %d, want 40", len(sha))
	}
}

func TestScriptEngine_ScriptLoad(t *testing.T) {
	s := store.NewStore()
	engine := NewScriptEngine(s)

	script := "return redis.call('GET', KEYS[1])"
	sha := engine.ScriptLoad(script)

	if !engine.ScriptExists(sha) {
		t.Error("ScriptExists() = false, want true")
	}

	engine.ScriptFlush()
	if engine.ScriptExists(sha) {
		t.Error("ScriptExists() after flush = true, want false")
	}
}

func TestScriptEngine_EvalSHA(t *testing.T) {
	s := store.NewStore()
	engine := NewScriptEngine(s)

	script := "return redis.ARGV[1]"
	sha := engine.ScriptLoad(script)

	got, err := engine.EvalSHA(sha, []string{}, []string{"hello"})
	if err != nil {
		t.Errorf("EvalSHA() error = %v", err)
		return
	}
	if got != "hello" {
		t.Errorf("EvalSHA() = %v, want hello", got)
	}

	_, err = engine.EvalSHA("nonexistent", []string{}, []string{})
	if err == nil {
		t.Error("EvalSHA() with nonexistent sha should return error")
	}
}

func compareSlices(a, b interface{}) bool {
	aSlice, ok1 := a.([]interface{})
	bSlice, ok2 := b.([]interface{})
	if !ok1 || !ok2 {
		return false
	}
	if len(aSlice) != len(bSlice) {
		return false
	}
	for i := range aSlice {
		if aSlice[i] != bSlice[i] {
			return false
		}
	}
	return true
}
