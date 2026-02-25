package command

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestIntegrationCommandsRateLimitLowCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterIntegrationCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"RATELIMIT.CHECK no args", "RATELIMIT.CHECK", nil},
		{"RATELIMIT.CHECK missing args", "RATELIMIT.CHECK", [][]byte{[]byte("key1")}},
		{"RATELIMIT.CHECK not found", "RATELIMIT.CHECK", [][]byte{[]byte("notfound"), []byte("10"), []byte("60")}},
		{"RATELIMIT.CREATE no args", "RATELIMIT.CREATE", nil},
		{"RATELIMIT.CREATE limit", "RATELIMIT.CREATE", [][]byte{[]byte("limit1"), []byte("100"), []byte("60")}},
		{"RATELIMIT.RESET no args", "RATELIMIT.RESET", nil},
		{"RATELIMIT.RESET not found", "RATELIMIT.RESET", [][]byte{[]byte("notfound")}},
		{"RATELIMIT.DELETE no args", "RATELIMIT.DELETE", nil},
		{"RATELIMIT.DELETE not found", "RATELIMIT.DELETE", [][]byte{[]byte("notfound")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestIntegrationCommandsCacheLockLowCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterIntegrationCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"CACHE.LOCK no args", "CACHE.LOCK", nil},
		{"CACHE.LOCK key", "CACHE.LOCK", [][]byte{[]byte("key1"), []byte("token1"), []byte("30")}},
		{"CACHE.UNLOCK no args", "CACHE.UNLOCK", nil},
		{"CACHE.UNLOCK not found", "CACHE.UNLOCK", [][]byte{[]byte("notfound"), []byte("token1")}},
		{"CACHE.LOCKED no args", "CACHE.LOCKED", nil},
		{"CACHE.LOCKED not found", "CACHE.LOCKED", [][]byte{[]byte("notfound")}},
		{"CACHE.REFRESH no args", "CACHE.REFRESH", nil},
		{"CACHE.REFRESH not found", "CACHE.REFRESH", [][]byte{[]byte("notfound")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestIntegrationCommandsObjectLowCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterIntegrationCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"OBJECT.KEYS no args", "OBJECT.KEYS", nil},
		{"OBJECT.KEYS not found", "OBJECT.KEYS", [][]byte{[]byte("notfound")}},
		{"OBJECT.VALUES no args", "OBJECT.VALUES", nil},
		{"OBJECT.VALUES not found", "OBJECT.VALUES", [][]byte{[]byte("notfound")}},
		{"OBJECT.ENTRIES no args", "OBJECT.ENTRIES", nil},
		{"OBJECT.ENTRIES not found", "OBJECT.ENTRIES", [][]byte{[]byte("notfound")}},
		{"OBJECT.FROMENTRIES no args", "OBJECT.FROMENTRIES", nil},
		{"OBJECT.FROMENTRIES entries", "OBJECT.FROMENTRIES", [][]byte{[]byte("key1"), []byte("value1")}},
		{"OBJECT.MERGE no args", "OBJECT.MERGE", nil},
		{"OBJECT.MERGE missing args", "OBJECT.MERGE", [][]byte{[]byte("obj1")}},
		{"OBJECT.PICK no args", "OBJECT.PICK", nil},
		{"OBJECT.PICK not found", "OBJECT.PICK", [][]byte{[]byte("notfound"), []byte("key1")}},
		{"OBJECT.OMIT no args", "OBJECT.OMIT", nil},
		{"OBJECT.OMIT not found", "OBJECT.OMIT", [][]byte{[]byte("notfound"), []byte("key1")}},
		{"OBJECT.HAS no args", "OBJECT.HAS", nil},
		{"OBJECT.HAS not found", "OBJECT.HAS", [][]byte{[]byte("notfound"), []byte("key1")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestIntegrationCommandsArrayLowCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterIntegrationCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"ARRAY.PUSH no args", "ARRAY.PUSH", nil},
		{"ARRAY.PUSH not found", "ARRAY.PUSH", [][]byte{[]byte("notfound"), []byte("value")}},
		{"ARRAY.POP no args", "ARRAY.POP", nil},
		{"ARRAY.POP not found", "ARRAY.POP", [][]byte{[]byte("notfound")}},
		{"ARRAY.SHIFT no args", "ARRAY.SHIFT", nil},
		{"ARRAY.SHIFT not found", "ARRAY.SHIFT", [][]byte{[]byte("notfound")}},
		{"ARRAY.UNSHIFT no args", "ARRAY.UNSHIFT", nil},
		{"ARRAY.UNSHIFT not found", "ARRAY.UNSHIFT", [][]byte{[]byte("notfound"), []byte("value")}},
		{"ARRAY.SLICE no args", "ARRAY.SLICE", nil},
		{"ARRAY.SLICE not found", "ARRAY.SLICE", [][]byte{[]byte("notfound"), []byte("0"), []byte("10")}},
		{"ARRAY.SPLICE no args", "ARRAY.SPLICE", nil},
		{"ARRAY.SPLICE not found", "ARRAY.SPLICE", [][]byte{[]byte("notfound"), []byte("0"), []byte("1")}},
		{"ARRAY.REVERSE no args", "ARRAY.REVERSE", nil},
		{"ARRAY.REVERSE not found", "ARRAY.REVERSE", [][]byte{[]byte("notfound")}},
		{"ARRAY.SORT no args", "ARRAY.SORT", nil},
		{"ARRAY.SORT not found", "ARRAY.SORT", [][]byte{[]byte("notfound")}},
		{"ARRAY.MERGE no args", "ARRAY.MERGE", nil},
		{"ARRAY.MERGE missing args", "ARRAY.MERGE", [][]byte{[]byte("arr1")}},
		{"ARRAY.INTERSECT no args", "ARRAY.INTERSECT", nil},
		{"ARRAY.INTERSECT missing args", "ARRAY.INTERSECT", [][]byte{[]byte("arr1")}},
		{"ARRAY.DIFF no args", "ARRAY.DIFF", nil},
		{"ARRAY.DIFF missing args", "ARRAY.DIFF", [][]byte{[]byte("arr1")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestIntegrationCommandsCircuitBreakerLowCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterIntegrationCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"CIRCUITBREAKER.STATE no args", "CIRCUITBREAKER.STATE", nil},
		{"CIRCUITBREAKER.STATE not found", "CIRCUITBREAKER.STATE", [][]byte{[]byte("notfound")}},
		{"CIRCUITBREAKER.TRIP no args", "CIRCUITBREAKER.TRIP", nil},
		{"CIRCUITBREAKER.TRIP not found", "CIRCUITBREAKER.TRIP", [][]byte{[]byte("notfound")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestIntegrationCommandsMathLowCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterIntegrationCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"MATH.ADD no args", "MATH.ADD", nil},
		{"MATH.ADD values", "MATH.ADD", [][]byte{[]byte("1"), []byte("2")}},
		{"MATH.SUB no args", "MATH.SUB", nil},
		{"MATH.SUB values", "MATH.SUB", [][]byte{[]byte("5"), []byte("3")}},
		{"MATH.MUL no args", "MATH.MUL", nil},
		{"MATH.MUL values", "MATH.MUL", [][]byte{[]byte("2"), []byte("3")}},
		{"MATH.DIV no args", "MATH.DIV", nil},
		{"MATH.DIV by zero", "MATH.DIV", [][]byte{[]byte("10"), []byte("0")}},
		{"MATH.DIV values", "MATH.DIV", [][]byte{[]byte("10"), []byte("2")}},
		{"MATH.MOD no args", "MATH.MOD", nil},
		{"MATH.MOD values", "MATH.MOD", [][]byte{[]byte("10"), []byte("3")}},
		{"MATH.POW no args", "MATH.POW", nil},
		{"MATH.POW values", "MATH.POW", [][]byte{[]byte("2"), []byte("3")}},
		{"MATH.SQRT no args", "MATH.SQRT", nil},
		{"MATH.SQRT value", "MATH.SQRT", [][]byte{[]byte("16")}},
		{"MATH.ABS no args", "MATH.ABS", nil},
		{"MATH.ABS value", "MATH.ABS", [][]byte{[]byte("-5")}},
		{"MATH.MIN no args", "MATH.MIN", nil},
		{"MATH.MIN values", "MATH.MIN", [][]byte{[]byte("3"), []byte("1"), []byte("2")}},
		{"MATH.MAX no args", "MATH.MAX", nil},
		{"MATH.MAX values", "MATH.MAX", [][]byte{[]byte("3"), []byte("1"), []byte("2")}},
		{"MATH.FLOOR no args", "MATH.FLOOR", nil},
		{"MATH.FLOOR value", "MATH.FLOOR", [][]byte{[]byte("3.7")}},
		{"MATH.CEIL no args", "MATH.CEIL", nil},
		{"MATH.CEIL value", "MATH.CEIL", [][]byte{[]byte("3.2")}},
		{"MATH.ROUND no args", "MATH.ROUND", nil},
		{"MATH.ROUND value", "MATH.ROUND", [][]byte{[]byte("3.5")}},
		{"MATH.SUM no args", "MATH.SUM", nil},
		{"MATH.SUM values", "MATH.SUM", [][]byte{[]byte("1"), []byte("2"), []byte("3")}},
		{"MATH.AVG no args", "MATH.AVG", nil},
		{"MATH.AVG values", "MATH.AVG", [][]byte{[]byte("1"), []byte("2"), []byte("3")}},
		{"MATH.MEDIAN no args", "MATH.MEDIAN", nil},
		{"MATH.MEDIAN values", "MATH.MEDIAN", [][]byte{[]byte("1"), []byte("2"), []byte("3")}},
		{"MATH.STDDEV no args", "MATH.STDDEV", nil},
		{"MATH.STDDEV values", "MATH.STDDEV", [][]byte{[]byte("1"), []byte("2"), []byte("3")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestIntegrationCommandsGeoLowCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterIntegrationCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"GEO.ENCODE no args", "GEO.ENCODE", nil},
		{"GEO.ENCODE coordinates", "GEO.ENCODE", [][]byte{[]byte("40.7128"), []byte("-74.0060")}},
		{"GEO.DECODE no args", "GEO.DECODE", nil},
		{"GEO.DECODE hash", "GEO.DECODE", [][]byte{[]byte("dr5r9x8"), []byte("7")}},
		{"GEO.DISTANCE no args", "GEO.DISTANCE", nil},
		{"GEO.DISTANCE not found", "GEO.DISTANCE", [][]byte{[]byte("notfound1"), []byte("notfound2")}},
		{"GEO.BOUNDINGBOX no args", "GEO.BOUNDINGBOX", nil},
		{"GEO.BOUNDINGBOX coordinates", "GEO.BOUNDINGBOX", [][]byte{[]byte("40.7128"), []byte("-74.0060"), []byte("1000")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestIntegrationCommandsCaptchaLowCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterIntegrationCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"CAPTCHA.GENERATE no args", "CAPTCHA.GENERATE", nil},
		{"CAPTCHA.GENERATE create", "CAPTCHA.GENERATE", [][]byte{[]byte("captcha1")}},
		{"CAPTCHA.VERIFY no args", "CAPTCHA.VERIFY", nil},
		{"CAPTCHA.VERIFY not found", "CAPTCHA.VERIFY", [][]byte{[]byte("notfound"), []byte("answer")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}
