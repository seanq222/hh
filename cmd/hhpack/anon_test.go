package main

import (
	"fmt"
	"maps"
	"slices"
	"testing"

	"github.com/seanq222/hh"
)

func TestAssignAnon(t *testing.T) {
	anon := anonNames(16)
	want := map[string]bool{"Alice": true, "Bob": true}
	used := map[string]int{}
	for id := int64(1000000); id < 1010000; id++ {
		names := []string{"Carol", "Alice", "Dave", "Erin", "Frank", "Bob", "Grace", "Heidi", "Ivan"}
		assignAnon(names, want, anon, hh.WPN, id)
		if names[1] != "Alice" || names[5] != "Bob" {
			t.Fatalf("hand %d: a player with a pack was renamed: %v", id, names)
		}
		if !distinct(names) {
			t.Fatalf("hand %d: two players got the same pack: %v", id, names)
		}
		for i, n := range names {
			if i != 1 && i != 5 {
				if !hh.IsAnonPlayer(n) || !slices.Contains(anon, n) {
					t.Fatalf("hand %d: %q is not an anonymous pack", id, n)
				}
				used[n]++
			}
		}
		again := []string{"Carol", "Alice", "Dave", "Erin", "Frank", "Bob", "Grace", "Heidi", "Ivan"}
		assignAnon(again, want, anon, hh.WPN, id)
		if !slices.Equal(names, again) {
			t.Fatalf("hand %d: a second run chose differently", id)
		}
	}
	// 70,000 players over 16 packs: each should get close to 4,375.
	for _, n := range anon {
		if used[n] < 4000 || used[n] > 4750 {
			t.Errorf("%s got %d of 70000 players", n, used[n])
		}
	}
	// The same hand ID on another site is another hand.
	a := []string{"Carol", "Dave"}
	b := []string{"Carol", "Dave"}
	diff := 0
	for id := int64(1); id <= 100; id++ {
		a[0], a[1], b[0], b[1] = "Carol", "Dave", "Carol", "Dave"
		assignAnon(a, nil, anon, hh.WPN, id)
		assignAnon(b, nil, anon, hh.CoinPoker, id)
		if !slices.Equal(a, b) {
			diff++
		}
	}
	if diff < 80 {
		t.Errorf("only %d of 100 hands differ between sites", diff)
	}
}

func TestPackIDs(t *testing.T) {
	ids, err := packIDs([]string{"Carol", "Alice", "Bob"}, anonNames(2))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]uint16{"Carol": 1, "Alice": 2, "Bob": 3, "AnonPlayer00": 4, "AnonPlayer01": 5}
	if !maps.Equal(ids, want) {
		t.Errorf("ids %v, want %v", ids, want)
	}
}

// Pack IDs are 16 bits and 0 is no pack: 65535 packs fit, 65536 do not.
func TestPackIDsLimit(t *testing.T) {
	names := make([]string, 0xFFFF-16)
	for i := range names {
		names[i] = fmt.Sprint("Player", i)
	}
	ids, err := packIDs(names, anonNames(16))
	if err != nil || ids["AnonPlayer15"] != 0xFFFF {
		t.Errorf("65535 packs: %v, last ID %d", err, ids["AnonPlayer15"])
	}
	if _, err := packIDs(append(names, "OneMore"), anonNames(16)); err == nil {
		t.Error("65536 packs accepted")
	}
}
