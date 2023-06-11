package cmd

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/kballard/go-shellquote"
	"github.com/spf13/viper"
	"golang.org/x/sys/unix"
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

type Tokener interface {
	Len() int
	Copy() Tokener
	Append(s ...string) Tokener
	Prepend(s ...string) Tokener
	Content() []string
	IsEmpty() bool
	Index(pos int) string
	ShellCmd() string
	String() string
	WriteTo(dest io.Writer) (n int, err error)
}

type Tokens []string

func (t Tokens) Len() int {
	return len(t)
}

func (t Tokens) Copy() Tokener {
	result := make([]string, len(t))
	_ = copy(result, t)
	return Tokens(result)
}

func (t Tokens) Append(s ...string) Tokener {
	return append(t, s...)
}

func (t Tokens) Prepend(s ...string) Tokener {
	return append(Tokens(s), t...)
}

func (t Tokens) Content() []string {
	return ([]string)(t)
}

func (t Tokens) IsEmpty() bool {
	return len(t) == 0
}

func (t Tokens) Index(pos int) (result string) {
	if pos >= 0 && pos < len(t) {
		result = t[pos]
		return
	}
	inform("warning", fmt.Errorf("%w: not a valid index: %d", ErrInvalid, pos).Error())
	return
}

func (t Tokens) ShellCmd() string {
	return strings.TrimSpace(strings.Join(t, ` `))
}

func (t Tokens) String() string {
	return `[` + t.ShellCmd() + `]`
}

func (t Tokens) WriteTo(dest io.Writer) (n int, err error) {
	n, err = fmt.Fprintf(dest, t.ShellCmd()+"\n")
	return
}

type Command struct {
	Tokener
	skipRuntime bool
}

func NewCommand(s ...string) Command {
	return Command{Tokener: Tokens(s)}
}

func FromShellCmd(cmd string) Command {
	s, err := ShellSplit(cmd)
	fatal(wrap(err))
	return NewCommand(s...)
}

func (c Command) Copy() Tokener {
	s := make([]string, len(c.Content()))
	_ = copy(s, c.Content())
	return Command{Tokener: Tokens(s), skipRuntime: c.skipRuntime}
}

func userTest(shell string) string {
	switch filepath.Base(shell) {
	case "sh", "dash":
		return "[ $( id -u ) -ne 0 ]"
	case "bash":
		return "[ $UID -ne 0 ]"
	case "zsh":
		return "(( $UID ))"
	default:
		return "[ $( id -u ) -ne 0 ]"
	}
}

func SudoCommand(shell string) Command {
	replace := viper.GetString("sudo")
	if len(replace) == 0 {
		replace = "$SUDO"
	}
	result := NewCommand(userTest(shell) + " && SUDO=" + replace + " || unset SUDO")
	result.skipRuntime = true
	return result
}

func ManagerCommand(shell string) Command {
	result := NewCommand(userTest(shell) + " && manager=user || manager=system")
	result.skipRuntime = true
	return result
}

func (c *Command) RequireSysCapability() (comm string, flag bool) {
	var pos int
	if c.Index(0) == "$SUDO" {
		pos = 1
	}
	// Set flag for given system utilies
	comm = filepath.Base(c.Index(pos))
	switch comm {
	case "renice", "chrt", "ionice", "choom":
		flag = true
	}
	return
}

func (c *Command) Runtime(pid, uid int) Command {
	var tokens []string
	for _, token := range c.Content() {
		switch {
		case token == "--${manager}":
			if uid != 0 {
				tokens = append(tokens, "--user")
			} else {
				tokens = append(tokens, "--system")
			}
		case strings.Contains(token, "$$"):
			tokens = append(tokens, strings.Replace(token, "$$", strconv.Itoa(pid), 1))
		default:
			tokens = append(tokens, token)
		}
	}
	return NewCommand(tokens...)
}

func (c *Command) Output() (output []string, err error) {
	data, err := exec.Command(c.Content()[0], c.Content()[1:]...).Output()
	if err != nil {
		return
	}
	output = strings.Split(string(data), "\n")
	return
}

type Streams struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

func (c *Command) Scan(std *Streams) Command {
	var tokens []string
	for _, token := range c.Content() {
		switch {
		case strings.HasPrefix(token, ">") || strings.HasPrefix(token, "1>"):
			arg := strings.TrimPrefix(token, "1>")
			arg = strings.TrimPrefix(token, ">")
			arg = strings.TrimSpace(arg)
			if arg == "/dev/null" {
				std.Stdout = nil
			} else if arg == "&2" {
				std.Stdout = std.Stderr
			} else {
				warn(fmt.Errorf("%w: invalid redirection: %v", ErrInvalid, token))
				continue
			}
		case strings.HasPrefix(token, "2>"):
			name := strings.TrimPrefix(token, "2>")
			name = strings.TrimSpace(name)
			if name == "/dev/null" {
				std.Stderr = nil
			} else if name == "&2" {
				std.Stderr = std.Stdout
			} else {
				warn(fmt.Errorf("%w: invalid redirection: %v", ErrInvalid, token))
				continue
			}
		default:
			tokens = append(tokens, token)
		}
	}
	return NewCommand(tokens...)
}

func (c *Command) getCmd(std *Streams) (cmd *exec.Cmd, err error) {
	tokens := c.Scan(std).Content()
	command, err := exec.LookPath(tokens[0])
	if err != nil {
		return
	}
	cmd = exec.Command(command, tokens[1:]...)
	// Connect input/output
	cmd.Stdin = std.Stdin
	cmd.Stdout = std.Stdout
	cmd.Stderr = std.Stderr
	return
}

func whenVerbose(tag, path string, args ...string) {
	if viper.GetBool("dry-run") || viper.GetBool("verbose") {
		inform(tag, strings.Join(append([]string{path}, args...), ` `))
	}
}

func (c *Command) preRun() (err error) {
	tokens := c.Copy().Content()
	if tokens[0] == "exec" {
		tokens = tokens[1:]
	}
	if tokens[0] == "$SUDO" { // discard sudo prefix when not required
		if (os.Getuid() == 0) || viper.GetBool("ambient") {
			tokens = tokens[1:]
		} else {
			tokens[0] = viper.GetString("sudo")
		}
	}
	tokens = Filter(tokens, func(s string) bool { return len(s) > 0 })
	if len(tokens) == 0 {
		err = fmt.Errorf("%w: empty command", ErrInvalid)
	}
	*c = NewCommand(tokens...)
	return
}

func (c *Command) Start(tag string, stdin io.Reader, stdout, stderr io.Writer) (cmd *exec.Cmd, err error) {
	if err = c.preRun(); err != nil {
		return
	}
	// set command
	if cmd, err = c.getCmd(&Streams{stdin, stdout, stderr}); err != nil {
		return
	}
	whenVerbose(tag, cmd.Path, cmd.Args[1:]...)
	// Return if dry-run
	if viper.GetBool("dry-run") {
		return
	}
	cmd.SysProcAttr = &unix.SysProcAttr{Setpgid: true}
	err = cmd.Start()
	return
}

func (c *Command) StartWait(tag string, stdin io.Reader, stdout, stderr io.Writer) (err error) {
	var cmd *exec.Cmd
	if cmd, err = c.Start(tag, stdin, stdout, stderr); err != nil {
		return
	}
	// Return if dry-run
	if viper.GetBool("dry-run") {
		return
	}
	return cmd.Wait()
}

func (c *Command) Exec(tag string) error {
	if err := c.preRun(); err != nil {
		return err
	}
	tokens := c.Content()
	command, err := exec.LookPath(tokens[0])
	if err != nil {
		return err
	}
	whenVerbose(tag, command, tokens[1:]...)
	// Return if dry-run
	if viper.GetBool("dry-run") {
		return nil
	}
	// Do not propagate capabilities
	nonfatal(updatePrivileges(false, true)) // clear all
	args := append([]string{filepath.Base(command)}, tokens[1:]...)
	return unix.Exec(command, args, os.Environ())
}

func (c *Command) Run(tag string, stdin io.Reader, stdout, stderr io.Writer) error {
	if c.Index(0) == "exec" {
		return c.Exec(tag)
	}
	return c.StartWait(tag, stdin, stdout, stderr)
}

func (c *Command) Split() (path string, args []string, err error) {
	if c.skipRuntime || c.IsEmpty() {
		err = fmt.Errorf("%w: no runnable command", ErrInvalid)
		return
	}
	tokens := c.Content()
	if path = LookPath(tokens[0]); len(path) > 0 {
		if len(tokens) > 1 {
			args = tokens[1:]
		}
		return
	}
	err = &exec.Error{tokens[0], exec.ErrNotFound}
	return
}

func (c *Command) RunJob(shell string) (job *ProcJob, args []string, err error) {
	var cmd string
	if cmd, args, err = c.Split(); err != nil {
		return
	}
	// prepare channels
	jobs := make(chan *ProcJob, 2)
	inputs := make(chan *Request)
	// spin up workers
	go presetCache.GenerateJobs(inputs, jobs, nil)
	input := NewPathRequest(cmd, shell)
	input.Proc = GetCalling()
	inputs <- input // send input
	close(inputs)
	job = <-jobs // wait for result
	return
}

type Script []Command

func (s Script) String() string {
	return Reduce(s, `[`, func(result string, c Command) string {
		return fmt.Sprintf("%s %s", result, c.String())
	}) + `]`
}

// vim: set ft=go fdm=indent ts=2 sw=2 tw=79 noet:
