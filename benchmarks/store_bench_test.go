package benchmarks

import (
	"strconv"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func BenchmarkSet(b *testing.B) {
	s := store.NewStore()
	value := &store.StringValue{Data: []byte("benchmark-value")}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := "key:" + strconv.Itoa(i)
		s.Set(key, value, store.SetOptions{})
	}
}

func BenchmarkGet(b *testing.B) {
	s := store.NewStore()

	for i := 0; i < 10000; i++ {
		key := "key:" + strconv.Itoa(i)
		s.Set(key, &store.StringValue{Data: []byte("value")}, store.SetOptions{})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := "key:" + strconv.Itoa(i%10000)
		s.Get(key)
	}
}

func BenchmarkGetParallel(b *testing.B) {
	s := store.NewStore()

	for i := 0; i < 10000; i++ {
		key := "key:" + strconv.Itoa(i)
		s.Set(key, &store.StringValue{Data: []byte("value")}, store.SetOptions{})
	}

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := "key:" + strconv.Itoa(i%10000)
			s.Get(key)
			i++
		}
	})
}

func BenchmarkSetParallel(b *testing.B) {
	s := store.NewStore()
	value := &store.StringValue{Data: []byte("benchmark-value")}

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := "key:" + strconv.Itoa(i)
			s.Set(key, value, store.SetOptions{})
			i++
		}
	})
}

func BenchmarkDelete(b *testing.B) {
	s := store.NewStore()

	for i := 0; i < b.N; i++ {
		key := "key:" + strconv.Itoa(i)
		s.Set(key, &store.StringValue{Data: []byte("value")}, store.SetOptions{})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := "key:" + strconv.Itoa(i)
		s.Delete(key)
	}
}
