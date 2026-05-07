package benchmarks

import (
	"strconv"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func runStoreDelete(n int, s *store.Store, keys []string) {
	for i := 0; i < n; i++ {
		s.Delete(keys[i])
	}
}

func runStoreDeleteBatch(keys []string, s *store.Store) {
	s.DeleteBatch(keys)
}

func BenchmarkStoreDelete100(b *testing.B) {
	s := store.NewStore()
	tag := "bench-tag"
	keys := make([]string, 100)

	for i := 0; i < 100; i++ {
		key := "key:" + strconv.Itoa(i)
		keys[i] = key
		s.Set(key, &store.StringValue{Data: []byte("data")}, store.SetOptions{Tags: []string{tag}})
	}

	var result bool
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < 100; j++ {
			result = s.Delete(keys[j])
		}
	}
	_ = result
}

func BenchmarkStoreDeleteBatch100(b *testing.B) {
	s := store.NewStore()
	tag := "bench-tag"
	keys := make([]string, 100)

	for i := 0; i < 100; i++ {
		key := "key:" + strconv.Itoa(i)
		keys[i] = key
		s.Set(key, &store.StringValue{Data: []byte("data")}, store.SetOptions{Tags: []string{tag}})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.DeleteBatch(keys)
	}
}

func BenchmarkStoreDelete1000(b *testing.B) {
	s := store.NewStore()
	tag := "bench-tag"
	keys := make([]string, 1000)

	for i := 0; i < 1000; i++ {
		key := "key:" + strconv.Itoa(i)
		keys[i] = key
		s.Set(key, &store.StringValue{Data: []byte("data")}, store.SetOptions{Tags: []string{tag}})
	}

	var result int
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < 1000; j++ {
			s.Delete(keys[j])
		}
		result = 1
	}
	_ = result
}

func BenchmarkStoreDeleteBatch1000(b *testing.B) {
	s := store.NewStore()
	tag := "bench-tag"
	keys := make([]string, 1000)

	for i := 0; i < 1000; i++ {
		key := "key:" + strconv.Itoa(i)
		keys[i] = key
		s.Set(key, &store.StringValue{Data: []byte("data")}, store.SetOptions{Tags: []string{tag}})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.DeleteBatch(keys)
	}
}

func BenchmarkStoreDelete10000(b *testing.B) {
	s := store.NewStore()
	tag := "bench-tag"
	keys := make([]string, 10000)

	for i := 0; i < 10000; i++ {
		key := "key:" + strconv.Itoa(i)
		keys[i] = key
		s.Set(key, &store.StringValue{Data: []byte("data")}, store.SetOptions{Tags: []string{tag}})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < 10000; j++ {
			s.Delete(keys[j])
		}
	}
}

func BenchmarkStoreDeleteBatch10000(b *testing.B) {
	s := store.NewStore()
	tag := "bench-tag"
	keys := make([]string, 10000)

	for i := 0; i < 10000; i++ {
		key := "key:" + strconv.Itoa(i)
		keys[i] = key
		s.Set(key, &store.StringValue{Data: []byte("data")}, store.SetOptions{Tags: []string{tag}})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.DeleteBatch(keys)
	}
}
