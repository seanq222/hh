package pk

import (
	"fmt"
	"testing"
)

func TestEvaluate5(t *testing.T) {
	date := map[HandRank]string{
		7462: "7s5d4c3s2d", // worst high card
		6252: "AsKsJc7h5d", // high card
		3448: "AsAcJc7h5d", // pair
		2497: "AsAcJcJd5d", // two pair
		1636: "AsAcAdJd5d", // three of a kind
		1600: "AsKsQdJhTd", // straight
		1609: "5s4s3d2hAd", // Ace low straight
		1542: "Ts7s4s3s2s", // flush
		298:  "4s4c4d2s2h", // full house
		19:   "AsAcAdAh5h", // four of a kind
		1:    "AsKsQsJsTs", // straight flush
	}
	for rank, s := range date {
		cards, err := ParseBoard(s)
		if err != nil {
			t.Errorf("ParseBoard(%s) = %v", s, err)
			continue
		}

		if Evaluate5(cards) != rank {
			t.Errorf("Evaluate5(%s) = %d, want %d", s, Evaluate5(cards), rank)
		}
	}
}

func TestUniqueProduct(t *testing.T) {
	var unique = map[handProduct]bool{}
	ndups := 0

	for r0 := 0; r0 < 13; r0++ {
		for r1 := r0; r1 < 13; r1++ {
			for r2 := r1; r2 < 13; r2++ {
				for r3 := r2; r3 < 13; r3++ {
					for r4 := r3; r4 < 13; r4++ {
						p4 := primes[r0] * primes[r1] * primes[r2] * primes[r3] * primes[r4]
						if unique[p4] {
							ndups++
						}
						unique[p4] = true
						for r5 := r4; r5 < 13; r5++ {
							p5 := p4 * primes[r5]
							if unique[p5] {
								ndups++
							}
							unique[p5] = true
							for r6 := r5; r6 < 13; r6++ {
								p6 := p5 * primes[r6]
								if unique[p6] {
									ndups++
								}
								unique[p6] = true
							}
						}
					}
				}
			}
		}
	}

	if ndups != 0 {
		t.Errorf("ndups = %d, want 0", ndups)
	}

}

func TestEvaluate(t *testing.T) {
	var d Deck
	// Compare Evaluate5 to Evaluate
	for i := 0; i < 10000000; i++ {
		var cards [5]Card
		d.Shuffle()
		d.Draw(cards[:])
		if Evaluate(cards[:]...) != Evaluate5(cards[:]) {
			t.Errorf("Evaluate5(%v) = %d, want %d", cards[:], Evaluate(cards[:]...), Evaluate5(cards[:]))
		}
	}

	// Compare Evaluate7 to Evaluate
	for i := 0; i < 10000000; i++ {
		var cards [7]Card
		d.Shuffle()
		d.Draw(cards[:])
		if Evaluate(cards[:]...) != Evaluate7(cards[:]...) {
			t.Errorf("Evaluate5(%v) = %d, want %d", cards[:], Evaluate(cards[:]...), Evaluate7(cards[:]...))
		}
	}
}

func Evaluate6Brute(cards []Card) HandRank {
	best := maxRank + 1
	var hand [5]Card
	for i := 0; i < 6; i++ {
		c := hand[:0]
		for j := 0; j < 6; j++ {
			if i != j {
				c = append(c, cards[j])
			}
		}
		rank := Evaluate5(c)
		if rank == 0 {
			fmt.Printf("%v\n", c)
		}
		if rank < best {
			best = rank
		}

	}
	return best
}

func TestEvaluate6(t *testing.T) {
	var d Deck
	// Compare Evaluate5 to Evaluate
	for i := 0; i < 10000; i++ {
		var cards [6]Card
		d.Shuffle()
		d.Draw(cards[:])
		if Evaluate(cards[:]...) != Evaluate6Brute(cards[:]) {
			t.Errorf("Evaluate(%v) = %d, want %d", cards[:], Evaluate(cards[:]...), Evaluate6Brute(cards[:]))
		}
	}
}

func Evaluate7Brute(cards []Card) HandRank {
	best := maxRank + 1
	var hand [5]Card
	for i := 0; i < 7; i++ {
		for j := i + 1; j < 7; j++ {
			c := hand[:0]
			for k := 0; k < 7; k++ {
				if i != k && j != k {
					c = append(c, cards[k])
				}
			}
			rank := Evaluate5(c)
			if rank == 0 {
				fmt.Printf("%v\n", c)
			}
			if rank < best {
				best = rank
			}
		}
	}
	return best
}

func TestEvaluate7(t *testing.T) {
	var d Deck
	// Compare Evaluate5 to Evaluate
	for i := 0; i < 10000; i++ {
		var cards [7]Card
		d.Shuffle()
		d.Draw(cards[:])
		if Evaluate(cards[:]...) != Evaluate7Brute(cards[:]) {
			t.Errorf("Evaluate5(%v) = %d, want %d", cards[:], Evaluate(cards[:]...), Evaluate7Brute(cards[:]))
		}
	}
}

func DrawHands(nc int, n int) [][]Card {
	var d Deck
	var hands [][]Card

	for i := 0; i < n; i++ {
		d.Shuffle()
		cards := make([]Card, nc)
		d.Draw(cards[:])
		hands = append(hands, cards)
	}
	return hands
}

func BenchmarkEvaluate5(b *testing.B) {
	hands := DrawHands(5, 10000)
	idx := 0
	for b.Loop() {
		Evaluate5(hands[idx])
		idx++
		if idx == len(hands) {
			idx = 0
		}
	}
}

func BenchmarkEvaluate5g(b *testing.B) {
	hands := DrawHands(5, 10000)
	idx := 0
	for b.Loop() {
		Evaluate(hands[idx]...)
		idx++
		if idx == len(hands) {
			idx = 0
		}
	}
}

func BenchmarkEvaluate6g(b *testing.B) {
	hands := DrawHands(6, 10000)
	idx := 0
	for b.Loop() {
		Evaluate(hands[idx]...)
		idx++
		if idx == len(hands) {
			idx = 0
		}
	}
}

func BenchmarkEvaluate7g(b *testing.B) {
	hands := DrawHands(7, 10000)
	idx := 0
	for b.Loop() {
		Evaluate(hands[idx]...)
		idx++
		if idx == len(hands) {
			idx = 0
		}
	}
}

func BenchmarkEvaluate7(b *testing.B) {
	hands := DrawHands(7, 1000000)
	idx := 0
	for b.Loop() {
		Evaluate7(hands[idx]...)
		idx++
		if idx == len(hands) {
			idx = 0
		}
	}
}

func BenchmarkEvaluateAllSevenCards(b *testing.B) {
	for b.Loop() {
		for c0 := Card(0); c0 < 46; c0++ {
			for c1 := c0 + 1; c1 < 47; c1++ {
				for c2 := c1 + 1; c2 < 48; c2++ {
					for c3 := c2 + 1; c3 < 49; c3++ {
						for c4 := c3 + 1; c4 < 50; c4++ {
							for c5 := c4 + 1; c5 < 51; c5++ {
								for c6 := c5 + 1; c6 < 52; c6++ {
									Evaluate7(c0, c1, c2, c3, c4, c5, c6)
								}
							}
						}
					}
				}
			}
		}
	}
}

func TestHandRankClass(t *testing.T) {
	for want, s := range map[HandClass]string{
		HighCard:      "AsKsJc7h5d",
		OnePair:       "AsAcJc7h5d",
		TwoPair:       "AsAcJcJd5d",
		ThreeOfAKind:  "AsAcAdJd5d",
		Straight:      "5s4s3d2hAd",
		Flush:         "Ts7s4s3s2s",
		FullHouse:     "4s4c4d2s2h",
		FourOfAKind:   "AsAcAdAh5h",
		StraightFlush: "AsKsQsJsTs",
	} {
		if got := Evaluate5(mustBoard(t, s)).Class(); got != want {
			t.Errorf("%s: %v, want %v", s, got, want)
		}
	}
}
