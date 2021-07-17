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
	"io/ioutil"
	"path/filepath"
	"strings"
	"sort"
	"encoding/json"
	"time"
	"github.com/canalguada/nicy/jq"
	"github.com/spf13/viper"
)

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
	if strings.Contains(file, "/") {
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
	basename := filepath.Base(command)
  // Strip suffix, if any
	basename = strings.TrimSuffix(basename, ".nicy")
  // Set an absolute path for the final command
	prefix := expandPath(viper.GetString("scripts"))
	which, err := lookPath(basename)
	if err != nil {
		return
	}
	which, _ = filepath.EvalSymlinks(which)
	if command == basename || strings.HasPrefix(which, prefix) {
		var paths []string
		// Not a valid absolute path
		paths, err = lookAll(command)
		if err != nil {
			return
		}
		for _, path := range paths {
			if strings.HasPrefix(path, prefix) {
				continue
			}
			valid = path
			return
		}
	}
	valid = command
	return
}

func yieldScriptFrom(shell string, args []string) (Script, error) {
	cachedb, err := readCache()
	if err != nil {
		return nil, err
	}
	req := jq.NewRequest(`include "run"; run`, []string{"$cachedb"}, cachedb)
	req.LibDirs = []string{filepath.Join(expandPath(viper.GetString("libdir")), "jq")}
	// Prepare input
	command, err := findValidPath(args[0])
	if err != nil {
		return nil, err
	}
	input := NewJqInput(command)
	input.Shell = filepath.Base(shell)
	// Get result
	result, err := req.Output(input.Slice())
	switch {
	case err != nil:
		return nil, err
	case result[0] != "commands":
		return nil, fmt.Errorf("%w: %v", ErrInvalid, result[1:])
	}
	lines := result[1:]
	script := make([]CmdLine, len(lines))
	for i, data := range lines {
		s, ok := data.(string)
		if !(ok) {
			checkErr(fmt.Errorf("%w: not a string: %v", ErrInvalid, data))
		}
		script[i] = NewCommandLine(s)
	}
	return NewScript(shell, script...), nil
}

// vim: set ft=go fdm=indent ts=2 sw=2 tw=79 noet:
