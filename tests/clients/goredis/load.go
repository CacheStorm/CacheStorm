package main

import (
	"context"
	"fmt"
	"math/rand/v2"
	"os"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// clusterLoad writes and reads through a ClusterClient until the file stop
// names exists, then checks every key it wrote. Any error, or a value that
// differs from the last one written, fails it: during a resharding the
// client must follow MOVED, ASK and TRYAGAIN without losing anything.
func clusterLoad(addr, stop string) {
	ctx := context.Background()
	rdb := redis.NewClusterClient(&redis.ClusterOptions{
		Addrs:           []string{addr},
		Password:        os.Getenv("CACHESTORM_PASSWORD"),
		MaxRedirects:    16,
		MaxRetries:      10,
		MinRetryBackoff: 20 * time.Millisecond,
		MaxRetryBackoff: 200 * time.Millisecond,
	})
	defer rdb.Close()
	expect := map[string]string{}
	counters := map[string]int64{}
	var keys []string
	writes, reads, errs := 0, 0, 0
	fail := func(what string, err error) {
		errs++
		if errs <= 20 {
			fmt.Println("load error:", what, err)
		}
	}
	const maxKeys = 20000
	for i := 0; ; i++ {
		if i%100 == 0 {
			if _, err := os.Stat(stop); err == nil {
				break
			}
		}
		key := "load:" + strconv.Itoa(i%maxKeys)
		val := "v" + strconv.Itoa(i)
		if err := rdb.Set(ctx, key, val, 0).Err(); err != nil {
			fail("SET "+key, err)
		} else {
			if _, ok := expect[key]; !ok {
				keys = append(keys, key)
			}
			expect[key] = val
			writes++
		}
		if len(keys) > 0 {
			k := keys[rand.IntN(len(keys))]
			got, err := rdb.Get(ctx, k).Result()
			if err != nil {
				fail("GET "+k, err)
			} else if got != expect[k] {
				fail("GET "+k, fmt.Errorf("got %q, want %q", got, expect[k]))
			}
			reads++
		}
		c := "cnt:" + strconv.Itoa(i%50)
		if err := rdb.Incr(ctx, c).Err(); err != nil {
			fail("INCR "+c, err)
		} else {
			counters[c]++
		}
		if i%10 == 0 {
			tag := "{tag" + strconv.Itoa(i%97) + "}"
			if err := rdb.MSet(ctx, tag+"a", val, tag+"b", val).Err(); err != nil {
				fail("MSET "+tag, err)
			} else {
				for _, k := range []string{tag + "a", tag + "b"} {
					if _, ok := expect[k]; !ok {
						keys = append(keys, k)
					}
					expect[k] = val
				}
				vals, err := rdb.MGet(ctx, tag+"a", tag+"b").Result()
				if err != nil {
					fail("MGET "+tag, err)
				} else if vals[0] != val || vals[1] != val {
					fail("MGET "+tag, fmt.Errorf("got %v, want %q", vals, val))
				}
			}
		}
	}
	missing := 0
	for k, want := range expect {
		got, err := rdb.Get(ctx, k).Result()
		if err != nil || got != want {
			missing++
			fail("final GET "+k, fmt.Errorf("got %q (%v), want %q", got, err, want))
		}
	}
	for c, want := range counters {
		got, err := rdb.Get(ctx, c).Int64()
		if err != nil || got != want {
			fail("final GET "+c, fmt.Errorf("got %d (%v), want %d", got, err, want))
		}
	}
	fmt.Printf("cluster load: %d writes, %d reads, %d keys and %d counters checked, %d missing or wrong, %d errors\n",
		writes, reads, len(expect), len(counters), missing, errs)
	if errs > 0 {
		os.Exit(1)
	}
}
