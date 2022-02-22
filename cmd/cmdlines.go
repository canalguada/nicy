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
	"bytes"
	"strings"
	"golang.org/x/sys/unix"
	"github.com/kballard/go-shellquote"
	"github.com/spf13/viper"
)

func ShellJoin(words ...string) string {
	return shellquote.Join(words...)
}

func ShellSplit(input string) (words []string, err error) {
	return shellquote.Split(input)
}

func ShellQuote(words []string) (result []string) {
	for _, word := range words {
		// Bail out early for "simple" strings
		if word != "" && !strings.ContainsAny(word, "\\'\"`${[|&;<>()~*?! \t\n") {
			result = append(result, word)
		} else {
			var buf bytes.Buffer
			buf.WriteString("'")
			for i := 0; i < len(word); i++ {
				b := word[i]
				if b == '\'' {
					// Replace literal ' with a close ', a \', and a open '
					buf.WriteString("'\\''")
				} else {
					buf.WriteByte(b)
				}
			}
			buf.WriteString("'")
			result = append(result, buf.String())
		}
	}
	return
}

type Words []string

func (self Words) Copy() Words {
	buf := make(Words, len(self))
	_ = copy(buf, self)
	return buf
}

func (self *Words) Append(args ...string) {
	*self = append(*self, args...)
}

func (self *Words) Extend(other Words) {
	*self = append(*self, other...)
}

func (self Words) Index(pos int) (result string, err error) {
	if pos < 0 || pos > (len(self) - 1) {
		err = fmt.Errorf("%w: not a valid index: %d", ErrInvalid, pos)
	} else {
		result = self[pos]
	}
	return
}

func (self Words) IsEmpty() bool {
	return len(self) == 0
}

func (self Words) Line() string {
	return strings.TrimSpace(strings.Join(self, ` `))
}

func (self Words) String() string {
	return `[`+ self.Line() + `]`
}

func (self Words) WriteTo(dest io.Writer) (n int, err error) {
	n, err = fmt.Fprintf(dest, self.Line() + "\n")
	return
}

func (self Words) WriteVerboseTo(dest io.Writer) (n int, err error) {
	n, err = fmt.Fprintf(dest, "echo %s: %s: %s\n", prog, "run", self.Line())
	return
}

type WordFilter func(word string) bool

var Empty WordFilter = func(word string) bool {
	return len(word) == 0
}

func (self *Words) Filter(filter WordFilter) {
	var buf Words
	for _, word := range *self {
		if !(filter(word)) {
			buf = append(buf, word)
		}
	}
	*self = buf
}

type CmdLine struct {
	words Words
	skipWhenRun bool
}

func NewCmdLine(s ...string) CmdLine {
	return CmdLine{words: Words(s)}
}

func (line CmdLine) Copy() CmdLine {
	return CmdLine{words: line.words.Copy(), skipWhenRun: line.skipWhenRun}
}

func (line *CmdLine) Append(words ...string) {
	line.words.Append(words...)
}

func (line *CmdLine) Extend(other CmdLine) {
	line.words.Extend(other.words)
}

func (line CmdLine) Index(pos int) (result string) {
	word, err := line.words.Index(pos)
	if err != nil {
		fatal(err)
	} else {
		result = word
	}
	return
}

func (line CmdLine) IsEmpty() bool {
	return line.words.IsEmpty()
}

func DecodeCmdLine(input interface{}) (result CmdLine, err error) {
	if line, ok := input.([]interface{}); ok {
		for _, s := range line {
			if word, ok := s.(string); ok {
				if len(word) > 0 {
					result.Append(word)
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

func CmdLineFromString(line string) CmdLine {
	words, err := ShellSplit(line)
	fatal(wrap(err))
	return NewCmdLine(words...)
}

func (line CmdLine) ShellLine() string {
	// words are yet shell-quoted when required
	return line.words.Line()
}

func (line CmdLine) String() string {
	return line.words.String()
}

func (line CmdLine) WriteTo(dest io.Writer) (int, error) {
	return line.words.WriteTo(dest)
}

func (line CmdLine) WriteVerboseTo(dest io.Writer) (int, error) {
	return line.words.WriteVerboseTo(dest)
}

func (line CmdLine) RequireSysCapability(uid int) (comm string, flag bool) {
	var pos int
	if line.Index(0) == "$SUDO" {
		pos = 1
	}
	// Set flag for given system utilies
	comm = filepath.Base(line.Index(pos))
	switch comm {
	case "renice", "chrt", "ionice", "choom":
		flag = true
	}
	return
}

func (line CmdLine) Runtime(pid, uid int) CmdLine {
	words := line.words.Copy()
	for i, word := range words {
		switch {
		case word == "${user_or_system}":
			if uid != 0 {
				words[i] = "--user"
			} else {
				words[i] = "--system"
			}
		case strings.Contains(word, "$$"):
			words[i] = strings.Replace(word, "$$", strconv.Itoa(pid), 1)
		}
	}
	return NewCmdLine(words...)
}

func (line CmdLine) Output() (output []string, err error) {
	data, err := exec.Command(line.words[0], line.words[1:]...).Output()
	if err != nil {
		return
	}
	output = strings.Split(string(data), "\n")
	return
}

type Streams struct {
	Stdin io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

func (std *Streams) Scan(line CmdLine) (result Words) {
	for _, arg := range line.words {
		switch {
		case strings.HasPrefix(arg, ">") || strings.HasPrefix(arg, "1>"):
			name := strings.TrimPrefix(arg, "1>")
			name = strings.TrimPrefix(arg, ">")
			name = strings.TrimSpace(name)
			if name == "/dev/null" {
				std.Stdout = nil
			} else if name == "&2" {
				std.Stdout = std.Stderr
			} else {
				warn(fmt.Errorf("%w: invalid redirection: %v", ErrInvalid, arg))
				continue
			}
		case strings.HasPrefix(arg, "2>"):
			name := strings.TrimPrefix(arg, "2>")
			name = strings.TrimSpace(name)
			if name == "/dev/null" {
				std.Stderr = nil
			} else if name == "&2" {
				std.Stderr = std.Stdout
			} else {
				warn(fmt.Errorf("%w: invalid redirection: %v", ErrInvalid, arg))
				continue
			}
		default:
			result.Append(arg)
		}
	}
	return
}

func (line CmdLine) getCmd(tag string, std *Streams) (cmd *exec.Cmd, err error) {
	words := std.Scan(line)
	command, err := exec.LookPath(words[0])
	if err != nil {
		return
	}
	cmd = exec.Command(command, words[1:]...)
	// Connect input/output
	cmd.Stdin = std.Stdin
	cmd.Stdout = std.Stdout
	cmd.Stderr = std.Stderr
	return
}

func whenVerbose(tag, path string, args ...string) {
	if viper.GetBool("dry-run") || viper.GetBool("verbose") {
		words := append([]string{path}, args...)
		doVerbose(tag, words...)
	}
}

func doVerbose(tag string, args ...string) {
	var s = []string{viper.GetString("tag") + ":"}
	if viper.GetBool("dry-run") {
		s = append(s, "dry-run:")
	}
	if len(tag) > 0 {
		s = append(s, tag + ":")
	}
	if len(args) > 0 {
		s = append(s, args...)
	}
	inform(strings.Join(s, ` `))
}

func (line *CmdLine) preRun() (err error) {
	discardSudo := (viper.GetInt("UID") == 0) || viper.GetBool("ambient")
	words := line.words.Copy()
	if words[0] == "exec" {
		words = words[1:]
	}
	if words[0] == "$SUDO" {
		if discardSudo {
			words = words[1:]
		} else {
			words[0] = viper.GetString("sudo")
		}
	}
	words.Filter(Empty)
	if words.IsEmpty() {
		err = fmt.Errorf("%w: empty command line", ErrInvalid)
	}
	*line = NewCmdLine(words...)
	return
}

func (line CmdLine) Start(tag string, stdin io.Reader, stdout, stderr io.Writer) (cmd *exec.Cmd, err error) {
	// line.Trace("Start")
	err = line.preRun()
	if err != nil {
		return
	}
	cmd, err = line.getCmd(tag, &Streams{stdin, stdout, stderr})
	if err != nil {
		return
	}
	whenVerbose(tag, cmd.Path, cmd.Args[1:]...)
	// Return if dry-run
	if viper.GetBool("dry-run") {
		return
	}
	cmd.SysProcAttr = &unix.SysProcAttr{
		Setpgid: true,
	}
	err = cmd.Start()
	return
}

func (line CmdLine) StartWait(tag string, stdin io.Reader, stdout, stderr io.Writer) error {
	// line.Trace("StartWait")
	cmd, err := line.Start(tag, stdin, stdout, stderr)
	if err != nil {
		return err
	}
	// Return if dry-run
	if viper.GetBool("dry-run") {
		return nil
	}
	return cmd.Wait()
}

func (line CmdLine) Exec(tag string) error {
	// line.Trace("Exec")
	if err := line.preRun(); err != nil {
		return err
	}
	command, err := exec.LookPath(line.words[0])
	if err != nil {
		return err
	}
	whenVerbose(tag, command, line.words[1:]...)
	// Return if dry-run
	if viper.GetBool("dry-run") {
		return nil
	}
	// Do not propagate capabilities
	nonfatal(clearAllCapabilities())
	args := append([]string{filepath.Base(command)}, line.words[1:]...)
	return unix.Exec(command, args, os.Environ())
}

func (line CmdLine) Run(tag string, stdin io.Reader, stdout, stderr io.Writer) error {
	var flagExec bool
	if line.Index(0) == "exec" {
		flagExec = true
	}
	if flagExec {
		return line.Exec(tag)
	}
	return line.StartWait(tag, stdin, stdout, stderr)
}

func (line CmdLine) Trace(tag string) {
	debug(fmt.Sprintf("%s [0]:\t%#v\t\t[1:]:\t%#v\n", tag, line.words[0], line.words[1:]))
}

type Script []CmdLine

func (script *Script) Append(lines ...CmdLine) {
	for _, line := range lines {
		// line.words.Filter(Empty)
		if !(line.IsEmpty()) {
			*script = append(*script, line)
		}
	}
}

func (script *Script) Extend(other Script) {
	*script = append(*script, other...)
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
	script.Append(NewCmdLine("#!" + shell))
	for _, line := range lines {
		line.words.Filter(Empty)
		if !(line.IsEmpty()) {
			if line.Index(0) == "exec" {
				// Append first "$@"
				line.Append(`"$@"`)
			}
			script.Append(line)
		}
	}
	return
}

func (script Script) ShellLines() (result []string) {
	for _, line := range script {
		result = append(result, line.ShellLine())
	}
	return
}

func (script Script) String() (result string) {
	result = `[`
	for _, line := range script {
		result = result + ` ` + line.String()
	}
	result = result + `]`
	return
}

func (script Script) PrepareRun(args []string) (result []CmdLine) {
	for _, line := range script {
		// Remove tests and shebang line from loop
		if line.skipWhenRun {
			continue
		}
		// Append command args, if any, when required
		if line.Index(0) == "exec" && len(args) > 0 {
			line.Append(args...)
		}
		result = append(result, line)
	}
	return
}

func (script Script) Run(args []string) {
	// get temp file
	tmp, err := getTempFile(
		viper.GetString("runtimedir"),
		viper.GetString("PROG") + "_run_" + filepath.Base(args[0]) + "-*",
	)
	fatal(wrap(err))
	// write script
	name := tmp.Name()
	tmp.WriteString(fmt.Sprintf("#!%s\n", viper.GetString("shell")))
	tmp.WriteString(fmt.Sprintf("rm -f %s\n", name))
	debug(fmt.Sprintf("Writing %q script... ", name))
	for _, line := range script {
		if !(line.IsEmpty()) {
			word := line.Index(0)
			if word == "exec" {
				line.Append(args...)
			}
			if strings.Contains(word, "[") {
				line.WriteTo(tmp)
			} else if viper.GetBool("verbose") {
				line.WriteVerboseTo(tmp)
			} else {
				line.WriteTo(tmp)
			}
		}
	}
	debug("Done.")
	err = tmp.Close()
	fatal(wrap(err))
	err = os.Chmod(name, 0755)
	fatal(wrap(err))
	// run script
	debug(fmt.Sprintf("Running %q script... \n", name))
	shell := viper.GetString("shell")
	cmd := NewCmdLine(shell, "-c", name)
	_, err = cmd.Start("", nil, os.Stdout, os.Stderr)
	fatal(wrap(err))
}

// vim: set ft=go fdm=indent ts=2 sw=2 tw=79 noet:
