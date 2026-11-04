// Command hhflows reads the pack files in a folder and works out, at each stake,
// how much each group of players wins from each other group before rake, and
// how much rake each group pays on pots won from each.
//
// The groups are the players with a pack of their own, by their EV win rate at
// that stake: winners (above -win, 0 by default), losers (at or below -lose, -4
// by default) and the almost winners between; and the players in the anonymous
// packs, the anon group.
//
// It reads the hand table that hhpack writes beside the packs (hands.hht, see
// hh.HandTable), which has every player of every hand, and the packs' headers
// for which packs are anonymous. In each hand, each player's result before rake
// is their EV net plus their share of the rake in EV: the hand's rake divided by
// what each player was expected to win. Each player who lost is taken to have
// paid the players who won, in proportion to what each won, and each winner's
// rake is shared out the same way. Money that stays inside a group cancels out
// of the group's edge but its rake is still paid.
//
// For each group and each other group it prints the net edge, in big blinds per
// 100 hands: per 100 of the group's hands, which add up over the other groups to
// the group's whole transfer; and, as the edge, per 100 of the group's hands
// counted by the other group's share of the opponents. A hand in which a player
// has three opponents, one of them a winner, counts as a third of a hand against
// the winners, so that the edge is what the group would win if all its opponents
// were of the other group, whatever the size of the table.
//
// Grouping players by their results and then measuring the same hands favours
// the winners: a player who ran well is more likely to be among them, and brings
// the good run along. With -holdout the hands are split in two halves by a hash
// of the hand key, so that a hand is in the same half for everyone at the table.
// Each hand is then measured with the groups that the players' hands in the
// other half give them, so that no hand measured had a say in anyone's group.
//
// With -bysite the stakes of each site are taken together: players are grouped
// by their EV win rate in big blinds over all the site's stakes.
//
//	go run ./cmd/hhflows [-win 0] [-lose -4] [-holdout] [-bysite] DIR
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"sort"
	"sync"
	"time"

	"github.com/seanq222/hh"
)

// The groups, in the order they are printed.
const (
	winners = iota
	almost
	losers
	anon
	nGroups
)

var groupNames = [nGroups]string{"winners", "almost winners", "losers", "anon"}

type stake struct {
	site     hh.Site
	bigBlind hh.Cents
}

// String names the stake, or the site alone for all its stakes (bigBlind 0).
func (s stake) String() string {
	if s.bigBlind == 0 {
		return s.site.String()
	}
	return fmt.Sprintf("%s NL%d", s.site, s.bigBlind)
}

type playerStake struct {
	id   uint16
	s    stake
	half int8 // the half of the hands the group comes from; 0 without -holdout
}

// half returns the half, 0 or 1, that a hand is in with -holdout: a bit of a
// SplitMix64 hash of its key, salted so as not to follow the anonymous packs'
// draw from the same key.
func half(key uint64) int8 {
	z := key ^ 0x5851f42d4c957f2d
	z += 0x9e3779b97f4a7c15
	z = (z ^ z>>30) * 0xbf58476d1ce4e5b9
	z = (z ^ z>>27) * 0x94d049bb133111eb
	return int8((z ^ z>>31) >> 63)
}

// tally is a stake's totals, in big blinds.
type tally struct {
	hands [nGroups]int64            // player-hands
	ev    [nGroups]float64          // EV net
	flow  [nGroups][nGroups]float64 // flow[g][h]: what g won from h before rake
	rake  [nGroups][nGroups]float64 // rake[g][h]: rake g paid on what it won from h
	share [nGroups][nGroups]float64 // player-hands of g, each shared among its opponents' groups
}

func (t *tally) add(o *tally) {
	for g := range nGroups {
		t.hands[g] += o.hands[g]
		t.ev[g] += o.ev[g]
		for h := range nGroups {
			t.flow[g][h] += o.flow[g][h]
			t.rake[g][h] += o.rake[g][h]
			t.share[g][h] += o.share[g][h]
		}
	}
}

// seat is one player's part in a hand, from their view, in big blinds.
type seat struct {
	group     int8
	net, rake float32 // net before rake (EV net plus the rake paid), and the rake
}

// split adds the hand's flows to t: each loser pays each winner in proportion to
// what the winner won, and each winner's rake is shared out by where it came
// from in the same way.
func split(seats []seat, t *tally) {
	var won, lost float64
	var present [nGroups]int64
	for i := range seats {
		s := &seats[i]
		g := s.group
		t.hands[g]++
		t.ev[g] += float64(s.net - s.rake)
		present[g]++
		if s.net > 0 {
			won += float64(s.net)
		} else {
			lost -= float64(s.net)
		}
	}
	if opponents := float64(len(seats) - 1); opponents > 0 {
		for g := range nGroups {
			for h := range nGroups {
				others := present[h]
				if g == h {
					others--
				}
				if present[g] > 0 && others > 0 {
					t.share[g][h] += float64(present[g]*others) / opponents
				}
			}
		}
	}
	if won == 0 || lost == 0 {
		return
	}
	for i := range seats {
		w := &seats[i]
		if w.net <= 0 {
			continue
		}
		for j := range seats {
			l := &seats[j]
			if l.net >= 0 {
				continue
			}
			loss := -float64(l.net)
			t.flow[w.group][l.group] += loss * float64(w.net) / won
			t.rake[w.group][l.group] += float64(w.rake) * loss / lost
		}
	}
}

func main() {
	win := flag.Float64("win", 0, "winners have an EV win rate above this, in bb/100")
	lose := flag.Float64("lose", -4, "losers have an EV win rate at or below this, in bb/100")
	holdout := flag.Bool("holdout", false, "group the players on one half of the hands and measure the other")
	bySite := flag.Bool("bysite", false, "take each site's stakes together")
	cpuProfile := flag.String("cpuprofile", "", "write a CPU profile to this file")
	mutexProfile := flag.String("mutexprofile", "", "write a lock-contention profile to this file")
	flag.Parse()
	if *cpuProfile != "" {
		f, err := os.Create(*cpuProfile)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}
	if *mutexProfile != "" {
		runtime.SetMutexProfileFraction(1)
		defer func() {
			if f, err := os.Create(*mutexProfile); err == nil {
				pprof.Lookup("mutex").WriteTo(f, 0)
				f.Close()
			}
		}()
	}
	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: hhflows [-win 0] [-lose -4] [-holdout] [-bysite] DIR")
		os.Exit(2)
	}
	dir := flag.Arg(0)
	start := time.Now()
	ids, err := hh.ReadPackIDs(dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	table, err := hh.ReadHandTable(filepath.Join(dir, "hands.hht"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	read := time.Since(start)
	stakeOf := func(i int) stake {
		if *bySite {
			return stake{site: hh.KeySite(table.Keys[i])}
		}
		return stake{hh.KeySite(table.Keys[i]), table.BigBlind[i]}
	}
	halfOf := func(i int) int8 {
		if *holdout {
			return half(table.Keys[i])
		}
		return 0
	}

	// Pass 1: each player's EV win rate at each stake.
	type rate struct {
		hands int64
		ev    float64
	}
	var mu sync.Mutex
	rates := map[playerStake]*rate{}
	table.Each(func(lo, hi int) {
		local := map[playerStake]*rate{}
		for i := lo; i < hi; i++ {
			st, h := stakeOf(i), halfOf(i)
			bb := float64(table.BigBlind[i])
			a, b := table.Players(i)
			for p := a; p < b; p++ {
				k := playerStake{table.PackID[p], st, h}
				r := local[k]
				if r == nil {
					r = &rate{}
					local[k] = r
				}
				r.hands++
				r.ev += float64(table.NetEV(p)) / bb
			}
		}
		mu.Lock()
		for k, r := range local {
			if rates[k] == nil {
				rates[k] = &rate{}
			}
			rates[k].hands += r.hands
			rates[k].ev += r.ev
		}
		mu.Unlock()
	})
	groupOf := make(map[playerStake]int8, len(rates))
	members := map[stake]*[2][nGroups]int{}
	for k, r := range rates {
		g := int8(almost)
		switch per100 := 100 * r.ev / float64(r.hands); {
		case ids[k.id].Anon:
			g = anon
		case per100 > *win:
			g = winners
		case per100 <= *lose:
			g = losers
		}
		groupOf[k] = g
		if members[k.s] == nil {
			members[k.s] = &[2][nGroups]int{}
		}
		members[k.s][k.half][g]++
	}
	pass1 := time.Since(start) - read

	// Pass 2: split every hand.
	totals := map[stake]*tally{}
	var ungrouped int64 // player-hands of players with no hands in the other half
	table.Each(func(lo, hi int) {
		var none int64
		local := map[stake]*tally{}
		var seats [9]seat
		var shares [9]hh.Cents
		for i := lo; i < hi; i++ {
			st := stakeOf(i)
			bb := float64(table.BigBlind[i])
			from := halfOf(i)
			if *holdout {
				from = 1 - from
			}
			a, b := table.Players(i)
			// Each player's rake in EV: the hand's rake divided by what each was
			// expected to win, so that EV net plus it is the player's EV before rake.
			table.RakeShares(i, true, shares[:])
			for p := a; p < b; p++ {
				rake := float64(shares[p-a]) / bb
				g, ok := groupOf[playerStake{table.PackID[p], st, from}]
				if !ok {
					g = almost
					none++
				}
				seats[p-a] = seat{group: g,
					net: float32(float64(table.NetEV(p))/bb + rake), rake: float32(rake)}
			}
			t := local[st]
			if t == nil {
				t = &tally{}
				local[st] = t
			}
			split(seats[:b-a], t)
		}
		mu.Lock()
		ungrouped += none
		for s, t := range local {
			if totals[s] == nil {
				totals[s] = &tally{}
			}
			totals[s].add(t)
		}
		mu.Unlock()
	})

	var stakes []stake
	for s, t := range totals {
		var n int64
		for _, h := range t.hands {
			n += h
		}
		if n >= 1000000 {
			stakes = append(stakes, s)
		}
	}
	sort.Slice(stakes, func(i, j int) bool {
		a, b := totals[stakes[i]], totals[stakes[j]]
		return a.hands[anon]+a.hands[winners] > b.hands[anon]+b.hands[winners]
	})
	per100f := func(bb, n float64) float64 {
		if n == 0 {
			return 0
		}
		return 100 * bb / n
	}
	per100 := func(bb float64, n int64) float64 { return per100f(bb, float64(n)) }
	for _, s := range stakes {
		t := totals[s]
		fmt.Printf("\n%s\n", s)
		fmt.Printf("%-15s %7s %10s %10s %10s %10s %10s %10s |", "group", "players", "hands", "EV", "transfer", "rake", "rake in", "rake out")
		for h := range nGroups {
			fmt.Printf(" %18s", "vs "+groupNames[h])
		}
		fmt.Println()
		for g := range nGroups {
			n := t.hands[g]
			var transfer, rake float64
			for h := range nGroups {
				if h != g {
					transfer += t.flow[g][h] - t.flow[h][g]
				}
				rake += t.rake[g][h]
			}
			players := fmt.Sprint(members[s][0][g])
			if *holdout {
				players = fmt.Sprintf("%d/%d", members[s][0][g], members[s][1][g])
			}
			if g == anon {
				players = "-"
			}
			fmt.Printf("%-15s %7s %9.1fM %10.2f %10.2f %10.2f %10.2f %10.2f |", groupNames[g], players, float64(n)/1e6,
				per100(t.ev[g], n), per100(transfer, n), per100(rake, n), per100(t.rake[g][g], n), per100(rake-t.rake[g][g], n))
			for h := range nGroups {
				if h == g {
					fmt.Printf(" %18s", "")
					continue
				}
				edge := t.flow[g][h] - t.flow[h][g]
				fmt.Printf(" %8.2f (%7.2f)", per100(edge, n), per100f(edge, t.share[g][h]))
			}
			fmt.Println()
		}
	}
	fmt.Printf("\nbb/100 per 100 of the group's own hands; in brackets, the edge: per 100 of its hands counted by that group's share of the opponents.\n")
	fmt.Printf("rake in: on pots won from the group's own players; rake out: on pots won from other groups.\n")
	if *holdout {
		fmt.Printf("held out: each hand measured with the groups from the other half of the hands; players are those grouped on each half.\n")
		fmt.Printf("player-hands of players with no hands in the other half, counted as almost winners: %d\n", ungrouped)
	}
	fmt.Fprintf(os.Stderr, "%.1fs: read %d hands in %.1fs, grouped in %.1fs, split in %.1fs\n", time.Since(start).Seconds(),
		table.Len(), read.Seconds(), pass1.Seconds(), (time.Since(start) - read - pass1).Seconds())
}
