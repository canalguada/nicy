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
	// flag "github.com/spf13/pflag"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// installCmd represents the install command
var installCmd = &cobra.Command{
	Use:   "install [-r] [--shell SHELL] [--dest DESTDIR]",
	Short: "Install scripts",
	Long: `Install a shell script for each rule matching a command found in PATH.

The SHELL argument is a path to a POSIX shell. Default value is /bin/sh.
The installation path is set to :
- $HOME/bin/nicy for regular user;
- /usr/local/bin/nicy for system user;
- any writable path DESTDIR with --dest option.`,
	Args: cobra.MaximumNArgs(0),
	DisableFlagsInUseLine: true,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		// Bind flags
		bindFlags(cmd, "shell", "dest")
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		viper.Set("tag", "install")
		// Debug output
		debugOutput(cmd)
		// Real job goes here
	},
}

func init() {
	// rootCmd.AddCommand(installCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// installCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	fs := installCmd.Flags()
	fs.SortFlags = false
	fs.SetInterspersed(false)

	fs.String("shell", "", "generate script for `SHELL`")
	fs.String("dest", "", "install inside `DESTDIR`")
	fs.BoolP("run", "r", false, "use run command")

	installCmd.InheritedFlags().SortFlags = false
}

// vim: set ft=go fdm=indent ts=2 sw=2 tw=79 noet:
