package hh

// The "Main pot $X | Rake $Y" line that a file prints after a street marker. A
// HandHistory keeps it in Pots and Rakes, and String prints those two fields as
// they are. PotLinesStable checks that they are the lines a reader of the written
// hand gets, and FixPotLines sets them.

// chipsBefore returns the chips in the pot as street s begins. When no one acts on
// street s or after it, the bet that goes back is not in the pot.
func (h *HandHistory) chipsBefore(s Street) Cents {
	var chips Cents
	later := false
	for i := range h.Actions {
		if h.Actions[i].Street < s {
			chips += h.Actions[i].Amount
		} else {
			later = true
		}
	}
	if !later {
		if _, amount, ok := h.UncalledBet(); ok {
			chips -= amount
		}
	}
	return chips
}

// lastMarker returns the last street whose marker String writes: the street of the
// last action, or a later street whose cards are on the board.
func (h *HandHistory) lastMarker() Street {
	last := Preflop
	if n := len(h.Actions); n > 0 {
		last = h.Actions[n-1].Street
	}
	for s := last + 1; s <= River; s++ {
		if len(h.Board) >= cardsAtStreet[s] {
			last = s
		}
	}
	return last
}

// potLines works out the pot and rake lines that String writes for the hand and a
// parser reads back, from the actions and the result.
//
// A street that has a pot line keeps one, with the pot as the street began and the
// rake it had. A hand that is run twice has none. The showdown prints a pot line of
// its own, which a parser takes for the line of the last street when that street
// had no betting and no line. Only WPN's files print pot lines; a hand from
// another site has none.
func (h *HandHistory) potLines() (pots, rakes [4]Cents) {
	if h.Site != WPN || h.RunTwice || h.NoSummary {
		return pots, rakes
	}
	last := h.lastMarker()
	for s := Flop; s <= last; s++ {
		if h.Pots[s] == 0 {
			continue
		}
		if pot, rake := h.chipsBefore(s), h.Rakes[s]; pot >= rake {
			pots[s], rakes[s] = pot, rake
		}
	}
	bet := false
	for i := range h.Actions {
		if h.Actions[i].Street == last {
			bet = true
		}
	}
	if h.Showdown && h.TotalPot > 0 && last > Preflop && !bet && pots[last] == 0 {
		pots[last], rakes[last] = h.TotalPot+h.Rake+h.JPFee, h.Rake+h.JPFee
	}
	return pots, rakes
}

// PotLinesStable reports whether the pot and rake lines that the hand has, Pots and
// Rakes, are the ones a reader of the written hand gets. They are not when a pot
// line in the file is stale on a street, as on a street dealt with no betting after
// an all-in, or when the pot line that the showdown prints will be read as the pot
// of the last street. The hand's winnings, stacks and actions do not depend on
// them, but Hand2Note reads the lines.
func (h *HandHistory) PotLinesStable() bool {
	pots, rakes := h.potLines()
	return h.Pots == pots && h.Rakes == rakes
}

// FixPotLines sets the hand's Pots and Rakes to the lines a reader of the written
// hand gets, see PotLinesStable.
func (h *HandHistory) FixPotLines() {
	h.Pots, h.Rakes = h.potLines()
}

// fixPots repairs what the pot lines of a hand get wrong, and returns how many
// kinds of change it made. A total pot that disagrees with the actions is set to
// what the winners were paid, when the winnings confirm the actions. If the pot
// lines of the streets still disagree with the actions they are dropped. Last, the
// lines that are left are set to the ones a reader of the written hand gets, see
// FixPotLines.
func (h *HandHistory) fixPots() int {
	n := 0
	if !h.ChipsConsistent() {
		var chips, paid Cents
		for i := range h.Actions {
			chips += h.Actions[i].Amount
		}
		for i := range h.Players {
			paid += h.Players[i].Won
		}
		_, uncalled, ok := h.UncalledBet()
		if !ok {
			uncalled = 0
		}
		if paid > 0 && paid+h.Rake+h.JPFee == chips-uncalled && h.TotalPot != paid {
			h.TotalPot = paid
			n++
		}
		if !h.ChipsConsistent() && h.Pots != ([4]Cents{}) {
			clear(h.Pots[:]) // the lines are written again from the actions
			n++
		}
	}
	pots, rakes := h.Pots, h.Rakes
	h.FixPotLines()
	if h.Pots != pots || h.Rakes != rakes {
		n++
	}
	return n
}
