// Command hhwinrate reads the pack files in a folder and prints each player's
// win rate in big blinds per 100 hands, best first. Two rates are given: what
// the player won, and the expected win rate (EV), in which a hand that went all-in
// with cards to come is worth the player's equity at the time rather than what
// the board gave them. See PlayerHand.ExpectedNet; the pack files hold it
// already, as PlayerHand.WonEV. Players are sorted by EV.
//
//	go run ./cmd/hhwinrate [-plain] [-h2n] [-min 100000] [-n 50] DIR
package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"sync"

	"github.com/seanq222/hh"
)

type row struct {
	name  string
	hands int
	allIn int     // hands valued by equity
	bb    float64 // net result in big blinds
	ev    float64 // expected net result in big blinds
	net   hh.Cents
	evNet float64  // expected net result in cents
	rake  hh.Cents // rake and jackpot fee paid, from the hands won
}

func (r row) rate() float64   { return 100 * r.bb / float64(r.hands) }
func (r row) evRate() float64 { return 100 * r.ev / float64(r.hands) }

func (r *row) merge(o row) {
	r.hands += o.hands
	r.allIn += o.allIn
	r.bb += o.bb
	r.ev += o.ev
	r.net += o.net
	r.evNet += o.evNet
	r.rake += o.rake
}

// listed is one line of -hands output, kept with the hand's time so the lines can
// be put in order: the blocks of a pack arrive in no particular order.
type listed struct {
	when string
	line string
}

// listHands returns the hands valued by equity, one line each: hand ID, time
// (UTC), the street the money went in on, net, EV and EV less net, in dollars.
func listHands(hands []hh.PlayerHand) []listed {
	var out []listed
	for i := range hands {
		h := &hands[i]
		if h.BigBlind == 0 {
			continue
		}
		if ev, adjusted := expectedNet(h); adjusted {
			// The street of the last action is the one the chips went in on.
			last := hh.Preflop
			for st := range h.Steps() {
				last = st.Street
			}
			net := float64(h.Net())
			when := h.Time.Format("2006-01-02 15:04:05")
			out = append(out, listed{when + fmt.Sprint(h.HandID), fmt.Sprintf("%d\t%s\t%v\t%.2f\t%.2f\t%.2f",
				h.HandID, when[:16], last, net/100, ev/100, (ev-net)/100)})
		}
	}
	return out
}

// h2n says to work out side pots the way Hand2Note does, see
// PlayerHand.ExpectedNetPerPot.
var h2n = flag.Bool("h2n", false, "work out each side pot's equity the way Hand2Note does, with only its own players' cards out of the deck, to compare with Hand2Note's EV")

// expectedNet returns the hand's expected net and whether the equity of an all-in
// changed it. It is the one the pack file holds (see PlayerHand.WonEV), except with
// -h2n, which works it out again. An all-in whose result was settled, as when one
// player was drawing dead, comes out at what it won and is not counted.
func expectedNet(h *hh.PlayerHand) (float64, bool) {
	if *h2n {
		return h.ExpectedNetPerPot()
	}
	return float64(h.NetEV()), h.WonEV != h.Won
}

func main() {
	plain := flag.Bool("plain", false, "count only hands without antes and bomb pots")
	n := flag.Int("n", 0, "print only this many players (0 for all)")
	minHands := flag.Int("min", 0, "leave out players with fewer hands than this")
	noEV := flag.Bool("noev", false, "skip the all-in equity step; the EV column then equals the actual win rate")
	list := flag.String("hands", "", "instead of the table, list this player's hands valued by equity: hand ID, time, net and EV in dollars")
	flag.Parse()
	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: hhwinrate [-plain] [-min N] [-n N] DIR")
		os.Exit(2)
	}
	// The blocks of the packs are read on every CPU; each block's totals are added
	// to its player's under the lock.
	var mu sync.Mutex
	byPlayer := map[string]*row{}
	var lines []listed
	err := hh.ScanPacks(flag.Arg(0), func(player string, hands []hh.PlayerHand) {
		if *list != "" {
			if player == *list {
				l := listHands(hands)
				mu.Lock()
				lines = append(lines, l...)
				mu.Unlock()
			}
			return
		}
		var part row
		for i := range hands {
			h := &hands[i]
			if h.BigBlind == 0 || (*plain && h.Type != hh.Regular) {
				continue
			}
			part.hands++
			bb := float64(h.BigBlind)
			ev, adjusted := float64(h.Net()), false
			if !*noEV {
				ev, adjusted = expectedNet(h)
			}
			if adjusted {
				part.allIn++
			}
			part.bb += float64(h.Net()) / bb
			part.ev += ev / bb
			part.evNet += ev
			part.net += h.Net()
			part.rake += h.RakePaid()
		}
		mu.Lock()
		r := byPlayer[player]
		if r == nil {
			r = &row{name: player}
			byPlayer[player] = r
		}
		r.merge(part)
		mu.Unlock()
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	if *list != "" {
		sort.Slice(lines, func(i, j int) bool { return lines[i].when < lines[j].when })
		for _, l := range lines {
			fmt.Println(l.line)
		}
		return
	}

	var rows []row
	for _, r := range byPlayer {
		if r.hands > 0 && r.hands >= *minHands {
			rows = append(rows, *r)
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].evRate() > rows[j].evRate() })
	if *n > 0 && *n < len(rows) {
		rows = rows[:*n]
	}
	fmt.Printf("%-28s %10s %9s %12s %10s %12s %12s %12s\n", "player", "hands", "all-ins", "EV bb/100", "bb/100", "EV net $", "net $", "rake paid $")
	for _, r := range rows {
		fmt.Printf("%-28s %10d %9d %12.2f %10.2f %12.2f %12.2f %12.2f\n", r.name, r.hands, r.allIn, r.evRate(), r.rate(),
			r.evNet/100, float64(r.net)/100, float64(r.rake)/100)
	}
}
