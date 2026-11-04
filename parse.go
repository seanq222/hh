package hh

import (
	"bufio"
	"errors"
	"io"
	"math"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/seanq222/hh/pk"
)

var (
	reWaiting = regexp.MustCompile(`^Seat \d+: .+ will be allowed to play after the button$`)
	reSummSt  = regexp.MustCompile(`^folded on the (Pre-Flop|Flop|Turn|River)`)
	reShowed  = regexp.MustCompile(`^showed \[([^\]]*)\](?: and (?:won|collected) \$([\d.,]+)| and lost| and folded on the (Pre-Flop|Flop|Turn|River))?(?: with (.*))?$`)
	reWon     = regexp.MustCompile(`(?:won|collected) \$([\d.,]+)`)
)

// Reader reads consecutive hands from a hand history stream.
//
// Hands with a missing action are dropped, since the hand cannot be trusted.
// That means a live player who does not act when it is their turn, usually a
// missing call (see HandHistory.CheckTurnOrder), a blind or ante post that is
// missing (see HandHistory.CheckPosts), and a betting round that never closes because
// the call that ends it is missing (see HandHistory.StreetsReplayable), a returned bet that does not match the actions (see HandHistory.CheckReturn), and a pot line that does not match the actions (see HandHistory.ChipsConsistent). Before it decides, the reader
// reconstructs the missing actions that can be worked out, see
// HandHistory.Repair; set Repair to false to turn that off. Set
// KeepDefective before the first call to Next to receive the dropped hands
// anyway.
type Reader struct {
	// KeepDefective makes Next return defective hands instead of dropping them.
	KeepDefective bool

	// Repair makes Next reconstruct missing action lines before deciding whether
	// to drop a hand; see HandHistory.Repair. It is on by default; set it to
	// false before the first call to Next to drop such hands instead.
	Repair bool

	sc         *bufio.Scanner
	pending    string
	hasPending bool
	lineNo     int
	startLine  int
	dropped    int
	repaired   int
}

// Dropped returns how many defective hands Next has dropped so far.
func (r *Reader) Dropped() int { return r.dropped }

// Repaired returns how many missing actions Next has reconstructed so far.
func (r *Reader) Repaired() int { return r.repaired }

// NewReader returns a Reader over r.
func NewReader(r io.Reader) *Reader {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 4*1024*1024)
	return &Reader{sc: sc, Repair: true}
}

// Next returns the next hand, or io.EOF at the end of the input. A malformed
// hand yields a *ParseError; calling Next again continues with the next hand.
func (r *Reader) Next() (*HandHistory, error) {
	var block []string
	if r.hasPending {
		block = append(block, r.pending)
		r.hasPending = false
		r.startLine = r.lineNo
	}
	for r.sc.Scan() {
		line := strings.TrimRight(r.sc.Text(), " \t\r")
		r.lineNo++
		if r.lineNo == 1 {
			line = strings.TrimPrefix(line, "\ufeff")
		}
		if line == "" {
			continue
		}
		if IsHandStart(line) && len(block) > 0 {
			r.pending = line
			r.hasPending = true
			break
		}
		if len(block) == 0 {
			r.startLine = r.lineNo
		}
		block = append(block, line)
	}
	if err := r.sc.Err(); err != nil {
		return nil, err
	}
	if len(block) == 0 {
		return nil, io.EOF
	}
	h, err := parseHand(block, true)
	if err != nil {
		var pe *ParseError
		if errors.As(err, &pe) && pe.Line > 0 {
			pe.Line += r.startLine - 1
		}
		return nil, err
	}
	// Check, repair if a check failed, and check again.
	msg := h.FailedCheck()
	if msg != "" && r.Repair {
		r.repaired += h.Repair()
		msg = h.FailedCheck()
	}
	// A hand with no summary has no result; it is never returned.
	if msg != "" && (!r.KeepDefective || h.NoSummary) {
		r.dropped++
		return nil, &ParseError{
			HandID:    h.ID,
			Line:      r.startLine,
			Text:      block[0],
			Msg:       "dropped: " + msg,
			Defective: true,
		}
	}
	return h, nil
}

// FailedCheck runs the checks that make a hand trustworthy and returns what the
// first one that fails says, or "" if the hand passes them all.
func (h *HandHistory) FailedCheck() string {
	if h.NoSummary {
		return "hand has no summary, it is cut short"
	}
	if !h.RaiseAmountsMatch() {
		return "a raise adds other chips than its \"to\" figure says"
	}
	if !h.RaiseToMatches() {
		return "a raise's \"to\" figure is not what the bets add up to"
	}
	if !h.ShowdownMatches() {
		return "a showdown is marked but only one player is left"
	}
	if !h.SummaryNamesMatch() {
		return "the summary names a different player for a seat"
	}
	if !h.NamesUnique() {
		return "a player's name is at more than one seat"
	}
	if !h.StandardAnte() {
		return "the ante is above the table's, the file shows the largest ante as everyone's"
	}
	if len(h.Unknown) > 0 {
		return "a line the parser did not understand"
	}
	if !h.NoActsWithEmptyStack() {
		return "a player acts after putting their whole stack in"
	}
	if !h.NoLoneChecks() {
		return "a player checks with no one left to bet against"
	}
	if v := h.CheckTurnOrder(); v != TurnOK {
		return v.String()
	}
	if v := h.CheckPosts(); v != PostsOK {
		return v.String()
	}
	if !h.StreetsReplayable() {
		return "a betting round never closes, the call that ends it is missing"
	}
	if v := h.CheckReturn(); v != ReturnOK {
		return v.String()
	}
	if v := h.CheckAmounts(); v != AmountsOK {
		return v.String()
	}
	if !h.ChipsConsistent() {
		return "the pot lines do not match the actions"
	}
	if !h.WinnerPaid() {
		return "nobody is credited with winning the pot"
	}
	if !h.WinningsMatch() {
		return "what the winners were paid does not match the pot the actions make"
	}
	if !h.BoardWinnersMatch() {
		return "the best hand on a board won nothing from it"
	}
	return ""
}

// Stats counts what ParseFile did with the hands in a file.
type Stats struct {
	Hands     int // hands passed to the callback
	Dropped   int // defective hands skipped because an action is missing
	Malformed int // hands that could not be parsed at all
	Repaired  int // missing actions that were reconstructed, see HandHistory.Repair
}

// Add adds o to s.
func (s *Stats) Add(o Stats) {
	s.Hands += o.Hands
	s.Dropped += o.Dropped
	s.Malformed += o.Malformed
	s.Repaired += o.Repaired
}

// ParseFile calls fn for every hand in the file.
//
// Defective and malformed hands are passed to onErr (if non-nil) as a
// *ParseError and skipped; ParseError.Defective tells them apart. The returned
// Stats say how many of each there were. If fn returns an error, parsing stops.
func ParseFile(path string, fn func(*HandHistory) error, onErr func(error)) (Stats, error) {
	f, err := os.Open(path)
	if err != nil {
		return Stats{}, err
	}
	defer f.Close()
	return parseAll(NewReader(f), fn, onErr)
}

func parseAll(rd *Reader, fn func(*HandHistory) error, onErr func(error)) (Stats, error) {
	var st Stats
	for {
		h, err := rd.Next()
		st.Repaired = rd.Repaired()
		if err == io.EOF {
			return st, nil
		}
		if err != nil {
			var pe *ParseError
			if errors.As(err, &pe) {
				if pe.Defective {
					st.Dropped++
				} else {
					st.Malformed++
				}
				if onErr != nil {
					onErr(err)
				}
				continue
			}
			return st, err
		}
		st.Hands++
		if err := fn(h); err != nil {
			return st, err
		}
	}
}

// ParseHand parses the non-blank lines of a single hand.
func ParseHand(lines []string) (*HandHistory, error) { return parseHand(lines, false) }

// ParseHandPartial is ParseHand, but a hand that the file stops before its
// summary is returned, with NoSummary set, instead of being refused. Its result is
// not known; FailedCheck reports it and Repair may work it out from the actions.
func ParseHandPartial(lines []string) (*HandHistory, error) { return parseHand(lines, true) }

// IsHandStart reports whether the line is the first line of a hand, for any site.
func IsHandStart(line string) bool {
	return strings.HasPrefix(line, "Hand #") || strings.HasPrefix(line, "CoinPoker Hand #")
}

// parseHand gives the hand to the parser for its site, known by its first line.
// Each site writes its own format; they fill the same HandHistory.
func parseHand(lines []string, partial bool) (*HandHistory, error) {
	if len(lines) == 0 {
		return nil, &ParseError{Msg: "empty hand"}
	}
	if strings.HasPrefix(lines[0], "CoinPoker Hand #") {
		return parseCoinPoker(lines, partial)
	}
	return parseWPN(lines, partial)
}

// parseWPN parses a hand in the format of the Winning Poker Network (ACR).
func parseWPN(lines []string, partial bool) (*HandHistory, error) {
	h := &HandHistory{}
	fail := func(i int, msg string) error {
		t := ""
		if i < len(lines) {
			t = lines[i]
		}
		return &ParseError{HandID: h.ID, Line: i + 1, Text: t, Msg: msg}
	}

	hm, ok := parseHeaderLine(lines[0])
	if !ok {
		return nil, fail(0, "bad hand header")
	}
	h.ID, _ = strconv.ParseInt(hm[1], 10, 64)
	h.Game = strings.TrimSpace(hm[2])
	h.Limit = hm[3]
	h.SmallBlind, _ = parseMoney(hm[4])
	h.BigBlind, _ = parseMoney(hm[5])
	t, err := time.Parse("2006/01/02 15:04:05", hm[6])
	if err != nil {
		return nil, fail(0, "bad timestamp")
	}
	h.Time = t

	if len(lines) < 2 {
		return nil, fail(1, "missing table line")
	}
	tm, ok := parseTableLine(lines[1], false)
	if !ok {
		return nil, fail(1, "bad table line")
	}
	h.Table = tm[1]
	h.MaxSeats, _ = strconv.Atoi(tm[2])
	h.ButtonSeat, _ = strconv.Atoi(tm[3])

	i := 2
	for ; i < len(lines); i++ {
		num, name, amount, sittingOut, ok := parseSeatLine(lines[i], " ($", ")")
		if !ok {
			if reWaiting.MatchString(lines[i]) {
				// a player waiting for the button: seated, not dealt in
				num, _, _ := strings.Cut(strings.TrimPrefix(lines[i], "Seat "), ":")
				seat, _ := strconv.Atoi(num)
				h.Waiting = append(h.Waiting, WaitingSeat{Seat: seat, Text: lines[i]})
				continue
			}
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

	// Try longer names first so "Bob K" wins over "Bob". A name at two seats goes
	// to the first of them.
	byLen := make([]*Seat, len(h.Players))
	for k := range h.Players {
		byLen[k] = &h.Players[k]
	}
	sort.SliceStable(byLen, func(a, b int) bool { return len(byLen[a].Name) > len(byLen[b].Name) })
	findPrefix := func(s string) (*Seat, string) {
		for _, p := range byLen {
			if len(s) > len(p.Name) && s[len(p.Name)] == ' ' && strings.HasPrefix(s, p.Name) {
				return p, s[len(p.Name)+1:]
			}
		}
		return nil, ""
	}

	street := Preflop
	streetStart := 0 // len(h.Actions) when the current street began
	var sidePotSeen [4]map[string]bool
	for s := range sidePotSeen {
		sidePotSeen[s] = map[string]bool{}
	}
	inSummary := false
	holeSeen := false
	for ; i < len(lines); i++ {
		line := lines[i]

		if strings.HasPrefix(line, "*** ") {
			switch {
			case strings.HasPrefix(line, "*** HOLE CARDS ***"):
				holeSeen = true
			case strings.HasPrefix(line, "*** FLOP "): // "*** FLOP 1 ***" when the board is run twice before the flop
				street, streetStart = Flop, len(h.Actions)
				if err := addBoard(h, line, 0); err != nil {
					return nil, fail(i, err.Error())
				}
			case strings.HasPrefix(line, "*** RUN IT TWICE ***"):
				h.RunTwice = true
			case strings.HasPrefix(line, "*** TURN "): // "*** TURN ***", or "*** TURN 2 ***" in a second run
				street, streetStart = Turn, len(h.Actions)
				if err := addBoard(h, line, 1); err != nil {
					return nil, fail(i, err.Error())
				}
			case strings.HasPrefix(line, "*** RIVER "):
				street, streetStart = River, len(h.Actions)
				if err := addBoard(h, line, 1); err != nil {
					return nil, fail(i, err.Error())
				}
			case strings.HasPrefix(line, "*** SHOW DOWN"): // also "*** SHOW DOWN 1 ***"
				h.Showdown = true
			case strings.HasPrefix(line, "*** SUMMARY ***"):
				inSummary = true
			default:
				h.Unknown = append(h.Unknown, line)
			}
			continue
		}

		// Some files leave out the "*** SUMMARY ***" line; the summary then starts
		// with its total pot line, which appears nowhere else.
		if !inSummary && strings.HasPrefix(line, "Total pot ") {
			inSummary = true
		}
		if inSummary {
			parseSummaryLine(h, line)
			continue
		}

		switch {
		case line == "BombPot":
			h.Type = Bomb // until the antes are checked
			continue
		case strings.HasPrefix(line, "Main pot "):
			// "Main pot $X | Rake $Y" gives the pot, with rake, as the street began.
			// Only a line straight after the street marker says that. A hand that
			// ends on a street prints the final pot there, after the actions.
			pot, rake, _ := strings.Cut(line[len("Main pot "):], " | Rake ")
			if mp, ok := parseMoney(pot); ok && h.Pots[street] == 0 && len(h.Actions) == streetStart {
				rk, _ := parseMoney(rake)
				h.Pots[street], h.Rakes[street] = mp+rk, rk
			}
			continue
		case strings.HasPrefix(line, "Side pot"):
			// "Side pot(1) $X | Rake $Y": the client lists each pot on its own
			// line, so the pot as the street began is the main pot plus these.
			if end := strings.Index(line, ") "); end > 0 && h.Pots[street] != 0 && len(h.Actions) == streetStart {
				id := line[len("Side pot("):end]
				if !sidePotSeen[street][id] {
					pot, rake, _ := strings.Cut(line[end+2:], " | Rake ")
					if sp, ok := parseMoney(pot); ok {
						rk, _ := parseMoney(rake)
						h.Pots[street] += sp + rk
						h.Rakes[street] += rk
						sidePotSeen[street][id] = true
					}
				}
			}
			continue
		case strings.HasSuffix(line, " sitting out"), strings.HasSuffix(line, " sits out"), strings.HasSuffix(line, " waits for big blind"):
			// "NAME sitting out" for a player who is not dealt in. A player who sits
			// out when the blinds are due moves them on to the next player, so the
			// blinds are no longer posted by the seats the button implies.
			h.sitOut = true
			h.SitOutLines = append(h.SitOutLines, SitOutLine{Text: line, At: len(h.Actions), AfterHole: holeSeen})
			continue
		case strings.HasPrefix(line, "Uncalled bet ("):
			// Uncalled bet ($X) returned to NAME
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
		case strings.HasPrefix(line, "Dealt to "):
			rest := line[len("Dealt to "):]
			if p, _ := findPrefix(rest); p != nil {
				if g := bracketGroups(rest); len(g) > 0 {
					if hc, err := parseHole(g[0]); err == nil {
						p.Cards, p.HasCards = hc, true
						h.Hero = p.Name
						continue
					}
				}
			}
			h.Unknown = append(h.Unknown, line)
			continue
		}

		p, rest := findPrefix(line)
		if p == nil {
			h.Unknown = append(h.Unknown, line)
			continue
		}
		if strings.HasPrefix(rest, "collected $") || rest == "does not show" {
			// "NAME collected $X from main pot" and "NAME does not show" during
			// the showdown section.
			continue
		}
		if strings.HasPrefix(rest, "shows ") {
			g := bracketGroups(rest)
			if len(g) == 0 {
				h.Unknown = append(h.Unknown, line)
				continue
			}
			if err := setShown(p, g[0]); err != nil {
				return nil, fail(i, "bad shown cards")
			}
			continue
		}
		if kind, amt, to, allIn, ok := parseAction(rest); ok {
			h.Actions = append(h.Actions, Action{Street: street, Seat: p.Seat, Player: p.Name, Kind: kind, Amount: amt, To: to, AllIn: allIn})
			continue
		}
		h.Unknown = append(h.Unknown, line)
	}
	if !inSummary {
		// A file that stops mid-hand has no winner, so nobody's result is known.
		if !partial {
			return nil, &ParseError{HandID: h.ID, Line: len(lines), Msg: "hand has no summary, it is cut short", Defective: true}
		}
		h.NoSummary = true
	}
	switch ante := h.hasAnte(); {
	case !ante:
		h.Type = Regular // the label without antes marks an ordinary hand
	case h.Type != Bomb:
		h.Type = Ante
	}
	return h, nil
}

// UncalledBet works out, from the betting actions alone, which player gets a
// bet back and how much. The file's own returned-bet line (HandHistory.Returned) is
// not used. The amount is 0 when every bet was called.
//
// On the final street the highest bet is uncalled to the extent that it exceeds
// the next highest, and that part goes back to the bettor. ok is false when the
// actions cannot be right: a player who has not folded or gone all-in has less
// in than the top bet, so a call is missing.
func (h *HandHistory) UncalledBet() (seat int, amount Cents, ok bool) {
	if len(h.Actions) == 0 {
		return 0, 0, false
	}
	last := h.Actions[len(h.Actions)-1].Street
	var own, invested [32]Cents // each seat's bet on the final street (antes aside), and in all
	var folded, allIn, seen [32]bool
	for i := range h.Actions {
		a := &h.Actions[i]
		if a.Seat < 0 || a.Seat >= len(own) {
			return 0, 0, false
		}
		seen[a.Seat] = true
		invested[a.Seat] += a.Amount
		if a.Kind == Fold {
			folded[a.Seat] = true
		}
		if a.AllIn {
			allIn[a.Seat] = true
		}
		if a.Street != last {
			continue
		}
		own[a.Seat] += LiveAmount(h.Site, a.Kind, a.Amount, h.BigBlind)
	}
	top, second, topSeat := Cents(0), Cents(0), -1
	for s, v := range own {
		switch {
		case v > top:
			second, top, topSeat = top, v, s
		case v > second:
			second = v
		}
	}
	for s := range seen {
		if !seen[s] || s == topSeat || folded[s] || allIn[s] || own[s] >= top {
			continue
		}
		if pi := h.seatIndex(s); pi >= 0 && h.Players[pi].Stack > 0 && invested[s] >= h.Players[pi].Stack {
			continue // all their chips are in
		}
		return 0, 0, false // this player still owes a call
	}
	if topSeat < 0 {
		return 0, 0, true
	}
	return topSeat, top - second, true
}

// playersStillIn counts players who posted or acted and never folded. Seats
// that are listed but never act were not dealt in (the file does not always
// mark them as sitting out), so they are not counted.
func playersStillIn(h *HandHistory) int {
	n := 0
	for i := range h.Players {
		p := &h.Players[i]
		if p.SittingOut {
			continue
		}
		active, folded := false, false
		for j := range h.Actions {
			a := &h.Actions[j]
			if a.Seat != p.Seat {
				continue
			}
			active = true
			if a.Kind == Fold {
				folded = true
				break
			}
		}
		if active && !folded {
			n++
		}
	}
	return n
}

// parseAction parses the text following a player name.
func parseAction(rest string) (kind ActionKind, amt, to Cents, allIn, ok bool) {
	if s, found := strings.CutSuffix(rest, " and is all-in"); found {
		rest, allIn = s, true
	}
	switch {
	case rest == "folds":
		return Fold, 0, 0, allIn, true
	case rest == "checks":
		return Check, 0, 0, allIn, true
	case strings.HasPrefix(rest, "calls "):
		amt, ok = parseMoney(rest[len("calls "):])
		return Call, amt, 0, allIn, ok
	case strings.HasPrefix(rest, "bets "):
		amt, ok = parseMoney(rest[len("bets "):])
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
	case strings.HasPrefix(rest, "posts the small blind "):
		amt, ok = parseMoney(rest[len("posts the small blind "):])
		return PostSmallBlind, amt, 0, allIn, ok
	case strings.HasPrefix(rest, "posts the big blind "):
		amt, ok = parseMoney(rest[len("posts the big blind "):])
		return PostBigBlind, amt, 0, allIn, ok
	case strings.HasPrefix(rest, "posts ante "):
		amt, ok = parseMoney(rest[len("posts ante "):])
		return PostAnte, amt, 0, allIn, ok
	case strings.HasPrefix(rest, "posts dead "):
		amt, ok = parseMoney(rest[len("posts dead "):])
		return PostDead, amt, 0, allIn, ok
	case strings.HasPrefix(rest, "posts $"):
		// A blind posted out of turn or on entering the game.
		amt, ok = parseMoney(rest[len("posts "):])
		return PostEntry, amt, 0, allIn, ok
	}
	return 0, 0, 0, false, false
}

// parseSummaryLine handles one line after "*** SUMMARY ***". Lines it does not
// understand are recorded in h.Unknown.
func parseSummaryLine(h *HandHistory, line string) {
	switch {
	case strings.HasPrefix(line, "Total pot "):
		pot, rake, _ := strings.Cut(line[len("Total pot "):], " | Rake ")
		var ok bool
		if h.TotalPot, ok = parseMoney(pot); !ok {
			h.Unknown = append(h.Unknown, line)
			return
		}
		if rake != "" {
			// The rake may be followed by " | JP Fee $X", a jackpot fee that
			// also comes out of the pot.
			rake, jp, _ := strings.Cut(rake, " | JP Fee ")
			h.Rake, _ = parseMoney(rake)
			if jp != "" {
				h.JPFee, _ = parseMoney(jp)
			}
		}
	case strings.HasPrefix(line, "Board 2 ["):
		h.second = true
		if g := bracketGroups(line); len(g) > 0 {
			if b, err := parseCards(g[0]); err == nil {
				h.Board2 = b
				break
			}
		}
		h.Unknown = append(h.Unknown, line)
	case strings.HasPrefix(line, "Board: ["), strings.HasPrefix(line, "Board ["), strings.HasPrefix(line, "Board 1 ["):
		g := bracketGroups(line)
		if len(g) == 0 {
			h.Unknown = append(h.Unknown, line)
			return
		}
		b, err := parseCards(g[0])
		if err != nil {
			h.Unknown = append(h.Unknown, line)
			return
		}
		if len(b) >= len(h.Board) {
			h.Board = b
		}
	case line == "BombPot":
		h.Type = Bomb
	case strings.HasPrefix(line, "Seat "):
		parseSummarySeat(h, line)
	default:
		h.Unknown = append(h.Unknown, line)
	}
}

func parseSummarySeat(h *HandHistory, line string) {
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

	// Position labels in parentheses, e.g. "(button) ".
	for strings.HasPrefix(rest, "(") {
		end := strings.IndexByte(rest, ')')
		if end < 0 {
			break
		}
		label := rest[1:end]
		if label != "" && label != "button" && label != "small blind" && label != "big blind" {
			break
		}
		if p.Position == "" && label != "" {
			p.Position = label
		}
		rest = strings.TrimPrefix(rest[end+1:], " ")
	}

	switch {
	case reSummSt.MatchString(rest):
		p.Folded = true
		p.FoldedOn = streetNamed(reSummSt.FindStringSubmatch(rest)[1])
	case strings.HasPrefix(rest, "showed "):
		m := reShowed.FindStringSubmatch(rest)
		if m == nil {
			h.Unknown = append(h.Unknown, line)
			return
		}
		_ = setShown(p, m[1])
		if m[2] != "" {
			addWon(h, p, m[2])
		}
		if m[3] != "" { // showed cards after folding
			p.Folded, p.FoldedOn = true, streetNamed(m[3])
		}
		if h.second {
			p.HandDescr2 = stripKicker(m[4])
		} else {
			p.HandDescr = stripKicker(m[4])
		}
	default:
		// "did not show", with or without "and won $X", or "collected $X".
		if m := reWon.FindStringSubmatch(rest); m != nil {
			addWon(h, p, m[1])
		} else if !strings.HasPrefix(rest, "did not show") && rest != "" {
			h.Unknown = append(h.Unknown, line)
		}
	}
}

// addWon records what the summary says a player won. In a hand that was run
// twice the summary has a section for each board, and the winnings add up.
func addWon(h *HandHistory, p *Seat, amount string) {
	won, _ := parseMoney(amount)
	if h.second {
		p.WonSecond = won
		p.Won += won
	} else {
		p.Won = won
	}
}

// streetNamed converts the street name the summary uses.
func streetNamed(name string) Street {
	switch name {
	case "Pre-Flop":
		return Preflop
	case "Flop":
		return Flop
	case "Turn":
		return Turn
	}
	return River
}

// addBoard handles a street marker. Flop lines carry the three flop cards in
// bracket group 0. Turn and river lines repeat the earlier board in group 0 and
// carry the new card in group 1.
func addBoard(h *HandHistory, line string, idx int) error {
	g := bracketGroups(line)
	if idx >= len(g) {
		return errors.New("missing board cards")
	}
	cs, err := parseCards(g[idx])
	if err != nil {
		return errors.New("bad board cards")
	}
	// A board for a second run means the hand was run twice, whether or not a
	// "*** RUN IT TWICE ***" line said so; older files have none.
	if strings.HasPrefix(line, "*** FLOP 2 ") || strings.HasPrefix(line, "*** TURN 2 ") || strings.HasPrefix(line, "*** RIVER 2 ") {
		h.RunTwice = true
	}
	switch {
	case idx == 0 && strings.HasPrefix(line, "*** FLOP 2 "):
		h.Board2 = cs
	case idx == 0:
		h.Board = cs
	case strings.HasPrefix(line, "*** TURN 2 "):
		// The second run has its own flop if the board was run twice before it,
		// and shares the first run's flop otherwise.
		if len(h.Board2) < 3 && len(h.Board) >= 3 {
			h.Board2 = append(pk.Board(nil), h.Board[:3]...)
		}
		h.Board2 = append(h.Board2, cs...)
	case strings.HasPrefix(line, "*** RIVER 2 "):
		if len(h.Board2) == 0 && len(h.Board) >= 4 { // run after a shared turn
			h.Board2 = append(pk.Board(nil), h.Board[:4]...)
		}
		h.Board2 = append(h.Board2, cs...)
	default:
		h.Board = append(h.Board, cs...)
	}
	return nil
}

// bracketGroups returns the text inside each [...] group that is not nested
// inside parentheses.
func bracketGroups(s string) []string {
	var out []string
	paren := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '(':
			paren++
		case ')':
			if paren > 0 {
				paren--
			}
		case '[':
			if paren == 0 {
				if end := strings.IndexByte(s[i:], ']'); end > 0 {
					out = append(out, s[i+1:i+end])
					i += end
				}
			}
		}
	}
	return out
}

func parseCards(s string) (pk.Board, error) {
	f := strings.Fields(s)
	b := make(pk.Board, 0, len(f))
	for _, w := range f {
		c, err := pk.ParseCard(w)
		if err != nil {
			return nil, err
		}
		b = append(b, c)
	}
	return b, nil
}

// setShown records cards a player revealed. Both cards are shown as "Td Qd".
// When only one is revealed the other appears as "-", as in "- Jc".
func setShown(p *Seat, s string) error {
	p.Showed = true
	if hc, err := parseHole(s); err == nil {
		p.Cards, p.HasCards = hc, true
		return nil
	}
	f := strings.Fields(s)
	if len(f) == 2 && (f[0] == "-") != (f[1] == "-") {
		c := f[0]
		if c == "-" {
			c = f[1]
		}
		card, err := pk.ParseCard(c)
		if err != nil {
			return err
		}
		p.PartialCard, p.HasPartial, p.PartialFirst = card, true, f[0] != "-"
		return nil
	}
	if len(f) == 2 && f[0] == "-" && f[1] == "-" {
		return nil
	}
	return errors.New("bad shown cards")
}

// parseHole reads two cards, "Ah Kd" or "AhKd", and keeps them in the order given.
func parseHole(s string) ([2]pk.Card, error) {
	s = strings.Join(strings.Fields(s), "")
	if len(s) != 4 {
		return [2]pk.Card{}, errors.New("invalid hole cards string")
	}
	a, err := pk.ParseCard(s[:2])
	if err != nil {
		return [2]pk.Card{}, err
	}
	b, err := pk.ParseCard(s[2:])
	if err != nil {
		return [2]pk.Card{}, err
	}
	if a == b {
		return [2]pk.Card{}, errors.New("duplicate hole cards")
	}
	return [2]pk.Card{a, b}, nil
}

// parseMoney parses "$1,234.56", "₮0.50", "12.5" or "7" into cents.
func parseMoney(s string) (Cents, bool) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(strings.TrimPrefix(s, "$"), "₮")
	s = strings.ReplaceAll(s, ",", "")
	if s == "" {
		return 0, false
	}
	whole, frac, _ := strings.Cut(s, ".")
	var w int64
	if whole != "" {
		var err error
		if w, err = strconv.ParseInt(whole, 10, 64); err != nil {
			return 0, false
		}
	}
	var f int64
	if frac != "" {
		if len(frac) == 1 {
			frac += "0"
		}
		var err error
		if f, err = strconv.ParseInt(frac[:2], 10, 64); err != nil {
			return 0, false
		}
	}
	c := w*100 + f
	if c > math.MaxInt32 {
		return 0, false
	}
	return Cents(c), true
}
