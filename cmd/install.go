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
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// installCmd represents the install command
var installCmd = &cobra.Command{
	Use:   "install [-S SHELL] [-R] [-d DESTDIR]",
	Short: "Install scripts",
	Long: `Install a shell script for each rule matching a command found in PATH.

The SHELL argument is a path to a POSIX shell. Default value is /bin/sh.
The installation path is set to :
- $HOME/bin/nicy for regular user;
- /usr/local/bin/nicy for system user;
- any writable path DESTDIR with --dest option.`,
	Args:                  cobra.MaximumNArgs(0),
	DisableFlagsInUseLine: true,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		fs := cmd.LocalNonPersistentFlags()
		// Bind shared flags
		err := viper.BindPFlags(fs)
		if valid, err := ValidShell(viper.GetString("shell")); err != nil {
			return err
		} else {
			viper.Set("shell", valid)
		}
		dest := viper.GetString("dest")
		if len(dest) == 0 {
			dest = viper.GetString("scripts.location")
		}
		viper.Set("dest", expandPath(dest))
		return err
	},
	Run: func(cmd *cobra.Command, args []string) {
		viper.Set("tag", "install")
		// Debug output
		debugOutput(cmd)
		// Real job goes here
		presetCache = GetPresetCache() // get cache content, once for all goroutines
		err := doInstallCmd(viper.GetString("dest"))
		fatal(wrap(err))
	},
}

func init() {
	// Persistent flags
	// Local flags
	fs := installCmd.Flags()
	fs.SortFlags = false
	fs.SetInterspersed(false)
	addScriptFlags(installCmd)
	fs.StringP("dest", "d", "", "install inside `DESTDIR`")
	installCmd.InheritedFlags().SortFlags = false
}

func scriptOk(file string) (path string, ok bool) {
	if len(file) == 0 {
		return
	}
	path = file
	for _, pattern := range viper.GetStringSlice("scripts.ignore") {
		if matched, err := filepath.Match(pattern, path); matched {
			return
		} else if err != nil {
			inform("warning", err.Error())
		}
	}
	ok = true
	return
}

func linkOk(path string) (ok bool) {
	for _, pattern := range viper.GetStringSlice("scripts.link") {
		if matched, err := filepath.Match(pattern, path); matched {
			ok = true
			return
		} else if err != nil {
			inform("warning", err.Error())
		}
	}
	return
}

func doInstallCmd(location string) (err error) {
	if exists(location) {
		backup := location + "_old"
		if exists(backup) {
			if e := os.RemoveAll(backup); e != nil {
				inform("warning", e.Error())
			}
		}
		if e := os.Rename(location, backup); e != nil {
			inform("warning", e.Error())
		}
	}
	os.MkdirAll(location, 0755)
	// prepare channels
	jobs := make(chan *ProcJob, 8)
	inputs := make(chan *Request, 8)
	// spin up workers
	wg := getWaitGroup() // use a sync.WaitGroup to indicate completion
	shell := viper.GetString("shell")
	if viper.GetBool("run") {
		if valid, err := ValidShell("sh"); err != nil {
			return err
		} else {
			shell = valid
		}
	}
	wg.Add(1) // write commands
	go func() {
		defer wg.Done()
		for job := range jobs {
			lines, err := job.Script(shell)
			if err != nil {
				nonfatal(wrap(err))
				continue
			}
			dest := filepath.Join(location, job.Request.Name+".nicy")
			inform("", job.Request.Path)
			if !viper.GetBool("dry-run") { // write to dest
				fatal(wrap(os.WriteFile(dest, []byte(strings.Join(lines, "\n")), 0755)))
				if linkOk(job.Request.Name) {
					fatal(wrap(os.Symlink(dest, strings.TrimSuffix(dest, ".nicy"))))
				}
			}
		}
	}()
	wg.Add(1) // produce jobs
	go presetCache.GenerateJobs(inputs, jobs, &wg)
	wg.Add(1) // get inputs
	go func() {
		defer wg.Done()
		calling := GetCalling()
		for r := range IterCache(presetCache.Rules) {
			if path, ok := scriptOk(r.Path()); ok {
				input := NewRawRequest(r.RuleKey, path, viper.GetString("shell"))
				input.Proc = calling
				inputs <- input
			}
		}
		close(inputs)
	}()
	wg.Wait() // wait on the workers to finish
	if viper.GetBool("dry-run") || viper.GetBool("verbose") {
		inform("", "Done.")
	}
	return
}

// vim: set ft=go fdm=indent ts=2 sw=2 tw=79 noet:
