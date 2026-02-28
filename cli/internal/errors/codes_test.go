package errors

import (
	"fmt"
	"testing"
)

func TestExitCodeNil(t *testing.T) {
	if got := ExitCode(nil); got != ExitOK {
		t.Errorf("ExitCode(nil) = %d, want %d", got, ExitOK)
	}
}

func TestExitCodePlainError(t *testing.T) {
	err := fmt.Errorf("boom")
	if got := ExitCode(err); got != ExitError {
		t.Errorf("ExitCode(plain error) = %d, want %d", got, ExitError)
	}
}

func TestExitCodeUserError(t *testing.T) {
	err := NewUserError("bad flag: %s", "--bogus")
	if got := ExitCode(err); got != ExitUserError {
		t.Errorf("ExitCode(UserError) = %d, want %d", got, ExitUserError)
	}
	if err.Error() != "bad flag: --bogus" {
		t.Errorf("error message = %q", err.Error())
	}
}

func TestExitCodePlatformError(t *testing.T) {
	err := NewPlatformError("cluster down")
	if got := ExitCode(err); got != ExitPlatformError {
		t.Errorf("ExitCode(PlatformError) = %d, want %d", got, ExitPlatformError)
	}
}

func TestExitCodeTimeoutError(t *testing.T) {
	err := NewTimeoutError("waited 5m")
	if got := ExitCode(err); got != ExitTimeout {
		t.Errorf("ExitCode(TimeoutError) = %d, want %d", got, ExitTimeout)
	}
}

func TestHctlErrorUnwrap(t *testing.T) {
	inner := fmt.Errorf("root cause")
	outer := &HctlError{Code: ExitPlatformError, Err: inner}
	if outer.Unwrap() != inner {
		t.Error("Unwrap should return inner error")
	}
}

func TestExitCodeWrappedHctlError(t *testing.T) {
	hErr := NewUserError("bad input")
	wrapped := fmt.Errorf("command failed: %w", hErr)
	if got := ExitCode(wrapped); got != ExitUserError {
		t.Errorf("ExitCode(wrapped HctlError) = %d, want %d", got, ExitUserError)
	}
}
