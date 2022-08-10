// build +linux
package cmd

import (
	"log"
	"encoding/json"
	"sort"
	"path/filepath"
	"io/ioutil"
	"sync"
	"runtime"
	"github.com/canalguada/goprocfs"
)

type (
	Proc = goprocfs.Proc
	Filterer = goprocfs.Filterer
)

var (
	GetCalling = goprocfs.GetCalling
	NewProcFromStat = goprocfs.NewProcFromStat
	FilteredProcs = goprocfs.FilteredProcs
	GetFilterer = goprocfs.GetFilterer
	GetFormatter = goprocfs.GetFormatter
)

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
		User: p.Username(),
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
