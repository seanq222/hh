package pk

import (
	"math"
	"testing"

	"github.com/seanq222/hh/internal/float"
)

func TestEquity(t *testing.T) {
	a0 := CardFromRS(Ace, Diamond)
	a1 := CardFromRS(Ace, Heart)
	b0 := CardFromRS(King, Diamond)
	b1 := CardFromRS(King, Club)

	win, draw, total := Equity(a0, a1, b0, b1)
	if total != _48Choose5 {
		t.Errorf("total = %d, want %d", total, _48Choose5)
	}

	e := (float64(win) + float64(draw)/2) / float64(total)
	t.Logf("equity: %f", e)

	n := 10000000
	win, draw, total = EquitySample(n, a0, a1, b0, b1)
	if total != n {
		t.Errorf("total = %d, want %d", total, n)
	}

	ee := (float64(win) + float64(draw)/2) / float64(total)
	t.Logf("equity: %f", ee)

	if !float.EqualApprox(e, ee, 0.001) {
		t.Logf("equity match within tolerance: %f ~ %f", e, ee)
	}
}

func TestEquityEntireRange(t *testing.T) {
	a0 := CardFromRS(Eight, Spade)
	a1 := CardFromRS(Seven, Spade)

	win, draw, total := EquityEntireRange(a0, a1)
	e := (float64(win) + float64(draw)/2) / float64(total)
	t.Logf("equity: %f", e)

	n := 1000 * 1000 * 4

	win, draw, total = EquityRangeSample(n, a0, a1)
	if total != n {
		t.Errorf("total = %d, want %d", total, n)
	}
	ee := (float64(win) + float64(draw)/2) / float64(total)
	t.Logf("equity: %f", ee)

	if !float.EqualApprox(e, ee, 0.001) {
		t.Logf("equity match within tolerance: %f ~ %f", e, ee)
	}
}

func BenchmarkEquityRangeSample(b *testing.B) {
	a0 := CardFromRS(Eight, Spade)
	a1 := CardFromRS(Seven, Spade)

	for b.Loop() {
		EquityRangeSample(1000000, a0, a1)
	}
}

func BenchmarkEquityEntireRange(b *testing.B) {
	a0 := CardFromRS(Eight, Spade)
	a1 := CardFromRS(Seven, Spade)

	for b.Loop() {
		EquityEntireRange2(a0, a1)
	}
}

func BenchmarkEquitySample(b *testing.B) {
	a0 := CardFromRS(Eight, Spade)
	a1 := CardFromRS(Seven, Spade)
	b0 := CardFromRS(King, Diamond)
	b1 := CardFromRS(King, Club)

	for b.Loop() {
		EquitySample(1000000, a0, a1, b0, b1)
	}
}

func TestEquityPostFlop(t *testing.T) {
	var Range2p, Range5p Range

	Range2p.Parse("QQ+,AKs,AKo")
	Range5p.Parse("99+,AKs,AKo,AQs,KQs,AQo")

	flop, err := ParseFlop("5h3d8s")
	if err != nil {
		t.Fatalf("failed to parse flop: %v", err)
	}

	oop_e, _ := EquityPostFlop(flop, &Range2p, &Range5p)
	if math.Abs(float64(oop_e)-0.630887) > 0.0001 {
		t.Errorf("oop equity = %f, want 0.630887", oop_e)
	}
}

func TestEquityPostFlop2(t *testing.T) {
	var Range2p, Range5p Range

	Range2p.Parse("AA")
	Range5p.Parse("KK")

	flop, err := ParseFlop("5h3d8s")
	if err != nil {
		t.Fatalf("failed to parse flop: %v", err)
	}

	oop_e, _ := EquityPostFlop(flop, &Range2p, &Range5p)
	if math.Abs(float64(oop_e)-0.916162) > 0.0001 {
		t.Errorf("oop equity = %f, want 0.630887", oop_e)
	}
}

func BenchmarkEquity(b *testing.B) {
	hands := DrawHands(4, 10000)
	idx := 0
	for b.Loop() {
		c := hands[idx]
		Equity(c[0], c[1], c[2], c[3])
		idx++
		if idx == len(hands) {
			idx = 0
		}
	}
}
