package pk

type Hand struct {
	product [7]handProduct
	sum     [7]cardSum
}

func (h *Hand) AddCard(idx int, c ...Card) {
	for _, c := range c {
		if idx == 0 {
			h.product[0] = cardPrime[c]
			h.sum[0] = cardBits[c]
		} else {
			h.product[idx] = h.product[idx-1] * cardPrime[c]
			h.sum[idx] = h.sum[idx-1] + cardBits[c]
		}
		idx++
	}
}

func (h *Hand) Evaluate(idx int, c ...Card) HandRank {
	if idx == 0 {
		return Evaluate(c...)
	}

	sum := h.sum[idx-1]
	p := h.product[idx-1]
	for _, c := range c {
		sum += cardBits[c]
		p *= cardPrime[c]
	}
	sb := flushLookup[sum&0xfff]
	if sb != 0 {
		rs := (sum >> sb) & 0x1fff
		return flushRank[rs]
	}
	// Unsuited
	return unsuitedRank[p]
}
