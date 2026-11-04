package hh

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sync/atomic"
	"testing"
)

func sampleHandTable() *HandTable {
	return &HandTable{
		Keys:     []uint64{HandKey(WPN, 1000001), HandKey(WPN, 1000005), HandKey(CoinPoker, 7)},
		BigBlind: []Cents{50, 100, 200},
		Rake:     []Cents{5, 0, 10},
		Start:    []uint32{0, 3, 5, 8},
		PackID:   []uint16{1, 2, 3, 2, 300, 1, 3, 65535},
		Invested: []Cents{50, 50, 25, 120, 120, 200, 210, 210},
		Won:      []Cents{0, 120, 0, 240, 0, 0, 610, 0},
		WonEV:    []Cents{0, 115, 5, 240, 0, 50, 560, 0},
	}
}

func TestHandTableRoundTrip(t *testing.T) {
	want := sampleHandTable()
	path := filepath.Join(t.TempDir(), "hands.hht")
	if err := WriteHandTable(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := ReadHandTable(path)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got.Keys, want.Keys) || !slices.Equal(got.BigBlind, want.BigBlind) || !slices.Equal(got.Rake, want.Rake) ||
		!slices.Equal(got.Start, want.Start) || !slices.Equal(got.PackID, want.PackID) || !slices.Equal(got.Invested, want.Invested) ||
		!slices.Equal(got.Won, want.Won) || !slices.Equal(got.WonEV, want.WonEV) {
		t.Errorf("got %+v\nwant %+v", got, want)
	}
	if i, ok := got.Find(WPN, 1000005); !ok || i != 1 || KeySite(got.Keys[2]) != CoinPoker || KeyID(got.Keys[2]) != 7 {
		t.Errorf("Find: %d %v", i, ok)
	}
	if lo, hi := got.Players(1); lo != 3 || hi != 5 {
		t.Errorf("Players(1) = %d, %d", lo, hi)
	}
	// Every hand balances: what was won and the rake add up to what went in.
	for i := range got.Len() {
		lo, hi := got.Players(i)
		sum := got.Rake[i]
		for p := lo; p < hi; p++ {
			sum += got.Won[p] - got.Invested[p]
		}
		if sum != 0 {
			t.Errorf("hand %d is off by %d", i, sum)
		}
	}
	// The rake divided by what was won and by what was expected.
	shares := make([]Cents, 3)
	if got.RakeShares(0, false, shares); !slices.Equal(shares, []Cents{0, 5, 0}) {
		t.Errorf("rake by won %v", shares)
	}
	if got.RakeShares(0, true, shares); !slices.Equal(shares, []Cents{0, 5, 0}) {
		t.Errorf("rake by EV %v", shares) // 115 and 5 of 120: 4.79 and 0.21
	}
	if got.RakeShares(2, true, shares); !slices.Equal(shares, []Cents{1, 9, 0}) {
		t.Errorf("rake by EV %v", shares) // 50 and 560 of 610: 0.82 and 9.18
	}
	// Each covers every row once.
	var rows atomic.Int64
	got.Each(func(lo, hi int) { rows.Add(int64(hi - lo)) })
	if rows.Load() != 3 {
		t.Errorf("Each covered %d rows", rows.Load())
	}
}

func TestHandTableEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hands.hht")
	if err := WriteHandTable(path, &HandTable{Start: []uint32{0}}); err != nil {
		t.Fatal(err)
	}
	if got, err := ReadHandTable(path); err != nil || got.Len() != 0 {
		t.Errorf("%v, %v", got, err)
	}
}

func TestHandTableCorrupt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hands.hht")
	if err := WriteHandTable(path, sampleHandTable()); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(path)
	unordered := slices.Clone(b)
	copy(unordered[7:15], b[15:23]) // the first key the same as the second (the header is 7 bytes)
	for name, c := range map[string][]byte{
		"cut short":      b[:len(b)-1],
		"no end":         append(slices.Clone(b[:len(b)-4]), "XXXX"...),
		"bad magic":      append([]byte("XXXX"), b[4:]...),
		"keys not order": unordered,
	} {
		os.WriteFile(path, c, 0o644)
		if _, err := ReadHandTable(path); !errors.Is(err, ErrCorrupt) {
			t.Errorf("%s: %v, want ErrCorrupt", name, err)
		}
	}
	bad := sampleHandTable()
	bad.Won = bad.Won[:7]
	if err := WriteHandTable(path, bad); err == nil {
		t.Error("columns of different lengths written")
	}
}

func potTable(inv, won []Cents, rake Cents) *HandTable {
	return &HandTable{
		Keys: []uint64{1}, BigBlind: []Cents{100}, Rake: []Cents{rake}, Start: []uint32{0, uint32(len(inv))},
		PackID: make([]uint16, len(inv)), Invested: inv, Won: won, WonEV: won,
	}
}

func TestPots(t *testing.T) {
	// A chop: two winners of one pot with a third player who lost.
	pots, ok := potTable([]Cents{516, 516, 258, 50}, []Cents{636, 637, 0, 0}, 67).Pots(0)
	if !ok || len(pots) != 3 {
		t.Fatalf("chop: %v %v", pots, ok)
	}
	for _, p := range pots {
		if p.Winners != 0b11 {
			t.Errorf("chop: pot %+v should go to the first two", p)
		}
	}
	// Side pots: A all in for 500 wins the main pot, B the side pot from C.
	pots, ok = potTable([]Cents{500, 1000, 1000}, []Cents{1500, 1000, 0}, 0).Pots(0)
	if !ok || len(pots) != 2 || pots[0].Size != 1500 || pots[0].Winners != 0b001 || pots[1].Size != 1000 || pots[1].Winners != 0b010 {
		t.Errorf("side pots: %+v %v", pots, ok)
	}
	// One winner of everything.
	pots, ok = potTable([]Cents{300, 300, 100}, []Cents{0, 700, 0}, 0).Pots(0)
	if !ok || pots[0].Winners != 0b010 || pots[1].Winners != 0b010 {
		t.Errorf("one winner: %+v %v", pots, ok)
	}
	// Winnings that no pot can explain: the short stack won more than it could.
	if _, ok := potTable([]Cents{100, 1000}, []Cents{1100, 0}, 0).Pots(0); ok {
		t.Error("impossible winnings accepted")
	}
}
