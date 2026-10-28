package command

import "math/rand"

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
