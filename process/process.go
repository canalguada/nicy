// build +linux
package process

import (
	"fmt"
	"log"
	"encoding/json"
	"strconv"
	"strings"
	// "bytes"
	"sort"
	"os"
	"os/user"
	"path/filepath"
	"io/ioutil"
	"regexp"
	"sync"
	"runtime"
	"unsafe"
	"golang.org/x/sys/unix"
)

var (
	reStat *regexp.Regexp
)

func init() {
	reStat = regexp.MustCompile(`(?m)^(?P<pid>\d+) \((?P<comm>.+)\) (?P<fields>.*)$`)
}

func GetResource(pid int, rc string) ([]byte, error) {
	return ioutil.ReadFile(fmt.Sprintf("/proc/%d/%s", pid, rc))
}

func GetUid(pid int) int {
	var stat unix.Stat_t
	if unix.Stat(fmt.Sprintf("/proc/%d", pid), &stat) == nil {
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

type ProcStat struct {
	stat string
	Pid int					`json:"pid"`						// (1) %d
	Comm string			`json:"comm"`						// (2) %s
	State string		`json:"state"`					// (3) %s
	Ppid int				`json:"ppid"`						// (4) %d
	Pgrp int				`json:"pgrp"`						// (5) %d
	Priority int		`json:"priority"`				// (18) %d
	Nice int				`json:"nice"`						// (19) %d
	NumThreads int	`json:"num_threads"`		// (20) %d
	RTPrio int			`json:"rtprio"`					// (40) %d
	Policy int			`json:"policy"`					// (41) %d
}

func (stat *ProcStat) parseFields(submatch string) (err error) {
	var (
		nums = [...]int{0, 1, 2, 15, 16, 17, 37, 38}
		input []string
	)
	s := strings.Fields(submatch)
	for _, pos := range nums {
		input = append(input, s[pos])
	}
	_, err = fmt.Sscanf(
		strings.Join(input, ` `),
		"%s %d %d %d %d %d %d %d",
		&stat.State,
		&stat.Ppid,
		&stat.Pgrp,
		&stat.Priority,
		&stat.Nice,
		&stat.NumThreads,
		&stat.RTPrio,
		&stat.Policy,
	)
	return
}

func (stat *ProcStat) Load(buffer string) (err error) {
	stat.stat = buffer
	// parse
	matches := reStat.FindStringSubmatch(buffer)
	// check pid value
	if value, err := strconv.Atoi(matches[1]); err == nil {
		stat.Pid = value
		stat.Comm = matches[2]
		err = stat.parseFields(matches[3])
	}
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

type Proc struct {
	ProcStat
	Uid int						`json:"uid"`
	owner *user.User	`json:"-"`
	Cgroup [3]string	`json:"cgroup"`
	OomScoreAdj int		`json:"oom_score_adj"`
	IOPrioClass int		`json:"ioprio_class"`
	IOPrioData int		`json:"ionice"`
}

func (p *Proc) setUser() (err error) {
	p.Uid = GetUid(p.Pid)
	if owner, err := GetUser(p.Uid); err == nil {
		p.owner = owner
	}
	return
}

func (p *Proc) setCgroup() (err error) {
	if cgroup, err := GetCgroup(p.Pid); err == nil {
		parts := strings.Split(cgroup, `/`)
		p.Cgroup = [3]string{cgroup, parts[1], parts[len(parts) - 1]}
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
	if err:= p.ProcStat.Read(pid); err != nil {
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
		return
	}
	for _, function := range p.setters() {
		if err = function(); err != nil {
			break
		}
	}
	return
}

func (p *Proc) GoString() string {
	return "Proc" + p.String()
}

func (p *Proc) String() string {
	return fmt.Sprintf(
		"{Pid: %d, Comm: %q, State: %q, Ppid: %d, Pgrp: %d, Priority: %d, Nice: %d, NumThreads: %d, Uid: %d, Cgroup: [%v %v %v], RTPrio: %d, Policy: %d, OomScoreAdj: %d, IOPrioData: %d, IOPrioClass: %d}",
		p.Pid, p.Comm, p.State, p.Ppid, p.Pgrp, p.Priority, p.Nice, p.NumThreads,
		p.Uid, p.Cgroup[0], p.Cgroup[1], p.Cgroup[2],
		p.RTPrio, p.Policy, p.OomScoreAdj, p.IOPrioData, p.IOPrioClass,
	)
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
	return fmt.Sprintf("%d %d %d %d %s %s %s %s %d %d %d %d %d %d %d %d",
		p.Pid,
		p.Ppid,
		p.Pgrp,
		p.Uid,
		p.owner.Username,
		p.State,
		p.Comm,
		p.Cgroup[0],
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
	return fmt.Sprintf("[%d,%d,%d,%d,%q,%q,%q,%q,%q,%q,%d,%d,%d,%d,%d,%d,%q,%d]",
		p.Pid,
		p.Ppid,
		p.Pgrp,
		p.Uid,
		p.owner.Username,
		p.State,
		p.Cgroup[1],
		p.Cgroup[2],
		p.Comm,
		p.Cgroup[0],
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

type MainProperties struct {
	Nice int				`json:"nice"`
	Sched string		`json:"sched"`
	RTPrio int			`json:"rtprio"`
	IOClass string	`json:"ioclass"`
	IONice int			`json:"ionice"`
	OomScoreAdj int	`json:"oom_score_adj"`
}

func (p *MainProperties) GetStringMap() (result map[string]interface{}) {
	if data, err := json.Marshal(*p); err != nil {
		panic(err)
	} else if err := json.Unmarshal(data, &result); err != nil {
		panic(err)
	}
	return
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
	pm := &ProcMap{
		Pid: p.Pid,
		Ppid: p.Ppid,
		Pgrp: p.Pgrp,
		Uid: p.Uid,
		User: p.owner.Username,
		State: p.State,
		Slice: p.Cgroup[1],
		Unit: p.Cgroup[2],
		Comm: p.Comm,
		Cgroup: p.Cgroup[0],
		Priority: p.Priority,
		NumThreads: p.NumThreads,
		Runtime: MainProperties{
			Nice: p.Nice,
			Sched: p.Sched(),
			RTPrio: p.RTPrio,
			IOClass: p.IOClass(),
			IONice: p.IOPrioData,
			OomScoreAdj: p.OomScoreAdj,
		},
	}
	return pm
}

func (pm *ProcMap) GetStringMap() (result map[string]interface{}) {
	if data, err := json.Marshal(*pm); err != nil {
		panic(err)
	} else if err := json.Unmarshal(data, &result); err != nil {
		panic(err)
	}
	return
}

func (p *Proc) GetStringMap() map[string]interface{} {
	return NewProcMap(p).GetStringMap()
}

func (p *Proc) InUserSlice() bool {
	return p.Cgroup[1] == "user.slice"
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

type ProcFilter struct {
	filter func(p *Proc, err error) bool
	message string
}

func (self ProcFilter) Filter(p *Proc, err error) bool {
	return self.filter(p, err)
}

func (self ProcFilter) String() string {
	return self.message
}

type Filterer interface {
	Filter(p *Proc, err error) bool
	String() string
}

func GetFilterer(scope string) ProcFilter {
	switch strings.ToLower(scope) {
	case "global":
		return ProcFilter{
			filter: func(p *Proc, err error) bool {
				return err == nil && p.InUserSlice()
			},
			message: "processes inside any user slice",
		}
	case "system":
		return ProcFilter{
			filter: func(p *Proc, err error) bool {
				return err == nil && p.InSystemSlice()
			},
			message: "processes inside system slice",
		}
	case "all":
		return ProcFilter{
			filter: func(p *Proc, err error) bool {
				return err == nil
			},
			message: "all processes",
		}
	}
	// Default is user
	return ProcFilter{
		filter: func(p *Proc, err error) bool {
			return err == nil && p.Uid == os.Getuid() && p.InUserSlice()
		},
		message: "calling user processes",
	}
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

// ProcByPid implements sort.Interface for []*Proc based on Pid field
type ProcByPid []*Proc
func (s ProcByPid) Len() int { return len(s) }
func (s ProcByPid) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s ProcByPid) Less(i, j int) bool { return s[i].Pid < s[j].Pid }

// FilteredProcs returns a slice of Proc for filtered processes.
func FilteredProcs(filter Filterer) (result []*Proc) {
	files, _ := filepath.Glob("/proc/[0-9]*/stat")
	size := len(files)
	// make our channels for communicating work and results
	stats := make(chan string, size)
	procs := make(chan *Proc, size)
	// spin up workers and use a sync.WaitGroup to indicate completion
	var count = runtime.GOMAXPROCS(0)
	var wg sync.WaitGroup
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(){
			defer wg.Done()
			var p *Proc
			var err error
			for stat := range stats {
				p, err = NewProcFromStat(stat)
				if filter.Filter(p, err) {
					procs <- p
				}
			}
		}()
	}
	// start sending jobs
	wg.Add(1)
	go func() {
		defer wg.Done()
		for _, file := range files {
			data, err := ioutil.ReadFile(file)
			if err == nil {
				stats <- string(data)
			}
		}
		close(stats)
	}()
	// wait on the workers to finish and close the procs channel
	// to signal downstream that all work is done
	go func() {
		defer close(procs)
		wg.Wait()
	}()
	// collect result from procs channel
	for p := range procs {
		result = append(result, p)
	}
	// sort by Pid
	sort.Sort(ProcByPid(result))
	return
}

// ProcMapByPid implements sort.Interface for []*ProcMap based on Pid field
type ProcMapByPid []*ProcMap
func (s ProcMapByPid) Len() int { return len(s) }
func (s ProcMapByPid) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s ProcMapByPid) Less(i, j int) bool { return s[i].Pid < s[j].Pid }

// ProcMapByPgrp implements sort.Interface for []*ProcMap based on Pgrp field
type ProcMapByPgrp []*ProcMap
func (s ProcMapByPgrp) Len() int { return len(s) }
func (s ProcMapByPgrp) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s ProcMapByPgrp) Less(i, j int) bool { return s[i].Pgrp < s[j].Pgrp }

// FilteredProcMaps returns a slice of ProcMap for filtered processes.
func FilteredProcMaps(filter Filterer) (result []*ProcMap, err error) {
	var (
		count = runtime.GOMAXPROCS(0)
		wg sync.WaitGroup
	)
	// prepare channels
	stats := make(chan string, 8)
	procmaps := make(chan *ProcMap, 8)
	// spin up workers and use a sync.WaitGroup to indicate completion
	// collect result
	wg.Add(1)
	go func() {
		defer wg.Done()
		for procmap := range procmaps {
			result = append(result, procmap)
		}
	}()
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(){
			defer wg.Done()
			var (
				p *Proc
				e error
			)
			for stat := range stats {
				p, e = NewProcFromStat(stat)
				if filter.Filter(p, e) {
					procmaps <- NewProcMap(p)
				}
			}
		}()
	}
	// start sending jobs
	wg.Add(1)
	go func() {
		defer wg.Done()
		if files, e := filepath.Glob("/proc/[0-9]*/stat"); e == nil {
			for _, file := range files {
				data, e := ioutil.ReadFile(file)
				if e == nil {
					stats <- string(data)
				} else {
					log.Println(e)
				}
			}
		} else {
			log.Println(e)
		}
		close(stats)
	}()
	// wait on the workers to finish and close the result channel
	// to signal downstream that all work is done
	go func() {
		defer close(procmaps)
		wg.Wait()
	}()
	// sort by Pid
	sort.Sort(ProcMapByPid(result))
	return
}

// vim: set ft=go fdm=indent ts=2 sw=2 tw=79 noet:
