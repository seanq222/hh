package clean

import (
	"testing"

	"github.com/seanq222/hh"
)

// sampleAllIn is an invented hand in which Bob is all-in preflop for his whole
// stack of $20.00.
const sampleAllIn = `Hand #1000700 - Holdem (No Limit) - $0.25/$0.50 - 2026/01/01 12:00:00 UTC
TableOne 6-max Seat #1 is the button
Seat 1: Alice ($50.00)
Seat 2: Bob ($20.00)
Seat 3: Carol ($50.00)
Bob posts the small blind $0.25
Carol posts the big blind $0.50
*** HOLE CARDS ***
Alice folds
Bob raises $19.75 to $20.00 and is all-in
Carol calls $19.50
*** FLOP *** [2c 7d 9h]
Main pot $39.00 | Rake $1.00
*** TURN *** [2c 7d 9h] [Jd]
Main pot $39.00 | Rake $1.00
*** RIVER *** [2c 7d 9h Jd] [3s]
Main pot $39.00 | Rake $1.00
*** SHOW DOWN ***
Main pot $39.00 | Rake $1.00
Bob shows [Ah Ad] (a pair of Aces [Ah Ad Jd 9h 7d])
Carol shows [Kc Ks] (a pair of Kings [Kc Ks Jd 9h 7d])
Bob collected $39.00 from main pot
*** SUMMARY ***
Total pot $39.00 | Rake $1.00
Board [2c 7d 9h Jd 3s]
Seat 1: Alice (button) folded on the Pre-Flop
Seat 2: Bob (small blind) showed [Ah Ad] and won $39.00 with a pair of Aces [Ah Ad Jd 9h 7d]
Seat 3: Carol (big blind) showed [Kc Ks] and lost with a pair of Kings [Kc Ks Jd 9h 7d]
`

// A seat line that gives an all-in player more than they put in fails Check, and
// Fix sets the stack to what they put in.
func TestAllInForLessThanStack(t *testing.T) {
	hands := Split([]byte(sampleAllIn))
	if len(hands) != 1 {
		t.Fatalf("Split found %d hands, want 1", len(hands))
	}
	h, reason := Parse(hands[0])
	if reason != "" {
		t.Fatal(reason)
	}
	if m := Check(h); m != "" {
		t.Fatalf("the sample fails Check: %s", m)
	}
	bob := h.PlayerBySeat(2)
	bob.Stack = 2025 // the seat line missed a change since the last hand
	if m := Check(h); m != hh.AllInNotStack.String() {
		t.Fatalf("Check = %q", m)
	}
	if bob.Stack != 2025 {
		t.Fatal("Check changed the hand")
	}
	Fix(h)
	if m := Check(h); m != "" {
		t.Fatalf("after Fix, Check = %q", m)
	}
	if bob.Stack != 2000 {
		t.Errorf("after Fix Bob's stack is %d, want 2000", bob.Stack)
	}
}

// A byte-order mark before the first hand does not hide it.
func TestSplitSkipsByteOrderMark(t *testing.T) {
	if n := len(Split(append([]byte{0xEF, 0xBB, 0xBF}, sampleAllIn...))); n != 1 {
		t.Errorf("Split found %d hands, want 1", n)
	}
}
