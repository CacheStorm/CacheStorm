// Exercises CacheStorm with ioredis: AUTH, SELECT, pipelines, MULTI,
// Pub/Sub, EVAL and BLPOP.
const Redis = require("ioredis");
const [host, port] = process.env.CACHESTORM_ADDR.split(":");
const password = process.env.CACHESTORM_PASSWORD;
const assert = require("assert");
(async () => {
  const r = new Redis({ host, port: Number(port), password, db: 3, lazyConnect: true });
  await r.connect();
  await r.set("k", "v", "EX", 60);
  assert.equal(await r.get("k"), "v");
  assert.equal(await r.get("missing"), null);
  const res = await r.pipeline().incr("n").hset("h", "a", 1, "b", 2).hgetall("h").exec();
  assert.deepEqual(res[2][1], { a: "1", b: "2" });
  const tx = await r.multi().lpush("l", "x").llen("l").exec();
  assert.equal(tx[1][1], 1);
  const sub = new Redis({ host, port: Number(port), password });
  await sub.subscribe("ch");
  const got = new Promise((res) => sub.on("message", (c, m) => res(m)));
  await r.publish("ch", "hello");
  assert.equal(await got, "hello");
  assert.equal(await r.eval("return redis.call('GET', KEYS[1])", 1, "k"), "v");
  setTimeout(() => r.duplicate().rpush("q", "job"), 200);
  const bl = await r.duplicate().blpop("q", 2);
  assert.deepEqual(bl, ["q", "job"]);
  console.log("ioredis: all checks passed");
  process.exit(0);
})().catch((e) => { console.log("FAIL", e); process.exit(1); });
