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
	"reflect"
	"github.com/canalguada/nicy/process"
	"github.com/spf13/viper"
)


type JqInput struct {
	Name string
	Path string
	Preset string
	Cgroup string
	ForceCgroup bool
	Managed bool
	Quiet bool
	Verbosity int
	Shell string
	NumCPU int
	MaxNice int
	SchedRTPrio string
	SchedIOPrio string
}

func NewJqInput(command string) *JqInput {
	p := process.GetCalling()
	policy := process.CPU.Class[p.Policy]
	cpuinfo := fmt.Sprintf(
		"PID  %d: PRIO   %d, POLICY %c: %s",
		p.Pid, p.RTPrio, policy[6], policy,
	)
	ioinfo := fmt.Sprintf(
		"%s: prio %d", process.IO.Class[p.IOPrioClass], p.IOPrioData)
	return &JqInput{
		Name: filepath.Base(command),
		Path: command,
		Preset: viper.GetString("preset"),
		Cgroup: viper.GetString("cgroup"),
		ForceCgroup: viper.GetBool("force-cgroup"),
		Managed: viper.GetBool("managed"),
		Quiet: viper.GetBool("quiet"),
		Verbosity: viper.GetInt("verbose"),
		Shell: filepath.Base(viper.GetString("shell")),
		NumCPU: numCPU(),
		MaxNice: int(rlimitNice().Max),
		SchedRTPrio: cpuinfo,
		SchedIOPrio: ioinfo,
	}
}

func (input *JqInput) Slice() []interface{} {
	slice := make([]interface{}, 13)
	valueOf := reflect.ValueOf(*input)
	for i := 0; i< valueOf.NumField(); i++ {
		switch valueOf.Field(i).Type() {
		case reflect.TypeOf("string"):
			slice[i] = valueOf.Field(i).Interface()
		case reflect.TypeOf(true):
			slice[i] = fmt.Sprintf("%t", valueOf.Field(i).Interface())
		case reflect.TypeOf(10):
			slice[i] = fmt.Sprintf("%d", valueOf.Field(i).Interface())
		}
	}
	return slice
}


type JqManageInput struct {
	// pid int
	// ppid int
	// pgrp int
	// uid int
	// user string
	// state string
	// slice string
	// unit string
	// comm string
	// cgroup string
	// priority int
	// nice int
	// num_threads int
	// rtprio int
	// policy int
	// oom_score_adj int
	// ioclass string
	// ionice int
	objects []interface{}
}

func (input *JqManageInput) Append(p *process.Proc) error {
	input.objects = append(input.objects, p.Map())
	return nil
}

func (input *JqManageInput) Objects() []interface{} {
	return input.objects
}

type JqManageOutput struct {
	Pgrp int
	Comm string
	Unit string
	Pids []int
	Commands []CmdLine
}

func (out *JqManageOutput) Append(lines ...CmdLine) {
	out.Commands = append(out.Commands, lines...)
}

func NewJqManageOutput(obj *map[string]interface{}) *JqManageOutput {
	out := &JqManageOutput{}
	for k, v := range *obj {
		switch {
		case k == "Pgrp":
			if v, ok := v.(int); ok {
				out.Pgrp = v
			}
		case k == "Comm":
			if v, ok := v.(string); ok {
				out.Comm = v
			}
		case k == "Unit":
			if v, ok := v.(string); ok {
				out.Unit = v
			}
		case k == "Pids":
			if v, ok := v.([]interface{}); ok {
				for _, v := range v {
					if v, ok := v.(int); ok {
						out.Pids = append(out.Pids, v)
					}
				}
			}
		case k == "Commands":
			if lines, err := DecodeScript(v); err == nil {
				out.Commands = append(out.Commands, lines...)
			}
			// if v, ok := v.([]interface{}); ok {
			//   for _, vs := range v {
			//     if vs, ok := vs.([]interface{}); ok {
			//       var line CmdLine
			//       for _, s := range vs {
			//         if s, ok := s.(string); ok && len(s) > 0 {
			//           line = append(line, s)
			//         }
			//       }
			//       out.Append(line)
			//     }
			//   }
			// }
		}
	}
	return out
}

// vim: set ft=go fdm=indent ts=2 sw=2 tw=79 noet:
