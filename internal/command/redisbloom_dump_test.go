package command

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/cachestorm/cachestorm/internal/persistence"
	"github.com/cachestorm/cachestorm/internal/store"
)

// Redis 8.2's DUMP of TOPK.RESERVE tkg 3 8 2 1 then TOPK.ADD tkg a b a c a b d.
const redisTopKDump = "07814e8a4af9360f100002030208020204000000000000f03f05c336408001000080000410dea5ae01800ae004000497eddf2a02e002110317f53cf2e0030fc000e00727c000e0072f03e36c9ac0e0016701000005c33a40480fe36c9ac001000000788725e531730000400b40000317f53cf2400b01488880170003401420000397eddf2a40170058a0170002201320000000050264000502610005026200000c0098a930750d40536b"

// Redis 8.2's DUMP of TDIGEST.CREATE tdg COMPRESSION 10 then TDIGEST.ADD of
// 1 to 20.
const redisTDigestDump = "07814c3212f9360f100004000000000000244004000000000000f03f04000000000000344002404602080200020104000000000000344004000000000000000004000000000000f03f040000000000000040040000000000001040040000000000002240040000000000002d4004000000000080314004000000000000334004000000000000344004000000000000f03f04000000000000f03f040000000000000840040000000000001c4004000000000000104004000000000000004004000000000000f03f04000000000000f03f000c0085467c9d17ae18d0"

func TestRestoreRedisTopKAndTDigest(t *testing.T) {
	s := store.NewStore()
	r := NewRouter()
	RegisterProbabilisticCommands(r)
	RegisterStatsCommands(r)
	for key, h := range map[string]string{"tkg": redisTopKDump, "tdg": redisTDigestDump} {
		payload, err := hex.DecodeString(h)
		if err != nil {
			t.Fatal(err)
		}
		v, err := persistence.DecodeDump(payload)
		if err != nil {
			t.Fatalf("%s: %v", key, err)
		}
		if err := s.Set(key, v, store.SetOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		want string
		args []string
	}{
		{"*6\r\n$1\r\na\r\n:3\r\n$1\r\nb\r\n:2\r\n$1\r\nd\r\n:1\r\n", []string{"TOPK.LIST", "tkg", "WITHCOUNT"}},
		{bulks("10.5", "19"), []string{"TDIGEST.QUANTILE", "tdg", "0.5", "0.9"}},
		{"*18\r\n+Compression\r\n:10\r\n+Capacity\r\n:70\r\n+Merged nodes\r\n:8\r\n+Unmerged nodes\r\n:0\r\n+Merged weight\r\n:20\r\n+Unmerged weight\r\n:0\r\n+Observations\r\n:20\r\n+Total compressions\r\n:1\r\n+Memory usage\r\n:1128\r\n", []string{"TDIGEST.INFO", "tdg"}},
	} {
		if got := runCmd(t, s, r, tc.args[0], tc.args[1:]...); got != tc.want {
			t.Errorf("%v\n got %q\nwant %q", tc.args, got, tc.want)
		}
	}
	// Dumped again, the t-digest is byte for byte Redis's.
	e, _ := s.Get("tdg")
	again, err := persistence.EncodeDump(e.Value)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := hex.DecodeString(redisTDigestDump)
	if !bytes.Equal(again[:len(again)-10], want[:len(want)-10]) {
		t.Fatalf("dumped\n%x\nRedis dumps\n%x", again, want)
	}
}
