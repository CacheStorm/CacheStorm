package command

import (
	"fmt"
	"strings"

	"github.com/cachestorm/cachestorm/internal/graph"
	"github.com/cachestorm/cachestorm/internal/resp"
)

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
	router.Register(&CommandDef{Name: "GRAPH.NEIGHBORS", Handler: cmdGRAPHNEIGHBORS})
}

func cmdGRAPHCREATE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	gm := graph.GetGraphManager()
	gm.Create(name)

	return ctx.WriteOK()
}

func cmdGRAPHDELETE(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	gm := graph.GetGraphManager()

	if !gm.Delete(name) {
		return ctx.WriteError(fmt.Errorf("ERR graph not found"))
	}

	return ctx.WriteOK()
}

func cmdGRAPHINFO(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	gm := graph.GetGraphManager()

	g, ok := gm.Get(name)
	if !ok {
		return ctx.WriteError(fmt.Errorf("ERR graph not found"))
	}

	info := g.Info()

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("name"),
		resp.BulkString(info["name"].(string)),
		resp.BulkString("nodes"),
		resp.IntegerValue(int64(info["nodes"].(int))),
		resp.BulkString("edges"),
		resp.IntegerValue(int64(info["edges"].(int))),
		resp.BulkString("labels"),
		resp.IntegerValue(int64(info["labels"].(int))),
		resp.BulkString("relations"),
		resp.IntegerValue(int64(info["relations"].(int))),
	})
}

func cmdGRAPHLIST(ctx *Context) error {
	gm := graph.GetGraphManager()
	names := gm.List()

	results := make([]*resp.Value, 0, len(names))
	for _, name := range names {
		results = append(results, resp.BulkString(name))
	}

	return ctx.WriteArray(results)
}

func cmdGRAPHADDNODE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	label := ctx.ArgString(1)

	props := make(map[string]interface{})
	for i := 2; i+1 < ctx.ArgCount(); i += 2 {
		key := ctx.ArgString(i)
		val := ctx.ArgString(i + 1)
		props[key] = val
	}

	gm := graph.GetGraphManager()
	g, ok := gm.Get(name)
	if !ok {
		g = gm.Create(name)
	}

	node := g.AddNode(label, props)

	return ctx.WriteInteger(int64(node.ID))
}

func cmdGRAPHGETNODE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	nodeID := parseInt64(ctx.ArgString(1))

	gm := graph.GetGraphManager()
	g, ok := gm.Get(name)
	if !ok {
		return ctx.WriteError(fmt.Errorf("ERR graph not found"))
	}

	node, ok := g.GetNode(uint64(nodeID))
	if !ok {
		return ctx.WriteNull()
	}

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("id"),
		resp.IntegerValue(int64(node.ID)),
		resp.BulkString("label"),
		resp.BulkString(node.Label),
		resp.BulkString("properties"),
		formatProps(node.Properties),
	})
}

func cmdGRAPHDELNODE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	nodeID := parseInt64(ctx.ArgString(1))

	gm := graph.GetGraphManager()
	g, ok := gm.Get(name)
	if !ok {
		return ctx.WriteError(fmt.Errorf("ERR graph not found"))
	}

	if !g.DeleteNode(uint64(nodeID)) {
		return ctx.WriteInteger(0)
	}

	return ctx.WriteInteger(1)
}

func cmdGRAPHADDEDGE(ctx *Context) error {
	if ctx.ArgCount() < 4 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	from := parseInt64(ctx.ArgString(1))
	to := parseInt64(ctx.ArgString(2))
	relation := ctx.ArgString(3)

	props := make(map[string]interface{})
	for i := 4; i+1 < ctx.ArgCount(); i += 2 {
		key := ctx.ArgString(i)
		val := ctx.ArgString(i + 1)
		props[key] = val
	}

	gm := graph.GetGraphManager()
	g, ok := gm.Get(name)
	if !ok {
		g = gm.Create(name)
	}

	edge, err := g.AddEdge(uint64(from), uint64(to), relation, props)
	if err != nil {
		return ctx.WriteError(err)
	}

	return ctx.WriteInteger(int64(edge.ID))
}

func cmdGRAPHGETEDGE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	edgeID := parseInt64(ctx.ArgString(1))

	gm := graph.GetGraphManager()
	g, ok := gm.Get(name)
	if !ok {
		return ctx.WriteError(fmt.Errorf("ERR graph not found"))
	}

	edge, ok := g.GetEdge(uint64(edgeID))
	if !ok {
		return ctx.WriteNull()
	}

	return ctx.WriteArray([]*resp.Value{
		resp.BulkString("id"),
		resp.IntegerValue(int64(edge.ID)),
		resp.BulkString("from"),
		resp.IntegerValue(int64(edge.From)),
		resp.BulkString("to"),
		resp.IntegerValue(int64(edge.To)),
		resp.BulkString("relation"),
		resp.BulkString(edge.Relation),
		resp.BulkString("properties"),
		formatProps(edge.Properties),
	})
}

func cmdGRAPHDELEDGE(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	edgeID := parseInt64(ctx.ArgString(1))

	gm := graph.GetGraphManager()
	g, ok := gm.Get(name)
	if !ok {
		return ctx.WriteError(fmt.Errorf("ERR graph not found"))
	}

	if !g.DeleteEdge(uint64(edgeID)) {
		return ctx.WriteInteger(0)
	}

	return ctx.WriteInteger(1)
}

func cmdGRAPHQUERY(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	query := ctx.ArgString(1)

	gm := graph.GetGraphManager()
	g, ok := gm.Get(name)
	if !ok {
		return ctx.WriteArray([]*resp.Value{})
	}

	result, err := g.Query(query)
	if err != nil {
		return ctx.WriteError(err)
	}

	results := make([]*resp.Value, 0)
	results = append(results, resp.IntegerValue(int64(len(result.Data))))

	for _, row := range result.Data {
		rowValues := make([]*resp.Value, 0)
		for _, val := range row {
			rowValues = append(rowValues, resp.BulkString(fmt.Sprintf("%v", val)))
		}
		results = append(results, resp.ArrayValue(rowValues))
	}

	return ctx.WriteArray(results)
}

func cmdGRAPHNEIGHBORS(ctx *Context) error {
	if ctx.ArgCount() < 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	name := ctx.ArgString(0)
	nodeID := parseInt64(ctx.ArgString(1))
	relation := ""
	if ctx.ArgCount() >= 3 {
		relation = ctx.ArgString(2)
	}

	gm := graph.GetGraphManager()
	g, ok := gm.Get(name)
	if !ok {
		return ctx.WriteArray([]*resp.Value{})
	}

	neighbors := g.Neighbors(uint64(nodeID), relation)

	results := make([]*resp.Value, 0, len(neighbors))
	for _, node := range neighbors {
		results = append(results, resp.ArrayValue([]*resp.Value{
			resp.IntegerValue(int64(node.ID)),
			resp.BulkString(node.Label),
		}))
	}

	return ctx.WriteArray(results)
}

func formatProps(props map[string]interface{}) *resp.Value {
	if len(props) == 0 {
		return resp.ArrayValue([]*resp.Value{})
	}

	results := make([]*resp.Value, 0, len(props)*2)
	for k, v := range props {
		results = append(results, resp.BulkString(k), resp.BulkString(fmt.Sprintf("%v", v)))
	}
	return resp.ArrayValue(results)
}

func init() {
	_ = strings.ToLower("")
}
