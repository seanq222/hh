package hh

import (
	"strings"
	"testing"
)

// A hand run twice after the turn: both runs share the flop and turn.
const sampleRunTwice = `Hand #1000006 - Holdem (No Limit) - $0.10/$0.25 - 2026/01/01 00:00:00 UTC
Table05 6-max Seat #1 is the button
Seat 1: Player027 ($41.70)
Seat 2: Player022 ($36.17)
Seat 3: Player079 ($49.83)
Seat 4: Player044 ($16.08)
Seat 5: Player070 ($48.16)
Seat 6: seanq222 ($28.62)
Player022 posts the small blind $0.10
Player079 posts the big blind $0.25
*** HOLE CARDS ***
Dealt to seanq222 [5h Qh]
Player044 raises $0.50 to $0.50
Player070 raises $2.05 to $2.05
seanq222 folds
Player027 folds
Player022 folds
Player079 folds
Player044 calls $1.55
*** FLOP *** [4d Qd 9s]
Main pot $4.25 | Rake $0.20
Player044 checks
Player070 bets $2.78
Player044 calls $2.78
*** TURN *** [4d Qd 9s] [Jd]
Main pot $9.56 | Rake $0.45
Player044 checks
Player070 bets $43.33 and is all-in
Player044 calls $11.25 and is all-in
Uncalled bet ($32.08) returned to Player070
*** RUN IT TWICE ***
*** RIVER 1 *** [4d Qd 9s Jd] [Ks]
*** SHOW DOWN 1 ***
Player044 shows [4h 4s] (three of a kind, Set of Fours [4s 4h 4d Qd Ks])
Player070 shows [Ac As] (a pair of Aces [Ac As Ks Qd Jd])
*** RIVER 2 *** [4d Qd 9s Jd] [9c]
*** SHOW DOWN 2 ***
Player044 shows [4h 4s] (a full house, Fours full of Nines [4s 4h 4d 9s 9c])
Player070 shows [Ac As] (two pair, Aces and Nines [Ac As 9s 9c Qd])
*** SUMMARY ***
Total pot $30.89 | Rake $1.38 | JP Fee $0.24
Board 1 [4d Qd 9s Jd Ks]
Seat 1: Player027 (button) folded on the Pre-Flop
Seat 2: Player022 (small blind) folded on the Pre-Flop
Seat 3: Player079 (big blind) folded on the Pre-Flop
Seat 4: Player044 showed [4h 4s] and won $15.45 with three of a kind, Set of Fours [4s 4h 4d Qd Ks]
Seat 5: Player070 showed [Ac As] and lost with a pair of Aces [Ac As Ks Qd Jd]
Seat 6: seanq222 folded on the Pre-Flop
Board 2 [4d Qd 9s Jd 9c]
Seat 1: Player027 (button) folded on the Pre-Flop
Seat 2: Player022 (small blind) folded on the Pre-Flop
Seat 3: Player079 (big blind) folded on the Pre-Flop
Seat 4: Player044 showed [4h 4s] and won $15.44 with a full house, Fours full of Nines [4s 4h 4d 9s 9c]
Seat 5: Player070 showed [Ac As] and lost with two pair, Aces and Nines [Ac As 9s 9c Qd]
Seat 6: seanq222 folded on the Pre-Flop
`

// An older file's hand run twice on the river: no "*** RUN IT TWICE ***" line,
// only the numbered river and showdown of each run.
const sampleRunTwiceOldStyle = `Hand #1000703 - Holdem(No Limit) - $0.50/$1.00 - 2024/01/01 00:00:00 UTC
TableOne 6-max Seat #4 is the button
Seat 2: Alice ($40.00)
Seat 4: Bob ($185.00)
Seat 6: Carol ($100.00)
Carol posts the small blind $0.50
Alice posts the big blind $1.00
*** HOLE CARDS ***
Bob raises $2.00 to $2.00
Carol folds
Alice calls $1.00
*** FLOP *** [9s Kh 6h]
Main pot $4.30 | Rake $0.20
Alice checks
Bob checks
*** TURN *** [9s Kh 6h] [Ts]
Main pot $4.30 | Rake $0.20
Alice bets $2.64
Bob raises $9.60 to $9.60
Alice raises $35.36 to $38.00 and is all-in
Bob calls $28.40
*** RIVER 1 *** [9s Kh 6h Ts] [Qh]
Main pot 1 $39.75 | Rake $0.50
*** SHOW DOWN 1 ***
Main pot 1 $39.75 | Rake $0.50
Alice shows [Js 7s]
Bob shows [9h Td]
Alice collected $39.75 from main pot 1
*** RIVER 2 *** [9s Kh 6h Ts] [3c]
Main pot 2 $39.75 | Rake $0.50
*** SHOW DOWN 2 ***
Main pot $39.75 | Rake $0.50
Alice shows [Js 7s]
Bob shows [9h Td]
Bob collected $39.75 from main pot 2
*** SUMMARY ***
Total pot $79.50 | Rake $1.00 | JP Fee $0.00
Board 1 [9s Kh 6h Ts Qh]
Seat 2: Alice () showed [Js 7s] and won $39.75 with a good hand
Seat 4: Bob () showed [9h Td] and lost with a bad hand
Seat 6: Carol folded on the Pre-Flop
Board 2 [9s Kh 6h Ts 3c]
Seat 2: Alice () showed [Js 7s] and lost with a bad hand
Seat 4: Bob () showed [9h Td] and won $39.75 with a good hand
Seat 6: Carol folded on the Pre-Flop
`

// The second run's board says the hand was run twice, and the hand is written
// that way and reads back the same.
func TestRunItTwiceOldStyle(t *testing.T) {
	h := parseSample(t, sampleRunTwiceOldStyle)
	if !h.RunTwice {
		t.Fatal("the hand should be read as run twice")
	}
	if len(h.Board2) != 5 || h.Board2[4] == h.Board[4] {
		t.Fatalf("second board %v", h.Board2)
	}
	// As the cleaner does: the pot lines before the runs are not the ones a hand
	// run twice prints, so the hand fails the check and Repair puts them right.
	if !h.PotLinesStable() && h.Repair() == 0 {
		t.Fatal("the pot lines should be repaired")
	}
	if d := h.Diff(parseSample(t, h.String())); d != "" {
		t.Errorf("reads back differently, %s", d)
	}
}

func TestRunItTwice(t *testing.T) {
	h := parseSample(t, sampleRunTwice)
	if !h.RunTwice || len(h.Board) != 5 || len(h.Board2) != 5 {
		t.Fatalf("run twice %v, boards %v and %v", h.RunTwice, h.Board, h.Board2)
	}
	if h.Board[:4].String() != h.Board2[:4].String() || h.Board[4] == h.Board2[4] {
		t.Errorf("the runs share the turn and differ on the river: %v and %v", h.Board, h.Board2)
	}
	if len(h.Unknown) != 0 {
		t.Errorf("unknown lines %q", h.Unknown)
	}
	// The summary lists the winnings by board; they add up.
	if p := h.PlayerByName("Player044"); p.Won != 3089 || p.WonSecond != 1544 {
		t.Errorf("won %v, of it %v on the second board", p.Won, p.WonSecond)
	}
	if !h.ChipsConsistent() {
		t.Error("the pot should add up")
	}
}

// The best hand on each board must win something on it. With the ace of hearts on
// the second river Player070's aces beat the fours there, yet the summary still
// gives the second board to Player044.
func TestBoardWinnersMatch(t *testing.T) {
	if !parseSample(t, sampleRunTwice).BoardWinnersMatch() {
		t.Error("the sample's boards go to the best hands")
	}
	if !parseSample(t, sampleShowdown).BoardWinnersMatch() {
		t.Error("the one-board sample goes to the best hand")
	}
	h := parseSample(t, strings.ReplaceAll(sampleRunTwice, "9c", "Ah"))
	if h.BoardWinnersMatch() {
		t.Error("the second board went to the worse hand")
	}
	if h.FailedCheck() == "" {
		t.Error("the hand should fail the checks")
	}
}
