package pk

// boardClass returns the class of a board of 3 to 5 cards on its own. A board
// of fewer than five cards can only pair, so its class comes from its ranks.
func boardClass(board []Card) HandClass {
	if len(board) == 5 {
		return Evaluate5(board).Class()
	}
	var n [13]int
	pairs, trips := 0, 0
	for _, c := range board {
		n[c.Rank()]++
		switch n[c.Rank()] {
		case 2:
			pairs++
		case 3:
			pairs--
			trips++
		case 4:
			return FourOfAKind
		}
	}
	switch {
	case trips > 0:
		return ThreeOfAKind
	case pairs == 2:
		return TwoPair
	case pairs == 1:
		return OnePair
	}
	return HighCard
}

// Draws is what the next card can do for a hand on the flop or the turn.
type Draws struct {
	Class  HandClass // the hand's class now
	Unseen int       // the cards it has not seen, any of which can come next

	// Outs[c] is the number of next cards that make the hand class c or better
	// and better than the board alone with that card: cards that improve the
	// hand through its hole cards, not by improving the board for everyone. It
	// is 0 for the classes the hand already has.
	Outs [NumHandClasses]int
}

// NextCard works out the draws of a hand on a flop or turn board by trying every
// card that can come next.
func NextCard(hole HoleCards, board []Card) Draws {
	if len(board) != 3 && len(board) != 4 {
		panic("NextCard needs a flop or a turn")
	}
	seen := NewCardSet(hole[0], hole[1]).Insert(board[0])
	for _, c := range board[1:] {
		seen = seen.Insert(c)
	}
	// The hand with room for the next card, and the board likewise.
	var cards [7]Card
	cards[0], cards[1] = hole[0], hole[1]
	n := 2 + copy(cards[2:], board)
	var next [5]Card
	m := copy(next[:], board)
	d := Draws{Class: Evaluate(cards[:n]...).Class()}
	for x := Card(0); x < 52; x++ {
		if seen.Contains(x) {
			continue
		}
		d.Unseen++
		cards[n] = x
		mine := Evaluate(cards[:n+1]...).Class()
		next[m] = x
		if mine <= boardClass(next[:m+1]) {
			continue // the card helps the board, not the hand
		}
		for c := d.Class + 1; c <= mine; c++ {
			d.Outs[c]++
		}
	}
	return d
}

// Runouts counts how a hand on the flop ends up over every turn and river.
type Runouts struct {
	Total int // turn and river pairs: 1,081 with 47 cards unseen

	// Class[c] is the number of runouts after which the hand is class c, a
	// higher class than the board's own; Board is the number in which its class
	// is no higher than the board's.
	Class [NumHandClasses]int
	Board int
}

// RunoutsFromFlop works out every turn and river for a hand on the flop.
func RunoutsFromFlop(hole HoleCards, flop []Card) Runouts {
	if len(flop) != 3 {
		panic("RunoutsFromFlop needs a flop")
	}
	seen := NewCardSet(hole[0], hole[1], flop[0], flop[1], flop[2])
	var r Runouts
	// The hand, whose last five cards are the board.
	cards := [7]Card{hole[0], hole[1], flop[0], flop[1], flop[2]}
	for t := Card(0); t < 52; t++ {
		if seen.Contains(t) {
			continue
		}
		for v := t + 1; v < 52; v++ {
			if seen.Contains(v) {
				continue
			}
			cards[5], cards[6] = t, v
			r.Total++
			mine := Evaluate7(cards[:]...).Class()
			if mine <= Evaluate5(cards[2:]).Class() {
				r.Board++
				continue
			}
			r.Class[mine]++
		}
	}
	return r
}
