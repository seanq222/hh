package hh

import (
	"slices"

	"github.com/seanq222/hh/pk"
)

// A hand with players all-in for different amounts has a main pot and side pots.
// The client prints one line for each, and at the showdown says who collected
// each. The pots follow from the actions, and the winners from the cards.

// sidePot is one layer of the pot.
type sidePot struct {
	gross    Cents // what went in, rake included
	rake     Cents
	eligible []int // the seats that can win it: not folded, and in for the layer
}

// net is what the pot pays out.
func (p sidePot) net() Cents { return p.gross - p.rake }

// potsBefore returns the pots as street s begins, or at the showdown when s is
// past the river. Each distinct amount that a player who has not folded has put
// in ends a layer. The rake is shared out among the pots in proportion to their
// size, and the odd cents go to the first pots. It returns nil if there is a
// single pot, or if the pots do not add up to total.
func (h *HandHistory) potsBefore(s Street, rake, total Cents) []sidePot {
	contrib := map[int]Cents{}
	folded := map[int]bool{}
	later := false
	for i := range h.Actions {
		a := &h.Actions[i]
		if a.Street >= s {
			later = true
			continue
		}
		contrib[a.Seat] += a.Amount
		if a.Kind == Fold {
			folded[a.Seat] = true
		}
	}
	if !later {
		if seat, amount, ok := h.UncalledBet(); ok {
			contrib[seat] -= amount
		}
	}
	var levels []Cents
	for seat, c := range contrib {
		if !folded[seat] && c > 0 && !slices.Contains(levels, c) {
			levels = append(levels, c)
		}
	}
	slices.Sort(levels)
	if len(levels) < 2 {
		return nil
	}
	var pots []sidePot
	var prev, sum Cents
	for _, level := range levels {
		p := sidePot{}
		for seat, c := range contrib {
			p.gross += min(c, level) - min(c, prev)
			if !folded[seat] && c >= level {
				p.eligible = append(p.eligible, seat)
			}
		}
		slices.Sort(p.eligible)
		pots = append(pots, p)
		sum += p.gross
		prev = level
	}
	if sum != total {
		return nil
	}
	var used Cents
	for i := range pots {
		pots[i].rake = rake * pots[i].gross / total
		used += pots[i].rake
	}
	for i := 0; used < rake; i = (i + 1) % len(pots) {
		pots[i].rake++
		used++
	}
	return pots
}

// potName is how the client names pot i: the main pot, then side pots 1, 2.
func potName(i int) string {
	if i == 0 {
		return "Main pot"
	}
	return "Side pot(" + string(rune('0'+i)) + ")"
}

// winners returns the seats that win a pot at the showdown: the best hands among
// the players that can win it and whose cards are known.
func (h *HandHistory) winners(p sidePot) []int {
	if len(h.Board) < 5 {
		return nil
	}
	best := pk.HandRank(1 << 14)
	var win []int
	for _, seat := range p.eligible {
		pl := h.PlayerBySeat(seat)
		if pl == nil || !pl.HasCards {
			continue
		}
		cards := append([]pk.Card{pl.Cards[0], pl.Cards[1]}, h.Board[:5]...)
		switch r := pk.Evaluate(cards...); {
		case r < best:
			best, win = r, []int{seat}
		case r == best:
			win = append(win, seat)
		}
	}
	return win
}

// writeCollected writes who collected each pot at the showdown: the winners of
// each pot share its payout equally, and the odd cents go to the first.
func (h *HandHistory) writeCollected(w func(string, ...any), pots []sidePot) {
	for i, p := range pots {
		win := h.winners(p)
		if len(win) == 0 {
			continue
		}
		name := "main pot"
		if i > 0 {
			name = "side pot-" + string(rune('0'+i))
		}
		share, odd := p.net()/Cents(len(win)), int(p.net()%Cents(len(win)))
		for k, seat := range win {
			amount := share
			if k < odd {
				amount++
			}
			w("%s collected %s from %s", h.PlayerBySeat(seat).Name, amount, name)
		}
	}
}
