// build +linux
package cmd

import (
	"io/ioutil"
	"log"
	"path/filepath"
	"sort"
	"strings"

	"github.com/canalguada/procfs"
)

type (
	Proc         = procfs.Proc
	ProcFilterer = procfs.Filterer[Proc]
	ProcFilter   = procfs.ProcFilter
	FilterProc   = procfs.FilterAny[Proc]
)

var (
	GetCalling           = procfs.GetCalling
	NewProcFromStat      = procfs.NewProcFromStat
	FilteredProcs        = procfs.FilteredProcs
	GetScopeOnlyFilterer = procfs.GetFilterer
	NewProcScopeFilter   = procfs.NewProcScopeFilter
	GetFormatter         = procfs.GetFormatter
)

var ruleFilter = FilterProc{
	Filter: func(p *Proc, err error) bool {
		if err == nil {
			if _, err = presetCache.Rule(strings.Split(p.Comm, `:`)[0]); err == nil {
				return true
			}
		}
		return false
	},
	Message: "processes with a preset",
}

func GetFilterer(scope string) ProcFilterer {
	if presetCache.Date == "" {
		presetCache = GetPresetCache()
	}
	filter := NewProcScopeFilter(scope)
	return ProcFilterer(ProcFilter{
		FilterProc: FilterProc{
			Filter: func(p *Proc, err error) bool {
				if ruleFilter.Filter(p, err) {
					return filter.Filter(p, nil)
				}
				return false
			},
			Message: filter.String(),
		},
		Scope: scope,
	})
}

type ProcMap struct {
	Pid        int         `json:"pid"`
	Ppid       int         `json:"ppid"`
	Pgrp       int         `json:"pgrp"`
	Uid        int         `json:"uid"`
	User       string      `json:"user"`
	State      string      `json:"state"`
	Slice      string      `json:"slice"`
	Unit       string      `json:"unit"`
	Comm       string      `json:"comm"`
	Cgroup     string      `json:"cgroup"`
	Priority   int         `json:"priority"`
	NumThreads int         `json:"num_threads"`
	Runtime    BaseProfile `json:"runtime"`
}

func NewProcMap(p *Proc) *ProcMap {
	pm := &ProcMap{
		Pid:        p.Pid,
		Ppid:       p.Ppid,
		Pgrp:       p.Pgrp,
		Uid:        p.Uid,
		User:       p.Username(),
		State:      p.State,
		Slice:      p.Cgroup[1],
		Unit:       p.Cgroup[2],
		Comm:       p.Comm,
		Cgroup:     p.Cgroup[0],
		Priority:   p.Priority,
		NumThreads: p.NumThreads,
		Runtime: BaseProfile{
			Nice:        p.Nice,
			Sched:       p.Sched(),
			RTPrio:      p.RTPrio,
			IOClass:     p.IOClass(),
			IONice:      p.IOPrioData,
			OomScoreAdj: p.OomScoreAdj,
		},
	}
	return pm
}

// ProcMapByPid implements sort.Interface for []*ProcMap based on Pid field
type ProcMapByPid []*ProcMap

func (s ProcMapByPid) Len() int           { return len(s) }
func (s ProcMapByPid) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s ProcMapByPid) Less(i, j int) bool { return s[i].Pid < s[j].Pid }

// ProcMapByPgrp implements sort.Interface for []*ProcMap based on Pgrp field
type ProcMapByPgrp []*ProcMap

func (s ProcMapByPgrp) Len() int           { return len(s) }
func (s ProcMapByPgrp) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s ProcMapByPgrp) Less(i, j int) bool { return s[i].Pgrp < s[j].Pgrp }

// FilteredProcMaps returns a slice of ProcMap for filtered processes.
func FilteredProcMaps(filter ProcFilterer) (result []*ProcMap, err error) {
	wg := getWaitGroup()
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
	for i := 0; i < goMaxProcs; i++ {
		wg.Add(1)
		go func() {
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
