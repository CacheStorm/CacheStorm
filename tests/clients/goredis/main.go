// Command goredis exercises CacheStorm with go-redis v9 over RESP2 and RESP3:
// HELLO, AUTH, SELECT, pipelines, MULTI, Pub/Sub, EVAL, BLPOP and SSCAN, and
// with CACHESTORM_CLUSTER_ADDR set, ClusterClient against a cluster-mode node;
// CACHESTORM_CLUSTER_LOAD runs ClusterClient traffic until a stop file appears.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

func main() {
	if os.Getenv("CACHESTORM_ADDR") != "" {
		for _, proto := range []int{2, 3} {
			check(proto)
		}
	}
	if addr := os.Getenv("CACHESTORM_CLUSTER_LOAD"); addr != "" {
		clusterLoad(addr, os.Getenv("CACHESTORM_LOAD_STOP"))
		return
	}
	if addr := os.Getenv("CACHESTORM_CLUSTER_ADDR"); addr != "" {
		for _, proto := range []int{2, 3} {
			checkCluster(addr, proto)
		}
	}
}

func check(proto int) {
	ctx := context.Background()
	rdb := redis.NewClient(&redis.Options{Addr: os.Getenv("CACHESTORM_ADDR"), Password: os.Getenv("CACHESTORM_PASSWORD"), DB: 2, Protocol: proto})
	defer rdb.Close()
	fail := func(step string, err error) { fmt.Println("FAIL", "protocol", proto, step, err); os.Exit(1) }
	if err := rdb.Set(ctx, "k", "v", time.Minute).Err(); err != nil {
		fail("set", err)
	}
	if v, err := rdb.Get(ctx, "k").Result(); err != nil || v != "v" {
		fail("get", err)
	}
	if _, err := rdb.Get(ctx, "missing").Result(); err != redis.Nil {
		fail("nil", err)
	}
	pipe := rdb.Pipeline()
	pipe.Incr(ctx, "n")
	pipe.HSet(ctx, "h", "a", 1, "b", 2)
	all := pipe.HGetAll(ctx, "h")
	if _, err := pipe.Exec(ctx); err != nil {
		fail("pipeline", err)
	}
	if m := all.Val(); m["a"] != "1" || m["b"] != "2" {
		fail("hgetall", fmt.Errorf("%v", m))
	}
	if _, err := rdb.TxPipelined(ctx, func(p redis.Pipeliner) error { p.LPush(ctx, "l", "x"); p.LLen(ctx, "l"); return nil }); err != nil {
		fail("multi", err)
	}
	sub := rdb.Subscribe(ctx, "ch")
	if _, err := sub.Receive(ctx); err != nil {
		fail("subscribe", err)
	}
	rdb.Publish(ctx, "ch", "hello")
	msg, err := sub.ReceiveMessage(ctx)
	if err != nil || msg.Payload != "hello" {
		fail("pubsub", err)
	}
	if v, err := rdb.Eval(ctx, "return redis.call('GET', KEYS[1])", []string{"k"}).Result(); err != nil || v != "v" {
		fail("eval", err)
	}
	go func() { time.Sleep(200 * time.Millisecond); rdb.RPush(ctx, "q", "job") }()
	if v, err := rdb.BLPop(ctx, 2*time.Second, "q").Result(); err != nil || v[1] != "job" {
		fail("blpop", err)
	}
	var cursor uint64
	n := 0
	for i := 0; i < 100; i++ {
		rdb.SAdd(ctx, "s", i)
	}
	for {
		keys, next, err := rdb.SScan(ctx, "s", cursor, "", 10).Result()
		if err != nil {
			fail("sscan", err)
		}
		n += len(keys)
		if cursor = next; cursor == 0 {
			break
		}
	}
	if n != 100 {
		fail("sscan count", fmt.Errorf("%d", n))
	}
	rdb.Del(ctx, "s", "l", "q", "n", "h")
	fmt.Printf("go-redis v9 protocol=%d: all checks passed\n", proto)
}
