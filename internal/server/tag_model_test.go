package server

import (
	"fmt"
	"math/rand"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/config"
	"github.com/cachestorm/cachestorm/internal/resp"
)

// tagModel is what a server's tags should be after a sequence of commands:
// per database, each key's value and tags and the hierarchy's links.
type tagModel struct {
	dbs [2]*tagModelDB
	cur int
}

type tagModelKey struct {
	val  string
	tags map[string]bool
}

type tagModelDB struct {
	keys  map[string]*tagModelKey
	links map[string]map[string]bool // parent -> children
}

func newTagModel() *tagModel {
	m := &tagModel{}
	for i := range m.dbs {
		m.dbs[i] = &tagModelDB{keys: map[string]*tagModelKey{}, links: map[string]map[string]bool{}}
	}
	return m
}

func (m *tagModel) db() *tagModelDB { return m.dbs[m.cur] }

func (k *tagModelKey) clone() *tagModelKey {
	c := &tagModelKey{val: k.val, tags: map[string]bool{}}
	for t := range k.tags {
		c.tags[t] = true
	}
	return c
}

// reaches reports whether to is from or below it in d's hierarchy.
func (d *tagModelDB) reaches(from, to string) bool {
	seen := map[string]bool{}
	queue := []string{from}
	for len(queue) > 0 {
		t := queue[0]
		queue = queue[1:]
		if t == to {
			return true
		}
		if seen[t] {
			continue
		}
		seen[t] = true
		for c := range d.links[t] {
			queue = append(queue, c)
		}
	}
	return false
}

var (
	tagModelKeys = []string{"k0", "k1", "k2", "k3", "k4", "k5", "k6", "k7"}
	tagModelTags = []string{"t0", "t1", "t2", "t3", "t4"}
)

func pickTags(rng *rand.Rand, min int) []string {
	n := min + rng.Intn(3)
	out := make([]string, n)
	for i := range out {
		out[i] = tagModelTags[rng.Intn(len(tagModelTags))]
	}
	return out
}

func tagSet(tags []string) map[string]bool {
	s := map[string]bool{}
	for _, t := range tags {
		s[t] = true
	}
	return s
}

// tagModelStep picks a command, applies it to m and returns it with the
// reply it should get: an integer, "OK", "ERR" for any error, or "" when the
// reply is not checked.
func tagModelStep(rng *rand.Rand, m *tagModel) (args []string, want string) {
	d := m.db()
	key := tagModelKeys[rng.Intn(len(tagModelKeys))]
	other := tagModelKeys[rng.Intn(len(tagModelKeys))]
	for other == key {
		other = tagModelKeys[rng.Intn(len(tagModelKeys))]
	}
	val := strconv.Itoa(rng.Intn(100))
	if rng.Intn(3) == 0 {
		val = "v" + val
	}
	itoa := func(n int) string { return strconv.Itoa(n) }
	isList := func(name string) bool { k, ok := d.keys[name]; return ok && k.val == "<list>" }
	switch op := rng.Intn(31); op {
	case 0, 1, 2:
		tags := pickTags(rng, 1)
		args = append([]string{"SET", key, val}, "TAGS")
		args = append(args, tags...)
		if rng.Intn(3) == 0 {
			args = []string{"SET", key, val, "EX", "1000", "TAGS"}
			args = append(args, tags...)
		}
		d.keys[key] = &tagModelKey{val: val, tags: tagSet(tags)}
		return args, "OK"
	case 3:
		tags := pickTags(rng, 1)
		d.keys[key] = &tagModelKey{val: val, tags: tagSet(tags)}
		return append([]string{"SETTAG", key, val}, tags...), "OK"
	case 4:
		// A plain SET (or GETSET) replaces the value and drops the tags.
		if rng.Intn(2) == 0 {
			if isList(key) {
				return []string{"GETSET", key, val}, "ERR"
			}
			d.keys[key] = &tagModelKey{val: val, tags: map[string]bool{}}
			return []string{"GETSET", key, val}, ""
		}
		d.keys[key] = &tagModelKey{val: val, tags: map[string]bool{}}
		return []string{"SET", key, val}, "OK"
	case 5:
		// NX/XX with tags write only when the condition holds.
		tags := pickTags(rng, 1)
		_, exists := d.keys[key]
		cond := "NX"
		if rng.Intn(2) == 0 {
			cond = "XX"
		}
		if (cond == "NX") != exists {
			d.keys[key] = &tagModelKey{val: val, tags: tagSet(tags)}
		}
		return append([]string{"SET", key, val, cond, "TAGS"}, tags...), ""
	case 6, 7:
		tags := pickTags(rng, 1)
		k, ok := d.keys[key]
		if !ok {
			return append([]string{"ADDTAG", key}, tags...), "0"
		}
		added := 0
		for _, t := range tags {
			if !k.tags[t] {
				k.tags[t] = true
				added++
			}
		}
		return append([]string{"ADDTAG", key}, tags...), itoa(added)
	case 8:
		tags := pickTags(rng, 1)
		k, ok := d.keys[key]
		removed := 0
		if ok {
			for _, t := range tags {
				if k.tags[t] {
					delete(k.tags, t)
					removed++
				}
			}
		}
		return append([]string{"REMTAG", key}, tags...), itoa(removed)
	case 9:
		_, ok := d.keys[key]
		delete(d.keys, key)
		cmd := "DEL"
		if rng.Intn(2) == 0 {
			cmd = "UNLINK"
		}
		if ok {
			return []string{cmd, key}, "1"
		}
		return []string{cmd, key}, "0"
	case 10:
		// In-place writes keep the tags.
		k, ok := d.keys[key]
		if isList(key) {
			return []string{"INCR", key}, "ERR"
		}
		if !ok {
			d.keys[key] = &tagModelKey{val: "1", tags: map[string]bool{}}
			return []string{"INCR", key}, "1"
		}
		n, err := strconv.Atoi(k.val)
		if err != nil {
			return []string{"INCR", key}, "ERR"
		}
		k.val = itoa(n + 1)
		return []string{"INCR", key}, itoa(n + 1)
	case 11:
		k, ok := d.keys[key]
		if isList(key) {
			return []string{"APPEND", key, "x"}, "ERR"
		}
		if !ok {
			k = &tagModelKey{tags: map[string]bool{}}
			d.keys[key] = k
		}
		k.val += "x"
		return []string{"APPEND", key, "x"}, itoa(len(k.val))
	case 12:
		k, ok := d.keys[key]
		if !ok {
			return []string{"RENAME", key, other}, "ERR"
		}
		delete(d.keys, key)
		d.keys[other] = k
		return []string{"RENAME", key, other}, "OK"
	case 13:
		k, ok := d.keys[key]
		_, busy := d.keys[other]
		if rng.Intn(2) == 0 {
			if ok {
				d.keys[other] = k.clone()
				return []string{"COPY", key, other, "REPLACE"}, "1"
			}
			return []string{"COPY", key, other, "REPLACE"}, "0"
		}
		if ok && !busy {
			d.keys[other] = k.clone()
			return []string{"COPY", key, other}, "1"
		}
		return []string{"COPY", key, other}, "0"
	case 14:
		dst := m.dbs[1-m.cur]
		k, ok := d.keys[key]
		if _, busy := dst.keys[key]; !ok || busy {
			return []string{"MOVE", key, itoa(1 - m.cur)}, "0"
		}
		delete(d.keys, key)
		dst.keys[key] = k
		return []string{"MOVE", key, itoa(1 - m.cur)}, "1"
	case 15:
		m.dbs[0], m.dbs[1] = m.dbs[1], m.dbs[0]
		return []string{"SWAPDB", "0", "1"}, "OK"
	case 16:
		m.cur = 1 - m.cur
		return []string{"SELECT", itoa(m.cur)}, "OK"
	case 17, 18, 19:
		tags := pickTags(rng, 1)
		args = append([]string{"INVALIDATE"}, tags...)
		reach := map[string]bool{}
		cascade := rng.Intn(2) == 0
		if cascade {
			args = append(args, []string{"CASCADE", "cascade", "Cascade"}[rng.Intn(3)])
			for _, t := range tags {
				for _, u := range tagModelTags {
					if d.reaches(t, u) {
						reach[u] = true
					}
				}
			}
		} else {
			reach = tagSet(tags)
		}
		deleted := 0
		for name, k := range d.keys {
			for t := range k.tags {
				if reach[t] {
					delete(d.keys, name)
					deleted++
					break
				}
			}
		}
		return args, itoa(deleted)
	case 20:
		p := tagModelTags[rng.Intn(len(tagModelTags))]
		c := tagModelTags[rng.Intn(len(tagModelTags))]
		if p == c || d.reaches(c, p) {
			return []string{"TAGLINK", p, c}, "ERR"
		}
		if d.links[p] == nil {
			d.links[p] = map[string]bool{}
		}
		d.links[p][c] = true
		return []string{"TAGLINK", p, c}, ""
	case 21:
		p := tagModelTags[rng.Intn(len(tagModelTags))]
		c := tagModelTags[rng.Intn(len(tagModelTags))]
		delete(d.links[p], c)
		return []string{"TAGUNLINK", p, c}, ""
	case 22:
		_, ok := d.keys[key]
		delete(d.keys, key)
		if ok {
			return []string{"EXPIRE", key, "-1"}, "1"
		}
		return []string{"EXPIRE", key, "-1"}, "0"
	case 23:
		d.keys[key] = &tagModelKey{val: val, tags: map[string]bool{}}
		d.keys[other] = &tagModelKey{val: val, tags: map[string]bool{}}
		return []string{"MSET", key, val, other, val}, "OK"
	case 25:
		// A key that expires leaves its tags.
		_, ok := d.keys[key]
		delete(d.keys, key)
		if ok {
			return []string{"PEXPIRE", key, "1"}, "1"
		}
		return []string{"PEXPIRE", key, "1"}, "0"
	case 26:
		k, ok := d.keys[key]
		if _, busy := d.keys[other]; !ok || busy {
			if !ok {
				return []string{"RENAMENX", key, other}, "ERR"
			}
			return []string{"RENAMENX", key, other}, "0"
		}
		delete(d.keys, key)
		d.keys[other] = k
		return []string{"RENAMENX", key, other}, "1"
	case 27:
		tags := pickTags(rng, 1)
		if isList(key) {
			return append([]string{"SET", key, val, "GET", "TAGS"}, tags...), "ERR"
		}
		d.keys[key] = &tagModelKey{val: val, tags: tagSet(tags)}
		return append([]string{"SET", key, val, "GET", "TAGS"}, tags...), ""
	case 28, 29:
		// Tags work on any type; a push keeps them.
		_, ok := d.keys[key]
		if ok && !isList(key) {
			return []string{"RPUSH", key, "e"}, "ERR"
		}
		if !ok {
			d.keys[key] = &tagModelKey{val: "<list>", tags: map[string]bool{}}
		}
		return []string{"RPUSH", key, "e"}, ""
	default:
		if rng.Intn(10) == 0 {
			// The hierarchy is kept: it is not data of the keys.
			d.keys = map[string]*tagModelKey{}
			return []string{"FLUSHDB"}, "OK"
		}
		if isList(key) {
			return []string{"GETDEL", key}, "ERR"
		}
		delete(d.keys, key)
		return []string{"GETDEL", key}, ""
	}
}

// replyText renders a reply for comparing with a tagModelStep want.
func replyText(v *resp.Value) string {
	switch v.Type {
	case resp.TypeError:
		return "ERR"
	case resp.TypeInteger:
		return strconv.FormatInt(v.Int, 10)
	case resp.TypeSimpleString:
		return v.Str
	}
	return str(v)
}

func sortedBulks(v *resp.Value) string {
	out := make([]string, 0, len(v.Array))
	for _, e := range v.Array {
		out = append(out, string(e.Bulk))
	}
	sort.Strings(out)
	return strings.Join(out, ",")
}

// tagState reads what a server holds of the model's keys, tags and links,
// in a form comparable with modelState. It leaves database 0 selected.
func tagState(t *testing.T, c *replClient) string {
	t.Helper()
	var b strings.Builder
	for db := 0; db < 2; db++ {
		c.str("SELECT", strconv.Itoa(db))
		fmt.Fprintf(&b, "db%d:", db)
		for _, k := range tagModelKeys {
			v := c.do("GET", k)
			if v.Type == resp.TypeBulkString && !v.IsNull {
				fmt.Fprintf(&b, " %s=%s[%s]", k, v.Bulk, sortedBulks(c.do("TAGS", k)))
			} else if v.Type == resp.TypeError {
				fmt.Fprintf(&b, " %s=<%s>[%s]", k, c.str("TYPE", k), sortedBulks(c.do("TAGS", k)))
			}
		}
		for _, tag := range tagModelTags {
			fmt.Fprintf(&b, " %s{%s|%d|%s}", tag, sortedBulks(c.do("TAGKEYS", tag)), c.do("TAGCOUNT", tag).Int, sortedBulks(c.do("TAGCHILDREN", tag)))
		}
		b.WriteString("\n")
	}
	c.str("SELECT", "0")
	return b.String()
}

func modelState(m *tagModel) string {
	var b strings.Builder
	for db := 0; db < 2; db++ {
		d := m.dbs[db]
		fmt.Fprintf(&b, "db%d:", db)
		for _, k := range tagModelKeys {
			if mk, ok := d.keys[k]; ok {
				tags := make([]string, 0, len(mk.tags))
				for tag := range mk.tags {
					tags = append(tags, tag)
				}
				sort.Strings(tags)
				fmt.Fprintf(&b, " %s=%s[%s]", k, mk.val, strings.Join(tags, ","))
			}
		}
		for _, tag := range tagModelTags {
			var keys, children []string
			for name, mk := range d.keys {
				if mk.tags[tag] {
					keys = append(keys, name)
				}
			}
			for c := range d.links[tag] {
				children = append(children, c)
			}
			sort.Strings(keys)
			sort.Strings(children)
			fmt.Fprintf(&b, " %s{%s|%d|%s}", tag, strings.Join(keys, ","), len(keys), strings.Join(children, ","))
		}
		b.WriteString("\n")
	}
	return b.String()
}

// Random tag commands, some in MULTI/EXEC, checked step by step against a
// model of what they should do; then the replica, an AOF restart and a
// rewritten AOF must hold the same keys, tags and hierarchy.
func TestTagCommandsMatchModelX(t *testing.T) {
	for _, seed := range []int64{1, 2, 3, 4} {
		t.Run(strconv.FormatInt(seed, 10), func(t *testing.T) {
			cfg := aofRewriteConfig(t, t.TempDir(), func(c *config.Config) { c.Persistence.AOFRewritePercentage = 0 })
			master := startReplServerCfg(t, cfg)
			replica := startReplServer(t, nil)
			replicate(t, replica, master)
			c := dialRepl(t, master)

			rng := rand.New(rand.NewSource(seed))
			m := newTagModel()
			var history []string
			for step := 0; step < 600; step++ {
				if rng.Intn(8) == 0 {
					// A few commands in one transaction: the model applies
					// them in order; the replies are not checked.
					c.str("MULTI")
					n := 2 + rng.Intn(3)
					for i := 0; i < n; i++ {
						args, _ := tagModelStep(rng, m)
						history = append(history, "  "+strings.Join(args, " "))
						if args[0] == "PEXPIRE" {
							// In a transaction the key would only expire
							// after the commands queued after it ran.
							args[2] = "-1"
						}
						if got := c.do(args...); got.Type == resp.TypeError {
							t.Fatalf("step %d: %v not queued: %s", step, args, got.Err)
						}
					}
					history = append(history, "EXEC")
					c.do("EXEC")
				} else {
					args, want := tagModelStep(rng, m)
					history = append(history, strings.Join(args, " "))
					got := replyText(c.do(args...))
					if args[0] == "PEXPIRE" {
						time.Sleep(3 * time.Millisecond)
					}
					if want != "" && got != want {
						t.Fatalf("step %d: %v = %s, want %s\nhistory:\n%s", step, args, got, want, tail(history, 30))
					}
				}
				if got, want := tagState(t, c), modelState(m); got != want {
					t.Fatalf("step %d: state differs\nserver:\n%smodel:\n%shistory:\n%s", step, got, want, tail(history, 30))
				}
				if m.cur != 0 {
					c.str("SELECT", strconv.Itoa(m.cur))
				}
			}
			want := modelState(m)

			waitSynced(t, replica, master)
			if got := tagState(t, dialRepl(t, replica)); got != want {
				t.Fatalf("replica differs\nreplica:\n%smodel:\n%s", got, want)
			}

			stopReplServer(master)
			master = startReplServerCfg(t, cfg)
			c = dialRepl(t, master)
			if got := tagState(t, c); got != want {
				t.Fatalf("after an AOF restart\nserver:\n%smodel:\n%s", got, want)
			}
			if got := c.do("BGREWRITEAOF"); got.Type == resp.TypeError {
				t.Fatalf("BGREWRITEAOF = %s", got.Err)
			}
			waitAOFRewrites(t, master, 1)
			stopReplServer(master)
			master = startReplServerCfg(t, cfg)
			c = dialRepl(t, master)
			if got := tagState(t, c); got != want {
				t.Fatalf("after a rewritten AOF\nserver:\n%smodel:\n%s", got, want)
			}
			if got := c.str("SAVE"); got != "OK" {
				t.Fatalf("SAVE = %s", got)
			}
			stopReplServer(master)
			rdb := *cfg
			rdb.Persistence.AOF = false
			if got := tagState(t, dialRepl(t, startReplServerCfg(t, &rdb))); got != want {
				t.Fatalf("after a snapshot restart\nserver:\n%smodel:\n%s", got, want)
			}
		})
	}
}

func tail(lines []string, n int) string {
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(slices.Clone(lines), "\n")
}

// Keys evicted under maxmemory leave their tags: every tag lists only keys
// that exist, its count agrees, and a replica ends with the same tags.
func TestEvictedKeysLeaveTheirTagsX(t *testing.T) {
	for _, policy := range []string{"allkeys-lru", "allkeys-random", "volatile-ttl"} {
		t.Run(policy, func(t *testing.T) {
			master := startReplServer(t, nil)
			replica := startReplServer(t, nil)
			replicate(t, replica, master)
			c := dialRepl(t, master)
			c.str("CONFIG", "SET", "maxmemory-policy", policy)
			c.str("CONFIG", "SET", "maxmemory", "4mb")
			value := strings.Repeat("v", 512)
			var batch []byte
			for i := 0; i < 20000; i++ {
				key := "key:" + strconv.Itoa(i)
				batch = append(batch, encodeArgs("SET", key, value, "EX", "1000", "TAGS", "tag"+strconv.Itoa(i%7), "all")...)
				if len(batch) > 64<<10 || i == 19999 {
					_ = c.conn.SetDeadline(time.Now().Add(20 * time.Second))
					if _, err := c.conn.Write(batch); err != nil {
						t.Fatal(err)
					}
					batch = batch[:0]
				}
			}
			// Read the replies of every SET.
			for i := 0; i < 20000; i++ {
				if v, err := c.rd.ReadValue(); err != nil {
					t.Fatal(err)
				} else if v.Type == resp.TypeError {
					t.Fatalf("SET %d: %s", i, v.Err)
				}
			}
			if n := c.info("stats")["evicted_keys"]; n == "0" || n == "" {
				t.Fatalf("nothing evicted (evicted_keys %q)", n)
			}
			checkTagsMatchKeys(t, c)
			waitSynced(t, replica, master)
			rc := dialRepl(t, replica)
			checkTagsMatchKeys(t, rc)
			if got, want := sortedBulks(rc.do("TAGKEYS", "all")), sortedBulks(c.do("TAGKEYS", "all")); got != want {
				t.Fatalf("replica's tagged keys differ from the master's")
			}
		})
	}
}

// checkTagsMatchKeys checks that each tag of TestEvictedKeysLeaveTheirTagsX
// lists exactly the live keys carrying it.
func checkTagsMatchKeys(t *testing.T, c *replClient) {
	t.Helper()
	live := c.do("DBSIZE").Int
	all := c.do("TAGKEYS", "all").Array
	if int64(len(all)) != live || c.do("TAGCOUNT", "all").Int != live {
		t.Fatalf("tag all has %d keys (TAGCOUNT %d), the database %d", len(all), c.do("TAGCOUNT", "all").Int, live)
	}
	sum := int64(0)
	for i := 0; i < 7; i++ {
		tag := "tag" + strconv.Itoa(i)
		keys := c.do("TAGKEYS", tag).Array
		if n := c.do("TAGCOUNT", tag).Int; n != int64(len(keys)) {
			t.Fatalf("%s: TAGCOUNT %d, TAGKEYS %d", tag, n, len(keys))
		}
		for _, k := range keys {
			if c.do("EXISTS", string(k.Bulk)).Int != 1 {
				t.Fatalf("%s lists %s, which is gone", tag, k.Bulk)
			}
		}
		sum += int64(len(keys))
	}
	if sum != live {
		t.Fatalf("the tags list %d keys, the database has %d", sum, live)
	}
	if m := c.info("tags")["tag_memberships"]; m != strconv.FormatInt(2*live, 10) {
		t.Fatalf("tag_memberships %s, want %d", m, 2*live)
	}
}
