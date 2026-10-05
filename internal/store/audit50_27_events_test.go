package store

import (
	"bytes"
	"reflect"
	"testing"
)

func checkAudit50R27X(t *testing.T, label string, want, got interface{}) {
	t.Helper()
	t.Logf("%s EXPECTED: %v | ACTUAL: %v", label, want, got)
	if !reflect.DeepEqual(want, got) {
		t.Errorf("%s mismatch", label)
	}
}

func TestAudit50R27X(t *testing.T) {
	defer func() {
		if t.Failed() {
			t.Log("PROBLEM CONFIRMED")
		} else {
			t.Log("FIX VERIFIED")
		}
	}()

	input := []byte("hello world")
	rle := &RLECompressor{}
	encoded, _ := rle.Compress(input)
	decoded, _ := rle.Decompress(encoded)
	checkAudit50R27X(t, "control RLE round trip", input, decoded)
	lz := &LZ4Compressor{}
	encoded, _ = lz.Compress(input)
	decoded, _ = lz.Decompress(encoded)
	checkAudit50R27X(t, "LZ4 normal round trip", input, decoded)

	for _, input := range [][]byte{nil, {}, []byte("x"), []byte("abcdabcdabcdabcd"), bytes.Repeat([]byte("a"), 60), bytes.Repeat([]byte("0123456789abcdefghijklmnop"), 5), {0, 255, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 255}} {
		encoded, _ := lz.Compress(input)
		decoded, _ := lz.Decompress(encoded)
		checkAudit50R27X(t, "empty/binary/long/matching round trip", string(input), string(decoded))
	}
	em := NewEventManager()
	ch := em.Subscribe("event")
	em.Emit("event", nil)
	select {
	case event := <-ch:
		checkAudit50R27X(t, "event delivery control", "event", event.Name)
	default:
		t.Error("no delivery")
	}
	em.Unsubscribe("event", ch)
	em.Unsubscribe("event", ch)
	_, open := <-ch
	checkAudit50R27X(t, "unsubscription closes channel", false, open)
	em.Emit("event", nil)
	checkAudit50R27X(t, "listener removed", 0, len(em.Listeners))

}
