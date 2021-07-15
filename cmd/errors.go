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
	"github.com/canalguada/nicy/jq"
)

// Error codes
const (
	SUCCESS = 0
	FAILURE = 1
	EPARSE = 2
	ENOTCONF = 3
	ENOTDIR = 4
	EACCES = 5
	EINVAL = 6
	EALREADY = 16
	EJQUSAGE = 66
	EJQCOMPILE = 67
	EPERM = 126
	ENOTFOUND = 127
)

var ErrFailure = errors.New("fail")
var ErrParse = errors.New("cannot parse options")
var ErrNotConfDir = errors.New("unknown directory")
var ErrNotDir = errors.New("not a directory")
var ErrNotWritableDir = errors.New("not writable directory")
var ErrInvalid = errors.New("invalid argument")
var ErrAlready = errors.New("already running")
var ErrPermission = errors.New("permission denied")
var ErrNotFound = errors.New("not found")

// checkErr prints the error message with the prefix 'Error:' and exits with
// proper error code. If the error is nil, it does nothing.
func checkErr(e error) {
	if e != nil {
		printErrln(e)
		switch {
		case errors.Is(e, ErrFailure):
			os.Exit(FAILURE)
		case errors.Is(e, ErrParse):
			os.Exit(EPARSE)
		case errors.Is(e, ErrNotConfDir):
			os.Exit(ENOTCONF)
		case errors.Is(e, ErrNotDir):
			os.Exit(ENOTDIR)
		case errors.Is(e, ErrNotWritableDir):
			os.Exit(EACCES)
		case errors.Is(e, ErrInvalid):
			os.Exit(EINVAL)
		case errors.Is(e, ErrAlready):
			os.Exit(EALREADY)
		case errors.Is(e, jq.ErrUsage):
			os.Exit(EJQUSAGE)
		case errors.Is(e, jq.ErrCompile):
			os.Exit(EJQCOMPILE)
		case errors.Is(e, ErrPermission):
			os.Exit(EPERM)
		case errors.Is(e, ErrNotFound):
			os.Exit(ENOTFOUND)
		default:
			os.Exit(FAILURE)
		}
	}
}

func wrapError(e error) error {
	if e == nil {
		return nil
	}
	switch {
	case errors.Is(e, os.ErrInvalid):
		return fmt.Errorf("%w: %v", ErrInvalid, e)
	case errors.Is(e, os.ErrPermission):
		return fmt.Errorf("%w: %v", ErrPermission, e)
	case errors.Is(e, exec.ErrNotFound):
		return fmt.Errorf("%w: %v", ErrNotFound, e)
	case errors.Is(e, jq.ErrUsage): // Yet wrapped
		return e
	case errors.Is(e, jq.ErrCompile): // Yet wrapped
		return e
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
			return wrapError(e.Unwrap())
		default:
			return fmt.Errorf("%w: %v", ErrFailure, e)
		}
	}
}


// vim: set ft=go fdm=indent ts=2 sw=2 tw=79 noet:
