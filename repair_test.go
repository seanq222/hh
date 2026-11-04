package hh

import (
	"strings"
	"testing"
)

// A real hand: Player071 bets, then "folds" to its own bet. Player048's raise
// is missing, but the $2.49 returned to Player048 gives it away.
const sampleMissingRaise = `Hand #1000021 - Holdem(No Limit) - $0.25/$0.50 - 2026/01/01 00:00:00 UTC
Table15 6-max Seat #3 is the button
Seat 1: Player048 ($111.38)
Seat 3: Player071 ($81.28)
Player071 posts the small blind $0.25
Player048 posts the big blind $0.50
*** HOLE CARDS ***
Player071 raises $1.00 to $1.25
Player048 calls $0.75
*** FLOP *** [7d 3s Jc]
Main pot $2.38 | Rake $0.12
Player048 checks
Player071 bets $1.30
Player071 folds
Uncalled bet ($2.49) returned to Player048
*** SUMMARY ***
Total pot $4.85 | Rake $0.25
Board: [7d 3s Jc]
Seat 1: Player048 (big blind) did not show and won $4.85
Seat 3: Player071 (button) folded on the Flop
`

func TestRepairMissingRaise(t *testing.T) {
	h := parseSample(t, sampleMissingRaise)
	if h.ChipsConsistent() {
		t.Fatal("the sample should start out short of chips")
	}
	if n := h.Repair(); n != 1 {
		t.Fatalf("repaired %d", n)
	}
	if !h.ChipsConsistent() {
		t.Error("chips should balance after the repair")
	}
	var got *Action
	for i := range h.Actions {
		if h.Actions[i].Inferred {
			got = &h.Actions[i]
			if i == 0 || h.Actions[i-1].Kind != Bet || h.Actions[i+1].Kind != Fold {
				t.Errorf("inserted at the wrong place: %d", i)
			}
		}
	}
	if got == nil {
		t.Fatal("no inferred action")
	}
	if got.Player != "Player048" || got.Kind != Raise || got.Amount != 379 || got.To != 379 || got.Street != Flop || got.AllIn {
		t.Errorf("inferred action: %+v", got)
	}
	if n := h.Repair(); n != 0 {
		t.Errorf("a second repair changed %d", n)
	}

	// The views carry the inferred event, and the replay balances.
	views, err := h.PlayerHands()
	if err != nil {
		t.Fatal(err)
	}
	var inferred int
	for _, e := range views[0].Events {
		if e.Inferred {
			inferred++
		}
	}
	if inferred != 1 {
		t.Errorf("inferred events: %d", inferred)
	}
	// The big blind invests 0.50 + 0.75 + 3.79 and gets 2.49 back, so 2.55,
	// and wins 4.85.
	var bb *PlayerHand
	for i := range views {
		if views[i].Players[views[i].Hero].Position == BB {
			bb = &views[i]
		}
	}
	if bb.Invested() != 255 || bb.Won != 485 || bb.Net() != 230 {
		t.Errorf("invested %d won %d net %d", bb.Invested(), bb.Won, bb.Net())
	}
}

// The small blind's call of the raise is missing. The turn order says whose turn
// was skipped, and the pot says what the call was.
func TestRepairInsertsSkippedCall(t *testing.T) {
	h := parseSample(t, sampleMissingCall)
	if n := h.Repair(); n != 1 {
		t.Fatalf("repaired %d", n)
	}
	var got *Action
	for i := range h.Actions {
		if h.Actions[i].Inferred {
			got = &h.Actions[i]
		}
	}
	if got == nil || got.Player != "Player005" || got.Kind != Call || got.Amount != 87 || got.Street != Preflop {
		t.Errorf("inserted %+v", got)
	}
	if v := h.CheckTurnOrder(); v != TurnOK || !h.ChipsConsistent() {
		t.Errorf("not consistent after the repair: %v", v)
	}
}

func TestRepairLeavesUnrelatedDefectsAlone(t *testing.T) {
	// A hand that is already fine is untouched.
	h := parseSample(t, sampleShowdown)
	before := len(h.Actions)
	if n := h.Repair(); n != 0 || len(h.Actions) != before {
		t.Errorf("repaired %d", n)
	}
}

func TestReaderRepairs(t *testing.T) {
	// With repair turned off, the hand with the missing raise is dropped.
	rd := NewReader(strings.NewReader(sampleMissingRaise))
	rd.Repair = false
	if _, err := rd.Next(); err == nil || rd.Repaired() != 0 || rd.Dropped() != 1 {
		t.Errorf("err %v repaired %d dropped %d", err, rd.Repaired(), rd.Dropped())
	}
	// By default the raise is reconstructed and the hand is kept.
	rd = NewReader(strings.NewReader(sampleMissingRaise))
	h, err := rd.Next()
	if err != nil || rd.Repaired() != 1 {
		t.Fatalf("err %v repaired %d", err, rd.Repaired())
	}
	if !h.ChipsConsistent() {
		t.Error("not consistent after reading")
	}
}

// A player checks, the opponent's bet is missing, and the player folds.
const sampleMissingBet = `Hand #1000030 - Holdem(No Limit) - $0.25/$0.50 - 2026/01/01 00:00:00 UTC
Table26 6-max Seat #1 is the button
Seat 1: Alpha ($50.00)
Seat 2: Bravo ($50.00)
Alpha posts the small blind $0.25
Bravo posts the big blind $0.50
*** HOLE CARDS ***
Alpha calls $0.25
Bravo checks
*** FLOP *** [Ah 7c 2d]
Main pot $1.00 | Rake $0.00
Bravo checks
Bravo folds
Uncalled bet ($0.60) returned to Alpha
*** SUMMARY ***
Total pot $1.00
Board: [Ah 7c 2d]
Seat 1: Alpha (button) did not show and won $1.00
Seat 2: Bravo (big blind) folded on the Flop
`

func TestRepairMissingBetAfterCheck(t *testing.T) {
	h := parseSample(t, sampleMissingBet)
	// The missing bet and the returned-bet line for it cancel in the pot, so the
	// pot check cannot see it. The returned bet and the turn order can.
	if h.CheckReturn() != ReturnMismatch || h.CheckTurnOrder() != SkippedSeat {
		t.Fatalf("should start out inconsistent: %v %v", h.CheckReturn(), h.CheckTurnOrder())
	}
	if n := h.Repair(); n != 1 || !h.consistent() {
		t.Fatalf("repaired %d, consistent %v", n, h.consistent())
	}
	var got Action
	for _, a := range h.Actions {
		if a.Inferred {
			got = a
		}
	}
	if got.Player != "Alpha" || got.Kind != Bet || got.Amount != 60 || got.To != 0 || got.Street != Flop {
		t.Errorf("inferred: %+v", got)
	}
	if _, err := h.PlayerHands(); err != nil {
		t.Errorf("views: %v", err)
	}
}

// The dealer is seat 6, so the small blind belongs to seat 1, Player068, whose
// post line is missing: the pot of $1.25 needs it.
const sampleMissingSmallBlind = `Hand #1000022 - Holdem(No Limit) - $0.25/$0.50 - 2026/01/01 00:00:00 UTC
Table10 6-max Seat #6 is the button
Seat 1: Player068 ($83.40)
Seat 3: Player080 ($53.26)
Seat 4: Player077 ($20.82)
Seat 5: Player048 ($55.00)
Seat 6: Player072 ($33.55)
Player009 sitting out
Player080 posts the big blind $0.50
*** HOLE CARDS ***
Player077 folds
Player048 raises $1.50 to $1.50
Player072 folds
Player068 folds
Player080 folds
Uncalled bet ($1.00) returned to Player048
*** SUMMARY ***
Total pot $1.25
Seat 1: Player068 folded on the Pre-Flop and did not bet
Seat 3: Player080 (big blind) folded on the Pre-Flop
Seat 4: Player077 folded on the Pre-Flop and did not bet
Seat 5: Player048 did not show and won $1.25
Seat 6: Player072 (button) folded on the Pre-Flop and did not bet
`

func TestRepairMissingSmallBlind(t *testing.T) {
	h := parseSample(t, sampleMissingSmallBlind)
	if h.ChipsConsistent() {
		t.Fatal("should start out short of chips")
	}
	if n := h.Repair(); n != 1 || !h.ChipsConsistent() {
		t.Fatalf("repaired %d, consistent %v", n, h.ChipsConsistent())
	}
	var got Action
	var at int
	for i, a := range h.Actions {
		if a.Inferred {
			got, at = a, i
		}
	}
	if got.Player != "Player068" || got.Kind != PostSmallBlind || got.Amount != 25 || got.Street != Preflop {
		t.Errorf("inferred: %+v", got)
	}
	if h.Actions[at+1].Kind != PostBigBlind {
		t.Errorf("the small blind should come just before the big blind")
	}
	views, err := h.PlayerHands()
	if err != nil {
		t.Fatal(err)
	}
	var sbName string
	for _, v := range views {
		if v.Players[v.Hero].Position == SB {
			sbName = "found"
			if v.Invested() != 25 || v.Net() != -25 {
				t.Errorf("small blind invested %d net %d", v.Invested(), v.Net())
			}
		}
	}
	if sbName == "" {
		t.Error("no small blind view")
	}
}

// With no small blind owed (the big blind sits next to the dealer) a hand that
// balances is left alone.
func TestRepairLeavesLegitimateNoSmallBlind(t *testing.T) {
	s := `Hand #1000031 - Holdem(No Limit) - $0.25/$0.50 - 2026/01/01 00:00:00 UTC
Table26 6-max Seat #3 is the button
Seat 1: Alpha ($50.00)
Seat 2: Bravo ($50.00)
Seat 3: Charlie ($50.00)
Alpha posts the big blind $0.50
*** HOLE CARDS ***
Bravo folds
Charlie folds
*** SUMMARY ***
Total pot $0.50
Seat 1: Alpha (big blind) did not show and won $0.50
Seat 2: Bravo folded on the Pre-Flop and did not bet
Seat 3: Charlie (button) folded on the Pre-Flop and did not bet
`
	h := parseSample(t, s)
	before := len(h.Actions)
	if n := h.Repair(); n != 0 || len(h.Actions) != before {
		t.Errorf("repaired %d", n)
	}
}

func TestRepairMissingAnte(t *testing.T) {
	s := strings.Replace(sampleAnteAllIn, "Player082 posts ante $2.50\n", "", 1)
	h := parseSample(t, s)
	if h.ChipsConsistent() {
		t.Fatal("should start out short of chips")
	}
	if n := h.Repair(); n != 1 || !h.ChipsConsistent() {
		t.Fatalf("repaired %d, consistent %v", n, h.ChipsConsistent())
	}
	for _, a := range h.Actions {
		if a.Inferred && (a.Player != "Player082" || a.Kind != PostAnte || a.Amount != 250) {
			t.Errorf("inferred: %+v", a)
		}
	}
}

// Player069 raises, everyone folds, and the "Uncalled bet ($1.00) returned"
// line is missing. The pot ($1.25) shows it: 0.25 + 0.50 + 1.50 - 1.00.
const sampleMissingReturn = `Hand #1000016 - Holdem(No Limit) - $0.25/$0.50 - 2026/01/01 00:00:00 UTC
Table02 6-max Seat #3 is the button
Seat 2: Player062 ($55.29)
Seat 3: Player069 ($52.93)
Seat 4: Player066 ($50.51)
Player066 posts the small blind $0.25
Player062 posts the big blind $0.50
*** HOLE CARDS ***
Player069 raises $1.50 to $1.50
Player066 folds
Player062 folds
*** SUMMARY ***
Total pot $1.25
Seat 2: Player062 (big blind) folded on the Pre-Flop
Seat 3: Player069 (button) did not show and won $1.25
Seat 4: Player066 (small blind) folded on the Pre-Flop
`

func TestUncalledBetWithoutTheLine(t *testing.T) {
	h := parseSample(t, sampleMissingReturn)
	if h.Returned.Amount != 0 {
		t.Fatalf("the file has no returned-bet line: %+v", h.Returned)
	}
	seat, amount, ok := h.UncalledBet()
	if !ok || seat != 3 || amount != 100 {
		t.Fatalf("uncalled bet: seat %d amount %d ok %v", seat, amount, ok)
	}
	if !h.ChipsConsistent() {
		t.Error("the pot should balance with the uncalled bet taken out")
	}
	views, err := h.PlayerHands()
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range views {
		if v.Players[v.Hero].Position == BTN {
			// invests 1.50, gets 1.00 back, wins 1.25: net +0.75
			if v.Invested() != 50 || v.Won != 125 || v.Net() != 75 {
				t.Errorf("invested %d won %d net %d", v.Invested(), v.Won, v.Net())
			}
		}
	}
}

// A hand with its own returned-bet line keeps it as data, and it agrees with the
// actions.
func TestReturnedLineIsKeptAsData(t *testing.T) {
	h := parseSample(t, sampleUncalled)
	if h.Returned.Amount != 50 || h.Returned.Player != "Player061" {
		t.Errorf("returned: %+v", h.Returned)
	}
	if h.CheckReturn() != ReturnOK {
		t.Error("the file's line should match the actions")
	}
}

// The uncalled bet comes from the actions, not from the file's pot: with a wrong
// pot it is the same. A wrong total pot line alone is tolerated when the winner
// was paid what the actions say, but a wrong payout as well does not balance.
func TestUncalledBetIgnoresThePot(t *testing.T) {
	s := strings.Replace(sampleMissingReturn, "Total pot $1.25", "Total pot $1.40", 1)
	h := parseSample(t, s)
	if seat, amount, ok := h.UncalledBet(); !ok || seat != 3 || amount != 100 {
		t.Errorf("uncalled bet: seat %d amount %d ok %v", seat, amount, ok)
	}
	if !h.ChipsConsistent() {
		t.Error("a stale total pot line is tolerated when the payout matches the actions")
	}
	s = strings.Replace(s, "won $1.25", "won $1.40", 1)
	if parseSample(t, s).ChipsConsistent() {
		t.Error("a wrong pot and a wrong payout should not balance")
	}
}

// A bet that a player who is still in has not called is a missing call, not a
// returned bet.
func TestImpliedReturnNotForAMissingCall(t *testing.T) {
	s := `Hand #1000032 - Holdem(No Limit) - $0.25/$0.50 - 2026/01/01 00:00:00 UTC
Table26 6-max Seat #1 is the button
Seat 1: Alpha ($50.00)
Seat 2: Bravo ($50.00)
Alpha posts the small blind $0.25
Bravo posts the big blind $0.50
*** HOLE CARDS ***
Alpha calls $0.25
Bravo checks
*** FLOP *** [Ah 7c 2d]
Main pot $1.00 | Rake $0.00
Bravo checks
Alpha checks
*** TURN *** [Ah 7c 2d] [9s]
Main pot $1.00 | Rake $0.00
Bravo checks
Alpha checks
*** RIVER *** [Ah 7c 2d 9s] [3h]
Main pot $1.00 | Rake $0.00
Bravo checks
Alpha bets $2.00
*** SHOW DOWN ***
Main pot $5.00 | Rake $0.00
Alpha shows [Ad Kd]
Bravo shows [7d 7h]
*** SUMMARY ***
Total pot $5.00
Board: [Ah 7c 2d 9s 3h]
Seat 1: Alpha (button) showed [Ad Kd] and won $5.00 with a pair of Aces
Seat 2: Bravo (big blind) showed [7d 7h] and lost with three of a kind
`
	if _, _, ok := parseSample(t, s).UncalledBet(); ok {
		t.Error("a missing call must not be read as a returned bet")
	}
}

// A hand the file stops after the last fold: no summary, so no result.
const sampleCutOff = `Hand #1000100 - Holdem(No Limit) - $0.25/$0.50 - 2026/01/01 00:00:00 UTC
TableOne 6-max Seat #4 is the button
Seat 1: Alice ($50.00)
Seat 2: Bob ($50.00)
Seat 3: Carol ($50.00)
Seat 4: Dave ($50.00)
Alice posts the small blind $0.25
Bob posts the big blind $0.50
*** HOLE CARDS ***
Carol folds
Dave folds
Alice folds
`

func TestCutOffHandIsCompletedByRepair(t *testing.T) {
	lines := strings.Split(strings.TrimSpace(sampleCutOff), "\n")
	if _, err := ParseHand(lines); err == nil {
		t.Fatal("ParseHand should refuse a hand with no summary")
	}
	h, err := ParseHandPartial(lines)
	if err != nil || !h.NoSummary {
		t.Fatalf("partial: %v, NoSummary %v", err, h != nil && h.NoSummary)
	}
	if h.FailedCheck() == "" {
		t.Fatal("a hand with no summary should fail a check")
	}
	if n := h.Repair(); n != 1 {
		t.Fatalf("repaired %d", n)
	}
	if h.NoSummary || !h.Completed {
		t.Errorf("NoSummary %v, Completed %v", h.NoSummary, h.Completed)
	}
	bob := h.PlayerByName("Bob")
	if bob.Won != 50 || h.TotalPot != 50 || h.Returned.Amount != 25 || h.Returned.Player != "Bob" {
		t.Errorf("result: won %d, pot %d, returned %+v", bob.Won, h.TotalPot, h.Returned)
	}
	if m := h.FailedCheck(); m != "" {
		t.Errorf("still fails a check: %s", m)
	}
}

func TestCutOffHandAfterTheFlopIsNotCompleted(t *testing.T) {
	s := strings.TrimSpace(sampleCutOff)
	s = strings.Replace(s, "Dave folds\nAlice folds", "Dave calls $0.50\nAlice calls $0.25\nBob checks\n*** FLOP *** [2c 7d Th]\nAlice checks", 1)
	h, err := ParseHandPartial(strings.Split(s, "\n"))
	if err != nil {
		t.Fatal(err)
	}
	before := *h
	if n := h.Repair(); n != 0 || !h.NoSummary || len(h.Actions) != len(before.Actions) {
		t.Errorf("repaired %d, NoSummary %v", n, h.NoSummary)
	}
}

// A hand with antes and a missing big blind. A file's "raises $X to $Y" is short by
// the raiser's own ante, and a raise that follows the big blind is short by the
// blind too. The parser keeps the figures, a check finds them, and Repair puts the
// blind back and sets what each raise adds from its "to" figure.
const sampleAnteMissingBlind = `Hand #1000200 - Holdem(No Limit) - $0.25/$0.50 - 2026/01/01 00:00:00 UTC
TableOne 6-max Seat #6 is the button
Seat 1: Alice ($53.83)
Seat 4: Dave ($45.50)
Seat 5: Carol ($50.00)
Seat 6: Bob ($50.00)
Alice posts ante $0.25
Dave posts ante $0.25
Carol posts ante $0.25
Bob posts ante $0.25
Alice posts the small blind $0.25
*** HOLE CARDS ***
Carol raises $1.00 to $1.25
Bob folds
Alice folds
Dave raises $4.75 to $5.00
Carol folds
Uncalled bet ($3.75) returned to Dave
*** SUMMARY ***
Total pot $3.75
Seat 1: Alice (small blind) folded on the Pre-Flop
Seat 4: Dave did not show and won $3.75
Seat 5: Carol folded on the Pre-Flop
Seat 6: Bob (button) folded on the Pre-Flop
`

func TestAnteHandMissingBlindIsRepaired(t *testing.T) {
	h := parseSample(t, sampleAnteMissingBlind)
	// The parser keeps what the file says.
	if h.RaiseAmountsMatch() {
		t.Fatal("the raise that follows the missing blind should not match its figure")
	}
	if h.FailedCheck() == "" {
		t.Fatal("the hand should fail a check")
	}
	if n := h.Repair(); n == 0 {
		t.Fatal("Repair should put the hand right")
	}
	if m := h.FailedCheck(); m != "" {
		t.Fatalf("still fails: %s", m)
	}
	var blind *Action
	var raises []Cents
	for i := range h.Actions {
		a := &h.Actions[i]
		if a.Inferred && a.Kind == PostBigBlind {
			blind = a
		}
		if a.Kind == Raise {
			raises = append(raises, a.Amount)
		}
	}
	if blind == nil || blind.Player != "Dave" || blind.Amount != 50 {
		t.Errorf("big blind: %+v", blind)
	}
	if len(raises) != 2 || raises[0] != 125 || raises[1] != 450 {
		t.Errorf("raises add %v, want $1.25 and $4.50", raises)
	}
}

// The summary names Dave for the seat the seat list gives to Carol. The line is
// stale. It is a defect in the file, and Repair drops the line when it is not the
// winner's.
const sampleStaleSummaryName = `Hand #1000400 - Holdem(No Limit) - $0.25/$0.50 - 2026/01/01 00:00:00 UTC
TableOne 6-max Seat #3 is the button
Seat 1: Alice ($50.00)
Seat 2: Bob ($50.00)
Seat 3: Carol ($50.00)
Alice posts the small blind $0.25
Bob posts the big blind $0.50
*** HOLE CARDS ***
Carol folds
Alice folds
Uncalled bet ($0.25) returned to Bob
*** SUMMARY ***
Total pot $0.50
Seat 1: Alice (small blind) folded on the Pre-Flop
Seat 2: Bob (big blind) did not show and won $0.50
Seat 3: Dave (button) folded on the Pre-Flop and did not bet
`

func TestStaleSummaryNameIsCheckedAndDropped(t *testing.T) {
	h := parseSample(t, sampleStaleSummaryName)
	if h.SummaryNamesMatch() || !strings.Contains(h.FailedCheck(), "summary names") {
		t.Fatalf("the check should find the stale name, got %q", h.FailedCheck())
	}
	if n := h.Repair(); n == 0 {
		t.Fatal("Repair should drop the stale line")
	}
	if m := h.FailedCheck(); m != "" {
		t.Errorf("still fails: %s", m)
	}
}

func TestStaleSummaryNameOnTheWinnerIsNotRepaired(t *testing.T) {
	s := strings.Replace(sampleStaleSummaryName, "Seat 2: Bob (big blind)", "Seat 2: Dave (big blind)", 1)
	h := parseSample(t, s)
	h.Repair()
	// The stale line for the folded seat goes, the winner's stays, and the hand still
	// fails the check, since dropping the winner's line would lose the win.
	var winnerLine bool
	for _, u := range h.StaleSummary {
		winnerLine = winnerLine || strings.Contains(u, " won $")
	}
	if !winnerLine || h.SummaryNamesMatch() || h.FailedCheck() == "" {
		t.Errorf("winner's line kept %v, names match %v, check %q", winnerLine, h.SummaryNamesMatch(), h.FailedCheck())
	}
}

// A stale line that says its player folded on a street is dropped only if the seat's
// player folded on that street.
func TestStaleSummaryFoldMustAgreeWithTheActions(t *testing.T) {
	s := strings.Replace(sampleStaleSummaryName, "Dave (button) folded on the Pre-Flop", "Dave (button) folded on the Flop", 1)
	h := parseSample(t, s)
	h.Repair()
	if h.SummaryNamesMatch() {
		t.Error("Carol folded before the flop, so a line that says the seat folded on the flop is not harmless")
	}
}

// The name in the summary must be the whole name: Bob2 is not Bob.
func TestSummaryNameIsMatchedWhole(t *testing.T) {
	s := strings.Replace(sampleStaleSummaryName, "Seat 2: Bob (big blind)", "Seat 2: Bob2 (big blind)", 1)
	if h := parseSample(t, s); h.SummaryNamesMatch() {
		t.Error("Bob2 is not the player in seat 2")
	}
}

// A line the parser does not understand fails the check: what is on it is in no
// field.
func TestUnknownLineFailsTheCheck(t *testing.T) {
	s := strings.Replace(sampleStaleSummaryName, "Alice folds", "Alice does something odd", 1)
	h := parseSample(t, s)
	if len(h.Unknown) == 0 || h.FailedCheck() == "" {
		t.Errorf("unknown %q, check %q", h.Unknown, h.FailedCheck())
	}
}

// A dead blind written as "posts dead $0.00" next to the post that carries the
// money: the hand adds up without the empty line.
const sampleEmptyDeadPost = `Hand #1000600 - Holdem(No Limit) - $0.25/$0.50 - 2026/01/01 00:00:00 UTC
TableOne 6-max Seat #1 is the button
Seat 1: Alice ($103.40)
Seat 2: Bob ($50.00)
Seat 3: Carol ($35.75)
Seat 4: Dave ($64.89)
Seat 5: Erin ($61.50)
Seat 6: Frank ($47.50)
Bob posts the small blind $0.25
Carol posts the big blind $0.50
Frank posts dead $0.00
Frank posts $0.75
*** HOLE CARDS ***
Dave folds
Erin raises $1.50 to $1.50
Frank folds
Alice folds
Bob folds
Carol folds
Uncalled bet ($1.00) returned to Erin
*** SUMMARY ***
Total pot $1.75
Seat 1: Alice (button) folded on the Pre-Flop and did not bet
Seat 2: Bob (small blind) folded on the Pre-Flop
Seat 3: Carol (big blind) folded on the Pre-Flop
Seat 4: Dave folded on the Pre-Flop and did not bet
Seat 5: Erin did not show and won $2.00
Seat 6: Frank folded on the Pre-Flop
`

func TestEmptyDeadPostIsDropped(t *testing.T) {
	h := parseSample(t, sampleEmptyDeadPost)
	if h.CheckPosts() != LostDeadBlind {
		t.Fatalf("the check should find the empty dead blind: %v", h.CheckPosts())
	}
	if n := h.Repair(); n == 0 {
		t.Fatal("Repair should drop the empty post")
	}
	if m := h.FailedCheck(); m != "" {
		t.Fatalf("still fails: %s", m)
	}
	for _, a := range h.Actions {
		if a.Kind == PostDead && a.Amount == 0 {
			t.Error("the empty post is still there")
		}
	}
}

// A line saying a player sits out stays where it was among the posts when Repair
// removes a post before it or puts a missing one back, and the hand reads back
// as it was written.
func TestRepairKeepsSitOutLineInPlace(t *testing.T) {
	withLine := strings.Replace(sampleEmptyDeadPost, "Frank posts $0.75\n", "Frank posts $0.75\nGrace sitting out\n", 1)
	for name, s := range map[string]string{
		"empty dead post removed": withLine,
		"small blind put back":    strings.Replace(withLine, "Bob posts the small blind $0.25\n", "", 1),
	} {
		h := parseSample(t, s)
		if h.Repair() == 0 {
			t.Fatalf("%s: nothing repaired", name)
		}
		text := h.String()
		before, _, _ := strings.Cut(text, "*** HOLE CARDS ***")
		if !strings.HasSuffix(strings.TrimRight(before, "\r\n"), "Frank posts $0.75\nGrace sitting out") {
			t.Errorf("%s: the sitting-out line is not after the last post:\n%s", name, before)
		}
		if d := h.Diff(parseSample(t, text)); d != "" {
			t.Errorf("%s: reads back differently, %s", name, d)
		}
	}
}

// Bob's call on the flop takes his last chips but does not say so, and the file
// then has him and Alice checking the turn and the river.
const sampleChecksWithEmptyStack = `Hand #1000702 - Holdem (No Limit) - $0.25/$0.50 - 2026/01/01 00:00:00 UTC
TableOne 6-max Seat #1 is the button
Seat 1: Alice ($50.00)
Seat 2: Bob ($4.50)
Seat 3: Carol ($50.00)
Bob posts the small blind $0.25
Carol posts the big blind $0.50
*** HOLE CARDS ***
Alice raises $1.50 to $1.50
Bob calls $1.25
Carol folds
*** FLOP *** [2c 7h 9s]
Main pot $3.50 | Rake $0.00
Bob checks
Alice bets $3.00
Bob calls $3.00
*** TURN *** [2c 7h 9s] [Jd]
Main pot $9.50 | Rake $0.00
Bob checks
Alice checks
*** RIVER *** [2c 7h 9s Jd] [3s]
Main pot $9.50 | Rake $0.00
Bob checks
Alice checks
*** SHOW DOWN ***
Main pot $9.50 | Rake $0.00
Alice shows [Ah Ad] (a pair of Aces [Ah Ad Jd 9s 7h])
Bob shows [Ks Kc] (a pair of Kings [Ks Kc Jd 9s 7h])
Alice collected $9.50 from main pot
*** SUMMARY ***
Total pot $9.50 | Rake $0.00
Board [2c 7h 9s Jd 3s]
Seat 1: Alice (button) showed [Ah Ad] and won $9.50 with a pair of Aces [Ah Ad Jd 9s 7h]
Seat 2: Bob (small blind) showed [Ks Kc] and lost with a pair of Kings [Ks Kc Jd 9s 7h]
Seat 3: Carol (big blind) folded on the Pre-Flop
`

// A player who acts after putting the whole stack in fails the check; Repair
// marks the call all-in and removes the checks no one could make.
func TestChecksWithEmptyStackAreDropped(t *testing.T) {
	h := parseSample(t, sampleChecksWithEmptyStack)
	if h.NoActsWithEmptyStack() {
		t.Fatal("the check should find Bob's checks")
	}
	if h.Repair() == 0 {
		t.Fatal("nothing repaired")
	}
	if m := h.FailedCheck(); m != "" {
		t.Fatalf("still fails: %s", m)
	}
	for _, a := range h.Actions {
		if a.Street > Flop {
			t.Errorf("an action is left on the %v: %v", a.Street, a)
		}
	}
	last := h.Actions[len(h.Actions)-1]
	if last.Player != "Bob" || last.Kind != Call || !last.AllIn {
		t.Errorf("Bob's call should be the last action and all-in: %+v", last)
	}
	if d := h.Diff(parseSample(t, h.String())); d != "" {
		t.Errorf("reads back differently, %s", d)
	}
}

// A player the seat list marks as sitting out folds: the mark is stale.
func TestStaleSitOutMarkIsCleared(t *testing.T) {
	h := parseSample(t, sampleSittingOutActs)
	if h.CheckTurnOrder() != ActedWhileSittingOut {
		t.Fatalf("the check should find it: %v", h.CheckTurnOrder())
	}
	h.Repair()
	if h.CheckTurnOrder() == ActedWhileSittingOut {
		t.Error("the stale mark is still there")
	}
	for _, p := range h.Players {
		if p.SittingOut {
			t.Errorf("%s is still marked sitting out", p.Name)
		}
	}
}

// Bob moved from seat 2 to seat 5 and the file left him at both. He plays from
// seat 5, the seat the summary names: heads-up the big blind is the seat after
// the button, which is seat 5 only once seat 2 is gone.
const sampleNameAtTwoSeats = `Hand #1000703 - Holdem (No Limit) - $0.25/$0.50 - 2026/01/01 00:00:00 UTC
TableOne 6-max Seat #6 is the button
Seat 2: Bob ($50.00)
Seat 5: Bob ($50.00)
Seat 6: Alice ($50.25)
Alice posts the small blind $0.25
Bob posts the big blind $0.50
*** HOLE CARDS ***
Alice raises $1.25 to $1.50
Bob calls $1.00
*** FLOP *** [2c 7h 9s]
Main pot $3.00 | Rake $0.00
Bob checks
Alice bets $2.00
Bob folds
Uncalled bet ($2.00) returned to Alice
Alice does not show
*** SUMMARY ***
Total pot $3.00 | Rake $0.00
Board [2c 7h 9s]
Seat 5: Bob (big blind) folded on the Flop
Seat 6: Alice (small blind) did not show and won $3.00
`

// A name at two seats fails the check; Repair drops the seat the summary does not
// name and gives its actions to the other.
func TestNameAtTwoSeats(t *testing.T) {
	h := parseSample(t, sampleNameAtTwoSeats)
	if h.NamesUnique() {
		t.Fatal("the check should find Bob at two seats")
	}
	if h.Repair() == 0 {
		t.Fatal("nothing repaired")
	}
	if m := h.FailedCheck(); m != "" {
		t.Fatalf("still fails: %s", m)
	}
	if len(h.Players) != 2 || h.PlayerByName("Bob").Seat != 5 {
		t.Fatalf("Bob should be left at seat 5 alone: %+v", h.Players)
	}
	for _, a := range h.Actions {
		if a.Player == "Bob" && a.Seat != 5 {
			t.Errorf("Bob's action is at seat %d: %+v", a.Seat, a)
		}
	}
	if d := h.Diff(parseSample(t, h.String())); d != "" {
		t.Errorf("reads back differently, %s", d)
	}
}

// Bob is all-in from the small blind for less than the big blind, and Carol, the
// only player left with chips, checks: there is no one she could bet against.
const sampleLoneCheck = `Hand #1000704 - Holdem (No Limit) - $0.25/$0.50 - 2026/01/01 00:00:00 UTC
TableOne 6-max Seat #1 is the button
Seat 1: Alice ($50.00)
Seat 2: Bob ($0.40)
Seat 3: Carol ($50.00)
Bob posts the small blind $0.25
Carol posts the big blind $0.50
*** HOLE CARDS ***
Alice folds
Bob calls $0.15 and is all-in
Carol checks
Uncalled bet ($0.10) returned to Carol
*** FLOP *** [2c 7h 9s]
Main pot $0.80 | Rake $0.00
*** TURN *** [2c 7h 9s] [Jd]
Main pot $0.80 | Rake $0.00
*** RIVER *** [2c 7h 9s Jd] [3s]
Main pot $0.80 | Rake $0.00
*** SHOW DOWN ***
Main pot $0.80 | Rake $0.00
Bob shows [Ah Ad] (a pair of Aces [Ah Ad Jd 9s 7h])
Carol shows [Ks Kc] (a pair of Kings [Ks Kc Jd 9s 7h])
Bob collected $0.80 from main pot
*** SUMMARY ***
Total pot $0.80 | Rake $0.00
Board [2c 7h 9s Jd 3s]
Seat 1: Alice (button) folded on the Pre-Flop
Seat 2: Bob (small blind) showed [Ah Ad] and won $0.80 with a pair of Aces [Ah Ad Jd 9s 7h]
Seat 3: Carol (big blind) showed [Ks Kc] and lost with a pair of Kings [Ks Kc Jd 9s 7h]
`

// The lone check fails the check; Repair removes it.
func TestLoneCheckIsDropped(t *testing.T) {
	h := parseSample(t, sampleLoneCheck)
	if h.NoLoneChecks() {
		t.Fatal("the check should find Carol's check")
	}
	if h.Repair() == 0 {
		t.Fatal("nothing repaired")
	}
	if m := h.FailedCheck(); m != "" {
		t.Fatalf("still fails: %s", m)
	}
	for _, a := range h.Actions {
		if a.Kind == Check {
			t.Errorf("a check is left: %+v", a)
		}
	}
	if d := h.Diff(parseSample(t, h.String())); d != "" {
		t.Errorf("reads back differently, %s", d)
	}
}

// The dead blind written as "posts dead $0.00" with its amount left in the next
// post, and taken out of that player's raise and the pot: the money is lost and
// the hand cannot add up. The raise's "to" figure is not what the chips come to,
// so the check fails, and repair cannot mend it.
const sampleLostDeadRaise = `Hand #1000601 - Holdem(No Limit) - $0.50/$1.00 - 2026/01/01 00:00:00 UTC
TableOne 6-max Seat #1 is the button
Seat 1: Alice ($33.75)
Seat 2: Bob ($132.28)
Seat 3: Carol ($101.78)
Seat 5: Dave ($98.50)
Seat 6: Frank ($89.82)
Bob posts the small blind $0.50
Carol posts the big blind $1.00
Frank posts dead $0.00
Frank posts $1.50
*** HOLE CARDS ***
Dave folds
Frank raises $1.00 to $2.50
Alice folds
Bob folds
Carol folds
Uncalled bet ($1.50) returned to Frank
*** SUMMARY ***
Total pot $2.50
Seat 1: Alice (button) folded on the Pre-Flop and did not bet
Seat 2: Bob (small blind) folded on the Pre-Flop
Seat 3: Carol (big blind) folded on the Pre-Flop
Seat 5: Dave folded on the Pre-Flop and did not bet
Seat 6: Frank did not show and won $2.50
`

func TestLostDeadBlindRaiseFails(t *testing.T) {
	h := parseSample(t, sampleLostDeadRaise)
	if h.RaiseToMatches() {
		t.Error("the raise's to figure should not match the chips")
	}
	h.Repair()
	if h.FailedCheck() == "" {
		t.Error("the hand should still fail after repair")
	}
	// The same hand with the dead blind written as the client writes it adds up.
	good := strings.NewReplacer("Frank posts dead $0.00", "Frank posts dead $0.50", "Frank posts $1.50", "Frank posts $1.00",
		"Frank raises $1.00 to $2.50", "Frank raises $1.50 to $2.50", "Total pot $2.50", "Total pot $3.00", "won $2.50", "won $3.00").Replace(sampleLostDeadRaise)
	if h := parseSample(t, good); !h.RaiseToMatches() || h.FailedCheck() != "" {
		t.Errorf("the client's version should pass: to matches %v, check %q", h.RaiseToMatches(), h.FailedCheck())
	}
}
