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
	"strings"
	"strconv"
	"regexp"
	"sort"
	"io"
	"encoding/json"
	"github.com/spf13/viper"
)

var (
	reCallingUser *regexp.Regexp
	reUser *regexp.Regexp
	reSession *regexp.Regexp
	reManaged *regexp.Regexp
	reSystem *regexp.Regexp
	reAnyNicy *regexp.Regexp
)

func init() {
	reUser = regexp.MustCompile(`user-\d+\.slice`)
	reSession = regexp.MustCompile(`user-\d+\.slice/session-\d+\.scope`)
	reManaged = regexp.MustCompile(`user@\d+\.service/.+\.slice`)
	reSystem = regexp.MustCompile(`system\.slice`)
	reAnyNicy = regexp.MustCompile(`nicy.slice/nicy-.+\.slice`)
}

// Commands here only change properties of some process or group of processes.

func renice(priority, pid, pgrp int) (result CmdLine) {
	// expects priority in range [-20..19]
	result.Append("renice", "-n", strconv.Itoa(priority))
	if pid == 0 && pgrp == 0 {
		result.Append("-p", "$$")
	} else if pgrp > 0 {
		result.Append("-g", strconv.Itoa(pgrp))
	} else if pid > 0 {
		result.Append("-p", strconv.Itoa(pid))
	}
	return
}

func choom(score, pid int) (result CmdLine) {
	// expects score in range [-1000..1000]
	result.Append("choom", "-n", strconv.Itoa(score))
	if pid > 0 {
		result.Append("-p", strconv.Itoa(pid))
	} else if pid == 0 {
		result.Append("-p", "$$")
	} else {
		result.Append("--")
	}
	return
}

func chrt(sched string, rtprio int, pid int) (result CmdLine) {
	// expects sched in ["other", "fifo", "rr", "batch", "idle"]
	// expects rtprio in range [1..99]
	result.Append("chrt")
	if len(sched) > 0 {
		result.Append("--" + sched)
	}
	if pid > 0 {
		result.Append("-a", "-p", strconv.Itoa(rtprio), strconv.Itoa(pid))
	} else if pid == 0 {
		result.Append("-a", "-p", strconv.Itoa(rtprio), "$$")
	} else {
		result.Append(strconv.Itoa(rtprio))
	}
	return
}

func ionice(class string, level int, pid int, pgrp int) (result CmdLine) {
	// expects class in ["none", "realtime", "best-effort", "idle"]
	// expects level in range [0..7] when it must be set
	result.Append("ionice")
	if len(class) > 0 {
		policy := map[string]string{
			"none": "0",
			"realtime": "1",
			"best-effort": "2",
			"idle": "3",
		}[class]
		result.Append("-c", policy)
	}
	if level >= 0 {
		result.Append("-n", strconv.Itoa(level))
	}
	if pgrp > 0 {
		result.Append("-P", strconv.Itoa(pgrp))
	} else if pid > 0 {
		result.Append("-p", strconv.Itoa(pid))
	} else if pid == 0 {
		result.Append("-p", "$$")
	}
	return
}

type ProcJob struct {
	ProcMap
	Request RunInput		`json:"request"`
	Entries CStringMap	`json:"entries"`
	Commands Script			`json:"commands"`
}

func NewProcJob(obj CStringMap) *ProcJob {
	job := &ProcJob{}
	if data, err := json.Marshal(obj); err == nil {
		fatal(json.Unmarshal(data, job))
	}
	return job
}

func (job ProcJob) HasEntry(name string) bool {
	_, found := job.Entries[name]
	return found
}

func (job ProcJob) GetIntEntry(name string) (result int) {
	value, found := job.Entries[name]
	if found {
		switch value.(type) {
		case int:
			value, _ := value.(int)
			result = value
		case float64:
			value, _ := value.(float64)
			result = int(value)
		}
	}
	return
}

func (job ProcJob) GetStringEntry(name string) (result string) {
	value, found := job.Entries[name]
	if found {
		if value, ok := value.(string); ok {
			result = value
		}
	}
	return
}

func (job ProcJob) GetStringSliceEntry(name string) (result []string) {
	value, found := job.Entries[name]
	if found {
		switch value.(type) {
		case []interface{}:
			value, _ := value.([]interface{})
			for _, v := range value {
				switch v.(type) {
				case string:
					v, _ := v.(string)
					result = append(result, v)
				}
			}
		case []string:
			value, _ := value.([]string)
			for _, v := range value {
				result = append(result, v)
			}
		}
	}
	return
}

func (job ProcJob) IsPrivilegeRequired(command string) bool {
	value := job.GetStringSliceEntry("cred")
	for _, val := range value {
		if val == command {
			return true
		}
	}
	return false
}

func (job ProcJob) IsSudoSet() bool {
	return job.IsPrivilegeRequired("sudo")
}

func (job *ProcJob) SetPrivilege(command string) {
	if !(job.HasEntry("cred")) {
		job.Entries["cred"] = []string{command}
	} else {
		value := job.GetStringSliceEntry("cred")
		job.Entries["cred"] = append(value, command)
	}
}

func (job *ProcJob) SetSudoIfRequired() {
	if !(job.IsSudoSet()) {
		job.SetPrivilege("sudo")
		line:= CmdLineFromString("[ $( id -u ) -ne 0 ] && SUDO=$SUDO || SUDO=")
		line.skipWhenRun = true
		job.Commands.Append(line)
	}
}

func (job *ProcJob) AdjustNice(pid, pgrp int) error {
	if job.HasEntry("nice") {
		var line CmdLine
		if job.IsPrivilegeRequired("nice") {
			job.SetSudoIfRequired()
			line.Append("$SUDO")
		}
		line.Extend(renice(job.GetIntEntry("nice"), pid, pgrp))
		line.Append(">/dev/null")
		job.Commands.Append(line)
		return nil
	}
	return fmt.Errorf("%w: cannot find entry: '%s'", ErrInvalid, "nice")
}

func (job *ProcJob) AdjustOomScoreAdj(pid int) error {
	if job.HasEntry("oom_score_adj") {
		var line CmdLine
		if job.IsPrivilegeRequired("oom_score_adj") {
			job.SetSudoIfRequired()
			line.Append("$SUDO")
		}
		line.Extend(choom(job.GetIntEntry("oom_score_adj"), pid))
		line.Append(">/dev/null")
		job.Commands.Append(line)
		return nil
	}
	return fmt.Errorf("%w: cannot find entry: '%s'", ErrInvalid, "oom_score_adj")
}

func (job *ProcJob) AdjustSchedRTPrio(pid int) error {
	policy := strings.Split(job.Request.CPUSched, `:`)[0]
	var sched string
	var rtprio int
	if job.HasEntry("sched") {
		sched = job.GetStringEntry("sched")
	}
	if job.HasEntry("rtprio") {
		condition := (policy == "1") || (policy == "2")
		condition = (len(sched) == 0) && condition
		if (sched == "fifo") || (sched == "rr") || condition {
			rtprio = job.GetIntEntry("rtprio")
		} else {
			rtprio = -1
		}
	}
	if len(sched) > 0 || rtprio != 0 {
		var line CmdLine
		if job.IsPrivilegeRequired("sched") {
			job.SetSudoIfRequired()
			line.Append("$SUDO")
		}
		line.Extend(chrt(sched, rtprio, pid))
		line.Append(">/dev/null")
		job.Commands.Append(line)
		return nil
	}
	return fmt.Errorf("%w: cannot find entry: 'sched', 'rtprio'", ErrInvalid)
}

func (job *ProcJob) AdjustIOClassIONice(pid, pgrp int) error {
	policy := strings.Split(job.Request.IOSched, `:`)[0]
	var class string
	level := -1
	if job.HasEntry("ioclass") {
		class = job.GetStringEntry("ioclass")
	}
	if job.HasEntry("ionice") {
		condition := (policy == "1") || (policy == "2")
		condition = (len(class) == 0) && condition
		if (class == "realtime") || (class == "best-effort") || condition {
			level = job.GetIntEntry("ionice")
		}
	}
	if len(class) > 0 || level != -1 {
		var line CmdLine
		if job.IsPrivilegeRequired("ioclass") {
			job.SetSudoIfRequired()
			line.Append("$SUDO")
		}
		line.Extend(ionice(class, level, pid, pgrp))
		line.Append(">/dev/null")
		job.Commands.Append(line)
		return nil
	}
	return fmt.Errorf("%w: cannot find entry: 'ioclass', 'ionice'", ErrInvalid)
}

func (job *ProcJob) getEnvironment() (result []string) {
	if job.HasEntry("env") {
		env := job.Entries["env"]
		value, ok := env.(map[string]interface{})
		if ok {
			for k, v := range value {
				var s string
				switch v.(type) {
				case int:
					v, _ := v.(int)
					s = strconv.Itoa(v)
				case string:
					v, _ := v.(string)
					s = v
				}
				result = append(result, fmt.Sprintf("%s=%s", k, s))
			}
		}
	}
	return
}

func  (job *ProcJob) sliceUnit() string {
	return fmt.Sprintf("nicy-%s.slice", job.GetStringEntry("cgroup"))
}

func (job *ProcJob) inCurrentUserSlice() bool {
	reCallingUser = regexp.MustCompile(
		`user-` + strconv.Itoa(viper.GetInt("UID")) + `\.slice`,
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
	if job.HasEntry("cgroup") {
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
		if viper.GetInt("UID") != 0 {
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
		if viper.GetInt("UID") != 0 { // current user must run the command as root
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
	properties := strings.Split(job.GetStringEntry("slice_properties"), ` `)
	for _, property := range properties {
		if value := strings.TrimPrefix(property, `CPUQuota=`); value != property {
			v, err := strconv.Atoi(strings.TrimSuffix(value, `%`))
			if err == nil {
				value = fmt.Sprintf("%d%%", job.Request.NumCPU * v)
			}
			result = append(result, fmt.Sprintf("CPUQuota=%s", value))
		} else {
			result = append(result, property)
		}
	}
	return
}
func (job *ProcJob) startSliceUnit() (unit string) {
	if !(job.HasEntry("cgroup")) {
		return
	}
	var base []string
	unit = job.sliceUnit()
	prefix := job.prefix()
	if len(prefix) > 0 {
		base = append(base, prefix...)
	}
	base = append(base, "systemctl", job.manager())
	words := append(base, "start", unit, ">/dev/null") // start unit
	job.Commands.Append(NewCmdLine(words...))
	if job.HasEntry("slice_properties") { // set properties
		words := append(base, "--runtime", "set-property", unit)
		properties := job.convertSliceProperties()
		words = append(words, properties...)
		words = append(words, ">/dev/null")
		job.Commands.Append(NewCmdLine(words...))
	}
	return
}

func (job *ProcJob) getUnitProperty(entry string) (result string, present bool) {
	if job.HasEntry(entry) {
		value := job.GetStringEntry(entry)
		if entry == "CPUQuota" {
			v, err := strconv.Atoi(strings.TrimSuffix(value, `%`))
			if err == nil {
				value = fmt.Sprintf("%d%%", job.Request.NumCPU * v)
			}
		}
		present = true
		result = fmt.Sprintf("%s=%s", entry, value)
	}
	return
}

func (job *ProcJob) unitPattern() string {
	tokens := []string{job.Request.Name}
	// add cgroup to get unique pattern, when moving processes
	if job.HasEntry("cgroup") {
		tokens = append(tokens, job.GetStringEntry("cgroup"))
	}
	tokens = append(tokens, `$$`)
	return strings.Join(tokens, `-`)
}

func (job *ProcJob) AddExecCmd() error {
	line := NewCmdLine("exec")
	useScope := job.Request.Managed || job.HasEntry("cgroup")
	if useScope {
		skipped := CmdLineFromString("[ $( id -u ) -ne 0 ] && user_or_system=--user || user_or_system=--system")
		skipped.skipWhenRun = true
		job.Commands.Append(skipped)
	}
	execCmd := NewCmdLine(job.Request.Path)
	cmdargs := job.GetStringSliceEntry("cmdargs")
	if len(cmdargs) > 0 {
		execCmd.Extend(NewCmdLine(cmdargs...))
	}
	if useScope {
		var quiet string
		if job.Request.Quiet || (job.Request.Verbosity < 2) {
			quiet = "--quiet"
		}
		line.Append(
			"systemd-run", "${user_or_system}", "-G", "-d",
			"--no-ask-password", quiet, "--scope",
			fmt.Sprintf("--unit=%s", job.unitPattern()),
		)
		if unit := job.startSliceUnit(); len(unit) > 0 {
			line.Append(fmt.Sprintf("--slice=%s", unit))
		}
		for _, entry := range cfgMap["systemd"] {
			value, ok := job.getUnitProperty(entry)
			if ok {
				line.Append("-p", value)
			}
		}
		for _, envvar := range job.getEnvironment() { // Adjust environment
			line.Append("-E", envvar)
		}
	} else {
		envvars := job.getEnvironment()
		if len(envvars) > 0 { // Adjust environment
			line.Append("env")
			quoted := ShellQuote(envvars)
			line.Append(quoted...)
		}
	}
	line.Extend(execCmd)
	job.Commands.Append(line)
	return nil
}

func (job *ProcJob) PrepareLaunch() error {
	job.AdjustNice(0, 0) // No pid nor pgrp
	job.AdjustSchedRTPrio(0)
	job.AdjustIOClassIONice(0, 0)
	job.AdjustOomScoreAdj(0)
	job.AddExecCmd()
	return nil
}

func (job *ProcJob) Show() (result []string, err error) {
	job.Commands[len(job.Commands) - 1].Append(`"$@"`)
	result = job.Commands.ShellLines()
	return
}

func (job *ProcJob) Run(tag string, args []string, stdin io.Reader, stdout, stderr io.Writer) (err error) {
	lines := job.Commands.PrepareRun(args)
	var unprivilegedLines Script
	// setup execution environment
	uid := viper.GetInt("UID")
	pid := os.Getpid()
	// set ambient
	nonfatal(setAmbient(true))
	viper.Set("ambient", getAmbient())
	nonfatal(setDumpable())
	for _, line := range lines {
		// run only lines that require some capabilities
		_, flag := line.RequireSysCapability(job.Uid)
		if !(flag) {
			unprivilegedLines.Append(line)
			continue
		}
		runline := line.Runtime(pid, uid)
		err = runline.StartWait(tag, stdin, stdout, stderr)
		if err != nil {
			debug(getCapabilities())
			return
		}
	}
	// clear capabilities in order to :
	// - allow access to /proc/[pid] filesystem
	// - do not propagate capabilities
	// TODO: check "error: operation not permitted"
	nonfatal(clearAllCapabilities())
	viper.Set("ambient", getAmbient())
	debug(getCapabilities())
	// finally adjust oom_score, prepare systemd slice and exec
	for _, line := range unprivilegedLines {
		runline := line.Runtime(pid, uid)
		if runline.Index(0) == "exec" {
			return runline.Exec(tag)
		} else {
			err = runline.StartWait(tag, stdin, stdout, stderr)
			if err != nil {
				return
			}
		}
	}
	return
}

type ProcGroupJob struct {
	Pgrp int					`json:"pgrp"`
	Pids []int				`json:"pids"`
	Diff CStringMap		`json:"diff"`
	Commands Script		`json:"commands"`
	Procs []*ProcJob		`json:"procs"`
}

func NewProcGroupJob(obj CStringMap) *ProcGroupJob {
	job := &ProcGroupJob{}
	if data, err := json.Marshal(obj); err == nil {
		fatal(json.Unmarshal(data, job))
	}
	return job
}

func (job *ProcGroupJob) HasDiff(name string) bool {
	_, found := job.Diff[name]
	return found
}

func (job *ProcGroupJob) adjustProperties() error {
	// first, using first ProcJob and Pgrp
	if job.HasDiff("nice") {
		job.Procs[0].AdjustNice(0, job.Pgrp)
	}
	if job.HasDiff("ioclass") || job.HasDiff("ionice") {
		job.Procs[0].AdjustIOClassIONice(0, job.Pgrp)
	}
	// second, using each ProcJob and its Pid
	if job.HasDiff("sched") || job.HasDiff("rtprio") {
		for i, _ := range job.Procs {
			job.Procs[i].AdjustSchedRTPrio(job.Procs[i].Pid)
		}
	}
	if job.HasDiff("oom_score_adj") {
		for i, _ := range job.Procs {
			job.Procs[i].AdjustOomScoreAdj(job.Procs[i].Pid)
		}
	}
	return nil
}

func (job *ProcGroupJob) AnyDiff(keys []string) (flag bool) {
	for _, key := range keys {
		if job.HasDiff(key) {
			flag = true
			return
		}
	}
	return
}

func (job *ProcGroupJob) requireScope() bool {
	return job.AnyDiff(cfgMap["cgroup"])
}

func (job *ProcGroupJob) scopeUnit() string {
	tokens := []string{job.Procs[0].Request.Name}
	// add cgroup to get unique pattern, when moving processes
	if job.Procs[0].HasEntry("cgroup") {
		tokens = append(tokens, job.Procs[0].GetStringEntry("cgroup"))
	}
	tokens = append(tokens, fmt.Sprintf("%d", job.Procs[0].Pid))
	return fmt.Sprintf("%s.scope", strings.Join(tokens, `-`))
}

func (job *ProcGroupJob) moveProcGroup() error {
	var (
		count int = 1
		line CmdLine
	)
	useSlice := job.HasDiff("cgroup")
	if useSlice {
		count = 2
	}
	prefix := job.Procs[0].prefix()
	if len(prefix) > 0 {
		line.Append(prefix...)
	}
	line.Append(
		"busctl", "call", "--quiet", job.Procs[0].manager(),
		"org.freedesktop.systemd1", "/org/freedesktop/systemd1",
		"org.freedesktop.systemd1.Manager", "StartTransientUnit",
		"ssa(sv)a(sa(sv))", job.scopeUnit(), "fail", strconv.Itoa(count),
		"PIDs", "au", strconv.Itoa(len(job.Pids)),
	)
	for _, pid := range job.Pids {
		line.Append(strconv.Itoa(pid))
	}
	if useSlice {
		line.Append("Slice", "s", job.Procs[0].sliceUnit())
	}
	line.Append("0")
	job.Procs[0].Commands.Append(line)
	return nil
}

func (job *ProcGroupJob) hasScopeProperties() bool {
	return job.AnyDiff(cfgMap["systemd"])
}

func (job *ProcGroupJob) adjustNewScopeProperties() error {
	var (
		prefix = job.Procs[0].prefix()
		line CmdLine
	)
	if len(prefix) > 0 {
		line.Append(prefix...)
	}
	line.Append(
		"systemctl", job.Procs[0].manager(), "--runtime",
		"set-property", job.scopeUnit(),
	)
	for _, entry := range cfgMap["systemd"] {
		if job.HasDiff(entry) {
			value, ok := job.Procs[0].getUnitProperty(entry)
			if ok {
				line.Append(value)
			}
		}
	}
	job.Procs[0].Commands.Append(line)
	return nil
}

// Adjust each property with its own command.
// Set the properties of the leaf service and don't deal with inner nodes.
// If the cgroup controller for one property is not available, another property
// may still be set, if not requiring it.
func (job *ProcGroupJob) adjustUnitProperties(properties []string) error {
	var (
		prefix = job.Procs[0].prefix()
		base CmdLine
	)
	if len(prefix) > 0 {
		base.Append(prefix...)
	}
	base.Append(
		"systemctl", job.Procs[0].manager(), "--runtime",
		"set-property", job.Procs[0].Unit,
	)
	for _, property := range properties {
		line := base.Copy()
		line.Append(property)
		job.Procs[0].Commands.Append(line)
	}
	return nil
}

func (job *ProcGroupJob) PrepareAdjust() error {
	if len(job.Procs) == 0 {
		return nil
	}
	job.adjustProperties()
	// use process group leader to check cgroup
	cgroup := job.Procs[0].Cgroup
	if reSession.MatchString(cgroup) {
		// set processes running in some session scope
		// check if require moving processes into own scope
		if job.requireScope() {
			// also require slice
			if job.HasDiff("cgroup") {
				job.Procs[0].startSliceUnit()
			}
			// start scope unit, moving all processes sharing same process group
			job.moveProcGroup()
			if job.hasScopeProperties() {
				job.adjustNewScopeProperties()
			}
		}
	} else if reManaged.MatchString(cgroup) || reSystem.MatchString(cgroup) {
		// set processes yet controlled by systemd manager in user or system slice
		var properties []string
		if job.HasDiff("cgroup") {
			// move if running yet inside other nicy slice
			if reAnyNicy.MatchString(cgroup){
				job.Procs[0].startSliceUnit()
				job.moveProcGroup()
			} else if job.Procs[0].HasEntry("slice_properties") {
				properties = job.Procs[0].convertSliceProperties()
			}
		}
		if job.hasScopeProperties() {
			// adjust relevant properties
			for _, entry := range cfgMap["systemd"] {
				if job.HasDiff(entry) {
					value, ok := job.Procs[0].getUnitProperty(entry)
					if ok {
						properties = append(properties, value)
					}
				}
			}
		}
		if len(properties) > 0 {
			job.adjustUnitProperties(properties)
		}
	}
	// collect commands
	for i, _ := range job.Procs {
		job.Commands.Extend(job.Procs[i].Commands)
	}
	return nil
}

func (job *ProcGroupJob) Run(tag string, stdout, stderr io.Writer) error {
	if len(job.Procs) == 0 {
		return nil
	}
	// Show job details
	id := fmt.Sprintf("%s[%d]", job.Procs[0].Comm, job.Pgrp)
	info := fmt.Sprintf("%s: cgroup:%s pids:%v", id, job.Procs[0].Unit, job.Pids)
	if viper.GetBool("verbose") || viper.GetBool("dry-run") {
		doVerbose(tag, info)
	}
	if viper.GetBool("verbose") && viper.GetBool("debug") {
		doVerbose(tag, fmt.Sprintf( "%s: diff: %v", id, job.Diff))
		doVerbose("", fmt.Sprintf( "%s: commands: %v", id, job.Commands))
	}
	// Finally run commands
	var unprivilegedLines Script
	nonfatal(setAmbient(true)) // set ambient
	viper.Set("ambient", getAmbient())
	nonfatal(setDumpable())
	for _, line := range job.Commands {
		if line.skipWhenRun {
			continue
		}
		_, flag := line.RequireSysCapability(job.Procs[0].Uid)
		if !(flag) { // run only lines that require some capabilities
			unprivilegedLines.Append(line)
			continue
		}
		if err := line.StartWait(id, nil, stdout, stderr); err != nil {
			warn(err)
			break // exit loop on error
		}
	}
	nonfatal(setAmbient(false)) // reset ambient capabilities
	viper.Set("ambient", getAmbient())
	nonfatal(setDumpable())
	for _, line := range unprivilegedLines { // don't require any capability
		if err := line.StartWait(id, nil, stdout, stderr); err != nil {
			warn(err)
			break // exit loop on error
		}
	}
	return nil
}

func ProcMapToProcJob(procmaps []*ProcMap) (result []*ProcJob) {
	num := numCPU()
	limit := int(rlimitNice().Max)
	// sort first by Pgrp
	sort.Sort(ProcMapByPgrp(procmaps))
	for _, procmap := range procmaps {
		name := strings.Split(procmap.Comm, `:`)[0]
		job := &ProcJob{
			ProcMap: *procmap,
			Request: RunInput{
				Name: name,
				Path: fmt.Sprintf("%%%s%%", name),
				Preset: "auto",
				Quiet: true,
				Shell: "/bin/sh",
				NumCPU: num,
				MaxNice: limit,
				CPUSched: "0:other:0",
				IOSched: "0:none:0",
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
		Pgrp: leader.Pgrp,
		Pids: []int{leader.Pid},
		Diff: make(CStringMap), // TODO: redefine struct and use Preset
		Procs: []*ProcJob{leader},
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

func ReviewGroupJobDiff(groupjob *ProcGroupJob) (err error) {
	if len(groupjob.Procs) == 0 {
		return
	}
	leader := groupjob.Procs[0] // process group leader only
	input := leader.Request
	// get entries
	if preset, e := contentCache.GetRunPreset(&input); e != nil {
		err = e
		nonfatal(err)
	} else { // review diff
		leader.Entries = preset.StringMap() // set entries
		groupjob.Diff = preset.Diff(leader.Runtime.GetStringMap()).StringMap()
		// check cgroup: processes are easily movable when running inside
		// session scope and some managed nicy slice
		if _, found := groupjob.Diff["cgroup"]; found {
			if leader.inProperSlice() { // running yet inside proper cgroup
				delete(groupjob.Diff, "cgroup")
			} else if !(leader.inAnyNicySlice()) && !(leader.inSessionScope()) {
				delete(groupjob.Diff, "cgroup") // remove cgroup from groupjob.Diff
			}
		}
	} // end process group leader only
	return
}

// vim: set ft=go fdm=indent ts=2 sw=2 tw=79 noet:
