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

// buildCmd represents the list command
var buildCmd = &cobra.Command{
	Use:                   "build [-f] [-d]",
	Short:                 "Build yaml cache",
	Long:                  `Build the yaml cache and exit`,
	Args:                  cobra.ExactArgs(0),
	DisableFlagsInUseLine: true,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		// Bind shared flags
		return viper.BindPFlags(cmd.LocalNonPersistentFlags())
	},
	Run: func(cmd *cobra.Command, args []string) {
		// Debug output
		debugOutput(cmd)
		// Real job goes here
		cacheFile := viper.GetString("cache")
		presetCache = GetPresetCache()
		if viper.GetBool("dump") {
			data, err := presetCache.GetContent()
			fatal(wrap(err))
			fmt.Fprintln(cmd.OutOrStdout(), string(data))
		} else {
			if presetCache.Origin == "file" && !viper.GetBool("force") {
				cmd.PrintErrf("Found %q cache file. Use --force to rebuild.", cacheFile)
			} else {
				// Write cache file
				cmd.PrintErrf("Writing %q cache file... ", cacheFile)
				fatal(wrap(presetCache.WriteCache(cacheFile)))
				cmd.PrintErrln("Done.")
			}
		}
	},
}

func init() {
	// Persistent flags
	// Local flags
	fs := buildCmd.Flags()
	fs.SortFlags = false
	fs.SetInterspersed(false)
	// addCacheFlag(buildCmd)
	fs.BoolP("dump", "d", false, "dump to stdout without saving")
	buildCmd.InheritedFlags().SortFlags = false
}

// vim: set ft=go fdm=indent ts=2 sw=2 tw=79 noet:
