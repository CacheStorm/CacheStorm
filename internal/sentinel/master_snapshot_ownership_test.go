package sentinel

import "testing"

func TestSentinelMasterGettersOwnSnapshots(t *testing.T) {
	for _, list := range []bool{false, true} {
		t.Run(map[bool]string{false: "GetMaster", true: "Masters"}[list], func(t *testing.T) {
			s := New(Config{ID: "snapshot"})
			if err := s.Monitor("master", "host", 6379, 2); err != nil {
				t.Fatal(err)
			}
			s.masters["master"].Flags = []string{"master"}
			s.masters["master"].Replicas = []*ReplicaInfo{{Addr: "replica", Offset: 1}}
			addr, port, err := s.GetMasterAddr("master")
			if err != nil || addr != "host" || port != 6379 {
				t.Fatal("unaffected scalar getter control failed")
			}
			t.Log("CONTROL: scalar getter returns owned values")
			snapshot, ok := s.GetMaster("master")
			if !ok {
				t.Fatal("missing master")
			}
			if list {
				snapshot = s.Masters()[0]
			}
			update := make(chan struct{})
			done := make(chan struct{})
			go func() {
				<-update
				s.mu.Lock()
				m := s.masters["master"]
				m.State = MasterStateOK
				m.Flags[0] = "changed"
				m.Replicas[0].Offset = 99
				s.mu.Unlock()
				close(done)
			}()
			close(update)
			<-done
			t.Logf("EXPECTED: snapshot state=0 flag=master offset=1; ACTUAL: state=%d flag=%s offset=%d", snapshot.State, snapshot.Flags[0], snapshot.Replicas[0].Offset)
			if snapshot.State != MasterStateNone || snapshot.Flags[0] != "master" || snapshot.Replicas[0].Offset != 1 {
				t.Fatal("PROBLEM CONFIRMED")
			}
			snapshot.Addr = "caller-change"
			snapshot.Flags[0] = "caller-change"
			snapshot.Replicas[0].Addr = "caller-change"
			fresh, _ := s.GetMaster("master")
			if fresh.Addr != "host" || fresh.Flags[0] != "changed" || fresh.Replicas[0].Addr != "replica" || fresh.State != MasterStateOK {
				t.Fatal("caller mutation reached owner")
			}
			if err := s.Remove("master"); err != nil {
				t.Fatal(err)
			}
			if missing, ok := s.GetMaster("master"); ok || missing != nil {
				t.Fatal("removed master returned")
			}
			if err := s.Monitor("master", "replacement", 6380, 2); err != nil {
				t.Fatal(err)
			}
			fresh, _ = s.GetMaster("master")
			if fresh.Addr != "replacement" || fresh.Flags != nil || len(fresh.Replicas) != 0 {
				t.Fatal("replacement snapshot invalid")
			}
			s.masters["master"].Replicas = []*ReplicaInfo{nil}
			fresh, _ = s.GetMaster("master")
			if len(fresh.Replicas) != 1 || fresh.Replicas[0] != nil {
				t.Fatal("nil replica not preserved")
			}
			t.Log("FIX VERIFIED")
		})
	}
}
