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
	// "github.com/google/shlex"
	// flag "github.com/spf13/pflag"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func CommonPreRunE(cmd *cobra.Command, args []string) error {
	var err error
	fs := cmd.LocalNonPersistentFlags()
	for _, slice := range [3][]string{
		[]string{"quiet", "verbose"},
		[]string{"preset", "default", "cgroup-only"},
		[]string{"cgroup", "cpu"},
	} {
		err = checkConsistency(fs, slice)
		if err != nil {
			return err
		}
	}
	// Bind shared flags
	bindFlags(
		cmd,
		"dry-run", "quiet", "verbose", "preset", "default",
		"cgroup-only", "cgroup", "cpu", "managed", "force-cgroup",
	)
	// Set runtime values where needed
	switch {
	case fs.Changed("default"):
		viper.Set("preset", "default")
	case fs.Changed("cgroup-only"):
		viper.Set("preset", "cgroup-only")
	}
	if fs.Changed("cpu") {
		viper.Set("cgroup", "cpu" + viper.GetString("cpu"))
	}
	return nil
}

// showCmd represents the show command
var showCmd = &cobra.Command{
	Use:   "show [-q|-v] [--preset PRESET|-d|-z] [--cgroup CGROUP|--cpu QUOTA] [-m] [-u] COMMAND",
	Short: "Show effective script for given command",
	Long: `Show the effective script for the given COMMAND

The PRESET argument can be: 'auto' to use some specific rule for the command, if available; 'cgroup-only' to use only the cgroup properties of that rule, if any; 'default' to use this special fallback preset; or any other generic type. The CGROUP argument can be a cgroup defined in configuration files. The QUOTA argument can be an integer ranging from 1 to 99 that represents a percentage relative to the total CPU time available on all cores.`,
	Args: cobra.MinimumNArgs(1),
	DisableFlagsInUseLine: true,
	PreRunE: CommonPreRunE,
	Run: func(cmd *cobra.Command, args []string) {
		viper.Set("tag", "show")
		// Debug output
		debugOutput(cmd)
		// Real job goes here
		_, err := lookPath(args[0])
		checkErr(err)
		script, err := yieldScriptFrom(viper.GetString("shell"), args)
		checkErr(err)
		for _, line := range script {
			cmd.Println(line)
		}
	},
}

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:   "run [-n] [-q|-v] [--preset PRESET|-d|-z] [--cgroup CGROUP|--cpu QUOTA] [-m] [-u] COMMAND [ARGUMENT]...",
	Short: "Run given command in pre-set execution environment",
	Long: `Run the COMMAND with its ARGUMENT(S) in a pre-set execution environment

The PRESET argument can be: 'auto' to use some specific rule for the command, if available; 'cgroup-only' to use only the cgroup properties of that rule, if any; 'default' to use this special fallback preset; or any other generic type. The CGROUP argument can be a cgroup defined in configuration files. The QUOTA argument can be an integer ranging from 1 to 99 that represents a percentage relative to the total CPU time available on all cores.`,
	Args: cobra.MinimumNArgs(1),
	DisableFlagsInUseLine: true,
	PreRunE: CommonPreRunE,
	Run: func(cmd *cobra.Command, args []string) {
		viper.Set("tag", "run")
		// Debug output
		debugOutput(cmd)
		// Real job goes here
		_, err := lookPath(args[0])
		checkErr(err)
		// No shell builtin before last exec cmdline
		script, err := yieldScriptFrom("/bin/sh", args)
		checkErr(err)

		if err := setCapabilities(true); err != nil {
			cmd.PrintErrln(err)
		}
		defer func() {
			if err := setCapabilities(false); err != nil {
				cmd.PrintErrln(err)
			}
		}()
		for _, cmdline := range script.RunCmdLines(args) {
			// Run command and do not abort on error
			if err := cmdline.Run("", nil, cmd.OutOrStdout(), cmd.ErrOrStderr()); err != nil {
				cmd.PrintErrln(err)
			}
		}
	},
}

func init() {
	// rootCmd.AddCommand(showCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// showCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	fs := showCmd.Flags()
	fs.SortFlags = false
	fs.SetInterspersed(false)

	addRunShowFlags(showCmd)

	showCmd.InheritedFlags().SortFlags = false
}

func init() {
	// rootCmd.AddCommand(runCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// runCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	fs := runCmd.Flags()
	fs.SortFlags = false
	fs.SetInterspersed(false)

	addRunShowFlags(runCmd)
	addDryRunFlag(runCmd)

	runCmd.InheritedFlags().SortFlags = false
}

// vim: set ft=go fdm=indent ts=2 sw=2 tw=79 noet:
