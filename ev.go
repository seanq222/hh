package hh

import (
	"slices"

	"github.com/seanq222/hh/pk"
)

// ExpectedNet returns the hero's result in cents with the luck of the runout
// taken out: a hand that went all-in with cards still to come is valued at the
// equity the hero had when the money went in, not at what the board gave them.
// Every other hand is worth what it was, Net. adjusted says which.
//
// The pot is the chips that went in, less the uncalled bet and the rake, and
// each player's share of it is their equity against the cards that were shown.
// Side pots are worked out layer by layer: a player can win from a layer only if
// they put at least that much in.
//
// A hand is valued this way only when
//   - the hero did not fold and at least one other player was still in,
//   - the money went in before the river: either the last betting happened
//     before it and at most one player still in had chips left, so the rest of
//     the board was dealt without any action, or the hero was all-in before it,
//   - the cards of every player still in are known.
//
// A hero who is all-in is valued on the board of the street where their last
// chips went in: what the others bet after that is a side pot the hero cannot
// win, and it does not change the hero's share. A hero who covered an all-in
// player and kept betting with another, for example, is left at what they won.
func (p *PlayerHand) ExpectedNet() (net float64, adjusted bool) { return p.expectedNet(false) }

// ExpectedNetPerPot is ExpectedNet worked out the way Hand2Note does it, for
// comparing with its figures: the equity in each pot is counted with only the
// cards of the players who can win that pot taken out of the deck. A side pot
// thus puts back the cards of the all-in player it leaves out, though they were
// shown. It differs from ExpectedNet only in hands with a side pot.
func (p *PlayerHand) ExpectedNetPerPot() (net float64, adjusted bool) { return p.expectedNet(true) }

func (p *PlayerHand) expectedNet(perPot bool) (net float64, adjusted bool) {
	actual := float64(p.Net())
	if !p.Showdown() || p.Reached != River {
		return actual, false
	}
	n := len(p.Players)
	if n > 16 {
		return actual, false
	}
	var contrib [16]Cents
	var folded [16]bool
	last := Preflop
	heroAllIn, heroStreet := false, Preflop // whether and when the hero had nothing behind
	for st := range p.Steps() {
		e := st.Event
		contrib[e.Actor] += e.Amount
		if e.Kind == Fold {
			folded[e.Actor] = true
		}
		if e.Actor == p.Hero && !heroAllIn && (e.AllIn || (p.Players[p.Hero].Stack > 0 && contrib[p.Hero] >= p.Players[p.Hero].Stack)) {
			heroAllIn, heroStreet = true, st.Street
		}
		last = st.Street
	}
	if heroAllIn {
		last = heroStreet
	}
	if last >= River {
		return actual, false
	}
	actor, amount, ok := p.UncalledBet()
	if !ok {
		return actual, false
	}
	contrib[actor] -= amount

	var hands []pk.HoleCards
	var who []int // the player each hand belongs to
	var alive [16]bool
	withChips := 0
	for i := 0; i < n; i++ {
		if folded[i] {
			continue
		}
		pl := &p.Players[i]
		if !pl.HasCards {
			return actual, false
		}
		alive[i] = true
		who = append(who, i)
		hands = append(hands, pl.Cards)
		if pl.Stack-contrib[i] > 0 {
			withChips++
		}
	}
	if (withChips > 1 && !heroAllIn) || !alive[p.Hero] {
		return actual, false
	}
	board := p.Board[:cardsAtStreet[last]]

	won, ok := expectedWinnings(contrib[:n], alive[:n], who, hands, board, perPot)
	if !ok {
		return actual, false
	}
	var pot Cents
	for _, c := range contrib[:n] {
		pot += c
	}
	if pot <= 0 {
		return actual, false
	}
	// Take the rake off the pot in proportion, then the hero's share is what
	// they win less what they put in.
	share := won[p.Hero] * float64(pot-p.Rake) / float64(pot)
	return share - float64(contrib[p.Hero]), true
}

// expectedWinnings returns, for each player still in, the chips they win on
// average from the pot, given the cards on the board so far, 0, 3 or 4 of them.
// contrib is what every player put in, folders included, alive marks the players
// still in, and who and hands list those players and their cards. The result is
// indexed by player. With perPot, each pot's runouts are dealt from a deck that
// lacks only the cards of the players who can win that pot.
func expectedWinnings(contrib []Cents, alive []bool, who []int, hands []pk.HoleCards, board []pk.Card, perPot bool) ([]float64, bool) {
	if len(hands) < 2 || len(board) > 4 || len(board) == 1 || len(board) == 2 {
		return nil, false
	}
	used := pk.NewCardSet(board...)
	for _, h := range hands {
		if used.ContainsAny(h[0], h[1]) {
			return nil, false
		}
		used = used.Insert(h[0]).Insert(h[1])
	}

	// Layers of the pot. A layer's level is a stack depth, and the layer holds the
	// chips between the last level and this one from everyone, folders too.
	var levels []Cents
	for _, i := range who {
		if !slices.Contains(levels, contrib[i]) {
			levels = append(levels, contrib[i])
		}
	}
	slices.Sort(levels)
	type layer struct {
		pot      Cents
		eligible uint32 // bit k set: hands[k] can win this layer
	}
	layers := make([]layer, len(levels))
	prev := Cents(0)
	for l, level := range levels {
		top := level
		if l == len(levels)-1 {
			top = Cents(1<<31 - 1) // anything above the last level is dead money
		}
		for _, c := range contrib {
			layers[l].pot += min(c, top) - min(c, prev)
		}
		for k, i := range who {
			if contrib[i] >= level {
				layers[l].eligible |= 1 << k
			}
		}
		prev = level
	}

	if perPot && len(layers) > 1 {
		// Each pot on its own, as a hand among only the players who can win it.
		won := make([]float64, len(contrib))
		for l := range layers {
			var sub []pk.HoleCards
			var subWho []int
			for k, i := range who {
				if layers[l].eligible>>k&1 != 0 {
					sub = append(sub, hands[k])
					subWho = append(subWho, i)
				}
			}
			if len(sub) == 1 {
				won[subWho[0]] += float64(layers[l].pot)
				continue
			}
			// One chip from each of them makes a single pot that they share by equity.
			one := make([]Cents, len(contrib))
			for _, i := range subWho {
				one[i] = 1
			}
			part, ok := expectedWinnings(one, alive, subWho, sub, board, false)
			if !ok {
				return nil, false
			}
			for _, i := range subWho {
				won[i] += float64(layers[l].pot) * part[i] / float64(len(subWho))
			}
		}
		return won, true
	}

	// Walk every runout once and share each layer among the best hands in it.
	shares := make([][]float64, len(layers))
	for l := range shares {
		shares[l] = make([]float64, len(hands))
	}
	runouts := 0
	if len(hands) == 2 && len(board) == 0 && len(layers) == 1 && !noHeadsUpShortcut {
		// Heads up before the flop: the library's exact count, from its table.
		wins, draws, total := headsUpEquity(hands[0], hands[1])
		shares[0][0] = float64(wins) + float64(draws)/2
		shares[0][1] = float64(total) - shares[0][0]
		runouts = total
	} else {
		var deck []pk.Card
		for c := pk.Card(0); c < 52; c++ {
			if !used.Contains(c) {
				deck = append(deck, c)
			}
		}
		var cards [5]pk.Card
		copy(cards[:], board)
		need := 5 - len(board)
		ranks := make([]pk.HandRank, len(hands))
		var inv [17]float64
		for k := 1; k < len(inv); k++ {
			inv[k] = 1 / float64(k)
		}
		var walk func(start, k int)
		walk = func(start, k int) {
			if k == need {
				runouts++
				for h := range hands {
					ranks[h] = pk.Evaluate7(hands[h][0], hands[h][1], cards[0], cards[1], cards[2], cards[3], cards[4])
				}
				for l := range layers {
					best, winners := pk.HandRank(1<<14), 0
					for h := range hands {
						if layers[l].eligible>>h&1 == 0 {
							continue
						}
						switch {
						case ranks[h] < best:
							best, winners = ranks[h], 1
						case ranks[h] == best:
							winners++
						}
					}
					for h := range hands {
						if layers[l].eligible>>h&1 != 0 && ranks[h] == best {
							shares[l][h] += inv[winners]
						}
					}
				}
				return
			}
			for i := start; i < len(deck); i++ {
				cards[len(board)+k] = deck[i]
				walk(i+1, k+1)
			}
		}
		walk(0, 0)
	}

	won := make([]float64, len(contrib))
	for l := range layers {
		for k, i := range who {
			won[i] += float64(layers[l].pot) * shares[l][k] / float64(runouts)
		}
	}
	return won, true
}

// headsUpEquity is pk.Equity, the count over all 1.7 million boards, looked up in
// the library's table of every heads-up matchup.
func headsUpEquity(a, b pk.HoleCards) (wins, draws, total int) {
	return pk.PreflopEquity(a[0], a[1], b[0], b[1])
}

// noHeadsUpShortcut makes the tests check the general count against the library's.
var noHeadsUpShortcut bool
