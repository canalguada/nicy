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
	// "io"
	"io/ioutil"
	"encoding/json"
	"sync"
	"github.com/canalguada/nicy/process"
	"github.com/canalguada/nicy/jq"
	"github.com/spf13/viper"
)

type CStringMap = map[string]interface{}
type CSlice = []interface{}

var cacheContent CStringMap

func ReadCache() (cache CStringMap, err error) {
	data, err := ioutil.ReadFile(viper.GetString("database"))
	if err != nil {
		return
	}
	err = json.Unmarshal(data, &cache)
	return
}

func ReadCategoryCache(category string) (cache CSlice, err error) {
	data, err := ioutil.ReadFile(viper.GetString(category))
	if err != nil {
		return
	}
	err = json.Unmarshal(data, &cache)
	return
}

type Request struct {
	jq.Request
}

func NewRequest(script string, vars []string, values ...interface{}) *Request {
	req := &Request{}
	req.Request = *jq.NewRequest(script, vars, values...)
	return req
}

func NewCacheRequest(script string, vars []string, values ...interface{}) *Request {
	var rvars = []string{"$cachedb"}
	var rvalues = CSlice{cacheContent}
	if len(vars) != 0 {
		rvars = append(rvars, vars...)
		rvalues = append(rvalues, values...)
	}
	req := NewRequest(script, rvars, rvalues...)
	req.LibDirs = []string{expandPath(viper.GetString("jqlibdir"))}
	return req
}

type TransformMap func(o CStringMap) CStringMap
type TransformSlice func(o CSlice) CSlice

var PassMap = func(o CStringMap) CStringMap { return o }
var PassSlice = func(a CSlice) CSlice { return a }

func mapResultToChan(input CSlice, output chan<- CStringMap, f TransformMap) {
	for _, val := range input {
		result, ok := val.(CStringMap)
		if !(ok) {
			// Skip if not valid result
			warn(fmt.Errorf("%w: map expected, but got: %T %v", ErrInvalid, val, val))
			continue
		}
		// Extract map output
		if viper.GetBool("verbose") {
			debug(fmt.Sprintf("map expected: %T %v", result, result))
		}
		output <- f(result)
	}
}

func (req *Request) MapResultToChan(input interface{}, output chan<- CStringMap, f TransformMap) {
	// Get result
	objects, err := req.Result(input)
	if err != nil {
		// Skip if not valid object
		warn(fmt.Errorf("%w: %v", ErrInvalid, err))
	} else {
		// Send result
		mapResultToChan(objects, output, f)
	}
}

func sliceResultToChan(input CSlice, output chan<- CSlice, f TransformSlice) {
	if viper.GetBool("verbose") {
		debug(fmt.Sprintf("slice expected: %T %v", input, input))
	}
	output <- f(input)
}

func (req *Request) SliceResultToChan(input interface{}, output chan<- CSlice, f TransformSlice) {
	// Get result
	result, err := req.Result(input)
	if err != nil {
		// Skip if not valid result
		warn(fmt.Errorf("%w: %v", ErrInvalid, err))
	} else {
		// Send result
		sliceResultToChan(result, output, f)
	}
}

func (req *Request) GetMapFromMap(input <-chan CStringMap, output chan<- CStringMap, wg *sync.WaitGroup) {
	defer wg.Done()
	for obj := range input {
		req.MapResultToChan(obj, output, PassMap)
	}
	close(output)
}

func (req *Request) GetMapFromSlice(input <-chan CSlice, output chan<- CStringMap, wg *sync.WaitGroup) {
	defer wg.Done()
	for obj := range input {
		req.MapResultToChan(obj, output, PassMap)
	}
	close(output)
}

func (req *Request) GetSliceFromSlice(input <-chan CSlice, output chan<- CSlice, wg *sync.WaitGroup) {
	defer wg.Done()
	for obj := range input {
		req.SliceResultToChan(obj, output, PassSlice)
	}
	close(output)
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
		Shell: filepath.Base(shell),
		NumCPU: numCPU(),
		MaxNice: int(rlimitNice().Max),
		CPUSched: p.CPUSchedInfo(),
		IOSched: p.IOSchedInfo(),
	}
}

func (input *RunInput) GetStringMap() CStringMap {
	data, err := json.Marshal(*input)
	fatal(wrap(err))
	var result CStringMap
	fatal(json.Unmarshal(data, &result))
	return result
}

type ManageInput struct {
	CSlice
}

func (input *ManageInput) Append(p *process.Proc) error {
	input.CSlice = append(input.CSlice, p.GetStringMap())
	return nil
}

func NewManageInput(filter process.Filter) *ManageInput {
	input := &ManageInput{}
	for _, p := range process.FilteredProcs(filter) {
		input.Append(&p)
	}
	return input
}

func (input ManageInput) GetSlice() CSlice {
	return input.CSlice
}

// vim: set ft=go fdm=indent ts=2 sw=2 tw=79 noet:
