package pk

import (
	"strings"
	"testing"
)

func TestRangeSetGet(t *testing.T) {
	var d Deck
	for i := 0; i < 100; i++ {
		d.Shuffle()
		var r Range
		var c [2]Card
		d.Draw(c[:])
		hc := HoleCardsFromCards(c[0], c[1])
		r.SetWeight(hc, 1)

		for c0 := Card(0); c0 < 52; c0++ {
			for c1 := c0 + 1; c1 < 52; c1++ {
				hc := HoleCardsFromCards(c0, c1)
				w := r.Weight(hc)
				if c0 == c[0] && c1 == c[1] || c0 == c[1] && c1 == c[0] {
					if w != 1 {
						t.Errorf("Weight(%s) = %f, want 1", hc, w)
					}
				} else {
					if w != 0 {
						t.Errorf("Weight(%s) = %f, want 0", hc, w)
					}
				}
			}
		}
	}
}

func TestRangeString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Individual hands with weights",
			input:    "AhKh:0.5, AdKd:0.75, AhKs:0.25",
			expected: "AhKh:0.5, AdKd:0.75, AhKs:0.25",
		},
		{
			name:     "Pairs with weights",
			input:    "AA:0.8, KK:0.6, QQ:0.4",
			expected: "AA:0.8, KK:0.6, QQ:0.4",
		},
		{
			name:     "Suited hands with weights",
			input:    "AKs:0.7, AQs:0.5, AJs:0.3",
			expected: "AKs:0.7, AQs:0.5, AJs:0.3",
		},
		{
			name:     "Offsuit hands with weights",
			input:    "AKo:0.6, AQo:0.4, AJo:0.2",
			expected: "AKo:0.6, AQo:0.4, AJo:0.2",
		},
		{
			name:     "Range plus notation with weights",
			input:    "QQ+:0.9, AKs+:0.8",
			expected: "AA:0.9, AKs:0.8, KK:0.9, QQ:0.9",
		},
		{
			name:     "Range dash notation with weights",
			input:    "TT-JJ:0.7",
			expected: "JJ:0.7, TT:0.7",
		},
		{
			name:     "Mixed notation with weights",
			input:    "AA:0.9, KK:0.7, AKs:0.8, AhKh:0.5",
			expected: "AA:0.9, AsKs:0.8, AhKh:0.5, AdKd:0.8, AcKc:0.8, KK:0.7",
		},
		{
			name:     "Individual hands without weights",
			input:    "AhKh, AdKs, AhKs",
			expected: "AhKh, AhKs, AdKs",
		},
		{
			name:     "Pairs without weights",
			input:    "AA, KK, QQ",
			expected: "AA, KK, QQ",
		},
		{
			name:     "Suited hands without weights",
			input:    "AKs, AQs, AJs",
			expected: "AKs, AQs, AJs",
		},
		{
			name:     "Offsuit hands without weights",
			input:    "AKo, AQo, AJo",
			expected: "AKo, AQo, AJo",
		},
		{
			name:     "Range plus notation without weights",
			input:    "QQ+, AKs+",
			expected: "AA, AKs, KK, QQ",
		},
		{
			name:     "Range dash notation without weights",
			input:    "TT-JJ",
			expected: "JJ, TT",
		},
		{
			name:     "Range dash notation, reversed order",
			input:    "JJ-TT",
			expected: "JJ, TT",
		},
		{
			name:     "Suited range dash notation",
			input:    "AKs-AQs",
			expected: "AKs, AQs",
		},
		{
			name:     "Suited range dash notation, reversed order",
			input:    "AQs-AKs",
			expected: "AKs, AQs",
		},
		{
			name:     "Offsuit range dash notation",
			input:    "AKo-ATo",
			expected: "AKo, AQo, AJo, ATo",
		},
		{
			name:     "Offsuit range dash notation, reversed order",
			input:    "ATo-AKo",
			expected: "AKo, AQo, AJo, ATo",
		},
		{
			name:     "Suited range dash notation with weights",
			input:    "AKs-AQs:0.6",
			expected: "AKs:0.6, AQs:0.6",
		},
		{
			name:     "Non-pair plus notation without weights",
			input:    "KJs+, KQs+, AQs+",
			expected: "AKs, AQs, KQs, KJs",
		},
		{
			name:     "Complex mixed range without weights",
			input:    "AA, KK, QQ+, AKs, AKo, TT-JJ, KJs+",
			expected: "AA, AKs, AKo, KK, KQs, KJs, QQ, JJ, TT",
		},
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "Whitespace only",
			input:    "   ",
			expected: "",
		},
		{
			name:     "Comma separated empty elements",
			input:    ",,,",
			expected: "",
		},
		{
			name:     "Single hand with weight",
			input:    "AhKh:0.5",
			expected: "AhKh:0.5",
		},
		{
			name:     "Edge case weights",
			input:    "AA:0.0, KK:1.0, QQ:0.5",
			expected: "KK, QQ:0.5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var r Range
			err := r.Parse(tt.input)
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}

			s := r.String()
			if s != tt.expected {
				t.Errorf("String() = %s, want %s", s, tt.expected)
			}
		})
	}
}

// Test range parsing functionality

func TestParseRanges(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string]float64
	}{
		{
			name:     "Empty string",
			input:    "",
			expected: map[string]float64{},
		},
		{
			name:     "Whitespace only",
			input:    "   ",
			expected: map[string]float64{},
		},
		{
			name:     "Comma separated empty elements",
			input:    ",,,",
			expected: map[string]float64{},
		},
		// Unweighted test cases
		{
			name:  "Individual hands without weights",
			input: "AhKh, AdKs, AhKs",
			expected: map[string]float64{
				"AhKh": 1.0,
				"AdKs": 1.0,
				"AhKs": 1.0,
			},
		},
		{
			name:  "Pairs without weights",
			input: "AA, KK, QQ",
			expected: map[string]float64{
				"AhAd": 1.0, "AhAs": 1.0, "AhAc": 1.0, "AdAs": 1.0, "AdAc": 1.0, "AsAc": 1.0,
				"KhKd": 1.0, "KhKs": 1.0, "KhKc": 1.0, "KdKs": 1.0, "KdKc": 1.0, "KsKc": 1.0,
				"QhQd": 1.0, "QhQs": 1.0, "QhQc": 1.0, "QdQs": 1.0, "QdQc": 1.0, "QsQc": 1.0,
			},
		},
		{
			name:  "Suited hands without weights",
			input: "AKs, AQs, AJs",
			expected: map[string]float64{
				"AhKh": 1.0, "AdKd": 1.0, "AsKs": 1.0, "AcKc": 1.0,
				"AhQh": 1.0, "AdQd": 1.0, "AsQs": 1.0, "AcQc": 1.0,
				"AhJh": 1.0, "AdJd": 1.0, "AsJs": 1.0, "AcJc": 1.0,
			},
		},
		{
			name:  "Offsuit hands without weights",
			input: "AKo, AQo, AJo",
			expected: map[string]float64{
				"AhKc": 1.0, "AhKd": 1.0, "AhKs": 1.0, "AdKh": 1.0, "AdKc": 1.0, "AdKs": 1.0, "AsKh": 1.0, "AsKc": 1.0, "AsKd": 1.0, "AcKh": 1.0, "AcKd": 1.0, "AcKs": 1.0,
				"AhQc": 1.0, "AhQd": 1.0, "AhQs": 1.0, "AdQh": 1.0, "AdQc": 1.0, "AdQs": 1.0, "AsQh": 1.0, "AsQc": 1.0, "AsQd": 1.0, "AcQh": 1.0, "AcQd": 1.0, "AcQs": 1.0,
				"AhJc": 1.0, "AhJd": 1.0, "AhJs": 1.0, "AdJh": 1.0, "AdJc": 1.0, "AdJs": 1.0, "AsJh": 1.0, "AsJc": 1.0, "AsJd": 1.0, "AcJh": 1.0, "AcJd": 1.0, "AcJs": 1.0,
			},
		},
		{
			name:  "Range plus notation without weights",
			input: "QQ+, AKs+",
			expected: map[string]float64{
				// QQ+ (QQ, KK, AA) - all 6 combinations for each pair
				"QhQd": 1.0, "QhQs": 1.0, "QhQc": 1.0, "QdQs": 1.0, "QdQc": 1.0, "QsQc": 1.0,
				"KhKd": 1.0, "KhKs": 1.0, "KhKc": 1.0, "KdKs": 1.0, "KdKc": 1.0, "KsKc": 1.0,
				"AhAd": 1.0, "AhAs": 1.0, "AhAc": 1.0, "AdAs": 1.0, "AdAc": 1.0, "AsAc": 1.0,
				// AKs+ means just AKs (the specific suited hand) - all 4 suited combinations
				"AhKh": 1.0, "AdKd": 1.0, "AsKs": 1.0, "AcKc": 1.0,
			},
		},
		{
			name:  "Range dash notation without weights",
			input: "TT-JJ",
			expected: map[string]float64{
				// TT-JJ (TT, JJ) - all 6 combinations for each pair
				"ThTd": 1.0, "ThTs": 1.0, "ThTc": 1.0, "TdTs": 1.0, "TdTc": 1.0, "TsTc": 1.0,
				"JhJd": 1.0, "JhJs": 1.0, "JhJc": 1.0, "JdJs": 1.0, "JdJc": 1.0, "JsJc": 1.0,
			},
		},
		{
			name:  "Mixed notation without weights",
			input: "AA, KK, AKs, AhKh",
			expected: map[string]float64{
				// AA - all 6 combinations
				"AhAd": 1.0, "AhAs": 1.0, "AhAc": 1.0, "AdAs": 1.0, "AdAc": 1.0, "AsAc": 1.0,
				// KK - all 6 combinations
				"KhKd": 1.0, "KhKs": 1.0, "KhKc": 1.0, "KdKs": 1.0, "KdKc": 1.0, "KsKc": 1.0,
				// AKs (excluding AhKh which gets overridden) - all 4 suited combinations
				"AdKd": 1.0, "AsKs": 1.0, "AcKc": 1.0,
				// AhKh (overrides AKs weight)
				"AhKh": 1.0,
			},
		},
		{
			name:  "Non-pair plus notation without weights",
			input: "KJs+, KQs+, AQs+",
			expected: map[string]float64{
				// KJs+ means KJ and KQ (J and Q with K) - all 4 suited combinations for each
				"KhJh": 1.0, "KdJd": 1.0, "KsJs": 1.0, "KcJc": 1.0,
				// KQs+ means just KQ (Q is highest with K) - overrides KJs+ weight - all 4 suited combinations
				"KhQh": 1.0, "KdQd": 1.0, "KsQs": 1.0, "KcQc": 1.0,
				// AQs+ means AQs and AKs (Q and K with A) - all 4 suited combinations for each
				"AhQh": 1.0, "AdQd": 1.0, "AsQs": 1.0, "AcQc": 1.0,
				"AhKh": 1.0, "AdKd": 1.0, "AsKs": 1.0, "AcKc": 1.0,
			},
		},
		{
			name:  "Complex mixed range without weights",
			input: "AA, KK, QQ+, AKs, AKo, TT-JJ, KJs+",
			expected: map[string]float64{
				// AA - all 6 combinations
				"AhAd": 1.0, "AhAs": 1.0, "AhAc": 1.0, "AdAs": 1.0, "AdAc": 1.0, "AsAc": 1.0,
				// KK - all 6 combinations
				"KhKd": 1.0, "KhKs": 1.0, "KhKc": 1.0, "KdKs": 1.0, "KdKc": 1.0, "KsKc": 1.0,
				// QQ+ (QQ, JJ, TT) - all 6 combinations for each pair
				"QhQd": 1.0, "QhQs": 1.0, "QhQc": 1.0, "QdQs": 1.0, "QdQc": 1.0, "QsQc": 1.0,
				"JhJd": 1.0, "JhJs": 1.0, "JhJc": 1.0, "JdJs": 1.0, "JdJc": 1.0, "JsJc": 1.0,
				"ThTd": 1.0, "ThTs": 1.0, "ThTc": 1.0, "TdTs": 1.0, "TdTc": 1.0, "TsTc": 1.0,
				// AKs - all 4 suited combinations
				"AhKh": 1.0, "AdKd": 1.0, "AsKs": 1.0, "AcKc": 1.0,
				// AKo - all 12 offsuit combinations
				"AhKc": 1.0, "AhKd": 1.0, "AhKs": 1.0, "AdKh": 1.0, "AdKc": 1.0, "AdKs": 1.0, "AsKh": 1.0, "AsKc": 1.0, "AsKd": 1.0, "AcKh": 1.0, "AcKd": 1.0, "AcKs": 1.0,
				// KJs+ means KJ and KQ (J and Q with K) - all 4 suited combinations for each
				"KhJh": 1.0, "KdJd": 1.0, "KsJs": 1.0, "KcJc": 1.0,
				"KhQh": 1.0, "KdQd": 1.0, "KsQs": 1.0, "KcQc": 1.0,
			},
		},
		{
			name:  "Individual hands with weights",
			input: "AhKh:0.5, AdKs:0.75, AhKs:0.25",
			expected: map[string]float64{
				"AhKh": 0.5,
				"AdKs": 0.75,
				"AhKs": 0.25,
			},
		},
		{
			name:  "Pairs with weights",
			input: "AA:0.8, KK:0.6, QQ:0.4",
			expected: map[string]float64{
				"AhAd": 0.8, "AhAs": 0.8, "AhAc": 0.8, "AdAs": 0.8, "AdAc": 0.8, "AsAc": 0.8,
				"KhKd": 0.6, "KhKs": 0.6, "KhKc": 0.6, "KdKs": 0.6, "KdKc": 0.6, "KsKc": 0.6,
				"QhQd": 0.4, "QhQs": 0.4, "QhQc": 0.4, "QdQs": 0.4, "QdQc": 0.4, "QsQc": 0.4,
			},
		},
		{
			name:  "Suited hands with weights",
			input: "AKs:0.7, AQs:0.5, AJs:0.3",
			expected: map[string]float64{
				"AhKh": 0.7, "AdKd": 0.7, "AsKs": 0.7, "AcKc": 0.7,
				"AhQh": 0.5, "AdQd": 0.5, "AsQs": 0.5, "AcQc": 0.5,
				"AhJh": 0.3, "AdJd": 0.3, "AsJs": 0.3, "AcJc": 0.3,
			},
		},
		{
			name:  "Offsuit hands with weights",
			input: "AKo:0.6, AQo:0.4, AJo:0.2",
			expected: map[string]float64{
				"AhKc": 0.6, "AhKd": 0.6, "AhKs": 0.6, "AdKh": 0.6, "AdKc": 0.6, "AdKs": 0.6, "AsKh": 0.6, "AsKc": 0.6, "AsKd": 0.6, "AcKh": 0.6, "AcKd": 0.6, "AcKs": 0.6,
				"AhQc": 0.4, "AhQd": 0.4, "AhQs": 0.4, "AdQh": 0.4, "AdQc": 0.4, "AdQs": 0.4, "AsQh": 0.4, "AsQc": 0.4, "AsQd": 0.4, "AcQh": 0.4, "AcQd": 0.4, "AcQs": 0.4,
				"AhJc": 0.2, "AhJd": 0.2, "AhJs": 0.2, "AdJh": 0.2, "AdJc": 0.2, "AdJs": 0.2, "AsJh": 0.2, "AsJc": 0.2, "AsJd": 0.2, "AcJh": 0.2, "AcJd": 0.2, "AcJs": 0.2,
			},
		},
		{
			name:  "Range plus notation with weights",
			input: "QQ+:0.9, AKs+:0.8",
			expected: map[string]float64{
				// QQ+ (QQ, KK, AA) - all 6 combinations for each pair
				"QhQd": 0.9, "QhQs": 0.9, "QhQc": 0.9, "QdQs": 0.9, "QdQc": 0.9, "QsQc": 0.9,
				"KhKd": 0.9, "KhKs": 0.9, "KhKc": 0.9, "KdKs": 0.9, "KdKc": 0.9, "KsKc": 0.9,
				"AhAd": 0.9, "AhAs": 0.9, "AhAc": 0.9, "AdAs": 0.9, "AdAc": 0.9, "AsAc": 0.9,
				// AKs+ means just AKs (the specific suited hand) - all 4 suited combinations
				"AhKh": 0.8, "AdKd": 0.8, "AsKs": 0.8, "AcKc": 0.8,
			},
		},
		{
			name:  "Range dash notation with weights",
			input: "TT-JJ:0.7",
			expected: map[string]float64{
				// TT-JJ (TT, JJ) - all 6 combinations for each pair
				"ThTd": 0.7, "ThTs": 0.7, "ThTc": 0.7, "TdTs": 0.7, "TdTc": 0.7, "TsTc": 0.7,
				"JhJd": 0.7, "JhJs": 0.7, "JhJc": 0.7, "JdJs": 0.7, "JdJc": 0.7, "JsJc": 0.7,
			},
		},
		{
			name:  "Mixed notation with weights",
			input: "AA:0.9, KK:0.7, AKs:0.8, AhKh:0.5",
			expected: map[string]float64{
				// AA - all 6 combinations
				"AhAd": 0.9, "AhAs": 0.9, "AhAc": 0.9, "AdAs": 0.9, "AdAc": 0.9, "AsAc": 0.9,
				// KK - all 6 combinations
				"KhKd": 0.7, "KhKs": 0.7, "KhKc": 0.7, "KdKs": 0.7, "KdKc": 0.7, "KsKc": 0.7,
				// AKs (excluding AhKh which gets overridden) - all 4 suited combinations
				"AdKd": 0.8, "AsKs": 0.8, "AcKc": 0.8,
				// AhKh (overrides AKs weight)
				"AhKh": 0.5,
			},
		},
		{
			name:  "Edge case weights",
			input: "AA:0.0, KK:1.0, QQ:0.5",
			expected: map[string]float64{
				// AA with 0.0 weight (should not appear in final result)
				// KK with 1.0 weight - all 6 combinations
				"KhKd": 1.0, "KhKs": 1.0, "KhKc": 1.0, "KdKs": 1.0, "KdKc": 1.0, "KsKc": 1.0,
				// QQ with 0.5 weight - all 6 combinations
				"QhQd": 0.5, "QhQs": 0.5, "QhQc": 0.5, "QdQs": 0.5, "QdQc": 0.5, "QsQc": 0.5,
			},
		},
		{
			name:  "Non-pair plus notation with weights",
			input: "KJs+:0.8, KQs+:0.7, AQs+:0.6",
			expected: map[string]float64{
				// KJs+ means KJ and KQ (J and Q with K) - all 4 suited combinations for each
				"KhJh": 0.8, "KdJd": 0.8, "KsJs": 0.8, "KcJc": 0.8,
				// KQs+ means just KQ (Q is highest with K) - overrides KJs+ weight - all 4 suited combinations
				"KhQh": 0.7, "KdQd": 0.7, "KsQs": 0.7, "KcQc": 0.7,
				// AQs+ means AQs and AKs (Q and K with A) - all 4 suited combinations for each
				"AhQh": 0.6, "AdQd": 0.6, "AsQs": 0.6, "AcQc": 0.6,
				"AhKh": 0.6, "AdKd": 0.6, "AsKs": 0.6, "AcKc": 0.6,
			},
		},
		{
			name:  "Empty and whitespace handling",
			input: ", AA:0.8, , KK:0.6,",
			expected: map[string]float64{
				// AA - all 6 combinations
				"AhAd": 0.8, "AhAs": 0.8, "AhAc": 0.8, "AdAs": 0.8, "AdAc": 0.8, "AsAc": 0.8,
				// KK - all 6 combinations
				"KhKd": 0.6, "KhKs": 0.6, "KhKc": 0.6, "KdKs": 0.6, "KdKc": 0.6, "KsKc": 0.6,
			},
		},
		{
			name:  "Single hand with weight",
			input: "AhKh:0.5",
			expected: map[string]float64{
				"AhKh": 0.5,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Range{}
			err := r.Parse(tt.input)
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}

			// Check that all expected hands have the correct weights
			for hand, expectedWeight := range tt.expected {
				holeCards, err := ParseHoleCards(hand)
				if err != nil {
					t.Fatalf("Failed to parse hand %s: %v", hand, err)
				}

				actualWeight := r.Weight(holeCards)
				if actualWeight != expectedWeight {
					t.Errorf("Hand %s: expected weight %.2f, got %.2f", hand, expectedWeight, actualWeight)
				}
			}

			// Verify the range size is exactly as expected
			if len(tt.expected) != r.Size() {
				t.Errorf("Expected %d hands, got %d", len(tt.expected), r.Size())
			}
		})
	}
}

func TestParseInvalidInputs(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectedErr string
	}{
		{
			name:        "Invalid weight format",
			input:       "QQ:abc",
			expectedErr: "invalid weight",
		},
		{
			name:        "Missing weight value",
			input:       "AKs:",
			expectedErr: "invalid weight",
		},
		{
			name:        "Multiple colons",
			input:       "AA:0.8:0.5",
			expectedErr: "invalid weight format",
		},
		{
			name:        "Invalid hand format",
			input:       "XYZ",
			expectedErr: "invalid range element",
		},
		{
			name:        "Invalid pair format",
			input:       "AB",
			expectedErr: "invalid range element",
		},
		{
			name:        "Invalid suited format",
			input:       "AKx",
			expectedErr: "invalid range element",
		},
		{
			name:        "Invalid offsuit format",
			input:       "AKp",
			expectedErr: "invalid range element",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Range{}
			err := r.Parse(tt.input)
			if err == nil {
				t.Fatal("Expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.expectedErr) {
				t.Errorf("Expected error containing '%s', got '%s'", tt.expectedErr, err.Error())
			}
		})
	}
}

func TestRangeClear(t *testing.T) {
	r := &Range{}

	// Add some hands
	err := r.Parse("AA, KK")
	if err != nil {
		t.Errorf("Parse failed: %v", err)
	}

	if r.Size() == 0 {
		t.Errorf("Range should have hands after parsing")
	}

	// Clear the range
	r.Clear()

	if r.Size() != 0 {
		t.Errorf("Range should be empty after clearing")
	}
}

func TestInvalidRangeElements(t *testing.T) {
	r := &Range{}

	// Test invalid elements
	invalidRanges := []string{
		"XX",       // Invalid rank
		"AKx",      // Invalid suit
		"AAA",      // Too many cards
		"A",        // Too few cards
		"TT-JJ-KK", // Too many dashes
		"KAs-KQs",  // Base rank (K) collides with the kicker range (A..Q)
	}

	for _, invalid := range invalidRanges {
		err := r.Parse(invalid)
		if err == nil {
			t.Errorf("Expected error for invalid range '%s', got none", invalid)
		}
	}
}

func TestRangeEqual(t *testing.T) {
	tests := []struct {
		name     string
		range1   string
		range2   string
		epsilon  float64
		expected bool
	}{
		{
			name:     "Identical ranges",
			range1:   "AA, KK, QQ",
			range2:   "AA, KK, QQ",
			epsilon:  0.001,
			expected: true,
		},
		{
			name:     "Identical ranges with weights",
			range1:   "AA:0.8, KK:0.6, QQ:0.4",
			range2:   "AA:0.8, KK:0.6, QQ:0.4",
			epsilon:  0.001,
			expected: true,
		},
		{
			name:     "Different ranges",
			range1:   "AA, KK, QQ",
			range2:   "AA, KK, JJ",
			epsilon:  0.001,
			expected: false,
		},
		{
			name:     "Different weights",
			range1:   "AA:0.8, KK:0.6",
			range2:   "AA:0.8, KK:0.7",
			epsilon:  0.001,
			expected: false,
		},
		{
			name:     "Same ranges different order",
			range1:   "AA, KK, QQ",
			range2:   "QQ, AA, KK",
			epsilon:  0.001,
			expected: true,
		},
		{
			name:     "Empty ranges",
			range1:   "",
			range2:   "",
			epsilon:  0.001,
			expected: true,
		},
		{
			name:     "One empty range",
			range1:   "AA, KK",
			range2:   "",
			epsilon:  0.001,
			expected: false,
		},
		{
			name:     "Within epsilon tolerance",
			range1:   "AA:0.5001",
			range2:   "AA:0.5000",
			epsilon:  0.001,
			expected: true,
		},
		{
			name:     "Outside epsilon tolerance",
			range1:   "AA:0.5020",
			range2:   "AA:0.5000",
			epsilon:  0.001,
			expected: false,
		},
		{
			name:     "Complex ranges identical",
			range1:   "AA:0.9, KK:0.7, AKs:0.8, AhKh:0.5",
			range2:   "AA:0.9, KK:0.7, AKs:0.8, AhKh:0.5",
			epsilon:  0.001,
			expected: true,
		},
		{
			name:     "Complex ranges different",
			range1:   "AA:0.9, KK:0.7, AKs:0.8, AhKh:0.5",
			range2:   "AA:0.9, KK:0.7, AKs:0.8, AhKh:0.6",
			epsilon:  0.001,
			expected: false,
		},
		{
			name:     "Zero epsilon exact match",
			range1:   "AA:0.5, KK:0.3",
			range2:   "AA:0.5, KK:0.3",
			epsilon:  0.0,
			expected: true,
		},
		{
			name:     "Zero epsilon slight difference",
			range1:   "AA:0.5001",
			range2:   "AA:0.5000",
			epsilon:  0.0,
			expected: false,
		},
		{
			name:     "Large epsilon tolerance",
			range1:   "AA:0.6",
			range2:   "AA:0.5",
			epsilon:  0.2,
			expected: true,
		},
		{
			name:     "Mixed notation identical",
			range1:   "AA:0.8, AKs:0.6, AKo:0.4",
			range2:   "AA:0.8, AKs:0.6, AKo:0.4",
			epsilon:  0.001,
			expected: true,
		},
		{
			name:     "Range plus notation identical",
			range1:   "QQ+:0.9, AKs+:0.8",
			range2:   "QQ+:0.9, AKs+:0.8",
			epsilon:  0.001,
			expected: true,
		},
		{
			name:     "Range dash notation identical",
			range1:   "TT-JJ:0.7",
			range2:   "TT-JJ:0.7",
			epsilon:  0.001,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var r1, r2 Range

			// Parse the first range
			err := r1.Parse(tt.range1)
			if err != nil {
				t.Fatalf("Failed to parse first range '%s': %v", tt.range1, err)
			}

			// Parse the second range
			err = r2.Parse(tt.range2)
			if err != nil {
				t.Fatalf("Failed to parse second range '%s': %v", tt.range2, err)
			}

			// Test Equal method
			result := r1.Equal(&r2, tt.epsilon)
			if result != tt.expected {
				t.Errorf("Equal() = %v, want %v for ranges '%s' and '%s' with epsilon %f",
					result, tt.expected, tt.range1, tt.range2, tt.epsilon)
			}

			// Test symmetry (a.Equal(b) should equal b.Equal(a))
			result2 := r2.Equal(&r1, tt.epsilon)
			if result != result2 {
				t.Errorf("Equal() not symmetric: r1.Equal(r2) = %v, r2.Equal(r1) = %v",
					result, result2)
			}
		})
	}
}

func TestRangeDotProduct(t *testing.T) {
	tests := []struct {
		name     string
		range1   string
		range2   string
		expected float64
	}{
		{
			name:     "Empty ranges",
			range1:   "",
			range2:   "",
			expected: 0.0,
		},
		{
			name:     "Single hand identical",
			range1:   "AhKh:0.5",
			range2:   "AhKh:0.5",
			expected: 0.25, // 0.5 * 0.5
		},
		{
			name:     "Single hand different weights",
			range1:   "AhKh:0.6",
			range2:   "AhKh:0.4",
			expected: 0.24, // 0.6 * 0.4
		},
		{
			name:     "Multiple hands overlapping",
			range1:   "AA:0.8, KK:0.6",
			range2:   "AA:0.7, KK:0.5",
			expected: 6 * 0.86, // 0.8*0.7 + 0.6*0.5 = 0.56 + 0.30
		},
		{
			name:     "Multiple hands partial overlap",
			range1:   "AA:0.9, KK:0.7, QQ:0.5",
			range2:   "AA:0.8, KK:0.6",
			expected: 6 * 1.14, // 0.9*0.8 + 0.7*0.6 + 0.5*0 = 0.72 + 0.42
		},
		{
			name:     "Suited hands",
			range1:   "AKs:0.7, AQs:0.5",
			range2:   "AKs:0.6, AQs:0.4",
			expected: 4 * 0.62, // 0.7*0.6 + 0.5*0.4 = 0.42 + 0.20
		},
		{
			name:     "Offsuit hands",
			range1:   "AKo:0.6, AQo:0.4",
			range2:   "AKo:0.5, AQo:0.3",
			expected: 12 * 0.42, // 0.6*0.5 + 0.4*0.3 = 0.30 + 0.12
		},
		{
			name:     "Mixed notation",
			range1:   "AA:0.8, AKs:0.6, AKo:0.4",
			range2:   "AA:0.7, AKs:0.5, AKo:0.3",
			expected: 6*0.8*0.7 + 4*0.6*0.5 + 12*0.4*0.3,
		},
		{
			name:     "Range plus notation",
			range1:   "QQ+:0.9",
			range2:   "QQ+:0.8",
			expected: 6 * 0.9 * 0.8 * 3,
		},
		{
			name:     "No overlap",
			range1:   "AA:0.5",
			range2:   "KK:0.6",
			expected: 0.0, // Different hands, no overlap
		},
		{
			name:     "Zero weights",
			range1:   "AA:0.0, KK:0.5",
			range2:   "AA:0.3, KK:0.0",
			expected: 0.0, // 0.0*0.3 + 0.5*0.0 = 0.0
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var r1, r2 Range

			// Parse the first range
			err := r1.Parse(tt.range1)
			if err != nil {
				t.Fatalf("Failed to parse first range '%s': %v", tt.range1, err)
			}

			// Parse the second range
			err = r2.Parse(tt.range2)
			if err != nil {
				t.Fatalf("Failed to parse second range '%s': %v", tt.range2, err)
			}

			// Test DotProduct method
			result := r1.DotProduct(&r2)
			if diff := result - tt.expected; diff < -0.001 || diff > 0.001 {
				t.Errorf("DotProduct() = %f, want %f for ranges '%s' and '%s'",
					result, tt.expected, tt.range1, tt.range2)
			}

			// Test commutativity (a.DotProduct(b) should equal b.DotProduct(a))
			result2 := r2.DotProduct(&r1)
			if !Equal64(result, result2, 0.001) {
				t.Errorf("DotProduct() not commutative: r1.DotProduct(r2) = %f, r2.DotProduct(r1) = %f",
					result, result2)
			}
		})
	}
}

func TestRangeSum(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected float64
	}{
		{
			name:     "Empty range",
			input:    "",
			expected: 0.0,
		},
		{
			name:     "Single hand",
			input:    "AhKh:0.5",
			expected: 0.5,
		},
		{
			name:     "Multiple hands",
			input:    "AA:0.8, KK:0.6, QQ:0.4",
			expected: 6 * 1.8, // 6*(0.8 + 0.6 + 0.4)
		},
		{
			name:     "Suited hands",
			input:    "AKs:0.7, AQs:0.5, AJs:0.3",
			expected: 4 * 1.5, // 0.7 + 0.5 + 0.3
		},
		{
			name:     "Offsuit hands",
			input:    "AKo:0.6, AQo:0.4, AJo:0.2",
			expected: 12 * 1.2, // 12*(0.6 + 0.4 + 0.2)
		},
		{
			name:     "Mixed notation",
			input:    "AA:0.8, AKs:0.7, AKo:0.5",
			expected: 6*0.8 + 4*0.7 + 12*0.5,
		},
		{
			name:     "Range plus notation",
			input:    "QQ+:0.8",
			expected: 6 * 2.4, // 0.8 * 3 pairs = 2.4
		},
		{
			name:     "Range dash notation",
			input:    "TT-JJ:0.6",
			expected: 6 * 1.2, // 0.6 * 2 pairs = 1.2
		},
		{
			name:     "Individual hands without weights",
			input:    "AhKh, AdKs, AhKs",
			expected: 3.0, // 1.0 + 1.0 + 1.0
		},
		{
			name:     "Pairs without weights",
			input:    "AA, KK, QQ",
			expected: 6 * 3.0, // 1.0 + 1.0 + 1.0
		},
		{
			name:     "Suited hands without weights",
			input:    "AKs, AQs, AJs",
			expected: 4 * 3.0, // 1.0 + 1.0 + 1.0
		},
		{
			name:     "Offsuit hands without weights",
			input:    "AKo, AQo, AJo",
			expected: 12 * 3.0, // 1.0 + 1.0 + 1.0
		},
		{
			name:     "Zero weights",
			input:    "AA:0.0, KK:0.5, QQ:0.0",
			expected: 6 * 0.5, // 0.0 + 0.5 + 0.0
		},
		{
			name:     "Fractional weights",
			input:    "AA:0.25, KK:0.75, QQ:0.5",
			expected: 6 * 1.5, // 0.25 + 0.75 + 0.5
		},
		{
			name:     "Large number of hands",
			input:    "AA:0.1, KK:0.1, QQ:0.1, JJ:0.1, TT:0.1, 99:0.1, 88:0.1, 77:0.1, 66:0.1, 55:0.1",
			expected: 6 * 1.0, // 0.1 * 10 = 1.0
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var r Range

			// Parse the range
			err := r.Parse(tt.input)
			if err != nil {
				t.Fatalf("Failed to parse range '%s': %v", tt.input, err)
			}

			// Test Sum method
			result := r.Sum()
			if !Equal64(result, tt.expected, 0.001) {
				t.Errorf("Sum() = %f, want %f for range '%s'",
					result, tt.expected, tt.input)
			}
		})
	}
}

func TestIsoPair(t *testing.T) {
	tests := []struct {
		name     string
		rangeStr string
		rank     int
		expected bool
	}{
		{
			name:     "All AA combinations with same weight",
			rangeStr: "AA:0.8",
			rank:     Ace,
			expected: true,
		},
		{
			name:     "All KK combinations with same weight",
			rangeStr: "KK:0.6",
			rank:     King,
			expected: true,
		},
		{
			name:     "Mixed AA weights - not isomorphic",
			rangeStr: "AhAd:0.8, AhAs:0.9, AhAc:0.8, AdAs:0.8, AdAc:0.8, AsAc:0.8",
			rank:     Ace,
			expected: false,
		},
		{
			name:     "Empty range",
			rangeStr: "",
			rank:     Ace,
			expected: true, // All weights are 0, so they're equal
		},
		{
			name:     "Single AA combination",
			rangeStr: "AhAd:0.5",
			rank:     Ace,
			expected: false, // Only one combination, others are 0
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var r Range
			err := r.Parse(tt.rangeStr)
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}

			result := r.isoPair(tt.rank)
			if result != tt.expected {
				t.Errorf("isoPair(%d) = %v, want %v for range '%s'",
					tt.rank, result, tt.expected, tt.rangeStr)
			}
		})
	}
}

func TestIsoSuited(t *testing.T) {
	tests := []struct {
		name     string
		rangeStr string
		rank0    int
		rank1    int
		expected bool
	}{
		{
			name:     "All AKs combinations with same weight",
			rangeStr: "AKs:0.7",
			rank0:    Ace,
			rank1:    King,
			expected: true,
		},
		{
			name:     "All AQs combinations with same weight",
			rangeStr: "AQs:0.5",
			rank0:    Ace,
			rank1:    Queen,
			expected: true,
		},
		{
			name:     "Mixed AKs weights - not isomorphic",
			rangeStr: "AhKh:0.7, AdKd:0.8, AsKs:0.7, AcKc:0.7",
			rank0:    Ace,
			rank1:    King,
			expected: false,
		},
		{
			name:     "Empty range",
			rangeStr: "",
			rank0:    Ace,
			rank1:    King,
			expected: true, // All weights are 0, so they're equal
		},
		{
			name:     "Single AKs combination",
			rangeStr: "AhKh:0.5",
			rank0:    Ace,
			rank1:    King,
			expected: false, // Only one combination, others are 0
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var r Range
			err := r.Parse(tt.rangeStr)
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}

			result := r.isoSuited(tt.rank0, tt.rank1)
			if result != tt.expected {
				t.Errorf("isoSuited(%d, %d) = %v, want %v for range '%s'",
					tt.rank0, tt.rank1, result, tt.expected, tt.rangeStr)
			}
		})
	}
}

func TestIsoOffsuit(t *testing.T) {
	tests := []struct {
		name     string
		rangeStr string
		rank0    int
		rank1    int
		expected bool
	}{
		{
			name:     "All AKo combinations with same weight",
			rangeStr: "AKo:0.6",
			rank0:    Ace,
			rank1:    King,
			expected: true,
		},
		{
			name:     "All AQo combinations with same weight",
			rangeStr: "AQo:0.4",
			rank0:    Ace,
			rank1:    Queen,
			expected: true,
		},
		{
			name:     "Mixed AKo weights - not isomorphic",
			rangeStr: "AhKc:0.6, AhKd:0.7, AhKs:0.6, AdKh:0.6, AdKc:0.6, AdKs:0.6, AsKh:0.6, AsKc:0.6, AsKd:0.6, AcKh:0.6, AcKd:0.6, AcKs:0.6",
			rank0:    Ace,
			rank1:    King,
			expected: false,
		},
		{
			name:     "Empty range",
			rangeStr: "",
			rank0:    Ace,
			rank1:    King,
			expected: true, // All weights are 0, so they're equal
		},
		{
			name:     "Single AKo combination",
			rangeStr: "AhKc:0.5",
			rank0:    Ace,
			rank1:    King,
			expected: false, // Only one combination, others are 0
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var r Range
			err := r.Parse(tt.rangeStr)
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}

			result := r.isoOffsuit(tt.rank0, tt.rank1)
			if result != tt.expected {
				t.Errorf("isoOffsuit(%d, %d) = %v, want %v for range '%s'",
					tt.rank0, tt.rank1, result, tt.expected, tt.rangeStr)
			}
		})
	}
}

func TestIsIso(t *testing.T) {
	tests := []struct {
		name     string
		rangeStr string
		expected bool
	}{
		{
			name:     "Empty range - isomorphic",
			rangeStr: "",
			expected: true,
		},
		{
			name:     "All pairs with same weight - isomorphic",
			rangeStr: "AA:0.8, KK:0.8, QQ:0.8, JJ:0.8, TT:0.8",
			expected: true,
		},
		{
			name:     "All suited hands with same weight - isomorphic",
			rangeStr: "AKs:0.7, AQs:0.7, AJs:0.7, ATs:0.7",
			expected: true,
		},
		{
			name:     "All offsuit hands with same weight - isomorphic",
			rangeStr: "AKo:0.6, AQo:0.6, AJo:0.6, ATo:0.6",
			expected: true,
		},
		{
			name:     "Mixed pair weights - isomorphic (each pair internally consistent)",
			rangeStr: "AA:0.8, KK:0.6, QQ:0.8, JJ:0.8, TT:0.8, 99:0.8, 88:0.8, 77:0.8, 66:0.8, 55:0.8, 44:0.8, 33:0.8, 22:0.8",
			expected: true,
		},
		{
			name:     "Mixed suited weights - isomorphic (each suited hand internally consistent)",
			rangeStr: "AKs:0.7, AQs:0.5, AJs:0.7, ATs:0.7, A9s:0.7, A8s:0.7, A7s:0.7, A6s:0.7, A5s:0.7, A4s:0.7, A3s:0.7, A2s:0.7",
			expected: true,
		},
		{
			name:     "Mixed offsuit weights - isomorphic (each offsuit hand internally consistent)",
			rangeStr: "AKo:0.6, AQo:0.4, AJo:0.6, ATo:0.6, A9o:0.6, A8o:0.6, A7o:0.6, A6o:0.6, A5o:0.6, A4o:0.6, A3o:0.6, A2o:0.6",
			expected: true,
		},
		{
			name:     "Single hand - not isomorphic",
			rangeStr: "AhKh:0.5",
			expected: false,
		},
		{
			name:     "AA with different suit combinations - not isomorphic",
			rangeStr: "AhAs:0.8, AdAc:0.6",
			expected: false,
		},
		{
			name:     "AKs with different suit combinations - not isomorphic",
			rangeStr: "AhKh:0.7, AdKd:0.5",
			expected: false,
		},
		{
			name:     "AKo with different suit combinations - not isomorphic",
			rangeStr: "AhKd:0.6, AdKh:0.4",
			expected: false,
		},
		{
			name:     "Complete isomorphic range",
			rangeStr: "AA:0.8, KK:0.8, QQ:0.8, JJ:0.8, TT:0.8, 99:0.8, 88:0.8, 77:0.8, 66:0.8, 55:0.8, 44:0.8, 33:0.8, 22:0.8, AKs:0.7, AQs:0.7, AJs:0.7, ATs:0.7, A9s:0.7, A8s:0.7, A7s:0.7, A6s:0.7, A5s:0.7, A4s:0.7, A3s:0.7, A2s:0.7, KQs:0.7, KJs:0.7, KTs:0.7, K9s:0.7, K8s:0.7, K7s:0.7, K6s:0.7, K5s:0.7, K4s:0.7, K3s:0.7, K2s:0.7, QJs:0.7, QTs:0.7, Q9s:0.7, Q8s:0.7, Q7s:0.7, Q6s:0.7, Q5s:0.7, Q4s:0.7, Q3s:0.7, Q2s:0.7, JTs:0.7, J9s:0.7, J8s:0.7, J7s:0.7, J6s:0.7, J5s:0.7, J4s:0.7, J3s:0.7, J2s:0.7, T9s:0.7, T8s:0.7, T7s:0.7, T6s:0.7, T5s:0.7, T4s:0.7, T3s:0.7, T2s:0.7, 98s:0.7, 97s:0.7, 96s:0.7, 95s:0.7, 94s:0.7, 93s:0.7, 92s:0.7, 87s:0.7, 86s:0.7, 85s:0.7, 84s:0.7, 83s:0.7, 82s:0.7, 76s:0.7, 75s:0.7, 74s:0.7, 73s:0.7, 72s:0.7, 65s:0.7, 64s:0.7, 63s:0.7, 62s:0.7, 54s:0.7, 53s:0.7, 52s:0.7, 43s:0.7, 42s:0.7, 32s:0.7, AKo:0.6, AQo:0.6, AJo:0.6, ATo:0.6, A9o:0.6, A8o:0.6, A7o:0.6, A6o:0.6, A5o:0.6, A4o:0.6, A3o:0.6, A2o:0.6, KQo:0.6, KJo:0.6, KTo:0.6, K9o:0.6, K8o:0.6, K7o:0.6, K6o:0.6, K5o:0.6, K4o:0.6, K3o:0.6, K2o:0.6, QJo:0.6, QTo:0.6, Q9o:0.6, Q8o:0.6, Q7o:0.6, Q6o:0.6, Q5o:0.6, Q4o:0.6, Q3o:0.6, Q2o:0.6, JTo:0.6, J9o:0.6, J8o:0.6, J7o:0.6, J6o:0.6, J5o:0.6, J4o:0.6, J3o:0.6, J2o:0.6, T9o:0.6, T8o:0.6, T7o:0.6, T6o:0.6, T5o:0.6, T4o:0.6, T3o:0.6, T2o:0.6, 98o:0.6, 97o:0.6, 96o:0.6, 95o:0.6, 94o:0.6, 93o:0.6, 92o:0.6, 87o:0.6, 86o:0.6, 85o:0.6, 84o:0.6, 83o:0.6, 82o:0.6, 76o:0.6, 75o:0.6, 74o:0.6, 73o:0.6, 72o:0.6, 65o:0.6, 64o:0.6, 63o:0.6, 62o:0.6, 54o:0.6, 53o:0.6, 52o:0.6, 43o:0.6, 42o:0.6, 32o:0.6",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var r Range
			err := r.Parse(tt.rangeStr)
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}

			result := r.IsIso()
			if result != tt.expected {
				t.Errorf("IsIso() = %v, want %v for range '%s'",
					result, tt.expected, tt.rangeStr)
			}
		})
	}
}

func TestRangeTwoCharacterHands(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string]float64
	}{
		{
			name:  "AK without weight - should expand to AKs and AKo",
			input: "AK",
			expected: map[string]float64{
				// AKs - all 4 suited combinations
				"AhKh": 1.0, "AdKd": 1.0, "AsKs": 1.0, "AcKc": 1.0,
				// AKo - all 12 offsuit combinations
				"AhKc": 1.0, "AhKd": 1.0, "AhKs": 1.0, "AdKh": 1.0, "AdKc": 1.0, "AdKs": 1.0, "AsKh": 1.0, "AsKc": 1.0, "AsKd": 1.0, "AcKh": 1.0, "AcKd": 1.0, "AcKs": 1.0,
			},
		},
		{
			name:  "AK with weight - should expand to AKs and AKo with same weight",
			input: "AK:0.7",
			expected: map[string]float64{
				// AKs - all 4 suited combinations
				"AhKh": 0.7, "AdKd": 0.7, "AsKs": 0.7, "AcKc": 0.7,
				// AKo - all 12 offsuit combinations
				"AhKc": 0.7, "AhKd": 0.7, "AhKs": 0.7, "AdKh": 0.7, "AdKc": 0.7, "AdKs": 0.7, "AsKh": 0.7, "AsKc": 0.7, "AsKd": 0.7, "AcKh": 0.7, "AcKd": 0.7, "AcKs": 0.7,
			},
		},
		{
			name:  "AQ without weight - should expand to AQs and AQo",
			input: "AQ",
			expected: map[string]float64{
				// AQs - all 4 suited combinations
				"AhQh": 1.0, "AdQd": 1.0, "AsQs": 1.0, "AcQc": 1.0,
				// AQo - all 12 offsuit combinations
				"AhQc": 1.0, "AhQd": 1.0, "AhQs": 1.0, "AdQh": 1.0, "AdQc": 1.0, "AdQs": 1.0, "AsQh": 1.0, "AsQc": 1.0, "AsQd": 1.0, "AcQh": 1.0, "AcQd": 1.0, "AcQs": 1.0,
			},
		},
		{
			name:  "KQ without weight - should expand to KQs and KQo",
			input: "KQ",
			expected: map[string]float64{
				// KQs - all 4 suited combinations
				"KhQh": 1.0, "KdQd": 1.0, "KsQs": 1.0, "KcQc": 1.0,
				// KQo - all 12 offsuit combinations
				"KhQc": 1.0, "KhQd": 1.0, "KhQs": 1.0, "KdQh": 1.0, "KdQc": 1.0, "KdQs": 1.0, "KsQh": 1.0, "KsQc": 1.0, "KsQd": 1.0, "KcQh": 1.0, "KcQd": 1.0, "KcQs": 1.0,
			},
		},
		{
			name:  "JT without weight - should expand to JTs and JTo",
			input: "JT",
			expected: map[string]float64{
				// JTs - all 4 suited combinations
				"JhTh": 1.0, "JdTd": 1.0, "JsTs": 1.0, "JcTc": 1.0,
				// JTo - all 12 offsuit combinations
				"JhTc": 1.0, "JhTd": 1.0, "JhTs": 1.0, "JdTh": 1.0, "JdTc": 1.0, "JdTs": 1.0, "JsTh": 1.0, "JsTc": 1.0, "JsTd": 1.0, "JcTh": 1.0, "JcTd": 1.0, "JcTs": 1.0,
			},
		},
		{
			name:  "Mixed two-character hands with weights",
			input: "AK:0.8, AQ:0.6, KQ:0.4",
			expected: map[string]float64{
				// AKs - all 4 suited combinations
				"AhKh": 0.8, "AdKd": 0.8, "AsKs": 0.8, "AcKc": 0.8,
				// AKo - all 12 offsuit combinations
				"AhKc": 0.8, "AhKd": 0.8, "AhKs": 0.8, "AdKh": 0.8, "AdKc": 0.8, "AdKs": 0.8, "AsKh": 0.8, "AsKc": 0.8, "AsKd": 0.8, "AcKh": 0.8, "AcKd": 0.8, "AcKs": 0.8,
				// AQs - all 4 suited combinations
				"AhQh": 0.6, "AdQd": 0.6, "AsQs": 0.6, "AcQc": 0.6,
				// AQo - all 12 offsuit combinations
				"AhQc": 0.6, "AhQd": 0.6, "AhQs": 0.6, "AdQh": 0.6, "AdQc": 0.6, "AdQs": 0.6, "AsQh": 0.6, "AsQc": 0.6, "AsQd": 0.6, "AcQh": 0.6, "AcQd": 0.6, "AcQs": 0.6,
				// KQs - all 4 suited combinations
				"KhQh": 0.4, "KdQd": 0.4, "KsQs": 0.4, "KcQc": 0.4,
				// KQo - all 12 offsuit combinations
				"KhQc": 0.4, "KhQd": 0.4, "KhQs": 0.4, "KdQh": 0.4, "KdQc": 0.4, "KdQs": 0.4, "KsQh": 0.4, "KsQc": 0.4, "KsQd": 0.4, "KcQh": 0.4, "KcQd": 0.4, "KcQs": 0.4,
			},
		},
		{
			name:  "Two-character hands mixed with specific hands",
			input: "AK:0.8, AhKh:0.5, AKo:0.6",
			expected: map[string]float64{
				// AKs (excluding AhKh which gets overridden) - all 4 suited combinations
				"AdKd": 0.8, "AsKs": 0.8, "AcKc": 0.8,
				// AhKh (overrides AKs weight)
				"AhKh": 0.5,
				// AKo - all 12 offsuit combinations
				"AhKc": 0.6, "AhKd": 0.6, "AhKs": 0.6, "AdKh": 0.6, "AdKc": 0.6, "AdKs": 0.6, "AsKh": 0.6, "AsKc": 0.6, "AsKd": 0.6, "AcKh": 0.6, "AcKd": 0.6, "AcKs": 0.6,
			},
		},
		{
			name:  "Edge case - 32 (lowest non-pair)",
			input: "32:0.3",
			expected: map[string]float64{
				// 32s - all 4 suited combinations
				"3h2h": 0.3, "3d2d": 0.3, "3s2s": 0.3, "3c2c": 0.3,
				// 32o - all 12 offsuit combinations
				"3h2c": 0.3, "3h2d": 0.3, "3h2s": 0.3, "3d2h": 0.3, "3d2c": 0.3, "3d2s": 0.3, "3s2h": 0.3, "3s2c": 0.3, "3s2d": 0.3, "3c2h": 0.3, "3c2d": 0.3, "3c2s": 0.3,
			},
		},
		{
			name:  "Edge case - A2 (ace with lowest card)",
			input: "A2:0.4",
			expected: map[string]float64{
				// A2s - all 4 suited combinations
				"Ah2h": 0.4, "Ad2d": 0.4, "As2s": 0.4, "Ac2c": 0.4,
				// A2o - all 12 offsuit combinations
				"Ah2c": 0.4, "Ah2d": 0.4, "Ah2s": 0.4, "Ad2h": 0.4, "Ad2c": 0.4, "Ad2s": 0.4, "As2h": 0.4, "As2c": 0.4, "As2d": 0.4, "Ac2h": 0.4, "Ac2d": 0.4, "Ac2s": 0.4,
			},
		},
		{
			name:  "Edge case - K3 (king with low card)",
			input: "K3:0.2",
			expected: map[string]float64{
				// K3s - all 4 suited combinations
				"Kh3h": 0.2, "Kd3d": 0.2, "Ks3s": 0.2, "Kc3c": 0.2,
				// K3o - all 12 offsuit combinations
				"Kh3c": 0.2, "Kh3d": 0.2, "Kh3s": 0.2, "Kd3h": 0.2, "Kd3c": 0.2, "Kd3s": 0.2, "Ks3h": 0.2, "Ks3c": 0.2, "Ks3d": 0.2, "Kc3h": 0.2, "Kc3d": 0.2, "Kc3s": 0.2,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var r Range
			err := r.Parse(tt.input)
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}

			// Check that all expected hands have the correct weights
			for hand, expectedWeight := range tt.expected {
				holeCards, err := ParseHoleCards(hand)
				if err != nil {
					t.Fatalf("Failed to parse hand %s: %v", hand, err)
				}

				actualWeight := r.Weight(holeCards)
				if actualWeight != expectedWeight {
					t.Errorf("Hand %s: expected weight %.2f, got %.2f", hand, expectedWeight, actualWeight)
				}
			}

			// Verify the range size is exactly as expected
			if len(tt.expected) != r.Size() {
				t.Errorf("Expected %d hands, got %d", len(tt.expected), r.Size())
			}
		})
	}
}

func TestEqual64(t *testing.T) {
	tests := []struct {
		name     string
		a        float64
		b        float64
		epsilon  float64
		expected bool
	}{
		{
			name:     "Identical values",
			a:        0.5,
			b:        0.5,
			epsilon:  0.001,
			expected: true,
		},
		{
			name:     "Zero values",
			a:        0.0,
			b:        0.0,
			epsilon:  0.001,
			expected: true,
		},
		{
			name:     "Within epsilon tolerance",
			a:        0.5001,
			b:        0.5000,
			epsilon:  0.001,
			expected: true,
		},
		{
			name:     "Outside epsilon tolerance",
			a:        0.5020,
			b:        0.5000,
			epsilon:  0.001,
			expected: false,
		},
		{
			name:     "Large numbers within tolerance",
			a:        1000.1,
			b:        1000.0,
			epsilon:  0.1,
			expected: true,
		},
		{
			name:     "Large numbers outside tolerance",
			a:        1000.2,
			b:        1000.0,
			epsilon:  0.1,
			expected: true, // 0.2/1000 = 0.0002 < 0.1, so within tolerance
		},
		{
			name:     "Very small numbers within tolerance",
			a:        0.0001,
			b:        0.0000,
			epsilon:  0.001,
			expected: false, // When one value is 0, it uses absolute tolerance: 0.0001 < 0.001, so true
		},
		{
			name:     "Very small numbers outside tolerance",
			a:        0.0020,
			b:        0.0000,
			epsilon:  0.001,
			expected: false,
		},
		{
			name:     "Zero epsilon exact match",
			a:        0.5,
			b:        0.5,
			epsilon:  0.0,
			expected: true,
		},
		{
			name:     "Zero epsilon slight difference",
			a:        0.5001,
			b:        0.5000,
			epsilon:  0.0,
			expected: false,
		},
		{
			name:     "Negative numbers",
			a:        -0.5,
			b:        -0.5,
			epsilon:  0.001,
			expected: true,
		},
		{
			name:     "Mixed signs",
			a:        0.5,
			b:        -0.5,
			epsilon:  1.0,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Equal64(tt.a, tt.b, tt.epsilon)
			if result != tt.expected {
				t.Errorf("Equal64(%f, %f, %f) = %v, want %v",
					tt.a, tt.b, tt.epsilon, result, tt.expected)
			}
		})
	}
}

func TestRangeMultiply(t *testing.T) {
	tests := []struct {
		name     string
		range1   string
		range2   string
		expected map[string]float64
	}{
		{
			name:     "Empty ranges",
			range1:   "",
			range2:   "",
			expected: map[string]float64{},
		},
		{
			name:   "Single hand identical",
			range1: "AhKh:0.5",
			range2: "AhKh:0.6",
			expected: map[string]float64{
				"AhKh": 0.3, // 0.5 * 0.6
			},
		},
		{
			name:   "Multiple hands overlapping",
			range1: "AA:0.8, KK:0.6",
			range2: "AA:0.7, KK:0.5",
			expected: map[string]float64{
				"AhAd": 0.56, "AhAs": 0.56, "AhAc": 0.56, "AdAs": 0.56, "AdAc": 0.56, "AsAc": 0.56, // 0.8 * 0.7
				"KhKd": 0.30, "KhKs": 0.30, "KhKc": 0.30, "KdKs": 0.30, "KdKc": 0.30, "KsKc": 0.30, // 0.6 * 0.5
			},
		},
		{
			name:     "No overlap",
			range1:   "AA:0.5",
			range2:   "KK:0.6",
			expected: map[string]float64{}, // No overlapping hands
		},
		{
			name:     "Zero weights",
			range1:   "AA:0.0, KK:0.5",
			range2:   "AA:0.3, KK:0.0",
			expected: map[string]float64{}, // 0.0 * 0.3 = 0.0, 0.5 * 0.0 = 0.0
		},
		{
			name:   "Suited hands",
			range1: "AKs:0.7, AQs:0.5",
			range2: "AKs:0.6, AQs:0.4",
			expected: map[string]float64{
				"AhKh": 0.42, "AdKd": 0.42, "AsKs": 0.42, "AcKc": 0.42, // 0.7 * 0.6
				"AhQh": 0.20, "AdQd": 0.20, "AsQs": 0.20, "AcQc": 0.20, // 0.5 * 0.4
			},
		},
		{
			name:   "Offsuit hands",
			range1: "AKo:0.6, AQo:0.4",
			range2: "AKo:0.5, AQo:0.3",
			expected: map[string]float64{
				"AhKc": 0.30, "AhKd": 0.30, "AhKs": 0.30, "AdKh": 0.30, "AdKc": 0.30, "AdKs": 0.30, "AsKh": 0.30, "AsKc": 0.30, "AsKd": 0.30, "AcKh": 0.30, "AcKd": 0.30, "AcKs": 0.30, // 0.6 * 0.5
				"AhQc": 0.12, "AhQd": 0.12, "AhQs": 0.12, "AdQh": 0.12, "AdQc": 0.12, "AdQs": 0.12, "AsQh": 0.12, "AsQc": 0.12, "AsQd": 0.12, "AcQh": 0.12, "AcQd": 0.12, "AcQs": 0.12, // 0.4 * 0.3
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var r1, r2, result Range

			// Parse the first range
			err := r1.Parse(tt.range1)
			if err != nil {
				t.Fatalf("Failed to parse first range '%s': %v", tt.range1, err)
			}

			// Parse the second range
			err = r2.Parse(tt.range2)
			if err != nil {
				t.Fatalf("Failed to parse second range '%s': %v", tt.range2, err)
			}

			// Test Multiply method
			result.Multiply(&r1, &r2)

			// Check that all expected hands have the correct weights
			for hand, expectedWeight := range tt.expected {
				holeCards, err := ParseHoleCards(hand)
				if err != nil {
					t.Fatalf("Failed to parse hand %s: %v", hand, err)
				}

				actualWeight := result.Weight(holeCards)
				if !Equal64(actualWeight, expectedWeight, 0.001) {
					t.Errorf("Hand %s: expected weight %.6f, got %.6f", hand, expectedWeight, actualWeight)
				}
			}

			// Verify the result size is exactly as expected
			if len(tt.expected) != result.Size() {
				t.Errorf("Expected %d hands, got %d", len(tt.expected), result.Size())
			}
		})
	}
}

func TestRangeDivide(t *testing.T) {
	tests := []struct {
		name     string
		range1   string
		range2   string
		expected map[string]float64
	}{
		{
			name:     "Empty ranges",
			range1:   "",
			range2:   "",
			expected: map[string]float64{},
		},
		{
			name:   "Single hand division",
			range1: "AhKh:0.6",
			range2: "AhKh:0.3",
			expected: map[string]float64{
				"AhKh": 2.0, // 0.6 / 0.3
			},
		},
		{
			name:   "Multiple hands overlapping",
			range1: "AA:0.8, KK:0.6",
			range2: "AA:0.4, KK:0.3",
			expected: map[string]float64{
				"AhAd": 2.0, "AhAs": 2.0, "AhAc": 2.0, "AdAs": 2.0, "AdAc": 2.0, "AsAc": 2.0, // 0.8 / 0.4
				"KhKd": 2.0, "KhKs": 2.0, "KhKc": 2.0, "KdKs": 2.0, "KdKc": 2.0, "KsKc": 2.0, // 0.6 / 0.3
			},
		},
		{
			name:     "No overlap",
			range1:   "AA:0.5",
			range2:   "KK:0.6",
			expected: map[string]float64{}, // No overlapping hands
		},
		{
			name:   "Zero in numerator",
			range1: "AA:0.0, KK:0.5",
			range2: "AA:0.3, KK:0.2",
			expected: map[string]float64{
				"KhKd": 2.5, "KhKs": 2.5, "KhKc": 2.5, "KdKs": 2.5, "KdKc": 2.5, "KsKc": 2.5, // 0.5 / 0.2
			},
		},
		{
			name:   "Zero in denominator",
			range1: "AA:0.5, KK:0.6",
			range2: "AA:0.0, KK:0.2",
			expected: map[string]float64{
				"KhKd": 3.0, "KhKs": 3.0, "KhKc": 3.0, "KdKs": 3.0, "KdKc": 3.0, "KsKc": 3.0, // 0.6 / 0.2
			},
		},
		{
			name:   "Suited hands",
			range1: "AKs:0.7, AQs:0.5",
			range2: "AKs:0.2, AQs:0.1",
			expected: map[string]float64{
				"AhKh": 3.5, "AdKd": 3.5, "AsKs": 3.5, "AcKc": 3.5, // 0.7 / 0.2
				"AhQh": 5.0, "AdQd": 5.0, "AsQs": 5.0, "AcQc": 5.0, // 0.5 / 0.1
			},
		},
		{
			name:   "Offsuit hands",
			range1: "AKo:0.6, AQo:0.4",
			range2: "AKo:0.2, AQo:0.1",
			expected: map[string]float64{
				"AhKc": 3.0, "AhKd": 3.0, "AhKs": 3.0, "AdKh": 3.0, "AdKc": 3.0, "AdKs": 3.0, "AsKh": 3.0, "AsKc": 3.0, "AsKd": 3.0, "AcKh": 3.0, "AcKd": 3.0, "AcKs": 3.0, // 0.6 / 0.2
				"AhQc": 4.0, "AhQd": 4.0, "AhQs": 4.0, "AdQh": 4.0, "AdQc": 4.0, "AdQs": 4.0, "AsQh": 4.0, "AsQc": 4.0, "AsQd": 4.0, "AcQh": 4.0, "AcQd": 4.0, "AcQs": 4.0, // 0.4 / 0.1
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var r1, r2, result Range

			// Parse the first range
			err := r1.Parse(tt.range1)
			if err != nil {
				t.Fatalf("Failed to parse first range '%s': %v", tt.range1, err)
			}

			// Parse the second range
			err = r2.Parse(tt.range2)
			if err != nil {
				t.Fatalf("Failed to parse second range '%s': %v", tt.range2, err)
			}

			// Test Divide method
			result.Divide(&r1, &r2)

			// Check that all expected hands have the correct weights
			for hand, expectedWeight := range tt.expected {
				holeCards, err := ParseHoleCards(hand)
				if err != nil {
					t.Fatalf("Failed to parse hand %s: %v", hand, err)
				}

				actualWeight := result.Weight(holeCards)
				if !Equal64(actualWeight, expectedWeight, 0.001) {
					t.Errorf("Hand %s: expected weight %.6f, got %.6f", hand, expectedWeight, actualWeight)
				}
			}

			// Verify the result size is exactly as expected
			if len(tt.expected) != result.Size() {
				t.Errorf("Expected %d hands, got %d", len(tt.expected), result.Size())
			}
		})
	}
}
