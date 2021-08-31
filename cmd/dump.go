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
	// "os"
	"github.com/canalguada/nicy/process"
	// flag "github.com/spf13/pflag"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// dumpCmd represents the dump command
var dumpCmd = &cobra.Command{
	Use:   "dump [--user|--global|--system|--all] [--raw|--json|--nicy]",
	Short: "Dump running processes statistics",
	Long: `Dump statistics for the running processes`,
	Args: cobra.MaximumNArgs(0),
	DisableFlagsInUseLine: true,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		var slices = map[int][]string{
			0: []string{"user", "global", "system", "all"},
			1: []string{"json", "raw", "nicy"},
		}
		fs := cmd.LocalNonPersistentFlags()
		for key := range slices {
			if err := checkConsistency(fs, slices[key]); err != nil {
				return err
			}
		}
		// Bind shared flags
		bindFlags(
			cmd,
			"user", "global", "system", "all",
			"raw", "json", "values",
		)
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		// Debug output
		debugOutput(cmd)
		// Real job goes here
		var filterFunc process.Filter
		var formatterFunc process.Formatter
		var message string
		switch {
		case viper.GetBool("user"):
			filterFunc = process.GetFilter("user")
			message = "calling user processes"
		case viper.GetBool("global"):
			filterFunc = process.GetFilter("global")
			message = "processes inside any user slice"
		case viper.GetBool("system"):
			filterFunc = process.GetFilter("system")
			message = "processes inside system slice"
		case viper.GetBool("all"):
			filterFunc = process.GetFilter("all")
			message = "all processes"
		default:
			filterFunc = process.GetFilter("user")
			message = "calling user processes"
		}
		if viper.GetBool("verbose") {
			cmd.PrintErrln("Dumping stats for", message + "...")
		}
		switch {
		case viper.GetBool("json"):
			formatterFunc = process.GetFormatter("json")
		case viper.GetBool("raw"):
			formatterFunc = process.GetFormatter("raw")
		case viper.GetBool("values"):
			formatterFunc = process.GetFormatter("values")
		default:
			formatterFunc = process.GetFormatter("string")
		}
		for _, p := range process.AllProcs(filterFunc) {
			cmd.Println(formatterFunc(&p))
		}
	},
}

func init() {
	// rootCmd.AddCommand(dumpCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// dumpCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	fs := dumpCmd.Flags()
	fs.SortFlags = false
	fs.SetInterspersed(false)

	addDumpManageFlags(dumpCmd)

	fs.BoolP("raw", "r", false, "use raw format")
	fs.BoolP("json", "j", false, "use json format")
	fs.BoolP("values", "v", false, "use nicy format")

	dumpCmd.InheritedFlags().SortFlags = false
}

// vim: set ft=go fdm=indent ts=2 sw=2 tw=79 noet:
