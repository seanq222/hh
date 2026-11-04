package float

import "golang.org/x/exp/constraints"

func Abs[T constraints.Float](a T) T {

	if a > 0 {
		return a
	}

	// Handle negative zero
	if a == 0 {
		return 0
	}

	return -a
}

func EqualApprox[T constraints.Float](a, b T, epsilon T) bool {
	if a == b {
		return true
	}

	diff := Abs(a - b)
	m := max(Abs(a), Abs(b))

	return diff < m*epsilon
}
