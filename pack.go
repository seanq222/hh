package hh

import (
	"bufio"
	"bytes"
	"cmp"
	"compress/flate"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"iter"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/seanq222/hh/pk"
)

// A pack file holds the PlayerHands of one player. Layout:
//
//	magic "HHPK", version byte, codec byte, player name (uvarint length, bytes),
//	pack ID (uvarint, at most 65535), kind byte (bit 0: anonymous)
//	block*: hands in the block (uvarint), size once unpacked (uvarint),
//	        size as stored (uvarint), the stored bytes
//	footer: 0 (uvarint, no more blocks), then the player's totals, PackStats:
//	        Hands (uvarint), Net, NetEV, Rake (varints), BB, EVBB, RakeBB
//	        (float64, little-endian); then the length of the totals (uint32,
//	        little-endian) and "HHPE", so that they can be read from the end
//
// A block is up to blockHands hands, written one column at a time so that
// similar values sit together and compress well. A column is n values, one for
// each hand, except where it says otherwise:
//
//	HandID        varint, difference from the previous hand
//	Time          varint, seconds since 1970, difference from the previous hand
//	SmallBlind    varint
//	BigBlind      varint
//	shape         byte: Type (2 bits), Reached (2), Hero (4)
//	board         the cards seen: 0, 3, 4 or 5 bytes, by Reached
//	player count  byte
//	event count   uvarint
//	positions     byte per player: Position (4 bits), HasCards (1)
//	hole cards    2 bytes for each player with HasCards
//	stacks        varint per player
//	actors        byte per event: Actor (4 bits), Kind (4)
//	flags         byte per event: AllIn (1), Inferred (2)
//	amounts       varint per event
//	Won           varint
//	Rake          varint
//	WonEV         varint: WonEV less Won, 0 for nearly every hand
//	Site          byte
//	pack IDs      uvarint per player: Player.PackID
//
// Files of any other version are refused and must be written again.
const (
	packMagic   = "HHPK"
	packVersion = 4

	blockHands   = 4096 // hands per block
	maxBlockSize = 1 << 28
)

// Codec is how the blocks of a pack file are compressed.
type Codec uint8

const (
	CodecNone  Codec = iota // stored as they are
	CodecFlate              // compress/flate
)

// ErrCorrupt is returned for a pack file that is damaged or is not one.
var ErrCorrupt = errors.New("hh: pack file is corrupt")

// The columns of a block, in the order they are stored.
const (
	colHandID = iota
	colTime
	colSmallBlind
	colBigBlind
	colShape
	colBoard
	colPlayerCount
	colEventCount
	colPositions
	colHoleCards
	colStacks
	colActors
	colFlags
	colAmounts
	colWon
	colRake
	colWonEV
	colSite
	colPackIDs
	numCols
)

// PackWriter writes the PlayerHands of one player to a pack file. A hand is
// encoded as it is written, its values appended to the columns of the block being
// built, so that a hand waiting for its block to fill takes the few dozen bytes
// it takes in the file, not the size of a PlayerHand.
type PackWriter struct {
	w     io.Writer
	codec Codec
	size  int // hands per block
	err   error

	n                int // hands in the block being built
	cols             [numCols][]byte
	prevID, prevTime int64     // the block's previous hand, for the differences
	stats            PackStats // of the hands written so far, for the footer
	closed           bool
}

// PackStats are the totals of a pack's hands, kept in its footer so that they
// can be read without reading the hands.
type PackStats struct {
	Hands  int64
	Net    int64   // what the player won less what they put in, in cents
	NetEV  int64   // the same with all-in luck taken out (see PlayerHand.NetEV)
	Rake   int64   // the rake and jackpot fee they paid (see PlayerHand.RakePaid)
	BB     float64 // Net in big blinds, each hand in its own
	EVBB   float64 // NetEV in big blinds
	RakeBB float64 // Rake in big blinds
}

// Per100 returns bb, a total in big blinds, per 100 hands: Per100(s.EVBB) is
// the EV win rate.
func (s PackStats) Per100(bb float64) float64 {
	if s.Hands == 0 {
		return 0
	}
	return 100 * bb / float64(s.Hands)
}

// Add counts a hand in the totals.
func (s *PackStats) Add(p *PlayerHand) {
	s.Hands++
	net, ev, rake := p.Net(), p.NetEV(), p.RakePaid()
	s.Net += int64(net)
	s.NetEV += int64(ev)
	s.Rake += int64(rake)
	if p.BigBlind > 0 {
		bb := float64(p.BigBlind)
		s.BB += float64(net) / bb
		s.EVBB += float64(ev) / bb
		s.RakeBB += float64(rake) / bb
	}
}

// PackInfo is what a pack file's header says about its hands.
type PackInfo struct {
	Player string // whose hands they are, or an anonymous pack's name
	// ID is the pack's number in the build that wrote it, the PackID its player
	// has in every view (see Player.PackID); NoPack for a pack written on its own.
	ID uint16
	// Anon says the pack is anonymous: a sample of the players with too few
	// hands for a pack of their own, not one player (see AnonPrefix).
	Anon bool
}

// NewPackWriter writes the header of a pack file and returns a writer for the
// hands. The caller must call Close.
func NewPackWriter(w io.Writer, info PackInfo, codec Codec) (*PackWriter, error) {
	if codec != CodecNone && codec != CodecFlate {
		return nil, fmt.Errorf("hh: unknown codec %d", codec)
	}
	hdr := append([]byte(packMagic), packVersion, byte(codec))
	hdr = binary.AppendUvarint(hdr, uint64(len(info.Player)))
	hdr = append(hdr, info.Player...)
	hdr = binary.AppendUvarint(hdr, uint64(info.ID))
	var kind byte
	if info.Anon {
		kind = 1
	}
	hdr = append(hdr, kind)
	if _, err := w.Write(hdr); err != nil {
		return nil, err
	}
	return &PackWriter{w: w, codec: codec, size: blockHands}, nil
}

// Write adds a hand. It returns an error for a hand the format cannot hold,
// which cannot happen to one that PlayerHands built.
func (pw *PackWriter) Write(p *PlayerHand) error {
	if pw.err != nil {
		return pw.err
	}
	if err := p.packable(); err != nil {
		return err
	}
	pw.add(p)
	if pw.n >= pw.size {
		pw.err = pw.flush()
	}
	return pw.err
}

// Buffered is the number of hands written but not yet stored in a block.
func (pw *PackWriter) Buffered() int { return pw.n }

// BufferedBytes is the memory the hands not yet stored in a block take, encoded.
func (pw *PackWriter) BufferedBytes() int {
	n := 0
	for i := range pw.cols {
		n += len(pw.cols[i])
	}
	return n
}

// Flush stores the hands buffered so far as a block, even a short one. Blocks
// that are too short compress less well.
func (pw *PackWriter) Flush() error {
	if pw.err == nil && pw.n > 0 {
		pw.err = pw.flush()
	}
	return pw.err
}

// Close writes the last block and the footer with the player's totals. It does
// not close the underlying writer.
func (pw *PackWriter) Close() error {
	if pw.err == nil && pw.n > 0 {
		pw.err = pw.flush()
	}
	if pw.err == nil && !pw.closed {
		pw.closed = true
		_, pw.err = pw.w.Write(appendFooter([]byte{0}, pw.stats))
	}
	return pw.err
}

// Stats are the totals of the hands written so far.
func (pw *PackWriter) Stats() PackStats { return pw.stats }

const (
	footerMagic = "HHPE"
	footerTail  = 4 + len(footerMagic) // the totals' length and the magic
)

// appendFooter appends the totals, their length and the magic.
func appendFooter(b []byte, s PackStats) []byte {
	start := len(b)
	b = binary.AppendUvarint(b, uint64(s.Hands))
	b = binary.AppendVarint(b, s.Net)
	b = binary.AppendVarint(b, s.NetEV)
	b = binary.AppendVarint(b, s.Rake)
	for _, f := range []float64{s.BB, s.EVBB, s.RakeBB} {
		b = binary.LittleEndian.AppendUint64(b, math.Float64bits(f))
	}
	b = binary.LittleEndian.AppendUint32(b, uint32(len(b)-start))
	return append(b, footerMagic...)
}

// parseFooter reads the totals from b, the footer after its leading 0, up to
// the end of the file.
func parseFooter(b []byte) (PackStats, error) {
	var s PackStats
	if len(b) < footerTail || string(b[len(b)-len(footerMagic):]) != footerMagic {
		return s, ErrCorrupt
	}
	body := b[:len(b)-footerTail]
	if int(binary.LittleEndian.Uint32(b[len(body):])) != len(body) {
		return s, ErrCorrupt
	}
	c := &colReader{b: body}
	hands := c.uvarint()
	s.Hands, s.Net, s.NetEV, s.Rake = int64(hands), c.varint(), c.varint(), c.varint()
	for _, f := range []*float64{&s.BB, &s.EVBB, &s.RakeBB} {
		if len(c.b) < 8 {
			return s, ErrCorrupt
		}
		*f = math.Float64frombits(binary.LittleEndian.Uint64(c.b))
		c.b = c.b[8:]
	}
	if c.err != nil || len(c.b) != 0 || hands > 1<<40 {
		return s, ErrCorrupt
	}
	return s, nil
}

// add appends the hand's values to the columns of the block being built.
func (pw *PackWriter) add(p *PlayerHand) {
	c := &pw.cols
	c[colHandID] = binary.AppendVarint(c[colHandID], p.HandID-pw.prevID)
	pw.prevID = p.HandID
	t := p.Time.Unix()
	c[colTime] = binary.AppendVarint(c[colTime], t-pw.prevTime)
	pw.prevTime = t
	c[colSmallBlind] = binary.AppendVarint(c[colSmallBlind], int64(p.SmallBlind))
	c[colBigBlind] = binary.AppendVarint(c[colBigBlind], int64(p.BigBlind))
	c[colShape] = append(c[colShape], byte(p.Type)|byte(p.Reached)<<2|p.Hero<<4)
	for _, card := range p.Board[:cardsAtStreet[p.Reached]] {
		c[colBoard] = append(c[colBoard], byte(card))
	}
	c[colPlayerCount] = append(c[colPlayerCount], byte(len(p.Players)))
	c[colEventCount] = binary.AppendUvarint(c[colEventCount], uint64(len(p.Events)))
	for _, pl := range p.Players {
		v := byte(pl.Position)
		if pl.HasCards {
			v |= 1 << 4
			c[colHoleCards] = append(c[colHoleCards], byte(pl.Cards[0]), byte(pl.Cards[1]))
		}
		c[colPositions] = append(c[colPositions], v)
		c[colStacks] = binary.AppendVarint(c[colStacks], int64(pl.Stack))
	}
	for _, e := range p.Events {
		c[colActors] = append(c[colActors], e.Actor<<4|byte(e.Kind))
		var f byte
		if e.AllIn {
			f |= 1
		}
		if e.Inferred {
			f |= 2
		}
		c[colFlags] = append(c[colFlags], f)
		c[colAmounts] = binary.AppendVarint(c[colAmounts], int64(e.Amount))
	}
	c[colWon] = binary.AppendVarint(c[colWon], int64(p.Won))
	c[colRake] = binary.AppendVarint(c[colRake], int64(p.Rake))
	c[colWonEV] = binary.AppendVarint(c[colWonEV], int64(p.WonEV-p.Won))
	c[colSite] = append(c[colSite], byte(p.Site))
	pw.stats.Add(p)
	for _, pl := range p.Players {
		c[colPackIDs] = binary.AppendUvarint(c[colPackIDs], uint64(pl.PackID))
	}
	pw.n++
}

func (p *PlayerHand) packable() error {
	switch {
	case len(p.Players) > 15 || p.Hero > 15:
		return fmt.Errorf("hh: hand %d has too many players to pack", p.HandID)
	case p.Type > Bomb || p.Reached > River:
		return fmt.Errorf("hh: hand %d has a type or street that cannot be packed", p.HandID)
	}
	for i := range p.Events {
		if p.Events[i].Actor > 15 || p.Events[i].Kind > 15 {
			return fmt.Errorf("hh: hand %d has an event that cannot be packed", p.HandID)
		}
	}
	for i := range p.Players {
		if p.Players[i].Position > 15 {
			return fmt.Errorf("hh: hand %d has a position that cannot be packed", p.HandID)
		}
	}
	return nil
}

// flush stores the block being built: its columns one after another, and starts
// the next. The columns' memory is let go, since a writer may wait long for its
// next hand.
func (pw *PackWriter) flush() error {
	n := 0
	for i := range pw.cols {
		n += len(pw.cols[i])
	}
	b := make([]byte, 0, n)
	for i := range pw.cols {
		b = append(b, pw.cols[i]...)
		pw.cols[i] = nil
	}
	hands := pw.n
	pw.n, pw.prevID, pw.prevTime = 0, 0, 0

	stored := b
	if pw.codec == CodecFlate {
		var buf bytes.Buffer
		fw, err := flate.NewWriter(&buf, flate.DefaultCompression)
		if err != nil {
			return err
		}
		if _, err := fw.Write(b); err != nil {
			return err
		}
		if err := fw.Close(); err != nil {
			return err
		}
		stored = buf.Bytes()
	}
	hdr := binary.AppendUvarint(nil, uint64(hands))
	hdr = binary.AppendUvarint(hdr, uint64(len(b)))
	hdr = binary.AppendUvarint(hdr, uint64(len(stored)))
	if _, err := pw.w.Write(hdr); err != nil {
		return err
	}
	_, err := pw.w.Write(stored)
	return err
}

// PackReader reads the PlayerHands from a pack file.
type PackReader struct {
	r     *bufio.Reader
	info  PackInfo
	codec Codec
	read  int64        // hands in the blocks read so far, to check against the footer
	stats PackStats    // from the footer, once the last block has been read
	hands []PlayerHand // the current block
	next  int
	err   error
}

// NewPackReader reads the header of a pack file.
func NewPackReader(r io.Reader) (*PackReader, error) {
	br := bufio.NewReaderSize(r, 1<<16)
	var hdr [6]byte
	if _, err := io.ReadFull(br, hdr[:]); err != nil || string(hdr[:4]) != packMagic {
		return nil, ErrCorrupt
	}
	if hdr[4] != packVersion {
		return nil, fmt.Errorf("hh: pack file version %d is not supported, write the file again", hdr[4])
	}
	codec := Codec(hdr[5])
	if codec != CodecNone && codec != CodecFlate {
		return nil, fmt.Errorf("hh: pack file uses unknown codec %d", codec)
	}
	n, err := binary.ReadUvarint(br)
	if err != nil || n > 1<<16 {
		return nil, ErrCorrupt
	}
	name := make([]byte, n)
	if _, err := io.ReadFull(br, name); err != nil {
		return nil, ErrCorrupt
	}
	id, err := binary.ReadUvarint(br)
	if err != nil || id > 0xFFFF {
		return nil, ErrCorrupt
	}
	kind, err := br.ReadByte()
	if err != nil || kind > 1 {
		return nil, ErrCorrupt
	}
	return &PackReader{r: br, info: PackInfo{Player: string(name), ID: uint16(id), Anon: kind == 1}, codec: codec}, nil
}

// Player is the name of the player the hands belong to.
func (pr *PackReader) Player() string { return pr.info.Player }

// Info is what the file's header says about its hands.
func (pr *PackReader) Info() PackInfo { return pr.info }

// Stats are the player's totals from the footer, once Next has returned io.EOF.
func (pr *PackReader) Stats() PackStats { return pr.stats }

// Next returns the next hand, or io.EOF after the last. The Players and Events
// of the hands from one block share storage, so treat them as read-only.
func (pr *PackReader) Next() (PlayerHand, error) {
	for pr.next >= len(pr.hands) {
		if pr.err != nil {
			return PlayerHand{}, pr.err
		}
		pr.err = pr.readBlock()
		if pr.err != nil && pr.err != io.EOF {
			return PlayerHand{}, pr.err
		}
	}
	p := pr.hands[pr.next]
	pr.next++
	return p, nil
}

// All returns the hands in order. It stops at the first error.
func (pr *PackReader) All() iter.Seq2[PlayerHand, error] {
	return func(yield func(PlayerHand, error) bool) {
		for {
			p, err := pr.Next()
			if err == io.EOF {
				return
			}
			if !yield(p, err) || err != nil {
				return
			}
		}
	}
}

func (pr *PackReader) readBlock() error {
	b, err := pr.readStored()
	if err != nil {
		return err
	}
	hands, err := b.decode(pr.codec, pr.hands[:0])
	if err != nil {
		return err
	}
	pr.hands, pr.next = hands, 0
	return nil
}

// storedBlock is a block as the file stores it, not yet decoded.
type storedBlock struct {
	n      int // hands
	rawLen uint64
	stored []byte
}

// readStored reads the next block's bytes, or returns io.EOF after the last.
func (pr *PackReader) readStored() (storedBlock, error) {
	n, err := binary.ReadUvarint(pr.r)
	if err == io.EOF {
		return storedBlock{}, ErrCorrupt // cut short: no footer
	}
	if err == nil && n == 0 {
		// The footer: the rest of the file, a few dozen bytes.
		rest, err := io.ReadAll(io.LimitReader(pr.r, 1<<10))
		if _, extra := pr.r.ReadByte(); err != nil || extra != io.EOF {
			return storedBlock{}, ErrCorrupt
		}
		s, err := parseFooter(rest)
		if err != nil || s.Hands != pr.read {
			return storedBlock{}, ErrCorrupt
		}
		pr.stats = s
		return storedBlock{}, io.EOF
	}
	rawLen, err2 := binary.ReadUvarint(pr.r)
	storedLen, err3 := binary.ReadUvarint(pr.r)
	if err != nil || err2 != nil || err3 != nil || n == 0 || n > 1<<20 || rawLen > maxBlockSize || storedLen > maxBlockSize {
		return storedBlock{}, ErrCorrupt
	}
	stored := make([]byte, storedLen)
	if _, err := io.ReadFull(pr.r, stored); err != nil {
		return storedBlock{}, ErrCorrupt
	}
	pr.read += int64(n)
	return storedBlock{int(n), rawLen, stored}, nil
}

// decode decompresses the block and unpacks its hands, reusing dst.
func (b storedBlock) decode(codec Codec, dst []PlayerHand) ([]PlayerHand, error) {
	raw := b.stored
	if codec == CodecFlate {
		raw = make([]byte, b.rawLen)
		fr := flate.NewReader(bytes.NewReader(b.stored))
		if _, err := io.ReadFull(fr, raw); err != nil {
			return nil, ErrCorrupt
		}
	} else if uint64(len(raw)) != b.rawLen {
		return nil, ErrCorrupt
	}
	return unpackBlock(raw, b.n, dst)
}

// ScanPacks calls fn for every block of hands in the pack files under dir, with
// the name of the player the hands belong to. The files are read largest first,
// and the blocks are decoded on every CPU, so fn is called from many goroutines
// at once, also for one player, and must be safe for that. The hands passed to fn
// are valid only during the call. It returns the first error met, after reading
// what it can.
func ScanPacks(dir string, fn func(player string, hands []PlayerHand)) error {
	return ScanPackInfos(dir, func(info PackInfo, hands []PlayerHand) { fn(info.Player, hands) })
}

// ScanPackInfos is ScanPacks, giving fn the whole of each pack's header.
func ScanPackInfos(dir string, fn func(info PackInfo, hands []PlayerHand)) error {
	type file struct {
		path string
		size int64
	}
	var files []file
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.EqualFold(filepath.Ext(p), ".hhpk") {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		files = append(files, file{p, info.Size()})
		return nil
	})
	if err != nil {
		return err
	}
	slices.SortFunc(files, func(a, b file) int { return cmp.Compare(b.size, a.size) })

	type job struct {
		info  PackInfo
		codec Codec
		block storedBlock
	}
	var (
		errOnce  sync.Once
		firstErr error
	)
	fail := func(err error) { errOnce.Do(func() { firstErr = err }) }

	// A few readers fetch the blocks from the files, the largest first; the
	// workers decode them.
	jobs := make(chan job, 4*runtime.NumCPU())
	paths := make(chan string)
	var readers sync.WaitGroup
	for range min(4, len(files)) {
		readers.Add(1)
		go func() {
			defer readers.Done()
			for p := range paths {
				f, err := os.Open(p)
				if err != nil {
					fail(err)
					continue
				}
				pr, err := NewPackReader(f)
				if err != nil {
					f.Close()
					fail(fmt.Errorf("%s: %w", p, err))
					continue
				}
				for {
					b, err := pr.readStored()
					if err == io.EOF {
						break
					}
					if err != nil {
						fail(fmt.Errorf("%s: %w", p, err))
						break
					}
					jobs <- job{pr.info, pr.codec, b}
				}
				f.Close()
			}
		}()
	}
	var workers sync.WaitGroup
	for range runtime.NumCPU() {
		workers.Add(1)
		go func() {
			defer workers.Done()
			var buf []PlayerHand
			for j := range jobs {
				hands, err := j.block.decode(j.codec, buf[:0])
				if err != nil {
					fail(fmt.Errorf("%s: %w", j.info.Player, err))
					continue
				}
				fn(j.info, hands)
				buf = hands
			}
		}()
	}
	for _, f := range files {
		paths <- f.path
	}
	close(paths)
	readers.Wait()
	close(jobs)
	workers.Wait()
	return firstErr
}

// colReader reads values from a block. The first failure is kept and every read
// after it returns zero.
type colReader struct {
	b   []byte
	err error
}

func (c *colReader) byte() byte {
	if len(c.b) == 0 {
		c.err = ErrCorrupt
		return 0
	}
	v := c.b[0]
	c.b = c.b[1:]
	return v
}

func (c *colReader) varint() int64 {
	v, n := binary.Varint(c.b)
	if n <= 0 {
		c.err = ErrCorrupt
		return 0
	}
	c.b = c.b[n:]
	return v
}

func (c *colReader) uvarint() uint64 {
	v, n := binary.Uvarint(c.b)
	if n <= 0 {
		c.err = ErrCorrupt
		return 0
	}
	c.b = c.b[n:]
	return v
}

func (c *colReader) cents() Cents {
	v := c.varint()
	if v != int64(Cents(v)) {
		c.err = ErrCorrupt
	}
	return Cents(v)
}

// unpackBlock decodes n hands from raw, reusing dst.
func unpackBlock(raw []byte, n int, dst []PlayerHand) ([]PlayerHand, error) {
	c := &colReader{b: raw}
	hands := append(dst, make([]PlayerHand, n)...)
	var id, t int64
	for i := range hands {
		id += c.varint()
		hands[i].HandID = id
	}
	for i := range hands {
		t += c.varint()
		hands[i].Time = time.Unix(t, 0).UTC()
	}
	for i := range hands {
		hands[i].SmallBlind = c.cents()
	}
	for i := range hands {
		hands[i].BigBlind = c.cents()
	}
	for i := range hands {
		v := c.byte()
		p := &hands[i]
		p.Type, p.Reached, p.Hero = HandType(v&3), Street(v>>2&3), v>>4
		if p.Type > Bomb {
			c.err = ErrCorrupt
		}
	}
	for i := range hands {
		p := &hands[i]
		for k := 0; k < cardsAtStreet[p.Reached]; k++ {
			p.Board[k] = pk.Card(c.byte())
		}
	}
	nPlayers := make([]byte, n)
	var totalPlayers, totalEvents int
	for i := range nPlayers {
		nPlayers[i] = c.byte()
		totalPlayers += int(nPlayers[i])
	}
	nEvents := make([]int, n)
	for i := range nEvents {
		v := c.uvarint()
		if v > uint64(len(raw)) { // each event takes at least a byte
			return nil, ErrCorrupt
		}
		nEvents[i] = int(v)
		totalEvents += nEvents[i]
	}
	if c.err != nil || totalEvents > len(raw) {
		return nil, ErrCorrupt
	}

	// The players and events of the whole block share one allocation each.
	players := make([]Player, totalPlayers)
	events := make([]Event, totalEvents)
	for i, pi := 0, 0; i < n; i++ {
		k := int(nPlayers[i])
		hands[i].Players = players[pi : pi+k : pi+k]
		pi += k
	}
	for i, ei := 0, 0; i < n; i++ {
		k := nEvents[i]
		hands[i].Events = events[ei : ei+k : ei+k]
		ei += k
	}
	for i := range players {
		v := c.byte()
		players[i].Position = Position(v & 15)
		players[i].HasCards = v&(1<<4) != 0
	}
	for i := range players {
		if players[i].HasCards {
			players[i].Cards = pk.HoleCards{pk.Card(c.byte()), pk.Card(c.byte())}
		}
	}
	for i := range players {
		players[i].Stack = c.cents()
	}
	for i := range events {
		v := c.byte()
		events[i].Actor, events[i].Kind = v>>4, ActionKind(v&15)
	}
	for i := range events {
		f := c.byte()
		events[i].AllIn, events[i].Inferred = f&1 != 0, f&2 != 0
	}
	for i := range events {
		events[i].Amount = c.cents()
	}
	for i := range hands {
		hands[i].Won = c.cents()
	}
	for i := range hands {
		hands[i].Rake = c.cents()
	}
	for i := range hands {
		hands[i].WonEV = hands[i].Won + c.cents()
	}
	for i := range hands {
		hands[i].Site = Site(c.byte())
		if hands[i].Site > CoinPoker {
			c.err = ErrCorrupt
		}
	}
	for i := range players {
		v := c.uvarint()
		if v > 0xFFFF {
			c.err = ErrCorrupt
		}
		players[i].PackID = uint16(v)
	}
	if c.err != nil || len(c.b) != 0 {
		return nil, ErrCorrupt
	}
	return hands, nil
}

// WritePackFile writes the hands to a pack file.
func WritePackFile(path string, info PackInfo, hands iter.Seq[PlayerHand], codec Codec) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	bw := bufio.NewWriterSize(f, 1<<16)
	pw, err := NewPackWriter(bw, info, codec)
	if err == nil {
		for p := range hands {
			if err = pw.Write(&p); err != nil {
				break
			}
		}
	}
	if err == nil {
		err = pw.Close()
	}
	if err == nil {
		err = bw.Flush()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return err
}

// ReadPackFile reads every hand in a pack file and the name of its player.
func ReadPackFile(path string) (player string, hands []PlayerHand, err error) {
	f, err := os.Open(path)
	if err != nil {
		return "", nil, err
	}
	defer f.Close()
	pr, err := NewPackReader(f)
	if err != nil {
		return "", nil, err
	}
	for p, err := range pr.All() {
		if err != nil {
			return "", nil, err
		}
		hands = append(hands, p)
	}
	return pr.Player(), hands, nil
}
