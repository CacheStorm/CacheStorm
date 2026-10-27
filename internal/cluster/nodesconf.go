package cluster

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func readFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil, nil
	}
	return data, nil
}

func parseFlags(s string) (Flags, error) {
	var f Flags
	if s == "noflags" {
		return 0, nil
	}
	for _, name := range strings.Split(s, ",") {
		switch name {
		case "myself":
			f |= FlagMyself
		case "master":
			f |= FlagMaster
		case "slave", "replica":
			f |= FlagSlave
		case "fail?":
			f |= FlagPFail
		case "fail":
			f |= FlagFail
		case "handshake":
			f |= FlagHandshake
		case "noaddr":
			f |= FlagNoAddr
		case "nofailover":
			f |= FlagNoFailover
		default:
			return 0, fmt.Errorf("unknown node flag %q", name)
		}
	}
	return f, nil
}

// parseAddress parses ip:port@cport[,hostname][,aux=value...].
func parseAddress(n *Node, s string) error {
	fields := strings.Split(s, ",")
	addr := fields[0]
	if len(fields) > 1 {
		n.Hostname = fields[1]
		for _, aux := range fields[2:] {
			if aux == "" {
				continue
			}
			k, v, ok := strings.Cut(aux, "=")
			if !ok {
				return fmt.Errorf("invalid aux field %q", aux)
			}
			switch k {
			case "shard-id":
				if !ValidID(v) {
					return fmt.Errorf("invalid shard-id %q", v)
				}
				n.ShardID = strings.ToLower(v)
			case "tls-port":
				p, err := strconv.Atoi(v)
				if err != nil {
					return fmt.Errorf("invalid tls-port %q", v)
				}
				n.TLSPort = p
			case "tcp-port":
				p, err := strconv.Atoi(v)
				if err != nil {
					return fmt.Errorf("invalid tcp-port %q", v)
				}
				n.Port = p
			}
		}
	}
	colon := strings.LastIndexByte(addr, ':')
	if colon < 0 {
		return fmt.Errorf("invalid address %q", addr)
	}
	n.IP = addr[:colon]
	ports := addr[colon+1:]
	cport := ""
	if at := strings.IndexByte(ports, '@'); at >= 0 {
		ports, cport = ports[:at], ports[at+1:]
	}
	port, err := strconv.Atoi(ports)
	if err != nil || port < 0 || port > 65535 {
		return fmt.Errorf("invalid port in %q", addr)
	}
	if n.Port == 0 {
		n.Port = port
	}
	n.CPort = port + 10000
	if cport != "" {
		if n.CPort, err = strconv.Atoi(cport); err != nil || n.CPort < 0 || n.CPort > 65535 {
			return fmt.Errorf("invalid bus port in %q", addr)
		}
	}
	return nil
}

func parseSlot(s string) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 || n >= Slots {
		return 0, fmt.Errorf("invalid slot %q", s)
	}
	return n, nil
}

// Parse builds the cluster state from the text of a nodes.conf file.
func Parse(text string) (*Cluster, error) {
	c := &Cluster{nodes: make(map[string]*Node)}
	node := func(id string) *Node {
		id = strings.ToLower(id)
		n := c.nodes[id]
		if n == nil {
			n = &Node{ID: id, Flags: FlagNoAddr}
			c.nodes[id] = n
		}
		return n
	}
	type pending struct {
		slot  int
		peer  string
		owner *Node
		kind  byte
	}
	var opens []pending
	for lineNo, line := range strings.Split(text, "\n") {
		argv := strings.Fields(line)
		if len(argv) == 0 {
			continue
		}
		bad := func(err error) error {
			return fmt.Errorf("corrupted line %d: %w", lineNo+1, err)
		}
		if argv[0] == "vars" {
			for i := 1; i+1 < len(argv); i += 2 {
				v, err := strconv.ParseUint(argv[i+1], 10, 64)
				if err != nil {
					return nil, bad(err)
				}
				switch argv[i] {
				case "currentEpoch":
					c.currentEpoch = v
				case "lastVoteEpoch":
					c.lastVoteEpoch = v
				}
			}
			continue
		}
		if len(argv) < 8 {
			return nil, bad(errors.New("too few fields"))
		}
		if !ValidID(argv[0]) {
			return nil, bad(fmt.Errorf("invalid node ID %q", argv[0]))
		}
		n := node(argv[0])
		if err := parseAddress(n, argv[1]); err != nil {
			return nil, bad(err)
		}
		flags, err := parseFlags(argv[2])
		if err != nil {
			return nil, bad(err)
		}
		n.Flags = flags
		if flags&FlagMyself != 0 {
			if c.myself != nil && c.myself != n {
				return nil, bad(errors.New("more than one node is flagged myself"))
			}
			c.myself = n
		}
		if argv[3] != "-" {
			if !ValidID(argv[3]) {
				return nil, bad(fmt.Errorf("invalid master ID %q", argv[3]))
			}
			n.MasterID = node(argv[3]).ID
		}
		if n.PingSent, err = strconv.ParseInt(argv[4], 10, 64); err != nil {
			return nil, bad(err)
		}
		if n.PongRecv, err = strconv.ParseInt(argv[5], 10, 64); err != nil {
			return nil, bad(err)
		}
		if n.ConfigEpoch, err = strconv.ParseUint(argv[6], 10, 64); err != nil {
			return nil, bad(err)
		}
		for _, tok := range argv[8:] {
			if strings.HasPrefix(tok, "[") {
				body := strings.TrimSuffix(tok[1:], "]")
				if i := strings.Index(body, "->-"); i > 0 {
					slot, err := parseSlot(body[:i])
					if err != nil {
						return nil, bad(err)
					}
					opens = append(opens, pending{slot, body[i+3:], n, '>'})
				} else if i := strings.Index(body, "-<-"); i > 0 {
					slot, err := parseSlot(body[:i])
					if err != nil {
						return nil, bad(err)
					}
					opens = append(opens, pending{slot, body[i+3:], n, '<'})
				} else {
					return nil, bad(fmt.Errorf("invalid slot token %q", tok))
				}
				continue
			}
			start, end := tok, tok
			if i := strings.IndexByte(tok, '-'); i > 0 {
				start, end = tok[:i], tok[i+1:]
			}
			a, err := parseSlot(start)
			if err != nil {
				return nil, bad(err)
			}
			b, err := parseSlot(end)
			if err != nil || b < a {
				return nil, bad(fmt.Errorf("invalid slot range %q", tok))
			}
			for slot := a; slot <= b; slot++ {
				c.addSlotLocked(n, slot)
			}
		}
	}
	if c.myself == nil {
		return nil, errors.New("no node is flagged myself")
	}
	for _, o := range opens {
		if o.owner != c.myself {
			continue
		}
		if !ValidID(o.peer) {
			return nil, fmt.Errorf("invalid node ID %q in an open slot", o.peer)
		}
		if o.kind == '>' {
			c.migrating[o.slot] = node(o.peer)
		} else {
			c.importing[o.slot] = node(o.peer)
		}
	}
	for _, n := range c.nodes {
		if n.ShardID == "" {
			n.ShardID = NewID()
		}
	}
	for _, n := range c.nodes {
		if m := c.nodes[n.MasterID]; n.replica() && m != nil {
			n.ShardID = m.ShardID
		}
	}
	return c, nil
}
