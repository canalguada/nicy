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

// dumpCmd represents the dump command
var dumpCmd = &cobra.Command{
	Use:                   "dump [-u|-g|-s|-a] [-r|-j|-n] [-m]",
	Short:                 "Dump processes information",
	Long:                  `Dump information on the running processes`,
	Args:                  cobra.MaximumNArgs(0),
	DisableFlagsInUseLine: true,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		// Bind shared flags
		return viper.BindPFlags(cmd.LocalNonPersistentFlags())
	},
	Run: func(cmd *cobra.Command, args []string) {
		// Debug output
		debugOutput(cmd)
		// Real job goes here
		presetCache = GetPresetCache() // get cache, once for all goroutines
		var (
			format    = GetStringFromFlags("string", viper.GetStringSlice("formats")...)
			formatter = GetFormatter(format)
			scope     = GetStringFromFlags("user", viper.GetStringSlice("scopes")...)
			filterer  ProcFilterer
		)
		if viper.GetBool("manageable") {
			filterer = presetCache.GetFilterer(scope)
		} else {
			filterer = GetScopeOnlyFilterer(scope)
		}
		if viper.GetBool("verbose") {
			// cmd.PrintErrln("Dumping stats for", filterer.String()+"...")
			fmt.Fprintln(cmd.ErrOrStderr(), "Dumping stats for", filterer.String()+"...")
		}
		for _, p := range FilteredProcs(filterer) {
			// cmd.Println(formatter(p))
			fmt.Fprintln(cmd.OutOrStdout(), formatter(p))
		}
	},
}

func init() {
	// Persistent flags
	// Local flags
	fs := dumpCmd.Flags()
	fs.SortFlags = false
	fs.SetInterspersed(false)
	viper.Set("scopes", addScopeFlags(dumpCmd))
	viper.Set("formats", addFormatFlags(dumpCmd))
	fs.BoolP("manageable", "m", false, "only manageable processes")
	// addVerboseFlag(dumpCmd)
	dumpCmd.InheritedFlags().SortFlags = false
}

// vim: set ft=go fdm=indent ts=2 sw=2 tw=79 noet:
