package benchmarks

import (
	"strconv"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func BenchmarkTagAdd(b *testing.B) {
	ti := store.NewTagIndex()
	tags := []string{"tag1", "tag2", "tag3"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := "key:" + strconv.Itoa(i)
		ti.AddTags(key, tags)
	}
}

func BenchmarkTagInvalidate(b *testing.B) {
	ti := store.NewTagIndex()
	tag := "benchmark-tag"

	for i := 0; i < 10000; i++ {
		key := "key:" + strconv.Itoa(i)
		ti.AddTags(key, []string{tag})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ti.Invalidate(tag)

		for j := 0; j < 10000; j++ {
			key := "key:" + strconv.Itoa(j)
			ti.AddTags(key, []string{tag})
		}
	}
}

func BenchmarkTagGetKeys(b *testing.B) {
	ti := store.NewTagIndex()
	tag := "benchmark-tag"

	for i := 0; i < 1000; i++ {
		key := "key:" + strconv.Itoa(i)
		ti.AddTags(key, []string{tag})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ti.GetKeys(tag)
	}
}

func BenchmarkTagCount(b *testing.B) {
	ti := store.NewTagIndex()
	tag := "benchmark-tag"

	for i := 0; i < 1000; i++ {
		key := "key:" + strconv.Itoa(i)
		ti.AddTags(key, []string{tag})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ti.Count(tag)
	}
}
