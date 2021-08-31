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
	"path/filepath"
	"io"
	"encoding/json"
	"github.com/canalguada/nicy/process"
	"github.com/spf13/viper"
)


type RunInput struct {
	Name string					`json:"name"`
	Path string					`json:"cmd"`
	Preset string				`json:"preset"`
	Cgroup string				`json:"cgroup"`
	ForceCgroup bool		`json:"probe_cgroup"`
	Managed bool				`json:"managed"`
	Quiet bool					`json:"quiet"`
	Verbosity int				`json:"verbosity"`
	Shell string				`json:"shell"`
	NumCPU int					`json:"nproc"`
	MaxNice int					`json:"max_nice"`
	CPUSched string			`json:"cpusched"`
	IOSched string			`json:"iosched"`
}

func NewRunInput(cmdPath string) *RunInput {
	p := process.GetCalling()
	return &RunInput{
		Name: filepath.Base(cmdPath),
		Path: cmdPath,
		Preset: viper.GetString("preset"),
		Cgroup: viper.GetString("cgroup"),
		ForceCgroup: viper.GetBool("force-cgroup"),
		Managed: viper.GetBool("managed"),
		Quiet: viper.GetBool("quiet"),
		//TODO: Update jq library
		Verbosity: 1,
		Shell: filepath.Base(viper.GetString("shell")),
		NumCPU: numCPU(),
		MaxNice: int(rlimitNice().Max),
		CPUSched: p.CPUSchedInfo(),
		IOSched: p.IOSchedInfo(),
	}
}

func (input *RunInput) Map() map[string]interface{} {
	data, err := json.Marshal(*input)
	checkErr(err)
	result := make(map[string]interface{})
	checkErr(json.Unmarshal(data, &result))
	return result
}

type ManageInput struct {
	procmaps []interface{}
}

func (input *ManageInput) Append(p *process.Proc) error {
	input.procmaps = append(input.procmaps, p.Map())
	return nil
}

func (input *ManageInput) Slice() []interface{} {
	s := make([]interface{}, len(input.procmaps))
	copy(s, input.procmaps)
	return s
}

type ManageJob struct {
	Pgrp int						`json:"Pgrp"`
	Comm string					`json:"Comm"`
	Unit string					`json:"Unit"`
	Pids []int					`json:"Pids"`
	Commands []CmdLine	`json:"Commands"`
}

func (job *ManageJob) Append(lines ...CmdLine) {
	job.Commands = append(job.Commands, lines...)
}

func NewManageJob(obj *map[string]interface{}) *ManageJob {
	job := &ManageJob{}
	if data, err := json.Marshal(*obj); err == nil {
		checkErr(json.Unmarshal(data, job))
	}
	return job
}

func (job *ManageJob) Run(stdout, stderr io.Writer) {
	// Set job tag
	tag := fmt.Sprintf("%s[%d]", job.Comm, job.Pgrp)
	fmt.Fprintln(
		stderr,
		prog + ":",
		viper.GetString("tag") + ":",
		tag + ":",
		fmt.Sprintf(
			"cgroup:%s pids:%v",
			job.Unit,
			job.Pids,
		),
	)
	// Finally run commands
	for _, cmdline := range Script(job.Commands).ManageCmdLines() {
		if err := cmdline.Run(tag, nil, stdout, stderr); err != nil {
			// Loop exit on error
			fmt.Fprintln(stderr, err)
			break
		}
	}
}

// vim: set ft=go fdm=indent ts=2 sw=2 tw=79 noet:
