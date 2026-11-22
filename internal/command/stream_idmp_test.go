package command

import (
	"encoding/hex"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

// The expected iids come from Redis 8.10.3's createIdempotencyHash, run
// with the xxHash it bundles.
func TestIDMPAutoIID(t *testing.T) {
	pairs := func(kv ...string) []store.StreamField {
		out := make([]store.StreamField, 0, len(kv)/2)
		for i := 0; i < len(kv); i += 2 {
			out = append(out, store.StreamField{Name: kv[i], Value: []byte(kv[i+1])})
		}
		return out
	}
	for _, tc := range []struct {
		pairs []store.StreamField
		want  string
	}{
		{pairs("f", "v"), "6e95729ee727f0814aedffba55730079"},
		{pairs("temperature", "21.5", "sensor", "kitchen-01", "field-with-a-much-longer-name-to-pass-sixteen-bytes", "x"), "cd2a1239b479a56ce1c154c605f8a783"},
		{pairs("sensor", "kitchen-01", "temperature", "21.5"), "785dfc6f7424a0f4b1599df054a9550a"},
		// The pairs' order does not matter.
		{pairs("temperature", "21.5", "sensor", "kitchen-01"), "785dfc6f7424a0f4b1599df054a9550a"},
	} {
		if got := hex.EncodeToString([]byte(idmpAutoIID(tc.pairs))); got != tc.want {
			t.Errorf("idmpAutoIID(%v) = %s, want %s", tc.pairs, got, tc.want)
		}
	}
}
