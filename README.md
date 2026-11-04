# hh

A Go package and command-line tools for No-Limit Hold'em hand histories. It
parses Winning Poker Network (ACR) and CoinPoker hand history text, checks every
hand for defects and repairs the ones that can be worked out, and turns each hand
into per-player views that it writes to compact pack files. After that, any
analysis is a fast scan over the packs on every CPU: EV win rates, red and blue
lines, 3-bet frequencies, or a new statistic written for the question at hand.

Working with Claude Code? [CLAUDE.md](CLAUDE.md) is the guide a session reads:
which command answers which question, and how to add a new statistic.

## Setup

Go 1.27 or later. Cards, hand evaluation and equity are in this module's `pk`
package.

```bash
git clone git@github.com:seanq222/hh.git
```

Then, in `hh`:

```bash
go build -o bin/ ./cmd/... && go test ./...
```

## Quick start

Every command takes folders, `.txt` files and `.zip` archives, including zips
inside zips, and reads them without unpacking.

Check that the files parse:

```bash
bin/hhcheck ~/hands
```

Build pack files: a pack for each player with at least 100,000 hands, and the
rest of the pool in 16 anonymous packs. Hands that fail a check are repaired if
they can be, and a hand found in several files is used once:

```bash
bin/hhpack -repair -min 100000 -out ~/packs ~/hands
```

Then, over the packs:

```bash
bin/hhwinrate -n 20 ~/packs
```

```bash
bin/hhlines -n 20 ~/packs
```

```bash
bin/hh3bet -pos BB -opener BTN -n 20 ~/packs
```

`hhwinrate` gives actual and EV win rates in big blinds per 100 hands, `hhlines`
the red, blue and green lines, and `hh3bet` how often players 3-bet from one
position against an open from another (`-player NAME` for every pair of
positions). `hhstats -out DIR` writes every player's stats in the layout
of Hand2Note's player page, as JSON and as a page with a search box, the values
outside their usual range coloured. `hhflows` shows who wins from whom, and
`hhrake` the rake at each stake. `hhclean` writes cleaned copies of hand
histories for Hand2Note, and `hhextract` copies out every hand of some players.

## The package

```go
// Parse a file; a malformed hand is reported and the next one read.
st, err := hh.ParseFile("hands.txt", func(h *hh.HandHistory) error {
	fmt.Println(h.ID, h.Table, len(h.Players), h.Showdown)
	return nil
}, func(err error) { log.Println(err) })
```

```go
// Scan pack files on every CPU: fn runs concurrently, and the hands are only
// valid during the call.
err := hh.ScanPacks("packs", func(player string, hands []hh.PlayerHand) {
	for i := range hands {
		p := &hands[i]
		bb := float64(p.NetEV()) / float64(p.BigBlind) // EV result in big blinds
		_ = bb
	}
})
```

The main types:

- `HandHistory` is a parsed hand: seats, actions by street, board, summary.
  `FailedCheck` says what is wrong with it, `Repair` puts back what can be worked
  out, `String` writes it out again in the site client's format.
- `PlayerHand` is one hand as one player saw it: positions, stacks, the cards
  they could see, the actions up to their last one, and what they won, actual
  (`Won`) and expected (`WonEV`, with all-in luck taken out). `Steps` replays the
  actions with the street, pot and amount to call.
- Money is `hh.Cents`, a count of cents; cards are `pk.Card`.

`go doc github.com/seanq222/hh` and each command's doc comment have the details.

## Speed

Rough figures on a 16-core desktop:

- Building packs with `hhpack`, both passes, checks and repairs included: about
  0.3 million hands a second.
- A statistic over the packs, such as `hhstats` writing every player's stats:
  about 7 million hands a second, which is some 35 million player views.

The packs take about a third of the space of the hand history text unzipped,
though more than the zipped text: they hold every player's view of each hand,
uncompressed for speed (`-codec flate` makes them about three times smaller).

## License

MIT; see [LICENSE](LICENSE).
