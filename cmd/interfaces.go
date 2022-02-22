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
	"path/filepath"
	"io/ioutil"
	"encoding/json"
	"github.com/spf13/viper"
)

type CStringMap = map[string]interface{}
type CSlice = []interface{}

func ReadCategoryCache(category string) (cache CSlice, err error) {
	if data, e := ioutil.ReadFile(viper.GetString(category + `s`)); e != nil {
		err = e
	} else {
		err = json.Unmarshal(data, &cache)
	}
	return
}

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

func NewRunInput(shell string, cmdPath string) *RunInput {
	p := GetCalling()
	return &RunInput{
		Name: filepath.Base(cmdPath),
		Path: cmdPath,
		Preset: viper.GetString("preset"),
		Cgroup: viper.GetString("cgroup"),
		ForceCgroup: viper.GetBool("force-cgroup"),
		Managed: viper.GetBool("managed"),
		Quiet: viper.GetBool("quiet"),
		Verbosity: 1,
		Shell: filepath.Base(shell),
		NumCPU: numCPU(),
		MaxNice: int(rlimitNice().Max),
		CPUSched: p.CPUSchedInfo(),
		IOSched: p.IOSchedInfo(),
	}
}

func (input *RunInput) GetStringMap() (result CStringMap) {
	data, err := json.Marshal(*input)
	fatal(wrap(err))
	fatal(json.Unmarshal(data, &result))
	return
}

// vim: set ft=go fdm=indent ts=2 sw=2 tw=79 noet:
