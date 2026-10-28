package command

import (
	"strconv"
	"strings"
	"sync"

	"github.com/cachestorm/cachestorm/internal/resp"
)

// commandMeta is one command or subcommand as Redis describes it in COMMAND
// INFO and COMMAND DOCS (see redis_command_data.go).
type commandMeta struct {
	name     string
	arity    int
	flags    []string
	info     *resp.Value
	docs     *resp.Value
	keySpecs []keySpec
	subs     map[string]*commandMeta
}

// keySpec is a Redis key specification: where the search for keys begins
// (an index or a keyword) and how the keys are found from there (a range or
// a key count).
type keySpec struct {
	flags       []string
	bsType      string
	bsIndex     int
	bsKeyword   string
	bsStartFrom int
	fkType      string
	lastKey     int
	keyStep     int
	limit       int
	keyNumIdx   int
	firstKey    int
}

func (m *commandMeta) hasFlag(flag string) bool {
	for _, f := range m.flags {
		if f == flag {
			return true
		}
	}
	return false
}

func (k *keySpec) hasFlag(flag string) bool {
	for _, f := range k.flags {
		if f == flag {
			return true
		}
	}
	return false
}

var (
	commandMetaOnce  sync.Once
	commandMetaTable map[string]*commandMeta
)

// redisCommandMeta returns the Redis description of the command or
// "container|subcommand" name, lower case, or nil.
func redisCommandMeta(name string) *commandMeta {
	commandMetaOnce.Do(loadCommandMeta)
	return commandMetaTable[strings.ToLower(name)]
}

// lookupRedisCommandMeta resolves argv the way Redis' lookupCommand does: a
// container command with a further argument resolves to that subcommand, or
// to nil when it has no such subcommand.
func lookupRedisCommandMeta(argv []string) *commandMeta {
	if len(argv) == 0 {
		return nil
	}
	base := redisCommandMeta(argv[0])
	if base == nil || len(argv) == 1 || len(base.subs) == 0 {
		return base
	}
	return base.subs[strings.ToLower(argv[1])]
}

func loadCommandMeta() {
	commandMetaTable = make(map[string]*commandMeta, len(redisCommandInfoRESP3)*2)
	for name, raw := range redisCommandInfoRESP3 {
		info, _ := decodeRESP3(raw)
		m := newCommandMeta(info)
		commandMetaTable[name] = m
		for _, sub := range m.subs {
			commandMetaTable[sub.name] = sub
		}
	}
	for name, raw := range redisCommandDocsRESP3 {
		docs, _ := decodeRESP3(raw)
		m := commandMetaTable[name]
		if m == nil {
			continue
		}
		m.docs = docs
		if subDocs := pairsField(docs, "subcommands"); subDocs != nil {
			for i := 0; i+1 < len(subDocs.Array); i += 2 {
				if sub := commandMetaTable[string(subDocs.Array[i].Bulk)]; sub != nil {
					sub.docs = subDocs.Array[i+1]
				}
			}
		}
	}
}

func newCommandMeta(info *resp.Value) *commandMeta {
	a := info.Array
	m := &commandMeta{name: string(a[0].Bulk), arity: int(a[1].Int), info: info}
	for _, f := range a[2].Array {
		m.flags = append(m.flags, f.Str)
	}
	for _, s := range a[8].Array {
		m.keySpecs = append(m.keySpecs, parseKeySpec(s))
	}
	if len(a[9].Array) > 0 {
		m.subs = make(map[string]*commandMeta, len(a[9].Array))
		for _, s := range a[9].Array {
			sub := newCommandMeta(s)
			_, short, _ := strings.Cut(sub.name, "|")
			m.subs[short] = sub
		}
	}
	return m
}

// pairsField returns the value of key in an ordered RESP3 map.
func pairsField(v *resp.Value, key string) *resp.Value {
	if v == nil || v.Type != resp.TypeMap {
		return nil
	}
	for i := 0; i+1 < len(v.Array); i += 2 {
		k := v.Array[i]
		if string(k.Bulk) == key || k.Str == key {
			return v.Array[i+1]
		}
	}
	return nil
}

func pairsInt(v *resp.Value, key string) int {
	if f := pairsField(v, key); f != nil {
		return int(f.Int)
	}
	return 0
}

func pairsString(v *resp.Value, key string) string {
	if f := pairsField(v, key); f != nil {
		if f.Type == resp.TypeSimpleString {
			return f.Str
		}
		return string(f.Bulk)
	}
	return ""
}

func parseKeySpec(v *resp.Value) keySpec {
	var k keySpec
	if fl := pairsField(v, "flags"); fl != nil {
		for _, f := range fl.Array {
			k.flags = append(k.flags, f.Str)
		}
	}
	bs := pairsField(v, "begin_search")
	k.bsType = pairsString(bs, "type")
	bsSpec := pairsField(bs, "spec")
	switch k.bsType {
	case "index":
		k.bsIndex = pairsInt(bsSpec, "index")
	case "keyword":
		k.bsKeyword = pairsString(bsSpec, "keyword")
		k.bsStartFrom = pairsInt(bsSpec, "startfrom")
	}
	fk := pairsField(v, "find_keys")
	k.fkType = pairsString(fk, "type")
	fkSpec := pairsField(fk, "spec")
	switch k.fkType {
	case "range":
		k.lastKey = pairsInt(fkSpec, "lastkey")
		k.keyStep = pairsInt(fkSpec, "keystep")
		k.limit = pairsInt(fkSpec, "limit")
	case "keynum":
		k.keyNumIdx = pairsInt(fkSpec, "keynumidx")
		k.firstKey = pairsInt(fkSpec, "firstkey")
		k.keyStep = pairsInt(fkSpec, "keystep")
	}
	return k
}

// decodeRESP3 decodes the RESP3 value at the start of s: maps keep their
// order (resp.PairsValue) and sets stay sets, so a RESP2 client gets the flat
// arrays Redis sends it.
func decodeRESP3(s string) (*resp.Value, string) {
	end := strings.Index(s, "\r\n")
	if end < 1 {
		return resp.NullValue(), ""
	}
	kind, body, rest := s[0], s[1:end], s[end+2:]
	switch kind {
	case '+':
		return resp.SimpleString(body), rest
	case '-':
		return resp.ErrorValue(body), rest
	case ':':
		n, err := strconv.ParseInt(body, 10, 64)
		if err != nil {
			return resp.NullValue(), rest
		}
		return resp.IntegerValue(n), rest
	case '_':
		return resp.NullValue(), rest
	case '$':
		n, err := strconv.Atoi(body)
		if err != nil || n < 0 || n+2 > len(rest) {
			return resp.NullBulkString(), rest
		}
		return resp.BulkString(rest[:n]), rest[n+2:]
	case '*', '~', '%':
		n, err := strconv.Atoi(body)
		if err != nil || n < 0 {
			return resp.NullValue(), rest
		}
		if kind == '%' {
			n *= 2
		}
		items := make([]*resp.Value, 0, n)
		for i := 0; i < n; i++ {
			var it *resp.Value
			it, rest = decodeRESP3(rest)
			items = append(items, it)
		}
		switch kind {
		case '~':
			return resp.SetValue(items), rest
		case '%':
			return resp.PairsValue(items), rest
		}
		return resp.ArrayValue(items), rest
	}
	return resp.NullValue(), rest
}

// commandKeyRef is one key COMMAND GETKEYS reports, with its key-spec flags.
type commandKeyRef struct {
	name  string
	flags []string
}

// keysFromSpecs finds the keys of argv (command name first) with the
// command's key specs, as Redis' getKeysUsingKeySpecs does. ok is false when
// a spec cannot be applied (an incomplete spec, a bad key count), so the
// caller falls back to the command's own key extraction.
func keysFromSpecs(m *commandMeta, argv []string) (keys []commandKeyRef, ok bool) {
	return keysFromSpecsMode(m, argv, false)
}

// keysFromSpecsPartial is keysFromSpecs that skips the specs it cannot
// apply and keeps the keys of incomplete ones, as Redis'
// GET_KEYSPEC_RETURN_PARTIAL does.
func keysFromSpecsPartial(m *commandMeta, argv []string) []commandKeyRef {
	keys, _ := keysFromSpecsMode(m, argv, true)
	return keys
}

func keysFromSpecsMode(m *commandMeta, argv []string, partial bool) (keys []commandKeyRef, ok bool) {
	argc := len(argv)
	for i := range m.keySpecs {
		spec := &m.keySpecs[i]
		if spec.hasFlag("not_key") {
			continue
		}
		if spec.hasFlag("variable_flags") && !partial {
			return nil, false
		}
		first := 0
		switch spec.bsType {
		case "index":
			first = spec.bsIndex
		case "keyword":
			start, end := spec.bsStartFrom, argc-1
			if start <= 0 {
				start, end = argc+spec.bsStartFrom, 0
			}
			for j := start; j != end; {
				if j >= argc || j < 1 {
					break
				}
				if strings.EqualFold(argv[j], spec.bsKeyword) {
					first = j + 1
					break
				}
				if start <= end {
					j++
				} else {
					j--
				}
			}
			if first == 0 {
				continue
			}
		default:
			if partial {
				continue
			}
			return nil, false
		}
		var last, step int
		switch spec.fkType {
		case "range":
			step = spec.keyStep
			switch {
			case spec.lastKey >= 0:
				last = first + spec.lastKey
			case spec.limit == 0:
				last = argc + spec.lastKey
			default:
				last = first + ((argc-first)/spec.limit + spec.lastKey)
			}
		case "keynum":
			step = spec.keyStep
			if spec.keyNumIdx >= argc-first {
				if partial {
					continue
				}
				return nil, false
			}
			n, err := strconv.ParseInt(argv[first+spec.keyNumIdx], 10, 64)
			if err != nil || n < 0 || n > int64(argc) {
				if partial {
					continue
				}
				return nil, false
			}
			first += spec.firstKey
			last = first + int(n) - 1
		default:
			if partial {
				continue
			}
			return nil, false
		}
		if last >= argc || last < first || first >= argc {
			if partial || (spec.fkType == "keynum" && last == first-1) {
				continue
			}
			return nil, false
		}
		if step < 1 {
			step = 1
		}
		for j := first; j <= last; j += step {
			keys = append(keys, commandKeyRef{name: argv[j], flags: spec.flags})
		}
		if spec.hasFlag("incomplete") && !partial {
			return nil, false
		}
	}
	return keys, true
}

// commandHasKeys reports whether a command has a key spec for a real key, as
// Redis' doesCommandHaveKeys does.
func commandHasKeys(m *commandMeta) bool {
	for i := range m.keySpecs {
		if !m.keySpecs[i].hasFlag("not_key") {
			return true
		}
	}
	return false
}
