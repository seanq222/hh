package main

import (
	"strings"
	"testing"

	"github.com/seanq222/hh"
)

// Four-handed: Dave in the cutoff opens, Carol on the button folds, Alice in
// the small blind calls and Bob in the big blind folds. On the flop Alice
// checks, Dave c-bets and Alice folds.
const sampleCbet = `Hand #1000201 - Holdem (No Limit) - $0.25/$0.50 - 2026/01/01 00:00:00 UTC
TableOne 6-max Seat #4 is the button
Seat 1: Alice ($50.00)
Seat 2: Bob ($50.00)
Seat 3: Dave ($50.00)
Seat 4: Carol ($50.00)
Alice posts the small blind $0.25
Bob posts the big blind $0.50
*** HOLE CARDS ***
Dave raises $1.25 to $1.25
Carol folds
Alice calls $1.00
Bob folds
*** FLOP *** [Ah 8h 6s]
Alice checks
Dave bets $2.00
Alice folds
Uncalled bet ($2.00) returned to Dave
*** SUMMARY ***
Total pot $3.00
Board: [Ah 8h 6s]
Seat 1: Alice (small blind) folded on the Flop
Seat 2: Bob (big blind) folded on the Pre-Flop
Seat 3: Dave did not show and won $3.00
Seat 4: Carol (button) folded on the Pre-Flop
`

func TestTally(t *testing.T) {
	h, err := hh.ParseHandPartial(strings.Split(strings.TrimSpace(sampleCbet), "\n"))
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		hero string
		want map[string][2]int64 // [done, chances]; every other stat has no chances
	}{
		{"Dave", map[string][2]int64{
			"VPIP": {1, 1}, "PFR": {1, 1}, "open": {1, 1}, "open CO": {1, 1}, "limp": {0, 1},
			// Last to act on the flop, so checking would have been a missed c-bet in position.
			"cbet flop": {1, 1}, "missed cbet IP rate": {0, 1}, "WTSD": {0, 1}, "WWSF": {1, 1},
		}},
		{"Alice", map[string][2]int64{
			"VPIP": {1, 1}, "PFR": {0, 1}, "call open": {1, 1}, "call open SB": {1, 1},
			"3-bet": {0, 1}, "3-bet SB": {0, 1}, "3-bet OOP": {0, 1}, "3-bet OOP SB": {0, 1},
			// A 2bb call, so it counts for the fold to the c-bet.
			"fold to cbet flop": {1, 1}, "vs cbet raised pot fold": {1, 1},
			"vs cbet raised pot call": {0, 1}, "vs cbet raised pot raise": {0, 1},
			// First to act after a 2bb call, so Alice could have led.
			"donk flop": {0, 1}, "WTSD": {0, 1}, "WWSF": {0, 1},
		}},
		{"Bob", map[string][2]int64{
			// Alice called the open first, so Bob could squeeze, not cold-call.
			"VPIP": {0, 1}, "PFR": {0, 1}, "3-bet": {0, 1}, "squeeze": {0, 1},
		}},
	} {
		v, err := h.PlayerHandFor(c.hero)
		if err != nil {
			t.Fatal(err)
		}
		st := newStats()
		tally(&v, st)
		got := st.counts(c.hero).Stats
		for name, w := range c.want {
			if got[name] != w {
				t.Errorf("%s: %s = %v, want %v", c.hero, name, got[name], w)
			}
		}
		for name, g := range got {
			if _, ok := c.want[name]; !ok {
				t.Errorf("%s: %s = %v, want no chances", c.hero, name, g)
			}
		}
	}
}

// Every stat with a normal range must exist, so a renamed stat can't silently
// lose its colour.
func TestNormalNames(t *testing.T) {
	known := map[string]bool{"AF0": true, "AF1": true, "AF2": true, "AFQ0": true, "AFQ1": true, "AFQ2": true, "MISSED": true}
	for _, name := range statNames {
		known[name] = true
	}
	for name, r := range normal {
		if !known[name] {
			t.Errorf("normal range for %q, which is not a stat", name)
		}
		if r[0] > r[1] {
			t.Errorf("%s: range %v runs backwards", name, r)
		}
	}
}

func TestFileName(t *testing.T) {
	for in, want := range map[string]string{"Alice": "Alice", "a/b": "a_b", "_x": "-_x", ".x": "-.x", "Bo Bo": "Bo Bo"} {
		if got := fileName(in); got != want {
			t.Errorf("fileName(%q) = %q, want %q", in, got, want)
		}
	}
}
