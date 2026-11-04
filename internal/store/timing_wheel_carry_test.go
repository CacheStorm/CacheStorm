package store

import (
	"testing"
	"time"
)

// A key still in a level-0 slot on the slot's last sweep, due before the
// next slot is done, moves to the next slot instead of waiting an hour.
func TestWheelCarriesKeysLeftOnASlotsLastSweep(t *testing.T) {
	s := NewStore()
	tw := NewTimingWheel(s)
	now := time.Now().UnixMilli()
	l := tw.levels[0]
	tw.subtick = int(time.Second/wheelTickInterval) - 1
	cur := l.current
	l.slots[cur].keys["soon"] = now + 500
	l.slots[cur].keys["later"] = now + 30*60*1000
	tw.tick()
	if l.current != cur+1 {
		t.Fatalf("pointer at %d, want %d", l.current, cur+1)
	}
	if _, ok := l.slots[cur+1].keys["soon"]; !ok {
		t.Fatal("a key due in 500ms was left behind in the slot just passed")
	}
	if _, ok := l.slots[cur].keys["soon"]; ok {
		t.Fatal("the carried key is still in the old slot")
	}
	if _, ok := l.slots[cur].keys["later"]; !ok {
		t.Fatal("a key due in half an hour was carried")
	}
}

func TestWheelCarryKeepsTheLaterSchedule(t *testing.T) {
	tw := NewTimingWheel(NewStore())
	from, to := newWheelBucket(), newWheelBucket()
	from.keys["k"] = 100
	to.keys["k"] = 200
	from.keys["j"] = 300
	to.keys["j"] = 150
	tw.carry(from, to, 1000)
	if to.keys["k"] != 200 || to.keys["j"] != 300 || len(from.keys) != 0 {
		t.Fatalf("to %v from %v", to.keys, from.keys)
	}
}
