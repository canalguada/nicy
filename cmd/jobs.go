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
	"path/filepath"
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
	*ProcMap
	*Request `json:"request"`
	Result   Rule      `json:"result"`
	Commands []Command `json:"commands"`
}

func NewMapJob(m *ProcMap) *ProcJob {
	name := strings.Split(m.Comm, `:`)[0]
	r := NewRawRequest(name, fmt.Sprintf("%%%s%%", name), "/bin/sh")
	r.Quiet = true
	return &ProcJob{ProcMap: m, Request: r}
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
		job.AddProfileCommand("nice", renice(job.Result.Nice, pid, pgrp).Content())
	}
	return nil
}

func (job *ProcJob) AdjustOomScoreAdj(pid int) error {
	if job.Result.HasOomScoreAdj() {
		job.AddProfileCommand("oom_score_adj", choom(job.Result.OomScoreAdj, pid).Content())
	}
	return nil
}

func (job *ProcJob) AdjustSchedRTPrio(pid int) error {
	var sched string
	var rtprio int
	if job.Result.HasSched() {
		sched = job.Result.Sched
	}
	if job.Result.HasRtprio() {
		if policy := job.Request.Proc.Policy; (sched == "fifo") ||
			(sched == "rr") || (len(sched) == 0 && (policy == 1 || policy == 2)) {
			rtprio = job.Result.RTPrio
		} else {
			rtprio = -1
		}
	}
	if len(sched) > 0 || rtprio != 0 {
		job.AddProfileCommand("sched", chrt(sched, rtprio, pid).Content())
	}
	return nil
}

func (job *ProcJob) AdjustIOClassIONice(pid, pgrp int) error {
	var class string
	level := -1
	if job.Result.HasIoclass() {
		class = job.Result.IOClass
	}
	if job.Result.HasIonice() {
		if policy := job.Request.Proc.IOPrioClass; (class == "realtime") ||
			(class == "best-effort") || (len(class) == 0 && (policy == 1 || policy == 2)) {
			level = job.Result.IONice
		}
	}
	if len(class) > 0 || level != -1 {
		job.AddProfileCommand("ioclass", ionice(class, level, pid, pgrp).Content())
	}
	return nil
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
	return reCallingUser.MatchString(job.ProcMap.Cgroup)
}

func (job *ProcJob) inUserSlice() bool {
	return reUser.MatchString(job.ProcMap.Cgroup)
}

func (job *ProcJob) inSessionScope() bool {
	return reSession.MatchString(job.ProcMap.Cgroup)
}

func (job *ProcJob) inManagedSlice() bool {
	return reManaged.MatchString(job.ProcMap.Cgroup)
}

func (job *ProcJob) inSystemSlice() bool {
	return reSystem.MatchString(job.ProcMap.Cgroup)
}

func (job *ProcJob) inAnyNicySlice() bool {
	return reAnyNicy.MatchString(job.ProcMap.Cgroup)
}

func (job *ProcJob) inProperSlice() bool {
	if job.Result.HasCgroupKey() {
		flag, _ := regexp.MatchString(job.sliceUnit(), job.ProcMap.Cgroup)
		return flag
	}
	return true
}

func (job *ProcJob) manager() (result string) {
	if job.ProcMap.Pid == 0 { // run command
		result = "--${manager}"
	} else if job.inUserSlice() {
		result = "--user"
	} else {
		result = "--system"
	}
	return
}

func (job *ProcJob) prefix() (result []string) {
	if job.ProcMap.Pid == 0 || job.inCurrentUserSlice() {
		return // run command or no privilege required
		// only root is expected to manage processes outside his slice.
	} else if job.inUserSlice() {
		// Current user MUST run the command as the slice owner.
		// Environment variables are required for busctl and systemctl commands
		if os.Getuid() != 0 {
			result = append(result, "$SUDO")
		}
		path := fmt.Sprintf("/run/user/%d", job.ProcMap.Uid)
		result = append(
			result,
			"runuser", "-u", job.ProcMap.Username(), "--", "env",
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
		fatal(fmt.Errorf("%w: invalid cgroup: %s", ErrInvalid, job.ProcMap.Cgroup))
	}
	return
}

func (job *ProcJob) convertSliceProperties() (result []string) {
	var runtimeCPUQuota = func(value string) string {
		if v, err := strconv.Atoi(strings.TrimSuffix(value, `%`)); err == nil {
			value = fmt.Sprintf("%d%%", job.Request.NumCPU*v)
		}
		return value
	}
	return Reduce(
		job.Result.SliceProperties,
		result,
		func(r []string, property string) []string {
			if value := strings.TrimPrefix(property, `CPUQuota=`); value != property {
				property = fmt.Sprintf("CPUQuota=%s", runtimeCPUQuota(value))
			}
			return append(r, property)
		},
	)
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
			"systemd-run", "--${manager}", "-G", "-d",
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
	if len(job.Commands) > 0 {
		c := job.Commands[len(job.Commands)-1].Content()
		c = append(c, `"$@"`)
		job.Commands[len(job.Commands)-1] = NewCommand(c...)
		result = Map(job.Commands, func(c Command) string { return c.ShellCmd() })
		return
	}
	return nil, fmt.Errorf("%w: missing commands", ErrNotFound)
}

func (job *ProcJob) NicyExec() string {
	return fmt.Sprintf(`exec nicy run -- %v "$@"`, job.Request.Path)
}

func (job *ProcJob) Script(shell string) (result []string, err error) {
	lines, err := job.Show()
	if err != nil {
		return
	}
	result = append(result, "#!"+shell)
	if viper.GetBool("run") {
		result = append(
			result,
			"", "# With capabilities, no other permission is required",
			"command -v nicy >/dev/null 2>&1 &&", "  "+job.NicyExec(),
		)
	}
	result = append(result, "", "# May require sudo credentials")
	result = append(result, lines...)
	result = append(result, "", "# vim: set ft="+filepath.Base(shell))
	return
}

func (job *ProcJob) RuntimeCommands(args ...string) []Command {
	return Reduce(([]Command)(job.Commands), []Command{}, func(s []Command, c Command) []Command {
		if !c.skipRuntime { // Remove tests and shebang line from loop
			// Append command args, if any, when required
			if c.Index(0) == "exec" && len(args) > 0 {
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
	uid := viper.GetInt("uid")
	pid := viper.GetInt("pid")
	nonfatal(updatePrivileges(true))
	var unprivileged []Command
	for _, c := range commands {
		// run only lines that require some capabilities
		_, flag := c.RequireSysCapability()
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
		if cToRun.Index(0) == "exec" {
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
			job.Procs[i].AdjustSchedRTPrio(job.Procs[i].ProcMap.Pid)
		}
	}
	if job.Diff.HasOomScoreAdj() {
		for i, _ := range job.Procs {
			job.Procs[i].AdjustOomScoreAdj(job.Procs[i].ProcMap.Pid)
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
	tokens = append(tokens, fmt.Sprintf("%d", job.leader.ProcMap.Pid))
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
		"set-property", job.leader.ProcMap.Unit,
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
	cgroup := job.leader.ProcMap.Cgroup
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
	id := fmt.Sprintf("%s[%d]", job.leader.ProcMap.Comm, job.Pgrp)
	if viper.GetBool("verbose") || viper.GetBool("dry-run") {
		inform(tag, fmt.Sprintf(
			"%s: cgroup:%s pids:%v",
			id, job.leader.ProcMap.Unit, job.Pids,
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
		_, flag := c.RequireSysCapability()
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

func ProcMapToProcJob(procmaps []*ProcMap) []*ProcJob {
	// sort first by Pgrp
	sort.Sort(ProcMapByPgrp(procmaps))
	return Map(procmaps, NewMapJob)
}

func (job *ProcGroupJob) Add(p *ProcJob) (err error) {
	if p.ProcMap.Pgrp == job.Pgrp {
		job.Pids = append(job.Pids, p.ProcMap.Pid)
		job.Procs = append(job.Procs, p)
		if len(job.Procs) == 1 {
			job.leader = job.Procs[0]
		}
	} else {
		err = fmt.Errorf("%w: wrong pgrp: %d", ErrInvalid, p.ProcMap.Pgrp)
		warn(err)
	}
	return
}

func (job *ProcGroupJob) LeaderInfo() (result string) {
	if len(job.Procs) > 0 {
		result = fmt.Sprintf(
			"Group pgrp %d leader (%s)[%d]",
			job.Pgrp, job.Procs[0].ProcMap.Comm, job.Procs[0].ProcMap.Pid,
		)
	}
	return
}

// vim: set ft=go fdm=indent ts=2 sw=2 tw=79 noet:
