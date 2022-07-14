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
	// flag "github.com/spf13/pflag"
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
	Args: cobra.MaximumNArgs(0),
	DisableFlagsInUseLine: true,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		fs := cmd.LocalNonPersistentFlags()
		if err := checkConsistency(fs, cfgMap["scopes"]); err != nil {
			return err
		}
		names := append(cfgMap["scopes"], "dry-run")
		// Bind shared flags
		bindFlags(cmd, names...)
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		viper.Set("tag", "set")
		// Debug output
		debugOutput(cmd)
		// Real job goes here
		var scope = "user"
		if value, ok := firstTrue(cfgMap["scopes"]); ok {
			scope = value
		}
		if err := setCapabilities(true); err != nil {
			cmd.PrintErrln(err)
		}
		defer func() {
			if err := setCapabilities(false); err != nil {
				cmd.PrintErrln(err)
			}
		}()
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
					job.Run("", cmd.OutOrStdout(), cmd.ErrOrStderr())
				}
			}()
		}
		wg.Add(1) // get jobs
		go getGroupJobs(procs, jobs, &wg)
		// send input
		filter := GetFilterer(scope)
		if viper.GetBool("verbose") {
			fmt.Fprintf(cmd.ErrOrStderr(), "Setting %v...\n", filter)
		}
		procs <- FilteredProcs(filter)
		close(procs)
		wg.Wait() // wait on the workers to finish
	},
}

func init() {
	// rootCmd.AddCommand(setCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// setCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	fs := setCmd.Flags()
	fs.SortFlags = false
	fs.SetInterspersed(false)

	addDumpManageFlags(setCmd)
	addDryRunFlag(setCmd)

	setCmd.InheritedFlags().SortFlags = false
}

// vim: set ft=go fdm=indent ts=2 sw=2 tw=79 noet:
