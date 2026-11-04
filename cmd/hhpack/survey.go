package main

import (
	"bytes"
	"fmt"
	"hash/fnv"
	"sort"
	"sync"

	"github.com/seanq222/hh"
)

// A survey is what the first pass learns about every hand: which of its copies to
// use, how many hands each player has, and whether the copies' texts agree. It is
// sharded so that the readers rarely wait for each other.
type survey struct {
	shards [256]struct {
		mu    sync.Mutex
		hands map[uint64]handEntry
	}

	filesMu   sync.Mutex
	paths     []string          // file index -> path
	fileIndex map[string]uint32 // path -> file index

	countsMu sync.Mutex
	counts   map[string]int // player -> hands, counted once per hand

	statsMu    sync.Mutex
	copies     int64    // copies read
	compared   int64    // copies compared with the first copy's text
	mismatches []string // "site\tID\tfirst file index\tother file index"
}

// handEntry is what the survey keeps about a hand.
type handEntry struct {
	best      copyRef
	parsed    bool   // some copy parses
	firstHash uint64 // text hash of the first copy read, for the comparison
	firstFile uint32
}

// copyRef names a copy of a hand, the file and its position there, with how good
// it is and the hash of its text.
type copyRef struct {
	q     uint8
	file  uint32
	block uint32
	hash  uint64
}

// quality ranks a copy: one that passes the checks as it is beats one that does
// not, and then the copy that shows more players' hole cards wins.
func quality(clean bool, cards int) uint8 {
	q := uint8(min(cards, 31))
	if clean {
		q |= 1 << 5
	}
	return q
}

func isClean(q uint8) bool { return q&(1<<5) != 0 }

// better reports whether copy a is to be used over copy b: the higher quality,
// and for equal quality the lower hash of the text, so that every run picks the
// same copy. Copies with the same hash have the same text.
func better(a, b copyRef) bool {
	if a.q != b.q {
		return a.q > b.q
	}
	return a.hash < b.hash
}

func newSurvey() *survey {
	s := &survey{fileIndex: map[string]uint32{}, counts: map[string]int{}}
	for i := range s.shards {
		s.shards[i].hands = map[uint64]handEntry{}
	}
	return s
}

// file registers a file and returns its index.
func (s *survey) file(path string) uint32 {
	s.filesMu.Lock()
	defer s.filesMu.Unlock()
	if i, ok := s.fileIndex[path]; ok {
		return i
	}
	i := uint32(len(s.paths))
	s.paths = append(s.paths, path)
	s.fileIndex[path] = i
	return i
}

// add records a copy of a hand; parsed says whether it parsed at all. Its text
// is compared with the first copy's. add reports whether this is the first copy
// of the hand that parses, so that the hand's players are counted once.
func (s *survey) add(site hh.Site, id int64, c copyRef, parsed bool) (firstParsed bool) {
	key := uint64(id) ^ uint64(site)<<56
	sh := &s.shards[key%uint64(len(s.shards))]
	sh.mu.Lock()
	e, seen := sh.hands[key]
	compared, differs := false, false
	if !seen {
		e.best, e.firstHash, e.firstFile = c, c.hash, c.file
	} else {
		compared, differs = true, c.hash != e.firstHash
		if better(c, e.best) {
			e.best = c
		}
	}
	firstParsed = parsed && !e.parsed
	e.parsed = e.parsed || parsed
	firstFile := e.firstFile
	sh.hands[key] = e
	sh.mu.Unlock()
	if compared {
		s.statsMu.Lock()
		s.compared++
		if differs {
			s.mismatches = append(s.mismatches, fmt.Sprintf("%v\t%d\t%d\t%d", site, id, firstFile, c.file))
		}
		s.statsMu.Unlock()
	}
	return firstParsed
}

// chosen reports whether the copy is the one to use for its hand.
func (s *survey) chosen(site hh.Site, id int64, file, block uint32) bool {
	key := uint64(id) ^ uint64(site)<<56
	sh := &s.shards[key%uint64(len(s.shards))]
	sh.mu.Lock()
	e := sh.hands[key]
	sh.mu.Unlock()
	return e.parsed && e.best.file == file && e.best.block == block
}

// count adds hands to their players' counts.
func (s *survey) count(names []string) {
	s.countsMu.Lock()
	for _, n := range names {
		s.counts[n]++
	}
	s.countsMu.Unlock()
}

// tally returns the hands seen, those with a copy that parses, and those whose
// chosen copy passes the checks as it is.
func (s *survey) tally() (hands, parsed, clean int) {
	for i := range s.shards {
		sh := &s.shards[i]
		sh.mu.Lock()
		for _, e := range sh.hands {
			hands++
			if e.parsed {
				parsed++
				if isClean(e.best.q) {
					clean++
				}
			}
		}
		sh.mu.Unlock()
	}
	return hands, parsed, clean
}

// mismatchLines returns the copies whose text differed from the first copy's, with
// the file paths, sorted.
func (s *survey) mismatchLines() []string {
	var lines []string
	for _, m := range s.mismatches {
		var site string
		var id int64
		var a, b uint32
		fmt.Sscanf(m, "%s\t%d\t%d\t%d", &site, &id, &a, &b)
		lines = append(lines, fmt.Sprintf("%s\t%d\t%s\t%s", site, id, s.paths[a], s.paths[b]))
	}
	sort.Strings(lines)
	return lines
}

// textHash hashes a hand's text line by line, without carriage returns, trailing
// blanks or blank lines, so that only what the lines say counts.
func textHash(text []byte) uint64 {
	h := fnv.New64a()
	for len(text) > 0 {
		line := text
		if i := bytes.IndexByte(text, '\n'); i >= 0 {
			line, text = text[:i], text[i+1:]
		} else {
			text = nil
		}
		line = bytes.TrimRight(line, " \t\r")
		if len(line) == 0 {
			continue
		}
		h.Write(line)
		h.Write([]byte{'\n'})
	}
	return h.Sum64()
}

// blockID returns the site and ID of a hand's text, from its first line: "Hand #N"
// (WPN) or "CoinPoker Hand #N:" (CoinPoker). ok is false if there is no ID.
func blockID(block []byte) (site hh.Site, id int64, ok bool) {
	i := bytes.Index(block, []byte("Hand #"))
	if i < 0 {
		return 0, 0, false
	}
	if bytes.HasPrefix(block, []byte("CoinPoker ")) {
		site = hh.CoinPoker
	}
	for _, c := range block[i+len("Hand #"):] {
		if c < '0' || c > '9' {
			break
		}
		id = id*10 + int64(c-'0')
		ok = true
	}
	return site, id, ok
}
