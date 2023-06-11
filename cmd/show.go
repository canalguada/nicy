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
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// showCmd represents the show command
var showCmd = &cobra.Command{
	Use:   "show [-q] [-S SHELL] [-R] [-p PRESET|-d|-z] [-c CGROUP|--cpu QUOTA] [-m] [-u] COMMAND",
	Short: "Show effective script for given command",
	Long: `Show the effective script for the given COMMAND

The PRESET argument can be:
- 'auto' to use some specific rule for the command, if available;
- 'cgroup-only' to use only the cgroup properties of that rule, if any;
- 'default' to use this special fallback preset;
-  any other generic type.
The CGROUP argument can be a cgroup defined in configuration files.
The QUOTA argument can be an integer ranging from 1 to 99.
It represents a percentage of the whole CPU time available, on all cores.`,
	Args:                  cobra.MinimumNArgs(1),
	DisableFlagsInUseLine: true,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		fs := cmd.LocalNonPersistentFlags()
		// Bind shared flags
		err := viper.BindPFlags(fs)
		// Set runtime values where needed
		switch {
		case fs.Changed("default"):
			viper.Set("preset", "default")
		case fs.Changed("cgroup-only"):
			viper.Set("preset", "cgroup-only")
		}
		if fs.Changed("cpu") {
			viper.Set("cgroup", "cpu"+viper.GetString("cpu"))
		}
		if valid, err := ValidShell(viper.GetString("shell")); err != nil {
			return err
		} else {
			viper.Set("shell", valid)
		}
		return err
	},
	Run: func(cmd *cobra.Command, args []string) {
		viper.Set("tag", "show")
		// Debug output
		debugOutput(cmd)
		// Real job goes here
		presetCache = GetPresetCache() // get cache content, once for all goroutines
		lines, err := doShowCmd(args)
		fatal(wrap(err))
		cmd.SetOut(os.Stdout)
		cmd.Println(strings.Join(lines, "\n"))
	},
}

func init() {
	// Persistent flags
	// Local flags
	fs := showCmd.Flags()
	fs.SortFlags = false
	fs.SetInterspersed(false)
	addJobFlags(showCmd)
	addScriptFlags(showCmd)
	showCmd.InheritedFlags().SortFlags = false
}

func doShowCmd(command []string) (result []string, err error) {
	shell := viper.GetString("shell")
	c := NewCommand(command...)
	if job, _, err := c.RunJob(shell); err == nil {
		// spin up workers
		wg := getWaitGroup() // use a sync.WaitGroup to indicate completion
		wg.Add(1)            // get result
		go func() {
			defer wg.Done()
			result, err = job.Script(shell)
		}()
		wg.Wait() // wait on the workers to finish
	}
	return
}

// vim: set ft=go fdm=indent ts=2 sw=2 tw=79 noet:
