package store

import (
	"testing"
)

func TestNewJSONValue(t *testing.T) {
	data := map[string]interface{}{"key": "value"}
	jv, err := NewJSONValue(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if jv == nil {
		t.Fatal("expected JSONValue")
	}
}

func TestNewJSONValueInvalid(t *testing.T) {
	_, err := NewJSONValue(make(chan int))
	if err == nil {
		t.Error("expected error for invalid type")
	}
}

func TestJSONValueType(t *testing.T) {
	jv, _ := NewJSONValue("test")
	if jv.Type() != DataTypeString {
		t.Errorf("expected DataTypeString, got %v", jv.Type())
	}
}

func TestJSONValueSizeOf(t *testing.T) {
	jv, _ := NewJSONValue("test")
	size := jv.SizeOf()
	if size <= 0 {
		t.Errorf("expected positive size, got %d", size)
	}
}

func TestJSONValueString(t *testing.T) {
	jv, _ := NewJSONValue("test")
	if jv.String() != `"test"` {
		t.Errorf("expected quoted string, got %s", jv.String())
	}
}

func TestJSONValueClone(t *testing.T) {
	jv, _ := NewJSONValue("test")
	clone := jv.Clone()
	if clone == nil {
		t.Fatal("expected clone")
	}
}

func TestJSONValueGet(t *testing.T) {
	jv, _ := NewJSONValue(map[string]interface{}{"key": "value"})
	result, err := jv.Get()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := result.(map[string]interface{})
	if m["key"] != "value" {
		t.Errorf("expected key=value, got %v", m)
	}
}

func TestJSONValueGetPath(t *testing.T) {
	data := map[string]interface{}{
		"name": "test",
		"nested": map[string]interface{}{
			"field": "value",
		},
	}
	jv, _ := NewJSONValue(data)

	root, err := jv.GetPath("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if root == nil {
		t.Error("expected root object")
	}

	name, err := jv.GetPath("$.name")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "test" {
		t.Errorf("expected 'test', got %v", name)
	}

	nested, err := jv.GetPath("$.nested.field")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if nested != "value" {
		t.Errorf("expected 'value', got %v", nested)
	}
}

func TestJSONValueGetPathNotFound(t *testing.T) {
	jv, _ := NewJSONValue(map[string]interface{}{"key": "value"})
	result, err := jv.GetPath("$.nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil for nonexistent path, got %v", result)
	}
}

func TestJSONValueSet(t *testing.T) {
	jv, _ := NewJSONValue("test")
	err := jv.Set("new value")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if jv.String() != `"new value"` {
		t.Errorf("expected 'new value', got %s", jv.String())
	}
}

func TestJSONValueSetPath(t *testing.T) {
	jv, _ := NewJSONValue(map[string]interface{}{})

	err := jv.SetPath("$.field", "value")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result, _ := jv.GetPath("$.field")
	if result != "value" {
		t.Errorf("expected 'value', got %v", result)
	}
}

func TestJSONValueDeletePath(t *testing.T) {
	jv, _ := NewJSONValue(map[string]interface{}{"key": "value", "other": "data"})

	err := jv.DeletePath("$.key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result, _ := jv.GetPath("$.key")
	if result != nil {
		t.Errorf("expected nil after delete, got %v", result)
	}
}

func TestJSONValueTypeAt(t *testing.T) {
	data := map[string]interface{}{
		"str":   "hello",
		"num":   42.0,
		"float": 3.14,
		"bool":  true,
		"arr":   []interface{}{1, 2, 3},
		"obj":   map[string]interface{}{},
		"null":  nil,
	}
	jv, _ := NewJSONValue(data)

	tests := []struct {
		path     string
		expected string
	}{
		{"$.str", "string"},
		{"$.num", "integer"},
		{"$.float", "number"},
		{"$.bool", "boolean"},
		{"$.arr", "array"},
		{"$.obj", "object"},
		{"$.null", "null"},
		{"$.nonexistent", "null"},
	}

	for _, tt := range tests {
		result, err := jv.TypeAt(tt.path)
		if err != nil {
			t.Errorf("TypeAt(%s) error: %v", tt.path, err)
		}
		if result != tt.expected {
			t.Errorf("TypeAt(%s) = %s, expected %s", tt.path, result, tt.expected)
		}
	}
}

func TestJSONValueNumIncrBy(t *testing.T) {
	jv, _ := NewJSONValue(map[string]interface{}{"counter": 10.0})

	result, err := jv.NumIncrBy("$.counter", 5.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 15.0 {
		t.Errorf("expected 15, got %f", result)
	}
}

func TestJSONValueArrAppend(t *testing.T) {
	jv, _ := NewJSONValue(map[string]interface{}{
		"arr": []interface{}{1, 2, 3},
	})

	length, err := jv.ArrAppend("$.arr", []interface{}{4, 5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if length != 5 {
		t.Errorf("expected length 5, got %d", length)
	}
}

func TestJSONValueStrLen(t *testing.T) {
	jv, _ := NewJSONValue(map[string]interface{}{"text": "hello world"})

	length, err := jv.StrLen("$.text")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if length != 11 {
		t.Errorf("expected length 11, got %d", length)
	}
}

func TestJSONValueObjLen(t *testing.T) {
	jv, _ := NewJSONValue(map[string]interface{}{
		"obj": map[string]interface{}{"a": 1, "b": 2, "c": 3},
	})

	length, err := jv.ObjLen("$.obj")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if length != 3 {
		t.Errorf("expected length 3, got %d", length)
	}
}

func TestJSONValueArrLen(t *testing.T) {
	jv, _ := NewJSONValue(map[string]interface{}{
		"arr": []interface{}{1, 2, 3, 4},
	})

	length, err := jv.ArrLen("$.arr")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if length != 4 {
		t.Errorf("expected length 4, got %d", length)
	}
}

func TestParseJSONPath(t *testing.T) {
	tests := []struct {
		path     string
		expected []string
	}{
		{"", nil},
		{"$", nil},
		{".", nil},
		{"$.field", []string{"field"}},
		{"$.nested.field", []string{"nested", "field"}},
		{"$[0]", []string{"0"}},
		{"$.arr[1].field", []string{"arr", "1", "field"}},
	}

	for _, tt := range tests {
		result := parseJSONPath(tt.path)
		if len(result) != len(tt.expected) {
			t.Errorf("parseJSONPath(%s) = %v, expected %v", tt.path, result, tt.expected)
		}
	}
}

func TestGetByPathArray(t *testing.T) {
	data := map[string]interface{}{
		"arr": []interface{}{10.0, 20.0, 30.0},
	}

	result, err := getByPath(data, ".arr[1]")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 20.0 {
		t.Errorf("expected 20, got %v", result)
	}
}

func TestGetByPathInvalidIndex(t *testing.T) {
	data := map[string]interface{}{
		"arr": []interface{}{1, 2, 3},
	}

	result, err := getByPath(data, "$.arr[10]")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil for invalid index, got %v", result)
	}
}

func TestNewConsumerGroup(t *testing.T) {
	cg := NewConsumerGroup("group1")
	if cg == nil {
		t.Fatal("expected ConsumerGroup")
	}
	if cg.Name != "group1" {
		t.Errorf("expected name 'group1', got %s", cg.Name)
	}
}

func TestConsumerGroupGetOrCreateConsumer2(t *testing.T) {
	cg := NewConsumerGroup("group1")

	c1 := cg.GetOrCreateConsumer("consumer1")
	if c1 == nil {
		t.Fatal("expected consumer")
	}

	c2 := cg.GetOrCreateConsumer("consumer1")
	if c1 != c2 {
		t.Error("expected same consumer instance")
	}
}

func TestConsumerGroupAddPending2(t *testing.T) {
	cg := NewConsumerGroup("group1")
	cg.GetOrCreateConsumer("consumer1")

	cg.AddPending("1-0", "consumer1")

	if len(cg.Pending) != 1 {
		t.Errorf("expected 1 pending entry, got %d", len(cg.Pending))
	}
}

func TestConsumerGroupAck2(t *testing.T) {
	cg := NewConsumerGroup("group1")
	cg.GetOrCreateConsumer("consumer1")
	cg.AddPending("1-0", "consumer1")

	result := cg.Ack("1-0")
	if !result {
		t.Error("expected true for ack")
	}

	if len(cg.Pending) != 0 {
		t.Errorf("expected 0 pending entries, got %d", len(cg.Pending))
	}
}

func TestConsumerGroupAckNotFound(t *testing.T) {
	cg := NewConsumerGroup("group1")

	result := cg.Ack("nonexistent")
	if result {
		t.Error("expected false for nonexistent entry")
	}
}

func TestConsumerGroupClaim2(t *testing.T) {
	cg := NewConsumerGroup("group1")
	cg.GetOrCreateConsumer("consumer1")
	cg.GetOrCreateConsumer("consumer2")
	cg.AddPending("1-0", "consumer1")
	cg.AddPending("1-1", "consumer1")

	claimed := cg.Claim([]string{"1-0", "1-1"}, "consumer2")
	if len(claimed) != 2 {
		t.Errorf("expected 2 claimed entries, got %d", len(claimed))
	}
}

func TestConsumerGroupGetPending(t *testing.T) {
	cg := NewConsumerGroup("group1")
	cg.GetOrCreateConsumer("consumer1")
	cg.AddPending("1-0", "consumer1")
	cg.AddPending("1-1", "consumer1")
	cg.AddPending("2-0", "consumer1")

	pending := cg.GetPending("1-0", "1-9", 10)
	if len(pending) != 2 {
		t.Errorf("expected 2 pending entries, got %d", len(pending))
	}
}

func TestConsumerGroupGetPendingCount(t *testing.T) {
	cg := NewConsumerGroup("group1")
	cg.GetOrCreateConsumer("consumer1")
	cg.AddPending("1-0", "consumer1")
	cg.AddPending("1-1", "consumer1")

	count := cg.GetPendingCount()
	if count != 2 {
		t.Errorf("expected count 2, got %d", count)
	}
}

func TestConsumerGroupGetFirstLastID(t *testing.T) {
	cg := NewConsumerGroup("group1")
	cg.GetOrCreateConsumer("consumer1")
	cg.AddPending("1-0", "consumer1")
	cg.AddPending("1-5", "consumer1")
	cg.AddPending("1-3", "consumer1")

	first, last := cg.GetFirstLastID()
	if first != "1-0" {
		t.Errorf("expected first '1-0', got %s", first)
	}
	if last != "1-5" {
		t.Errorf("expected last '1-5', got %s", last)
	}
}

func TestConsumerGroupGetFirstLastIDEmpty(t *testing.T) {
	cg := NewConsumerGroup("group1")

	first, last := cg.GetFirstLastID()
	if first != "" {
		t.Errorf("expected empty first, got %s", first)
	}
	if last != "" {
		t.Errorf("expected empty last, got %s", last)
	}
}
