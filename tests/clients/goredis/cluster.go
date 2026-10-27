package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// checkCluster runs ClusterClient against a single cluster-mode node, giving
// it every slot first when it has none.
func checkCluster(addr string, proto int) {
	ctx := context.Background()
	fail := func(step string, err error) { fmt.Println("FAIL", "cluster protocol", proto, step, err); os.Exit(1) }
	password := os.Getenv("CACHESTORM_PASSWORD")
	plain := redis.NewClient(&redis.Options{Addr: addr, Password: password})
	info, err := plain.ClusterInfo(ctx).Result()
	if err != nil {
		fail("cluster info", err)
	}
	if strings.Contains(info, "cluster_slots_assigned:0\r\n") {
		if err := plain.ClusterAddSlotsRange(ctx, 0, 16383).Err(); err != nil {
			fail("addslotsrange", err)
		}
	}
	plain.Close()

	rdb := redis.NewClusterClient(&redis.ClusterOptions{Addrs: []string{addr}, Password: password, Protocol: proto})
	defer rdb.Close()
	if err := rdb.Set(ctx, "{user1}.name", "ada", time.Minute).Err(); err != nil {
		fail("set", err)
	}
	if err := rdb.Set(ctx, "{user1}.lang", "go", 0).Err(); err != nil {
		fail("set", err)
	}
	if v, err := rdb.Get(ctx, "{user1}.name").Result(); err != nil || v != "ada" {
		fail("get", err)
	}
	if v, err := rdb.MGet(ctx, "{user1}.name", "{user1}.lang").Result(); err != nil || v[0] != "ada" || v[1] != "go" {
		fail("mget", fmt.Errorf("%v %v", v, err))
	}
	if err := rdb.MGet(ctx, "a", "b").Err(); err == nil || !strings.HasPrefix(err.Error(), "CROSSSLOT") {
		fail("crossslot", err)
	}
	pipe := rdb.Pipeline()
	incr := pipe.Incr(ctx, "counter")
	pipe.HSet(ctx, "{user1}.h", "a", 1)
	get := pipe.Get(ctx, "{user1}.lang")
	other := pipe.Set(ctx, "another-slot", "x", 0)
	if _, err := pipe.Exec(ctx); err != nil {
		fail("pipeline", err)
	}
	if incr.Val() < 1 || get.Val() != "go" || other.Val() != "OK" {
		fail("pipeline replies", fmt.Errorf("%d %q %q", incr.Val(), get.Val(), other.Val()))
	}
	if _, err := rdb.TxPipelined(ctx, func(p redis.Pipeliner) error {
		p.Incr(ctx, "{user1}.n")
		p.Get(ctx, "{user1}.name")
		return nil
	}); err != nil {
		fail("multi", err)
	}
	if v, err := rdb.Eval(ctx, "return redis.call('GET', KEYS[1])", []string{"{user1}.name"}).Result(); err != nil || v != "ada" {
		fail("eval", err)
	}
	if err := rdb.ForEachShard(ctx, func(ctx context.Context, c *redis.Client) error { return c.Ping(ctx).Err() }); err != nil {
		fail("foreachshard", err)
	}
	rdb.Del(ctx, "{user1}.name", "{user1}.lang", "{user1}.h", "{user1}.n")
	rdb.Del(ctx, "counter")
	rdb.Del(ctx, "another-slot")
	fmt.Printf("go-redis v9 ClusterClient protocol=%d: all checks passed\n", proto)
}
