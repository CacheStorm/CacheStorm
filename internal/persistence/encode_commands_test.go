package persistence

import "testing"

func TestEncodeDecodeCommandsRoundTrip(t *testing.T) {
	cmds := [][]string{{"FT.CREATE", "idx"}, {"FT.ADD", "idx", "d", "1", "FIELDS", "f", "a\r\nb\x00"}}
	got, err := DecodeCommands(EncodeCommands(cmds))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(cmds) {
		t.Fatalf("decoded %d commands, want %d", len(got), len(cmds))
	}
	for i, c := range cmds {
		if got[i].Name != c[0] || len(got[i].Args) != len(c)-1 {
			t.Fatalf("command %d = %+v", i, got[i])
		}
		for j, a := range c[1:] {
			if string(got[i].Args[j]) != a {
				t.Fatalf("command %d arg %d = %q, want %q", i, j, got[i].Args[j], a)
			}
		}
	}
	if got, err := DecodeCommands(nil); err != nil || len(got) != 0 {
		t.Fatalf("DecodeCommands(nil) = %v, %v", got, err)
	}
	if _, err := DecodeCommands([]byte("*2\r\n$3\r\nabc")); err == nil {
		t.Fatal("truncated payload decoded without an error")
	}
}
