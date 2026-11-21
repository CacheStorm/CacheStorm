package server

import (
	"strconv"
	"testing"
	"time"
)

// Redis 8.8's INCREX, with the replies of its t_string.c.
func TestINCREX(t *testing.T) {
	s := startReplServerCfg(t, replTestConfig(t, nil))
	c := dialRepl(t, s)
	expect := func(want string, args ...string) {
		t.Helper()
		if got := c.str(args...); got != want {
			t.Fatalf("%v = %q, want %q", args, got, want)
		}
	}
	expect("[1 1]", "INCREX", "n")
	expect("[11 10]", "INCREX", "n", "BYINT", "10")
	expect("[11 0]", "INCREX", "n", "BYINT", "10", "UBOUND", "20")
	expect("11", "GET", "n")
	expect("[20 9]", "INCREX", "n", "BYINT", "10", "UBOUND", "20", "SATURATE")
	expect("[0 -20]", "INCREX", "n", "BYINT", "-100", "LBOUND", "0", "SATURATE")
	expect("[0 0]", "INCREX", "n", "BYINT", "-1", "LBOUND", "0")
	c.do("SET", "big", strconv.FormatInt(1<<62, 10))
	expect("[4611686018427387904 0]", "INCREX", "big", "BYINT", "4611686018427387904")
	expect("[9223372036854775807 4611686018427387903]", "INCREX", "big", "BYINT", "4611686018427387904", "SATURATE")
	c.do("SET", "neg", "-9223372036854775808")
	expect("ERR:ERR applied increment would overflow", "INCREX", "neg", "BYINT", "1", "LBOUND", "9223372036854775807", "SATURATE")

	expect("[1.5 1.5]", "INCREX", "f", "BYFLOAT", "1.5")
	expect("[1.8 0.3]", "INCREX", "f", "BYFLOAT", "0.3")
	expect("[1.8 0]", "INCREX", "f", "BYFLOAT", "10", "UBOUND", "5")
	expect("[5 3.2]", "INCREX", "f", "BYFLOAT", "10", "UBOUND", "5", "SATURATE")

	// Expiry.
	expect("[1 1]", "INCREX", "e", "EX", "100")
	if ttl, _ := strconv.Atoi(c.str("TTL", "e")); ttl <= 0 || ttl > 100 {
		t.Fatalf("TTL e = %d", ttl)
	}
	expect("[2 1]", "INCREX", "e")
	if ttl, _ := strconv.Atoi(c.str("TTL", "e")); ttl <= 0 {
		t.Fatalf("INCREX without an expiry option lost the TTL: %d", ttl)
	}
	expect("[3 1]", "INCREX", "e", "EX", "1000", "ENX")
	if ttl, _ := strconv.Atoi(c.str("TTL", "e")); ttl > 100 {
		t.Fatalf("ENX replaced the TTL: %d", ttl)
	}
	expect("[4 1]", "INCREX", "e", "PERSIST")
	expect("-1", "TTL", "e")
	expect("[5 1]", "INCREX", "e", "PXAT", strconv.FormatInt(time.Now().Add(-time.Second).UnixMilli(), 10))
	expect("0", "EXISTS", "e")

	// Errors.
	expect("ERR:ERR syntax error", "INCREX", "x", "BYINT", "1", "BYFLOAT", "1")
	expect("ERR:ERR syntax error", "INCREX", "x", "EX", "1", "PX", "1")
	expect("ERR:ERR syntax error", "INCREX", "x", "PERSIST", "ENX")
	expect("ERR:ERR Increment is not an integer or out of range", "INCREX", "x", "BYINT", "1.5")
	expect("ERR:ERR Increment is not a valid float", "INCREX", "x", "BYFLOAT", "abc")
	expect("ERR:ERR BYFLOAT increment cannot be Infinity", "INCREX", "x", "BYFLOAT", "inf")
	expect("ERR:ERR LBOUND is not an integer or out of range", "INCREX", "x", "LBOUND", "1.5")
	expect("ERR:ERR UBOUND is not a valid float", "INCREX", "x", "BYFLOAT", "1", "UBOUND", "z")
	expect("ERR:ERR LBOUND can't be greater than UBOUND", "INCREX", "x", "LBOUND", "5", "UBOUND", "1")
	expect("ERR:ERR ENX flag requires an expiration", "INCREX", "x", "ENX")
	expect("ERR:ERR invalid expire time in 'increx' command", "INCREX", "x", "EX", "0")
	c.do("SET", "s", "abc")
	expect("ERR:ERR value is not an integer or out of range", "INCREX", "s")
	expect("ERR:ERR value is not a valid float", "INCREX", "s", "BYFLOAT", "1")
	c.do("SET", "inf", "inf")
	expect("ERR:ERR value cannot be Infinity", "INCREX", "inf", "BYFLOAT", "1")
	c.do("RPUSH", "l", "x")
	expect("ERR:WRONGTYPE Operation against a key holding the wrong kind of value", "INCREX", "l")
	expect("0", "EXISTS", "x")
}
