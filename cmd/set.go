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
	"io"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// setCmd represents the set command
var setCmd = &cobra.Command{
	Use:   "set [-n] [-u|-g|-s|-a]",
	Short: "Set running processes attributes",
	Long: `Set once the running processes attributes, applying presets, if any

The processes are selected when their group leader matches an existing rule.
The --user option is the implied default, when none is given.
Only superuser can run set command with --system, --global or --all option.`,
	Args:                  cobra.MaximumNArgs(0),
	DisableFlagsInUseLine: true,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		// Bind shared flags
		return viper.BindPFlags(cmd.LocalNonPersistentFlags())
	},
	Run: func(cmd *cobra.Command, args []string) {
		viper.Set("tag", "set")
		// Debug output
		debugOutput(cmd)
		// Real job goes here
		if err := setCapabilities(true); err != nil {
			cmd.PrintErrln(err)
		}
		defer func() {
			if err := setCapabilities(false); err != nil {
				cmd.PrintErrln(err)
			}
		}()
		scope := GetStringFromFlags("user", viper.GetStringSlice("scopes")...)
		filter := GetScopeOnlyFilterer(scope)
		err := doSetCmd("", filter, cmd.OutOrStdout(), cmd.ErrOrStderr())
		fatal(wrap(err))
	},
}

func init() {
	// Persistent flags
	// Local flags
	fs := setCmd.Flags()
	fs.SortFlags = false
	fs.SetInterspersed(false)
	viper.Set("scopes", addScopeFlags(setCmd))
	addDryRunFlag(setCmd)
	// addVerboseFlag(setCmd)
	setCmd.InheritedFlags().SortFlags = false
}

func doSetCmd(tag string, filter ProcFilterer, stdout, stderr io.Writer) (err error) {
	// prepare channels
	jobs := make(chan *ProcGroupJob, 8)
	procs := make(chan []*Proc, 8)
	// spin up workers
	wg := getWaitGroup() // use a sync.WaitGroup to indicate completion
	for i := 0; i < (goMaxProcs + 1); i++ {
		wg.Add(1) // run jobs
		go func() {
			defer wg.Done()
			for job := range jobs {
				err = job.Run("", stdout, stderr)
				if err != nil {
					return
				}
			}
		}()
	}
	wg.Add(1) // get jobs
	go generateGroupJobs(procs, jobs, &wg)
	// send input
	// filter := GetScopeOnlyFilterer(scope)
	if viper.GetBool("dry-run") || viper.GetBool("verbose") {
		inform("", fmt.Sprintf("Setting %v...", filter))
	}
	procs <- FilteredProcs(filter)
	close(procs)
	wg.Wait() // wait on the workers to finish
	if viper.GetBool("dry-run") || viper.GetBool("verbose") {
		inform("", "Done.")
	}
	return
}

// vim: set ft=go fdm=indent ts=2 sw=2 tw=79 noet:
