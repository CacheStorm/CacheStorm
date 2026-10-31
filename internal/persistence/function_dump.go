package persistence

import (
	"encoding/binary"
	"errors"
)

// Redis' FUNCTION DUMP payload holds each library's code as an RDB string
// after the RDB_OPCODE_FUNCTION2 opcode, followed by the RDB version and the
// checksum, as a key's DUMP payload is.
const (
	rdbOpcodeFunction2     = 245
	rdbOpcodeFunctionPreGA = 246
)

// EncodeFunctionDump returns the FUNCTION DUMP payload of the libraries
// whose code is codes.
func EncodeFunctionDump(codes []string) []byte {
	var out []byte
	for _, code := range codes {
		out = append(out, rdbOpcodeFunction2)
		out = appendDumpLength(out, uint64(len(code)))
		out = append(out, code...)
	}
	out = binary.LittleEndian.AppendUint16(out, 11)
	return binary.LittleEndian.AppendUint64(out, rdbChecksum(0, out))
}

// DecodeFunctionDump returns the library codes of a FUNCTION DUMP payload,
// failing with Redis' FUNCTION RESTORE errors.
func DecodeFunctionDump(payload []byte) ([]string, error) {
	if len(payload) < 10 {
		return nil, ErrDumpChecksum
	}
	version := binary.LittleEndian.Uint16(payload[len(payload)-10:])
	if version > 12 || binary.LittleEndian.Uint64(payload[len(payload)-8:]) != rdbChecksum(0, payload[:len(payload)-8]) {
		return nil, ErrDumpChecksum
	}
	d := &dumpDecoder{data: payload[:len(payload)-10]}
	var codes []string
	for d.pos < len(d.data) {
		switch d.byte() {
		case rdbOpcodeFunction2:
		case rdbOpcodeFunctionPreGA:
			return nil, errors.New("ERR Pre-GA function format not supported")
		default:
			return nil, errors.New("ERR given type is not a function")
		}
		code := d.str()
		if d.err != nil {
			return nil, errors.New("ERR failed loading the given functions payload")
		}
		codes = append(codes, code)
	}
	return codes, nil
}
