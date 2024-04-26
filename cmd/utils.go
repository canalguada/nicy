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
	reNoComment, reIsShell *regexp.Regexp
	goMaxProcs             = runtime.GOMAXPROCS(0)
	numCPU                 = runtime.NumCPU()
	maxNice                = int(rlimitNice().Max)
)

func init() {
	reNoComment = regexp.MustCompile(`^[ ]*#`)
	reIsShell = regexp.MustCompile(`^(/usr/bin/|/bin)??(sh|dash|bash|zsh)$`)
}

func getWaitGroup() (wg sync.WaitGroup) {
	return
}

func ValidShell(shell string) (string, error) {
	switch {
	case !reIsShell.MatchString(shell):
		return shell, fmt.Errorf("%w: shell: %q", ErrInvalid, shell)
	default:
		if path, err := exec.LookPath(shell); err == nil {
			return path, nil
		} else {
			return shell, fmt.Errorf("%w: shell: %q", err, shell)
		}
	}
}

// Build command

func timestamp() string {
	return fmt.Sprintf("%v", time.Now().Unix())
}

// func getTempFile(dest, pattern string) (*os.File, error) {
// 	return os.CreateTemp(dest, pattern)
// }

// Run and show commands

type BaseRequest struct {
	Name    string `json:"name"`
	Path    string `json:"cmd"`
	Preset  string `json:"preset"`
	Shell   string `json:"shell"`
	NumCPU  int    `json:"nproc"`
	MaxNice int    `json:"max_nice"`
}

func NewBaseRequest(name, path, shell string) *BaseRequest {
	return &BaseRequest{
		Name:    name,
		Path:    path,
		Preset:  "auto",
		Shell:   shell,
		NumCPU:  numCPU,
		MaxNice: maxNice,
	}
}

type Request struct {
	*BaseRequest
	*Proc
	CgroupKey   string `json:"cgroup"`
	ForceCgroup bool   `json:"probe_cgroup"`
	Managed     bool   `json:"managed"`
	Quiet       bool   `json:"quiet"`
	Verbosity   int    `json:"verbosity"`
}

func (r *Request) MergeFlags() {
	r.Preset = viper.GetString("preset")
	r.CgroupKey = viper.GetString("cgroup")
	r.ForceCgroup = viper.GetBool("force-cgroup")
	r.Managed = viper.GetBool("managed")
	r.Quiet = viper.GetBool("quiet")
}

func NewRequest(name, path, shell string) *Request {
	return &Request{BaseRequest: NewBaseRequest(name, path, shell), Proc: &Proc{}}
}

func NewPathRequest(path, shell string) *Request {
	r := NewRequest(filepath.Base(path), path, shell)
	r.Verbosity = 1
	r.MergeFlags()
	return r
}

func removeFromPath(root string) error {
	dirs := filepath.SplitList(os.Getenv("PATH"))
	dirs = Filter(dirs, func(path string) bool {
		return !strings.HasPrefix(path, root)
	})
	return os.Setenv("PATH", strings.Join(dirs, ":"))
}

func prependToPath(root string) error {
	return os.Setenv("PATH", root+":"+os.Getenv("PATH"))
}

// LookAll searches for all executable named files in the
// directories named by the PATH environment variable.
// If file contains a slash, it is tried directly and the PATH is not consulted.
func LookAll(file string) chan string {
	ch := make(chan string)
	if strings.Contains(file, string(filepath.Separator)) {
		go func(ch chan string) {
			if path, e := exec.LookPath(file); e == nil {
				// path, _ = filepath.EvalSymlinks(path)
				ch <- path
			}
			close(ch)
		}(ch)
	} else {
		go func(ch chan string) {
			for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
				if dir == "" {
					dir = "." // Unix shell semantics: path element "" means "."
				}
				if path, e := exec.LookPath(filepath.Join(dir, file)); e == nil {
					// path, _ = filepath.EvalSymlinks(path)
					ch <- path
				}
			}
			close(ch)
		}(ch)
	}
	return ch
}

// LookPath looks for a valid executable file outside scripts location.
func LookPath(file string) string {
	return ChanFirst(LookAll(file), func(path string) bool {
		return !strings.HasPrefix(path, viper.GetString("scripts.location")) &&
			!strings.HasSuffix(path, "."+prog)
	})
}

// vim: set ft=go fdm=indent ts=2 sw=2 tw=79 noet:
