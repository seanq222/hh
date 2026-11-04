package hh

import (
	"reflect"
	"strings"
	"testing"
)

// The checks only look at a hand. Repair is the only thing that changes it.
func TestChecksDoNotChangeTheHand(t *testing.T) {
	samples := map[string]string{
		"showdown":         sampleShowdown,
		"bomb pot":         sampleRealBombPot,
		"run twice":        sampleRunTwice,
		"stale total pot":  sampleStaleTotalPot,
		"missing call":     sampleMissingCall,
		"acts twice":       sampleActsTwice,
		"acts after close": sampleActsAfterClose,
		"sitting out acts": sampleSittingOutActs,
		"wrong call":       sampleWrongCall,
		"blind twice":      sampleBlindPostedTwice,
		"cut off":          sampleCutOff,
	}
	for name, s := range samples {
		lines := strings.Split(strings.TrimSpace(s), "\n")
		h, err := ParseHandPartial(lines)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		want, _ := ParseHandPartial(lines)
		h.FailedCheck()
		h.CheckTurnOrder()
		h.CheckPosts()
		h.CheckReturn()
		h.CheckAmounts()
		h.ChipsConsistent()
		h.WinningsMatch()
		h.ReturnLineMissing()
		h.StreetsReplayable()
		h.PotLinesStable()
		if !reflect.DeepEqual(h, want) {
			t.Errorf("%s: a check changed the hand: %s", name, h.Diff(want))
		}
	}
}

// A pot line that is not the one the hand prints is found by the check and put
// right by FixPotLines.
func TestPotLinesAreCheckedAndFixed(t *testing.T) {
	s := strings.Replace(strings.TrimSpace(sampleShowdown), "Main pot $", "Main pot $9", 1)
	h, err := ParseHand(strings.Split(s, "\n"))
	if err != nil {
		t.Fatal(err)
	}
	if h.PotLinesStable() {
		t.Fatal("a stale pot line should not be stable")
	}
	h.FixPotLines()
	if !h.PotLinesStable() {
		t.Fatal("FixPotLines should make the lines stable")
	}
}

// The lines FixPotLines works out from the actions are the lines a reader gets
// from the written hand: write it, parse it, and the pot and rake are unchanged.
func TestPotLinesAreWhatIsReadBack(t *testing.T) {
	samples := map[string]string{
		"showdown":        sampleShowdown,
		"bomb pot":        sampleRealBombPot,
		"run twice":       sampleRunTwice,
		"stale total pot": sampleStaleTotalPot,
		"stale flop line": strings.Replace(strings.TrimSpace(sampleShowdown), "Main pot $", "Main pot $9", 1),
		"ante all in":     sampleAnteAllIn,
	}
	for name, s := range samples {
		h, err := ParseHand(strings.Split(strings.TrimSpace(s), "\n"))
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		h.FixPotLines()
		g := h.readBack()
		if g == nil {
			t.Errorf("%s: the written hand cannot be parsed", name)
			continue
		}
		if g.Pots != h.Pots || g.Rakes != h.Rakes {
			t.Errorf("%s: pots %v rakes %v, read back pots %v rakes %v", name, h.Pots, h.Rakes, g.Pots, g.Rakes)
		}
	}
}

// readBack writes the hand with String and parses the text, as a program that
// reads the written file would. It returns nil if the text cannot be parsed.
func (h *HandHistory) readBack() *HandHistory {
	var lines []string
	for _, l := range strings.Split(h.String(), "\n") {
		if l = strings.TrimRight(l, " \t\r"); l != "" {
			lines = append(lines, l)
		}
	}
	g, err := ParseHand(lines)
	if err != nil {
		return nil
	}
	return g
}
