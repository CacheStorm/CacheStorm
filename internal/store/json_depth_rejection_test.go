package store

import (
	"strings"
	"testing"
)

func TestJSONPathsRejectExcessDepthWithoutMutation(t *testing.T) {
	for _, depth := range []int{0, 1, maxJSONPathDepth - 1, maxJSONPathDepth, maxJSONPathDepth + 1} {
		for _, op := range []string{"get", "set", "delete", "incr", "append"} {
			var data interface{} = float64(1)
			if op == "append" {
				data = []interface{}{float64(1)}
			}
			for i := 0; i < depth; i++ {
				data = map[string]interface{}{"n": data}
			}
			v, err := NewJSONValue(data)
			if err != nil {
				t.Fatal(err)
			}
			before := v.String()
			path := "$" + strings.Repeat(".n", depth)
			var got interface{}
			switch op {
			case "get":
				got, err = v.GetPath(path)
			case "set":
				err = v.SetPath(path, 99)
			case "delete":
				err = v.DeletePath(path)
			case "incr":
				got, err = v.NumIncrBy(path, 1)
			case "append":
				got, err = v.ArrAppend(path, []interface{}{2})
			}
			if depth > maxJSONPathDepth {
				if err == nil || v.String() != before {
					t.Fatalf("%s excessive depth error=%v unchanged=%t", op, err, v.String() == before)
				}
				continue
			}
			if err != nil {
				t.Fatalf("%s depth%d: %v", op, depth, err)
			}
			if op == "incr" && got != float64(2) {
				t.Fatalf("increment=%v", got)
			}
			if op == "append" && got != 2 {
				t.Fatalf("append length=%v", got)
			}
			if op == "set" {
				got, err = v.GetPath(path)
				if err != nil || got != float64(99) {
					t.Fatalf("set result=%v error=%v", got, err)
				}
			}
		}
	}
	t.Log("FIX VERIFIED")
}
