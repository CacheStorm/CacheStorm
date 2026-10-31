package main

import (
	"reflect"
	"testing"
)

func TestSplitArgs(t *testing.T) {
	for _, tc := range []struct {
		args         []string
		flags, redis []string
		file         string
	}{
		{[]string{"-config", "c.yaml", "-port", "7000"}, []string{"-config", "c.yaml", "-port", "7000"}, nil, ""},
		{[]string{"--config=c.yaml", "--version"}, []string{"--config=c.yaml", "--version"}, nil, ""},
		{[]string{"redis.conf", "--port", "7000", "--save", "", "--maxmemory", "1gb"}, []string{"--port", "7000"}, []string{"--save", "", "--maxmemory", "1gb"}, "redis.conf"},
		{[]string{"--requirepass", "pw", "--port", "7000"}, nil, []string{"--requirepass", "pw", "--port", "7000"}, ""},
	} {
		flags, file, redis := splitArgs(tc.args)
		if !reflect.DeepEqual(flags, tc.flags) || file != tc.file || !reflect.DeepEqual(redis, tc.redis) {
			t.Errorf("splitArgs(%q) = %q, %q, %q", tc.args, flags, file, redis)
		}
	}
}
