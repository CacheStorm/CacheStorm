package command

import (
	"bytes"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func TestTimeSeriesLiveInTheKeyspace(t *testing.T) {
	router := NewRouter()
	RegisterTSCommands(router)
	RegisterKeyCommands(router)
	RegisterStringCommands(router)
	RegisterServerCommands(router)
	st := store.NewStore()
	run := func(args ...string) string {
		t.Helper()
		var out bytes.Buffer
		ctx := NewContext(args[0], bytesArgs(args[1:]...), st, resp.NewWriter(&out))
		if err := router.Execute(ctx); err != nil {
			t.Fatal(err)
		}
		ctx.Writer.Flush()
		return out.String()
	}
	for _, c := range [][2][]string{
		{{"TS.CREATE", "temp:1", "LABELS", "sensor", "temp", "room", "a"}, {"+OK\r\n"}},
		{{"TS.CREATE", "temp:1"}, {"-ERR TSDB: key already exists\r\n"}},
		{{"TS.ADD", "temp:2", "10", "1.5", "LABELS", "sensor", "temp", "room", "b"}, {":10\r\n"}},
		{{"TS.ADD", "hum:1", "10", "40", "LABELS", "sensor", "hum"}, {":10\r\n"}},
		{{"TYPE", "temp:1"}, {"+TSDB-TYPE\r\n"}},
		{{"EXISTS", "temp:1", "temp:2"}, {":2\r\n"}},
		{{"SCAN", "0", "TYPE", "tsdb-type", "MATCH", "hum*"}, {"*2\r\n$1\r\n0\r\n*1\r\n$5\r\nhum:1\r\n"}},
		{{"TS.QUERYINDEX", "sensor=temp"}, {"*2\r\n$6\r\ntemp:1\r\n$6\r\ntemp:2\r\n"}},
		{{"TS.QUERYINDEX", "sensor=temp", "room!=a"}, {"*1\r\n$6\r\ntemp:2\r\n"}},
		{{"TS.QUERYINDEX", "sensor=(hum,temp)", "room="}, {"*1\r\n$5\r\nhum:1\r\n"}},
		{{"TS.QUERYINDEX", "room!="}, {"-ERR TSDB: please provide at least one matcher\r\n"}},
		{{"TS.QUERYINDEX", "sensor"}, {"-ERR TSDB: failed parsing labels\r\n"}},
		{{"SET", "s", "v"}, {"+OK\r\n"}},
		{{"TS.ADD", "s", "1", "1"}, {"-WRONGTYPE Operation against a key holding the wrong kind of value\r\n"}},
		{{"GET", "temp:2"}, {"-WRONGTYPE Operation against a key holding the wrong kind of value\r\n"}},
		{{"DEL", "temp:2"}, {":1\r\n"}},
		{{"TS.GET", "temp:2"}, {"-ERR TSDB: the key does not exist\r\n"}},
		{{"TS.DEL", "temp:2", "-", "+"}, {"-ERR TSDB: the key does not exist\r\n"}},
	} {
		if got := run(c[0]...); got != c[1][0] {
			t.Fatalf("%v = %q, want %q", c[0], got, c[1][0])
		}
	}
}
