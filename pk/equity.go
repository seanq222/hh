package pk

const (
	//	_52Choose5 = 2598960
	//	_50Choose7 = 12103014
	_48Choose5 = 1712304
)

func Equity(a0, a1, b0, b1 Card) (wins, draws, total int) {
	var a, b Hand
	var cards [52 - 4]Card

	a.AddCard(0, a0)
	a.AddCard(1, a1)
	b.AddCard(0, b0)
	b.AddCard(1, b1)

	cs := NewCardSet(a0, a1, b0, b1)
	idx := 0
	for c := Card(0); c < 52; c++ {
		if cs.Contains(c) {
			continue
		}
		cards[idx] = c
		idx++
	}

	for i0 := 0; i0 < 44; i0++ {
		a.AddCard(2, cards[i0])
		b.AddCard(2, cards[i0])
		for i1 := i0 + 1; i1 < 45; i1++ {
			a.AddCard(3, cards[i1])
			b.AddCard(3, cards[i1])
			for i2 := i1 + 1; i2 < 46; i2++ {
				a.AddCard(4, cards[i2])
				b.AddCard(4, cards[i2])
				for i3 := i2 + 1; i3 < 47; i3++ {
					a.AddCard(5, cards[i3])
					b.AddCard(5, cards[i3])
					for i4 := i3 + 1; i4 < 48; i4++ {
						ra := a.Evaluate(6, cards[i4])
						rb := b.Evaluate(6, cards[i4])
						if ra < rb {
							wins++
						} else if ra == rb {
							draws++
						}
						total++
					}
				}
			}
		}
	}
	return wins, draws, total
}

func EquitySample(n int, a0, a1, b0, b1 Card) (wins, draws, total int) {
	var d Deck
	var a, b Hand
	var c [5]Card

	a.AddCard(0, a0, a1)
	b.AddCard(0, b0, b1)

	d.Remove(a0, a1, b0, b1)
	for i := 0; i < n; i++ {
		d.Shuffle()
		d.Draw(c[:])
		ra := a.Evaluate(2, c[:]...)
		rb := b.Evaluate(2, c[:]...)
		if ra < rb {
			wins++
		} else if ra == rb {
			draws++
		}
		total++
	}
	return wins, draws, total
}

func EquityPostFlop(f Flop, oop, ip *Range) (float64, float64) {
	var oop_e, ip_e float64

	cs := NewCardSet(f[0], f[1], f[2])
	for c3 := Card(0); c3 < 52; c3++ {
		if (cs & (1 << c3)) != 0 {
			continue
		}
		for c4 := c3 + 1; c4 < 52; c4++ {
			if (cs & (1 << c4)) != 0 {
				continue
			}
			e0, e1 := equityPostRiver(f, c3, c4, oop, ip)
			oop_e += e0
			ip_e += e1
		}
	}

	return oop_e / (oop_e + ip_e), ip_e / (oop_e + ip_e)
}

func equityPostRiver(f Flop, c3, c4 Card, oop, ip *Range) (float64, float64) {
	var oop_e, ip_e float64
	var h Hand

	cs := NewCardSet(f[0], f[1], f[2], c3, c4)
	h.AddCard(0, f[0], f[1], f[2], c3, c4)

	oop_c, oop_w := oop.CardsAndWeights()
	ip_c, ip_w := ip.CardsAndWeights()

	ranks := map[[2]Card]HandRank{}
	for _, c := range oop_c {
		if cs&(1<<c[0]) != 0 || cs&(1<<c[1]) != 0 {
			continue
		}
		ranks[c] = h.Evaluate(5, c[0], c[1])
	}
	for _, c := range ip_c {
		if ranks[c] != 0 {
			continue
		}
		if cs&(1<<c[0]) != 0 || cs&(1<<c[1]) != 0 {
			continue
		}
		ranks[c] = h.Evaluate(5, c[0], c[1])
	}

	for i0, c0 := range oop_c {
		if cs&(1<<c0[0]) != 0 || cs&(1<<c0[1]) != 0 {
			continue
		}
		for i1, c1 := range ip_c {
			if cs&(1<<c1[0]) != 0 || cs&(1<<c1[1]) != 0 {
				continue
			}
			if c0[0] == c1[0] || c0[0] == c1[1] || c0[1] == c1[0] || c0[1] == c1[1] {
				continue
			}
			if ranks[c0] < ranks[c1] {
				oop_e += oop_w[i0] * ip_w[i1]
			} else if ranks[c0] > ranks[c1] {
				ip_e += oop_w[i0] * ip_w[i1]
			} else {
				oop_e += oop_w[i0] * ip_w[i1] / 2
				ip_e += oop_w[i0] * ip_w[i1] / 2
			}
		}
	}

	return oop_e, ip_e
}

func EquityEntireRange(a0, a1 Card) (wins, draws, total int) {
	var a, b Hand
	var cards [52 - 2]Card

	a.AddCard(0, a0, a1)

	cs := NewCardSet(a0, a1)
	idx := 0
	for c := Card(0); c < 52; c++ {
		if cs.Contains(c) {
			continue
		}
		cards[idx] = c
		idx++
	}

	for i0, c0 := range cards {
		a.AddCard(2, c0)
		b.AddCard(0, c0)
		for i1, c1 := range cards[i0+1:] {
			a.AddCard(3, c1)
			b.AddCard(1, c1)
			for i2, c2 := range cards[i0+i1+2:] {
				a.AddCard(4, c2)
				b.AddCard(2, c2)
				for i3, c3 := range cards[i0+i1+i2+3:] {
					a.AddCard(5, c3)
					b.AddCard(3, c3)
					for _, c4 := range cards[i0+i1+i2+i3+4:] {
						ra := a.Evaluate(6, c4)
						b.AddCard(4, c4)
						cs := NewCardSet(c0, c1, c2, c3, c4)
						for i5, c5 := range cards {
							if cs.Contains(c5) {
								continue
							}
							b.AddCard(5, c5)
							for _, c6 := range cards[i5+1:] {
								if cs.Contains(c6) {
									continue
								}
								rb := b.Evaluate(6, c6)
								if ra < rb {
									wins++
								} else if ra == rb {
									draws++
								}
								total++
							}
						}
					}
				}
			}
		}
	}

	return wins, draws, total
}

func EquityEntireRange2(a0, a1 Card) (wins, draws, total int) {
	cs := NewCardSet(a0, a1)
	for b0 := Card(0); b0 < 52; b0++ {
		if cs.Contains(b0) {
			continue
		}
		for b1 := b0 + 1; b1 < 52; b1++ {
			if cs.Contains(b1) {
				continue
			}
			w, d, t := Equity(a0, a1, b0, b1)
			wins += w
			draws += d
			total += t
		}
	}
	return wins, draws, total
}

func EquityRangeSample(n int, a0, a1 Card) (wins, draws, total int) {
	var d Deck
	var a Hand
	var c [7]Card

	a.AddCard(0, a0, a1)

	d.Remove(a0, a1)
	for i := 0; i < n; i++ {
		d.Shuffle()
		d.Draw(c[:])
		ra := a.Evaluate(2, c[2:]...)
		rb := Evaluate(c[:]...)
		if ra < rb {
			wins++
		} else if ra == rb {
			draws++
		}
		total++
	}
	return wins, draws, total
}
