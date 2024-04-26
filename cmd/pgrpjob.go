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
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/viper"
)

type ProcGroupJob struct {
	Pgrp     int        `json:"pgrp"`
	Pids     []int      `json:"pids"`
	Diff     Rule       `json:"diff"`
	Commands []Command  `json:"commands"`
	Jobs     []*ProcJob `json:"jobs"`
	leader   *ProcJob   `json:"-"`
}

func (job *ProcGroupJob) adjustProperties() error {
	j := job.leader
	r := job.Diff
	// using leader ProcJob and its Pgrp where supported
	j.AdjustNice(-job.Pgrp)
	if r.HasIoclass() || r.HasIonice() {
		j.AdjustIOClassIONice(-job.Pgrp)
	}
	// using each ProcJob and its own Pid
	if r.HasSched() || r.HasRtprio() {
		for _, procjob := range job.Jobs {
			procjob.AdjustSchedRTPrio(procjob.Proc.Pid)
		}
	}
	if r.HasOomScoreAdj() {
		for _, procjob := range job.Jobs {
			procjob.AdjustOomScoreAdj(procjob.Proc.Pid)
		}
	}
	return nil
}

func (job *ProcGroupJob) scopeUnit() string {
	j := job.leader
	r := job.leader.Rule
	tokens := []string{j.Request.Name}
	// add cgroup to get unique pattern, when moving processes
	if r.HasCgroupKey() {
		tokens = append(tokens, r.CgroupKey)
	}
	tokens = append(tokens, fmt.Sprintf("%d", j.Proc.Pid))
	return fmt.Sprintf("%s.scope", strings.Join(tokens, `-`))
}

func (job *ProcGroupJob) moveProcGroup() error {
	j := job.leader
	count := 1
	useSlice := job.Diff.HasCgroupKey()
	if useSlice {
		count = 2
	}
	c := []string{
		"busctl", "call", "--quiet", j.manager(),
		"org.freedesktop.systemd1", "/org/freedesktop/systemd1",
		"org.freedesktop.systemd1.Manager", "StartTransientUnit",
		"ssa(sv)a(sa(sv))", job.scopeUnit(), "fail", strconv.Itoa(count),
		"PIDs", "au", strconv.Itoa(len(job.Pids)),
	}
	prefix := j.prefix()
	if len(prefix) > 0 {
		c = append(prefix, c...)
	}
	for _, pid := range job.Pids {
		c = append(c, strconv.Itoa(pid))
	}
	if useSlice {
		c = append(c, "Slice", "s", j.sliceUnit())
	}
	c = append(c, "0")
	j.AddCommand(NewCommand(c...))
	return nil
}

func (job *ProcGroupJob) adjustNewScopeProperties() error {
	j := job.leader
	prefix := j.prefix()
	c := []string{
		"systemctl", j.manager(), "--runtime",
		"set-property", job.scopeUnit(),
	}
	if len(prefix) > 0 {
		c = append(prefix, c...)
	}
	for _, value := range job.Diff.ScopeProperties() {
		c = append(c, value)
	}
	j.AddCommand(NewCommand(c...))
	return nil
}

// Adjust each property with its own command.
// Set the properties of the leaf service and don't deal with inner nodes.
// If the cgroup controller for one property is not available, another property
// may still be set, if not requiring it.
func (job *ProcGroupJob) adjustUnitProperties(properties []string) error {
	j := job.leader
	prefix := j.prefix()
	c := []string{
		"systemctl", j.manager(), "--runtime", "set-property", j.Proc.Unit,
	}
	if len(prefix) > 0 {
		c = append(prefix, c...)
	}
	// TODO: changing multiple properties at the same time
	// which is preferable over setting them individually
	// TODO: check if the systemctl command is still bugged
	c = append(c, properties...)
	j.AddCommand(NewCommand(c...))
	return nil
}

func (job *ProcGroupJob) PrepareAdjust() error {
	j := job.leader
	r := job.Diff
	if len(job.Jobs) == 0 {
		return nil
	}
	job.adjustProperties()
	// use process group leader to check cgroup
	if j.inSessionScope() {
		// set processes running in some session scope
		// check if require moving processes into own scope
		if r.NeedScope() {
			// also require slice
			if r.HasCgroupKey() {
				j.startSliceUnit()
			}
			// start scope unit, moving all processes sharing same process group
			job.moveProcGroup()
			if r.HasScopeProperties() {
				job.adjustNewScopeProperties()
			}
		}
	} else if j.inManagedSlice() || j.inSystemSlice() {
		// set processes yet controlled by systemd manager in user or system slice
		var properties []string
		if r.HasCgroupKey() {
			// move if running yet inside other nicy slice
			if j.inAnyNicySlice() {
				j.startSliceUnit()
				job.moveProcGroup()
			} else if len(j.Rule.SliceProperties) > 0 {
				properties = j.convertSliceProperties()
			}
		}
		if r.HasScopeProperties() {
			// adjust relevant properties
			properties = append(properties, r.ScopeProperties()...)
		}
		if len(properties) > 0 {
			job.adjustUnitProperties(properties)
		}
	}
	// collect commands
	job.Commands = Reduce(
		job.Jobs,
		job.Commands,
		func(commands []Command, j *ProcJob) []Command {
			return append(commands, j.Commands...)
		},
	)
	return nil
}

func (job *ProcGroupJob) Run(tag string, std *Streams) error {
	if len(job.Jobs) == 0 {
		return nil
	}
	j := job.leader
	// Show job details
	id := fmt.Sprintf("%s[%d]", j.Proc.Comm, job.Pgrp)
	if viper.GetBool("verbose") || viper.GetBool("dry-run") {
		inform(tag, fmt.Sprintf("%s: cgroup:%s pids:%v", id, j.Proc.Unit, job.Pids))
	}
	if viper.GetBool("verbose") && viper.GetBool("debug") {
		inform(tag, fmt.Sprintf("%s: diff: %v", id, job.Diff))
		inform("", fmt.Sprintf("%s: commands: %v", id, job.Commands))
	}
	// Finally run commands
	nonfatal(updatePrivileges(true))
	var unprivileged []Command
	for _, c := range job.Commands {
		if c.skipRuntime {
			continue
		}
		if !(c.RequireSysCapability()) { // run only lines that require some capabilities
			if !(c.IsEmpty()) {
				unprivileged = append(unprivileged, c)
			}
			continue
		}
		if err := c.StartWait(id, std); err != nil {
			warn(err)
			break // exit loop on error
		}
	}
	nonfatal(updatePrivileges(false)) // reset ambient capabilities
	for _, c := range unprivileged {  // don't require any capability
		if err := c.StartWait(id, std); err != nil {
			warn(err)
			break // exit loop on error
		}
	}
	return nil
}

func ProcToProcJob(procs []*Proc) []*ProcJob {
	// sort first by Pgrp
	sort.Sort(ProcByPgrp(procs))
	return Map(procs, NewProcJob)
}

func (job *ProcGroupJob) Add(p *ProcJob) (err error) {
	if p.Proc.Pgrp == job.Pgrp {
		job.Pids = append(job.Pids, p.Proc.Pid)
		job.Jobs = append(job.Jobs, p)
		if len(job.Jobs) == 1 {
			job.leader = job.Jobs[0]
		}
	} else {
		err = fmt.Errorf("%w: wrong pgrp: %d", ErrInvalid, p.Proc.Pgrp)
		warn(err)
	}
	return
}

func (job *ProcGroupJob) LeaderInfo() (result string) {
	if len(job.Jobs) > 0 {
		result = fmt.Sprintf(
			"Group pgrp %d leader (%s)[%d]",
			job.Pgrp, job.Jobs[0].Proc.Comm, job.Jobs[0].Proc.Pid,
		)
	}
	return
}

// vim: set ft=go fdm=indent ts=2 sw=2 tw=79 noet:
