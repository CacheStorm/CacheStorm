package command

import (
	"bytes"
	"strconv"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

type pubsubClientX struct {
	t   *testing.T
	s   *store.Store
	sub *store.Subscriber
}

func (c *pubsubClientX) run(handler func(*Context) error, args ...string) string {
	c.t.Helper()
	var buf bytes.Buffer
	argv := make([][]byte, len(args))
	for i, a := range args {
		argv[i] = []byte(a)
	}
	ctx := NewContext("X", argv, c.s, resp.NewWriter(&buf))
	ctx.Subscriber = c.sub
	if err := handler(ctx); err != nil {
		c.t.Fatalf("handler error: %v", err)
	}
	ctx.Writer.Flush()
	c.sub = ctx.Subscriber
	return buf.String()
}

func ackX(kind, name string, n int) string {
	nameEnc := "$-1\r\n"
	if name != "\x00nil" {
		nameEnc = "$" + strconv.Itoa(len(name)) + "\r\n" + name + "\r\n"
	}
	return "*3\r\n$" + strconv.Itoa(len(kind)) + "\r\n" + kind + "\r\n" + nameEnc + ":" + strconv.Itoa(n) + "\r\n"
}

func expectX(t *testing.T, label, got, want string) {
	t.Helper()
	if got != want {
		t.Fatalf("%s:\n got  %q\n want %q", label, got, want)
	}
}

func TestPubSubAcksMatchRedis(t *testing.T) {
	s := store.NewStore()
	c := &pubsubClientX{t: t, s: s}

	expectX(t, "SUBSCRIBE a b", c.run(cmdSUBSCRIBE, "a", "b"),
		ackX("subscribe", "a", 1)+ackX("subscribe", "b", 2))
	expectX(t, "PSUBSCRIBE p*", c.run(cmdPSUBSCRIBE, "p*"), ackX("psubscribe", "p*", 3))
	expectX(t, "SSUBSCRIBE s", c.run(cmdSSUBSCRIBE, "s"), ackX("ssubscribe", "s", 1))
	expectX(t, "UNSUBSCRIBE a", c.run(cmdUNSUBSCRIBE, "a"), ackX("unsubscribe", "a", 2))
	expectX(t, "UNSUBSCRIBE (all)", c.run(cmdUNSUBSCRIBE), ackX("unsubscribe", "b", 1))
	expectX(t, "UNSUBSCRIBE (none left)", c.run(cmdUNSUBSCRIBE), ackX("unsubscribe", "\x00nil", 1))
	expectX(t, "PUNSUBSCRIBE (all)", c.run(cmdPUNSUBSCRIBE), ackX("punsubscribe", "p*", 0))
	expectX(t, "SUNSUBSCRIBE x s", c.run(cmdSUNSUBSCRIBE, "x", "s"),
		ackX("sunsubscribe", "x", 1)+ackX("sunsubscribe", "s", 0))
	expectX(t, "SUNSUBSCRIBE (none left)", c.run(cmdSUNSUBSCRIBE), ackX("sunsubscribe", "\x00nil", 0))
	if c.sub.TotalSubscriptions() != 0 {
		t.Fatal("subscriber should hold nothing")
	}
}

func TestPubSubUnsubscribeWithoutSubscriber(t *testing.T) {
	s := store.NewStore()
	c := &pubsubClientX{t: t, s: s}
	expectX(t, "UNSUBSCRIBE x", c.run(cmdUNSUBSCRIBE, "x"), ackX("unsubscribe", "x", 0))
	expectX(t, "PUNSUBSCRIBE", c.run(cmdPUNSUBSCRIBE), ackX("punsubscribe", "\x00nil", 0))
	if c.sub != nil {
		t.Fatal("unsubscribing must not create a subscriber")
	}
}

func TestPubSubPublishQueuesFrames(t *testing.T) {
	s := store.NewStore()
	subscriber := &pubsubClientX{t: t, s: s}
	publisher := &pubsubClientX{t: t, s: s}

	subscriber.run(cmdSUBSCRIBE, "news.a")
	subscriber.run(cmdPSUBSCRIBE, "news.*")
	subscriber.run(cmdSSUBSCRIBE, "sh")

	expectX(t, "PUBLISH", publisher.run(cmdPUBLISH, "news.a", "x"), ":2\r\n")
	expectX(t, "SPUBLISH", publisher.run(cmdSPUBLISH, "sh", "y"), ":1\r\n")
	expectX(t, "SPUBLISH to plain channel", publisher.run(cmdSPUBLISH, "news.a", "z"), ":0\r\n")

	want := []string{
		"*3\r\n$7\r\nmessage\r\n$6\r\nnews.a\r\n$1\r\nx\r\n",
		"*4\r\n$8\r\npmessage\r\n$6\r\nnews.*\r\n$6\r\nnews.a\r\n$1\r\nx\r\n",
		"*3\r\n$8\r\nsmessage\r\n$2\r\nsh\r\n$1\r\ny\r\n",
	}
	for i, w := range want {
		select {
		case f := <-subscriber.sub.Channel():
			expectX(t, "frame", string(f), w)
		default:
			t.Fatalf("frame %d missing", i)
		}
	}
	select {
	case f := <-subscriber.sub.Channel():
		t.Fatalf("unexpected frame %q", f)
	default:
	}
}

func TestPubSubIntrospectionShard(t *testing.T) {
	s := store.NewStore()
	a := &pubsubClientX{t: t, s: s}
	b := &pubsubClientX{t: t, s: s}
	a.run(cmdSUBSCRIBE, "a1", "a2")
	a.run(cmdSSUBSCRIBE, "s1")
	b.run(cmdSSUBSCRIBE, "s1", "s2")
	b.run(cmdPSUBSCRIBE, "p*", "q*")

	q := &pubsubClientX{t: t, s: s}
	expectX(t, "CHANNELS", q.run(cmdPUBSUB, "CHANNELS"), "*2\r\n$2\r\na1\r\n$2\r\na2\r\n")
	expectX(t, "CHANNELS a1*", q.run(cmdPUBSUB, "channels", "*1"), "*1\r\n$2\r\na1\r\n")
	expectX(t, "NUMSUB", q.run(cmdPUBSUB, "NUMSUB", "a1", "s1"), "*4\r\n$2\r\na1\r\n:1\r\n$2\r\ns1\r\n:0\r\n")
	expectX(t, "NUMSUB none", q.run(cmdPUBSUB, "NUMSUB"), "*0\r\n")
	expectX(t, "NUMPAT", q.run(cmdPUBSUB, "NUMPAT"), ":2\r\n")
	expectX(t, "SHARDCHANNELS", q.run(cmdPUBSUB, "SHARDCHANNELS"), "*2\r\n$2\r\ns1\r\n$2\r\ns2\r\n")
	expectX(t, "SHARDCHANNELS s2", q.run(cmdPUBSUB, "SHARDCHANNELS", "s[2]"), "*1\r\n$2\r\ns2\r\n")
	expectX(t, "SHARDNUMSUB", q.run(cmdPUBSUB, "SHARDNUMSUB", "s1", "s2", "a1"),
		"*6\r\n$2\r\ns1\r\n:2\r\n$2\r\ns2\r\n:1\r\n$2\r\na1\r\n:0\r\n")
	expectX(t, "unknown", q.run(cmdPUBSUB, "BOGUS"), "-ERR unknown subcommand 'BOGUS'. Try PUBSUB HELP.\r\n")
}
