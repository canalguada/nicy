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
	"io"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:   "run [-n] [-q|-v] [-p PRESET|-d|-z] [-c CGROUP|--cpu QUOTA] [-m] [-u] COMMAND [ARGUMENT]...",
	Short: "Run given command in pre-set execution environment",
	Long: `Run the COMMAND with its ARGUMENT(S) in a pre-set execution environment

The PRESET argument can be:
- 'auto' to use some specific rule for the command, if available;
- 'cgroup-only' to use only the cgroup properties of that rule, if any;
- 'default' to use this special fallback preset;
-  any other generic profile.
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
		return err
	},
	Run: func(cmd *cobra.Command, args []string) {
		viper.Set("tag", "run")
		// Debug output
		debugOutput(cmd)
		// Real job goes here
		presetCache = GetPresetCache() // get cache content, once for all goroutines
		if err := setCapabilities(true); err != nil {
			cmd.PrintErrln(err)
		}
		defer func() {
			if err := setCapabilities(false); err != nil {
				cmd.PrintErrln(err)
			}
		}()
		err := doRunCmd("", args, nil, cmd.OutOrStdout(), cmd.ErrOrStderr())
		fatal(wrap(err))
	},
}

func init() {
	// Persistent flags
	// Local flags
	fs := runCmd.Flags()
	fs.SortFlags = false
	fs.SetInterspersed(false)
	addJobFlags(runCmd)
	addDryRunFlag(runCmd)
	// addVerboseFlag(runCmd)
	runCmd.InheritedFlags().SortFlags = false
}

func doRunCmd(tag string, command []string, stdin io.Reader, stdout, stderr io.Writer) (err error) {
	c := NewCommand(command...)
	viper.Set("pid", os.Getpid())
	if job, args, err := c.RunJob("/bin/sh"); err == nil {
		// spin up workers
		wg := getWaitGroup() // use a sync.WaitGroup to indicate completion
		wg.Add(1)            // run commands
		go func() {
			defer wg.Done()
			err = job.Run(tag, args, stdin, stdout, stderr)
		}()
		wg.Wait() // wait on the workers to finish
	}
	return
}

// vim: set ft=go fdm=indent ts=2 sw=2 tw=79 noet:
