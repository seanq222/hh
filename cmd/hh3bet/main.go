// Command hh3bet reads the pack files in a folder and prints, for each player,
// how often they 3-bet from one position when a player in another opens:
// everyone before the opener folds, the opener raises (an all-in counts), everyone
// between the opener and the player folds, and the player then raises (a 3-bet),
// calls or folds. With -opener any the open can come from anyone before the
// player, so the spot is any single open raise. Calls are counted as well, with
// the ratio of calls to 3-bets and the calls' share of the two, and beside them
// each player's hands and EV win rate in bb/100 over all their hands. Only
// hands with three or more players dealt in count, since heads-up the button
// opens from the small blind; bomb pots and straddled hands are left out.
// Players are sorted by how often they were in the spot.
//
//	go run ./cmd/hh3bet [-pos BB] [-opener BTN|any] [-min 1000] [-n 50] DIR
package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/seanq222/hh"
)

// parsePosition reads a position as Position.String writes it.
func parsePosition(s string) (hh.Position, bool) {
	for p := hh.UTG; p <= hh.BB; p++ {
		if p.String() == s {
			return p, true
		}
	}
	return 0, false
}

// anyOpener as the opener accepts an open from any position before the hero.
const anyOpener hh.Position = 0

// facedOpen reports whether the hero of p, sitting at pos, faced a single open
// raise by the player at opener (or anyone, with anyOpener) with everyone else
// before them folding, and if so what the hero did: Raise, Call or Fold.
func facedOpen(p *hh.PlayerHand, pos, opener hh.Position) (faced bool, kind hh.ActionKind) {
	if len(p.Players) < 3 || p.Type == hh.Bomb || p.Players[p.Hero].Position != pos {
		return false, 0
	}
	opened := false
	for st := range p.Steps() {
		if st.Street != hh.Preflop {
			break
		}
		e := st.Event
		switch e.Kind {
		case hh.PostStraddle:
			return false, 0
		case hh.PostSmallBlind, hh.PostBigBlind, hh.PostAnte, hh.PostDead, hh.PostEntry:
			continue
		}
		at := p.Players[e.Actor].Position
		switch {
		case e.Actor == p.Hero:
			return opened, e.Kind // before an open, the hero was not in the spot
		case !opened && (at == opener || opener == anyOpener && e.Kind != hh.Fold):
			if e.Kind != hh.Raise {
				return false, 0 // the opener limped or folded
			}
			opened = true
		case e.Kind != hh.Fold:
			return false, 0 // someone else came in first, or called or raised the open
		}
	}
	return false, 0
}

type row struct {
	name                  string
	faced, raised, called int
	hands                 int     // all the player's hands, for their win rate
	evBB                  float64 // EV net over all their hands, in big blinds
}

func (r *row) add(o row) {
	r.faced += o.faced
	r.raised += o.raised
	r.called += o.called
	r.hands += o.hands
	r.evBB += o.evBB
}

// count adds a spot in which the hero did kind.
func (r *row) count(kind hh.ActionKind) {
	r.faced++
	switch kind {
	case hh.Raise:
		r.raised++
	case hh.Call:
		r.called++
	}
}

func main() {
	posName := flag.String("pos", "BB", "the position of the player who faces the open")
	openerName := flag.String("opener", "BTN", "the position of the player who opens, or any for a single open raise from anyone before -pos")
	minFaced := flag.Int("min", 0, "leave out players who were in the spot fewer times than this")
	n := flag.Int("n", 0, "print only this many players (0 for all)")
	only := flag.String("player", "", "instead, print this player's 3-bet rate from every position against an open from every position before it; * for all players together")
	byStake := flag.Bool("stakes", false, "a row for each player at each stake (site and big blind), and the totals of each stake")
	flag.Parse()
	if *only != "" && flag.NArg() == 1 {
		playerTable(flag.Arg(0), *only)
		return
	}
	pos, ok1 := parsePosition(*posName)
	opener, ok2 := parsePosition(*openerName)
	if *openerName == "any" {
		opener, ok2 = anyOpener, true
	}
	if flag.NArg() != 1 || !ok1 || !ok2 || pos == opener {
		fmt.Fprintln(os.Stderr, "usage: hh3bet [-pos BB] [-opener BTN|any] [-min N] [-n N] DIR; positions are UTG UTG+1 UTG+2 LJ HJ CO BTN SB BB")
		os.Exit(2)
	}
	start := time.Now()
	// The blocks of the packs are read on every CPU; each block's counts are added
	// to its player's under the lock.
	var mu sync.Mutex
	byPlayer := map[string]*row{}
	stakeTotals := map[string]*row{}
	var all row
	err := hh.ScanPacks(flag.Arg(0), func(player string, hands []hh.PlayerHand) {
		parts := map[string]*row{} // by row name: the player, or the player at a stake
		for i := range hands {
			h := &hands[i]
			key := player
			if *byStake {
				key = player + " @ " + stake(h)
			}
			part := parts[key]
			if part == nil {
				part = &row{name: key}
				parts[key] = part
			}
			if h.BigBlind > 0 {
				part.hands++
				part.evBB += float64(h.NetEV()) / float64(h.BigBlind)
			}
			if faced, kind := facedOpen(h, pos, opener); faced {
				part.count(kind)
			}
		}
		mu.Lock()
		for key, part := range parts {
			r := byPlayer[key]
			if r == nil {
				r = &row{name: key}
				byPlayer[key] = r
			}
			r.add(*part)
			all.add(*part)
			if *byStake {
				s := key[len(player)+len(" @ "):]
				if stakeTotals[s] == nil {
					stakeTotals[s] = &row{name: "all players @ " + s}
				}
				stakeTotals[s].add(*part)
			}
		}
		mu.Unlock()
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}

	var rows []row
	for _, r := range byPlayer {
		if r.faced > 0 && r.faced >= *minFaced {
			rows = append(rows, *r)
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].faced > rows[j].faced })
	if *n > 0 && *n < len(rows) {
		rows = rows[:*n]
	}
	against := opener.String()
	if opener == anyOpener {
		against = "single"
	}
	fmt.Printf("%s against a %s open raise\n", pos, against)
	fmt.Printf("%-40s %10s %10s %8s %10s %8s %11s %8s %10s %10s\n", "player", "spots", "3-bets", "3-bet %", "calls", "call %",
		"call/3-bet", "call sh%", "hands", "EV bb/100")
	line := func(r row) {
		ratio := "-"
		if r.raised > 0 {
			ratio = fmt.Sprintf("%.2f", float64(r.called)/float64(r.raised))
		}
		f := float64(max(r.faced, 1))
		share := 100 * float64(r.called) / float64(max(r.raised+r.called, 1))
		fmt.Printf("%-40s %10d %10d %8.1f %10d %8.1f %11s %8.1f %10d %10.2f\n", r.name, r.faced, r.raised, 100*float64(r.raised)/f,
			r.called, 100*float64(r.called)/f, ratio, share, r.hands, 100*r.evBB/float64(max(r.hands, 1)))
	}
	for _, r := range rows {
		line(r)
	}
	if *byStake {
		var totals []row
		for _, r := range stakeTotals {
			totals = append(totals, *r)
		}
		sort.Slice(totals, func(i, j int) bool { return totals[i].faced > totals[j].faced })
		for _, r := range totals {
			line(r)
		}
	}
	all.name = "all players"
	line(all)
	fmt.Fprintf(os.Stderr, "%d rows in %.1fs\n", len(byPlayer), time.Since(start).Seconds())
}

// stake names a hand's site and stake by its big blind, as in "WPN NL50" for a
// $0.25/$0.50 game.
func stake(h *hh.PlayerHand) string {
	return fmt.Sprintf("%s NL%d", h.Site, h.BigBlind)
}

// playerTable prints one player's 3-bet rate for every pair of positions, the
// opener acting before the player.
func playerTable(dir, name string) {
	start := time.Now()
	type pair struct{ pos, opener hh.Position }
	var mu sync.Mutex
	counts := map[pair]*row{}
	err := hh.ScanPacks(dir, func(player string, hands []hh.PlayerHand) {
		if name != "*" && player != name {
			return
		}
		local := map[pair]*row{}
		for i := range hands {
			pos := hands[i].Players[hands[i].Hero].Position
			for opener := hh.UTG; opener < pos; opener++ {
				if faced, kind := facedOpen(&hands[i], pos, opener); faced {
					r := local[pair{pos, opener}]
					if r == nil {
						r = &row{}
						local[pair{pos, opener}] = r
					}
					r.count(kind)
					break // only one player opens a hand
				}
			}
		}
		mu.Lock()
		for k, r := range local {
			if counts[k] == nil {
				counts[k] = &row{}
			}
			counts[k].add(*r)
		}
		mu.Unlock()
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	// Only the positions the player's tables have; a 6-max table has no UTG+1.
	var seen [hh.BB + 1]bool
	for k := range counts {
		seen[k.pos], seen[k.opener] = true, true
	}
	fmt.Printf("%s: 3-bet %% against an open (spots in brackets)\n", name)
	fmt.Printf("%-8s", "3-bettor")
	for opener := hh.UTG; opener < hh.BB; opener++ {
		if seen[opener] {
			fmt.Printf(" %14s", "vs "+opener.String())
		}
	}
	fmt.Println()
	for pos := hh.UTG + 1; pos <= hh.BB; pos++ {
		if !seen[pos] {
			continue
		}
		fmt.Printf("%-8s", pos)
		for opener := hh.UTG; opener < hh.BB; opener++ {
			if !seen[opener] {
				continue
			}
			r := counts[pair{pos, opener}]
			switch {
			case opener >= pos:
				fmt.Printf(" %14s", "")
			case r == nil:
				fmt.Printf(" %14s", "-")
			default:
				fmt.Printf(" %14s", fmt.Sprintf("%.1f (%d)", 100*float64(r.raised)/float64(r.faced), r.faced))
			}
		}
		fmt.Println()
	}
	fmt.Fprintf(os.Stderr, "%.1fs\n", time.Since(start).Seconds())
}
