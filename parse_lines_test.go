package hh

import (
	"regexp"
	"strings"
	"testing"
)

// The patterns the hand-written line parsers replace. The parsers must accept
// exactly the lines these match and return the same groups.
var (
	reHeader   = regexp.MustCompile(`^Hand #(\d+) - (.+?) ?\((.+?)\) - \$([\d.,]+)/\$([\d.,]+) - (\d{4}/\d\d/\d\d \d\d:\d\d:\d\d) UTC$`)
	reTable    = regexp.MustCompile(`^(.+) (\d+)-max Seat #(\d+) is the button$`)
	reSeat     = regexp.MustCompile(`^Seat (\d+): (.+) \(\$([\d.,]+)\)( is sitting out)?$`)
	reCPHeader = regexp.MustCompile(`^CoinPoker Hand #(\d+): (.+?) \(₮([\d.,]+)/₮([\d.,]+)(?: - Ante ₮[\d.,]+)?(?: \w+)?\) - (\d{4}/\d\d/\d\d \d\d:\d\d:\d\d) (ET|[+-]\d\d)$`)
	reCPTable  = regexp.MustCompile(`^Table '(.*)' (\d+)-max Seat #(\d+) is the button$`)
	reCPSeat   = regexp.MustCompile(`^Seat (\d+): (.+) \(₮([\d.,]+) in chips\)( is sitting out)?$`)
)

var lineSeeds = []string{
	"Hand #1000001 - Holdem(No Limit) - $0.25/$0.50 - 2023/01/01 00:00:00 UTC",
	"Hand #1000001 - Holdem (No Limit) - $0.25/$0.50 - 2026/01/01 00:00:00 UTC",
	"Hand #1 - Omaha (Pot Limit) (x) - $1,000.00/$2,000.00 - 2026/01/01 00:00:00 UTC",
	"Hand #1 - (a)(b) - $1/$2 - 2026/01/01 00:00:00 UTC",
	"Hand #1 - Holdem () - $1/$2 - 2026/01/01 00:00:00 UTC",
	"Hand #1 - Holdem(No Limit) - $0.25/$0.50 - 2026/1/01 00:00:00 UTC",
	"Hand #1 - Holdem(No Limit) - $/$0.50 - 2026/01/01 00:00:00 UTC",
	"Hand #x - Holdem(No Limit) - $0.25/$0.50 - 2026/01/01 00:00:00 UTC",
	"TableOne 6-max Seat #6 is the button",
	"Table 1 2 9-max Seat #10 is the button",
	"T 6-max Seat #x is the button",
	" 6-max Seat #1 is the button",
	"Seat 4: Alice ($27.95)",
	"Seat 4: Alice ($27.95) is sitting out",
	"Seat 4: Al ($1) ice ($27.95)",
	"Seat 4: Alice is sitting out ($3)",
	"Seat 4:  ($3)",
	"Seat 4: ($3)",
	"Seat 4: Alice ($)",
	"Seat 12: Bob K: x ($1,000.00)",
	"Seat 2: Bob will be allowed to play after the button",
	"CoinPoker Hand #123: Hold'em No Limit (₮0.50/₮1.00) - 2026/05/15 10:00:00 +00",
	"CoinPoker Hand #123: Hold'em No Limit (₮0.50/₮1.00 - Ante ₮0.10) - 2025/10/22 10:00:00 ET",
	"CoinPoker Hand #123: Hold'em No Limit (₮0.50/₮1.00 - Ante ₮0.10 Bomb) - 2025/10/22 10:00:00 -05",
	"CoinPoker Hand #123: Hold'em No Limit (₮0.50/₮1.00 Straddle) - 2025/10/22 10:00:00 ET",
	"CoinPoker Hand #123: A (₮1/₮2) B (₮0.50/₮1.00) - 2025/10/22 10:00:00 ET",
	"CoinPoker Hand #123: A (₮1/₮2 - Ante ₮) - 2025/10/22 10:00:00 ET",
	"CoinPoker Hand #123: A (₮1/₮2) - 2025/10/22 10:00:00 EST",
	"Table 'Main 3' 6-max Seat #2 is the button",
	"Table '' 6-max Seat #2 is the button",
	"Table 'it's' 6-max' 9-max Seat #2 is the button",
	"Seat 1: Alice (₮100.00 in chips)",
	"Seat 1: Alice (₮100.00 in chips) is sitting out",
}

// checkLine compares one line's hand-written parses with the patterns.
func checkLine(t *testing.T, s string) {
	t.Helper()
	same := func(what string, want []string, got []string, ok bool) {
		t.Helper()
		if (want != nil) != ok {
			t.Errorf("%s %q: pattern matches %v, parser %v", what, s, want != nil, ok)
			return
		}
		for i := 1; ok && i < len(want); i++ {
			if want[i] != got[i] {
				t.Errorf("%s %q: group %d is %q, parser gives %q", what, s, i, want[i], got[i])
			}
		}
	}
	hm, ok := parseHeaderLine(s)
	same("header", reHeader.FindStringSubmatch(s), hm[:], ok)
	tm, ok := parseTableLine(s, false)
	same("table", reTable.FindStringSubmatch(s), tm[:], ok)
	num, name, amount, out, ok := parseSeatLine(s, " ($", ")")
	same("seat", reSeat.FindStringSubmatch(s), []string{"", num, name, amount, map[bool]string{true: " is sitting out"}[out]}, ok)
	cm, ok := parseCPHeaderLine(s)
	same("CoinPoker header", reCPHeader.FindStringSubmatch(s), cm[:], ok)
	tm, ok = parseTableLine(s, true)
	same("CoinPoker table", reCPTable.FindStringSubmatch(s), tm[:], ok)
	num, name, amount, out, ok = parseSeatLine(s, " (₮", " in chips)")
	same("CoinPoker seat", reCPSeat.FindStringSubmatch(s), []string{"", num, name, amount, map[bool]string{true: " is sitting out"}[out]}, ok)
}

func TestLineParsersMatchPatterns(t *testing.T) {
	for _, s := range lineSeeds {
		checkLine(t, s)
	}
	for _, text := range allSamples {
		for _, line := range strings.Split(text, "\n") {
			checkLine(t, line)
		}
	}
}

func FuzzLineParsers(f *testing.F) {
	for _, s := range lineSeeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		checkLine(t, s)
	})
}
