package pk

import "testing"

func mustHole(t testing.TB, s string) HoleCards {
	t.Helper()
	h, err := ParseHoleCards(s)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func mustBoard(t testing.TB, s string) Board {
	t.Helper()
	b, err := ParseBoard(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestNextCard(t *testing.T) {
	for _, tc := range []struct {
		hole, board string
		class       HandClass
		outs        map[HandClass]int
	}{
		// The nut flush draw with two overcards: 9 hearts, and 6 aces and kings.
		{"AhKh", "Qh7h2c", HighCard, map[HandClass]int{OnePair: 15, Flush: 9, Straight: 9, StraightFlush: 0}},
		// An open-ended straight draw: four tens and four fives; a 9, 8, 7 or 6
		// pairs a hole card.
		{"9c8d", "7h6s2c", HighCard, map[HandClass]int{OnePair: 14, Straight: 8, Flush: 0}},
		// A gutshot: only a ten.
		{"9c8d", "Jh7s2c", HighCard, map[HandClass]int{Straight: 4}},
		// A two on the board pairs it for everyone, so it is not an out.
		{"AsKd", "Qh7h2c", HighCard, map[HandClass]int{OnePair: 6}},
		// A set on the turn: the board pairing or the fourth seven fills up.
		{"7c7d", "7h5s2cKd", ThreeOfAKind, map[HandClass]int{FullHouse: 10, FourOfAKind: 1, TwoPair: 0}},
	} {
		board := mustBoard(t, tc.board)
		d := NextCard(mustHole(t, tc.hole), board)
		if d.Class != tc.class {
			t.Errorf("%s on %s: class %v, want %v", tc.hole, tc.board, d.Class, tc.class)
		}
		if want := 52 - 2 - len(board); d.Unseen != want {
			t.Errorf("%s on %s: %d unseen, want %d", tc.hole, tc.board, d.Unseen, want)
		}
		for c, n := range tc.outs {
			if d.Outs[c] != n {
				t.Errorf("%s on %s: %d outs to %v, want %d", tc.hole, tc.board, d.Outs[c], c, n)
			}
		}
	}
}

func TestRunoutsFromFlop(t *testing.T) {
	r := RunoutsFromFlop(mustHole(t, "AhKh"), mustBoard(t, "Qh7h2c"))
	if r.Total != 1081 {
		t.Errorf("total %d, want 1081", r.Total)
	}
	sum := r.Board
	for _, n := range r.Class {
		sum += n
	}
	if sum != r.Total {
		t.Errorf("classes add up to %d, want %d", sum, r.Total)
	}
	// Two more hearts from 9: 36 runouts; one more with any other card: 9 * 38.
	// Some of the 36 and 342 make a straight flush instead (Jh Th).
	flushes := r.Class[Flush] + r.Class[StraightFlush]
	if flushes != 36+9*38 {
		t.Errorf("%d flushes or better, want %d", flushes, 36+9*38)
	}
}

func BenchmarkNextCard(b *testing.B) {
	hole := mustHole(b, "AhKh")
	flop := mustBoard(b, "Qh7h2c")
	turn := mustBoard(b, "Qh7h2c9s")
	for b.Loop() {
		NextCard(hole, flop)
		NextCard(hole, turn)
	}
}
