package replication

import (
	"bufio"
	"bytes"
	"io"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/config"
	"github.com/cachestorm/cachestorm/internal/resp"
)

type handshakeFramesX struct{ frames [][]byte }

func (w *handshakeFramesX) Write(p []byte) (int, error) {
	w.frames = append(w.frames, bytes.Clone(p))
	return len(p), nil
}

func TestReplicationHandshakeRESPFraming(t *testing.T) {
	for _, port := range []int{0, 1, 7, 99, 6379, 65535} {
		m := &Manager{cfg: &config.ReplicationConfig{ReplicaAnnouncePort: port}}
		wire := &handshakeFramesX{}
		if err := m.sendHandshake(bufio.NewWriter(wire)); err != nil {
			t.Fatal(err)
		}
		if len(wire.frames) != 4 {
			t.Fatalf("frames=%d", len(wire.frames))
		}
		r := resp.NewReader(bytes.NewReader(wire.frames[0]))
		name, args, err := r.ReadCommand()
		if err != nil || name != "PING" || len(args) != 0 {
			t.Fatalf("unaffected control: %s %q %v", name, args, err)
		}
		t.Log("CONTROL: PING is valid RESP")
		for i, want := range []struct {
			name string
			args [][]byte
		}{
			{"REPLCONF", [][]byte{[]byte("listening-port"), []byte(strconv.Itoa(port))}},
			{"REPLCONF", [][]byte{[]byte("capa"), []byte("psync2")}},
			{"PSYNC", [][]byte{[]byte(strings.Repeat("?", 40)), []byte("-1")}},
		} {
			t.Run(want.name+strconv.Itoa(i), func(t *testing.T) {
				r := resp.NewReader(bytes.NewReader(wire.frames[i+1]))
				name, args, err := r.ReadCommand()
				t.Logf("EXPECTED: %s %q error=nil; ACTUAL: %s %q error=%v", want.name, want.args, name, args, err)
				if err != nil || name != want.name || !reflect.DeepEqual(args, want.args) {
					t.Fatal("PROBLEM CONFIRMED")
				}
				if _, _, err := r.ReadCommand(); err != io.EOF {
					t.Fatalf("trailing bytes: %v", err)
				}
			})
		}
	}
	if !t.Failed() {
		t.Log("FIX VERIFIED")
	}
}
