package hh

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The small blind's call is missing from the preflop betting, yet the flop is
// dealt and the small blind acts on it. This is a real hand from the files.
const sampleMissingCall = `Hand #1000017 - Holdem(No Limit) - $0.25/$0.50 - 2026/01/01 00:00:00 UTC
Table03 6-max Seat #6 is the button
Seat 4: Player005 ($27.95)
Seat 5: Player083 ($50.00)
Seat 6: Player029 ($51.25)
Player005 posts the small blind $0.25
Player083 posts the big blind $0.50
*** HOLE CARDS ***
Player029 raises $1.12 to $1.12
Player083 folds
*** FLOP *** [Ah 8h 6s]
Player005 checks
Player029 bets $1.37
Player005 folds
Uncalled bet ($1.37) returned to Player029
*** SUMMARY ***
Total pot $2.61 | Rake $0.13
Board: [Ah 8h 6s]
Seat 4: Player005 (small blind) folded on the Flop
Seat 5: Player083 (big blind) folded on the Pre-Flop
Seat 6: Player029 (button) did not show and won $2.61
`

// The big blind's post line is missing, but the betting is intact.
const sampleMissingBlindPost = `Hand #1000014 - Holdem(No Limit) - $0.25/$0.50 - 2026/01/01 00:00:00 UTC
Table09 6-max Seat #3 is the button
Seat 3: Player008 ($86.11)
Seat 5: Player052 ($317.45)
Player008 posts the small blind $0.25
*** HOLE CARDS ***
Player008 raises $1.25 to $1.50
Player052 folds
Uncalled bet ($1.00) returned to Player008
*** SUMMARY ***
Total pot $1.00
Seat 3: Player008 (button) did not show and won $1.00
Seat 5: Player052 folded on the Pre-Flop and did not bet
`

func TestReaderDropsMissingCall(t *testing.T) {
	in := sampleShowdown + "\n" + sampleMissingCall + "\n" + sampleMissingBlindPost + "\n" + sampleUncalled
	rd := NewReader(strings.NewReader(in))
	rd.Repair = false // these hands are meant to be dropped
	var ids []int64
	var defective []int64
	for {
		h, err := rd.Next()
		if err != nil {
			pe, ok := err.(*ParseError)
			if ok && pe.Defective {
				defective = append(defective, pe.HandID)
				continue
			}
			break
		}
		ids = append(ids, h.ID)
	}
	// A missing call and a missing blind post are both missing actions.
	if len(defective) != 2 || defective[0] != 1000017 || defective[1] != 1000014 || rd.Dropped() != 2 {
		t.Errorf("defective %v, dropped %d", defective, rd.Dropped())
	}
	want := []int64{1000010, 1000011}
	if len(ids) != len(want) {
		t.Fatalf("ids %v", ids)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Errorf("ids %v, want %v", ids, want)
		}
	}
}

func TestKeepDefective(t *testing.T) {
	rd := NewReader(strings.NewReader(sampleMissingCall))
	rd.KeepDefective = true
	h, err := rd.Next()
	if err != nil || h.ID != 1000017 || rd.Dropped() != 0 {
		t.Errorf("h %v err %v dropped %d", h, err, rd.Dropped())
	}
}

func TestParseFileStats(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hands.txt")
	// The missing call is repaired; a player who acts after the round closed is
	// not.
	in := sampleShowdown + "\n" + sampleActsAfterClose + "\n" + sampleUncalled + "\nHand #1 - garbage\nnope\n"
	if err := os.WriteFile(path, []byte(in), 0o644); err != nil {
		t.Fatal(err)
	}
	var seen int
	st, err := ParseFile(path, func(*HandHistory) error { seen++; return nil }, nil)
	if err != nil {
		t.Fatal(err)
	}
	if st.Hands != 2 || seen != 2 || st.Dropped != 1 || st.Malformed != 1 {
		t.Errorf("stats %+v seen %d", st, seen)
	}
	var sum Stats
	sum.Add(st)
	sum.Add(st)
	if sum.Hands != 4 || sum.Dropped != 2 {
		t.Errorf("sum %+v", sum)
	}
}

func TestChipsConsistentFlagsMissingPosts(t *testing.T) {
	if h := parseSample(t, sampleMissingBlindPost); h.ChipsConsistent() {
		t.Error("a missing blind post leaves the chips short")
	}
	if h := parseSample(t, sampleShowdown); !h.ChipsConsistent() {
		t.Error("a complete hand should balance")
	}
	if !parseSample(t, sampleMissingBlindPost).StreetsReplayable() {
		t.Error("a missing blind post does not break the betting")
	}
	if parseSample(t, sampleMissingCall).StreetsReplayable() {
		t.Error("a missing call breaks the betting")
	}
}

// Carol checks preflop after the round has closed, so the betting puts her
// check on the flop while the file has it before the flop.
const sampleActionAfterClose = `Hand #1000021 - Holdem(No Limit) - $0.25/$0.50 - 2026/01/01 00:00:00 UTC
TableOne 6-max Seat #6 is the button
Seat 4: Alice ($50.00)
Seat 5: Bob ($50.00)
Seat 6: Carol ($50.00)
Alice posts the small blind $0.25
Bob posts the big blind $0.50
*** HOLE CARDS ***
Carol calls $0.50
Alice calls $0.25
Bob checks
Carol checks
*** FLOP *** [Ah 8h 6s]
Alice bets $1.00
Bob folds
Carol folds
Uncalled bet ($1.00) returned to Alice
*** SUMMARY ***
Total pot $1.50
Board: [Ah 8h 6s]
Seat 4: Alice (small blind) did not show and won $1.50
Seat 5: Bob (big blind) folded on the Flop
Seat 6: Carol (button) folded on the Flop
`

// StreetsReplayable replays the actions itself; it must agree with
// PlayerHands, which refuses the same hands with ErrInconsistent.
func TestStreetsReplayableAgreesWithPlayerHands(t *testing.T) {
	closed := strings.Replace(sampleActionAfterClose, "Bob checks\nCarol checks\n", "Bob checks\n", 1)
	for _, c := range []struct {
		name, text string
		want       bool
	}{
		{"showdown", sampleShowdown, true},
		{"uncalled", sampleUncalled, true},
		{"missing blind post", sampleMissingBlindPost, true},
		{"missing call", sampleMissingCall, false},
		{"action after the round closed", sampleActionAfterClose, false},
		{"round closed in time", closed, true},
	} {
		h := parseSample(t, c.text)
		if got := h.StreetsReplayable(); got != c.want {
			t.Errorf("%s: StreetsReplayable %v, want %v", c.name, got, c.want)
		}
		_, err := h.PlayerHands()
		if old := !errors.Is(err, ErrInconsistent); old != c.want {
			t.Errorf("%s: PlayerHands says replayable %v, want %v", c.name, old, c.want)
		}
	}
}

// A hand cut off before its summary has no winner, so it is dropped.
func TestTruncatedHandIsDropped(t *testing.T) {
	cut := sampleShowdown[:strings.Index(sampleShowdown, "*** SUMMARY ***")]
	_, err := ParseHand(strings.Split(strings.TrimSpace(cut), "\n"))
	var pe *ParseError
	if !errors.As(err, &pe) || !pe.Defective {
		t.Errorf("%v, want a defective hand", err)
	}
}

// The total pot line is the pot at the start of the river, but the river was an
// all-in call and the winner was paid the real pot. The actions are right.
const sampleStaleTotalPot = `Hand #1000028 - Holdem(No Limit) - $0.25/$0.50 - 2026/01/01 00:00:00 UTC
Table04 6-max Seat #2 is the button
Seat 1: Player007 ($92.47)
Seat 2: Player081 ($59.39)
Seat 3: Player065 ($50.02)
Seat 4: Player051 ($50.00)
Seat 5: Player071 ($150.02)
Seat 6: Player017 ($83.32)
Player065 posts the small blind $0.25
Player051 posts the big blind $0.50
*** HOLE CARDS ***
Player071 raises $1.25 to $1.25
Player017 folds
Player007 raises $4.00 to $4.00
Player081 folds
Player065 folds
Player051 folds
Player071 raises $9.75 to $11.00
Player007 calls $7.00
*** FLOP *** [4d Qs Ac]
Main pot $21.62 | Rake $1.13
Player071 bets $5.68
Player007 calls $5.68
*** TURN *** [4d Qs Ac] [Qc]
Main pot $32.41 | Rake $1.70
Player071 bets $17.73
Player007 calls $17.73
*** RIVER *** [4d Qs Ac Qc] [5d]
Main pot $66.57 | Rake $3.00
Player071 bets $115.61 and is all-in
Player007 calls $58.06 and is all-in
*** SHOW DOWN ***
Main pot $66.57 | Rake $3.00
Player007 shows [Ks As] (two pair, Aces and Queens [As Ac Qs Qc Ks])
Player071 shows [Ah Ad] (a full house (Ace/Queen) [Ah Ad Ac Qs Qc])
*** SUMMARY ***
Total pot $66.57 | Rake $3.00
Board: [4d Qs Ac Qc 5d]
Seat 1: Player007 showed [Ks As] and lost with two pair, Aces and Queens [As Ac Qs Qc Ks]
Seat 2: Player081 (button) folded on the Pre-Flop and did not bet
Seat 3: Player065 (small blind) folded on the Pre-Flop
Seat 4: Player051 (big blind) folded on the Pre-Flop
Seat 5: Player071 showed [Ah Ad] and won $182.69 with a full house (Ace/Queen) [Ah Ad Ac Qs Qc]
Seat 6: Player017 folded on the Pre-Flop and did not bet
`

func TestStaleTotalPotLine(t *testing.T) {
	h := parseSample(t, sampleStaleTotalPot)
	if !h.ChipsConsistent() {
		t.Error("the winners' payouts match the actions, so the hand is sound")
	}
	// A missing call really is a defect: the same hand without Booty's river call.
	cut := strings.Replace(sampleStaleTotalPot, "Player007 calls $58.06 and is all-in\n", "", 1)
	if parseSample(t, cut).ChipsConsistent() {
		t.Error("a missing call should still fail")
	}
}

// The winner's summary line is missing: chips went in and nobody is credited
// with winning them.
const sampleNoWinnerLine = `Hand #1000022 - Holdem (No Limit) - $0.25/$0.5 - 2026/01/01 00:00:00 UTC
TableOne 6-max Seat #2 is the button
Seat 1: Alice ($50)
Seat 2: Bob ($50)
Seat 3: Carol ($46.09)
Seat 4: Dave ($52.8)
Seat 5: Erin ($50)
Seat 6: Frank ($49)
Carol posts the small blind $0.25
Dave posts the big blind $0.5
Alice posts $0.5
*** HOLE CARDS ***
Erin folds
Frank raises $2.5 to $2.5
Alice folds
Bob folds
Carol folds
Dave folds
Uncalled bet ($2) returned to Frank
Frank does not show
*** SUMMARY ***
Total pot $1.75 | Rake $0
Seat 1: Alice folded on the Pre-Flop
Seat 2: Bob folded on the Pre-Flop and did not bet
Seat 3: Carol (small blind) folded on the Pre-Flop
Seat 4: Dave (big blind) folded on the Pre-Flop
Seat 5: Erin folded on the Pre-Flop and did not bet
`

func TestWinnerPaid(t *testing.T) {
	h := parseSample(t, sampleNoWinnerLine)
	if h.WinnerPaid() || h.FailedCheck() == "" {
		t.Errorf("no winner: WinnerPaid %v, check %q", h.WinnerPaid(), h.FailedCheck())
	}
	with := parseSample(t, sampleNoWinnerLine+"Seat 6: Frank did not show and won $1.75\n")
	if !with.WinnerPaid() || with.FailedCheck() != "" {
		t.Errorf("with the winner's line: WinnerPaid %v, check %q", with.WinnerPaid(), with.FailedCheck())
	}
	// Only the big blind posts and everyone folds: the blind goes back, the pot
	// is empty, and nobody needs to have won.
	empty := parseSample(t, `Hand #1000023 - Holdem(No Limit) - $0.50/$1.00 - 2026/01/01 00:00:00 UTC
TableOne 6-max Seat #1 is the button
Seat 1: Alice ($136.88)
Seat 4: Bob ($115.98)
Seat 5: Carol ($102.13)
Bob posts the big blind $1.00
*** HOLE CARDS ***
Carol folds
Alice folds
*** SUMMARY ***
Total pot $0.00
Seat 1: Alice (button) folded on the Pre-Flop and did not bet
Seat 4: Bob (big blind) did not show and won $0.00
Seat 5: Carol folded on the Pre-Flop and did not bet
`)
	if !empty.WinnerPaid() {
		t.Error("a big blind's own blind going back needs no winner")
	}
}
