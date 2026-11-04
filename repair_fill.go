package hh

// Repairs for actions that the file left out of the betting: a live player's turn
// that passed without a line, and a betting round that was never closed.

// fillSkipped inserts the actions of players who had to act and have no line, up
// to depth of them, and reports whether the hand then passes the pot,
// returned-bet and turn-order checks and every round closes. Where the hand does
// not settle the kind of a missing action, each candidate is tried in turn; the
// hand is left as it was if none leads to a consistent hand.
func (h *HandHistory) fillSkipped(depth int) bool {
	var at, want int
	var street Street
	v, at0, want0 := h.TurnOrderDetail()
	switch v {
	case TurnOK:
		a, st, w, open := h.firstOpenRound()
		if !open {
			return h.repaired()
		}
		at, street, want = a, st, w
	case SkippedSeat:
		if at0 < 0 || want0 < 0 {
			return false
		}
		at, street, want = at0, h.Actions[at0].Street, want0
	default:
		return false
	}
	if depth == 0 || want < 0 {
		return false
	}
	for _, cand := range h.skippedCandidates(at, street, want) {
		orig, origLines := h.Actions, h.SitOutLines
		h.insertAction(at, cand)
		if h.fillSkipped(depth - 1) {
			return true
		}
		h.Actions, h.SitOutLines = orig, origLines
	}
	return false
}

// streetState is the betting on one street, as it stands before an action.
type streetState struct {
	own      [32]Cents // each seat's bet on the street, antes aside
	invested [32]Cents // each seat's chips in the hand
	maxBet   Cents
}

// replay returns the state on the given street after the first n actions. Streets
// before it are only counted in invested.
func (h *HandHistory) replay(n int, street Street) (st streetState, ok bool) {
	cur := Preflop
	for j := 0; j < n; j++ {
		a := &h.Actions[j]
		if a.Seat < 0 || a.Seat >= len(st.own) {
			return st, false
		}
		if a.Street != cur {
			cur = a.Street
			st.own = [32]Cents{}
			st.maxBet = 0
		}
		st.invested[a.Seat] += a.Amount
		switch a.Kind {
		case PostSmallBlind, PostBigBlind, PostStraddle, PostEntry, Call, Bet, Raise:
			st.own[a.Seat] += LiveAmount(h.Site, a.Kind, a.Amount, h.BigBlind)
		default:
			continue
		}
		st.maxBet = max(st.maxBet, st.own[a.Seat])
	}
	if cur != street {
		st.own, st.maxBet = [32]Cents{}, 0
	}
	return st, true
}

// stackOf returns the seat line stack of the seat, 0 if unknown.
func (h *HandHistory) stackOf(seat int) Cents {
	if pi := h.seatIndex(seat); pi >= 0 {
		return h.Players[pi].Stack
	}
	return 0
}

// firstOpenRound finds the first betting round that the log leaves open: players
// are still to act when the next street begins or the hand ends, or a street that
// two or more players could bet on has no action at all. at is where the missing
// action goes, street its street and want the seat that has to act. A player whose
// chips are all in does not have to act.
func (h *HandHistory) firstOpenRound() (at int, street Street, want int, ok bool) {
	seats := h.dealtInSeats()
	if len(seats) < 2 || len(seats) > 16 || len(h.Actions) == 0 {
		return 0, 0, 0, false
	}
	var folded, allIn, toAct [32]bool
	var stack, invested [32]Cents
	for _, s := range seats {
		stack[s] = h.stackOf(s)
	}
	live := func(s int) bool {
		return !folded[s] && !allIn[s] && !(stack[s] > 0 && invested[s] >= stack[s])
	}
	count := func(f func(int) bool) int {
		n := 0
		for _, s := range seats {
			if f(s) {
				n++
			}
		}
		return n
	}
	// nextWhere returns the first seat clockwise after from that passes f.
	nextWhere := func(from int, f func(int) bool) int {
		for _, s := range seats {
			if s > from && f(s) {
				return s
			}
		}
		for _, s := range seats {
			if f(s) {
				return s
			}
		}
		return -1
	}
	stillToAct := func(s int) bool { return toAct[s] && live(s) }
	inHand := func(s int) bool { return !folded[s] }

	cur := Preflop
	started := false
	prev := -1
	for i := range h.Actions {
		a := &h.Actions[i]
		if a.Seat < 0 || a.Seat >= len(toAct) {
			return 0, 0, 0, false
		}
		if a.Street != cur {
			if started && count(stillToAct) > 0 && count(inHand) >= 2 {
				w := nextWhere(prev, stillToAct)
				return i, cur, w, w >= 0
			}
			for st := cur + 1; st < a.Street; st++ {
				if count(live) >= 2 {
					w := nextWhere(h.ButtonSeat, live)
					return i, st, w, w >= 0
				}
			}
			cur, started, prev = a.Street, false, -1
		}
		invested[a.Seat] += a.Amount
		switch a.Kind {
		case Fold, Check, Call, Bet, Raise:
		default:
			continue
		}
		if !started {
			for _, s := range seats {
				toAct[s] = live(s)
			}
			started = true
		}
		toAct[a.Seat] = false
		switch a.Kind {
		case Fold:
			folded[a.Seat] = true
		case Bet, Raise:
			if h.raisesBet(i) {
				for _, s := range seats {
					toAct[s] = s != a.Seat && live(s)
				}
			}
		}
		if a.AllIn {
			allIn[a.Seat] = true
		}
		prev = a.Seat
	}
	if started && count(stillToAct) > 0 && count(inHand) >= 2 {
		w := nextWhere(prev, stillToAct)
		return len(h.Actions), cur, w, w >= 0
	}
	return 0, 0, 0, false
}

// skippedCandidates returns the actions that the player at seat may have made in
// the turn that the file left out, just before action at, on the given street.
//
// A bet or raise is a candidate when the file shows one: the uncalled bet that is
// returned to the player is the part of their bet nobody matched, and a later call
// by the player for less than the bet then standing shows how much they had in.
// A player who acts again later, or who showed cards or won, did not fold. The
// summary says on which street a player folded. Otherwise the player either folded
// or, if nothing is owed, checked, else called.
func (h *HandHistory) skippedCandidates(at int, street Street, seat int) []Action {
	pi := h.seatIndex(seat)
	if pi < 0 {
		return nil
	}
	p := &h.Players[pi]
	st, ok := h.replay(at, street)
	if !ok {
		return nil
	}
	toCall := st.maxBet - st.own[seat]
	var room Cents
	if stack := p.Stack; stack > 0 {
		room = max(stack-st.invested[seat], 0)
	}
	stay := Action{Street: street, Seat: seat, Player: p.Name, Inferred: true}
	if toCall > 0 {
		stay.Kind, stay.Amount = Call, toCall
		if room > 0 && toCall >= room {
			stay.Amount, stay.AllIn = room, true
		}
	} else {
		stay.Kind = Check
	}
	fold := Action{Street: street, Seat: seat, Player: p.Name, Kind: Fold, Inferred: true}

	// raiseTo makes the bet or raise that takes the player's bet on the street to to.
	var cands []Action
	raiseTo := func(to Cents) {
		add := to - st.own[seat]
		if add <= 0 || to <= st.maxBet {
			return
		}
		kind := Raise
		if st.maxBet == 0 {
			kind = Bet
		}
		a := Action{Street: street, Seat: seat, Player: p.Name, Kind: kind, Amount: add, To: to, Inferred: true}
		if kind == Bet {
			a.To = 0 // a bet line says how much, not "to"
		}
		if room > 0 && add >= room {
			a.Amount, a.AllIn = room, true
		}
		cands = append(cands, a)
	}
	// The uncalled bet: everyone else's largest bet at the end of the street, plus
	// what was returned.
	if r := h.Returned; r.Amount > 0 && r.Seat == seat && r.Street == street {
		end := at
		for end < len(h.Actions) && h.Actions[end].Street == street {
			end++
		}
		if last, ok := h.replay(end, street); ok {
			var others Cents
			for s := range last.own {
				if s != seat {
					others = max(others, last.own[s])
				}
			}
			raiseTo(others + r.Amount)
		}
	}
	// A later call for less than what was owed.
	later := -1
	for j := at; j < len(h.Actions); j++ {
		if h.Actions[j].Seat == seat {
			later = j
			break
		}
	}
	if later >= 0 && h.Actions[later].Street == street && h.Actions[later].Kind == Call {
		if call, ok := h.replay(later, street); ok && call.own[seat] == st.own[seat] {
			if owed := call.maxBet - call.own[seat]; h.Actions[later].Amount < owed {
				raiseTo(call.maxBet - h.Actions[later].Amount)
			}
		}
	}
	// The next call on the street by another player for more than what stood:
	// the missing bet or raise brought the street to what that call reaches.
	for j := at; j < len(h.Actions) && h.Actions[j].Street == street; j++ {
		a := &h.Actions[j]
		if a.Seat == seat || a.Kind != Call {
			continue
		}
		if call, ok := h.replay(j, street); ok {
			if reach := call.own[a.Seat] + a.Amount; reach > call.maxBet && !a.AllIn {
				raiseTo(reach)
			}
		}
		break
	}

	switch {
	case later >= 0, p.Showed, p.Won > 0, p.Folded && p.FoldedOn > street:
		return append(cands, stay)
	case p.Folded && p.FoldedOn == street, toCall > 0:
		return append(cands, fold, stay)
	}
	return append(cands, stay)
}

// completeNoSummary works out the result of a hand that the file stops before its
// summary, when the actions alone say how it ended. It does so only when no flop
// was dealt and exactly one player who posted or acted has not folded: that
// player wins everything that was put in less the uncalled bet, and there is no
// rake without a flop. The returned bet, the total pot, the winner's winnings and
// who folded are filled in, NoSummary is cleared and Completed is set. It reports
// whether it did; the hand is not changed if it did not.
func (h *HandHistory) completeNoSummary() bool {
	if !h.NoSummary || len(h.Actions) == 0 || len(h.Board) > 0 || h.Actions[len(h.Actions)-1].Street != Preflop {
		return false
	}
	var seen, folded [32]bool
	var chips Cents
	for i := range h.Actions {
		a := &h.Actions[i]
		if a.Seat < 0 || a.Seat >= len(seen) || a.Street != Preflop {
			return false
		}
		seen[a.Seat] = true
		chips += a.Amount
		if a.Kind == Fold {
			folded[a.Seat] = true
		}
	}
	winner := -1
	for s := range seen {
		if seen[s] && !folded[s] {
			if winner >= 0 {
				return false // two players are still in
			}
			winner = s
		}
	}
	wi := h.seatIndex(winner)
	if winner < 0 || wi < 0 || h.Players[wi].SittingOut {
		return false
	}
	seat, uncalled, ok := h.UncalledBet()
	if !ok || (uncalled > 0 && seat != winner) {
		return false
	}
	if uncalled >= chips {
		// The big blind alone, as when the small blind's seat is empty. The site's own
		// client keeps the blind in the pot and writes no returned-bet line.
		uncalled = 0
	}
	if uncalled > 0 {
		h.Returned = Return{Seat: winner, Player: h.Players[wi].Name, Amount: uncalled, Street: Preflop}
	}
	h.TotalPot = chips - uncalled
	h.Players[wi].Won = h.TotalPot
	for i := range h.Players {
		p := &h.Players[i]
		if p.Seat >= 0 && p.Seat < len(folded) && folded[p.Seat] {
			p.Folded, p.FoldedOn = true, Preflop
		}
		if p.Seat == h.ButtonSeat {
			p.Position = "button"
		}
	}
	for i := range h.Actions {
		a := &h.Actions[i]
		if pi := h.seatIndex(a.Seat); pi >= 0 && h.Players[pi].Position == "" {
			switch a.Kind {
			case PostSmallBlind:
				h.Players[pi].Position = "small blind"
			case PostBigBlind:
				h.Players[pi].Position = "big blind"
			}
		}
	}
	h.NoSummary = false
	h.Completed = true
	return true
}
