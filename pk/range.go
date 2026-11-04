package pk

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/seanq222/hh/internal/slice"
)

type Range struct {
	Weights [NumHoleCardPairs]float64
}

func Equal64(a, b, epsilon float64) bool {
	// Handle direct equality for cases where numbers are exactly the same,
	// including positive/negative zero and infinities.
	if a == b {
		return true
	}

	// Calculate the absolute difference.
	diff := math.Abs(a - b)

	// Calculate the scaling factor for the relative tolerance.
	// Use the maximum of the absolute values of a and b to ensure the
	// tolerance scales appropriately for large numbers.
	// Add a small constant to prevent division by zero if both a and b are very close to zero.
	scale := math.Max(math.Abs(a), math.Abs(b))
	if scale == 0 { // Handles the case where both a and b are zero (already covered by a == b)
		return diff < epsilon
	}

	// Compare the absolute difference to the scaled tolerance.
	return diff/scale < epsilon
}

func (r *Range) Clear() {
	slice.Set(r.Weights[:], 0)
}

func (r *Range) Set(w float64) {
	slice.Set(r.Weights[:], w)
}

func (r *Range) Size() int {
	n := 0
	for i := range r.Weights {
		if r.Weights[i] > 0 {
			n++
		}
	}
	return n
}

func (r *Range) Weight(hc HoleCards) float64 {
	return r.Weights[hc.Index()]
}

func addElement(buf *strings.Builder, s string, w float64) {
	if w == 0 {
		return
	}
	if buf.Len() > 0 {
		buf.WriteString(", ")
	}
	buf.WriteString(s)
	if w != 1 {
		buf.WriteString(fmt.Sprintf(":%g", w))
	}
}

func (r *Range) isoPair(r0 int) bool {
	hc := HoleCardsFromCards(CardFromRS(r0, 0), CardFromRS(r0, 1))
	w := r.Weight(hc)
	for s0 := 0; s0 < 4; s0++ {
		for s1 := s0 + 1; s1 < 4; s1++ {
			hc := HoleCardsFromCards(CardFromRS(r0, s0), CardFromRS(r0, s1))
			if r.Weight(hc) != w {
				return false
			}
		}
	}
	return true
}

func (r *Range) isoSuited(r0, r1 int) bool {
	hc := HoleCardsFromCards(CardFromRS(r0, 0), CardFromRS(r1, 0))
	w := r.Weight(hc)
	for s0 := 1; s0 < 4; s0++ {
		hc := HoleCardsFromCards(CardFromRS(r0, s0), CardFromRS(r1, s0))
		if r.Weight(hc) != w {
			return false
		}
	}
	return true
}

func (r *Range) isoOffsuit(r0, r1 int) bool {
	hc := HoleCardsFromCards(CardFromRS(r0, 0), CardFromRS(r1, 1))
	w := r.Weight(hc)
	for s0 := 0; s0 < 4; s0++ {
		for s1 := 0; s1 < 4; s1++ {
			if s0 == s1 {
				continue
			}
			hc := HoleCardsFromCards(CardFromRS(r0, s0), CardFromRS(r1, s1))
			if r.Weight(hc) != w {
				return false
			}
		}
	}
	return true
}

func (r *Range) IsIso() bool {
	for r0 := 0; r0 < 13; r0++ {
		if !r.isoPair(r0) {
			return false
		}
		for r1 := r0 + 1; r1 < 13; r1++ {
			if !r.isoOffsuit(r0, r1) || !r.isoSuited(r0, r1) {
				return false
			}
		}
	}
	return true
}

func (r *Range) String() string {
	buf := strings.Builder{}

	for r0 := 0; r0 < 13; r0++ {

		for r1 := 0; r1 < r0; r1++ {
			// Offsuit - note that r1 is the rank of the first card
			if r.isoOffsuit(r1, r0) {
				hc := HoleCardsFromCards(CardFromRS(r1, 0), CardFromRS(r0, 1))
				addElement(&buf, fmt.Sprintf("%c%co", RankChar[r1], RankChar[r0]), r.Weight(hc))
			} else {
				for s0 := 0; s0 < 4; s0++ {
					for s1 := 0; s1 < 4; s1++ {
						if s0 == s1 {
							continue
						}
						hc := HoleCardsFromCards(CardFromRS(r0, s0), CardFromRS(r1, s1))
						addElement(&buf, fmt.Sprintf("%s%s", hc[0], hc[1]), r.Weight(hc))
					}
				}
			}
		}

		// Pair
		if r.isoPair(r0) {
			hc := HoleCardsFromCards(CardFromRS(r0, 0), CardFromRS(r0, 1))
			addElement(&buf, fmt.Sprintf("%c%c", RankChar[r0], RankChar[r0]), r.Weight(hc))
		} else {
			for s0 := 0; s0 < 4; s0++ {
				for s1 := s0 + 1; s1 < 4; s1++ {
					hc := HoleCardsFromCards(CardFromRS(r0, s0), CardFromRS(r0, s1))
					addElement(&buf, fmt.Sprintf("%s%s", hc[0], hc[1]), r.Weight(hc))
				}
			}
		}

		for r1 := r0 + 1; r1 < 13; r1++ {
			// Suited
			if r.isoSuited(r0, r1) {
				hc := HoleCardsFromCards(CardFromRS(r0, 0), CardFromRS(r1, 0))
				addElement(&buf, fmt.Sprintf("%c%cs", RankChar[r0], RankChar[r1]), r.Weight(hc))
			} else {
				for s0 := 0; s0 < 4; s0++ {
					hc := HoleCardsFromCards(CardFromRS(r0, s0), CardFromRS(r1, s0))
					addElement(&buf, fmt.Sprintf("%s%s", hc[0], hc[1]), r.Weight(hc))
				}
			}
		}
	}

	return buf.String()
}

func (r *Range) SetWeight(hc HoleCards, Weight float64) {
	r.Weights[hc.Index()] = Weight
}

func (r *Range) SetPairWeight(r0 int, Weight float64) {
	for s0 := 0; s0 < 4; s0++ {
		for s1 := s0 + 1; s1 < 4; s1++ {
			hc := HoleCardsFromCards(CardFromRS(r0, s0), CardFromRS(r0, s1))
			r.SetWeight(hc, Weight)
		}
	}
}

func (r *Range) SetSuitedWeight(r0, r1 int, Weight float64) {
	for s0 := 0; s0 < 4; s0++ {
		hc := HoleCardsFromCards(CardFromRS(r0, s0), CardFromRS(r1, s0))
		r.SetWeight(hc, Weight)
	}
}

func (r *Range) SetOffsuitWeight(r0, r1 int, Weight float64) {
	for s0 := 0; s0 < 4; s0++ {
		for s1 := 0; s1 < 4; s1++ {
			if s0 == s1 {
				continue
			}
			hc := HoleCardsFromCards(CardFromRS(r0, s0), CardFromRS(r1, s1))
			r.SetWeight(hc, Weight)
		}
	}
}

func (r *Range) CardsAndWeights() ([]HoleCards, []float64) {
	n := 0
	for i := range r.Weights {
		if r.Weights[i] != 0 {
			n++
		}
	}
	cards := make([]HoleCards, 0, n)
	Weights := make([]float64, 0, n)
	for c0 := Card(0); c0 < 52; c0++ {
		for c1 := c0 + 1; c1 < 52; c1++ {
			Weight := r.Weight(HoleCardsFromCards(c0, c1))
			if Weight > 0 {
				hc := HoleCardsFromCards(c0, c1)
				cards = append(cards, hc)
				Weights = append(Weights, Weight)
			}

		}
	}

	return cards, Weights
}

// Examples of supported notation:
// "AA, KK, QQ" - specific pairs (Weight 1.0)
// "AA:0.8, KK:0.6" - pairs with specific Weights
// "AKs, AQs" - specific suited hands (Weight 1.0)
// "AKs:0.7, AQs:0.5" - suited hands with specific Weights
// "AKo, AQo" - specific offsuit hands (Weight 1.0)
// "AKo:0.6, AQo:0.4" - offsuit hands with specific Weights
// "QQ+" - all pairs QQ and higher (Weight 1.0)
// "AKs+" - all suited hands AK and higher (Weight 1.0)
// "TT-JJ" - pairs from TT to JJ (Weight 1.0)
// "AhKh:0.5, AdKs:0.75" - specific hand combinations with Weights

func (r *Range) Parse(s string) error {
	r.Clear()
	for _, s := range strings.Split(s, ",") {
		if err := r.parseElement(s); err != nil {
			return err
		}
	}
	return nil
}

func (r *Range) parseElement(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}

	// Check if element has Weight specification (e.g., "AhKh:0.5", "AA:0.8")
	var Weight float64 = 1.0
	var handStr string = s

	if strings.Contains(s, ":") {
		parts := strings.Split(s, ":")
		if len(parts) == 2 {
			handStr = strings.TrimSpace(parts[0])
			WeightStr := strings.TrimSpace(parts[1])

			// Parse the Weight
			parsedWeight, err := strconv.ParseFloat(WeightStr, 64)
			if err != nil {
				return fmt.Errorf("invalid weight '%s': %v", WeightStr, err)
			}
			Weight = parsedWeight
		} else {
			return fmt.Errorf("invalid weight format: %s", s)
		}
	}

	// Check for individual hand (e.g., "AhKh")
	if len(handStr) == 4 {
		holeCards, err := ParseHoleCards(handStr)
		if err == nil {
			r.SetWeight(holeCards, Weight)
			return nil
		}
	}

	// Check for pair or all 16 combination of non-pairs
	if len(handStr) == 2 {
		rank0 := strings.IndexByte(RankChar, handStr[0])
		rank1 := strings.IndexByte(RankChar, handStr[1])
		if rank0 != -1 && rank1 != -1 {
			if rank0 == rank1 {
				r.SetPairWeight(rank0, Weight)
				return nil
			} else {
				r.SetSuitedWeight(rank0, rank1, Weight)
				r.SetOffsuitWeight(rank0, rank1, Weight)
				return nil
			}
		}
	}

	// Check for suited hands (e.g., "AKs", "QJs")
	if len(handStr) == 3 && handStr[2] == 's' {
		rank0 := strings.IndexByte(RankChar, handStr[0])
		rank1 := strings.IndexByte(RankChar, handStr[1])
		if rank0 != -1 && rank1 != -1 && rank0 != rank1 {
			r.SetSuitedWeight(rank0, rank1, Weight)
			return nil
		}
	}

	// Check for offsuit hands (e.g., "AKo", "QJo")
	if len(handStr) == 3 && handStr[2] == 'o' {
		rank0 := strings.IndexByte(RankChar, handStr[0])
		rank1 := strings.IndexByte(RankChar, handStr[1])
		if rank0 != -1 && rank1 != -1 && rank0 != rank1 {
			r.SetOffsuitWeight(rank0, rank1, Weight)
			return nil
		}
	}

	// Check for range notation (e.g., "QQ+", "AKs+")
	if strings.Contains(handStr, "+") {
		return r.parseRangePlus(handStr, Weight)
	}

	// Check for range notation (e.g., "TT-JJ", "AKs-AQs")
	if strings.Contains(handStr, "-") {
		return r.parseRangeDash(handStr, Weight)
	}

	return fmt.Errorf("invalid range element: %s", s)
}

// parseRangePlus handles notation like "QQ+", "AKs+"
func (r *Range) parseRangePlus(s string, Weight float64) error {
	base := strings.TrimSuffix(s, "+")

	// Handle pairs
	if len(base) == 2 && base[0] == base[1] {
		rank := strings.IndexByte(RankChar, base[0])
		if rank == -1 {
			return fmt.Errorf("invalid pair rank: %s", base)
		}

		// Add all pairs from this rank up to AA
		for rank := rank; rank >= Ace; rank-- {
			r.SetPairWeight(rank, Weight)
		}
		return nil
	}

	// Handle suited hands
	if len(base) == 3 && base[2] == 's' {
		rank0 := strings.IndexByte(RankChar, base[0])
		rank1 := strings.IndexByte(RankChar, base[1])
		if rank0 == -1 || rank1 == -1 || rank0 >= rank1 {
			return fmt.Errorf("invalid suited hand: %s", base)
		}

		// Add all suited hands from this rank combination up
		for r1 := rank1; r1 > rank0; r1-- {
			r.SetSuitedWeight(rank0, r1, Weight)
		}

		return nil
	}

	// Handle offsuit hands
	if len(base) == 3 && base[2] == 'o' {
		rank0 := strings.IndexByte(RankChar, base[0])
		rank1 := strings.IndexByte(RankChar, base[1])
		if rank0 == -1 || rank1 == -1 || rank0 >= rank1 {
			return fmt.Errorf("invalid offsuit hand: %s", base)
		}

		// Add all offsuit hands from this rank combination up
		for r1 := rank1; r1 > rank0; r1-- {
			r.SetOffsuitWeight(rank0, r1, Weight)
		}

		return nil
	}

	return fmt.Errorf("invalid range plus notation: %s", s)
}

// parseRangeDash handles notation like "TT-JJ" or "JJ-TT", "AKs-AQs" or
// "AQs-AKs" -- either order is accepted (matching PioSolver's behavior) and
// normalized internally, so callers don't need to remember a direction.
func (r *Range) parseRangeDash(s string, Weight float64) error {
	parts := strings.Split(s, "-")
	if len(parts) != 2 {
		return fmt.Errorf("invalid range dash notation: %s", s)
	}

	start := strings.TrimSpace(parts[0])
	end := strings.TrimSpace(parts[1])

	// Handle pair ranges (e.g., "TT-JJ" or "JJ-TT")
	if len(start) == 2 && start[0] == start[1] && len(end) == 2 && end[0] == end[1] {
		startRank := strings.IndexByte(RankChar, start[0])
		endRank := strings.IndexByte(RankChar, end[0])
		if startRank == -1 || endRank == -1 {
			return fmt.Errorf("invalid pair ranks: %s-%s", start, end)
		}

		lo, hi := startRank, endRank
		if lo > hi {
			lo, hi = hi, lo
		}

		// Add all pairs in the range
		for rank := lo; rank <= hi; rank++ {
			r.SetPairWeight(rank, Weight)
		}
		return nil
	}

	// Handle suited hand ranges (e.g., "AKs-AQs" or "AQs-AKs")
	if len(start) == 3 && start[2] == 's' && len(end) == 3 && end[2] == 's' {
		startRank0 := strings.IndexByte(RankChar, start[0])
		startRank1 := strings.IndexByte(RankChar, start[1])
		endRank0 := strings.IndexByte(RankChar, end[0])
		endRank1 := strings.IndexByte(RankChar, end[1])

		if startRank0 == -1 ||
			startRank1 == -1 ||
			endRank0 == -1 ||
			endRank1 == -1 ||
			startRank0 != endRank0 {
			return fmt.Errorf("invalid suited hand ranks: %s-%s", start, end)
		}

		lo, hi := startRank1, endRank1
		if lo > hi {
			lo, hi = hi, lo
		}
		if startRank0 >= lo && startRank0 <= hi {
			return fmt.Errorf("invalid suited hand ranks: %s-%s", start, end)
		}

		// Add all suited hands in the range
		for r1 := lo; r1 <= hi; r1++ {
			r.SetSuitedWeight(startRank0, r1, Weight)
		}

		return nil
	}

	// Handle offsuit hand ranges (e.g., "AKo-AQo" or "AQo-AKo")
	if len(start) == 3 && start[2] == 'o' && len(end) == 3 && end[2] == 'o' {
		startRank0 := strings.IndexByte(RankChar, start[0])
		startRank1 := strings.IndexByte(RankChar, start[1])
		endRank0 := strings.IndexByte(RankChar, end[0])
		endRank1 := strings.IndexByte(RankChar, end[1])

		if startRank0 == -1 ||
			startRank1 == -1 ||
			endRank0 == -1 ||
			endRank1 == -1 ||
			startRank0 != endRank0 {
			return fmt.Errorf("invalid offsuit hand ranks: %s-%s", start, end)
		}

		lo, hi := startRank1, endRank1
		if lo > hi {
			lo, hi = hi, lo
		}
		if startRank0 >= lo && startRank0 <= hi {
			return fmt.Errorf("invalid offsuit hand ranks: %s-%s", start, end)
		}

		// Add all offsuit hands in the range
		for r1 := lo; r1 <= hi; r1++ {
			r.SetOffsuitWeight(startRank0, r1, Weight)
		}

		return nil
	}

	return fmt.Errorf("invalid range dash notation: %s", s)
}

func (r *Range) Equal(other *Range, epsilon float64) bool {
	return slice.EqualApprox(r.Weights[:], other.Weights[:], epsilon)
}

func (s0 *Range) DotProduct(s1 *Range) float64 {
	return slice.DotProduct(s0.Weights[:], s1.Weights[:])
}

func (r *Range) Sum() float64 {
	return slice.Sum(r.Weights[:])
}

func (dst *Range) Multiply(s0 *Range, s1 *Range) {
	slice.Mul(dst.Weights[:], s0.Weights[:], s1.Weights[:])
}

func (dst *Range) Divide(s0 *Range, s1 *Range) {
	slice.Div(dst.Weights[:], s0.Weights[:], s1.Weights[:], 0)
}
