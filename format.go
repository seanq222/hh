package hh

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/seanq222/hh/pk"
)

// String writes the hand in the site client's text format. It is built from the
// parsed fields only, so it is a check on the parser: a file that parsed without
// losing anything reads back as the same text, apart from the lines the model
// does not keep (see below).
//
// The "Main pot | Rake" line after each street marker and after the showdown
// marker is written again, with the pot worked out from the actions.
//
// Not reproduced, because the model has no place for them: "Side pot" lines,
// "NAME collected" and "NAME does not show" in the showdown, "sitting out" and
// "waiting for the button" lines, and the order the showdown shows cards in.
func (h *HandHistory) String() string {
	if h.Site == CoinPoker {
		return h.coinPokerString()
	}
	var b strings.Builder
	w := func(format string, args ...any) {
		fmt.Fprintf(&b, format, args...)
		b.WriteByte('\n')
	}
	// The client wrote "Holdem(No Limit)" until the afternoon of 2023-11-02, and
	// "Holdem (No Limit)" since. The files have no hand between the two forms in the
	// hours in between.
	space := " "
	if h.Time.Before(time.Date(2023, 11, 2, 12, 0, 0, 0, time.UTC)) {
		space = ""
	}
	w("Hand #%d - %s%s(%s) - %s/%s - %s UTC", h.ID, h.Game, space, h.Limit, h.SmallBlind, h.BigBlind,
		h.Time.Format("2006/01/02 15:04:05"))
	w("%s %d-max Seat #%d is the button", h.Table, h.MaxSeats, h.ButtonSeat)
	// The seats in order, with the players waiting for the button among them.
	waiting := slices.Clone(h.Waiting)
	slices.SortStableFunc(waiting, func(a, b WaitingSeat) int { return a.Seat - b.Seat })
	wi := 0
	for i := range h.Players {
		p := &h.Players[i]
		for wi < len(waiting) && waiting[wi].Seat < p.Seat {
			w("%s", waiting[wi].Text)
			wi++
		}
		out := ""
		if p.SittingOut {
			out = " is sitting out"
		}
		w("Seat %d: %s (%s)%s", p.Seat, p.Name, p.Stack, out)
	}
	for ; wi < len(waiting); wi++ {
		w("%s", waiting[wi].Text)
	}
	// Lines that say a player sits out, written where the file had them.
	sitOut := func(at int, afterHole bool) {
		for _, l := range h.SitOutLines {
			if l.AfterHole == afterHole && l.At == at {
				w("%s", l.Text)
			}
		}
	}

	// Posts come before the hole cards; everything else after.
	i := 0
	for ; i < len(h.Actions) && h.Actions[i].Street == Preflop && isPost(h.Actions[i].Kind); i++ {
		sitOut(i, false)
		w("%s", h.actionText(i))
	}
	sitOut(i, false)
	w("*** HOLE CARDS ***")
	if h.Type == Bomb {
		// A bomb pot has no betting before the flop: the client shows the pot the
		// antes made, with no rake.
		var antes Cents
		for i := range h.Actions {
			if h.Actions[i].Street == Preflop {
				antes += h.Actions[i].Amount
			}
		}
		w("Main pot %s | Rake %s", antes-h.Rakes[Preflop], h.Rakes[Preflop])
	}
	if h.Hero != "" {
		if p := h.PlayerByName(h.Hero); p != nil && p.HasCards {
			w("Dealt to %s [%s]", p.Name, cardList(p.Cards[:]))
		}
	}
	// The returned bet is written when the betting on its street ends.
	returned := h.Returned.Amount == 0
	endStreet := func(next Street) {
		if !returned && next > h.Returned.Street {
			returned = true
			w("Uncalled bet (%s) returned to %s", h.Returned.Amount, h.Returned.Player)
		}
	}
	street := Preflop
	for ; i < len(h.Actions); i++ {
		for street < h.Actions[i].Street {
			street++
			endStreet(street)
			h.streetMarker(w, street)
		}
		sitOut(i, true)
		w("%s", h.actionText(i))
	}
	sitOut(len(h.Actions), true)
	last := River
	shared := 0 // board cards both runs have in common
	if h.RunTwice {
		for shared < min(len(h.Board), len(h.Board2), 4) && h.Board[shared] == h.Board2[shared] {
			shared++
		}
		// The streets already dealt when the board was run are written once.
		for last = River; last > Preflop && cardsAtStreet[last] > shared; last-- {
		}
	}
	for street < last && len(h.Board) >= cardsAtStreet[street+1] {
		street++
		endStreet(street)
		h.streetMarker(w, street)
	}
	if !h.Showdown && !h.RunTwice {
		h.writeShows(w, true, 0) // a player who folds and shows does so as the betting ends
	}
	endStreet(River + 1)
	if h.RunTwice {
		w("*** RUN IT TWICE ***")
		h.writeRun(w, 1, h.Board, shared)
		h.writeRun(w, 2, h.Board2, shared)
	} else {
		if h.Showdown {
			w("*** SHOW DOWN ***")
			if h.TotalPot > 0 {
				rake := h.Rake + h.JPFee
				if pots := h.potsBefore(River+1, rake, h.TotalPot+rake); pots != nil {
					for i, p := range pots {
						w("%s %s | Rake %s", potName(i), p.net(), p.rake)
					}
				} else {
					w("Main pot %s | Rake %s", h.TotalPot, rake)
				}
			}
		}
		h.writeShows(w, false, 0)
		if h.Showdown {
			h.writeShows(w, true, 0)
		}
		// The client says who won: the winners of a hand with no showdown did not
		// show, and the winners of a showdown collected the pot. They are listed in
		// seat order, as the client does in all but a few hands.
		rake := h.Rake + h.JPFee
		pots := h.potsBefore(River+1, rake, h.TotalPot+rake)
		for i := range h.Players {
			p := &h.Players[i]
			switch {
			case p.Won == 0:
			case h.Showdown && pots == nil:
				w("%s collected %s from main pot", p.Name, p.Won)
			case !h.Showdown && !p.Showed:
				w("%s does not show", p.Name)
			}
		}
		if h.Showdown && pots != nil {
			h.writeCollected(w, pots)
		}
	}

	w("*** SUMMARY ***")
	if h.Type == Bomb {
		w("BombPot")
	}
	total := "Total pot " + h.TotalPot.String()
	if h.Rake > 0 || h.JPFee > 0 {
		total += " | Rake " + h.Rake.String()
	}
	if h.JPFee > 0 {
		total += " | JP Fee " + h.JPFee.String()
	}
	w("%s", total)
	boards := 1
	if h.RunTwice {
		boards = 2
	}
	for run := 1; run <= boards; run++ {
		switch {
		case h.RunTwice && run == 1:
			w("Board 1 [%s]", cardList(h.Board))
		case h.RunTwice:
			w("Board 2 [%s]", cardList(h.Board2))
		case len(h.Board) > 0:
			w("Board [%s]", cardList(h.Board))
		}
		for i := range h.Players {
			if p := &h.Players[i]; p.Folded || p.Showed || p.Won > 0 || p.InSummary { // the summary leaves the others out
				w("%s", h.summarySeat(p, boardOf(h, run)))
			}
		}
	}
	return b.String()
}

// boardOf returns 0 for an ordinary hand and the run number for a hand run twice.
func boardOf(h *HandHistory, run int) int {
	if h.RunTwice {
		return run
	}
	return 0
}

// writeRun writes one run of a hand that was run twice: the streets after the
// first shared cards, then the showdown.
func (h *HandHistory) writeRun(w func(string, ...any), run int, board pk.Board, shared int) {
	if shared < 3 && len(board) >= 3 {
		w("*** FLOP %d *** [%s]", run, cardList(board[:3]))
		h.writeRunPot(w, run)
	}
	if shared < 4 && len(board) >= 4 {
		w("*** TURN %d *** [%s] [%s]", run, cardList(board[:3]), board[3])
		h.writeRunPot(w, run)
	}
	if len(board) >= 5 {
		w("*** RIVER %d *** [%s] [%s]", run, cardList(board[:4]), board[4])
		h.writeRunPot(w, run)
	}
	if h.Showdown {
		w("*** SHOW DOWN %d ***", run)
		h.writeRunPot(w, run)
	}
	h.writeShows(w, false, run)
	h.writeShows(w, true, run)
	if h.Showdown {
		for i := range h.Players {
			if p := &h.Players[i]; p.wonOn(run) > 0 {
				w("%s collected %s from main pot %d", p.Name, p.wonOn(run), run)
			}
		}
	}
}

// writeRunPot writes the pot line of a run: the pot and the rake of that run.
func (h *HandHistory) writeRunPot(w func(string, ...any), run int) {
	// The two runs share the pot and the rake, and the odd cent of each goes to the
	// first run, as the client does.
	rake := h.Rake + h.JPFee
	gross := h.TotalPot + rake
	r, g := rake/2, gross/2
	if run == 1 {
		r, g = r+rake%2, g+gross%2
	}
	w("Main pot %d %s | Rake %s", run, g-r, r)
}

func isPost(k ActionKind) bool {
	switch k {
	case PostSmallBlind, PostBigBlind, PostStraddle, PostAnte, PostDead, PostEntry:
		return true
	}
	return false
}

func cardList(cs []pk.Card) string {
	parts := make([]string, len(cs))
	for i, c := range cs {
		parts[i] = c.String()
	}
	return strings.Join(parts, " ")
}

func (p *Seat) shownCards() string {
	switch {
	case p.HasCards:
		return cardList(p.Cards[:])
	case p.HasPartial:
		if p.PartialFirst {
			return p.PartialCard.String() + " -"
		}
		return "- " + p.PartialCard.String()
	}
	return "- -"
}

// streetMarker writes the line that starts a street.
func (h *HandHistory) streetMarker(w func(string, ...any), s Street) {
	h.streetMarkerLine(w, s)
	// The line the files print after the marker: the pot as the street began,
	// without the rake, and the rake, as the hand keeps them. Only streets that have
	// the line get it. Whether they are right is for PotLinesStable to say.
	if !h.RunTwice && h.Pots[s] != 0 && h.Pots[s] >= h.Rakes[s] {
		// With players all-in for different amounts the pot is a main pot and side
		// pots, one line each.
		if pots := h.potsBefore(s, h.Rakes[s], h.Pots[s]); pots != nil {
			for i, p := range pots {
				w("%s %s | Rake %s", potName(i), p.net(), p.rake)
			}
		} else {
			w("Main pot %s | Rake %s", h.Pots[s]-h.Rakes[s], h.Rakes[s])
		}
	}
}

// streetMarkerLine writes the line that starts a street.
func (h *HandHistory) streetMarkerLine(w func(string, ...any), s Street) {
	n := cardsAtStreet[s]
	if len(h.Board) < n {
		w("*** %s ***", strings.ToUpper(s.String()))
		return
	}
	if s == Flop {
		w("*** FLOP *** [%s]", cardList(h.Board[:3]))
		return
	}
	w("*** %s *** [%s] [%s]", strings.ToUpper(s.String()), cardList(h.Board[:n-1]), h.Board[n-1])
}

// actionText writes action i the way the file does.
func (h *HandHistory) actionText(i int) string {
	a := &h.Actions[i]
	var s string
	switch a.Kind {
	case Fold:
		s = "folds"
	case Check:
		s = "checks"
	case Call:
		s = "calls " + a.Amount.String()
	case Bet:
		s = "bets " + a.Amount.String()
	case Raise:
		// The file's first figure is not kept, only the chips added; in a hand with
		// antes it often differs. See fixAnteRaises.
		add := a.Amount
		s = fmt.Sprintf("raises %s to %s", add, a.To)
	case PostSmallBlind:
		s = "posts the small blind " + a.Amount.String()
	case PostBigBlind:
		s = "posts the big blind " + a.Amount.String()
	case PostAnte:
		s = "posts ante " + a.Amount.String()
	case PostDead:
		s = "posts dead " + a.Amount.String()
	case PostEntry:
		s = "posts " + a.Amount.String()
	}
	if a.AllIn {
		s += " and is all-in"
	}
	return a.Player + " " + s
}

// summarySeat writes a player's line in the summary.
func (h *HandHistory) summarySeat(p *Seat, run int) string {
	s := fmt.Sprintf("Seat %d: %s", p.Seat, p.Name)
	if p.Position != "" {
		s += " (" + p.Position + ")"
	}
	switch {
	case p.Showed:
		s += " showed [" + p.shownCards() + "]"
		switch {
		case p.Folded:
			s += " and folded on the " + p.FoldedOn.summaryName()
		case p.wonOn(run) > 0:
			s += " and won " + p.wonOn(run).String()
		default:
			s += " and lost"
		}
		if d := p.descr(run); d != "" {
			s += " with " + d
			if run == 0 && p.wonOn(run) > 0 && !p.Folded {
				s += h.kickerClause(p)
			}
		}
	case p.Folded:
		s += " folded on the " + p.FoldedOn.summaryName()
		if h.didNotBet(p) {
			s += " and did not bet"
		}
	case p.wonOn(run) > 0:
		s += " did not show and won " + p.wonOn(run).String()
	default:
		s += " did not show"
	}
	return s
}

// didNotBet reports whether the site's client adds "and did not bet" to the
// summary line of a player who folded. It does for a player who folded before the
// flop without having posted a blind, and who is not on the button, whatever else
// they put in: a call or a raise does not count. A player who folded on a later
// street never gets it. This is what the client's files do.
func (h *HandHistory) didNotBet(p *Seat) bool {
	if p.FoldedOn != Preflop || p.Position == "button" {
		return false
	}
	for i := range h.Actions {
		a := &h.Actions[i]
		if a.Seat != p.Seat {
			continue
		}
		switch a.Kind {
		case PostSmallBlind, PostBigBlind:
			return false
		}
	}
	return true
}

// summaryName is the street as the summary writes it.
func (s Street) summaryName() string {
	return [...]string{"Pre-Flop", "Flop", "Turn", "River"}[s]
}

// writeShows writes a "shows" line for every player who showed cards and did,
// or did not, fold.
func (h *HandHistory) writeShows(w func(string, ...any), folded bool, run int) {
	// A player can show cards without a showdown, as when they muck a winner.
	for i := range h.Players {
		if p := &h.Players[i]; p.Showed && p.Folded == folded {
			descr := ""
			if d := p.descr(run); d != "" {
				descr = " (" + d + ")"
			}
			w("%s shows [%s]%s", p.Name, p.shownCards(), descr)
		}
	}
}

// wonOn is what the player won on a board: run 0 is an ordinary hand, 1 and 2
// are the runs of a hand that was run twice.
func (p *Seat) wonOn(run int) Cents {
	switch run {
	case 1:
		return p.Won - p.WonSecond
	case 2:
		return p.WonSecond
	}
	return p.Won
}

// descr is the description of the player's hand on a board.
func (p *Seat) descr(run int) string {
	if run == 2 {
		return p.HandDescr2
	}
	return p.HandDescr
}
