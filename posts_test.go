package hh

import (
	"strings"
	"testing"
)

func TestCheckPosts(t *testing.T) {
	for name, tc := range map[string]struct {
		sample string
		want   PostViolation
	}{
		"small blind missing, seat between dealer and big blind": {sampleMissingSmallBlind, MissingSmallBlind},
		"big blind missing":       {sampleMissingBlindPost, MissingBigBlind},
		"complete hand":           {sampleShowdown, PostsOK},
		"dead and combined posts": {samplePostsAndPartial, PostsOK},
		"sitting out":             {sampleSittingOut, PostsOK},
		"bomb pot, antes only":    {sampleRealBombPot, PostsOK},
		"ante hand, all posted":   {sampleAnteAllIn, PostsOK},
	} {
		if got := parseSample(t, tc.sample).CheckPosts(); got != tc.want {
			t.Errorf("%s: %v, want %v", name, got, tc.want)
		}
	}
}

// A hand that is not a bomb pot must have a blind; a bomb pot has none.
func TestCheckPostsNoBlinds(t *testing.T) {
	h := parseSample(t, sampleAllInPreflop)
	var kept []Action
	for _, a := range h.Actions {
		if a.Kind != PostSmallBlind && a.Kind != PostBigBlind {
			kept = append(kept, a)
		}
	}
	h.Actions = kept
	if v := h.CheckPosts(); v != NoBlinds {
		t.Errorf("a regular hand with no blinds: %v", v)
	}
	if v := parseSample(t, sampleRealBombPot).CheckPosts(); v == NoBlinds {
		t.Errorf("a bomb pot has no blinds and that is right: %v", v)
	}
}

func TestCheckPostsMissingAnte(t *testing.T) {
	s := strings.Replace(sampleAnteAllIn, "Player082 posts ante $2.50\n", "", 1)
	if got := parseSample(t, s).CheckPosts(); got != MissingAnte {
		t.Errorf("%v", got)
	}
}

// With the big blind directly next to the dealer no small blind is owed.
func TestCheckPostsNoSmallBlindOwed(t *testing.T) {
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
	if got := parseSample(t, s).CheckPosts(); got != PostsOK {
		t.Errorf("%v", got)
	}
}

// The "BombPot" line in the summary of an ordinary hand does not make it a
// bomb pot. A real hand, from Player071's file.
const sampleBombPotLabelNoAntes = `Hand #1000019 - Holdem(No Limit) - $0.25/$0.50 - 2026/01/01 00:00:00 UTC
Table17 6-max Seat #2 is the button
Seat 1: Player009 ($50.00)
Seat 2: Player001 ($67.50)
Seat 3: Player071 ($77.71)
Player071 posts the small blind $0.25
Player002 sitting out
Player003 sitting out
Player009 posts the big blind $0.50
*** HOLE CARDS ***
Player001 raises $1.25 to $1.25
Player071 folds
Player009 folds
Uncalled bet ($0.75) returned to Player001
*** SUMMARY ***
BombPot
Total pot $1.25
Seat 1: Player009 (big blind) folded on the Pre-Flop
Seat 2: Player001 (button) did not show and won $1.25
Seat 3: Player071 (small blind) folded on the Pre-Flop
`

func TestBombPotNeedsAntes(t *testing.T) {
	if parseSample(t, sampleBombPotLabelNoAntes).Type != Regular {
		t.Error("a hand with the label and no antes is not a bomb pot")
	}
	if parseSample(t, sampleRealBombPot).Type != Bomb {
		t.Error("a hand with the label and antes is a bomb pot")
	}
}

// A dead blind written as "$0.00", with the amount on a separate "posts" line,
// leaves the raise that follows short by that amount. The client
// file for the same hand has "posts dead $0.25" and "raises $1.25 to $1.25".
func TestCheckPostsLostDeadBlind(t *testing.T) {
	s := strings.Replace(sampleMissingBlindPost, "Player008 posts the small blind $0.25",
		`Player008 posts the small blind $0.25
Player052 posts dead $0.00`, 1)
	if got := parseSample(t, s).CheckPosts(); got != LostDeadBlind {
		t.Errorf("%v", got)
	}
	// A dead blind with its amount is fine.
	s = strings.Replace(sampleMissingBlindPost, "Player008 posts the small blind $0.25",
		`Player008 posts the small blind $0.25
Player052 posts dead $0.25`, 1)
	if got := parseSample(t, s).CheckPosts(); got == LostDeadBlind {
		t.Errorf("a dead blind with an amount is not lost")
	}
}

// The same walk written both ways: the site's client keeps the big blind in the
// pot and writes no returned-bet line; other files take the excess out and say so.
// The big blind wins the small blind either way.
const sampleWalkClientStyle = `Hand #1000033 - Holdem (No Limit) - $0.10/$0.25 - 2026/01/01 00:00:00 UTC
Table05 6-max Seat #1 is the button
Seat 1: Alpha ($27.35)
Seat 2: Bravo ($40.78)
Seat 3: Charlie ($57.10)
Bravo posts the small blind $0.10
Charlie posts the big blind $0.25
*** HOLE CARDS ***
Alpha folds
Bravo folds
Charlie does not show
*** SUMMARY ***
Total pot $0.35
Seat 1: Alpha (button) folded on the Pre-Flop and did not bet
Seat 2: Bravo (small blind) folded on the Pre-Flop
Seat 3: Charlie (big blind) did not show and won $0.35
`

const sampleWalkOtherStyle = `Hand #1000034 - Holdem(No Limit) - $0.10/$0.25 - 2026/01/01 00:00:00 UTC
Table05 6-max Seat #1 is the button
Seat 1: Alpha ($27.35)
Seat 2: Bravo ($40.78)
Seat 3: Charlie ($57.10)
Bravo posts the small blind $0.10
Charlie posts the big blind $0.25
*** HOLE CARDS ***
Alpha folds
Bravo folds
Uncalled bet ($0.15) returned to Charlie
*** SUMMARY ***
Total pot $0.20
Seat 1: Alpha (button) folded on the Pre-Flop and did not bet
Seat 2: Bravo (small blind) folded on the Pre-Flop
Seat 3: Charlie (big blind) did not show and won $0.20
`

// A walk that took the excess out of the pot but wrote no returned-bet
// line (the pot is $0.20 against the client's $0.35). The big blind still wins
// the small blind: $0.20 won, $0.25 posted, $0.15 of it taken back out.
const sampleWalkOtherNoLine = `Hand #1000035 - Holdem(No Limit) - $0.10/$0.25 - 2026/01/01 00:00:00 UTC
Table05 6-max Seat #1 is the button
Seat 1: Alpha ($27.35)
Seat 2: Bravo ($40.78)
Seat 3: Charlie ($57.10)
Bravo posts the small blind $0.10
Charlie posts the big blind $0.25
*** HOLE CARDS ***
Alpha folds
Bravo folds
*** SUMMARY ***
Total pot $0.20
Seat 1: Alpha (button) folded on the Pre-Flop and did not bet
Seat 2: Bravo (small blind) folded on the Pre-Flop
Seat 3: Charlie (big blind) did not show and won $0.20
`

func TestWalkBothConventions(t *testing.T) {
	for name, tc := range map[string]struct {
		sample string
	}{
		"client style":                           {sampleWalkClientStyle},
		"excess taken out":                       {sampleWalkOtherStyle},
		"excess taken out, no returned-bet line": {sampleWalkOtherNoLine},
	} {
		h := parseSample(t, tc.sample)
		if !h.ChipsConsistent() || h.CheckReturn() != ReturnOK {
			t.Errorf("%s: should be consistent", name)
		}
		v, err := h.PlayerHandFor("Charlie")
		if err != nil {
			t.Fatal(err)
		}
		if v.Net() != 10 {
			t.Errorf("%s: the big blind wins the small blind, net %d", name, v.Net())
		}
	}
}

// A real hand from Player071's file. Only Player071 and Player036 are dealt in, so
// the dealer posts the small blind heads-up. Here the dealer, on seat 5, posts
// the big blind and Player071 posts the small blind.
const sampleButtonPostsBigBlind = `Hand #1000008 - Holdem(No Limit) - $0.25/$0.50 - 2026/01/01 00:00:00 UTC
Table27 6-max Seat #5 is the button
Seat 1: Player082 ($50.00) is sitting out
Seat 2: Player071 ($192.44)
Seat 3: Player032 ($81.98)
Seat 4: Player040 ($50.00)
Seat 5: Player036 ($50.00)
Seat 6: Player058 ($55.36)
Player071 posts the small blind $0.25
Player036 posts the big blind $0.50
*** HOLE CARDS ***
Player071 raises $1.25 to $1.50
Player036 folds
Uncalled bet ($1.00) returned to Player071
*** SUMMARY ***
Total pot $1.00
Seat 2: Player071 did not show and won $1.00
Seat 5: Player036 (big blind) folded on the Pre-Flop
`

// A real hand from the client's file. Player042, due the big blind, sits out, so
// the blind moves on to the dealer. That is allowed.
const sampleSitOutMovesBlind = `Hand #1000012 - Holdem (No Limit) - $0.10/$0.25 - 2026/01/01 00:00:00 UTC
Table08 6-max Seat #5 is the button
Seat 2: Player042 ($26.10)
Seat 4: Player003 ($51.32) is sitting out
Seat 5: seanq222 ($46.42)
Seat 6: Player021 ($27.92)
Player021 posts the small blind $0.10
Player042 sits out
seanq222 posts the big blind $0.25
*** HOLE CARDS ***
Dealt to seanq222 [3h As]
Player021 folds
seanq222 does not show
*** SUMMARY ***
Total pot $0.35
Seat 5: seanq222 did not show and won $0.35
Seat 6: Player021 (small blind) folded on the Pre-Flop
`

func TestCheckPostsWrongBlindSeat(t *testing.T) {
	if got := parseSample(t, sampleButtonPostsBigBlind).CheckPosts(); got != WrongBlindSeat {
		t.Errorf("the dealer posting the big blind heads-up: %v", got)
	}
	if got := parseSample(t, sampleSitOutMovesBlind).CheckPosts(); got != PostsOK {
		t.Errorf("a player who sits out moves the blind on: %v", got)
	}
	// The ordinary samples have the right seats posting.
	for name, s := range map[string]string{"showdown": sampleShowdown, "bomb pot": sampleRealBombPot, "walk": sampleWalkClientStyle} {
		if got := parseSample(t, s).CheckPosts(); got != PostsOK {
			t.Errorf("%s: %v", name, got)
		}
	}
}
