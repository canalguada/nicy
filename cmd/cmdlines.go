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
	"golang.org/x/sys/unix"
	"github.com/google/shlex"
	"github.com/spf13/viper"
)


type CmdLine []string

func DecodeCmdLine(input interface{}) (cmdline CmdLine, err error) {
	if line, ok := input.([]interface{}); ok {
		for _, s := range line {
			if token, ok := s.(string); ok {
				if len(token) > 0 {
					cmdline = append(cmdline, token)
				}
			} else {
				err = fmt.Errorf("%w: not a string: %v", ErrInvalid, s)
				break
			}
		}
	} else {
		err = fmt.Errorf("%w: can't decode %#v into CmdLine", ErrInvalid, input)
	}
	return
}

func NewCmdLine(line string) CmdLine {
	// shlex.Split removes comments and shebang lines
	args, err := shlex.Split(line)
	checkErr(err)
	return CmdLine(args)
}

func (line CmdLine) RequireSysNice() bool {
	if line[0] == "$SUDO" {
		for _, c := range []string{"renice", "chrt", "ionice"} {
				if line[1] == c {
					return true
				}
		}
	}
	return false
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

// func (line *CmdLine) ShrinkLeft(count uint) {
//   if int(count) >= (len(*line) - 1) {
//     *line = CmdLine{}
//   } else {
//     slice := *line
//     slice = slice[count:]
//     *line = slice
//   }
// }

// func (line *CmdLine) ShrinkRight(count uint) {
//   if int(count) >= (len(*line) - 1) {
//     *line = CmdLine{}
//   } else {
//     slice := *line
//     slice = slice[:len(*line) - int(count)]
//     *line = slice
//   }
// }

func (line CmdLine) String() string {
	return strings.TrimSpace(strings.Join(line, " "))
}

func (line CmdLine) Output() (output []string, err error) {
	data, err := exec.Command(line[0], line[1:]...).Output()
	if err != nil {
		err = wrapError(err)
	} else {
		output = strings.Split(string(data), "\n")
	}
	return
}

func (line CmdLine) UnprivilegedRun(stdin io.Reader, stdout, stderr io.Writer) error {
	args := CmdLine(line[:])
	if args[len(args) - 1] == ">/dev/null" {
		stdout = nil
		args = args[:len(args) - 1]
	}
	if viper.GetBool("dry-run") {
		// Write to stderr what would be run
		fmt.Fprintln(
			stderr,
			prog + ":",
			viper.GetString("tag") + ":",
			"dry-run:",
			args,
		)
	} else if viper.GetInt("verbose") > 0 {
		// Write to stderr what would be run
		fmt.Fprintln(
			stderr,
			prog + ":",
			viper.GetString("tag") + ":",
			args,
		)
	}
	if viper.GetBool("dry-run") {
		return nil
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

func (line CmdLine) ExecRun() error {
	pos := 0
	if line[pos] == "exec" {
		pos++
	}
	command, err := lookPath(line[pos])
	if err != nil {
		return err
	}
	if viper.GetBool("dry-run") {
		// Write to stderr what would be run
		fmt.Fprintln(
			os.Stderr,
			prog + ":",
			viper.GetString("tag") + ":",
			"dry-run:",
			command,
			line[pos + 1:],
		)
	} else if viper.GetInt("verbose") > 0 {
		// Write to stderr what would be run
		fmt.Fprintln(
			os.Stderr,
			prog + ":",
			viper.GetString("tag") + ":",
			command,
			line[pos + 1:],
		)
	}
	if viper.GetBool("dry-run") {
		return nil
	}
	args := append([]string{filepath.Base(command)}, line[pos + 1:]...)
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

func (line CmdLine) Run(stdin io.Reader, stdout, stderr io.Writer) error {
	disableAmbient := false
	defer func() {
		if disableAmbient {
			if err := setAmbientSysNice(false); err != nil {
				fmt.Fprintln(stderr, err)
			}
		}
	}()
	for ; line[0] == "$SUDO" ; {
		if viper.GetInt("UID") != 0 {
			if line.RequireSysNice() {
				if err := setAmbientSysNice(true); err == nil {
					disableAmbient = true
					// Discard `$SUDO` if CAP_SYS_NICE in local ambient set
					goto Discard
				} else {
					fmt.Fprintln(stderr, err)
				}
			}
		} else {
			// Discard `$SUDO` for superuser
			goto Discard
		}
		// Fallback to sudo
		line[0] = viper.GetString("sudo")
		break
		Discard:
			line = line[1:]
	}
	if line[0] == "exec" {
		// Do not propagate capabilities
		if err := setCapabilities(false); err != nil {
			fmt.Fprintln(stderr, err)
		}
		return line.ExecRun()
	}
	return line.UnprivilegedRun(stdin, stdout, stderr)
}

func (line CmdLine) Trace(tag string) {
	if viper.GetBool("debug") {
		fmt.Fprintf(os.Stderr, "%s [0]:\t%#v\t\t[1:]:\t%#v\n", tag, line[0], line[1:])
	}
}

func (line CmdLine) Trim() {
	for ; len(line) > 0 && len(line[0]) == 0 ; {
		line = line[1:]
	}
}

func (line CmdLine) Command() (command string) {
	if len(line) > 0 {
		command = line[0]
	}
	return
}

func (line CmdLine) Args() (args []string) {
	if len(line) > 1 {
		args = line[1:]
	}
	return
}


type Script []CmdLine

func (script *Script) Append(lines ...CmdLine) {
	slice := *script
	slice = append(slice, lines...)
	*script = slice
}

func DecodeScript(input interface{}) (script Script, err error) {
	if content, ok := input.([]interface{}); ok {
		// Iterate over interface{} content
		for _, line := range content {
			// Trying to decode a CmdLine
			if cmdline, err := DecodeCmdLine(line); err == nil {
				script.Append(cmdline)
			} else {
				break
			}
		}
	} else {
		err = fmt.Errorf("%w: can't decode %#v into Script", ErrInvalid, input)
	}
	return
}

func NewShellScript(shell string, lines ...CmdLine) (script Script) {
	// result := make([]CmdLine, len(lines) + 1)
	// result[0] = CmdLine{"#!" + shell}
	// for i, line := range lines {
	//   if len(line) > 0 {
	//     result[i + 1] = line
	//     if line[0] == "exec" {
	//       result[i + 1].Append(`"$@"`)
	//     }
	//   }
	// }
	// return Script(result)
	script.Append(CmdLine{"#!" + shell})
	for _, line := range lines {
		line.Trim()
		if len(line) > 0 {
			if line[0] == "exec" {
				// Append first "$@"
				line.Append(`"$@"`)
			}
			script.Append(line)
		}
	}
	return
}

func (script Script) RunCmdLines(args []string) (result []CmdLine) {
	// Remove tests and shebang line from loop
	for _, line := range []CmdLine(script) {
		if strings.HasPrefix(line[0], "[") || strings.HasPrefix(line[0], "#!") {
			continue
		}
		if line[0] == "exec" {
			// Replace `"$@"` with runtime args
			line = line[:len(line) - 1]
			line.Append(args[1:]...)
		}
		result = append(result, line.Runtime(os.Getpid(), viper.GetInt("UID")))
	}
	return
}

func (script Script) ManageCmdLines() (result []CmdLine) {
	// Remove tests from loop
	for _, line := range []CmdLine(script) {
		if strings.HasPrefix(line[0], "[") {
			continue
		}
		result = append(result, line)
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
