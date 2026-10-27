package command

import (
	"bytes"
	"strconv"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func auditD2Router() *Router {
	r := NewRouter()
	RegisterEncodingCommands(r)
	RegisterTemplateCommands(r)
	return r
}

func auditD2Run(t *testing.T, cmd string, args ...[]byte) string {
	t.Helper()
	ctx := newTestContext(cmd, args, store.NewStore())
	buf := &bytes.Buffer{}
	ctx.Writer = resp.NewWriter(buf)
	_ = auditD2Router().Execute(ctx)
	return buf.String()
}

func auditD2Str(t *testing.T, cmd string, args ...string) string {
	t.Helper()
	b := make([][]byte, len(args))
	for i, a := range args {
		b[i] = []byte(a)
	}
	return auditD2Run(t, cmd, b...)
}

func bulk(s string) string {
	return "$" + itoaD2(len(s)) + "\r\n" + s + "\r\n"
}

func itoaD2(n int) string {
	return strconv.Itoa(n)
}

// bulkPayload extracts the payload of a single bulk-string reply.
func bulkPayload(t *testing.T, reply string) []byte {
	t.Helper()
	if !strings.HasPrefix(reply, "$") {
		t.Fatalf("expected bulk reply, got %q", reply)
	}
	i := strings.Index(reply, "\r\n")
	return []byte(reply[i+2 : len(reply)-2])
}

func TestAuditD2BinaryCodecRoundTrip(t *testing.T) {
	docs := []struct{ in, out string }{
		{`null`, `null`},
		{`true`, `true`},
		{`"héllo <&>"`, `"héllo <&>"`},
		{`0`, `0`},
		{`-1`, `-1`},
		{`-33`, `-33`},
		{`-129`, `-129`},
		{`-40000`, `-40000`},
		{`-3000000000`, `-3000000000`},
		{`-9223372036854775808`, `-9223372036854775808`},
		{`255`, `255`},
		{`65536`, `65536`},
		{`18446744073709551615`, `18446744073709551615`},
		{`1.0`, `1.0`},
		{`1.5e300`, `1.5e+300`},
		{`-0.25`, `-0.25`},
		{`[]`, `[]`},
		{`{}`, `{}`},
		{`{"b":1,"a":[1,2.5,{"x":null}],"c":{"d":[true,false]}}`, `{"b":1,"a":[1,2.5,{"x":null}],"c":{"d":[true,false]}}`},
		{`["` + strings.Repeat("x", 40) + `","` + strings.Repeat("y", 300) + `","` + strings.Repeat("z", 70000) + `"]`,
			`["` + strings.Repeat("x", 40) + `","` + strings.Repeat("y", 300) + `","` + strings.Repeat("z", 70000) + `"]`},
	}
	for _, format := range []string{"MSGPACK", "CBOR"} {
		for _, d := range docs {
			enc := auditD2Str(t, format+".ENCODE", d.in)
			got := auditD2Run(t, format+".DECODE", bulkPayload(t, enc))
			if got != bulk(d.out) {
				t.Errorf("%s round trip of %.60q: got %.80q", format, d.in, got)
			}
		}
	}

	big16 := `[` + strings.TrimSuffix(strings.Repeat("1,", 20), ",") + `]`
	for _, format := range []string{"MSGPACK", "CBOR"} {
		enc := auditD2Str(t, format+".ENCODE", big16)
		if got := auditD2Run(t, format+".DECODE", bulkPayload(t, enc)); got != bulk(big16) {
			t.Errorf("%s array of 20: %q", format, got)
		}
	}
}

func TestAuditD2BinaryCodecWireFormat(t *testing.T) {
	cases := []struct {
		cmd, in string
		want    []byte
	}{
		{"MSGPACK.ENCODE", `{"key":"value"}`, []byte{0x81, 0xa3, 'k', 'e', 'y', 0xa5, 'v', 'a', 'l', 'u', 'e'}},
		{"MSGPACK.ENCODE", `[1,-1,null,true]`, []byte{0x94, 0x01, 0xff, 0xc0, 0xc3}},
		{"MSGPACK.ENCODE", `200`, []byte{0xcc, 0xc8}},
		{"CBOR.ENCODE", `{"a":[1,-1]}`, []byte{0xa1, 0x61, 'a', 0x82, 0x01, 0x20}},
		{"CBOR.ENCODE", `[null,false,true,24]`, []byte{0x84, 0xf6, 0xf4, 0xf5, 0x18, 0x18}},
		{"CBOR.ENCODE", `-18446744073709551616`, []byte{0x3b, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}},
	}
	for _, c := range cases {
		if got := bulkPayload(t, auditD2Str(t, c.cmd, c.in)); !bytes.Equal(got, c.want) {
			t.Errorf("%s %s = % x, want % x", c.cmd, c.in, got, c.want)
		}
	}
}

func TestAuditD2BinaryCodecDecodeForeign(t *testing.T) {
	cases := []struct {
		cmd  string
		in   []byte
		want string
	}{
		{"CBOR.DECODE", []byte{0xbf, 0xff}, `{}`},
		{"CBOR.DECODE", []byte{0x9f, 0x01, 0x9f, 0xff, 0xff}, `[1,[]]`},
		{"CBOR.DECODE", []byte{0x7f, 0x62, 'a', 'b', 0x61, 'c', 0xff}, `"abc"`},
		{"CBOR.DECODE", []byte{0xf9, 0x3c, 0x00}, `1.0`},
		{"CBOR.DECODE", []byte{0xf9, 0xc4, 0x00}, `-4.0`},
		{"CBOR.DECODE", []byte{0xfa, 0x3f, 0xc0, 0x00, 0x00}, `1.5`},
		{"MSGPACK.DECODE", []byte{0xca, 0x3f, 0xc0, 0x00, 0x00}, `1.5`},
		{"MSGPACK.DECODE", []byte{0xcf, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}, `18446744073709551615`},
		{"MSGPACK.DECODE", []byte{0xd3, 0x80, 0, 0, 0, 0, 0, 0, 0}, `-9223372036854775808`},
		{"MSGPACK.DECODE", []byte{0xa2, '"', '\\'}, `"\"\\"`},
	}
	for _, c := range cases {
		if got := auditD2Run(t, c.cmd, c.in); got != bulk(c.want) {
			t.Errorf("%s % x = %q, want %q", c.cmd, c.in, got, c.want)
		}
	}
}

func TestAuditD2BinaryCodecErrors(t *testing.T) {
	cases := []struct {
		cmd string
		in  []byte
	}{
		{"MSGPACK.ENCODE", []byte(`{"a":`)},
		{"MSGPACK.ENCODE", []byte(`1 2`)},
		{"MSGPACK.ENCODE", []byte(`18446744073709551616`)},
		{"MSGPACK.ENCODE", []byte(`-9223372036854775809`)},
		{"MSGPACK.ENCODE", []byte(`1e400`)},
		{"CBOR.ENCODE", []byte(`-18446744073709551617`)},
		{"MSGPACK.DECODE", []byte{}},
		{"MSGPACK.DECODE", []byte{0xc1}},
		{"MSGPACK.DECODE", []byte{0xc4, 0x01, 0x00}},
		{"MSGPACK.DECODE", []byte{0xda, 0x00, 0x10}},
		{"MSGPACK.DECODE", []byte{0x01, 0x02}},
		{"MSGPACK.DECODE", []byte{0x81, 0x01, 0x01}},
		{"MSGPACK.DECODE", []byte{0xa1, 0xff}},
		{"MSGPACK.DECODE", []byte{0xdd, 0xff, 0xff, 0xff, 0xff}},
		{"MSGPACK.DECODE", []byte{0xcb, 0x7f, 0xf8, 0, 0, 0, 0, 0, 0}},
		{"MSGPACK.DECODE", bytes.Repeat([]byte{0x91}, 1000)},
		{"CBOR.DECODE", []byte(`invalid cbor`)},
		{"CBOR.DECODE", []byte{0x41, 0x00}},
		{"CBOR.DECODE", []byte{0xc0, 0x60}},
		{"CBOR.DECODE", []byte{0xf7}},
		{"CBOR.DECODE", []byte{0xff}},
		{"CBOR.DECODE", []byte{0x81, 0xff}},
		{"CBOR.DECODE", []byte{0x9f, 0x81, 0xff, 0xff}},
		{"CBOR.DECODE", []byte{0xbf, 0x61, 'a', 0xff}},
		{"CBOR.DECODE", []byte{0xa1, 0x01, 0x01}},
		{"CBOR.DECODE", []byte{0x1c}},
		{"CBOR.DECODE", []byte{0x9b, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}},
		{"CBOR.DECODE", []byte{0xf9, 0x7c, 0x00}},
		{"CBOR.DECODE", []byte{0x01, 0x01}},
	}
	for _, c := range cases {
		if got := auditD2Run(t, c.cmd, c.in); !strings.HasPrefix(got, "-ERR") {
			t.Errorf("%s % x: expected error, got %q", c.cmd, c.in, got)
		}
	}
}

func TestAuditD2YAML(t *testing.T) {
	in := `{"name":"x","n":3,"f":2.0,"s":"true","e":"","nil":null,"list":[1,"2",{"k":"multi\nline"}],"o":{}}`
	enc := string(bulkPayload(t, auditD2Str(t, "YAML.ENCODE", in)))
	if !strings.HasPrefix(enc, "name: x\n") || !strings.Contains(enc, `s: "true"`) {
		t.Errorf("unexpected YAML: %q", enc)
	}
	if got := auditD2Str(t, "YAML.DECODE", enc); got != bulk(in) {
		t.Errorf("YAML round trip: got %q from %q", got, enc)
	}

	decodes := []struct{ in, want string }{
		{"a: 1\nb: [x, 2.5, ~]\nc:\n  d: yes\n", `{"a":1,"b":["x",2.5,null],"c":{"d":"yes"}}`},
		{"base: &b {x: 1}\nref: *b\n", `{"base":{"x":1},"ref":{"x":1}}`},
		{"0x1F", `31`},
		{"", `null`},
		{"- 2001-12-14\n- true\n", `["2001-12-14",true]`},
	}
	for _, d := range decodes {
		if got := auditD2Str(t, "YAML.DECODE", d.in); got != bulk(d.want) {
			t.Errorf("YAML.DECODE %q = %q, want %q", d.in, got, d.want)
		}
	}

	for _, bad := range []string{"a: [1", "a: 1\n---\nb: 2\n", "x: !!binary aGk=\n", "? [1]\n: 2\n", "<<: {a: 1}\n", ".inf", "a: !custom 1\n"} {
		if got := auditD2Str(t, "YAML.DECODE", bad); !strings.HasPrefix(got, "-ERR") {
			t.Errorf("YAML.DECODE %q: expected error, got %q", bad, got)
		}
	}
	if got := auditD2Str(t, "YAML.ENCODE", "not json"); !strings.HasPrefix(got, "-ERR") {
		t.Errorf("YAML.ENCODE invalid JSON: %q", got)
	}
}

func TestAuditD2TOMLRemoved(t *testing.T) {
	r := auditD2Router()
	for _, name := range []string{"TOML.ENCODE", "TOML.DECODE"} {
		if _, ok := r.Get(name); ok {
			t.Errorf("%s should not be registered", name)
		}
	}
}

func TestAuditD2XML(t *testing.T) {
	enc := auditD2Str(t, "XML.ENCODE", "item", "a<b & \"c\"\r\n")
	if enc != bulk("<item>a&lt;b &amp; &quot;c&quot;&#xD;\n</item>") {
		t.Errorf("XML.ENCODE = %q", enc)
	}
	want := "*2\r\n" + bulk("item") + bulk("a<b & \"c\"\r\n")
	if got := auditD2Run(t, "XML.DECODE", bulkPayload(t, enc)); got != want {
		t.Errorf("XML round trip = %q", got)
	}

	decodes := []struct{ in, tag, val string }{
		{`<?xml version="1.0"?><!-- c --><a>x<![CDATA[<y>]]>&#65;</a>`, "a", "x<y>A"},
		{`<empty/>`, "empty", ""},
		{"  <a> sp </a>\n", "a", " sp "},
	}
	for _, d := range decodes {
		want := "*2\r\n" + bulk(d.tag) + bulk(d.val)
		if got := auditD2Str(t, "XML.DECODE", d.in); got != want {
			t.Errorf("XML.DECODE %q = %q, want %q", d.in, got, want)
		}
	}

	for _, bad := range []string{`<a>x</b>`, `<a>`, `x`, `<a><b/></a>`, `<a k="v">x</a>`, `<a>x</a><b/>`, `<a>&bogus;</a>`, `<a>x</a>tail`, `<p:a>x</p:a>`} {
		if got := auditD2Str(t, "XML.DECODE", bad); !strings.HasPrefix(got, "-ERR") {
			t.Errorf("XML.DECODE %q: expected error, got %q", bad, got)
		}
	}
	for _, bad := range [][2]string{{"a b", "x"}, {"1a", "x"}, {"a>", "x"}, {"", "x"}, {"a", "bad\x01"}, {"a", "\xff"}} {
		if got := auditD2Str(t, "XML.ENCODE", bad[0], bad[1]); !strings.HasPrefix(got, "-ERR") {
			t.Errorf("XML.ENCODE %q: expected error, got %q", bad, got)
		}
	}
}

func diffReply(added, removed, unchanged int, lines ...string) string {
	var sb strings.Builder
	sb.WriteString("*8\r\n" + bulk("added") + ":" + itoaD2(added) + "\r\n" + bulk("removed") + ":" + itoaD2(removed) + "\r\n" + bulk("unchanged") + ":" + itoaD2(unchanged) + "\r\n" + bulk("diff"))
	sb.WriteString("*" + itoaD2(len(lines)) + "\r\n")
	for _, l := range lines {
		sb.WriteString(bulk(l))
	}
	return sb.String()
}

func TestAuditD2DiffText(t *testing.T) {
	cases := []struct {
		a, b string
		want string
	}{
		{"a\nb\nc", "a\nx\nc", diffReply(1, 1, 2, " a", "-b", "+x", " c")},
		{"a\na\nb", "a\nb", diffReply(0, 1, 2, " a", "-a", " b")},
		{"", "", diffReply(0, 0, 0)},
		{"", "x\ny\n", diffReply(2, 0, 0, "+x", "+y")},
		{"x\n", "x", diffReply(0, 0, 1, " x")},
		{"a\nb\nc\nd", "b\nc\na\nd", diffReply(1, 1, 3, "-a", " b", " c", "+a", " d")},
		{"x\ny", "y\nx", diffReply(1, 1, 1, "-x", " y", "+x")},
	}
	for _, c := range cases {
		if got := auditD2Str(t, "DIFF.TEXT", c.a, c.b); got != c.want {
			t.Errorf("DIFF.TEXT %q %q =\n%q\nwant\n%q", c.a, c.b, got, c.want)
		}
	}

	many := strings.Repeat("a\n", 3000)
	if got := auditD2Str(t, "DIFF.TEXT", many, strings.Repeat("b\n", 3000)); !strings.HasPrefix(got, "-ERR") {
		t.Errorf("expected too-large error, got %.60q", got)
	}
	if got := auditD2Str(t, "DIFF.TEXT", many+"z\n", many); !strings.HasPrefix(got, "*8") {
		t.Errorf("large but similar texts should diff, got %.60q", got)
	}
}

func TestAuditD2TimestampPeriods(t *testing.T) {
	const ts = "1709913600" // Fri 2024-03-08 16:00:00 UTC
	cases := []struct {
		cmd, unit, want string
	}{
		{"TIMESTAMP.STARTOF", "minute", "1709913600"},
		{"TIMESTAMP.ENDOF", "minute", "1709913659"},
		{"TIMESTAMP.STARTOF", "hour", "1709913600"},
		{"TIMESTAMP.ENDOF", "hour", "1709917199"},
		{"TIMESTAMP.STARTOF", "day", "1709856000"},
		{"TIMESTAMP.ENDOF", "day", "1709942399"},
		{"TIMESTAMP.STARTOF", "week", "1709424000"},
		{"TIMESTAMP.ENDOF", "WEEK", "1710028799"},
		{"TIMESTAMP.STARTOF", "month", "1709251200"},
		{"TIMESTAMP.ENDOF", "month", "1711929599"},
		{"TIMESTAMP.STARTOF", "year", "1704067200"},
		{"TIMESTAMP.ENDOF", "years", "1735689599"},
		{"TIMESTAMP.ENDOF", "second", "1709913600"},
	}
	for _, c := range cases {
		if got := auditD2Str(t, c.cmd, ts, c.unit); got != ":"+c.want+"\r\n" {
			t.Errorf("%s %s = %q, want %s", c.cmd, c.unit, got, c.want)
		}
	}
	if got := auditD2Str(t, "TIMESTAMP.ENDOF", "1707350400", "month"); got != ":1709251199\r\n" {
		t.Errorf("leap February end = %q", got)
	}
	if got := auditD2Str(t, "TIMESTAMP.STARTOF", "-1", "day"); got != ":-86400\r\n" {
		t.Errorf("negative timestamp start of day = %q", got)
	}
	for _, args := range [][]string{{"abc", "day"}, {ts, "fortnight"}} {
		if got := auditD2Str(t, "TIMESTAMP.ENDOF", args...); !strings.HasPrefix(got, "-ERR") {
			t.Errorf("ENDOF %v: expected error, got %q", args, got)
		}
	}
}

func TestAuditD2EvalExpr(t *testing.T) {
	cases := map[string]string{
		"1+2*3":         "7",
		"(1+2)*3":       "9",
		"10-4-3":        "3",
		"2*(3+4)-5":     "9",
		"100/10/5":      "2",
		"-3+5":          "2",
		"-(2+3)":        "-5",
		"2*-3":          "-6",
		"--4":           "4",
		"-2^2":          "-4",
		"2^3^2":         "512",
		"2^-1":          "0.5",
		"7%3":           "1",
		"-7%3":          "-1",
		"5.5%2":         "1.5",
		" 1.5e2 + 0.5 ": "150.5",
		"1/3":           "0.3333333333333333",
		"0.1+0.2":       "0.30000000000000004",
		"(((1)))":       "1",
		"1e21*10":       "1e+22",
		"0*-1":          "0",
		"5%0.5":         "0",
	}
	for expr, want := range cases {
		if got := auditD2Str(t, "EVAL.EXPR", expr); got != bulk(want) {
			t.Errorf("EVAL.EXPR %q = %q, want %q", expr, got, want)
		}
	}
	for _, bad := range []string{"1/0", "5%0", "5%(1-1)", "", "1+", "(1+2", "1+2)", "2**3", "abc", "1 2", "10^400", "1e999", strings.Repeat("(", 1000) + "1" + strings.Repeat(")", 1000), "(-1)^0.5"} {
		if got := auditD2Str(t, "EVAL.EXPR", bad); !strings.HasPrefix(got, "-ERR") {
			t.Errorf("EVAL.EXPR %q: expected error, got %q", bad, got)
		}
	}
}

func TestAuditD2JSONPath(t *testing.T) {
	doc := `{"store":{"book":[{"title":"A, \"B\"","price":8.95},{"title":"C","price":12}],"a.b":{"x":1}},"n":null}`
	cases := map[string]string{
		"$":                      doc,
		"$.store.book[0].title":  `"A, \"B\""`,
		"$.store.book[1].price":  `12`,
		"$.store.book[-1].title": `"C"`,
		"$['store']['a.b'].x":    `1`,
		`$["store"].book[0]`:     `{"title":"A, \"B\"","price":8.95}`,
		"$.n":                    `null`,
	}
	for path, want := range cases {
		if got := auditD2Str(t, "EVAL.JSONPATH", doc, path); got != bulk(want) {
			t.Errorf("EVAL.JSONPATH %s = %q, want %q", path, got, want)
		}
	}
	for _, missing := range []string{"$.nope", "$.store.book[5]", "$.store.book[-3]", "$.store.book.title", "$.n.x", "$.store[0]"} {
		if got := auditD2Str(t, "EVAL.JSONPATH", doc, missing); got != "$-1\r\n" && got != "_\r\n" {
			t.Errorf("EVAL.JSONPATH %s: expected nil, got %q", missing, got)
		}
	}
	for _, bad := range []string{"store", "$.", "$..title", "$.store.*", "$.store.book[*]", "$.store.book[0:1]", "$.store.book[x]", "$[", "$['a'", "$x"} {
		if got := auditD2Str(t, "EVAL.JSONPATH", doc, bad); !strings.HasPrefix(got, "-ERR") {
			t.Errorf("EVAL.JSONPATH %s: expected error, got %q", bad, got)
		}
	}
	if got := auditD2Str(t, "EVAL.JSONPATH", `{"a":`, "$.a"); !strings.HasPrefix(got, "-ERR") {
		t.Errorf("invalid JSON: %q", got)
	}
}

func TestAuditD2Validators(t *testing.T) {
	check := func(cmd string, want int, args ...string) {
		t.Helper()
		if got := auditD2Str(t, cmd, args...); got != ":"+itoaD2(want)+"\r\n" {
			t.Errorf("%s %q = %q, want %d", cmd, args, got, want)
		}
	}
	for _, ok := range []string{"192.168.1.1", "0.0.0.0", "255.255.255.255", "::1", "2001:db8::8a2e:370:7334", "::ffff:10.0.0.1"} {
		check("VALIDATE.IP", 1, ok)
	}
	for _, bad := range []string{"", "256.1.1.1", "1.2.3", "1.2.3.4.5", "01.2.3.4", "1..2.3", "1.2.3.-4", "fe80::1%eth0", "::g", " 1.2.3.4", "1.2.3.4 "} {
		check("VALIDATE.IP", 0, bad)
	}

	for _, ok := range []string{"user@example.com", "first.last+tag@sub.example.co.uk", "a@b.io", "x@münchen.de"} {
		check("VALIDATE.EMAIL", 1, ok)
	}
	for _, bad := range []string{"", "plain", "@example.com", "user@", "user@localhost", "a@b@c.com", "user@.com", "user@exa mple.com", "Name <user@example.com>", "user@example.com (c)", "user.@example.com", "user@-a.com", "user@a..com", strings.Repeat("a", 250) + "@x.com"} {
		check("VALIDATE.EMAIL", 0, bad)
	}

	for _, ok := range []string{"http://example.com", "https://example.com:8443/p?q=1#f", "ftp://user:pw@files.example.org/x", "http://127.0.0.1/", "http://[::1]:80/", "custom+s://host.local"} {
		check("VALIDATE.URL", 1, ok)
	}
	for _, bad := range []string{"", "http://", "example.com", "/relative", "mailto:a@b.com", "http://exa mple.com", "http://example.com:99999", "http://example.com:0", "http://example.com:", "http://-bad-.com", "http://[zz]/", "://x.com", "http://a.com/a b"} {
		check("VALIDATE.URL", 0, bad)
	}

	check("VALIDATE.RANGE", 1, "5", "1", "10")
	check("VALIDATE.RANGE", 1, "1", "1", "10")
	check("VALIDATE.RANGE", 1, "10", "1", "10")
	check("VALIDATE.RANGE", 0, "10.0000001", "1", "10")
	check("VALIDATE.RANGE", 0, "0.9999999", "1", "10")
	check("VALIDATE.RANGE", 1, "0.3", "0.3", "0.3")
	check("VALIDATE.RANGE", 1, "-5", "-10", "-1")
	check("VALIDATE.RANGE", 1, "1e3", "999", "1000")
	check("VALIDATE.RANGE", 0, "9007199254740993", "9007199254740992", "9007199254740992")
	check("VALIDATE.RANGE", 1, "9007199254740993", "9007199254740993", "9007199254740994")
	check("VALIDATE.RANGE", 1, "1e-300", "0", "1")
	check("VALIDATE.RANGE", 0, "0", "1e-300", "1")
	for _, bad := range [][]string{{"abc", "1", "2"}, {"5x", "1", "10"}, {"5", "", "10"}, {"nan", "1", "2"}, {"inf", "1", "2"}, {"5", "10", "1"}, {"1e-99999999999", "0", "1"}, {"1e999", "0", "1"}} {
		if got := auditD2Str(t, "VALIDATE.RANGE", bad...); !strings.HasPrefix(got, "-ERR") {
			t.Errorf("VALIDATE.RANGE %q: expected error, got %q", bad, got)
		}
	}

	check("VALIDATE.JSON", 1, `{"a":[1,2]}`)
	check("VALIDATE.JSON", 1, `"s"`)
	check("VALIDATE.JSON", 0, `{not json}`)
	check("VALIDATE.JSON", 0, `[1,]`)
}
