package hh

// PostViolation says which blind or ante post is missing from a hand.
type PostViolation uint8

const (
	// PostsOK means no blind or ante post is missing.
	PostsOK PostViolation = iota
	// MissingSmallBlind means the small blind's post line is missing.
	MissingSmallBlind
	// MissingBigBlind means the big blind's post line is missing.
	MissingBigBlind
	// MissingAnte means a player who plays the hand has no ante post although
	// others do.
	MissingAnte
	// LostDeadBlind means a dead blind was recorded as $0.00, so the raise and
	// the pot after it come out short by that amount, and they still agree with
	// each other.
	LostDeadBlind
	// WrongBlindSeat means a blind was posted by a player who was not due to
	// post it, for example the dealer posting the big blind with three or more
	// players, or the big blind posting the small blind heads-up.
	WrongBlindSeat
	// BlindPostedTwice means the small blind or the big blind was posted more
	// than once in the hand, as when a post line is repeated.
	BlindPostedTwice
	// NoBlinds means a hand that is not a bomb pot has no small or big blind
	// post at all. The pot lines then leave the blinds out too, so the file is
	// consistent, but nothing says who was due to post or how much: the hand
	// cannot be put right.
	NoBlinds
)

func (v PostViolation) String() string {
	switch v {
	case PostsOK:
		return "no blind or ante post is missing"
	case MissingSmallBlind:
		return "the small blind's post is missing"
	case MissingBigBlind:
		return "the big blind's post is missing"
	case MissingAnte:
		return "a player's ante post is missing"
	case LostDeadBlind:
		return "a dead blind was recorded as $0.00, its amount is lost"
	case WrongBlindSeat:
		return "a blind was posted by the wrong seat"
	case BlindPostedTwice:
		return "a blind is posted twice"
	case NoBlinds:
		return "no blind is posted in a hand that is not a bomb pot"
	}
	return "unknown post violation"
}

// CheckPosts checks that the blinds and antes were posted as the seating
// requires, so that a missing post is found even when the hand has no pot
// lines to add up.
//
// The small blind belongs to the first player dealt in after the dealer and
// the big blind to the next. Heads-up, the dealer posts the small blind. If the
// big blind sits directly next to the dealer, no small blind is owed. When a
// hand has antes, every player who plays it must have posted one. A player who
// entered the game and posted a dead or combined blind has posted.
//
// A missing post is a missing action: the line was left out of the file.
func (h *HandHistory) CheckPosts() PostViolation {
	for i := range h.Actions {
		if h.Actions[i].Kind == PostDead && h.Actions[i].Amount == 0 {
			return LostDeadBlind
		}
	}
	var smallPosts, bigPosts int
	for i := range h.Actions {
		switch h.Actions[i].Kind {
		case PostSmallBlind:
			smallPosts++
		case PostBigBlind:
			bigPosts++
		}
	}
	if smallPosts > 1 || bigPosts > 1 {
		return BlindPostedTwice
	}
	if smallPosts == 0 && bigPosts == 0 && h.Type != Bomb {
		return NoBlinds
	}
	if kind, _, _ := h.missingBlind(); kind == PostSmallBlind {
		return MissingSmallBlind
	} else if kind == PostBigBlind {
		return MissingBigBlind
	}
	if len(h.missingAnteSeats()) > 0 {
		return MissingAnte
	}
	if h.wrongBlindSeat() {
		return WrongBlindSeat
	}
	return PostsOK
}

// wrongBlindSeat reports whether the small or big blind was posted by a seat
// that was not due to post it. The small blind is due from the first dealt-in
// seat after the dealer and the big blind from the next; heads-up the dealer
// posts the small blind. When no small blind was posted, the big blind may sit
// directly next to the dealer.
func (h *HandHistory) wrongBlindSeat() bool {
	if h.sitOut {
		return false // the blinds moved on from the player who sat out
	}
	sbSeat, bbSeat := -1, -1
	for i := range h.Actions {
		switch h.Actions[i].Kind {
		case PostSmallBlind:
			sbSeat = h.Actions[i].Seat
		case PostBigBlind:
			bbSeat = h.Actions[i].Seat
		}
	}
	if sbSeat < 0 && bbSeat < 0 {
		return false
	}
	wantSB, wantBB, ok := h.dueBlinds()
	if !ok {
		return false
	}
	if sbSeat >= 0 && sbSeat != wantSB {
		return true
	}
	if bbSeat >= 0 {
		if sbSeat >= 0 {
			return bbSeat != wantBB
		}
		return bbSeat != wantBB && bbSeat != wantSB
	}
	return false
}

// dueBlinds returns the seats due to post the small and the big blind, going by
// who is dealt in and where the button is.
func (h *HandHistory) dueBlinds() (sb, bb int, ok bool) {
	seats := h.dealtInSeats()
	n := len(seats)
	if n < 2 {
		return 0, 0, false
	}
	b := n - 1 // the dealer: the dealt-in seat at or before the button seat
	for i, s := range seats {
		if s <= h.ButtonSeat {
			b = i
		}
	}
	seq := func(i int) int { return seats[(b+1+i)%n] }
	if n == 2 {
		return seq(1), seq(0), true
	}
	return seq(0), seq(1), true
}

// hasAnyPost reports whether the seat made a blind, dead or other post.
func (h *HandHistory) hasAnyPost(seat int) bool {
	for i := range h.Actions {
		a := &h.Actions[i]
		if a.Seat != seat {
			continue
		}
		switch a.Kind {
		case PostSmallBlind, PostBigBlind, PostStraddle, PostDead, PostEntry:
			return true
		}
	}
	return false
}

// missingBlind finds a blind post that is missing when the other blind was
// posted. It returns the kind (PostSmallBlind or PostBigBlind), the seat that
// should have posted, and the index where the post belongs. The kind is 0 if
// nothing is missing.
func (h *HandHistory) missingBlind() (kind ActionKind, seat, at int) {
	sbSeat, bbSeat, bbIdx, sbIdx := -1, -1, -1, -1
	for i := range h.Actions {
		switch h.Actions[i].Kind {
		case PostSmallBlind:
			sbSeat, sbIdx = h.Actions[i].Seat, i
		case PostBigBlind:
			bbSeat, bbIdx = h.Actions[i].Seat, i
		}
	}
	if (sbSeat < 0) == (bbSeat < 0) {
		return 0, 0, 0 // both posted, or neither: nothing to anchor on
	}
	seats := h.dealtInSeats()
	n := len(seats)
	if n < 2 {
		return 0, 0, 0
	}
	b := n - 1 // the dealer: the dealt-in seat at or before the button seat
	for i, s := range seats {
		if s <= h.ButtonSeat {
			b = i
		}
	}
	seq := func(i int) int { return seats[(b+1+i)%n] }
	wantSB, wantBB := seq(0), seq(1)
	if n == 2 {
		wantSB, wantBB = seq(1), seq(0)
	}
	if sbSeat < 0 { // the small blind is missing
		if bbSeat != wantBB || h.hasAnyPost(wantSB) {
			return 0, 0, 0
		}
		return PostSmallBlind, wantSB, bbIdx
	}
	// the big blind is missing
	if sbSeat != wantSB || h.hasAnyPost(wantBB) {
		return 0, 0, 0
	}
	return PostBigBlind, wantBB, sbIdx + 1
}

// missingAnteSeats returns the seats that play the hand without an ante post,
// when somebody else posted one.
func (h *HandHistory) missingAnteSeats() []int {
	posted := map[int]bool{}
	any := false
	for i := range h.Actions {
		if h.Actions[i].Kind == PostAnte {
			posted[h.Actions[i].Seat] = true
			any = true
		}
	}
	if !any {
		return nil
	}
	var missing []int
	for _, seat := range h.dealtInSeats() {
		if !posted[seat] {
			missing = append(missing, seat)
		}
	}
	return missing
}

// hasAnte reports whether anyone posted an ante in the hand.
func (h *HandHistory) hasAnte() bool {
	for i := range h.Actions {
		if h.Actions[i].Kind == PostAnte {
			return true
		}
	}
	return false
}
