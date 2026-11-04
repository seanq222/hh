package hh

import (
	"regexp"
	"strings"
)

// The client adds "with the kicker Jack" to the winner's summary line when the pot
// was decided by a kicker. The shows line does not have it. HandDescr is kept
// without the clause, and the clause is worked out from the hands.

var reKickerClause = regexp.MustCompile(` with the kicker \w+$`)

// stripKicker removes the kicker clause from a hand description.
func stripKicker(d string) string { return reKickerClause.ReplaceAllString(d, "") }

var rankWord = map[byte]string{'A': "Ace", 'K': "King", 'Q': "Queen", 'J': "Jack", 'T': "Ten",
	'9': "Nine", '8': "Eight", '7': "Seven", '6': "Six", '5': "Five", '4': "Four", '3': "Three", '2': "Two"}

// headOfDescr is the words of a description before its list of cards, such as
// "a pair of Queens", and the cards in the brackets.
func headOfDescr(d string) (head string, cards []string) {
	i := strings.Index(d, " [")
	if i < 0 || !strings.HasSuffix(d, "]") {
		return "", nil
	}
	return d[:i], strings.Fields(d[i+2 : len(d)-1])
}

// hasKickers reports whether a kind of hand is decided by kickers.
func hasKickers(head string) bool {
	for _, p := range []string{"a pair of", "two pair,", "three of a kind,", "four of a kind,", "a high card"} {
		if strings.HasPrefix(head, p) {
			return true
		}
	}
	return false
}

// kickerClause returns " with the kicker Jack" for a winner who beat another shown
// hand of the same kind by a kicker: the first of the five cards, in order, whose
// rank differs from the other hand's is named. It returns "" otherwise.
func (h *HandHistory) kickerClause(p *Seat) string {
	head, mine := headOfDescr(p.HandDescr)
	if head == "" || !hasKickers(head) {
		return ""
	}
	for i := range h.Players {
		o := &h.Players[i]
		if o == p || !o.Showed {
			continue
		}
		oh, theirs := headOfDescr(o.HandDescr)
		if oh != head {
			continue
		}
		for k := range mine {
			if k < len(theirs) && mine[k][0] != theirs[k][0] {
				return " with the kicker " + rankWord[mine[k][0]]
			}
		}
	}
	return ""
}
