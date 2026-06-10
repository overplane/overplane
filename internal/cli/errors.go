package cli

import (
	"errors"
	"fmt"
)

type ExitError struct {
	Code int
	Err  error
}

func (e ExitError) Error() string {
	if e.Err == nil {
		return fmt.Sprintf("exit %d", e.Code)
	}
	return e.Err.Error()
}

func (e ExitError) Unwrap() error { return e.Err }

func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	var ee ExitError
	if errors.As(err, &ee) {
		return ee.Code
	}
	return 1
}

func UsageError(format string, args ...any) error {
	return ExitError{Code: 2, Err: fmt.Errorf(format, args...)}
}

func ValidationError(err error) error {
	return ExitError{Code: 3, Err: err}
}

func IOError(err error) error {
	return ExitError{Code: 4, Err: err}
}

func InternalError(err error) error {
	return ExitError{Code: 5, Err: err}
}

func EnvError(err error) error {
	return ExitError{Code: 6, Err: err}
}
