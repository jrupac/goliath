package jsonsafe

import (
	"errors"
	"fmt"
	"strconv"
)

// MaxInt is the largest integer a double holds exactly, along with every
// integer below it.
const MaxInt = 1<<53 - 1

// ErrImprecise is returned for an Int a JavaScript client would read back as a
// different number.
var ErrImprecise = errors.New("integer cannot be read exactly by a JavaScript client")

// Int is an integer that is sent as a JSON number and refuses to encode if a
// JavaScript client would misread it.
//
// It is for values a protocol requires as numbers and that stay far below
// 2^53, such as Unix times in seconds and counts. An ID, or anything else that
// may use all 64 bits, is sent as a string instead. Encoding one beyond
// ±MaxInt fails, so a response holding it is refused rather than delivered
// with the number silently changed.
type Int int64

// MarshalJSON encodes i as a JSON number, or fails if it is beyond ±MaxInt.
func (i Int) MarshalJSON() ([]byte, error) {
	if i > MaxInt || i < -MaxInt {
		return nil, fmt.Errorf("%w: %d", ErrImprecise, int64(i))
	}
	return strconv.AppendInt(nil, int64(i), 10), nil
}
