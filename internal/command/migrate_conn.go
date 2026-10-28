package command

import (
	"crypto/tls"
	"errors"
	"net"
	"sync"
	"time"
)

// MIGRATE keeps its connection to each target open for reuse, as Redis'
// migrateGetSocket does: an idle connection is closed after
// migrateConnIdle, and at most migrateConnMax are kept.
const (
	migrateConnIdle = 10 * time.Second
	migrateConnMax  = 64
)

type migrateConn struct {
	conn    net.Conn
	db      int64
	lastUse time.Time
}

var migrateConns = struct {
	sync.Mutex
	m map[string]*migrateConn
}{m: make(map[string]*migrateConn)}

// migrateGetConn returns a connection to addr, the cached one if there is
// one (cached is then true). The connection belongs to the caller until it
// hands it back with migratePutConn or closes it.
func migrateGetConn(addr string, timeout time.Duration, cfg *tls.Config) (mc *migrateConn, cached bool, err error) {
	migrateConns.Lock()
	if mc = migrateConns.m[addr]; mc != nil {
		delete(migrateConns.m, addr)
	}
	migrateConns.Unlock()
	if mc != nil {
		return mc, true, nil
	}
	var conn net.Conn
	if cfg != nil {
		conn, err = tls.DialWithDialer(&net.Dialer{Timeout: timeout}, "tcp", addr, cfg)
	} else {
		conn, err = net.DialTimeout("tcp", addr, timeout)
	}
	if err != nil {
		return nil, false, err
	}
	return &migrateConn{conn: conn, db: -1}, false, nil
}

// migratePutConn caches mc for the next MIGRATE to addr and closes it once
// it has been idle for migrateConnIdle.
func migratePutConn(addr string, mc *migrateConn) {
	migrateConns.Lock()
	mc.lastUse = time.Now()
	if old := migrateConns.m[addr]; old != nil || len(migrateConns.m) >= migrateConnMax {
		migrateConns.Unlock()
		_ = mc.conn.Close()
		return
	}
	migrateConns.m[addr] = mc
	used := mc.lastUse
	migrateConns.Unlock()
	time.AfterFunc(migrateConnIdle, func() {
		migrateConns.Lock()
		if cur := migrateConns.m[addr]; cur == mc && mc.lastUse.Equal(used) {
			delete(migrateConns.m, addr)
			migrateConns.Unlock()
			_ = mc.conn.Close()
			return
		}
		migrateConns.Unlock()
	})
}

// migrateCachedConns is the number of cached MIGRATE connections.
func migrateCachedConns() int {
	migrateConns.Lock()
	defer migrateConns.Unlock()
	return len(migrateConns.m)
}

func isTimeout(err error) bool {
	var ne net.Error
	return errors.As(err, &ne) && ne.Timeout()
}
