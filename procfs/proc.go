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
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/sys/unix"
)

var (
	CPUSched map[int]string
)

func init() {
	CPUSched = make(map[int]string)
	for k, v := range CPU.Class {
		CPUSched[k] = strings.ToLower(strings.TrimPrefix(v, `SCHED_`))
	}
}

func GetStat(path string) (stat unix.Stat_t, err error) {
	err = unix.Stat(path, &stat)
	return
}

func GetUid(pid int) int {
	if stat, err := GetStat(fmt.Sprintf("/proc/%d", pid)); err == nil {
		return int(stat.Uid)
	}
	return -1
}

func GetUser(uid int) (*user.User, error) {
	return user.LookupId(strconv.Itoa(uid))
}

func GetCgroup(pid int) (cgroup string, err error) {
	if data, err := GetResource(pid, "cgroup"); err == nil {
		cgroup = strings.TrimSpace(string(data))
	}
	return
}

func GetOomScoreAdj(pid int) (score int, err error) {
	if data, err := GetResource(pid, "oom_score_adj"); err == nil {
		return strconv.Atoi(strings.TrimSpace(string(data)))
	}
	return
}

type Proc struct {
	ProcStat
	Uid         int       `json:"uid"`
	owner       user.User `json:"-"`
	Cgroup      string    `json:"cgroup"`
	Slice       string    `json:"slice"`
	Unit        string    `json:"unit"`
	OomScoreAdj int       `json:"oom_score_adj"`
	IOPrioClass int       `json:"ioprio_class"`
	IOPrioData  int       `json:"ionice"`
}

func (p *Proc) setUser() (err error) {
	p.Uid = GetUid(p.Pid)
	if owner, err := GetUser(p.Uid); err == nil {
		p.owner = *owner
	}
	return
}

func (p *Proc) setCgroup() (err error) {
	if cgroup, err := GetCgroup(p.Pid); err == nil {
		if cgroup != "0::/" {
			parts := strings.Split(cgroup, `/`)
			p.Cgroup, p.Slice, p.Unit = cgroup, parts[1], parts[len(parts)-1]
		} else {
			p.Cgroup, p.Slice, p.Unit = "0::/", "", ""
		}
	}
	return
}

func (p *Proc) setOomScoreAdj() (err error) {
	if scoreadj, err := GetOomScoreAdj(p.Pid); err == nil {
		p.OomScoreAdj = scoreadj
	}
	return
}

func (p *Proc) setIOPrio() (err error) {
	if ioprio, err := IOPrio_Get(p.Pid); err == nil {
		IOPrio_Split(ioprio, &p.IOPrioClass, &p.IOPrioData)
	}
	return
}

type setter = func() error

func (p *Proc) setters() []setter {
	return []setter{p.setUser, p.setCgroup, p.setOomScoreAdj, p.setIOPrio}
}

func NewProc(pid int) *Proc {
	p := &Proc{ProcStat: ProcStat{Pid: pid}}
	if err := p.ProcStat.Read(pid); err != nil {
		panic(err)
	}
	for _, function := range p.setters() {
		if err := function(); err != nil {
			panic(err)
		}
	}
	return p
}

func GetCalling() *Proc {
	return NewProc(os.Getpid())
}

func NewProcFromStat(stat string) (p *Proc, err error) {
	p = new(Proc)
	// Stat
	err = p.ProcStat.Load(stat)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return
	}
	for _, function := range p.setters() {
		if err = function(); err != nil {
			break
		}
	}
	return
}

func (p *Proc) entries() string {
	return fmt.Sprintf("Uid: %v, owner: %+v, Cgroup: %v, Slice: %v, Unit: %v, RTPrio: %v, Policy: %v, OomScoreAdj: %v, IOPrioData: %v, IOPrioClass: %v",
		p.Uid, p.owner, p.Cgroup, p.Slice, p.Unit, p.RTPrio, p.Policy, p.OomScoreAdj, p.IOPrioData, p.IOPrioClass,
	)
}

func (p *Proc) GoString() string {
	return "Proc" + fmt.Sprintf("{ProcStat: %s, %s}", p.ProcStat.GoString(), p.entries())
}

func (p *Proc) String() string {
	return fmt.Sprintf("{ProcStat: %s, %s}", p.ProcStat.String(), p.entries())
}

func (p Proc) Json() (result string) {
	if data, err := json.Marshal(p); err != nil {
		panic(err)
	} else {
		result = string(data)
	}
	return
}

func (p Proc) Raw() string {
	return strings.TrimSuffix(
		fmt.Sprintln(
			p.Pid,
			p.Ppid,
			p.Pgrp,
			p.Uid,
			p.Username(),
			p.State,
			p.Comm,
			p.Cgroup,
			p.Priority,
			p.Nice,
			p.NumThreads,
			p.RTPrio,
			p.Policy,
			p.OomScoreAdj,
			p.IOPrioClass,
			p.IOPrioData,
		),
		"\n",
	)
}

func (p *Proc) Sched() string {
	return CPUSched[p.Policy]
}

func (p *Proc) CPUSchedInfo() string {
	return fmt.Sprintf(
		"%d:%s:%d", p.Policy, p.Sched(), p.RTPrio,
	)
}

func (p *Proc) IOClass() string {
	return IO.Class[p.IOPrioClass]
}

func (p *Proc) IOSchedInfo() string {
	return fmt.Sprintf(
		"%d:%s:%d", p.IOPrioClass, p.IOClass(), p.IOPrioData,
	)
}

func (p *Proc) Values() string {
	return fmt.Sprintf("[%d,%d,%d,%d,%q,%q,%q,%q,%q,%q,%d,%d,%d,%d,%d,%d,%q,%d]",
		p.Pid,
		p.Ppid,
		p.Pgrp,
		p.Uid,
		p.Username(),
		p.State,
		p.Slice,
		p.Unit,
		p.Comm,
		p.Cgroup,
		p.Priority,
		p.Nice,
		p.NumThreads,
		p.RTPrio,
		p.Policy,
		p.OomScoreAdj,
		p.IOClass(),
		p.IOPrioData,
	)
}

func (p *Proc) GetStringMap() (result map[string]interface{}) {
	if data, err := json.Marshal(*p); err != nil {
		panic(err)
	} else if err := json.Unmarshal(data, &result); err != nil {
		panic(err)
	}
	return
}

func (p *Proc) Username() string {
	return p.owner.Username
}

func (p *Proc) InUserSlice() bool {
	return p.Slice == "user.slice"
}

func (p *Proc) InSystemSlice() bool {
	return !(p.InUserSlice())
}

func Stat(stat string) (result string) {
	if p, err := NewProcFromStat(stat); err != nil {
		panic(err)
	} else {
		result = p.Values()
	}
	return
}

type Formatter func(p *Proc) string

func GetFormatter(format string) Formatter {
	switch strings.ToLower(format) {
	case "json":
		return func(p *Proc) string { return p.Json() }
	case "raw":
		return func(p *Proc) string { return p.Raw() }
	case "values":
		return func(p *Proc) string { return p.Values() }
	default:
		return func(p *Proc) string { return p.String() }
	}
}

// ProcByPid implements sort.Interface for []*Proc based on Pid field
type ProcByPid []*Proc

func (s ProcByPid) Len() int           { return len(s) }
func (s ProcByPid) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s ProcByPid) Less(i, j int) bool { return s[i].Pid < s[j].Pid }

// FilteredProcs returns a slice of Proc for filtered processes.
func FilteredProcs(filter Filterer[Proc]) (result []*Proc) {
	files, err := filepath.Glob("/proc/[0-9]*/stat")
	if err != nil {
		return
	}
	// make our channels for communicating work and results
	stats := make(chan string, len(files))
	// spin up workers and use a sync.WaitGroup to indicate completion
	var wg sync.WaitGroup
	for i := 0; i < runtime.GOMAXPROCS(0); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for stat := range stats {
				if p, err := NewProcFromStat(stat); filter.Filter(p, err) {
					result = append(result, p)
				}
			}
		}()
	}
	// start sending jobs
	wg.Add(1)
	go func() {
		defer wg.Done()
		for _, file := range files {
			if data, err := os.ReadFile(file); err == nil {
				stats <- string(data)
			}
		}
		close(stats)
	}()
	// wait on the workers to finish
	wg.Wait()
	// sort by Pid
	sort.Sort(ProcByPid(result))
	return
}

// AllProcs returns a slice of Proc for all processes.
func AllProcs() (result []*Proc) {
	return FilteredProcs(GetFilterer("all"))
}

// vim: set ft=go fdm=indent ts=2 sw=2 tw=79 noet:
