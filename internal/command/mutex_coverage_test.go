package command

import (
	"testing"
)

// TestSyncRWMutexLock tests syncRWMutex Lock method
func TestSyncRWMutexLock(t *testing.T) {
	var m syncRWMutex
	// These should not panic
	m.Lock()
}

// TestSyncRWMutexUnlock tests syncRWMutex Unlock method
func TestSyncRWMutexUnlock(t *testing.T) {
	var m syncRWMutex
	m.Lock()
	m.Unlock()
}

// TestSyncRWMutexRLock tests syncRWMutex RLock method
func TestSyncRWMutexRLock(t *testing.T) {
	var m syncRWMutex
	// These should not panic
	m.RLock()
}

// TestSyncRWMutexRUnlock tests syncRWMutex RUnlock method
func TestSyncRWMutexRUnlock(t *testing.T) {
	var m syncRWMutex
	m.RLock()
	m.RUnlock()
}

// TestSyncRWMutexMultipleOperations tests syncRWMutex with multiple operations
func TestSyncRWMutexMultipleOperations(t *testing.T) {
	var m syncRWMutex

	// Multiple lock/unlock cycles
	for i := 0; i < 10; i++ {
		m.Lock()
		m.Unlock()
	}

	// Multiple rlock/runlock cycles
	for i := 0; i < 10; i++ {
		m.RLock()
		m.RUnlock()
	}

	// Mixed operations
	m.Lock()
	m.Unlock()
	m.RLock()
	m.RUnlock()
	m.Lock()
	m.Unlock()
}

// TestSyncRWMutexConcurrent tests syncRWMutex with concurrent access
func TestSyncRWMutexConcurrent(t *testing.T) {
	var m syncRWMutex
	done := make(chan bool, 10)

	// Concurrent writes
	for i := 0; i < 5; i++ {
		go func() {
			m.Lock()
			m.Unlock()
			done <- true
		}()
	}

	// Concurrent reads
	for i := 0; i < 5; i++ {
		go func() {
			m.RLock()
			m.RUnlock()
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

// TestSyncRWMutexExtLock tests syncRWMutexExt Lock method
func TestSyncRWMutexExtLock(t *testing.T) {
	var m syncRWMutexExt
	// These should not panic
	m.Lock()
}

// TestSyncRWMutexExtUnlock tests syncRWMutexExt Unlock method
func TestSyncRWMutexExtUnlock(t *testing.T) {
	var m syncRWMutexExt
	m.Lock()
	m.Unlock()
}

// TestSyncRWMutexExtRLock tests syncRWMutexExt RLock method
func TestSyncRWMutexExtRLock(t *testing.T) {
	var m syncRWMutexExt
	// These should not panic
	m.RLock()
}

// TestSyncRWMutexExtRUnlock tests syncRWMutexExt RUnlock method
func TestSyncRWMutexExtRUnlock(t *testing.T) {
	var m syncRWMutexExt
	m.RLock()
	m.RUnlock()
}

// TestSyncRWMutexExtMultipleOperations tests syncRWMutexExt with multiple operations
func TestSyncRWMutexExtMultipleOperations(t *testing.T) {
	var m syncRWMutexExt

	// Multiple lock/unlock cycles
	for i := 0; i < 10; i++ {
		m.Lock()
		m.Unlock()
	}

	// Multiple rlock/runlock cycles
	for i := 0; i < 10; i++ {
		m.RLock()
		m.RUnlock()
	}
}

// TestSyncRWMutexExtConcurrent tests syncRWMutexExt with concurrent access
func TestSyncRWMutexExtConcurrent(t *testing.T) {
	var m syncRWMutexExt
	done := make(chan bool, 10)

	// Concurrent writes
	for i := 0; i < 5; i++ {
		go func() {
			m.Lock()
			m.Unlock()
			done <- true
		}()
	}

	// Concurrent reads
	for i := 0; i < 5; i++ {
		go func() {
			m.RLock()
			m.RUnlock()
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}
