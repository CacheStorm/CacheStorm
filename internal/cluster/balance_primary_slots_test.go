package cluster_test

import (
	"github.com/cachestorm/cachestorm/internal/cluster"
	"reflect"
	"testing"
)

type auditResult struct {
	Name      string
	Want, Got any
}

func auditMust(err error) {
	if err != nil {
		panic(err)
	}
}
func auditCheck(t *testing.T, results []auditResult) {
	t.Helper()
	for _, result := range results {
		if !reflect.DeepEqual(result.Want, result.Got) {
			t.Errorf("%s: got %v, want %v", result.Name, result.Got, result.Want)
		}
	}
}
func Cluster(verify bool) []auditResult {
	c := cluster.New("primary", "127.0.0.1", 0, 0, nil)
	c.AddNode(&cluster.Node{ID: "control", Role: cluster.RolePrimary})
	c.BalanceSlots()
	results := []auditResult{{"control: two primaries", 8192, c.GetSlotDistribution()["primary"]}}
	c.RemoveNode("control")
	c.AddNode(&cluster.Node{ID: "replica", Role: cluster.RoleReplica, ReplicaOf: "primary"})
	c.BalanceSlots()
	dist := c.GetSlotDistribution()
	results = append(results, auditResult{"primary with replica", 16384, dist["primary"]}, auditResult{"replica primary slots", 0, dist["replica"]})
	if verify {
		c.BalanceSlots()
		results = append(results, auditResult{"repeat balance", 16384, c.GetSlotDistribution()["primary"]})
		c.RemoveNode("replica")
		c.BalanceSlots()
		results = append(results, auditResult{"single primary", 16384, c.GetSlotDistribution()["primary"]})
	}
	return results
}

func TestAuditBalancePrimarySlots(t *testing.T) { auditCheck(t, Cluster(true)) }
