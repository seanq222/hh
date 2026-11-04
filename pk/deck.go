package pk

import (
	"math/rand"
)

// Deck represents a standard 52-card deck. cards holds a permutation of all
// 52 cards and index is its inverse (index[c] is c's position in cards).
// available and undrawn are offsets into cards: cards[:available] have been
// Remove()'d, cards[available:undrawn] have been Draw()'n in the current
// shuffle but not removed, and cards[undrawn:] are available to draw.
type Deck struct {
	cards       [52]Card
	index       [52]uint8
	initialized bool
	available   int
	undrawn     int
}

func (d *Deck) Reset() {
	for i := range d.cards {
		d.cards[i] = Card(i)
	}
	for i := range d.index {
		d.index[i] = uint8(i)
	}
	d.available = 0
	d.undrawn = 0
	d.initialized = true
}

func (d *Deck) check() bool {
	for i, c := range d.cards {
		if d.index[c] != uint8(i) {
			return false
		}
	}
	return true
}

func (d *Deck) Shuffle() {
	if !d.initialized {
		d.Reset()
	}
	d.undrawn = d.available
}

func (d *Deck) Remove(c ...Card) {
	if !d.initialized {
		d.Reset()
	}
	for _, c0 := range c {
		i := int(d.index[c0])
		if i < d.available {
			// Already removed
			return
		}

		if i >= d.undrawn {
			// c0 is inside the undrawn region: evict it by swapping with
			// the undrawn region's own front card first, so the wrong
			// card isn't dropped below.
			c1 := d.cards[d.undrawn]
			d.cards[d.undrawn], d.cards[i] = c0, c1
			d.index[c0], d.index[c1] = d.index[c1], d.index[c0]
			i = d.undrawn
			d.undrawn++
		}

		c1 := d.cards[d.available]
		d.cards[d.available], d.cards[i] = c0, c1
		d.index[c0], d.index[c1] = d.index[c1], d.index[c0]
		d.available++
	}
}

func (d *Deck) Draw(c []Card) {
	n := len(c)
	nn := 52 - d.undrawn
	for i := 0; i < n; i++ {
		j := d.undrawn + rand.Intn(nn)
		k := d.undrawn + i
		c0, c1 := d.cards[k], d.cards[j]
		d.cards[j], d.cards[k] = c0, c1
		d.index[c0], d.index[c1] = d.index[c1], d.index[c0]
	}
	copy(c, d.cards[d.undrawn:d.undrawn+n])
	d.undrawn += n
}
