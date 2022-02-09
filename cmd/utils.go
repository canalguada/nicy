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
	"github.com/canalguada/nicy/process"
	// "github.com/canalguada/nicy/jq"
	"github.com/spf13/viper"
)


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
				// Failure accessing path
				return err
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
				// Invalid pattern
				return err
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

type UnmarshalFunc func(s string) (CStringMap, error)

func dumpContent(path string, fn UnmarshalFunc) (content CSlice, err error) {
	var bufline []byte
	var item CStringMap
	re := regexp.MustCompile(`^[ ]*#`)
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	// optionally, resize scanner's capacity for content over 64K
	for scanner.Scan() {
		bufline = scanner.Bytes()
		if len(bufline) != 0 && !(re.Match(bufline)) {
			item, err = fn(scanner.Text())
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
	pattern := fmt.Sprintf("*.%s", category)
	if category == "rules" {
		mindepth = 1
		maxdepth = 10
	}
	files, err := findFiles(root, pattern, mindepth, maxdepth)
	if err != nil {
		return
	}
	sort.Sort(sort.Reverse(sort.StringSlice(files)))
	for _, file := range files {
		// Each line contains a json object to unmarshal into
		// CStringMap
		lines, e := dumpContent(
			file,
			func(s string) (CStringMap, error) {
				var result CStringMap
				if err := json.Unmarshal([]byte(s), &result); err != nil {
					return nil, err
				}
				result["origin"] = root
				return result, nil
			},
		)
		objects = append(objects, lines...)
		if e != nil {
			err = e
			break
		}
	}
	return
}

// List command

func listObjects(category string) (output CSlice, err error) {
	var input CSlice
	// Prepare input
	if viper.IsSet("from") {
		input = CSlice{viper.Get("from")}
	} else {
		slice := viper.GetStringSlice("confdirs")
		input = make(CSlice, len(slice))
		for i, path := range slice {
			input[i] = expandPath(path)
		}
	}
	// Prepare variables
	cacheContent, err = ReadCache()
	fatal(wrap(err))
	// Prepare request
	req := NewCacheRequest(
		`include "list"; list`,
		[]string{"$kind"},
		strings.TrimRight(category, "s"),
	)
	// Get result
	output, err = req.Result(input)
	return
}

// Run and show commands

// func findAllPaths(command string) ([]string, error) {
//   return CmdLine{"sh", "-c", fmt.Sprintf("which -a %s", command)}.Output()
// }

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
			// Unix shell semantics: path element "" means "."
			dir = "."
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

func prepareRunJob(shell string, args []string, output chan<- ProcJob, wgmain *sync.WaitGroup) {
	defer wgmain.Done()
	// Check command path
	command, err := findValidPath(args[0])
	if err != nil {
		return
	}
	// get cache content, once for both go routines
	cacheContent, err = ReadCache()
	fatal(wrap(err))
	// prepare channels
	requests := make(chan CStringMap, 2)
	inputs := make(chan CStringMap, 2)
	entries := make(chan CStringMap, 2)
	// spin up workers and use a sync.WaitGroup to indicate completion
	var wg sync.WaitGroup
	var script string
	// get commands
	wg.Add(1)
	// go prepareLaunch(entries, output, &wg)
	go func() {
		defer wg.Done()
		for obj := range entries {
			job := NewProcJob(&obj)
			nonfatal(job.PrepareLaunch())
			output <- *job
		}
		close(output)
	}()
	// get entries
	wg.Add(1)
	script = `include "common"; get_entries`
	go NewCacheRequest(script, []string{}).GetMapFromMap(inputs, entries, &wg)
	// format input
	wg.Add(1)
	script = `{ "request": ., "entries": { "cred": [] }, "commands": [] }`
	go NewRequest(script, []string{}).GetMapFromMap(requests, inputs, &wg)
	// send input
	input := NewRunInput(shell, command)
	requests <- input.GetStringMap()
	close(requests)
	// wait on the workers to finish
	wg.Wait()
	return
}

func showCommand(shell string, args []string) (result []string, err error) {
	// prepare channels
	jobs := make(chan ProcJob, 2)
	// spin up workers and use a sync.WaitGroup to indicate completion
	var wg sync.WaitGroup
	// get result
	wg.Add(1)
	result = append(result, `#!`+ shell)
	go func () {
		defer wg.Done()
		for job := range jobs {
			lines, err := job.Show()
			fatal(err)
			result = append(result, lines...)
		}
	}()
	// get commands
	wg.Add(1)
	go prepareRunJob(shell, args, jobs, &wg)
	// wait on the workers to finish
	wg.Wait()
	return
}

// Run

func prettyJson(v interface{}) (result string) {
	// pretty print json
	pretty, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		warn(err)
	} else {
		result = fmt.Sprintln(string(pretty))
	}
	return
}

func doFork() (err error) {
	// usage
	// err = doFork()
	// fatal(err)
	// // now, we are child
	// pid = os.Getpid()
	ret, _, errNo := unix.RawSyscall(unix.SYS_FORK, 0, 0, 0)
	if errNo != 0 {
		err = fmt.Errorf("%w: fork failed err: %d", ErrFailure, errNo)
		return
	}
	switch ret {
	case 0:
		break
	default:
		// parent
		os.Exit(0)
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
	jobs := make(chan ProcJob, 2)
	// spin up workers and use a sync.WaitGroup to indicate completion
	var wg sync.WaitGroup
	// get result
	wg.Add(1)
	go func () {
		defer wg.Done()
		for job := range jobs {
			if err := job.Run(tag, args[1:], stdin, stdout, stderr); err != nil {
				fatal(err)
			}
		}
	}()
	// get commands
	wg.Add(1)
	go prepareRunJob("/bin/sh", args, jobs, &wg)
	// wait on the workers to finish
	wg.Wait()
	return nil
}

// Manage command

func prepareAdjust(input <-chan CStringMap, output chan<- ProcGroupJob, wg *sync.WaitGroup) {
	defer wg.Done()
	for obj := range input {
		job := NewProcGroupJob(&obj)
		nonfatal(job.PrepareAdjust())
		output <- *job
	}
	close(output)
}

func manageCommand(tag string, filter process.Filter, stdout, stderr io.Writer) (err error) {
	// get cache content, once for both go routines
	cacheContent, err = ReadCache()
	fatal(wrap(err))
	// prepare channels
	runjobs := make(chan ProcGroupJob, 8)
	jobs := make(chan CStringMap, 8)
	maps := make(chan CSlice, 8)
	inputs := make(chan CSlice, 8)
	// spin up workers and use a sync.WaitGroup to indicate completion
	var wg sync.WaitGroup
	count := runtime.GOMAXPROCS(0)
	// running jobs
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(input <-chan ProcGroupJob, wg *sync.WaitGroup, tag string, stdout, stderr io.Writer) {
			defer wg.Done()
			for job := range input {
				job.Run(tag, stdout, stderr)
			}
		}(runjobs, &wg, tag, stdout, stderr)
	}
	// prepare process group jobs
	wg.Add(1)
	go prepareAdjust(jobs, runjobs, &wg)
	// filter process group jobs
	wg.Add(1)
	go NewCacheRequest(
		`include "manage"; get_process_group_job`,
		[]string{"$nproc", "$max_nice", "$uid", "$shell"},
		numCPU(),
		int(rlimitNice().Max),
		viper.GetInt("UID"),
		"/bin/sh",
	).GetMapFromSlice(maps, jobs, &wg)
	// preparing input
	wg.Add(1)
	go NewCacheRequest(
		`include "common"; .[] | select(.comm | within(rule_names))`,
		[]string{},
	).GetSliceFromSlice(inputs, maps, &wg)
	inputs <- NewManageInput(filter).GetSlice()
	close(inputs)
	// wait on the workers to finish
	wg.Wait()
	return err
}

func getManageInput(filter process.Filter) CSlice {
	return NewManageInput(filter).GetSlice()
}

// TODO: Install command

// vim: set ft=go fdm=indent ts=2 sw=2 tw=79 noet:
