package command

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/cachestorm/cachestorm/internal/graph"
	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// A graph is a hash key marked __graph, whose marker holds the next node
// and edge ids ("<node> <edge>"). Node n is the field n:<n>, holding its
// label and properties; edge e is the field e:<e>, holding its source,
// target, relation and properties. Each value is a run of length-prefixed
// strings ("5:Alice"). Being a key, a graph is per database, persisted and
// replicated, recorded as HSET, HDEL and DEL.

const graphMarker = "__graph"

func RegisterGraphCommands(router *Router) {
	router.Register(&CommandDef{Name: "GRAPH.CREATE", Handler: cmdGRAPHCREATE})
	router.Register(&CommandDef{Name: "GRAPH.DELETE", Handler: cmdGRAPHDELETE})
	router.Register(&CommandDef{Name: "GRAPH.INFO", Handler: cmdGRAPHINFO})
	router.Register(&CommandDef{Name: "GRAPH.LIST", Handler: cmdGRAPHLIST})
	router.Register(&CommandDef{Name: "GRAPH.ADDNODE", Handler: cmdGRAPHADDNODE})
	router.Register(&CommandDef{Name: "GRAPH.GETNODE", Handler: cmdGRAPHGETNODE})
	router.Register(&CommandDef{Name: "GRAPH.DELNODE", Handler: cmdGRAPHDELNODE})
	router.Register(&CommandDef{Name: "GRAPH.ADDEDGE", Handler: cmdGRAPHADDEDGE})
	router.Register(&CommandDef{Name: "GRAPH.GETEDGE", Handler: cmdGRAPHGETEDGE})
	router.Register(&CommandDef{Name: "GRAPH.DELEDGE", Handler: cmdGRAPHDELEDGE})
	router.Register(&CommandDef{Name: "GRAPH.QUERY", Handler: cmdGRAPHQUERY})
	router.Register(&CommandDef{Name: "GRAPH.RO_QUERY", Handler: cmdGRAPHQUERY})
	router.Register(&CommandDef{Name: "GRAPH.NEIGHBORS", Handler: cmdGRAPHNEIGHBORS})
}

func encodeStrings(items ...string) string {
	var b strings.Builder
	for _, s := range items {
		b.WriteString(strconv.Itoa(len(s)))
		b.WriteByte(':')
		b.WriteString(s)
	}
	return b.String()
}

func decodeStrings(v string) ([]string, bool) {
	var out []string
	for len(v) > 0 {
		i := strings.IndexByte(v, ':')
		if i <= 0 {
			return nil, false
		}
		n, err := strconv.Atoi(v[:i])
		if err != nil || n < 0 || n > len(v)-i-1 {
			return nil, false
		}
		out = append(out, v[i+1:i+1+n])
		v = v[i+1+n:]
	}
	return out, true
}

func pairsToProps(pairs []string) map[string]string {
	props := make(map[string]string, len(pairs)/2)
	for i := 0; i+1 < len(pairs); i += 2 {
		props[pairs[i]] = pairs[i+1]
	}
	return props
}

func propPairs(props map[string]string) []string {
	var out []string
	for _, k := range sortedKeys(props) {
		out = append(out, k, props[k])
	}
	return out
}

// graphData is a graph as its hash holds it.
type graphData struct {
	nextNode, nextEdge uint64
	nodes              []*graph.Node
	edges              []*graph.Edge
}

func nodeField(id uint64) string { return "n:" + strconv.FormatUint(id, 10) }
func edgeField(id uint64) string { return "e:" + strconv.FormatUint(id, 10) }

// readGraph loads the graph at key: nil when the key is missing, WRONGTYPE
// when it holds anything else.
func readGraph(ctx *Context, key string) (*graphData, error) {
	r, err := readMarked(ctx, key, graphMarker)
	if err != nil || r == nil {
		return nil, err
	}
	g := &graphData{}
	if _, err := fmt.Sscanf(r[graphMarker], "%d %d", &g.nextNode, &g.nextEdge); err != nil {
		return nil, errors.New("ERR corrupt graph header")
	}
	for f, v := range r {
		if len(f) < 3 || f[1] != ':' {
			continue
		}
		id, err := strconv.ParseUint(f[2:], 10, 64)
		if err != nil {
			continue
		}
		parts, ok := decodeStrings(v)
		switch {
		case !ok:
			return nil, fmt.Errorf("ERR corrupt graph element %s", f)
		case f[0] == 'n' && len(parts) >= 1 && len(parts)%2 == 1:
			g.nodes = append(g.nodes, &graph.Node{ID: id, Label: parts[0], Properties: pairsToProps(parts[1:])})
		case f[0] == 'e' && len(parts) >= 3 && len(parts)%2 == 1:
			from, err1 := strconv.ParseUint(parts[0], 10, 64)
			to, err2 := strconv.ParseUint(parts[1], 10, 64)
			if err1 != nil || err2 != nil {
				return nil, fmt.Errorf("ERR corrupt graph element %s", f)
			}
			g.edges = append(g.edges, &graph.Edge{ID: id, From: from, To: to, Relation: parts[2], Properties: pairsToProps(parts[3:])})
		}
	}
	sort.Slice(g.nodes, func(i, j int) bool { return g.nodes[i].ID < g.nodes[j].ID })
	sort.Slice(g.edges, func(i, j int) bool { return g.edges[i].ID < g.edges[j].ID })
	return g, nil
}

func loadGraph(ctx *Context, key string) (*graphData, error) {
	g, err := readGraph(ctx, key)
	if err == nil && g == nil {
		err = errors.New("ERR graph not found")
	}
	return g, err
}

func (g *graphData) header() string {
	return strconv.FormatUint(g.nextNode, 10) + " " + strconv.FormatUint(g.nextEdge, 10)
}

func (g *graphData) node(id uint64) *graph.Node {
	for _, n := range g.nodes {
		if n.ID == id {
			return n
		}
	}
	return nil
}

func (g *graphData) edge(id uint64) *graph.Edge {
	for _, e := range g.edges {
		if e.ID == id {
			return e
		}
	}
	return nil
}

func parseElementID(arg string) (uint64, error) {
	id, err := strconv.ParseUint(arg, 10, 64)
	if err != nil {
		return 0, ErrNotInteger
	}
	return id, nil
}

// GRAPH.CREATE name makes an empty graph; an existing one is kept.
func cmdGRAPHCREATE(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	key := ctx.ArgString(0)
	g, err := readGraph(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if g != nil {
		ctx.PropagateAs()
		return ctx.WriteOK()
	}
	if err := setMarkedFieldsRaw(ctx, key, graphMarker, "1 1"); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

// setMarkedFieldsRaw sets the marker to header along with pairs, recorded
// as one HSET.
func setMarkedFieldsRaw(ctx *Context, key, marker, header string, pairs ...string) error {
	return updateRecord(ctx, key, append([]string{marker, header}, pairs...)...)
}

func cmdGRAPHDELETE(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	if _, err := loadGraph(ctx, ctx.ArgString(0)); err != nil {
		ctx.PropagateAs()
		return ctx.WriteError(err)
	}
	ctx.Store.Delete(ctx.ArgString(0))
	ctx.notify(store.NotifyGeneric, "del", ctx.ArgString(0))
	ctx.propagateDelete(ctx.ArgString(0))
	return ctx.WriteOK()
}

func cmdGRAPHINFO(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	g, err := loadGraph(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	labels, relations := map[string]bool{}, map[string]bool{}
	for _, n := range g.nodes {
		labels[n.Label] = true
	}
	for _, e := range g.edges {
		relations[e.Relation] = true
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("name"), resp.BulkString(ctx.ArgString(0)),
		resp.BulkString("nodes"), resp.IntegerValue(int64(len(g.nodes))),
		resp.BulkString("edges"), resp.IntegerValue(int64(len(g.edges))),
		resp.BulkString("labels"), resp.IntegerValue(int64(len(labels))),
		resp.BulkString("relations"), resp.IntegerValue(int64(len(relations))),
	})
}

func cmdGRAPHLIST(ctx *Context) error { return listMarked(ctx, graphMarker) }

// GRAPH.ADDNODE graph label [key value ...] adds a node, making the graph
// when missing, and replies its id.
func cmdGRAPHADDNODE(ctx *Context) error {
	if ctx.ArgCount() < 2 || ctx.ArgCount()%2 != 0 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	key := ctx.ArgString(0)
	g, err := readGraph(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if g == nil {
		g = &graphData{nextNode: 1, nextEdge: 1}
	}
	id := g.nextNode
	g.nextNode++
	value := encodeStrings(append([]string{ctx.ArgString(1)}, propPairs(pairsToProps(argsFrom(ctx, 2)))...)...)
	if err := setMarkedFieldsRaw(ctx, key, graphMarker, g.header(), nodeField(id), value); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteInteger(int64(id))
}

func propsReply(props map[string]string) *resp.Value {
	return resp.ArrayValue(bulkStrings(propPairs(props)))
}

func cmdGRAPHGETNODE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id, err := parseElementID(ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(err)
	}
	g, err := loadGraph(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	n := g.node(id)
	if n == nil {
		return ctx.WriteNull()
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("id"), resp.IntegerValue(int64(n.ID)),
		resp.BulkString("label"), resp.BulkString(n.Label),
		resp.BulkString("properties"), propsReply(n.Properties),
	})
}

// GRAPH.DELNODE graph id removes a node and the edges touching it.
func cmdGRAPHDELNODE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id, err := parseElementID(ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(err)
	}
	key := ctx.ArgString(0)
	g, err := loadGraph(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if g.node(id) == nil {
		ctx.PropagateAs()
		return ctx.WriteInteger(0)
	}
	fields := []string{nodeField(id)}
	for _, e := range g.edges {
		if e.From == id || e.To == id {
			fields = append(fields, edgeField(e.ID))
		}
	}
	if _, err := removeFields(ctx, key, graphMarker, fields...); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteInteger(1)
}

// GRAPH.ADDEDGE graph from to relation [key value ...] adds an edge between
// two existing nodes and replies its id.
func cmdGRAPHADDEDGE(ctx *Context) error {
	if ctx.ArgCount() < 4 || ctx.ArgCount()%2 != 0 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	from, err := parseElementID(ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(err)
	}
	to, err := parseElementID(ctx.ArgString(2))
	if err != nil {
		return ctx.WriteError(err)
	}
	key := ctx.ArgString(0)
	g, err := readGraph(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if g == nil {
		g = &graphData{nextNode: 1, nextEdge: 1}
	}
	if g.node(from) == nil {
		return ctx.WriteError(fmt.Errorf("ERR source node %d not found", from))
	}
	if g.node(to) == nil {
		return ctx.WriteError(fmt.Errorf("ERR target node %d not found", to))
	}
	id := g.nextEdge
	g.nextEdge++
	parts := append([]string{strconv.FormatUint(from, 10), strconv.FormatUint(to, 10), ctx.ArgString(3)}, propPairs(pairsToProps(argsFrom(ctx, 4)))...)
	if err := setMarkedFieldsRaw(ctx, key, graphMarker, g.header(), edgeField(id), encodeStrings(parts...)); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteInteger(int64(id))
}

func cmdGRAPHGETEDGE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id, err := parseElementID(ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(err)
	}
	g, err := loadGraph(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	e := g.edge(id)
	if e == nil {
		return ctx.WriteNull()
	}
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("id"), resp.IntegerValue(int64(e.ID)),
		resp.BulkString("from"), resp.IntegerValue(int64(e.From)),
		resp.BulkString("to"), resp.IntegerValue(int64(e.To)),
		resp.BulkString("relation"), resp.BulkString(e.Relation),
		resp.BulkString("properties"), propsReply(e.Properties),
	})
}

func cmdGRAPHDELEDGE(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id, err := parseElementID(ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(err)
	}
	key := ctx.ArgString(0)
	if _, err := loadGraph(ctx, key); err != nil {
		return ctx.WriteError(err)
	}
	n, err := removeFields(ctx, key, graphMarker, edgeField(id))
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteInteger(int64(n))
}

// GRAPH.NEIGHBORS graph id [relation] replies the targets of the node's
// outgoing edges, in edge order, as id and label.
func cmdGRAPHNEIGHBORS(ctx *Context) error {
	if ctx.ArgCount() != 2 && ctx.ArgCount() != 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	id, err := parseElementID(ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(err)
	}
	g, err := readGraph(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	out := []*resp.Value{}
	if g == nil {
		return ctx.WriteArray(out)
	}
	relation := ""
	if ctx.ArgCount() == 3 {
		relation = ctx.ArgString(2)
	}
	s := graph.NewSnapshot(g.nodes, g.edges)
	for _, e := range s.Out(id) {
		if relation != "" && e.Relation != relation {
			continue
		}
		n := s.Node(e.To)
		out = append(out, resp.ArrayValue([]*resp.Value{resp.IntegerValue(int64(n.ID)), resp.BulkString(n.Label)}))
	}
	return ctx.WriteArray(out)
}

// GRAPH.QUERY graph query (and GRAPH.RO_QUERY) runs a read-only Cypher
// query and replies as RedisGraph does: the column names, the rows, and
// statistics. A missing graph is an empty one.
func cmdGRAPHQUERY(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	start := time.Now()
	g, err := readGraph(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	if g == nil {
		g = &graphData{}
	}
	res, err := graph.Query(graph.NewSnapshot(g.nodes, g.edges), ctx.ArgString(1))
	if err != nil {
		return ctx.WriteError(errors.New("ERR " + err.Error()))
	}
	rows := make([]*resp.Value, len(res.Rows))
	for i, row := range res.Rows {
		cells := make([]*resp.Value, len(row))
		for j, v := range row {
			cells[j] = graphValue(v)
		}
		rows[i] = resp.ArrayValue(cells)
	}
	elapsed := float64(time.Since(start).Nanoseconds()) / 1e6
	return ctx.WriteArray([]*resp.Value{
		resp.ArrayValue(bulkStrings(res.Columns)),
		resp.ArrayValue(rows),
		resp.ArrayValue([]*resp.Value{
			resp.BulkString("Cached execution: 0"),
			resp.BulkString(fmt.Sprintf("Query internal execution time: %.6f milliseconds", elapsed)),
		}),
	})
}

func graphProps(props map[string]string) *resp.Value {
	out := []*resp.Value{}
	for _, k := range sortedKeys(props) {
		out = append(out, resp.ArrayValue([]*resp.Value{resp.BulkString(k), resp.BulkString(props[k])}))
	}
	return resp.ArrayValue(out)
}

func pair(name string, v *resp.Value) *resp.Value {
	return resp.ArrayValue([]*resp.Value{resp.BulkString(name), v})
}

// graphValue renders a query value as RedisGraph's verbose result set does.
func graphValue(v any) *resp.Value {
	switch x := v.(type) {
	case nil:
		return resp.NullValue()
	case bool:
		return resp.BulkString(strconv.FormatBool(x))
	case int64:
		return resp.IntegerValue(x)
	case float64:
		return resp.BulkString(graph.FormatFloat(x))
	case string:
		return resp.BulkString(x)
	case []any:
		out := make([]*resp.Value, len(x))
		for i, it := range x {
			out[i] = graphValue(it)
		}
		return resp.ArrayValue(out)
	case *graph.Node:
		return resp.ArrayValue([]*resp.Value{
			pair("id", resp.IntegerValue(int64(x.ID))),
			pair("labels", resp.ArrayValue([]*resp.Value{resp.BulkString(x.Label)})),
			pair("properties", graphProps(x.Properties)),
		})
	case *graph.Edge:
		return resp.ArrayValue([]*resp.Value{
			pair("id", resp.IntegerValue(int64(x.ID))),
			pair("type", resp.BulkString(x.Relation)),
			pair("src_node", resp.IntegerValue(int64(x.From))),
			pair("dest_node", resp.IntegerValue(int64(x.To))),
			pair("properties", graphProps(x.Properties)),
		})
	}
	return resp.NullValue()
}
