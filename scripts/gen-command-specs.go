//go:build ignore

// gen-command-specs regenerates internal/command/redis_command_specs.go and
// internal/command/redis_command_data.go from a real Redis: it starts
// redis:7.4 in Docker, reads COMMAND and COMMAND DOCS over RESP3 and writes
// the arity/flags/key/category table plus the verbatim RESP3 COMMAND INFO and
// COMMAND DOCS entries. Commands CacheStorm implements from later Redis
// releases (redis8Commands) are taken from redis:8.0.
//
//	go run scripts/gen-command-specs.go
package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"go/format"
	"net"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	redis74Image = "redis:7.4"
	redis8Image  = "redis:8.0-alpine"
	outDir       = "internal/command"
)

// redis8Commands are Redis 8 commands CacheStorm implements; their entries
// come from Redis 8.0 because Redis 7.4 does not have them.
var redis8Commands = []string{"hgetdel", "hgetex", "hsetex"}

type val struct {
	kind  byte
	str   string
	num   int64
	items []*val
	raw   []byte
}

type parser struct {
	r   *bufio.Reader
	buf bytes.Buffer
}

func (p *parser) line() (string, error) {
	s, err := p.r.ReadString('\n')
	if err != nil {
		return "", err
	}
	p.buf.WriteString(s)
	return strings.TrimSuffix(s, "\r\n"), nil
}

func (p *parser) value() (*val, error) {
	start := p.buf.Len()
	l, err := p.line()
	if err != nil {
		return nil, err
	}
	if l == "" {
		return nil, errors.New("empty line")
	}
	v := &val{kind: l[0]}
	body := l[1:]
	switch v.kind {
	case '+', '-':
		v.str = body
	case ':':
		v.num, err = strconv.ParseInt(body, 10, 64)
	case '_':
	case '$':
		n, _ := strconv.Atoi(body)
		if n >= 0 {
			b := make([]byte, n+2)
			if _, err = ioReadFull(p.r, b); err != nil {
				return nil, err
			}
			p.buf.Write(b)
			v.str = string(b[:n])
		}
	case '*', '~', '%':
		n, _ := strconv.Atoi(body)
		if v.kind == '%' {
			n *= 2
		}
		for i := 0; i < n; i++ {
			it, err := p.value()
			if err != nil {
				return nil, err
			}
			v.items = append(v.items, it)
		}
	default:
		return nil, fmt.Errorf("unexpected type %q", v.kind)
	}
	v.raw = append([]byte(nil), p.buf.Bytes()[start:]...)
	return v, err
}

func ioReadFull(r *bufio.Reader, b []byte) (int, error) {
	n := 0
	for n < len(b) {
		m, err := r.Read(b[n:])
		n += m
		if err != nil {
			return n, err
		}
	}
	return n, nil
}

type dump struct {
	info map[string]*val
	docs map[string]*val
}

func startRedis(image string) (addr string, stop func(), err error) {
	out, err := exec.Command("docker", "run", "-d", "--rm", "-p", "127.0.0.1::6379", image).Output()
	if err != nil {
		return "", nil, fmt.Errorf("docker run %s: %w", image, err)
	}
	id := strings.TrimSpace(string(out))
	stop = func() { _ = exec.Command("docker", "stop", "-t", "0", id).Run() }
	port, err := exec.Command("docker", "port", id, "6379/tcp").Output()
	if err != nil {
		stop()
		return "", nil, err
	}
	first := strings.SplitN(strings.TrimSpace(string(port)), "\n", 2)[0]
	return first, stop, nil
}

func fetch(image string) (*dump, error) {
	addr, stop, err := startRedis(image)
	if err != nil {
		return nil, err
	}
	defer stop()
	var conn net.Conn
	for i := 0; i < 50; i++ {
		if conn, err = net.Dial("tcp", addr); err == nil {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	p := &parser{r: bufio.NewReaderSize(conn, 1<<20)}
	call := func(args ...string) (*val, error) {
		var b strings.Builder
		fmt.Fprintf(&b, "*%d\r\n", len(args))
		for _, a := range args {
			fmt.Fprintf(&b, "$%d\r\n%s\r\n", len(a), a)
		}
		if _, err := conn.Write([]byte(b.String())); err != nil {
			return nil, err
		}
		p.buf.Reset()
		return p.value()
	}
	for i := 0; i < 50; i++ {
		v, err := call("HELLO", "3")
		if err == nil && v.kind == '%' {
			break
		}
		if err != nil {
			return nil, err
		}
		time.Sleep(200 * time.Millisecond)
	}
	d := &dump{info: map[string]*val{}, docs: map[string]*val{}}
	info, err := call("COMMAND")
	if err != nil {
		return nil, err
	}
	for _, e := range info.items {
		d.info[e.items[0].str] = e
	}
	docs, err := call("COMMAND", "DOCS")
	if err != nil {
		return nil, err
	}
	for i := 0; i+1 < len(docs.items); i += 2 {
		d.docs[docs.items[i].str] = docs.items[i+1]
	}
	return d, nil
}

func strs(v *val, trimAt bool) []string {
	out := make([]string, 0, len(v.items))
	for _, it := range v.items {
		s := it.str
		if trimAt {
			s = strings.TrimPrefix(s, "@")
		}
		out = append(out, s)
	}
	return out
}

func goStrings(ss []string) string {
	q := make([]string, len(ss))
	for i, s := range ss {
		q[i] = strconv.Quote(s)
	}
	return "[]string{" + strings.Join(q, ", ") + "}"
}

func main() {
	d74, err := fetch(redis74Image)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	d8, err := fetch(redis8Image)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for _, name := range redis8Commands {
		if d8.info[name] == nil || d8.docs[name] == nil {
			fmt.Fprintf(os.Stderr, "%s missing from %s\n", name, redis8Image)
			os.Exit(1)
		}
		d74.info[name], d74.docs[name] = d8.info[name], d8.docs[name]
	}
	names := make([]string, 0, len(d74.info))
	for n := range d74.info {
		names = append(names, n)
	}
	sort.Strings(names)

	var specs, subs, data bytes.Buffer
	specs.WriteString("package command\n\n// Code generated by scripts/gen-command-specs.go from the COMMAND reply of Redis 7.4; DO NOT EDIT.\n\n")
	specs.WriteString("// redisCommandSpecs holds the arity, flags, key positions and ACL categories\n// Redis reports for its own commands. Arity follows Redis: a positive value is\n// an exact argument count including the command name, a negative value a\n// minimum.\nvar redisCommandSpecs = map[string]commandSpec{\n")
	subs.WriteString("// redisSubcommandCategories holds the ACL categories of each subcommand of a\n// container command, keyed as \"CONTAINER|SUBCOMMAND\".\nvar redisSubcommandCategories = map[string][]string{\n")
	var subNames []string
	subCats := map[string][]string{}
	for _, n := range names {
		e := d74.info[n]
		fmt.Fprintf(&specs, "\t%q: {Arity: %d, Flags: %s, FirstKey: %d, LastKey: %d, KeyStep: %d, Categories: %s},\n",
			strings.ToUpper(n), e.items[1].num, goStrings(strs(e.items[2], false)), e.items[3].num, e.items[4].num, e.items[5].num,
			goStrings(strs(e.items[6], true)))
		for _, s := range e.items[9].items {
			up := strings.ToUpper(s.items[0].str)
			subNames = append(subNames, up)
			subCats[up] = strs(s.items[6], true)
		}
	}
	sort.Strings(subNames)
	for _, s := range subNames {
		fmt.Fprintf(&subs, "\t%q: %s,\n", s, strings.TrimPrefix(goStrings(subCats[s]), "[]string"))
	}
	specs.WriteString("}\n\n")
	subs.WriteString("}\n")
	specs.Write(subs.Bytes())

	data.WriteString("package command\n\n// Code generated by scripts/gen-command-specs.go from Redis 7.4; DO NOT EDIT.\n\n")
	data.WriteString("// redisCommandInfoRESP3 holds each command's COMMAND INFO entry, with its\n// subcommands, as Redis encodes it over RESP3.\nvar redisCommandInfoRESP3 = map[string]string{\n")
	for _, n := range names {
		fmt.Fprintf(&data, "\t%q: %s,\n", n, strconv.Quote(string(d74.info[n].raw)))
	}
	data.WriteString("}\n\n// redisCommandDocsRESP3 holds each command's COMMAND DOCS entry as Redis\n// encodes it over RESP3.\nvar redisCommandDocsRESP3 = map[string]string{\n")
	for _, n := range names {
		doc := d74.docs[n]
		if doc == nil {
			continue
		}
		fmt.Fprintf(&data, "\t%q: %s,\n", n, strconv.Quote(string(doc.raw)))
	}
	data.WriteString("}\n")

	for file, b := range map[string][]byte{"redis_command_specs.go": specs.Bytes(), "redis_command_data.go": data.Bytes()} {
		src, err := format.Source(b)
		if err != nil {
			fmt.Fprintln(os.Stderr, file, err)
			os.Exit(1)
		}
		if err := os.WriteFile(outDir+"/"+file, src, 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	fmt.Printf("%d commands, %d subcommands\n", len(names), len(subNames))
}
