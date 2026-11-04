package hh

// AmountViolation says which betting amount does not fit the hand.
type AmountViolation uint8

const (
	// AmountsOK means every call, bet and raise is a legal size.
	AmountsOK AmountViolation = iota
	// WrongCallAmount means a call is not for what the player owed. It is for
	// less, though the player kept chips, or for more.
	WrongCallAmount
	// BetOutOfRange means a bet is smaller than the big blind without being
	// all-in, or larger than the player's stack.
	BetOutOfRange
	// RaiseOutOfRange means a raise is smaller than the minimum without being
	// all-in, or larger than the player's stack.
	RaiseOutOfRange
	// AllInNotStack means an action is marked all-in but the player's chips in
	// the hand are not their stack.
	AllInNotStack
)

func (v AmountViolation) String() string {
	switch v {
	case AmountsOK:
		return "every call, bet and raise is a legal size"
	case WrongCallAmount:
		return "a call is not for what the player owed"
	case BetOutOfRange:
		return "a bet is smaller than the big blind or larger than the stack"
	case RaiseOutOfRange:
		return "a raise is smaller than the minimum or larger than the stack"
	case AllInNotStack:
		return "a player is all-in but has chips left, or has put in more than the stack"
	}
	return "unknown amount violation"
}

// CheckAmounts replays the betting and checks the size of every call, bet and
// raise against the bet it answers and the player's stack. It is the check that
// the "possible actions" of a poker client would make.
//
// A call must be for exactly what the player owes, unless it uses their whole
// stack for less. A bet must be at least the big blind and no more than the
// stack. A raise must reach at least the last bet plus the last raise, unless it
// is all-in, and not more than the stack allows.
//
// An action marked all-in must bring the player's chips in the hand to their
// stack. The stack on a seat line is not always right, as when a player was
// topped up after it was written; the check is strict anyway, so a hand where
// the stack and the actions disagree is not trusted. A hand with no stack for a
// player is not checked for that player.
func (h *HandHistory) CheckAmounts() AmountViolation {
	var stack, invested, own [32]Cents
	var known [32]bool
	for i := range h.Players {
		if sn := h.Players[i].Seat; sn >= 0 && sn < len(stack) {
			stack[sn] = h.Players[i].Stack
			known[sn] = h.Players[i].Stack > 0
		}
	}
	bb := h.BigBlind
	var maxBet, lastRaise Cents
	street := Preflop
	lastRaise = bb
	for i := range h.Actions {
		a := &h.Actions[i]
		if a.Seat < 0 || a.Seat >= len(stack) {
			return AmountsOK // not a layout this check understands
		}
		if a.Street != street {
			street = a.Street
			own = [32]Cents{}
			maxBet, lastRaise = 0, bb
		}
		s := a.Seat
		before := invested[s]
		switch a.Kind {
		case PostSmallBlind, PostBigBlind, PostStraddle:
			own[s] += a.Amount
			maxBet = max(maxBet, own[s])
		case PostEntry:
			own[s] += LiveAmount(h.Site, a.Kind, a.Amount, bb)
			maxBet = max(maxBet, own[s])
		case PostAnte, PostDead:
			// dead money: it does not count toward the bet
		case Call:
			toCall := maxBet - own[s]
			if a.Amount != toCall && !(a.Amount < toCall && known[s] && before+a.Amount == stack[s]) {
				return WrongCallAmount
			}
			own[s] += a.Amount
		case Bet:
			if known[s] && (a.Amount > stack[s]-before || (a.Amount < bb && before+a.Amount != stack[s])) {
				return BetOutOfRange
			}
			own[s] += a.Amount
			maxBet, lastRaise = own[s], a.Amount
		case Raise:
			to := own[s] + a.Amount // RaiseToMatches checks the file's figure against it
			allIn := known[s] && before+a.Amount == stack[s]
			if known[s] && before+a.Amount > stack[s] {
				return RaiseOutOfRange
			}
			if to < maxBet+lastRaise && !allIn {
				return RaiseOutOfRange
			}
			if to > maxBet {
				lastRaise = to - maxBet
			}
			own[s] = to
			maxBet = max(maxBet, to)
		}
		invested[s] += a.Amount
		if a.AllIn && known[s] && invested[s] != stack[s] {
			return AllInNotStack
		}
	}
	return AmountsOK
}
