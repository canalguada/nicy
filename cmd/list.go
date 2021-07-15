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
	"strings"
	"path"
	"github.com/canalguada/nicy/jq"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// listCmd represents the list command
var listCmd = &cobra.Command{
	Use:   "list [--from=directory] category",
	Short: "List json objects",
	Long: `List the objects from cgroups, types or rules category, removing all duplicates`,
	ValidArgs: []string{"cgroups", "types", "rules"},
	Args: cobra.ExactValidArgs(1),
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		// Debug output
		w := debugOutput(cmd)

		// Real job goes here
		var confdirs []interface{}

		// Prepare input
		if viper.IsSet("from") {
			confdirs = []interface{}{viper.Get("from")}
		} else {
			dirs := viper.GetStringSlice("confdirs")
			confdirs = make([]interface{}, len(dirs))
			for i, dir := range dirs {
				confdirs[i] = dir
			}
		}
		// Prepare variables
		cachedb, err := readCache()
		checkErr(err)

		w.Init(cmd.OutOrStdout(), 8, 8, 0, '\t', 0)
		defer w.Flush()

		req := jq.NewRequest(
			`include "list"; list`,
			[]string{"$cachedb", "$kind"},
			cachedb,
			strings.TrimRight(args[0], "s"),
		)
		req.LibDirs = []string{path.Join(viper.GetString("libdir"), "jq")}
		output, err := req.Output(confdirs)
		checkErr(err)

		if viper.GetBool("no-headers") {
			for _, line := range output[1:] {
				fmt.Fprintln(w, line)
			}
		} else {
			for _, line := range output {
				fmt.Fprintln(w, line)
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(listCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// listCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	fs := listCmd.Flags()
	fs.SortFlags = false
	fs.SetInterspersed(false)

	fs.StringP("from", "f", "", "list only objects from confdir `directory`")
	fs.BoolP("no-headers", "n", false, "do not print headers")

	viper.BindPFlags(fs)

	listCmd.InheritedFlags().SortFlags = false
}

// vim: set ft=go fdm=indent ts=2 sw=2 tw=79 noet:
