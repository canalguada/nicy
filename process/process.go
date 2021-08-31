// build +linux
package process

import (
	"fmt"
	"encoding/json"
	"strconv"
	"strings"
	"sort"
	"os"
	"os/user"
	"path/filepath"
	"io/ioutil"
	"sync"
	"runtime"
	"unsafe"
	"golang.org/x/sys/unix"
)

func panicIfError(e error) {
	if e != nil {
		panic(e)
	}
}

func GetUid(pid int) int {
	var stat unix.Stat_t
	if unix.Stat(fmt.Sprintf("/proc/%d", pid), &stat) == nil {
		return int(stat.Uid)
	}
	return -1
}

func GetUser(pid int) (*user.User, error) {
	return user.LookupId(strconv.Itoa(GetUid(pid)))
}

func GetCgroup(pid int) (cgroup string, err error) {
	data, err := ioutil.ReadFile(fmt.Sprintf("/proc/%d/cgroup", pid))
	if err == nil {
		cgroup = strings.TrimSpace(string(data))
	}
	return
}

func GetOomScoreAdj(pid int) (score int, err error) {
	data, err := ioutil.ReadFile(fmt.Sprintf("/proc/%d/oom_score_adj", pid))
	if err == nil {
		return strconv.Atoi(strings.TrimSpace(string(data)))
	}
	return
}

type SchedulingPolicy struct {
	Class map[int]string
	NeedPriority []int
	NeedCredentials []int
	Low int
	High int
	None int
}

var IO SchedulingPolicy = SchedulingPolicy{
	Class: map[int]string{
		0: "none",
		1: "realtime",
		2: "best-effort",
		3: "idle",
	},
	NeedPriority: []int{1, 2},
	NeedCredentials: []int{1},
	Low: 7,
	High: 0,
	None: 4,
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
	NeedPriority: []int{1, 2},
	NeedCredentials: []int{1, 2},
	Low: 1,
	High: 99,
	None: 0,
}

type Sched_Param struct {
	Sched_Priority int
}

func Sched_GetScheduler(pid int) (int, error) {
	class, _, err := unix.Syscall(
		unix.SYS_SCHED_GETSCHEDULER, uintptr(pid), 0, 0,
	)
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

type Proc struct {
	Pid int					`json:"pid"`
	Ppid int				`json:"ppid"`
	Pgrp int				`json:"pgrp"`
	Uid int					`json:"uid"`
	User string			`json:"user"`
	State string		`json:"state"`
	Comm string			`json:"comm"`
	Cgroup string		`json:"cgroup"`
	Priority int		`json:"priority"`
	Nice int				`json:"nice"`
	NumThreads int	`json:"num_threads"`
	RTPrio int			`json:"rtprio"`
	Policy int			`json:"policy"`
	OomScoreAdj int	`json:"oom_score_adj"`
	IOPrioClass int	`json:"ioprio_class"`
	IOPrioData int	`json:"ionice"`
}

func parseInt(s string) int {
	v, err := strconv.Atoi(strings.TrimSpace(s))
	panicIfError(err)
	return v
}

func (p *Proc) setUser() (err error) {
	owner, err := GetUser(p.Pid)
	if err != nil {
		return
	}
	p.Uid = parseInt(owner.Uid)
	p.User = owner.Username
	return
}

func (p *Proc) setCgroup() (err error) {
	cgroup, err := GetCgroup(p.Pid)
	if err != nil {
		return
	}
	p.Cgroup = cgroup
	return
}

func (p *Proc) setOomScoreAdj() (err error) {
	scoreadj, err := GetOomScoreAdj(p.Pid)
	if err != nil {
		return
	}
	p.OomScoreAdj = scoreadj
	return
}

func (p *Proc) setIOPrio() (err error) {
	ioprio, err := IOPrio_Get(p.Pid)
	if err != nil {
		return
	}
	IOPrio_Split(ioprio, &p.IOPrioClass, &p.IOPrioData)
	return
}

func (p *Proc) ReadStat(stat string) (err error) {
	s := strings.Fields(stat)
	if !(strings.HasPrefix(s[1], "(")) || !(strings.HasSuffix(s[1], ")")) {
		return fmt.Errorf("invalid format, cannot extract comm value")
	}
	p.Comm = strings.Trim(s[1], "()")
	p.State = s[2]
	_, err = fmt.Sscanf(
		strings.Join(
			[]string{s[0], s[3], s[4], s[17], s[18], s[19], s[39], s[40]},
			" ",
		),
		"%d %d %d %d %d %d %d %d",
		&p.Pid,
		&p.Ppid,
		&p.Pgrp,
		&p.Priority,
		&p.Nice,
		&p.NumThreads,
		&p.RTPrio,
		&p.Policy,
	)
	return
}

func (p *Proc) LoadStat() (err error) {
	if p.Pid == 0 {
		return fmt.Errorf("pid required")
	}
	data, err := ioutil.ReadFile(fmt.Sprintf("/proc/%d/stat", p.Pid))
	if err == nil {
		err = p.ReadStat(string(data))
	}
	return
}

func NewProc(pid int) *Proc {
	p := &Proc{Pid: pid}
	panicIfError(p.LoadStat())
	// User
	panicIfError(p.setUser())
	// Cgroup
	panicIfError(p.setCgroup())
	// Oom score adjust
	panicIfError(p.setOomScoreAdj())
	// IO scheduling
	panicIfError(p.setIOPrio())
	return p
}

func GetCalling() *Proc {
	return NewProc(os.Getpid())
}

func NewProcFromStat(stat string) (p *Proc, err error) {
	p = new(Proc)
	// Stat
	err = p.ReadStat(stat)
	if err != nil {
		return
	}
	// User
	err = p.setUser()
	if err != nil {
		return
	}
	// Cgroup
	err = p.setCgroup()
	if err != nil {
		return
	}
	// Oom score adjust
	err = p.setOomScoreAdj()
	if err != nil {
		return
	}
	// IO scheduling
	err = p.setIOPrio()
	return
}

func (p *Proc) String() string {
	return fmt.Sprintf("%#v", *p)
}

func (p *Proc) Json() string {
	data, err := json.Marshal(*p)
	panicIfError(err)
	return string(data)
}

func (p *Proc) Raw() string {
	return fmt.Sprintf("%d %d %d %d %s %s %s %s %d %d %d %d %d %d %d %d",
		p.Pid,
		p.Ppid,
		p.Pgrp,
		p.Uid,
		p.User,
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
	)
}

func (p *Proc) Sched() string {
	return map[int]string{
		0: "other",
		1: "fifo",
		2: "rr",
		3: "batch",
		// 4: "iso", // Reserved but not implemented yet in linux
		5: "idle",
		6: "deadline",
	}[p.Policy]
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
	parts := strings.Split(p.Cgroup, "/")
	return fmt.Sprintf("[%d,%d,%d,%d,%q,%q,%q,%q,%q,%q,%d,%d,%d,%d,%d,%d,%q,%d]",
		p.Pid,
		p.Ppid,
		p.Pgrp,
		p.Uid,
		p.User,
		p.State,
		parts[1],
		parts[len(parts) - 1],
		p.Comm,
		p.Cgroup,
		p.Priority,
		p.Nice,
		p.NumThreads,
		p.RTPrio,
		p.Policy,
		p.OomScoreAdj,
		IO.Class[p.IOPrioClass],
		p.IOPrioData,
	)
}

func (p *Proc) RuntimeProperties() map[string]interface{} {
	result := make(map[string]interface{})
	result["nice"] = p.Nice
	result["sched"] = p.Sched()
	result["rtprio"] = p.RTPrio
	result["ioclass"] = IO.Class[p.IOPrioClass]
	result["ionice"] = p.IOPrioData
	result["oom_score_adj"] = p.OomScoreAdj
	return result
}

type MainProperties struct {
	Nice int				`json:"nice"`
	Sched string		`json:"sched"`
	RTPrio int			`json:"rtprio"`
	IOClass string	`json:"ioclass"`
	IONice int			`json:"ionice"`
	OomScoreAdj int	`json:"oom_score_adj"`
}

type ProcMap struct {
	Pid int					`json:"pid"`
	Ppid int				`json:"ppid"`
	Pgrp int				`json:"pgrp"`
	Uid int					`json:"uid"`
	User string			`json:"user"`
	State string		`json:"state"`
	Slice string		`json:"slice"`
	Unit string			`json:"unit"`
	Comm string			`json:"comm"`
	Cgroup string		`json:"cgroup"`
	Priority int		`json:"priority"`
	NumThreads int	`json:"num_threads"`
	Runtime MainProperties	`json:"runtime"`
}

func NewProcMap(p *Proc) *ProcMap {
	parts := strings.Split(p.Cgroup, "/")
	pm := &ProcMap{
		Pid: p.Pid,
		Ppid: p.Ppid,
		Pgrp: p.Pgrp,
		Uid: p.Uid,
		User: p.User,
		State: p.State,
		Slice: parts[1],
		Unit: parts[len(parts) - 1],
		Comm: p.Comm,
		Cgroup: p.Cgroup,
		Priority: p.Priority,
		NumThreads: p.NumThreads,
		Runtime: MainProperties{
			Nice: p.Nice,
			Sched: p.Sched(),
			RTPrio: p.RTPrio,
			IOClass: IO.Class[p.IOPrioClass],
			IONice: p.IOPrioData,
			OomScoreAdj: p.OomScoreAdj,
		},
	}
	return pm
}

func (pm *ProcMap) Map() map[string]interface{} {
	data, err := json.Marshal(*pm)
	panicIfError(err)
	result := make(map[string]interface{})
	panicIfError(json.Unmarshal(data, &result))
	return result
}

func (p *Proc) Map() map[string]interface{} {
	return NewProcMap(p).Map()
}

func (p *Proc) InUserSlice() bool {
	return strings.Contains(p.Cgroup, "/user.slice/")
}

func (p *Proc) InSystemSlice() bool {
	return !(p.InUserSlice())
}

func Stat(stat string) string {
	p, err := NewProcFromStat(stat)
	panicIfError(err)
	return p.Values()
}

type Filter func(p *Proc, err error) bool

var (
	filterUser = func(p *Proc, err error) bool {
		return err == nil && p.Uid == os.Getuid() && p.InUserSlice()
	}
	filterGlobal = func(p *Proc, err error) bool {
		return err == nil && p.InUserSlice()
	}
	filterSystem = func(p *Proc, err error) bool {
		return err == nil && p.InSystemSlice()
	}
	filterAll = func(p *Proc, err error) bool {
		return err == nil
	}
)

func GetFilter(what string) Filter {
	switch strings.ToLower(what) {
	case "user":
		return filterUser
	case "global":
		return filterGlobal
	case "system":
		return filterSystem
	case "all":
		return filterAll
	}
	return filterUser
}

type Formatter func (p *Proc) string

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

// ByPid implements sort.Interface for []Proc based on Pid field
type ByPid []Proc

func (s ByPid) Len() int { return len(s) }

func (s ByPid) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

func (s ByPid) Less(i, j int) bool { return s[i].Pid < s[j].Pid }

// AllProcs returns a list of all currently available processes.
// Filters result with filterFunc, if not nil.
func AllProcs(filterFunc Filter) (result []Proc) {
	files, _ := filepath.Glob("/proc/[0-9]*/stat")
	size := len(files)
	result = make([]Proc, 0, size)
	// make our channels for communicating work and results
	stats := make(chan string, size)
	procs := make(chan Proc, size)
	// spin up workers and use a sync.WaitGroup to indicate completion
	var count = runtime.GOMAXPROCS(0)
	var wg sync.WaitGroup
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(stats <-chan string, procs chan<- Proc, wg *sync.WaitGroup){
			defer wg.Done()
			var p *Proc
			var err error
			for stat := range stats {
				p, err = NewProcFromStat(stat)
				if filterFunc(p, err) {
					procs <- *p
				}
			}
		}(stats, procs, &wg)
	}
	// wait on the workers to finish and close the result channel
	// to signal downstream that all work is done
	go func() {
		defer close(procs)
		wg.Wait()
	}()
	// start sending jobs
	go func() {
		defer close(stats)
		for _, file := range files {
			data, err := ioutil.ReadFile(file)
			if err == nil {
				stats <- string(data)
			}
		}
	}()
	// collect result from procs channel
	for p := range procs {
		result = append(result, p)
	}
	// sort by Pid
	sort.Sort(ByPid(result))
	return
}

// vim: set ft=go fdm=indent ts=2 sw=2 tw=79 noet:
