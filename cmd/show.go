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
	flag "github.com/spf13/pflag"
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
	// Set runtime values where needed
	switch {
	case fs.Changed("default"):
		viper.Set("preset", "default")
	case fs.Changed("cgroup-only"):
		viper.Set("preset", "cgroup-only")
	}
	if fs.Changed("cpu") {
		viper.Set("cgroup", viper.GetString("cpu"))
	}
	return nil
}

// showCmd represents the show command
var showCmd = &cobra.Command{
	Use:   "show [-q|-v] [--preset PRESET|-d|-z] [--cgroup CGROUP|--cpu QUOTA] [-m] [-u] COMMAND",
	Short: "Show effective script for given command",
	Long: `Show the effective script for the given COMMAND`,
	Args: cobra.MinimumNArgs(1),
	DisableFlagsInUseLine: true,
	PreRunE: CommonPreRunE,
	Run: func(cmd *cobra.Command, args []string) {
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
	Long: `Run the COMMAND in a pre-set execution environment`,
	Args: cobra.MinimumNArgs(1),
	DisableFlagsInUseLine: true,
	PreRunE: CommonPreRunE,
	Run: func(cmd *cobra.Command, args []string) {
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
		for _, cmdline := range script.RuntimeCommands(args) {
			disableAmbient := false
			if cmdline[0] == "$SUDO" && viper.GetInt("UID") != 0 {
				err := setAmbientSysNice(true)
				if err != nil {
					// Fallback to sudo command if CAP_SYS_NICE not in local ambient set
					cmdline[0] = viper.GetString("sudo")
					cmd.PrintErrln(err)
				} else {
					cmdline.ShrinkLeft(1)
					disableAmbient = true
				}
			}
			if viper.GetBool("dry-run") || (viper.GetInt("verbose") > 0) {
				// Write to stderr which command lines would be run
				cmd.PrintErrln(prog + ":", "run:", cmdline)
			}
			if viper.GetBool("dry-run") {
				goto ResetAmbient
			}
			if cmdline[0] == "exec" {
				err = cmdline.ExecRun()
				checkErr(err)
			// Run command and do not abort on error
			} else if err := cmdline.Run(nil, cmd.OutOrStdout(), cmd.ErrOrStderr()); err != nil {
				cmd.PrintErrln(err)
			}
			// Finally
			ResetAmbient:
				if disableAmbient {
					if err := setAmbientSysNice(false); err != nil {
						cmd.PrintErrln(err)
					}
				}
		}
	},
}

var fsRunShow *flag.FlagSet

func init() {
	fsRunShow = flag.NewFlagSet("run and show flags", flag.ExitOnError)
	fsRunShow.SortFlags = false
	fsRunShow.BoolP("quiet", "q", false, "suppress additional output")
	fsRunShow.CountP("verbose", "v", "display which command is launched")
	fsRunShow.StringP("preset", "p", "auto", "apply this `PRESET`")
	fsRunShow.BoolP("default", "d", false, "like --preset=default")
	fsRunShow.BoolP("cgroup-only", "z", false, "like --preset=cgroup-only")
	fsRunShow.StringP("cgroup", "c", "null", "run as part of this `CGROUP`")
	fsRunShow.Int("cpu", 0, "like --cgroup=cpu`QUOTA`")
	fsRunShow.BoolP("managed", "m", false, "always run inside its own scope")
	fsRunShow.BoolP("force-cgroup", "u", false, "run inside a cgroup matching properties")
	viper.BindPFlags(fsRunShow)
}

func init() {
	rootCmd.AddCommand(showCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// showCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	fsShow := showCmd.Flags()
	fsShow.SortFlags = false
	fsShow.SetInterspersed(false)

	fsShow.AddFlagSet(fsRunShow)

	showCmd.InheritedFlags().SortFlags = false
}

func init() {
	rootCmd.AddCommand(runCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// runCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	fsRun := runCmd.Flags()
	fsRun.SortFlags = false
	fsRun.SetInterspersed(false)
	fsRun.BoolP("dry-run", "n", false, "display commands but do not run them")

	viper.BindPFlags(fsRun)

	fsRun.AddFlagSet(fsRunShow)

	runCmd.InheritedFlags().SortFlags = false
}

// vim: set ft=go fdm=indent ts=2 sw=2 tw=79 noet:
