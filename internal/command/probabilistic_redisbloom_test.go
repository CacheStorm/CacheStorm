package command

import (
	"bytes"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// The expected replies were taken from Redis 8.2, whose probabilistic
// structures are RedisBloom's.
func TestProbabilisticRepliesMatchRedisBloom(t *testing.T) {
	router := NewRouter()
	RegisterProbabilisticCommands(router)
	RegisterStringCommands(router)
	RegisterServerCommands(router)
	RegisterKeyCommands(router)
	st := store.NewStore()
	for _, c := range []struct{ cmd, want string }{
		{"BF.RESERVE b 0.01 100", "+OK\r\n"},
		{"BF.RESERVE b 0.01 100", "-ERR item exists\r\n"},
		{"BF.RESERVE n 1 100", "-ERR error rate must be in the range (0.000000, 1.000000)\r\n"},
		{"BF.RESERVE n 0.01 0", "-ERR capacity must be in the range [1, 1073741824]\r\n"},
		{"BF.RESERVE n x 100", "-ERR bad error rate\r\n"},
		{"BF.RESERVE n 0.01 100 NONSCALING EXPANSION 2", "-Nonscaling filters cannot expand\r\n"},
		{"BF.MADD b a b", "*2\r\n:1\r\n:1\r\n"},
		{"BF.ADD b a", ":0\r\n"},
		{"BF.MEXISTS b a zz", "*2\r\n:1\r\n:0\r\n"},
		{"BF.CARD b", ":2\r\n"},
		{"BF.CARD nokey", ":0\r\n"},
		{"BF.INFO nokey", "-ERR not found\r\n"},
		{"BF.INFO b ITEMS", "*1\r\n:2\r\n"},
		{"BF.INFO b foo", "-Invalid information value\r\n"},
		{"BF.INSERT nc NOCREATE ITEMS a", "-ERR not found\r\n"},
		{"BF.INSERT i foo ITEMS a", "-Unknown argument received\r\n"},
		{"BF.INSERT i2 CAPACITY 2 NONSCALING ITEMS 1 2 3 4", "*3\r\n:1\r\n:1\r\n-ERR non scaling filter is full\r\n"},
		{"BF.INFO i2 EXPANSION", "*1\r\n$-1\r\n"},
		{"TYPE b", "+MBbloom--\r\n"},
		{"SET s v", "+OK\r\n"},
		{"BF.ADD s x", "-WRONGTYPE Operation against a key holding the wrong kind of value\r\n"},
		{"BF.EXISTS s x", ":0\r\n"},

		{"CF.RESERVE c 0", "-Capacity must be in the range [2 * BUCKETSIZE, 1073741824]\r\n"},
		{"CF.RESERVE c 10", "+OK\r\n"},
		{"CF.RESERVE c2 10 BUCKETSIZE 0", "-BUCKETSIZE: value must be in the range [1, 255]\r\n"},
		{"CF.RESERVE c2 10 MAXITERATIONS 0", "-MAXITERATIONS: value must be in the range [1, 65535]\r\n"},
		{"CF.ADD c a", ":1\r\n"},
		{"CF.ADD c a", ":1\r\n"},
		{"CF.ADDNX c a", ":0\r\n"},
		{"CF.COUNT c a", ":2\r\n"},
		{"CF.DEL c a", ":1\r\n"},
		{"CF.DEL nokey a", "-Not found\r\n"},
		{"CF.MEXISTS nokey a b", "*2\r\n:0\r\n:0\r\n"},
		{"CF.INFO c", "*16\r\n+Size\r\n:72\r\n+Number of buckets\r\n:8\r\n+Number of filters\r\n:1\r\n+Number of items inserted\r\n:1\r\n+Number of items deleted\r\n:1\r\n+Bucket size\r\n:2\r\n+Expansion rate\r\n:1\r\n+Max iterations\r\n:20\r\n"},
		{"CF.RESERVE cf2 2 BUCKETSIZE 1 MAXITERATIONS 1 EXPANSION 0", "+OK\r\n"},
		{"CF.INSERT cf2 ITEMS 1 2 3", "*3\r\n:1\r\n:1\r\n-Filter is full\r\n"},
		{"TYPE c", "+MBbloomCF\r\n"},

		{"CMS.INITBYDIM m 0 5", "-CMS: invalid width\r\n"},
		{"CMS.INITBYPROB p 0.001 0.01", "+OK\r\n"},
		{"CMS.INFO p", "*6\r\n+width\r\n:2000\r\n+depth\r\n:7\r\n+count\r\n:0\r\n"},
		{"CMS.INITBYDIM m 2000 5", "+OK\r\n"},
		{"CMS.INITBYDIM s 10 5", "-CMS: key already exists\r\n"},
		{"CMS.INCRBY m a 5 b 3 a 2", "*3\r\n:5\r\n:3\r\n:7\r\n"},
		{"CMS.INCRBY m a -1", "-CMS: Number cannot be negative\r\n"},
		{"CMS.INCRBY nokey a 1", "-CMS: key does not exist\r\n"},
		{"CMS.INITBYDIM m2 2000 5", "+OK\r\n"},
		{"CMS.INCRBY m2 a 10", "*1\r\n:10\r\n"},
		{"CMS.INITBYDIM d 2000 5", "+OK\r\n"},
		{"CMS.MERGE d 2 m m2 WEIGHTS 2 3", "+OK\r\n"},
		{"CMS.QUERY d a", "*1\r\n:44\r\n"},
		{"CMS.MERGE d 2 m p", "-CMS: width/depth is not equal\r\n"},
		{"CMS.MERGE d 1 m WEIGHTS 1 2", "-CMS: wrong number of keys/weights\r\n"},
		{"CMS.INCRBY m a 4294967295", "*1\r\n-CMS: INCRBY overflow\r\n"},
		{"TYPE m", "+CMSk-TYPE\r\n"},

		{"TOPK.RESERVE t 0", "-TopK: invalid k\r\n"},
		{"TOPK.RESERVE t 3", "+OK\r\n"},
		{"TOPK.RESERVE t 3", "-TopK: key already exists\r\n"},
		{"TOPK.RESERVE t2 3 8 7 1.5", "-TopK: invalid decay value. must be '<= 1' & '> 0'\r\n"},
		{"TOPK.INFO t", "*8\r\n+k\r\n:3\r\n+width\r\n:8\r\n+depth\r\n:7\r\n+decay\r\n$3\r\n0.9\r\n"},
		{"TOPK.INCRBY t z 10", "*1\r\n$-1\r\n"},
		{"TOPK.LIST t WITHCOUNT", "*2\r\n$1\r\nz\r\n:10\r\n"},
		{"TOPK.LIST t foo", "-WITHCOUNT keyword expected\r\n"},
		{"TOPK.QUERY t z y", "*2\r\n:1\r\n:0\r\n"},
		{"TOPK.ADD nokey a", "-TopK: key does not exist\r\n"},
		{"TYPE t", "+TopK-TYPE\r\n"},
	} {
		args := strings.Fields(c.cmd)
		var out bytes.Buffer
		ctx := NewContext(strings.ToUpper(args[0]), bytesArgs(args[1:]...), st, resp.NewWriter(&out))
		if err := router.Execute(ctx); err != nil {
			t.Fatalf("%s: %v", c.cmd, err)
		}
		ctx.Writer.Flush()
		if out.String() != c.want {
			t.Errorf("%s = %q, want %q", c.cmd, out.String(), c.want)
		}
	}
}

func TestModuleCommandKeys(t *testing.T) {
	for _, c := range []struct {
		cmd  string
		want string
	}{
		{"CMS.MERGE d 2 a b WEIGHTS 1 2", "d a b"},
		{"TDIGEST.MERGE d 1 a COMPRESSION 100", "d a"},
		{"TS.MADD a 1 1 b 2 2", "a b"},
		{"JSON.MSET a $ 1 b $ 2", "a b"},
		{"JSON.MGET a b $", "a b"},
		{"TS.QUERYINDEX sensor=temp", ""},
		{"FT.SEARCH idx hello", ""},
		{"BF.ADD k x", "k"},
	} {
		args := strings.Fields(c.cmd)
		if got := strings.Join(CommandKeys(args[0], bytesArgs(args[1:]...)), " "); got != c.want {
			t.Errorf("keys of %s = %q, want %q", c.cmd, got, c.want)
		}
	}
}
