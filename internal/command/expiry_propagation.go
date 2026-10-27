package command

import "strconv"

func (ctx *Context) propagateExpiry(key string, when int64) {
	ctx.PropagateAs(ExecutedCommand{Name: "PEXPIREAT", Args: [][]byte{[]byte(key), strconv.AppendInt(nil, when, 10)}, DB: ctx.DB})
}

func (ctx *Context) propagateDelete(key string) {
	ctx.PropagateAs(ExecutedCommand{Name: "DEL", Args: [][]byte{[]byte(key)}, DB: ctx.DB})
}

func (ctx *Context) propagateSetX(key string, value []byte, when int64) {
	args := [][]byte{[]byte(key), value}
	if when != 0 {
		args = append(args, []byte("PXAT"), strconv.AppendInt(nil, when, 10))
	}
	ctx.PropagateAs(ExecutedCommand{Name: "SET", Args: args, DB: ctx.DB})
}
