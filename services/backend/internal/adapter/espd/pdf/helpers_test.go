package pdf_test

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Small shims so the assertions above read as prose rather than as fmt calls.

func errString(s string) error { return errors.New(s) }

// fmtSscan parses a base-10 integer. It is strconv and not fmt.Sscan on
// purpose: an xref offset is zero-padded ("0000000009"), and fmt reads a
// leading zero as an octal prefix, which silently turns every offset into
// nonsense.
func fmtSscan(s string, v *int) (int, error) {
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return 0, errString("no number to read")
	}
	n, err := strconv.Atoi(fields[0])
	if err != nil {
		return 0, err
	}
	*v = n
	return 1, nil
}

func itoa(n int) string { return strconv.Itoa(n) }

func octal(b byte) string { return fmt.Sprintf("%03o", b) }

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
