package command

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestEncodingCommandsBSFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterEncodingCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"BSON.ENCODE map", "BSON.ENCODE", [][]byte{[]byte(`{"key":"value","num":123}`)}},
		{"BSON.ENCODE array", "BSON.ENCODE", [][]byte{[]byte(`[1,2,3,4,5]`)}},
		{"BSON.ENCODE invalid", "BSON.ENCODE", [][]byte{[]byte(`invalid json`)}},
		{"BSON.ENCODE no args", "BSON.ENCODE", nil},
		{"BSON.DECODE valid", "BSON.DECODE", [][]byte{[]byte{0x05, 0x00, 0x00, 0x00, 0x00}}},
		{"BSON.DECODE invalid", "BSON.DECODE", [][]byte{[]byte(`invalid bson`)}},
		{"BSON.DECODE no args", "BSON.DECODE", nil},
		{"CBOR.ENCODE", "CBOR.ENCODE", [][]byte{[]byte(`{"key":"value"}`)}},
		{"CBOR.ENCODE no args", "CBOR.ENCODE", nil},
		{"CBOR.DECODE valid", "CBOR.DECODE", [][]byte{[]byte{0xbf, 0xff}}},
		{"CBOR.DECODE invalid", "CBOR.DECODE", [][]byte{[]byte(`invalid cbor`)}},
		{"CBOR.DECODE no args", "CBOR.DECODE", nil},
		{"MSGPACK.ENCODE", "MSGPACK.ENCODE", [][]byte{[]byte(`{"key":"value"}`)}},
		{"MSGPACK.ENCODE no args", "MSGPACK.ENCODE", nil},
		{"MSGPACK.DECODE valid", "MSGPACK.DECODE", [][]byte{[]byte{0x81, 0xa3, 0x6b, 0x65, 0x79, 0xa5, 0x76, 0x61, 0x6c, 0x75, 0x65}}},
		{"MSGPACK.DECODE invalid header", "MSGPACK.DECODE", [][]byte{[]byte{0x00}}},
		{"MSGPACK.DECODE incomplete", "MSGPACK.DECODE", [][]byte{[]byte{0xda, 0x00, 0x10}}},
		{"MSGPACK.DECODE no args", "MSGPACK.DECODE", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestEncodingCommandsTOMLFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterEncodingCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"TOML.ENCODE valid", "TOML.ENCODE", [][]byte{[]byte(`{"title":"Test","owner":{"name":"John"}}`)}},
		{"TOML.ENCODE no args", "TOML.ENCODE", nil},
		{"TOML.DECODE valid", "TOML.DECODE", [][]byte{[]byte(`title = "Test"`)}},
		{"TOML.DECODE invalid", "TOML.DECODE", [][]byte{[]byte(`invalid toml [[[`)}},
		{"TOML.DECODE no args", "TOML.DECODE", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestEncodingCommandsMSGPACKFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterEncodingCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"MSGPACK.ENCODE simple", "MSGPACK.ENCODE", [][]byte{[]byte(`{"key":"value"}`)}},
		{"MSGPACK.ENCODE nested", "MSGPACK.ENCODE", [][]byte{[]byte(`{"outer":{"inner":"value"}}`)}},
		{"MSGPACK.ENCODE array", "MSGPACK.ENCODE", [][]byte{[]byte(`[1,2,3,4,5]`)}},
		{"MSGPACK.ENCODE no args", "MSGPACK.ENCODE", nil},
		{"MSGPACK.DECODE simple", "MSGPACK.DECODE", [][]byte{[]byte{0x81, 0xa3, 0x6b, 0x65, 0x79, 0xa5, 0x76, 0x61, 0x6c, 0x75, 0x65}}},
		{"MSGPACK.DECODE empty", "MSGPACK.DECODE", [][]byte{[]byte{}}},
		{"MSGPACK.DECODE invalid", "MSGPACK.DECODE", [][]byte{[]byte{0x01, 0x02, 0x03}}},
		{"MSGPACK.DECODE no args", "MSGPACK.DECODE", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}
