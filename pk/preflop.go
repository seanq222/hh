package pk

import (
	_ "embed"
	"sort"
)

//go:generate go run ./internal/mkpreflop -out preflop.bin

// preflopTable holds the result of Equity for every heads-up matchup, worked out
// in advance by mkpreflop. Matchups that differ only by a renaming of the suits,
// or by which hand is first, have the same result, which leaves 47,008 of them.
// Each is a record of three 3-byte big-endian numbers: the matchup's key (see
// PreflopKey), and the first hand's wins and draws. The records are sorted by
// key.
//
//go:embed preflop.bin
var preflopTable []byte

const preflopRecord = 9

// PreflopEquity returns what Equity does, the first hand's wins and draws over
// all 1,712,304 boards, from a table instead of by counting.
func PreflopEquity(a0, a1, b0, b1 Card) (wins, draws, total int) {
	key, swapped := PreflopKey(a0, a1, b0, b1)
	n := len(preflopTable) / preflopRecord
	i := sort.Search(n, func(i int) bool { return get24(preflopTable[i*preflopRecord:]) >= key })
	if i == n || get24(preflopTable[i*preflopRecord:]) != key {
		panic("pk: matchup missing from the preflop table")
	}
	r := preflopTable[i*preflopRecord:]
	wins, draws = int(get24(r[3:])), int(get24(r[6:]))
	if swapped {
		wins = _48Choose5 - wins - draws // the other hand's wins
	}
	return wins, draws, _48Choose5
}

// PreflopKey returns the same key for heads-up matchups that differ only by a
// renaming of the suits or by which hand comes first. The key is the four cards
// of the matchup in a standard form, six bits each: the first hand's lower and
// higher card, then the second hand's. swapped says that b0 b1 is the key's first
// hand; a matchup that is the same either way round says false.
func PreflopKey(a0, a1, b0, b1 Card) (key uint32, swapped bool) {
	key = 1<<24 - 1
	for _, perm := range suitPerms {
		var c [4]uint32
		for i, card := range [4]Card{a0, a1, b0, b1} {
			c[i] = uint32(card.Rank()<<2 | perm[card.Suit()])
		}
		if c[0] > c[1] {
			c[0], c[1] = c[1], c[0]
		}
		if c[2] > c[3] {
			c[2], c[3] = c[3], c[2]
		}
		if k := c[0]<<18 | c[1]<<12 | c[2]<<6 | c[3]; k < key {
			key, swapped = k, false
		}
		if k := c[2]<<18 | c[3]<<12 | c[0]<<6 | c[1]; k < key {
			key, swapped = k, true
		}
	}
	return key, swapped
}

// PreflopKeyCards returns the matchup a key stands for, in its standard form.
func PreflopKeyCards(key uint32) (a0, a1, b0, b1 Card) {
	return Card(key >> 18 & 63), Card(key >> 12 & 63), Card(key >> 6 & 63), Card(key & 63)
}

// suitPerms are the 24 ways to rename the four suits.
var suitPerms = func() (perms [][4]int) {
	var rec func(cur [4]int, used, k int)
	rec = func(cur [4]int, used, k int) {
		if k == 4 {
			perms = append(perms, cur)
			return
		}
		for s := 0; s < 4; s++ {
			if used>>s&1 == 0 {
				cur[k] = s
				rec(cur, used|1<<s, k+1)
			}
		}
	}
	rec([4]int{}, 0, 0)
	return
}()

func get24(b []byte) uint32 { return uint32(b[0])<<16 | uint32(b[1])<<8 | uint32(b[2]) }
