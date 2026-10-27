package command

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestHashExistingEmptyKeyX(t *testing.T) {
	for _, name := range []string{"HSET", "HMSET", "HSETNX", "HINCRBY", "HINCRBYFLOAT"} {
		t.Run(name, func(t *testing.T) {
			s := store.NewStore()
			if err := s.SetEntry("", store.NewEntry(&store.HashValue{Fields: map[string][]byte{"f": []byte("1")}})); err != nil {
				t.Fatal(err)
			}
			r := NewRouter()
			RegisterHashCommands(r)
			got := runEntryTransfer(t, s, nil, r, name, "", "g", "2")
			if len(got) == 0 || got[0] == '-' {
				t.Fatalf("existing empty-key hash write=%q", got)
			}
			if got := runEntryTransfer(t, s, nil, r, "HGET", "", "g"); got != "$1\r\n2\r\n" {
				t.Fatal(got)
			}
		})
	}
}
