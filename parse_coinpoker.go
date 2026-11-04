package hh

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata" // CoinPoker's older files give the time in US Eastern time

	"github.com/seanq222/hh/pk"
)

// CoinPoker hand histories are PokerStars-style text
// in USDT (₮). parseCoinPoker reads one into the same HandHistory as the WPN
// parser; only the way the lines are written differs:
//
//   - the header names the site, the game in words and the ante, and gives the
//     time in US Eastern time ("ET") or as an offset from UTC ("+00");
//   - the table name is quoted and a seat's stack says "in chips";
//   - an action line puts a colon after the name ("Bob: folds");
//   - "raises ₮4 to ₮5" means raised by ₮4 to a total of ₮5, so the chips added
//     follow from the second figure, not the first;
//   - there is no pot line as each street begins;
//   - a hand run twice has FIRST and SECOND streets, showdowns and boards, and
//     the summary gives only each player's total; what a player won on the second
//     board is the "collected" line of the second showdown.
var (
	reCPShowed = regexp.MustCompile(`^showed \[([^\]]*)\] and (?:won \(₮([\d.,]+)\)|lost)(?: with ?(.*))?$`)
)

// cpLimits are the betting structures a CoinPoker header can name after the game.
var cpLimits = []string{"No Limit", "Pot Limit", "Fixed Limit", "Limit"}

var easternTime = func() *time.Location {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		panic(err)
	}
	return loc
}()

func parseCoinPoker(lines []string, partial bool) (*HandHistory, error) {
	h := &HandHistory{Site: CoinPoker}
	fail := func(i int, msg string) error {
		t := ""
		if i < len(lines) {
			t = lines[i]
		}
		return &ParseError{HandID: h.ID, Line: i + 1, Text: t, Msg: msg}
	}

	hm, ok := parseCPHeaderLine(lines[0])
	if !ok {
		return nil, fail(0, "bad hand header")
	}
	h.ID, _ = strconv.ParseInt(hm[1], 10, 64)
	h.Game = hm[2]
	for _, l := range cpLimits {
		if g, ok := strings.CutSuffix(hm[2], " "+l); ok {
			h.Game, h.Limit = g, l
			break
		}
	}
	h.SmallBlind, _ = parseMoney(hm[3])
	h.BigBlind, _ = parseMoney(hm[4])
	loc := easternTime
	if hm[6] != "ET" {
		off, _ := strconv.Atoi(hm[6])
		loc = time.FixedZone(hm[6], off*3600)
	}
	t, err := time.ParseInLocation("2006/01/02 15:04:05", hm[5], loc)
	if err != nil {
		return nil, fail(0, "bad timestamp")
	}
	h.Time = t.UTC()

	if len(lines) < 2 {
		return nil, fail(1, "missing table line")
	}
	tm, ok := parseTableLine(lines[1], true)
	if !ok {
		return nil, fail(1, "bad table line")
	}
	h.Table = tm[1]
	h.MaxSeats, _ = strconv.Atoi(tm[2])
	h.ButtonSeat, _ = strconv.Atoi(tm[3])

	i := 2
	for ; i < len(lines); i++ {
		num, name, amount, sittingOut, ok := parseSeatLine(lines[i], " (₮", " in chips)")
		if !ok {
			break
		}
		seat, _ := strconv.Atoi(num)
		stack, ok := parseMoney(amount)
		if !ok {
			return nil, fail(i, "bad stack")
		}
		h.Players = append(h.Players, Seat{Seat: seat, Name: name, Stack: stack, SittingOut: sittingOut})
	}
	if len(h.Players) < 2 {
		return nil, fail(i, "fewer than two seats")
	}

	// An action line is "NAME: action" and a few others start "NAME ". Longer
	// names are tried first so that "Bob K" wins over "Bob"; a name at two seats
	// goes to the first of them.
	byLen := make([]*Seat, len(h.Players))
	for k := range h.Players {
		byLen[k] = &h.Players[k]
	}
	sort.SliceStable(byLen, func(a, b int) bool { return len(byLen[a].Name) > len(byLen[b].Name) })
	findName := func(s string, sep byte) (*Seat, string) {
		for _, p := range byLen {
			if len(s) > len(p.Name) && s[len(p.Name)] == sep && strings.HasPrefix(s, p.Name) {
				return p, strings.TrimPrefix(s[len(p.Name)+1:], " ")
			}
		}
		return nil, ""
	}

	street := Preflop
	var own [32]Cents // each seat's bet on the current street, antes aside
	run := 0          // 1 or 2 inside the showdown of a hand run twice
	inSummary := false
	bigBlindPosted := false
	for ; i < len(lines); i++ {
		line := lines[i]
		if line == "*** SUMMARY ***" {
			inSummary = true
			continue
		}
		if inSummary {
			parseCPSummaryLine(h, line)
			continue
		}
		switch {
		case line == "*** HOLE CARDS ***":
			continue
		case strings.HasPrefix(line, "*** "):
			if !cpStreetMarker(h, line, &street, &run) {
				h.Unknown = append(h.Unknown, line)
			}
			own = [32]Cents{}
			continue
		case strings.HasPrefix(line, "Uncalled bet ("):
			// Uncalled bet (₮X) returned to NAME
			end := strings.Index(line, ") returned to ")
			if end < 0 {
				h.Unknown = append(h.Unknown, line)
				continue
			}
			amt, ok := parseMoney(line[len("Uncalled bet ("):end])
			p := h.PlayerByName(line[end+len(") returned to "):])
			if !ok || p == nil {
				h.Unknown = append(h.Unknown, line)
				continue
			}
			h.Returned = Return{Seat: p.Seat, Player: p.Name, Amount: amt, Street: street}
			continue
		}
		if p, rest := findName(line, ' '); p != nil {
			if amt, found := strings.CutPrefix(rest, "collected "); found {
				// "NAME collected ₮X from pot": the summary has the total; in the
				// second showdown of a hand run twice it is what the second board paid.
				amt, _, _ = strings.Cut(amt, " from ")
				won, ok := parseMoney(amt)
				if !ok {
					h.Unknown = append(h.Unknown, line)
				} else if run == 2 {
					p.WonSecond += won
				}
				continue
			}
		}
		p, rest := findName(line, ':')
		if p == nil {
			h.Unknown = append(h.Unknown, line)
			continue
		}
		switch {
		case rest == "doesn't show hand" || rest == "mucks hand":
			// Who showed, mucked or won without a showdown follows from the cards
			// shown and the summary.
			continue
		case strings.HasPrefix(rest, "shows "):
			// "shows [] (Straight)" is a player who mucks at the showdown: the
			// hand is described but the cards are not shown.
			g := bracketGroups(rest)
			if len(g) == 0 || (strings.TrimSpace(g[0]) != "" && setShown(p, g[0]) != nil) {
				return nil, fail(i, "bad shown cards")
			}
			if open := strings.LastIndex(rest, " ("); open > 0 && strings.HasSuffix(rest, ")") {
				d := rest[open+2 : len(rest)-1]
				if run == 2 {
					p.HandDescr2 = d
				} else {
					p.HandDescr = d
				}
			}
			continue
		}
		kind, amt, to, allIn, ok := parseCPAction(rest)
		if !ok {
			h.Unknown = append(h.Unknown, line)
			continue
		}
		// A player joining the game posts a big blind out of turn, and CoinPoker
		// writes that as a second "posts big blind". It is what WPN writes as
		// "posts $X", PostEntry: live, but not the big blind the turn order counts
		// from.
		if kind == PostBigBlind {
			if bigBlindPosted {
				kind = PostEntry
			}
			bigBlindPosted = true
		}
		if p.Seat < 0 || p.Seat >= len(own) {
			return nil, fail(i, "seat number out of range")
		}
		// CoinPoker sometimes writes an all-in call for less than the bet as a raise
		// by ₮0.00 to the player's total, "raises ₮0.00 to ₮199.68 and is all-in",
		// and sometimes as "calls". It is kept as the raise the file says, one that
		// does not go above the bet, so that the hand is written again as it was;
		// the checks treat such a raise as a call.
		switch kind {
		case PostSmallBlind, PostBigBlind, PostStraddle, Call, Bet:
			own[p.Seat] += amt
		case PostEntry:
			own[p.Seat] += LiveAmount(CoinPoker, kind, amt, h.BigBlind)
		case Raise:
			amt = to - own[p.Seat] // the chips added: the first figure is the size of the raise
			own[p.Seat] = to
		}
		h.Actions = append(h.Actions, Action{Street: street, Seat: p.Seat, Player: p.Name, Kind: kind, Amount: amt, To: to, AllIn: allIn})
	}
	if !inSummary {
		if !partial {
			return nil, &ParseError{HandID: h.ID, Line: len(lines), Msg: "hand has no summary, it is cut short", Defective: true}
		}
		h.NoSummary = true
	}
	if h.hasAnte() {
		h.Type = Ante
	}
	return h, nil
}

// cpStandardAntes are the antes CoinPoker's tables have, in hundredths of a big
// blind: none, 0.10 BB (late 2025), 0.16 BB (2026) and 0.25 BB (from October
// 2026).
var cpStandardAntes = []Cents{0, 10, 16, 25}

// StandardAnte reports whether the hand's ante is one its site's tables have. It
// is false for a CoinPoker hand whose ante is above the table's: a player paid an
// extra ante, and the file shows the largest ante paid as everyone's, in the
// header and on every ante line, as the players' stacks show. Who paid the extra is
// not in the file. A hand from another site always passes.
func (h *HandHistory) StandardAnte() bool {
	if h.Site != CoinPoker || h.BigBlind == 0 {
		return true
	}
	var ante Cents
	for i := range h.Actions {
		if h.Actions[i].Kind == PostAnte {
			ante = max(ante, h.Actions[i].Amount)
		}
	}
	for _, a := range cpStandardAntes {
		if ante*100 == h.BigBlind*a {
			return true
		}
	}
	return false
}

// cpStreetMarker handles a "*** ... ***" line before the summary and reports
// whether it knew it. A street marker's bracket groups together are the board so
// far; FIRST and SECOND markers are the two runs of a hand run twice.
func cpStreetMarker(h *HandHistory, line string, street *Street, run *int) bool {
	name, _, _ := strings.Cut(strings.TrimPrefix(line, "*** "), " ***")
	board := &h.Board
	switch {
	case strings.HasPrefix(name, "FIRST "):
		name, h.RunTwice = name[len("FIRST "):], true
		*run = 1
	case strings.HasPrefix(name, "SECOND "):
		name, h.RunTwice, board = name[len("SECOND "):], true, &h.Board2
		*run = 2
	}
	switch name {
	case "SHOW DOWN":
		h.Showdown = true
		return true
	case "FLOP":
		*street = Flop
	case "TURN":
		*street = Turn
	case "RIVER":
		*street = River
	default:
		return false
	}
	var cards pk.Board
	for _, g := range bracketGroups(line) {
		cs, err := parseCards(g)
		if err != nil {
			return false
		}
		cards = append(cards, cs...)
	}
	*board = cards
	return true
}

// parseCPAction parses what follows "NAME: ". For a raise amt is the first
// figure, the size of the raise; the caller works out the chips added.
func parseCPAction(rest string) (kind ActionKind, amt, to Cents, allIn, ok bool) {
	if s, found := strings.CutSuffix(rest, " and is all-in"); found {
		rest, allIn = s, true
	}
	money := func(prefix string) (Cents, bool) { return parseMoney(rest[len(prefix):]) }
	switch {
	case rest == "folds":
		return Fold, 0, 0, allIn, true
	case rest == "checks":
		return Check, 0, 0, allIn, true
	case strings.HasPrefix(rest, "calls "):
		amt, ok = money("calls ")
		return Call, amt, 0, allIn, ok
	case strings.HasPrefix(rest, "bets "):
		amt, ok = money("bets ")
		return Bet, amt, 0, allIn, ok
	case strings.HasPrefix(rest, "raises "):
		a, b, found := strings.Cut(rest[len("raises "):], " to ")
		if !found {
			return 0, 0, 0, false, false
		}
		var ok1, ok2 bool
		amt, ok1 = parseMoney(a)
		to, ok2 = parseMoney(b)
		return Raise, amt, to, allIn, ok1 && ok2
	case strings.HasPrefix(rest, "posts the ante "):
		amt, ok = money("posts the ante ")
		return PostAnte, amt, 0, allIn, ok
	case strings.HasPrefix(rest, "posts small blind "):
		amt, ok = money("posts small blind ")
		return PostSmallBlind, amt, 0, allIn, ok
	case strings.HasPrefix(rest, "posts big blind "):
		amt, ok = money("posts big blind ")
		return PostBigBlind, amt, 0, allIn, ok
	case strings.HasPrefix(rest, "posts straddle "):
		amt, ok = money("posts straddle ")
		return PostStraddle, amt, 0, allIn, ok
	}
	return 0, 0, 0, false, false
}

// parseCPSummaryLine handles one line after "*** SUMMARY ***". Lines it does not
// understand are recorded in h.Unknown.
func parseCPSummaryLine(h *HandHistory, line string) {
	switch {
	case strings.HasPrefix(line, "Total pot "):
		pot, rake, _ := strings.Cut(line[len("Total pot "):], " | Rake ")
		var ok1, ok2 bool
		h.TotalPot, ok1 = parseMoney(pot)
		h.Rake, ok2 = parseMoney(rake)
		if !ok1 || !ok2 {
			h.Unknown = append(h.Unknown, line)
		}
	case line == "Hand was run twice":
		h.RunTwice = true
	case strings.HasPrefix(line, "Board ["), strings.HasPrefix(line, "FIRST Board ["), strings.HasPrefix(line, "SECOND Board ["):
		g := bracketGroups(line)
		var b pk.Board
		var err error
		if len(g) > 0 {
			b, err = parseCards(g[0])
		}
		if len(g) == 0 || err != nil {
			h.Unknown = append(h.Unknown, line)
			return
		}
		if strings.HasPrefix(line, "SECOND ") {
			h.Board2 = b
		} else if len(b) >= len(h.Board) {
			h.Board = b
		}
	case strings.HasPrefix(line, "Seat "):
		parseCPSummarySeat(h, line)
	default:
		h.Unknown = append(h.Unknown, line)
	}
}

func parseCPSummarySeat(h *HandHistory, line string) {
	num, rest, ok := strings.Cut(line[len("Seat "):], ": ")
	seat, err := strconv.Atoi(num)
	if !ok || err != nil {
		h.Unknown = append(h.Unknown, line)
		return
	}
	p := h.PlayerBySeat(seat)
	if p == nil || (rest != p.Name && !strings.HasPrefix(rest, p.Name+" ")) {
		h.StaleSummary = append(h.StaleSummary, line)
		return
	}
	rest = strings.TrimPrefix(rest[len(p.Name):], " ")
	p.InSummary = true
	// Position labels; heads-up the button has two, "(button) (small blind)".
	for strings.HasPrefix(rest, "(") {
		end := strings.IndexByte(rest, ')')
		label := ""
		if end > 0 {
			label = rest[1:end]
		}
		if label != "button" && label != "small blind" && label != "big blind" {
			break
		}
		if p.Position == "" {
			p.Position = label
		}
		rest = strings.TrimPrefix(rest[end+1:], " ")
	}
	switch {
	case rest == "folded before Flop" || rest == "folded before Flop (didn't bet)":
		p.Folded, p.FoldedOn = true, Preflop
	case strings.HasPrefix(rest, "folded on the "):
		st := rest[len("folded on the "):]
		if st != "Flop" && st != "Turn" && st != "River" {
			h.Unknown = append(h.Unknown, line)
			return
		}
		p.Folded, p.FoldedOn = true, streetNamed(st)
	case rest == "mucked":
	case strings.HasPrefix(rest, "collected (₮") && strings.HasSuffix(rest, ")"):
		won, ok := parseMoney(rest[len("collected (₮") : len(rest)-1])
		if !ok {
			h.Unknown = append(h.Unknown, line)
			return
		}
		p.Won = won
	case strings.HasPrefix(rest, "showed "):
		m := reCPShowed.FindStringSubmatch(rest)
		if m == nil || setShown(p, m[1]) != nil {
			h.Unknown = append(h.Unknown, line)
			return
		}
		if m[2] != "" {
			p.Won, _ = parseMoney(m[2])
		}
		if m[3] != "" {
			p.HandDescr = m[3]
		}
	default:
		h.Unknown = append(h.Unknown, line)
	}
}
