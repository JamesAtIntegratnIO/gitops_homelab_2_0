package errors

import (
	"errors"
	"fmt"
)

// Exit codes for hctl. These give callers (CI, scripts) a machine-readable
// signal about what went wrong.
const (
	// ExitOK indicates success.
	ExitOK = 0
	// ExitError is a general/unclassified error.
	ExitError = 1
	// ExitUserError indicates invalid input, bad flags, or missing arguments.
	ExitUserError = 2
	// ExitPlatformError indicates the platform is unreachable or unhealthy.
	ExitPlatformError = 3
	// ExitTimeout indicates an operation timed out.
	ExitTimeout = 4
)

// HctlError is an error that carries an exit code.
type HctlError struct {
	// Code is the process exit code.
	Code int
	// Err is the underlying error.
	Err error
}

func (e *HctlError) Error() string {
	return e.Err.Error()
}

func (e *HctlError) Unwrap() error {
	return e.Err
}

// NewUserError wraps an error with ExitUserError code.
func NewUserError(format string, args ...any) *HctlError {
	return &HctlError{Code: ExitUserError, Err: fmt.Errorf(format, args...)}
}

// NewPlatformError wraps an error with ExitPlatformError code.
func NewPlatformError(format string, args ...any) *HctlError {
	return &HctlError{Code: ExitPlatformError, Err: fmt.Errorf(format, args...)}
}

// NewTimeoutError wraps an error with ExitTimeout code.
func NewTimeoutError(format string, args ...any) *HctlError {
	return &HctlError{Code: ExitTimeout, Err: fmt.Errorf(format, args...)}
}

// ExitCode extracts the exit code from an error. Defaults to ExitError for
// non-HctlError errors, and ExitOK for nil.
func ExitCode(err error) int {
	if err == nil {
		return ExitOK
	}
	var hErr *HctlError
	if errors.As(err, &hErr) {
		return hErr.Code
	}
	return ExitError
}
