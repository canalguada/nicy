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
	"github.com/canalguada/nicy/process"
	"github.com/canalguada/nicy/jq"
	"github.com/spf13/viper"
)

// Build command

// readDirNames reads the directory named by dirname and returns a sorted list
// of directory entries.
func readDirNames(dirname string) ([]string, error) {
	f, err := os.Open(dirname)
	if err != nil {
		return nil, wrapError(err)
	}
	names, err := f.Readdirnames(-1)
	f.Close()
	if err != nil {
		return nil, wrapError(err)
	}
	sort.Strings(names)
	return names, nil
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
				printErrf("failure accessing %q: %v\n", path, err)
				return wrapError(err)
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
				return fmt.Errorf("%w: bad pattern %+v", ErrInvalid, pattern)
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

func exists(path string) bool {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false
	}
	return true
}

func timestamp() string {
	return fmt.Sprintf("%v", time.Now().Unix())
}

func readFile(path string) ([]byte, error) {
	out, err := ioutil.ReadFile(path)
	return out, wrapError(err)
}

func getTempFile(dest, pattern string) (*os.File, error) {
	file, err := ioutil.TempFile(dest, pattern)
	if err != nil {
		return nil, wrapError(err)
	}
	return file, nil
}

func writeTo(path string, buf []byte) (n int, err error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		err = wrapError(err)
		return
	}
	defer f.Close()
	n, err = f.Write(buf)
	if err != nil {
		err = wrapError(err)
	}
	return
}

func appendTo(path string, buf []byte) (n int, err error) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		err = wrapError(err)
		return
	}
	defer f.Close()
	n, err = f.Write(buf)
	if err != nil {
		err = wrapError(err)
	}
	return
}

type UnmarshalFunc func(s string) (map[string]interface{}, error)

func dumpContent(path string, fn UnmarshalFunc) (content []interface{}, err error) {
	re := regexp.MustCompile(`^[ ]*#`)
	file, err := os.Open(path)
	if err != nil {
		err = wrapError(err)
		return
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	// optionally, resize scanner's capacity for content over 64K
	for scanner.Scan() {
		bufline := scanner.Bytes()
		if len(bufline) != 0 && !(re.Match(bufline)) {
			item, e := fn(scanner.Text())
			if e != nil {
				err = wrapError(e)
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

func dumpObjects(category, root string) (objects []interface{}, err error) {
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
		// map[string]interface{}
		lines, e := dumpContent(
			file,
			func(s string) (map[string]interface{}, error) {
				var result map[string]interface{}
				if err := json.Unmarshal([]byte(s), &result); err != nil {
					return nil, wrapError(err)
				}
				result["origin"] = root
				return result, nil
			},
		)
		objects = append(objects, lines...)
		if e != nil {
			err = e // Yet wrapped error
			return
		}
	}
	return objects, nil
}

func readCache() (cache map[string]interface{}, err error) {
	filename := viper.GetString("database")
	data, err := readFile(filename)
	if err != nil {
		return
	}
	if err = json.Unmarshal(data, &cache); err != nil {
		err = wrapError(err)
	}
	return
}

func readCategoryCache(category string) (cache []interface{}, err error) {
	filename := viper.GetString(category)
	data, err := readFile(filename)
	if err != nil {
		return
	}
	if err = json.Unmarshal(data, &cache); err != nil {
		err = wrapError(err)
	}
	return
}

// List command

func listObjects(category string) (output []interface{}, err error) {
	var input []interface{}
	// Prepare input
	if viper.IsSet("from") {
		input = []interface{}{viper.Get("from")}
	} else {
		slice := viper.GetStringSlice("confdirs")
		input = make([]interface{}, len(slice))
		for i, path := range slice {
			input[i] = expandPath(path)
		}
	}
	// Prepare variables
	cachedb, err := readCache()
	checkErr(err)

	req := jq.NewRequest(
		`include "list"; list`,
		[]string{"$cachedb", "$kind"},
		cachedb,
		strings.TrimRight(category, "s"),
	)
	req.LibDirs = []string{filepath.Join(expandPath(viper.GetString("libdir")), "jq")}
	output, err = req.Result(input)
	return
}

// Run and show commands

func lookPath(command string) (result string, err error) {
	result, err = exec.LookPath(command)
	if err != nil {
		err = wrapError(err)
	}
	return
}

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
		e := findExecutable(file)
		if e == nil {
			result = append(result, file)
			return
		}
		err = wrapError(e)
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

func yieldScriptFrom(shell string, args []string) (Script, error) {
	// Prepare variables
	cachedb, err := readCache()
	if err != nil {
		return nil, err
	}
	req := jq.NewRequest(
		`include "run"; run`,
		[]string{"$cachedb"},
		cachedb)
	req.LibDirs = []string{filepath.Join(expandPath(viper.GetString("libdir")), "jq")}
	// Prepare input
	command, err := findValidPath(args[0])
	if err != nil {
		return nil, err
	}
	input := NewRunInput(command)
	input.Shell = filepath.Base(shell)
	// Get output
	output, err := req.Result(input.Map())
	switch {
	case err != nil:
		return nil, err
	case output[0] != "commands":
		return nil, fmt.Errorf("%w: %v", ErrInvalid, output[1:])
	}
	output = output[1:]
	script, err := DecodeScript(output)
	checkErr(err)
	return NewShellScript(shell, script...), nil
}

// Manage command

func streamProcAdjust(filterFunc process.Filter) (objects []interface{}, err error) {
	// Prepare variables
	cachedb, err := readCache()
	checkErr(err)
	req := jq.NewRequest(
		`include "manage"; manage_runtime`,
		[]string{"$cachedb", "$nproc", "$max_nice", "$uid", "$shell"},
		cachedb,
		numCPU(),
		int(rlimitNice().Max),
		viper.GetInt("UID"),
		"/bin/sh",
	)
	req.LibDirs = []string{filepath.Join(expandPath(viper.GetString("libdir")), "jq")}
	// Prepare input
	input := ManageInput{}
	for _, p :=range process.AllProcs(filterFunc) {
		input.Append(&p)
	}
	// Get result
	objects, err = req.Result(input.Slice())
	if err != nil {
		err = fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	return
}

func runProcAdjust(objects []interface{}, stdout, stderr io.Writer) {
	// make our channel for communicating work
	jobs := make(chan ManageJob, len(objects))
	// spin up workers and use a sync.WaitGroup to indicate completion
	var count = runtime.GOMAXPROCS(0)
	var wg sync.WaitGroup
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(jobs <-chan ManageJob, wg *sync.WaitGroup){
			defer wg.Done()
			// Do work on job here
			for job := range jobs {
				job.Run(stdout, stderr)
			}
		}(jobs, &wg)
	}
	// start sending jobs
	go func() {
		defer close(jobs)
		for _, json := range objects {
			obj, ok := json.(map[string]interface{})
			if !(ok) {
				// Skip if not valid object
				fmt.Fprintf(stderr, "not a valid object: %#v\n", json)
				continue
			}
			// TODO: Remove after debug
			// fmt.Fprintf(stderr, "%#v\n", obj)
			// Extract job per process group
			jobs <- *NewManageJob(&obj)
		}
	}()
	// wait on the workers to finish
	wg.Wait()
}

// TODO: Install command

// vim: set ft=go fdm=indent ts=2 sw=2 tw=79 noet:
