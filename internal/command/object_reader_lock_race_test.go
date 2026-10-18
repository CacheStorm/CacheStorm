package command

import (
	"sync"
	"testing"
)

// This test reuses runArrayCmd -- the generic "execute one command through a
// real Router" helper declared in array_reader_lock_race_test.go -- because
// that helper is command-agnostic.
//
// The OBJECT.* readers copied only the INNER MAP header under objectsMu.RLock,
// released the lock, and then read that map -- while cmdOBJECTSET wrote
// objects[name][key] and cmdOBJECTOMIT deleted keys in place under the write
// lock. On a Go map this is worse than the ARRAY slice race it mirrors:
// concurrent map read and map write is an unrecoverable runtime throw
// ("fatal error: concurrent map read and map write") that kills the process,
// not merely undefined behaviour.
//
// Only meaningful under -race, which is the project's CI gate
// (`go test -race ./internal/... -count=1`).
func TestObjectReadersDoNotRaceWithInPlaceWriters(t *testing.T) {
	const objName = "race-regression-object"

	router := NewRouter()
	RegisterIntegrationCommands(router)

	// Keep the package-global OBJECT state this test touches from leaking
	// into, or depending on, other tests in this package.
	t.Cleanup(func() {
		objectsMu.Lock()
		delete(objects, objName)
		objectsMu.Unlock()
	})

	// OBJECT.SET takes one field/value pair per call.
	for i := 0; i < 8; i++ {
		args := []string{"OBJECT.SET", objName, string(rune('a' + i)), string(rune('A' + i))}
		if err := runArrayCmd(router, args...); err != nil {
			t.Fatalf("%v errored: %v", args, err)
		}
	}

	readers := [][]string{
		{"OBJECT.GET", objName, "d"},
		{"OBJECT.HAS", objName, "d"},
		{"OBJECT.KEYS", objName},
		{"OBJECT.VALUES", objName},
		{"OBJECT.ENTRIES", objName},
		{"OBJECT.PICK", objName, "d"},
	}
	writers := [][]string{
		{"OBJECT.SET", objName, "d", "x"},
		{"OBJECT.OMIT", objName, "a"},
		{"OBJECT.SET", objName, "a", "z"},
	}

	var wg sync.WaitGroup
	for g := 0; g < 4; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 2000; i++ {
				for _, cmd := range readers {
					// Errors ignored off the test goroutine: t.Fatalf is not
					// legal there. The seeding above already asserts validity.
					_ = runArrayCmd(router, cmd...)
				}
			}
		}()
	}
	for g := 0; g < 2; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 2000; i++ {
				for _, cmd := range writers {
					_ = runArrayCmd(router, cmd...)
				}
			}
		}()
	}
	wg.Wait()

	// The object must still be addressable and coherent after the storm.
	if err := runArrayCmd(router, "OBJECT.HAS", objName, "a"); err != nil {
		t.Errorf("post-race OBJECT.HAS errored: %v", err)
	}
}
