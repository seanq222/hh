package hh

import (
	"strings"
	"testing"
)

// A real hand: Player039's call of the opening raise is missing, so after
// Player059 calls, the next player to act is skipped.
const sampleSkippedSeat = `Hand #1000020 - Holdem(No Limit) - $0.25/$0.50 - 2026/01/01 00:00:00 UTC
Table14 6-max Seat #3 is the button
Seat 1: Player059 ($127.45)
Seat 2: Player039 ($51.07)
Seat 3: Player067 ($50.53)
Seat 4: Player080 ($69.47)
Seat 5: Player087 ($88.11)
Seat 6: Player055 ($54.47)
Player080 posts the small blind $0.25
Player087 posts the big blind $0.50
*** HOLE CARDS ***
Player055 raises $1.20 to $1.20
Player059 calls $1.20
Player067 folds
Player080 raises $8.30 to $8.55
Player087 folds
Player055 folds
Player059 calls $7.35
Player039 calls $7.35
*** FLOP *** [9c 2c 5s]
Main pot $25.99 | Rake $1.36
Player080 checks
Player059 bets $13.67
Player039 raises $42.52 to $42.52 and is all-in
Player080 folds
Player059 folds
Uncalled bet ($28.85) returned to Player039
*** SUMMARY ***
Total pot $51.96 | Rake $2.73
Board: [9c 2c 5s]
Seat 1: Player059 folded on the Flop
Seat 2: Player039 did not show and won $51.96
Seat 3: Player067 (button) folded on the Pre-Flop and did not bet
Seat 4: Player080 (small blind) folded on the Flop
Seat 5: Player087 (big blind) folded on the Pre-Flop
Seat 6: Player055 folded on the Pre-Flop
`

func TestTurnOrderAcceptsGoodHands(t *testing.T) {
	for name, s := range map[string]string{
		"showdown":   sampleShowdown,
		"uncalled":   sampleUncalled,
		"posts":      samplePostsAndPartial,
		"bombpot":    sampleRealBombPot,
		"sittingout": sampleSittingOut,
		"allinbomb":  sampleAllInBomb,
		"hero":       sampleHero,
	} {
		if v := parseSample(t, s).CheckTurnOrder(); v != TurnOK {
			t.Errorf("%s: %v", name, v)
		}
	}
}

func TestTurnOrderSkippedSeat(t *testing.T) {
	if v := parseSample(t, sampleSkippedSeat).CheckTurnOrder(); v != SkippedSeat {
		t.Errorf("Player039 hand: %v", v)
	}
	// The small blind's call is missing, so the big blind folds out of turn.
	if v := parseSample(t, sampleMissingCall).CheckTurnOrder(); v != SkippedSeat {
		t.Errorf("Table03 hand: %v", v)
	}
}

// A player who bets and then folds to their own bet is really a skipped
// opponent: the opponent's raise is missing.
func TestTurnOrderMissingRaise(t *testing.T) {
	if v := parseSample(t, sampleMissingRaise).CheckTurnOrder(); v != SkippedSeat {
		t.Errorf("before repair: %v", v)
	}
	h := parseSample(t, sampleMissingRaise)
	h.Repair()
	if v := h.CheckTurnOrder(); v != TurnOK {
		t.Errorf("after repair: %v", v)
	}
}

func TestTurnOrderActedAfterFold(t *testing.T) {
	// Player072 folds early and then calls out of turn.
	s := strings.Replace(sampleShowdown, "Player030 folds", "Player072 calls $7.00", 1)
	if v := parseSample(t, s).CheckTurnOrder(); v != ActedAfterFold {
		t.Errorf("%v", v)
	}
}

func TestReaderDropsOnTurnOrder(t *testing.T) {
	rd := NewReader(strings.NewReader(sampleShowdown + "\n" + sampleSkippedSeat + "\n" + sampleMissingRaise))
	rd.Repair = false
	var ids []int64
	var dropped []int64
	for {
		h, err := rd.Next()
		if err != nil {
			pe, ok := err.(*ParseError)
			if ok && pe.Defective {
				dropped = append(dropped, pe.HandID)
				if !strings.Contains(pe.Msg, "skipped") {
					t.Errorf("message %q", pe.Msg)
				}
				continue
			}
			break
		}
		ids = append(ids, h.ID)
	}
	// With repair off the missing raise is dropped as well.
	if len(dropped) != 2 || dropped[0] != 1000020 || dropped[1] != 1000021 || rd.Dropped() != 2 {
		t.Errorf("dropped %v", dropped)
	}
	if len(ids) != 1 || ids[0] != 1000010 {
		t.Errorf("ids %v", ids)
	}
}

// Player075's ante is his whole stack, so he is all-in before the flop and is
// not asked to act. A real hand.
const sampleAnteAllIn = `Hand #1000027 - Holdem(No Limit) - $0.25/$0.50 - 2026/01/01 00:00:00 UTC
Table19 6-max Seat #1 is the button
Seat 1: Player016 ($80.99)
Seat 2: Player082 ($50.00)
Seat 3: Player023 ($50.00)
Seat 4: Player075 ($2.12)
Seat 5: Player028 ($48.50)
Seat 6: Player053 ($33.50)
Player016 posts ante $2.50
Player082 posts ante $2.50
Player023 posts ante $2.50
Player075 posts ante $2.12
Player028 posts ante $2.50
Player053 posts ante $2.50
*** HOLE CARDS ***
*** FLOP *** [Jc 5c Tc]
Main pot $13.89 | Rake $0.73
Player082 checks
Player023 checks
Player028 checks
Player053 checks
Player016 bets $3.57
Player082 folds
Player023 folds
Player028 calls $3.57
Player053 calls $3.57
*** TURN *** [Jc 5c Tc] [7s]
Main pot $24.07 | Rake $1.26
Player028 checks
Player053 checks
Player016 checks
*** RIVER *** [Jc 5c Tc 7s] [3h]
Main pot $24.07 | Rake $1.26
Player028 checks
Player053 bets $6.33
Player016 folds
Player028 calls $6.33
*** SHOW DOWN ***
Main pot $36.10 | Rake $1.89
Player028 shows [Jh 4h] (a pair of Jacks [Jh Jc Tc 7s 5c])
*** SUMMARY ***
BombPot
Total pot $36.10 | Rake $1.89
Board: [Jc 5c Tc 7s 3h]
Seat 1: Player016 (button) folded on the River
Seat 2: Player082 folded on the Flop
Seat 3: Player023 folded on the Flop
Seat 4: Player075 did not show
Seat 5: Player028 showed [Jh 4h] and won $36.10 with a pair of Jacks [Jh Jc Tc 7s 5c]
Seat 6: Player053 did not show
`

func TestTurnOrderAnteAllIn(t *testing.T) {
	if v := parseSample(t, sampleAnteAllIn).CheckTurnOrder(); v != TurnOK {
		t.Errorf("%v", v)
	}
}

// Player004's call uses his whole stack but is not marked all-in. The file
// still logs him acting on the next street ("checks and is all-in").
func TestTurnOrderCallAllInIsLoggedLater(t *testing.T) {
	s := `Hand #1000026 - Holdem(No Limit) - $0.25/$0.50 - 2026/01/01 00:00:00 UTC
Table06 6-max Seat #2 is the button
Seat 1: Player072 ($86.54)
Seat 2: Player004 ($6.25)
Seat 3: Player010 ($55.13)
Player072 posts ante $2.50
Player004 posts ante $2.50
Player010 posts ante $2.50
*** HOLE CARDS ***
*** FLOP *** [8d 6h 3d]
Main pot $7.50 | Rake $0.00
Player010 checks
Player072 bets $3.75
Player004 calls $3.75
Player010 folds
*** TURN *** [8d 6h 3d] [8s]
Main pot $15.00 | Rake $0.00
Player072 checks
Player004 checks and is all-in
*** SUMMARY ***
Total pot $15.00
Board: [8d 6h 3d 8s]
Seat 1: Player072 folded on the Turn
Seat 2: Player004 (button) did not show and won $15.00
Seat 3: Player010 folded on the Flop
`
	if v := parseSample(t, s).CheckTurnOrder(); v != TurnOK {
		t.Errorf("%v", v)
	}
}

// Player085, the button, never acts but gets a bet back and wins: the raise
// that made them the last aggressor is missing. A real hand.
const sampleWinnerNeverActs = `Hand #1000018 - Holdem(No Limit) - $0.25/$0.50 - 2026/01/01 00:00:00 UTC
Table18 6-max Seat #1 is the button
Seat 1: Player085 ($49.00)
Seat 2: Player054 ($50.00)
Seat 3: Player063 ($49.75)
Seat 4: Player076 ($53.62)
Seat 5: Player069 ($114.12)
Seat 6: Player086 ($60.17)
Player054 posts the small blind $0.25
Player063 posts the big blind $0.50
*** HOLE CARDS ***
Player076 folds
Player069 raises $1.50 to $1.50
Player086 folds
Player054 folds
Player063 folds
Player069 folds
Uncalled bet ($3.00) returned to Player085
*** SUMMARY ***
Total pot $3.75
Seat 1: Player085 (button) did not show and won $3.75
Seat 2: Player054 (small blind) folded on the Pre-Flop
Seat 3: Player063 (big blind) folded on the Pre-Flop
Seat 4: Player076 folded on the Pre-Flop and did not bet
Seat 5: Player069 folded on the Pre-Flop
Seat 6: Player086 folded on the Pre-Flop and did not bet
`

// Player043 is first to act after the big blind, but Player078 folds first, and
// Player043 appears only through the returned bet and the win.
const sampleFirstToActMissing = `Hand #1000025 - Holdem(No Limit) - $0.25/$0.50 - 2026/01/01 00:00:00 UTC
Table13 6-max Seat #4 is the button
Seat 1: Player037 ($50.00)
Seat 2: Player043 ($50.87)
Seat 3: Player078 ($69.17)
Seat 4: Player074 ($49.75)
Seat 6: Player047 ($64.90)
Player047 posts the small blind $0.25
Player037 posts the big blind $0.50
*** HOLE CARDS ***
Player078 folds
Player074 folds
Player047 folds
Player037 folds
Uncalled bet ($0.50) returned to Player043
*** SUMMARY ***
Total pot $1.25
Seat 1: Player037 (big blind) folded on the Pre-Flop
Seat 2: Player043 did not show and won $1.25
Seat 3: Player078 folded on the Pre-Flop and did not bet
Seat 4: Player074 (button) folded on the Pre-Flop and did not bet
Seat 6: Player047 (small blind) folded on the Pre-Flop
`

// Player084 posted an ante, is listed in the summary, and has no fold line.
const sampleSilentFold = `Hand #1000029 - Holdem(No Limit) - $0.25/$0.50 - 2026/01/01 00:00:00 UTC
Table16 6-max Seat #5 is the button
Seat 1: Player031 ($50.00)
Seat 2: Player002 ($76.36)
Seat 3: Player084 ($50.00)
Seat 5: Player013 ($107.99)
Seat 6: Player041 ($148.26)
Player031 posts ante $0.25
Player002 posts ante $0.25
Player084 posts ante $0.25
Player013 posts ante $0.25
Player041 posts ante $0.25
Player041 posts the small blind $0.25
Player031 posts the big blind $0.50
*** HOLE CARDS ***
Player002 folds
Player013 folds
Player041 folds
Uncalled bet ($0.25) returned to Player031
*** SUMMARY ***
Total pot $1.75
Seat 1: Player031 (big blind) did not show and won $1.75
Seat 2: Player002 folded on the Pre-Flop
Seat 3: Player084 did not show
Seat 5: Player013 (button) folded on the Pre-Flop
Seat 6: Player041 (small blind) folded on the Pre-Flop
`

func TestTurnOrderPlayerWithNoLogLines(t *testing.T) {
	for name, s := range map[string]string{
		"a winner who never acts (missing raise)":      sampleWinnerNeverActs,
		"first to act is missing, only a returned bet": sampleFirstToActMissing,
		"a listed player with no fold line":            sampleSilentFold,
	} {
		if v := parseSample(t, s).CheckTurnOrder(); v != SkippedSeat {
			t.Errorf("%s: %v", name, v)
		}
	}
}
