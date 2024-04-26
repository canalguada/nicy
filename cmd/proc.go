// build +linux
package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unsafe"

	"golang.org/x/sys/unix"
)

// Scheduling policies

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

var CPUSched map[int]string

func init() {
	CPUSched = make(map[int]string)
	for k, v := range CPU.Class {
		CPUSched[k] = strings.ToLower(strings.TrimPrefix(v, `SCHED_`))
	}
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

// From /proc filesystem

// See the following discussions:
//
// - https://github.com/prometheus/node_exporter/issues/52
// - https://github.com/prometheus/procfs/pull/2
// - http://stackoverflow.com/questions/17410841/how-does-user-hz-solve-the-jiffy-scaling-issue
const userHZ = 100

func GetResource(pid int, rc string) ([]byte, error) {
	return os.ReadFile(fmt.Sprintf("/proc/%d/%s", pid, rc))
}

// % cat /proc/$(pidof nvim)/stat
// 14066 (nvim) S 14064 14063 14063 0 -1 4194304 5898 6028 495 394 487 64 88 68 39 19 1 0 1256778 18685952 2655 4294967295 4620288 7319624 3219630688 0 0 0 0 2 536891909 1 0 0 17 0 0 0 0 0 0 8366744 8490776 38150144 3219638342 3219638506 3219638506 3219644398 0

type ProcStat struct {
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
	stat                string `json:"-"`
}

func (stat *ProcStat) UnmarshalText(buffer []byte) (err error) {
	stat.stat = strings.TrimSpace(string(buffer))
	rv := reflect.ValueOf(stat).Elem()
	fields := strings.Split(stat.stat, " ")
	for i := 0; i < (rv.NumField() - 1); i++ {
		switch f := rv.Field(i); f.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			v, err := strconv.ParseInt(fields[i], 0, f.Type().Bits())
			if err != nil {
				return err
			}
			f.SetInt(v)
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			v, err := strconv.ParseUint(fields[i], 0, f.Type().Bits())
			if err != nil {
				return err
			}
			f.SetUint(v)
		case reflect.String:
			f.SetString(fields[i])
		}
	}
	stat.Comm = strings.Trim(stat.Comm, "()")
	return nil
}

func (stat *ProcStat) Read(pid int) error {
	data, err := GetResource(pid, "stat") // read stat data for pid
	if err == nil {                       // load
		return stat.UnmarshalText(bytes.TrimSpace(data))
	}
	return err
}

func (stat *ProcStat) GoString() string {
	return "ProcStat" + stat.String()
}

func (stat *ProcStat) String() string {
	var s []string
	base := reflect.ValueOf(stat).Elem()
	typeOfBase := base.Type()
	for i := 0; i < (base.NumField() - 1); i++ {
		s = append(
			s,
			fmt.Sprintf(
				"%s: %v",
				typeOfBase.Field(i).Name,
				base.Field(i).Interface(),
			),
		)
	}
	return "{" + strings.Join(s, ", ") + "}"
}

// CPUTime returns the total CPU user and system time in seconds.
func (stat *ProcStat) CPUTime() float64 {
	return float64(stat.UTime+stat.STime) / userHZ
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
		if parts := strings.Split(strings.TrimPrefix(cgroup, "0::/"), `/`); strings.HasPrefix(cgroup, "0::/") && len(parts) > 0 {
			p.Cgroup, p.Slice, p.Unit = cgroup, parts[0], parts[len(parts)-1]
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

func NewProcFromStat(stat []byte) (p *Proc, err error) {
	p = new(Proc)
	// Stat
	err = p.ProcStat.UnmarshalText(stat)
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
			p.Priority,
			p.Nice,
			p.NumThreads,
			p.RTPrio,
			p.Policy,
			p.OomScoreAdj,
			p.IOPrioClass,
			p.IOPrioData,
			p.Comm,
			p.Cgroup,
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

func (p *Proc) Runtime() BaseProfile {
	return BaseProfile{
		Nice:        p.Nice,
		Sched:       p.Sched(),
		RTPrio:      p.RTPrio,
		IOClass:     p.IOClass(),
		IONice:      p.IOPrioData,
		OomScoreAdj: p.OomScoreAdj,
	}
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
	stats := make(chan []byte, len(files))
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
				stats <- bytes.TrimSpace(data)
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

type Filterer[P Proc | ProcStat] interface {
	Filter(p *P, err error) bool
	String() string
}

type ProcFilterer = Filterer[Proc]

type FilterAny[T any] struct {
	Filter  func(p *T, err error) bool
	Message string
}
type FilterProc = FilterAny[Proc]

type ProcFilter struct {
	FilterProc
	Scope string
}

func (pf ProcFilter) Filter(p *Proc, err error) bool {
	return pf.FilterProc.Filter(p, err)
}

func (pf ProcFilter) String() string {
	return pf.Message
}

func NewProcScopeFilter(scope string) Filterer[Proc] {
	var inner FilterAny[Proc]
	switch strings.ToLower(scope) {
	case "global":
		inner = FilterAny[Proc]{
			Filter: func(p *Proc, err error) bool {
				return err == nil && p.InUserSlice()
			},
			Message: "processes inside any user slice",
		}
	case "system":
		inner = FilterAny[Proc]{
			Filter: func(p *Proc, err error) bool {
				return err == nil && p.InSystemSlice()
			},
			Message: "processes inside system slice",
		}
	case "all":
		inner = FilterAny[Proc]{
			Filter: func(p *Proc, err error) bool {
				return err == nil
			},
			Message: "all processes",
		}
	case "user":
		inner = FilterAny[Proc]{
			Filter: func(p *Proc, err error) bool {
				return err == nil && p.Uid == os.Getuid() && p.InUserSlice()
			},
			Message: "calling user processes",
		}
	}
	return Filterer[Proc](ProcFilter{FilterProc: inner, Scope: scope})
}

func GetFilterer(scope string) Filterer[Proc] {
	scope = strings.ToLower(scope)
	switch scope {
	case "global", "system", "all":
		return NewProcScopeFilter(scope)
	default:
		return NewProcScopeFilter("user")
	}
}

var GetScopeOnlyFilterer = GetFilterer

// ProcByPgrp implements sort.Interface for []*Proc based on Pgrp field
type ProcByPgrp []*Proc

func (s ProcByPgrp) Len() int           { return len(s) }
func (s ProcByPgrp) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s ProcByPgrp) Less(i, j int) bool { return s[i].Pgrp < s[j].Pgrp }

func (s ProcByPgrp) ByPgrp() chan []*Proc {
	ch := make(chan []*Proc)
	go func(ch chan []*Proc) {
		byPgrp := make(map[int][]*Proc) // split input per Pgrp
		for _, p := range s {
			byPgrp[p.Pgrp] = append(byPgrp[p.Pgrp], p)
		}
		for _, procs := range byPgrp {
			ch <- procs
		}
		close(ch)
	}(ch)
	return ch
}

// vim: set ft=go fdm=indent ts=2 sw=2 tw=79 noet:
