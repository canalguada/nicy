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
	"strconv"
)

// Commands here only change properties of some process or group of processes.

func renice(priority, pid, pgrp int) Command {
	// expects priority in range [-20..19]
	result := []string{"renice", "-n", strconv.Itoa(priority)}
	if pid == 0 && pgrp == 0 {
		result = append(result, "-p", "$$")
	} else if pgrp > 0 {
		result = append(result, "-g", strconv.Itoa(pgrp))
	} else if pid > 0 {
		result = append(result, "-p", strconv.Itoa(pid))
	}
	return NewCommand(result...)
}

func choom(score, pid int) Command {
	// expects score in range [-1000..1000]
	result := []string{"choom", "-n", strconv.Itoa(score)}
	if pid > 0 {
		result = append(result, "-p", strconv.Itoa(pid))
	} else if pid == 0 {
		result = append(result, "-p", "$$")
	} else {
		result = append(result, "--")
	}
	return NewCommand(result...)
}

func chrt(sched string, rtprio int, pid int) Command {
	// expects sched in ["other", "fifo", "rr", "batch", "idle"]
	// expects rtprio in range [1..99]
	result := []string{"chrt"}
	if len(sched) > 0 {
		result = append(result, "--"+sched)
	}
	if pid > 0 {
		result = append(result, "-a", "-p", strconv.Itoa(rtprio), strconv.Itoa(pid))
	} else if pid == 0 {
		result = append(result, "-a", "-p", strconv.Itoa(rtprio), "$$")
	} else {
		result = append(result, strconv.Itoa(rtprio))
	}
	return NewCommand(result...)
}

func ionice(class string, level int, pid int, pgrp int) Command {
	// expects class in ["none", "realtime", "best-effort", "idle"]
	// expects level in range [0..7] when it must be set
	result := []string{"ionice"}
	if len(class) > 0 {
		policy := map[string]string{
			"none":        "0",
			"realtime":    "1",
			"best-effort": "2",
			"idle":        "3",
		}[class]
		result = append(result, "-c", policy)
	}
	if level >= 0 {
		result = append(result, "-n", strconv.Itoa(level))
	}
	if pgrp > 0 {
		result = append(result, "-P", strconv.Itoa(pgrp))
	} else if pid > 0 {
		result = append(result, "-p", strconv.Itoa(pid))
	} else if pid == 0 {
		result = append(result, "-p", "$$")
	}
	return NewCommand(result...)
}

// vim: set ft=go fdm=indent ts=2 sw=2 tw=79 noet:
