package hh

import (
	"strings"
	"testing"
	"time"
)

// Dave straddles and Alice raises by ₮4 to ₮6, putting in ₮6. The time is US
// Eastern, standard time in January. The summary has a line for Erin, who is not
// at the table.
const sampleCoinPokerStraddle = `CoinPoker Hand #900001: Hold'em No Limit (₮0.50/₮1 - Ante ₮0.10 CPCC) - 2026/01/15 20:00:00 ET
Table 'TableOne' 6-max Seat #1 is the button
Seat 1: Alice (₮100 in chips)
Seat 2: Bob (₮100 in chips)
Seat 3: Carol (₮100 in chips)
Seat 4: Dave (₮100 in chips)
Alice: posts the ante ₮0.10
Bob: posts the ante ₮0.10
Carol: posts the ante ₮0.10
Dave: posts the ante ₮0.10
Bob: posts small blind ₮0.50
Carol: posts big blind ₮1
Dave: posts straddle ₮2
*** HOLE CARDS ***
Alice: raises ₮4 to ₮6
Bob: folds
Carol: folds
Dave: calls ₮4
*** FLOP *** [2c 7h 9s]
Dave: checks
Alice: bets ₮5
Dave: folds
Uncalled bet (₮5) returned to Alice
Alice collected ₮13.90 from pot
Alice: doesn't show hand
*** SUMMARY ***
Total pot ₮13.90 | Rake ₮0
Board [2c 7h 9s]
Seat 1: Alice (button) collected (₮13.90)
Seat 2: Bob (small blind) folded before Flop
Seat 3: Carol (big blind) folded before Flop
Seat 4: Dave folded on the Flop
Seat 5: Erin folded before Flop (didn't bet)
`

// Alice joins the game with a big blind out of turn, a second "posts big blind",
// and is all-in against Erin. They run it twice: Alice wins the first board and
// Erin the second. The summary gives only each player's total.
const sampleCoinPokerRunTwice = `CoinPoker Hand #900002: Hold'em No Limit (₮0.50/₮1 - Ante ₮0.10 CPCC) - 2026/01/16 01:05:00 +00
Table 'TableOne' 6-max Seat #2 is the button
Seat 1: Alice (₮50 in chips)
Seat 2: Bob (₮100 in chips)
Seat 3: Carol (₮100 in chips)
Seat 5: Erin (₮100 in chips)
Alice: posts the ante ₮0.10
Bob: posts the ante ₮0.10
Carol: posts the ante ₮0.10
Erin: posts the ante ₮0.10
Carol: posts small blind ₮0.50
Erin: posts big blind ₮1
Alice: posts big blind ₮1
*** HOLE CARDS ***
Alice: raises ₮48.90 to ₮49.90 and is all-in
Bob: folds
Carol: folds
Erin: calls ₮48.90
*** FIRST FLOP *** [2c 7h 9s]
*** FIRST TURN *** [2c 7h 9s] [Jd]
*** FIRST RIVER *** [2c 7h 9s Jd] [3s]
*** SECOND FLOP *** [Kd 4c 5h]
*** SECOND TURN *** [Kd 4c 5h] [6s]
*** SECOND RIVER *** [Kd 4c 5h 6s] [Qc]
*** FIRST SHOW DOWN ***
Alice: shows [Ah Ad]
Erin: shows [Ks Kc]
Alice collected ₮49 from pot
*** SECOND SHOW DOWN ***
Alice: shows [Ah Ad]
Erin: shows [Ks Kc]
Erin collected ₮49 from pot
*** SUMMARY ***
Total pot ₮100.70 | Rake ₮2.70
Hand was run twice
FIRST Board [2c 7h 9s Jd 3s]
SECOND Board [Kd 4c 5h 6s Qc]
Seat 1: Alice showed [Ah Ad] and won (₮49)
Seat 2: Bob (button) folded before Flop (didn't bet)
Seat 3: Carol (small blind) folded before Flop
Seat 5: Erin (big blind) showed [Ks Kc] and won (₮49)
`

// Alice is all-in for less after Bob and Carol have matched each other, and the
// file writes it as a raise by ₮0.00. It does not reopen the betting: the round
// closes, and Bob and Carol play on.
const sampleCoinPokerShortRaise = `CoinPoker Hand #900003: Hold'em No Limit (₮0.50/₮1 - Ante ₮0.10 CPCC) - 2026/01/16 02:00:00 +00
Table 'TableOne' 6-max Seat #3 is the button
Seat 1: Alice (₮20 in chips)
Seat 2: Bob (₮200 in chips)
Seat 3: Carol (₮200 in chips)
Alice: posts the ante ₮0.10
Bob: posts the ante ₮0.10
Carol: posts the ante ₮0.10
Alice: posts small blind ₮0.50
Bob: posts big blind ₮1
*** HOLE CARDS ***
Carol: raises ₮2 to ₮3
Alice: calls ₮2.50
Bob: raises ₮96.90 to ₮99.90
Carol: calls ₮96.90
Alice: raises ₮0.00 to ₮19.90 and is all-in
*** FLOP *** [2c 7h 9s]
Bob: checks
Carol: checks
*** TURN *** [2c 7h 9s] [Jd]
Bob: checks
Carol: checks
*** RIVER *** [2c 7h 9s Jd] [3s]
Bob: checks
Carol: checks
*** SHOW DOWN ***
Alice: shows [Kh Kd]
Bob: shows [Qs Qc]
Carol: shows [Ah Ad]
Carol collected ₮218 from pot
*** SUMMARY ***
Total pot ₮220 | Rake ₮2
Board [2c 7h 9s Jd 3s]
Seat 1: Alice (small blind) showed [Kh Kd] and lost
Seat 2: Bob (big blind) showed [Qs Qc] and lost
Seat 3: Carol (button) showed [Ah Ad] and won (₮218)
`

func TestCoinPokerRaiseByNothing(t *testing.T) {
	h := parseSample(t, sampleCoinPokerShortRaise)
	if m := h.FailedCheck(); m != "" {
		t.Fatalf("fails: %s", m)
	}
	a := h.Actions[9]
	if a.Player != "Alice" || a.Kind != Raise || a.Amount != 1690 || a.To != 1990 || !a.AllIn {
		t.Errorf("Alice's all-in for less is %+v, want a raise adding ₮16.90 to ₮19.90", a)
	}
	// The player views keep it simple: there it is a call.
	views, err := h.PlayerHands()
	if err != nil {
		t.Fatal(err)
	}
	if e := views[0].Events[9]; e.Kind != Call || e.Amount != 1690 {
		t.Errorf("in the views Alice's all-in for less is %+v, want a call of ₮16.90", e)
	}
	text := h.String()
	if !strings.Contains(text, "Alice: raises ₮0.00 to ₮19.90 and is all-in") {
		t.Errorf("written as:\n%s", text)
	}
	if d := h.Diff(parseSample(t, text)); d != "" {
		t.Errorf("reads back differently, %s", d)
	}
}

// A CoinPoker hand with an ante above the table's standard fails the check: the
// file shows the largest ante paid as everyone's.
func TestCoinPokerRaisedAnte(t *testing.T) {
	if h := parseSample(t, sampleCoinPokerRunTwice); !h.StandardAnte() {
		t.Error("an ante of 0.10 big blinds is standard")
	}
	raised := strings.ReplaceAll(sampleCoinPokerRunTwice, "ante ₮0.10", "ante ₮0.30")
	raised = strings.ReplaceAll(raised, "Ante ₮0.10", "Ante ₮0.30")
	h := parseSample(t, raised)
	if h.StandardAnte() {
		t.Error("an ante of 0.30 big blinds is not standard")
	}
	if m := h.FailedCheck(); !strings.HasPrefix(m, "the ante is above the table's") {
		t.Errorf("fails with %q", m)
	}
}

func TestCoinPokerStraddle(t *testing.T) {
	h := parseSample(t, sampleCoinPokerStraddle)
	if h.Site != CoinPoker {
		t.Fatalf("site %v", h.Site)
	}
	if want := time.Date(2026, 1, 16, 1, 0, 0, 0, time.UTC); !h.Time.Equal(want) {
		t.Errorf("time %v, want %v", h.Time, want)
	}
	if h.Type != Ante || h.Game != "Hold'em" || h.Limit != "No Limit" || h.BigBlind != 100 {
		t.Errorf("header: type %v game %q limit %q big blind %v", h.Type, h.Game, h.Limit, h.BigBlind)
	}
	var straddle, raise *Action
	for i := range h.Actions {
		switch h.Actions[i].Kind {
		case PostStraddle:
			straddle = &h.Actions[i]
		case Raise:
			raise = &h.Actions[i]
		}
	}
	if straddle == nil || straddle.Player != "Dave" || straddle.Amount != 200 {
		t.Errorf("straddle %+v", straddle)
	}
	if raise == nil || raise.Amount != 600 || raise.To != 600 {
		t.Errorf("raise %+v, want ₮6 added to ₮6", raise)
	}
	if h.SummaryNamesMatch() {
		t.Error("the summary line for Erin, who is not seated, should be stale")
	}
	h.Repair()
	if m := h.FailedCheck(); m != "" {
		t.Fatalf("fails after repair: %s", m)
	}
	text := h.String()
	if !strings.HasPrefix(text, "CoinPoker Hand #900001: ") || !strings.Contains(text, "Alice: raises ₮4.00 to ₮6.00") {
		t.Errorf("written as:\n%s", text)
	}
	if d := h.Diff(parseSample(t, text)); d != "" {
		t.Errorf("reads back differently, %s", d)
	}
}

func TestCoinPokerRunTwice(t *testing.T) {
	h := parseSample(t, sampleCoinPokerRunTwice)
	if m := h.FailedCheck(); m != "" {
		t.Fatalf("fails: %s", m)
	}
	if !h.RunTwice || len(h.Board) != 5 || len(h.Board2) != 5 {
		t.Fatalf("run twice %v, boards %v and %v", h.RunTwice, h.Board, h.Board2)
	}
	if k := h.Actions[6].Kind; k != PostEntry {
		t.Errorf("Alice's big blind out of turn is %v, want an entry post", k)
	}
	alice, erin := h.PlayerByName("Alice"), h.PlayerByName("Erin")
	if alice.Won != 4900 || alice.WonSecond != 0 || erin.Won != 4900 || erin.WonSecond != 4900 {
		t.Errorf("Alice won %v (%v on the second board), Erin %v (%v)", alice.Won, alice.WonSecond, erin.Won, erin.WonSecond)
	}
	if d := h.Diff(parseSample(t, h.String())); d != "" {
		t.Errorf("reads back differently, %s\n%s", d, h.String())
	}
}
