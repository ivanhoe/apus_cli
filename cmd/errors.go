package cmd

import (
	"errors"
	"strings"
)

const (
	exitCodeGeneric     = 1
	exitCodeUsage       = 2
	exitCodePreflight   = 3
	exitCodeProject     = 4
	exitCodeTarget      = 5
	exitCodePackage     = 6
	exitCodeRollback    = 7
	exitCodeMutation    = 8
	exitCodeUnsupported = 9
)

type cliError struct {
	err     error
	code    int
	printed bool
}

func (e *cliError) Error() string {
	return e.err.Error()
}

func (e *cliError) Unwrap() error {
	return e.err
}

func markPrinted(err error) error {
	if err == nil {
		return nil
	}

	var cliErr *cliError
	if errors.As(err, &cliErr) {
		return &cliError{
			err:     cliErr.err,
			code:    cliErr.code,
			printed: true,
		}
	}

	return &cliError{
		err:     err,
		code:    exitCodeGeneric,
		printed: true,
	}
}

func shouldPrintError(err error) bool {
	var cliErr *cliError
	return !errors.As(err, &cliErr) || !cliErr.printed
}

func exitCode(err error) int {
	var cliErr *cliError
	if errors.As(err, &cliErr) && cliErr.code != 0 {
		return cliErr.code
	}
	return exitCodeGeneric
}

func withExitCode(err error, code int) error {
	if err == nil {
		return nil
	}

	var cliErr *cliError
	if errors.As(err, &cliErr) {
		next := &cliError{
			err:     cliErr.err,
			code:    cliErr.code,
			printed: cliErr.printed,
		}
		if next.code == 0 || next.code == exitCodeGeneric {
			next.code = code
		}
		return next
	}

	return &cliError{
		err:  err,
		code: code,
	}
}

func usageError(err error) error {
	return withExitCode(err, exitCodeUsage)
}

func preflightError(err error) error {
	return withExitCode(err, exitCodePreflight)
}

func projectError(err error) error {
	return withExitCode(err, exitCodeProject)
}

func targetError(err error) error {
	return withExitCode(err, exitCodeTarget)
}

func packageError(err error) error {
	return withExitCode(err, exitCodePackage)
}

func rollbackError(err error) error {
	return withExitCode(err, exitCodeRollback)
}

func mutationError(err error) error {
	return withExitCode(err, exitCodeMutation)
}

func unsupportedError(err error) error {
	return withExitCode(err, exitCodeUnsupported)
}

func classifyProjectError(err error) error {
	if err == nil {
		return nil
	}

	msg := err.Error()
	switch {
	case strings.Contains(msg, "multiple app targets found"),
		strings.Contains(msg, "not found — app targets"):
		return targetError(err)
	case strings.Contains(msg, "no .xcodeproj found"),
		strings.Contains(msg, "no app target found"),
		strings.Contains(msg, "xcodebuild -list"):
		return projectError(err)
	default:
		return projectError(err)
	}
}
