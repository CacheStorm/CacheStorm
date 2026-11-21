package command

import "sync/atomic"

// commandTraits is what dispatch needs to know about a command name from the
// static command tables, looked up once when the command is registered
// instead of in each table on every call.
type commandTraits struct {
	name string
	// arity is the Redis arity, checked when hasArity.
	arity     int
	hasArity  bool
	container *commandMeta
	hasSubs   bool
	denyOOM   bool
	mayGrow   bool
	readonly  bool
	// wholeValue: see wholeValueWriters.
	wholeValue   bool
	writeFlagged bool
	statCache    atomic.Pointer[statCache]
}

func traitsOf(name string) *commandTraits {
	spec, ok := redisCommandSpecs[name]
	return &commandTraits{
		name:         name,
		arity:        spec.Arity,
		hasArity:     ok,
		container:    loadContainerMeta()[name],
		hasSubs:      hasSubcommands(name),
		denyOOM:      denyOOMCommands[name],
		mayGrow:      extensionMayGrow(name),
		readonly:     readonlyCommands[name],
		wholeValue:   wholeValueWriters[name],
		writeFlagged: writeFlagged[name],
	}
}

// traitsFor returns the traits of name, the command d was registered as.
func (d *CommandDef) traitsFor(name string) *commandTraits {
	if t := d.traits; t != nil && t.name == name {
		return t
	}
	return traitsOf(name)
}

// argumentError is ArgumentError for the command.
func (t *commandTraits) argumentError(args [][]byte) error {
	if t.hasArity {
		n := len(args) + 1
		if (t.arity >= 0 && n != t.arity) || (t.arity < 0 && n < -t.arity) {
			return arityError(t.name)
		}
	}
	return subcommandError(t.container, t.name, args)
}
