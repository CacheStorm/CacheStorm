package lua

// This file ports the parts of Lua 5.1's ldebug.c that name the variable an
// error is about: getobjname, symbexec and luaF_getlocalname.

// localName returns the name of the n-th local (1-based) active at pc. A
// local's EndPc is its last instruction, where Lua's endpc is the next one.
func localName(proto *FunctionProto, n, pc int) string {
	for _, l := range proto.DbgLocals {
		if l.StartPc > pc {
			break
		}
		if pc <= l.EndPc {
			n--
			if n == 0 {
				return l.Name
			}
		}
	}
	return ""
}

// symbexec returns the last instruction before lastpc that set reg, or -1.
func symbexec(proto *FunctionProto, lastpc, reg int) int {
	code := proto.Code
	last := -1
	for pc := 0; pc < lastpc && pc < len(code); pc++ {
		inst := code[pc]
		op := opGetOpCode(inst)
		a := opGetArgA(inst)
		if op < len(opProps) && opProps[op].SetRegA && a == reg {
			last = pc
		}
		switch op {
		case OP_LOADNIL:
			if a <= reg && reg <= opGetArgB(inst) {
				last = pc
			}
		case OP_SELF:
			if reg == a+1 {
				last = pc
			}
		case OP_CALL, OP_TAILCALL:
			if reg >= a {
				last = pc
			}
		case OP_TFORLOOP:
			if reg >= a+2 {
				last = pc
			}
		case OP_SETLIST:
			if opGetArgC(inst) == 0 {
				pc++
			}
		case OP_CLOSURE:
			bx := opGetArgBx(inst)
			if bx < len(proto.FunctionPrototypes) {
				pc += int(proto.FunctionPrototypes[bx].NumUpvalues)
			}
		case OP_JMP, OP_FORLOOP, OP_FORPREP:
			dest := pc + 1 + opGetArgSbx(inst)
			if pc < dest && dest <= lastpc {
				pc = dest - 1
			}
		}
	}
	return last
}

func constName(proto *FunctionProto, rk int) string {
	if opIsK(rk) {
		if s, ok := proto.Constants[opIndexK(rk)].(LString); ok {
			return string(s)
		}
	}
	return "?"
}

// objName is getobjname: the kind ("local", "global", "field", "upvalue",
// "method") and name of what register reg holds at pc, if known.
func objName(proto *FunctionProto, pc, reg int) (string, string) {
	if name := localName(proto, reg+1, pc); name != "" {
		return "local", name
	}
	at := symbexec(proto, pc, reg)
	if at < 0 {
		return "", ""
	}
	inst := proto.Code[at]
	switch opGetOpCode(inst) {
	case OP_GETGLOBAL:
		if s, ok := proto.Constants[opGetArgBx(inst)].(LString); ok {
			return "global", string(s)
		}
	case OP_MOVE:
		if b := opGetArgB(inst); b < opGetArgA(inst) {
			return objName(proto, pc, b)
		}
	case OP_GETTABLE, OP_GETTABLEKS:
		return "field", constName(proto, opGetArgC(inst))
	case OP_GETUPVAL:
		if b := opGetArgB(inst); b < len(proto.DbgUpvalues) {
			return "upvalue", proto.DbgUpvalues[b]
		}
		return "upvalue", "?"
	case OP_SELF:
		return "method", constName(proto, opGetArgC(inst))
	}
	return "", ""
}

// valueVarInfo names v when the running Lua instruction reads it from a
// register, as luaG_typeerror does for values on the stack.
func (ls *LState) valueVarInfo(v LValue) (string, string) {
	cf := ls.currentFrame
	if cf == nil || cf.Fn == nil || cf.Fn.IsG || cf.Pc < 1 || cf.Pc > len(cf.Fn.Proto.Code) {
		return "", ""
	}
	proto := cf.Fn.Proto
	pc := cf.Pc - 1
	inst := proto.Code[pc]
	a, b, c := opGetArgA(inst), opGetArgB(inst), opGetArgC(inst)
	var regs []int
	switch opGetOpCode(inst) {
	case OP_CALL, OP_TAILCALL, OP_SETTABLE, OP_SETTABLEKS:
		regs = []int{a}
	case OP_GETTABLE, OP_GETTABLEKS, OP_SELF, OP_UNM, OP_LEN:
		regs = []int{b}
	case OP_ADD, OP_SUB, OP_MUL, OP_DIV, OP_MOD, OP_POW:
		for _, r := range []int{b, c} {
			if !opIsK(r) {
				regs = append(regs, r)
			}
		}
	case OP_CONCAT:
		for r := b; r <= c; r++ {
			regs = append(regs, r)
		}
	}
	for _, r := range regs {
		if ls.reg.Get(cf.LocalBase+r) == v {
			return objName(proto, pc, r)
		}
	}
	return "", ""
}

// typeError raises Lua 5.1's luaG_typeerror message for v.
func typeError(L *LState, v LValue, op string) {
	if kind, name := L.valueVarInfo(v); kind != "" {
		L.runError("attempt to %s %s '%s' (a %s value)", op, kind, name, v.Type().String())
	}
	L.runError("attempt to %s a %s value", op, v.Type().String())
}

// orderError raises Lua 5.1's luaG_ordererror message.
func orderError(L *LState, lhs, rhs LValue) {
	t1, t2 := lhs.Type().String(), rhs.Type().String()
	if t1 == t2 {
		L.runError("attempt to compare two %s values", t1)
	}
	L.runError("attempt to compare %s with %s", t1, t2)
}

// runError is luaG_runerror: the message gets the position of the running
// function only when that function is Lua code.
func (ls *LState) runError(format string, args ...interface{}) {
	if cf := ls.currentFrame; cf != nil && cf.Fn != nil && cf.Fn.IsG {
		ls.raiseError(0, format, args...)
	}
	ls.RaiseError(format, args...)
}

// callName is the name luaL_argerror gives the running Go function, and
// whether it was called as a method.
func (ls *LState) callName(fr *callFrame) (string, bool) {
	if fr == nil || fr.Parent == nil || fr.Parent.Fn.IsG {
		return "?", false
	}
	pc := fr.Parent.Pc - 1
	for _, call := range fr.Parent.Fn.Proto.DbgCalls {
		if call.Pc == pc {
			return call.Name, call.Method
		}
	}
	return "?", false
}

// readonlyError is Redis' luaG_runerror for a write to a readonly table: it
// has a position only when raised by Lua code.
func (ls *LState) readonlyError() {
	ls.runError("Attempt to modify a readonly table")
}
