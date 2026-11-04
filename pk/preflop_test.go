package pk

import (
	"math/rand"
	"testing"
)

// The table has one record for each of the 47,008 matchups, sorted, and each
// record's key is in its standard form.
func TestPreflopTableShape(t *testing.T) {
	n := len(preflopTable) / preflopRecord
	if n != 47008 || len(preflopTable)%preflopRecord != 0 {
		t.Fatalf("%d bytes, %d records, want 47008", len(preflopTable), n)
	}
	prev := uint32(0)
	for i := 0; i < n; i++ {
		r := preflopTable[i*preflopRecord:]
		key := get24(r)
		if i > 0 && key <= prev {
			t.Fatalf("record %d is out of order", i)
		}
		prev = key
		a0, a1, b0, b1 := PreflopKeyCards(key)
		if k, swapped := PreflopKey(a0, a1, b0, b1); k != key || swapped {
			t.Fatalf("record %d: key %06x is not in its standard form", i, key)
		}
		if w, d := get24(r[3:]), get24(r[6:]); w+d > _48Choose5 {
			t.Fatalf("record %d: %d wins and %d draws", i, w, d)
		}
	}
}

// The table gives what counting does, either way round.
func TestPreflopEquity(t *testing.T) {
	n := 40
	if testing.Short() {
		n = 5
	}
	rng := rand.New(rand.NewSource(1))
	for range n {
		p := rng.Perm(52)
		a0, a1, b0, b1 := Card(p[0]), Card(p[1]), Card(p[2]), Card(p[3])
		w, d, tot := Equity(a0, a1, b0, b1)
		gw, gd, gt := PreflopEquity(a0, a1, b0, b1)
		if gw != w || gd != d || gt != tot {
			t.Errorf("%v %v v %v %v: table %d %d %d, counted %d %d %d", a0, a1, b0, b1, gw, gd, gt, w, d, tot)
		}
		if rw, rd, _ := PreflopEquity(b0, b1, a0, a1); rw != tot-w-d || rd != d {
			t.Errorf("%v %v v %v %v the other way round: %d wins %d draws, want %d %d", a0, a1, b0, b1, rw, rd, tot-w-d, d)
		}
	}
}

func BenchmarkPreflopEquity(b *testing.B) {
	hands := DrawHands(4, 10000)
	idx := 0
	for b.Loop() {
		c := hands[idx]
		PreflopEquity(c[0], c[1], c[2], c[3])
		idx = (idx + 1) % len(hands)
	}
}
