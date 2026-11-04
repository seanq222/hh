// Package clean holds the checks and repairs that decide which hands are good
// enough to give Hand2Note, shared by the commands that write hands (hhclean) and
// that count them (hhpack), so that both keep the same hands in the same form.
//
// The order is always: parse, Check, and only if a check failed and repairs are
// wanted, Fix and Check again. A hand that still fails is dropped.
package clean

import (
	"bytes"
	"fmt"
	"slices"
	"strings"

	"github.com/seanq222/hh"
)

// Split returns the hands in a file's bytes: each starts at a line that begins
// "Hand #" (WPN) or "CoinPoker Hand #" and runs to the next. A byte-order mark at
// the start of the file, which the site's client writes, is skipped.
func Split(b []byte) [][]byte {
	b = bytes.TrimPrefix(b, []byte{0xEF, 0xBB, 0xBF})
	var starts []int
	for pos := 0; pos < len(b); {
		if bytes.HasPrefix(b[pos:], []byte("Hand #")) || bytes.HasPrefix(b[pos:], []byte("CoinPoker Hand #")) {
			starts = append(starts, pos)
		}
		nl := bytes.IndexByte(b[pos:], '\n')
		if nl < 0 {
			break
		}
		pos += nl + 1
	}
	hands := make([][]byte, len(starts))
	for i, s := range starts {
		end := len(b)
		if i+1 < len(starts) {
			end = starts[i+1]
		}
		hands[i] = b[s:end]
	}
	return hands
}

// Parse splits a hand's text into its non-blank lines and parses them, keeping a
// hand that stops before its summary (see hh.ParseHandPartial). reason says why
// the hand cannot be used at all, "" if it parsed.
func Parse(block []byte) (h *hh.HandHistory, reason string) {
	// One string for the whole hand: the lines are slices of it.
	text := string(block)
	// Another hand that lost its header is glued on: two summaries in one hand.
	if strings.Count(text, "*** SUMMARY ***") > 1 || strings.Count(text, "*** HOLE CARDS ***") > 1 {
		return nil, "the hand contains the lines of another hand"
	}
	lines := make([]string, 0, bytes.Count(block, []byte{'\n'})+1)
	for len(text) > 0 {
		l := text
		if nl := strings.IndexByte(text, '\n'); nl >= 0 {
			l, text = text[:nl], text[nl+1:]
		} else {
			text = ""
		}
		n := len(l)
		for n > 0 && (l[n-1] == ' ' || l[n-1] == '\t' || l[n-1] == '\r') {
			n--
		}
		if n > 0 {
			lines = append(lines, l[:n])
		}
	}
	h, err := hh.ParseHandPartial(lines)
	if err != nil {
		return nil, "could not be parsed"
	}
	return h, ""
}

// Fix repairs a hand that failed Check: Repair puts back actions the file left out,
// and settle corrects the faults that do not change what was played. The caller
// checks the hand again afterwards.
func Fix(h *hh.HandHistory) {
	h.Repair()
	settle(h)
}

// Check runs every check on the hand and returns what the first one that fails
// says, or "" if the hand passes them all. The checks are those of the package
// (see HandHistory.FailedCheck) and the rules that Hand2Note needs. The faults that
// settle corrects count as defects too, so that they are found. Check does not
// change the hand.
func Check(h *hh.HandHistory) string {
	if h.NoSummary {
		return "cut off before the summary"
	}
	if m := h.FailedCheck(); m != "" {
		return m
	}
	if m := quirk(h); m != "" {
		return m
	}
	if h.ReturnLineMissing() {
		return "the returned-bet line is missing though the pot is net of it"
	}
	// The pot and rake lines the hand has must be the ones it prints and a reader
	// reads back.
	if !h.PotLinesStable() {
		return "the pot and rake lines are not the ones the hand prints"
	}
	// A player who puts in more than the stack on the seat line was topped up. (One
	// who is all-in for less than it fails the package's CheckAmounts.)
	out, _ := outlays(h)
	for seat, c := range out {
		if p := h.PlayerBySeat(seat); p != nil && c > p.Stack {
			return "a player puts in more than the stack on the seat line"
		}
	}
	// A seat that is listed, not sitting out, but never acts and is not in the
	// summary.
	for i := range h.Players {
		if p := &h.Players[i]; !p.SittingOut && !p.InSummary && !p.Folded && !p.Showed && p.Won == 0 && out[p.Seat] == 0 && !seatActs(h, p.Seat) {
			return "a seat is listed but never acts and is not in the summary"
		}
	}
	return ""
}

// RenameAll replaces every player name in the hand, including the lines kept as
// text, by name(old).
func RenameAll(h *hh.HandHistory, name func(string) string) {
	for i := range h.Players {
		h.Players[i].Name = name(h.Players[i].Name)
	}
	for i := range h.Actions {
		h.Actions[i].Player = name(h.Actions[i].Player)
	}
	if h.Returned.Player != "" {
		h.Returned.Player = name(h.Returned.Player)
	}
	if h.Hero != "" {
		h.Hero = name(h.Hero)
	}
	// Lines kept as text: a player who sits out, or waits for the button.
	for i := range h.SitOutLines {
		h.SitOutLines[i].Text = renameLine(h.SitOutLines[i].Text, "", []string{" sitting out", " sits out", " waits for big blind"}, name)
	}
	for i := range h.Waiting {
		h.Waiting[i].Text = renameLine(h.Waiting[i].Text, fmt.Sprintf("Seat %d: ", h.Waiting[i].Seat), []string{" will be allowed to play after the button"}, name)
	}
}

// PlayerName is the name a player has once Fix has settled the hand: a name of
// digits only, which Hand2Note reads as a seat number, gets an underscore in
// front, and every other name stays as it is.
func PlayerName(s string) string {
	if allDigits(s) {
		return "_" + s
	}
	return s
}

// allDigits reports whether s is a name of digits only, which Hand2Note reads as
// a seat number.
func allDigits(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return s != ""
}

// renameLine replaces the player's name in a line of the form prefix + name +
// suffix.
func renameLine(line, prefix string, suffixes []string, name func(string) string) string {
	rest := strings.TrimPrefix(line, prefix)
	for _, s := range suffixes {
		if real, ok := strings.CutSuffix(rest, s); ok {
			return prefix + name(real) + s
		}
	}
	return line
}

// outlays returns what each seat put in over the hand, and which seats an action
// marks all-in.
func outlays(h *hh.HandHistory) (out map[int]hh.Cents, allIn map[int]bool) {
	out, allIn = map[int]hh.Cents{}, map[int]bool{}
	for i := range h.Actions {
		out[h.Actions[i].Seat] += h.Actions[i].Amount
		if h.Actions[i].AllIn {
			allIn[h.Actions[i].Seat] = true
		}
	}
	return out, allIn
}

// seatActs reports whether the seat has an action in the hand.
func seatActs(h *hh.HandHistory, seat int) bool {
	for i := range h.Actions {
		if h.Actions[i].Seat == seat {
			return true
		}
	}
	return false
}

// settle corrects the faults in a parsed hand that do not change what was played,
// so that the hand can pass every check. Each correction is made only if its fault
// is there: a name of digits only gets an underscore in front (Hand2Note reads it
// as a seat number); a stack smaller than what the player put in is raised to it,
// as is the stack of a player who is all-in (an automatic top-up the file did not
// record); a seat that never acts and has no summary line was not dealt in, so it
// sits out; and the returned-bet line that the file left out is added. What is
// wrong with the actions or the pot lines is for Repair. Last it puts the parts that
// only the summary carries into the form the written hand gives back, see
// canonical.
func settle(h *hh.HandHistory) {
	RenameAll(h, PlayerName)
	out, allIn := outlays(h)
	for i := range h.Players {
		p := &h.Players[i]
		// A player who is all-in put in the whole stack, whatever the seat line says;
		// no one can put in more than the stack.
		if c := out[p.Seat]; c > p.Stack || (allIn[p.Seat] && c > 0 && c != p.Stack) {
			p.Stack = c
		}
		if !p.SittingOut && !p.InSummary && !p.Folded && !p.Showed && p.Won == 0 && !seatActs(h, p.Seat) {
			p.SittingOut = true // nobody who is dealt in can end the hand without acting
		}
	}
	if h.ReturnLineMissing() {
		if seat, amt, ok := h.UncalledBet(); ok {
			if p := h.PlayerBySeat(seat); p != nil {
				h.Returned = hh.Return{Seat: seat, Player: p.Name, Amount: amt, Street: h.Actions[len(h.Actions)-1].Street}
			}
		}
	}
	// Last, the parts that only the summary carries, which the changes above and
	// the repair can alter.
	canonical(h)
}

// canonical puts the parts of the structure that only the summary carries into
// the form the text gives them back in, so that the structure and the structure
// read from its text can be identical. The summary has a line for every player who
// folded, showed cards or won, and for no one else, and only those lines carry a
// position. It changes no result.
func canonical(h *hh.HandHistory) {
	for i := range h.Players {
		if p := &h.Players[i]; p.Folded || p.Showed || p.Won > 0 {
			p.InSummary = true
		} else if !p.InSummary {
			p.Position = ""
		}
	}
}

// quirk returns the reason a hand is left out because Hand2Note is known to
// reject hands of that kind, though nothing in them is wrong in itself. The rules
// are deliberately a little wider than the faults seen, so that few errors are left.
func quirk(h *hh.HandHistory) string {
	// Heads-up with the button posting the big blind, as when the seat after the
	// button sits out. Hand2Note makes the small blind the dealer, so its positions
	// are wrong and its replay has the dealer act first after the flop.
	var dealt []int
	for i := range h.Actions {
		if s := h.Actions[i].Seat; !slices.Contains(dealt, s) {
			dealt = append(dealt, s)
		}
	}
	if len(dealt) == 2 {
		for i := range h.Actions {
			if h.Actions[i].Kind == hh.PostBigBlind && h.Actions[i].Seat == h.ButtonSeat {
				return "heads-up with the button posting the big blind"
			}
		}
	}
	// A name of digits only is read as a seat number.
	for i := range h.Players {
		if allDigits(h.Players[i].Name) {
			return "a player's name is all digits"
		}
	}
	// The same for a player who has no seat line, only a line that says they sit out
	// or wait.
	probe := func(line string) bool {
		for _, s := range []string{" sitting out", " sits out", " waits for big blind", " will be allowed to play after the button"} {
			if name, ok := strings.CutSuffix(line, s); ok {
				if _, rest, found := strings.Cut(name, ": "); found && strings.HasPrefix(name, "Seat ") {
					name = rest
				}
				return allDigits(name)
			}
		}
		return false
	}
	for i := range h.SitOutLines {
		if probe(h.SitOutLines[i].Text) {
			return "a player's name is all digits"
		}
	}
	for i := range h.Waiting {
		if probe(h.Waiting[i].Text) {
			return "a player's name is all digits"
		}
	}
	// An ante or a blind that takes a player's whole stack.
	posted := map[int]hh.Cents{}
	for i := range h.Actions {
		a := &h.Actions[i]
		switch a.Kind {
		case hh.PostAnte, hh.PostDead, hh.PostSmallBlind, hh.PostBigBlind, hh.PostEntry:
			posted[a.Seat] += a.Amount
		}
	}
	for i := range h.Players {
		if p := &h.Players[i]; p.Stack > 0 && posted[p.Seat] >= p.Stack {
			return "an ante or blind takes a player's whole stack"
		}
	}
	return ""
}
