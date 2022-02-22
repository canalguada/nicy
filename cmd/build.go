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
	"encoding/json"
	// flag "github.com/spf13/pflag"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// buildCmd represents the list command
var buildCmd = &cobra.Command{
	Use:   "build [-f]",
	Short: "Build json cache",
	Long: `Build the json cache and exit`,
	Args: cobra.ExactArgs(0),
	DisableFlagsInUseLine: true,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		// Bind flag
		bindFlags(cmd, "force")
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		// Debug output
		debugOutput(cmd)
		// Real job goes here
		cacheFile := viper.GetString("cache")
		contentCache := NewCache()
		if !(exists(cacheFile)) || viper.GetBool("force") {
			for _, category := range cfgMap["category"] {
				file := viper.GetString(category + `s`)
				if !(exists(file)) || viper.GetBool("force") {
					// Dump content of configuration files into category cache file
					var result []interface{}
					for _, root := range viper.GetStringSlice("confdirs") {
						objects, err := dumpObjects(category, expandPath(root))
						fatal(wrap(err))
						result = append(result, objects...)
					}
					// Write to category cache file
					data, err := json.MarshalIndent(result, "", "  ")
					fatal(wrap(err))
					cmd.PrintErrf("Writing content of %s files into cache... ", category)
					_, err = writeTo(file, data)
					fatal(wrap(err))
					cmd.PrintErrln("Done.")
					// Save content
					contentCache.AppendContent(category, result)
				} else {
					// Read content from category cache file
					cmd.PrintErrf("Reading %s presets from cache... ", category)
					result, err := ReadCategoryCache(category)
					fatal(wrap(err))
					cmd.PrintErrln("Done.")
					// Save content
					contentCache.AppendContent(category, result)
				}
			}
			// Write to cache file
			data, err := json.MarshalIndent(contentCache, "", "  ")
			fatal(wrap(err))
			cmd.PrintErrf("Writing %q cache file... ", cacheFile)
			_, err = writeTo(cacheFile, data)
			fatal(wrap(err))
			cmd.PrintErrln("Done.")
		}
	},
}

func init() {
	// rootCmd.AddCommand(buildCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// buildCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	fs := buildCmd.Flags()
	fs.SortFlags = false
	fs.SetInterspersed(false)

	fs.BoolP("force", "f", false, "ignore existing files in cache")

	buildCmd.InheritedFlags().SortFlags = false
}

// vim: set ft=go fdm=indent ts=2 sw=2 tw=79 noet:
