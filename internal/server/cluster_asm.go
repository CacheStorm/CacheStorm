package server

import (
	"bufio"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cachestorm/cachestorm/internal/cluster"
	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/logger"
	"github.com/cachestorm/cachestorm/internal/persistence"
	"github.com/cachestorm/cachestorm/internal/resp"
)

// Redis 8.4's atomic slot migration. The destination runs CLUSTER MIGRATION
// IMPORT: it opens a main channel to the node serving the slots (CLUSTER
// SYNCSLOTS SYNC) and an RDB channel (CLUSTER SYNCSLOTS RDBCHANNEL). The
// source holds writes off for a moment to take a snapshot of the slots,
// sends it as RESTORE commands on the RDB channel and from then on copies
// every write to the slots to the main channel. The destination applies the
// snapshot, then the writes, and acknowledges how far it got; once it is
// close enough the source pauses writes, sends what is left and STREAM-EOF,
// and the destination takes the slots over with a new config epoch. When
// the source learns that from the cluster bus it resumes writes and trims
// the slots' keys.

const (
	asmImport  = "import"
	asmMigrate = "migrate"
)

// The task states, named as Redis names them.
const (
	asmNone              = "none"
	asmConnecting        = "connecting"
	asmAuthReply         = "auth-reply"
	asmCanceled          = "canceled"
	asmFailed            = "failed"
	asmCompleted         = "completed"
	asmSendHandshake     = "send-handshake"
	asmHandshakeReply    = "handshake-reply"
	asmSendSyncSlots     = "send-syncslots"
	asmSyncSlotsReply    = "syncslots-reply"
	asmInitRDBChannel    = "init-rdbchannel"
	asmAccumulateBuf     = "accumulate-buffer"
	asmReadyToStream     = "ready-to-stream"
	asmStreamingBuf      = "streaming-buffer"
	asmWaitStreamEOF     = "wait-stream-eof"
	asmTakeover          = "takeover"
	asmWaitRDBChannel    = "wait-rdbchannel"
	asmWaitBgsaveStart   = "wait-bgsave-start"
	asmSendBulkAndStream = "send-bulk-and-stream"
	asmSendStream        = "send-stream"
	asmHandoffPrep       = "handoff-prep"
	asmHandoff           = "handoff"
	asmStreamEOF         = "stream-eof"
	asmRDBRequest        = "rdbchannel-request"
	asmRDBReply          = "rdbchannel-reply"
	asmRDBTransfer       = "rdbchannel-transfer"
)

type asmTask struct {
	id       string
	op       string
	ranges   []cluster.SlotRange
	state    string
	rdbState string
	// destState is the destination's state as its ACKs report it.
	destState    string
	source, dest string
	lastErr      string
	retries      int64
	// Times in milliseconds; start and end are -1 until set.
	create, start, end, paused int64
	destSnapshotTime           int64
	destAccumApplied           int64

	// gen tells an import attempt's goroutines whether they still run the
	// current attempt.
	gen   int
	conns []net.Conn

	// The destination's copy of the main channel: the commands read but
	// not applied yet, and the bytes it applied.
	queue      []asmQueued
	queueReady chan struct{}
	ackMu      sync.Mutex
	mainConn   net.Conn
	destOffset int64

	// The source side: the writes not yet sent on the main channel and the
	// bytes queued in all.
	stream       []byte
	wake         chan struct{}
	sourceOffset int64
	lastAck      int64
	crossSlot    bool
	rdbConn      net.Conn
	streamDone   bool
}

type asmQueued struct {
	args [][]byte
	size int64
	eof  bool
	err  error
}

type asmManager struct {
	s *Server

	mu        sync.Mutex
	tasks     []*asmTask
	archived  []*asmTask
	recording atomic.Bool
	paused    bool
	// trims are slot ranges whose keys the cron deletes before it starts
	// an import, so a late trim never meets the data of a new attempt.
	trims [][]cluster.SlotRange
	runs  int64
	wake  chan struct{}
}

func newASMManager(s *Server) *asmManager {
	return &asmManager{s: s, wake: make(chan struct{}, 1)}
}

func nowMilli() int64 { return time.Now().UnixMilli() }

func (m *asmManager) cluster() *cluster.Cluster { return m.s.router.Cluster() }

func (m *asmManager) kick() {
	select {
	case m.wake <- struct{}{}:
	default:
	}
}

// run is asmCron and asmBeforeSleep: every 100 milliseconds, or when kicked.
func (m *asmManager) run(stop <-chan struct{}) {
	t := time.NewTicker(100 * time.Millisecond)
	defer t.Stop()
	for {
		tick := false
		select {
		case <-stop:
			return
		case <-t.C:
			tick = true
		case <-m.wake:
		}
		m.step(tick)
	}
}

func (m *asmManager) step(tick bool) {
	m.mu.Lock()
	trims := m.trims
	m.trims = nil
	m.mu.Unlock()
	for _, r := range trims {
		m.s.trimSlotRanges(r, true)
	}

	m.mu.Lock()
	if tick {
		m.runs++
	}
	var ack *asmTask
	var unpause bool
	_, pauseTimeout, drainTimeout, maxArchived := command.SlotMigrationSettings()
	if len(m.tasks) > 0 {
		t := m.tasks[0]
		now := nowMilli()
		switch t.op {
		case asmImport:
			switch {
			case t.state == asmNone:
				m.startImportLocked(t)
			case t.state == asmFailed && tick && m.runs%10 == 0:
				m.resetLocked(t)
				t.retries++
				m.startImportLocked(t)
			case t.state == asmWaitStreamEOF && tick:
				ack = t
			}
		case asmMigrate:
			switch {
			case t.crossSlot:
				unpause = m.cancelLocked(t, "propagating cross slot command")
			case t.state == asmSendStream && tick:
				timeout := int64(m.s.cfg.Replication.ReplTimeout) * 1000
				if timeout <= 0 {
					timeout = 60000
				}
				if now-t.lastAck > timeout {
					unpause = m.failLocked(t, "Main channel - Connection timeout")
				} else if t.destState == asmWaitStreamEOF && t.destAccumApplied != 0 &&
					now-t.destAccumApplied > max(drainTimeout, (t.destAccumApplied-t.destSnapshotTime)*2) {
					unpause = m.failLocked(t, "Sync buffer drain timeout")
				}
			case (t.state == asmHandoff || t.state == asmStreamEOF) && now-t.paused >= pauseTimeout:
				unpause = m.failLocked(t, fmt.Sprintf("Write pause timeout during slot handoff: destination did not take ownership within %d ms.", pauseTimeout))
			}
		}
	}
	for int64(len(m.archived)) > maxArchived {
		m.archived = m.archived[:len(m.archived)-1]
	}
	m.mu.Unlock()
	if unpause {
		m.s.unpauseClients()
	}
	if ack != nil {
		m.sendAck(ack)
	}
}

// validateLocked is validateImportSlotRanges: this node is a master, no
// slot is migrating or importing the legacy way, no other import overlaps
// and one node serves every slot. It returns that node's ID.
func (m *asmManager) validateLocked(ranges []cluster.SlotRange, current *asmTask) (string, error) {
	c := m.cluster()
	if c.IsReplica() {
		return "", errors.New("ERR slot migration not allowed on replica.") //nolint:staticcheck // Redis reply text
	}
	if c.AnySlotOpen() {
		return "", errors.New("ERR all slot states must be STABLE to start a slot migration task.") //nolint:staticcheck // Redis reply text
	}
	source := ""
	for _, r := range ranges {
		for _, t := range m.tasks {
			if t != current && t.op == asmImport && rangesOverlap(t.ranges, r) {
				return "", fmt.Errorf("ERR overlapping import exists for slot range: %d-%d", r.Start, r.End)
			}
		}
		for j := r.Start; j <= r.End; j++ {
			st := c.Slot(j)
			if !st.Assigned {
				return "", fmt.Errorf("ERR slot has no owner: %d", j)
			}
			if source == "" {
				source = st.Owner.ID
			} else if source != st.Owner.ID {
				return "", errors.New("ERR slots belong to different source nodes")
			}
		}
	}
	return source, nil
}

func rangesOverlap(ranges []cluster.SlotRange, r cluster.SlotRange) bool {
	for _, x := range ranges {
		if x.Start <= r.End && x.End >= r.Start {
			return true
		}
	}
	return false
}

func rangesEqual(a, b []cluster.SlotRange) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func newASMTask(id, op string, ranges []cluster.SlotRange) *asmTask {
	if id == "" {
		id = cluster.NewID()
	}
	return &asmTask{id: id, op: op, ranges: ranges, state: asmNone, rdbState: asmNone, destState: asmNone,
		create: nowMilli(), start: -1, end: -1}
}

// Import is CLUSTER MIGRATION IMPORT.
func (m *asmManager) Import(ranges []cluster.SlotRange) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	source, err := m.validateLocked(ranges, nil)
	if err != nil {
		return "", err
	}
	if source == m.cluster().MyID() {
		return "", errors.New("ERR this node is already the owner of the slot range")
	}
	var unpause bool
	if len(m.tasks) > 0 {
		cur := m.tasks[0]
		if cur.state != asmFailed {
			return "", errors.New("ERR another ASM task is already in progress")
		}
		unpause = m.cancelLocked(cur, "new import requested")
	}
	t := newASMTask("", asmImport, ranges)
	t.source, t.dest = source, m.cluster().MyID()
	m.tasks = append(m.tasks, t)
	logger.Info().Str("task", t.id).Str("source", source).Str("slots", command.SlotRangesString(ranges)).Msg("ASM: import task created")
	if unpause {
		go m.s.unpauseClients()
	}
	m.kick()
	return t.id, nil
}

// Cancel is CLUSTER MIGRATION CANCEL.
func (m *asmManager) Cancel(id, reason string) int {
	m.mu.Lock()
	n, unpause := 0, false
	for _, t := range append([]*asmTask(nil), m.tasks...) {
		if id == "" || t.id == id {
			unpause = m.cancelLocked(t, reason) || unpause
			n++
		}
	}
	m.mu.Unlock()
	if unpause {
		go m.s.unpauseClients()
	}
	return n
}

// CancelSlots cancels the tasks moving any slot of ranges.
func (m *asmManager) CancelSlots(ranges []cluster.SlotRange, reason string) {
	m.mu.Lock()
	unpause := false
	for _, t := range append([]*asmTask(nil), m.tasks...) {
		for _, r := range ranges {
			if rangesOverlap(t.ranges, r) {
				unpause = m.cancelLocked(t, reason) || unpause
				break
			}
		}
	}
	m.mu.Unlock()
	if unpause {
		go m.s.unpauseClients()
	}
}

// CancelNode cancels the tasks whose source or destination is node id.
func (m *asmManager) CancelNode(id, reason string) {
	m.mu.Lock()
	if id == m.cluster().MyID() {
		m.mu.Unlock()
		m.Cancel("", reason)
		return
	}
	unpause := false
	for _, t := range append([]*asmTask(nil), m.tasks...) {
		if t.source == id || t.dest == id {
			unpause = m.cancelLocked(t, reason) || unpause
		}
	}
	m.mu.Unlock()
	if unpause {
		go m.s.unpauseClients()
	}
}

func (m *asmManager) statusOf(t *asmTask) command.SlotMigrationStatus {
	pause := int64(0)
	if t.op == asmMigrate && t.state == asmCompleted {
		pause = t.end - t.paused
	}
	return command.SlotMigrationStatus{ID: t.id, Slots: command.SlotRangesString(t.ranges), Source: t.source, Dest: t.dest,
		Operation: t.op, State: t.state, LastError: t.lastErr, Retries: t.retries,
		CreateTime: t.create, StartTime: t.start, EndTime: t.end, WritePauseMs: pause}
}

// Status is CLUSTER MIGRATION STATUS.
func (m *asmManager) Status(id string) []command.SlotMigrationStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []command.SlotMigrationStatus
	for _, list := range [][]*asmTask{m.tasks, m.archived} {
		for _, t := range list {
			if id == "" || t.id == id {
				out = append(out, m.statusOf(t))
			}
		}
	}
	return out
}

// Info is the slot migration part of CLUSTER INFO.
func (m *asmManager) Info() string {
	m.mu.Lock()
	active := 0
	for _, t := range m.tasks {
		if t.op == asmImport || t.state != asmFailed {
			active++
		}
	}
	m.mu.Unlock()
	return fmt.Sprintf("cluster_slot_migration_active_tasks:%d\r\n"+
		"cluster_slot_migration_active_trim_running:0\r\n"+
		"cluster_slot_migration_active_trim_current_job_keys:0\r\n"+
		"cluster_slot_migration_active_trim_current_job_trimmed:0\r\n"+
		"cluster_slot_migration_stats_active_trim_started:0\r\n"+
		"cluster_slot_migration_stats_active_trim_completed:0\r\n"+
		"cluster_slot_migration_stats_active_trim_cancelled:0\r\n", active)
}

// resetLocked is asmTaskReset, before an import attempt or a migrate task
// reused for the destination's retry.
func (m *asmManager) resetLocked(t *asmTask) {
	t.gen++
	for _, c := range t.conns {
		_ = c.Close()
	}
	t.conns = nil
	t.state, t.rdbState, t.destState = asmNone, asmNone, asmNone
	t.queue, t.mainConn = nil, nil
	t.destOffset, t.sourceOffset = 0, 0
	t.stream, t.streamDone, t.crossSlot = nil, false, false
	t.rdbConn = nil
	t.paused, t.destSnapshotTime, t.destAccumApplied = 0, 0, 0
}

// failLocked is asmTaskSetFailed. It reports whether writes paused for the
// task's handoff must resume.
func (m *asmManager) failLocked(t *asmTask, reason string) bool {
	t.lastErr = fmt.Sprintf("%s (state: %s, rdb_channel_state: %s)", reason, t.state, t.rdbState)
	logger.Warn().Str("task", t.id).Str("operation", t.op).Str("slots", command.SlotRangesString(t.ranges)).Str("error", t.lastErr).Msg("ASM: task failed")
	if t.state == asmFailed {
		return false
	}
	t.gen++
	for _, c := range t.conns {
		_ = c.Close()
	}
	t.conns = nil
	unpause := false
	if t.op == asmImport {
		t.queue = nil
		t.state = asmFailed
		if !m.cluster().IsReplica() {
			m.trims = append(m.trims, t.ranges)
		}
		m.signalQueue(t)
	} else {
		if t.mainConn != nil {
			_ = t.mainConn.Close()
		}
		if t.rdbConn != nil {
			_ = t.rdbConn.Close()
		}
		t.mainConn, t.rdbConn, t.stream = nil, nil, nil
		t.state = asmFailed
		m.stopRecordingLocked()
		if m.paused {
			m.paused, unpause = false, true
		}
		m.signalWriter(t)
	}
	return unpause
}

func (m *asmManager) cancelLocked(t *asmTask, reason string) bool {
	if t.state == asmCanceled {
		return false
	}
	unpause := m.failLocked(t, "Cancelled due to "+reason)
	t.state = asmCanceled
	m.finalizeLocked(t)
	return unpause
}

// finalizeLocked archives a completed or cancelled task.
func (m *asmManager) finalizeLocked(t *asmTask) {
	t.end = nowMilli()
	for i, x := range m.tasks {
		if x == t {
			m.tasks = append(m.tasks[:i], m.tasks[i+1:]...)
			break
		}
	}
	m.archived = append([]*asmTask{t}, m.archived...)
}

func (m *asmManager) stopRecordingLocked() {
	if m.recording.Load() {
		m.recording.Store(false)
		m.s.repl.SetTap(nil)
	}
}

func (m *asmManager) signalQueue(t *asmTask) {
	if t.queueReady != nil {
		select {
		case t.queueReady <- struct{}{}:
		default:
		}
	}
}

func (m *asmManager) signalWriter(t *asmTask) {
	if t.wake != nil {
		select {
		case t.wake <- struct{}{}:
		default:
		}
	}
}

// stop cancels every task when the server stops.
func (m *asmManager) stop() {
	if m == nil {
		return
	}
	m.Cancel("", "server shutdown")
}

// --- The destination side ---------------------------------------------------

// startImportLocked is asmStartImportTask.
func (m *asmManager) startImportLocked(t *asmTask) {
	source, err := m.validateLocked(t.ranges, t)
	if err != nil {
		if m.cancelLocked(t, strings.TrimPrefix(err.Error(), "ERR ")) {
			go m.s.unpauseClients()
		}
		return
	}
	if source == m.cluster().MyID() {
		m.cancelLocked(t, "slots owned by myself now")
		return
	}
	if source != t.source {
		logger.Info().Str("task", t.id).Str("source", source).Msg("ASM: import task source node changed")
		t.source = source
	}
	t.state = asmConnecting
	t.start = nowMilli()
	t.gen++
	t.queueReady = make(chan struct{}, 1)
	gen := t.gen
	logger.Info().Str("task", t.id).Str("slots", command.SlotRangesString(t.ranges)).Msg("ASM: import task started")
	go m.runImport(t, gen)
}

// asmClient is one channel to the source: commands out, replies in.
type asmClient struct {
	conn net.Conn
	br   *bufio.Reader
}

func (c *asmClient) send(args ...string) error {
	b := make([][]byte, len(args))
	for i, a := range args {
		b[i] = []byte(a)
	}
	_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	_, err := c.conn.Write(asmAppendCommand(nil, b[0], b[1:]))
	return err
}

// reply reads one line reply, skipping the empty lines a source sends to
// keep a waiting channel alive.
func (c *asmClient) reply() (string, error) {
	for {
		line, err := c.br.ReadString('\n')
		if err != nil {
			return "", err
		}
		line = strings.TrimRight(line, "\r\n")
		if line != "" {
			return line, nil
		}
	}
}

func (m *asmManager) dial(source string) (*asmClient, error) {
	addr, _, ok := m.cluster().NodeAddr(source)
	if !ok {
		return nil, fmt.Errorf("Source node %.40s was not found", source) //nolint:staticcheck // Redis log text
	}
	endpoint := net.JoinHostPort(addr.IP, strconv.Itoa(addr.Port))
	d := net.Dialer{Timeout: 5 * time.Second}
	var conn net.Conn
	var err error
	if cfg := m.s.router.MigrateTLSConfig(); cfg != nil {
		conn, err = tls.DialWithDialer(&d, "tcp", endpoint, cfg)
	} else {
		conn, err = d.Dial("tcp", endpoint)
	}
	if err != nil {
		return nil, err
	}
	return &asmClient{conn: conn, br: bufio.NewReaderSize(conn, 64<<10)}, nil
}

// setState moves an import attempt on; it reports false once the attempt
// is over.
func (m *asmManager) setState(t *asmTask, gen int, state string, rdb bool) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if t.gen != gen {
		return false
	}
	if rdb {
		t.rdbState = state
	} else {
		t.state = state
	}
	return true
}

func (m *asmManager) importFailed(t *asmTask, gen int, reason string) {
	m.mu.Lock()
	if t.gen == gen {
		m.failLocked(t, reason)
	}
	m.mu.Unlock()
}

func (m *asmManager) track(t *asmTask, gen int, c *asmClient) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if t.gen != gen {
		_ = c.conn.Close()
		return false
	}
	t.conns = append(t.conns, c.conn)
	return true
}

// handshake authenticates a channel with the internal secret.
func (m *asmManager) authenticate(c *asmClient) error {
	if err := c.send("AUTH", "internal connection", m.cluster().InternalSecret()); err != nil {
		return err
	}
	return nil
}

func (m *asmManager) runImport(t *asmTask, gen int) {
	// Keys left in the slots by an earlier attempt or a legacy migration
	// would meet the snapshot.
	m.s.trimSlotRanges(t.ranges, true)
	main, err := m.dial(t.source)
	if err != nil {
		m.importFailed(t, gen, "Main channel - Failed to connect to source node: "+err.Error())
		return
	}
	if !m.track(t, gen, main) {
		return
	}
	mainFail := func(msg string) {
		m.importFailed(t, gen, "Main channel - Failed to sync with source node: "+msg)
	}
	if err := m.authenticate(main); err != nil {
		mainFail(err.Error())
		return
	}
	if !m.setState(t, gen, asmAuthReply, false) {
		return
	}
	if r, err := main.reply(); err != nil {
		mainFail(err.Error())
		return
	} else if r != "+OK" {
		mainFail("Error reply to AUTH from the source: " + r)
		return
	}
	if !m.setState(t, gen, asmSendHandshake, false) {
		return
	}
	if err := main.send("CLUSTER", "SYNCSLOTS", "CONF", "NODE-ID", m.cluster().MyID()); err != nil {
		mainFail(err.Error())
		return
	}
	if !m.setState(t, gen, asmHandshakeReply, false) {
		return
	}
	if r, err := main.reply(); err != nil {
		mainFail(err.Error())
		return
	} else if r != "+OK" {
		mainFail("Error reply to CLUSTER SYNCSLOTS CONF from the source: " + r)
		return
	}
	for {
		if !m.setState(t, gen, asmSendSyncSlots, false) {
			return
		}
		args := []string{"CLUSTER", "SYNCSLOTS", "SYNC", t.id}
		for _, r := range t.ranges {
			args = append(args, strconv.Itoa(r.Start), strconv.Itoa(r.End))
		}
		if err := main.send(args...); err != nil {
			mainFail(err.Error())
			return
		}
		if !m.setState(t, gen, asmSyncSlotsReply, false) {
			return
		}
		r, err := main.reply()
		if err != nil {
			mainFail(err.Error())
			return
		}
		if strings.HasPrefix(r, "+RDBCHANNELSYNCSLOTS") {
			break
		}
		if !strings.HasPrefix(r, "-NOTREADY") {
			mainFail("Error reply to CLUSTER SYNCSLOTS SYNC from the source: " + r)
			return
		}
		if !m.setState(t, gen, asmSendSyncSlots, false) {
			return
		}
		time.Sleep(time.Second)
	}
	if !m.setState(t, gen, asmInitRDBChannel, false) {
		return
	}

	rdbFail := func(msg string) {
		m.importFailed(t, gen, "RDB channel - Failed to sync with the source node: "+msg)
	}
	rdb, err := m.dial(t.source)
	if err != nil {
		rdbFail(err.Error())
		return
	}
	if !m.track(t, gen, rdb) || !m.setState(t, gen, asmConnecting, true) {
		return
	}
	if err := m.authenticate(rdb); err != nil {
		rdbFail("Failed to send command to the source node: " + err.Error())
		return
	}
	if !m.setState(t, gen, asmAuthReply, true) {
		return
	}
	if r, err := rdb.reply(); err != nil {
		rdbFail(err.Error())
		return
	} else if r != "+OK" {
		rdbFail("Error reply to AUTH from source: " + r)
		return
	}
	if !m.setState(t, gen, asmRDBRequest, true) {
		return
	}
	if err := rdb.send("CLUSTER", "SYNCSLOTS", "RDBCHANNEL", t.id); err != nil {
		rdbFail("Failed to send command to the source node: " + err.Error())
		return
	}
	if !m.setState(t, gen, asmRDBReply, true) {
		return
	}
	if r, err := rdb.reply(); err != nil {
		rdbFail(err.Error())
		return
	} else if !strings.HasPrefix(r, "+SLOTSSNAPSHOT") {
		rdbFail("Error reply to CLUSTER SYNCSLOTS RDBCHANNEL from the source: " + r)
		return
	}

	m.mu.Lock()
	if t.gen != gen {
		m.mu.Unlock()
		return
	}
	t.state, t.rdbState = asmAccumulateBuf, asmRDBTransfer
	t.mainConn = main.conn
	m.mu.Unlock()
	logger.Info().Str("task", t.id).Msg("ASM: source replied to SLOTSSNAPSHOT, syncing the slots snapshot")
	go m.readMain(t, gen, main)

	// The snapshot, applied as it arrives.
	rr := resp.NewReader(rdb.br)
	for {
		_ = rdb.conn.SetReadDeadline(time.Now().Add(m.replTimeout()))
		name, args, err := rr.ReadCommand()
		if err != nil {
			m.importFailed(t, gen, "RDB channel - Connection is closed")
			return
		}
		if isSyncSlots(name, args, "SNAPSHOT-EOF") {
			break
		}
		if isSyncSlots(name, args, "CONF") || strings.EqualFold(name, "SELECT") {
			continue
		}
		if !m.apply(t, gen, name, args) {
			return
		}
	}
	_ = rdb.conn.Close()
	if !m.setState(t, gen, asmCompleted, true) || !m.setState(t, gen, asmReadyToStream, false) {
		return
	}
	logger.Info().Str("task", t.id).Msg("ASM: slots snapshot applied, streaming the accumulated buffer")
	m.setState(t, gen, asmStreamingBuf, false)

	// Then the writes the main channel brought, until STREAM-EOF.
	for {
		m.mu.Lock()
		if t.gen != gen {
			m.mu.Unlock()
			return
		}
		if len(t.queue) == 0 {
			if t.state == asmStreamingBuf {
				t.state = asmWaitStreamEOF
				m.mu.Unlock()
				logger.Info().Str("task", t.id).Int64("offset", t.destOffset).Msg("ASM: accumulated buffer applied")
				m.sendAck(t)
				continue
			}
			ready := t.queueReady
			m.mu.Unlock()
			<-ready
			continue
		}
		q := t.queue[0]
		t.queue = t.queue[1:]
		m.mu.Unlock()
		if q.err != nil {
			m.importFailed(t, gen, "Main channel - Connection is closed")
			return
		}
		if q.eof {
			m.takeover(t, gen)
			return
		}
		if !strings.EqualFold(string(q.args[0]), "SELECT") && !m.apply(t, gen, string(q.args[0]), q.args[1:]) {
			return
		}
		m.mu.Lock()
		if t.gen == gen {
			t.destOffset += q.size
		}
		m.mu.Unlock()
	}
}

func (m *asmManager) replTimeout() time.Duration {
	if s := m.s.cfg.Replication.ReplTimeout; s > 0 {
		return time.Duration(s) * time.Second
	}
	return time.Minute
}

func isSyncSlots(name string, args [][]byte, sub string) bool {
	return strings.EqualFold(name, "CLUSTER") && len(args) >= 2 && strings.EqualFold(string(args[0]), "SYNCSLOTS") && strings.EqualFold(string(args[1]), sub)
}

// readMain queues what the source sends on the main channel.
func (m *asmManager) readMain(t *asmTask, gen int, c *asmClient) {
	rr := resp.NewReader(c.br)
	for {
		name, args, err := rr.ReadCommand()
		q := asmQueued{err: err}
		if err == nil {
			all := append([][]byte{[]byte(name)}, args...)
			q.args, q.size = all, int64(len(asmAppendCommand(nil, all[0], all[1:])))
			q.eof = isSyncSlots(name, args, "STREAM-EOF")
		}
		m.mu.Lock()
		if t.gen != gen {
			m.mu.Unlock()
			return
		}
		t.queue = append(t.queue, q)
		m.signalQueue(t)
		m.mu.Unlock()
		if err != nil || q.eof {
			return
		}
	}
}

// apply runs one command of the slots' data as the master client would.
func (m *asmManager) apply(t *asmTask, gen int, name string, args [][]byte) bool {
	s := m.s
	s.LockWrites()
	defer s.UnlockWrites()
	m.mu.Lock()
	current := t.gen == gen
	m.mu.Unlock()
	if !current {
		return false
	}
	ctx := command.NewContext(name, args, s.dbs.DB(0), resp.NewWriter(io.Discard))
	ctx.DB = 0
	ctx.DBs = s.dbs
	ctx.Transaction = command.NewTransaction()
	ctx.SetAuthenticated(true)
	ctx.InExec = true
	ctx.FromMaster = true
	err := s.router.Execute(ctx)
	if err == nil && ctx.Writer.Errors() > 0 {
		err = errors.New("error reply")
	}
	if err != nil {
		logger.Warn().Str("task", t.id).Str("command", name).Err(err).Msg("ASM: applying a command of the imported slots failed")
	}
	return true
}

func (m *asmManager) sendAck(t *asmTask) {
	m.mu.Lock()
	conn, state, offset := t.mainConn, t.state, t.destOffset
	gen := t.gen
	m.mu.Unlock()
	if conn == nil || (state != asmWaitStreamEOF && state != asmStreamingBuf) {
		return
	}
	c := &asmClient{conn: conn}
	t.ackMu.Lock()
	err := c.send("CLUSTER", "SYNCSLOTS", "ACK", state, strconv.FormatInt(offset, 10))
	t.ackMu.Unlock()
	if err != nil {
		m.importFailed(t, gen, "Main channel - Failed to send ACK: "+err.Error())
	}
}

// takeover is asmImportTakeover: the slots become this node's.
func (m *asmManager) takeover(t *asmTask, gen int) {
	m.mu.Lock()
	if t.gen != gen {
		m.mu.Unlock()
		return
	}
	for _, c := range t.conns {
		_ = c.Close()
	}
	t.conns, t.mainConn = nil, nil
	t.state = asmTakeover
	m.mu.Unlock()
	logger.Info().Str("task", t.id).Str("slots", command.SlotRangesString(t.ranges)).Msg("ASM: import task is ready to take over the slots")
	m.cluster().TakeOverSlots(t.ranges)
	m.mu.Lock()
	if t.gen == gen && t.state == asmTakeover {
		t.lastErr = ""
		t.state = asmCompleted
		m.finalizeLocked(t)
		logger.Info().Str("task", t.id).Msg("ASM: import task completed")
	}
	m.mu.Unlock()
}

// --- The source side --------------------------------------------------------

// SyncSlots is CLUSTER SYNCSLOTS on an internal connection.
func (m *asmManager) SyncSlots(ctx *command.Context, args [][]byte) error {
	sub := strings.ToLower(string(args[0]))
	switch {
	case sub == "sync" && len(args) >= 4:
		if len(args)%2 != 0 {
			return ctx.WriteError(errors.New("ERR wrong number of arguments for 'cluster|syncslots' command"))
		}
		return m.syncCommand(ctx, string(args[1]), args[2:])
	case sub == "rdbchannel" && len(args) == 2:
		return m.rdbChannelCommand(ctx, string(args[1]))
	case (sub == "snapshot-eof" || sub == "stream-eof") && len(args) == 1:
		ctx.CloseConnection = true
		return nil
	case sub == "ack" && len(args) == 3, sub == "fail" && len(args) == 2:
		return nil
	case sub == "conf" && len(args) >= 3:
		return m.confCommand(ctx, args[1:])
	}
	return ctx.WriteError(command.ErrSyntaxError)
}

func (m *asmManager) confCommand(ctx *command.Context, args [][]byte) error {
	for j := 0; j < len(args); j += 2 {
		if j+1 >= len(args) {
			return ctx.WriteError(errors.New("ERR wrong number of arguments for 'cluster|syncslots' command"))
		}
		value := string(args[j+1])
		switch strings.ToLower(string(args[j])) {
		case "node-id":
			if len(value) != cluster.IDLen {
				return ctx.WriteError(fmt.Errorf("ERR Invalid node id length %d", len(value)))
			}
			if _, _, ok := m.cluster().NodeAddr(value); !ok {
				return ctx.WriteError(fmt.Errorf("ERR Node %s not found in cluster", value))
			}
			if ctx.ClusterConn != nil {
				ctx.ClusterConn.NodeID = value
			}
		case "slot-info":
			parts := strings.Split(value, ":")
			valid := len(parts) == 3
			for i := 0; valid && i < 3; i++ {
				n, err := strconv.ParseInt(parts[i], 10, 64)
				valid = err == nil && n >= 0 && (i != 0 || n < cluster.Slots)
			}
			if !valid {
				return ctx.WriteError(fmt.Errorf("ERR Invalid slot info: %s", value))
			}
		case "asm-task":
			if !m.cluster().IsReplica() {
				return ctx.WriteError(errors.New("ERR CLUSTER SYNCSLOTS CONF ASM-TASK only allowed on replica"))
			}
		case "capa":
		default:
			return ctx.WriteError(fmt.Errorf("ERR Unknown option %s", args[j]))
		}
	}
	return ctx.WriteOK()
}

func (m *asmManager) syncCommand(ctx *command.Context, id string, rangeArgs [][]byte) error {
	ranges, err := command.ParseSlotRanges(rangeArgs)
	if err != nil {
		return ctx.WriteError(err)
	}
	c := m.cluster()
	m.mu.Lock()
	source, err := m.validateLocked(ranges, nil)
	if err != nil {
		m.mu.Unlock()
		return ctx.WriteError(err)
	}
	if source != c.MyID() {
		m.mu.Unlock()
		return ctx.WriteError(errors.New("ERR This node is not the owner of the slots"))
	}
	dest := ""
	if ctx.ClusterConn != nil {
		dest = ctx.ClusterConn.NodeID
	}
	if dest != "" {
		if _, master, ok := c.NodeAddr(dest); !ok || !master {
			m.mu.Unlock()
			return ctx.WriteError(fmt.Errorf("ERR Destination node %.40s is not a master", dest))
		}
	}
	var t *asmTask
	unpause := false
	if len(m.tasks) > 0 {
		cur := m.tasks[0]
		switch {
		case cur.id == id && cur.op == asmMigrate && cur.state == asmFailed && rangesEqual(cur.ranges, ranges) && cur.dest == dest:
			m.resetLocked(cur)
			cur.retries++
			t = cur
		case cur.state == asmFailed:
			unpause = m.cancelLocked(cur, "new migration requested")
		default:
			m.mu.Unlock()
			return ctx.WriteError(errors.New("ERR Another ASM task is already in progress"))
		}
	}
	if t == nil {
		t = newASMTask(id, asmMigrate, ranges)
		t.start = nowMilli()
		m.tasks = append(m.tasks, t)
	}
	t.ranges, t.op = ranges, asmMigrate
	t.source, t.dest = c.MyID(), dest
	t.state = asmWaitRDBChannel
	t.wake = make(chan struct{}, 1)
	gen := t.gen
	m.mu.Unlock()
	if unpause {
		go m.s.unpauseClients()
	}
	logger.Info().Str("task", t.id).Str("dest", dest).Str("slots", command.SlotRangesString(ranges)).Msg("ASM: migrate task created")
	if err := ctx.WriteSimpleString("RDBCHANNELSYNCSLOTS"); err != nil {
		return err
	}
	ctx.Handoff = func(conn net.Conn) command.ConnHandler {
		h := &asmMainHandler{m: m, t: t, gen: gen}
		m.mu.Lock()
		if t.gen != gen || t.state != asmWaitRDBChannel {
			m.mu.Unlock()
			_ = conn.Close()
			return h
		}
		t.mainConn = conn
		t.lastAck = nowMilli()
		m.mu.Unlock()
		go m.writeMain(t, gen, conn)
		return h
	}
	return nil
}

func (m *asmManager) rdbChannelCommand(ctx *command.Context, id string) error {
	if len(id) != cluster.IDLen {
		return ctx.WriteError(errors.New("ERR Invalid task id"))
	}
	m.mu.Lock()
	if len(m.tasks) == 0 {
		m.mu.Unlock()
		return ctx.WriteError(errors.New("ERR No slot migration task in progress"))
	}
	t := m.tasks[0]
	if t.op != asmMigrate || t.state != asmWaitRDBChannel || t.id != id {
		m.mu.Unlock()
		return ctx.WriteError(errors.New("ERR Another migration task is already in progress"))
	}
	if t.mainConn == nil {
		m.mu.Unlock()
		return ctx.WriteError(errors.New("ERR Main channel connection is not established"))
	}
	t.state, t.rdbState = asmWaitBgsaveStart, asmWaitBgsaveStart
	gen := t.gen
	m.mu.Unlock()
	ctx.Handoff = func(conn net.Conn) command.ConnHandler {
		m.mu.Lock()
		if t.gen != gen {
			m.mu.Unlock()
			_ = conn.Close()
		} else {
			t.rdbConn = conn
			m.mu.Unlock()
			go m.sendSnapshot(t, gen, conn)
		}
		return asmIgnoreHandler{}
	}
	return nil
}

// sendSnapshot takes the snapshot of the slots with writes held off, starts
// copying the writes to them to the main channel, then sends the snapshot.
func (m *asmManager) sendSnapshot(t *asmTask, gen int, conn net.Conn) {
	s := m.s
	s.LockWrites()
	m.mu.Lock()
	if t.gen != gen || t.state != asmWaitBgsaveStart {
		m.mu.Unlock()
		s.UnlockWrites()
		return
	}
	ranges := t.ranges
	m.mu.Unlock()
	snapshot := m.slotSnapshot(ranges)
	m.mu.Lock()
	if t.gen != gen {
		m.mu.Unlock()
		s.UnlockWrites()
		return
	}
	t.state, t.rdbState, t.destState = asmSendBulkAndStream, asmRDBTransfer, asmAccumulateBuf
	t.destSnapshotTime = nowMilli()
	m.recording.Store(true)
	s.repl.SetTap(m.tap)
	m.mu.Unlock()
	s.UnlockWrites()
	logger.Info().Str("task", t.id).Int("bytes", len(snapshot)).Msg("ASM: sending the slots snapshot")

	_ = conn.SetWriteDeadline(time.Now().Add(m.replTimeout()))
	_, err := conn.Write(append([]byte("+SLOTSSNAPSHOT\r\n"), snapshot...))
	if err == nil {
		_, err = conn.Write(asmCommand("CLUSTER", "SYNCSLOTS", "SNAPSHOT-EOF"))
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if t.gen != gen {
		return
	}
	t.rdbConn = nil
	if err != nil {
		if m.failLocked(t, "RDB channel - Failed to send slots snapshot") {
			go m.s.unpauseClients()
		}
		return
	}
	_ = conn.Close()
	if t.state == asmSendBulkAndStream {
		t.state, t.rdbState = asmSendStream, asmCompleted
		t.lastAck = nowMilli()
	}
}

// slotSnapshot is slotSnapshotSaveRio: the function libraries, then every
// key of the slots as a RESTORE with its absolute expiry.
func (m *asmManager) slotSnapshot(ranges []cluster.SlotRange) []byte {
	var out []byte
	if reg := m.s.router.FunctionRegistry(); reg != nil {
		out = asmAppendCommand(out, []byte("FUNCTION"), [][]byte{[]byte("RESTORE"), reg.DumpPayload(), []byte("REPLACE")})
	}
	db := m.s.dbs.DB(0)
	if db.KeyCount() == 0 {
		return out
	}
	out = asmAppendCommand(out, []byte("SELECT"), [][]byte{[]byte("0")})
	now := nowMilli()
	for _, r := range ranges {
		for slot := r.Start; slot <= r.End; slot++ {
			n := db.CountKeysInSlot(slot)
			if n == 0 {
				continue
			}
			keys := db.KeysInSlot(slot, n)
			expires := 0
			var body []byte
			count := 0
			for _, k := range keys {
				e, ok := db.Peek(k)
				if !ok {
					continue
				}
				exp := e.ExpiresAt.Load()
				if exp != 0 && exp <= now {
					continue
				}
				payload, err := persistence.EncodeDump(e.Value)
				if err != nil {
					logger.Warn().Str("key", k).Err(err).Msg("ASM: a key of the slots could not be encoded")
					continue
				}
				if exp != 0 {
					expires++
				}
				body = asmAppendCommand(body, []byte("RESTORE"), [][]byte{[]byte(k), []byte(strconv.FormatInt(exp, 10)), payload, []byte("ABSTTL")})
				count++
			}
			if count == 0 {
				continue
			}
			info := fmt.Sprintf("%d:%d:%d", slot, count, expires)
			out = asmAppendCommand(out, []byte("CLUSTER"), [][]byte{[]byte("SYNCSLOTS"), []byte("CONF"), []byte("SLOT-INFO"), []byte(info)})
			out = append(out, body...)
		}
	}
	return out
}

// tap is asmFeedMigrationClient: the writes to the migrating slots go to
// the main channel.
func (m *asmManager) tap(cmds []command.ExecutedCommand) {
	if !m.recording.Load() {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.tasks) == 0 {
		return
	}
	t := m.tasks[0]
	if t.op != asmMigrate || t.crossSlot {
		return
	}
	switch t.state {
	case asmSendBulkAndStream, asmSendStream, asmHandoffPrep, asmHandoff:
	default:
		return
	}
	fed := false
	for _, c := range cmds {
		if c.DB != 0 {
			continue
		}
		slot := command.CommandSlot(c.Name, c.Args)
		if slot == -1 {
			continue
		}
		if slot == -2 {
			t.crossSlot = true
			m.kick()
			return
		}
		if !rangesOverlap(t.ranges, cluster.SlotRange{Start: slot, End: slot}) {
			continue
		}
		n := len(t.stream)
		t.stream = asmAppendCommand(t.stream, []byte(c.Name), c.Args)
		t.sourceOffset += int64(len(t.stream) - n)
		fed = true
	}
	if fed {
		m.signalWriter(t)
	}
}

// writeMain sends the main channel's writes, then STREAM-EOF once the
// handoff began and nothing is left.
func (m *asmManager) writeMain(t *asmTask, gen int, conn net.Conn) {
	for {
		m.mu.Lock()
		if t.gen != gen || t.streamDone {
			m.mu.Unlock()
			return
		}
		data := t.stream
		t.stream = nil
		eof := len(data) == 0 && t.state == asmHandoff
		if eof {
			t.state, t.streamDone = asmStreamEOF, true
			if t.rdbConn != nil {
				_ = t.rdbConn.Close()
				t.rdbConn = nil
				t.rdbState = asmCompleted
			}
			m.stopRecordingLocked()
		}
		wake := t.wake
		m.mu.Unlock()
		if eof {
			logger.Info().Str("task", t.id).Msg("ASM: slot migration stream drained, sending STREAM-EOF")
			data = asmCommand("CLUSTER", "SYNCSLOTS", "STREAM-EOF")
		}
		if len(data) > 0 {
			_ = conn.SetWriteDeadline(time.Now().Add(m.replTimeout()))
			if _, err := conn.Write(data); err != nil {
				m.mu.Lock()
				if t.gen == gen {
					if m.failLocked(t, "Main channel - Failed to send the stream: "+err.Error()) {
						go m.s.unpauseClients()
					}
				}
				m.mu.Unlock()
				return
			}
			continue
		}
		<-wake
	}
}

// asmMainHandler gets the destination's ACKs on the main channel.
type asmMainHandler struct {
	m   *asmManager
	t   *asmTask
	gen int
}

func (h *asmMainHandler) Command(cmd string, args [][]byte) {
	if !isSyncSlots(cmd, args, "ACK") || len(args) != 4 {
		return
	}
	state := strings.ToLower(string(args[2]))
	if state != asmStreamingBuf && state != asmWaitStreamEOF {
		return
	}
	offset, err := strconv.ParseInt(string(args[3]), 10, 64)
	if err != nil {
		return
	}
	m, t := h.m, h.t
	maxLag, _, _, _ := command.SlotMigrationSettings()
	m.mu.Lock()
	if t.gen != h.gen || t.streamDone {
		m.mu.Unlock()
		return
	}
	t.destState = state
	t.lastAck = nowMilli()
	if t.destOffset > offset {
		m.mu.Unlock()
		return
	}
	t.destOffset = offset
	if state == asmWaitStreamEOF && t.destAccumApplied == 0 {
		t.destAccumApplied = nowMilli()
	}
	prep := (t.state == asmSendBulkAndStream || t.state == asmSendStream) && t.destOffset+maxLag >= t.sourceOffset
	if prep {
		logger.Info().Str("task", t.id).Int64("lag", t.sourceOffset-t.destOffset).Msg("ASM: pausing writes for slot handoff")
		t.state = asmHandoffPrep
		m.paused = true
	}
	m.mu.Unlock()
	if !prep {
		return
	}
	// Writes stop; the ones already running reach the stream first.
	m.s.pauseWritesForFailover(time.Now().Add(24 * time.Hour))
	m.mu.Lock()
	if t.gen == h.gen && t.state == asmHandoffPrep {
		t.state = asmHandoff
		t.paused = nowMilli()
		m.signalWriter(t)
		m.mu.Unlock()
		return
	}
	still := m.paused
	m.mu.Unlock()
	if !still {
		m.s.unpauseClients()
	}
}

func (h *asmMainHandler) Closed() {
	m, t := h.m, h.t
	m.mu.Lock()
	unpause := false
	if t.gen == h.gen && !t.streamDone && t.state != asmFailed && t.state != asmCanceled && t.state != asmCompleted {
		t.mainConn = nil
		unpause = m.failLocked(t, "Main and RDB channel clients are disconnected.")
	}
	m.mu.Unlock()
	if unpause {
		m.s.unpauseClients()
	}
}

type asmIgnoreHandler struct{}

func (asmIgnoreHandler) Command(string, [][]byte) {}
func (asmIgnoreHandler) Closed()                  {}

// slotsLost is asmNotifyConfigUpdated on the source: another node took
// over slots of this one. The migrate task moving exactly those slots
// completes; any other task touching them is cancelled.
func (m *asmManager) slotsLost(slots []int) {
	ranges := slotsToRanges(slots)
	m.mu.Lock()
	var task *asmTask
	for _, t := range m.tasks {
		if rangesEqual(t.ranges, ranges) {
			task = t
			break
		}
	}
	unpause, trim := false, false
	switch {
	case task == nil:
		for _, t := range append([]*asmTask(nil), m.tasks...) {
			for _, r := range ranges {
				if rangesOverlap(t.ranges, r) {
					unpause = m.cancelLocked(t, "slots configuration updated") || unpause
					break
				}
			}
		}
	case task.op == asmMigrate && task.state == asmStreamEOF:
		task.lastErr = ""
		task.state = asmCompleted
		task.gen++
		m.finalizeLocked(task)
		m.stopRecordingLocked()
		if m.paused {
			m.paused, unpause = false, true
		}
		trim = true
		logger.Info().Str("task", task.id).Str("slots", command.SlotRangesString(ranges)).Msg("ASM: migrate task completed")
	default:
		logger.Warn().Str("task", task.id).Str("state", task.state).Msg("ASM: task is not in the correct state for config update")
		unpause = m.cancelLocked(task, "slots configuration updated")
	}
	m.mu.Unlock()
	if unpause {
		m.s.unpauseClients()
	}
	if trim {
		m.s.trimSlotRanges(ranges, true)
	}
}

// cancelSlotMigrationOnFlush cancels the slot migration tasks when database
// 0 is flushed: neither side can keep its promise about the slots' data
// then, and the import retries.
func (s *Server) cancelSlotMigrationOnFlush(db int, cmd string) {
	if s.router.Cluster() == nil || (cmd != "FLUSHALL" && (cmd != "FLUSHDB" || db != 0)) {
		return
	}
	go s.asm.Cancel("", "flush")
}

func slotsToRanges(slots []int) []cluster.SlotRange {
	var out []cluster.SlotRange
	for _, s := range slots {
		if n := len(out); n > 0 && out[n-1].End+1 == s {
			out[n-1].End = s
		} else {
			out = append(out, cluster.SlotRange{Start: s, End: s})
		}
	}
	return out
}

// trimSlotRanges deletes the keys of ranges and, when propagate is set and
// some were deleted, records a TRIMSLOTS in the AOF and the replication
// stream, as Redis' propagateTrimSlots does.
func (s *Server) trimSlotRanges(ranges []cluster.SlotRange, propagate bool) {
	s.LockWrites()
	defer s.UnlockWrites()
	db := s.dbs.DB(0)
	if command.TrimSlots(db, ranges) == 0 || !propagate {
		return
	}
	c := command.TrimSlotsCommand(ranges)
	if aof := s.aof(); aof != nil {
		if err := aof.AppendDB(0, c.Name, c.Args); err != nil {
			logger.Error().Err(err).Msg("AOF append failed")
		} else {
			s.syncRemoval(aof)
		}
	}
	s.repl.Feed(0, c.Name, c.Args)
}

func asmAppendCommand(buf, name []byte, args [][]byte) []byte {
	buf = append(buf, '*')
	buf = strconv.AppendInt(buf, int64(len(args)+1), 10)
	buf = append(buf, '\r', '\n')
	for _, a := range append([][]byte{name}, args...) {
		buf = append(buf, '$')
		buf = strconv.AppendInt(buf, int64(len(a)), 10)
		buf = append(buf, '\r', '\n')
		buf = append(buf, a...)
		buf = append(buf, '\r', '\n')
	}
	return buf
}

func asmCommand(args ...string) []byte {
	b := make([][]byte, len(args))
	for i, a := range args {
		b[i] = []byte(a)
	}
	return asmAppendCommand(nil, b[0], b[1:])
}
