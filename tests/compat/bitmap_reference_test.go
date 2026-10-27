//go:build compat

package compat

import (
	"bytes"
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"testing"
)

func TestBitfieldReferenceSequences(t *testing.T) {
	redis := &Target{Name: "redis", Addr: startRedis(t), conns: map[int]*Conn{}}
	cs := &Target{Name: "cachestorm", Addr: startCacheStorm(t), conns: map[int]*Conn{}}
	defer redis.resetConns()
	defer cs.resetConns()
	random := rand.New(rand.NewSource(7142026))
	for trial := 0; trial < 80; trial++ {
		t.Run(fmt.Sprintf("sequence-%02d", trial), func(t *testing.T) {
			initial := make([]byte, random.Intn(10))
			_, _ = random.Read(initial)
			for _, target := range []*Target{redis, cs} {
				if r := target.adminDo("SET", "k", string(initial)); r.Err != "" || r.Val == nil || r.Val.Kind == '-' {
					t.Fatalf("setup %s: %s", target.Name, r.Display())
				}
			}
			args := []string{"BITFIELD", "k"}
			for count := random.Intn(12) + 1; count > 0; count-- {
				operation := []string{"GET", "SET", "INCRBY"}[random.Intn(3)]
				args = append(args, "OVERFLOW", []string{"WRAP", "SAT", "FAIL"}[random.Intn(3)], operation)
				signed := random.Intn(2) == 0
				width := random.Intn(63) + 1
				sign := "u"
				if signed {
					sign = "i"
					if random.Intn(4) == 0 {
						width = 64
					}
				}
				encoding := sign + strconv.Itoa(width)
				offset := strconv.Itoa(random.Intn(80))
				if random.Intn(3) == 0 {
					offset = "#" + strconv.Itoa(random.Intn(8))
				}
				args = append(args, encoding, offset)
				if operation != "GET" {
					values := []int64{0, 1, -1, math.MinInt64, math.MaxInt64, int64(random.Uint64())}
					args = append(args, strconv.FormatInt(values[random.Intn(len(values))], 10))
				}
			}
			expected, actual := redis.adminDo(args...), cs.adminDo(args...)
			if expected.Err != "" || actual.Err != "" || !bytes.Equal(normalize(Step{}, expected), normalize(Step{}, actual)) {
				t.Fatalf("args=%q\nredis=%s\ncachestorm=%s", args, expected.Display(), actual.Display())
			}
			expected, actual = redis.adminDo("GET", "k"), cs.adminDo("GET", "k")
			if expected.Err != "" || actual.Err != "" || !bytes.Equal(expected.Raw, actual.Raw) {
				t.Fatalf("args=%q\nredis bytes=%s\ncachestorm bytes=%s", args, expected.Display(), actual.Display())
			}
		})
	}
}
