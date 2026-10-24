//go:build compat

package compat

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

type Norm int

const (
	NormExact Norm = iota
	NormSorted
	NormSortedPairs
	NormTTL
	NormKind
	NormLen
	NormScanAll
	NormScanAllPairs
	NormNested
)

type Step struct {
	Conn     int
	Args     []string
	Norm     Norm
	Tol      int64
	SendOnly bool
	RecvOnly bool
	Sleep    time.Duration
	Timeout  time.Duration
	Raw      string
}

type Case struct {
	Group   string
	Name    string
	Steps   []Step
	NoState bool
	MultiDB bool
}

func c(args ...string) Step              { return Step{Args: args} }
func on(conn int, args ...string) Step   { return Step{Conn: conn, Args: args} }
func srt(args ...string) Step            { return Step{Args: args, Norm: NormSorted} }
func pairs(args ...string) Step          { return Step{Args: args, Norm: NormSortedPairs} }
func nested(args ...string) Step         { return Step{Args: args, Norm: NormNested} }
func ttl(tol int64, args ...string) Step { return Step{Args: args, Norm: NormTTL, Tol: tol} }
func kind(args ...string) Step           { return Step{Args: args, Norm: NormKind} }
func length(args ...string) Step         { return Step{Args: args, Norm: NormLen} }
func scanAll(args ...string) Step        { return Step{Args: args, Norm: NormScanAll} }
func scanAllPairs(args ...string) Step   { return Step{Args: args, Norm: NormScanAllPairs} }
func send(conn int, args ...string) Step { return Step{Conn: conn, Args: args, SendOnly: true} }
func recv(conn int) Step                 { return Step{Conn: conn, RecvOnly: true} }
func sleep(ms int) Step                  { return Step{Sleep: time.Duration(ms) * time.Millisecond} }

type Val struct {
	Kind byte
	Str  string
	Null bool
	Arr  []*Val
}

func (v *Val) Encode() []byte {
	var b bytes.Buffer
	v.encode(&b)
	return b.Bytes()
}

func (v *Val) encode(b *bytes.Buffer) {
	switch v.Kind {
	case '+', '-', ':':
		b.WriteByte(v.Kind)
		b.WriteString(v.Str)
		b.WriteString("\r\n")
	case '$':
		if v.Null {
			b.WriteString("$-1\r\n")
			return
		}
		fmt.Fprintf(b, "$%d\r\n%s\r\n", len(v.Str), v.Str)
	case '*':
		if v.Null {
			b.WriteString("*-1\r\n")
			return
		}
		fmt.Fprintf(b, "*%d\r\n", len(v.Arr))
		for _, e := range v.Arr {
			e.encode(b)
		}
	default:
		b.WriteByte(v.Kind)
		b.WriteString(v.Str)
		b.WriteString("\r\n")
	}
}

type recorder struct {
	r   *bufio.Reader
	buf bytes.Buffer
}

func (rc *recorder) line() (string, error) {
	s, err := rc.r.ReadString('\n')
	rc.buf.WriteString(s)
	if err != nil {
		return "", err
	}
	if !strings.HasSuffix(s, "\r\n") {
		return "", errors.New("line not CRLF terminated")
	}
	return s[:len(s)-2], nil
}

func (rc *recorder) read() (*Val, error) {
	l, err := rc.line()
	if err != nil {
		return nil, err
	}
	if l == "" {
		return nil, errors.New("empty line")
	}
	k, rest := l[0], l[1:]
	switch k {
	case '+', '-', ':', '_', '#', ',', '(':
		return &Val{Kind: k, Str: rest}, nil
	case '$', '=', '!':
		n, err := strconv.Atoi(rest)
		if err != nil {
			return nil, fmt.Errorf("bad bulk len %q", rest)
		}
		if n < 0 {
			return &Val{Kind: '$', Null: true}, nil
		}
		data := make([]byte, n+2)
		if _, err := ioReadFull(rc.r, data); err != nil {
			rc.buf.Write(data)
			return nil, err
		}
		rc.buf.Write(data)
		return &Val{Kind: '$', Str: string(data[:n])}, nil
	case '*', '~', '>', '%':
		n, err := strconv.Atoi(rest)
		if err != nil {
			return nil, fmt.Errorf("bad array len %q", rest)
		}
		if n < 0 {
			return &Val{Kind: '*', Null: true}, nil
		}
		if k == '%' {
			n *= 2
		}
		v := &Val{Kind: k}
		for i := 0; i < n; i++ {
			e, err := rc.read()
			if err != nil {
				return nil, err
			}
			v.Arr = append(v.Arr, e)
		}
		return v, nil
	}
	return nil, fmt.Errorf("unknown RESP type byte %q", k)
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

type Conn struct {
	addr string
	nc   net.Conn
	r    *bufio.Reader
}

func dial(addr string) (*Conn, error) {
	nc, err := net.DialTimeout("tcp", addr, 3*time.Second)
	if err != nil {
		return nil, err
	}
	return &Conn{addr: addr, nc: nc, r: bufio.NewReaderSize(nc, 1<<16)}, nil
}

func (c *Conn) Close() {
	if c != nil && c.nc != nil {
		_ = c.nc.Close()
	}
}

func encodeCmd(args []string) []byte {
	var b bytes.Buffer
	fmt.Fprintf(&b, "*%d\r\n", len(args))
	for _, a := range args {
		fmt.Fprintf(&b, "$%d\r\n%s\r\n", len(a), a)
	}
	return b.Bytes()
}

func (c *Conn) Send(args []string) error {
	_ = c.nc.SetWriteDeadline(time.Now().Add(3 * time.Second))
	_, err := c.nc.Write(encodeCmd(args))
	return err
}

type Reply struct {
	Raw []byte
	Val *Val
	Err string
}

func (r Reply) Display() string {
	s := strconv.Quote(string(r.Raw))
	if len(s) > 600 {
		s = s[:600] + "...(truncated)"
	}
	if r.Err != "" {
		if len(r.Raw) == 0 {
			return "<" + r.Err + ">"
		}
		return "<" + r.Err + "> partial=" + s
	}
	return s
}

func (c *Conn) Recv(timeout time.Duration) Reply {
	_ = c.nc.SetReadDeadline(time.Now().Add(timeout))
	rc := &recorder{r: c.r}
	v, err := rc.read()
	if err != nil {
		var ne net.Error
		msg := err.Error()
		if errors.As(err, &ne) && ne.Timeout() {
			msg = fmt.Sprintf("TIMEOUT after %v", timeout)
		}
		return Reply{Raw: rc.buf.Bytes(), Err: msg}
	}
	return Reply{Raw: rc.buf.Bytes(), Val: v}
}

func (c *Conn) Do(timeout time.Duration, args ...string) Reply {
	if err := c.Send(args); err != nil {
		return Reply{Err: "write: " + err.Error()}
	}
	return c.Recv(timeout)
}

type Target struct {
	Name  string
	Addr  string
	admin *Conn
	conns map[int]*Conn
}

func (t *Target) conn(i int) (*Conn, error) {
	if c, ok := t.conns[i]; ok && c != nil {
		return c, nil
	}
	c, err := dial(t.Addr)
	if err != nil {
		return nil, err
	}
	t.conns[i] = c
	return c, nil
}

func (t *Target) resetConns() {
	for _, c := range t.conns {
		c.Close()
	}
	t.conns = map[int]*Conn{}
}

func (t *Target) adminDo(args ...string) Reply {
	if t.admin == nil {
		c, err := dial(t.Addr)
		if err != nil {
			return Reply{Err: "dial: " + err.Error()}
		}
		t.admin = c
	}
	r := t.admin.Do(5*time.Second, args...)
	if r.Err != "" {
		t.admin.Close()
		t.admin = nil
	}
	return r
}

type StepResult struct {
	Step  Step
	Redis Reply
	CS    Reply
	Match bool
}

type CaseResult struct {
	Case       Case
	Steps      []StepResult
	StateRedis string
	StateCS    string
	StateMatch bool
	Duration   time.Duration
}

func (cr CaseResult) Pass() bool {
	if !cr.StateMatch {
		return false
	}
	for _, s := range cr.Steps {
		if !s.Match {
			return false
		}
	}
	return true
}

func normalize(step Step, r Reply) []byte {
	if r.Err != "" || r.Val == nil {
		return append([]byte("ERR:"+r.Err+":"), r.Raw...)
	}
	v := r.Val
	switch step.Norm {
	case NormSorted:
		if v.Kind == '*' && !v.Null {
			cp := *v
			cp.Arr = append([]*Val(nil), v.Arr...)
			sort.Slice(cp.Arr, func(i, j int) bool { return bytes.Compare(cp.Arr[i].Encode(), cp.Arr[j].Encode()) < 0 })
			return cp.Encode()
		}
	case NormSortedPairs:
		if v.Kind == '*' && !v.Null && len(v.Arr)%2 == 0 {
			type pr struct{ k, v *Val }
			var ps []pr
			for i := 0; i < len(v.Arr); i += 2 {
				ps = append(ps, pr{v.Arr[i], v.Arr[i+1]})
			}
			sort.Slice(ps, func(i, j int) bool { return bytes.Compare(ps[i].k.Encode(), ps[j].k.Encode()) < 0 })
			cp := &Val{Kind: '*'}
			for _, p := range ps {
				cp.Arr = append(cp.Arr, p.k, p.v)
			}
			return cp.Encode()
		}
	case NormNested:
		return sortDeep(v).Encode()
	case NormKind:
		return []byte{v.Kind, '0' + boolByte(v.Null)}
	case NormLen:
		return []byte(fmt.Sprintf("%c%d", v.Kind, len(v.Arr)))
	}
	return r.Raw
}

func sortDeep(v *Val) *Val {
	if v.Kind != '*' || v.Null {
		return v
	}
	cp := &Val{Kind: '*'}
	for _, e := range v.Arr {
		cp.Arr = append(cp.Arr, sortDeep(e))
	}
	sort.Slice(cp.Arr, func(i, j int) bool { return bytes.Compare(cp.Arr[i].Encode(), cp.Arr[j].Encode()) < 0 })
	return cp
}

func boolByte(b bool) byte {
	if b {
		return 1
	}
	return 0
}

func compare(step Step, a, b Reply) bool {
	if step.Norm == NormTTL && a.Err == "" && b.Err == "" && a.Val != nil && b.Val != nil && a.Val.Kind == ':' && b.Val.Kind == ':' {
		x, e1 := strconv.ParseInt(a.Val.Str, 10, 64)
		y, e2 := strconv.ParseInt(b.Val.Str, 10, 64)
		if e1 == nil && e2 == nil {
			if x < 0 || y < 0 {
				return x == y
			}
			d := x - y
			if d < 0 {
				d = -d
			}
			return d <= step.Tol
		}
	}
	return bytes.Equal(normalize(step, a), normalize(step, b))
}

func scanCursorIndex(cmd string) int {
	if strings.EqualFold(cmd, "SCAN") {
		return 1
	}
	return 2
}

func (t *Target) scanAll(conn *Conn, step Step, timeout time.Duration) Reply {
	args := append([]string(nil), step.Args...)
	ci := scanCursorIndex(args[0])
	args[ci] = "0"
	var all []*Val
	var raw bytes.Buffer
	for i := 0; i < 10000; i++ {
		r := conn.Do(timeout, args...)
		if r.Err != "" || r.Val == nil || r.Val.Kind != '*' || len(r.Val.Arr) != 2 || r.Val.Arr[1].Kind != '*' {
			return r
		}
		all = append(all, r.Val.Arr[1].Arr...)
		cur := r.Val.Arr[0].Str
		if cur == "0" {
			break
		}
		args[ci] = cur
	}
	if step.Norm == NormScanAllPairs && len(all)%2 == 0 {
		type pr struct{ k, v string }
		var ps []pr
		for i := 0; i < len(all); i += 2 {
			ps = append(ps, pr{string(all[i].Encode()), string(all[i+1].Encode())})
		}
		sort.Slice(ps, func(i, j int) bool { return ps[i].k < ps[j].k })
		fmt.Fprintf(&raw, "*%d\r\n", len(all))
		for _, p := range ps {
			raw.WriteString(p.k + p.v)
		}
	} else {
		var ss []string
		seen := map[string]bool{}
		for _, e := range all {
			k := string(e.Encode())
			if !seen[k] {
				seen[k] = true
				ss = append(ss, k)
			}
		}
		sort.Strings(ss)
		fmt.Fprintf(&raw, "*%d\r\n", len(ss))
		for _, s := range ss {
			raw.WriteString(s)
		}
	}
	return Reply{Raw: raw.Bytes(), Val: &Val{Kind: '*'}}
}

func (t *Target) runStep(step Step) Reply {
	timeout := step.Timeout
	if timeout == 0 {
		timeout = 3 * time.Second
	}
	conn, err := t.conn(step.Conn)
	if err != nil {
		return Reply{Err: "dial: " + err.Error()}
	}
	var r Reply
	switch {
	case step.Norm == NormScanAll || step.Norm == NormScanAllPairs:
		r = t.scanAll(conn, step, timeout)
	case step.Raw != "":
		_ = conn.nc.SetWriteDeadline(time.Now().Add(3 * time.Second))
		if _, err := conn.nc.Write([]byte(step.Raw)); err != nil {
			r = Reply{Err: "write: " + err.Error()}
		} else {
			r = conn.Recv(timeout)
		}
	case step.SendOnly:
		if err := conn.Send(step.Args); err != nil {
			r = Reply{Err: "write: " + err.Error()}
		} else {
			r = Reply{Raw: []byte("(sent)")}
		}
	case step.RecvOnly:
		r = conn.Recv(timeout)
	default:
		r = conn.Do(timeout, step.Args...)
	}
	if r.Err != "" {
		conn.Close()
		delete(t.conns, step.Conn)
	}
	return r
}

var stateCmds = map[string][]string{
	"string": {"GET"},
	"list":   {"LRANGE", "", "0", "-1"},
	"hash":   {"HGETALL"},
	"set":    {"SMEMBERS"},
	"zset":   {"ZRANGE", "", "0", "-1", "WITHSCORES"},
	"stream": {"XRANGE", "", "-", "+"},
}

func (t *Target) dumpState(dbs []string) string {
	var b strings.Builder
	for _, db := range dbs {
		if r := t.adminDo("SELECT", db); r.Err != "" {
			return "select failed: " + r.Display()
		}
		cursor := "0"
		var keys []string
		for i := 0; i < 10000; i++ {
			r := t.adminDo("SCAN", cursor, "COUNT", "1000")
			if r.Err != "" || r.Val == nil || r.Val.Kind != '*' || len(r.Val.Arr) != 2 {
				fmt.Fprintf(&b, "db%s scan failed: %s\n", db, r.Display())
				break
			}
			for _, k := range r.Val.Arr[1].Arr {
				keys = append(keys, k.Str)
			}
			cursor = r.Val.Arr[0].Str
			if cursor == "0" {
				break
			}
		}
		sort.Strings(keys)
		keys = dedupe(keys)
		for _, k := range keys {
			tr := t.adminDo("TYPE", k)
			typ := ""
			if tr.Val != nil {
				typ = tr.Val.Str
			}
			fmt.Fprintf(&b, "db%s %q type=%s", db, k, typ)
			if tmpl, ok := stateCmds[typ]; ok {
				args := append([]string(nil), tmpl...)
				if len(args) == 1 {
					args = append(args, k)
				} else {
					args[1] = k
				}
				step := Step{}
				if typ == "hash" {
					step.Norm = NormSortedPairs
				}
				if typ == "set" {
					step.Norm = NormSorted
				}
				v := t.adminDo(args...)
				fmt.Fprintf(&b, " value=%s", strconv.Quote(string(normalize(step, v))))
			}
			pt := t.adminDo("PTTL", k)
			if pt.Val != nil {
				n, _ := strconv.ParseInt(pt.Val.Str, 10, 64)
				switch {
				case n > 0:
					b.WriteString(" ttl=yes")
				default:
					fmt.Fprintf(&b, " ttl=%d", n)
				}
			}
			b.WriteString("\n")
		}
	}
	t.adminDo("SELECT", "0")
	return b.String()
}

func dedupe(s []string) []string {
	var out []string
	for i, x := range s {
		if i == 0 || x != s[i-1] {
			out = append(out, x)
		}
	}
	return out
}

func runCase(redis, cs *Target, cs0 Case) CaseResult {
	start := time.Now()
	res := CaseResult{Case: cs0}
	for _, t := range []*Target{redis, cs} {
		t.resetConns()
		t.adminDo("FLUSHALL")
		t.adminDo("SCRIPT", "FLUSH")
	}
	for _, st := range cs0.Steps {
		if st.Sleep > 0 && len(st.Args) == 0 && !st.RecvOnly {
			time.Sleep(st.Sleep)
			continue
		}
		var rr, cr Reply
		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); rr = redis.runStep(st) }()
		go func() { defer wg.Done(); cr = cs.runStep(st) }()
		wg.Wait()
		res.Steps = append(res.Steps, StepResult{Step: st, Redis: rr, CS: cr, Match: compare(st, rr, cr)})
	}
	redis.resetConns()
	cs.resetConns()
	if cs0.NoState {
		res.StateMatch = true
	} else {
		dbs := []string{"0"}
		if cs0.MultiDB {
			dbs = append(dbs, "1")
		}
		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); res.StateRedis = redis.dumpState(dbs) }()
		go func() { defer wg.Done(); res.StateCS = cs.dumpState(dbs) }()
		wg.Wait()
		res.StateMatch = res.StateRedis == res.StateCS
	}
	res.Duration = time.Since(start)
	return res
}

func freePort(t *testing.T) int {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("free port: %v", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func waitReady(addr string, d time.Duration) error {
	deadline := time.Now().Add(d)
	var last error
	for time.Now().Before(deadline) {
		c, err := dial(addr)
		if err == nil {
			r := c.Do(time.Second, "PING")
			c.Close()
			if r.Err == "" && string(r.Raw) == "+PONG\r\n" {
				return nil
			}
			last = fmt.Errorf("ping: %s", r.Display())
		} else {
			last = err
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("not ready: %v", last)
}

func startRedis(t *testing.T) string {
	if a := os.Getenv("COMPAT_REDIS_ADDR"); a != "" {
		return a
	}
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not available and COMPAT_REDIS_ADDR not set")
	}
	image := os.Getenv("COMPAT_REDIS_IMAGE")
	if image == "" {
		image = "redis:7.4-alpine"
	}
	port := freePort(t)
	name := fmt.Sprintf("cachestorm-compat-%d-%d", os.Getpid(), port)
	out, err := exec.Command("docker", "run", "-d", "--rm", "--name", name,
		"-p", fmt.Sprintf("127.0.0.1:%d:6379", port), image,
		"redis-server", "--save", "", "--appendonly", "no").CombinedOutput()
	if err != nil {
		t.Skipf("docker run failed: %v: %s", err, out)
	}
	t.Cleanup(func() { _ = exec.Command("docker", "rm", "-f", name).Run() })
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	if err := waitReady(addr, 30*time.Second); err != nil {
		t.Fatalf("redis: %v", err)
	}
	return addr
}

func startCacheStorm(t *testing.T) string {
	if a := os.Getenv("COMPAT_CACHESTORM_ADDR"); a != "" {
		return a
	}
	dir := t.TempDir()
	bin := os.Getenv("COMPAT_CACHESTORM_BIN")
	if bin == "" {
		bin = filepath.Join(dir, "cachestorm")
		root, _ := filepath.Abs("../..")
		cmd := exec.Command("go", "build", "-o", bin, "./cmd/cachestorm")
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("build cachestorm: %v: %s", err, out)
		}
	}
	port := freePort(t)
	sport := freePort(t)
	cfg := fmt.Sprintf(`server:
  bind: 127.0.0.1
  port: %d
http:
  enabled: false
persistence:
  enabled: false
  data_dir: %s
plugins:
  metrics:
    enabled: false
sentinel:
  addr: 127.0.0.1
  port: %d
logging:
  level: error
`, port, dir, sport)
	cfgPath := filepath.Join(dir, "cachestorm.yaml")
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	logf, _ := os.Create(filepath.Join(dir, "cachestorm.log"))
	cmd := exec.Command(bin, "-config", cfgPath)
	cmd.Stdout = logf
	cmd.Stderr = logf
	if err := cmd.Start(); err != nil {
		t.Fatalf("start cachestorm: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
		logf.Close()
	})
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	if err := waitReady(addr, 15*time.Second); err != nil {
		t.Fatalf("cachestorm: %v", err)
	}
	return addr
}

func stepString(s Step) string {
	if s.Sleep > 0 && len(s.Args) == 0 {
		return fmt.Sprintf("(sleep %v)", s.Sleep)
	}
	prefix := fmt.Sprintf("c%d> ", s.Conn)
	if s.RecvOnly {
		return prefix + "(read next frame)"
	}
	if s.Raw != "" {
		return prefix + "(raw bytes) " + strconv.Quote(s.Raw)
	}
	var parts []string
	for _, a := range s.Args {
		if a == "" || strings.ContainsAny(a, " \"\r\n\t\x00") || !isPrintable(a) {
			parts = append(parts, strconv.Quote(a))
		} else {
			parts = append(parts, a)
		}
	}
	tag := ""
	switch s.Norm {
	case NormSorted:
		tag = "  [sorted]"
	case NormSortedPairs:
		tag = "  [sorted pairs]"
	case NormTTL:
		tag = fmt.Sprintf("  [±%d]", s.Tol)
	case NormKind:
		tag = "  [type only]"
	case NormLen:
		tag = "  [length only]"
	case NormScanAll, NormScanAllPairs:
		tag = "  [full scan, sorted]"
	case NormNested:
		tag = "  [deep sorted]"
	}
	if s.SendOnly {
		tag += "  [send, read later]"
	}
	return prefix + strings.Join(parts, " ") + tag
}

func isPrintable(s string) bool {
	for _, r := range s {
		if r < 0x20 || r == 0x7f || r == 0xfffd {
			return false
		}
	}
	return true
}

func truncLines(s string) string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if len(l) > 400 {
			l = l[:400] + "...(truncated)"
		}
		out = append(out, l)
	}
	return strings.Join(out, "\n")
}

func writeCase(b *strings.Builder, r CaseResult) {
	b.WriteString("```\n")
	for _, s := range r.Steps {
		mark := "  "
		if !s.Match {
			mark = "!!"
		}
		fmt.Fprintf(b, "%s %s\n", mark, stepString(s.Step))
		if !s.Match {
			fmt.Fprintf(b, "     redis:  %s\n     cstorm: %s\n", s.Redis.Display(), s.CS.Display())
		} else if !s.Step.SendOnly {
			fmt.Fprintf(b, "     both:   %s\n", s.Redis.Display())
		}
	}
	if !r.StateMatch {
		fmt.Fprintf(b, "!! STATE DIFF (keyspace after the case)\n--- redis\n%s--- cachestorm\n%s", truncLines(r.StateRedis), truncLines(r.StateCS))
	}
	b.WriteString("```\n\n")
}

func writeReport(path, redisVersion string, results []CaseResult) error {
	var b strings.Builder
	pass := 0
	bySev := map[int][]CaseResult{}
	classCount := map[string]int{}
	for _, r := range results {
		if r.Pass() {
			pass++
			continue
		}
		v := verdictFor(r.Case.Group + "/" + r.Case.Name)
		bySev[v.Sev] = append(bySev[v.Sev], r)
		if v.Class == "" {
			classCount["unclassified"]++
		} else {
			classCount[v.Class]++
		}
	}
	fmt.Fprintf(&b, "Redis version: %s\n\nCases: %d, pass: %d, fail: %d\n\n", redisVersion, len(results), pass, len(results)-pass)
	fmt.Fprintf(&b, "| Severity | Failing cases |\n|---|---|\n")
	for _, sev := range []int{SevCritical, SevCommon, SevEdge, 0} {
		fmt.Fprintf(&b, "| %s | %d |\n", sevTitle[sev], len(bySev[sev]))
	}
	fmt.Fprintf(&b, "\nBy classification: bug %d, intentional difference %d, harness noise %d, unclassified %d\n\n", classCount[bug], classCount[intent], classCount[noise], classCount["unclassified"])
	for _, sev := range []int{SevCritical, SevCommon, SevEdge, 0} {
		rs := bySev[sev]
		if len(rs) == 0 {
			continue
		}
		fmt.Fprintf(&b, "## %s\n\n", sevTitle[sev])
		for _, r := range rs {
			name := r.Case.Group + "/" + r.Case.Name
			v := verdictFor(name)
			class := v.Class
			if class == "" {
				class = "unclassified"
			}
			fmt.Fprintf(&b, "### %s\n\n**%s**: %s\n\n", name, class, v.Note)
			writeCase(&b, r)
		}
	}
	b.WriteString("## Passing cases\n\n")
	var names []string
	for _, r := range results {
		if r.Pass() {
			names = append(names, "`"+r.Case.Group+"/"+r.Case.Name+"`")
		}
	}
	b.WriteString(strings.Join(names, ", ") + "\n")
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func TestCompat(t *testing.T) {
	redisAddr := startRedis(t)
	csAddr := startCacheStorm(t)
	redis := &Target{Name: "redis", Addr: redisAddr, conns: map[int]*Conn{}}
	cs := &Target{Name: "cachestorm", Addr: csAddr, conns: map[int]*Conn{}}
	defer redis.resetConns()
	defer cs.resetConns()

	ver := "unknown"
	if r := redis.adminDo("INFO", "server"); r.Val != nil {
		for _, l := range strings.Split(r.Val.Str, "\r\n") {
			if strings.HasPrefix(l, "redis_version:") {
				ver = strings.TrimPrefix(l, "redis_version:")
			}
		}
	}
	t.Logf("redis %s at %s, cachestorm at %s", ver, redisAddr, csAddr)

	filter := os.Getenv("COMPAT_FILTER")
	var results []CaseResult
	for _, cc := range corpus() {
		full := cc.Group + "/" + cc.Name
		if filter != "" && !strings.Contains(full, filter) {
			continue
		}
		cc := cc
		t.Run(full, func(t *testing.T) {
			r := runCase(redis, cs, cc)
			results = append(results, r)
			if !r.Pass() {
				for _, s := range r.Steps {
					if !s.Match {
						t.Errorf("%s\n  redis:  %s\n  cstorm: %s", stepString(s.Step), s.Redis.Display(), s.CS.Display())
					}
				}
				if !r.StateMatch {
					t.Errorf("state differs\n--- redis\n%s--- cachestorm\n%s", r.StateRedis, r.StateCS)
				}
			}
		})
	}
	out := os.Getenv("COMPAT_OUT")
	if out == "" {
		out = filepath.Join(os.TempDir(), "cachestorm-compat-results.md")
	}
	if err := writeReport(out, ver, results); err != nil {
		t.Logf("write report: %v", err)
	}
	pass := 0
	for _, r := range results {
		if r.Pass() {
			pass++
		}
	}
	t.Logf("cases=%d pass=%d fail=%d report=%s", len(results), pass, len(results)-pass, out)
}
