package hh

import (
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"
	"unsafe"
)

// A real bomb pot: the antes are followed straight by the flop.
const sampleRealBombPot = `Hand #1000015 - Holdem(No Limit) - $1.00/$2.00 - 2026/01/01 00:00:00 UTC
Table09 6-max Seat #1 is the button
Seat 1: Player064 ($222.28)
Seat 2: Player026 ($363.27)
Seat 3: Player073 ($218.15)
Seat 4: Player015 ($412.73)
Player064 posts ante $10.00
Player026 posts ante $10.00
Player073 posts ante $10.00
Player015 posts ante $10.00
*** HOLE CARDS ***
*** FLOP *** [9s 8c 6h]
Main pot $37.00 | Rake $3.00
Player026 checks
Player073 checks
Player015 checks
Player064 checks
*** TURN *** [9s 8c 6h] [Jh]
Main pot $37.00 | Rake $3.00
Player026 bets $10.00
Player073 folds
Player015 folds
Player064 calls $10.00
*** RIVER *** [9s 8c 6h Jh] [5h]
Main pot $57.00 | Rake $3.00
Player026 checks
Player064 checks
*** SHOW DOWN ***
Main pot $57.00 | Rake $3.00
Player026 shows [Ah Kh]
Player064 shows [2c 2d]
*** SUMMARY ***
BombPot
Total pot $57.00 | Rake $3.00
Board: [9s 8c 6h Jh 5h]
Seat 1: Player064 (button) showed [2c 2d] and lost with a pair of Twos
Seat 2: Player026 showed [Ah Kh] and won $57.00 with a flush
Seat 3: Player073 folded on the Turn
Seat 4: Player015 folded on the Turn
`

func parseSample(t *testing.T, s string) *HandHistory {
	t.Helper()
	h, err := ParseHand(strings.Split(strings.TrimSpace(s), "\n"))
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func TestPlayerHandSizes(t *testing.T) {
	t.Logf("PlayerHand %d bytes, Player %d, Event %d",
		unsafe.Sizeof(PlayerHand{}), unsafe.Sizeof(Player{}), unsafe.Sizeof(Event{}))
	if s := unsafe.Sizeof(Event{}); s != 8 {
		t.Errorf("Event is %d bytes, want 8", s)
	}
	if s := unsafe.Sizeof(Player{}); s != 12 {
		t.Errorf("Player is %d bytes, want 12", s)
	}
}

func TestPositions(t *testing.T) {
	h := parseSample(t, sampleShowdown)
	views, err := h.PlayerHands()
	if err != nil {
		t.Fatal(err)
	}
	if len(views) != 6 {
		t.Fatalf("views: %d", len(views))
	}
	want := []Position{UTG, HJ, CO, BTN, SB, BB}
	for i, p := range views[0].Players {
		if p.Position != want[i] {
			t.Errorf("slot %d: %v, want %v", i, p.Position, want[i])
		}
	}
	for i := range views {
		if int(views[i].Hero) != i {
			t.Errorf("view %d has hero %d", i, views[i].Hero)
		}
	}
	if got := views[0].Players[0].Stack; got != 10003 {
		t.Errorf("UTG stack %d", got)
	}
}

func TestHeroFoldsTruncates(t *testing.T) {
	h := parseSample(t, sampleShowdown)
	views, _ := h.PlayerHands()
	hj := views[1] // Player072 folds preflop after the UTG raise
	if hj.Showdown() || hj.Won != 0 || hj.Rake != 0 || hj.Reached != Preflop {
		t.Errorf("folded view: %+v", hj)
	}
	last := hj.Events[len(hj.Events)-1]
	if last.Kind != Fold || last.Actor != hj.Hero {
		t.Errorf("last event %+v", last)
	}
	if len(hj.Events) != 4 { // SB, BB, UTG raise, own fold
		t.Errorf("events: %d", len(hj.Events))
	}
	for _, p := range hj.Players {
		if p.HasCards {
			t.Error("a folded hero cannot see cards")
		}
	}
}

func TestShowdownView(t *testing.T) {
	h := parseSample(t, sampleShowdown)
	views, _ := h.PlayerHands()
	sb := views[4] // Player057
	if !sb.Showdown() || sb.Won != 3943 || sb.Rake != 207 || sb.Reached != River {
		t.Errorf("view: %+v", sb)
	}
	// Player057 (slot 4) showed; Player011 (slot 3) did not.
	if !sb.Players[4].HasCards || sb.Players[3].HasCards {
		t.Errorf("cards: %+v", sb.Players)
	}
	// SB posts 0.50 and calls 6.50 preflop and 12.25 on the flop.
	if got := sb.Invested(); got != 50+650+1225 {
		t.Errorf("invested %d", got)
	}
	if got := sb.Net(); got != 3943-(50+650+1225) {
		t.Errorf("net %d", got)
	}
}

func TestSteps(t *testing.T) {
	h := parseSample(t, sampleShowdown)
	views, _ := h.PlayerHands()
	var steps []Step
	for s := range views[4].Steps() {
		steps = append(steps, s)
	}
	if len(steps) != len(views[4].Events) {
		t.Fatalf("steps %d events %d", len(steps), len(views[4].Events))
	}
	// Event 2 is the UTG raise to 2.00: pot 1.50, 2.00 to call.
	if s := steps[2]; s.Pot != 150 || s.ToCall != 100 || s.StreetTotal != 200 || s.Street != Preflop {
		t.Errorf("raise step: %+v", s)
	}
	// The SB's call completes the 7.00 raise: 6.50 added to the 0.50 posted.
	var call Step
	for _, s := range steps {
		if s.Event.Kind == Call && s.Street == Preflop && s.Event.Actor == 4 {
			call = s
		}
	}
	if call.ToCall != 650 || call.StreetTotal != 700 {
		t.Errorf("call step: %+v", call)
	}
	// The flop starts with a pot of 16.15 + 0.85 rake = 17.00.
	for _, s := range steps {
		if s.Street == Flop {
			if s.Pot != 1700 {
				t.Errorf("pot at flop: %d", s.Pot)
			}
			break
		}
	}
}

// Every inferred street must agree with the street the parser saw.
func TestInferredStreets(t *testing.T) {
	for name, s := range map[string]string{
		"showdown":  sampleShowdown,
		"uncalled":  sampleUncalled,
		"posts":     samplePostsAndPartial,
		"bombpot":   sampleRealBombPot,
		"sittingot": sampleSittingOut,
	} {
		h := parseSample(t, s)
		views, err := h.PlayerHands()
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		for vi := range views {
			i := 0
			for st := range views[vi].Steps() {
				if st.Street != h.Actions[i].Street {
					t.Errorf("%s view %d event %d: inferred %v, real %v", name, vi, i, st.Street, h.Actions[i].Street)
				}
				i++
			}
			if i != len(views[vi].Events) {
				t.Errorf("%s view %d: %d steps", name, vi, i)
			}
		}
	}
}

func TestBombPotViews(t *testing.T) {
	h := parseSample(t, sampleRealBombPot)
	views, err := h.PlayerHands()
	if err != nil {
		t.Fatal(err)
	}
	if len(views) != 4 {
		t.Fatalf("views %d", len(views))
	}
	// A hero who folds on the turn saw the flop and turn but not the river.
	for _, v := range views {
		last := v.Events[len(v.Events)-1]
		if last.Kind == Fold && v.Reached != Turn {
			t.Errorf("folded on the turn but board is %v", v.BoardCards())
		}
	}
}

func TestNotPlayable(t *testing.T) {
	h := &HandHistory{Players: []Seat{{Seat: 1}, {Seat: 2}}}
	if _, err := h.PlayerHands(); err != ErrNotPlayable {
		t.Errorf("err = %v", err)
	}
}

// allSamples is every sample hand in the tests.
var allSamples = map[string]string{
	"CoinPokerStraddle": sampleCoinPokerStraddle, "CoinPokerRunTwice": sampleCoinPokerRunTwice,
	"CoinPokerShortRaise": sampleCoinPokerShortRaise, "AllInPreflop": sampleAllInPreflop,
	"AllInThenSidePot": sampleAllInThenSidePot, "MissingCall": sampleMissingCall,
	"MissingBlindPost": sampleMissingBlindPost, "ActionAfterClose": sampleActionAfterClose,
	"StaleTotalPot": sampleStaleTotalPot, "RealBombPot": sampleRealBombPot,
	"Showdown": sampleShowdown, "Uncalled": sampleUncalled, "AllInBomb": sampleAllInBomb,
	"Hero": sampleHero, "NoSummaryHeader": sampleNoSummaryHeader, "RunTwice": sampleRunTwice,
	"RunTwiceOldStyle": sampleRunTwiceOldStyle, "BombPotLabelNoAntes": sampleBombPotLabelNoAntes,
	"WalkClientStyle": sampleWalkClientStyle, "WalkOtherStyle": sampleWalkOtherStyle,
	"WalkOtherNoLine": sampleWalkOtherNoLine, "ButtonPostsBigBlind": sampleButtonPostsBigBlind,
	"SitOutMovesBlind": sampleSitOutMovesBlind, "ActsTwice": sampleActsTwice,
	"ActsAfterClose": sampleActsAfterClose, "SittingOutActs": sampleSittingOutActs,
	"WrongCall": sampleWrongCall, "BlindPostedTwice": sampleBlindPostedTwice,
	"SkippedSeat": sampleSkippedSeat, "AnteAllIn": sampleAnteAllIn,
	"WinnerNeverActs": sampleWinnerNeverActs, "FirstToActMissing": sampleFirstToActMissing,
	"SilentFold": sampleSilentFold, "MissingRaise": sampleMissingRaise, "MissingBet": sampleMissingBet,
	"MissingSmallBlind": sampleMissingSmallBlind, "MissingReturn": sampleMissingReturn,
	"CutOff": sampleCutOff, "AnteMissingBlind": sampleAnteMissingBlind,
	"StaleSummaryName": sampleStaleSummaryName, "EmptyDeadPost": sampleEmptyDeadPost,
	"ChecksWithEmptyStack": sampleChecksWithEmptyStack, "NameAtTwoSeats": sampleNameAtTwoSeats,
	"LoneCheck": sampleLoneCheck, "SittingOut": sampleSittingOut,
	"PostsAndPartial": samplePostsAndPartial, "MarkerNoShowdown": sampleMarkerNoShowdown,
	"Kicker": sampleKicker,
}

// parseAny parses a sample that may be defective; ok is false if it does not
// parse at all.
func parseAny(s string) (*HandHistory, bool) {
	h, err := ParseHandPartial(strings.Split(strings.TrimSpace(s), "\n"))
	return h, err == nil && h != nil
}

// Building the views of chosen players gives exactly the views those players
// get when every view is built.
func TestPlayerHandsForChosenSeats(t *testing.T) {
	for name, text := range allSamples {
		h, ok := parseAny(text)
		if !ok {
			continue
		}
		all, names, err := h.NamedPlayerHands()
		if err != nil {
			continue
		}
		for i, who := range names {
			one, oneNames, err := h.NamedPlayerHandsFor(func(n string) bool { return n == who })
			if err != nil || len(one) != 1 || oneNames[0] != who {
				t.Errorf("%s %s: got %d views, %v", name, who, len(one), err)
				continue
			}
			if !reflect.DeepEqual(one[0], all[i]) {
				t.Errorf("%s %s: the chosen view differs from the full build", name, who)
			}
			if v, err := h.PlayerHandFor(who); err != nil || !reflect.DeepEqual(v, all[i]) {
				t.Errorf("%s %s: PlayerHandFor differs from the full build (%v)", name, who, err)
			}
		}
		if none, _, err := h.NamedPlayerHandsFor(func(string) bool { return false }); err != nil || len(none) != 0 {
			t.Errorf("%s: want no views, got %d (%v)", name, len(none), err)
		}
	}
}

// StreetsReplayable gives the answer PlayerHands' own street check gives, on
// every sample.
func TestStreetsReplayableOnAllSamples(t *testing.T) {
	for name, text := range allSamples {
		h, ok := parseAny(text)
		if !ok {
			continue
		}
		_, err := h.PlayerHands()
		if old := !errors.Is(err, ErrInconsistent); old != h.StreetsReplayable() {
			t.Errorf("%s: PlayerHands says replayable %v, StreetsReplayable %v", name, old, !old)
		}
	}
}

func TestRakeShares(t *testing.T) {
	for _, c := range []struct {
		won  []Cents
		rake Cents
		want []Cents
	}{
		{[]Cents{0, 600, 0, 600, 600}, 200, []Cents{0, 67, 0, 67, 66}}, // three equal winners: the earlier get the odd cents
		{[]Cents{0, 1000, 0}, 50, []Cents{0, 50, 0}},                   // one winner pays it all
		{[]Cents{300, 700}, 100, []Cents{30, 70}},                      // in proportion
		{[]Cents{333, 667}, 10, []Cents{3, 7}},                         // 3.33 and 6.67
		{[]Cents{0, 0}, 25, []Cents{0, 0}},                             // nobody won
		{[]Cents{500, 500}, 0, []Cents{0, 0}},                          // no rake
		{[]Cents{1, 1, 1, 1, 1, 1, 1, 1, 1}, 4, []Cents{1, 1, 1, 1, 0, 0, 0, 0, 0}},
	} {
		got := make([]Cents, len(c.won))
		RakeShares(c.won, c.rake, got)
		if !slices.Equal(got, c.want) {
			t.Errorf("RakeShares(%v, %d) = %v, want %v", c.won, c.rake, got, c.want)
		}
	}
}
