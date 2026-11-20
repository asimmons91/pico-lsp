package cart

import (
	"errors"
	"fmt"
)

var (
	// ErrUnsupportedFormat reports a path that is not a text .p8 cart.
	ErrUnsupportedFormat = errors.New("cart: unsupported format")
	// ErrInvalidEncoding reports binary or invalid UTF-8 input.
	ErrInvalidEncoding = errors.New("cart: binary or invalid UTF-8 input")
	// ErrTooLarge reports input larger than the supported cart size.
	ErrTooLarge = errors.New("cart: input exceeds supported size")
	// ErrDuplicateLua reports a cart with more than one __lua__ section.
	ErrDuplicateLua = errors.New("cart: duplicate __lua__ section")
	// ErrNoCodeSection reports ReplaceCode on a cart without __lua__.
	ErrNoCodeSection = errors.New("cart: no __lua__ section")
	// ErrUnterminatedHeader reports a non-empty ReplaceCode into a __lua__
	// header that has no line terminator.
	ErrUnterminatedHeader = errors.New("cart: unterminated __lua__ header")
	// ErrMalformedSection reports a malformed section header.
	ErrMalformedSection = errors.New("cart: malformed section header")
	// ErrInvalidPath reports an empty, relative, or unsupported Owner path.
	ErrInvalidPath = errors.New("cart: invalid path")
)

// ParseError adds a zero-based line number to a parse error.
type ParseError struct {
	Line int
	Err  error
}

// Error implements error.
func (e *ParseError) Error() string {
	return fmt.Sprintf("%v at line %d", e.Err, e.Line)
}

// Unwrap returns the underlying sentinel error.
func (e *ParseError) Unwrap() error {
	return e.Err
}
