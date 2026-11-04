package clean

import (
	"sync"
	"sync/atomic"

	"github.com/seanq222/hh"
)

// Seen remembers hands by site and ID, so that a hand found in more than one file
// is used once. The sites number their hands independently, so the same ID on two
// sites is two hands. It is a bitmap in pages of 2^20 IDs, made when an ID in the
// page first turns up, so it takes about 128 KB for each million IDs the hands are
// spread over, however many hands there are. It is safe for concurrent use.
type Seen struct {
	mu    sync.RWMutex
	pages map[uint64]*[1 << 14]uint64
}

// NewSeen returns an empty set.
func NewSeen() *Seen { return &Seen{pages: map[uint64]*[1 << 14]uint64{}} }

// Add records the hand and reports whether it was new.
func (s *Seen) Add(site hh.Site, id int64) bool {
	page, bit := uint64(id)>>20|uint64(site)<<56, uint64(id)&(1<<20-1)
	s.mu.RLock()
	pg := s.pages[page]
	s.mu.RUnlock()
	if pg == nil {
		s.mu.Lock()
		if pg = s.pages[page]; pg == nil {
			pg = new([1 << 14]uint64)
			s.pages[page] = pg
		}
		s.mu.Unlock()
	}
	mask := uint64(1) << (bit & 63)
	return atomic.OrUint64(&pg[bit>>6], mask)&mask == 0
}
