package hh

import (
	"strings"
	"testing"
)

const sampleSittingOut = `Hand #1000023 - Holdem(No Limit) - $1.00/$2.00 - 2026/01/01 00:00:00 UTC
Table11 6-max Seat #5 is the button
Seat 1: Player035 ($31.16) is sitting out
Seat 2: Player060 ($522.75)
Seat 4: Player046 ($213.06)
Seat 5: Player056 ($198.00)
Seat 6: Player033 ($262.86)
Player033 posts the small blind $1.00
Player001 sitting out
Player060 posts the big blind $2.00
*** HOLE CARDS ***
Player046 raises $4.40 to $4.40
Player056 folds
Player033 folds
Player060 folds
Uncalled bet ($2.40) returned to Player046
*** SUMMARY ***
Total pot $5.00
Seat 2: Player060 (big blind) folded on the Pre-Flop
Seat 4: Player046 did not show and won $5.00
Seat 5: Player056 (button) folded on the Pre-Flop and did not bet
Seat 6: Player033 (small blind) folded on the Pre-Flop
`

const samplePostsAndPartial = `Hand #1000024 - Holdem(No Limit) - $1.00/$2.00 - 2026/01/01 00:00:00 UTC
Table20 6-max Seat #3 is the button
Seat 1: Player034 ($204.50)
Seat 2: Player049 ($306.26)
Seat 3: Player016 ($199.00)
Seat 4: Player006 ($208.58)
Seat 5: Player012 ($202.00)
Player006 posts the small blind $1.00
Player012 posts the big blind $2.00
Player049 posts dead $0.50
Player049 posts $1.00
*** HOLE CARDS ***
Player034 folds
Player049 raises $3.00 to $4.00
Player016 folds
Player006 folds
Player012 calls $2.00
*** FLOP *** [5s 5d Ac]
Main pot $9.50 | Rake $0.50
Player012 checks
Player049 checks
*** TURN *** [5s 5d Ac] [2c]
Main pot $9.50 | Rake $0.50
Player012 checks
Player049 checks
*** RIVER *** [5s 5d Ac 2c] [Jd]
Main pot $9.50 | Rake $0.50
Player012 checks
Player049 checks
*** SHOW DOWN ***
Player012 shows [- Jc]
*** SUMMARY ***
Total pot $9.50 | Rake $0.50
Board: [5s 5d Ac 2c Jd]
Seat 1: Player034 folded on the Pre-Flop and did not bet
Seat 2: Player049 did not show
Seat 3: Player016 (button) folded on the Pre-Flop and did not bet
Seat 4: Player006 (small blind) folded on the Pre-Flop
Seat 5: Player012 (big blind) showed [- Jc] and won $9.50 with a pair of Jacks
`

func TestSittingOut(t *testing.T) {
	h, err := ParseHand(strings.Split(strings.TrimSpace(sampleSittingOut), "\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(h.Unknown) != 0 {
		t.Errorf("unknown: %q", h.Unknown)
	}
	if len(h.Players) != 5 || !h.Players[0].SittingOut || h.Players[1].SittingOut {
		t.Errorf("players: %+v", h.Players)
	}
	if p := h.PlayerByName("Player046"); p.Won != 500 {
		t.Errorf("winner: %+v", p)
	}
}

func TestDeadPostsAndPartialShow(t *testing.T) {
	h, err := ParseHand(strings.Split(strings.TrimSpace(samplePostsAndPartial), "\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(h.Unknown) != 0 {
		t.Errorf("unknown: %q", h.Unknown)
	}
	var dead, other *Action
	for i := range h.Actions {
		switch h.Actions[i].Kind {
		case PostDead:
			dead = &h.Actions[i]
		case PostEntry:
			other = &h.Actions[i]
		}
	}
	if dead == nil || dead.Amount != 50 || other == nil || other.Amount != 100 {
		t.Errorf("posts: %+v %+v", dead, other)
	}
	p := h.PlayerByName("Player012")
	if !p.Showed || p.HasCards || !p.HasPartial || p.PartialCard.String() != "Jc" {
		t.Errorf("Player012: %+v", p)
	}
	if p.Won != 950 {
		t.Errorf("won %d", p.Won)
	}
	if len(h.Board) != 5 {
		t.Errorf("board %v", h.Board)
	}
}

const sampleMarkerNoShowdown = `Hand #1000007 - Holdem(No Limit) - $0.25/$0.50 - 2026/01/01 00:00:00 UTC
Table27 6-max Seat #1 is the button
Seat 1: Player011 ($50.50)
Seat 2: Player020 ($47.42)
Seat 3: Player004 ($40.00)
Player020 posts the small blind $0.25
Player004 posts the big blind $0.50
*** HOLE CARDS ***
Player011 raises $1.25 to $1.25
Player020 folds
Player004 calls $0.75
*** FLOP *** [Ah 3s 9d]
Main pot $2.62 | Rake $0.13
Player004 checks
Player011 bets $3.75
Player004 folds
Uncalled bet ($3.75) returned to Player011
*** SHOW DOWN ***
Main pot $2.62 | Rake $0.13
Player011 collected $2.62 from main pot
*** SUMMARY ***
Total pot $2.62 | Rake $0.13 | JP Fee $0.00
Board [Ah 3s 9d]
Seat 1: Player011 () did not show and won $2.62
Seat 3: Player004 () folded on the Flop
`

func TestShowdownMarkerWithoutShowdown(t *testing.T) {
	h, err := ParseHand(strings.Split(strings.TrimSpace(sampleMarkerNoShowdown), "\n"))
	if err != nil {
		t.Fatal(err)
	}
	// The parser keeps the marker the file prints. A check finds that nobody
	// reached a showdown, and Repair clears it.
	if !h.Showdown || h.ShowdownMatches() || h.FailedCheck() == "" {
		t.Error("the marker is there, but everyone folded to a bet")
	}
	if n := h.Repair(); n == 0 || h.Showdown {
		t.Errorf("repaired %d, Showdown %v: everyone folded to a bet, so this is not a showdown", n, h.Showdown)
	}
	h2, err := ParseHand(strings.Split(strings.TrimSpace(sampleShowdown), "\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !h2.Showdown {
		t.Error("two players reached showdown")
	}
}

// Shown cards are kept in the order the file writes them, and written back so.
func TestShownCardsKeepTheirOrder(t *testing.T) {
	// A partly shown hand with the hidden card second.
	s := strings.ReplaceAll(strings.TrimSpace(samplePostsAndPartial), "[- Jc]", "[Jc -]")
	h, err := ParseHand(strings.Split(s, "\n"))
	if err != nil {
		t.Fatal(err)
	}
	if p := h.PlayerByName("Player012"); !p.HasPartial || !p.PartialFirst || p.PartialCard.String() != "Jc" {
		t.Errorf("partial: %+v", p)
	}
	if out := h.String(); !strings.Contains(out, "shows [Jc -]") || !strings.Contains(out, "showed [Jc -]") {
		t.Errorf("the hidden card moved:\n%s", out)
	}
	// Two shown cards, the higher first.
	s = strings.ReplaceAll(strings.TrimSpace(sampleShowdown), "[Td Qd]", "[Qd Td]")
	h = parseSample(t, s)
	if p := h.PlayerByName("Player057"); cardList(p.Cards[:]) != "Qd Td" {
		t.Errorf("cards %s", cardList(p.Cards[:]))
	}
	if out := h.String(); !strings.Contains(out, "shows [Qd Td]") {
		t.Errorf("the cards were reordered:\n%s", out)
	}
	// The view built for analysis holds them in the library's order.
	views, err := h.PlayerHands()
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range views {
		for _, pl := range v.Players {
			if pl.HasCards && pl.Cards[0] > pl.Cards[1] {
				t.Errorf("hole cards not in order: %v", pl.Cards)
			}
		}
	}
}

// Two pair of Queens, and the winner's Ace beats the other's Jack. The client adds
// the kicker to the winner's summary line only.
const sampleKicker = `Hand #1000500 - Holdem (No Limit) - $0.25/$0.50 - 2026/01/01 00:00:00 UTC
TableOne 6-max Seat #1 is the button
Seat 1: Alice ($10.00)
Seat 2: Bob ($10.00)
Alice posts the small blind $0.25
Bob posts the big blind $0.50
*** HOLE CARDS ***
Alice calls $0.25
Bob checks
*** FLOP *** [Qc 7d 2h]
Main pot $0.95 | Rake $0.05
Bob checks
Alice checks
*** TURN *** [Qc 7d 2h] [9s]
Main pot $0.95 | Rake $0.05
Bob checks
Alice checks
*** RIVER *** [Qc 7d 2h 9s] [3c]
Main pot $0.95 | Rake $0.05
Bob checks
Alice checks
*** SHOW DOWN ***
Main pot $0.95 | Rake $0.05
Alice shows [Qd Ad] (a pair of Queens [Qc Qd Ad 9s 7d])
Bob shows [Qh Jd] (a pair of Queens [Qc Qh Jd 9s 7d])
Alice collected $0.95 from main pot
*** SUMMARY ***
Total pot $0.95 | Rake $0.05
Board [Qc 7d 2h 9s 3c]
Seat 1: Alice (button) showed [Qd Ad] and won $0.95 with a pair of Queens [Qc Qd Ad 9s 7d] with the kicker Ace
Seat 2: Bob (big blind) showed [Qh Jd] and lost with a pair of Queens [Qc Qh Jd 9s 7d]
`

func TestKickerClauseIsDerived(t *testing.T) {
	h := parseSample(t, sampleKicker)
	// The description is kept without the clause.
	if d := h.PlayerByName("Alice").HandDescr; strings.Contains(d, "kicker") {
		t.Errorf("description %q", d)
	}
	out := h.String()
	if !strings.Contains(out, "showed [Qd Ad] and won $0.95 with a pair of Queens [Qc Qd Ad 9s 7d] with the kicker Ace") {
		t.Errorf("the winner's summary line has no kicker:\n%s", out)
	}
	if strings.Contains(out, "Alice shows [Qd Ad] (a pair of Queens [Qc Qd Ad 9s 7d] with") {
		t.Errorf("the shows line has the kicker:\n%s", out)
	}
	if strings.Contains(out, "lost with a pair of Queens [Qc Qh Jd 9s 7d] with") {
		t.Errorf("the loser has a kicker:\n%s", out)
	}
	// The written text is the file's, line for line.
	if want := strings.TrimSpace(sampleKicker); strings.TrimSpace(out) != want {
		t.Errorf("not the same text:\n%s", out)
	}
}
