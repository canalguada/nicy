// build +linux
package cmd

import (
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

type ProcMap struct {
	*Proc
}

func NewProcMap(p *Proc) *ProcMap {
	return &ProcMap{Proc: p}
}

func (pm *ProcMap) Runtime() BaseProfile {
	return BaseProfile{
		Nice:        pm.Nice,
		Sched:       pm.Sched(),
		RTPrio:      pm.RTPrio,
		IOClass:     pm.IOClass(),
		IONice:      pm.IOPrioData,
		OomScoreAdj: pm.OomScoreAdj,
	}
}

// ProcMapByPgrp implements sort.Interface for []*ProcMap based on Pgrp field
type ProcMapByPgrp []*ProcMap

func (s ProcMapByPgrp) Len() int           { return len(s) }
func (s ProcMapByPgrp) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s ProcMapByPgrp) Less(i, j int) bool { return s[i].Pgrp < s[j].Pgrp }

func (s ProcMapByPgrp) ByPgrp() chan []*ProcMap {
	ch := make(chan []*ProcMap)
	go func(ch chan []*ProcMap) {
		byPgrp := make(map[int][]*ProcMap) // split input per Pgrp
		for _, procmap := range s {
			byPgrp[procmap.Pgrp] = append(byPgrp[procmap.Pgrp], procmap)
		}
		for _, maps := range byPgrp {
			ch <- maps
		}
		close(ch)
	}(ch)
	return ch
}

// vim: set ft=go fdm=indent ts=2 sw=2 tw=79 noet:
