package pk

import "fmt"

// Rank for the first of each class of hand.
type HandRank int16

const (
	straightFlush HandRank = 1
	fourOfAKind   HandRank = 11
	fullHouse     HandRank = 167
	flush         HandRank = 323
	straight      HandRank = 1600
	threeOfAKind  HandRank = 1610
	twoPair       HandRank = 2468
	pair          HandRank = 3326
	highCard      HandRank = 6186
	maxRank       HandRank = 7462
)

// HandClass is the kind of a poker hand, from high card up to straight flush,
// in order of strength.
type HandClass uint8

const (
	HighCard HandClass = iota
	OnePair
	TwoPair
	ThreeOfAKind
	Straight
	Flush
	FullHouse
	FourOfAKind
	StraightFlush
	NumHandClasses
)

var handClassNames = [NumHandClasses]string{"high card", "one pair", "two pair", "three of a kind", "straight", "flush", "full house", "four of a kind", "straight flush"}

func (c HandClass) String() string {
	if c < NumHandClasses {
		return handClassNames[c]
	}
	return fmt.Sprintf("HandClass(%d)", uint8(c))
}

// Class returns the class of a hand rank.
func (r HandRank) Class() HandClass {
	switch {
	case r < fourOfAKind:
		return StraightFlush
	case r < fullHouse:
		return FourOfAKind
	case r < flush:
		return FullHouse
	case r < straight:
		return Flush
	case r < threeOfAKind:
		return Straight
	case r < twoPair:
		return ThreeOfAKind
	case r < pair:
		return TwoPair
	case r < highCard:
		return OnePair
	}
	return HighCard
}

// handProduct is the product of the prime numbers for each card in a hand.
// We have a seperate type for enable us to select what is fastest.
// Note, the type does not *need* to be big enough to hold 41^7, it just
// to be big enough to ensure all products are unqiue! This is checked in
// by a test. 32 bits does work.
type handProduct int32

// A cardSum uses the low 12 bits to represent the number of cards in each
// suit and the upper 52 bits to repesent a bit set of the cards in the hand.
// cardSum can be added together and are used to compute if a set of upto
// 7 cards is a flush and the cards in the flush.
type cardSum uint64

var (
	// 13 primes for each of the 13 card ranks. The prime for the ace is duplicated at the end of the array so that we can
	// treat ace-low straights as a special case.
	primes    = []handProduct{2, 3, 5, 7, 11, 13, 17, 19, 23, 29, 31, 37, 41, 2}
	cardPrime [52]handProduct
	cardBits  [52]cardSum

	flushLookup = [1 << 12]uint8{}

	unsuitedRank = map[handProduct]HandRank{}
	flushRank    = [1 << 13]HandRank{}
)

func init() {

	for c := Card(0); c < 52; c++ {
		r := c.Rank()
		s := c.Suit()
		cardPrime[c] = primes[r]
		cardBits[c] = 1<<(s*3) | 0x1000<<(s*13+r)
	}

	for s := 0; s < 4; s++ {
		sb := 1 << (s * 3)
		shift := uint8(s*13 + 12)
		s5 := sb * 5
		flushLookup[s5] = shift
		for j := 0; j < 4; j++ {
			s6 := s5 + 1<<(j*3)
			flushLookup[s6] = shift
			for k := 0; k < 4; k++ {
				s7 := s6 + 1<<(k*3)
				flushLookup[s7] = shift
			}
		}
	}

	initRank5()
	expandFlushRank()
	expandUnsuitedRank()
}

func initRank5() {

	straights := map[handProduct]bool{}

	// Straight Flushes
	var rank HandRank = straightFlush
	for r := 0; r < 10; r++ {
		p := primes[r] * primes[r+1] * primes[r+2] * primes[r+3] * primes[r+4]
		straights[p] = true
		rs := 0x1f << r
		if r == 9 {
			// Ace-low straight flush
			rs = 0xf<<9 | 0x1
		}
		flushRank[rs] = rank
		rank++
	}

	// Four of a Kind
	if rank != fourOfAKind {
		panic("rank != fourOfAKind")
	}
	for r0 := 0; r0 < 13; r0++ {
		// For each kicker
		for r1 := 0; r1 < 13; r1++ {
			if r0 == r1 {
				continue
			}
			p := primes[r0] * primes[r0] * primes[r0] * primes[r0] * primes[r1]
			unsuitedRank[p] = rank
			rank++
		}
	}

	// Full House
	if rank != fullHouse {
		panic("rank != fullHouse")
	}
	for r0 := 0; r0 < 13; r0++ {
		for r1 := 0; r1 < 13; r1++ {
			if r0 == r1 {
				continue
			}
			p := primes[r0] * primes[r0] * primes[r0] * primes[r1] * primes[r1]
			unsuitedRank[p] = rank
			rank++
		}
	}

	// Flush
	if rank != flush {
		panic("rank != flush")
	}
	for r0 := 0; r0 < 13; r0++ {
		for r1 := r0 + 1; r1 < 13; r1++ {
			for r2 := r1 + 1; r2 < 13; r2++ {
				for r3 := r2 + 1; r3 < 13; r3++ {
					for r4 := r3 + 1; r4 < 13; r4++ {
						p := primes[r0] * primes[r1] * primes[r2] * primes[r3] * primes[r4]
						// skip straight flushes
						if straights[p] {
							continue
						}
						rs := 1<<r0 | 1<<r1 | 1<<r2 | 1<<r3 | 1<<r4
						flushRank[rs] = rank
						rank++
					}
				}
			}
		}
	}

	// Straight
	if rank != straight {
		panic("rank != straight")
	}
	for r0 := 0; r0 < 10; r0++ {
		product := primes[r0] * primes[r0+1] * primes[r0+2] * primes[r0+3] * primes[r0+4]
		unsuitedRank[product] = rank
		rank++
	}

	// Three of a Kind
	if rank != threeOfAKind {
		panic("rank != threeOfAKind")
	}
	for r0 := 0; r0 < 13; r0++ {
		for r1 := 0; r1 < 13; r1++ {
			if r0 == r1 {
				continue
			}
			for r2 := r1 + 1; r2 < 13; r2++ {
				if r0 == r2 {
					continue
				}
				product := primes[r0] * primes[r0] * primes[r0] * primes[r1] * primes[r2]
				unsuitedRank[product] = rank
				rank++
			}
		}
	}

	// Two Pair
	if rank != twoPair {
		panic("rank != twoPair")
	}

	for r0 := 0; r0 < 13; r0++ {
		for r1 := r0 + 1; r1 < 13; r1++ {
			for r2 := 0; r2 < 13; r2++ {
				if r0 == r2 || r1 == r2 {
					continue
				}
				product := primes[r0] * primes[r0] * primes[r1] * primes[r1] * primes[r2]
				unsuitedRank[product] = rank
				rank++
			}
		}
	}

	// Pair
	if rank != pair {
		panic("rank != pair")
	}
	for r0 := 0; r0 < 13; r0++ {
		for r1 := 0; r1 < 13; r1++ {
			if r0 == r1 {
				continue
			}
			for r2 := r1 + 1; r2 < 13; r2++ {
				if r0 == r2 {
					continue
				}
				for r3 := r2 + 1; r3 < 13; r3++ {
					if r0 == r3 {
						continue
					}
					product := primes[r0] * primes[r0] * primes[r1] * primes[r2] * primes[r3]
					unsuitedRank[product] = rank
					rank++
				}
			}
		}
	}

	// High Card
	if rank != highCard {
		panic("rank != highCard")
	}
	for r0 := 0; r0 < 13; r0++ {
		for r1 := r0 + 1; r1 < 13; r1++ {
			for r2 := r1 + 1; r2 < 13; r2++ {
				for r3 := r2 + 1; r3 < 13; r3++ {
					for r4 := r3 + 1; r4 < 13; r4++ {
						product := primes[r0] * primes[r1] * primes[r2] * primes[r3] * primes[r4]
						// skip straights
						if straights[product] {
							continue
						}
						unsuitedRank[product] = rank
						rank++
					}
				}
			}
		}
	}

	if rank != maxRank+1 {
		panic("rank != maxRank+1")
	}
}

func expandFlushRank() {
	for r0 := 0; r0 < 13; r0++ {
		for r1 := r0 + 1; r1 < 13; r1++ {
			for r2 := r1 + 1; r2 < 13; r2++ {
				for r3 := r2 + 1; r3 < 13; r3++ {
					for r4 := r3 + 1; r4 < 13; r4++ {
						for r5 := r4 + 1; r5 < 13; r5++ {
							setFlushRank(r0, r1, r2, r3, r4, r5)
						}
					}
				}
			}
		}
	}

	for r0 := 0; r0 < 13; r0++ {
		for r1 := r0 + 1; r1 < 13; r1++ {
			for r2 := r1 + 1; r2 < 13; r2++ {
				for r3 := r2 + 1; r3 < 13; r3++ {
					for r4 := r3 + 1; r4 < 13; r4++ {
						for r5 := r4 + 1; r5 < 13; r5++ {
							for r6 := r5 + 1; r6 < 13; r6++ {
								setFlushRank(r0, r1, r2, r3, r4, r5, r6)
							}
						}
					}
				}
			}
		}
	}
}

func expandUnsuitedRank() {
	for c0 := 0; c0 < 13; c0++ {
		for c1 := c0; c1 < 13; c1++ {
			for c2 := c1; c2 < 13; c2++ {
				for c3 := c2; c3 < 13; c3++ {
					for c4 := c3; c4 < 13; c4++ {
						for c5 := c4; c5 < 13; c5++ {
							if !handValid(c0, c1, c2, c3, c4, c5) {
								continue
							}
							setUnsuitedRank(c0, c1, c2, c3, c4, c5)
						}
					}
				}
			}
		}
	}

	for c0 := 0; c0 < 13; c0++ {
		for c1 := c0; c1 < 13; c1++ {
			for c2 := c1; c2 < 13; c2++ {
				for c3 := c2; c3 < 13; c3++ {
					for c4 := c3; c4 < 13; c4++ {
						for c5 := c4; c5 < 13; c5++ {
							for c6 := c5; c6 < 13; c6++ {
								if !handValid(c0, c1, c2, c3, c4, c5, c6) {
									continue
								}
								setUnsuitedRank(c0, c1, c2, c3, c4, c5, c6)
							}
						}
					}
				}
			}
		}
	}
}

func handValid(r ...int) bool {
	// check for no more than 4 of a kind
	counts := [13]int{}
	for _, rx := range r {
		counts[rx]++
		if counts[rx] > 4 {
			return false
		}
	}
	return true
}

func setUnsuitedRank(r ...int) {
	best := maxRank + 1
	for i := 0; i < len(r); i++ {
		p := handProduct(1)
		for j := 0; j < len(r); j++ {
			if i != j {
				p *= primes[r[j]]
			}
		}
		rank := unsuitedRank[p]
		if rank == 0 {
			panic("invalid hand")
		}
		if rank < best {
			best = rank
		}
	}

	p := handProduct(1)
	for _, rx := range r {
		p *= primes[rx]
	}

	if unsuitedRank[p] != 0 {
		panic("duplicate hand")
	}

	unsuitedRank[p] = best
}

func setFlushRank(r ...int) {
	best := maxRank + 1
	for i := 0; i < len(r); i++ {
		rs := 0
		for j := 0; j < len(r); j++ {
			if i != j {
				rs |= 1 << r[j]
			}
		}
		rank := flushRank[rs]
		if rank == 0 {
			panic("invalid hand")
		}
		if rank < best {
			best = rank
		}
	}

	rs := 0
	for _, rx := range r {
		rs |= 1 << rx
	}

	if flushRank[rs] != 0 {
		panic("duplicate hand")
	}

	flushRank[rs] = best
}

func Evaluate5(cards []Card) HandRank {
	p := cardPrime[cards[0]] *
		cardPrime[cards[1]] *
		cardPrime[cards[2]] *
		cardPrime[cards[3]] *
		cardPrime[cards[4]]

	cb0 := cardBits[cards[0]]
	cb1 := cardBits[cards[1]]
	cb2 := cardBits[cards[2]]
	cb3 := cardBits[cards[3]]
	cb4 := cardBits[cards[4]]

	if cb0&cb1&cb2&cb3&cb4 != 0 {
		sum := cb0 + cb1 + cb2 + cb3 + cb4
		rs := (sum>>12 | sum>>25 | sum>>38 | sum>>51) & 0x1fff

		return flushRank[rs]
	}

	return unsuitedRank[p]
}

func Evaluate7(cards ...Card) HandRank {
	p := cardPrime[cards[0]] *
		cardPrime[cards[1]] *
		cardPrime[cards[2]] *
		cardPrime[cards[3]] *
		cardPrime[cards[4]] *
		cardPrime[cards[5]] *
		cardPrime[cards[6]]

	sum := cardBits[cards[0]] +
		cardBits[cards[1]] +
		cardBits[cards[2]] +
		cardBits[cards[3]] +
		cardBits[cards[4]] +
		cardBits[cards[5]] +
		cardBits[cards[6]]

	sb := flushLookup[sum&0xfff]
	if sb != 0 {
		rs := (sum >> sb) & 0x1fff
		return flushRank[rs]
	}
	return unsuitedRank[p]
}

func Evaluate(cards ...Card) HandRank {
	sum := cardSum(0)
	p := handProduct(1)
	for _, card := range cards {
		sum += cardBits[card]
		p *= cardPrime[card]
	}
	sb := flushLookup[sum&0xfff]
	if sb != 0 {
		rs := (sum >> sb) & 0x1fff
		return flushRank[rs]
	}
	return unsuitedRank[p]
}
