// Command hhrake reads the pack files in a folder and prints the rake at each
// stake (site and big blind): the rake and jackpot fee the winners paid, per 100
// hands a player plays, in big blinds, beside the players' EV win rate. A hand
// counts once however many views it has, so the packs must hold every player's
// view of their hands, as hhpack writes them with -anon.
//
// With -players there is a row for each player instead: the rake they paid per
// 100 of their hands, their EV win rate, and the win rate they would have had
// with no rake. With -stakes as well, a row for each player at each stake.
//
//	go run ./cmd/hhrake [-players [-stakes] [-min N] [-n N]] DIR
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

type stake struct {
	site     hh.Site
	bigBlind hh.Cents
}

func (s stake) String() string { return fmt.Sprintf("%s NL%d", s.site, s.bigBlind) }

// key is what a row totals: a stake, a player, or a player at a stake.
type key struct {
	player string
	stake  stake
}

type row struct {
	views  int     // player-hands
	hands  float64 // hands, each view counting 1/players of one
	rake   hh.Cents
	rakeBB float64 // rake in big blinds
	evBB   float64 // EV net, in big blinds
}

func (r *row) add(o *row) {
	r.views += o.views
	r.hands += o.hands
	r.rake += o.rake
	r.rakeBB += o.rakeBB
	r.evBB += o.evBB
}

func main() {
	players := flag.Bool("players", false, "a row for each player: the rake they paid per 100 of their hands, their EV win rate and their win rate before rake")
	stakes := flag.Bool("stakes", false, "with -players, a row for each player at each stake")
	minHands := flag.Int("min", 0, "with -players, leave out rows with fewer hands than this")
	n := flag.Int("n", 0, "with -players, print only this many rows (0 for all)")
	flag.Parse()
	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: hhrake [-players [-stakes] [-min N] [-n N]] DIR")
		os.Exit(2)
	}
	start := time.Now()
	var mu sync.Mutex
	rows := map[key]*row{}
	err := hh.ScanPacks(flag.Arg(0), func(player string, hands []hh.PlayerHand) {
		local := map[key]*row{}
		for i := range hands {
			h := &hands[i]
			if h.BigBlind == 0 {
				continue
			}
			k := key{stake: stake{h.Site, h.BigBlind}}
			if *players {
				k.player = player
				if !*stakes {
					k.stake = stake{}
				}
			}
			r := local[k]
			if r == nil {
				r = &row{}
				local[k] = r
			}
			paid := h.RakePaid()
			r.views++
			r.hands += 1 / float64(len(h.Players))
			r.rake += paid
			r.rakeBB += float64(paid) / float64(h.BigBlind)
			r.evBB += float64(h.NetEV()) / float64(h.BigBlind)
		}
		mu.Lock()
		for k, r := range local {
			if rows[k] == nil {
				rows[k] = &row{}
			}
			rows[k].add(r)
		}
		mu.Unlock()
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	var keys []key
	for k, r := range rows {
		if !*players || r.views >= *minHands {
			keys = append(keys, k)
		}
	}
	if *players {
		sort.Slice(keys, func(i, j int) bool { return rows[keys[i]].views > rows[keys[j]].views })
		if *n > 0 && *n < len(keys) {
			keys = keys[:*n]
		}
		fmt.Printf("%-40s %10s %14s %14s %10s %16s\n", "player", "hands", "rake paid", "rake bb/100", "EV bb/100", "before rake bb/100")
		for _, k := range keys {
			r := rows[k]
			name := k.player
			if *stakes {
				name += " @ " + k.stake.String()
			}
			per := func(bb float64) float64 { return 100 * bb / float64(r.views) }
			fmt.Printf("%-40s %10d %14.2f %14.2f %10.2f %16.2f\n", name, r.views, float64(r.rake)/100,
				per(r.rakeBB), per(r.evBB), per(r.evBB+r.rakeBB))
		}
	} else {
		sort.Slice(keys, func(i, j int) bool { return rows[keys[i]].hands > rows[keys[j]].hands })
		fmt.Printf("%-16s %12s %8s %14s %14s %10s\n", "stake", "hands", "players", "rake", "rake bb/100", "EV bb/100")
		for _, k := range keys {
			r := rows[k]
			fmt.Printf("%-16s %12.0f %8.2f %14.2f %14.2f %10.2f\n", k.stake, r.hands, float64(r.views)/r.hands,
				float64(r.rake)/100, 100*r.rakeBB/float64(r.views), 100*r.evBB/float64(r.views))
		}
	}
	fmt.Fprintf(os.Stderr, "%.1fs\n", time.Since(start).Seconds())
}
