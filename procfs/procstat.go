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
	"fmt"
	"io/ioutil"
	"strings"
)

// See the following discussions:
//
// - https://github.com/prometheus/node_exporter/issues/52
// - https://github.com/prometheus/procfs/pull/2
// - http://stackoverflow.com/questions/17410841/how-does-user-hz-solve-the-jiffy-scaling-issue
const userHZ = 100

var (
	stProcStat string
)

func init() {
	stProcStat = "{" +
		"Pid: %d, " +
		"Comm: %s, " +
		"State: %s, " +
		"Ppid: %d, " +
		"Pgrp: %d, " +
		"Session: %d, " +
		"TtyNr: %d, " +
		"TPGid: %d, " +
		"Flags: %d, " +
		"MinFlt: %d, " +
		"CMinFlt: %d, " +
		"MajFlt: %d, " +
		"CMajFlt: %d, " +
		"UTime: %d, " +
		"STime: %d, " +
		"CUTime: %d, " +
		"CSTime: %d, " +
		"Priority: %d, " +
		"Nice: %d, " +
		"NumThreads: %d, " +
		"ITRealValue: %d, " +
		"StartTime: %d, " +
		"VSize: %d, " +
		"Rss: %d, " +
		"RssLim: %d, " +
		"StartCode: %d, " +
		"EndCode: %d, " +
		"StartStack: %d, " +
		"KStkESP: %d, " +
		"KStkEIP: %d, " +
		"Signal: %d, " +
		"Blocked: %d, " +
		"SigIgnore: %d, " +
		"SigCatch: %d, " +
		"WChan: %d, " +
		"NSwap: %d, " +
		"CNSwap: %d, " +
		"ExitSignal: %d, " +
		"Processor: %d, " +
		"RTPrio: %d, " +
		"Policy: %d, " +
		"DelayAcctBlkIOTicks: %d, " +
		"GuestTime: %d, " +
		"CGuestTime: %d, " +
		"StartData: %d, " +
		"EndData: %d, " +
		"StartBrk: %d, " +
		"ArgStart: %d, " +
		"ArgEnd: %d, " +
		"EnvStart: %d, " +
		"EnvEnd: %d, " +
		"ExitCode: %d" +
		"}"
}

func GetResource(pid int, rc string) ([]byte, error) {
	return ioutil.ReadFile(fmt.Sprintf("/proc/%d/%s", pid, rc))
}

// % cat /proc/$(pidof nvim)/stat
// 14066 (nvim) S 14064 14063 14063 0 -1 4194304 5898 6028 495 394 487 64 88 68 39 19 1 0 1256778 18685952 2655 4294967295 4620288 7319624 3219630688 0 0 0 0 2 536891909 1 0 0 17 0 0 0 0 0 0 8366744 8490776 38150144 3219638342 3219638506 3219638506 3219644398 0

type ProcStat struct {
	stat                string `json:"-"`
	Pid                 int    `json:"pid"`                   // (1) %d *
	Comm                string `json:"comm"`                  // (2) %s *
	State               string `json:"state"`                 // (3) %c *
	Ppid                int    `json:"ppid"`                  // (4) %d *
	Pgrp                int    `json:"pgrp"`                  // (5) %d *
	Session             int    `json:"session"`               // (6) %d
	TtyNr               int    `json:"tty_nr"`                // (7) %d
	TPGid               int    `json:"tpgid"`                 // (8) %d
	Flags               uint   `json:"flags"`                 // (9) %u
	MinFlt              uint   `json:"minflt"`                // (10) %lu
	CMinFlt             uint   `json:"cminflt"`               // (11) %lu
	MajFlt              uint   `json:"majflt"`                // (12) %lu
	CMajFlt             uint   `json:"cmajflt"`               // (13) %lu
	UTime               uint   `json:"utime"`                 // (14) %lu
	STime               uint   `json:"stime"`                 // (15) %lu
	CUTime              int    `json:"cutime"`                // (16) %ld
	CSTime              int    `json:"cstime"`                // (17) %ld
	Priority            int    `json:"priority"`              // (18) %ld *
	Nice                int    `json:"nice"`                  // (19) %ld *
	NumThreads          int    `json:"num_threads"`           // (20) %ld *
	ITRealValue         int    `json:"itrealvalue"`           // (21) %ld
	StartTime           uint64 `json:"starttime"`             // (22) %llu
	VSize               uint   `json:"vsize"`                 // (23) %lu
	Rss                 int    `json:"rss"`                   // (24) %ld
	RssLim              uint   `json:"rsslim"`                // (25) %lu
	StartCode           uint   `json:"startcode"`             // (26) %lu
	EndCode             uint   `json:"endcode"`               // (27) %lu
	StartStack          uint   `json:"startstack"`            // (28) %lu
	KStkESP             uint   `json:"kstkesp"`               // (29) %lu
	KStkEIP             uint   `json:"kstkeip"`               // (30) %lu
	Signal              uint   `json:"signal"`                // (31) %lu
	Blocked             uint   `json:"blocked"`               // (32) %lu
	SigIgnore           uint   `json:"sigignore"`             // (33) %lu
	SigCatch            uint   `json:"sigcatch"`              // (34) %lu
	WChan               uint   `json:"wchan"`                 // (35) %lu
	NSwap               uint   `json:"nswap"`                 // (36) %lu -
	CNSwap              uint   `json:"cnswap"`                // (37) %lu -
	ExitSignal          int    `json:"exit_signal"`           // (38) %d
	Processor           int    `json:"processor"`             // (39) %d
	RTPrio              int    `json:"rtprio"`                // (40) %u *
	Policy              int    `json:"policy"`                // (41) %u *
	DelayAcctBlkIOTicks uint64 `json:"delayacct_blkio_ticks"` // (42) %llu
	GuestTime           uint   `json:"guest_time"`            // (43) %lu
	CGuestTime          int    `json:"cguest_time"`           // ((44) %ld
	StartData           uint   `json:"start_data"`            // (45) %lu
	EndData             uint   `json:"end_data"`              // (46) %lu
	StartBrk            uint   `json:"start_brk"`             // (47) %lu
	ArgStart            uint   `json:"arg_start"`             // (48) %lu
	ArgEnd              uint   `json:"arg_end"`               // (49) %lu
	EnvStart            uint   `json:"env_start"`             // (50) %lu
	EnvEnd              uint   `json:"env_end"`               // (51) %lu
	ExitCode            int    `json:"exit_code"`             // (52) %d
}

func (stat *ProcStat) Load(buffer string) (err error) {
	stat.stat = buffer
	var comm string
	// parse
	_, err = fmt.Sscan(
		buffer,
		&stat.Pid, // (1) %d *
		&comm,
		// &stat.Comm,				// (2) %s *
		&stat.State,               // (3) %c *
		&stat.Ppid,                // (4) %d *
		&stat.Pgrp,                // (5) %d *
		&stat.Session,             // (6) %d
		&stat.TtyNr,               // (7) %d
		&stat.TPGid,               // (8) %d
		&stat.Flags,               // (9) %u
		&stat.MinFlt,              // (10) %lu
		&stat.CMinFlt,             // (11) %lu
		&stat.MajFlt,              // (12) %lu
		&stat.CMajFlt,             // (13) %lu
		&stat.UTime,               // (14) %lu
		&stat.STime,               // (15) %lu
		&stat.CUTime,              // (16) %ld
		&stat.CSTime,              // (17) %ld
		&stat.Priority,            // (18) %ld *
		&stat.Nice,                // (19) %ld *
		&stat.NumThreads,          // (20) %ld *
		&stat.ITRealValue,         // (21) %ld
		&stat.StartTime,           // (22) %llu
		&stat.VSize,               // (23) %lu
		&stat.Rss,                 // (24) %ld
		&stat.RssLim,              // (25) %lu
		&stat.StartCode,           // (26) %lu
		&stat.EndCode,             // (27) %lu
		&stat.StartStack,          // (28) %lu
		&stat.KStkESP,             // (29) %lu
		&stat.KStkEIP,             // (30) %lu
		&stat.Signal,              // (31) %lu
		&stat.Blocked,             // (32) %lu
		&stat.SigIgnore,           // (33) %lu
		&stat.SigCatch,            // (34) %lu
		&stat.WChan,               // (35) %lu
		&stat.NSwap,               // (36) %lu -
		&stat.CNSwap,              // (37) %lu -
		&stat.ExitSignal,          // (38) %d
		&stat.Processor,           // (39) %d
		&stat.RTPrio,              // (40) %u *
		&stat.Policy,              // (41) %u *
		&stat.DelayAcctBlkIOTicks, // (42) %llu
		&stat.GuestTime,           // (43) %lu
		&stat.CGuestTime,          // ((44) %ld
		&stat.StartData,           // (45) %lu
		&stat.EndData,             // (46) %lu
		&stat.StartBrk,            // (47) %lu
		&stat.ArgStart,            // (48) %lu
		&stat.ArgEnd,              // (49) %lu
		&stat.EnvStart,            // (50) %lu
		&stat.EnvEnd,              // (51) %lu
		&stat.ExitCode,            // (52) %d
	)
	stat.Comm = strings.Trim(comm, "()")
	return
}

func (stat *ProcStat) Read(pid int) (err error) {
	// read stat data for pid
	if data, err := GetResource(pid, "stat"); err == nil {
		// load
		err = stat.Load(string(data))
	}
	return
}

func (stat *ProcStat) GoString() string {
	return "ProcStat" + stat.String()
}

func (stat *ProcStat) String() string {
	return fmt.Sprintf(
		stProcStat,
		stat.Pid,
		stat.Comm,
		stat.State,
		stat.Ppid,
		stat.Pgrp,
		stat.Session,
		stat.TtyNr,
		stat.TPGid,
		stat.Flags,
		stat.MinFlt,
		stat.CMinFlt,
		stat.MajFlt,
		stat.CMajFlt,
		stat.UTime,
		stat.STime,
		stat.CUTime,
		stat.CSTime,
		stat.Priority,
		stat.Nice,
		stat.NumThreads,
		stat.ITRealValue,
		stat.StartTime,
		stat.VSize,
		stat.Rss,
		stat.RssLim,
		stat.StartCode,
		stat.EndCode,
		stat.StartStack,
		stat.KStkESP,
		stat.KStkEIP,
		stat.Signal,
		stat.Blocked,
		stat.SigIgnore,
		stat.SigCatch,
		stat.WChan,
		stat.NSwap,
		stat.CNSwap,
		stat.ExitSignal,
		stat.Processor,
		stat.RTPrio,
		stat.Policy,
		stat.DelayAcctBlkIOTicks,
		stat.GuestTime,
		stat.CGuestTime,
		stat.StartData,
		stat.EndData,
		stat.StartBrk,
		stat.ArgStart,
		stat.ArgEnd,
		stat.EnvStart,
		stat.EnvEnd,
		stat.ExitCode,
	)
}

// CPUTime returns the total CPU user and system time in seconds.
func (stat *ProcStat) CPUTime() float64 {
	return float64(stat.UTime+stat.STime) / userHZ
}

// vim: set ft=go fdm=indent ts=2 sw=2 tw=79 noet:
