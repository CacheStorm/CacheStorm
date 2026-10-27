package replication

import "testing"

func TestPromotionOfDivergedReplicaDiscardsPartialHistory(t *testing.T) {
	m, _ := newTestManager(t, Config{BacklogSize: 100})
	m.mu.Lock()
	m.role = RoleReplica
	m.createBacklogLocked()
	m.appendLocked([]byte("old-history"))
	oldID := m.replid
	m.mu.Unlock()
	m.MarkLocalWrite()
	if _, ok := m.SavedState(); ok {
		t.Fatal("dirty history remained resumable")
	}
	m.Promote()
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.replid == oldID || m.replid2 != zeroReplID || m.secondOffset != -1 || m.histLen != 0 || !m.cached {
		t.Fatal("promotion retained invalid partial history")
	}
}
