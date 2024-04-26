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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
)

type PresetCache struct {
	Date       string               `yaml:"date" json:"date"`
	Cgroups    map[string][]Cgroup  `yaml:"cgroups,flow" json:"cgroups"`
	Profiles   map[string][]Profile `yaml:"profiles,flow" json:"profiles"`
	Rules      map[string][]Rule    `yaml:"rules,flow" json:"rules"`
	Origin     string               `yaml:"-" json:"-"`
	RuleFilter FilterProc           `yaml:"-" json:"-"`
}

func NewPresetCache() PresetCache {
	pc := PresetCache{
		Cgroups:  make(map[string][]Cgroup),
		Profiles: make(map[string][]Profile),
		Rules:    make(map[string][]Rule),
	}
	pc.RuleFilter = FilterProc{
		Filter: func(p *Proc, err error) bool {
			if err == nil {
				if _, err = pc.Rule(strings.Split(p.Comm, `:`)[0]); err == nil {
					return true
				}
			}
			return false
		},
		Message: "processes with a preset",
	}
	return pc
}

func (pc *PresetCache) LoadFromCache(cacheFile string) error {
	if exists(cacheFile) {
		data, err := os.ReadFile(cacheFile)
		if err == nil {
			err = yaml.Unmarshal(data, &pc)
		}
		return failed(err)
	}
	return fmt.Errorf("%w: %v", ErrNotFound, cacheFile)
}

func (pc *PresetCache) GetContent() ([]byte, error) {
	return yaml.Marshal(*pc)
}

func (pc *PresetCache) WriteCache(cacheFile string) error {
	data, err := pc.GetContent()
	if err == nil {
		err = os.WriteFile(cacheFile, data, 0600)
	}
	return err
}

func (pc *PresetCache) LoadFromConfig() (err error) {
	var cfg Config
	for _, root := range Reverse(viper.GetStringSlice("confdirs")) {
		path := filepath.Join(root, confName+"."+confType)
		if exists(path) {
			if viper.GetBool("verbose") {
				inform("cache", "reading configuration file", path+"...")
			}
			if cfg, err = NewConfig(path); err == nil {
				wg := getWaitGroup()
				wg.Add(3)
				go func() {
					defer wg.Done()
					LoadConfig(pc.Cgroups, cfg.IterCgroups)
				}()
				go func() {
					defer wg.Done()
					LoadConfig(pc.Profiles, cfg.IterProfiles)
				}()
				go func() {
					defer wg.Done()
					LoadConfig(pc.Rules, cfg.IterRules)
				}()
				wg.Wait()
				continue
			}
			nonfatal(err)
		}
	}
	pc.Origin = "config"
	pc.Date = timestamp()
	// if viper.GetBool("force") {
	//   pc.WriteCache(viper.GetString("cache"))
	// }
	return
}

func GetPresetCache() PresetCache {
	if !viper.GetBool("force") && len(presetCache.Date) > 0 {
		return presetCache
	}
	var err error
	pc := NewPresetCache()
	if !viper.GetBool("force") {
		if err = pc.LoadFromCache(viper.GetString("cache")); err == nil {
			pc.Origin = "file"
			return pc
		}
	}
	if viper.GetBool("verbose") && err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			inform("warning", err.Error())
		default:
			nonfatal(failed(err))
		}
	}
	pc.LoadFromConfig()
	return pc
}

func (pc *PresetCache) List(category string) (result []string, err error) {
	switch category {
	case "cgroup":
		result = List(pc.Cgroups)
	case "profile":
		result = List(pc.Profiles)
	case "rule":
		result = List(pc.Rules)
	default:
		err = fmt.Errorf("%w: bad category: %s", ErrInvalid, category)
	}
	return
}

func (pc *PresetCache) ListFrom(category, origin string) (result []string, err error) {
	switch category {
	case "cgroup":
		result = ListFrom(pc.Cgroups, origin)
	case "profile":
		result = ListFrom(pc.Profiles, origin)
	case "rule":
		result = ListFrom(pc.Rules, origin)
	default:
		err = fmt.Errorf("%w: bad category: %s", ErrInvalid, category)
	}
	return
}

func (pc *PresetCache) HasPreset(category string, key string) bool {
	switch category {
	case "cgroup":
		return HasPreset(pc.Cgroups, key)
	case "profile":
		return HasPreset(pc.Profiles, key)
	case "rule":
		return HasPreset(pc.Rules, key)
	}
	return false
}

func notFound(category string, key string) error {
	err := fmt.Errorf("%s not found: %s", category, key)
	if viper.GetBool("debug") && viper.GetBool("verbose") {
		inform("warning", err.Error())
	}
	return err
}

func (pc *PresetCache) Rule(key string) (Rule, error) {
	return GetPreset(pc.Rules, key, "rule")
}

func (pc *PresetCache) Profile(key string) (Profile, error) {
	return GetPreset(pc.Profiles, key, "profile")
}

func (pc *PresetCache) Cgroup(key string) (Cgroup, error) {
	return GetPreset(pc.Cgroups, key, "cgroup")
}

func (pc *PresetCache) RawRule(input *Request) (Rule, error) {
	switch input.Preset {
	case "auto", "cgroup-only":
		if rule, err := pc.Rule(input.Name); err == nil {
			return rule, nil
		} else if profile, err := pc.Profile("default"); err == nil {
			return profile.ToRule(), nil
		} else {
			return Rule{}, err
		}
	default:
		if profile, err := pc.Profile(input.Preset); err == nil {
			return profile.ToRule(), nil
		} else {
			return Rule{}, err
		}
	}
}

func (pc *PresetCache) Expand(rule *Rule) error {
	if rule.HasProfileKey() {
		profile, err := pc.Profile(rule.ProfileKey)
		if err != nil {
			return err
		}
		UpdateRule(profile.BaseProfile, rule)
		if key := profile.CgroupKey; !rule.HasCgroupKey() && key != "" {
			rule.CgroupKey = key
			cgroup, err := pc.Cgroup(key)
			if err != nil {
				return err
			}
			UpdateRule(cgroup.BaseCgroup, rule)
		}
	}
	if rule.HasCgroupKey() {
		cgroup, err := pc.Cgroup(rule.CgroupKey)
		if err != nil {
			return err
		}
		UpdateRule(cgroup.BaseCgroup, rule)
	}
	return nil
}

func (pc *PresetCache) CgroupCandidate(base BaseCgroup) (string, bool) {
	counter := make(map[string]int)
	// count matching entries for each cgroup
	// pc.Cgroups is map[string][]Cgroup
	ch := make(chan Cgroup)
	go func() {
		// do not iterate over []Cgroup and all duplicates,
		// but over first Cgroup
		for _, cgroups := range pc.Cgroups {
			ch <- ActivePreset(cgroups)
		}
		close(ch)
	}()
	preset := ToInterface(base)
	for cgroup := range ch { // iter through cgroups
		candidate := cgroup.CgroupKey
		counter[candidate] = 0 // count matching pairs
		p := ToInterface(cgroup.BaseCgroup)
		if len(p) > len(preset) { // obvious mismatch
			continue
		}
		for pk, pv := range p {
			for k, v := range preset { // compare key and value
				if pk == k && pv == v {
					counter[candidate] = counter[candidate] + 1
					break
				}
			}
		}
	}
	trace("getCgroupCandidate", "counter", counter)
	// pick up the best candidate, if any
	var best int
	for _, count := range counter {
		if count > best {
			best = count
		}
	}
	// found if preset is a superset of candidate properties
	var subkey string
	var found bool
	if best > 0 {
	Loop:
		for count := best; count > 0; count-- {
			for candidate, value := range counter {
				if value != count {
					continue
				}
				entries := ToInterface(pc.Cgroups[candidate][0].BaseCgroup)
				if len(entries) == count {
					found = true
					subkey = candidate
					trace("getCgroupCandidate", "entries", entries)
					break Loop
				}
			}
		}
		trace("getCgroupCandidate", "cgroup", subkey)
	}
	return subkey, found
}

func (pc *PresetCache) SliceProperties(rule Rule) (result []string) {
	if rule.CgroupKey != "" {
		if cgroup, err := pc.Cgroup(rule.CgroupKey); err == nil {
			result = Properties(cgroup.BaseCgroup)
		}
	}
	return
}

func (pc *PresetCache) RequestRule(input *Request) Rule {
	rule, err := pc.RawRule(input)
	if err != nil {
		nonfatal(err)
		trace("RawRule", input.Name, "empty rule")
		return Rule{}
	}
	trace("RawRule", input.Name, rule)
	fatal(pc.Expand(&rule))
	trace("Expand", input.Name, rule)
	if input.Preset == "cgroup-only" {
		rule.CgroupOnly()
	}
	if len(input.CgroupKey) > 0 {
		rule.SetCgroup(input.CgroupKey)
	}
	if input.ForceCgroup && rule.CgroupKey == "" {
		if base, ok := rule.CgroupEntries(); ok {
			if cgroup, found := pc.CgroupCandidate(base); found {
				rule.SetCgroup(cgroup) // set cgroup
			}
		}
	}
	if rule.CgroupKey != "" { // reset entries belonging to cgroup
		if cgroup, err := pc.Cgroup(rule.CgroupKey); err == nil {
			ResetMatching(&cgroup.BaseCgroup, &rule)
		}
	}
	rule.SetSliceProperties(pc.SliceProperties(rule))
	rule.SetCredentials()
	rule.RuleKey = ""
	rule.ProfileKey = ""
	rule.Origin = ""
	trace("RunnableRule", input.Name, rule)
	return rule
}

func (pc *PresetCache) RequestJob(input *Request) *ProcJob {
	return &ProcJob{
		Proc:    &Proc{},
		Request: input,
		Rule:    pc.RequestRule(input),
	}
}

func (pc *PresetCache) GenerateJobs(inputs <-chan *Request, outputs chan<- *ProcJob, wgmain *sync.WaitGroup) (err error) {
	defer func() {
		close(outputs)
		if wgmain != nil {
			wgmain.Done()
		}
	}()
	// prepare channels
	jobs := make(chan *ProcJob)
	// spin up workers
	wg := getWaitGroup() // use a sync.WaitGroup to indicate completion
	wg.Add(1)            // get commands
	go func() {
		defer wg.Done()
		for job := range jobs {
			nonfatal(job.PrepareCommands())
			outputs <- job // send result
		}
	}()
	wg.Add(1) // get entries
	go func() {
		defer wg.Done()
		for input := range inputs {
			jobs <- pc.RequestJob(input)
		}
		close(jobs)
	}()
	wg.Wait() // wait on the workers to finish
	return
}

func (pc *PresetCache) GetFilterer(scope string) ProcFilterer {
	filter := NewProcScopeFilter(scope)
	return ProcFilterer(ProcFilter{
		FilterProc: FilterProc{
			Filter: func(p *Proc, err error) bool {
				if pc.RuleFilter.Filter(p, err) {
					return filter.Filter(p, nil)
				}
				return false
			},
			Message: filter.String(),
		},
		Scope: scope,
	})
}

func (pc *PresetCache) FilteredProcs(inputs <-chan []*Proc, outputs chan<- []*Proc, wg *sync.WaitGroup) {
	defer wg.Done()
	for procs := range inputs {
		filtered := Filter(procs, func(p *Proc) bool {
			return pc.RuleFilter.Filter(p, nil)
		})
		if len(filtered) > 0 {
			outputs <- filtered
		}
	}
	close(outputs)
}

func (pc *PresetCache) DiffReview(job *ProcGroupJob) (count int, err error) {
	if len(job.Jobs) == 0 {
		return
	}
	leader := job.leader // process group leader only
	// get rule and set leader rule for reference
	leader.Rule = pc.RequestRule(leader.Request)
	running := Rule{BaseProfile: leader.Runtime()}
	// review diff
	job.Diff, count = leader.Rule.GetDiff(running)
	// check cgroup: processes are easily movable when running inside
	// session scope and some managed nicy slice
	if job.Diff.HasCgroupKey() {
		if leader.inProperSlice() { // running yet inside proper cgroup
			job.Diff.CgroupKey = ""
		} else if !(leader.inAnyNicySlice()) && !(leader.inSessionScope()) {
			job.Diff.CgroupKey = "" // remove cgroup from groupjob.Diff
		}
	} // end process group leader only
	if viper.GetBool("debug") {
		id := fmt.Sprintf("%s[%d]", leader.Proc.Comm, leader.Proc.Pgrp)
		// info := groupjob.LeaderInfo()
		for k, v := range map[string]map[string]any{
			"preset":  ToInterface(leader.Rule),
			"runtime": ToInterface(running),
			"diff":    ToInterface(job.Diff),
		} {
			inform("", fmt.Sprintf("%s %s: %v", id, k, v))
		}
	}
	return
}

func (pc *PresetCache) RawGroupJobs(inputs <-chan []*Proc, output chan<- *ProcGroupJob, wg *sync.WaitGroup) {
	defer wg.Done()
	for procs := range inputs {
		for s := range ProcByPgrp(procs).ByPgrp() {
			child := getWaitGroup()
			child.Add(1)
			go func() {
				defer child.Done()
				jobs := ProcToProcJob(s)
				if len(jobs) > 0 {
					// sort content
					sort.SliceStable(jobs, func(i, j int) bool {
						return jobs[i].Proc.Pid < jobs[j].Proc.Pid
					})
					leader := jobs[0]
					groupjob := &ProcGroupJob{ // new group job
						Pgrp:   leader.Proc.Pgrp,
						Pids:   []int{leader.Proc.Pid},
						Diff:   Rule{},
						Jobs:   []*ProcJob{leader},
						leader: leader,
					}
					for _, job := range jobs[1:] { // add content
						groupjob.Pids = append(groupjob.Pids, job.Proc.Pid)
						groupjob.Jobs = append(groupjob.Jobs, job)
					}
					if count, _ := pc.DiffReview(groupjob); count > 0 {
						output <- groupjob
					}
				}
			}()
			child.Wait()
		}
	}
	close(output)
}

func (pc *PresetCache) GenerateGroupJobs(inputs <-chan []*Proc, output chan<- *ProcGroupJob, wgmain *sync.WaitGroup) (err error) {
	defer wgmain.Done()
	// prepare channels
	groupjobs := make(chan *ProcGroupJob, 8)
	procs := make(chan []*Proc, 8)
	// spin up workers
	wg := getWaitGroup() // use a sync.WaitGroup to indicate completion
	wg.Add(1)            // prepare process group jobs adding commands
	go func() {
		defer wg.Done()
		for groupjob := range groupjobs {
			nonfatal(groupjob.PrepareAdjust())
			output <- groupjob
		}
		close(output)
	}()
	wg.Add(1) // split procs and build process group jobs
	go pc.RawGroupJobs(procs, groupjobs, &wg)
	wg.Add(1) // filter procs
	go pc.FilteredProcs(inputs, procs, &wg)
	wg.Wait() // wait on the workers to finish
	return
}

// vim: set ft=go fdm=indent ts=2 sw=2 tw=79 noet:
