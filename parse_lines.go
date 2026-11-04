package hh

import "strings"

// The header, table and seat lines start every hand, so they are parsed by hand
// rather than with regular expressions, which took a third of the parse time.
// Each function accepts exactly the lines its pattern in the comment accepts and
// returns the same groups.

// isDigits reports whether s is one or more ASCII digits.
func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// isAmount reports whether s matches [\d.,]+.
func isAmount(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if c := s[i]; (c < '0' || c > '9') && c != '.' && c != ',' {
			return false
		}
	}
	return true
}

// isWord reports whether s matches \w+.
func isWord(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && c != '_' {
			return false
		}
	}
	return true
}

// amountPrefix returns the length of the [\d.,]* run at the start of s.
func amountPrefix(s string) int {
	n := 0
	for n < len(s) && (s[n] >= '0' && s[n] <= '9' || s[n] == '.' || s[n] == ',') {
		n++
	}
	return n
}

// cutDateTail splits "... - 2006/01/02 15:04:05" + tail off the end of s,
// returning what comes before " - " and the date, if s ends that way.
func cutDateTail(s, tail string) (before, date string, ok bool) {
	s, ok = strings.CutSuffix(s, tail)
	if !ok || len(s) < 19+3 {
		return "", "", false
	}
	date = s[len(s)-19:]
	for i := 0; i < 19; i++ {
		c := date[i]
		switch i {
		case 4, 7:
			ok = c == '/'
		case 10:
			ok = c == ' '
		case 13, 16:
			ok = c == ':'
		default:
			ok = c >= '0' && c <= '9'
		}
		if !ok {
			return "", "", false
		}
	}
	before, ok = strings.CutSuffix(s[:len(s)-19], " - ")
	return before, date, ok
}

// cutSeatID splits "Seat N: " off the front of s.
func cutSeatID(s string) (num, rest string, ok bool) {
	s, ok = strings.CutPrefix(s, "Seat ")
	if !ok {
		return "", "", false
	}
	num, rest, ok = strings.Cut(s, ": ")
	if !ok || !isDigits(num) {
		return "", "", false
	}
	return num, rest, true
}

// parseHeaderLine matches reHeader:
//
//	^Hand #(\d+) - (.+?) ?\((.+?)\) - \$([\d.,]+)/\$([\d.,]+) - (\d{4}/\d\d/\d\d \d\d:\d\d:\d\d) UTC$
//
// and returns id, game, limit, small blind, big blind and date.
func parseHeaderLine(s string) (m [7]string, ok bool) {
	s, ok = strings.CutPrefix(s, "Hand #")
	if !ok {
		return m, false
	}
	id, s, ok := strings.Cut(s, " - ")
	if !ok || !isDigits(id) {
		return m, false
	}
	s, date, ok := cutDateTail(s, " UTC")
	if !ok {
		return m, false
	}
	// s is now "GAME ?(LIMIT) - $SB/$BB"; the blinds cannot hold "$" or "/".
	k := strings.LastIndex(s, "/$")
	if k < 0 || !isAmount(s[k+2:]) {
		return m, false
	}
	bb := s[k+2:]
	s = s[:k]
	k = strings.LastIndex(s, ") - $")
	if k < 0 || !isAmount(s[k+5:]) {
		return m, false
	}
	sb := s[k+5:]
	s = s[:k] // "GAME ?(LIMIT"
	// The game is as short as it can be: it ends at the first "(" after its first
	// character, a space before the "(" going with neither; the limit is the rest
	// and must not be empty.
	for g := 1; g < len(s); g++ {
		p := g
		if s[p] == ' ' && p+1 < len(s) && s[p+1] == '(' {
			p++
		}
		if s[p] != '(' || p+1 >= len(s) {
			continue
		}
		return [7]string{"", id, s[:g], s[p+1:], sb, bb, date}, true
	}
	return m, false
}

// parseTableLine matches reTable, ^(.+) (\d+)-max Seat #(\d+) is the button$, or
// with quoted, reCPTable, ^Table '(.*)' (\d+)-max Seat #(\d+) is the button$, and
// returns the table, the seats and the button seat.
func parseTableLine(s string, quoted bool) (m [4]string, ok bool) {
	s, ok = strings.CutSuffix(s, " is the button")
	if !ok {
		return m, false
	}
	k := strings.LastIndex(s, "-max Seat #")
	if k < 0 || !isDigits(s[k+11:]) {
		return m, false
	}
	button := s[k+11:]
	s = s[:k]
	sep := " "
	if quoted {
		if s, ok = strings.CutPrefix(s, "Table '"); !ok {
			return m, false
		}
		sep = "' "
	}
	// The table name is as long as it can be, so the seats are the digits after
	// the last separator.
	k = strings.LastIndex(s, sep)
	if k < 0 || !isDigits(s[k+len(sep):]) || (!quoted && k == 0) {
		return m, false
	}
	return [4]string{"", s[:k], s[k+len(sep):], button}, true
}

// parseSeatLine matches reSeat, ^Seat (\d+): (.+) \(\$([\d.,]+)\)( is sitting out)?$,
// with open " ($" and close ")", and reCPSeat, the same with " (₮" and
// " in chips)". It returns the seat, the name, the stack and whether the player
// is sitting out.
func parseSeatLine(s, open, close string) (seat, name, stack string, sittingOut, ok bool) {
	seat, s, ok = cutSeatID(s)
	if !ok {
		return
	}
	s, sittingOut = strings.CutSuffix(s, " is sitting out")
	if s, ok = strings.CutSuffix(s, close); !ok {
		return
	}
	// The name is as long as it can be; the stack cannot hold the "(".
	k := strings.LastIndex(s, open)
	if k <= 0 || !isAmount(s[k+len(open):]) {
		return "", "", "", false, false
	}
	return seat, s[:k], s[k+len(open):], sittingOut, true
}

// parseCPHeaderLine matches reCPHeader:
//
//	^CoinPoker Hand #(\d+): (.+?) \(₮([\d.,]+)/₮([\d.,]+)(?: - Ante ₮[\d.,]+)?(?: \w+)?\) - (\d{4}/\d\d/\d\d \d\d:\d\d:\d\d) (ET|[+-]\d\d)$
//
// and returns id, game, small blind, big blind, date and zone.
func parseCPHeaderLine(s string) (m [7]string, ok bool) {
	s, ok = strings.CutPrefix(s, "CoinPoker Hand #")
	if !ok {
		return m, false
	}
	id, s, ok := strings.Cut(s, ": ")
	if !ok || !isDigits(id) {
		return m, false
	}
	var zone string
	if k := strings.LastIndexByte(s, ' '); k >= 0 {
		zone = s[k+1:]
		if !(zone == "ET" || len(zone) == 3 && (zone[0] == '+' || zone[0] == '-') && isDigits(zone[1:])) {
			return m, false
		}
	} else {
		return m, false
	}
	s, date, ok := cutDateTail(s[:len(s)-len(zone)], " ")
	if !ok {
		return m, false
	}
	s, ok = strings.CutSuffix(s, ")")
	if !ok {
		return m, false
	}
	// The game is as short as it can be: try each " (₮" after its first character.
	for from := 1; from < len(s); {
		k := strings.Index(s[from:], " (₮")
		if k < 0 {
			break
		}
		g := from + k
		if sb, bb, ok := cpBlinds(s[g+len(" (₮"):]); ok {
			return [7]string{"", id, s[:g], sb, bb, date, zone}, true
		}
		from = g + 1
	}
	return m, false
}

// cpBlinds matches ([\d.,]+)/₮([\d.,]+)(?: - Ante ₮[\d.,]+)?(?: \w+)? against all
// of s.
func cpBlinds(s string) (sb, bb string, ok bool) {
	n := amountPrefix(s)
	if n == 0 {
		return "", "", false
	}
	sb, s = s[:n], s[n:]
	if s, ok = strings.CutPrefix(s, "/₮"); !ok {
		return "", "", false
	}
	n = amountPrefix(s)
	if n == 0 {
		return "", "", false
	}
	bb, s = s[:n], s[n:]
	if rest, ok := strings.CutPrefix(s, " - Ante ₮"); ok {
		if n = amountPrefix(rest); n > 0 {
			if r := rest[n:]; r == "" || r[0] == ' ' && isWord(r[1:]) {
				return sb, bb, true
			}
		}
	}
	if s == "" || s[0] == ' ' && isWord(s[1:]) {
		return sb, bb, true
	}
	return "", "", false
}
