package cmd

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/viper"
	"golang.org/x/sys/unix"
)

func whenVerbose(tag, path string, args ...string) {
	if viper.GetBool("dry-run") || viper.GetBool("verbose") {
		inform(tag, strings.Join(append([]string{path}, args...), ` `))
	}
}

type Streams struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

type Command struct {
	Tokener
	skipRuntime bool
}

func NewCommand(s ...string) Command {
	return Command{Tokener: Tokens(s)}
}

func UserTernaryCommand(shell string, ok string, ko string) Command {
	var condition string
	switch filepath.Base(shell) {
	case "sh", "dash":
		condition = "[ $( id -u ) -ne 0 ]"
	case "bash":
		condition = "[ $UID -ne 0 ]"
	case "zsh":
		condition = "(( $UID ))"
	default:
		condition = "[ $( id -u ) -ne 0 ]"
	}
	result := NewCommand(fmt.Sprintf("%s && %s || %s", condition, ok, ko))
	result.skipRuntime = true
	return result
}
func SudoCommand(shell string) Command {
	replace := viper.GetString("sudo")
	if len(replace) == 0 {
		replace = "$SUDO"
	}
	return UserTernaryCommand(shell, "SUDO="+replace, "unset SUDO")
}

func ManagerCommand(shell string) Command {
	return UserTernaryCommand(shell, "manager=user", "manager=system")
}

func (c Command) Copy() Tokener {
	return Command{Tokener: Tokens(Clone(c.Content())), skipRuntime: c.skipRuntime}
}

func (c *Command) RequireSysCapability() bool {
	var pos int
	if c.Index(0) == "$SUDO" {
		pos = 1
	}
	// Set flag for given system utilies
	switch filepath.Base(c.Index(pos)) {
	case "renice", "chrt", "ionice", "choom":
		return true
	}
	return false
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
	if data, err := exec.Command(c.Content()[0], c.Content()[1:]...).Output(); err == nil {
		output = strings.Split(string(data), "\n")
	}
	return
}

func (c *Command) Scan(std *Streams) Command {
	var tokens []string
	for _, token := range c.Content() {
		switch {
		case strings.HasPrefix(token, ">") || strings.HasPrefix(token, "1>"):
			arg := strings.TrimPrefix(token, "1>")
			arg = strings.TrimPrefix(arg, ">")
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
			arg := strings.TrimPrefix(token, "2>")
			arg = strings.TrimSpace(arg)
			if arg == "/dev/null" {
				std.Stderr = nil
			} else if arg == "&1" {
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

func (c *Command) Start(tag string, std *Streams) (cmd *exec.Cmd, err error) {
	if err = c.preRun(); err != nil {
		return
	}
	// set command
	if cmd, err = c.getCmd(std); err != nil {
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

func (c *Command) StartWait(tag string, std *Streams) (err error) {
	var cmd *exec.Cmd
	if cmd, err = c.Start(tag, std); err != nil {
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

func (c *Command) Run(tag string, std *Streams) error {
	if c.Index(0) == "exec" {
		return c.Exec(tag)
	}
	return c.StartWait(tag, std)
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
	err = &exec.Error{Name: tokens[0], Err: exec.ErrNotFound}
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
