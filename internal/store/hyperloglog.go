package store

import (
	"encoding/binary"
	"fmt"
	"math"
	"math/bits"
	"sync"
)

const HLLRegisterCount = 1 << 14
const HLLDenseSize = 16 + HLLRegisterCount*6/8

type RedisStringValue interface{ RedisStringBytes() ([]byte, error) }

type HLLState struct {
	Registers [HLLRegisterCount]uint8
	Header    [16]byte
	Dense     bool
}

func HLLHash(data []byte) uint64 {
	const m uint64 = 0xc6a4a7935bd1e995
	h := uint64(0xadc83b19) ^ uint64(len(data))*m
	for len(data) >= 8 {
		k := binary.LittleEndian.Uint64(data)
		k *= m
		k ^= k >> 47
		k *= m
		h ^= k
		h *= m
		data = data[8:]
	}
	if len(data) > 0 {
		var tail uint64
		for i, b := range data {
			tail |= uint64(b) << uint(i*8)
		}
		h ^= tail
		h *= m
	}
	h ^= h >> 47
	h *= m
	h ^= h >> 47
	return h
}

func (h *HLLState) Add(data []byte) bool {
	hash := HLLHash(data)
	index := hash & (HLLRegisterCount - 1)
	rank := uint8(bits.TrailingZeros64((hash>>14)|(uint64(1)<<50)) + 1)
	if rank <= h.Registers[index] {
		return false
	}
	h.Registers[index] = rank
	return true
}
func (h *HLLState) Merge(other *HLLState) {
	for i, b := range &other.Registers {
		h.Registers[i] = max(h.Registers[i], b)
	}
	h.Dense = h.Dense || other.Dense
}

func hllSigma(x float64) float64 {
	if x == 1 {
		return math.Inf(1)
	}
	z, y := x, float64(1)
	for {
		x *= x
		old := z
		z += x * y
		y *= 2
		if z == old {
			return z
		}
	}
}
func hllTau(x float64) float64 {
	if x == 0 || x == 1 {
		return 0
	}
	z, y := 1-x, float64(1)
	for {
		x = math.Sqrt(x)
		old := z
		y *= 0.5
		difference := 1 - x
		z -= difference * difference * y
		if z == old {
			return z / 3
		}
	}
}
func (h *HLLState) Count() int64 {
	var histogram [64]int
	for _, b := range &h.Registers {
		if b > 63 {
			return math.MinInt64
		}
		histogram[b]++
	}
	m := float64(HLLRegisterCount)
	z := m * hllTau((m-float64(histogram[51]))/m)
	for j := 50; j >= 1; j-- {
		z = (z + float64(histogram[j])) * 0.5
	}
	z += m * hllSigma(float64(histogram[0])/m)
	count := math.Round(0.721347520444481703680 * m * m / z)
	if math.IsNaN(count) || math.IsInf(count, 0) || count >= float64(math.MaxInt64) {
		return math.MinInt64
	}
	return int64(count)
}

func ValidateHLL(data []byte) error {
	if len(data) < 16 || string(data[:4]) != "HYLL" || data[4] > 1 || (data[4] == 0 && len(data) != HLLDenseSize) {
		return fmt.Errorf("WRONGTYPE %s", "Key is not a valid HyperLogLog string value.")
	}
	return nil
}
func CorruptHLLError() error { return fmt.Errorf("INVALIDOBJ Corrupted HLL object detected") }

func DecodeHLL(data []byte) (*HLLState, error) {
	if err := ValidateHLL(data); err != nil {
		return nil, err
	}
	h := &HLLState{Dense: data[4] == 0}
	copy(h.Header[:], data[:16])
	if h.Dense {
		payload := data[16:]
		for i := range &h.Registers {
			position := i * 6
			index, shift := position/8, uint(position%8)
			value := uint16(payload[index])
			if index+1 < len(payload) {
				value |= uint16(payload[index+1]) << 8
			}
			h.Registers[i] = uint8(value>>shift) & 63
		}
		return h, nil
	}
	index := 0
	for pos := 16; pos < len(data); {
		opcode := data[pos]
		pos++
		run := 0
		value := byte(0)
		switch {
		case opcode&128 != 0:
			value = ((opcode >> 2) & 31) + 1
			run = int(opcode&3) + 1
		case opcode&64 != 0:
			if pos >= len(data) {
				return nil, CorruptHLLError()
			}
			run = (int(opcode&63)<<8 | int(data[pos])) + 1
			pos++
		default:
			run = int(opcode&63) + 1
		}
		if run > HLLRegisterCount-index {
			return nil, CorruptHLLError()
		}
		if value != 0 {
			for j := 0; j < run; j++ {
				h.Registers[index+j] = value
			}
		}
		index += run
	}
	if index != HLLRegisterCount {
		return nil, CorruptHLLError()
	}
	return h, nil
}

func (h *HLLState) Encode(invalidate bool) ([]byte, error) {
	data := append([]byte(nil), h.Header[:]...)
	copy(data, "HYLL")
	if invalidate {
		data[15] |= 128
	}
	dense := h.Dense
	for i := 0; i < HLLRegisterCount && !dense; {
		value := h.Registers[i]
		if value > 32 {
			dense = true
			break
		}
		end := i + 1
		for end < HLLRegisterCount && h.Registers[end] == value {
			end++
		}
		run := end - i
		if value == 0 {
			if run <= 64 {
				data = append(data, byte(run-1))
			} else {
				data = append(data, 64|byte((run-1)>>8), byte(run-1))
			}
		} else {
			for run > 0 {
				n := min(run, 4)
				data = append(data, 128|((value-1)<<2)|byte(n-1))
				run -= n
			}
		}
		i = end
		if len(data) > 3000 {
			dense = true
		}
	}
	if !dense {
		data[4] = 1
		return data, nil
	}
	data = append(data[:16], make([]byte, HLLDenseSize-16)...)
	data[4] = 0
	for i, value := range &h.Registers {
		if value > 63 {
			return nil, CorruptHLLError()
		}
		position := i * 6
		index, shift := 16+position/8, uint(position%8)
		data[index] |= value << shift
		if shift > 2 {
			data[index+1] |= value >> (8 - shift)
		}
	}
	return data, nil
}

type HyperLogLogValue struct {
	Registers [HLLRegisterCount]uint8
	mu        sync.RWMutex
}

func (v *HyperLogLogValue) Type() DataType { return DataTypeString }
func (v *HyperLogLogValue) SizeOf() int64  { return HLLRegisterCount + 24 }
func (v *HyperLogLogValue) Snapshot() *HLLState {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return &HLLState{Registers: v.Registers}
}
func (v *HyperLogLogValue) RedisStringBytes() ([]byte, error) { return v.Snapshot().Encode(true) }
func (v *HyperLogLogValue) String() string {
	data, err := v.RedisStringBytes()
	if err != nil {
		return err.Error()
	}
	return string(data)
}
func (v *HyperLogLogValue) Clone() Value { return &HyperLogLogValue{Registers: v.Snapshot().Registers} }
func (v *HyperLogLogValue) Add(data []byte) bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	hash := HLLHash(data)
	index := hash & (HLLRegisterCount - 1)
	rank := uint8(bits.TrailingZeros64(hash>>14|uint64(1)<<50) + 1)
	if rank <= v.Registers[index] {
		return false
	}
	v.Registers[index] = rank
	return true
}
func (v *HyperLogLogValue) Count() int64 { return v.Snapshot().Count() }
func (v *HyperLogLogValue) Merge(other *HyperLogLogValue) {
	state := other.Snapshot()
	v.mu.Lock()
	defer v.mu.Unlock()
	for i, b := range &state.Registers {
		v.Registers[i] = max(v.Registers[i], b)
	}
}

func AddDenseHLL(data []byte, elements [][]byte) ([]byte, bool, error) {
	if err := ValidateHLL(data); err != nil {
		return nil, false, err
	}
	if data[4] != 0 {
		return nil, false, CorruptHLLError()
	}
	var next []byte
	for _, element := range elements {
		hash := HLLHash(element)
		index := int(hash & (HLLRegisterCount - 1))
		rank := uint8(bits.TrailingZeros64(hash>>14|uint64(1)<<50) + 1)
		current := data
		if next != nil {
			current = next
		}
		bit := index * 6
		position, shift := 16+bit/8, uint(bit%8)
		packed := uint16(current[position])
		if position+1 < len(current) {
			packed |= uint16(current[position+1]) << 8
		}
		old := uint8(packed>>shift) & 63
		if rank <= old {
			continue
		}
		if next == nil {
			next = append([]byte(nil), data...)
		}
		mask := uint16(63) << shift
		packed = packed&^mask | uint16(rank)<<shift
		next[position] = byte(packed)
		if shift > 2 {
			next[position+1] = byte(packed >> 8)
		}
	}
	if next == nil {
		return data, false, nil
	}
	next[15] |= 128
	return next, true, nil
}
