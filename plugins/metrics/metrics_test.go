package metrics

import (
	"strings"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/command"
)

// Regression: counter values were rendered via string(rune(n)), which emits
// a control character instead of the decimal number for any n >= 10 (and
// non-digit characters for small n).
func TestExportPrometheusRendersNumbers(t *testing.T) {
	p := New(true)
	p.RecordHit()
	p.RecordMiss()
	p.SetKeysTotal(12)
	p.SetMemoryBytes(1474)

	out := p.ExportPrometheus()

	for _, want := range []string{
		"cachestorm_hit_total 1\n",
		"cachestorm_miss_total 1\n",
		"cachestorm_keys_total 12\n",
		"cachestorm_memory_bytes 1474\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output:\n%s", want, out)
		}
	}
	if strings.ContainsAny(out, "\x00\x01\x02\x0c") {
		t.Errorf("output contains control characters from rune conversion:\n%q", out)
	}
}

func TestAfterCommandCountsCommandWithDuration(t *testing.T) {
	p := New(true)

	ctx := command.NewContext("SET", [][]byte{[]byte("k"), []byte("v")}, nil, nil)
	ctx.StartTime = time.Now()
	p.AfterCommand(ctx)

	out := p.ExportPrometheus()
	if !strings.Contains(out, `cachestorm_commands_total{command="SET"} 1`+"\n") {
		t.Errorf("SET counter missing or not numeric:\n%s", out)
	}
}
