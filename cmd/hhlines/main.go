// Command hhlines reads the pack files in a folder and prints each player's red,
// blue and green lines, the way Hand2Note draws them, as win rates in big blinds
// per 100 hands: green is the whole win rate, blue what was won in hands that went
// to showdown (see PlayerHand.Showdown), red what was won in the others. All three
// are over all the player's hands, so red and blue add up to green. They are EV
// win rates: an all-in with cards to come counts at the player's equity (see
// PlayerHand.WonEV). Players are sorted by hands, most first.
//
//	go run ./cmd/hhlines [-plain] [-min 100000] [-n 50] DIR
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

type row struct {
	name      string
	hands     int
	showdowns int
	blue, red float64 // EV net in big blinds, in hands with and without a showdown
}

func (r row) per100(bb float64) float64 { return 100 * bb / float64(r.hands) }

func main() {
	plain := flag.Bool("plain", false, "count only hands without antes and bomb pots")
	n := flag.Int("n", 0, "print only this many players (0 for all)")
	minHands := flag.Int("min", 0, "leave out players with fewer hands than this")
	flag.Parse()
	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: hhlines [-plain] [-min N] [-n N] DIR")
		os.Exit(2)
	}
	start := time.Now()
	// The blocks of the packs are read on every CPU; each block's totals are added
	// to its player's under the lock.
	var mu sync.Mutex
	byPlayer := map[string]*row{}
	total := 0
	err := hh.ScanPacks(flag.Arg(0), func(player string, hands []hh.PlayerHand) {
		var part row
		for i := range hands {
			h := &hands[i]
			if h.BigBlind == 0 || (*plain && h.Type != hh.Regular) {
				continue
			}
			part.hands++
			bb := float64(h.NetEV()) / float64(h.BigBlind)
			if h.Showdown() {
				part.showdowns++
				part.blue += bb
			} else {
				part.red += bb
			}
		}
		mu.Lock()
		r := byPlayer[player]
		if r == nil {
			r = &row{name: player}
			byPlayer[player] = r
		}
		r.hands += part.hands
		r.showdowns += part.showdowns
		r.blue += part.blue
		r.red += part.red
		total += part.hands
		mu.Unlock()
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}

	var rows []row
	for _, r := range byPlayer {
		if r.hands > 0 && r.hands >= *minHands {
			rows = append(rows, *r)
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].hands > rows[j].hands })
	if *n > 0 && *n < len(rows) {
		rows = rows[:*n]
	}
	fmt.Printf("%-28s %10s %10s %10s %10s %10s\n", "player", "hands", "showdown %", "green", "blue", "red")
	for _, r := range rows {
		fmt.Printf("%-28s %10d %10.1f %10.2f %10.2f %10.2f\n", r.name, r.hands, 100*float64(r.showdowns)/float64(r.hands),
			r.per100(r.blue+r.red), r.per100(r.blue), r.per100(r.red))
	}
	fmt.Fprintf(os.Stderr, "%d player-hands of %d players in %.1fs\n", total, len(byPlayer), time.Since(start).Seconds())
}
