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
	"runtime"
	"golang.org/x/sys/unix"
)


func Getrlimit_Nice() (*unix.Rlimit, error) {
	var rLimit unix.Rlimit
	if err := unix.Getrlimit(unix.RLIMIT_NICE, &rLimit); err != nil {
		return nil, wrapError(err)
	}
	return &rLimit, nil
}

func rlimitNice() *unix.Rlimit {
	rLimit, err := Getrlimit_Nice()
	if err != nil {
		rLimit = &unix.Rlimit{20, 20}
	}
	return rLimit
}

func numCPU() int {
	return runtime.NumCPU()
}

// vim: set ft=go fdm=indent ts=2 sw=2 tw=79 noet:
