package store

import "encoding/binary"

// intset holds a set of integers as Redis' intset does: sorted, every member
// at the smallest width (2, 4 or 8 bytes) that fits all of them, with no
// spare capacity. Byte 0 is the width; the members follow little-endian.
// A change builds a new buffer, so members read from the old one stay valid
// and a small set never carries slack.
type intset []byte

func intsetWidth(n int64) int {
	switch {
	case n >= -1<<15 && n < 1<<15:
		return 2
	case n >= -1<<31 && n < 1<<31:
		return 4
	}
	return 8
}

// Len is the number of members.
func (s intset) Len() int {
	if len(s) == 0 {
		return 0
	}
	return (len(s) - 1) / int(s[0])
}

// at returns the i-th smallest member.
func (s intset) at(i int) int64 {
	w := int(s[0])
	b := s[1+i*w:]
	switch w {
	case 2:
		return int64(int16(binary.LittleEndian.Uint16(b)))
	case 4:
		return int64(int32(binary.LittleEndian.Uint32(b)))
	}
	return int64(binary.LittleEndian.Uint64(b))
}

// search returns where n is, or would be inserted, and whether it is there.
func (s intset) search(n int64) (int, bool) {
	lo, hi := 0, s.Len()
	for lo < hi {
		m := int(uint(lo+hi) >> 1)
		if s.at(m) < n {
			lo = m + 1
		} else {
			hi = m
		}
	}
	return lo, lo < s.Len() && s.at(lo) == n
}

func putIntsetMember(b []byte, w int, n int64) {
	switch w {
	case 2:
		binary.LittleEndian.PutUint16(b, uint16(n))
	case 4:
		binary.LittleEndian.PutUint32(b, uint32(n))
	default:
		binary.LittleEndian.PutUint64(b, uint64(n))
	}
}

// insert returns s with n added at position i (from search), widening every
// member when n needs more bytes than they have.
func (s intset) insert(i int, n int64) intset {
	count := s.Len()
	w := intsetWidth(n)
	if count > 0 && int(s[0]) > w {
		w = int(s[0])
	}
	out := make(intset, 1+(count+1)*w)
	out[0] = byte(w)
	for j := 0; j < i; j++ {
		putIntsetMember(out[1+j*w:], w, s.at(j))
	}
	putIntsetMember(out[1+i*w:], w, n)
	for j := i; j < count; j++ {
		putIntsetMember(out[1+(j+1)*w:], w, s.at(j))
	}
	return out
}

// remove returns s without its i-th member; like Redis it keeps the width.
func (s intset) remove(i int) intset {
	count := s.Len()
	if count <= 1 {
		return nil
	}
	w := int(s[0])
	out := make(intset, 1+(count-1)*w)
	out[0] = byte(w)
	copy(out[1:], s[1:1+i*w])
	copy(out[1+i*w:], s[1+(i+1)*w:])
	return out
}
