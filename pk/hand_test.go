package pk

import "testing"

func TestAllSevenCards(t *testing.T) {
	var h = &Hand{}

	for c0 := Card(0); c0 < 46; c0++ {
		h.AddCard(0, c0)
		for c1 := c0 + 1; c1 < 47; c1++ {
			h.AddCard(1, c1)
			for c2 := c1 + 1; c2 < 48; c2++ {
				h.AddCard(2, c2)
				for c3 := c2 + 1; c3 < 49; c3++ {
					h.AddCard(3, c3)
					for c4 := c3 + 1; c4 < 50; c4++ {
						if h.Evaluate(4, Card(c4)) != Evaluate(c0, c1, c2, c3, c4) {
							cards := []Card{c0, c1, c2, c3, c4}
							t.Errorf("Evaluate(%v) = %d, want != %d", cards, h.Evaluate(4, Card(c4)), Evaluate(c0, c1, c2, c3, c4))
							return
						}
						h.AddCard(4, c4)
						for c5 := c4 + 1; c5 < 51; c5++ {
							h.AddCard(5, c5)
							for c6 := c5 + 1; c6 < 52; c6++ {
								if h.Evaluate(6, c6) != Evaluate(c0, c1, c2, c3, c4, c5, c6) {
									cards := []Card{c0, c1, c2, c3, c4, c5, c6}
									t.Errorf("Evaluate(%v) = %d, want != %d", cards, h.Evaluate(6, c6), Evaluate(c0, c1, c2, c3, c4, c5, c6))
									return
								}
							}
						}
					}
				}
			}
		}
	}
}

func BenchmarkHandEvaluateAll5(b *testing.B) {
	h := &Hand{}
	for b.Loop() {
		for c0 := Card(0); c0 < 46; c0++ {
			h.AddCard(0, c0)
			for c1 := c0 + 1; c1 < 47; c1++ {
				h.AddCard(1, c1)
				for c2 := c1 + 1; c2 < 48; c2++ {
					h.AddCard(2, c2)
					for c3 := c2 + 1; c3 < 49; c3++ {
						h.AddCard(3, c3)
						for c4 := c3 + 1; c4 < 50; c4++ {
							h.Evaluate(4, c4)
						}
					}
				}
			}
		}
	}
}

func BenchmarkHandEvaluateAll7(b *testing.B) {
	h := &Hand{}
	for b.Loop() {
		for c0 := Card(0); c0 < 46; c0++ {
			h.AddCard(0, c0)
			for c1 := c0 + 1; c1 < 47; c1++ {
				h.AddCard(1, c1)
				for c2 := c1 + 1; c2 < 48; c2++ {
					h.AddCard(2, c2)
					for c3 := c2 + 1; c3 < 49; c3++ {
						h.AddCard(3, c3)
						for c4 := c3 + 1; c4 < 50; c4++ {
							h.AddCard(4, c4)
							for c5 := c4 + 1; c5 < 51; c5++ {
								h.AddCard(5, c5)
								for c6 := c5 + 1; c6 < 52; c6++ {
									h.Evaluate(6, c6)
								}
							}
						}
					}
				}
			}
		}
	}
}
