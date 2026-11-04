package hh

import "slices"

// Repair reconstructs action lines that the file left out, when the rest of the
// hand pins down exactly what the line must have been. It returns how many
// actions it inserted. Each inserted action has Inferred set.
//
// Repair is for a hand that has failed a check: the caller checks the hand first,
// calls Repair only if a check failed, and checks the hand again afterwards.
// Repair does not decide that a hand needs repairing and the checks know nothing
// of it. It works out for itself what happened in the hand, with its own replay of
// the betting, and keeps its changes only if the hand then passes the pot,
// returned-bet and turn-order checks (see ChipsConsistent, CheckReturn and
// CheckTurnOrder). Otherwise the hand is left exactly as it was. Several missing
// lines in one hand are repaired together, all or nothing.
//
// Repaired so far:
//
//   - A small or big blind post that is missing. The small blind belongs to
//     the first player dealt in after the dealer and the big blind to the
//     next; if the big blind sits next to the dealer, no small blind is owed.
//   - An ante post that is missing for a player who plays the hand.
//   - A bet or raise that is missing between a player's bet, raise or check
//     and their own fold. The file then shows a player acting twice in a row,
//     and the uncalled bet that is returned names the missing bettor. Their
//     bet took their total on the street to the folder's total plus the amount
//     returned. It is a Bet if nobody had bet on the street, else a Raise.
//   - The action of a player whose turn passed without a line, or who owed a
//     call or a check when the street ended: a fold, check or call, or a bet or
//     raise when the returned bet, the player's own later call, or the next call
//     by another player shows one. See fillSkipped. Where the hand does not say
//     which, each is tried and the one that makes the whole hand consistent is
//     kept.
//   - A dead blind post recorded as $0.00 next to the post that carries the
//     money, and a sitting-out mark on a player who posts or acts.
//   - Checks by a player who had put their whole stack in, written after a call
//     that did not say "and is all-in", and the checks on a street where at most
//     one player still had chips. See dropImpossibleChecks.
//   - A check by the only player left with chips who has nothing to call, such
//     as a big blind whose opponents are all-in for no more. See loneChecks.
//   - A name at two seats: the seat the player left is dropped. See
//     dropDuplicateSeats.
func (h *HandHistory) Repair() int {
	// The repairs are made on a copy, and the hand is replaced by it only if they
	// made the hand consistent.
	c := h.clone()
	n := c.repairSteps()
	if n == 0 || !c.repaired() {
		return 0
	}
	*h = *c
	return n
}

// repairSteps makes the repairs, one after another, and returns how many changes it
// made. It changes the hand it is called on, whether or not the result is good.
func (h *HandHistory) repairSteps() int {
	// What a raise adds follows from its "to" figure, and a showdown needs two
	// players. These come first, so that the repairs below see the hand as it was
	// played, and again after them, since a post that was put back can change them.
	n := h.fixRaiseAmounts()
	if h.fixShowdown() {
		n++
	}
	n += h.fixStaleSummary() + h.dropDuplicateSeats()
	n += h.dropEmptyDeadPosts() + h.clearStaleSitOut() + h.dropImpossibleChecks()
	n += h.insertMissingBlinds() + h.insertMissingAntes()
	n += h.fixRaiseAmounts()
	// A hand with no summary gets its result from the actions, once the posts are in
	// so that the uncalled bet is right.
	if h.completeNoSummary() {
		n++
	}
	for pass := 0; pass < 4; pass++ {
		if !h.repairOnce() {
			break
		}
		n++
	}
	before := len(h.Actions)
	if h.fillSkipped(6) {
		n += len(h.Actions) - before
	}
	// After the filling, which puts back a check owed by a player who had no one
	// left to bet against.
	n += h.dropLoneChecks()
	// Last, since they depend on the actions and on who is still in.
	n += h.fixPots()
	if h.fixShowdown() {
		n++
	}
	return n
}

// repaired reports whether the hand now passes what Repair is after: the pot,
// returned-bet and turn-order checks. Whether the betting rounds replay into the
// streets the file shows is for the caller to check afterwards, with the rest.
func (h *HandHistory) repaired() bool {
	return !h.NoSummary && h.consistent() && h.WinningsMatch()
}

// consistent reports whether the pot lines, the returned bet and the order of
// play all agree with the actions. A missing bet and the returned-bet line for
// it cancel in the pot arithmetic, so the pot lines alone would not notice it.
func (h *HandHistory) consistent() bool {
	return h.ChipsConsistent() && h.CheckReturn() == ReturnOK && h.CheckTurnOrder() == TurnOK
}

func (h *HandHistory) seatIndex(seat int) int {
	for i := range h.Players {
		if h.Players[i].Seat == seat {
			return i
		}
	}
	return -1
}

// repairOnce looks for the first player who acts twice in a row and tries to
// insert the missing raise. It reports whether it changed the hand.
func (h *HandHistory) repairOnce() bool {
	if len(h.Players) > 16 {
		return false
	}
	var own [16]Cents // each player's bet so far on the current street
	street := Preflop
	last := -1 // index of the previous betting action on this street
	for i := range h.Actions {
		a := &h.Actions[i]
		if a.Street != street {
			street = a.Street
			own = [16]Cents{}
			last = -1
		}
		pi := h.seatIndex(a.Seat)
		if pi < 0 {
			return false
		}
		switch a.Kind {
		case PostSmallBlind, PostBigBlind, PostStraddle:
			own[pi] += a.Amount
		case PostEntry:
			own[pi] += min(a.Amount, h.BigBlind)
		case Fold, Check, Call, Bet, Raise:
			if last >= 0 && h.Actions[last].Seat == a.Seat {
				// This player acted twice in a row; someone acted in between.
				prev := &h.Actions[last]
				if a.Kind == Fold && (prev.Kind == Bet || prev.Kind == Raise || prev.Kind == Check) {
					return h.insertMissingRaise(i, own)
				}
				return false
			}
			if a.Kind == Call || a.Kind == Bet || a.Kind == Raise {
				own[pi] += a.Amount
			}
			last = i
		}
	}
	return false
}

// insertMissingRaise inserts, before action i, the raise that must have been
// made against the bet that action i folds to. own holds each player's bet on
// the street before action i.
func (h *HandHistory) insertMissingRaise(i int, own [16]Cents) bool {
	x := &h.Actions[i]
	xi := h.seatIndex(x.Seat)
	// The returned uncalled bet names the raiser.
	ret := h.Returned
	if ret.Amount == 0 || ret.Street != x.Street || ret.Seat == x.Seat {
		return false
	}
	yi := h.seatIndex(ret.Seat)
	if yi < 0 {
		return false
	}
	to := own[xi] + ret.Amount // the raiser's total bet on the street
	add := to - own[yi]        // the chips the raise put in
	if add <= 0 {
		return false
	}
	// Is the raiser all-in? Compare everything they have put in with their stack.
	var invested Cents
	for j := 0; j < i; j++ {
		a := &h.Actions[j]
		if a.Seat != ret.Seat {
			continue
		}
		invested += a.Amount
	}
	kind := Bet
	for k := range h.Players {
		if own[k] > 0 {
			kind = Raise // somebody had already put chips in on this street
			break
		}
	}
	inferred := Action{
		Street:   x.Street,
		Seat:     ret.Seat,
		Player:   ret.Player,
		Kind:     kind,
		Amount:   add,
		To:       to,
		AllIn:    invested+add == h.Players[yi].Stack,
		Inferred: true,
	}
	h.insertAction(i, inferred)
	return true
}

// insertAction inserts a at index i. A line saying a player sits out stays
// where it was among the other actions, so one written after action i now
// comes after one more.
func (h *HandHistory) insertAction(i int, a Action) {
	if a.Kind == Bet {
		a.To = 0 // a bet line says how much, not "to"
	}
	fixed := make([]Action, 0, len(h.Actions)+1)
	fixed = append(fixed, h.Actions[:i]...)
	fixed = append(fixed, a)
	fixed = append(fixed, h.Actions[i:]...)
	h.Actions = fixed
	h.SitOutLines = slices.Clone(h.SitOutLines) // a copy may share them
	for k := range h.SitOutLines {
		if h.SitOutLines[k].At > i {
			h.SitOutLines[k].At++
		}
	}
}

// removeAction removes the action at index i, keeping the lines saying a player
// sits out where they were among the other actions.
func (h *HandHistory) removeAction(i int) {
	h.Actions = slices.Delete(slices.Clone(h.Actions), i, i+1)
	h.SitOutLines = slices.Clone(h.SitOutLines)
	for k := range h.SitOutLines {
		if h.SitOutLines[k].At > i {
			h.SitOutLines[k].At--
		}
	}
}

// dealtInSeats returns the seats that were dealt in, in seat order: those that
// posted or acted, those the summary proves were dealt in, and those a bet was
// returned to.
func (h *HandHistory) dealtInSeats() []int {
	var seats []int
	seen := map[int]bool{}
	add := func(seat int) {
		if !seen[seat] {
			seen[seat] = true
			seats = append(seats, seat)
		}
	}
	for i := range h.Actions {
		add(h.Actions[i].Seat)
	}
	for i := range h.Players {
		if h.Players[i].dealtInPerSummary() {
			add(h.Players[i].Seat)
		}
	}
	for i := 1; i < len(seats); i++ {
		for j := i; j > 0 && seats[j] < seats[j-1]; j-- {
			seats[j], seats[j-1] = seats[j-1], seats[j]
		}
	}
	return seats
}

// insertMissingBlinds inserts the small or big blind post found by missingBlind.
func (h *HandHistory) insertMissingBlinds() int {
	kind, seat, at := h.missingBlind()
	if kind == 0 {
		return 0
	}
	pi := h.seatIndex(seat)
	if pi < 0 {
		return 0
	}
	amount := h.SmallBlind
	if kind == PostBigBlind {
		amount = h.BigBlind
	}
	if h.Players[pi].Stack < amount {
		return 0
	}
	h.insertAction(at, Action{Street: Preflop, Seat: seat, Player: h.Players[pi].Name,
		Kind: kind, Amount: amount, Inferred: true})
	return 1
}

// insertMissingAntes inserts an ante for every player found by missingAnteSeats.
func (h *HandHistory) insertMissingAntes() int {
	seats := h.missingAnteSeats()
	if len(seats) == 0 {
		return 0
	}
	var ante Cents
	last := -1
	for i := range h.Actions {
		if h.Actions[i].Kind == PostAnte {
			ante = max(ante, h.Actions[i].Amount)
			last = i
		}
	}
	n := 0
	for _, seat := range seats {
		pi := h.seatIndex(seat)
		if pi < 0 {
			continue
		}
		amt := min(ante, h.Players[pi].Stack)
		h.insertAction(last+1, Action{Street: Preflop, Seat: seat, Player: h.Players[pi].Name,
			Kind: PostAnte, Amount: amt, AllIn: amt == h.Players[pi].Stack, Inferred: true})
		last++
		n++
	}
	return n
}
