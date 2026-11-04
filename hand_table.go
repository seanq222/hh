package hh

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"runtime"
	"slices"
	"sync"
)

// HandTable holds, for every hand, who played it and what each of them won:
// one row per hand, in order of site and hand ID, with the players in position
// order as in the views. It is the hands of a set of packs joined back together,
// for statistics about who wins from whom, which the packs, one player each,
// cannot answer without bringing every view of a hand together. hhpack
// writes it beside the packs.
//
// The columns are plain slices: hand i's players are Start[i] to Start[i+1] in
// the player columns. In every hand what the players won and the rake add up to
// what they put in, to the cent. The rake is the hand's; RakeShares divides it
// among the players.
type HandTable struct {
	Keys     []uint64 // HandKey(site, hand ID), ascending
	BigBlind []Cents
	Rake     []Cents  // the hand's rake and jackpot fee
	Start    []uint32 // len(Keys)+1 entries

	PackID   []uint16 // the player's pack (see Player.PackID)
	Invested []Cents  // what the player put in, less any bet returned to them
	Won      []Cents  // what they won, from the summary
	WonEV    []Cents  // what they were expected to win (see PlayerHand.WonEV)
}

// HandKey orders hands by site, then hand ID.
func HandKey(site Site, id int64) uint64 { return uint64(site)<<56 | uint64(id) }

// KeySite and KeyID take a HandKey apart.
func KeySite(k uint64) Site   { return Site(k >> 56) }
func KeyID(k uint64) int64    { return int64(k & (1<<56 - 1)) }
func (t *HandTable) Len() int { return len(t.Keys) }

// Players returns the range of hand i's players in the player columns.
func (t *HandTable) Players(i int) (lo, hi int) { return int(t.Start[i]), int(t.Start[i+1]) }

// Net returns what the player in row p of the player columns won less what they
// put in, and NetEV the same with all-in luck taken out.
func (t *HandTable) Net(p int) Cents   { return t.Won[p] - t.Invested[p] }
func (t *HandTable) NetEV(p int) Cents { return t.WonEV[p] - t.Invested[p] }

// RakeShares divides hand i's rake among its players into shares, one for each,
// in proportion to what they won, or with ev to what they were expected to win,
// so that the shares add up to the rake to the cent (see RakeShares).
func (t *HandTable) RakeShares(i int, ev bool, shares []Cents) {
	lo, hi := t.Players(i)
	won := t.Won[lo:hi]
	if ev {
		won = t.WonEV[lo:hi]
	}
	RakeShares(won, t.Rake[i], shares[:hi-lo])
}

// Find returns the row of a hand, by binary search.
func (t *HandTable) Find(site Site, id int64) (int, bool) {
	return slices.BinarySearch(t.Keys, HandKey(site, id))
}

// Each calls fn for ranges of rows that together cover the table once, on every
// CPU at once, and waits for them all.
func (t *HandTable) Each(fn func(lo, hi int)) {
	n := len(t.Keys)
	chunk := max(n/(16*runtime.NumCPU()), 1<<14)
	var wg sync.WaitGroup
	next := make(chan int, n/chunk+1)
	for lo := 0; lo < n; lo += chunk {
		next <- lo
	}
	close(next)
	for range runtime.NumCPU() {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for lo := range next {
				fn(lo, min(lo+chunk, n))
			}
		}()
	}
	wg.Wait()
}

// The file holds the columns one after another, every value little-endian:
//
//	"HHHT", version byte, hands (uvarint), players (uvarint)
//	Keys [hands]uint64, BigBlind, Rake [hands]int32, players per hand [hands]uint8,
//	PackID [players]uint16, Invested, Won, WonEV [players]int32
//	"HHTE"
//
// Each value is packed when written and unpacked when read.
const (
	handTableMagic   = "HHHT"
	handTableEnd     = "HHTE"
	handTableVersion = 1
)

// columnSizes are the bytes of each column, in order.
func columnSizes(hands, players int) [8]int {
	return [8]int{8 * hands, 4 * hands, 4 * hands, hands, 2 * players, 4 * players, 4 * players, 4 * players}
}

// WriteHandTable writes the table to a file.
func WriteHandTable(path string, t *HandTable) error {
	n, m := len(t.Keys), len(t.PackID)
	if len(t.BigBlind) != n || len(t.Rake) != n || len(t.Start) != n+1 || int(t.Start[n]) != m ||
		len(t.Invested) != m || len(t.Won) != m || len(t.WonEV) != m {
		return errors.New("hh: hand table columns do not match")
	}
	for i := range n {
		if c := t.Start[i+1] - t.Start[i]; c > 255 {
			return fmt.Errorf("hh: hand %d has %d players", KeyID(t.Keys[i]), c)
		}
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	w := bufio.NewWriterSize(f, 1<<20)
	hdr := append([]byte(handTableMagic), handTableVersion)
	hdr = binary.AppendUvarint(hdr, uint64(n))
	hdr = binary.AppendUvarint(hdr, uint64(m))
	w.Write(hdr)
	// Pack each column a chunk at a time.
	buf := make([]byte, 0, 1<<16)
	put := func(count int, appendOne func(b []byte, i int) []byte) {
		for i := range count {
			buf = appendOne(buf, i)
			if len(buf) >= 1<<16-8 {
				w.Write(buf)
				buf = buf[:0]
			}
		}
		w.Write(buf)
		buf = buf[:0]
	}
	le := binary.LittleEndian
	cents := func(col []Cents) func(b []byte, i int) []byte {
		return func(b []byte, i int) []byte { return le.AppendUint32(b, uint32(col[i])) }
	}
	put(n, func(b []byte, i int) []byte { return le.AppendUint64(b, t.Keys[i]) })
	put(n, cents(t.BigBlind))
	put(n, cents(t.Rake))
	put(n, func(b []byte, i int) []byte { return append(b, byte(t.Start[i+1]-t.Start[i])) })
	put(m, func(b []byte, i int) []byte { return le.AppendUint16(b, t.PackID[i]) })
	put(m, cents(t.Invested))
	put(m, cents(t.Won))
	put(m, cents(t.WonEV))
	w.WriteString(handTableEnd)
	err = w.Flush()
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return err
}

// ReadHandTable reads a table written by WriteHandTable, unpacking its columns
// on several CPUs at once.
func ReadHandTable(path string) (*HandTable, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(b) < 5 || string(b[:4]) != handTableMagic || b[4] != handTableVersion {
		return nil, ErrCorrupt
	}
	r := b[5:]
	n64, k := binary.Uvarint(r)
	if k <= 0 {
		return nil, ErrCorrupt
	}
	r = r[k:]
	m64, k := binary.Uvarint(r)
	if k <= 0 || n64 > 1<<32 || m64 > 1<<32 {
		return nil, ErrCorrupt
	}
	r = r[k:]
	n, m := int(n64), int(m64)
	sizes := columnSizes(n, m)
	total := 0
	for _, s := range sizes {
		total += s
	}
	if len(r) != total+len(handTableEnd) || string(r[total:]) != handTableEnd {
		return nil, ErrCorrupt
	}
	var cols [8][]byte
	for i, s := range sizes {
		cols[i], r = r[:s], r[s:]
	}
	t := &HandTable{
		Keys: make([]uint64, n), BigBlind: make([]Cents, n), Rake: make([]Cents, n), Start: make([]uint32, n+1),
		PackID: make([]uint16, m), Invested: make([]Cents, m), Won: make([]Cents, m), WonEV: make([]Cents, m),
	}
	le := binary.LittleEndian
	var wg sync.WaitGroup
	unpack := func(fn func()) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			fn()
		}()
	}
	unpack(func() {
		for i := range t.Keys {
			t.Keys[i] = le.Uint64(cols[0][8*i:])
		}
	})
	unpack(func() {
		for i := range t.PackID {
			t.PackID[i] = le.Uint16(cols[4][2*i:])
		}
	})
	for c, dst := range map[int][]Cents{1: t.BigBlind, 2: t.Rake, 5: t.Invested, 6: t.Won, 7: t.WonEV} {
		unpack(func() {
			for i := range dst {
				dst[i] = Cents(le.Uint32(cols[c][4*i:]))
			}
		})
	}
	var at uint64
	for i, c := range cols[3] {
		t.Start[i] = uint32(at)
		at += uint64(c)
	}
	wg.Wait()
	if at != m64 {
		return nil, ErrCorrupt
	}
	t.Start[n] = uint32(at)
	for i := 1; i < n; i++ {
		if t.Keys[i] <= t.Keys[i-1] {
			return nil, ErrCorrupt // not in order, or a hand twice
		}
	}
	return t, nil
}

// Pot is one layer of a hand's pot, rebuilt from what each player put in: every
// player who put in at least its level contributed to it, and its winners
// shared it equally. Contrib and Winners are by the hand's player order.
type Pot struct {
	Size    Cents
	Contrib [9]Cents
	Winners uint16 // bit per player
}

// Pots rebuilds hand i's pots before rake from what each player put in and won:
// a layer for each level a player stopped at, contributed to by everyone who put
// in that much. Each layer goes to the winners who reached it, working down
// from the top, shared equally, while a winner still has something they won to
// account for; the rake is taken from all the pots alike. ok is false when what
// was won cannot be matched to the pots that way, and the pots should not be
// used.
func (t *HandTable) Pots(i int) (pots []Pot, ok bool) {
	lo, hi := t.Players(i)
	n := hi - lo
	if n > 9 {
		return nil, false
	}
	inv, won := t.Invested[lo:hi], t.Won[lo:hi]
	var levels []Cents
	var total, totalWon Cents
	for p := range n {
		total += inv[p]
		totalWon += max(won[p], 0)
		if inv[p] > 0 && !slices.Contains(levels, inv[p]) {
			levels = append(levels, inv[p])
		}
	}
	slices.Sort(levels)
	if total <= 0 || totalWon <= 0 {
		return nil, false
	}
	// What each winner still has to account for, before rake: the rake comes
	// out of every pot in proportion, so before rake each winner won
	// won * total / totalWon.
	var left [9]float64
	for p := range n {
		if won[p] > 0 {
			left[p] = float64(won[p]) * float64(total) / float64(totalWon)
		}
	}
	pots = make([]Pot, len(levels))
	prev := Cents(0)
	for k, level := range levels {
		pot := &pots[k]
		for p := range n {
			c := min(inv[p], level) - min(inv[p], prev)
			pot.Contrib[p] = c
			pot.Size += c
		}
		prev = level
	}
	// The top pot first: only the winners who put in that much can have won it.
	for k := len(pots) - 1; k >= 0; k-- {
		pot := &pots[k]
		level := levels[k]
		var count int
		for p := range n {
			if won[p] > 0 && inv[p] >= level && left[p] > 0.5 {
				pot.Winners |= 1 << p
				count++
			}
		}
		if count == 0 {
			return nil, false
		}
		share := float64(pot.Size) / float64(count)
		for p := range n {
			if pot.Winners&(1<<p) != 0 {
				left[p] -= share
			}
		}
	}
	// Every winner's winnings must be accounted for, to within the odd chips.
	for p := range n {
		if left[p] > float64(n) || left[p] < -float64(n) {
			return nil, false
		}
	}
	return pots, true
}
