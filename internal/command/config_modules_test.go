package command

import (
	"strings"
	"testing"
)

// The replies are Redis 8.2's: module parameters with their ranges and
// errors, kept in step with FT.CONFIG, and the defaults they give new
// filters and series.
func TestModuleConfigParams(t *testing.T) {
	h := newLuaHarness(t)
	RegisterConfigCommands(h.router)
	RegisterSearchCommands(h.router)
	RegisterProbabilisticCommands(h.router)
	RegisterTSCommands(h.router)
	saved := globalConfig.params
	t.Cleanup(func() {
		h.do("CONFIG", "SET", "search-timeout", "500", "search-min-prefix", "2", "bf-error-rate", "0.01", "cf-bucket-size", "2",
			"ts-duplicate-policy", "block", "ts-compaction-policy", "", "ts-chunk-size-bytes", "4096")
		globalConfig.mu.Lock()
		globalConfig.params = saved
		globalConfig.mu.Unlock()
	})
	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"CONFIG", "GET", "search-max-aggregate-results"}, "2147483648"},
		{[]string{"CONFIG", "SET", "search-timeout", "0100"}, "argument 'search-timeout') - argument couldn't be parsed into an integer"},
		{[]string{"CONFIG", "SET", "search-min-prefix", "0"}, "argument must be between 1 and 9223372036854775807 inclusive"},
		{[]string{"CONFIG", "SET", "search-no-gc", "yes"}, "can't set immutable config"},
		{[]string{"CONFIG", "SET", "search-default-scorer", "TFIDF"}, "is unavailable when `search-enable-unstable-features` is off"},
		{[]string{"CONFIG", "SET", "search-min-prefix", "3"}, "+OK"},
		{[]string{"FT.CONFIG", "GET", "MINPREFIX"}, "$1\r\n3"},
		{[]string{"FT.CONFIG", "SET", "TIMEOUT", "900"}, "+OK"},
		{[]string{"CONFIG", "GET", "search-timeout"}, "$3\r\n900"},
		{[]string{"CONFIG", "SET", "bf-error-rate", "1"}, "Value for `bf-error-rate` must be in the range (0.000000, 1.000000)"},
		{[]string{"CONFIG", "SET", "cf-bucket-size", "03"}, "Invalid value for `cf-bucket-size`"},
		{[]string{"CONFIG", "SET", "ts-chunk-size-bytes", "50"}, "TSDB: CHUNK_SIZE value must be a multiple of 8 in the range [48 .. 1048576]"},
		{[]string{"CONFIG", "SET", "ts-duplicate-policy", "x"}, "Invalid duplicate policy: x"},
		{[]string{"CONFIG", "SET", "ts-compaction-policy", "MAX:1M:1H"}, "Invalid compaction policy: MAX:1M:1H"},
		{[]string{"CONFIG", "SET", "ts-compaction-policy", "max:60s:3600s;;avg:10s:1d:5s"}, "+OK"},
		{[]string{"CONFIG", "GET", "ts-compaction-policy"}, "max:1M:1h;avg:10s:1d:5s"},
		{[]string{"CONFIG", "SET", "ts-duplicate-policy", "LAST"}, "+OK"},
		{[]string{"TS.ADD", "t", "1000", "1", "LABELS", "a", "b"}, ":1000"},
		{[]string{"TS.INFO", "t"}, "+duplicatePolicy\r\n+last"},
		{[]string{"TS.INFO", "t_AVG_10000_5000"}, "+duplicatePolicy\r\n$-1"},
		{[]string{"TS.INFO", "t_MAX_60000"}, "*2\r\n$11\r\naggregation\r\n$3\r\nMAX\r\n*2\r\n$11\r\ntime_bucket\r\n$5\r\n60000\r\n+sourceKey\r\n$1\r\nt"},
		{[]string{"CONFIG", "SET", "bf-error-rate", "0.05"}, "+OK"},
		{[]string{"BF.ADD", "b", "x"}, ":1"},
		{[]string{"BF.INFO", "b"}, "+Size\r\n:"},
	} {
		if got := h.do(c.args...); !strings.Contains(got, c.want) {
			t.Errorf("%v: %q, want %q", c.args, got, c.want)
		}
	}
}
