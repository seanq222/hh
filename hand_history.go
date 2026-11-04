// Package hh parses poker hand history text files: the formats written by the
// Winning Poker Network (ACR, and Hand2Note exports of it) and by CoinPoker. Each
// site has its own parser and writer (parse.go and format.go for WPN,
// parse_coinpoker.go and format_coinpoker.go for CoinPoker); they fill and write
// the same HandHistory, whose Site says which it came from, and the checks and
// repairs are shared.
//
// All money values are Cents.
package hh

import (
	"fmt"
	"slices"
	"time"

	"github.com/seanq222/hh/pk"
)

// Street is a betting round.
type Street uint8

const (
	Preflop Street = iota
	Flop
	Turn
	River
)

func (s Street) String() string {
	switch s {
	case Preflop:
		return "Pre-Flop"
	case Flop:
		return "Flop"
	case Turn:
		return "Turn"
	case River:
		return "River"
	}
	return fmt.Sprintf("Street(%d)", uint8(s))
}

// ActionKind is the type of a betting action.
type ActionKind uint8

const (
	Fold ActionKind = iota + 1
	Check
	Call
	Bet
	Raise
	PostSmallBlind
	PostBigBlind
	PostAnte
	// PostDead is a dead blind ("posts dead $X").
	PostDead
	// PostEntry is the post a player makes on entering the game or posting out
	// of turn, a bare "posts $X" line. Up to the big blind of it counts as a live
	// blind; anything above that is dead money.
	PostEntry
	// PostStraddle is a straddle ("posts straddle ₮2"): a third live blind, made
	// before the cards are dealt by the player after the big blind. It is live
	// like the big blind, so the first player to act before the flop is the one
	// after the straddler, and the straddler acts last.
	PostStraddle
)

func (k ActionKind) String() string {
	switch k {
	case Fold:
		return "fold"
	case Check:
		return "check"
	case Call:
		return "call"
	case Bet:
		return "bet"
	case Raise:
		return "raise"
	case PostSmallBlind:
		return "post small blind"
	case PostBigBlind:
		return "post big blind"
	case PostAnte:
		return "post ante"
	case PostDead:
		return "post dead"
	case PostEntry:
		return "post entry"
	case PostStraddle:
		return "post straddle"
	}
	return fmt.Sprintf("ActionKind(%d)", uint8(k))
}

// LiveAmount returns how much of an action's amount counts toward the player's
// bet on the street, which is what the next player must match: all of a blind,
// straddle, call, bet or raise, and none of an ante or dead post. A post on
// entering the game is live up to the big blind; more than that is a dead small
// blind posted with it. Every count of the bets uses this, so that the "to"
// figures and the returned bet follow from the amounts; RaiseToMatches checks
// that they do.
//
// Some files record a dead blind as "posts dead $0.00" and leave its amount in
// the next "posts" line, and take it out of the raise that follows and the pot,
// so that the money is lost. Such a hand cannot add up, and its raise's "to"
// figure is not what the chips come to, so RaiseToMatches leaves it out.
func LiveAmount(site Site, kind ActionKind, amount, bigBlind Cents) Cents {
	switch kind {
	case PostSmallBlind, PostBigBlind, PostStraddle, Call, Bet, Raise:
		return amount
	case PostEntry:
		return min(amount, bigBlind)
	}
	return 0
}

// Action is one line of betting.
//
// Amount is the chips this line adds to the pot: the call or bet size, or the
// blind or ante. A returned bet is not an action; see HandHistory.UncalledBet. For a
// Raise it is the first figure of
// "raises $X to $Y", which is the chips added now, not the size of the raise:
// a player who posted a $0.25 blind and "raises $1.00 to $1.25" has $1.25 in.
// To is the second figure, the player's total bet on the street, antes aside.
//
// In a hand with antes the first figure comes out short by the raiser's ante,
// so there Amount is worked out as To less what the player already had in on
// the street, which is what the pot needs.
type Action struct {
	Street Street
	Seat   int
	Player string
	Kind   ActionKind
	Amount Cents
	To     Cents
	AllIn  bool

	// Inferred is set on an action that was missing from the file and was
	// reconstructed by HandHistory.Repair.
	Inferred bool
}

// Seat is one seat at the table, with what the hand reveals about them.
type Seat struct {
	Seat  int
	Name  string
	Stack Cents // starting stack

	// Position is the label from the summary: "button", "small blind",
	// "big blind", or "" (the summary gives no label for other seats).
	Position string

	// Cards are the hole cards, in the order the file gives them. They are known
	// only if shown at showdown, or dealt to the hero in your own histories. The
	// views built from the hand (PlayerHand) hold them as pk.HoleCards, in the
	// library's order.
	HasCards bool
	Cards    [2]pk.Card

	// SittingOut is set for a seat listed as "is sitting out"; such a player is
	// not dealt in.
	SittingOut bool

	// PartialCard is set when only one card was revealed ("shows [- Jc]").
	// PartialFirst is set when it is written first ("shows [Jc -]").
	HasPartial   bool
	PartialCard  pk.Card
	PartialFirst bool

	Folded    bool
	FoldedOn  Street // valid if Folded
	Won       Cents  // amount won according to the summary, over both boards if the hand was run twice
	Showed    bool
	HandDescr string // e.g. "a pair of Aces"

	// WonSecond is the part of Won that came from the second board of a hand
	// that was run twice, and HandDescr2 is the hand's description on it.
	WonSecond  Cents
	HandDescr2 string

	// InSummary is set when the summary has a line for the seat, whatever it
	// says. A seat can be there with nothing but "did not show".
	InSummary bool
}

// SitOutLine is a line saying a player sits out, with the number of actions
// before it and whether it comes after the hole cards.
type SitOutLine struct {
	Text      string
	At        int
	AfterHole bool
}

// WaitingSeat is the seat line of a player waiting for the button.
type WaitingSeat struct {
	Seat int
	Text string
}

// HandType says how a hand was set up.
type HandType uint8

const (
	Regular HandType = iota // blinds, no antes
	Ante                    // antes with the blinds
	Bomb                    // a bomb pot: antes, then straight to the flop
)

func (t HandType) String() string {
	switch t {
	case Ante:
		return "Ante"
	case Bomb:
		return "Bomb"
	}
	return "Regular"
}

// Site is the poker site a hand was played on. Each site writes its hand
// histories in its own format, read by its own parser, into the same HandHistory.
type Site uint8

const (
	WPN       Site = iota // Winning Poker Network (ACR); the zero value
	CoinPoker             // CoinPoker, PokerStars-style text in USDT (₮)
)

func (s Site) String() string {
	switch s {
	case WPN:
		return "WPN"
	case CoinPoker:
		return "CoinPoker"
	}
	return fmt.Sprintf("Site(%d)", s)
}

// HandHistory is a single parsed hand.
type HandHistory struct {
	Site       Site
	ID         int64
	Game       string // e.g. "Holdem"
	Limit      string // e.g. "No Limit"
	SmallBlind Cents
	BigBlind   Cents
	Time       time.Time // UTC
	Table      string
	MaxSeats   int
	ButtonSeat int

	Players []Seat // in the order listed
	Hero    string // set when a "Dealt to" line is present

	// Type is Bomb for a bomb pot: everyone antes and the hand goes straight to
	// the flop. The summary's "BombPot" line alone does not make one. It also
	// appears in ordinary hands with blinds and no antes, and those are Regular,
	// so the line needs antes as well. Ante is a hand with antes and no label.
	Type HandType

	// Showdown is set when the hand reached a "SHOW DOWN" section with at
	// least two players still in. Some files print that section in every hand,
	// so the marker alone is not enough.
	Showdown bool

	Actions []Action
	Board   pk.Board

	// RunTwice is set when the players ran the board twice. Board is then the
	// first run, the flop, turn and river, and Board2 is the second. The second
	// run has the same flop unless the board was run twice before the flop.
	RunTwice bool
	Board2   pk.Board
	second   bool // the parser is in the second board's part of the summary
	sitOut   bool // a line says a player sat out after the seats were listed

	// SitOutLines are the lines that say a player sits out or waits for the big
	// blind, and Waiting the seat lines of players waiting for the button. The
	// parser keeps them so that String can write the hand again.
	SitOutLines []SitOutLine
	Waiting     []WaitingSeat

	// Returned is the "Uncalled bet returned" line the file wrote, if any. It
	// is a check, not a source of truth: the uncalled bet follows from the
	// actions, see UncalledBet, and CheckReturn compares the two. The files do
	// not always write the line.
	Returned Return

	TotalPot Cents
	Rake     Cents
	JPFee    Cents // the jackpot fee that also comes out of the pot; 0 if none

	// NoSummary is set when the file stops before the summary, so the winner and
	// the pot are not known. Only ParseHandPartial returns such a hand. FailedCheck
	// reports it, and Repair clears it when the actions say how the hand ended.
	NoSummary bool

	// Completed is set when Repair worked out the result of a hand that had no
	// summary from the actions.
	Completed bool

	// Pots[s] is the pot, rake included, as street s began, taken from the
	// "Main pot" line the file prints after each street marker. It is 0 for
	// streets without such a line. It lets you check that no action is missing.
	Pots [4]Cents

	// Rakes[s] is the rake, jackpot fee included, that the same "Main pot" line
	// shows for street s. It is kept so that String can write the line again.
	Rakes [4]Cents

	// StaleSummary holds the summary lines for a seat that name another player than
	// the seat list does, or a seat that is not in it. Some files print the summary
	// from a moment when the seat had another player. Nothing on such a line is in
	// the fields. FailedCheck reports them and Repair drops those it can show to be
	// harmless.
	StaleSummary []string

	// Unknown holds lines the parser did not recognise. The parser does not fail on
	// them, but FailedCheck does: what is on them is in no field.
	Unknown []string
}

// clone returns a copy of the hand that shares no slice with it.
func (h *HandHistory) clone() *HandHistory {
	c := *h
	c.Players = slices.Clone(h.Players)
	c.Actions = slices.Clone(h.Actions)
	c.Board = slices.Clone(h.Board)
	c.Board2 = slices.Clone(h.Board2)
	c.SitOutLines = slices.Clone(h.SitOutLines)
	c.Waiting = slices.Clone(h.Waiting)
	c.Unknown = slices.Clone(h.Unknown)
	return &c
}

// PlayerByName returns the named player, or nil.
func (h *HandHistory) PlayerByName(name string) *Seat {
	for i := range h.Players {
		if h.Players[i].Name == name {
			return &h.Players[i]
		}
	}
	return nil
}

// PlayerBySeat returns the player in a seat, or nil.
func (h *HandHistory) PlayerBySeat(seat int) *Seat {
	for i := range h.Players {
		if h.Players[i].Seat == seat {
			return &h.Players[i]
		}
	}
	return nil
}

// StreetsSeen reports how many streets were dealt: 1 is preflop only, 4 is
// through the river.
func (h *HandHistory) StreetsSeen() int {
	switch len(h.Board) {
	case 0:
		return 1
	case 3:
		return 2
	case 4:
		return 3
	default:
		return 4
	}
}

// ParseError describes a hand that could not be parsed.
type ParseError struct {
	HandID int64 // 0 if unknown
	Line   int   // 1-based line number in the input, 0 if unknown
	Text   string
	Msg    string

	// Defective is set when the hand parsed but was dropped because its actions
	// do not add up to the pot, so an action line is probably missing.
	Defective bool
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("hand %d line %d: %s: %q", e.HandID, e.Line, e.Msg, e.Text)
}

// dealtInPerSummary reports whether the summary proves the player was dealt in:
// it says they folded, showed cards or won. A bare "did not show" proves
// nothing, since it is also written for a player who sat down and is waiting
// for the big blind.
func (s *Seat) dealtInPerSummary() bool {
	return !s.SittingOut && (s.Folded || s.Showed || s.Won > 0)
}

// Return is an uncalled bet that went back to a player.
type Return struct {
	Seat   int
	Player string
	Amount Cents // 0 means there is no such line
	Street Street
}
