package ax

import (
	"errors"
	"fmt"
)

// CodeNoValue is kAXErrorNoValue. Missing focused app is this code, not a hang.
const CodeNoValue = -25212

// Error is a raw AXError from the C shim. Code is not negated.
type Error struct {
	Op   string
	Code int
}

func (e Error) Error() string {
	return fmt.Sprintf("%s: AXError %d", e.Op, e.Code)
}

func IsNoValue(err error) bool {
	var e Error
	return errors.As(err, &e) && e.Code == CodeNoValue
}
