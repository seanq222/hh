# hh: guide for Claude Code

This file is for a Claude Code session opened in this repository. It says what the
code does, how to run it on someone's hand histories, and how to add a new
statistic. Read it before doing anything; the package documentation
(`go doc github.com/seanq222/hh`) and each command's doc comment have the details.

## What this is

A Go package and set of commands for No-Limit Hold'em hand histories:

1. **Parse** hand history text into a `HandHistory`: the Winning Poker Network
   (ACR) format, from the site's own client or from other sources, and CoinPoker.
2. **Check** each hand for defects (missing actions or blinds, a betting round
   that never closes, chips that do not add up, wrong winners) and optionally
   **repair** the ones that can be worked out.
3. Build **player views** (`PlayerHand`): one hand as one player saw it, with
   their position, cards, the actions up to their last one, and what they won,
   both actual and EV (all-in luck taken out).
4. Write the views to **pack files**, one `.hhpk` per player, fast to scan.
5. Compute **statistics** over the packs: win rates, red/blue/green lines,
   3-bet frequencies, and whatever the user asks for next. A scan of every pack
   takes a few seconds, so answer a new question by writing a small command and
   running it over all the data, not by sampling.

It can also write cleaned copies of the hand histories for Hand2Note (`hhclean`).

## Setup

- Go 1.27 or later. Check with `go version`.
- Cards, hand evaluation and equity are in the `pk` package of this module.
  The only outside dependency is `golang.org/x/exp`, which `go` downloads.
- Build every command into `bin/`, then run the tests:

```bash
go build -o bin/ ./cmd/...
```

```bash
go vet ./... && go test ./...
```

## The user's hand histories

They can be anywhere: a command takes folders, `.txt` files and `.zip` archives,
including zips inside zips, and reads them without unpacking (`hh.WalkText`). A
tidy layout is one folder holding everything, for example `~/hh/hands/`, with
packs written next to it in `~/hh/packs/`.

The same hand often appears in several files (the user's own client and other
sources). `hhpack` keeps one copy of each hand, the best one: a copy that
passes the checks, then the one showing more hole cards.

**Never put hand histories, packs or real player names into this repository.**
Work on them where they are; write output outside the repo.

## Common tasks

Ask the user where their hands are, then:

| Task | Command |
|---|---|
| Do the files parse? Counts, errors, lines not understood | `bin/hhcheck PATH...` |
| Build packs from everything | `bin/hhpack -repair -out PACKS PATH...` |
| Win rates, actual and EV, in bb/100 | `bin/hhwinrate [-min N] [-n N] PACKS` |
| Red, blue and green lines (EV bb/100) | `bin/hhlines [-min N] [-n N] PACKS` |
| 3-bet % from one position against an open from another | `bin/hh3bet [-pos BB] [-opener BTN] PACKS` |
| One player's 3-bet % for every pair of positions | `bin/hh3bet -player NAME PACKS` (`'*'` for everyone) |
| Every player's stats as on Hand2Note's player page: JSON per player, the whole pool, and a page with a search box | `bin/hhstats -out DIR PACKS` (open `DIR/index.html`) |
| Clean copies of the hands for Hand2Note | `bin/hhclean -regenerate -repair -out DIR SOURCE` |
| Every hand of some players, copied out to small files | `bin/hhextract -out DIR -names A,B PATH...` |
| Who wins from whom: each group of players' winnings from each other group, by stake | `bin/hhflows PACKS` (needs the packs' `hands.hht`) |
| Rake per 100 hands at each stake, beside the players' EV win rate | `bin/hhrake PACKS` (needs the anonymous packs) |

`hhpack` notes:

- `-min N` (default 10000): players with at least N hands get their own pack.
- `-name A,B`: only these players get a pack, however many hands they have,
  and no anonymous packs unless `-anon` is given.
- `-read FILE`: read a pack back and summarise it (hands, size, read speed,
  net result).
- `-anon N` (default 16): everyone else goes into N anonymous packs,
  `AnonPlayer00` and on. A player gets a different one in each hand, so an
  anonymous pack is a sample of the pool, not a person. `-anon 0` drops them.
- `-repair` repairs hands that fail a check, then checks them again; without it
  they are left out.
- It reads everything twice (choose the best copy of each hand, then build), on
  every CPU. `-pass1` stops after the first pass.

Read a command's doc comment before using it.

## Writing a new statistic

Most requests are "for each player, how often do they X". Copy `cmd/hh3bet` or
`cmd/hhlines`, which are short, and change the counting. The pattern:

```go
var mu sync.Mutex
byPlayer := map[string]*row{}
err := hh.ScanPacks(dir, func(player string, hands []hh.PlayerHand) {
	var part row // count this block locally, then merge under the lock
	for i := range hands {
		p := &hands[i]
		me := p.Players[p.Hero] // me.Position, me.Cards, me.Stack
		for st := range p.Steps() { // replays the actions: st.Street, st.Event, st.Pot, st.ToCall
			_ = st
		}
		part.hands++
	}
	mu.Lock()
	// add part into byPlayer[player]
	mu.Unlock()
})
```

Things to know:

- `ScanPacks` calls the function from many goroutines at once, even for one
  player, and the hands are only valid during the call. Copy what you keep.
- A `PlayerHand` stops at the hero's last action: a hero who folds sees nothing
  after the fold. `Events` holds actions only; street, pot and amount to call
  come from replaying them with `Steps()`.
- `Players[i].Position` is UTG, UTG+1, UTG+2, LJ, HJ, CO, BTN, SB or BB, sorted in
  preflop acting order. A 6-max table uses UTG, HJ, CO, BTN, SB, BB. **Heads-up**
  the two players are BTN (who posts the small blind) and BB, so leave heads-up
  out of position statistics (`len(p.Players) < 3`).
- `p.Type` is `hh.Regular`, `hh.Ante` or `hh.Bomb` (a bomb pot: no preflop
  betting). Straddles show up as an `hh.PostStraddle` event.
- Money is `hh.Cents`. Divide by `p.BigBlind` for big blinds.
- Result in EV: `p.NetEV()`; actual: `p.Net()`. `p.Showdown()` says whether the
  hero went to showdown.
- Test with invented hands: write the hand text in a test, parse it with
  `hh.ParseHandPartial(lines)`, take `h.PlayerHandFor("Bob")`, and check the
  counting function on it. `cmd/hh3bet/main_test.go` is an example.
- Build it, run it on a small pack folder first, then on everything, and report
  how long it took.

## Rules

- **Report EV, not actual,** for win rates and anything built on them, unless the
  user asks for actual. Every per-player table includes the player's EV bb/100
  over all their hands, whatever the statistic.
- **Use every CPU.** Batch tools read files in parallel, one worker per CPU
  (`hh.ScanPacks` and `hh.WalkText` already do).
- **No real hands, players, hand IDs or table names in the repository,** not even
  in tests. Test hands are invented (Alice, Bob, Player001, hand #1000001).
- **Do not name where the user's files come from** in code, comments, output or
  commit messages; say "other files" or describe the behaviour.
- **Parse, check, repair:** parsing reads what is in the file with no
  interpretation; a check only reports and never changes a hand; repair is the
  only thing that changes a hand, runs only after a check failed, and is followed
  by checking again. A check works on the actions directly and does not build
  `PlayerHand`s; views are built only for the players who get a pack entry.
- **Derive, don't store:** work facts out from the hand's structure rather than
  storing what the file says, and treat a failure to derive as a sign the
  structure is missing something.
- **Measure, don't guess.** Check claims against the real files, say what was
  measured and what was not, and give counts.
- Commit only when the user asks.
