// Command hhcheck parses every .txt hand history under the given files,
// directories or .zip archives (see hh.WalkText) and reports hand counts, parse
// errors and unrecognised lines.
//
//	go run ./hh/cmd/hhcheck [-j N] [-v] path...
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/seanq222/hh"
)

var (
	digits = regexp.MustCompile(`\d+`)
	names  = regexp.MustCompile(`^\S+ `)
)

type stats struct {
	files, hands, errors, dropped, repaired, bombPots int
	unknown, reasons                                  map[string]int
	errSamples                                        []string
}

func main() {
	workers := flag.Int("j", runtime.NumCPU(), "parallel workers")
	verbose := flag.Bool("v", false, "print every parse error")
	flag.Parse()
	if flag.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "usage: hhcheck [-j N] [-v] path...")
		os.Exit(2)
	}

	start := time.Now()
	results := make(chan *stats)
	go func() {
		err := hh.WalkText(flag.Args(), *workers, func(p string, b []byte) { results <- scan(p, b) })
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
		close(results)
	}()

	total := &stats{unknown: map[string]int{}, reasons: map[string]int{}}
	for s := range results {
		total.files += s.files
		total.hands += s.hands
		total.errors += s.errors
		total.dropped += s.dropped
		total.repaired += s.repaired
		total.bombPots += s.bombPots
		for k, v := range s.unknown {
			total.unknown[k] += v
		}
		for k, v := range s.reasons {
			total.reasons[k] += v
		}
		for _, e := range s.errSamples {
			if *verbose || len(total.errSamples) < 10 {
				total.errSamples = append(total.errSamples, e)
			}
		}
	}

	el := time.Since(start)
	fmt.Printf("files: %d  hands: %d  bomb pots: %d  parse errors: %d  (%.1fs, %.0f hands/s)\n",
		total.files, total.hands, total.bombPots, total.errors, el.Seconds(), float64(total.hands)/el.Seconds())
	read := total.hands + total.dropped + total.errors
	fmt.Printf("dropped, a missing action: %d of %d hands read (%.2f%%)\n",
		total.dropped, read, 100*float64(total.dropped)/float64(max(read, 1)))
	var rs []string
	for k := range total.reasons {
		rs = append(rs, k)
	}
	sort.Slice(rs, func(i, j int) bool { return total.reasons[rs[i]] > total.reasons[rs[j]] })
	for _, k := range rs {
		fmt.Printf("  %8d  %s\n", total.reasons[k], strings.TrimPrefix(k, "dropped: "))
	}
	fmt.Printf("missing actions reconstructed: %d\n", total.repaired)
	for _, e := range total.errSamples {
		fmt.Println("  error:", e)
	}
	if len(total.unknown) > 0 {
		type kv struct {
			k string
			v int
		}
		var l []kv
		for k, v := range total.unknown {
			l = append(l, kv{k, v})
		}
		sort.Slice(l, func(i, j int) bool { return l[i].v > l[j].v })
		fmt.Println("unrecognised line patterns (digits replaced by N):")
		for i, e := range l {
			if i == 15 {
				break
			}
			fmt.Printf("  %8d  %s\n", e.v, e.k)
		}
	}
}

func scan(path string, data []byte) *stats {
	s := &stats{files: 1, unknown: map[string]int{}, reasons: map[string]int{}}
	st, err := hh.ParseText(data, func(h *hh.HandHistory) error {
		s.hands++
		if h.Type == hh.Bomb {
			s.bombPots++
		}
		for _, u := range h.Unknown {
			s.unknown[digits.ReplaceAllString(names.ReplaceAllString(u, ""), "N")]++
		}
		return nil
	}, func(err error) {
		var pe *hh.ParseError
		if errors.As(err, &pe) && pe.Defective {
			s.reasons[pe.Msg]++
			return
		}
		s.errors++
		if len(s.errSamples) < 5 {
			s.errSamples = append(s.errSamples, filepath.Base(path)+": "+err.Error())
		}
	})
	s.dropped += st.Dropped
	s.repaired += st.Repaired
	if err != nil {
		s.errors++
		s.errSamples = append(s.errSamples, path+": "+err.Error())
	}
	return s
}
