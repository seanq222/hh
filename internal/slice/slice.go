package slice

import (
	"fmt"

	"github.com/seanq222/hh/internal/float"
	"golang.org/x/exp/constraints"
)

func Equal[T constraints.Ordered](a, b []T) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func Max[T constraints.Float](dst, s0, s1 []T) {
	if len(s0) != len(s1) || len(dst) != len(s0) {
		panic("slice lengths do not match")
	}
	for i := range s0 {
		if s0[i] > s1[i] {
			dst[i] = s0[i]
		} else {
			dst[i] = s1[i]
		}
	}
}

func Sum[T constraints.Float](s []T) T {
	var sum T
	for _, v := range s {
		sum += v
	}
	return sum
}

func Set[T constraints.Float](dst []T, value T) {
	for i := range dst {
		dst[i] = value
	}
}

func Scale[T constraints.Float](dst, s []T, scalar T) {
	if len(dst) != len(s) {
		panic("slice lengths do not match")
	}
	for i := range s {
		dst[i] = s[i] * scalar
	}
}

func ScaleInPlace[T constraints.Float](s []T, scalar T) {
	if len(s) == 0 {
		return
	}
	for i := range s {
		s[i] *= scalar
	}
}

func Add[T constraints.Float](dst []T, s0, s1 []T) {
	if len(s0) != len(s1) || len(dst) != len(s0) {
		panic("slice lengths do not match")
	}
	for i := range s0 {
		dst[i] = s0[i] + s1[i]
	}
}

func AddRows[T constraints.Float](dst []T, s []T) {
	n := len(dst)
	if len(s) < n {
		panic("slice length does not match")
	}
	copy(dst, s[:n])
	for s = s[n:]; len(s) >= n; s = s[n:] {
		Add(dst, dst, s[:n])
	}
	if len(s) > 0 {
		panic(fmt.Sprintf("slice length does not match: %d", len(s)))
	}
}

func Sub[T constraints.Float](dst, s0, s1 []T) {
	if len(s0) != len(s1) || len(dst) != len(s0) {
		panic("slice lengths do not match")
	}
	for i := range s0 {
		dst[i] = s0[i] - s1[i]
	}
}

func Mul[T constraints.Float](dst, s0, s1 []T) {
	if len(s0) != len(s1) || len(dst) != len(s0) {
		panic("slice lengths do not match")
	}
	for i := range s0 {
		dst[i] = s0[i] * s1[i]
	}
}

func Div[T constraints.Float](dst, s0, s1 []T, default_ T) {
	if len(s0) != len(s1) || len(dst) != len(s0) {
		panic("slice lengths do not match")
	}
	for i := range s0 {
		if s1[i] == 0 {
			dst[i] = default_
		} else {
			dst[i] = s0[i] / s1[i]
		}
	}
}

func Reciprocal[T constraints.Float](dst, s []T, default_ T) {
	if len(s) != len(dst) {
		panic("slice lengths do not match")
	}
	for i := range s {
		if s[i] == 0 {
			dst[i] = default_
		} else {
			dst[i] = 1 / s[i]
		}
	}
}

func DotProduct[T constraints.Float](s0, s1 []T) T {
	if len(s0) != len(s1) {
		panic("slice lengths do not match")
	}
	var sum T
	for i := range s0 {
		sum += s0[i] * s1[i]
	}
	return sum
}

func MulAcc[T constraints.Float](dst, s0, s1 []T) {
	if len(s0) != len(s1) || len(dst) != len(s0) {
		panic("slice lengths do not match")
	}
	for i := range s0 {
		dst[i] += s0[i] * s1[i]
	}
}

func EqualApprox[T constraints.Float](a, b []T, epsilon T) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !float.EqualApprox(a[i], b[i], epsilon) {
			return false
		}
	}
	return true
}
