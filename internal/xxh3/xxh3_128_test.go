package xxh3

import (
	"encoding/binary"
	"testing"
)

// The expected hashes come from xxHash 0.8's C implementation, as bundled
// with Redis 8.10.3, over the same pseudo-random bytes as TestHash64.
func TestHash128(t *testing.T) {
	buf := make([]byte, 5000)
	x := uint64(0x123456789abcdef)
	for i := range buf {
		x = x*6364136223846793005 + 1442695040888963407
		buf[i] = byte(x >> 56)
	}
	for _, tc := range []struct {
		n      int
		lo, hi uint64
	}{
		{0, 0x6001c324468d497f, 0x99aa06d3014798d8},
		{1, 0x32cd626f54ba457e, 0xe3eef7a32ad3d570},
		{2, 0x2ccf4391dedeb5a3, 0x7656ad1c5284d5c4},
		{3, 0x4b1ee21562e4323d, 0x7ecaceef4197dca1},
		{4, 0x63d7e8e9493f2a7d, 0x1c64992f68f4e894},
		{5, 0x5c09fb20d234d7b1, 0x37da0a079f5ffdbf},
		{7, 0x642351737286eeb4, 0xdd29efd11caef202},
		{8, 0x98d7cc0a399de05f, 0xce7e75b4919d08bb},
		{9, 0xb34daf97a7930506, 0x2fd9ffd95ae29ee2},
		{12, 0x4fe01a3b0c35a5d7, 0x1dbfd53891a19219},
		{15, 0xe9f8e3dd5a2a78b7, 0xf60af90b78e9291d},
		{16, 0xee25b925971c758c, 0xc57356e55d59ba83},
		{17, 0xd474ead3d8e2c174, 0x57c77549c942b5b5},
		{31, 0x3bf100ceca06dd8f, 0xcbf608ff34172c91},
		{32, 0x0d6230412c92ce93, 0x99d6f8dcf9b33a9b},
		{33, 0xb75297099145bc03, 0x155248dbb3ba6741},
		{63, 0xe7018069ee3df5cb, 0x3dc3380f180a80c5},
		{64, 0xadfbb2ed99675c6e, 0x6cd238bd09fd965c},
		{65, 0x9297b13093bcf9a0, 0x184ae039a1e544fb},
		{95, 0x4b5641eb4dd5f825, 0x655e22e51093ab38},
		{96, 0xe2b0ac2eeec3ff22, 0xa64beb068108d976},
		{97, 0x1b6080419228a603, 0x9f49cf82a624e984},
		{127, 0x8ebeb03bc8b30525, 0xb5971fc1044f7cc9},
		{128, 0x1f605730a5327aff, 0xaf783b27fddd5905},
		{129, 0xbd1771ef1f238993, 0x0cd99d2ed2c64c69},
		{130, 0x1f615484f59db711, 0x237c3d545dd430b1},
		{160, 0x0ceda854936f1f91, 0x3b7016bd2d4e1d2a},
		{191, 0x1907645f0409a7a1, 0x768add2619319392},
		{192, 0xf406d72a4a04193c, 0x7e2fff4d4fb53af9},
		{200, 0x35c962e76cb4cced, 0x050d508273ae34ca},
		{239, 0x2925fd530862f068, 0x5f8e3a30d1a5ac88},
		{240, 0xc94ae87d66c58e20, 0x3e72ca4e6a6acc92},
		{241, 0x8c73e5677b5bbf3d, 0xdd678bb9c64aa272},
		{255, 0x92222a12c161befd, 0xfa01bc06330ba090},
		{256, 0x085a65011e5efe88, 0x30c0ba7056448c71},
		{257, 0x164674dd23d9b2a1, 0xe6f32cb93a0619dc},
		{500, 0xba9c2662ca5dbd55, 0xa0489edd3a4d9407},
		{1023, 0xf0af0ffdaa17cef0, 0xe03546c5d3aab7f1},
		{1024, 0xd1e23a588641df5c, 0xf77072a062e9095d},
		{1025, 0xad702c245b7bbcad, 0x54757a64a97c4776},
		{1087, 0xecc01405482477cf, 0xf6f5b3b2daa17172},
		{1088, 0x55ce8b19897926d5, 0x128809c61423c41b},
		{1089, 0x02a93a7ff8d5a2a6, 0xb2704a40f79cddaa},
		{2047, 0x9513dd1d8cd58fe9, 0x65e75e91cb9fa318},
		{2048, 0x8f191c89a8f6cfcd, 0xa8ec27ebde43e1eb},
		{2049, 0x888dbc1862e683c2, 0x1be9c01c60f7063e},
		{4096, 0xde477e9d79bf8788, 0xf6391ee055dcaf8d},
		{4999, 0x55b7c6ceee0ce8f7, 0xe3a074441a0bd811},
	} {
		if got := Hash128(buf[:tc.n]); got != (Uint128{tc.lo, tc.hi}) {
			t.Errorf("Hash128(%d bytes) = %016x %016x, want %016x %016x", tc.n, got.Lo, got.Hi, tc.lo, tc.hi)
		}
	}
	// Redis hashes a field and value pair streaming; the same bytes at once.
	pair := []byte("f")
	pair = binary.LittleEndian.AppendUint64(pair, 1)
	pair = append(pair, 'v')
	if got := Hash128(pair); got != (Uint128{0xe59a95169d83bccd, 0xb235597881f894d1}) {
		t.Errorf("pair hash = %016x %016x", got.Lo, got.Hi)
	}
}
