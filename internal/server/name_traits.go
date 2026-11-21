package server

// nameTraits are the per-name command properties the connection path asks
// about on every command, gathered so one map lookup answers them all.
type nameTraits uint8

const (
	ntWrite nameTraits = 1 << iota
	ntBatchBreak
	ntBlocking
	ntExclusive
	ntUngated
)

var nameTraitTable = func() map[string]nameTraits {
	m := make(map[string]nameTraits)
	for set, bit := range map[*map[string]bool]nameTraits{
		&writeCommands: ntWrite, &batchBreakers: ntBatchBreak, &blockingCommands: ntBlocking,
		&exclusiveWrites: ntExclusive, &ungatedCommands: ntUngated,
	} {
		for name, on := range *set {
			if on {
				m[name] |= bit
			}
		}
	}
	return m
}()

func traitsOfName(name string) nameTraits { return nameTraitTable[name] }

// traits returns the traits of the canonical command name, remembering the
// last name asked about: a pipeline mostly repeats one command.
func (c *Connection) traits(name string) nameTraits {
	if name != c.traitName {
		c.traitName, c.traitBits = name, traitsOfName(name)
	}
	return c.traitBits
}

func (t nameTraits) has(bit nameTraits) bool { return t&bit != 0 }
