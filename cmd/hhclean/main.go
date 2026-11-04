// Command hhclean copies the good hands from a folder of hand history files to a
// new folder, byte for byte, and leaves out the hands that have an error of any
// kind. By default nothing is repaired or rewritten: a hand is either copied
// exactly as it is in the source or not at all.
//
//	go run ./cmd/hhclean -out DIR SOURCE...
//	go run ./cmd/hhclean -regenerate [-repair] [-anonymize] -out DIR SOURCE...
//
// With -regenerate each good hand is written again from its parsed form, read back,
// and must come back identical. With -repair as well, a hand that fails a check is
// repaired first (missing actions and posts put back, a hand cut off after the last
// fold completed, stacks and pot lines corrected) and kept if it then passes every
// check. The order is: parse, check, repair only if a check failed, check again,
// write, parse the text, compare.
//
// SOURCE is a folder, a .txt file or a .zip archive;
// archives, and archives inside them, are read without unpacking them (see
// hh.WalkText). Each source file becomes a file at the same relative path under
// DIR holding its good hands, in their original order, an archive becoming a
// folder of its name without ".zip". A file with no good hands is not written.
// DIR/dropped.tsv lists every dropped hand with the reason and the source file.
//
// A hand has an error if it
//   - cannot be parsed, or is cut off before its summary;
//   - fails any check of the package: a player's turn is skipped, a blind or ante
//     post is missing or posted by the wrong seat, a dead blind lost its amount,
//     a betting round never closes, the returned bet does not match the actions,
//     or the pot does not match the actions;
//   - has a player put in more than the stack on the seat line, which means an
//     automatic top-up the file did not record;
//   - lists a seat that is not marked sitting out, never acts and is not in the
//     summary.
//
// With -only, only files whose name contains the text are read. With -player,
// only the hands that player is dealt into are kept (with -anonymize, before the
// names are replaced); the other hands are neither written nor counted.
//
// A good hand that appears in more than one file is written once, from whichever
// file is read first; the copies are counted, not listed. With several SOURCEs they
// are read one after another in the order given, so a hand in the first is
// written from it; each gets a folder under DIR named after it (without ".zip").
package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/seanq222/hh"
	"github.com/seanq222/hh/internal/clean"
)

func main() {
	out := flag.String("out", "", "folder to write the good hands to")
	only := flag.String("only", "", "read only files whose name contains this text")
	regen = flag.Bool("regenerate", false, "write each good hand again from the parsed data instead of copying its text")
	anon = flag.Bool("anonymize", false, "with -regenerate, give the players in each hand random names Player00 to Player99; the real names are in DIR/mapping.tsv")
	repair = flag.Bool("repair", false, "with -regenerate, repair a hand that fails a check instead of dropping it: put back missing actions and posts, complete a hand cut off after the last fold, and correct stacks, pot lines and the like; the repaired hand is written, read back and must be identical")
	player := flag.String("player", "", "keep only the hands this player is dealt into")
	flag.Parse()
	if *repair && !*regen {
		// Repairs change the hand, so the text must be written again from it; copying
		// the original text would write the faulty hand.
		fmt.Fprintln(os.Stderr, "-repair needs -regenerate")
		os.Exit(2)
	}
	if *out == "" || flag.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "usage: hhclean -out DIR SOURCE...")
		os.Exit(2)
	}
	if st, err := os.Stat(*out); err == nil && st.IsDir() {
		if entries, _ := os.ReadDir(*out); len(entries) > 0 {
			fmt.Fprintln(os.Stderr, "the output folder is not empty:", *out)
			os.Exit(1)
		}
	}
	if err := os.MkdirAll(*out, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	dropLog, err := os.Create(filepath.Join(*out, "dropped.tsv"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	dropWriter := bufio.NewWriterSize(dropLog, 1<<20)
	if *anon {
		mf, err := os.Create(filepath.Join(*out, "mapping.tsv"))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		mapWriter = bufio.NewWriterSize(mf, 1<<20)
	}
	var logMu sync.Mutex

	var mu sync.Mutex
	var hands, kept, duplicates, filesRead int
	seen := clean.NewSeen()
	var filesWritten int
	var bytesWritten int64
	reasons := map[string]int{}

	start := time.Now()
	for _, src := range flag.Args() {
		src = filepath.Clean(src)
		// With several sources, each gets a folder of its own, named after it.
		prefix := ""
		if flag.NArg() > 1 {
			prefix = strings.TrimSuffix(filepath.Base(src), ".zip")
		}
		err = hh.WalkText([]string{src}, runtime.NumCPU(), func(p string, b []byte) {
			if !strings.Contains(filepath.Base(p), *only) {
				return
			}
			rel := filepath.Join(prefix, hh.RelText(src, p))
			var good, maps bytes.Buffer
			var h, k, dups int
			local := map[string]int{}
			var dropped []string
			for _, block := range clean.Split(b) {
				if *player != "" && !bytes.Contains(block, []byte(*player)) {
					continue
				}
				h++
				hand, id, mapping, reason := examine(block, *repair, *anon)
				if reason == "" && *player != "" && !dealtIn(hand, *player) {
					h--
					continue
				}
				if reason != "" {
					local[reason]++
					dropped = append(dropped, fmt.Sprintf("%s\t%s\t%s\n", id, reason, rel))
					continue
				}
				// A good hand that another file already gave is not written again.
				if !seen.Add(hand.Site, hand.ID) {
					dups++
					continue
				}
				if *regen {
					text, problem := regenerate(hand)
					if problem != "" {
						// Not written: the hand is not consistent.
						reason := "regenerated: " + problem
						local[reason]++
						dropped = append(dropped, fmt.Sprintf("%s\t%s\t%s\n", id, reason, rel))
						continue
					}
					k++
					good.WriteString(text)
					maps.WriteString(mapping)
				} else {
					k++
					good.Write(block)
				}
			}
			if good.Len() > 0 {
				dst := filepath.Join(*out, rel)
				if err := os.MkdirAll(filepath.Dir(dst), 0o755); err == nil {
					if err := os.WriteFile(dst, good.Bytes(), 0o644); err != nil {
						fmt.Fprintln(os.Stderr, err)
					}
				}
			}
			if len(dropped) > 0 {
				logMu.Lock()
				for _, line := range dropped {
					dropWriter.WriteString(line)
				}
				logMu.Unlock()
			}
			mu.Lock()
			filesRead++
			hands += h
			duplicates += dups
			if mapWriter != nil {
				mapWriter.Write(maps.Bytes())
			}
			kept += k
			if good.Len() > 0 {
				filesWritten++
				bytesWritten += int64(good.Len())
			}
			for r, n := range local {
				reasons[r] += n
			}
			mu.Unlock()
		})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}
	dropWriter.Flush()
	if mapWriter != nil {
		mapWriter.Flush()
	}
	dropLog.Close()

	fmt.Printf("%d files read in %.0fs, %d hands: %d good hands copied to %d files (%.1f GB), %d copies of a good hand already written left out, %d dropped (%.2f%%)\n",
		filesRead, time.Since(start).Seconds(), hands, kept, filesWritten, float64(bytesWritten)/(1<<30), duplicates,
		hands-kept-duplicates, 100*float64(hands-kept-duplicates)/float64(max(hands, 1)))
	var keys []string
	for k := range reasons {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return reasons[keys[i]] > reasons[keys[j]] })
	for _, k := range keys {
		fmt.Printf("  %9d  %s\n", reasons[k], k)
	}
}

var (
	regen     *bool
	anon      *bool
	repair    *bool
	mapWriter *bufio.Writer
)

// regenerate writes a hand that has passed the checks again from its structure,
// followed by a blank line, parses that text, and requires the new structure to be
// identical to the first (see HandHistory.Diff). problem says the first field that
// differs, or why the new text cannot be parsed. The checks are not run again on
// the text: an identical structure would give the same answer.
func regenerate(h *hh.HandHistory) (text, problem string) {
	text = h.String() + "\n"
	var again []string
	for _, l := range strings.Split(text, "\n") {
		if l = strings.TrimRight(l, " \t\r"); l != "" {
			again = append(again, l)
		}
	}
	g, err := hh.ParseHand(again)
	if err != nil {
		return text, "the new text cannot be parsed: " + err.Error()
	}
	if d := h.Diff(g); d != "" {
		return text, "the new text reads back differently, " + d
	}
	return text, ""
}

// anonymize gives the players in the hand random names, Player00 to Player99, all
// different within the hand. The choice depends only on the hand ID, so it can be
// repeated. It returns one line per name, "ID<TAB>new name<TAB>real name".
func anonymize(h *hh.HandHistory) string {
	pool := rand.New(rand.NewSource(h.ID)).Perm(100)
	next := 0
	names := map[string]string{}
	var order []string
	name := func(real string) string {
		if n, ok := names[real]; ok {
			return n
		}
		n := fmt.Sprintf("Player%02d", pool[next])
		next++
		names[real] = n
		order = append(order, real)
		return n
	}
	clean.RenameAll(h, name)
	var b strings.Builder
	for _, real := range order {
		fmt.Fprintf(&b, "%d\t%s\t%s\n", h.ID, names[real], real)
	}
	return b.String()
}

// examine parses the hand and returns its structure with the hand ID and "" if the
// hand is good, or the reason it is not. The order is that of package clean: check,
// and with repair, fix a hand that failed and check it again.
//
// With anon, the players are given other names as soon as the hand is parsed, so
// that every check sees the names that will be written. mapping lists the new
// names, see anonymize.
func examine(block []byte, repair, anon bool) (h *hh.HandHistory, id, mapping, reason string) {
	id = handID(block)
	h, reason = clean.Parse(block)
	if reason != "" {
		return nil, id, "", reason
	}
	if anon {
		mapping = anonymize(h)
	}
	reason = clean.Check(h)
	if repair && reason != "" {
		clean.Fix(h)
		reason = clean.Check(h)
	}
	return h, id, mapping, reason
}

// handID is the number after "Hand #" on the hand's first line, which starts
// "Hand #" for WPN and "CoinPoker Hand #" for CoinPoker.
func handID(hand []byte) string {
	rest := hand
	if i := bytes.Index(hand, []byte("Hand #")); i >= 0 {
		rest = hand[i+len("Hand #"):]
	}
	n := 0
	for n < len(rest) && rest[n] >= '0' && rest[n] <= '9' {
		n++
	}
	return string(rest[:n])
}

// dealtIn reports whether the player has a seat in the hand and is not sitting
// out.
func dealtIn(h *hh.HandHistory, player string) bool {
	for i := range h.Players {
		if h.Players[i].Name == player {
			return !h.Players[i].SittingOut
		}
	}
	return false
}
