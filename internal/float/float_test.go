package float

import (
	"math"
	"testing"
)

func TestAbs(t *testing.T) {
	tests := []struct {
		name     string
		input    float64
		expected float64
	}{
		{
			name:     "positive number",
			input:    5.5,
			expected: 5.5,
		},
		{
			name:     "negative number",
			input:    -3.7,
			expected: 3.7,
		},
		{
			name:     "zero",
			input:    0.0,
			expected: 0.0,
		},
		{
			name:     "negative zero",
			input:    math.Copysign(0, -1),
			expected: 0.0,
		},
		{
			name:     "large positive number",
			input:    1e10,
			expected: 1e10,
		},
		{
			name:     "large negative number",
			input:    -1e10,
			expected: 1e10,
		},
		{
			name:     "small positive number",
			input:    1e-10,
			expected: 1e-10,
		},
		{
			name:     "small negative number",
			input:    -1e-10,
			expected: 1e-10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Abs(tt.input)
			if result != tt.expected {
				t.Errorf("Abs(%v) = %v, expected %v", tt.input, result, tt.expected)
			}
			if math.Signbit(result) {
				t.Errorf("Abs(%v) has negative sign", tt.input)
			}
		})
	}
}

func TestAbsFloat32(t *testing.T) {
	tests := []struct {
		name     string
		input    float32
		expected float32
	}{
		{
			name:     "positive float32",
			input:    3.14,
			expected: 3.14,
		},
		{
			name:     "negative float32",
			input:    -2.71,
			expected: 2.71,
		},
		{
			name:     "zero float32",
			input:    0.0,
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Abs(tt.input)
			if result != tt.expected {
				t.Errorf("Abs(%v) = %v, expected %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestEqualApprox(t *testing.T) {
	tests := []struct {
		name     string
		a        float64
		b        float64
		epsilon  float64
		expected bool
	}{
		{
			name:     "exact equality",
			a:        1.0,
			b:        1.0,
			epsilon:  1e-10,
			expected: true,
		},
		{
			name:     "within epsilon",
			a:        1.0,
			b:        1.0000001,
			epsilon:  1e-6,
			expected: true,
		},
		{
			name:     "outside epsilon",
			a:        1.0,
			b:        1.001,
			epsilon:  1e-6,
			expected: false,
		},
		{
			name:     "zero values",
			a:        0.0,
			b:        0.0,
			epsilon:  1e-10,
			expected: true,
		},
		{
			name:     "very small numbers within epsilon",
			a:        1e-10,
			b:        1.1e-10,
			epsilon:  0.2,
			expected: true,
		},
		{
			name:     "very small numbers outside epsilon",
			a:        1e-10,
			b:        2e-10,
			epsilon:  0.1,
			expected: false,
		},
		{
			name:     "large numbers within epsilon",
			a:        1e10,
			b:        1.0000001e10,
			epsilon:  1e-6,
			expected: true,
		},
		{
			name:     "large numbers outside epsilon",
			a:        1e10,
			b:        1.001e10,
			epsilon:  1e-6,
			expected: false,
		},
		{
			name:     "negative numbers within epsilon",
			a:        -1.0,
			b:        -1.0000001,
			epsilon:  1e-6,
			expected: true,
		},
		{
			name:     "mixed signs within epsilon",
			a:        0.0000001,
			b:        -0.0000001,
			epsilon:  3.0,
			expected: true,
		},
		{
			name:     "mixed signs outside epsilon",
			a:        0.01,
			b:        -0.01,
			epsilon:  1e-5,
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

func TestEqualApproxFloat32(t *testing.T) {
	tests := []struct {
		name     string
		a        float32
		b        float32
		epsilon  float32
		expected bool
	}{
		{
			name:     "exact equality float32",
			a:        1.0,
			b:        1.0,
			epsilon:  1e-6,
			expected: true,
		},
		{
			name:     "within epsilon float32",
			a:        1.0,
			b:        1.0001,
			epsilon:  1e-3,
			expected: true,
		},
		{
			name:     "outside epsilon float32",
			a:        1.0,
			b:        1.01,
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
		// Test with infinity values
		inf := math.Inf(1)
		negInf := math.Inf(-1)

		// Same infinity should be equal
		if !EqualApprox(inf, inf, 1e-10) {
			t.Error("EqualApprox(inf, inf) should be true")
		}

		// Different infinities should not be equal
		if EqualApprox(inf, negInf, 1e-10) {
			t.Error("EqualApprox(inf, negInf) should be false")
		}
	})

	t.Run("NaN comparison", func(t *testing.T) {
		// NaN should not be equal to anything, including itself
		nan := math.NaN()

		if EqualApprox(nan, nan, 1e-10) {
			t.Error("EqualApprox(NaN, NaN) should be false")
		}

		if EqualApprox(nan, 1.0, 1e-10) {
			t.Error("EqualApprox(NaN, 1.0) should be false")
		}
	})

	t.Run("very small epsilon", func(t *testing.T) {
		// Test with very small epsilon
		a := 1.0
		b := 1.0 + 1e-15
		epsilon := 1e-20

		// Should be false because the difference is larger than epsilon * max(|a|, |b|)
		if EqualApprox(a, b, epsilon) {
			t.Error("EqualApprox with very small epsilon should be false")
		}
	})

	t.Run("very large epsilon", func(t *testing.T) {
		// Test with very large epsilon
		a := 1.0
		b := 2.0
		epsilon := 2.0

		// Should be true because the difference is smaller than epsilon * max(|a|, |b|)
		if !EqualApprox(a, b, epsilon) {
			t.Error("EqualApprox with very large epsilon should be true")
		}
	})
}

func TestAbsWithSpecialValues(t *testing.T) {
	t.Run("infinity", func(t *testing.T) {
		inf := math.Inf(1)
		negInf := math.Inf(-1)

		if Abs(inf) != inf {
			t.Error("Abs(inf) should be inf")
		}

		if Abs(negInf) != inf {
			t.Error("Abs(-inf) should be inf")
		}
	})

	t.Run("NaN", func(t *testing.T) {
		nan := math.NaN()
		result := Abs(nan)

		if !math.IsNaN(result) {
			t.Error("Abs(NaN) should be NaN")
		}
	})
}

// Benchmark tests
func BenchmarkAbs(b *testing.B) {
	val := -3.14159
	for b.Loop() {
		Abs(val)
	}
}

func BenchmarkEqualApprox(b *testing.B) {
	a := 1.0
	val := 1.0000001
	epsilon := 1e-6

	for b.Loop() {
		EqualApprox(a, val, epsilon)
	}
}
