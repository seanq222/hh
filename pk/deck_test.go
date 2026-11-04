package pk

import (
	"testing"
)

func TestDeck(t *testing.T) {
	var d Deck
	var c [10]Card

	for i := 0; i < 10; i++ {
		d.Shuffle()
		d.Draw(c[:])
		if !d.check() {
			t.Errorf("check failed after shuffle and draw")
		}
	}
}

func TestDeckRemove(t *testing.T) {
	var d Deck
	var c [10]Card

	cr := []Card{0, 10, 20, 51}

	d.Remove(cr...)
	for i := 0; i < 100; i++ {
		d.Shuffle()
		d.Draw(c[:])
		if !d.check() {
			t.Errorf("check failed after shuffle and draw")
		}
		for _, c0 := range cr {
			for _, c1 := range c {
				if c0 == c1 {
					t.Errorf("removed card %d was drawn", c0)
				}
			}
		}

		if !d.check() {
			t.Errorf("check failed after shuffle and draw")
		}
	}
}

// TestDeckRemoveFromUndrawn exercises Remove()'d being called on a card that
// is undrawn but not at the very front of the Undrawn region, after some
// cards have already been drawn in the current shuffle round. This is the
// case Remove()'s swap-to-front logic gets wrong: it only correctly evicts
// the target card from Undrawn when the target happens to be Undrawn[0].
func TestDeckRemoveFromUndrawn(t *testing.T) {
	var d Deck

	// Draw a few cards so that Undrawn is a strict, offset subset of
	// available (i.e. some cards are already "dealt" in this round).
	drawn := make([]Card, 5)
	d.Shuffle()
	d.Draw(drawn)

	if !d.check() {
		t.Fatalf("check failed after initial draw")
	}

	// Target a card from the interior of Undrawn, not the front.
	target := d.cards[d.undrawn+3]
	before := append([]Card{}, d.cards[d.undrawn:]...)

	d.Remove(target)

	if !d.check() {
		t.Fatalf("check failed after Remove")
	}

	if 52-d.undrawn != len(before)-1 {
		t.Errorf("len(Undrawn) = %d, want %d", 52-d.undrawn, len(before)-1)
	}

	for _, c := range d.cards[d.undrawn:] {
		if c == target {
			t.Errorf("removed card %v is still in Undrawn", target)
		}
	}

	// An already-drawn card must never reappear in Undrawn: dealing it
	// again would produce a duplicate card in the hand/board.
	for _, dc := range drawn {
		for _, c := range d.cards[d.undrawn:] {
			if c == dc {
				t.Errorf("already-drawn card %v reappeared in Undrawn after Remove", dc)
			}
		}
	}

	// Every card that was undrawn before Remove (other than the removed
	// one) must still be undrawn afterwards -- Remove must not silently
	// drop an unrelated card from the pool.
	after := map[Card]bool{}
	for _, c := range d.cards[d.undrawn:] {
		after[c] = true
	}
	for _, c := range before {
		if c == target {
			continue
		}
		if !after[c] {
			t.Errorf("card %v was silently dropped from Undrawn by Remove", c)
		}
	}
}

func BenchmarkDraw(b *testing.B) {
	var d Deck
	var c [7]Card

	for b.Loop() {
		d.Shuffle()
		d.Draw(c[:])
	}

}
