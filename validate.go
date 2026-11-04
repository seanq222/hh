package hh

import (
	"github.com/seanq222/hh/pk"
)

// BoardWinnersMatch reports whether, on each board dealt to the river, the
// players with the best hand won something from it. With side pots the best hand
// still wins a share of the main pot, which every player left is in. Only boards
// where every player who did not fold shows cards are checked. In a hand run twice
// each board is checked against what was won on it.
func (h *HandHistory) BoardWinnersMatch() bool {
	folded := map[int]bool{}
	for _, a := range h.Actions {
		if a.Kind == Fold {
			folded[a.Seat] = true
		}
	}
	var live []*Seat
	for _, s := range h.dealtInSeats() {
		if folded[s] {
			continue
		}
		p := h.PlayerBySeat(s)
		if p == nil || !p.HasCards {
			return true // a hand is not known
		}
		live = append(live, p)
	}
	if len(live) < 2 {
		return true
	}
	bestWon := func(board pk.Board, won func(*Seat) Cents) bool {
		if len(board) < 5 {
			return true
		}
		ranks := make([]pk.HandRank, len(live))
		best := pk.HandRank(9999) // worse than any hand
		for i, p := range live {
			ranks[i] = pk.Evaluate7(p.Cards[0], p.Cards[1], board[0], board[1], board[2], board[3], board[4])
			best = min(best, ranks[i])
		}
		for i, p := range live {
			if ranks[i] == best && won(p) <= 0 {
				return false
			}
		}
		return true
	}
	if !bestWon(h.Board, func(p *Seat) Cents { return p.Won - p.WonSecond }) {
		return false
	}
	return !h.RunTwice || bestWon(h.Board2, func(p *Seat) Cents { return p.WonSecond })
}

// StreetsReplayable reports whether the betting in the log can be replayed
// into the streets the file shows. When an action line is missing, most often
// the call that closes a round, the replay never sees the round end and the
// later actions land on the wrong street. PlayerHands refuses such hands.
// Reader does not drop on this check; it drops on CheckTurnOrder.
//
// Hands that merely lack a blind or ante post line pass: their betting is
// intact. Hands that cannot be turned into player views for other reasons, such
// as a table with fewer than two players dealt in, also pass.
//
// It replays the actions themselves and builds no player views.
func (h *HandHistory) StreetsReplayable() bool {
	if len(h.Players) > 16 {
		return true
	}
	// The replay keeps its state per seat, numbered 0 to n-1 in the order the
	// seats first act; any numbering works, so positions are not needed.
	var index [16]int8
	for i := range index {
		index[i] = -1
	}
	n := 0
	events := make([]Event, len(h.Actions))
	for i := range h.Actions {
		a := &h.Actions[i]
		pi := -1
		for j := range h.Players {
			if h.Players[j].Seat == a.Seat {
				pi = j
				break
			}
		}
		if pi < 0 {
			return true
		}
		if index[pi] < 0 {
			index[pi] = int8(n)
			n++
		}
		events[i] = Event{Actor: uint8(index[pi]), Kind: h.eventKind(i), AllIn: a.AllIn, Amount: a.Amount}
	}
	if n < 2 || n > 9 {
		return true
	}
	return h.streetsMatch(events, n)
}

// streetsMatch reports whether replaying events, one for each action, puts
// every action on the street the parser read it on.
func (h *HandHistory) streetsMatch(events []Event, n int) bool {
	i := 0
	for st := range replay(events, n, h.BigBlind, h.Site) {
		if st.Street != h.Actions[i].Street {
			return false
		}
		i++
	}
	return true
}

// eventKind is the kind of action i as the replay counts it.
func (h *HandHistory) eventKind(i int) ActionKind {
	kind := h.Actions[i].Kind
	if kind == Raise && !h.raisesBet(i) {
		// A raise that does not go above the bet, an all-in for less that
		// CoinPoker writes as a raise by ₮0.00, is a call.
		kind = Call
	}
	return kind
}

// ChipsConsistent reports whether the logged actions add up to the pots the
// file itself states: the "Main pot" line printed as each street begins, and
// the total pot plus rake in the summary.
//
// The pot lines are a check on the log, not a source of truth, but when they
// disagree the hand cannot be trusted: a line is missing from the log or a pot
// line is wrong. Reader drops such hands. A hand with no pot lines to compare
// is treated as consistent.
func (h *HandHistory) ChipsConsistent() bool {
	// The uncalled bet goes back when the betting ends. Streets that are dealt
	// afterwards with no betting, as when the last player is all-in, start with
	// the pot after it has gone back.
	_, uncalled, ok := h.UncalledBet()
	if !ok {
		uncalled = 0
	}
	var chips Cents
	next := Flop
	for i := range h.Actions {
		a := &h.Actions[i]
		for next <= River && a.Street >= next {
			if h.Pots[next] != 0 && chips != h.Pots[next] {
				return false
			}
			next++
		}
		chips += a.Amount
	}
	// Streets dealt after the last bet have no action, so their pot lines say
	// nothing about the log. Some files print a stale pot on them, as when the
	// last player is all-in. The total pot below still checks the betting.
	if h.TotalPot == 0 {
		return true
	}
	want := h.TotalPot + h.Rake + h.JPFee
	// The pot is what went in less the uncalled bet.
	if chips-uncalled == want {
		return true
	}
	// The total pot line is sometimes stale, as when it is the pot at the start
	// of the river and the river was an all-in. What the winners were paid is a
	// second witness: if that and the rake add up to the actions, the actions
	// are right and the line is wrong.
	var paid Cents
	for i := range h.Players {
		paid += h.Players[i].Won
	}
	if paid > 0 && paid+h.Rake+h.JPFee == chips-uncalled {
		return true
	}
	// Everyone folded to the big blind. The site's own client keeps the big
	// blind in the pot and writes no returned-bet line, while other files
	// take its excess out. Either way the blinds are the pot.
	return h.isWalk() && chips == want
}

// WinnerPaid reports whether someone is credited with winning the pot: a hand
// with chips in the pot, after any bet that went back, and nobody's summary line
// saying they won is missing that line, and cannot add up. A hand where only the
// big blind posted and everyone folded has an empty pot: the blind goes back.
func (h *HandHistory) WinnerPaid() bool {
	var paid, chips Cents
	for i := range h.Players {
		paid += max(h.Players[i].Won, 0)
	}
	if paid > 0 {
		return true
	}
	for i := range h.Actions {
		chips += h.Actions[i].Amount
	}
	if _, uncalled, ok := h.UncalledBet(); ok {
		chips -= uncalled
	}
	return chips <= 0
}

// WinningsMatch reports whether what the winners were paid, with the rake and the
// jackpot fee, is the chips the actions put in less the uncalled bet. The total
// pot line can be stale and still agree with the actions when an action is wrong
// or missing; the winnings cannot, so they are the witness that settles it. A hand
// with no winnings has nothing to compare; WinnerPaid fails one with chips in it.
func (h *HandHistory) WinningsMatch() bool {
	var paid, chips Cents
	for i := range h.Players {
		paid += h.Players[i].Won
	}
	if paid == 0 {
		return true
	}
	for i := range h.Actions {
		chips += h.Actions[i].Amount
	}
	_, uncalled, ok := h.UncalledBet()
	if !ok {
		return false
	}
	want := paid + h.Rake + h.JPFee
	return chips-uncalled == want || (h.isWalk() && chips == want)
}

// ReturnLineMissing reports whether the file leaves out the returned-bet line
// although its total pot is the chips less the uncalled bet. Hand2Note cannot
// match the winner's collected amount to the pot without the line.
func (h *HandHistory) ReturnLineMissing() bool {
	if h.Returned.Amount != 0 {
		return false
	}
	_, uncalled, ok := h.UncalledBet()
	if !ok || uncalled == 0 || h.TotalPot == 0 {
		return false
	}
	var chips Cents
	for i := range h.Actions {
		chips += h.Actions[i].Amount
	}
	return h.TotalPot+h.Rake+h.JPFee == chips-uncalled
}

// isWalk reports whether nobody made a voluntary bet, so everyone folded to the
// big blind.
func (h *HandHistory) isWalk() bool {
	for i := range h.Actions {
		switch h.Actions[i].Kind {
		case Call, Bet, Raise:
			return false
		}
	}
	return true
}
