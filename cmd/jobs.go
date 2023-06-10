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
	"io"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/viper"
)

var (
	reCallingUser *regexp.Regexp
	reUser        *regexp.Regexp
	reSession     *regexp.Regexp
	reManaged     *regexp.Regexp
	reSystem      *regexp.Regexp
	reAnyNicy     *regexp.Regexp
)

func init() {
	reUser = regexp.MustCompile(`user-\d+\.slice`)
	reSession = regexp.MustCompile(`user-\d+\.slice/session-\d+\.scope`)
	reManaged = regexp.MustCompile(`user@\d+\.service/.+\.slice`)
	reSystem = regexp.MustCompile(`system\.slice`)
	reAnyNicy = regexp.MustCompile(`nicy.slice/nicy-.+\.slice`)
}

type ProcJob struct {
	ProcMap
	Request  *RunInput `json:"request"`
	Result   Rule      `json:"result"`
	Commands []Command `json:"commands"`
}

func (job *ProcJob) AddCommand(c Command) {
	if !(c.IsEmpty()) {
		job.Commands = append(job.Commands, c)
	}
}

func (job *ProcJob) AddProfileCommand(property string, content []string) {
	if job.IsPrivilegeRequired(property) {
		job.SetSudoIfRequired()
		content = append([]string{"$SUDO"}, content...)
	}
	content = append(content, ">/dev/null")
	job.AddCommand(NewCommand(content...))
}

func (job ProcJob) IsPrivilegeRequired(command string) bool {
	for _, value := range job.Result.Credentials {
		if value == command {
			return true
		}
	}
	return false
}

func (job ProcJob) IsSudoSet() bool {
	return job.IsPrivilegeRequired("sudo")
}

func (job *ProcJob) SetPrivilege(command string) {
	for _, value := range job.Result.Credentials {
		if value == command {
			return
		}
	}
	job.Result.Credentials = append(job.Result.Credentials, command)
}

func (job *ProcJob) SetSudoIfRequired() {
	if !(job.IsSudoSet()) {
		job.SetPrivilege("sudo")
		trace("SetSudoIfRequired", "shell", job.Request.Shell)
		job.AddCommand(SudoCommand(job.Request.Shell))
	}
}

func (job *ProcJob) AdjustNice(pid, pgrp int) error {
	if job.Result.HasNice() {
		// c := renice(job.Result.Nice, pid, pgrp).Content()
		// if job.IsPrivilegeRequired("nice") {
		//   job.SetSudoIfRequired()
		//   c = append([]string{"$SUDO"}, c...)
		// }
		// c = append(c, ">/dev/null")
		// job.AddCommand(NewCommand(c...))
		job.AddProfileCommand("nice", renice(job.Result.Nice, pid, pgrp).Content())
		return nil
	}
	return fmt.Errorf("%w: cannot find entry: '%s'", ErrInvalid, "nice")
}

func (job *ProcJob) AdjustOomScoreAdj(pid int) error {
	if job.Result.HasOomScoreAdj() {
		// c := choom(job.Result.OomScoreAdj, pid).Content()
		// if job.IsPrivilegeRequired("oom_score_adj") {
		//   job.SetSudoIfRequired()
		//   c = append([]string{"$SUDO"}, c...)
		// }
		// c = append(c, ">/dev/null")
		// job.AddCommand(NewCommand(c...))
		job.AddProfileCommand("oom_score_adj", choom(job.Result.OomScoreAdj, pid).Content())
		return nil
	}
	return fmt.Errorf("%w: cannot find entry: '%s'", ErrInvalid, "oom_score_adj")
}

func (job *ProcJob) AdjustSchedRTPrio(pid int) error {
	policy := strings.Split(job.Request.CPUSched, `:`)[0]
	var sched string
	var rtprio int
	if job.Result.HasSched() {
		sched = job.Result.Sched
	}
	if job.Result.HasRtprio() {
		condition := (policy == "1") || (policy == "2")
		condition = (len(sched) == 0) && condition
		if (sched == "fifo") || (sched == "rr") || condition {
			rtprio = job.Result.RTPrio
		} else {
			rtprio = -1
		}
	}
	if len(sched) > 0 || rtprio != 0 {
		// c := chrt(sched, rtprio, pid).Content()
		// if job.IsPrivilegeRequired("sched") {
		//   job.SetSudoIfRequired()
		//   c = append([]string{"$SUDO"}, c...)
		// }
		// c = append(c, ">/dev/null")
		// job.AddCommand(NewCommand(c...))
		job.AddProfileCommand("sched", chrt(sched, rtprio, pid).Content())
		return nil
	}
	return fmt.Errorf("%w: cannot find entry: 'sched', 'rtprio'", ErrInvalid)
}

func (job *ProcJob) AdjustIOClassIONice(pid, pgrp int) error {
	policy := strings.Split(job.Request.IOSched, `:`)[0]
	var class string
	level := -1
	if job.Result.HasIoclass() {
		class = job.Result.IOClass
	}
	if job.Result.HasIonice() {
		condition := (policy == "1") || (policy == "2")
		condition = (len(class) == 0) && condition
		if (class == "realtime") || (class == "best-effort") || condition {
			level = job.Result.IONice
		}
	}
	if len(class) > 0 || level != -1 {
		// c := ionice(class, level, pid, pgrp).Content()
		// if job.IsPrivilegeRequired("ioclass") {
		//   job.SetSudoIfRequired()
		//   c = append([]string{"$SUDO"}, c...)
		// }
		// c = append(c, ">/dev/null")
		// job.AddCommand(NewCommand(c...))
		job.AddProfileCommand("ioclass", ionice(class, level, pid, pgrp).Content())
		return nil
	}
	return fmt.Errorf("%w: cannot find entry: 'ioclass', 'ionice'", ErrInvalid)
}

func (job *ProcJob) getEnvironment() (result []string) {
	if len(job.Result.Env) > 0 {
		for key, value := range job.Result.Env {
			result = append(result, fmt.Sprintf("%s=%s", key, value))
		}
	}
	return
}

func (job *ProcJob) sliceUnit() string {
	return fmt.Sprintf("nicy-%s.slice", job.Result.CgroupKey)
}

func (job *ProcJob) inCurrentUserSlice() bool {
	reCallingUser = regexp.MustCompile(
		`user-` + strconv.Itoa(os.Getuid()) + `\.slice`,
	)
	return reCallingUser.MatchString(job.Cgroup)
}

func (job *ProcJob) inUserSlice() bool {
	return reUser.MatchString(job.Cgroup)
}

func (job *ProcJob) inSessionScope() bool {
	return reSession.MatchString(job.Cgroup)
}

func (job *ProcJob) inManagedSlice() bool {
	return reManaged.MatchString(job.Cgroup)
}

func (job *ProcJob) inSystemSlice() bool {
	return reSystem.MatchString(job.Cgroup)
}

func (job *ProcJob) inAnyNicySlice() bool {
	return reAnyNicy.MatchString(job.Cgroup)
}

func (job *ProcJob) inProperSlice() bool {
	if job.Result.HasCgroupKey() {
		flag, _ := regexp.MatchString(job.sliceUnit(), job.Cgroup)
		return flag
	}
	return true
}

func (job *ProcJob) manager() (result string) {
	if job.Pid == 0 { // run command
		result = "${user_or_system}"
	} else if job.inUserSlice() {
		result = "--user"
	} else {
		result = "--system"
	}
	return
}

func (job *ProcJob) prefix() (result []string) {
	if job.Pid == 0 || job.inCurrentUserSlice() {
		return // run command or no privilege required
		// only root is expected to manage processes outside his slice.
	} else if job.inUserSlice() {
		// Current user MUST run the command as the slice owner.
		// Environment variables are required for busctl and systemctl commands
		if os.Getuid() != 0 {
			result = append(result, "$SUDO")
		}
		path := fmt.Sprintf("/run/user/%d", job.Uid)
		result = append(
			result,
			"runuser", "-u", job.User, "--", "env",
			fmt.Sprintf("DBUS_SESSION_BUS_ADDRESS=unix:path=%s/bus", path),
			fmt.Sprintf("XDG_RUNTIME_DIR=%s", path),
		)
	} else if job.inSystemSlice() {
		if os.Getuid() != 0 { // current user must run the command as root
			result = append(result, "$SUDO")
		}
	} else {
		// Detected processes run inside default slices, using user or system
		// manager as expected. You should not see this.
		fatal(fmt.Errorf("%w: invalid cgroup: %s", ErrInvalid, job.Cgroup))
	}
	return
}

func (job *ProcJob) convertSliceProperties() (result []string) {
	for _, property := range job.Result.SliceProperties {
		// TODO: use Reduce
		if value := strings.TrimPrefix(property, `CPUQuota=`); value != property {
			v, err := strconv.Atoi(strings.TrimSuffix(value, `%`))
			if err == nil {
				value = fmt.Sprintf("%d%%", job.Request.NumCPU*v)
			}
			result = append(result, fmt.Sprintf("CPUQuota=%s", value))
		} else {
			result = append(result, property)
		}
	}
	return
}
func (job *ProcJob) startSliceUnit() (unit string) {
	if !job.Result.HasCgroupKey() {
		return
	}
	base := []string{"systemctl", job.manager()}
	prefix := job.prefix()
	if len(prefix) > 0 {
		base = append(prefix, base...)
	}
	unit = job.sliceUnit()
	args := append(base, "start", unit, ">/dev/null") // start unit
	job.AddCommand(NewCommand(args...))
	if len(job.Result.SliceProperties) > 0 { // set properties
		args := append(base, "--runtime", "set-property", unit)
		properties := job.convertSliceProperties()
		args = append(args, properties...)
		args = append(args, ">/dev/null")
		job.AddCommand(NewCommand(args...))
	}
	return
}

func (job *ProcJob) unitPattern() string {
	unit := []string{job.Request.Name}
	// add cgroup to get unique pattern, when moving processes
	if job.Result.HasCgroupKey() {
		unit = append(unit, job.Result.CgroupKey)
	}
	unit = append(unit, `$$`)
	return strings.Join(unit, `-`)
}

func (job *ProcJob) AddExecCmd() error {
	useScope := job.Result.NeedScope() || job.Request.Managed
	trace("AddExecCmd", "useScope", useScope)
	if useScope {
		job.AddCommand(ManagerCommand(job.Request.Shell))
	}
	c := []string{"exec"}
	if useScope {
		var quiet string
		if job.Request.Quiet || (job.Request.Verbosity < 2) {
			quiet = "--quiet"
		}
		c = append(c,
			"systemd-run", "${user_or_system}", "-G", "-d",
			"--no-ask-password", quiet, "--scope",
			fmt.Sprintf("--unit=%s", job.unitPattern()),
		)
		if unit := job.startSliceUnit(); len(unit) > 0 {
			c = append(c, fmt.Sprintf("--slice=%s", unit))
		}
		for _, value := range job.Result.BaseCgroup.ScopeProperties() {
			c = append(c, "-p", value)
		}
		for _, envvar := range job.getEnvironment() { // Adjust environment
			c = append(c, "-E", envvar)
		}
	} else {
		envvars := job.getEnvironment()
		if len(envvars) > 0 { // Adjust environment
			c = append(c, "env")
			quoted := ShellQuote(envvars)
			c = append(c, quoted...)
		}
	}
	c = append(c, job.Request.Path)
	if len(job.Result.CmdArgs) > 0 {
		c = append(c, job.Result.CmdArgs...)
	}
	trace("AddExecCmd", "command content", c)
	job.AddCommand(NewCommand(c...))
	return nil
}

func (job *ProcJob) PrepareCommands() error {
	job.AdjustNice(0, 0) // No pid nor pgrp
	job.AdjustSchedRTPrio(0)
	job.AdjustIOClassIONice(0, 0)
	job.AdjustOomScoreAdj(0)
	job.AddExecCmd()
	return nil
}

func (job *ProcJob) Show() (result []string, err error) {
	c := job.Commands[len(job.Commands)-1].Content()
	c = append(c, `"$@"`)
	job.Commands[len(job.Commands)-1] = NewCommand(c...)
	result = Map(job.Commands, func(c Command) string { return c.ShellCmd() })
	return
}

func (job *ProcJob) RuntimeCommands(args ...string) []Command {
	return Reduce(([]Command)(job.Commands), []Command{}, func(s []Command, c Command) []Command {
		if !c.skipRuntime { // Remove tests and shebang line from loop
			// Append command args, if any, when required
			if token, _ := c.Index(0); token == "exec" && len(args) > 0 {
				c = NewCommand(append(c.Content(), args...)...)
			}
			s = append(s, c)
		}
		return s
	})
}

func (job *ProcJob) Run(tag string, args []string, stdin io.Reader, stdout, stderr io.Writer) (err error) {
	commands := job.RuntimeCommands(args...)
	// setup execution environment
	uid := os.Getuid()
	pid := os.Getpid()
	nonfatal(updatePrivileges(true))
	var unprivileged []Command
	for _, c := range commands {
		// run only lines that require some capabilities
		_, flag := c.RequireSysCapability(job.Uid)
		if !(flag) {
			if !(c.IsEmpty()) {
				unprivileged = append(unprivileged, c)
			}
			continue
		}
		cToRun := c.Runtime(pid, uid)
		err = cToRun.StartWait(tag, stdin, stdout, stderr)
		if err != nil {
			debug(getCapabilities())
			return
		}
	}
	nonfatal(updatePrivileges(false, true)) // clear all
	// finally adjust oom_score, prepare systemd slice and exec
	for _, c := range unprivileged {
		cToRun := c.Runtime(pid, uid)
		if token, _ := cToRun.Index(0); token == "exec" {
			return cToRun.Exec(tag)
		} else {
			err = cToRun.StartWait(tag, stdin, stdout, stderr)
			if err != nil {
				return
			}
		}
	}
	return
}

type ProcGroupJob struct {
	Pgrp     int        `json:"pgrp"`
	Pids     []int      `json:"pids"`
	Diff     Rule       `json:"diff"`
	Commands []Command  `json:"commands"`
	Procs    []*ProcJob `json:"procs"`
	leader   *ProcJob   `json:"-"`
}

func (job *ProcGroupJob) adjustProperties() error {
	// first, using first ProcJob and Pgrp
	if job.Diff.HasNice() {
		job.leader.AdjustNice(0, job.Pgrp)
	}
	if job.Diff.HasIoclass() || job.Diff.HasIonice() {
		job.leader.AdjustIOClassIONice(0, job.Pgrp)
	}
	// second, using each ProcJob and its Pid
	if job.Diff.HasSched() || job.Diff.HasRtprio() {
		for i, _ := range job.Procs {
			job.Procs[i].AdjustSchedRTPrio(job.Procs[i].Pid)
		}
	}
	if job.Diff.HasOomScoreAdj() {
		for i, _ := range job.Procs {
			job.Procs[i].AdjustOomScoreAdj(job.Procs[i].Pid)
		}
	}
	return nil
}

func (job *ProcGroupJob) scopeUnit() string {
	tokens := []string{job.leader.Request.Name}
	// add cgroup to get unique pattern, when moving processes
	if job.leader.Result.HasCgroupKey() {
		tokens = append(tokens, job.leader.Result.CgroupKey)
	}
	tokens = append(tokens, fmt.Sprintf("%d", job.leader.Pid))
	return fmt.Sprintf("%s.scope", strings.Join(tokens, `-`))
}

func (job *ProcGroupJob) moveProcGroup() error {
	count := 1
	useSlice := job.Diff.HasCgroupKey()
	if useSlice {
		count = 2
	}
	c := []string{
		"busctl", "call", "--quiet", job.leader.manager(),
		"org.freedesktop.systemd1", "/org/freedesktop/systemd1",
		"org.freedesktop.systemd1.Manager", "StartTransientUnit",
		"ssa(sv)a(sa(sv))", job.scopeUnit(), "fail", strconv.Itoa(count),
		"PIDs", "au", strconv.Itoa(len(job.Pids)),
	}
	prefix := job.leader.prefix()
	if len(prefix) > 0 {
		c = append(prefix, c...)
	}
	for _, pid := range job.Pids {
		c = append(c, strconv.Itoa(pid))
	}
	if useSlice {
		c = append(c, "Slice", "s", job.leader.sliceUnit())
	}
	c = append(c, "0")
	job.leader.AddCommand(NewCommand(c...))
	return nil
}

func (job *ProcGroupJob) adjustNewScopeProperties() error {
	prefix := job.leader.prefix()
	c := []string{
		"systemctl", job.leader.manager(), "--runtime",
		"set-property", job.scopeUnit(),
	}
	if len(prefix) > 0 {
		c = append(prefix, c...)
	}
	for _, value := range job.Diff.ScopeProperties() {
		c = append(c, value)
	}
	job.leader.AddCommand(NewCommand(c...))
	return nil
}

// Adjust each property with its own command.
// Set the properties of the leaf service and don't deal with inner nodes.
// If the cgroup controller for one property is not available, another property
// may still be set, if not requiring it.
func (job *ProcGroupJob) adjustUnitProperties(properties []string) error {
	prefix := job.leader.prefix()
	c := []string{
		"systemctl", job.leader.manager(), "--runtime",
		"set-property", job.leader.Unit,
	}
	if len(prefix) > 0 {
		c = append(prefix, c...)
	}
	// TODO: changing multiple properties at the same time
	// which is preferable over setting them individually
	// TODO: check if the systemctl command is still bugged
	c = append(c, properties...)
	job.leader.AddCommand(NewCommand(c...))
	return nil
}

func (job *ProcGroupJob) PrepareAdjust() error {
	if len(job.Procs) == 0 {
		return nil
	}
	job.adjustProperties()
	// use process group leader to check cgroup
	cgroup := job.leader.Cgroup
	if reSession.MatchString(cgroup) {
		// set processes running in some session scope
		// check if require moving processes into own scope
		if job.Diff.NeedScope() {
			// also require slice
			if job.Diff.HasCgroupKey() {
				job.leader.startSliceUnit()
			}
			// start scope unit, moving all processes sharing same process group
			job.moveProcGroup()
			if job.Diff.HasScopeProperties() {
				job.adjustNewScopeProperties()
			}
		}
	} else if reManaged.MatchString(cgroup) || reSystem.MatchString(cgroup) {
		// set processes yet controlled by systemd manager in user or system slice
		var properties []string
		if job.Diff.HasCgroupKey() {
			// move if running yet inside other nicy slice
			if reAnyNicy.MatchString(cgroup) {
				job.leader.startSliceUnit()
				job.moveProcGroup()
			} else if len(job.leader.Result.SliceProperties) > 0 {
				properties = job.leader.convertSliceProperties()
			}
		}
		if job.Diff.HasScopeProperties() {
			// adjust relevant properties
			properties = append(properties, job.Diff.ScopeProperties()...)
		}
		if len(properties) > 0 {
			job.adjustUnitProperties(properties)
		}
	}
	// collect commands
	job.Commands = Reduce(
		job.Procs,
		job.Commands,
		func(commands []Command, procjob *ProcJob) []Command {
			return append(commands, procjob.Commands...)
		},
	)
	return nil
}

func (job *ProcGroupJob) Run(tag string, stdout, stderr io.Writer) error {
	if len(job.Procs) == 0 {
		return nil
	}
	// Show job details
	id := fmt.Sprintf("%s[%d]", job.leader.Comm, job.Pgrp)
	if viper.GetBool("verbose") || viper.GetBool("dry-run") {
		inform(tag, fmt.Sprintf(
			"%s: cgroup:%s pids:%v",
			id, job.leader.Unit, job.Pids,
		))
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
		_, flag := c.RequireSysCapability(job.leader.Uid)
		if !(flag) { // run only lines that require some capabilities
			if !(c.IsEmpty()) {
				unprivileged = append(unprivileged, c)
			}
			continue
		}
		if err := c.StartWait(id, nil, stdout, stderr); err != nil {
			warn(err)
			break // exit loop on error
		}
	}
	nonfatal(updatePrivileges(false)) // reset ambient capabilities
	for _, c := range unprivileged {  // don't require any capability
		if err := c.StartWait(id, nil, stdout, stderr); err != nil {
			warn(err)
			break // exit loop on error
		}
	}
	return nil
}

func ProcMapToProcJob(procmaps []*ProcMap) (result []*ProcJob) {
	limit := int(rlimitNice().Max)
	// sort first by Pgrp
	sort.Sort(ProcMapByPgrp(procmaps))
	for _, procmap := range procmaps {
		name := strings.Split(procmap.Comm, `:`)[0]
		job := &ProcJob{
			ProcMap: *procmap,
			Request: &RunInput{
				Name:     name,
				Path:     fmt.Sprintf("%%%s%%", name),
				Preset:   "auto",
				Quiet:    true,
				Shell:    "/bin/sh",
				NumCPU:   numCPU,
				MaxNice:  limit,
				CPUSched: "0:other:0",
				IOSched:  "0:none:0",
			},
		}
		result = append(result, job)
	}
	return
}

func (job *ProcGroupJob) Add(p *ProcJob) (err error) {
	if p.Pgrp == job.Pgrp {
		job.Pids = append(job.Pids, p.Pid)
		job.Procs = append(job.Procs, p)
		if len(job.Procs) == 1 {
			job.leader = job.Procs[0]
		}
	} else {
		err = fmt.Errorf("%w: wrong pgrp: %d", ErrInvalid, p.Pgrp)
		warn(err)
	}
	return
}

func GroupProcJobs(jobs []*ProcJob) (result *ProcGroupJob, remainder []*ProcJob) {
	if len(jobs) == 0 {
		return
	}
	// sort content
	sort.SliceStable(jobs, func(i, j int) bool {
		return jobs[i].Pid < jobs[j].Pid
	})
	leader := jobs[0]
	result = &ProcGroupJob{ // new group job
		Pgrp:   leader.Pgrp,
		Pids:   []int{leader.Pid},
		Diff:   Rule{},
		Procs:  []*ProcJob{leader},
		leader: leader,
	}
	for _, job := range jobs[1:] { // add content
		if job.Pgrp == result.Pgrp {
			result.Pids = append(result.Pids, job.Pid)
			result.Procs = append(result.Procs, job)
		} else {
			remainder = append(remainder, job)
		}
	}
	return
}

func (job *ProcGroupJob) LeaderInfo() (result string) {
	if len(job.Procs) > 0 {
		result = fmt.Sprintf(
			"Group pgrp %d leader (%s)[%d]",
			job.Pgrp, job.Procs[0].Comm, job.Procs[0].Pid,
		)
	}
	return
}

func ReviewGroupJobDiff(groupjob *ProcGroupJob) (count int, err error) {
	if len(groupjob.Procs) == 0 {
		return
	}
	leader := groupjob.leader // process group leader only
	input := leader.Request
	// get rule and set leader rule for reference
	leader.Result = presetCache.GetRunInputRule(input)
	running := Rule{
		BaseProfile: leader.Runtime,
	}
	// review diff
	groupjob.Diff, count = leader.Result.GetDiff(running)
	// check cgroup: processes are easily movable when running inside
	// session scope and some managed nicy slice
	if groupjob.Diff.HasCgroupKey() {
		if leader.inProperSlice() { // running yet inside proper cgroup
			groupjob.Diff.CgroupKey = ""
		} else if !(leader.inAnyNicySlice()) && !(leader.inSessionScope()) {
			groupjob.Diff.CgroupKey = "" // remove cgroup from groupjob.Diff
		}
	} // end process group leader only
	if viper.GetBool("debug") {
		id := fmt.Sprintf("%s[%d]", leader.Comm, leader.Pgrp)
		// info := groupjob.LeaderInfo()
		for k, v := range map[string]map[string]any{
			"preset":  ToInterface(leader.Result),
			"runtime": ToInterface(running),
			"diff":    ToInterface(groupjob.Diff),
		} {
			inform("", fmt.Sprintf("%s %s: %v", id, k, v))
		}
	}
	return
}

// vim: set ft=go fdm=indent ts=2 sw=2 tw=79 noet:
