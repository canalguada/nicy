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
	// "fmt"
	// "os"
	// "strings"
	// "encoding/json"
	"github.com/canalguada/nicy/process"
	// "github.com/canalguada/nicy/jq"
	// flag "github.com/spf13/pflag"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// manageCmd represents the manage command
var manageCmd = &cobra.Command{
	Use:   "manage [-n] [-u|-g|-s|-a]",
	Short: "Manage running processes",
	Long: `Manage the running processes, applying presets

The processes are managed per process group, when a specific rule is available for the process group leader. The --system, --global and --all options require root credentials.`,
	Args: cobra.MaximumNArgs(0),
	DisableFlagsInUseLine: true,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		slice := []string{"user", "global", "system", "all"}
		fs := cmd.LocalNonPersistentFlags()
		if err := checkConsistency(fs, slice); err != nil {
			return err
		}
		// Bind shared flags
		bindFlags(cmd, "user", "global", "system", "all", "dry-run")
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		viper.Set("tag", "manage")
		// Debug output
		debugOutput(cmd)
		// Real job goes here
		var filter process.Filter
		var message string
		switch {
		case viper.GetBool("user"):
			filter = process.GetFilter("user")
			message = "calling user processes"
		case viper.GetBool("global"):
			filter = process.GetFilter("global")
			message = "processes inside any user slice"
		case viper.GetBool("system"):
			filter = process.GetFilter("system")
			message = "processes inside system slice"
		case viper.GetBool("all"):
			filter = process.GetFilter("all")
			message = "all processes"
		default:
			filter = process.GetFilter("user")
			message = "calling user processes"
		}
		// Get result
		if viper.GetBool("verbose") {
			cmd.PrintErrln("Managing", message + "...")
		}
		if err := setCapabilities(true); err != nil {
			cmd.PrintErrln(err)
		}
		defer func() {
			if err := setCapabilities(false); err != nil {
				cmd.PrintErrln(err)
			}
		}()
		err := manageCommand("", filter, cmd.OutOrStdout(), cmd.ErrOrStderr())
		fatal(wrap(err))
		// cmd.Println(prettyJson(getManageInput(filter)))
	},
}

func init() {
	// rootCmd.AddCommand(manageCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// manageCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	fs := manageCmd.Flags()
	fs.SortFlags = false
	fs.SetInterspersed(false)

	addDumpManageFlags(manageCmd)
	addDryRunFlag(manageCmd)

	manageCmd.InheritedFlags().SortFlags = false
}

// vim: set ft=go fdm=indent ts=2 sw=2 tw=79 noet:
