package server

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
)

func TestExtensionWaitsAreUnblockable(t *testing.T) {
	s := startReplServer(t, nil)
	admin := dialRepl(t, s)
	b := dialRepl(t, s)
	bid := strconv.FormatInt(b.do("CLIENT", "ID").Int, 10)
	pid := string(admin.do("PROMISE.CREATE").Bulk)
	fid := string(admin.do("FUTURE.CREATE").Bulk)
	admin.do("SEM.CREATE", "unblock-s", "1")
	admin.do("SEM.ACQUIRE", "unblock-s", "1")
	waits := [][]string{
		{"SEM.ACQUIRE", "unblock-s", "1", "0"},
		{"PROMISE.AWAIT", pid, "0"},
		{"FUTURE.GET", fid, "0"},
	}
	for _, w := range waits {
		done := make(chan *resp.Value, 1)
		go func() { done <- b.do(w...) }()
		eventually(t, 3*time.Second, w[0]+" blocked", func() bool {
			return strings.Contains(string(admin.do("CLIENT", "LIST", "ID", bid).Bulk), "flags=b")
		})
		if got := admin.do("CLIENT", "UNBLOCK", bid); got.Int != 1 {
			t.Fatalf("CLIENT UNBLOCK during %s = %+v", w[0], got)
		}
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Fatalf("%s was not unblocked", w[0])
		}
	}
}
