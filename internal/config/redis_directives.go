package config

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/cachestorm/cachestorm/internal/resp"
)

// Directive is one Redis configuration directive: a name and its arguments,
// as a redis.conf line or a "--name value" command-line option gives it.
type Directive struct {
	Name string
	Args []string
	// Source says where the directive came from, for error messages.
	Source string
}

// ParseRedisArgs splits Redis-style command-line options ("--maxmemory 1gb
// --save 60 1000") into directives: each "--name" takes the arguments up to
// the next "--".
func ParseRedisArgs(args []string) ([]Directive, error) {
	var out []Directive
	for i, a := range args {
		if strings.HasPrefix(a, "--") && len(a) > 2 {
			out = append(out, Directive{Name: strings.ToLower(a[2:]), Source: "argument " + a})
			continue
		}
		if len(out) == 0 {
			return nil, fmt.Errorf("argument %q (position %d) is not an option", a, i+1)
		}
		out[len(out)-1].Args = append(out[len(out)-1].Args, a)
	}
	return out, nil
}

// ParseRedisConf reads the directives of a redis.conf file. Blank lines and
// comments are skipped; arguments follow Redis' quoting rules.
func ParseRedisConf(text, path string) ([]Directive, error) {
	var out []Directive
	for n, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line[0] == '#' {
			continue
		}
		args, err := resp.SplitArgs(line)
		if err != nil {
			return nil, fmt.Errorf("%s:%d: %w", path, n+1, err)
		}
		if len(args) == 0 {
			continue
		}
		out = append(out, Directive{Name: strings.ToLower(args[0]), Args: args[1:], Source: fmt.Sprintf("%s:%d", path, n+1)})
	}
	return out, nil
}

var directiveAliases = map[string]string{
	"slaveof":                       "replicaof",
	"slave-read-only":               "replica-read-only",
	"slave-announce-ip":             "replica-announce-ip",
	"slave-announce-port":           "replica-announce-port",
	"cluster-slave-validity-factor": "cluster-replica-validity-factor",
	"cluster-slave-no-failover":     "cluster-replica-no-failover",
}

// ApplyDirectives applies the directives the server reads at startup and
// returns the rest, which CONFIG SET or ACL SETUSER apply once the server
// exists.
func (c *Config) ApplyDirectives(ds []Directive) ([]Directive, error) {
	var rest []Directive
	// Save rules add up over several lines, as in redis.conf; an empty rule
	// clears the ones before it.
	var save []string
	saveSeen, aofSeen, wasEnabled := false, false, c.Persistence.Enabled
	for _, d := range ds {
		if alias, ok := directiveAliases[d.Name]; ok {
			d.Name = alias
		}
		aofSeen = aofSeen || d.Name == "appendonly"
		if d.Name == "save" {
			args := d.Args
			if len(args) == 1 {
				args = strings.Fields(args[0])
			}
			if len(args) == 0 {
				save = nil
			}
			save, saveSeen = append(save, args...), true
			continue
		}
		handled, err := c.applyDirective(d)
		if err != nil {
			return nil, fmt.Errorf("%s: %s: %w", d.Source, d.Name, err)
		}
		if !handled {
			rest = append(rest, d)
		}
	}
	if saveSeen {
		if err := c.applySave(save); err != nil {
			return nil, fmt.Errorf("save: %w", err)
		}
	}
	// As in Redis, the append-only file is off unless appendonly asks for
	// it: save rules alone turn on snapshots only.
	if !wasEnabled && c.Persistence.Enabled && !aofSeen {
		c.Persistence.AOF = false
	}
	return rest, nil
}

func (c *Config) applyDirective(d Directive) (bool, error) {
	one := func() (string, error) {
		if len(d.Args) != 1 {
			return "", fmt.Errorf("wrong number of arguments")
		}
		return d.Args[0], nil
	}
	yesNo := func(dst *bool) (bool, error) {
		v, err := one()
		if err != nil {
			return true, err
		}
		switch strings.ToLower(v) {
		case "yes":
			*dst = true
		case "no":
			*dst = false
		default:
			return true, fmt.Errorf("argument must be 'yes' or 'no'")
		}
		return true, nil
	}
	integer := func(dst *int) (bool, error) {
		v, err := one()
		if err != nil {
			return true, err
		}
		n, err := strconv.Atoi(v)
		if err != nil {
			return true, fmt.Errorf("argument couldn't be parsed into an integer")
		}
		*dst = n
		return true, nil
	}
	str := func(dst *string) (bool, error) {
		v, err := one()
		if err != nil {
			return true, err
		}
		*dst = v
		return true, nil
	}

	switch d.Name {
	case "port":
		return integer(&c.Server.Port)
	case "bind":
		if len(d.Args) == 0 {
			return true, fmt.Errorf("wrong number of arguments")
		}
		// CacheStorm listens on one address: the first one Redis would
		// require (a "-" prefix marks an optional address).
		addr := strings.TrimPrefix(d.Args[0], "-")
		if addr == "*" {
			addr = "0.0.0.0"
		}
		c.Server.Bind = addr
		return true, nil
	case "protected-mode":
		return yesNo(&c.Server.ProtectedMode)
	case "requirepass":
		return str(&c.Server.RequirePass)
	case "databases":
		return integer(&c.Server.Databases)
	case "maxclients":
		return integer(&c.Server.MaxConnections)
	case "tcp-keepalive":
		return integer(&c.Server.TCPKeepAlive)
	case "notify-keyspace-events":
		return str(&c.Server.NotifyKeyspaceEvents)
	case "maxmemory":
		return str(&c.Memory.MaxMemory)
	case "maxmemory-policy":
		return str(&c.Memory.EvictionPolicy)
	case "maxmemory-samples":
		return integer(&c.Memory.SampleSize)
	case "appendonly":
		var on bool
		if _, err := yesNo(&on); err != nil {
			return true, err
		}
		c.Persistence.AOF = on
		if on {
			c.Persistence.Enabled = true
		}
		return true, nil
	case "appendfsync":
		return str(&c.Persistence.AOFSync)
	case "appendfilename", "dbfilename":
		v, err := one()
		if err != nil {
			return true, err
		}
		want := map[string]string{"appendfilename": "appendonly.aof", "dbfilename": "dump.rdb"}[d.Name]
		if v != want {
			return true, fmt.Errorf("CacheStorm always uses %q", want)
		}
		return true, nil
	case "dir":
		return str(&c.Persistence.DataDir)
	case "loglevel":
		v, err := one()
		if err != nil {
			return true, err
		}
		level, ok := map[string]string{"debug": "debug", "verbose": "debug", "notice": "info", "warning": "warn", "nothing": "error"}[strings.ToLower(v)]
		if !ok {
			return true, fmt.Errorf("argument must be one of debug, verbose, notice, warning, nothing")
		}
		c.Logging.Level = level
		return true, nil
	case "logfile":
		v, err := one()
		if err != nil {
			return true, err
		}
		if v == "" {
			v = "stdout"
		}
		c.Logging.Output = v
		return true, nil
	case "daemonize":
		var on bool
		if _, err := yesNo(&on); err != nil {
			return true, err
		}
		if on {
			return true, fmt.Errorf("CacheStorm does not daemonize; run it under a service manager")
		}
		return true, nil
	case "replicaof":
		if len(d.Args) != 2 {
			return true, fmt.Errorf("wrong number of arguments")
		}
		if strings.EqualFold(d.Args[0], "no") && strings.EqualFold(d.Args[1], "one") {
			c.Replication.Role, c.Replication.MasterHost, c.Replication.MasterPort = "master", "", 0
			return true, nil
		}
		port, err := strconv.Atoi(d.Args[1])
		if err != nil {
			return true, fmt.Errorf("invalid master port")
		}
		c.Replication.Role, c.Replication.MasterHost, c.Replication.MasterPort = "replica", d.Args[0], port
		return true, nil
	case "masterauth":
		return str(&c.Replication.MasterAuth)
	case "replica-read-only":
		return yesNo(&c.Replication.ReadOnly)
	case "replica-announce-ip":
		return str(&c.Replication.ReplicaAnnounceIP)
	case "replica-announce-port":
		return integer(&c.Replication.ReplicaAnnouncePort)
	case "repl-timeout":
		return integer(&c.Replication.ReplTimeout)
	case "repl-backlog-size":
		v, err := one()
		if err != nil {
			return true, err
		}
		n, err := ParseMemorySize(v)
		if err != nil || n <= 0 || n > 1<<62 {
			return true, fmt.Errorf("argument must be a memory value")
		}
		c.Replication.BacklogSize = int(n)
		return true, nil
	case "cluster-enabled":
		return yesNo(&c.Cluster.Enabled)
	case "cluster-config-file":
		return str(&c.Cluster.ConfigFile)
	case "cluster-node-timeout":
		return integer(&c.Cluster.NodeTimeout)
	case "cluster-port":
		return integer(&c.Cluster.BusPort)
	case "cluster-announce-ip":
		return str(&c.Cluster.AnnounceIP)
	case "cluster-announce-port":
		return integer(&c.Cluster.AnnouncePort)
	case "cluster-announce-bus-port":
		return integer(&c.Cluster.AnnounceBusPort)
	case "cluster-require-full-coverage":
		return yesNo(&c.Cluster.RequireFullCoverage)
	case "cluster-allow-reads-when-down":
		return yesNo(&c.Cluster.AllowReadsWhenDown)
	case "cluster-replica-validity-factor":
		return integer(&c.Cluster.ReplicaValidityFactor)
	case "cluster-replica-no-failover":
		return yesNo(&c.Cluster.ReplicaNoFailover)
	case "cluster-allow-replica-migration":
		return yesNo(&c.Cluster.AllowReplicaMigration)
	case "cluster-migration-barrier":
		return integer(&c.Cluster.MigrationBarrier)
	case "include", "rename-command", "aclfile", "loadmodule":
		return true, fmt.Errorf("not supported by CacheStorm")
	}
	return false, nil
}

// applySave maps Redis' "save <seconds> <changes> ..." rules onto the
// snapshot interval: snapshots run every <seconds> of the most frequent
// rule. An empty rule turns snapshots off.
func (c *Config) applySave(args []string) error {
	if len(args) == 0 {
		c.Persistence.SnapshotInterval = "0"
		return nil
	}
	if len(args)%2 != 0 {
		return fmt.Errorf("invalid save parameters")
	}
	shortest := -1
	for i := 0; i < len(args); i += 2 {
		secs, err1 := strconv.Atoi(args[i])
		changes, err2 := strconv.Atoi(args[i+1])
		if err1 != nil || err2 != nil || secs < 1 || changes < 0 {
			return fmt.Errorf("invalid save parameters")
		}
		if shortest < 0 || secs < shortest {
			shortest = secs
		}
	}
	c.Persistence.Enabled = true
	c.Persistence.SnapshotInterval = strconv.Itoa(shortest) + "s"
	return nil
}
