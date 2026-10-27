package command

import (
	"strings"
	"sync"
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

// apiKeyFromReply extracts the generated key from an APIKEY.CREATE reply
// ("$<len>\r\n<key>\r\n").
func apiKeyFromReply(t *testing.T, reply string) string {
	t.Helper()
	if !strings.HasPrefix(reply, "$") {
		t.Fatalf("APIKEY.CREATE reply = %q, want a bulk string", reply)
	}
	end := strings.Index(reply, "\r\n")
	if end < 0 || len(reply) < end+4 {
		t.Fatalf("APIKEY.CREATE reply = %q, want a bulk string", reply)
	}
	return reply[end+2 : len(reply)-2]
}

// TestAPIKeyUsageIsSafeWhileAPIKeyValidateWrites reads an API key's usage
// while another client validates it.
//
// The read path took the map lock only to look the key up and then released
// it, reading the key's Usage counter with no lock at all while
// APIKEY.VALIDATE incremented that counter under the write lock.
func TestAPIKeyUsageIsSafeWhileAPIKeyValidateWrites(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMoreCommands(router)

	const iterations = 2000

	key := apiKeyFromReply(t, registryRun(t, router, s, "APIKEY.CREATE", "race-key"))

	var wg sync.WaitGroup
	start := make(chan struct{})

	// Writer: APIKEY.VALIDATE increments the usage counter.
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < iterations; i++ {
			ctx, _ := bufCtx("APIKEY.VALIDATE", bytesArgs(key), s)
			if err := router.Execute(ctx); err != nil {
				t.Errorf("APIKEY.VALIDATE: unexpected error: %v", err)
				return
			}
		}
	}()

	for r := 0; r < 2; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for i := 0; i < iterations; i++ {
				ctx, buf := bufCtx("APIKEY.USAGE", bytesArgs(key), s)
				if err := router.Execute(ctx); err != nil {
					t.Errorf("APIKEY.USAGE: unexpected error: %v", err)
					return
				}
				if reply := buf.String(); !strings.HasPrefix(reply, ":") {
					t.Errorf("APIKEY.USAGE: expected an integer reply, got %q", reply)
					return
				}
			}
		}()
	}

	close(start)
	wg.Wait()
}

// TestAlertHistoryIsSafeWhileAlertTriggerWrites reads an alert's history
// while another client triggers it.
//
// The read path took the map lock only to look the alert up and then released
// it, reading the alert's Status string with no lock at all while
// ALERT.TRIGGER rewrote that string under the write lock.
func TestAlertHistoryIsSafeWhileAlertTriggerWrites(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMoreCommands(router)

	const iterations = 2000

	registryCleanup(t, &alertsMu, alerts, "alert-race")
	if reply := registryRun(t, router, s, "ALERT.CREATE", "alert-race", "message"); reply != "+OK\r\n" {
		t.Fatalf("ALERT.CREATE reply = %q, want +OK", reply)
	}

	var wg sync.WaitGroup
	start := make(chan struct{})

	// Writer: ALERT.TRIGGER rewrites the status string.
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < iterations; i++ {
			ctx, _ := bufCtx("ALERT.TRIGGER", bytesArgs("alert-race"), s)
			if err := router.Execute(ctx); err != nil {
				t.Errorf("ALERT.TRIGGER: unexpected error: %v", err)
				return
			}
		}
	}()

	for r := 0; r < 2; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for i := 0; i < iterations; i++ {
				ctx, buf := bufCtx("ALERT.HISTORY", bytesArgs("alert-race"), s)
				if err := router.Execute(ctx); err != nil {
					t.Errorf("ALERT.HISTORY: unexpected error: %v", err)
					return
				}
				if reply := buf.String(); !strings.HasPrefix(reply, "*") {
					t.Errorf("ALERT.HISTORY: expected a RESP array reply, got %q", reply)
					return
				}
			}
		}()
	}

	close(start)
	wg.Wait()
}

// TestLeaseGetIsSafeWhileLeaseRenewWrites reads a lease while another client
// renews it.
//
// The read path took the map lock only to look the lease up and then released
// it, reading the lease's expiry with no lock at all while LEASE.RENEW wrote
// that expiry under the write lock.
func TestLeaseGetIsSafeWhileLeaseRenewWrites(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMoreCommands(router)

	const iterations = 2000

	registryCleanup(t, &leasesMu, leases, "lease-race")
	if reply := registryRun(t, router, s, "LEASE.CREATE", "lease-race", "holder", "60000"); reply != "+OK\r\n" {
		t.Fatalf("LEASE.CREATE reply = %q, want +OK", reply)
	}

	var wg sync.WaitGroup
	start := make(chan struct{})

	// Writer: LEASE.RENEW moves the expiry.
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < iterations; i++ {
			ctx, _ := bufCtx("LEASE.RENEW", bytesArgs("lease-race", "60000"), s)
			if err := router.Execute(ctx); err != nil {
				t.Errorf("LEASE.RENEW: unexpected error: %v", err)
				return
			}
		}
	}()

	for r := 0; r < 2; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for i := 0; i < iterations; i++ {
				ctx, buf := bufCtx("LEASE.GET", bytesArgs("lease-race"), s)
				if err := router.Execute(ctx); err != nil {
					t.Errorf("LEASE.GET: unexpected error: %v", err)
					return
				}
				if reply := buf.String(); !strings.HasPrefix(reply, "*") {
					t.Errorf("LEASE.GET: expected a RESP array reply, got %q", reply)
					return
				}
			}
		}()
	}

	close(start)
	wg.Wait()
}

// TestIdempotencyGetIsSafeWhileIdempotencyReplaceWrites reads an idempotency
// entry while another client republishes it.
//
// IDEMPOTENCY.GET releases the map lock right after the lookup and then reads
// the entry's response with no lock at all. The entry itself is never
// mutated — IDEMPOTENCY.SET publishes a NEW entry and refuses to overwrite
// one — so the unlocked read touches memory no writer ever writes again.
// The test pins that conclusion rather than assuming it: if a writer ever
// mutated a published entry, the race detector would report it.
func TestIdempotencyGetIsSafeWhileIdempotencyReplaceWrites(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMoreCommands(router)

	const iterations = 2000

	registryCleanup(t, &idempotencyKeysMu, idempotencyKeys, "idem-race")
	if reply := registryRun(t, router, s, "IDEMPOTENCY.SET", "idem-race", "response", "60000"); reply != "+OK\r\n" {
		t.Fatalf("IDEMPOTENCY.SET reply = %q, want +OK", reply)
	}

	var wg sync.WaitGroup
	start := make(chan struct{})

	// Writer: delete and republish the entry, so the map slot is replaced
	// again and again while the reader reads the published entry.
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < iterations; i++ {
			for _, cmd := range []string{"IDEMPOTENCY.DELETE", "IDEMPOTENCY.SET"} {
				ctx, _ := bufCtx(cmd, bytesArgs("idem-race", "response", "60000"), s)
				if err := router.Execute(ctx); err != nil {
					// IDEMPOTENCY.SET refuses to overwrite, which is
					// expected on every other iteration.
					_ = err
				}
			}
		}
	}()

	for r := 0; r < 2; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for i := 0; i < iterations; i++ {
				ctx, buf := bufCtx("IDEMPOTENCY.GET", bytesArgs("idem-race"), s)
				if err := router.Execute(ctx); err != nil {
					t.Errorf("IDEMPOTENCY.GET: unexpected error: %v", err)
					return
				}
				reply := buf.String()
				if !strings.HasPrefix(reply, "$") && !strings.HasPrefix(reply, "_") {
					t.Errorf("IDEMPOTENCY.GET: expected a bulk string or null reply, got %q", reply)
					return
				}
			}
		}()
	}

	close(start)
	wg.Wait()
}

// TestIdempotencyCheckIsSafeWhileIdempotencyReplaceWrites checks an
// idempotency entry while another client republishes it. The entry's expiry
// is never mutated after publication, so the unlocked read is safe — the
// test pins that conclusion.
func TestIdempotencyCheckIsSafeWhileIdempotencyReplaceWrites(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMoreCommands(router)

	const iterations = 2000

	registryCleanup(t, &idempotencyKeysMu, idempotencyKeys, "idem-check-race")
	if reply := registryRun(t, router, s, "IDEMPOTENCY.SET", "idem-check-race", "response", "60000"); reply != "+OK\r\n" {
		t.Fatalf("IDEMPOTENCY.SET reply = %q, want +OK", reply)
	}

	var wg sync.WaitGroup
	start := make(chan struct{})

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < iterations; i++ {
			for _, cmd := range []string{"IDEMPOTENCY.DELETE", "IDEMPOTENCY.SET"} {
				ctx, _ := bufCtx(cmd, bytesArgs("idem-check-race", "response", "60000"), s)
				if err := router.Execute(ctx); err != nil {
					_ = err
				}
			}
		}
	}()

	for r := 0; r < 2; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for i := 0; i < iterations; i++ {
				ctx, buf := bufCtx("IDEMPOTENCY.CHECK", bytesArgs("idem-check-race"), s)
				if err := router.Execute(ctx); err != nil {
					t.Errorf("IDEMPOTENCY.CHECK: unexpected error: %v", err)
					return
				}
				if reply := buf.String(); !strings.HasPrefix(reply, ":") {
					t.Errorf("IDEMPOTENCY.CHECK: expected an integer reply, got %q", reply)
					return
				}
			}
		}()
	}

	close(start)
	wg.Wait()
}

// TestTenantGetIsSafeWhileTenantConfigWrites reads a tenant while another
// client writes its configuration.
//
// TENANT.GET releases the map lock right after the lookup and then reads the
// tenant's immutable fields. TENANT.CONFIG writes the tenant's Config map —
// a different field — so the two do not overlap. The test pins that
// conclusion: if the reader ever touched Config, the race detector would
// report it.
func TestTenantGetIsSafeWhileTenantConfigWrites(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMoreCommands(router)

	const iterations = 2000

	registryCleanup(t, &tenantsMu, tenants, "tenant-race")
	if reply := registryRun(t, router, s, "TENANT.CREATE", "tenant-race", "name"); reply != "+OK\r\n" {
		t.Fatalf("TENANT.CREATE reply = %q, want +OK", reply)
	}

	var wg sync.WaitGroup
	start := make(chan struct{})

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < iterations; i++ {
			ctx, _ := bufCtx("TENANT.CONFIG", bytesArgs("tenant-race", "key", "value"), s)
			if err := router.Execute(ctx); err != nil {
				t.Errorf("TENANT.CONFIG: unexpected error: %v", err)
				return
			}
		}
	}()

	for r := 0; r < 2; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for i := 0; i < iterations; i++ {
				ctx, buf := bufCtx("TENANT.GET", bytesArgs("tenant-race"), s)
				if err := router.Execute(ctx); err != nil {
					t.Errorf("TENANT.GET: unexpected error: %v", err)
					return
				}
				if reply := buf.String(); !strings.HasPrefix(reply, "*") {
					t.Errorf("TENANT.GET: expected a RESP array reply, got %q", reply)
					return
				}
			}
		}()
	}

	close(start)
	wg.Wait()
}

// TestAPIKeyUsageReportsValidatedCount is the control case: with no
// concurrency the usage counter must report exactly how many validations
// happened. It must pass both before and after the fix, so a broken harness
// cannot masquerade as the defect.
func TestAPIKeyUsageReportsValidatedCount(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterMoreCommands(router)

	key := apiKeyFromReply(t, registryRun(t, router, s, "APIKEY.CREATE", "control-key"))

	for i := 0; i < 3; i++ {
		if reply := registryRun(t, router, s, "APIKEY.VALIDATE", key); reply != ":1\r\n" {
			t.Fatalf("APIKEY.VALIDATE reply = %q, want :1", reply)
		}
	}
	if reply := registryRun(t, router, s, "APIKEY.USAGE", key); reply != ":3\r\n" {
		t.Fatalf("APIKEY.USAGE reply = %q, want :3", reply)
	}
}
