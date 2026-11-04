package hh

// TurnViolation says how a hand breaks the order of play.
type TurnViolation uint8

const (
	// TurnOK means every live player acted in turn.
	TurnOK TurnViolation = iota
	// SkippedSeat means a live player's turn passed without an action, so
	// that action is missing from the file. When only two players are left,
	// this shows up as the other player acting twice in a row.
	SkippedSeat
	// ActedAfterFold means a player acted after folding.
	ActedAfterFold
	// ActedAfterAllIn means a player acted after going all-in.
	ActedAfterAllIn
	// ActedTwice means a player acted twice in a row, so another player's action
	// in between is missing.
	ActedTwice
	// ActedAfterClose means a player acted after the betting round had closed:
	// every player still to act had acted and matched the bet.
	ActedAfterClose
	// ActedWhileSittingOut means a player the seat list marks as sitting out
	// acted.
	ActedWhileSittingOut
)

func (v TurnViolation) String() string {
	switch v {
	case TurnOK:
		return "every live player acts in turn"
	case SkippedSeat:
		return "a live player's turn is skipped, its action is missing"
	case ActedAfterFold:
		return "a player acts after folding"
	case ActedAfterAllIn:
		return "a player acts after going all-in"
	case ActedTwice:
		return "a player acts twice in a row, an action between is missing"
	case ActedAfterClose:
		return "a player acts after the betting round has closed"
	case ActedWhileSittingOut:
		return "a player listed as sitting out acts"
	}
	return "unknown turn violation"
}

// CheckTurnOrder checks that every live player acts when it is their turn.
//
// A street starts with the first live player after the big blind (preflop) or
// after the button (later streets). After that, each action must come from the
// next live player clockwise: not from someone who already folded or is
// all-in, not from the same player again, and not skipping a live player.
// A violation means an action line is missing from the file.
//
// Only betting actions are checked; posts and returned bets are not. If the
// big blind's post is missing, the first preflop action is accepted as it is.
func (h *HandHistory) CheckTurnOrder() TurnViolation {
	v, _, _ := h.TurnOrderDetail()
	return v
}

// TurnOrderDetail is CheckTurnOrder with the place of the first violation. It
// returns the index in h.Actions of the action that came out of turn, and the
// seat that should have acted instead. Both are -1 if there is no violation.
func (h *HandHistory) TurnOrderDetail() (v TurnViolation, at, want int) {
	// Everyone who posted or acted, in seat order. Seat numbers are small.
	var seatsBuf [16]int
	seats := seatsBuf[:0]
	var dealt, out, folded [32]bool
	var stack, invested [32]Cents // starting stack and chips put in so far, by seat
	for i := range h.Players {
		if sn := h.Players[i].Seat; sn >= 0 && sn < len(stack) {
			stack[sn] = h.Players[i].Stack
		}
	}
	bb := -1
	for i := range h.Actions {
		a := &h.Actions[i]
		if a.Seat < 0 || a.Seat >= len(dealt) {
			return TurnOK, -1, -1 // not a layout this check understands
		}
		if !dealt[a.Seat] {
			dealt[a.Seat] = true
			if len(seats) == len(seatsBuf) {
				return TurnOK, -1, -1
			}
			seats = append(seats, a.Seat)
		}
		if a.Kind == PostBigBlind || a.Kind == PostStraddle {
			bb = a.Seat // the last live blind: the first player to act is the one after it
		}
	}
	// Players who were in the hand without a line in the log: the summary says
	// they folded, showed cards or won, or a bet was returned to them. Their own
	// action is what is missing. A summary line that only says "did not show"
	// is no proof: a player who just sat down and waits for the big blind is
	// listed that way and was not dealt in.
	addSeat := func(seat int) bool {
		if seat < 0 || seat >= len(dealt) || len(seats) == len(seatsBuf) {
			return false
		}
		if !dealt[seat] {
			dealt[seat] = true
			seats = append(seats, seat)
		}
		return true
	}
	for i := range h.Players {
		if p := &h.Players[i]; p.dealtInPerSummary() && !addSeat(p.Seat) {
			return TurnOK, -1, -1
		}
	}
	if h.Returned.Amount != 0 && !addSeat(h.Returned.Seat) {
		return TurnOK, -1, -1
	}
	if len(seats) < 2 {
		return TurnOK, -1, -1
	}
	for i := 1; i < len(seats); i++ { // insertion sort
		for j := i; j > 0 && seats[j] < seats[j-1]; j-- {
			seats[j], seats[j-1] = seats[j-1], seats[j]
		}
	}
	// next returns the first live seat clockwise after from, or -1.
	next := func(from int) int {
		for _, s := range seats {
			if s > from && !out[s] {
				return s
			}
		}
		for _, s := range seats {
			if !out[s] {
				return s
			}
		}
		return -1
	}

	// A player the seat list marks as sitting out is not in the hand.
	for i := range h.Actions {
		for j := range h.Players {
			if h.Players[j].SittingOut && h.Players[j].Seat == h.Actions[i].Seat {
				return ActedWhileSittingOut, i, -1
			}
		}
	}

	// toAct holds the live players who still have to act on the street. It starts
	// as everyone live, and a bet or raise puts everyone else back in it. When it
	// is empty the betting round is closed.
	var toAct [32]bool
	nToAct := 0
	started := false
	startStreet := func(street Street) {
		nToAct = 0
		for _, s := range seats {
			toAct[s] = !out[s]
			if toAct[s] {
				nToAct++
			}
		}
		// After the flop a lone live player has no one to bet against: the others
		// are all-in or have folded, so there is no betting on the street.
		if street != Preflop && nToAct < 2 {
			nToAct = 0
		}
	}

	street := Preflop
	prev := -1
	for i := range h.Actions {
		a := &h.Actions[i]
		if a.Street != street {
			street = a.Street
			prev = -1
			started = false
		}
		// A player whose ante or blind took their whole stack is all-in before
		// the first action and is never asked to act. A call or bet that uses
		// the whole stack is different: the file marks that player all-in only
		// when it logs their next action, so those wait for the flag.
		invested[a.Seat] += a.Amount
		allIn := stack[a.Seat] > 0 && invested[a.Seat] >= stack[a.Seat]
		switch a.Kind {
		case Fold, Check, Call, Bet, Raise:
		default:
			if allIn {
				out[a.Seat] = true
			}
			continue
		}
		if !started {
			startStreet(street)
			started = true
		}
		switch {
		case prev >= 0:
			want = next(prev)
		case street == Preflop && bb < 0:
			want = a.Seat // no big blind to count from; accept the first actor
		case street == Preflop:
			want = next(bb)
		default:
			want = next(h.ButtonSeat)
		}
		if a.Seat == prev && want == prev {
			// Nobody else is live, so the player is asked to act again after
			// acting. The round was over.
			return ActedTwice, i, want
		}
		if a.Seat != want {
			switch {
			case folded[a.Seat]:
				return ActedAfterFold, i, want
			case out[a.Seat]:
				return ActedAfterAllIn, i, want
			}
			return SkippedSeat, i, want
		}
		if nToAct == 0 {
			return ActedAfterClose, i, -1
		}
		if a.Kind == Fold {
			out[a.Seat], folded[a.Seat] = true, true
		}
		if a.AllIn { // a call or bet that uses the whole stack is not marked until the file says so
			out[a.Seat] = true
		}
		// A bet or raise puts every other live player back to act; anything else,
		// a raise that does not go above the bet included, takes this player off
		// the list.
		if (a.Kind == Bet || a.Kind == Raise) && h.raisesBet(i) {
			nToAct = 0
			for _, s := range seats {
				toAct[s] = !out[s] && s != a.Seat
				if toAct[s] {
					nToAct++
				}
			}
		} else if toAct[a.Seat] {
			toAct[a.Seat] = false
			nToAct--
		}
		prev = a.Seat
	}
	return TurnOK, -1, -1
}

// raisesBet reports whether action i puts the bet up, so that the other players
// must act again: a bet does, and a raise does if its total is above every bet
// made on the street before it. A raise that is not, an all-in for less that
// CoinPoker writes as "raises ₮0.00 to ₮199.68", works like a call.
func (h *HandHistory) raisesBet(i int) bool {
	a := &h.Actions[i]
	switch a.Kind {
	case Bet:
		return true
	case Raise:
	default:
		return false
	}
	var own [32]Cents
	var top Cents
	for j := 0; j < i; j++ {
		b := &h.Actions[j]
		if b.Street != a.Street || b.Seat < 0 || b.Seat >= len(own) {
			continue
		}
		own[b.Seat] += LiveAmount(h.Site, b.Kind, b.Amount, h.BigBlind)
		top = max(top, own[b.Seat])
	}
	if a.Seat < 0 || a.Seat >= len(own) {
		return a.To > top
	}
	return own[a.Seat]+a.Amount > top
}
