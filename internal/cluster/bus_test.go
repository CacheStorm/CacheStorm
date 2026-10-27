package cluster

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"net"
	"strings"
	"testing"
	"time"
)

func TestMessageRoundTrip(t *testing.T) {
	m := &message{typ: msgPing, sender: otherID, currentEpoch: 9, configEpoch: 4, replOffset: 77, flags: FlagMaster,
		port: 7000, cport: 17000, tlsPort: 7443, stateOK: true, ip: "10.0.0.1", hostname: "h1",
		shardID: strings.Repeat("a", 40), masterID: strings.Repeat("b", 40)}
	setBit(m.slots[:], 0)
	setBit(m.slots[:], 16383)
	m.gossip = []gossipEntry{{id: strings.Repeat("c", 40), ip: "10.0.0.2", port: 7001, cport: 17001, flags: FlagMaster | FlagPFail, pingSent: 5, pongRecv: 6}}
	got, err := decodeMessage(m.encode())
	if err != nil {
		t.Fatal(err)
	}
	if got.sender != m.sender || got.currentEpoch != 9 || got.configEpoch != 4 || got.replOffset != 77 || got.flags != FlagMaster ||
		got.port != 7000 || got.cport != 17000 || got.tlsPort != 7443 || !got.stateOK || got.ip != "10.0.0.1" || got.hostname != "h1" ||
		got.shardID != m.shardID || got.masterID != m.masterID || got.slots != m.slots || len(got.gossip) != 1 || got.gossip[0] != m.gossip[0] {
		t.Fatalf("round trip: %+v", got)
	}

	pub := &message{typ: msgPublish, sender: otherID, channel: []byte("ch"), payload: []byte("hello")}
	if got, err := decodeMessage(pub.encode()); err != nil || string(got.channel) != "ch" || string(got.payload) != "hello" {
		t.Fatalf("publish: %v %+v", err, got)
	}
	up := &message{typ: msgUpdate, sender: otherID, about: strings.Repeat("d", 40), updateEpoch: 3}
	setBit(up.updateSlots[:], 42)
	if got, err := decodeMessage(up.encode()); err != nil || got.about != up.about || got.updateEpoch != 3 || !testBit(got.updateSlots[:], 42) {
		t.Fatalf("update: %v %+v", err, got)
	}
}

func TestDecodeRejectsBadFrames(t *testing.T) {
	good := (&message{typ: msgPing, sender: otherID}).encode()
	for name, frame := range map[string][]byte{
		"short":     good[:prefixLen+10],
		"magic":     append([]byte("XXXX"), good[4:]...),
		"version":   func() []byte { f := bytes.Clone(good); binary.BigEndian.PutUint16(f[8:], 99); return f }(),
		"sender":    func() []byte { f := bytes.Clone(good); copy(f[prefixLen:], "not-an-id"); return f }(),
		"truncated": good[:len(good)-1],
	} {
		if _, err := decodeMessage(frame); err == nil {
			t.Errorf("%s: decoded", name)
		}
	}
	huge := bytes.Clone(good)
	binary.BigEndian.PutUint32(huge[4:], maxFrameLen+1)
	if _, err := readFrame(bufio.NewReader(bytes.NewReader(huge))); err == nil {
		t.Fatal("an oversized frame was accepted")
	}
	if f, err := readFrame(bufio.NewReader(bytes.NewReader(good))); err != nil || !bytes.Equal(f, good) {
		t.Fatalf("readFrame: %v", err)
	}
}

// testBus gives c a stopped bus, enough for the packet logic.
func testBus(c *Cluster) *bus {
	b := &bus{timeout: 1000, inbound: map[*link]struct{}{}}
	c.bus = b
	return b
}

func TestGossipBlacklistAndFailureReports(t *testing.T) {
	c := New("", Options{RequireFullCoverage: true})
	b := testBus(c)
	peer := &Node{ID: otherID, IP: "127.0.0.1", Port: 7001, CPort: 17001, Flags: FlagMaster}
	third := strings.Repeat("3", 40)
	c.mu.Lock()
	c.addNodeLocked(peer)
	c.mu.Unlock()
	in := newLink(nil, true)
	m := &message{typ: msgPing, sender: otherID, flags: FlagMaster, port: 7001, cport: 17001, ip: "127.0.0.1",
		gossip: []gossipEntry{{id: third, ip: "127.0.0.1", port: 7002, cport: 17002, flags: FlagMaster}}}

	if err := c.Forget(third); err == nil {
		t.Fatal("FORGET of an unknown node succeeded")
	}
	c.blacklist = map[string]int64{third: nowMs() + 60000}
	c.processPacket(b, in, m)
	if len(c.nodes) != 2 {
		t.Fatalf("a blacklisted node was added: %d nodes", len(c.nodes))
	}
	if err := c.Forget(third); err != nil {
		t.Fatalf("FORGET of a blacklisted node: %v", err)
	}
	c.blacklist[third] = nowMs() - 1
	c.processPacket(b, in, m)
	if c.nodes[third] == nil {
		t.Fatal("gossip did not add the node once the blacklist entry expired")
	}

	// A PFAIL node becomes FAIL once a majority of the masters report it.
	c.mu.Lock()
	c.addSlotLocked(c.myself, 0)
	c.addSlotLocked(peer, 1)
	c.addSlotLocked(c.nodes[third], 2)
	c.nodes[third].Flags |= FlagPFail
	c.updateStateLocked()
	c.mu.Unlock()
	m.gossip[0].flags |= FlagPFail
	c.processPacket(b, in, m)
	if n, _ := c.CountFailureReports(third); n != 1 {
		t.Fatalf("failure reports: %d", n)
	}
	if c.nodes[third].Flags&FlagFail == 0 || c.Healthy() {
		t.Fatalf("flags %s, healthy %v", c.nodes[third].Flags, c.Healthy())
	}
	c.nodes[third].failReports[peer] = nowMs() - 3000
	if n, _ := c.CountFailureReports(third); n != 0 {
		t.Fatalf("an expired failure report counted: %d", n)
	}
}

func TestUpdateSlotsConfigByEpoch(t *testing.T) {
	c := New("", Options{})
	testBus(c)
	peer := &Node{ID: otherID, Flags: FlagMaster, ConfigEpoch: 2}
	c.mu.Lock()
	c.addNodeLocked(peer)
	c.myself.ConfigEpoch = 3
	c.addSlotLocked(c.myself, 10)
	var claim [Slots / 8]byte
	setBit(claim[:], 10)
	setBit(claim[:], 11)
	c.updateSlotsConfigWithLocked(peer, 2, &claim)
	if c.owner[10] != c.myself || c.owner[11] != peer {
		t.Fatalf("lower epoch: owners %v %v", c.owner[10], c.owner[11])
	}
	c.updateSlotsConfigWithLocked(peer, 4, &claim)
	if c.owner[10] != peer {
		t.Fatal("a higher config epoch did not win the slot")
	}
	var none [Slots / 8]byte
	c.updateSlotsConfigWithLocked(peer, 4, &none)
	if c.owner[10] != peer || !c.unclaimedLocked(10) {
		t.Fatal("a slot its owner stopped claiming must stay bound but unclaimed")
	}
	c.mu.Unlock()
}

func TestBusMeetOverTCP(t *testing.T) {
	start := func() *Cluster {
		c := New("", Options{})
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		p := ln.Addr().(*net.TCPAddr).Port
		c.SetMyself("127.0.0.1", p-1, p, 0, "")
		if err := c.StartBus(BusOptions{Listener: ln, NodeTimeout: 300 * time.Millisecond}); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(c.StopBus)
		return c
	}
	a, b := start(), start()
	_, cport, _ := net.SplitHostPort(a.BusAddr())
	var port int
	for _, ch := range cport {
		port = port*10 + int(ch-'0')
	}
	if err := b.Meet("127.0.0.1", port-1, port); err != nil {
		t.Fatal(err)
	}
	if err := b.Meet("localhost", 1, 2); err == nil {
		t.Fatal("MEET accepted a host name")
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Count(a.Nodes(), " connected") == 2 && strings.Count(b.Nodes(), " connected") == 2 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("no handshake:\n%s\n%s", a.Nodes(), b.Nodes())
}

func TestBusMeetMyself(t *testing.T) {
	c := New("", Options{})
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	p := ln.Addr().(*net.TCPAddr).Port
	c.SetMyself("127.0.0.1", p-1, p, 0, "")
	if err := c.StartBus(BusOptions{Listener: ln, NodeTimeout: 300 * time.Millisecond}); err != nil {
		t.Fatal(err)
	}
	defer c.StopBus()
	if err := c.Meet("127.0.0.1", p-1, p); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for strings.Count(c.Nodes(), "\n") != 1 {
		if time.Now().After(deadline) {
			t.Fatalf("meeting myself left nodes behind:\n%s", c.Nodes())
		}
		time.Sleep(20 * time.Millisecond)
	}
}
