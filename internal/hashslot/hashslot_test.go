package hashslot

import "testing"

func TestKey(t *testing.T) {
	cases := map[string]int{
		"foo":                  12182,
		"bar":                  5061,
		"hello":                866,
		"":                     0,
		"{user1000}.following": 3443,
		"{user1000}.followers": 3443,
		"user1000":             3443,
		"foo{}{bar}":           Key("foo{}{bar}"),
		"foo{{bar}}zap":        Key("{bar"),
		"foo{bar}{zap}":        Key("bar"),
		"{}":                   Key("{}"),
		"a{b":                  Key("a{b"),
	}
	for key, want := range cases {
		if got := Key(key); got != want {
			t.Errorf("Key(%q) = %d, want %d", key, got, want)
		}
	}
	if CRC16("123456789") != 0x31C3 {
		t.Fatalf("CRC16 check value %#x", CRC16("123456789"))
	}
	if Key("foo{}{bar}") != int(CRC16("foo{}{bar}")&(Count-1)) {
		t.Fatal("an empty tag must hash the whole key")
	}
}
