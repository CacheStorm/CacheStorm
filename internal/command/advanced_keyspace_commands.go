package command

import (
	"bytes"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// SECRET, CONFIG.*, RING, ACTOR and DAG keep each object in a hash key
// marked by a field naming its kind; a TRIE is a sorted set of its words,
// all scored 0, so prefixes are lexicographic ranges. Being keys, they are
// per database, persisted and replicated; changes are recorded as HSET,
// HDEL, ZADD, ZREM and DEL.

const (
	secretMarker = "__secret"
	configMarker = "__config"
	ringMarker   = "__ring"
	actorMarker  = "__actor"
	dagMarker    = "__dag"
)

// --- SECRET ---

type secret struct {
	value            string
	version          int64
	created, updated int64
}

func readSecret(ctx *Context, key string) (*secret, error) {
	r, err := readMarked(ctx, key, secretMarker)
	if err != nil || r == nil {
		return nil, err
	}
	s := &secret{value: r[secretMarker]}
	var e1, e2, e3 error
	s.version, e1 = strconv.ParseInt(r["version"], 10, 64)
	s.created, e2 = strconv.ParseInt(r["created_at"], 10, 64)
	s.updated, e3 = strconv.ParseInt(r["updated_at"], 10, 64)
	if e1 != nil || e2 != nil || e3 != nil {
		return nil, store.ErrWrongType
	}
	return s, nil
}

// writeSecret stores value as the secret's next version and returns it.
func writeSecret(ctx *Context, key, value string, s *secret) (int64, error) {
	now := ctx.nowMS()
	if s == nil {
		return 1, replaceRecord(ctx, key, secretMarker, value, "version", "1", "created_at", i64(now), "updated_at", i64(now))
	}
	return s.version + 1, updateRecord(ctx, key, secretMarker, value, "version", i64(s.version+1), "updated_at", i64(now))
}

func cmdSECRETSET(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	key := ctx.ArgString(0)
	s, err := readSecret(ctx, key)
	if err == nil {
		_, err = writeSecret(ctx, key, ctx.ArgString(1), s)
	}
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func cmdSECRETGET(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	s, err := readSecret(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	if s == nil {
		return ctx.WriteNull()
	}
	return ctx.WriteBulkString(s.value)
}

func cmdSECRETDELETE(ctx *Context) error { return deleteMarked(ctx, secretMarker) }

func cmdSECRETLIST(ctx *Context) error {
	if ctx.ArgCount() != 0 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	return ctx.WriteArray(bulkStrings(markedKeys(ctx, secretMarker)))
}

// SECRET.ROTATE key value replaces an existing secret and replies its new
// version.
func cmdSECRETROTATE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	key := ctx.ArgString(0)
	s, err := readSecret(ctx, key)
	if err == nil && s == nil {
		err = errors.New("ERR secret not found")
	}
	var version int64
	if err == nil {
		version, err = writeSecret(ctx, key, ctx.ArgString(1), s)
	}
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteInteger(version)
}

func cmdSECRETVERSION(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	s, err := readSecret(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	if s == nil {
		return ctx.WriteNull()
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("version"), resp.IntegerValue(s.version),
		resp.BulkString("updated_at"), resp.IntegerValue(s.updated),
	})
}

// --- CONFIG.* ---

func readConfig(ctx *Context, ns string) (map[string]string, error) {
	r, err := readMarked(ctx, ns, configMarker)
	if r != nil {
		delete(r, configMarker)
	}
	return r, err
}

func configPairs(cfg map[string]string, prefix string) []*resp.Value {
	out := make([]*resp.Value, 0, 2*len(cfg))
	for _, k := range sortedKeys(cfg) {
		out = append(out, resp.BulkString(prefix+k), resp.BulkString(cfg[k]))
	}
	return out
}

func checkConfigKey(k string) error {
	if k == configMarker {
		return errors.New("ERR the setting name __config is reserved")
	}
	return nil
}

func cmdCONFIGSET(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	ns := ctx.ArgString(0)
	err := checkConfigKey(ctx.ArgString(1))
	if err == nil {
		_, err = readConfig(ctx, ns)
	}
	if err == nil {
		err = setMarkedFields(ctx, ns, configMarker, ctx.ArgString(1), ctx.ArgString(2))
	}
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func cmdCONFIGGET(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	cfg, err := readConfig(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	v, ok := cfg[ctx.ArgString(1)]
	if !ok {
		return ctx.WriteNull()
	}
	return ctx.WriteBulkString(v)
}

func cmdCONFIGDELETE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	ns := ctx.ArgString(0)
	if _, err := readConfig(ctx, ns); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteInteger(int64(deleteMarkedFields(ctx, ns, configMarker, ctx.ArgString(1))))
}

// CONFIG.LIST [ns] replies the namespace's settings, or every namespace's
// as ns:key, sorted.
func cmdCONFIGLIST(ctx *Context) error {
	if ctx.ArgCount() > 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	if ctx.ArgCount() == 1 {
		cfg, err := readConfig(ctx, ctx.ArgString(0))
		if err != nil {
			return ctx.WriteError(err)
		}
		return ctx.WriteArray(configPairs(cfg, ""))
	}
	var out []*resp.Value
	for _, ns := range markedKeys(ctx, configMarker) {
		if cfg, err := readConfig(ctx, ns); err == nil {
			out = append(out, configPairs(cfg, ns+":")...)
		}
	}
	return ctx.WriteArray(out)
}

// CONFIG.NAMESPACE ns makes an empty namespace.
func cmdCONFIGNAMESPACE(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	ns := ctx.ArgString(0)
	cfg, err := readMarked(ctx, ns, configMarker)
	if err != nil {
		return ctx.WriteError(err)
	}
	if cfg != nil {
		ctx.PropagateAs()
		return ctx.WriteOK()
	}
	if err := setMarkedFields(ctx, ns, configMarker); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

// CONFIG.IMPORT ns key value [key value ...] sets the pairs and replies how
// many it set.
func cmdCONFIGIMPORT(ctx *Context) error {
	if ctx.ArgCount() < 3 || (ctx.ArgCount()-1)%2 != 0 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	ns := ctx.ArgString(0)
	pairs := make([]string, 0, ctx.ArgCount()-1)
	for i := 1; i < ctx.ArgCount(); i++ {
		if i%2 == 1 {
			if err := checkConfigKey(ctx.ArgString(i)); err != nil {
				return ctx.WriteError(err)
			}
		}
		pairs = append(pairs, ctx.ArgString(i))
	}
	if _, err := readConfig(ctx, ns); err != nil {
		return ctx.WriteError(err)
	}
	if err := setMarkedFields(ctx, ns, configMarker, pairs...); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteInteger(int64(len(pairs) / 2))
}

func cmdCONFIGEXPORT(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	cfg, err := readConfig(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteArray(configPairs(cfg, ""))
}

// --- TRIE ---

// maxTrieWordLen bounds the words a trie stores.
const maxTrieWordLen = 4096

func readTrie(ctx *Context, name string) (*store.SortedSetValue, error) {
	entry, ok := ctx.Store.Get(name)
	if !ok {
		return nil, nil
	}
	z, ok := entry.Value.(*store.SortedSetValue)
	if !ok {
		return nil, store.ErrWrongType
	}
	return z, nil
}

// prefixRange returns the lexicographic range of the words starting with
// prefix.
func prefixRange(prefix string) (store.LexBound, store.LexBound) {
	lo := store.LexBound{Val: prefix}
	b := []byte(prefix)
	for len(b) > 0 && b[len(b)-1] == 0xff {
		b = b[:len(b)-1]
	}
	if len(b) == 0 {
		return lo, store.LexBound{Inf: 1}
	}
	b[len(b)-1]++
	return lo, store.LexBound{Val: string(b), Ex: true}
}

func trieWords(ctx *Context, name, prefix string, limit int) ([]string, error) {
	z, err := readTrie(ctx, name)
	if err != nil || z == nil {
		return nil, err
	}
	lo, hi := prefixRange(prefix)
	z.RLock()
	entries := z.LexRange(lo, hi, false, 0, limit)
	z.RUnlock()
	words := make([]string, len(entries))
	for i, e := range entries {
		words[i] = e.Member
	}
	return words, nil
}

// runAs runs handler with args in place of the command's arguments and
// records it as name with those arguments.
func runAs(ctx *Context, name string, handler func(*Context) error, args ...string) error {
	saved := ctx.Args
	ctx.Args = make([][]byte, len(args))
	for i, a := range args {
		ctx.Args[i] = []byte(a)
	}
	defer func() { ctx.Args = saved }()
	return recordedAs(ctx, name, func() error { return handler(ctx) })
}

// TRIE.ADD name word adds the word; OK.
func cmdTRIEADD(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	word := ctx.ArgString(1)
	if len(word) > maxTrieWordLen {
		return ctx.WriteError(fmt.Errorf("ERR word must be at most %d bytes", maxTrieWordLen))
	}
	name := ctx.ArgString(0)
	if _, err := readTrie(ctx, name); err != nil {
		return ctx.WriteError(err)
	}
	// ZADD replies how many members it added; TRIE.ADD replies OK, so the
	// ZADD reply goes to a sink and only an error is passed on.
	saved := ctx.Writer
	var sink bytes.Buffer
	ctx.Writer = resp.NewWriter(&sink)
	err := runAs(ctx, "ZADD", cmdZADD, name, "0", word)
	ctx.Writer = saved
	if err != nil {
		return err
	}
	if reply := sink.String(); strings.HasPrefix(reply, "-") {
		ctx.PropagateAs()
		return ctx.WriteError(errors.New(strings.TrimRight(reply[1:], "\r\n")))
	}
	return ctx.WriteOK()
}

func cmdTRIESEARCH(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	z, err := readTrie(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	found := false
	if z != nil {
		z.RLock()
		_, found = z.GetScore(ctx.ArgString(1))
		z.RUnlock()
	}
	return ctx.WriteInteger(boolInt(found))
}

// TRIE.PREFIX name prefix is 1 when a word starts with prefix.
func cmdTRIEPREFIX(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	words, err := trieWords(ctx, ctx.ArgString(0), ctx.ArgString(1), 1)
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteInteger(boolInt(len(words) > 0))
}

func cmdTRIEDELETE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	return runAs(ctx, "ZREM", cmdZREM, ctx.ArgString(0), ctx.ArgString(1))
}

// TRIE.AUTOCOMPLETE name prefix replies the words starting with prefix,
// sorted.
func cmdTRIEAUTOCOMPLETE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	words, err := trieWords(ctx, ctx.ArgString(0), ctx.ArgString(1), -1)
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteArray(bulkStrings(words))
}

// --- RING ---

const maxRingReplicas = 1024

type ringPoint struct {
	hash uint64
	node string
}

// ringPoints caches each ring's points by its replicas and nodes, so a
// lookup does not rehash the ring.
var (
	ringPointsMu sync.Mutex
	ringPoints   = map[string][]ringPoint{}
)

func ringPointsOf(replicas int, nodes []string) []ringPoint {
	sig := strconv.Itoa(replicas) + "\x00" + strings.Join(nodes, "\x00")
	ringPointsMu.Lock()
	points, ok := ringPoints[sig]
	ringPointsMu.Unlock()
	if ok {
		return points
	}
	points = make([]ringPoint, 0, len(nodes)*replicas)
	for _, node := range nodes {
		for i := 0; i < replicas; i++ {
			points = append(points, ringPoint{hash: ringHash(node + "#" + strconv.Itoa(i)), node: node})
		}
	}
	sort.Slice(points, func(i, j int) bool {
		if points[i].hash != points[j].hash {
			return points[i].hash < points[j].hash
		}
		return points[i].node < points[j].node
	})
	ringPointsMu.Lock()
	if len(ringPoints) >= 256 {
		ringPoints = map[string][]ringPoint{}
	}
	ringPoints[sig] = points
	ringPointsMu.Unlock()
	return points
}

func loadRing(ctx *Context, name string) (int, []string, error) {
	r, err := loadMarked(ctx, name, ringMarker, "ring")
	if err != nil {
		return 0, nil, err
	}
	replicas, err := strconv.Atoi(r[ringMarker])
	if err != nil || replicas < 1 || replicas > maxRingReplicas {
		return 0, nil, store.ErrWrongType
	}
	var nodes []string
	for f := range r {
		if n, ok := strings.CutPrefix(f, "n:"); ok {
			nodes = append(nodes, n)
		}
	}
	sort.Strings(nodes)
	return replicas, nodes, nil
}

// RING.CREATE name replicas makes an empty ring, replacing one at name.
func cmdRINGCREATE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	replicas, err := strconv.Atoi(ctx.ArgString(1))
	if err != nil || replicas < 1 || replicas > maxRingReplicas {
		return ctx.WriteError(fmt.Errorf("ERR replicas must be an integer between 1 and %d", maxRingReplicas))
	}
	if err := createMarked(ctx, ctx.ArgString(0), ringMarker, ringMarker, strconv.Itoa(replicas)); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

// RING.ADD name node adds node and replies the node count.
func cmdRINGADD(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name, node := ctx.ArgString(0), ctx.ArgString(1)
	_, nodes, err := loadRing(ctx, name)
	if err != nil {
		return ctx.WriteError(err)
	}
	if i := sort.SearchStrings(nodes, node); i < len(nodes) && nodes[i] == node {
		ctx.PropagateAs()
		return ctx.WriteInteger(int64(len(nodes)))
	}
	if err := updateRecord(ctx, name, "n:"+node, ""); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteInteger(int64(len(nodes) + 1))
}

// RING.GET name key replies the node owning key, null on an empty ring.
func cmdRINGGET(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	replicas, nodes, err := loadRing(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	if len(nodes) == 0 {
		return ctx.WriteNull()
	}
	points := ringPointsOf(replicas, nodes)
	h := ringHash(ctx.ArgString(1))
	i := sort.Search(len(points), func(i int) bool { return points[i].hash >= h })
	if i == len(points) {
		i = 0
	}
	return ctx.WriteBulkString(points[i].node)
}

func cmdRINGNODES(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	_, nodes, err := loadRing(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteArray(bulkStrings(nodes))
}

func cmdRINGREMOVE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name := ctx.ArgString(0)
	r, err := readMarked(ctx, name, ringMarker)
	if err != nil {
		return ctx.WriteError(err)
	}
	if r == nil {
		ctx.PropagateAs()
		return ctx.WriteInteger(0)
	}
	return ctx.WriteInteger(int64(deleteMarkedFields(ctx, name, ringMarker, "n:"+ctx.ArgString(1))))
}

// --- ACTOR ---

// An actor's mailbox is a queue in its hash: messages are the fields m:<seq>
// from head (the next to receive) to tail (the next to send).

type mailbox struct{ head, tail int64 }

func readActor(ctx *Context, id string) (*mailbox, error) {
	h, err := readMarkedHash(ctx, id, actorMarker)
	if err != nil || h == nil {
		return nil, err
	}
	h.RLock()
	head, _ := h.Get("head")
	tail, _ := h.Get("tail")
	h.RUnlock()
	m := &mailbox{}
	var e1, e2 error
	m.head, e1 = strconv.ParseInt(string(head), 10, 64)
	m.tail, e2 = strconv.ParseInt(string(tail), 10, 64)
	if e1 != nil || e2 != nil || m.head > m.tail {
		return nil, store.ErrWrongType
	}
	return m, nil
}

func loadActor(ctx *Context, id string) (*mailbox, error) {
	m, err := readActor(ctx, id)
	if err == nil && m == nil {
		err = errors.New("ERR actor not found")
	}
	return m, err
}

func actorMessage(ctx *Context, id string, seq int64) (string, error) {
	h, err := readMarkedHash(ctx, id, actorMarker)
	if err != nil || h == nil {
		return "", errWrongTypeOr(true, err)
	}
	h.RLock()
	v, ok := h.Get("m:" + strconv.FormatInt(seq, 10))
	h.RUnlock()
	if !ok {
		return "", store.ErrWrongType
	}
	return string(v), nil
}

func emptyActor(ctx *Context, id string) error {
	return replaceRecord(ctx, id, actorMarker, "1", "head", "0", "tail", "0")
}

func cmdACTORCREATE(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	_, err := readActor(ctx, id)
	if err == nil {
		err = emptyActor(ctx, id)
	}
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func cmdACTORDELETE(ctx *Context) error { return deleteMarked(ctx, actorMarker) }

// ACTOR.SEND id message queues message and replies the mailbox length.
func cmdACTORSEND(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	m, err := loadActor(ctx, id)
	if err == nil {
		err = updateRecord(ctx, id, "m:"+strconv.FormatInt(m.tail, 10), ctx.ArgString(1), "tail", i64(m.tail+1))
	}
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteInteger(m.tail + 1 - m.head)
}

func cmdACTORRECV(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	m, err := loadActor(ctx, id)
	if err != nil {
		return ctx.WriteError(err)
	}
	if m.head == m.tail {
		ctx.PropagateAs()
		return ctx.WriteNull()
	}
	msg, err := actorMessage(ctx, id, m.head)
	if err != nil {
		return ctx.WriteError(err)
	}
	deleteMarkedFields(ctx, id, actorMarker, "m:"+strconv.FormatInt(m.head, 10))
	hdel, _ := ctx.Propagation()
	if err := updateRecord(ctx, id, "head", i64(m.head+1)); err != nil {
		return ctx.WriteError(err)
	}
	hset, _ := ctx.Propagation()
	ctx.PropagateAs(append(hdel, hset...)...)
	return ctx.WriteBulkString(msg)
}

func cmdACTORPOKE(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	m, err := loadActor(ctx, id)
	if err != nil {
		return ctx.WriteError(err)
	}
	if m.head == m.tail {
		return ctx.WriteNull()
	}
	msg, err := actorMessage(ctx, id, m.head)
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteBulkString(msg)
}

func cmdACTORPEEK(ctx *Context) error { return cmdACTORPOKE(ctx) }

func cmdACTORLEN(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	m, err := readActor(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	if m == nil {
		return ctx.WriteInteger(0)
	}
	return ctx.WriteInteger(m.tail - m.head)
}

func cmdACTORLIST(ctx *Context) error {
	if ctx.ArgCount() != 0 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	return ctx.WriteArray(bulkStrings(markedKeys(ctx, actorMarker)))
}

// ACTOR.CLEAR id empties the mailbox.
func cmdACTORCLEAR(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id := ctx.ArgString(0)
	m, err := readActor(ctx, id)
	if err != nil {
		return ctx.WriteError(err)
	}
	if m == nil || m.head == m.tail {
		ctx.PropagateAs()
		return ctx.WriteOK()
	}
	if err := emptyActor(ctx, id); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

// --- DAG ---

// A DAG's nodes are the fields n:<node> and its edges e<len(from)>:<from><to>.

type dag struct {
	nodes map[string]bool
	edges map[string][]string
}

func loadDAG(ctx *Context, name string) (*dag, error) {
	r, err := loadMarked(ctx, name, dagMarker, "DAG")
	if err != nil {
		if strings.HasPrefix(err.Error(), "ERR DAG") {
			return nil, errors.New("ERR DAG not found")
		}
		return nil, err
	}
	d := &dag{nodes: map[string]bool{}, edges: map[string][]string{}}
	for f := range r {
		switch {
		case f == dagMarker:
		case strings.HasPrefix(f, "n:"):
			d.nodes[f[2:]] = true
		default:
			kind, from, to, ok := unpackPair(f)
			if !ok || kind != 'e' {
				return nil, store.ErrWrongType
			}
			d.edges[from] = append(d.edges[from], to)
		}
	}
	for from := range d.edges {
		sort.Strings(d.edges[from])
	}
	return d, nil
}

// reaches reports whether target is reachable from start.
func (d *dag) reaches(start, target string) bool {
	seen := map[string]bool{start: true}
	stack := []string{start}
	for len(stack) > 0 {
		node := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if node == target {
			return true
		}
		for _, next := range d.edges[node] {
			if !seen[next] {
				seen[next] = true
				stack = append(stack, next)
			}
		}
	}
	return false
}

func cmdDAGCREATE(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	if err := createMarked(ctx, ctx.ArgString(0), dagMarker, dagMarker, "1"); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func cmdDAGADDNODE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name, node := ctx.ArgString(0), ctx.ArgString(1)
	d, err := loadDAG(ctx, name)
	if err != nil {
		return ctx.WriteError(err)
	}
	if d.nodes[node] {
		ctx.PropagateAs()
		return ctx.WriteInteger(0)
	}
	if err := updateRecord(ctx, name, "n:"+node, ""); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteInteger(1)
}

func cmdDAGADDEDGE(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name, from, to := ctx.ArgString(0), ctx.ArgString(1), ctx.ArgString(2)
	d, err := loadDAG(ctx, name)
	if err != nil {
		return ctx.WriteError(err)
	}
	if !d.nodes[from] || !d.nodes[to] {
		return ctx.WriteError(errors.New("ERR node not found"))
	}
	for _, c := range d.edges[from] {
		if c == to {
			ctx.PropagateAs()
			return ctx.WriteOK()
		}
	}
	if from == to || d.reaches(to, from) {
		return ctx.WriteError(errors.New("ERR edge would create a cycle"))
	}
	if err := updateRecord(ctx, name, orsetField('e', from, to), ""); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

// DAG.TOPO name replies the nodes in topological order, the smallest ready
// node first.
func cmdDAGTOPO(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	d, err := loadDAG(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	inDegree := make(map[string]int, len(d.nodes))
	for _, children := range d.edges {
		for _, c := range children {
			inDegree[c]++
		}
	}
	var ready []string
	for node := range d.nodes {
		if inDegree[node] == 0 {
			ready = append(ready, node)
		}
	}
	sort.Strings(ready)
	result := make([]string, 0, len(d.nodes))
	for len(ready) > 0 {
		node := ready[0]
		ready = ready[1:]
		result = append(result, node)
		for _, child := range d.edges[node] {
			if inDegree[child]--; inDegree[child] == 0 {
				i := sort.SearchStrings(ready, child)
				ready = append(ready, "")
				copy(ready[i+1:], ready[i:])
				ready[i] = child
			}
		}
	}
	if len(result) != len(d.nodes) {
		return ctx.WriteError(errors.New("ERR DAG contains a cycle"))
	}
	return ctx.WriteArray(bulkStrings(result))
}

func cmdDAGPARENTS(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	d, err := loadDAG(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	node := ctx.ArgString(1)
	var parents []string
	for from, children := range d.edges {
		for _, c := range children {
			if c == node {
				parents = append(parents, from)
				break
			}
		}
	}
	sort.Strings(parents)
	return ctx.WriteArray(bulkStrings(parents))
}

func cmdDAGCHILDREN(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	d, err := loadDAG(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteArray(bulkStrings(d.edges[ctx.ArgString(1)]))
}

func cmdDAGDELETE(ctx *Context) error { return deleteMarked(ctx, dagMarker) }

func cmdDAGLIST(ctx *Context) error {
	if ctx.ArgCount() != 0 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	return ctx.WriteArray(bulkStrings(markedKeys(ctx, dagMarker)))
}
