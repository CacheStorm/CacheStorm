//go:build compat

package compat

import "encoding/hex"

func restoreCases() []Case {
	raw, err := hex.DecodeString("00086100620d0a2c3d260c0079fc5b6578887069")
	if err != nil {
		panic(err)
	}
	payload := string(raw)
	return []Case{
		k("restore", "bitmap-setbit-keeps-expiry", c("SET", "k", "A", "PXAT", "4102444800000"), c("SETBIT", "k", "0", "1"), c("GET", "k"), c("PEXPIRETIME", "k")),
		k("restore", "bitmap-bitfield-keeps-expiry", c("SET", "k", "A", "PXAT", "4102444800000"), c("BITFIELD", "k", "SET", "u8", "0", "42"), c("GET", "k"), c("PEXPIRETIME", "k")),
		k("restore", "relative-ttl", c("RESTORE", "k", "60000", payload), c("GET", "k"), ttl(1500, "PTTL", "k")),
		k("restore", "permanent", c("RESTORE", "k", "0", payload), c("GET", "k"), c("PTTL", "k")),
		k("restore", "absolute-max", c("RESTORE", "k", "9223372036854775807", payload, "ABSTTL"), c("GET", "k"), c("PEXPIRETIME", "k")),
		k("restore", "absolute-past", c("RESTORE", "k", "1", payload, "ABSTTL"), c("GET", "k"), c("EXISTS", "k")),
		k("restore", "replace", c("SET", "k", "old"), c("RESTORE", "k", "0", payload, "REPLACE"), c("GET", "k")),
		k("restore", "replace-past", c("SET", "k", "old"), c("RESTORE", "k", "1", payload, "REPLACE", "ABSTTL"), c("GET", "k")),
		k("restore", "busy", c("SET", "k", "old"), c("RESTORE", "k", "0", payload), c("GET", "k")),
		k("restore", "checksum-preserves", c("SET", "k", "old"), c("RESTORE", "k", "0", payload[:len(payload)-1], "REPLACE"), c("GET", "k")),
		k("restore", "negative-ttl", c("RESTORE", "k", "-1", payload), c("EXISTS", "k")),
		k("restore", "invalid-integer", c("RESTORE", "k", "invalid", payload), c("RESTORE", "k", "01", payload)),
		k("restore", "invalid-options", c("RESTORE", "k", "0", payload, "UNKNOWN"), c("RESTORE", "k", "0", payload, "IDLETIME"), c("RESTORE", "k", "0", payload, "IDLETIME", "0", "FREQ", "1"), c("EXISTS", "k")),
		k("restore", "invalid-idletime", c("RESTORE", "k", "0", payload, "IDLETIME", "-1"), c("RESTORE", "k", "0", payload, "IDLETIME", "invalid")),
		k("restore", "invalid-freq", c("RESTORE", "k", "0", payload, "FREQ", "-1"), c("RESTORE", "k", "0", payload, "FREQ", "256"), c("RESTORE", "k", "0", payload, "FREQ", "invalid")),
		k("restore", "idletime", c("RESTORE", "k", "0", payload, "IDLETIME", "30"), ttl(1, "OBJECT", "IDLETIME", "k")),
		k("restore", "freq-ignored-without-lfu", c("RESTORE", "k", "0", payload, "FREQ", "30"), c("GET", "k")),
		k("restore", "duplicate-options", c("RESTORE", "k", "0", payload, "REPLACE", "REPLACE", "ABSTTL", "ABSTTL"), c("GET", "k")),
		k("restore", "overflow-relative-ttl", c("RESTORE", "k", "9223372036854775807", payload), c("EXISTS", "k")),
	}
}
