package command

import (
	"bytes"
	"github.com/cachestorm/cachestorm/internal/acl"
	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
	"strings"
	"testing"
)

func TestSetAccessKeysX(t *testing.T) {
	for _, name := range []string{"SUNION", "SINTER", "SDIFF", "SUNIONSTORE", "SINTERSTORE", "SDIFFSTORE", "SMOVE"} {
		t.Run(name, func(t *testing.T) {
			s := store.NewStore()
			r := NewRouter()
			RegisterSetCommands(r)
			for _, key := range []string{"user:s", "secret", "user:out"} {
				member := "public"
				if key == "secret" {
					member = "classified"
				}
				if _, err := s.AddSetMembers(key, []string{member}); err != nil {
					t.Fatal(err)
				}
			}
			u, err := acl.NewACL().CreateUser("limited")
			if err != nil {
				t.Fatal(err)
			}
			u.Enable()
			u.AllowCommand("*")
			u.AllowKey("user:*")
			run := func(args ...string) string {
				var out bytes.Buffer
				ctx := NewContext(name, bytesArgs(args...), s, resp.NewWriter(&out))
				ctx.ACLUser = u
				if err := r.Execute(ctx); err != nil {
					t.Fatal(err)
				}
				return out.String()
			}
			control := []string{"user:s"}
			denied := []string{"user:s", "secret"}
			if name == "SINTER" {
				denied = []string{"secret", "secret"}
			}
			if name == "SDIFF" {
				denied = []string{"secret", "user:s"}
			}
			if strings.HasSuffix(name, "STORE") {
				control = []string{"user:out", "user:s"}
				denied = []string{"user:out", "secret"}
			}
			if name == "SMOVE" {
				control = []string{"user:s", "user:out", "absent"}
				denied = []string{"user:s", "secret", "public"}
			}
			if got := run(control...); strings.HasPrefix(got, "-") {
				t.Fatal("CONTROL authorized keys rejected", got)
			}
			got := run(denied...)
			t.Logf("EXPECTED: NOPERM | ACTUAL: %q", got)
			if !strings.HasPrefix(got, "-NOPERM") {
				t.Error("out-of-scope key accepted")
			}
		})
	}
}
