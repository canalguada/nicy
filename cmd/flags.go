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
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func addJobFlags(cmd *cobra.Command) (names []string) {
	fs := cmd.Flags()
	fs.StringP("preset", "p", "auto", "apply this `PRESET`")
	fs.BoolP("default", "d", false, "like --preset default")
	fs.BoolP("cgroup-only", "z", false, "like --preset cgroup-only")
	fs.StringP("cgroup", "c", "", "run as part of this `CGROUP`")
	fs.Int("cpu", 0, "like --cgroup cpu`QUOTA`")
	fs.BoolP("managed", "m", false, "always run inside its own scope")
	fs.BoolP("force-cgroup", "u", false, "run inside a cgroup matching properties")
	cmd.MarkFlagsMutuallyExclusive("preset", "default", "cgroup-only")
	cmd.MarkFlagsMutuallyExclusive("cgroup", "cpu")
	names = append(names, "preset", "default", "cgroup-only", "cgroup", "cpu", "managed", "force-cgroup")
	return
}

func addDryRunFlag(cmd *cobra.Command) {
	fs := cmd.Flags()
	fs.BoolP("dry-run", "n", false, "display external commands instead running them")
}

// func addCacheFlag(cmd *cobra.Command) {
// fs := cmd.Flags()
// fs.BoolP("force", "f", false, "ignore existing cache and forcefully build new")
// }

func addScopeFlags(cmd *cobra.Command) (names []string) {
	fs := cmd.Flags()
	fs.BoolP("user", "u", false, "only processes from calling user slice")
	fs.BoolP("global", "g", false, "processes from any user slice")
	fs.BoolP("system", "s", false, "only processes from system slice")
	fs.BoolP("all", "a", false, "all running processes")
	names = append(names, "user", "global", "system", "all")
	cmd.MarkFlagsMutuallyExclusive(names...)
	return
}

func addFormatFlags(cmd *cobra.Command) (names []string) {
	fs := cmd.Flags()
	fs.BoolP("raw", "r", false, "use raw format")
	fs.BoolP("json", "j", false, "use json format")
	fs.BoolP("values", "n", false, "use nicy values format")
	names = append(names, "raw", "json", "values")
	cmd.MarkFlagsMutuallyExclusive(names...)
	return
}

func addScriptFlags(cmd *cobra.Command) {
	fs := cmd.Flags()
	fs.StringP("shell", "S", viper.GetString("shell"), "use this `SHELL` generating the script")
	fs.BoolP("run", "R", false, "replace script with a nicy run command")
}

// func bindFlags(cmd *cobra.Command, names ...string) {
//   fs := cmd.Flags()
//   for _, name := range names {
//     viper.BindPFlag(name, fs.Lookup(name))
//   }
// }

func GetStringFromFlags(fallback string, names ...string) string {
	for _, name := range names {
		if viper.GetBool(name) {
			return name
		}
	}
	return fallback
}

// vim: set ft=go fdm=indent ts=2 sw=2 tw=79 noet:
