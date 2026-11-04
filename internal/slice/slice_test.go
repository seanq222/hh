package slice

import (
	"math"
	"testing"
)

func TestEqual(t *testing.T) {
	tests := []struct {
		name     string
		a        []int
		b        []int
		expected bool
	}{
		{
			name:     "identical slices",
			a:        []int{1, 2, 3, 4},
			b:        []int{1, 2, 3, 4},
			expected: true,
		},
		{
			name:     "different lengths",
			a:        []int{1, 2, 3},
			b:        []int{1, 2, 3, 4},
			expected: false,
		},
		{
			name:     "same length, different values",
			a:        []int{1, 2, 3, 4},
			b:        []int{1, 2, 3, 5},
			expected: false,
		},
		{
			name:     "empty slices",
			a:        []int{},
			b:        []int{},
			expected: true,
		},
		{
			name:     "one empty, one not",
			a:        []int{},
			b:        []int{1},
			expected: false,
		},
		{
			name:     "single element, equal",
			a:        []int{42},
			b:        []int{42},
			expected: true,
		},
		{
			name:     "single element, different",
			a:        []int{42},
			b:        []int{43},
			expected: false,
		},
		{
			name:     "nil slices",
			a:        nil,
			b:        nil,
			expected: true,
		},
		{
			name:     "one nil, one empty",
			a:        nil,
			b:        []int{},
			expected: true,
		},
		{
			name:     "one nil, one not empty",
			a:        nil,
			b:        []int{1},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Equal(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("Equal(%v, %v) = %v, expected %v", tt.a, tt.b, result, tt.expected)
			}
		})
	}
}

func TestEqualString(t *testing.T) {
	tests := []struct {
		name     string
		a        []string
		b        []string
		expected bool
	}{
		{
			name:     "identical string slices",
			a:        []string{"hello", "world"},
			b:        []string{"hello", "world"},
			expected: true,
		},
		{
			name:     "different string slices",
			a:        []string{"hello", "world"},
			b:        []string{"hello", "go"},
			expected: false,
		},
		{
			name:     "empty string slices",
			a:        []string{},
			b:        []string{},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Equal(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("Equal(%v, %v) = %v, expected %v", tt.a, tt.b, result, tt.expected)
			}
		})
	}
}

func TestSum(t *testing.T) {
	tests := []struct {
		name     string
		input    []float64
		expected float64
	}{
		{
			name:     "positive numbers",
			input:    []float64{1.0, 2.0, 3.0, 4.0},
			expected: 10.0,
		},
		{
			name:     "mixed positive and negative",
			input:    []float64{1.0, -2.0, 3.0, -4.0},
			expected: -2.0,
		},
		{
			name:     "empty slice",
			input:    []float64{},
			expected: 0.0,
		},
		{
			name:     "single element",
			input:    []float64{42.5},
			expected: 42.5,
		},
		{
			name:     "all zeros",
			input:    []float64{0.0, 0.0, 0.0},
			expected: 0.0,
		},
		{
			name:     "large numbers",
			input:    []float64{1e10, 2e10, 3e10},
			expected: 6e10,
		},
		{
			name:     "small numbers",
			input:    []float64{1e-10, 2e-10, 3e-10},
			expected: 6e-10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Sum(tt.input)
			if result != tt.expected {
				t.Errorf("Sum(%v) = %v, expected %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSumFloat32(t *testing.T) {
	tests := []struct {
		name     string
		input    []float32
		expected float32
	}{
		{
			name:     "float32 positive numbers",
			input:    []float32{1.0, 2.0, 3.0},
			expected: 6.0,
		},
		{
			name:     "float32 mixed signs",
			input:    []float32{1.0, -2.0, 3.0},
			expected: 2.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Sum(tt.input)
			if result != tt.expected {
				t.Errorf("Sum(%v) = %v, expected %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestAdd(t *testing.T) {
	tests := []struct {
		name        string
		dst         []float64
		s0          []float64
		s1          []float64
		expected    []float64
		shouldPanic bool
	}{
		{
			name:        "normal addition",
			dst:         make([]float64, 3),
			s0:          []float64{1.0, 2.0, 3.0},
			s1:          []float64{4.0, 5.0, 6.0},
			expected:    []float64{5.0, 7.0, 9.0},
			shouldPanic: false,
		},
		{
			name:        "negative numbers",
			dst:         make([]float64, 3),
			s0:          []float64{1.0, -2.0, 3.0},
			s1:          []float64{-1.0, 2.0, -3.0},
			expected:    []float64{0.0, 0.0, 0.0},
			shouldPanic: false,
		},
		{
			name:        "zero addition",
			dst:         make([]float64, 3),
			s0:          []float64{1.0, 2.0, 3.0},
			s1:          []float64{0.0, 0.0, 0.0},
			expected:    []float64{1.0, 2.0, 3.0},
			shouldPanic: false,
		},
		{
			name:        "empty slices",
			dst:         []float64{},
			s0:          []float64{},
			s1:          []float64{},
			expected:    []float64{},
			shouldPanic: false,
		},
		{
			name:        "mismatched lengths - dst too short",
			dst:         make([]float64, 2),
			s0:          []float64{1.0, 2.0, 3.0},
			s1:          []float64{4.0, 5.0, 6.0},
			expected:    nil,
			shouldPanic: true,
		},
		{
			name:        "mismatched lengths - s0 and s1 different",
			dst:         make([]float64, 3),
			s0:          []float64{1.0, 2.0},
			s1:          []float64{4.0, 5.0, 6.0},
			expected:    nil,
			shouldPanic: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					if !tt.shouldPanic {
						t.Errorf("Add panicked unexpectedly: %v", r)
					}
				} else if tt.shouldPanic {
					t.Error("Add should have panicked but didn't")
				}
			}()

			Add(tt.dst, tt.s0, tt.s1)

			if !tt.shouldPanic && !Equal(tt.dst, tt.expected) {
				t.Errorf("Add result = %v, expected %v", tt.dst, tt.expected)
			}
		})
	}
}

func TestSub(t *testing.T) {
	tests := []struct {
		name        string
		dst         []float64
		s0          []float64
		s1          []float64
		expected    []float64
		shouldPanic bool
	}{
		{
			name:        "normal subtraction",
			dst:         make([]float64, 3),
			s0:          []float64{5.0, 7.0, 9.0},
			s1:          []float64{1.0, 2.0, 3.0},
			expected:    []float64{4.0, 5.0, 6.0},
			shouldPanic: false,
		},
		{
			name:        "negative result",
			dst:         make([]float64, 3),
			s0:          []float64{1.0, 2.0, 3.0},
			s1:          []float64{4.0, 5.0, 6.0},
			expected:    []float64{-3.0, -3.0, -3.0},
			shouldPanic: false,
		},
		{
			name:        "zero subtraction",
			dst:         make([]float64, 3),
			s0:          []float64{1.0, 2.0, 3.0},
			s1:          []float64{0.0, 0.0, 0.0},
			expected:    []float64{1.0, 2.0, 3.0},
			shouldPanic: false,
		},
		{
			name:        "mismatched lengths",
			dst:         make([]float64, 2),
			s0:          []float64{1.0, 2.0, 3.0},
			s1:          []float64{4.0, 5.0, 6.0},
			expected:    nil,
			shouldPanic: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					if !tt.shouldPanic {
						t.Errorf("Sub panicked unexpectedly: %v", r)
					}
				} else if tt.shouldPanic {
					t.Error("Sub should have panicked but didn't")
				}
			}()

			Sub(tt.dst, tt.s0, tt.s1)

			if !tt.shouldPanic && !Equal(tt.dst, tt.expected) {
				t.Errorf("Sub result = %v, expected %v", tt.dst, tt.expected)
			}
		})
	}
}

func TestMul(t *testing.T) {
	tests := []struct {
		name        string
		dst         []float64
		s0          []float64
		s1          []float64
		expected    []float64
		shouldPanic bool
	}{
		{
			name:        "normal multiplication",
			dst:         make([]float64, 3),
			s0:          []float64{2.0, 3.0, 4.0},
			s1:          []float64{5.0, 6.0, 7.0},
			expected:    []float64{10.0, 18.0, 28.0},
			shouldPanic: false,
		},
		{
			name:        "zero multiplication",
			dst:         make([]float64, 3),
			s0:          []float64{1.0, 2.0, 3.0},
			s1:          []float64{0.0, 0.0, 0.0},
			expected:    []float64{0.0, 0.0, 0.0},
			shouldPanic: false,
		},
		{
			name:        "negative multiplication",
			dst:         make([]float64, 3),
			s0:          []float64{2.0, -3.0, 4.0},
			s1:          []float64{-1.0, 2.0, -3.0},
			expected:    []float64{-2.0, -6.0, -12.0},
			shouldPanic: false,
		},
		{
			name:        "mismatched lengths",
			dst:         make([]float64, 2),
			s0:          []float64{1.0, 2.0, 3.0},
			s1:          []float64{4.0, 5.0, 6.0},
			expected:    nil,
			shouldPanic: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					if !tt.shouldPanic {
						t.Errorf("Mul panicked unexpectedly: %v", r)
					}
				} else if tt.shouldPanic {
					t.Error("Mul should have panicked but didn't")
				}
			}()

			Mul(tt.dst, tt.s0, tt.s1)

			if !tt.shouldPanic && !Equal(tt.dst, tt.expected) {
				t.Errorf("Mul result = %v, expected %v", tt.dst, tt.expected)
			}
		})
	}
}

func TestDiv(t *testing.T) {
	tests := []struct {
		name        string
		dst         []float64
		s0          []float64
		s1          []float64
		expected    []float64
		shouldPanic bool
	}{
		{
			name:        "normal division",
			dst:         make([]float64, 3),
			s0:          []float64{10.0, 18.0, 28.0},
			s1:          []float64{2.0, 3.0, 4.0},
			expected:    []float64{5.0, 6.0, 7.0},
			shouldPanic: false,
		},
		{
			name:        "division by one",
			dst:         make([]float64, 3),
			s0:          []float64{1.0, 2.0, 3.0},
			s1:          []float64{1.0, 1.0, 1.0},
			expected:    []float64{1.0, 2.0, 3.0},
			shouldPanic: false,
		},
		{
			name:        "negative division",
			dst:         make([]float64, 3),
			s0:          []float64{-6.0, 8.0, -12.0},
			s1:          []float64{2.0, -4.0, 3.0},
			expected:    []float64{-3.0, -2.0, -4.0},
			shouldPanic: false,
		},
		{
			name:        "mismatched lengths",
			dst:         make([]float64, 2),
			s0:          []float64{1.0, 2.0, 3.0},
			s1:          []float64{4.0, 5.0, 6.0},
			expected:    nil,
			shouldPanic: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					if !tt.shouldPanic {
						t.Errorf("Div panicked unexpectedly: %v", r)
					}
				} else if tt.shouldPanic {
					t.Error("Div should have panicked but didn't")
				}
			}()

			Div(tt.dst, tt.s0, tt.s1, 0.)

			if !tt.shouldPanic && !Equal(tt.dst, tt.expected) {
				t.Errorf("Div result = %v, expected %v", tt.dst, tt.expected)
			}
		})
	}
}

func TestDotProduct(t *testing.T) {
	tests := []struct {
		name        string
		s0          []float64
		s1          []float64
		expected    float64
		shouldPanic bool
	}{
		{
			name:        "normal dot product",
			s0:          []float64{1.0, 2.0, 3.0},
			s1:          []float64{4.0, 5.0, 6.0},
			expected:    32.0, // 1*4 + 2*5 + 3*6 = 4 + 10 + 18 = 32
			shouldPanic: false,
		},
		{
			name:        "zero dot product",
			s0:          []float64{1.0, 2.0, 3.0},
			s1:          []float64{0.0, 0.0, 0.0},
			expected:    0.0,
			shouldPanic: false,
		},
		{
			name:        "negative dot product",
			s0:          []float64{1.0, -2.0, 3.0},
			s1:          []float64{-1.0, 2.0, -3.0},
			expected:    -14.0, // 1*(-1) + (-2)*2 + 3*(-3) = -1 - 4 - 9 = -14
			shouldPanic: false,
		},
		{
			name:        "empty slices",
			s0:          []float64{},
			s1:          []float64{},
			expected:    0.0,
			shouldPanic: false,
		},
		{
			name:        "single element",
			s0:          []float64{5.0},
			s1:          []float64{3.0},
			expected:    15.0,
			shouldPanic: false,
		},
		{
			name:        "mismatched lengths",
			s0:          []float64{1.0, 2.0},
			s1:          []float64{4.0, 5.0, 6.0},
			expected:    0.0,
			shouldPanic: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					if !tt.shouldPanic {
						t.Errorf("DotProduct panicked unexpectedly: %v", r)
					}
				} else if tt.shouldPanic {
					t.Error("DotProduct should have panicked but didn't")
				}
			}()

			result := DotProduct(tt.s0, tt.s1)

			if !tt.shouldPanic && result != tt.expected {
				t.Errorf("DotProduct(%v, %v) = %v, expected %v", tt.s0, tt.s1, result, tt.expected)
			}
		})
	}
}

func TestDotProductFloat32(t *testing.T) {
	tests := []struct {
		name        string
		s0          []float32
		s1          []float32
		expected    float32
		shouldPanic bool
	}{
		{
			name:        "float32 dot product",
			s0:          []float32{1.0, 2.0, 3.0},
			s1:          []float32{4.0, 5.0, 6.0},
			expected:    32.0,
			shouldPanic: false,
		},
		{
			name:        "float32 single element",
			s0:          []float32{2.5},
			s1:          []float32{4.0},
			expected:    10.0,
			shouldPanic: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					if !tt.shouldPanic {
						t.Errorf("DotProduct panicked unexpectedly: %v", r)
					}
				} else if tt.shouldPanic {
					t.Error("DotProduct should have panicked but didn't")
				}
			}()

			result := DotProduct(tt.s0, tt.s1)

			if !tt.shouldPanic && result != tt.expected {
				t.Errorf("DotProduct(%v, %v) = %v, expected %v", tt.s0, tt.s1, result, tt.expected)
			}
		})
	}
}

func TestEqualApprox(t *testing.T) {
	tests := []struct {
		name     string
		a        []float64
		b        []float64
		epsilon  float64
		expected bool
	}{
		{
			name:     "exact equality",
			a:        []float64{1.0, 2.0, 3.0},
			b:        []float64{1.0, 2.0, 3.0},
			epsilon:  1e-10,
			expected: true,
		},
		{
			name:     "within epsilon",
			a:        []float64{1.0, 2.0, 3.0},
			b:        []float64{1.0000001, 2.0000001, 3.0000001},
			epsilon:  1e-6,
			expected: true,
		},
		{
			name:     "outside epsilon",
			a:        []float64{1.0, 2.0, 3.0},
			b:        []float64{1.001, 2.001, 3.001},
			epsilon:  1e-6,
			expected: false,
		},
		{
			name:     "different lengths",
			a:        []float64{1.0, 2.0},
			b:        []float64{1.0, 2.0, 3.0},
			epsilon:  1e-10,
			expected: false,
		},
		{
			name:     "empty slices",
			a:        []float64{},
			b:        []float64{},
			epsilon:  1e-10,
			expected: true,
		},
		{
			name:     "very small numbers within epsilon",
			a:        []float64{1e-10, 2e-10},
			b:        []float64{1.1e-10, 2.1e-10},
			epsilon:  0.2,
			expected: true,
		},
		{
			name:     "very small numbers outside epsilon",
			a:        []float64{1e-10, 2e-10},
			b:        []float64{2e-10, 3e-10},
			epsilon:  0.1,
			expected: false,
		},
		{
			name:     "mixed signs within epsilon",
			a:        []float64{1.0, -2.0, 3.0},
			b:        []float64{1.0001, -2.0001, 3.0001},
			epsilon:  1e-3,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EqualApprox(tt.a, tt.b, tt.epsilon)
			if result != tt.expected {
				t.Errorf("EqualApprox(%v, %v, %v) = %v, expected %v", tt.a, tt.b, tt.epsilon, result, tt.expected)
			}
		})
	}
}

func TestEqualApproxFloat32(t *testing.T) {
	tests := []struct {
		name     string
		a        []float32
		b        []float32
		epsilon  float32
		expected bool
	}{
		{
			name:     "float32 exact equality",
			a:        []float32{1.0, 2.0, 3.0},
			b:        []float32{1.0, 2.0, 3.0},
			epsilon:  1e-6,
			expected: true,
		},
		{
			name:     "float32 within epsilon",
			a:        []float32{1.0, 2.0, 3.0},
			b:        []float32{1.0001, 2.0001, 3.0001},
			epsilon:  1e-3,
			expected: true,
		},
		{
			name:     "float32 outside epsilon",
			a:        []float32{1.0, 2.0, 3.0},
			b:        []float32{1.01, 2.01, 3.01},
			epsilon:  1e-3,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EqualApprox(tt.a, tt.b, tt.epsilon)
			if result != tt.expected {
				t.Errorf("EqualApprox(%v, %v, %v) = %v, expected %v", tt.a, tt.b, tt.epsilon, result, tt.expected)
			}
		})
	}
}

func TestEqualApproxEdgeCases(t *testing.T) {
	t.Run("infinity comparison", func(t *testing.T) {
		inf := math.Inf(1)
		negInf := math.Inf(-1)

		// Same infinity should be equal
		if !EqualApprox([]float64{inf}, []float64{inf}, 1e-10) {
			t.Error("EqualApprox with same infinity should be true")
		}

		// Different infinities should not be equal
		if EqualApprox([]float64{inf}, []float64{negInf}, 1e-10) {
			t.Error("EqualApprox with different infinities should be false")
		}
	})

	t.Run("NaN comparison", func(t *testing.T) {
		nan := math.NaN()

		// NaN should not be equal to anything, including itself
		if EqualApprox([]float64{nan}, []float64{nan}, 1e-10) {
			t.Error("EqualApprox with NaN should be false")
		}

		if EqualApprox([]float64{nan}, []float64{1.0}, 1e-10) {
			t.Error("EqualApprox with NaN and number should be false")
		}
	})

	t.Run("very large epsilon", func(t *testing.T) {
		// Test with very large epsilon
		a := []float64{1.0, 2.0}
		b := []float64{3.0, 4.0}
		epsilon := 2.0

		// Should be true because the differences are smaller than epsilon * max(|a|, |b|)
		if !EqualApprox(a, b, epsilon) {
			t.Error("EqualApprox with very large epsilon should be true")
		}
	})
}

// Benchmark tests
func BenchmarkEqual(b *testing.B) {
	a := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	c := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

	for b.Loop() {
		Equal(a, c)
	}
}

func BenchmarkSum(b *testing.B) {
	s := []float64{1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0, 9.0, 10.0}

	for b.Loop() {
		Sum(s)
	}
}

func BenchmarkAdd(b *testing.B) {
	dst := make([]float64, 10)
	s0 := []float64{1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0, 9.0, 10.0}
	s1 := []float64{1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0, 9.0, 10.0}

	for b.Loop() {
		Add(dst, s0, s1)
	}
}

func BenchmarkDotProduct(b *testing.B) {
	s0 := []float64{1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0, 9.0, 10.0}
	s1 := []float64{1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0, 9.0, 10.0}

	for b.Loop() {
		DotProduct(s0, s1)
	}
}

func BenchmarkEqualApprox(b *testing.B) {
	a := []float64{1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0, 9.0, 10.0}
	val := []float64{1.0000001, 2.0000001, 3.0000001, 4.0000001, 5.0000001, 6.0000001, 7.0000001, 8.0000001, 9.0000001, 10.0000001}
	epsilon := 1e-6

	for b.Loop() {
		EqualApprox(a, val, epsilon)
	}
}
