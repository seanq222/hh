package pk

import (
	"fmt"
	"math/rand"
	"os"
	"testing"
)

func TestCardString(t *testing.T) {
	data := map[Card]string{
		CardFromRS(Ace, Spade):     "As",
		CardFromRS(King, Spade):    "Ks",
		CardFromRS(Queen, Heart):   "Qh",
		CardFromRS(Jack, Diamond):  "Jd",
		CardFromRS(Ten, Club):      "Tc",
		CardFromRS(Nine, Spade):    "9s",
		CardFromRS(Eight, Heart):   "8h",
		CardFromRS(Seven, Diamond): "7d",
		CardFromRS(Six, Club):      "6c",
		CardFromRS(Five, Spade):    "5s",
		CardFromRS(Four, Heart):    "4h",
		CardFromRS(Three, Diamond): "3d",
		CardFromRS(Two, Club):      "2c",
	}
	for c, s := range data {
		if c.String() != s {
			t.Errorf("Card.String() = %s, want %s", c.String(), s)
		}
	}
}

func TestParseCard(t *testing.T) {
	data := map[string]Card{
		"As": CardFromRS(Ace, Spade),
		"Ks": CardFromRS(King, Spade),
		"Qh": CardFromRS(Queen, Heart),
		"Jd": CardFromRS(Jack, Diamond),
		"Tc": CardFromRS(Ten, Club),
		"9s": CardFromRS(Nine, Spade),
		"8h": CardFromRS(Eight, Heart),
		"7d": CardFromRS(Seven, Diamond),
		"6c": CardFromRS(Six, Club),
		"5s": CardFromRS(Five, Spade),
		"4h": CardFromRS(Four, Heart),
		"3d": CardFromRS(Three, Diamond),
		"2c": CardFromRS(Two, Club),
	}
	for s, c := range data {
		if card, err := ParseCard(s); err != nil || card != c {
			t.Errorf("ParseCard(%s) = %d, %v, want %d, nil", s, card, err, c)
		}
	}
}

func TestParseCardError(t *testing.T) {
	data := []string{
		"",
		" ",
		"1",
		"1s",
		"1h",
		"1d",
		"1c",
		"Ax",
		"Ass",
		"1hh",
		"Ds",
		"as",
		"AS",
	}
	for _, s := range data {
		if card, err := ParseCard(s); err == nil {
			t.Errorf("ParseCard(\"%s\") = %d, nil, want _, error", s, card)
		}
	}
}

func TestHoldCardArray(t *testing.T) {
	m := make(map[HoleCards]bool)
	for _, hc := range HoleCardsArray {
		if _, ok := m[hc]; ok {
			t.Errorf("HoleCardsArray contains duplicate %s", hc)
		}
		m[hc] = true
	}
	if len(m) != NumHoleCardPairs {
		t.Errorf("HoleCardsArray contains %d duplicates", NumHoleCardPairs-len(m))
	}
}

func TestParseBoard(t *testing.T) {
	data := map[string][]Card{
		"As":       {CardFromRS(Ace, Spade)},
		"AsKdJc":   {CardFromRS(Ace, Spade), CardFromRS(King, Diamond), CardFromRS(Jack, Club)},
		"AsKdJc2h": {CardFromRS(Ace, Spade), CardFromRS(King, Diamond), CardFromRS(Jack, Club), CardFromRS(Two, Heart)},
	}
	for s, c := range data {
		cards, err := ParseBoard(s)

		if err != nil || !EqualHands(c, cards) {
			t.Errorf("ParseBoard(%s) = %s, %v, want %s, nil", s, cards, err, c)
		}
	}
}

func TestHoleCardsIndex(t *testing.T) {
	idx := 0
	for c0 := Card(0); c0 < 51; c0++ {
		for c1 := c0 + 1; c1 < 52; c1++ {
			hc := HoleCardsFromCards(c0, c1)
			if hc.Index() != idx {
				t.Errorf("HoleCards.Index() = %d, want %d", hc.Index(), idx)
			}
			idx++
		}
	}
}

func BenchmarkHoleCardsIndex(b *testing.B) {
	var hc [NumHoleCardPairs]HoleCards

	for i := range hc {
		fmt.Fprintf(os.Stderr, "%d %s\n", i, HoleCardsArray[i])
		hc[i] = HoleCardsArray[i]
	}

	for i := range hc {
		j := rand.Intn(NumHoleCardPairs)
		hc[j], hc[i] = hc[i], hc[j]
	}

	idx := 0
	for b.Loop() {
		hc[idx].Index()
		idx++
		if idx == NumHoleCardPairs {
			idx = 0
		}
	}
}
