// Command hhextract copies, byte for byte, every hand in which a named player
// sits at the table into one file per player, so that a small file can be
// iterated on instead of the whole collection. A hand whose ID was already
// copied is skipped, unless -dups is given.
//
//	go run ./cmd/hhextract [-dups] -out DIR -names PLAYER1,PLAYER2 path...
//
// The paths are directories, files or .zip archives (see hh.WalkText); their .txt
// files are read in path order.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/seanq222/hh"
)

var anteNoBomb, keepDups *bool

type block struct {
	id   int64
	data []byte
}

func main() {
	out := flag.String("out", ".", "directory to write the files to")
	namesFlag := flag.String("names", "", "comma-separated player names")
	anteNoBomb = flag.Bool("antenobomb", false, "keep only hands that have antes but no BombPot label")
	keepDups = flag.Bool("dups", false, "keep every copy of a hand instead of skipping the ones already copied")
	flag.Parse()
	names := strings.Split(*namesFlag, ",")
	if *namesFlag == "" || flag.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "usage: hhextract -out DIR -names a,b path...")
		os.Exit(2)
	}
	if err := os.MkdirAll(*out, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	// Read the files in parallel, keep the results in path order.
	var mu sync.Mutex
	byPath := map[string][][]block{} // file -> [name] -> blocks
	err := hh.WalkText(flag.Args(), runtime.NumCPU(), func(p string, b []byte) {
		r := split(b, names)
		mu.Lock()
		byPath[p] = r
		mu.Unlock()
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	var paths []string
	for p := range byPath {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	results := make([][][]block, len(paths))
	for i, p := range paths {
		results[i] = byPath[p]
	}

	seen := make([]map[int64]bool, len(names))
	files := make([]*os.File, len(names))
	counts := make([]int, len(names))
	sizes := make([]int64, len(names))
	for n, name := range names {
		seen[n] = map[int64]bool{}
		f, err := os.Create(filepath.Join(*out, name+".txt"))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		files[n] = f
	}
	dups := 0
	for _, perName := range results {
		for n, blocks := range perName {
			for _, b := range blocks {
				if seen[n][b.id] {
					dups++
					if !*keepDups {
						continue
					}
				}
				seen[n][b.id] = true
				if _, err := files[n].Write(b.data); err != nil {
					fmt.Fprintln(os.Stderr, err)
					os.Exit(1)
				}
				counts[n]++
				sizes[n] += int64(len(b.data))
			}
		}
	}
	for n, name := range names {
		files[n].Close()
		fmt.Printf("%-16s %8d hands  %7.1f MB  %s\n", name, counts[n], float64(sizes[n])/(1<<20), filepath.Join(*out, name+".txt"))
	}
	if *keepDups {
		fmt.Printf("%d files read, %d of the hands are repeats and were kept\n", len(paths), dups)
	} else {
		fmt.Printf("%d files read, %d repeated hands skipped\n", len(paths), dups)
	}
}

// split cuts a file into hands, each exactly as it is in the file, and returns
// for each name the hands in which that player is listed at the table.
func split(b []byte, names []string) [][]block {
	res := make([][]block, len(names))
	var starts []int
	for pos := 0; pos < len(b); {
		if bytes.HasPrefix(b[pos:], []byte("Hand #")) {
			starts = append(starts, pos)
		}
		nl := bytes.IndexByte(b[pos:], '\n')
		if nl < 0 {
			break
		}
		pos += nl + 1
	}
	for i, s := range starts {
		end := len(b)
		if i+1 < len(starts) {
			end = starts[i+1]
		}
		hand := b[s:end]
		if *anteNoBomb && !anteHandWithoutBombPot(hand) {
			continue
		}
		id := handID(hand)
		for n, name := range names {
			if seated(hand, name) {
				data := hand
				if !bytes.HasSuffix(data, []byte("\n")) {
					data = append(append([]byte(nil), data...), '\n')
				}
				res[n] = append(res[n], block{id: id, data: data})
			}
		}
	}
	return res
}

func handID(hand []byte) int64 {
	rest := hand[len("Hand #"):]
	end := 0
	for end < len(rest) && rest[end] >= '0' && rest[end] <= '9' {
		end++
	}
	id, _ := strconv.ParseInt(string(rest[:end]), 10, 64)
	return id
}

// seated reports whether a seat line "Seat N: NAME ($..." names the player.
func seated(hand []byte, name string) bool {
	needle := []byte(": " + name + " ($")
	for from := 0; ; {
		i := bytes.Index(hand[from:], needle)
		if i < 0 {
			return false
		}
		i += from
		ls := bytes.LastIndexByte(hand[:i], '\n') + 1
		if bytes.HasPrefix(hand[ls:], []byte("Seat ")) {
			return true
		}
		from = i + len(needle)
	}
}

// anteHandWithoutBombPot reports whether a hand has antes and no BombPot label.
func anteHandWithoutBombPot(hand []byte) bool {
	var lines []string
	for _, l := range strings.Split(string(hand), "\n") {
		if l = strings.TrimRight(l, " \t\r"); l != "" {
			lines = append(lines, l)
		}
	}
	h, err := hh.ParseHand(lines)
	return err == nil && h.Type == hh.Ante
}
