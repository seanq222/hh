package hh

import (
	"encoding/binary"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// AnonPrefix starts the names of the anonymous packs that hhpack writes for
// the players with too few hands for a pack of their own: AnonPlayer00 and on.
// Each holds a sample of the pool, not one player.
const AnonPrefix = "AnonPlayer"

// IsAnonPlayer reports whether name is an anonymous pack's: AnonPrefix followed
// by digits.
func IsAnonPlayer(name string) bool {
	rest, ok := strings.CutPrefix(name, AnonPrefix)
	if !ok || rest == "" {
		return false
	}
	for i := 0; i < len(rest); i++ {
		if rest[i] < '0' || rest[i] > '9' {
			return false
		}
	}
	return true
}

// PackSummary is what can be read about a pack without reading its hands: its
// header, and the player's totals from its footer.
type PackSummary struct {
	PackInfo
	Stats PackStats
}

// ReadPackSummary reads the header and footer of a pack file.
func ReadPackSummary(path string) (PackSummary, error) {
	f, err := os.Open(path)
	if err != nil {
		return PackSummary{}, err
	}
	defer f.Close()
	pr, err := NewPackReader(f)
	if err != nil {
		return PackSummary{}, err
	}
	st, err := f.Stat()
	if err != nil {
		return PackSummary{}, err
	}
	size := st.Size()
	var tail [footerTail]byte
	if size < int64(footerTail) {
		return PackSummary{}, ErrCorrupt
	}
	if _, err := f.ReadAt(tail[:], size-int64(footerTail)); err != nil {
		return PackSummary{}, err
	}
	n := int64(binary.LittleEndian.Uint32(tail[:4]))
	if n > 1<<10 || size < n+int64(footerTail)+1 {
		return PackSummary{}, ErrCorrupt
	}
	b := make([]byte, 1+n+int64(footerTail)) // the 0 that ends the blocks, the totals, the tail
	if _, err := f.ReadAt(b, size-int64(len(b))); err != nil {
		return PackSummary{}, err
	}
	if b[0] != 0 {
		return PackSummary{}, ErrCorrupt
	}
	s, err := parseFooter(b[1:])
	if err != nil {
		return PackSummary{}, err
	}
	return PackSummary{pr.Info(), s}, nil
}

// ReadPackIDs reads the header and footer of every pack file under dir and
// returns each pack's summary by its ID, so that the pack IDs in the views (see
// Player.PackID) can be turned back into players. Packs with no ID (NoPack)
// are left out; two packs with one ID are an error.
func ReadPackIDs(dir string) (map[uint16]PackSummary, error) {
	ids := map[uint16]PackSummary{}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.EqualFold(filepath.Ext(p), ".hhpk") {
			return err
		}
		s, err := ReadPackSummary(p)
		if err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
		if s.ID == NoPack {
			return nil
		}
		if other, ok := ids[s.ID]; ok {
			return fmt.Errorf("%s: pack ID %d is also %s's", p, s.ID, other.Player)
		}
		ids[s.ID] = s
		return nil
	})
	return ids, err
}
