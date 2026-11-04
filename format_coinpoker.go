package hh

import (
	"fmt"
	"strings"

	"github.com/seanq222/hh/pk"
)

// coinPokerString writes the hand in CoinPoker's format, the way parseCoinPoker
// reads it. What the parser does not keep is worked out from the hand: the ante in
// the header from the ante posts, the first figure of a raise from the bets, the
// position labels of the summary from the button and the blind posts, "(didn't
// bet)" from what a player put in, and the showdown's shows, mucks and collected
// lines from the cards and the winnings. The time is written in UTC ("+00").
func (h *HandHistory) coinPokerString() string {
	var b strings.Builder
	w := func(format string, args ...any) {
		fmt.Fprintf(&b, format, args...)
		b.WriteByte('\n')
	}
	game := strings.TrimSpace(h.Game + " " + h.Limit)
	stakes := fmt.Sprintf("%s/%s", usdt(h.SmallBlind), usdt(h.BigBlind))
	var ante Cents
	for i := range h.Actions {
		if h.Actions[i].Kind == PostAnte {
			ante = max(ante, h.Actions[i].Amount)
		}
	}
	if ante > 0 {
		stakes += " - Ante " + usdt(ante)
	}
	w("CoinPoker Hand #%d: %s (%s CPCC) - %s +00", h.ID, game, stakes, h.Time.UTC().Format("2006/01/02 15:04:05"))
	w("Table '%s' %d-max Seat #%d is the button", h.Table, h.MaxSeats, h.ButtonSeat)
	for i := range h.Players {
		p := &h.Players[i]
		out := ""
		if p.SittingOut {
			out = " is sitting out"
		}
		w("Seat %d: %s (%s in chips)%s", p.Seat, p.Name, usdt(p.Stack), out)
	}

	// The board each street shows, and where the two runs of a hand run twice part.
	shared := len(h.Board)
	if h.RunTwice {
		shared = 0
		for shared < len(h.Board) && shared < len(h.Board2) && h.Board[shared] == h.Board2[shared] {
			shared++
		}
	}
	marker := func(prefix string, s Street, board pk.Board) {
		n := cardsAtStreet[s]
		if len(board) < n {
			return
		}
		if s == Flop {
			w("*** %s%s *** [%s]", prefix, s.markerName(), cardList(board[:3]))
		} else {
			w("*** %s%s *** [%s] [%s]", prefix, s.markerName(), cardList(board[:n-1]), board[n-1])
		}
	}

	var own [32]Cents
	var maxBet Cents
	street := Preflop
	holeCards := false
	for i := range h.Actions {
		a := &h.Actions[i]
		if !holeCards && !isPost(a.Kind) {
			w("*** HOLE CARDS ***")
			holeCards = true
		}
		for street < a.Street {
			if h.Returned.Amount != 0 && h.Returned.Street == street {
				w("Uncalled bet (%s) returned to %s", usdt(h.Returned.Amount), h.Returned.Player)
			}
			street++
			marker("", street, h.Board)
			own, maxBet = [32]Cents{}, 0
		}
		s := ""
		switch a.Kind {
		case Fold:
			s = "folds"
		case Check:
			s = "checks"
		case Call:
			s = "calls " + usdt(a.Amount)
		case Bet:
			s = "bets " + usdt(a.Amount)
		case Raise:
			// A raise that does not go above the bet, an all-in for less, is by ₮0.00.
			s = fmt.Sprintf("raises %s to %s", usdt(max(a.To-maxBet, 0)), usdt(a.To))
		case PostAnte:
			s = "posts the ante " + usdt(a.Amount)
		case PostSmallBlind:
			s = "posts small blind " + usdt(a.Amount)
		case PostBigBlind, PostEntry:
			s = "posts big blind " + usdt(a.Amount)
		case PostStraddle:
			s = "posts straddle " + usdt(a.Amount)
		case PostDead:
			s = "posts dead " + usdt(a.Amount)
		}
		if a.AllIn {
			s += " and is all-in"
		}
		w("%s: %s", a.Player, s)
		if a.Seat >= 0 && a.Seat < len(own) {
			switch a.Kind {
			case PostSmallBlind, PostBigBlind, PostStraddle, Call, Bet:
				own[a.Seat] += a.Amount
			case PostEntry:
				own[a.Seat] += min(a.Amount, h.BigBlind)
			case Raise:
				own[a.Seat] = a.To
			}
			maxBet = max(maxBet, own[a.Seat])
		}
	}
	if !holeCards {
		w("*** HOLE CARDS ***")
	}
	if h.Returned.Amount != 0 && h.Returned.Street == street {
		w("Uncalled bet (%s) returned to %s", usdt(h.Returned.Amount), h.Returned.Player)
	}
	// The streets dealt after the last action: the shared ones, then each run.
	for s := street + 1; s <= River; s++ {
		if cardsAtStreet[s] <= shared {
			marker("", s, h.Board)
			street = s
		}
	}
	if h.RunTwice {
		for s := street + 1; s <= River; s++ {
			marker("FIRST ", s, h.Board)
		}
		for s := street + 1; s <= River; s++ {
			marker("SECOND ", s, h.Board2)
		}
	}

	// The showdown, or the winner of a hand that ended before one.
	switch {
	case h.Showdown && h.RunTwice:
		w("*** FIRST SHOW DOWN ***")
		h.writeCPShowdown(w, 1)
		w("*** SECOND SHOW DOWN ***")
		h.writeCPShowdown(w, 2)
	case h.Showdown:
		w("*** SHOW DOWN ***")
		h.writeCPShowdown(w, 0)
	default:
		for i := range h.Players {
			if p := &h.Players[i]; p.Won > 0 {
				w("%s collected %s from pot", p.Name, usdt(p.Won))
				w("%s: doesn't show hand", p.Name)
			}
		}
	}

	w("*** SUMMARY ***")
	w("Total pot %s | Rake %s", usdt(h.TotalPot), usdt(h.Rake))
	switch {
	case h.RunTwice:
		w("Hand was run twice")
		w("FIRST Board [%s]", cardList(h.Board))
		w("SECOND Board [%s]", cardList(h.Board2))
	case len(h.Board) > 0:
		w("Board [%s]", cardList(h.Board))
	}
	for i := range h.Players {
		if p := &h.Players[i]; p.InSummary {
			w("%s", h.cpSummarySeat(p))
		}
	}
	return b.String()
}

// writeCPShowdown writes a showdown: the cards each player still in shows, or
// that they muck, and what each collects. run is 1 or 2 in a hand run twice,
// else 0.
func (h *HandHistory) writeCPShowdown(w func(string, ...any), run int) {
	for i := range h.Players {
		p := &h.Players[i]
		if p.Folded || !h.cpInHand(p) {
			continue
		}
		won := p.wonOnCP(run)
		d := p.HandDescr
		if run == 2 {
			d = p.HandDescr2
		}
		if d != "" {
			d = " (" + d + ")"
		}
		switch {
		case p.HasCards:
			w("%s: shows [%s]%s", p.Name, cardList(p.Cards[:]), d)
		case d != "" && won == 0:
			w("%s: shows []%s", p.Name, d) // a muck that names the hand
		case won > 0:
			w("%s: doesn't show hand", p.Name)
		default:
			w("%s: mucks hand", p.Name)
		}
	}
	for i := range h.Players {
		if p := &h.Players[i]; p.wonOnCP(run) > 0 {
			w("%s collected %s from pot", p.Name, usdt(p.wonOnCP(run)))
		}
	}
}

// wonOnCP is what the player won on run 1 or 2 of a hand run twice, or in all for
// run 0.
func (p *Seat) wonOnCP(run int) Cents {
	switch run {
	case 1:
		return p.Won - p.WonSecond
	case 2:
		return p.WonSecond
	}
	return p.Won
}

// cpInHand reports whether the player posted or acted, that is was dealt in.
func (h *HandHistory) cpInHand(p *Seat) bool {
	for i := range h.Actions {
		if h.Actions[i].Seat == p.Seat {
			return true
		}
	}
	return false
}

// cpSummarySeat writes the player's line in the summary.
func (h *HandHistory) cpSummarySeat(p *Seat) string {
	s := fmt.Sprintf("Seat %d: %s", p.Seat, p.Name)
	if p.Seat == h.ButtonSeat {
		s += " (button)"
	}
	for i := range h.Actions {
		if a := &h.Actions[i]; a.Seat == p.Seat {
			switch a.Kind {
			case PostSmallBlind:
				s += " (small blind)"
			case PostBigBlind:
				s += " (big blind)"
			}
		}
	}
	switch {
	case p.Folded && p.FoldedOn == Preflop:
		s += " folded before Flop"
		if h.cpDidNotBet(p) {
			s += " (didn't bet)"
		}
	case p.Folded:
		s += " folded on the " + p.FoldedOn.summaryName()
	case p.HasCards && p.Won > 0:
		s += fmt.Sprintf(" showed [%s] and won (%s)", cardList(p.Cards[:]), usdt(p.Won))
		if p.HandDescr != "" {
			s += " with " + p.HandDescr
		}
	case p.HasCards:
		s += fmt.Sprintf(" showed [%s] and lost", cardList(p.Cards[:]))
		if p.HandDescr != "" {
			s += " with " + p.HandDescr
		}
	case p.Won > 0:
		s += fmt.Sprintf(" collected (%s)", usdt(p.Won))
	default:
		s += " mucked"
	}
	return s
}

// cpDidNotBet reports whether the player put nothing in but the ante, which
// CoinPoker's summary marks "(didn't bet)".
func (h *HandHistory) cpDidNotBet(p *Seat) bool {
	for i := range h.Actions {
		if a := &h.Actions[i]; a.Seat == p.Seat && a.Kind != PostAnte && a.Amount > 0 {
			return false
		}
	}
	return true
}

// usdt writes an amount in USDT, "₮1.50".
func usdt(c Cents) string { return "₮" + strings.TrimPrefix(c.String(), "$") }

// markerName is the street as a street marker writes it.
func (s Street) markerName() string {
	switch s {
	case Flop:
		return "FLOP"
	case Turn:
		return "TURN"
	}
	return "RIVER"
}
