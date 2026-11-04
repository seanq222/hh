package main

import (
	"math"
	"testing"

	"github.com/seanq222/hh"
)

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

// Two anon players lose 10 and 5; winners Carol and Dave win 12 and 3, Carol
// paying 1 in rake. Carol takes 80% of each loss, Dave 20%.
func TestSplitAcrossGroups(t *testing.T) {
	var tl tally
	split([]seat{
		{group: anon, net: -10},
		{group: anon, net: -5},
		{group: winners, net: 12, rake: 1},
		{group: winners, net: 3},
	}, &tl)
	if !near(tl.flow[winners][anon], 15) || !near(tl.flow[anon][winners], 0) || !near(tl.flow[winners][winners], 0) {
		t.Errorf("flows %v", tl.flow)
	}
	if !near(tl.rake[winners][anon], 1) {
		t.Errorf("rake %v", tl.rake)
	}
	if tl.hands[anon] != 2 || tl.hands[winners] != 2 || !near(tl.ev[winners], 14) {
		t.Errorf("hands %v, ev %v", tl.hands, tl.ev)
	}
	// Each player's three opponents are two of the other group and one of its
	// own, so each group's two hands are 4/3 against the other and 2/3 its own.
	if !near(tl.share[winners][anon], 4.0/3) || !near(tl.share[anon][winners], 4.0/3) || !near(tl.share[winners][winners], 2.0/3) || tl.share[winners][losers] != 0 {
		t.Errorf("share %v", tl.share)
	}
}

// Dave is anon too: his win of 3 from Alice and Bob stays inside the group, so it
// is a flow from anon to anon, which cancels out of the group's edge.
func TestSplitInsideGroup(t *testing.T) {
	var tl tally
	split([]seat{
		{group: anon, net: -10},
		{group: anon, net: -5},
		{group: winners, net: 12, rake: 1},
		{group: anon, net: 3, rake: 0.5},
	}, &tl)
	if !near(tl.flow[winners][anon], 12) || !near(tl.flow[anon][anon], 3) {
		t.Errorf("flows %v", tl.flow)
	}
	if !near(tl.rake[anon][anon], 0.5) || !near(tl.rake[winners][anon], 1) {
		t.Errorf("rake %v", tl.rake)
	}
}

// A hand nobody won before rake, such as a chop, moves nothing.
func TestSplitNoFlow(t *testing.T) {
	var tl tally
	split([]seat{{group: losers, net: 0}, {group: winners, net: 0}}, &tl)
	if tl.flow != ([nGroups][nGroups]float64{}) || tl.hands[losers] != 1 {
		t.Errorf("flows %v, hands %v", tl.flow, tl.hands)
	}
}

func TestHalfSplitsEvenly(t *testing.T) {
	var n [2]int
	for id := range uint64(100000) {
		n[half(hh.HandKey(hh.WPN, int64(id)))]++
	}
	if n[0] < 49000 || n[1] < 49000 {
		t.Errorf("halves %v, want about 50,000 each", n)
	}
}
