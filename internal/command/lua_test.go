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
