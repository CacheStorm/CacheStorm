package command

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestDigestCommandsCRYPTOFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterDigestCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"CRYPTO.HASH sha256", "CRYPTO.HASH", [][]byte{[]byte("sha256"), []byte("test data")}},
		{"CRYPTO.HASH md5", "CRYPTO.HASH", [][]byte{[]byte("md5"), []byte("test data")}},
		{"CRYPTO.HASH sha1", "CRYPTO.HASH", [][]byte{[]byte("sha1"), []byte("test data")}},
		{"CRYPTO.HASH sha512", "CRYPTO.HASH", [][]byte{[]byte("sha512"), []byte("test data")}},
		{"CRYPTO.HASH blake2b", "CRYPTO.HASH", [][]byte{[]byte("blake2b"), []byte("test data")}},
		{"CRYPTO.HASH blake2s", "CRYPTO.HASH", [][]byte{[]byte("blake2s"), []byte("test data")}},
		{"CRYPTO.HASH unknown", "CRYPTO.HASH", [][]byte{[]byte("unknown"), []byte("test data")}},
		{"CRYPTO.HASH no args", "CRYPTO.HASH", nil},
		{"CRYPTO.HMAC sha256", "CRYPTO.HMAC", [][]byte{[]byte("sha256"), []byte("key"), []byte("test data")}},
		{"CRYPTO.HMAC md5", "CRYPTO.HMAC", [][]byte{[]byte("md5"), []byte("key"), []byte("test data")}},
		{"CRYPTO.HMAC no args", "CRYPTO.HMAC", nil},
		{"BASE64.ENCODE", "BASE64.ENCODE", [][]byte{[]byte("test data")}},
		{"BASE64.ENCODE no args", "BASE64.ENCODE", nil},
		{"BASE64.DECODE valid", "BASE64.DECODE", [][]byte{[]byte("dGVzdCBkYXRh")}},
		{"BASE64.DECODE invalid", "BASE64.DECODE", [][]byte{[]byte("!!!invalid!!!")}},
		{"BASE64.DECODE no args", "BASE64.DECODE", nil},
		{"BASE64.URLENCODE", "BASE64.URLENCODE", [][]byte{[]byte("test data")}},
		{"BASE64.URLENCODE no args", "BASE64.URLENCODE", nil},
		{"BASE64.URLDECODE valid", "BASE64.URLDECODE", [][]byte{[]byte("dGVzdCBkYXRh")}},
		{"BASE64.URLDECODE no args", "BASE64.URLDECODE", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestDigestCommandsCHECKSUMFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterDigestCommands(router)

	// Setup test data
	s.Set("key1", &store.StringValue{Data: []byte("value1")}, store.SetOptions{})
	s.Set("key2", &store.StringValue{Data: []byte("value2")}, store.SetOptions{})
	s.Set("hash1", &store.HashValue{Fields: map[string][]byte{"field1": []byte("value1")}}, store.SetOptions{})
	s.Set("list1", &store.ListValue{Elements: [][]byte{[]byte("item1"), []byte("item2")}}, store.SetOptions{})
	s.Set("set1", &store.SetValue{Members: map[string]struct{}{"member1": {}}}, store.SetOptions{})

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"CHECKSUM.KEY string", "CHECKSUM.KEY", [][]byte{[]byte("key1")}},
		{"CHECKSUM.KEY hash", "CHECKSUM.KEY", [][]byte{[]byte("hash1")}},
		{"CHECKSUM.KEY list", "CHECKSUM.KEY", [][]byte{[]byte("list1")}},
		{"CHECKSUM.KEY set", "CHECKSUM.KEY", [][]byte{[]byte("set1")}},
		{"CHECKSUM.KEY not found", "CHECKSUM.KEY", [][]byte{[]byte("notfound")}},
		{"CHECKSUM.KEY no args", "CHECKSUM.KEY", nil},
		{"CHECKSUM.DB", "CHECKSUM.DB", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}

func TestDigestCommandsHashFullCoverage(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterDigestCommands(router)

	tests := []struct {
		name string
		cmd  string
		args [][]byte
	}{
		{"HASH.MURMUR3", "HASH.MURMUR3", [][]byte{[]byte("test data")}},
		{"HASH.MURMUR3 no args", "HASH.MURMUR3", nil},
		{"HASH.FNV", "HASH.FNV", [][]byte{[]byte("test data")}},
		{"HASH.FNV no args", "HASH.FNV", nil},
		{"HASH.CRC32", "HASH.CRC32", [][]byte{[]byte("test data")}},
		{"HASH.CRC32 no args", "HASH.CRC32", nil},
		{"HASH.CRC64", "HASH.CRC64", [][]byte{[]byte("test data")}},
		{"HASH.CRC64 no args", "HASH.CRC64", nil},
		{"HASH.XXHASH", "HASH.XXHASH", [][]byte{[]byte("test data")}},
		{"HASH.XXHASH no args", "HASH.XXHASH", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCommandTest(t, router, s, tt.cmd, tt.args)
		})
	}
}
