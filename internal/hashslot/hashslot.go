// Package hashslot maps keys to the 16384 Redis Cluster hash slots: CRC16
// (XMODEM) of the key, or of its {hash tag}, modulo 16384.
package hashslot

// Count is the number of hash slots.
const Count = 16384

var table [256]uint16

func init() {
	for i := range table {
		crc := uint16(i) << 8
		for j := 0; j < 8; j++ {
			if crc&0x8000 != 0 {
				crc = crc<<1 ^ 0x1021
			} else {
				crc <<= 1
			}
		}
		table[i] = crc
	}
}

// CRC16 is the CRC16-CCITT (XMODEM) checksum Redis Cluster uses.
func CRC16(s string) uint16 {
	var crc uint16
	for i := 0; i < len(s); i++ {
		crc = crc<<8 ^ table[byte(crc>>8)^s[i]]
	}
	return crc
}

// Key returns the slot of key. When the key holds a non-empty {...} section,
// only the part between the first '{' and the next '}' is hashed.
func Key(key string) int {
	for i := 0; i < len(key); i++ {
		if key[i] != '{' {
			continue
		}
		for j := i + 1; j < len(key); j++ {
			if key[j] == '}' {
				if j > i+1 {
					key = key[i+1 : j]
				}
				return int(CRC16(key) & (Count - 1))
			}
		}
		break
	}
	return int(CRC16(key) & (Count - 1))
}
