package hh

import (
	"strings"
	"testing"
)

// Hands that Hand2Note rejects, as made-up copies of the faults found in real
// files: players, tables, IDs and times are invented. Each breaks a rule the
// package now checks. Real hand histories do not belong in this repository.

// The only player left with chips checks and then folds on the river, though
// the other player is all-in: the betting round was over.
const sampleActsTwice = `Hand #1000001 - Holdem(No Limit) - $0.25/$0.50 - 2026/01/01 00:00:00 UTC
Table23 6-max Seat #6 is the button
Seat 1: Alice ($90.62)
Seat 2: Bob ($123.34)
Seat 3: Carol ($55.60)
Seat 4: Dave ($49.60)
Seat 5: Erin ($56.34)
Seat 6: Frank ($6.47)
Alice posts the small blind $0.25
Bob posts the big blind $0.50
*** HOLE CARDS ***
Carol folds
Dave folds
Erin folds
Frank raises $1.25 to $1.25
Alice calls $1.00
Bob folds
*** FLOP *** [7s 6s 4s]
Main pot $2.85 | Rake $0.15
Alice checks
Frank bets $1.25
Alice calls $1.25
*** TURN *** [7s 6s 4s] [4h]
Main pot $5.23 | Rake $0.27
Alice checks
Frank bets $3.97 and is all-in
Alice calls $3.97
*** RIVER *** [7s 6s 4s 4h] [Ah]
Main pot $12.77 | Rake $0.67
Alice checks
Alice folds
*** SUMMARY ***
Total pot $12.77 | Rake $0.67
Board: [7s 6s 4s 4h Ah]
Seat 1: Alice (small blind) folded on the River
Seat 2: Bob (big blind) folded on the Pre-Flop
Seat 3: Carol folded on the Pre-Flop
Seat 4: Dave folded on the Pre-Flop
Seat 5: Erin folded on the Pre-Flop
Seat 6: Frank (button) did not show and won $12.77
`

// The big blind is all-in on the turn and is called, so the river has no betting,
// but the caller folds on it.
const sampleActsAfterClose = `Hand #1000002 - Holdem(No Limit) - $0.25/$0.50 - 2026/01/01 00:00:00 UTC
Table25 6-max Seat #4 is the button
Seat 1: Grace ($57.47)
Seat 2: Heidi ($69.01)
Seat 3: Ivan ($49.75)
Seat 4: Judy ($34.92)
Seat 5: Mallory ($64.48)
Seat 6: Dave ($2.03)
Mallory posts the small blind $0.25
Dave posts the big blind $0.50
*** HOLE CARDS ***
Grace folds
Heidi raises $1.25 to $1.25
Ivan folds
Judy folds
Mallory folds
Dave calls $0.75
*** FLOP *** [5c 5s 7s]
Main pot $2.62 | Rake $0.13
Dave checks
Heidi checks
*** TURN *** [5c 5s 7s] [6c]
Main pot $2.62 | Rake $0.13
Dave bets $0.78 and is all-in
Heidi calls $0.78
*** RIVER *** [5c 5s 7s 6c] [3c]
Main pot $4.10 | Rake $0.21
Heidi folds
*** SUMMARY ***
Total pot $4.10 | Rake $0.21
Board: [5c 5s 7s 6c 3c]
Seat 1: Grace folded on the Pre-Flop and did not bet
Seat 2: Heidi folded on the River
Seat 3: Ivan folded on the Pre-Flop and did not bet
Seat 4: Judy (button) folded on the Pre-Flop and did not bet
Seat 5: Mallory (small blind) folded on the Pre-Flop
Seat 6: Dave (big blind) did not show and won $4.10
`

// A player the seat list marks as sitting out folds.
const sampleSittingOutActs = `Hand #1000003 - Holdem (No Limit) - $0.25/$0.50 - 2026/01/01 00:00:00 UTC
Table24 6-max Seat #5 is the button
Seat 2: Niaj ($71.34)
Seat 3: Olivia ($23.02) is sitting out
Seat 4: Peggy ($186.85)
Seat 5: Rupert ($51.34)
Seat 6: Sybil ($50.00)
Sybil posts the small blind $0.25
Niaj posts the big blind $0.50
*** HOLE CARDS ***
Olivia folds
Peggy folds
Rupert raises $1.50 to $1.50
Sybil folds
Niaj folds
Uncalled bet ($1.00) returned to Rupert
*** SUMMARY ***
Total pot $1.25
Seat 2: Niaj (big blind) folded on the Pre-Flop
Seat 3: Olivia folded on the Pre-Flop and did not bet
Seat 4: Peggy folded on the Pre-Flop and did not bet
Seat 5: Rupert (button) did not show and won $1.25
Seat 6: Sybil (small blind) folded on the Pre-Flop
`

// The caller is marked all-in but the call leaves $0.50 of the seat-line stack:
// $3.86 + $6.11 + $9.34 is $19.31 of $19.81.
const sampleWrongCall = `Hand #1000004 - Holdem(No Limit) - $0.25/$0.50 - 2026/01/01 00:00:00 UTC
Table22 6-max Seat #2 is the button
Seat 2: Frank ($19.81)
Seat 3: Trent ($56.12)
Seat 4: Victor ($55.73)
Seat 5: Walter ($48.50)
Seat 6: Zed ($87.28)
Trent posts the small blind $0.25
Victor posts the big blind $0.50
*** HOLE CARDS ***
Walter folds
Zed raises $1.43 to $1.43
Frank raises $3.86 to $3.86
Trent folds
Victor folds
Zed calls $2.43
*** FLOP *** [3c 9s Qs]
Main pot $8.05 | Rake $0.42
Zed checks
Frank checks
*** TURN *** [3c 9s Qs] [5c]
Main pot $8.05 | Rake $0.42
Zed checks
Frank bets $6.11
Zed calls $6.11
*** RIVER *** [3c 9s Qs 5c] [4d]
Main pot $19.66 | Rake $1.03
Zed bets $10.34
Frank calls $9.34 and is all-in
Uncalled bet ($1.00) returned to Zed
*** SHOW DOWN ***
Main pot $37.41 | Rake $1.96
Frank shows [9h Ad] (a pair of Nines [9s 9h Ad Qs 5c])
Zed shows [Ah 2h] (a straight, Five high [5c 4d 3c 2h Ah])
*** SUMMARY ***
Total pot $37.41 | Rake $1.96
Board: [3c 9s Qs 5c 4d]
Seat 2: Frank (button) showed [9h Ad] and lost with a pair of Nines [9s 9h Ad Qs 5c]
Seat 3: Trent (small blind) folded on the Pre-Flop
Seat 4: Victor (big blind) folded on the Pre-Flop
Seat 5: Walter folded on the Pre-Flop and did not bet
Seat 6: Zed showed [Ah 2h] and won $37.41 with a straight, Five high [5c 4d 3c 2h Ah]
`

// The small blind's post line is repeated around a sit-out line. Hand2Note
// rejects the hand: the blind posted is greater than the blind at the table.
const sampleBlindPostedTwice = `Hand #1000005 - Holdem(No Limit) - $0.25/$0.50 - 2026/01/01 00:00:00 UTC
Table21 6-max Seat #3 is the button
Seat 3: Quinn ($86.96)
Seat 4: Ursula ($34.49)
Quinn posts the small blind $0.25
Wendy sitting out
Quinn posts the small blind $0.25
Ursula posts the big blind $0.50
*** HOLE CARDS ***
Quinn raises $1.00 to $1.50
Ursula calls $1.00
*** FLOP *** [Jd 4c Ks]
Main pot $2.85 | Rake $0.15
Ursula checks
Quinn bets $1.05
Ursula calls $1.05
*** TURN *** [Jd 4c Ks] [Jc]
Main pot $4.85 | Rake $0.25
Ursula checks
Quinn bets $2.80
Ursula calls $2.80
*** RIVER *** [Jd 4c Ks Jc] [Js]
Main pot $10.20 | Rake $0.50
Ursula bets $6.85
Quinn raises $81.36 to $81.36 and is all-in
Ursula calls $22.29 and is all-in
Uncalled bet ($52.22) returned to Quinn
*** SHOW DOWN ***
Main pot $68.48 | Rake $0.50
Quinn shows [5h Jh] (four of Kind of Jacks [Js Jh Jd Jc Ks])
Ursula shows [Ad 8h] (three of a kind, Set of Jacks [Js Jd Jc Ad Ks])
*** SUMMARY ***
Total pot $68.48 | Rake $0.50
Board: [Jd 4c Ks Jc Js]
Seat 3: Quinn (button) showed [5h Jh] and won $68.48 with four of Kind of Jacks [Js Jh Jd Jc Ks]
Seat 4: Ursula (big blind) showed [Ad 8h] and lost with three of a kind, Set of Jacks [Js Jd Jc Ad Ks]
`

func TestBlindPostedTwice(t *testing.T) {
	if got := parseSample(t, sampleBlindPostedTwice).CheckPosts(); got != BlindPostedTwice {
		t.Errorf("a repeated small blind post: %v", got)
	}
	once := strings.Replace(sampleBlindPostedTwice, "Wendy sitting out\nQuinn posts the small blind $0.25\n", "Wendy sitting out\n", 1)
	if got := parseSample(t, once).CheckPosts(); got != PostsOK {
		t.Errorf("the blind posted once: %v", got)
	}
}

func TestStrictTurnOrder(t *testing.T) {
	for name, tc := range map[string]struct {
		sample string
		want   TurnViolation
	}{
		"the only live player acts on a river with no betting": {sampleActsTwice, ActedAfterClose},
		"a lone live player acts on a street with no betting":  {sampleActsAfterClose, ActedAfterClose},
		"a player listed as sitting out acts":                  {sampleSittingOutActs, ActedWhileSittingOut},
	} {
		if got := parseSample(t, tc.sample).CheckTurnOrder(); got != tc.want {
			t.Errorf("%s: %v, want %v", name, got, tc.want)
		}
	}
	// The ordinary samples are in order.
	for name, s := range map[string]string{"showdown": sampleShowdown, "bomb pot": sampleRealBombPot, "run twice": sampleRunTwice} {
		if got := parseSample(t, s).CheckTurnOrder(); got != TurnOK {
			t.Errorf("%s: %v", name, got)
		}
	}
}

func TestCheckAmounts(t *testing.T) {
	if got := parseSample(t, sampleWrongCall).CheckAmounts(); got != WrongCallAmount {
		t.Errorf("a call that leaves chips behind an all-in: %v", got)
	}
	// A call for the amount owed is fine, and so is an all-in call for less.
	fixed := strings.Replace(sampleWrongCall, "calls $9.34 and is all-in", "calls $9.84 and is all-in", 1)
	if got := parseSample(t, fixed).CheckAmounts(); got != AmountsOK {
		t.Errorf("the right call: %v", got)
	}
	for name, s := range map[string]string{"showdown": sampleShowdown, "bomb pot": sampleRealBombPot, "run twice": sampleRunTwice, "stale total pot": sampleStaleTotalPot} {
		if got := parseSample(t, s).CheckAmounts(); got != AmountsOK {
			t.Errorf("%s: %v", name, got)
		}
	}
}
