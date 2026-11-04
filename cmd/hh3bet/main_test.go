package main

import (
	"strings"
	"testing"

	"github.com/seanq222/hh"
)

// Three-handed: Carol on the button opens, Alice in the small blind folds and
// Bob in the big blind 3-bets.
const sampleBB3Bet = `Hand #1000101 - Holdem (No Limit) - $0.25/$0.50 - 2026/01/01 00:00:00 UTC
TableOne 6-max Seat #3 is the button
Seat 1: Alice ($50.00)
Seat 2: Bob ($50.00)
Seat 3: Carol ($50.00)
Alice posts the small blind $0.25
Bob posts the big blind $0.50
*** HOLE CARDS ***
Carol raises $1.25 to $1.25
Alice folds
Bob raises $3.75 to $4.25
Carol folds
Uncalled bet ($3.00) returned to Bob
*** SUMMARY ***
Total pot $2.75
Seat 1: Alice (small blind) folded on the Pre-Flop
Seat 2: Bob (big blind) did not show and won $2.75
Seat 3: Carol (button) folded on the Pre-Flop
`

// Heads-up: Carol on the button posts the small blind and opens.
const sampleHeadsUp = `Hand #1000102 - Holdem (No Limit) - $0.25/$0.50 - 2026/01/01 00:00:00 UTC
TableOne 6-max Seat #3 is the button
Seat 2: Bob ($50.00)
Seat 3: Carol ($50.00)
Carol posts the small blind $0.25
Bob posts the big blind $0.50
*** HOLE CARDS ***
Carol raises $1.00 to $1.25
Bob raises $3.75 to $4.25
Carol folds
Uncalled bet ($3.00) returned to Bob
*** SUMMARY ***
Total pot $2.50
Seat 2: Bob (big blind) did not show and won $2.50
Seat 3: Carol (button) folded on the Pre-Flop
`

// Four-handed: Dave in the cutoff opens, Carol on the button folds, Alice in the
// small blind calls and Bob in the big blind folds.
const sampleSBCall = `Hand #1000103 - Holdem (No Limit) - $0.25/$0.50 - 2026/01/01 00:00:00 UTC
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

func TestFacedOpen(t *testing.T) {
	bbRaise := "Bob raises $3.75 to $4.25\nCarol folds\nUncalled bet ($3.00) returned to Bob\n"
	for _, c := range []struct {
		name, text, hero string
		pos, opener      hh.Position
		faced            bool
		kind             hh.ActionKind
	}{
		{"big blind 3-bets", sampleBB3Bet, "Bob", hh.BB, hh.BTN, true, hh.Raise},
		{"big blind calls", strings.Replace(sampleBB3Bet, bbRaise, "Bob calls $0.75\n", 1), "Bob", hh.BB, hh.BTN, true, hh.Call},
		{"big blind folds", strings.Replace(sampleBB3Bet, bbRaise, "Bob folds\n", 1), "Bob", hh.BB, hh.BTN, true, hh.Fold},
		{"small blind calls first", strings.Replace(sampleBB3Bet, "Alice folds\n", "Alice calls $1.00\n", 1), "Bob", hh.BB, hh.BTN, false, 0},
		{"button limps", strings.Replace(sampleBB3Bet, "Carol raises $1.25 to $1.25\n", "Carol calls $0.50\n", 1), "Bob", hh.BB, hh.BTN, false, 0},
		{"heads-up", sampleHeadsUp, "Bob", hh.BB, hh.BTN, false, 0},
		{"any: small blind calls a cutoff open", sampleSBCall, "Alice", hh.SB, anyOpener, true, hh.Call},
		{"any: the button cold-calls first", strings.Replace(sampleSBCall, "Carol folds\n", "Carol calls $1.25\n", 1), "Alice", hh.SB, anyOpener, false, 0},
		{"any: the cutoff limps", strings.Replace(sampleSBCall,
			"Dave raises $1.25 to $1.25\nCarol folds\nAlice calls $1.00\nBob folds\n*** FLOP *** [Ah 8h 6s]\nAlice checks\n",
			"Dave calls $0.50\nCarol folds\nAlice calls $0.25\nBob checks\n*** FLOP *** [Ah 8h 6s]\nAlice checks\nBob checks\n", 1),
			"Alice", hh.SB, anyOpener, false, 0},
		{"button: the cutoff's open is not the button's", sampleSBCall, "Alice", hh.SB, hh.BTN, false, 0},
		{"cutoff: the small blind calls the cutoff's open", sampleSBCall, "Alice", hh.SB, hh.CO, true, hh.Call},
	} {
		h, err := hh.ParseHandPartial(strings.Split(strings.TrimSpace(c.text), "\n"))
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		v, err := h.PlayerHandFor(c.hero)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if faced, kind := facedOpen(&v, c.pos, c.opener); faced != c.faced || (faced && kind != c.kind) {
			t.Errorf("%s: faced %v kind %v, want %v %v", c.name, faced, kind, c.faced, c.kind)
		}
	}
}
