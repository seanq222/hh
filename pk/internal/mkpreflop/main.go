// Command mkpreflop writes the table behind pk.PreflopEquity: the result of
// pk.Equity for every heads-up matchup that differs from the others by more than
// a renaming of the suits or the order of the hands. It counts all 1,712,304
// boards for each of the 47,008, on every CPU, which takes a few minutes.
//
//	go run ./internal/mkpreflop -out preflop.bin
package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"slices"
	"sync"
	"time"

	"github.com/seanq222/hh/pk"
)

func main() {
	out := flag.String("out", "preflop.bin", "file to write the table to")
	flag.Parse()

	// Every matchup's key, once.
	seen := map[uint32]bool{}
	for a0 := pk.Card(0); a0 < 52; a0++ {
		for a1 := a0 + 1; a1 < 52; a1++ {
			for b0 := pk.Card(0); b0 < 52; b0++ {
				for b1 := b0 + 1; b1 < 52; b1++ {
					if b0 == a0 || b0 == a1 || b1 == a0 || b1 == a1 {
						continue
					}
					key, _ := pk.PreflopKey(a0, a1, b0, b1)
					seen[key] = true
				}
			}
		}
	}
	keys := make([]uint32, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	fmt.Printf("%d matchups\n", len(keys))

	// Count the boards for each, in its standard form, so that the counts are for
	// the key's first hand.
	type result struct{ wins, draws int }
	results := make([]result, len(keys))
	start := time.Now()
	var done sync.WaitGroup
	next := make(chan int)
	for range runtime.NumCPU() {
		done.Add(1)
		go func() {
			defer done.Done()
			for i := range next {
				a0, a1, b0, b1 := pk.PreflopKeyCards(keys[i])
				w, d, _ := pk.Equity(a0, a1, b0, b1)
				results[i] = result{w, d}
			}
		}()
	}
	for i := range keys {
		if i > 0 && i%5000 == 0 {
			fmt.Printf("%d done, %.0fs\n", i, time.Since(start).Seconds())
		}
		next <- i
	}
	close(next)
	done.Wait()

	b := make([]byte, 0, 9*len(keys))
	put := func(v uint32) { b = append(b, byte(v>>16), byte(v>>8), byte(v)) }
	for i, k := range keys {
		put(k)
		put(uint32(results[i].wins))
		put(uint32(results[i].draws))
	}
	if err := os.WriteFile(*out, b, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("wrote %s, %d bytes, in %.0fs\n", *out, len(b), time.Since(start).Seconds())
}
