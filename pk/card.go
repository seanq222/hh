package pk

import (
	"errors"
	"sort"
	"strings"
)

// Card represents a playing card. Bottom two bits are the suit, and next 4 are the rank
type Card byte

// HoldCards are sorted low to high
type HoleCards [2]Card
type Flop [3]Card
type Board []Card

const (
	NumHoleCardPairs = 51 * 52 / 2
)

var (
	HoleCardsArray [NumHoleCardPairs]HoleCards
)

func init() {
	for c0 := Card(0); c0 < 51; c0++ {
		for c1 := c0 + 1; c1 < 52; c1++ {
			hc := HoleCardsFromCards(c0, c1)
			HoleCardsArray[hc.Index()] = hc
		}
	}
}

// CardSet represents a set of cards. Each bit represents a card in the deck.
type CardSet uint64

// Card ranks
const (
	Ace   = 0
	King  = 1
	Queen = 2
	Jack  = 3
	Ten   = 4
	Nine  = 5
	Eight = 6
	Seven = 7
	Six   = 8
	Five  = 9
	Four  = 10
	Three = 11
	Two   = 12
)

// Card suits
const (
	Spade   = 0
	Heart   = 1
	Diamond = 2
	Club    = 3
)

const (
	RankChar = "AKQJT98765432"
	SuitChar = "shdc"
)

var (
	cardString [52]string
)

func init() {
	for c := 0; c < 52; c++ {
		r := c >> 2
		s := c & 0x3
		cardString[c] = string(RankChar[r]) + string(SuitChar[s])
	}
}

func NewCardSet(c ...Card) CardSet {
	cs := CardSet(0)
	for _, c := range c {
		cs |= 1 << c
	}
	return cs
}

func (cs CardSet) Contains(c Card) bool {
	return (cs & (1 << c)) != 0
}

func (cs CardSet) ContainsAny(c ...Card) bool {
	cs2 := NewCardSet(c...)
	return cs2&cs != 0
}

func (cs CardSet) ContainsAnyHoleCard(hc HoleCards) bool {
	return cs.Contains(hc[0]) || cs.Contains(hc[1])
}

func (cs CardSet) Insert(c Card) CardSet {
	return cs | (1 << c)
}

func CardFromRS(rank, suit int) Card {
	return Card(rank<<2 | suit)
}

func (c Card) String() string {
	return cardString[c]
}

func (c Card) Rank() int {
	return int(c) >> 2
}

func (c Card) Suit() int {
	return int(c) & 0x3
}

func HoleCardsFromCards(c0, c1 Card) HoleCards {
	if c0 == c1 {
		panic("duplicate hole cards")
	}
	if c0 > c1 {
		return HoleCards{c1, c0}
	}
	return HoleCards{c0, c1}
}

func (hc HoleCards) String() string {
	return cardString[hc[0]] + cardString[hc[1]]
}

func (hc HoleCards) Index() int {
	// c0*52 + c1 - (c0+1)*(c0+2)/2
	// (c0*104-c0*c0-3*c0-2)/2 + c1
	// (-c0*c0+101*c0-2)/2 + c1
	// ((101-c0)*c0)/2 + c1 - 1

	c0 := int(hc[0])
	c1 := int(hc[1])

	return (c0*(101-c0))/2 + c1 - 1
}

func (hc HoleCards) Less(hc2 HoleCards) bool {
	return hc[0] < hc2[0] || (hc[0] == hc2[0] && hc[1] < hc2[1])
}

func (hc HoleCards) Greater(hc2 HoleCards) bool {
	return hc[0] > hc2[0] || (hc[0] == hc2[0] && hc[1] > hc2[1])
}

func (hc HoleCards) Equal(hc2 HoleCards) bool {
	return hc[0] == hc2[0] && hc[1] == hc2[1]
}

func ParseCard(s string) (Card, error) {
	if len(s) != 2 {
		return 0, errors.New("invalid card string")
	}

	rank := strings.IndexByte(RankChar, s[0])
	suit := strings.IndexByte(SuitChar, s[1])

	if rank == -1 || suit == -1 {
		return 0, errors.New("invalid card string")
	}

	return CardFromRS(rank, suit), nil
}

func ParseHoleCards(s string) (HoleCards, error) {
	if len(s) != 4 {
		return HoleCards{}, errors.New("invalid hole cards string")
	}

	hc := HoleCards{}
	for i := 0; i < 2; i++ {
		card, err := ParseCard(s[i*2 : i*2+2])
		if err != nil {
			return HoleCards{}, err
		}
		hc[i] = card
	}

	if hc[0] == hc[1] {
		return HoleCards{}, errors.New("duplicate hole cards")
	}

	if hc[0] > hc[1] {
		hc[0], hc[1] = hc[1], hc[0]
	}

	return hc, nil
}

func (f Flop) String() string {
	return cardString[f[0]] + cardString[f[1]] + cardString[f[2]]
}

func ParseFlop(s string) (Flop, error) {
	if len(s) != 6 {
		return Flop{}, errors.New("invalid flop string")
	}
	flop := Flop{}
	for i := 0; i < 3; i++ {
		card, err := ParseCard(s[i*2 : i*2+2])
		if err != nil {
			return Flop{}, err
		}
		flop[i] = card
	}
	sort.Slice(flop[:], func(i, j int) bool {
		return flop[i] < flop[j]
	})
	return flop, nil
}

func (b Board) String() string {
	var sb strings.Builder
	for _, card := range b {
		sb.WriteString(card.String())
	}
	return sb.String()
}

func (b Board) Len() int {
	return len(b)
}

func ParseBoard(s string) (Board, error) {
	b := make(Board, 0, len(s)/2)

	for i := 0; i < len(s); i += 2 {
		card, err := ParseCard(s[i : i+2])
		if err != nil {
			return nil, err
		}
		b = append(b, card)
	}

	return b, nil
}

func EqualHands(a, b []Card) bool {
	if len(a) != len(b) {
		return false
	}

	for i, ac := range a {
		if ac != b[i] {
			return false
		}
	}

	return true
}
