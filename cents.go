package hh

import "fmt"

// Cents is an amount of money in cents. An int32 holds up to about $21 million,
// far more than any stack or pot in these files.
type Cents int32

// String formats the amount in dollars, for example "$1.25" or "-$0.50".
func (c Cents) String() string {
	sign := ""
	v := int64(c)
	if v < 0 {
		sign, v = "-", -v
	}
	return fmt.Sprintf("%s$%d.%02d", sign, v/100, v%100)
}
