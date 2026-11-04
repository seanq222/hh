package hh

import (
	"math"
	"testing"

	"github.com/seanq222/hh/pk"
)

func card(t *testing.T, s string) pk.Card {
	t.Helper()
	c, err := pk.ParseCard(s)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func hole(t *testing.T, a, b string) pk.HoleCards {
	return pk.HoleCards{card(t, a), card(t, b)}
}

// The general count of boards agrees with the library's heads-up count.
func TestExpectedWinningsMatchesLibrary(t *testing.T) {
	aa, kk := hole(t, "Ah", "Ad"), hole(t, "Ks", "Kc")
	wins, draws, total := pk.Equity(aa[0], aa[1], kk[0], kk[1])
	want := (float64(wins) + float64(draws)/2) / float64(total)
	if want < 0.79 || want > 0.84 {
		t.Fatalf("aces against kings should be about 82%%, got %.3f", want)
	}
	noHeadsUpShortcut = true
	defer func() { noHeadsUpShortcut = false }()
	won, ok := expectedWinnings([]Cents{100, 100}, []bool{true, true}, []int{0, 1}, []pk.HoleCards{aa, kk}, nil, false)
	if !ok {
		t.Fatal("not worked out")
	}
	if got := won[0] / 200; math.Abs(got-want) > 1e-9 {
		t.Errorf("equity %.6f, library %.6f", got, want)
	}
}

// A short stack, a medium stack and a deep stack all in: the side pots are
// shared out, and between them the players win exactly the chips that went in.
func TestExpectedWinningsSidePots(t *testing.T) {
	hands := []pk.HoleCards{hole(t, "Ah", "Ad"), hole(t, "Ks", "Kc"), hole(t, "Qs", "Qd")}
	contrib := []Cents{1000, 3000, 5000}
	board := []pk.Card{card(t, "2c"), card(t, "7h"), card(t, "9s")}
	won, ok := expectedWinnings(contrib, []bool{true, true, true}, []int{0, 1, 2}, hands, board, false)
	if !ok {
		t.Fatal("not worked out")
	}
	var sum float64
	for _, w := range won {
		sum += w
	}
	if math.Abs(sum-9000) > 1e-6 {
		t.Errorf("players win %.2f of the 9000 that went in", sum)
	}
	// The queens cannot win the first layer ahead of the aces, but the kings and
	// queens fight over the layers above it, which the aces cannot reach.
	if won[0] <= 0 || won[1] <= 0 || won[2] <= 0 {
		t.Errorf("everyone has some equity: %v", won)
	}
	// The deepest stack was only called by the others for 3000: it has 2000 of
	// its own back in an uncontested layer.
	if won[2] < 2000 {
		t.Errorf("the deep stack gets its uncontested layer back: %v", won[2])
	}
}

// Counted per pot, as Hand2Note does, the side pot is a heads-up hand between the
// two players in it, dealt from a deck that has the short stack's cards back in.
func TestExpectedWinningsPerPot(t *testing.T) {
	a, s, m := hole(t, "As", "6c"), hole(t, "9h", "2s"), hole(t, "8h", "6h")
	board := []pk.Card{card(t, "6d"), card(t, "9c"), card(t, "6s")}
	alive := []bool{true, true, true}
	contrib := []Cents{4711, 3462, 4711}
	perPot, ok := expectedWinnings(contrib, alive, []int{0, 1, 2}, []pk.HoleCards{a, s, m}, board, true)
	if !ok {
		t.Fatal("not worked out")
	}
	// The main pot shared three ways, the side pot heads up.
	three, _ := expectedWinnings([]Cents{1, 1, 1}, alive, []int{0, 1, 2}, []pk.HoleCards{a, s, m}, board, false)
	two, _ := expectedWinnings([]Cents{1, 0, 1}, alive, []int{0, 2}, []pk.HoleCards{a, m}, board, false)
	mainPot, sidePot := 3*3462.0, 2*(4711.0-3462)
	want := mainPot*three[0]/3 + sidePot*two[0]/2
	if math.Abs(perPot[0]-want) > 1e-6 {
		t.Errorf("per pot the first player wins %.4f, want %.4f", perPot[0], want)
	}
	// With every shown card out of the deck the side pot comes out otherwise.
	all, _ := expectedWinnings(contrib, alive, []int{0, 1, 2}, []pk.HoleCards{a, s, m}, board, false)
	if math.Abs(all[0]-perPot[0]) < 1 {
		t.Errorf("both ways give %.2f; the side pot should differ", all[0])
	}
}

const sampleAllInPreflop = `Hand #1000036 - Holdem (No Limit) - $0.25/$0.50 - 2026/01/01 00:00:00 UTC
Table26 6-max Seat #1 is the button
Seat 1: Alpha ($50.00)
Seat 2: Bravo ($50.00)
Seat 3: Charlie ($50.00)
Bravo posts the small blind $0.25
Charlie posts the big blind $0.50
*** HOLE CARDS ***
Alpha raises $50.00 to $50.00 and is all-in
Bravo folds
Charlie calls $49.50 and is all-in
*** FLOP *** [2c 7h 9s]
*** TURN *** [2c 7h 9s] [3d]
*** RIVER *** [2c 7h 9s 3d] [4c]
*** SHOW DOWN ***
Alpha shows [Ah Ad]
Charlie shows [Ks Kc]
*** SUMMARY ***
Total pot $98.25 | Rake $2.00
Board [2c 7h 9s 3d 4c]
Seat 1: Alpha (button) showed [Ah Ad] and won $98.25 with a pair of Aces
Seat 2: Bravo (small blind) folded on the Pre-Flop
Seat 3: Charlie (big blind) showed [Ks Kc] and lost with a pair of Kings
`

func TestExpectedNetAllIn(t *testing.T) {
	h := parseSample(t, sampleAllInPreflop)
	var evs [2]float64
	for i, name := range []string{"Alpha", "Charlie"} {
		v, err := h.PlayerHandFor(name)
		if err != nil {
			t.Fatal(err)
		}
		ev, adjusted := v.ExpectedNet()
		if !adjusted {
			t.Fatalf("%s: an all-in with cards to come should be adjusted", name)
		}
		evs[i] = ev
		if name == "Alpha" {
			if v.Net() != 4825 {
				t.Errorf("Alpha won $48.25, got %v", v.Net())
			}
			// Aces are about 81-82% to win a pot of $98.25 after rake, for $50.
			if ev < 2900 || ev > 3400 {
				t.Errorf("Alpha's expected net is %.0f cents", ev)
			}
		}
	}
	// The pot after rake is 98.25 and 100.00 went in between them, bar the 0.25
	// that Bravo put in: the two win -1.75 between them.
	if math.Abs(evs[0]+evs[1]-(-175)) > 0.01 {
		t.Errorf("together they win %.2f cents, want -175", evs[0]+evs[1])
	}
}

// Alpha is all-in on the flop and called by two players who bet on into a side
// pot on the turn, where Alpha is drawing dead.
const sampleAllInThenSidePot = `Hand #1000701 - Holdem (No Limit) - $0.25/$0.50 - 2026/01/01 00:00:00 UTC
TableOne 6-max Seat #1 is the button
Seat 1: Alpha ($20.00)
Seat 2: Bravo ($50.00)
Seat 3: Charlie ($25.00)
Bravo posts the small blind $0.25
Charlie posts the big blind $0.50
*** HOLE CARDS ***
Alpha raises $1.50 to $1.50
Bravo calls $1.25
Charlie calls $1.00
*** FLOP *** [Tc 4c Kh]
Bravo checks
Charlie checks
Alpha bets $18.50 and is all-in
Bravo calls $18.50
Charlie calls $18.50
*** TURN *** [Tc 4c Kh] [Th]
Bravo bets $5.00
Charlie calls $5.00 and is all-in
*** RIVER *** [Tc 4c Kh Th] [2h]
*** SHOW DOWN ***
Alpha shows [Qd Jd]
Bravo shows [4d 4s]
Charlie shows [Kd Qc]
*** SUMMARY ***
Total pot $70.00 | Rake $0.00
Board [Tc 4c Kh Th 2h]
Seat 1: Alpha (button) showed [Qd Jd] and lost with a pair of Tens
Seat 2: Bravo (small blind) showed [4d 4s] and won $70.00 with a full house
Seat 3: Charlie (big blind) showed [Kd Qc] and lost with two pair
`

// A player who is all-in is valued on the board of the street where their chips
// went in, not on the board where the others stopped betting.
func TestExpectedNetAllInBeforeSidePot(t *testing.T) {
	v, err := parseSample(t, sampleAllInThenSidePot).PlayerHandFor("Alpha")
	if err != nil {
		t.Fatal(err)
	}
	ev, adjusted := v.ExpectedNet()
	if !adjusted {
		t.Fatal("Alpha's all-in on the flop should be valued by equity")
	}
	// Alpha can win only the main pot, $20.00 from each of the three, on the flop.
	board := []pk.Card{card(t, "Tc"), card(t, "4c"), card(t, "Kh")}
	hands := []pk.HoleCards{hole(t, "Qd", "Jd"), hole(t, "4d", "4s"), hole(t, "Kd", "Qc")}
	eq, _ := expectedWinnings([]Cents{1, 1, 1}, []bool{true, true, true}, []int{0, 1, 2}, hands, board, false)
	want := 6000*eq[0]/3 - 2000
	if math.Abs(ev-want) > 1e-6 {
		t.Errorf("Alpha's expected net is %.2f cents, want %.2f (on the turn Alpha is drawing dead: -2000)", ev, want)
	}
}

// Hands that are not decided by the board are left at what they were.
func TestExpectedNetOtherHands(t *testing.T) {
	for name, s := range map[string]string{"showdown with betting to the river": sampleShowdown, "walk": sampleWalkClientStyle} {
		views, err := parseSample(t, s).PlayerHands()
		if err != nil {
			t.Fatal(err)
		}
		for _, v := range views {
			if ev, adjusted := v.ExpectedNet(); adjusted || ev != float64(v.Net()) {
				t.Errorf("%s: %v %v", name, ev, adjusted)
			}
		}
	}
}
