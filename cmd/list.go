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
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// listCmd represents the list command
var listCmd = &cobra.Command{
	Use:   "list [-n] [-f ORIGIN] CATEGORY",
	Short: "List category content",
	Long: `List the content of cgroups, profiles or rules CATEGORY

The CATEGORY argument can be one out of 'rules', 'profiles' or 'cgroups'.
The ORIGIN argument can be one out of 'vendor', 'site', 'user', or
'other' when outside standard directories.
When filtering from ORIGIN, show otherwise removed duplicates.`,
	ValidArgs:             []string{"cgroups", "profiles", "rules"},
	Args:                  cobra.ExactValidArgs(1),
	DisableFlagsInUseLine: true,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		fs := cmd.LocalNonPersistentFlags()
		// Bind shared flags
		if err := viper.BindPFlags(fs); err != nil {
			return err
		}
		if fs.Changed("from") {
			value := viper.GetString("from")
			switch value {
			case "vendor", "site", "user", "other":
				return nil
			default:
				msg := "must be `vendor`, `site`, `user` or `other`"
				return fmt.Errorf("%w: %s, but got: `%s`", ErrInvalid, msg, value)
			}
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		// Debug output
		debugOutput(cmd)
		// Real job goes here
		tw := tabwriter.NewWriter(cmd.OutOrStdout(), 8, 8, 0, '\t', 0)
		// To update writer
		// tw.Init(cmd.OutOrStdout(), 8, 8, 0, '\t', 0)
		defer tw.Flush()
		presetCache = GetPresetCache()
		category := strings.TrimRight(args[0], "s")
		var (
			list []string
			err  error
		)
		if viper.IsSet("from") {
			list, err = presetCache.ListFrom(category, viper.GetString("from"))
		} else {
			list, err = presetCache.List(category)
		}
		sort.Strings(list)
		fatal(wrap(err))
		if !viper.GetBool("no-headers") {
			fmt.Fprintln(tw, fmt.Sprintf("%s\torigin\tcontent", category))
		}
		for _, line := range list {
			fmt.Fprintln(tw, line)
		}
	},
}

func init() {
	// Persistent flags
	// Local flags
	fs := listCmd.Flags()
	fs.SortFlags = false
	fs.SetInterspersed(false)
	fs.StringP("from", "f", "", "list only objects from `ORIGIN`")
	fs.BoolP("no-headers", "n", false, "do not print headers")
	listCmd.InheritedFlags().SortFlags = false
}

// vim: set ft=go fdm=indent ts=2 sw=2 tw=79 noet:
