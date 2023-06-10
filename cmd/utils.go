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
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/spf13/viper"
)

var (
	reNoComment *regexp.Regexp
	goMaxProcs  = runtime.GOMAXPROCS(0)
	numCPU      = runtime.NumCPU()
)

func init() {
	reNoComment = regexp.MustCompile(`^[ ]*#`)
}

func getWaitGroup() (wg sync.WaitGroup) {
	return
}

func Map[T, U any](s []T, f func(T) U) []U {
	r := make([]U, len(s))
	for i, v := range s {
		r[i] = f(v)
	}
	return r
}

func Filter[T any](s []T, f func(T) bool) []T {
	var r []T
	for _, v := range s {
		if f(v) {
			r = append(r, v)
		}
	}
	return r
}

func Reduce[T, U any](s []T, init U, f func(U, T) U) U {
	r := init
	for _, v := range s {
		r = f(r, v)
	}
	return r
}

// Build command

func timestamp() string {
	return fmt.Sprintf("%v", time.Now().Unix())
}

func getTempFile(dest, pattern string) (*os.File, error) {
	file, err := os.CreateTemp(dest, pattern)
	if err != nil {
		return nil, err
	}
	return file, nil
}

// Run and show commands

type RunInput struct {
	Name        string `json:"name"`
	Path        string `json:"cmd"`
	Preset      string `json:"preset"`
	Cgroup      string `json:"cgroup"`
	ForceCgroup bool   `json:"probe_cgroup"`
	Managed     bool   `json:"managed"`
	Quiet       bool   `json:"quiet"`
	Verbosity   int    `json:"verbosity"`
	Shell       string `json:"shell"`
	NumCPU      int    `json:"nproc"`
	MaxNice     int    `json:"max_nice"`
	CPUSched    string `json:"cpusched"`
	IOSched     string `json:"iosched"`
}

func NewRunInput(shell string, cmdPath string) *RunInput {
	p := GetCalling()
	return &RunInput{
		Name:        filepath.Base(cmdPath),
		Path:        cmdPath,
		Preset:      viper.GetString("preset"),
		Cgroup:      viper.GetString("cgroup"),
		ForceCgroup: viper.GetBool("force-cgroup"),
		Managed:     viper.GetBool("managed"),
		Quiet:       viper.GetBool("quiet"),
		Verbosity:   1,
		Shell:       shell,
		NumCPU:      numCPU,
		MaxNice:     int(rlimitNice().Max),
		CPUSched:    p.CPUSchedInfo(),
		IOSched:     p.IOSchedInfo(),
	}
}

func findExecutable(file string) error {
	d, err := os.Stat(file)
	if err != nil {
		return err
	}
	if m := d.Mode(); !m.IsDir() && m&0111 != 0 {
		return nil
	}
	return os.ErrPermission
}

// lookAll searches for all executable named files in the
// directories named by the PATH environment variable.
// If file contains a slash, it is tried directly and the PATH is not consulted.
// The result may be an absolute path or a path relative to the current directory.
func lookAll(file string) (result []string, err error) {
	if strings.Contains(file, string(filepath.Separator)) {
		err = findExecutable(file)
		if err == nil {
			result = append(result, file)
		}
		return
	}
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if dir == "" {
			dir = "." // Unix shell semantics: path element "" means "."
		}
		path := filepath.Join(dir, file)
		if e := findExecutable(path); e == nil {
			result = append(result, path)
		}
	}
	if len(result) == 0 {
		err = &exec.Error{file, exec.ErrNotFound}
	}
	return
}

func findValidPath(command string) (valid string, err error) {
	if strings.Contains(command, string(filepath.Separator)) {
		// Set an absolute path
		if command, err = filepath.Abs(command); err != nil {
			return
		}
	}
	var paths []string
	if paths, err = lookAll(command); err == nil {
		prefix := expandPath(viper.GetString("scripts.location"))
		for _, path := range paths {
			realpath, _ := filepath.EvalSymlinks(path)
			if !strings.HasPrefix(realpath, prefix) {
				valid = realpath
				return
			}
		}
	}
	err = fmt.Errorf("%w: %v not found", ErrInvalid, command)
	return
}

func splitCommand(command []string) (string, []string, error) {
	if cmd, err := findValidPath(command[0]); err != nil || len(cmd) == 0 {
		return "", command, err
	} else {
		return cmd, command[1:], nil
	}
}

func generateJobs(inputs <-chan *RunInput, outputs chan<- *ProcJob, wgmain *sync.WaitGroup) (err error) {
	defer func() {
		close(outputs)
		wgmain.Done()
	}()
	presetCache = GetPresetCache() // get cache content, once for all goroutines
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
			jobs <- &ProcJob{
				Request: input,
				Result:  presetCache.GetRunInputRule(input),
			}
		}
		close(jobs)
	}()
	wg.Wait() // wait on the workers to finish
	return
}

// Set and control commands

func prepareGroupJobs(input <-chan *ProcGroupJob, output chan<- *ProcGroupJob, wg *sync.WaitGroup) {
	defer wg.Done()
	for groupjob := range input {
		nonfatal(groupjob.PrepareAdjust())
		output <- groupjob
	}
	close(output)
}

func createGroupJob(pgrp int, procmaps []*ProcMap, output chan<- *ProcGroupJob, wg *sync.WaitGroup) {
	defer wg.Done()
	jobs := ProcMapToProcJob(procmaps)
	groupjob, _ := GroupProcJobs(jobs)
	count, _ := ReviewGroupJobDiff(groupjob)
	if pgrp != groupjob.Pgrp {
		warn(fmt.Errorf("%w: wrong pgrp: %d", ErrInvalid, groupjob.Pgrp))
	}
	if count > 0 {
		output <- groupjob
	}
	// no close here but in parent
}

// use []*Proc from FilteredProcs(filter) to produce ProcMap input only
// when relevant to existing content cache.
func filterProcMaps(input <-chan []*Proc, output chan<- []*ProcMap, wg *sync.WaitGroup) {
	defer wg.Done()
	for procs := range input {
		var maps []*ProcMap
		for _, p := range procs {
			if ruleFilter.Filter(p, nil) {
				maps = append(maps, NewProcMap(p))
			}
		}
		output <- maps
	}
	close(output)
}

func generateGroupJobs(inputs <-chan []*Proc, outputs chan<- *ProcGroupJob, wgmain *sync.WaitGroup) (err error) {
	defer wgmain.Done()
	// contentCache, err = ReadFile() // get cache content, once for all goroutines
	// fatal(wrap(err))
	presetCache = GetPresetCache()
	// prepare channels
	jobs := make(chan *ProcGroupJob, 8)
	procmaps := make(chan []*ProcMap, 8)
	// spin up workers
	wg := getWaitGroup() // use a sync.WaitGroup to indicate completion
	wg.Add(1)            // prepare process group jobs adding commands
	go prepareGroupJobs(jobs, outputs, &wg)
	wg.Add(1) // split procmaps and build process group jobs
	go func() {
		defer wg.Done()
		for maps := range procmaps {
			byPgrp := make(map[int][]*ProcMap) // split input per Pgrp
			for _, procmap := range maps {
				byPgrp[procmap.Pgrp] = append(byPgrp[procmap.Pgrp], procmap)
			}
			child := getWaitGroup()              // use a sync.WaitGroup to indicate completion
			for pgrp, pgrpmaps := range byPgrp { // spin up workers
				child.Add(1) // build ProcGroupJob
				go createGroupJob(pgrp, pgrpmaps, jobs, &child)
			}
			child.Wait() // wait on the workers to finish
		}
		close(jobs)
	}()
	wg.Add(1) // filter procs and prepare procmaps
	go filterProcMaps(inputs, procmaps, &wg)
	wg.Wait() // wait on the workers to finish
	return
}

// func adjustCommand(tag string, filter Filterer, stdout, stderr io.Writer) (err error) {
//   // prepare channels
//   runjobs := make(chan *ProcGroupJob, 8)
//   procs := make(chan []*Proc, 8)
//   // spin up workers
//   wg := getWaitGroup() // use a sync.WaitGroup to indicate completion
//   for i := 0; i < (goMaxProcs + 1); i++ {
//     wg.Add(1) // run jobs
//     go func() {
//       defer wg.Done()
//       for job := range runjobs {
//         job.Run(tag, stdout, stderr)
//       }
//     }()
//   }
//   wg.Add(1) // get jobs
//   go getGroupJobs(procs, runjobs, &wg)
//   // send input
//   if viper.GetBool("verbose") {
//     fmt.Fprintf(stderr, "Adjusting %v...\n", filter)
//   }
//   procs <- FilteredProcs(filter)
//   close(procs)
//   wg.Wait() // wait on the workers to finish
//   return
// }

// TODO: Install command

// vim: set ft=go fdm=indent ts=2 sw=2 tw=79 noet:
