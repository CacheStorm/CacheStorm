package cluster

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strings"
)

// The cluster bus frame: a fixed prefix of the magic, the total frame length
// (prefix included), the protocol version, the message type and message
// flags, then the sender header and the body of the type. Integers are big
// endian; a string is a one-byte length and its bytes, an ID is 40 bytes
// (all zeros for none) and a blob is a four-byte length and its bytes.
const (
	busMagic      = "CSCB"
	busVersion    = 1
	prefixLen     = 14
	maxFrameLen   = 16 << 20
	maxGossip     = 4096
	directReadLen = 64 << 10
)

type msgType uint16

// The message types, numbered as Redis numbers its own.
const (
	msgPing msgType = iota
	msgPong
	msgMeet
	msgFail
	msgPublish
	msgFailoverAuthRequest
	msgFailoverAuthAck
	msgUpdate
	msgMFStart
	msgModule
	msgPublishShard
	msgTypeCount
)

var msgTypeNames = [msgTypeCount]string{"ping", "pong", "meet", "fail", "publish", "auth-req", "auth-ack", "update", "mfstart", "module", "publishshard"}

type gossipEntry struct {
	id       string
	ip       string
	port     int
	cport    int
	tlsPort  int
	flags    Flags
	pingSent int64
	pongRecv int64
}

// message is a decoded bus frame. The header fields describe the sender;
// replOffset and masterID are for replicas.
type message struct {
	typ          msgType
	mflags       uint16
	sender       string
	currentEpoch uint64
	configEpoch  uint64
	replOffset   uint64
	flags        Flags
	port         int
	cport        int
	tlsPort      int
	stateOK      bool
	ip           string
	hostname     string
	shardID      string
	masterID     string
	slots        [Slots / 8]byte

	gossip      []gossipEntry
	about       string
	updateEpoch uint64
	updateSlots [Slots / 8]byte
	channel     []byte
	payload     []byte
}

type encoder struct{ b []byte }

func (e *encoder) u8(v uint8)   { e.b = append(e.b, v) }
func (e *encoder) u16(v int)    { e.b = binary.BigEndian.AppendUint16(e.b, uint16(v)) }
func (e *encoder) u32(v int)    { e.b = binary.BigEndian.AppendUint32(e.b, uint32(v)) }
func (e *encoder) u64(v uint64) { e.b = binary.BigEndian.AppendUint64(e.b, v) }
func (e *encoder) str(s string) {
	if len(s) > 255 {
		s = s[:255]
	}
	e.u8(uint8(len(s)))
	e.b = append(e.b, s...)
}
func (e *encoder) id(s string) {
	var b [IDLen]byte
	copy(b[:], s)
	e.b = append(e.b, b[:]...)
}
func (e *encoder) blob(b []byte) {
	e.u32(len(b))
	e.b = append(e.b, b...)
}

// encode returns the frame of m.
func (m *message) encode() []byte {
	e := &encoder{b: make([]byte, 0, 2300+len(m.gossip)*80+len(m.channel)+len(m.payload))}
	e.b = append(e.b, busMagic...)
	e.u32(0)
	e.u16(busVersion)
	e.u16(int(m.typ))
	e.u16(int(m.mflags))
	e.id(m.sender)
	e.u64(m.currentEpoch)
	e.u64(m.configEpoch)
	e.u64(m.replOffset)
	e.u16(int(m.flags))
	e.u16(m.port)
	e.u16(m.cport)
	e.u16(m.tlsPort)
	if m.stateOK {
		e.u8(1)
	} else {
		e.u8(0)
	}
	e.str(m.ip)
	e.str(m.hostname)
	e.id(m.shardID)
	e.id(m.masterID)
	e.b = append(e.b, m.slots[:]...)
	switch m.typ {
	case msgPing, msgPong, msgMeet:
		e.u16(len(m.gossip))
		for _, g := range m.gossip {
			e.id(g.id)
			e.str(g.ip)
			e.u16(g.port)
			e.u16(g.cport)
			e.u16(g.tlsPort)
			e.u16(int(g.flags))
			e.u64(uint64(g.pingSent))
			e.u64(uint64(g.pongRecv))
		}
	case msgFail:
		e.id(m.about)
	case msgUpdate:
		e.id(m.about)
		e.u64(m.updateEpoch)
		e.b = append(e.b, m.updateSlots[:]...)
	case msgPublish, msgPublishShard:
		e.blob(m.channel)
		e.blob(m.payload)
	}
	binary.BigEndian.PutUint32(e.b[4:], uint32(len(e.b)))
	return e.b
}

var errShortFrame = errors.New("cluster bus: truncated frame")

type decoder struct {
	b   []byte
	err error
}

func (d *decoder) take(n int) []byte {
	if d.err != nil || n > len(d.b) {
		d.err = errShortFrame
		return make([]byte, n)
	}
	out := d.b[:n]
	d.b = d.b[n:]
	return out
}
func (d *decoder) u8() uint8   { return d.take(1)[0] }
func (d *decoder) u16() int    { return int(binary.BigEndian.Uint16(d.take(2))) }
func (d *decoder) u32() int    { return int(binary.BigEndian.Uint32(d.take(4))) }
func (d *decoder) u64() uint64 { return binary.BigEndian.Uint64(d.take(8)) }
func (d *decoder) str() string { return string(d.take(int(d.u8()))) }
func (d *decoder) id() string {
	b := d.take(IDLen)
	if b[0] == 0 {
		return ""
	}
	return strings.ToLower(string(b))
}
func (d *decoder) blob() []byte {
	n := d.u32()
	if n > len(d.b) {
		d.err = errShortFrame
		return nil
	}
	return append([]byte(nil), d.take(n)...)
}

// decodeMessage parses a whole frame.
func decodeMessage(frame []byte) (*message, error) {
	if len(frame) < prefixLen || string(frame[:4]) != busMagic {
		return nil, errors.New("cluster bus: bad magic")
	}
	if v := binary.BigEndian.Uint16(frame[8:]); v != busVersion {
		return nil, fmt.Errorf("cluster bus: unsupported protocol version %d", v)
	}
	m := &message{typ: msgType(binary.BigEndian.Uint16(frame[10:])), mflags: binary.BigEndian.Uint16(frame[12:])}
	d := &decoder{b: frame[prefixLen:]}
	m.sender = d.id()
	m.currentEpoch = d.u64()
	m.configEpoch = d.u64()
	m.replOffset = d.u64()
	m.flags = Flags(d.u16())
	m.port, m.cport, m.tlsPort = d.u16(), d.u16(), d.u16()
	m.stateOK = d.u8() == 1
	m.ip = d.str()
	m.hostname = d.str()
	m.shardID = d.id()
	m.masterID = d.id()
	copy(m.slots[:], d.take(Slots/8))
	switch m.typ {
	case msgPing, msgPong, msgMeet:
		n := d.u16()
		if n > maxGossip {
			return nil, errors.New("cluster bus: gossip section too long")
		}
		for i := 0; i < n && d.err == nil; i++ {
			g := gossipEntry{id: d.id(), ip: d.str()}
			g.port, g.cport, g.tlsPort = d.u16(), d.u16(), d.u16()
			g.flags = Flags(d.u16())
			g.pingSent, g.pongRecv = int64(d.u64()), int64(d.u64())
			m.gossip = append(m.gossip, g)
		}
	case msgFail:
		m.about = d.id()
	case msgUpdate:
		m.about = d.id()
		m.updateEpoch = d.u64()
		copy(m.updateSlots[:], d.take(Slots/8))
	case msgPublish, msgPublishShard:
		m.channel = d.blob()
		m.payload = d.blob()
	}
	if d.err != nil {
		return nil, d.err
	}
	if !ValidID(m.sender) {
		return nil, errors.New("cluster bus: invalid sender ID")
	}
	return m, nil
}

// readFrame reads one frame, refusing one longer than maxFrameLen. A long
// frame's buffer grows as its bytes arrive rather than being allocated from
// the announced length.
func readFrame(r *bufio.Reader) ([]byte, error) {
	head, err := r.Peek(8)
	if err != nil {
		return nil, err
	}
	if string(head[:4]) != busMagic {
		return nil, errors.New("cluster bus: bad magic")
	}
	n := int(binary.BigEndian.Uint32(head[4:]))
	if n < prefixLen || n > maxFrameLen {
		return nil, fmt.Errorf("cluster bus: invalid frame length %d", n)
	}
	if n <= directReadLen {
		frame := make([]byte, n)
		_, err := io.ReadFull(r, frame)
		return frame, err
	}
	var buf bytes.Buffer
	if _, err := io.CopyN(&buf, r, int64(n)); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
