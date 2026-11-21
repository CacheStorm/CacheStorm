package xxh3

import "testing"

// The expected hashes come from xxHash 0.8's C implementation, as bundled
// with Redis 8.10.3, over the same pseudo-random bytes.
func TestHash64(t *testing.T) {
	buf := make([]byte, 5000)
	x := uint64(0x123456789abcdef)
	for i := range buf {
		x = x*6364136223846793005 + 1442695040888963407
		buf[i] = byte(x >> 56)
	}
	for _, tc := range []struct {
		n    int
		want uint64
	}{
		{0, 0x2d06800538d394c2},
		{1, 0x32cd626f54ba457e},
		{2, 0x2ccf4391dedeb5a3},
		{3, 0x4b1ee21562e4323d},
		{4, 0x9ed47e71ebdd1970},
		{5, 0x70d8846069d67fff},
		{7, 0x150140f46854b3a8},
		{8, 0x8368b17a687ac5be},
		{9, 0x07fdfeff3737644c},
		{12, 0x652d13a3782492b7},
		{15, 0x25b74146c9255649},
		{16, 0xe4cd981221579c6f},
		{17, 0xdf4916f527f90f58},
		{31, 0x9bb9da0c02eba259},
		{32, 0xbfeacf2d9c5e74db},
		{33, 0x08106d1e20687494},
		{63, 0x7663a5fb8c95eec0},
		{64, 0x7ae35ade472a64d5},
		{65, 0x4be824e7bb5a7e89},
		{95, 0x5bbb3a61730dae42},
		{96, 0xa4f68175b3095adc},
		{97, 0x2e37fc42173af97f},
		{127, 0xacebc693f6fb6e5b},
		{128, 0xe1c450ee5fbe2ac5},
		{129, 0xc9c2c4d9e2a6ebc8},
		{130, 0x6ea371b3298f3c38},
		{160, 0x64b42b25dc4df9a9},
		{200, 0xae9b8eb27c6959d2},
		{239, 0xf2b4dd9e8ccd4b4f},
		{240, 0x790c5975718712bb},
		{241, 0x8c73e5677b5bbf3d},
		{255, 0x92222a12c161befd},
		{256, 0x085a65011e5efe88},
		{257, 0x164674dd23d9b2a1},
		{500, 0xba9c2662ca5dbd55},
		{1023, 0xf0af0ffdaa17cef0},
		{1024, 0xd1e23a588641df5c},
		{1025, 0xad702c245b7bbcad},
		{1087, 0xecc01405482477cf},
		{1088, 0x55ce8b19897926d5},
		{1089, 0x02a93a7ff8d5a2a6},
		{2047, 0x9513dd1d8cd58fe9},
		{2048, 0x8f191c89a8f6cfcd},
		{2049, 0x888dbc1862e683c2},
		{4096, 0xde477e9d79bf8788},
		{4999, 0x55b7c6ceee0ce8f7},
	} {
		if got := Hash64(buf[:tc.n]); got != tc.want {
			t.Errorf("len %d: %016x, want %016x", tc.n, got, tc.want)
		}
	}
	if got := Hash64([]byte("hello")); got != 0x9555e8555c62dcfd {
		t.Errorf("hello: %016x", got)
	}
}
