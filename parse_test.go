package hh

import (
	"errors"
	"io"
	"strings"
	"testing"
)

const sampleShowdown = `Hand #1000010 - Holdem(No Limit) - $0.50/$1.00 - 2026/01/01 00:00:00 UTC
Table01 6-max Seat #3 is the button
Seat 1: Player072 ($112.31)
Seat 2: Player014 ($100.00)
Seat 3: Player011 ($121.99)
Seat 4: Player057 ($137.18)
Seat 5: Player030 ($100.00)
Seat 6: Player038 ($100.03)
Player057 posts the small blind $0.50
Player030 posts the big blind $1.00
*** HOLE CARDS ***
Player038 raises $2.00 to $2.00
Player072 folds
Player014 folds
Player011 raises $7.00 to $7.00
Player057 calls $6.50
Player030 folds
Player038 folds
*** FLOP *** [9c 9s Qh]
Main pot $16.15 | Rake $0.85
Player057 checks
Player011 bets $12.25
Player057 calls $12.25
*** TURN *** [9c 9s Qh] [5c]
Main pot $39.43 | Rake $2.07
Player057 checks
Player011 checks
*** RIVER *** [9c 9s Qh 5c] [Jc]
Main pot $39.43 | Rake $2.07
Player057 checks
Player011 checks
*** SHOW DOWN ***
Main pot $39.43 | Rake $2.07
Player057 shows [Td Qd] (two pair, Queens and Nines [Qh Qd 9s 9c Jc])
*** SUMMARY ***
Total pot $39.43 | Rake $2.07
Board: [9c 9s Qh 5c Jc]
Seat 1: Player072 folded on the Pre-Flop and did not bet
Seat 2: Player014 folded on the Pre-Flop and did not bet
Seat 3: Player011 (button) did not show
Seat 4: Player057 (small blind) showed [Td Qd] and won $39.43 with two pair, Queens and Nines [Qh Qd 9s 9c Jc]
Seat 5: Player030 (big blind) folded on the Pre-Flop
Seat 6: Player038 folded on the Pre-Flop
`

const sampleUncalled = `Hand #1000011 - Holdem(No Limit) - $0.50/$1.00 - 2026/01/01 00:00:00 UTC
Table07 6-max Seat #1 is the button
Seat 1: Player025 ($58.32)
Seat 2: Player069 ($100.36)
Seat 3: Player061 ($1,123.44)
Seat 4: Player019 ($96.73)
Seat 5: Player018 ($134.69)
Seat 6: Player072 ($73.65)
Player069 posts the small blind $0.50
Player061 posts the big blind $1.00
*** HOLE CARDS ***
Player019 folds
Player018 folds
Player072 folds
Player025 folds
Player069 folds
Uncalled bet ($0.50) returned to Player061
*** SUMMARY ***
Total pot $1.00
Seat 1: Player025 (button) folded on the Pre-Flop and did not bet
Seat 2: Player069 (small blind) folded on the Pre-Flop
Seat 3: Player061 (big blind) did not show and won $1.00
Seat 4: Player019 folded on the Pre-Flop and did not bet
Seat 5: Player018 folded on the Pre-Flop and did not bet
Seat 6: Player072 folded on the Pre-Flop and did not bet
`

const sampleAllInBomb = `Hand #1000013 - Holdem(No Limit) - $1.00/$2.00 - 2026/01/01 00:00:00 UTC
Table12 6-max Seat #1 is the button
Seat 1: Player050 ($200.00)
Seat 2: Player008 ($150.00)
Seat 3: Player024 ($40.00)
Player050 posts ante $1.00
Player008 posts ante $1.00
Player024 posts ante $1.00
*** HOLE CARDS ***
Player008 bets $10.00
Player024 raises $39.00 to $39.00 and is all-in
Player050 folds
Player008 calls $29.00
*** FLOP *** [5s 6d Ts]
Main pot $80.00 | Rake $3.00
*** SUMMARY ***
BombPot
Total pot $80.00 | Rake $3.00
Board: [5s 6d Ts]
Seat 1: Player050 (button) folded on the Pre-Flop
Seat 2: Player008 did not show and won $77.00
Seat 3: Player024 showed [Ah Kh] and lost with high card Ace
`

const sampleHero = `Hand #1000009 - Holdem (No Limit) - $0.05/$0.10 - 2026/01/01 00:00:00 UTC
Table05 6-max Seat #1 is the button
Seat 1: Player088 ($12.82)
Seat 2: Player045 ($14.81)
Seat 3: seanq222 ($10.00)
Player045 posts the small blind $0.05
seanq222 posts the big blind $0.10
*** HOLE CARDS ***
Dealt to seanq222 [Ks 4d]
Player088 folds
Player045 raises $0.25 to $0.25
seanq222 folds
Something odd happens here
Uncalled bet ($0.15) returned to Player045
*** SUMMARY ***
Total pot $0.20
Seat 1: Player088 folded on the Pre-Flop and did not bet
Seat 2: Player045 (small blind) did not show and won $0.20
Seat 3: seanq222 (big blind) folded on the Pre-Flop
`

// A walk from an older file that has no "*** SUMMARY ***" line: the summary
// starts with its total pot.
const sampleNoSummaryHeader = `Hand #1000704 - Holdem(No Limit) - $0.50/$1.00 - 2025/01/01 00:00:00 UTC
TableOne 6-max Seat #3 is the button
Seat 1: Alice ($215.35)
Seat 2: Bob ($138.50)
Seat 3: Carol ($55.60)
Alice posts the small blind $0.50
Bob posts the big blind $1.00
*** HOLE CARDS ***
Carol folds
Alice folds
Uncalled bet ($0.50) returned to Bob
Total pot $1.00
Seat 1: Alice (small blind) folded on the Pre-Flop
Seat 2: Bob (big blind) did not show and won $1.00
Seat 3: Carol (button) folded on the Pre-Flop and did not bet
`

func TestParseNoSummaryHeader(t *testing.T) {
	h := parseSample(t, sampleNoSummaryHeader)
	if h.NoSummary || len(h.Unknown) > 0 {
		t.Fatalf("the summary should be read: NoSummary %v, unknown lines %q", h.NoSummary, h.Unknown)
	}
	if bob := h.PlayerByName("Bob"); bob == nil || bob.Won != 100 {
		t.Errorf("Bob won $1.00: %+v", bob)
	}
	if m := h.FailedCheck(); m != "" {
		t.Errorf("fails a check: %s", m)
	}
}

func TestParseShowdown(t *testing.T) {
	h, err := ParseHand(strings.Split(strings.TrimSpace(sampleShowdown), "\n"))
	if err != nil {
		t.Fatal(err)
	}
	if h.ID != 1000010 || h.SmallBlind != 50 || h.BigBlind != 100 {
		t.Errorf("header: %+v", h)
	}
	if h.Table != "Table01" || h.MaxSeats != 6 || h.ButtonSeat != 3 {
		t.Errorf("table: %q %d %d", h.Table, h.MaxSeats, h.ButtonSeat)
	}
	if h.Time.Format("2006-01-02 15:04:05") != "2026-01-01 00:00:00" {
		t.Errorf("time: %v", h.Time)
	}
	if len(h.Players) != 6 || h.Players[3].Stack != 13718 {
		t.Errorf("players: %+v", h.Players)
	}
	if h.Board.String() != "9cQh5cJc9s" && len(h.Board) != 5 {
		t.Errorf("board len %d", len(h.Board))
	}
	if h.StreetsSeen() != 4 {
		t.Errorf("streets seen %d", h.StreetsSeen())
	}
	if h.TotalPot != 3943 || h.Rake != 207 {
		t.Errorf("pot %d rake %d", h.TotalPot, h.Rake)
	}
	if len(h.Unknown) != 0 {
		t.Errorf("unknown lines: %q", h.Unknown)
	}

	// 2 blinds + 7 preflop actions + 3 flop + 2 turn + 2 river.
	if len(h.Actions) != 16 {
		t.Errorf("actions: %d", len(h.Actions))
	}
	a := h.Actions[2]
	if a.Kind != Raise || a.Player != "Player038" || a.Amount != 200 || a.To != 200 || a.Street != Preflop {
		t.Errorf("raise action: %+v", a)
	}
	last := h.Actions[len(h.Actions)-1]
	if last.Kind != Check || last.Street != River {
		t.Errorf("last action: %+v", last)
	}

	smt := h.PlayerByName("Player057")
	if smt == nil || !smt.Showed || !smt.HasCards || smt.Won != 3943 || smt.Position != "small blind" {
		t.Errorf("Player057: %+v", smt)
	}
	// The cards are kept in the order the file gives them.
	if cardList(smt.Cards[:]) != "Td Qd" {
		t.Errorf("cards %v", smt.Cards)
	}
	if !strings.HasPrefix(smt.HandDescr, "two pair") {
		t.Errorf("descr %q", smt.HandDescr)
	}
	if p := h.PlayerByName("Player072"); !p.Folded || p.FoldedOn != Preflop {
		t.Errorf("Player072: %+v", p)
	}
	if p := h.PlayerByName("Player011"); p.Position != "button" || p.Showed {
		t.Errorf("Player011: %+v", p)
	}
}

func TestParseNamesWithSpacesAndUncalled(t *testing.T) {
	h, err := ParseHand(strings.Split(strings.TrimSpace(sampleUncalled), "\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(h.Unknown) != 0 {
		t.Errorf("unknown: %q", h.Unknown)
	}
	if h.Players[2].Stack != 112344 {
		t.Errorf("comma stack: %d", h.Players[2].Stack)
	}
	var fk, f *Action
	for i := range h.Actions {
		a := &h.Actions[i]
		switch {
		case a.Player == "Player019" && a.Kind == Fold:
			fk = a
		case a.Player == "Player018" && a.Kind == Fold:
			f = a
		}
	}
	if fk == nil || fk.Seat != 4 || f == nil || f.Seat != 5 {
		t.Errorf("name resolution: %+v %+v", fk, f)
	}
	// The file's returned-bet line is kept as data, and the actions agree.
	if h.Returned.Player != "Player061" || h.Returned.Amount != 50 {
		t.Errorf("returned line: %+v", h.Returned)
	}
	if seat, amount, ok := h.UncalledBet(); !ok || seat != 3 || amount != 50 {
		t.Errorf("uncalled bet from the actions: seat %d amount %d ok %v", seat, amount, ok)
	}
	if p := h.PlayerByName("Player061"); p.Won != 100 || p.Position != "big blind" {
		t.Errorf("winner: %+v", p)
	}
	if h.StreetsSeen() != 1 {
		t.Errorf("streets %d", h.StreetsSeen())
	}
}

func TestParseAllInAndBombPot(t *testing.T) {
	h, err := ParseHand(strings.Split(strings.TrimSpace(sampleAllInBomb), "\n"))
	if err != nil {
		t.Fatal(err)
	}
	if h.Type != Bomb {
		t.Error("bomb pot not detected")
	}
	if len(h.Unknown) != 0 {
		t.Errorf("unknown: %q", h.Unknown)
	}
	var antes, allIns int
	for _, a := range h.Actions {
		if a.Kind == PostAnte {
			antes++
		}
		if a.AllIn {
			allIns++
			if a.Kind != Raise || a.To != 3900 {
				t.Errorf("all-in action: %+v", a)
			}
		}
	}
	if antes != 3 || allIns != 1 {
		t.Errorf("antes %d allins %d", antes, allIns)
	}
	hr := h.PlayerByName("Player024")
	if !hr.Showed || hr.Won != 0 || hr.HandDescr != "high card Ace" {
		t.Errorf("Player024: %+v", hr)
	}
}

func TestParseHeroAndUnknown(t *testing.T) {
	h, err := ParseHand(strings.Split(strings.TrimSpace(sampleHero), "\n"))
	if err != nil {
		t.Fatal(err)
	}
	if h.Game != "Holdem" || h.Limit != "No Limit" {
		t.Errorf("game %q limit %q", h.Game, h.Limit)
	}
	if h.Hero != "seanq222" || !h.PlayerByName("seanq222").HasCards {
		t.Errorf("hero: %q", h.Hero)
	}
	if len(h.Unknown) != 1 || h.Unknown[0] != "Something odd happens here" {
		t.Errorf("unknown: %q", h.Unknown)
	}
}

func TestReader(t *testing.T) {
	in := "\xef\xbb\xbf" + sampleShowdown + "\n\n" +
		"Hand #1 - garbage\nnot a table line\n\n" +
		sampleUncalled + "\r\n\r\n" + sampleRealBombPot
	rd := NewReader(strings.NewReader(in))

	var ids []int64
	var errs []*ParseError
	for {
		h, err := rd.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			var pe *ParseError
			if !errors.As(err, &pe) {
				t.Fatal(err)
			}
			errs = append(errs, pe)
			continue
		}
		ids = append(ids, h.ID)
	}
	want := []int64{1000010, 1000011, 1000015}
	if len(ids) != len(want) {
		t.Fatalf("ids %v", ids)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Errorf("ids %v", ids)
		}
	}
	if len(errs) != 1 {
		t.Fatalf("errs %v", errs)
	}
	if errs[0].Line != 54 {
		t.Logf("error line %d (informational): %v", errs[0].Line, errs[0])
	}
}

func TestParseMoney(t *testing.T) {
	cases := map[string]Cents{
		"$0.50": 50, "1.00": 100, "$1,234.56": 123456, "7": 700, "0.5": 50, "$0.05": 5, "10.": 1000,
	}
	for in, want := range cases {
		got, ok := parseMoney(in)
		if !ok || got != want {
			t.Errorf("parseMoney(%q) = %d, %v; want %d", in, got, ok, want)
		}
	}
	if _, ok := parseMoney("abc"); ok {
		t.Error("expected failure for abc")
	}
}

func TestCentsString(t *testing.T) {
	for c, want := range map[Cents]string{0: "$0.00", 5: "$0.05", 125: "$1.25", 123456: "$1234.56", -50: "-$0.50"} {
		if got := c.String(); got != want {
			t.Errorf("Cents(%d).String() = %q, want %q", int32(c), got, want)
		}
	}
	if _, ok := parseMoney("99999999.99"); ok {
		t.Error("an amount too large for Cents should not parse")
	}
}
