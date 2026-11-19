package command

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

// Replies Redis 8.2 gives, step by step.
func TestVectorSetRedisReplies(t *testing.T) {
	s := store.NewStore()
	r := NewRouter()
	RegisterVectorSetCommands(r)
	RegisterStringCommands(r)
	for _, tc := range []struct {
		want string
		args []string
	}{
		{":1\r\n", []string{"VADD", "vg", "VALUES", "3", "1", "2", "3", "a", "SETATTR", `{"x":1}`}},
		{":1\r\n", []string{"VADD", "vg", "VALUES", "3", "-1", "0.5", "2", "b"}},
		{":1\r\n", []string{"VADD", "vg", "VALUES", "3", "0", "0", "-4", "c", "SETATTR", `{"x":5}`}},
		{bulks("0.992125928401947", "2.007874011993408", "3"), []string{"VEMB", "vg", "a"}},
		{"*4\r\n+int8\r\n$3\r\n*U\x7f\r\n$18\r\n3.7416574954986572\r\n$18\r\n0.8017836809158325\r\n", []string{"VEMB", "vg", "a", "RAW"}},
		{bulks("a", "0.962909996509552", "b", "0.690470278263092", "c", "0.21132487058639526"), []string{"VSIM", "vg", "VALUES", "3", "1", "1", "1", "TRUTH", "WITHSCORES"}},
		{bulks("c", "0.21132487058639526"), []string{"VSIM", "vg", "VALUES", "3", "1", "1", "1", "TRUTH", "WITHSCORES", "FILTER", ".x > 2"}},
		{"*6\r\n$1\r\nb\r\n$18\r\n0.9994409680366516\r\n$-1\r\n$1\r\na\r\n$18\r\n0.8519260883331299\r\n$7\r\n{\"x\":1}\r\n", []string{"VSIM", "vg", "ELE", "b", "TRUTH", "WITHSCORES", "WITHATTRIBS", "COUNT", "2"}},
		{errReply("ERR syntax error in FILTER expression near: .x >"), []string{"VSIM", "vg", "VALUES", "3", "1", "1", "1", "FILTER", ".x >"}},
		{":3\r\n", []string{"VDIM", "vg"}},
		{":3\r\n", []string{"VCARD", "vg"}},
		{"$7\r\n{\"x\":5}\r\n", []string{"VGETATTR", "vg", "c"}},
		{":1\r\n", []string{"VSETATTR", "vg", "c", ""}},
		{"$-1\r\n", []string{"VGETATTR", "vg", "c"}},
		{":1\r\n", []string{"VISMEMBER", "vg", "c"}},
		{":1\r\n", []string{"VREM", "vg", "c"}},
		{":0\r\n", []string{"VREM", "vg", "c"}},
		{errReply("ERR Vector dimension mismatch - got 2 but set has 3"), []string{"VADD", "vg", "VALUES", "2", "1", "2", "x"}},
		{errReply("ERR asked quantization mismatch with existing vector set"), []string{"VADD", "vg", "VALUES", "3", "1", "2", "3", "x", "BIN"}},
		{errReply("ERR asked M value mismatch with existing vector set"), []string{"VADD", "vg", "VALUES", "3", "1", "2", "3", "x", "M", "8"}},
		{errReply("ERR invalid vector specification"), []string{"VADD", "vg", "VALUES", "abc", "1", "x"}},
		{errReply("ERR invalid option after element"), []string{"VADD", "vg", "VALUES", "1", "1", "x", "BOGUS"}},
		{errReply("ERR invalid EF"), []string{"VADD", "vg", "VALUES", "1", "1", "x", "EF", "0"}},
		{errReply("ERR invalid M"), []string{"VADD", "vg", "VALUES", "1", "1", "x", "M", "3"}},
		{errReply("ERR invalid COUNT"), []string{"VSIM", "vg", "VALUES", "3", "1", "1", "1", "COUNT", "0"}},
		{errReply("ERR syntax error in VSIM command"), []string{"VSIM", "vg", "VALUES", "3", "1", "1", "1", "BOGUS"}},
		{errReply("ERR element not found in set"), []string{"VSIM", "vg", "ELE", "nosuch"}},
		{"*0\r\n", []string{"VSIM", "none", "VALUES", "1", "1"}},
		{errReply("ERR key does not exist"), []string{"VDIM", "none"}},
		{":0\r\n", []string{"VCARD", "none"}},
		{"$-1\r\n", []string{"VEMB", "none", "x"}},
		{"*-1\r\n", []string{"VINFO", "none"}},
		{"$-1\r\n", []string{"VRANDMEMBER", "none"}},
		{"*0\r\n", []string{"VRANDMEMBER", "none", "3"}},
		{errReply("ERR COUNT value is not an integer"), []string{"VRANDMEMBER", "vg", "abc"}},
		{"+OK\r\n", []string{"SET", "str", "x"}},
		{errReply("WRONGTYPE Operation against a key holding the wrong kind of value"), []string{"VADD", "str", "VALUES", "1", "1", "x"}},
		{":1\r\n", []string{"VADD", "vr", "REDUCE", "2", "VALUES", "4", "1", "2", "3", "4", "a"}},
		{errReply("ERR Vector dimension mismatch - got 4 but set has 2"), []string{"VADD", "vr", "REDUCE", "3", "VALUES", "4", "1", "1", "1", "1", "c"}},
		{":1\r\n", []string{"VADD", "vz", "VALUES", "3", "0", "0", "0", "z"}},
		{"*4\r\n+int8\r\n$3\r\n\x00\x00\x00\r\n$1\r\n1\r\n$1\r\n0\r\n", []string{"VEMB", "vz", "z", "RAW"}},
	} {
		if got := runCmd(t, s, r, tc.args[0], tc.args[1:]...); got != tc.want {
			t.Errorf("%v\n got %q\nwant %q", tc.args, got, tc.want)
		}
	}
}
