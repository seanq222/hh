package hh

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
)

func samePlayerHand(a, b *PlayerHand) bool {
	return a.Site == b.Site && a.HandID == b.HandID && a.Time.Equal(b.Time) &&
		a.SmallBlind == b.SmallBlind && a.BigBlind == b.BigBlind &&
		a.Type == b.Type && a.Hero == b.Hero && a.Reached == b.Reached && a.Board == b.Board &&
		slices.Equal(a.Players, b.Players) && slices.Equal(a.Events, b.Events) &&
		a.Won == b.Won && a.Rake == b.Rake && a.WonEV == b.WonEV
}

// packSamples returns every view of the sample hands.
func packSamples(t *testing.T) []PlayerHand {
	t.Helper()
	var all []PlayerHand
	for i, s := range []string{sampleShowdown, sampleRealBombPot, sampleRunTwice, sampleWalkClientStyle, sampleAllInPreflop, sampleAllInThenSidePot} {
		h := parseSample(t, s)
		if i == 0 {
			h.Site = CoinPoker // so that the site is packed and read back too
		}
		views, err := h.PlayerHands()
		if err != nil {
			t.Fatal(err)
		}
		// Pack IDs for every position, some needing two bytes, so that they are
		// packed and read back too.
		for k := range views {
			for p := range views[k].Players {
				views[k].Players[p].PackID = uint16(p*150 + i)
			}
		}
		all = append(all, views...)
	}
	return all
}

func TestPackRoundTrip(t *testing.T) {
	want := packSamples(t)
	for _, codec := range []Codec{CodecNone, CodecFlate} {
		for _, size := range []int{1, 3, blockHands} { // one block per hand, several blocks, one block
			var buf bytes.Buffer
			pw, err := NewPackWriter(&buf, PackInfo{Player: "Wille Zurmacht", ID: 300}, codec)
			if err != nil {
				t.Fatal(err)
			}
			pw.size = size
			for i := range want {
				if err := pw.Write(&want[i]); err != nil {
					t.Fatal(err)
				}
			}
			if err := pw.Close(); err != nil {
				t.Fatal(err)
			}
			pr, err := NewPackReader(&buf)
			if err != nil {
				t.Fatal(err)
			}
			if pr.Player() != "Wille Zurmacht" || pr.Info() != (PackInfo{Player: "Wille Zurmacht", ID: 300}) {
				t.Errorf("player %q", pr.Player())
			}
			var got []PlayerHand
			for p, err := range pr.All() {
				if err != nil {
					t.Fatal(err)
				}
				got = append(got, p)
			}
			if len(got) != len(want) {
				t.Fatalf("codec %d size %d: %d hands, want %d", codec, size, len(got), len(want))
			}
			for i := range want {
				if !samePlayerHand(&got[i], &want[i]) {
					t.Errorf("codec %d size %d hand %d differs:\n got %+v\nwant %+v", codec, size, i, got[i], want[i])
				}
			}
		}
	}
}

// The views carry WonEV: Won for a hand not decided by an all-in runout, and the
// expected win of one that is.
func TestWonEV(t *testing.T) {
	for _, v := range packSamples(t) {
		net, adjusted := v.ExpectedNet()
		if math.Abs(float64(v.NetEV())-net) > 0.5 {
			t.Errorf("hand %d hero %d: NetEV %d, ExpectedNet %.4f", v.HandID, v.Hero, v.NetEV(), net)
		}
		if !adjusted && v.WonEV != v.Won {
			t.Errorf("hand %d hero %d: WonEV %d, Won %d", v.HandID, v.Hero, v.WonEV, v.Won)
		}
	}
}

// ScanPacks gives every hand of every pack in the folder once, with its player,
// in small blocks for both codecs.
func TestScanPacks(t *testing.T) {
	want := packSamples(t)
	dir := t.TempDir()
	for _, f := range []struct {
		player string
		codec  Codec
	}{{"Alice", CodecNone}, {"Bob", CodecFlate}} {
		var buf bytes.Buffer
		pw, _ := NewPackWriter(&buf, PackInfo{Player: f.player}, f.codec)
		pw.size = 2 // several blocks per file
		for i := range want {
			if err := pw.Write(&want[i]); err != nil {
				t.Fatal(err)
			}
		}
		if err := pw.Close(); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, f.player+".hhpk"), buf.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var mu sync.Mutex
	got := map[string][]PlayerHand{}
	err := ScanPacks(dir, func(player string, hands []PlayerHand) {
		mu.Lock()
		got[player] = append(got[player], slices.Clone(hands)...)
		mu.Unlock()
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, player := range []string{"Alice", "Bob"} {
		hands := got[player]
		if len(hands) != len(want) {
			t.Fatalf("%s: %d hands, want %d", player, len(hands), len(want))
		}
		// The blocks arrive in no set order; match the hands by ID and hero.
		for i := range want {
			found := false
			for j := range hands {
				if samePlayerHand(&hands[j], &want[i]) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("%s: hand %d hero %d missing", player, want[i].HandID, want[i].Hero)
			}
		}
	}
}

func TestPackEmpty(t *testing.T) {
	var buf bytes.Buffer
	pw, _ := NewPackWriter(&buf, PackInfo{Player: "nobody"}, CodecFlate)
	if err := pw.Close(); err != nil {
		t.Fatal(err)
	}
	pr, err := NewPackReader(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pr.Next(); err != io.EOF {
		t.Errorf("%v, want EOF", err)
	}
}

func TestPackCorrupt(t *testing.T) {
	hands := packSamples(t)
	var buf bytes.Buffer
	pw, _ := NewPackWriter(&buf, PackInfo{Player: "x"}, CodecNone)
	for i := range hands {
		pw.Write(&hands[i])
	}
	pw.Close()
	good := buf.Bytes()

	read := func(b []byte) error {
		pr, err := NewPackReader(bytes.NewReader(b))
		if err != nil {
			return err
		}
		for _, err := range pr.All() {
			if err != nil {
				return err
			}
		}
		return nil
	}
	if err := read(good); err != nil {
		t.Fatal(err)
	}
	if err := read([]byte("not a pack file")); !errors.Is(err, ErrCorrupt) {
		t.Errorf("bad magic: %v", err)
	}
	if err := read(good[:len(good)-5]); !errors.Is(err, ErrCorrupt) {
		t.Errorf("truncated: %v", err)
	}
	// Damaged bytes must give an error or different data, and never a panic.
	for i := 8; i < len(good); i++ {
		b := slices.Clone(good)
		b[i] ^= 0xff
		_ = read(b)
	}
}

func TestPackSize(t *testing.T) {
	hands := packSamples(t)
	var plain, packed bytes.Buffer
	for _, c := range []struct {
		buf   *bytes.Buffer
		codec Codec
	}{{&plain, CodecNone}, {&packed, CodecFlate}} {
		pw, _ := NewPackWriter(c.buf, PackInfo{Player: "x"}, c.codec)
		for i := range hands {
			pw.Write(&hands[i])
		}
		pw.Close()
	}
	t.Logf("%d hands: %d bytes packed (%.0f per hand), %d with flate", len(hands), plain.Len(), float64(plain.Len())/float64(len(hands)), packed.Len())
}

func TestRakePaid(t *testing.T) {
	views, err := parseSample(t, sampleRealBombPot).PlayerHands()
	if err != nil {
		t.Fatal(err)
	}
	// Player200 won the $57.00 pot after $3.00 of rake.
	var winner PlayerHand
	for _, v := range views {
		if v.Won > 0 {
			winner = v
		}
	}
	if got := winner.RakePaid(); got != 300 {
		t.Errorf("the winner paid the hand's rake, got %v", got)
	}
	// Had the pot been chopped, each half pays half.
	winner.Won = 2850
	if got := winner.RakePaid(); got != 150 {
		t.Errorf("a half share of the pot pays half the rake, got %v", got)
	}
	for _, v := range views {
		if v.Won == 0 && v.RakePaid() != 0 {
			t.Errorf("a loser paid rake: %v", v.RakePaid())
		}
	}
}

// The header holds the pack's ID, up to 65535, and whether it is anonymous; a
// header with an ID out of range or an unknown kind is refused.
func TestPackHeader(t *testing.T) {
	for _, info := range []PackInfo{{Player: "Alice"}, {Player: "AnonPlayer07", ID: 0xFFFF, Anon: true}, {Player: "Bob", ID: 128}} {
		var buf bytes.Buffer
		pw, err := NewPackWriter(&buf, info, CodecNone)
		if err != nil {
			t.Fatal(err)
		}
		pw.Close()
		pr, err := NewPackReader(&buf)
		if err != nil {
			t.Fatal(err)
		}
		if pr.Info() != info {
			t.Errorf("header %+v, want %+v", pr.Info(), info)
		}
	}
	hdr := func(id uint64, kind byte) []byte {
		b := append([]byte(packMagic), packVersion, byte(CodecNone), 1, 'x')
		return append(binary.AppendUvarint(b, id), kind)
	}
	for _, c := range []struct {
		name string
		b    []byte
	}{
		{"ID 65536", hdr(0x10000, 0)},
		{"unknown kind", hdr(5, 2)},
		{"no kind", hdr(5, 0)[:len(hdr(5, 0))-1]},
	} {
		if _, err := NewPackReader(bytes.NewReader(c.b)); err == nil {
			t.Errorf("%s: header accepted", c.name)
		}
	}
	if _, err := NewPackReader(bytes.NewReader(hdr(0xFFFF, 1))); err != nil {
		t.Errorf("ID 65535 refused: %v", err)
	}
}

// The footer holds the player's totals; a file without it, cut short, or with a
// count that is not the blocks' is refused.
func TestPackFooter(t *testing.T) {
	hands := packSamples(t)
	var want PackStats
	for i := range hands {
		want.Add(&hands[i])
	}
	if want.Hands != int64(len(hands)) || want.Rake == 0 || want.EVBB == 0 {
		t.Fatalf("totals %+v", want)
	}
	path := filepath.Join(t.TempDir(), "Alice.hhpk")
	if err := WritePackFile(path, PackInfo{Player: "Alice", ID: 7}, slices.Values(hands), CodecFlate); err != nil {
		t.Fatal(err)
	}
	s, err := ReadPackSummary(path)
	if err != nil || s != (PackSummary{PackInfo{Player: "Alice", ID: 7}, want}) {
		t.Fatalf("summary %+v, %v; want %+v", s, err, want)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	readAll := func(b []byte) (PackStats, error) {
		pr, err := NewPackReader(bytes.NewReader(b))
		if err != nil {
			return PackStats{}, err
		}
		for _, err := range pr.All() {
			if err != nil {
				return PackStats{}, err
			}
		}
		return pr.Stats(), nil
	}
	if got, err := readAll(b); err != nil || got != want {
		t.Fatalf("whole file: %+v, %v", got, err)
	}
	// The totals start after the 0 that ends the blocks; their first value is
	// the count of hands, a single byte here.
	n := int(binary.LittleEndian.Uint32(b[len(b)-footerTail:]))
	wrongCount := slices.Clone(b)
	wrongCount[len(b)-footerTail-n]++
	for name, c := range map[string][]byte{
		"no footer":   b[:len(b)-footerTail-n-1],
		"cut footer":  b[:len(b)-3],
		"wrong count": wrongCount,
		"extra bytes": append(slices.Clone(b), 0),
	} {
		if _, err := readAll(c); !errors.Is(err, ErrCorrupt) {
			t.Errorf("%s: %v, want ErrCorrupt", name, err)
		}
	}
}
