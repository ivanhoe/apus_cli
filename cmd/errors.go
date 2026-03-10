package cmd

import "errors"

type printedError struct {
	error
}

func markPrinted(err error) error {
	if err == nil {
		return nil
	}
	return printedError{error: err}
}

func shouldPrintError(err error) bool {
	if err == nil {
		return false
	}

	var alreadyPrinted printedError
	return !errors.As(err, &alreadyPrinted)
}
