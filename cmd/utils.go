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
	"context"
	"os/signal"
	"bufio"
	"regexp"
	"io"
	"io/ioutil"
	"path/filepath"
	"strings"
	"sort"
	"encoding/json"
	"time"
	"runtime"
	"sync"
	"golang.org/x/sys/unix"
	"github.com/spf13/viper"
)

var (
	reNoComment *regexp.Regexp
	goMaxProcs = runtime.GOMAXPROCS(0)
)

func init() {
	reNoComment = regexp.MustCompile(`^[ ]*#`)
}

func getWaitGroup() (wg sync.WaitGroup) {
	return
}

// Build command

// readDirNames reads the directory named by dirname and returns a sorted list
// of directory entries.
func readDirNames(dirname string) (names []string, err error) {
	f, err := os.Open(dirname)
	if err != nil {
		return
	}
	names, err = f.Readdirnames(-1)
	f.Close()
	if err != nil {
		return
	}
	sort.Strings(names)
	return
}

// findFiles walks the file tree rooted at root and returns the sorted list of
// files that match pattern, mindepth and maxdepth requirements.
// Returns also any error that could arise visiting files and directories, or
// using malformed pattern.
func findFiles(root, pattern string, mindepth, maxdepth int) (result []string, err error) {
	var depth int
	var relative string
	err = filepath.Walk(
		root,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err // Failure accessing path
			}
			if path == root {
				depth = 0
			} else {
				relative, _ = filepath.Rel(root, path)
				depth = len(strings.Split(relative, string(filepath.Separator))) - 1
			}
			if info.Mode().IsDir() || depth < mindepth || depth > maxdepth {
				return nil
			}
			matched, err := filepath.Match(pattern, info.Name())
			if err != nil {
				return err // Invalid pattern
			}
			if matched {
				result = append(result, path)
			}
			return nil
		},
	)
	sort.Strings(result)
	return
}

func timestamp() string {
	return fmt.Sprintf("%v", time.Now().Unix())
}

func getTempFile(dest, pattern string) (*os.File, error) {
	file, err := ioutil.TempFile(dest, pattern)
	if err != nil {
		return nil, err
	}
	return file, nil
}

func writeTo(path string, buf []byte) (n int, err error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	n, err = f.Write(buf)
	return
}

func appendTo(path string, buf []byte) (n int, err error) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	n, err = f.Write(buf)
	return
}

type stringToStringMap func(s string) (CStringMap, error)

func dumpContent(path string, function stringToStringMap) (content CSlice, err error) {
	var (
		bufline []byte
		item CStringMap
	)
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		bufline = scanner.Bytes()
		if len(bufline) != 0 && !(reNoComment.Match(bufline)) {
			item, err = function(scanner.Text())
			if err != nil {
				return
			}
			content = append(content, item)
		}
	}
	if err = scanner.Err(); err != nil {
		err = fmt.Errorf("%w", err)
	}
	return
}

func dumpObjects(category, root string) (objects CSlice, err error) {
	var mindepth, maxdepth int
	pattern := fmt.Sprintf("*.%ss", category)
	if category == "rule" {
		mindepth = 1
		maxdepth = 10
	}
	files, err := findFiles(root, pattern, mindepth, maxdepth)
	if err != nil {
		return
	}
	sort.Sort(sort.Reverse(sort.StringSlice(files)))
	for _, file := range files {
		// Each line contains a json object to unmarshal into CStringMap
		lines, e := dumpContent(file, func(s string) (CStringMap, error) {
			var result CStringMap
			if err := json.Unmarshal([]byte(s), &result); err != nil {
				return nil, err
			}
			result["origin"] = root
			return result, nil
		})
		objects = append(objects, lines...)
		if e != nil {
			err = e
			break
		}
	}
	return
}

// List command

func listObjects(category string) (output []string, err error) {
	contentCache, err = ReadFile() // get cache content
	fatal(wrap(err))
	category = strings.TrimRight(category, "s")
	var list []string
	if viper.IsSet("from") {
		list, err = contentCache.ListFrom(category, viper.GetString("from"))
	} else {
		list, err = contentCache.List(category)
	}
	sort.Strings(list)
	output = append(output, fmt.Sprintf("%s\tcontent", category))
	output = append(output, list...)
	return
}

// Run and show commands

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
		command, err = filepath.Abs(command)
		if err != nil {
			return
		}
	}
	paths, err := lookAll(command)
	if err != nil {
		return
	}
	prefix := expandPath(viper.GetString("scripts"))
	for _, path := range paths {
		realpath, _ := filepath.EvalSymlinks(path)
		if ! strings.HasPrefix(realpath, prefix) {
			valid = realpath
			return
		}
	}
	err = fmt.Errorf("%w: \"%v\": valid executable file not found in $PATH", ErrInvalid, command)
	return
}

// Show

func getRunJob(shell string, args []string, output chan<- *ProcJob, wgmain *sync.WaitGroup) (err error) {
	defer wgmain.Done()
	// check command path
	command, err := findValidPath(args[0])
	if err != nil {
		return
	}
	contentCache, err = ReadFile() // get cache content, once for all goroutines
	fatal(wrap(err))
	// prepare channels
	inputs := make(chan *RunInput)
	jobs := make(chan *ProcJob)
	// spin up workers
	wg := getWaitGroup() // use a sync.WaitGroup to indicate completion
	wg.Add(1) // get commands
	go func() {
		defer wg.Done()
		job := <-jobs
		nonfatal(job.PrepareLaunch())
		// send result
		output <- job
		close(output)
	}()
	wg.Add(1) // get entries
	go func () {
		defer wg.Done()
		input := <-inputs
		preset, _ := contentCache.GetRunPreset(input) // build job
		jobs <- &ProcJob{Request: *input, Entries: preset.StringMap()}
		close(jobs)
	}()
	inputs <- NewRunInput(shell, command) // send input
	close(inputs)
	wg.Wait() // wait on the workers to finish
	return
}

func showCommand(shell string, args []string) (result []string, err error) {
	// prepare channels
	jobs := make(chan *ProcJob, 2)
	// spin up workers
	wg := getWaitGroup() // use a sync.WaitGroup to indicate completion
	wg.Add(1) // get result
	result = append(result, `#!`+ shell)
	go func () {
		defer wg.Done()
		for job := range jobs {
			lines, err := job.Show()
			fatal(err)
			result = append(result, lines...)
		}
	}()
	wg.Add(1) // get commands
	go getRunJob(shell, args, jobs, &wg)
	wg.Wait() // wait on the workers to finish
	return
}

// Run

func prettyJson(v interface{}) (result string) {
	pretty, err := json.MarshalIndent(v, "", "  ") // pretty print json
	if err != nil {
		warn(err)
	} else {
		result = fmt.Sprintln(string(pretty))
	}
	return
}

func doFork() (err error) {
	// usage
	//		err = doFork()
	// 		fatal(err)
	// 		// now, we are child
	// 		pid = os.Getpid()
	ret, _, errNo := unix.RawSyscall(unix.SYS_FORK, 0, 0, 0)
	if errNo != 0 {
		err = fmt.Errorf("%w: fork failed err: %d", ErrFailure, errNo)
		return
	}
	switch ret {
	case 0:
		break
	default:
		os.Exit(0) // parent
	}
	// now, we are child
	sid, err := unix.Setsid()
	if sid < 0 || err != nil {
		err = fmt.Errorf("%w: setsid failed err: %v", ErrFailure, err)
		return
	}
	return
}

func runCommand(tag string, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	// prepare channels
	jobs := make(chan *ProcJob, 2)
	// spin up workers
	wg := getWaitGroup() // use a sync.WaitGroup to indicate completion
	wg.Add(1) // run commands
	go func () {
		defer wg.Done()
		for job := range jobs {
			if err := job.Run(tag, args[1:], stdin, stdout, stderr); err != nil {
				fatal(err)
			}
		}
	}()
	wg.Add(1) // get commands
	go getRunJob("/bin/sh", args, jobs, &wg)
	wg.Wait() // wait on the workers to finish
	return nil
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
	_ = ReviewGroupJobDiff(groupjob)
	if viper.GetBool("debug") && viper.GetBool("verbose") {
		for k, v := range map[string]Preset {
			"preset": Preset(groupjob.Procs[0].Entries),
			"diff": Preset(groupjob.Diff),
		} {
			inform("", fmt.Sprintf(
				"%s %s: %v\n", groupjob.LeaderInfo(), k, v,
			))
		}
	}
	if pgrp != groupjob.Pgrp {
		warn(fmt.Errorf("%w: wrong pgrp: %d", ErrInvalid, groupjob.Pgrp))
	}
	if len(groupjob.Diff) > 0 {
		output <- groupjob
	}
	// no close here but in parent
}

func buildProcGroupJob(input <-chan []*ProcMap, output chan<- *ProcGroupJob, wgmain *sync.WaitGroup) (err error) {
	defer wgmain.Done()
	for procmaps := range input {
		// split input per Pgrp
		byPgrp := make(map[int][]*ProcMap)
		for _, procmap := range procmaps {
			byPgrp[procmap.Pgrp] = append(byPgrp[procmap.Pgrp], procmap)
		}
		// build ProcGroupJob
		wg := getWaitGroup() // use a sync.WaitGroup to indicate completion
		for pgrp, procmaps := range byPgrp { // spin up workers
			wg.Add(1)
			go func(pgrp int, procmaps []*ProcMap) {
				defer wg.Done()
				jobs := ProcMapToProcJob(procmaps)
				groupjob, _ := GroupProcJobs(jobs)
				_ = ReviewGroupJobDiff(groupjob)
				if viper.GetBool("debug") && viper.GetBool("verbose") {
					for k, v := range map[string]Preset {
						"preset": Preset(groupjob.Procs[0].Entries),
						"diff": Preset(groupjob.Diff),
					} {
						inform("", fmt.Sprintf(
							"%s %s: %v\n", groupjob.LeaderInfo(), k, v,
						))
					}
				}
				if pgrp != groupjob.Pgrp {
					warn(fmt.Errorf("%w: wrong pgrp: %d", ErrInvalid, groupjob.Pgrp))
				}
				if len(groupjob.Diff) > 0 {
					output <- groupjob
				}
			}(pgrp, procmaps)
		}
		wg.Wait() // wait on the workers to finish
	}
	close(output)
	return
}

// use []*Proc from FilteredProcs(filter) to produce ProcMap input only
// when relevant to existing content cache.
func filterProcMaps(input <-chan []*Proc, output chan<- []*ProcMap, wg *sync.WaitGroup) {
	defer wg.Done()
	for procs := range input {
		var maps []*ProcMap
		for _, p := range procs {
			name := strings.Split(p.Comm, `:`)[0]
			if contentCache.HasPreset("rule", name) {
				maps = append(maps, NewProcMap(p))
			}
		}
		output <- maps
	}
	close(output)
}

func getGroupJobs(input <-chan []*Proc, output chan<- *ProcGroupJob, wgmain *sync.WaitGroup) (err error) {
	defer wgmain.Done()
	contentCache, err = ReadFile() // get cache content, once for all goroutines
	fatal(wrap(err))
	// prepare channels
	jobs := make(chan *ProcGroupJob, 8)
	procmaps := make(chan []*ProcMap, 8)
	// spin up workers
	wg := getWaitGroup() // use a sync.WaitGroup to indicate completion
	wg.Add(1) // prepare process group jobs adding commands
	go prepareGroupJobs(jobs, output, &wg)
	wg.Add(1) // split procmaps and build process group jobs
	go func() {
		defer wg.Done()
		for maps := range procmaps {
			byPgrp := make(map[int][]*ProcMap) // split input per Pgrp
			for _, procmap := range maps {
				byPgrp[procmap.Pgrp] = append(byPgrp[procmap.Pgrp], procmap)
			}
			child := getWaitGroup() // use a sync.WaitGroup to indicate completion
			for pgrp, pgrpmaps := range byPgrp { // spin up workers
				child.Add(1) // build ProcGroupJob
				go createGroupJob(pgrp, pgrpmaps, jobs, &child)
			}
			child.Wait() // wait on the workers to finish
		}
		close(jobs)
	}()
	wg.Add(1) // filter procs and prepare procmaps
	go filterProcMaps(input, procmaps, &wg)
	wg.Wait() // wait on the workers to finish
	return
}

func adjustCommand(tag string, filter Filterer, stdout, stderr io.Writer) (err error) {
	// prepare channels
	runjobs := make(chan *ProcGroupJob, 8)
	procs := make(chan []*Proc, 8)
	// spin up workers
	wg := getWaitGroup() // use a sync.WaitGroup to indicate completion
	for i := 0; i < (goMaxProcs + 1); i++ {
		wg.Add(1) // run jobs
		go func() {
			defer wg.Done()
			for job := range runjobs {
				job.Run(tag, stdout, stderr)
			}
		}()
	}
	wg.Add(1) // get jobs
	go getGroupJobs(procs, runjobs, &wg)
	// send input
	if viper.GetBool("verbose") {
		fmt.Fprintf(stderr, "Adjusting %v...\n", filter)
	}
	procs <- FilteredProcs(filter)
	close(procs)
	wg.Wait() // wait on the workers to finish
	return
}

func controlCommand(tag string, filter Filterer, stdout, stderr io.Writer) (err error) {
	// prepare channels
	runjobs := make(chan *ProcGroupJob, 8)
	procs := make(chan []*Proc, 8)
	// and signal
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, unix.SIGHUP)
	ticker := time.NewTicker(viper.GetDuration("tick"))
	defer func() {
		signal.Stop(signalChan)
		ticker.Stop()
		cancel()
	}()
	// spin up workers
	wg := getWaitGroup() // use a sync.WaitGroup to indicate completion
	for i := 0; i < (goMaxProcs + 1); i++ {
		wg.Add(1) // run jobs
		go func() {
			defer wg.Done()
			for job := range runjobs {
				job.Run(tag, stdout, stderr)
			}
		}()
	}
	wg.Add(1) // get jobs
	go getGroupJobs(procs, runjobs, &wg)
	// send input
	if viper.GetBool("verbose") {
		fmt.Fprintf(stderr, "Controlling %v...\n", filter)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
				case s := <-signalChan:
					switch s {
						case unix.SIGHUP:
							// TODO: Reload cache here
							if viper.GetBool("dry-run") || viper.GetBool("verbose") {
								inform("monitor", "Reloading cache...")
							}
						case os.Interrupt:
							cancel()
							os.Exit(1)
					}
				case <-ctx.Done():
					inform("monitor", "Done.")
					break
					// os.Exit(0)
			}
		}
	}()
	procs <- FilteredProcs(filter)
	for {
		select {
			case <-ctx.Done():
				break
				// close(output)
				// return
			case <-ticker.C:
				procs <- FilteredProcs(filter)
		}
	}
	close(procs)
	wg.Wait() // wait on the workers to finish
	return

}
// TODO: Install command

// vim: set ft=go fdm=indent ts=2 sw=2 tw=79 noet:
