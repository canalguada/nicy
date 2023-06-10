// build +linux

/*
Copyright © 2022 David Guadalupe <guadalupe.david@gmail.com>

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
package procfs

import (
	"unsafe"

	"golang.org/x/sys/unix"
)

type SchedulingPolicy struct {
	Class           map[int]string
	NeedPriority    []int
	NeedCredentials []int
	Low             int
	High            int
	None            int
}

var IO SchedulingPolicy = SchedulingPolicy{
	Class: map[int]string{
		0: "none",
		1: "realtime",
		2: "best-effort",
		3: "idle",
	},
	NeedPriority:    []int{1, 2},
	NeedCredentials: []int{1},
	Low:             7,
	High:            0,
	None:            4,
}

const (
	IOPRIO_CLASS_NONE = iota
	IOPRIO_CLASS_RT
	IOPRIO_CLASS_BE
	IOPRIO_CLASS_IDLE
)

const (
	_ = iota
	IOPRIO_WHO_PROCESS
	IOPRIO_WHO_PGRP
	IOPRIO_WHO_USER
)

const IOPRIO_CLASS_SHIFT = 13

const (
	NONE = iota
	REALTIME
	BEST_EFFORT
	IDLE
)

func IOPrio_Get(pid int) (int, error) {
	ioprio, _, err := unix.Syscall(
		unix.SYS_IOPRIO_GET, IOPRIO_WHO_PROCESS, uintptr(pid), 0,
	)
	if err == 0 {
		return int(ioprio), nil
	}
	return -1, err
}

func IOPrio_Split(ioprio int, class, data *int) {
	// From https://www.kernel.org/doc/html/latest/block/ioprio.html
	*class = ioprio >> IOPRIO_CLASS_SHIFT
	*data = ioprio & 0xff
}

const (
	SCHED_OTHER = iota
	SCHED_FIFO
	SCHED_RR
	SCHED_BATCH
	SCHED_ISO
	SCHED_IDLE
	SCHED_DEADLINE
)

var CPU SchedulingPolicy = SchedulingPolicy{
	Class: map[int]string{
		0: "SCHED_OTHER",
		1: "SCHED_FIFO",
		2: "SCHED_RR",
		3: "SCHED_BATCH",
		// 4: "SCHED_ISO", // Reserved but not implemented yet in linux
		5: "SCHED_IDLE",
		6: "SCHED_DEADLINE",
	},
	NeedPriority:    []int{1, 2},
	NeedCredentials: []int{1, 2},
	Low:             1,
	High:            99,
	None:            0,
}

type Sched_Param struct {
	Sched_Priority int
}

func Sched_GetScheduler(pid int) (int, error) {
	class, _, err := unix.Syscall(unix.SYS_SCHED_GETSCHEDULER, uintptr(pid), 0, 0)
	if err == 0 {
		return int(class), nil
	}
	return -1, err
}

func Sched_GetParam(pid int) (int, error) {
	param := Sched_Param{}
	_, _, err := unix.Syscall(
		unix.SYS_SCHED_GETPARAM, uintptr(pid), uintptr(unsafe.Pointer(&param)), 0,
	)
	if err == 0 {
		return param.Sched_Priority, nil
	}
	return -1, err
}

// vim: set ft=go fdm=indent ts=2 sw=2 tw=79 noet:
