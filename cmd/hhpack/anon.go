package main

import (
	"fmt"

	"github.com/seanq222/hh"
)

// anonNames returns the names of n anonymous packs, AnonPlayer00 and on.
func anonNames(n int) []string {
	names := make([]string, n)
	for i := range names {
		names[i] = fmt.Sprintf("%s%02d", hh.AnonPrefix, i)
	}
	return names
}

// assignAnon renames, in place, every player of a hand who does not get a pack of
// their own to one of the anonymous packs, a different one for each, so that no
// pack gets two views of the hand. The choice looks random but depends only on
// the hand's site and ID, so every run and every copy of the hand gives the
// same. There must be no more such players than anonymous packs.
func assignAnon(names []string, want map[string]bool, anon []string, site hh.Site, id int64) {
	var perm [64]uint8
	for i := range anon {
		perm[i] = uint8(i)
	}
	rng := splitmix(uint64(site)<<56 ^ uint64(id))
	j := 0
	for i, name := range names {
		if want[name] {
			continue
		}
		// One step of a Fisher-Yates shuffle of the pack numbers.
		r := j + int(rng.next()%uint64(len(anon)-j))
		perm[j], perm[r] = perm[r], perm[j]
		names[i] = anon[perm[j]]
		j++
	}
}

// splitmix is the SplitMix64 generator: small, fast, and fixed, so the
// anonymous packs do not change with the Go release.
type splitmix uint64

func (s *splitmix) next() uint64 {
	*s += 0x9e3779b97f4a7c15
	z := uint64(*s)
	z = (z ^ z>>30) * 0xbf58476d1ce4e5b9
	z = (z ^ z>>27) * 0x94d049bb133111eb
	return z ^ z>>31
}

// distinct reports whether no name appears twice.
func distinct(names []string) bool {
	for i := range names {
		for j := i + 1; j < len(names); j++ {
			if names[i] == names[j] {
				return false
			}
		}
	}
	return true
}

// packIDs numbers the packs: the players who get one, from 1, in the order
// given (most hands first), then the anonymous packs. Each pack's header holds
// its ID (see hh.ReadPackIDs). hh.NoPack (0) is no pack, so there can be 65535
// packs at most.
func packIDs(players, anon []string) (map[string]uint16, error) {
	names := append(append([]string(nil), players...), anon...)
	if len(names) > 0xFFFF {
		return nil, fmt.Errorf("%d packs is more than the 65535 that pack IDs can number", len(names))
	}
	ids := make(map[string]uint16, len(names))
	for i, name := range names {
		ids[name] = uint16(i + 1)
	}
	return ids, nil
}
