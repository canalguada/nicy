/*
Copyright © 2021 David Guadalupe <guadalupe.david@gmail.com>

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program. If not, see <http://www.gnu.org/licenses/>.
*/
package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"errors"
	"encoding/json"
	// "github.com/canalguada/nicy/jq"
)

// Error codes
const (
	SUCCESS = 0
	FAILURE = 1
	EPARSE = 2
	ENOTCONF = 3
	ENOTDIR = 4
	EACCESS = 5
	EINVAL = 6
	EALREADY = 16
	// EUSAGE = 66
	// ECOMPILE = 67
	EPERM = 126
	ENOTFOUND = 127
)


type customError struct {
	msg string
	Code int
}

func (e customError) Error() string {
	return e.msg
}

var (
	ErrFailure = customError{"fail", FAILURE}
	ErrParse = customError{"cannot parse options", EPARSE}
	ErrNotConfDir = customError{"unknown directory", ENOTCONF}
	ErrNotDir = customError{"not a directory", ENOTDIR}
	ErrNotWritableDir = customError{"not writable directory", EACCESS}
	ErrInvalid = customError{"invalid argument", EINVAL}
	ErrAlready = customError{"already running", EALREADY}
	ErrPermission = customError{"permission denied", EPERM}
	ErrNotFound = customError{"not found", ENOTFOUND}
	// ErrUsage = customError{"gojq usage", EUSAGE}
	// ErrCompile = customError{"gojq compile", ECOMPILE}
)

// fatal prints the error message and exits with proper error code.
// If the error is nil, it does nothing.
func fatal(e error) {
	if e != nil {
		warn(e)
		var err *customError
		if errors.As(e, &err) {
			os.Exit(err.Code)
		} else {
			os.Exit(FAILURE)
		}
	}
}

func nonfatal(e error) bool {
	if e != nil {
		warn(e)
		return false
	}
	return true
}

func wrap(e error) error {
	if e == nil {
		return nil
	}
	// Yet wrapped
	var err *customError
	if errors.As(e, &err) {
		return e
	}
	switch {
	// case errors.Is(e, jq.ErrUsage):
	//   return fmt.Errorf("%w", ErrUsage)
	// case errors.Is(e, jq.ErrCompile):
	//   return fmt.Errorf("%w", ErrCompile)
	case errors.Is(e, os.ErrInvalid):
		return fmt.Errorf("%w: %v", ErrInvalid, e)
	case errors.Is(e, os.ErrPermission):
		return fmt.Errorf("%w: %v", ErrPermission, e)
	case errors.Is(e, exec.ErrNotFound):
		return fmt.Errorf("%w: %v", ErrNotFound, e)
	default:
		switch e := e.(type) {
		case *os.PathError:
			switch {
			case os.IsNotExist(e):
				return fmt.Errorf("%w: %q", ErrNotFound, e.Path)
			case os.IsPermission(e):
				return fmt.Errorf("%w: %q", ErrPermission, e.Path)
			default:
				return fmt.Errorf("%w: %v", ErrFailure, e)
			}
		case *json.InvalidUnmarshalError:
			return fmt.Errorf("%w: %v", ErrInvalid, e)
		case *exec.ExitError:
			return fmt.Errorf("%w: %v", ErrFailure, e)
		case *exec.Error:
			return wrap(e.Unwrap())
		default:
			return fmt.Errorf("%w: %v", ErrFailure, e)
		}
	}
}

// vim: set ft=go fdm=indent ts=2 sw=2 tw=79 noet:
