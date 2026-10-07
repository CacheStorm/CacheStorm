package batch

import "testing"

func TestPipelineOwnsQueuedArguments(t *testing.T) {
	control := NewPipeline()
	control.Add("SET", [][]byte{[]byte("key"), []byte("original")})
	snapshot := control.Commands()
	snapshot[0].Args[1][0] = 'X'
	if string(control.Commands()[0].Args[1]) != "original" {
		t.Fatal("unaffected control failed")
	}
	t.Log("CONTROL: returned snapshots own their data")
	for _, args := range [][][]byte{nil, {}, {nil, {}, []byte("value")}} {
		p := NewPipeline()
		p.Add("GET", args)
		got := p.Commands()[0].Args
		if (got == nil) != (args == nil) || len(got) != len(args) {
			t.Fatalf("nil/empty arguments changed: %v", got)
		}
		p.Clear()
		p.Add("PING", nil)
		if p.Len() != 1 || p.Commands()[0].Name != "PING" {
			t.Fatal("clear/reuse failed")
		}
	}
	for _, replace := range []bool{false, true} {
		t.Run(map[bool]string{false: "backing_bytes", true: "argument_slice"}[replace], func(t *testing.T) {
			p := NewPipeline()
			args := [][]byte{[]byte("key"), []byte("original")}
			p.Add("SET", args)
			reuse := make(chan struct{})
			done := make(chan struct{})
			go func() {
				<-reuse
				if replace {
					args[1] = []byte("replaced")
				} else {
					copy(args[1], "changed!")
				}
				close(done)
			}()
			close(reuse)
			<-done
			got := string(p.Commands()[0].Args[1])
			t.Logf("EXPECTED: queued value=original; ACTUAL: queued value=%s", got)
			if got != "original" {
				t.Fatal("PROBLEM CONFIRMED")
			}
			t.Log("FIX VERIFIED")
		})
	}
}
