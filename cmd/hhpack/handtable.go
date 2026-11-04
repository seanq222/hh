package main

import (
	"cmp"
	"fmt"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"sync"

	"github.com/seanq222/hh"
)

// The hand table (see hh.HandTable) is built from the hands of pass 2, which
// finish in no particular order, and written in order of site and hand ID. The
// hands are spread over buckets by ranges of their keys, so that sorting each
// bucket on its own, all on different CPUs, sorts the whole: the buckets need
// only be laid end to end. The ranges come from a sample of the keys taken after
// pass 1, so that the buckets are about the same size.
const (
	tableBuckets = 1024
	tableSample  = 10000
)

// priority is a key's place in the sample: the keys with the lowest priorities
// are the sample. It is a hash, so the sample is the same on every run and a
// hand found in several files counts once.
func priority(key uint64) uint64 {
	s := splitmix(key)
	return s.next()
}

// sampleKeys returns, in order, the k keys of the parsed hands with the lowest
// priorities: a uniform sample of the hands. Each shard keeps its own k lowest,
// and the lowest k of those are the sample.
func (s *survey) sampleKeys(k int) []uint64 {
	type cand struct{ prio, key uint64 }
	parts := make([][]cand, len(s.shards))
	var wg sync.WaitGroup
	for i := range s.shards {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var c []cand
			limit := ^uint64(0)
			for key, e := range s.shards[i].hands {
				if !e.parsed {
					continue
				}
				p := priority(key)
				if p >= limit {
					continue
				}
				c = append(c, cand{p, key})
				if len(c) >= 4*k { // keep the k lowest, now and then
					slices.SortFunc(c, func(a, b cand) int { return cmp.Compare(a.prio, b.prio) })
					c = c[:k]
					limit = c[k-1].prio
				}
			}
			parts[i] = c
		}()
	}
	wg.Wait()
	var all []cand
	for _, p := range parts {
		all = append(all, p...)
	}
	slices.SortFunc(all, func(a, b cand) int { return cmp.Compare(a.prio, b.prio) })
	all = all[:min(k, len(all))]
	keys := make([]uint64, len(all))
	for i, c := range all {
		keys[i] = c.key
	}
	slices.Sort(keys)
	return keys
}

// tablePlayer is a player's part in a hand, in the order of the views.
type tablePlayer struct {
	packID               uint16
	invested, won, wonEV hh.Cents
}

// tableHand is a hand waiting to be written, its players at players[start:]
// in its bucket.
type tableHand struct {
	key      uint64
	bb, rake hh.Cents
	start    uint32
	n        uint8
}

// fileHand is a hand of one file, before it goes to its bucket.
type fileHand struct {
	bucket  int
	hand    tableHand
	players [9]tablePlayer
}

type tableBuilder struct {
	bounds []uint64 // bucket i holds the keys below bounds[i] and at or above bounds[i-1]
	mu     sync.Mutex
	hands  [tableBuckets][]tableHand
	ps     [tableBuckets][]tablePlayer
	broken int // hands that could not be put in the table
}

func newTableBuilder(sample []uint64) *tableBuilder {
	tb := &tableBuilder{}
	for i := 1; i < tableBuckets; i++ {
		if len(sample) > 0 {
			tb.bounds = append(tb.bounds, sample[i*len(sample)/tableBuckets])
		}
	}
	return tb
}

func (tb *tableBuilder) bucket(key uint64) int {
	return sort.Search(len(tb.bounds), func(i int) bool { return key < tb.bounds[i] })
}

// hand makes a hand's row from its views, which must be one for each player.
func (tb *tableBuilder) hand(site hh.Site, id int64, views []hh.PlayerHand, slotID *[16]uint16) (fileHand, bool) {
	var fh fileHand
	if len(views) == 0 || len(views) > 9 || len(views) != len(views[0].Players) {
		return fh, false
	}
	key := hh.HandKey(site, id)
	fh.bucket = tb.bucket(key)
	fh.hand = tableHand{key: key, bb: views[0].BigBlind, n: uint8(len(views))}
	var have uint16
	for k := range views {
		v := &views[k]
		fh.players[v.Hero] = tablePlayer{packID: slotID[v.Hero], invested: v.Invested(), won: v.Won, wonEV: v.WonEV}
		fh.hand.rake = max(fh.hand.rake, v.Rake) // the hand's rake is in the views of those who did not fold
		have |= 1 << v.Hero
	}
	return fh, have == 1<<len(views)-1
}

// add puts a file's hands in their buckets, under one lock for the file.
func (tb *tableBuilder) add(hands []fileHand, broken int) {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	tb.broken += broken
	for i := range hands {
		fh := &hands[i]
		b := fh.bucket
		fh.hand.start = uint32(len(tb.ps[b]))
		tb.hands[b] = append(tb.hands[b], fh.hand)
		tb.ps[b] = append(tb.ps[b], fh.players[:fh.hand.n]...)
	}
}

// write sorts the buckets, each on its own CPU, lays them end to end and writes
// the table to hands.hht in dir. It returns the number of hands.
func (tb *tableBuilder) write(dir string) (int, error) {
	if tb.broken > 0 {
		return 0, fmt.Errorf("%d hands lack a view for some player; no hand table written", tb.broken)
	}
	var handsAt, playersAt [tableBuckets + 1]int
	for b := range tableBuckets {
		handsAt[b+1] = handsAt[b] + len(tb.hands[b])
		playersAt[b+1] = playersAt[b] + len(tb.ps[b])
	}
	n, m := handsAt[tableBuckets], playersAt[tableBuckets]
	t := &hh.HandTable{
		Keys: make([]uint64, n), BigBlind: make([]hh.Cents, n), Rake: make([]hh.Cents, n), Start: make([]uint32, n+1),
		PackID: make([]uint16, m), Invested: make([]hh.Cents, m), Won: make([]hh.Cents, m), WonEV: make([]hh.Cents, m),
	}
	t.Start[n] = uint32(m)
	next := make(chan int, tableBuckets)
	for b := range tableBuckets {
		next <- b
	}
	close(next)
	var wg sync.WaitGroup
	var dupMu sync.Mutex
	var dupErr error
	for range runtime.NumCPU() {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for b := range next {
				hs, ps := tb.hands[b], tb.ps[b]
				slices.SortFunc(hs, func(x, y tableHand) int { return cmp.Compare(x.key, y.key) })
				i, p := handsAt[b], playersAt[b]
				for j := range hs {
					h := &hs[j]
					if j > 0 && h.key == hs[j-1].key {
						dupMu.Lock()
						dupErr = fmt.Errorf("hand %d is in the table twice", hh.KeyID(h.key))
						dupMu.Unlock()
					}
					t.Keys[i], t.BigBlind[i], t.Rake[i], t.Start[i] = h.key, h.bb, h.rake, uint32(p)
					for _, pl := range ps[h.start : h.start+uint32(h.n)] {
						t.PackID[p], t.Invested[p], t.Won[p], t.WonEV[p] = pl.packID, pl.invested, pl.won, pl.wonEV
						p++
					}
					i++
				}
				tb.hands[b], tb.ps[b] = nil, nil // let the bucket's memory go
			}
		}()
	}
	wg.Wait()
	if dupErr != nil {
		return 0, dupErr
	}
	return n, hh.WriteHandTable(filepath.Join(dir, "hands.hht"), t)
}
