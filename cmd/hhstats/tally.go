package main

import "github.com/seanq222/hh"

// A stat is how often something was done out of the chances to do it.
type stat struct{ n, d int64 }

// Stats are counted by number, not name, which is much faster: each is
// numbered by newStat before main runs, and statNames names them.
type statID int

var statNames []string

func newStat(name string) statID {
	statNames = append(statNames, name)
	return statID(len(statNames) - 1)
}

// A byGroup numbers a stat split by groupNames, as "open CO".
type byGroup [len(groupNames)]statID

func newByGroup(prefix string) (b byGroup) {
	for i, g := range groupNames {
		b[i] = newStat(prefix + " " + g)
	}
	return b
}

var (
	vpipS, pfrS = newStat("VPIP"), newStat("PFR")

	openS, openG      = newStat("open"), newByGroup("open")
	limpS, limpFoldS  = newStat("limp"), newStat("limp fold")
	stealS, stealG    = newStat("steal"), newByGroup("steal")
	callOpenS         = newStat("call open")
	callOpenG         = newByGroup("call open")
	threeBetS         = newStat("3-bet")
	threeBetG         = newByGroup("3-bet")
	threeBetIPS       = newStat("3-bet IP")
	threeBetIPG       = newByGroup("3-bet IP")
	threeBetOOPS      = newStat("3-bet OOP")
	threeBetOOPG      = newByGroup("3-bet OOP")
	squeezeS          = newStat("squeeze")
	vsStealFoldG      = newByGroup("vs steal fold")
	vsStealCallG      = newByGroup("vs steal call")
	vsStealThreeBetG  = newByGroup("vs steal 3-bet")
	foldTo3BetS       = newStat("fold to 3-bet")
	foldTo3BetG       = newByGroup("fold to 3-bet")
	fourBetS          = newStat("4-bet")
	stealFoldTo3BetS  = newStat("steal fold to 3-bet")
	stealFoldTo3BetG  = newByGroup("steal fold to 3-bet")
	stealFourBetS     = newStat("steal 4-bet")
	stealFourBetG     = newByGroup("steal 4-bet")
	callFourBetS      = newStat("call 4-bet")
	foldTo4BetS       = newStat("fold to 4-bet")
	fiveBetS          = newStat("5-bet")
	fourBetFoldS      = newStat("4-bet fold")
	stealFourBetFoldS = newStat("steal 4-bet fold")
	stealFourBetFoldG = newByGroup("steal 4-bet fold")

	cbetS = [hh.River + 1]statID{hh.Flop: newStat("cbet flop"), hh.Turn: newStat("cbet turn"), hh.River: newStat("cbet river")}
	donkS = [hh.River + 1]statID{hh.Flop: newStat("donk flop"), hh.Turn: newStat("donk turn"), hh.River: newStat("donk river")}

	foldToCbetS = [hh.River + 1]statID{hh.Flop: newStat("fold to cbet flop"), hh.Turn: newStat("fold to cbet turn"), hh.River: newStat("fold to cbet river")}

	missedIPS, missedIPFoldS   = newStat("missed cbet IP rate"), newStat("missed cbet IP fold")
	missedOOPS, missedOOPFoldS = newStat("missed cbet OOP rate"), newStat("missed cbet OOP fold")
	vsMissedS                  = newStat("vs missed cbet")
	vsMissedAnyIPS             = newStat("vs missed any IP")
	vsMissedAnyOOPTurnS        = newStat("vs missed any OOP turn")
	vsMissedBetIPS             = newStat("vs missed cbet bet IP")
	vsMissedCheckFoldIPS       = newStat("vs missed cbet check|fold")
	vsMissedBetOOPTurnS        = newStat("vs missed cbet bet OOP turn")
	vsMissedCheckFoldS         = newStat("vs missed cbet check-fold")

	// vsCbetS[pot][fold, call, raise] against the flop c-bet, by potNames.
	vsCbetS = func() (v [len(potNames)][3]statID) {
		for i, pot := range potNames {
			for j, act := range []string{"fold", "call", "raise"} {
				v[i][j] = newStat("vs cbet " + pot + " " + act)
			}
		}
		return v
	}()

	wtsdS, wsdS, wwsfS = newStat("WTSD"), newStat("W$SD"), newStat("WWSF")
)

// potNames names the preflop pots that the stats against the c-bet split.
var potNames = [...]string{"raised pot", "3-bet pot"}

type stats struct {
	hands   int64
	net, ev float64     // bb
	c       []stat      // by statID
	aggr    [3][5]int64 // [street-1][fold, check, call, bet, raise]
	streets [3][2]int64 // [street-1][hands with an action on the street, those with a bet or raise]

	steps []hh.Step // the hand being tallied, kept to save allocating it
}

func (st *stats) hit(id statID, yes bool) {
	c := &st.c[id]
	c.d++
	if yes {
		c.n++
	}
}

// byName returns the stats with any chances, by name.
func (st *stats) byName() map[string]*stat {
	m := map[string]*stat{}
	for id, c := range st.c {
		if c.d > 0 {
			m[statNames[id]] = &c
		}
	}
	return m
}

// groupNames are the preflop positions as Hand2Note names them at a 6-max
// table.
var groupNames = [...]string{"EP", "MP", "CO", "BTN", "SB", "BB", "?"}

// group returns the index in groupNames of a preflop position, with n players
// dealt in: at five or fewer the first seat is MP, not EP.
func group(p hh.Position, n int) int {
	switch p {
	case hh.UTG, hh.UTG1, hh.UTG2, hh.LJ:
		if n <= 5 {
			return 1
		}
		return 0
	case hh.HJ:
		return 1
	case hh.CO:
		return 2
	case hh.BTN:
		return 3
	case hh.SB:
		return 4
	case hh.BB:
		return 5
	}
	return 6
}

// preflopPos is player a's position as Hand2Note names it preflop: the button of a
// heads-up hand, who posts the small blind, is the small blind.
func preflopPos(p *hh.PlayerHand, a int) hh.Position {
	if pos := p.Players[a].Position; !(pos == hh.BTN && len(p.Players) == 2) {
		return pos
	}
	return hh.SB
}

func isRaise(k hh.ActionKind) bool { return k == hh.Raise || k == hh.Bet }

func kindIndex(k hh.ActionKind) int {
	switch k {
	case hh.Fold:
		return 0
	case hh.Check:
		return 1
	case hh.Call:
		return 2
	case hh.Bet:
		return 3
	case hh.Raise:
		return 4
	}
	return -1
}

// tally counts one player view into st. The definitions follow Hand2Note's
// default stats; the comments below say where they are not what the name
// suggests. Clean means
// no one had called a raise before the next one came.
func tally(p *hh.PlayerHand, st *stats) {
	h := p.Hero
	pos := preflopPos(p, int(h))
	g := group(pos, len(p.Players))
	bb := float64(p.BigBlind)
	st.hands++
	in := p.Invested() // as Net and NetEV, but replaying the hand once
	st.net += float64(p.Won-in) / bb
	st.ev += float64(p.WonEV-in) / bb

	steps := st.steps[:0]
	for s := range p.Steps() {
		steps = append(steps, s)
	}
	st.steps = steps
	n := len(p.Players)
	folded := make([]bool, n)
	order := func(a int) int { return p.Players[a].Position.PostflopOrder() }

	// Preflop.
	raises, limpers, callers := 0, 0, 0
	var raisers []int
	var raiseCallers []int // callers of the previous raise when each raise was made
	heroRaise := 0         // which raise the hero's last raise was: 1 open, 2 3-bet...
	heroLimped, heroActed, vpip, pfr := false, false, false, false
	steal := false // the hero opened from the button or small blind
	flopAt := -1
	var heroIn hh.Cents
	lastRaiseAllIn := false
	limpThreeBet := false
	// threeBet counts a 3-bet chance against the first raise (by the player
	// in position op): the total counts squeezes too, the split by position and
	// in or out of position only opens no one has called yet.
	threeBet := func(k hh.ActionKind, op hh.Position) {
		st.hit(threeBetS, isRaise(k))
		if callers == 0 {
			st.hit(threeBetG[g], isRaise(k))
			// In position by position: Hand2Note splits only the blinds, taking
			// the names, so the big blind is in position against a heads-up
			// button; EP to the button count every chance as in position. The
			// totals use the seats' real order.
			if (pos != hh.SB && pos != hh.BB) || pos.PostflopOrder() > op.PostflopOrder() {
				st.hit(threeBetIPG[g], isRaise(k))
			} else {
				st.hit(threeBetOOPG[g], isRaise(k))
			}
			if order(int(h)) > order(raisers[0]) {
				st.hit(threeBetIPS, isRaise(k))
			} else {
				st.hit(threeBetOOPS, isRaise(k))
			}
		} else {
			st.hit(squeezeS, isRaise(k))
		}
	}
	for j, s := range steps {
		e := s.Event
		if s.Street != hh.Preflop {
			flopAt = j
			break
		}
		allInBefore := lastRaiseAllIn // whether the bet the actor faces is all-in
		if isRaise(e.Kind) {
			lastRaiseAllIn = e.AllIn
		}
		heroBefore := heroIn // what the hero had put in before this action
		if int(e.Actor) == int(h) {
			heroIn += e.Amount
		}
		switch e.Kind {
		case hh.PostAnte, hh.PostDead, hh.PostEntry, hh.PostSmallBlind, hh.PostBigBlind, hh.PostStraddle:
			continue
		}
		a := int(e.Actor)
		if a != int(h) {
			switch {
			case e.Kind == hh.Fold:
				folded[a] = true
			case isRaise(e.Kind):
				raises++
				raisers = append(raisers, a)
				raiseCallers = append(raiseCallers, callers)
				callers = 0
			case e.Kind == hh.Call:
				if raises == 0 {
					limpers++
				} else {
					callers++
				}
			}
			continue
		}
		// The hero's action.
		if !heroActed {
			heroActed = true
			// The total open raise and the limp count spots with limpers too,
			// as in Hand2Note; the total open also counts the big blind raising
			// limpers, the limp leaves out the small blind.
			if raises == 0 {
				st.hit(openS, isRaise(e.Kind))
				if pos != hh.SB && pos != hh.BB {
					st.hit(limpS, e.Kind == hh.Call)
				}
			}
			switch {
			case raises == 0 && limpers == 0 && pos != hh.BB:
				st.hit(openG[g], isRaise(e.Kind))
				if pos == hh.BTN || pos == hh.SB {
					st.hit(stealG[g], isRaise(e.Kind))
					st.hit(stealS, isRaise(e.Kind))
					steal = isRaise(e.Kind)
				}
			case raises == 1:
				op := preflopPos(p, raisers[0])
				threeBet(e.Kind, op)
				// Calls count only opens no one has called yet, as in Hand2Note.
				if callers == 0 {
					st.hit(callOpenG[g], e.Kind == hh.Call)
					st.hit(callOpenS, e.Kind == hh.Call)
				}
				// Against a steal: an open from the button or small blind with no
				// limpers before it and no callers, as in Hand2Note.
				if callers == 0 && limpers == 0 && (op == hh.BTN || op == hh.SB) && (pos == hh.SB || pos == hh.BB) {
					st.hit(vsStealFoldG[g], e.Kind == hh.Fold)
					st.hit(vsStealCallG[g], e.Kind == hh.Call)
					st.hit(vsStealThreeBetG[g], isRaise(e.Kind))
				}
			}
			if raises == 0 && e.Kind == hh.Call && pos != hh.BB {
				heroLimped = true
			}
		} else {
			switch {
			case heroRaise == 1 && raises == 2:
				// Hand2Note counts only a clean 3-bet: no one had called the open
				// or has called the 3-bet. The 4-bet also needs the 3-bet not all-in.
				if raiseCallers[1] == 0 && callers == 0 {
					st.hit(foldTo3BetG[g], e.Kind == hh.Fold)
					st.hit(foldTo3BetS, e.Kind == hh.Fold)
					if !allInBefore {
						st.hit(fourBetS, isRaise(e.Kind))
					}
					if steal {
						st.hit(stealFoldTo3BetG[g], e.Kind == hh.Fold)
						st.hit(stealFoldTo3BetS, e.Kind == hh.Fold)
						if !allInBefore {
							st.hit(stealFourBetG[g], isRaise(e.Kind))
							st.hit(stealFourBetS, isRaise(e.Kind))
						}
					}
				}
			case heroRaise == 2 && raises == 3:
				// As in Hand2Note: the fold and 5-bet need every raise clean (no
				// callers of the open, the 3-bet or the 4-bet), the call only the
				// 3-bet and 4-bet; the 5-bet also needs the hero able to raise.
				clean := raiseCallers[2] == 0 && callers == 0
				if clean {
					st.hit(callFourBetS, e.Kind == hh.Call)
				}
				if clean && raiseCallers[1] == 0 {
					st.hit(foldTo4BetS, e.Kind == hh.Fold)
					if !allInBefore && s.ToCall < p.Players[h].Stack-heroBefore {
						st.hit(fiveBetS, isRaise(e.Kind))
					}
				}
			case heroRaise == 3 && raises == 4:
				// The hero opened and 4-bet, with no callers anywhere.
				if raisers[0] == int(h) && raiseCallers[1] == 0 && raiseCallers[2] == 0 && raiseCallers[3] == 0 && callers == 0 {
					st.hit(fourBetFoldS, e.Kind == hh.Fold)
					if steal {
						st.hit(stealFourBetFoldG[g], e.Kind == hh.Fold)
						st.hit(stealFourBetFoldS, e.Kind == hh.Fold)
					}
				}
			case heroLimped && raises == 1 && heroRaise == 0 && !limpThreeBet:
				// Hand2Note counts a limper facing the first raise as a 3-bet chance.
				limpThreeBet = true
				threeBet(e.Kind, preflopPos(p, raisers[0]))
				if callers == 0 {
					st.hit(limpFoldS, e.Kind == hh.Fold)
				}
				heroLimped = false
			case heroLimped && raises >= 1:
				// Hand2Note counts the limp-fold only against the first raise
				// with no callers of it yet.
				if raises == 1 && callers == 0 {
					st.hit(limpFoldS, e.Kind == hh.Fold)
				}
				heroLimped = false
			}
		}
		switch {
		case e.Kind == hh.Fold:
			folded[h] = true
		case isRaise(e.Kind):
			raises++
			raisers = append(raisers, int(h))
			raiseCallers = append(raiseCallers, callers)
			heroRaise = raises
			callers = 0
			vpip, pfr = true, true
		case e.Kind == hh.Call:
			vpip = true
		}
	}
	if heroActed {
		st.hit(vpipS, vpip)
		st.hit(pfrS, pfr)
	}
	if flopAt < 0 || folded[h] {
		return
	}

	// Postflop.
	pfa := -1
	if len(raisers) > 0 {
		pfa = raisers[len(raisers)-1]
	}
	pot := raises - 1 // in potNames, if 0 or 1
	inHand := func(a int) bool { return !folded[a] }
	lastToAct := func() int { // the player in position among those left
		best := -1
		for a := range n {
			if inHand(a) && (best < 0 || order(a) > order(best)) {
				best = a
			}
		}
		return best
	}
	heroIP := lastToAct() == int(h)
	// lastCall[street]: the street's last action other than folds was the
	// hero's call.
	var lastCall [5]bool
	var preCall float64 // the hero's last preflop call, in big blinds
	for _, s := range steps {
		if s.Street == hh.Preflop && int(s.Event.Actor) == int(h) && s.Event.Kind == hh.Call {
			preCall = float64(s.ToCall) / bb
		}
	}

	for _, s := range steps {
		if e := s.Event; e.Kind != hh.Fold {
			lastCall[s.Street] = int(e.Actor) == int(h) && e.Kind == hh.Call
		}
	}
	// The hero's call was the last preflop action other than folds: no one
	// called or raised behind him.
	heroCalledLast := false
	for j := flopAt - 1; j >= 0; j-- {
		if e := steps[j].Event; e.Kind != hh.Fold {
			heroCalledLast = int(e.Actor) == int(h) && e.Kind == hh.Call
			break
		}
	}
	// heroLast[street]: the hero's last action on the street, which is what
	// Hand2Note's "last player's action on street" means.
	var heroLast [5]hh.ActionKind
	for _, s := range steps {
		if int(s.Event.Actor) == int(h) {
			heroLast[s.Street] = s.Event.Kind
		}
	}
	heroFirstToAct := func() bool {
		for a := range n {
			if a != int(h) && inHand(a) && order(a) < order(int(h)) {
				return false
			}
		}
		return true
	}

	// Per street state.
	var aggressor [5]int // the last bettor or raiser on each street, -1 if none
	for i := range aggressor {
		aggressor[i] = -1
	}
	aggressor[hh.Preflop] = pfa
	var cbet [5]bool        // the hero c-bet the street
	var villainCbet [5]bool // the preflop raiser (not the hero) bet first on the street
	var heroCalledCbet [5]bool
	var heroFirst [5]bool   // the hero has acted on the street
	var aggrStreet [5]bool  // the hero bet or raised on the street
	var bets [5]int         // bets and raises so far on the street
	var heroChecked [5]bool // the hero's first action on the street was a check
	var pfaChecked [5]bool  // the villain preflop raiser checked on the street
	missedOOP, missedIP := false, false
	vsMissedIP, turnCheckFold := false, false
	sd := false
	others := func() int {
		c := 0
		for a := range n {
			if a != int(h) && inHand(a) {
				c++
			}
		}
		return c
	}

	for _, s := range steps[flopAt:] {
		e := s.Event
		a := int(e.Actor)
		street := s.Street
		if a != int(h) {
			if e.Kind == hh.Fold {
				folded[a] = true
			}
			if a == pfa && e.Kind == hh.Check && bets[street] == 0 {
				pfaChecked[street] = true
			}
			if isRaise(e.Kind) {
				if a == pfa && bets[street] == 0 && pfa != int(h) {
					prevOK := street == hh.Flop || villainCbet[street-1] && heroCalledCbet[street-1]
					villainCbet[street] = prevOK
				}
				bets[street]++
				aggressor[street] = a
			}
			continue
		}
		k := kindIndex(e.Kind)
		if k >= 0 {
			st.aggr[street-1][k]++
		}
		first := !heroFirst[street]
		heroFirst[street] = true
		if first {
			st.streets[street-1][0]++
		}
		if isRaise(e.Kind) && !aggrStreet[street] {
			aggrStreet[street] = true
			st.streets[street-1][1]++
		}

		// C-bets.
		if first && s.ToCall == 0 {
			prev := aggressor[street-1]
			if prev == int(h) && (street == hh.Flop || cbet[street-1]) {
				// C-bets count only pots the hero opened with the only raise, as
				// in Hand2Note.
				if raises == 1 {
					st.hit(cbetS[street], e.Kind == hh.Bet)
				}
				cbet[street] = e.Kind == hh.Bet
				// Missed c-bets count only pots the hero opened with the only
				// raise, out of position meaning first to act, as in Hand2Note.
				if street == hh.Flop && raises == 1 && (heroIP || heroFirstToAct()) {
					if heroIP {
						st.hit(missedIPS, e.Kind == hh.Check)
						missedIP = e.Kind == hh.Check
					} else {
						st.hit(missedOOPS, e.Kind == hh.Check)
						missedOOP = e.Kind == hh.Check
					}
				}
			} else if prev >= 0 && prev != int(h) && inHand(prev) && order(prev) > order(int(h)) {
				// Hand2Note counts a flop donk only by a hero first to act who
				// called 2bb or more preflop.
				if street == hh.Flop && heroFirstToAct() && preCall >= 2 {
					st.hit(donkS[hh.Flop], e.Kind == hh.Bet)
				}
			}
			// A turn or river donk, as Hand2Note counts it: the hero's last action
			// preflop and on the street before was a call, and he is first to act.
			if street > hh.Flop && heroFirstToAct() && heroLast[hh.Preflop] == hh.Call && heroLast[street-1] == hh.Call {
				st.hit(donkS[street], e.Kind == hh.Bet)
			}
			// Against a missed c-bet, again only in pots opened with the only
			// raise: in position meaning last to act, and the hero's call the
			// last preflop, out of position first to act.
			if street == hh.Flop && pfa >= 0 && pfa != int(h) && pfaChecked[hh.Flop] && bets[hh.Flop] == 0 &&
				raises == 1 && heroIP && heroCalledLast {
				st.hit(vsMissedBetIPS, e.Kind == hh.Bet)
				vsMissedIP = e.Kind == hh.Check
			}
			// The page's vs. Missed Continuation Bet sums Hand2Note's "Called Any
			// Raise" versions instead: any raised pot, heads up, the hero after the
			// raiser on the flop or before him on the turn of a checked flop.
			if pfa >= 0 && pfa != int(h) && inHand(pfa) && others() == 1 && bets[hh.Flop] == 0 && pfaChecked[hh.Flop] {
				switch {
				case street == hh.Flop && order(int(h)) > order(pfa):
					st.hit(vsMissedAnyIPS, e.Kind == hh.Bet)
					st.hit(vsMissedS, e.Kind == hh.Bet)
				case street == hh.Turn && order(int(h)) < order(pfa):
					st.hit(vsMissedAnyOOPTurnS, e.Kind == hh.Bet)
					st.hit(vsMissedS, e.Kind == hh.Bet)
				}
			}
			if street == hh.Turn && pfa >= 0 && pfa != int(h) && inHand(pfa) && bets[hh.Flop] == 0 &&
				pfaChecked[hh.Flop] && order(pfa) > order(int(h)) && raises == 1 && heroFirstToAct() && others() == 1 {
				// The probe counts only heads up.
				st.hit(vsMissedBetOOPTurnS, e.Kind == hh.Bet)
			}
			heroChecked[street] = e.Kind == hh.Check
		}
		// Facing a bet.
		if s.ToCall > 0 {
			// Fold to the river c-bet, as Hand2Note counts it: the hero called
			// 2bb or more preflop, then a single bet on the flop and on the
			// turn, each call the street's last action other than folds; any
			// villain bets the river.
			if street == hh.River && bets[hh.River] == 1 && preCall >= 2 &&
				lastCall[hh.Flop] && bets[hh.Flop] == 1 && lastCall[hh.Turn] && bets[hh.Turn] == 1 {
				st.hit(foldToCbetS[hh.River], e.Kind == hh.Fold)
			}
			if villainCbet[street] && bets[street] == 1 && !heroCalledCbet[street] && street != hh.River {
				// As in Hand2Note: the flop and turn folds need a preflop call of
				// 2bb or more (the turn's count and result match Hand2Note's).
				// Against the flop c-bet by pot: in a raised pot the
				// hero's call closed preflop; in a 3-bet pot he opened and called
				// 2bb or more, heads up.
				switch {
				case street == hh.Flop && preCall >= 2:
					st.hit(foldToCbetS[hh.Flop], e.Kind == hh.Fold)
				case street == hh.Turn && preCall >= 2:
					st.hit(foldToCbetS[hh.Turn], e.Kind == hh.Fold)
				}
				heroOpened := len(raisers) > 0 && raisers[0] == int(h)
				if street == hh.Flop && (pot == 0 && heroCalledLast ||
					pot == 1 && heroOpened && preCall >= 2 && others() == 1) {
					st.hit(vsCbetS[pot][0], e.Kind == hh.Fold)
					st.hit(vsCbetS[pot][1], e.Kind == hh.Call)
					st.hit(vsCbetS[pot][2], isRaise(e.Kind))
				}
				heroCalledCbet[street] = e.Kind == hh.Call
			}
			if missedOOP && street == hh.Flop {
				st.hit(missedOOPFoldS, e.Kind == hh.Fold)
				missedOOP = false
			}
			if missedIP && street == hh.Turn {
				if others() == 1 { // heads up only, as in Hand2Note
					st.hit(missedIPFoldS, e.Kind == hh.Fold)
				}
				missedIP = false
			}
			if vsMissedIP && street == hh.Turn {
				st.hit(vsMissedCheckFoldIPS, e.Kind == hh.Fold)
				vsMissedIP = false
			}
			// Hand2Note's check-fold out of position: a single raised pot, heads
			// up, the hero's last action preflop a call and on the flop a check,
			// facing the first bet on the turn. Despite the name, no position
			// condition fits.
			if street == hh.Turn && bets[hh.Turn] == 1 && raises == 1 && others() == 1 &&
				heroLast[hh.Preflop] == hh.Call && heroLast[hh.Flop] == hh.Check && !turnCheckFold {
				st.hit(vsMissedCheckFoldS, e.Kind == hh.Fold)
				turnCheckFold = true
			}
		}
		if e.Kind == hh.Fold {
			folded[h] = true
		}
		if isRaise(e.Kind) {
			bets[street]++
			aggressor[street] = int(h)
		}
	}
	sd = !folded[h] && p.Showdown()
	st.hit(wtsdS, sd)
	if sd {
		st.hit(wsdS, p.Won > 0)
	}
	st.hit(wwsfS, p.Won > 0)
}

// merge adds o's counts to st.
func (st *stats) merge(o *stats) {
	st.hands += o.hands
	st.net += o.net
	st.ev += o.ev
	for id := range st.c {
		st.c[id].n += o.c[id].n
		st.c[id].d += o.c[id].d
	}
	for i := range st.aggr {
		for j := range st.aggr[i] {
			st.aggr[i][j] += o.aggr[i][j]
		}
		for j := range st.streets[i] {
			st.streets[i][j] += o.streets[i][j]
		}
	}
}
