// Command hhstats reads the pack files in a folder and writes each player's
// statistics in the layout of Hand2Note's player page: preflop by position,
// steals, 3- and 4-bets, c-bets, aggression and showdown. It writes into OUT
//
//   - NAME.json for each player with their own pack (not the anonymous ones),
//   - _all.json for every hand in the packs, the anonymous ones included,
//   - index.html, a page with every player's stats and a search box.
//
// The page shows a value in blue when it is below its normal range and in red
// when above (see ranges.go); low and high mean less and more than usual, not
// worse or better. Values from fewer than 50 chances are dimmed, with the
// count beside them, and not coloured. Above a player's tables, a few lines
// sum them up: their style, and the strengths and weaknesses that stand out
// most against the normal ranges, each with how to play against it.
//
//	go run ./cmd/hhstats -out DIR PACKS
//
// Each JSON file holds the player's hands, net and EV result in big blinds,
// every stat's [times done, chances] by name, the actions after the flop by
// street ([fold, check, call, bet, raise]) and, by street, the hands with an
// action and those with a bet or raise. The definitions follow Hand2Note's
// default stats; tally.go says what each one counts.
package main

import (
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/seanq222/hh"
)

//go:embed page.html
var page string

func newStats() *stats { return &stats{c: make([]stat, len(statNames))} }

// counts is a player's stats as written to JSON and into the page.
type counts struct {
	Player  string              `json:"player"`
	Hands   int64               `json:"hands"`
	Net     float64             `json:"net"`
	EV      float64             `json:"ev"`
	Stats   map[string][2]int64 `json:"stats"`
	Aggr    [3][5]int64         `json:"aggr"`
	Streets [3][2]int64         `json:"streets"`
}

func (st *stats) counts(player string) counts {
	c := counts{Player: player, Hands: st.hands, Net: st.net, EV: st.ev,
		Stats: map[string][2]int64{}, Aggr: st.aggr, Streets: st.streets}
	for name, s := range st.byName() {
		c.Stats[name] = [2]int64{s.n, s.d}
	}
	return c
}

// fileName makes a player's name safe to use as a file name.
func fileName(player string) string {
	name := strings.Map(func(r rune) rune {
		if strings.ContainsRune(`/\:*?"<>|`, r) || r < ' ' {
			return '_'
		}
		return r
	}, player)
	if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
		name = "-" + name // keep clear of hidden files and _all.json
	}
	return name
}

func writeFile(path string, write func(w io.Writer) error) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := write(f); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func writeJSON(path string, v any) error {
	return writeFile(path, func(w io.Writer) error { return json.NewEncoder(w).Encode(v) })
}

func main() {
	out := flag.String("out", "", "the folder to write the JSON files and index.html into")
	flag.Parse()
	if flag.NArg() != 1 || *out == "" {
		fmt.Fprintln(os.Stderr, "usage: hhstats -out DIR PACKS")
		os.Exit(2)
	}
	start := time.Now()

	// Each block of hands is counted on its own, then added to its player's
	// counts and to the total.
	all := newStats()
	players := map[string]*stats{}
	var mu sync.Mutex
	err := hh.ScanPackInfos(flag.Arg(0), func(info hh.PackInfo, hands []hh.PlayerHand) {
		w := newStats()
		for i := range hands {
			tally(&hands[i], w)
		}
		mu.Lock()
		defer mu.Unlock()
		all.merge(w)
		if !info.Anon {
			if players[info.Player] == nil {
				players[info.Player] = newStats()
			}
			players[info.Player].merge(w)
		}
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if err := os.MkdirAll(*out, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	byName := map[string]counts{}
	for name, st := range players {
		byName[name] = st.counts(name)
		if err := writeJSON(filepath.Join(*out, fileName(name)+".json"), byName[name]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	total := all.counts("All players")
	if err := writeJSON(filepath.Join(*out, "_all.json"), total); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	// The page holds everything, so it opens from the folder with no server.
	data, err := json.Marshal(map[string]any{"players": byName, "all": total, "normal": normal})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	// Keep the data from closing the page's script element.
	js := strings.ReplaceAll(string(data), "</", `<\/`)
	html := strings.Replace(page, "/*DATA*/null", js, 1)
	if err := os.WriteFile(filepath.Join(*out, "index.html"), []byte(html), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("%d players, %d hands, in %.1f s; open %s\n",
		len(players), all.hands, time.Since(start).Seconds(), filepath.Join(*out, "index.html"))
}
