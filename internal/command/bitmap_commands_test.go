package command

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestAllBitmapCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterBitmapCommands(router)

	tests := []struct {
		name  string
		cmd   string
		args  [][]byte
		setup func()
	}{
		{"SETBIT new", "SETBIT", [][]byte{[]byte("bit1"), []byte("7"), []byte("1")}, nil},
		{"SETBIT existing", "SETBIT", [][]byte{[]byte("bit2"), []byte("0"), []byte("1")}, func() {
			s.Set("bit2", &store.StringValue{Data: []byte{0x00}}, store.SetOptions{})
		}},
		{"GETBIT set", "GETBIT", [][]byte{[]byte("bit3"), []byte("7")}, func() {
			s.Set("bit3", &store.StringValue{Data: []byte{0x80}}, store.SetOptions{}) // bit 7 set
		}},
		{"GETBIT unset", "GETBIT", [][]byte{[]byte("bit4"), []byte("0")}, func() {
			s.Set("bit4", &store.StringValue{Data: []byte{0x00}}, store.SetOptions{})
		}},
		{"GETBIT nonexistent", "GETBIT", [][]byte{[]byte("nonexistent"), []byte("0")}, nil},
		{"BITCOUNT", "BITCOUNT", [][]byte{[]byte("bit5")}, func() {
			s.Set("bit5", &store.StringValue{Data: []byte{0xFF, 0x0F}}, store.SetOptions{}) // 12 bits set
		}},
		{"BITCOUNT with range", "BITCOUNT", [][]byte{[]byte("bit6"), []byte("0"), []byte("0")}, func() {
			s.Set("bit6", &store.StringValue{Data: []byte{0xFF, 0x0F}}, store.SetOptions{})
		}},
		{"BITPOS", "BITPOS", [][]byte{[]byte("bit7"), []byte("1")}, func() {
			s.Set("bit7", &store.StringValue{Data: []byte{0x00, 0x80}}, store.SetOptions{}) // bit 15 is set
		}},
		{"BITOP AND", "BITOP", [][]byte{[]byte("AND"), []byte("result1"), []byte("bit8"), []byte("bit9")}, func() {
			s.Set("bit8", &store.StringValue{Data: []byte{0xFF, 0x0F}}, store.SetOptions{})
			s.Set("bit9", &store.StringValue{Data: []byte{0x0F, 0xFF}}, store.SetOptions{})
		}},
		{"BITOP OR", "BITOP", [][]byte{[]byte("OR"), []byte("result2"), []byte("bit10"), []byte("bit11")}, func() {
			s.Set("bit10", &store.StringValue{Data: []byte{0xFF, 0x00}}, store.SetOptions{})
			s.Set("bit11", &store.StringValue{Data: []byte{0x00, 0xFF}}, store.SetOptions{})
		}},
		{"BITOP XOR", "BITOP", [][]byte{[]byte("XOR"), []byte("result3"), []byte("bit12"), []byte("bit13")}, func() {
			s.Set("bit12", &store.StringValue{Data: []byte{0xFF, 0x00}}, store.SetOptions{})
			s.Set("bit13", &store.StringValue{Data: []byte{0x00, 0xFF}}, store.SetOptions{})
		}},
		{"BITOP NOT", "BITOP", [][]byte{[]byte("NOT"), []byte("result4"), []byte("bit14")}, func() {
			s.Set("bit14", &store.StringValue{Data: []byte{0xFF, 0x00}}, store.SetOptions{})
		}},
		{"BITFIELD SET", "BITFIELD", [][]byte{[]byte("bf1"), []byte("SET"), []byte("u8"), []byte("0"), []byte("255")}, nil},
		{"BITFIELD GET", "BITFIELD", [][]byte{[]byte("bf2"), []byte("GET"), []byte("u8"), []byte("0")}, func() {
			s.Set("bf2", &store.StringValue{Data: []byte{0xFF, 0x00}}, store.SetOptions{})
		}},
		{"BITFIELD INCRBY", "BITFIELD", [][]byte{[]byte("bf3"), []byte("INCRBY"), []byte("u8"), []byte("0"), []byte("1")}, func() {
			s.Set("bf3", &store.StringValue{Data: []byte{0x00}}, store.SetOptions{})
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
