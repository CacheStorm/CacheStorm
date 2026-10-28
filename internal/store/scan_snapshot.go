package store

import "sync/atomic"

// ScanSnapshot is a collection's members ordered by their scan hash, kept
// between the pages of an SSCAN, HSCAN or ZSCAN so a page costs a binary
// search instead of a pass over the collection.
type ScanSnapshot struct {
	Hashes  []uint64
	Members []string
}

// scanCache holds a collection's ScanSnapshot until the collection changes.
type scanCache struct {
	snap atomic.Pointer[ScanSnapshot]
}

// LoadScan returns the snapshot, nil when there is none or the collection
// changed since it was taken.
func (c *scanCache) LoadScan() *ScanSnapshot { return c.snap.Load() }

// StoreScan keeps s for the next pages. The caller holds the collection's
// lock, at least for reading, and s describes its current members.
func (c *scanCache) StoreScan(s *ScanSnapshot) { c.snap.Store(s) }

func (c *scanCache) invalidateScan() {
	if c.snap.Load() != nil {
		c.snap.Store(nil)
	}
}
