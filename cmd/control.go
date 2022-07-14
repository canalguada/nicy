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
	"time"
	// flag "github.com/spf13/pflag"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// controlCmd represents the control command
var controlCmd = &cobra.Command{
	Use:   "control [-n] [-u|-g|-s|-a] [-t SECONDS]",
	Short: "Control running processes",
	Long: `Control the running processes, applying rules, if any

The processes are selected when their group leader matches an existing rule.
The --user option is the implied default, when none is given.
Only superuser can fully run manage command with --system, --global or --all option.`,
	Args: cobra.MaximumNArgs(0),
	DisableFlagsInUseLine: true,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		fs := cmd.LocalNonPersistentFlags()
		if err := checkConsistency(fs, cfgMap["scopes"]); err != nil {
			return err
		}
		names := append(cfgMap["scopes"], "dry-run", "tick")
		// Bind shared flags
		bindFlags(cmd, names...)
		if tick := viper.GetDuration("tick"); tick.Seconds() < 5 || tick.Seconds() > 3600 {
			msg := fmt.Sprintf("must range from 5s to 1h, got %v", tick)
			return fmt.Errorf("%w: %s", ErrParse, msg)
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		viper.Set("tag", "control")
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
		err := controlCommand(
			"",
			GetFilterer(scope),
			cmd.OutOrStdout(), cmd.ErrOrStderr(),
		)
		fatal(wrap(err))
	},
}

func init() {
	// rootCmd.AddCommand(controlCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// controlCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	fs := controlCmd.Flags()
	fs.SortFlags = false
	fs.SetInterspersed(false)

	addDumpManageFlags(controlCmd)
	addDryRunFlag(controlCmd)
	// fs.BoolP("monitor", "m", false, "run continuously")
	fs.DurationP("tick", "t", 5 * time.Second, "delay between consecutive runs in seconds")

	controlCmd.InheritedFlags().SortFlags = false
}

// vim: set ft=go fdm=indent ts=2 sw=2 tw=79 noet:
