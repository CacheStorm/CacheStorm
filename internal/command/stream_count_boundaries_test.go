package command_test

import (
	"bytes"
	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
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
func streamRun(s *store.Store, name string, args ...string) *resp.Value {
	r := command.NewRouter()
	command.RegisterStreamCommands(r)
	argv := make([][]byte, len(args))
	for i, arg := range args {
		argv[i] = []byte(arg)
	}
	var out bytes.Buffer
	auditMust(r.Execute(command.NewContext(name, argv, s, resp.NewWriter(&out))))
	v, err := resp.NewReader(&out).ReadValue()
	auditMust(err)
	return v
}
func streamIDs(v *resp.Value, read bool) []string {
	ids := []string{}
	if v.Type != resp.TypeArray {
		return ids
	}
	entries := v.Array
	if read {
		if len(entries) == 0 {
			return ids
		}
		entries = entries[0].Array[1].Array
	}
	for _, e := range entries {
		ids = append(ids, string(e.Array[0].Bulk))
	}
	return ids
}
func Stream(read, verify bool) []auditResult {
	s := store.NewStore()
	for _, id := range []string{"1-0", "2-0", "3-0"} {
		v := streamRun(s, "XADD", "s", id, "f", "v")
		if v.Type == resp.TypeError {
			panic(v.Err)
		}
	}
	var results []auditResult
	if !read {
		results = []auditResult{{"control: unlimited descending range", []string{"3-0", "2-0", "1-0"}, streamIDs(streamRun(s, "XREVRANGE", "s", "+", "-"), false)}, {"descending COUNT 1", []string{"3-0"}, streamIDs(streamRun(s, "XREVRANGE", "s", "+", "-", "COUNT", "1"), false)}}
		if verify {
			results = append(results, auditResult{"bounded COUNT 1", []string{"2-0"}, streamIDs(streamRun(s, "XREVRANGE", "s", "2", "-", "COUNT", "1"), false)}, auditResult{"COUNT larger than range", []string{"3-0", "2-0", "1-0"}, streamIDs(streamRun(s, "XREVRANGE", "s", "+", "-", "COUNT", "9"), false)}, auditResult{"empty range", []string{}, streamIDs(streamRun(s, "XREVRANGE", "s", "1", "3", "COUNT", "1"), false)})
		}
	} else {
		results = []auditResult{{"control: boundary absent", []string{"1-0"}, streamIDs(streamRun(s, "XREAD", "COUNT", "1", "STREAMS", "s", "0-0"), true)}, {"COUNT excludes present boundary", []string{"2-0"}, streamIDs(streamRun(s, "XREAD", "COUNT", "1", "STREAMS", "s", "1-0"), true)}}
		if verify {
			results = append(results, auditResult{"partial boundary", []string{"3-0"}, streamIDs(streamRun(s, "XREAD", "COUNT", "1", "STREAMS", "s", "2"), true)}, auditResult{"no entries after last ID", []string{}, streamIDs(streamRun(s, "XREAD", "COUNT", "1", "STREAMS", "s", "3-0"), true)}, auditResult{"COUNT two", []string{"2-0", "3-0"}, streamIDs(streamRun(s, "XREAD", "COUNT", "2", "STREAMS", "s", "1-0"), true)})
		}
	}
	return results
}

func TestAuditXReverseRangeCount(t *testing.T) { auditCheck(t, Stream(false, true)) }

func TestAuditXReadExclusiveCount(t *testing.T) { auditCheck(t, Stream(true, true)) }
