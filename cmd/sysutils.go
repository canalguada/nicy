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
	"strings"
	"runtime"
	"golang.org/x/sys/unix"
	"kernel.org/pub/linux/libs/security/libcap/cap"
)


var (
	effective []cap.Value
	inheritable []cap.Value
)

func init() {
	inheritable = []cap.Value{cap.SYS_NICE, cap.SYS_RESOURCE}
	effective = append([]cap.Value{cap.SETPCAP}, inheritable...)
}

func rlimitNice() *unix.Rlimit {
	var rLimit unix.Rlimit
	if err := unix.Getrlimit(unix.RLIMIT_NICE, &rLimit); err != nil {
		rLimit = unix.Rlimit{20, 20}
	}
	return &rLimit
}

func numCPU() int {
	return runtime.NumCPU()
}

func setDumpable() error {
	attr, _ := unix.PrctlRetInt(unix.PR_GET_DUMPABLE, 0, 0, 0, 0)
	if attr != 1 {
		if err := unix.Prctl(unix.PR_SET_DUMPABLE, 1, 0, 0, 0); err != nil {
			return fmt.Errorf("%w: unable to set dumpable: %v", ErrFailure, err)
		}
	}
	return nil
}

func clearAllCapabilities() error {
	if err:= cap.ResetAmbient(); err != nil {
		return fmt.Errorf("%w: unable to reset ambient bits: %v", ErrFailure, err)
	}
	c := cap.GetProc()
	if err := c.Clear(); err != nil {
		return fmt.Errorf("%w: unable to clear %q: %v", ErrFailure, c, err)
	}
	if err := c.SetProc(); err != nil {
		return fmt.Errorf("%w: unable to update %q: %v", ErrFailure, c, err)
	}
	return setDumpable()
}

func getCapabilities() string {
	c:= cap.GetProc()
	result := []string{fmt.Sprintf("caps: %s", c.String())}
	var buf []string
	for _, val := range effective {
		flag, err := cap.GetAmbient(val)
		if err != nil {
			warn(fmt.Errorf("%w: unable to read ambient set: %v", ErrFailure, err))
		} else if flag {
			buf = append(buf, val.String())
		}
	}
	result = append(result, fmt.Sprintf("ambs: %s", strings.Join(buf, `,`)))
	return strings.Join(result, `, `)
}

func setCapabilities(enable bool) error {
	c := cap.GetProc()
	if err := c.SetFlag(cap.Effective, enable, effective...); err != nil {
		return fmt.Errorf("%w: unable to change flag: %v", ErrFailure, err)
	}
	if err := c.SetFlag(cap.Inheritable, enable, inheritable...); err != nil {
		return fmt.Errorf("%w: unable to change flag: %v", ErrFailure, err)
	}
	if err := c.SetProc(); err != nil {
		return fmt.Errorf("%w: unable to update %q: %v", ErrFailure, c, err)
	}
	return nil
}

func setAmbient(enable bool) (err error) {
	if e := cap.SetAmbient(enable, inheritable...); e != nil {
		err = fmt.Errorf("%w: unable to change ambient set: %v", ErrFailure, e)
	}
	return
}

func getAmbient() (enable bool) {
	for _, v := range inheritable {
		if flag, err := cap.GetAmbient(v); err != nil || !(flag) {
			return
		}
	}
	enable = true
	return
}

// vim: set ft=go fdm=indent ts=2 sw=2 tw=79 noet:
