package hh

import (
	"regexp"
	"strconv"
	"strings"
)

// Facts that a file states in a way that the hand does not bear out: the chips a
// raise adds in an ante hand, the showdown mark, and the player a summary line names. The parser
// keeps what the file says; a check finds the disagreement and Repair puts it right.

// raiseAmounts returns, for each action, the chips it adds, as they follow from the
// "to" figure of a raise.
//
// In "raises $X to $Y", Y is the player's total bet on the street, antes not
// included. Without antes X is the chips added now, which is Y less what the player
// already has in on the street. In a hand with antes X comes out short by the
// raiser's own ante, so the chips added are taken from Y instead. For a hand
// without antes, and for every action that is not a raise, the figure is the one
// the action has.
func (h *HandHistory) raiseAmounts() []Cents {
	want := make([]Cents, len(h.Actions))
	ante := false
	for i := range h.Actions {
		want[i] = h.Actions[i].Amount
		if h.Actions[i].Kind == PostAnte {
			ante = true
		}
	}
	if !ante {
		return want
	}
	var own [32]Cents // each seat's bet so far on the current street, antes aside
	street := Preflop
	for i := range h.Actions {
		a := &h.Actions[i]
		if a.Seat < 0 || a.Seat >= len(own) {
			return want
		}
		if a.Street != street {
			street = a.Street
			own = [32]Cents{}
		}
		switch a.Kind {
		case PostSmallBlind, PostBigBlind, PostStraddle, Call, Bet:
			own[a.Seat] += a.Amount
		case PostEntry:
			own[a.Seat] += LiveAmount(h.Site, a.Kind, a.Amount, h.BigBlind)
		case Raise:
			if add := a.To - own[a.Seat]; add > 0 {
				want[i] = add
			}
			own[a.Seat] = a.To
		}
	}
	return want
}

// RaiseToMatches reports whether every raise's "to" figure, the player's total
// bet on the street that the file gives, is what the chips add up to: the
// player's bet before it, counted by LiveAmount, plus the chips the raise adds.
// The bets are always counted from the amounts, so a hand whose figures say
// otherwise is one whose amounts cannot be trusted.
func (h *HandHistory) RaiseToMatches() bool {
	var own [32]Cents
	street := Preflop
	for i := range h.Actions {
		a := &h.Actions[i]
		if a.Seat < 0 || a.Seat >= len(own) {
			return true // not a layout this check understands
		}
		if a.Street != street {
			street = a.Street
			own = [32]Cents{}
		}
		if a.Kind == Raise && a.To != 0 && own[a.Seat]+a.Amount != a.To {
			return false
		}
		own[a.Seat] += LiveAmount(h.Site, a.Kind, a.Amount, h.BigBlind)
	}
	return true
}

// RaiseAmountsMatch reports whether every raise adds the chips its "to" figure
// says, see raiseAmounts.
func (h *HandHistory) RaiseAmountsMatch() bool {
	want := h.raiseAmounts()
	for i := range h.Actions {
		if h.Actions[i].Kind == Raise && h.Actions[i].Amount != want[i] {
			return false
		}
	}
	return true
}

// fixRaiseAmounts sets the chips each raise adds from its "to" figure. It returns
// how many it changed.
func (h *HandHistory) fixRaiseAmounts() int {
	want := h.raiseAmounts()
	n := 0
	for i := range h.Actions {
		if h.Actions[i].Kind == Raise && h.Actions[i].Amount != want[i] {
			h.Actions[i].Amount = want[i]
			n++
		}
	}
	return n
}

// ShowdownMatches reports whether a showdown is marked only in a hand that has one.
// Some files print "SHOW DOWN" in every hand, even when all but one player folded;
// a real showdown has at least two players still in.
func (h *HandHistory) ShowdownMatches() bool {
	return !h.Showdown || playersStillIn(h) >= 2
}

// fixShowdown clears the showdown mark of a hand that has no showdown. It reports
// whether it changed the hand.
func (h *HandHistory) fixShowdown() bool {
	if h.ShowdownMatches() {
		return false
	}
	h.Showdown = false
	return true
}

// SummaryNamesMatch reports whether every summary line for a seat names the player
// in that seat.
func (h *HandHistory) SummaryNamesMatch() bool {
	return len(h.StaleSummary) == 0
}

// NamesUnique reports whether no player's name is at more than one seat. The
// action lines name players, not seats, so a name at two seats leaves it open
// whose actions they are.
func (h *HandHistory) NamesUnique() bool {
	seen := map[string]bool{}
	for _, p := range h.Players {
		if seen[p.Name] {
			return false
		}
		seen[p.Name] = true
	}
	return true
}

// dropDuplicateSeats keeps one seat for a name that is at more than one, and
// returns how many seats it dropped. The seat kept is the one the summary
// names, else the first not sitting out, else the first. The name's actions,
// returned bet and cards go to the seat kept.
func (h *HandHistory) dropDuplicateSeats() int {
	if h.NamesUnique() {
		return 0
	}
	keep := map[string]int{} // name -> index in h.Players of the seat kept
	for i, p := range h.Players {
		k, ok := keep[p.Name]
		if !ok {
			keep[p.Name] = i
			continue
		}
		q := h.Players[k]
		if (p.InSummary && !q.InSummary) || (p.InSummary == q.InSummary && q.SittingOut && !p.SittingOut) {
			keep[p.Name] = i
		}
	}
	var kept []Seat
	for i, p := range h.Players {
		k := keep[p.Name]
		if i == k {
			continue
		}
		q := &h.Players[k]
		// Cards and shown hands are put on whichever seat the parser met first.
		if p.HasCards && !q.HasCards {
			q.HasCards, q.Cards, q.Showed, q.HandDescr, q.HandDescr2 = true, p.Cards, p.Showed, p.HandDescr, p.HandDescr2
		}
		if p.HasPartial && !q.HasPartial {
			q.HasPartial, q.PartialCard, q.PartialFirst = true, p.PartialCard, p.PartialFirst
		}
	}
	n := 0
	for i, p := range h.Players {
		if i == keep[p.Name] {
			kept = append(kept, p)
		} else {
			n++
		}
	}
	for i := range h.Actions {
		if k, ok := keep[h.Actions[i].Player]; ok {
			h.Actions[i].Seat = h.Players[k].Seat
		}
	}
	if k, ok := keep[h.Returned.Player]; ok && h.Returned.Amount != 0 {
		h.Returned.Seat = h.Players[k].Seat
	}
	h.Players = kept
	return n
}

// reStaleFold finds the street a stale summary line says its player folded on.
var reStaleFold = regexp.MustCompile(` folded on the (Pre-Flop|Flop|Turn|River)`)

// fixStaleSummary drops the stale summary lines that can be shown to be harmless,
// and returns how many it dropped. Such a line names a player who is not the one in
// the seat. It is dropped if it does not say the player won, since the win would be
// lost, and if what it says does not contradict the actions: a line that says the
// player folded on a street is dropped only if the seat's player folded on that
// street, or if no player sits in the seat. Any other line is kept, and the hand
// stays as it is for the check to reject.
func (h *HandHistory) fixStaleSummary() int {
	var kept []string
	n := 0
	for _, line := range h.StaleSummary {
		if h.staleLineIsHarmless(line) {
			n++
		} else {
			kept = append(kept, line)
		}
	}
	h.StaleSummary = kept
	return n
}

func (h *HandHistory) staleLineIsHarmless(line string) bool {
	num, rest, ok := strings.Cut(strings.TrimPrefix(line, "Seat "), ": ")
	seat, err := strconv.Atoi(num)
	if !ok || err != nil || strings.Contains(rest, " won $") {
		return false
	}
	p := h.PlayerBySeat(seat)
	m := reStaleFold.FindStringSubmatch(rest)
	if m == nil || p == nil {
		return true // it says nothing about the player in the seat, or nobody sits there
	}
	street := streetNamed(m[1])
	for i := range h.Actions {
		a := &h.Actions[i]
		if a.Seat == p.Seat && a.Kind == Fold && a.Street == street {
			return true
		}
	}
	return false
}

// dropEmptyDeadPosts removes the dead blind posts recorded as $0.00, and returns
// how many it removed. Some files write such a line next to the post that carries
// the money, as "posts dead $0.00" followed by "posts $0.75"; the line adds
// nothing. If the amount was really lost, the hand does not add up without it, and
// Repair keeps nothing.
func (h *HandHistory) dropEmptyDeadPosts() int {
	n := 0
	for i := len(h.Actions) - 1; i >= 0; i-- {
		if a := &h.Actions[i]; a.Kind == PostDead && a.Amount == 0 {
			h.removeAction(i)
			n++
		}
	}
	return n
}

// emptyStackActs returns the indices of the actions taken by a player who had
// already put their whole stack in: the stack on the seat line, when it is known.
// Such a player is all-in and cannot act again, whether or not the line that
// emptied the stack says "and is all-in".
func (h *HandHistory) emptyStackActs() []int {
	var stack, invested [32]Cents
	for i := range h.Players {
		if s := h.Players[i].Seat; s >= 0 && s < len(stack) {
			stack[s] = h.Players[i].Stack
		}
	}
	var acts []int
	for i := range h.Actions {
		a := &h.Actions[i]
		if a.Seat < 0 || a.Seat >= len(stack) {
			return nil
		}
		if stack[a.Seat] > 0 && invested[a.Seat] >= stack[a.Seat] {
			acts = append(acts, i)
		}
		invested[a.Seat] += a.Amount
	}
	return acts
}

// NoActsWithEmptyStack reports whether no player acts after putting their whole
// stack in. The file sometimes leaves "and is all-in" off the call that empties a
// stack and then shows the player checking on the streets after it.
func (h *HandHistory) NoActsWithEmptyStack() bool { return len(h.emptyStackActs()) == 0 }

// dropImpossibleChecks removes the checks that no one could have made, and
// returns how many it removed: those of a player who had put their whole stack
// in, whose emptying action it marks all-in, and those on a street where at most
// one player still had chips, so that there was no one to bet against. It removes
// nothing if such a player does more than check.
func (h *HandHistory) dropImpossibleChecks() int {
	acts := h.emptyStackActs()
	if len(acts) == 0 {
		return 0
	}
	for _, i := range acts {
		if h.Actions[i].Kind != Check || h.Actions[i].Amount != 0 {
			return 0
		}
	}
	// The action that emptied each stack is the player's last one before the
	// checks; it takes the all-in mark that the checks may carry.
	for _, i := range acts {
		seat := h.Actions[i].Seat
		for j := i - 1; j >= 0; j-- {
			if h.Actions[j].Seat == seat {
				if h.Actions[j].Amount > 0 {
					h.Actions[j].AllIn = true
				}
				break
			}
		}
	}
	n := 0
	for k := len(acts) - 1; k >= 0; k-- {
		h.removeAction(acts[k])
		n++
	}
	// A street after the flop on which at most one player still has chips has no
	// betting: checks there are what the file wrote for the cards being dealt.
	var stack, invested [32]Cents
	var folded, allIn [32]bool
	for i := range h.Players {
		if s := h.Players[i].Seat; s >= 0 && s < len(stack) {
			stack[s] = h.Players[i].Stack
		}
	}
	seats := h.dealtInSeats()
	street := Preflop
	live := 0
	for i := 0; i < len(h.Actions); i++ {
		a := &h.Actions[i]
		if a.Street != street {
			street = a.Street
			live = 0
			for _, s := range seats {
				if s >= 0 && s < len(stack) && !folded[s] && !allIn[s] && !(stack[s] > 0 && invested[s] >= stack[s]) {
					live++
				}
			}
		}
		if street > Preflop && live < 2 && a.Kind == Check && a.Amount == 0 {
			h.removeAction(i)
			i--
			n++
			continue
		}
		invested[a.Seat] += a.Amount
		if a.Kind == Fold {
			folded[a.Seat] = true
		}
		if a.AllIn {
			allIn[a.Seat] = true
		}
	}
	return n
}

// loneChecks returns the indices of the checks made when no one was left to bet
// against: every other player dealt in had folded or put their whole stack in,
// and the checking player had nothing to call. Hand2Note expects no action then.
// The file writes one for a big blind whose only opponents are all-in for no more
// than the blind.
func (h *HandHistory) loneChecks() []int {
	var stack, invested [32]Cents
	var out [32]bool // folded or all-in
	for i := range h.Players {
		if s := h.Players[i].Seat; s >= 0 && s < len(stack) {
			stack[s] = h.Players[i].Stack
		}
	}
	seats := h.dealtInSeats()
	var checks []int
	for i := range h.Actions {
		a := &h.Actions[i]
		if a.Seat < 0 || a.Seat >= len(stack) {
			return nil
		}
		if a.Kind == Check && a.Amount == 0 {
			lone := true
			for _, s := range seats {
				if s != a.Seat && (!(out[s] || (stack[s] > 0 && invested[s] >= stack[s])) || invested[s] > invested[a.Seat]) {
					lone = false
					break
				}
			}
			if lone {
				checks = append(checks, i)
			}
		}
		invested[a.Seat] += a.Amount
		if a.Kind == Fold || a.AllIn {
			out[a.Seat] = true
		}
	}
	return checks
}

// NoLoneChecks reports whether no player checks when no one is left to bet
// against. See loneChecks.
func (h *HandHistory) NoLoneChecks() bool { return len(h.loneChecks()) == 0 }

// dropLoneChecks removes the checks made when no one was left to bet against, and
// returns how many it removed.
func (h *HandHistory) dropLoneChecks() int {
	checks := h.loneChecks()
	for k := len(checks) - 1; k >= 0; k-- {
		h.removeAction(checks[k])
	}
	return len(checks)
}

// clearStaleSitOut clears the sitting-out mark of a player who posts or acts in
// the hand, and returns how many it cleared. The seat list says the player sits
// out, but the actions show they were dealt in.
func (h *HandHistory) clearStaleSitOut() int {
	n := 0
	for i := range h.Players {
		p := &h.Players[i]
		if !p.SittingOut {
			continue
		}
		for j := range h.Actions {
			if h.Actions[j].Seat == p.Seat {
				p.SittingOut = false
				n++
				break
			}
		}
	}
	return n
}
