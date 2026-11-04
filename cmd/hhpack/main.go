// Command hhpack reads every hand history under the given folders and writes
// a pack file for each player who has enough hands, named after the player.
//
//	go run ./cmd/hhpack -out DIR [-min 10000] [-repair] path...
//
// A path can be a folder, a text file or a .zip archive;
// archives, and archives inside them, are read without unpacking them (see
// hh.WalkText). All the paths are read together, every CPU busy until the end.
//
// It reads them twice. The first pass parses and checks every copy of every hand,
// the way hhclean checks them (see package clean) but without repairing, and keeps
// for each hand, by its site and ID, the best copy: one that passes the checks as
// it is over one that does not, then the one that shows more players' hole cards,
// then the one whose text has the lowest hash, so that every run picks the same.
// It also counts each player's hands, once per hand, and compares the copies of a
// hand: it counts the copies whose text differs from the first copy's (carriage
// returns, trailing blanks and blank lines aside), and with -mismatches FILE lists
// them there.
//
// The second pass reads the paths again and uses only each hand's chosen copy.
// A copy that fails a check is left out, or with -repair it is repaired and
// checked again, and left out only if it still fails. Views are built only for
// the players the first pass found to have -min hands, and their blocks go
// straight to their files. Hands that have not filled a block wait in memory,
// already encoded, a few dozen bytes each; -budget limits the memory they take,
// and when it is passed the longest short blocks are written early.
//
// The players with fewer than -min hands are not dropped but go into -anon N
// packs (16 unless set; -anon 0 drops them), AnonPlayer00 to AnonPlayerN-1: in
// each hand every such player is given one of them, a different one for each,
// chosen from the hand's site and ID so that every run chooses the same. An
// anonymous pack is a sample of the pool, not one player. No pack, anonymous or
// not, gets two views of a hand; a hand that would give one is left out and the
// run fails.
//
// With -name A,B only those players get a pack, however many hands they have,
// and the others none unless -anon is given too.
//
// With -pass1 it stops after the first pass, which is useful for timing it.
//
//	go run ./cmd/hhpack -read DIR/PLAYER.hhpk
//
// -read reads a pack file back, times it and totals the player's result.
package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/seanq222/hh"
	"github.com/seanq222/hh/internal/clean"
)

var (
	wanted     map[int64]struct{} // nil: every hand
	nameFilter string
	repair     bool
	failed     atomic.Int64 // hands left out because they fail a check
	twoViews   atomic.Int64 // hands left out because two of their views would go to one pack
)

// examine parses a copy of a hand and checks it the way hhclean does (see package
// clean). h is nil if it does not parse; isClean says whether it passes the checks
// as it is. With fix, a copy that fails is repaired and checked again, and ok says
// whether it passes in the end.
func examine(block []byte, fix bool) (h *hh.HandHistory, isClean, ok bool) {
	h, reason := clean.Parse(block)
	if reason != "" {
		return nil, false, false
	}
	if clean.Check(h) == "" {
		return h, true, true
	}
	if !fix {
		return h, false, false
	}
	clean.Fix(h)
	return h, false, clean.Check(h) == ""
}

// shownCards counts the players whose hole cards the hand shows.
func shownCards(h *hh.HandHistory) int {
	n := 0
	for i := range h.Players {
		if h.Players[i].HasCards {
			n++
		}
	}
	return n
}

// viewNames returns the players who get a view of the hand (see
// HandHistory.PlayerHands): those who post or act, if two to nine of them do.
func viewNames(h *hh.HandHistory) []string {
	var names []string
	seen := map[int]bool{}
	for i := range h.Actions {
		s := h.Actions[i].Seat
		if seen[s] {
			continue
		}
		seen[s] = true
		if p := h.PlayerBySeat(s); p != nil {
			names = append(names, clean.PlayerName(p.Name)) // as the views in pass 2 name them
		}
	}
	if len(names) < 2 || len(names) > 9 {
		return nil
	}
	return names
}

// readIDs reads hand IDs, one per line. Anything after the first run of digits,
// such as a tab and a reason, is ignored.
func readIDs(path string) (map[int64]struct{}, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	ids := map[int64]struct{}{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimPrefix(strings.TrimSpace(sc.Text()), "#")
		n := 0
		for n < len(line) && line[n] >= '0' && line[n] <= '9' {
			n++
		}
		if id, err := strconv.ParseInt(line[:n], 10, 64); err == nil {
			ids[id] = struct{}{}
		}
	}
	return ids, sc.Err()
}

type entry struct {
	name string
	p    hh.PlayerHand
}

func main() {
	out := flag.String("out", "", "directory to write the pack files to")
	minHands := flag.Int("min", 10000, "write a file only for players with at least this many hands")
	budget := flag.Int("budget", 8192, "most memory, in MB, that hands waiting for their block to fill may take before short blocks are written")
	codec := flag.String("codec", "none", "none or flate: uncompressed packs are about three times larger and read about twice as fast")
	idsFile := flag.String("ids", "", "use only the hands whose ID is in this file, one per line (text after the ID is ignored); the checks still apply")
	onlyName := flag.String("only", "", "read only files whose name contains this text")
	mismatches := flag.String("mismatches", "", "list the copies of a hand whose text differs from the first copy's in this file")
	flag.BoolVar(&repair, "repair", false, "repair a hand that fails a check, as hhclean -repair does, instead of leaving it out")
	pass1 := flag.Bool("pass1", false, "stop after the first pass and write no packs; -out is not needed")
	nanon := flag.Int("anon", 16, "put the players with fewer than -min hands into this many packs, AnonPlayer00 and on, each hand's such players to different ones; at least 9, or 0 to leave them out")
	names := flag.String("name", "", "comma-separated players: pack only these, whatever -min, and no anonymous packs unless -anon is given")
	read := flag.String("read", "", "read this pack file back and summarise it, instead of building packs")
	maxPacks := flag.Int("maxpacks", 1000, "give at most this many players a pack of their own, those with the most hands; the others go to the anonymous packs. With -anon it must leave room for those under 65536 pack IDs")
	flag.Parse()
	if *read != "" {
		readBack(*read)
		return
	}
	var only map[string]bool // -name: the players to pack, as the views name them
	if *names != "" {
		only = map[string]bool{}
		for _, name := range strings.Split(*names, ",") {
			only[clean.PlayerName(name)] = true
		}
		anonSet := false
		flag.Visit(func(f *flag.Flag) { anonSet = anonSet || f.Name == "anon" })
		if !anonSet {
			*nanon = 0
		}
	}
	stopProfiles := startProfiles()
	defer stopProfiles()
	if *idsFile != "" {
		want, err := readIDs(*idsFile)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		wanted = want
	}
	nameFilter = *onlyName
	if (*out == "" && !*pass1) || flag.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "usage: hhpack -out DIR [-min N | -name A,B] [-maxpacks N] [-anon N] [-budget MB] [-codec none|flate] [-repair] [-pass1] path...")
		fmt.Fprintln(os.Stderr, "       hhpack -read FILE")
		os.Exit(2)
	}
	c := hh.CodecFlate
	switch *codec {
	case "none":
		c = hh.CodecNone
	case "flate":
	default:
		fmt.Fprintln(os.Stderr, "codec must be none or flate")
		os.Exit(2)
	}
	*minHands = max(*minHands, 1)
	// A hand has up to nine players dealt in, each needing a different pack.
	if *nanon != 0 && (*nanon < 9 || *nanon > 64) {
		fmt.Fprintln(os.Stderr, "-anon must be 0 or from 9 to 64")
		os.Exit(2)
	}
	// Pack IDs are 16 bits and 0 is no pack, so 65535 packs at most.
	if *maxPacks < 1 || *maxPacks+*nanon > 0xFFFF {
		fmt.Fprintf(os.Stderr, "-maxpacks must be from 1 to %d with -anon %d\n", 0xFFFF-*nanon, *nanon)
		os.Exit(2)
	}
	if !*pass1 {
		if err := os.MkdirAll(*out, 0o755); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}

	// Players are divided among shards by name. Each shard has one goroutine that
	// owns its players, so the packing runs in parallel and needs no locks.
	nshards := runtime.NumCPU()
	files := &fileNames{taken: map[string]string{}}
	shards := make([]*shard, nshards)
	var swg sync.WaitGroup
	for i := range shards {
		shards[i] = &shard{
			// Only players known from the survey to have -min hands reach the
			// shards, so each gets a file with its first hand.
			in: make(chan []entry, 16), dir: *out, minHands: 1, codec: c,
			budget: max(*budget<<20/nshards, 1), files: files, players: map[string]*player{},
		}
		swg.Add(1)
		go func() {
			defer swg.Done()
			shards[i].run()
		}()
	}

	// Pass 1: survey every copy of every hand, all sources together.
	start := time.Now()
	roots := flag.Args()
	sv := newSurvey()
	nfiles := surveyPass(roots, sv)
	nhands, nparsed, nclean := sv.tally()
	// The players who get a file: those with -min hands, and of them only the
	// -maxpacks with the most hands, ties broken by name so every run keeps the
	// same. ranked is in that order, which is also the order of the pack IDs.
	var ranked []string
	for name, n := range sv.counts {
		if only == nil && n >= *minHands || only[name] {
			ranked = append(ranked, name)
		}
	}
	for name := range only {
		if sv.counts[name] == 0 {
			fmt.Fprintf(os.Stderr, "%s has no hands\n", name)
		}
	}
	sort.Slice(ranked, func(i, j int) bool {
		a, b := sv.counts[ranked[i]], sv.counts[ranked[j]]
		return a > b || a == b && ranked[i] < ranked[j]
	})
	over := len(ranked)
	ranked = ranked[:min(len(ranked), *maxPacks)]
	want := make(map[string]bool, len(ranked))
	for _, name := range ranked {
		want[name] = true
	}
	fmt.Printf("pass 1: %d files, %d copies of %d hands: %d parse, %d of them with a copy that passes the checks as it is (%.1fs)\n",
		nfiles, sv.copies, nhands, nparsed, nclean, time.Since(start).Seconds())
	which := fmt.Sprintf("with at least %d hands", *minHands)
	if only != nil {
		which = "named by -name"
	}
	fmt.Printf("pass 1: copies compared with an earlier copy's text: %d, with different text: %d; %d players, %d %s\n",
		sv.compared, len(sv.mismatches), len(sv.counts), over, which)
	if over > len(ranked) {
		fmt.Printf("pass 1: only the %d with the most hands get a pack of their own (-maxpacks)\n", len(ranked))
	}
	if *mismatches != "" {
		text := "site\thand\tfirst file\tother file\n" + strings.Join(sv.mismatchLines(), "\n") + "\n"
		if err := os.WriteFile(*mismatches, []byte(text), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
		} else {
			fmt.Printf("pass 1: the copies with different text are listed in %s\n", *mismatches)
		}
	}
	if *pass1 {
		return
	}
	var anon []string
	if *nanon > 0 {
		anon = anonNames(*nanon)
		for name := range sv.counts {
			if hh.IsAnonPlayer(name) {
				fmt.Fprintf(os.Stderr, "a player is called %s, which is the name of an anonymous pack\n", name)
				os.Exit(1)
			}
		}
	}

	ids, err := packIDs(ranked, anon)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for _, s := range shards {
		s.ids = ids // read only from here on; the shards get their first hands after this
	}

	// Pass 2: build the views of the players who get a file, from each hand's
	// chosen copy.
	var tb *tableBuilder
	if anon != nil {
		tb = newTableBuilder(sv.sampleKeys(tableSample))
	}
	used := buildPass(roots, sv, want, anon, ids, tb, shards)
	fmt.Printf("pass 2: %d hands used (%.1fs)\n", used, time.Since(start).Seconds())
	for _, s := range shards {
		close(s.in)
	}
	swg.Wait()
	if tb != nil {
		n, err := tb.write(*out)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error: hand table:", err)
			os.Exit(1)
		}
		fmt.Printf("hand table: %d hands in hands.hht (%.1fs)\n", n, time.Since(start).Seconds())
	}

	var kept, keptHands, bytesOut int
	var firstErr error
	var top []result
	for _, s := range shards {
		if s.err != nil && firstErr == nil {
			firstErr = s.err
		}
		kept += len(s.results)
		for _, r := range s.results {
			keptHands += r.hands
			bytesOut += r.bytes
		}
		top = append(top, s.results...)
	}
	if firstErr != nil {
		fmt.Fprintln(os.Stderr, "error:", firstErr)
		os.Exit(1)
	}
	if n := twoViews.Load(); n > 0 {
		fmt.Fprintf(os.Stderr, "error: %d hands would have put two views in one pack; they were left out\n", n)
	}
	var anonPacks, anonHands, anonBytes int
	for _, r := range top {
		if hh.IsAnonPlayer(r.name) {
			anonPacks++
			anonHands += r.hands
			anonBytes += r.bytes
		}
	}
	fmt.Printf("%d files, %d hands used in %.1fs, %d hands whose chosen copy failed a check left out\n",
		nfiles, used, time.Since(start).Seconds(), failed.Load())
	fmt.Printf("%d players %s got a file (%d hands, %.1f MB)\n",
		kept-anonPacks, which, keptHands-anonHands, float64(bytesOut-anonBytes)/(1<<20))
	if anonPacks > 0 {
		fmt.Printf("%d anonymous packs for the other players (%d hands, %.1f MB)\n",
			anonPacks, anonHands, float64(anonBytes)/(1<<20))
	}
	sort.Slice(top, func(i, j int) bool { return top[i].hands > top[j].hands })
	shown := 0
	for _, r := range top {
		if shown == 10 {
			break
		}
		if hh.IsAnonPlayer(r.name) {
			continue
		}
		fmt.Printf("  %-24s %9d hands %9d bytes  %s\n", r.name, r.hands, r.bytes, filepath.Base(r.path))
		shown++
	}
	if twoViews.Load() > 0 {
		stopProfiles()
		os.Exit(1)
	}
}

// surveyPass is the first pass: it reads every copy of every hand under the
// roots, parses and checks it without repairing it, and records in the survey
// which copy of each hand is the best and how many hands each player has. It
// returns how many files it read.
func surveyPass(roots []string, sv *survey) int64 {
	var files atomic.Int64
	err := hh.WalkText(roots, runtime.NumCPU(), func(p string, b []byte) {
		if !strings.Contains(filepath.Base(p), nameFilter) {
			return
		}
		files.Add(1)
		f := sv.file(p)
		var names []string // players of the hands first seen here, once per hand
		blocks := clean.Split(b)
		for bi, block := range blocks {
			site, id, ok := blockID(block)
			if !ok {
				continue
			}
			if _, ok := wanted[id]; wanted != nil && !ok {
				continue
			}
			h, isClean, _ := examine(block, false)
			cards := 0
			if h != nil {
				cards = shownCards(h)
			}
			c := copyRef{q: quality(isClean, cards), file: f, block: uint32(bi), hash: textHash(block)}
			if sv.add(site, id, c, h != nil) {
				names = append(names, viewNames(h)...)
			}
		}
		sv.count(names)
		sv.statsMu.Lock()
		sv.copies += int64(len(blocks))
		sv.statsMu.Unlock()
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	return files.Load()
}

// buildPass is the second pass: it reads the files again, takes the copy of each
// hand that the survey chose, and sends the views of the players in want to the
// shards, and with anon the views of all other players too, under the anonymous
// pack names. Each view gets the pack ID of every player in it (see packIDs). It
// returns how many hands it used.
func buildPass(roots []string, sv *survey, want map[string]bool, anon []string, ids map[string]uint16, tb *tableBuilder, shards []*shard) int64 {
	accept := func(name string) bool { return want[name] }
	if anon != nil {
		accept = nil // everyone dealt in
	}
	nshards := len(shards)
	var hands atomic.Int64
	err := hh.WalkText(roots, runtime.NumCPU(), func(p string, b []byte) {
		if !strings.Contains(filepath.Base(p), nameFilter) {
			return
		}
		f, ok := sv.fileIndex[p] // read only, after the survey
		if !ok {
			return
		}
		pending := make([][]entry, nshards)
		send := func(s int) {
			if len(pending[s]) > 0 {
				shards[s].in <- pending[s]
				pending[s] = nil
			}
		}
		var n int64
		var fileHands []fileHand // this file's rows of the hand table
		broken := 0
		for bi, block := range clean.Split(b) {
			site, id, ok := blockID(block)
			if !ok || !sv.chosen(site, id, f, uint32(bi)) {
				continue
			}
			h, _, ok := examine(block, repair)
			if !ok {
				failed.Add(1)
				continue
			}
			n++
			views, names, err := h.NamedPlayerHandsFor(accept)
			if err != nil {
				continue
			}
			if anon != nil {
				assignAnon(names, want, anon, site, id)
			}
			// A pack must not hold two views of one hand.
			if !distinct(names) {
				twoViews.Add(1)
				continue
			}
			// The pack ID of each position, from the view of whoever sits there;
			// a player whose hands go to no pack has no view and stays NoPack.
			var slotID [16]uint16
			for k := range views {
				slotID[views[k].Hero] = ids[names[k]]
			}
			for k := range views {
				for p := range views[k].Players {
					views[k].Players[p].PackID = slotID[p]
				}
			}
			if tb != nil {
				if fh, ok := tb.hand(site, id, views, &slotID); ok {
					fileHands = append(fileHands, fh)
				} else {
					broken++
				}
			}
			for i := range views {
				s := shardOf(names[i], nshards)
				pending[s] = append(pending[s], entry{names[i], views[i]})
				if len(pending[s]) >= 256 {
					send(s)
				}
			}
		}
		for s := range pending {
			send(s)
		}
		if tb != nil {
			tb.add(fileHands, broken)
		}
		hands.Add(n)
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	return hands.Load()
}

func shardOf(name string, n int) int {
	h := fnv.New32a()
	h.Write([]byte(name))
	return int(h.Sum32() % uint32(n))
}

// result describes a file that was written.
type result struct {
	name, path   string
	hands, bytes int
}

// shard owns the players whose names hash to it.
type shard struct {
	in       chan []entry
	dir      string
	minHands int
	codec    hh.Codec
	budget   int // bytes of hands waiting for their block before short blocks are written
	files    *fileNames
	ids      map[string]uint16 // pack IDs, set before the first hand arrives

	players      map[string]*player
	buffered     int // bytes of hands waiting for their block
	nextRelieve  int // buffered at which relieve runs again; see relieve
	results      []result
	dropped      int
	droppedHands int
	err          error
}

// player is one player's pack writer and where its output goes.
type player struct {
	name string
	pw   *hh.PackWriter
	out  *spool
	n    int // hands so far
}

func (s *shard) run() {
	for batch := range s.in {
		for i := range batch {
			if s.err == nil {
				s.err = s.add(batch[i].name, &batch[i].p)
			}
		}
	}
	if s.err == nil {
		s.err = s.finish()
	}
}

func (s *shard) add(name string, p *hh.PlayerHand) error {
	pl := s.players[name]
	if pl == nil {
		pl = &player{name: name, out: &spool{}}
		var err error
		if pl.pw, err = hh.NewPackWriter(pl.out, hh.PackInfo{Player: name, ID: s.ids[name], Anon: hh.IsAnonPlayer(name)}, s.codec); err != nil {
			return err
		}
		s.players[name] = pl
	}
	before := pl.pw.BufferedBytes()
	if err := pl.pw.Write(p); err != nil {
		return err
	}
	s.buffered += pl.pw.BufferedBytes() - before
	pl.n++
	if pl.n == s.minHands {
		if err := s.commit(pl); err != nil {
			return err
		}
	}
	if s.buffered > max(s.budget, s.nextRelieve) {
		return s.relieve()
	}
	return nil
}

// relieve writes the blocks that have a useful number of hands, longest first,
// until the shard is back under half its budget. If it cannot get there, because
// the hands are spread over many players with few each, it does not run again
// until the memory has grown by another half of the budget: running it on every
// hand would sort the players on every hand.
func (s *shard) relieve() error {
	list := make([]*player, 0, len(s.players))
	for _, pl := range s.players {
		list = append(list, pl)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].pw.BufferedBytes() > list[j].pw.BufferedBytes() })
	for _, pl := range list {
		if s.buffered <= s.budget/2 || pl.pw.Buffered() < 128 {
			break
		}
		s.buffered -= pl.pw.BufferedBytes()
		if err := pl.pw.Flush(); err != nil {
			return err
		}
	}
	s.nextRelieve = s.buffered + s.budget/2
	return nil
}

// commit gives the player a file, which gets everything held so far.
func (s *shard) commit(pl *player) error {
	path := filepath.Join(s.dir, s.files.name(pl.name)+".hhpk")
	return pl.out.attach(path)
}

func (s *shard) finish() error {
	names := make([]string, 0, len(s.players))
	for name := range s.players {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		pl := s.players[name]
		if pl.n < s.minHands {
			s.dropped++
			s.droppedHands += pl.n
			continue
		}
		if err := pl.pw.Close(); err != nil {
			return err
		}
		size, err := pl.out.close()
		if err != nil {
			return err
		}
		s.results = append(s.results, result{name: name, path: pl.out.path, hands: pl.n, bytes: size})
	}
	return nil
}

// spool holds a pack file's bytes in memory until it is given a file, and then
// writes them there.
type spool struct {
	mem  bytes.Buffer
	f    *os.File
	bw   *bufio.Writer
	path string
	size int
}

func (sp *spool) Write(b []byte) (int, error) {
	sp.size += len(b)
	if sp.bw != nil {
		return sp.bw.Write(b)
	}
	return sp.mem.Write(b)
}

func (sp *spool) attach(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	sp.f, sp.path = f, path
	sp.bw = bufio.NewWriterSize(f, 1<<16)
	_, err = sp.bw.Write(sp.mem.Bytes())
	sp.mem = bytes.Buffer{}
	return err
}

func (sp *spool) close() (int, error) {
	err := sp.bw.Flush()
	if cerr := sp.f.Close(); err == nil {
		err = cerr
	}
	return sp.size, err
}

// fileNames turns player names into file names that are safe and do not
// collide. Windows ignores case in file names, so "Bob" and "bob" would.
type fileNames struct {
	mu    sync.Mutex
	taken map[string]string // lower-case file name -> player
}

func (f *fileNames) name(player string) string {
	var b strings.Builder
	for _, r := range player {
		if r < 0x20 || strings.ContainsRune(`<>:"/\|?*`, r) {
			r = '_'
		}
		b.WriteRune(r)
	}
	name := strings.TrimRight(b.String(), ". ")
	switch strings.ToUpper(name) {
	case "", "CON", "PRN", "AUX", "NUL", "COM1", "COM2", "COM3", "COM4", "LPT1", "LPT2", "LPT3":
		name += "_"
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if other, taken := f.taken[strings.ToLower(name)]; taken && other != player {
		h := fnv.New32a()
		h.Write([]byte(player))
		name = fmt.Sprintf("%s~%08x", name, h.Sum32())
	}
	f.taken[strings.ToLower(name)] = player
	return name
}

// readBack reads a pack file and prints its player, hands, size, how long it
// took and the player's net result over the hands.
func readBack(path string) {
	t := time.Now()
	player, hands, err := hh.ReadPackFile(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	el := time.Since(t)
	var net hh.Cents
	for i := range hands {
		net += hands[i].Net()
	}
	st, err := os.Stat(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("%s: %d hands, %d bytes, read in %.0f ms (%.0f hands/s)\n", player, len(hands), st.Size(),
		float64(el.Microseconds())/1000, float64(len(hands))/el.Seconds())
	fmt.Printf("net over all of them: %s\n", net)
}
