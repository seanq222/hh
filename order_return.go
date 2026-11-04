package hh

// ReturnViolation says whether the returned bet in a hand agrees with the
// actions.
type ReturnViolation uint8

const (
	// ReturnOK means the returned bet, if any, is what the actions give.
	ReturnOK ReturnViolation = iota
	// ReturnMismatch means the file's returned-bet line differs from the one
	// the actions give, so an action is missing.
	ReturnMismatch
)

func (v ReturnViolation) String() string {
	if v == ReturnMismatch {
		return "the returned bet does not match the actions, an action is missing"
	}
	return "the returned bet matches the actions"
}

// CheckReturn compares the file's "Uncalled bet returned" line with the one the
// actions give. The line is a check, not a source of truth: the actions decide
// what was uncalled, and a line that disagrees means an action is missing, for
// example a raise that the returned bet belongs to. A hand without such a line
// passes: the uncalled bet is worked out from the actions when it is needed.
func (h *HandHistory) CheckReturn() ReturnViolation {
	if h.Returned.Amount == 0 {
		return ReturnOK
	}
	seat, amount, ok := h.UncalledBet()
	if !ok || amount != h.Returned.Amount || seat != h.Returned.Seat {
		return ReturnMismatch
	}
	return ReturnOK
}
