package hh

import (
	"errors"
	"iter"
	"math"
	"time"

	"github.com/seanq222/hh/pk"
)

// Position is a player's role in a hand relative to the dealer button.
//
// The numeric order is the preflop acting order, from UTG to BB. After the flop
// the order is SB, BB, then UTG up to BTN; see PostflopOrder.
type Position uint8

const (
	UTG  Position = iota + 1 // under the gun
	UTG1                     // UTG+1
	UTG2                     // UTG+2
	LJ                       // lojack
	HJ                       // hijack
	CO                       // cutoff
	BTN                      // button
	SB                       // small blind
	BB                       // big blind
)

var positionNames = [...]string{"?", "UTG", "UTG+1", "UTG+2", "LJ", "HJ", "CO", "BTN", "SB", "BB"}

func (p Position) String() string {
	if int(p) < len(positionNames) {
		return positionNames[p]
	}
	return "?"
}

// PostflopOrder returns a number that sorts positions into acting order after
// the flop: SB first and BTN last.
func (p Position) PostflopOrder() int {
	switch p {
	case SB:
		return 0
	case BB:
		return 1
	}
	return int(p) + 1
}

// midPositions names the seats between the big blind and the button, indexed
// by how many there are (players dealt in minus three).
var midPositions = [7][]Position{
	0: nil,
	1: {CO},
	2: {UTG, CO},
	3: {UTG, HJ, CO},
	4: {UTG, LJ, HJ, CO},
	5: {UTG, UTG1, LJ, HJ, CO},
	6: {UTG, UTG1, UTG2, LJ, HJ, CO},
}

// Player is one player in a PlayerHand.
type Player struct {
	Position Position
	HasCards bool
	Cards    pk.HoleCards // the hero's cards, or a villain's if shown and the hero reached showdown
	Stack    Cents        // starting stack
	// PackID is the pack the player's own hands went to, numbered by the build
	// that wrote the packs (hhpack lists them in the folder's ids.txt), or
	// NoPack if they went to none. It names no one, so a view can say who the
	// other players were without holding their names.
	PackID uint16
}

// NoPack is the PackID of a player whose hands went to no pack.
const NoPack uint16 = 0

// Event is one betting action.
type Event struct {
	Actor    uint8 // index into PlayerHand.Players
	Kind     ActionKind
	AllIn    bool
	Inferred bool  // the line was missing from the file and was reconstructed; see HandHistory.Repair
	Amount   Cents // chips this action added to the pot
}

// PlayerHand is one hand as seen by one player, the hero. It holds only what
// the hero was involved in and stops where the hero's part ends: a hero who
// folds sees nothing after the fold.
//
// Street, pot, amount to call and chips invested are not stored. They are
// calculated by replaying Events; see Steps, Invested and Net.
//
// The Players and Events slices may be shared between the views of one
// hand, so treat them as read-only.
type PlayerHand struct {
	Site   Site
	HandID int64
	Time   time.Time

	SmallBlind Cents
	BigBlind   Cents
	Type       HandType
	Hero       uint8 // index into Players

	// Reached is the last street the hero saw. Board holds the cards dealt by
	// then: none preflop, 3 on the flop, 4 on the turn, 5 on the river. See
	// BoardCards.
	Reached Street
	Board   [5]pk.Card

	Players []Player // everyone dealt in, sorted by Position (preflop acting order)
	Events  []Event  // preflop to the hero's last action, in order

	Won  Cents // chips the hero won, from the summary
	Rake Cents // zero if the hero folded, since the pot is then unknown

	// WonEV is what the hero is expected to win, the counterpart of Won: Won with
	// the luck of an all-in runout taken out (see ExpectedNet), to the nearest
	// cent. It is worked out once when the view is made, since that is slow for an
	// all-in. It differs from Won only in a hand valued by equity, and not always
	// then: one whose result was settled, as when a player was drawing dead, comes
	// out at Won. NetEV is WonEV less what the hero put in.
	WonEV Cents
}

// BoardCards returns the board cards the hero saw. The slice is a view of p, so
// it is valid only as long as p is.
func (p *PlayerHand) BoardCards() pk.Board { return p.Board[:cardsAtStreet[p.Reached]] }

// setBoard stores the board cards b, which must be 0, 3, 4 or 5 of them.
func (p *PlayerHand) setBoard(b pk.Board) {
	copy(p.Board[:], b)
	for s := River; s > Preflop; s-- {
		if len(b) >= cardsAtStreet[s] {
			p.Reached = s
			return
		}
	}
	p.Reached = Preflop
}

// ErrNotPlayable is returned for a hand that cannot be turned into player
// views, for example one with fewer than two or more than nine players dealt in.
var ErrNotPlayable = errors.New("hh: hand cannot be turned into player views")

// ErrInconsistent is returned for a hand whose actions cannot be replayed into
// the streets in the file, usually because an action line is missing.
var ErrInconsistent = errors.New("hh: hand log is inconsistent, an action is probably missing")

var cardsAtStreet = [...]int{Preflop: 0, Flop: 3, Turn: 4, River: 5}

// PlayerHands returns one view for every player dealt in, that is every player
// who posted or acted. Views are in position order, so view i is the hand as
// seen by Players[i].
func (h *HandHistory) PlayerHands() ([]PlayerHand, error) {
	views, _, err := h.playerHands(nil)
	return views, err
}

// NamedPlayerHands is PlayerHands with the name of the player each view belongs
// to: names[i] is the player who is the hero of views[i].
func (h *HandHistory) NamedPlayerHands() (views []PlayerHand, names []string, err error) {
	return h.NamedPlayerHandsFor(nil)
}

// NamedPlayerHandsFor is NamedPlayerHands for only the players want accepts; a
// nil want accepts everyone. Views and their expected winnings are worked out
// only for those players, so ask for no more than you need.
func (h *HandHistory) NamedPlayerHandsFor(want func(name string) bool) (views []PlayerHand, names []string, err error) {
	var accept func(*Seat) bool
	if want != nil {
		accept = func(p *Seat) bool { return want(p.Name) }
	}
	views, seats, err := h.playerHands(accept)
	if err != nil {
		return nil, nil, err
	}
	names = make([]string, len(seats))
	for i, seat := range seats {
		if p := h.PlayerBySeat(seat); p != nil {
			names[i] = p.Name
		}
	}
	return views, names, nil
}

// PlayerHandFor returns the view for the named player. It returns
// ErrNotPlayable if the player was not dealt in, and the errors of PlayerHands
// if the hand cannot be turned into views.
func (h *HandHistory) PlayerHandFor(name string) (PlayerHand, error) {
	p := h.PlayerByName(name)
	if p == nil {
		return PlayerHand{}, ErrNotPlayable
	}
	seat := p.Seat
	views, _, err := h.playerHands(func(q *Seat) bool { return q.Seat == seat })
	if err != nil {
		return PlayerHand{}, err
	}
	if len(views) == 0 {
		return PlayerHand{}, ErrNotPlayable
	}
	return views[0], nil
}

// playerHands builds the views of the players dealt in that want accepts (all of
// them if want is nil), in position order, and also returns the seat of each.
func (h *HandHistory) playerHands(want func(*Seat) bool) ([]PlayerHand, []int, error) {
	np := len(h.Players)
	if np > 16 {
		return nil, nil, ErrNotPlayable
	}
	// seat index of each action, and who was dealt in.
	var dealt [16]bool
	actor := make([]int8, len(h.Actions))
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
			return nil, nil, ErrNotPlayable
		}
		actor[i] = int8(pi)
		dealt[pi] = true
	}

	// order lists the dealt-in players by seat number.
	var orderBuf [16]int
	order := orderBuf[:0]
	for i := 0; i < np; i++ {
		if dealt[i] {
			order = append(order, i)
		}
	}
	n := len(order)
	if n < 2 || n > 9 {
		return nil, nil, ErrNotPlayable
	}
	for i := 1; i < n; i++ { // insertion sort by seat
		for j := i; j > 0 && h.Players[order[j]].Seat < h.Players[order[j-1]].Seat; j-- {
			order[j], order[j-1] = order[j-1], order[j]
		}
	}

	// The button is the dealt-in seat at or just before the button seat.
	b := n - 1
	for i, pi := range order {
		if h.Players[pi].Seat <= h.ButtonSeat {
			b = i
		}
	}
	// seq is clockwise from the seat after the button; the button comes last.
	var seqBuf [16]int
	seq := seqBuf[:n]
	for i := 0; i < n; i++ {
		seq[i] = order[(b+1+i)%n]
	}
	var pos [16]Position
	if n == 2 {
		pos[seq[0]], pos[seq[1]] = BB, BTN
	} else {
		pos[seq[0]], pos[seq[1]], pos[seq[n-1]] = SB, BB, BTN
		for j, m := range midPositions[n-3] {
			pos[seq[2+j]] = m
		}
	}

	// slots lists the players sorted by position; slotOf inverts it.
	slots := orderBuf[:n]
	for i := 1; i < n; i++ {
		for j := i; j > 0 && pos[slots[j]] < pos[slots[j-1]]; j-- {
			slots[j], slots[j-1] = slots[j-1], slots[j]
		}
	}
	var slotOf [16]uint8
	for s, pi := range slots {
		slotOf[pi] = uint8(s)
	}

	events := make([]Event, len(h.Actions))
	for i := range h.Actions {
		a := &h.Actions[i]
		events[i] = Event{Actor: slotOf[actor[i]], Kind: h.eventKind(i), AllIn: a.AllIn, Inferred: a.Inferred, Amount: a.Amount}
	}
	board := append(pk.Board(nil), h.Board...)

	// full holds every known card; it is what a hero who stays in sees.
	full := make([]Player, n)
	for s, pi := range slots {
		sp := &h.Players[pi]
		full[s] = Player{Position: pos[pi], Stack: sp.Stack, HasCards: sp.HasCards}
		if sp.HasCards {
			full[s].Cards = pk.HoleCardsFromCards(sp.Cards[0], sp.Cards[1])
		}
	}
	var blank []Player // for heroes who fold; built when first needed

	// Streets are inferred from the events, so refuse a hand whose log cannot
	// be replayed into the streets the parser saw. That happens when an action
	// is missing, which the files do now and then.
	if !h.streetsMatch(events, n) {
		return nil, nil, ErrInconsistent
	}

	views := make([]PlayerHand, 0, n)
	seatOf := make([]int, 0, n)
	for s, pi := range slots {
		sp := &h.Players[pi]
		if want != nil && !want(sp) {
			continue
		}
		seatOf = append(seatOf, sp.Seat)
		views = append(views, PlayerHand{})
		v := &views[len(views)-1]
		*v = PlayerHand{
			Site:       h.Site,
			HandID:     h.ID,
			Time:       h.Time,
			SmallBlind: h.SmallBlind,
			BigBlind:   h.BigBlind,
			Type:       h.Type,
			Hero:       uint8(s),
		}
		foldAt := -1
		for i := range h.Actions {
			if h.Actions[i].Kind == Fold && int(actor[i]) == pi {
				foldAt = i
				break
			}
		}
		if foldAt < 0 {
			v.Players = full
			v.setBoard(board)
			v.Events = events[:len(events):len(events)]
			v.Won = sp.Won
			v.Rake = h.Rake + h.JPFee
			continue
		}
		if blank == nil {
			blank = make([]Player, n)
			for t, pj := range slots {
				blank[t] = Player{Position: pos[pj], Stack: h.Players[pj].Stack}
			}
		}
		v.Players = blank
		if sp.HasCards { // only your own histories know a folded hand's cards
			own := append([]Player(nil), blank...)
			own[s].HasCards, own[s].Cards = true, pk.HoleCardsFromCards(sp.Cards[0], sp.Cards[1])
			v.Players = own
		}
		k := min(cardsAtStreet[h.Actions[foldAt].Street], len(board))
		v.setBoard(board[:k])
		v.Events = events[: foldAt+1 : foldAt+1]
	}
	for i := range views {
		v := &views[i]
		net, _ := v.ExpectedNet()
		v.WonEV = Cents(math.Round(net)) + v.Invested()
	}
	return views, seatOf, nil
}

// Step is one event with the state derived by replaying the hand up to it.
type Step struct {
	Street      Street
	Event       Event
	Pot         Cents // the pot before the event
	ToCall      Cents // what the actor needed to put in to call; 0 if they could check
	StreetTotal Cents // the actor's total bet on this street after the event
}

// Steps replays the events and works out the street, pot and amount to call.
//
// A street ends when every player still in has either gone all-in or has acted
// since the last bet or raise and matched the highest bet. Antes and dead
// blinds go into the pot but do not count toward a player's bet on the street.
// In a bomb pot the antes are followed straight by the flop.
func (p *PlayerHand) Steps() iter.Seq[Step] {
	return replay(p.Events, len(p.Players), p.BigBlind, p.Site)
}

// replay is Steps for events by n players, whose Actor is 0 to n-1.
// HandHistory.StreetsReplayable uses it on the actions without building views.
func replay(events []Event, n int, bigBlind Cents, site Site) iter.Seq[Step] {
	return func(yield func(Step) bool) {
		var total [16]Cents
		var folded, allIn, acted [16]bool
		var pot, maxBet Cents
		street := Preflop
		sawAnte := false
		for _, e := range events {
			// In a bomb pot the antes are followed straight by the flop. The
			// antes, not the "BombPot" line, mark one: the line also appears
			// in some hands that were dealt normally.
			switch e.Kind {
			case PostAnte:
				sawAnte = true
			case PostSmallBlind, PostBigBlind, PostStraddle, PostEntry, PostDead:
				sawAnte = false // antes plus blinds is an ordinary hand with antes
			default:
				if sawAnte && street == Preflop {
					street = Flop
				}
				sawAnte = false
			}
			a := int(e.Actor)
			toCall := max(maxBet-total[a], 0)
			st := Step{Street: street, Event: e, Pot: pot, ToCall: toCall}
			switch e.Kind {
			case PostAnte, PostDead:
				pot += e.Amount
			case PostSmallBlind, PostBigBlind, PostStraddle:
				total[a] += e.Amount
				pot += e.Amount
				maxBet = max(maxBet, total[a])
			case PostEntry:
				// How much of a post on joining the game is live depends on the
				// site; see LiveAmount.
				total[a] += LiveAmount(site, PostEntry, e.Amount, bigBlind)
				pot += e.Amount
				maxBet = max(maxBet, total[a])
			case Call:
				total[a] += e.Amount
				pot += e.Amount
				acted[a] = true
			case Bet, Raise:
				total[a] += e.Amount
				pot += e.Amount
				if total[a] > maxBet {
					maxBet = total[a]
					for i := 0; i < n; i++ {
						acted[i] = false
					}
				}
				acted[a] = true
			case Check:
				acted[a] = true
			case Fold:
				folded[a] = true
			}
			if e.AllIn {
				allIn[a] = true
			}
			st.StreetTotal = total[a]
			if !yield(st) {
				return
			}
			if street == River {
				continue
			}
			in, closed := 0, true
			for i := 0; i < n; i++ {
				if folded[i] {
					continue
				}
				in++
				if !allIn[i] && (!acted[i] || total[i] != maxBet) {
					closed = false
				}
			}
			if in >= 2 && closed {
				street++
				total = [16]Cents{}
				acted = [16]bool{}
				maxBet = 0
			}
		}
	}
}

// Invested returns the chips the hero put into the pot, net of the uncalled
// bet that went back to them, if any. That bet is not stored; it is worked out
// from the events. A hero who folded gets nothing back.
func (p *PlayerHand) Invested() Cents {
	var s Cents
	for _, e := range p.Events {
		if e.Actor == p.Hero {
			s += e.Amount
		}
	}
	if n := len(p.Events); n > 0 && p.Events[n-1].Kind == Fold && p.Events[n-1].Actor == p.Hero {
		return s
	}
	if actor, amount, ok := p.UncalledBet(); ok && actor == p.Hero {
		if p.walk() {
			// Everyone folded to the big blind. The site's own client keeps the
			// big blind's excess in the pot, so the winner's amount includes it
			// and nothing is taken back out. Other files take it out of the
			// pot, so the winner's amount is smaller by that much. The amount
			// the hero won says which it is.
			var total Cents
			for _, e := range p.Events {
				total += e.Amount
			}
			if p.Won+p.Rake == total {
				return s
			}
		}
		s -= amount
	}
	return s
}

// walk reports whether nobody made a voluntary bet, so everyone folded to the
// big blind.
func (p *PlayerHand) walk() bool {
	for _, e := range p.Events {
		switch e.Kind {
		case Call, Bet, Raise:
			return false
		}
	}
	return true
}

// Showdown reports whether the hero went to showdown: the hero did not fold,
// and at least one other player had not folded when the betting ended. Whether
// they won, lost or chopped is in Won.
func (p *PlayerHand) Showdown() bool {
	var folded [16]bool
	for _, e := range p.Events {
		if e.Kind == Fold {
			folded[e.Actor] = true
		}
	}
	if folded[p.Hero] {
		return false
	}
	in := 0
	for i := range p.Players {
		if !folded[i] {
			in++
		}
	}
	return in >= 2
}

// UncalledBet works out which player gets a bet back and how much, from the
// events alone. On the final street the highest bet is uncalled to the extent
// that it exceeds the next highest. ok is false when a player who has not
// folded or gone all-in still owes a call, which means an event is missing.
func (p *PlayerHand) UncalledBet() (actor uint8, amount Cents, ok bool) {
	if len(p.Events) == 0 {
		return 0, 0, false
	}
	last := Preflop
	for st := range p.Steps() {
		last = st.Street
	}
	var own, invested [16]Cents
	var folded, allIn, seen [16]bool
	for st := range p.Steps() {
		a := int(st.Event.Actor)
		seen[a] = true
		invested[a] += st.Event.Amount
		if st.Event.Kind == Fold {
			folded[a] = true
		}
		if st.Event.AllIn {
			allIn[a] = true
		}
		if st.Street == last {
			own[a] = st.StreetTotal
		}
	}
	top, second, topActor := Cents(0), Cents(0), -1
	for a, v := range own {
		switch {
		case v > top:
			second, top, topActor = top, v, a
		case v > second:
			second = v
		}
	}
	for a := range seen {
		if !seen[a] || a == topActor || folded[a] || allIn[a] || own[a] >= top {
			continue
		}
		if a < len(p.Players) && p.Players[a].Stack > 0 && invested[a] >= p.Players[a].Stack {
			continue // all their chips are in
		}
		return 0, 0, false
	}
	if topActor < 0 {
		return 0, 0, true
	}
	return uint8(topActor), top - second, true
}

// Net returns what the hero won minus what they invested.
func (p *PlayerHand) Net() Cents { return p.Won - p.Invested() }

// NetEV returns what the hero is expected to win minus what they invested, the
// counterpart of Net; see WonEV.
func (p *PlayerHand) NetEV() Cents { return p.WonEV - p.Invested() }

// RakePaid returns the rake and jackpot fee the hero paid. The site takes it
// from the pot, so it comes out of the winners' shares: a hero who won the whole
// pot paid all of it, one who chopped paid their share of it, and one who won
// nothing paid none. p.Rake is the whole hand's rake. A view knows only its own
// winnings, so the shares of a split pot are each rounded on their own and can
// miss the rake by a cent; RakeShares, given every winner's, does not.
func (p *PlayerHand) RakePaid() Cents {
	if p.Won <= 0 || p.Rake <= 0 {
		return 0
	}
	// The pot the winners shared: the chips that went in, less the uncalled bet
	// that went back, less the rake.
	var chips Cents
	for _, e := range p.Events {
		chips += e.Amount
	}
	if _, amount, ok := p.UncalledBet(); ok {
		chips -= amount
	}
	pot := chips - p.Rake
	if pot <= p.Won {
		return p.Rake
	}
	return Cents(math.Round(float64(p.Rake) * float64(p.Won) / float64(pot)))
}

// RakeShares shares rake out among the players in proportion to what each won,
// into shares, so that the shares add up to rake exactly: each gets the share
// rounded down, and the cents left over go to the players with the largest
// fractions, the earlier position first when two are equal. Players who won
// nothing pay nothing.
func RakeShares(won []Cents, rake Cents, shares []Cents) {
	clear(shares)
	var total int64
	for _, w := range won {
		if w > 0 {
			total += int64(w)
		}
	}
	if total == 0 || rake <= 0 {
		return
	}
	var rem [16]int64
	left := int64(rake)
	for i, w := range won {
		if w > 0 {
			exact := int64(rake) * int64(w)
			shares[i] = Cents(exact / total)
			rem[i] = exact % total
			left -= exact / total
		}
	}
	for ; left > 0; left-- {
		best := -1
		for i, w := range won {
			if w > 0 && (best < 0 || rem[i] > rem[best]) {
				best = i
			}
		}
		shares[best]++
		rem[best] = -1 // one cent each at most
	}
}
