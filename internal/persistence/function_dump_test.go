package persistence

import (
	"encoding/binary"
	"encoding/hex"
	"testing"
)

func TestFunctionDumpReadsRedisPayloads(t *testing.T) {
	// FUNCTION DUMP of Redis 7.4 holding one library.
	payload, _ := hex.DecodeString("f5406023216c7561206e616d653d6d69670a72656469732e72656769737465725f66756e6374696f6e28276d66272c2066756e6374696f6e286b6579732c2061726773292072657475726e202768656c6c6f2066726f6d2072656469732720656e64290c00cb04a93d934dafd6")
	codes, err := DecodeFunctionDump(payload)
	want := "#!lua name=mig\nredis.register_function('mf', function(keys, args) return 'hello from redis' end)"
	if err != nil || len(codes) != 1 || codes[0] != want {
		t.Fatalf("DecodeFunctionDump = %q, %v", codes, err)
	}
	round, err := DecodeFunctionDump(EncodeFunctionDump([]string{want, "b"}))
	if err != nil || len(round) != 2 || round[0] != want || round[1] != "b" {
		t.Fatalf("round trip = %q, %v", round, err)
	}
	if codes, err := DecodeFunctionDump(EncodeFunctionDump(nil)); err != nil || len(codes) != 0 {
		t.Fatalf("empty payload = %q, %v", codes, err)
	}
	bad := append([]byte{0}, payload[1:]...)
	if _, err := DecodeFunctionDump(bad); err != ErrDumpChecksum {
		t.Fatalf("corrupt payload error = %v", err)
	}
	for opcode, msg := range map[byte]string{rdbOpcodeFunctionPreGA: "ERR Pre-GA function format not supported", 0: "ERR given type is not a function"} {
		body := EncodeFunctionDump([]string{"x"})
		body = body[:len(body)-8]
		body[0] = opcode
		p := binary.LittleEndian.AppendUint64(body, rdbChecksum(0, body))
		if _, err := DecodeFunctionDump(p); err == nil || err.Error() != msg {
			t.Fatalf("opcode %d: error %v, want %s", opcode, err, msg)
		}
	}
}
