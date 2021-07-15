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
	"os"
	"strings"
	"text/tabwriter"
	"path"
	"github.com/spf13/cobra"
	"kernel.org/pub/linux/libs/security/libcap/cap"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const (
	prog = "nicy"
	version = "0.1.6"
)

var cfgFile string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   prog,
	Short: "Set the execution environment and configure the resources that spawned and running processes are allowed to share",
	Long: `nicy can be used to ease the control upon the execution environment of the managed
processes and to configure the available resources, applying them generic or
specific presets.

nicy can alter their CPU scheduling priority, set their real-time scheduling 
attributes or their I/O scheduling class and priority, and adjust their
Out-Of-Memory killer score setting.

nicy can start a transient systemd scope and either run the specified
command and its spawned processes within, or move running processes inside it.

nicy can also automatically change environment variables and add arguments when
launching a command.`,
	Version: version,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	// Run: func(cmd *cobra.Command, args []string) { },
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	checkErr(rootCmd.Execute())
}

func init() {
	cobra.OnInitialize(initConfig)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.
	fs := rootCmd.PersistentFlags()
	fs.StringVar(&cfgFile, "config", "", "config `file`")

	fs.StringSlice("confdirs", []string{}, "user and system presets `directories`")
	fs.String("libdir", "", "read-only library `directory`")
	fs.BoolP("debug", "D", false, "show debug output")

	fs.MarkHidden("confdirs")
	fs.MarkHidden("libdir")
	fs.MarkHidden("debug")

	fs.SortFlags = false

	viper.BindPFlags(fs)

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	// rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	rootCmd.Flags().SortFlags = false
}

func printErrln(a ...interface{}) (n int, err error){
	n, err = fmt.Fprintln(os.Stderr, a...)
	return
}

func printErrf(format string, a ...interface{}) (n int, err error){
	n, err = fmt.Fprintf(os.Stderr, format, a...)
	return
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	// TODO: Manage defaults and config paths for superuser
	viper.SetDefault("PROG", prog)
	// Find home directory.
	home, err := os.UserHomeDir()
	checkErr(wrapError(err))
	viper.SetDefault("HOME", home)
	// Set some default values
	// Find XDG_CONFIG_HOME directory
	configHome, err := os.UserConfigDir()
	checkErr(wrapError(err))
	viper.SetDefault("XDG_CONFIG_HOME", configHome)
	// UID
	viper.SetDefault("UID", os.Getuid())
	// XDG_RUNTIME_DIR and runtimedir
	dir, ok := os.LookupEnv("XDG_RUNTIME_DIR")
	if !(ok) {
		dir = path.Join("/run/user", fmt.Sprintf("%d", viper.GetInt("UID")))
	}
	viper.SetDefault("XDG_RUNTIME_DIR", dir)
	viper.SetDefault("runtimedir", path.Join(dir, prog))
	// XDG_CACHE_HOME and cachedir
	dir, err = os.UserCacheDir()
	checkErr(wrapError(err))
	viper.SetDefault("XDG_CACHE_HOME", dir)
	cachedir := path.Join(dir, prog)
	viper.SetDefault("cachedir", cachedir)
	// cgroups, types, rules and database
	for _, filename := range []string{"cgroups", "types", "rules", "database"} {
		viper.SetDefault(filename, path.Join(cachedir, filename))
	}
	// Config file
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Search config in XDG_CONFIG_HOME directory with name prog (without extension).
		viper.AddConfigPath(configHome)
		viper.SetConfigType("yaml")
		viper.SetConfigName(prog)
	}
	//Environment variables
	viper.SetEnvPrefix(prog)
	viper.AutomaticEnv() // read in environment variables that match
	// Default values
	viper.SetDefault(
		"confdirs",
		[]string{
			path.Join(configHome, prog),
			path.Join("/usr/local/etc", prog),
			path.Join("/etc", prog),
		},
	)
	viper.SetDefault("libdir", path.Join("/usr/lib", prog))
	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil && viper.GetBool("debug") {
		printErrln("Using config file:", viper.ConfigFileUsed())
	}
	// Debug
	if viper.GetBool("debug") {
		viper.WriteConfigAs(path.Join(viper.GetString("runtimedir"), "viper.yaml"))
	}
	// Provide SUDO environment variable to jq scripts.
	os.Setenv("SUDO", viper.GetString("sudo"))
	// Display the capabilities of the running process
	if viper.GetBool("debug") {
		printErrln("Current process has these caps:", cap.GetProc())
	}

	// rootCmd.SetOut(os.Stdout)
	// rootCmd.SetErr(os.Stderr)
}

func init() {
	// System utilities require CAP_SYS_NICE.
	// Use sudo only when this capability is not set.
	viper.SetDefault("sudo", "sudo")
	// Use kernel.org/pub/linux/libs/security/libcap/cap
}

func setCapabilities(enable bool) error {
	c := cap.GetProc()
	if err := c.SetFlag(cap.Effective, enable, cap.SETPCAP, cap.SYS_NICE); err != nil {
		return fmt.Errorf("unable to set capability: %v", err)
	}
	if err := c.SetFlag(cap.Inheritable, enable, cap.SYS_NICE); err != nil {
		return fmt.Errorf("unable to set capability: %v", err)
	}
	if err := c.SetProc(); err != nil {
		return fmt.Errorf("unable to raise capabilities %q: %v", c, err)
	}
	return nil
}

func setAmbientSysNice(enable bool) (err error) {
	// Set CAP_SYS_NICE in local ambient set
	err = cap.SetAmbient(true, cap.SYS_NICE)
	return
	// // Get ambient
	// ok, err := cap.GetAmbient(cap.SYS_NICE)
	// switch {
	// case err != nil:
	//   return false, err
	// case !(ok):
	//   if viper.GetBool("debug") {
	//     printErrln(prog + ": run: CAP_SYS_NICE is not in local ambient set")
	//   }
	//   return false, nil
	// default:
	//   return true, nil
	// }
}

func debugOutput(cmd *cobra.Command) *tabwriter.Writer {
	// Initialize tabwriter
	// output, minwidth, tabwidth, padding, padchar, flags
	w := tabwriter.NewWriter(cmd.ErrOrStderr(), 8, 8, 0, '\t', 0)
	// Debug output
	if viper.GetBool("debug") {
		w.Write([]byte("Viper key:\tValue\n"))
		for _, k := range viper.AllKeys() {
			w.Write([]byte(fmt.Sprintf("%s:\t%#v\n", k, viper.Get(k))))
		}
		w.Flush()
	}
	return w
}

// checkConsistency returns an error if two or more mutually exclusive flags
// have be been set.
func checkConsistency(fs *flag.FlagSet, flagNames []string) error {
	var msg string
	count := 0
	changed := make([]string, 0)
	for _, name := range flagNames {
		if fs.Changed(name) {
			count++
			changed = append(changed, name)
		}
	}
	switch count {
	case 0:
		return nil
	case 1:
		return nil
	case 2:
		msg = fmt.Sprintf("--%s and --%s", changed[0], changed[1])
	default:
		last := len(changed) - 1
		msg = strings.Join(
			func(names []string) []string {
				result := make([]string, 0)
				for _, name := range names {
					result = append(result, "--" + name)
				}
				return result
			}(changed[:last]),
			", ",
		)
		msg = fmt.Sprintf("%s and --%s", msg, changed[last])
	}
	msg += " are mutually exclusive"
	return fmt.Errorf("%w: %v", ErrParse, msg)
}

// vim: set ft=go fdm=indent ts=2 sw=2 tw=79 noet:
