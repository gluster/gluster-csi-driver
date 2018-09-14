package client

import (
	"fmt"
)

var errNotFoundStr = "not found"
var errExistsStr = "already exists with a different"
var errInProcessStr = "operation already in process for"
var errInvalidValueStr = "invalid value for"

type clientErr struct {
	errStr string
	vars   []string
}

func (f clientErr) Error() string {
	errFmt := ""
	if f.errStr == errNotFoundStr {
		errFmt = fmt.Sprintf("%s %s %s", f.vars[0], f.vars[1], f.errStr)
	} else if f.errStr == errExistsStr {
		errFmt = fmt.Sprintf("%s %s %s %s, %s", f.vars[0], f.vars[1], f.errStr, f.vars[2], f.vars[3])
	} else if f.errStr == errInProcessStr {
		errFmt = fmt.Sprintf("%s %s %s", f.errStr, f.vars[0], f.vars[1])
	} else if f.errStr == errInvalidValueStr {
		errFmt = fmt.Sprintf("%s '%s': %s", f.errStr, f.vars[0], f.vars[1])
	}

	return errFmt
}

func newClientErr(errStr string, vars ...string) clientErr {
	return clientErr{errStr, vars}
}

func matchClientErr(err error, errStr string) bool {
	cliErr, ok := err.(clientErr)
	return ok && cliErr.errStr == errStr
}

// ErrNotFound creates a new "not found" error
func ErrNotFound(kind, name string) error {
	return newClientErr(errNotFoundStr, kind, name)
}

// ErrExists creates a new "different owner" error
func ErrExists(kind, name, property, propVal string) error {
	return newClientErr(errExistsStr, kind, name, property, propVal)
}

// ErrInProcess creates a new "operation in process" error
func ErrInProcess(kind, name string) error {
	return newClientErr(errInProcessStr, kind, name)
}

// ErrInvalidValue creates a new "invalid value" error
func ErrInvalidValue(kind, errStr string) error {
	return newClientErr(errInvalidValueStr, kind, errStr)
}

// IsErrNotFound checks for an ErrNotFound
func IsErrNotFound(err error) bool {
	return matchClientErr(err, errNotFoundStr)
}

// IsErrExists checks for an ErrExists
func IsErrExists(err error) bool {
	return matchClientErr(err, errExistsStr)
}

// IsInProcess checks for an ErrInProcess
func IsInProcess(err error) bool {
	return matchClientErr(err, errInProcessStr)
}

// IsInvalidValue checks for an ErrInvalidValue
func IsInvalidValue(err error) bool {
	return matchClientErr(err, errInvalidValueStr)
}
