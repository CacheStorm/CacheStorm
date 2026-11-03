package command

import (
	"math/rand"

	"github.com/cachestorm/cachestorm/internal/store"
)

// randomKeyWindow is how many consecutive keys randomKey draws from, as
// Redis' dictGetFairRandomKey samples a handful of keys and picks one.
const randomKeyWindow = 16

// randomKey returns a key of m chosen like Redis' dictGetFairRandomKey:
// iteration starts at a random point of the map, and one of the next
// randomKeyWindow keys is picked at random, so the cost does not grow with
// the map.
func randomKey[V any](m map[string]V) (string, bool) {
	var window [randomKeyWindow]string
	n := 0
	for k := range m {
		window[n] = k
		n++
		if n == len(window) {
			break
		}
	}
	if n == 0 {
		return "", false
	}
	return window[rand.Intn(n)], true
}

// randomKeys picks keys of m for a random-member reply, as Redis does for
// SRANDMEMBER, HRANDFIELD and ZRANDMEMBER: |count| keys with repetition for
// a negative count; for a positive one distinct keys, all of them in random
// order when count covers the map, otherwise sampled one by one when count
// is small next to the map and drawn from a shuffled copy when it is not.
func randomKeys[V any](m map[string]V, count int64) []string {
	n := int64(len(m))
	if n == 0 || count == 0 {
		return nil
	}
	if count < 0 {
		reps := -count
		if reps > maxRandRepeats {
			reps = maxRandRepeats
		}
		out := make([]string, reps)
		for i := range out {
			out[i], _ = randomKey(m)
		}
		return out
	}
	if count*3 > n {
		all := make([]string, 0, n)
		for k := range m {
			all = append(all, k)
		}
		rand.Shuffle(len(all), func(i, j int) { all[i], all[j] = all[j], all[i] })
		if count < n {
			all = all[:count]
		}
		return all
	}
	out := make([]string, 0, count)
	seen := make(map[string]struct{}, count)
	for int64(len(out)) < count {
		k, _ := randomKey(m)
		if _, dup := seen[k]; dup {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, k)
	}
	return out
}

// randomIndexes is randomKeys for a collection of n items held in order.
func randomIndexes(n int, count int64) []int {
	if n == 0 || count == 0 {
		return nil
	}
	if count < 0 {
		reps := min(-count, maxRandRepeats)
		out := make([]int, reps)
		for i := range out {
			out[i] = rand.Intn(n)
		}
		return out
	}
	if count*3 > int64(n) {
		all := rand.Perm(n)
		if count < int64(n) {
			all = all[:count]
		}
		return all
	}
	out := make([]int, 0, count)
	seen := make(map[int]struct{}, count)
	for int64(len(out)) < count {
		k := rand.Intn(n)
		if _, dup := seen[k]; dup {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, k)
	}
	return out
}

// pickKeys is randomKeys for a collection held either as dict or, when dict
// is nil, as the items all lists.
func pickKeys[V any](dict map[string]V, all func() []string, count int64) []string {
	if dict != nil {
		return randomKeys(dict, count)
	}
	items := all()
	idx := randomIndexes(len(items), count)
	out := make([]string, len(idx))
	for i, j := range idx {
		out[i] = items[j]
	}
	return out
}

// pickKey is randomKey for a collection held as pickKeys' is.
func pickKey[V any](dict map[string]V, all func() []string) (string, bool) {
	if dict != nil {
		return randomKey(dict)
	}
	items := all()
	if len(items) == 0 {
		return "", false
	}
	return items[rand.Intn(len(items))], true
}

func setItems(set *store.SetValue) func() []string {
	return func() []string {
		out := make([]string, 0, set.Card())
		set.Range(func(m string) bool {
			out = append(out, m)
			return true
		})
		return out
	}
}

func hashItems(hash *store.HashValue) func() []string {
	return func() []string {
		out := make([]string, 0, hash.Len())
		hash.Range(func(f string, _ []byte) bool {
			out = append(out, f)
			return true
		})
		return out
	}
}
