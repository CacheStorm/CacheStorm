package command

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestReplayTimeSeriesUsesPrivateState(t *testing.T) {
	const key = "atomic-replay-time-series"
	tsManager.Delete(key)
	t.Cleanup(func() { tsManager.Delete(key) })
	if err := tsManager.Create(key, 0, nil); err != nil {
		t.Fatal(err)
	}
	old, _ := tsManager.Get(key)
	old.Add(1, 9)
	r := NewRouter()
	RegisterTSCommands(r)
	st := store.NewStore()
	replay := r.ForkForReplay(st, []string{"TS.CREATE", "TS.ADD"})
	for _, args := range [][]string{{"TS.CREATE", key}, {"TS.ADD", key, "2", "20"}} {
		ctx := NewContext(args[0], bytesArgs(args[1:]...), st, nil)
		if err := replay.ExecuteSilent(ctx); err != nil || ctx.Writer.Errors() > 0 {
			t.Fatalf("replay %v: %v", args, err)
		}
	}
	entry, exists := st.Get(key)
	if !exists {
		t.Fatal("replay lost the native time-series value")
	}
	value := entry.Value.(*store.TimeSeriesValue)
	if got := value.Range(0, 10); len(got) != 1 || got[0].Timestamp != 2 || got[0].Value != 20 {
		t.Fatalf("replayed samples = %v", got)
	}
	if got := old.Range(0, 10); len(got) != 1 || got[0].Timestamp != 1 || got[0].Value != 9 {
		t.Fatalf("replay modified the process-global extension state: %v", got)
	}
}
