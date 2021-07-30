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
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// listCmd represents the list command
var listCmd = &cobra.Command{
	Use:   "list [--from=DIRECTORY] CATEGORY",
	Short: "List json objects",
	Long: `List the objects from cgroups, types or rules CATEGORY, removing all duplicates

The CATEGORY argument can be 'rules', 'types' or 'cgroups', matching the extensions of configuration files. The DIRECTORY argument can be one out of preconfigured directories. When filtering per DIRECTORY, no duplicate is removed taking into account the priority between all of them.`,
	ValidArgs: []string{"cgroups", "types", "rules"},
	Args: cobra.ExactValidArgs(1),
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		// Debug output
		w := debugOutput(cmd)
		// Real job goes here
		output, err := listObjects(args[0])
		checkErr(err)

		w.Init(cmd.OutOrStdout(), 8, 8, 0, '\t', 0)
		defer w.Flush()

		if viper.GetBool("no-headers") {
			output = output[1:]
		}
		for _, line := range output {
			fmt.Fprintln(w, line)
		}
	},
}

func init() {
	// rootCmd.AddCommand(listCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// listCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	fs := listCmd.Flags()
	fs.SortFlags = false
	fs.SetInterspersed(false)

	fs.StringP("from", "f", "", "list only objects from `DIRECTORY`")
	fs.BoolP("no-headers", "n", false, "do not print headers")

	viper.BindPFlags(fs)

	listCmd.InheritedFlags().SortFlags = false
}

// vim: set ft=go fdm=indent ts=2 sw=2 tw=79 noet:
