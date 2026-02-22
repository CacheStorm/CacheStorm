package command

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestAllHyperLogLogCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterHyperLogLogCommands(router)

	tests := []struct {
		name  string
		cmd   string
		args  [][]byte
		setup func()
	}{
		{"PFADD single", "PFADD", [][]byte{[]byte("hll1"), []byte("element1")}, nil},
		{"PFADD multiple", "PFADD", [][]byte{[]byte("hll2"), []byte("a"), []byte("b"), []byte("c")}, nil},
		{"PFADD existing", "PFADD", [][]byte{[]byte("hll3"), []byte("new")}, func() {
			s.Set("hll3", &HyperLogLogValue{}, store.SetOptions{})
		}},
		{"PFCOUNT single", "PFCOUNT", [][]byte{[]byte("hll4")}, func() {
			s.Set("hll4", &HyperLogLogValue{}, store.SetOptions{})
		}},
		{"PFCOUNT multiple", "PFCOUNT", [][]byte{[]byte("hll5"), []byte("hll6")}, func() {
			s.Set("hll5", &HyperLogLogValue{}, store.SetOptions{})
			s.Set("hll6", &HyperLogLogValue{}, store.SetOptions{})
		}},
		{"PFMERGE", "PFMERGE", [][]byte{[]byte("merged"), []byte("hll7"), []byte("hll8")}, func() {
			s.Set("hll7", &HyperLogLogValue{}, store.SetOptions{})
			s.Set("hll8", &HyperLogLogValue{}, store.SetOptions{})
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				tt.setup()
			}

			handler, ok := router.Get(tt.cmd)
			if !ok {
				t.Fatalf("Command %s not found", tt.cmd)
			}

			ctx := newTestContext(tt.cmd, tt.args, s)
			if err := handler.Handler(ctx); err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestHyperLogLogValueOperations(t *testing.T) {
	t.Run("HyperLogLog Value Creation", func(t *testing.T) {
		hll := &HyperLogLogValue{}
		if hll == nil {
			t.Fatal("Failed to create HyperLogLogValue")
		}
	})

	t.Run("HyperLogLog Value Type", func(t *testing.T) {
		hll := &HyperLogLogValue{}
		if hll.Type() != store.DataTypeString {
			t.Error("HyperLogLog should have DataTypeString type")
		}
	})

	t.Run("HyperLogLog Value SizeOf", func(t *testing.T) {
		hll := &HyperLogLogValue{}
		size := hll.SizeOf()
		if size <= 0 {
			t.Error("SizeOf should return positive value")
		}
	})

	t.Run("HyperLogLog Add and Count", func(t *testing.T) {
		hll := &HyperLogLogValue{}

		// Add elements
		changed := hll.Add([]byte("element1"))
		if !changed {
			t.Error("Add should return true for new element")
		}

		// Add same element again
		changed = hll.Add([]byte("element1"))
		if changed {
			t.Error("Add should return false for duplicate element")
		}

		// Get count
		count := hll.Count()
		if count < 0 {
			t.Error("Count should return non-negative value")
		}
	})

	t.Run("HyperLogLog Merge", func(t *testing.T) {
		hll1 := &HyperLogLogValue{}
		hll2 := &HyperLogLogValue{}

		hll1.Add([]byte("a"))
		hll1.Add([]byte("b"))
		hll2.Add([]byte("c"))
		hll2.Add([]byte("d"))

		hll1.Merge(hll2)

		count := hll1.Count()
		if count < 0 {
			t.Error("Merged count should be non-negative")
		}
	})
}
