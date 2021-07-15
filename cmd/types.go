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
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"reflect"
	"runtime"
	"golang.org/x/sys/unix"
	"github.com/canalguada/nicy/process"
	"github.com/google/shlex"
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

// TODO: Move all syscall code into other package
func Getrlimit_Nice() (*unix.Rlimit, error) {
	var rLimit unix.Rlimit
	if err := unix.Getrlimit(unix.RLIMIT_NICE, &rLimit); err != nil {
		return nil, wrapError(err)
	}
	return &rLimit, nil
}

func NewJqInput(command string) *JqInput {
	rLimit, err := Getrlimit_Nice()
	if err != nil {
		rLimit = &unix.Rlimit{20, 20}
	}
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
		NumCPU: runtime.NumCPU(),
		MaxNice: int(rLimit.Max),
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

type CmdLine []string

func NewCommandLine(line string) CmdLine {
	// shlex.Split removes comments and shebang lines
	args, err := shlex.Split(line)
	checkErr(err)
	return CmdLine(args)
}

func (line CmdLine) Runtime(pid, uid int) CmdLine {
	runtime := line[:]
	for i, arg := range runtime {
		switch {
		case arg == "${user_or_system}":
			if uid != 0 {
				runtime[i] = "--user"
			} else {
				runtime[i] = ""
			}
		case strings.Contains(arg, "$$"):
			runtime[i] = strings.Replace(arg, "$$", strconv.Itoa(pid), 1)
		}
	}
	return CmdLine(runtime)
}

func (line *CmdLine) Append(args ...string) {
	slice := *line
	slice = append(slice, args...)
	*line = slice
}

func (line *CmdLine) ShrinkLeft(count uint) {
	if int(count) >= (len(*line) - 1) {
		*line = CmdLine{}
	} else {
		slice := *line
		slice = slice[count:]
		*line = slice
	}
}

func (line *CmdLine) ShrinkRight(count uint) {
	if int(count) >= (len(*line) - 1) {
		*line = CmdLine{}
	} else {
		slice := *line
		slice = slice[:len(*line) - int(count)]
		*line = slice
	}
}

func (line CmdLine) String() string {
	return strings.Join(line, " ")
}

func (line CmdLine) Output() ([]string, error) {
	out, err := exec.Command(line[0], line[1:]...).Output()
	if err != nil {
		return []string{}, wrapError(err)
	}
	return strings.Split(string(out), "\n"), nil
}

func (line CmdLine) Run(stdin io.Reader, stdout, stderr io.Writer) error {
	args := line[:]
	if line[len(line) - 1] == ">/dev/null" {
		stdout = nil
		args = line[:len(line) - 1]
	}
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Wait()
}

// func runCommand(args []string, std []*os.File) error {
//   cmd := exec.Command(args[0], args[1:]...)
//   cmd.Stdin = std[0]
//   cmd.Stdout = std[1]
//   cmd.Stderr = std[2]
//   if err := cmd.Start(); err != nil {
//     return err
//   }
//   return cmd.Wait()
// }

func (line CmdLine) ExecRun() error {
	pos := 0
	if line[pos] == "exec" {
		pos++
	}
	command, err := lookPath(line[pos])
	if err != nil {
		return err
	}
	args := []string{filepath.Base(command)}
	args = append(args, line[pos + 1:]...)
	return unix.Exec(command, args, os.Environ())
}

func (line CmdLine) WriteTo(dest io.Writer) (n int, err error) {
	n, err = fmt.Fprintf(dest, line.String() + "\n")
	return
}

func (line CmdLine) WriteVerboseTo(dest io.Writer) (n int, err error) {
	n, err = fmt.Fprintf(dest, "echo %s: %s: %s\n", prog, "run", line.String())
	return
}

type Script []CmdLine

func NewScript(shell string, lines ...CmdLine) Script {
	result := make([]CmdLine, len(lines) + 1)
	result[0] = CmdLine{"#!" + shell}
	for i, line := range lines {
		if len(line) > 0 {
			result[i + 1] = line
			if line[0] == "exec" {
				result[i + 1].Append(`"$@"`)
			}
		}
	}
	return Script(result)
}

func (script Script) RuntimeCommands(args []string) (result []CmdLine) {
	// Remove sheban line from loop
	for _, line := range (([]CmdLine)(script))[1:] {
		if strings.HasPrefix(line[0], "[") {
			continue
		}
		if line[0] == "exec" {
			line.ShrinkRight(1)
			line.Append(args[1:]...)
		}
		result = append(result, line.Runtime(os.Getpid(), viper.GetInt("UID")))
	}
	return
}

func (script Script) Run(args []string) {
	tmp, err := getTempFile(
		viper.GetString("runtimedir"),
		viper.GetString("PROG") + "_run_" + filepath.Base(args[0]) + "-*",
	)
	checkErr(err)
	name := tmp.Name()
	tmp.WriteString(fmt.Sprintf("#!%s\n", viper.GetString("shell")))
	tmp.WriteString(fmt.Sprintf("rm -f %s\n", name))
	if viper.GetBool("debug") {
		printErrf("Writing %q script... ", name)
	}
	for _, line := range script {
		if len(line) > 0 {
			if line[0] == "exec" {
				line.Append(args...)
			}
			if strings.HasPrefix(line[0], "[") {
				line.WriteTo(tmp)
			} else if viper.GetBool("verbose") {
				line.WriteVerboseTo(tmp)
			} else {
				line.WriteTo(tmp)
			}
		}
	}
	if viper.GetBool("debug") {
		printErrln("Done.")
	}
	err = tmp.Close()
	checkErr(err)
	err = os.Chmod(name, 0755)
	checkErr(err)

	if viper.GetBool("debug") {
		printErrf("Running %q script... \n", name)
	}
	shell := viper.GetString("shell")
	err = unix.Exec(
		shell,
		[]string{filepath.Base(shell), "-c", name},
		os.Environ(),
	)
	checkErr(err)
}

// vim: set ft=go fdm=indent ts=2 sw=2 tw=79 noet:
