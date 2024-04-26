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
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/spf13/viper"
)

var regexes map[string]*regexp.Regexp

func init() {
	regexes = make(map[string]*regexp.Regexp)
	for k, v := range map[string]string{
		"user":    `user-\d+\.slice`,
		"session": `user-\d+\.slice/session-\d+\.scope`,
		"managed": `user@\d+\.service/.+\.slice`,
		"system":  `system\.slice`,
		"nicy":    `nicy.slice/nicy-.+\.slice`,
	} {
		regexes[k] = regexp.MustCompile(v)
	}
}

type ProcJob struct {
	*Proc
	*Request `json:"request"`
	Rule     `json:"rule"`
	Commands []Command `json:"commands"`
}

func NewProcJob(p *Proc) *ProcJob {
	name := strings.Split(p.Comm, `:`)[0]
	req := NewRequest(name, fmt.Sprintf("%%%s%%", name), "/bin/sh")
	req.Quiet = true
	return &ProcJob{Proc: p, Request: req}
}

func (job *ProcJob) AddCommand(c Command) {
	if !(c.IsEmpty()) {
		job.Commands = append(job.Commands, c)
	}
}

func (job *ProcJob) AddProfileCommand(property string, c Command) {
	var tokens []string
	if job.IsPrivilegeRequired(property) {
		job.SetSudoIfRequired()
		tokens = append(tokens, "$SUDO")
	}
	tokens = append(tokens, c.Content()...)
	tokens = append(tokens, ">/dev/null")
	job.AddCommand(NewCommand(tokens...))
}

func (job ProcJob) IsPrivilegeRequired(command string) bool {
	for _, value := range job.Rule.Credentials {
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
	for _, value := range job.Rule.Credentials {
		if value == command {
			return
		}
	}
	job.Rule.Credentials = append(job.Rule.Credentials, command)
}

func (job *ProcJob) SetSudoIfRequired() {
	if !(job.IsSudoSet()) {
		job.SetPrivilege("sudo")
		trace("SetSudoIfRequired", "shell", job.Request.Shell)
		job.AddCommand(SudoCommand(job.Request.Shell))
	}
}

func (job *ProcJob) AdjustNice(pid int) error {
	job.AddProfileCommand("nice", renice(job.Rule.Nice, pid))
	return nil
}

func (job *ProcJob) AdjustOomScoreAdj(pid int) error {
	if r := job.Rule; r.HasOomScoreAdj() {
		job.AddProfileCommand("oom_score_adj", choom(job.Rule.OomScoreAdj, pid))
	}
	return nil
}

func (job *ProcJob) AdjustSchedRTPrio(pid int) error {
	var (
		sched  string
		rtprio int
		r      Rule = job.Rule
	)
	if r.HasSched() {
		sched = r.Sched
	}
	if r.HasRtprio() {
		if policy := job.Request.Proc.Policy; (sched == "fifo") ||
			(sched == "rr") || (len(sched) == 0 && (policy == 1 || policy == 2)) {
			rtprio = r.RTPrio
		} else {
			rtprio = -1
		}
	}
	if len(sched) > 0 || rtprio != 0 {
		job.AddProfileCommand("sched", chrt(sched, rtprio, pid))
	}
	return nil
}

func (job *ProcJob) AdjustIOClassIONice(pid int) error {
	var (
		class string
		level int  = -1
		r     Rule = job.Rule
	)
	if r.HasIoclass() {
		class = r.IOClass
	}
	if r.HasIonice() {
		if policy := job.Request.Proc.IOPrioClass; (class == "realtime") ||
			(class == "best-effort") || (len(class) == 0 && (policy == 1 || policy == 2)) {
			level = r.IONice
		}
	}
	if len(class) > 0 || level != -1 {
		job.AddProfileCommand("ioclass", ionice(class, level, pid))
	}
	return nil
}

func (job *ProcJob) getEnvironment() (result []string) {
	if len(job.Rule.Env) > 0 {
		for key, value := range job.Rule.Env {
			result = append(result, fmt.Sprintf("%s=%s", key, value))
		}
	}
	return
}

func (job *ProcJob) sliceUnit() string {
	return fmt.Sprintf("nicy-%s.slice", job.Rule.CgroupKey)
}

func (job *ProcJob) inCurrentUserSlice() bool {
	reCallingUser := regexp.MustCompile(
		`user-` + strconv.Itoa(os.Getuid()) + `\.slice`,
	)
	return reCallingUser.MatchString(job.Proc.Cgroup)
}

func (job *ProcJob) inContext(context string) bool {
	return regexes[context].MatchString(job.Proc.Cgroup)
}

func (job *ProcJob) inUserSlice() bool {
	return job.inContext("user")
}

func (job *ProcJob) inSessionScope() bool {
	return job.inContext("session")
}

func (job *ProcJob) inManagedSlice() bool {
	return job.inContext("managed")
}

func (job *ProcJob) inSystemSlice() bool {
	return job.inContext("system")
}

func (job *ProcJob) inAnyNicySlice() bool {
	return job.inContext("nicy")
}

func (job *ProcJob) inProperSlice() bool {
	if job.Rule.HasCgroupKey() {
		flag, _ := regexp.MatchString(job.sliceUnit(), job.Proc.Cgroup)
		return flag
	}
	return true
}

func (job *ProcJob) manager() string {
	if job.Proc.Pid == 0 { // run command
		return "--${manager}"
	}
	if job.inSystemSlice() {
		return "--system"
	}
	return "--user"
}

func (job *ProcJob) prefix() (result []string) {
	if job.Proc.Pid == 0 || job.inCurrentUserSlice() {
		return // run command or no privilege required
		// only root is expected to manage processes outside his slice.
	} else if job.inUserSlice() {
		// Current user MUST run the command as the slice owner.
		// Environment variables are required for busctl and systemctl commands
		if os.Getuid() != 0 {
			result = append(result, "$SUDO")
		}
		path := fmt.Sprintf("/run/user/%d", job.Proc.Uid)
		result = append(
			result,
			"runuser", "-u", job.Proc.Username(), "--", "env",
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
		fatal(fmt.Errorf("%w: invalid cgroup: %s", ErrInvalid, job.Proc.Cgroup))
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
		job.Rule.SliceProperties,
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
	if r := job.Rule; r.HasCgroupKey() {
		base := []string{"systemctl", job.manager()}
		prefix := job.prefix()
		if len(prefix) > 0 {
			base = append(prefix, base...)
		}
		unit = job.sliceUnit()
		args := append(base, "start", unit, ">/dev/null") // start unit
		job.AddCommand(NewCommand(args...))
		if len(r.SliceProperties) > 0 { // set properties
			args := append(base, "--runtime", "set-property", unit)
			properties := job.convertSliceProperties()
			args = append(args, properties...)
			args = append(args, ">/dev/null")
			job.AddCommand(NewCommand(args...))
		}
	}
	return
}

func (job *ProcJob) unitPattern() string {
	unit := []string{job.Request.Name}
	// add cgroup to get unique pattern, when moving processes
	if r := job.Rule; r.HasCgroupKey() {
		unit = append(unit, r.CgroupKey)
	}
	unit = append(unit, `$$`)
	return strings.Join(unit, `-`)
}

func (job *ProcJob) AddExecCmd() error {
	r := job.Rule
	useScope := r.NeedScope() || job.Request.Managed
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
		for _, value := range r.BaseCgroup.ScopeProperties() {
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
	if len(job.Rule.CmdArgs) > 0 {
		c = append(c, r.CmdArgs...)
	}
	trace("AddExecCmd", "command content", c)
	job.AddCommand(NewCommand(c...))
	return nil
}

func (job *ProcJob) PrepareCommands() error {
	job.AdjustNice(0) // No pid nor pgrp
	job.AdjustSchedRTPrio(0)
	job.AdjustIOClassIONice(0)
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

func (job *ProcJob) Run(tag string, args []string, std *Streams) (err error) {
	// setup execution environment
	uid := viper.GetInt("uid")
	pid := viper.GetInt("pid")
	var unprivileged []Command
	// first, run lines that require some capabilities
	nonfatal(updatePrivileges(true))
	for _, c := range Reduce(job.Commands, []Command{}, func(s []Command, c Command) []Command {
		// Remove tests, shebang line and empty line from loop
		if !c.skipRuntime && !c.IsEmpty() {
			// Append command args, if any, when required
			if len(args) > 0 && c.Index(0) == "exec" {
				c = NewCommand(append(c.Content(), args...)...)
			}
			s = append(s, c)
		}
		return s
	}) {
		if !(c.RequireSysCapability()) {
			unprivileged = append(unprivileged, c)
			continue
		}
		c := c.Runtime(pid, uid)
		if err = c.StartWait(tag, std); err != nil {
			debug(getCapabilities())
			return
		}
	}
	nonfatal(updatePrivileges(false, true)) // clear all
	// then adjust oom_score, prepare systemd slice and exec
	for _, c := range Map(unprivileged, func(c Command) Command {
		return c.Runtime(pid, uid)
	}) {
		if err = c.Run(tag, std); err != nil {
			return
		}
	}
	return
}

// vim: set ft=go fdm=indent ts=2 sw=2 tw=79 noet:
