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
	"io"
	"bytes"
	"strings"
	"strconv"
	"text/tabwriter"
	"path/filepath"
	"github.com/mitchellh/go-homedir"
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

// Flags

func addRunShowFlags(cmd *cobra.Command) {
	fs := cmd.Flags()
	fs.BoolP("quiet", "q", false, "suppress additional output")
	fs.BoolP("verbose", "v", false, "be verbose when running external commands")
	fs.StringP("preset", "p", "auto", "apply this `PRESET`")
	fs.BoolP("default", "d", false, "like --preset=default")
	fs.BoolP("cgroup-only", "z", false, "like --preset=cgroup-only")
	fs.StringP("cgroup", "c", "", "run as part of this `CGROUP`")
	fs.Int("cpu", 0, "like --cgroup=cpu`QUOTA`")
	fs.BoolP("managed", "m", false, "always run inside its own scope")
	fs.BoolP("force-cgroup", "u", false, "run inside a cgroup matching properties")
}

func addDryRunFlag(cmd *cobra.Command) {
	fs := cmd.Flags()
	fs.BoolP("dry-run", "n", false, "display external commands instead running them")
}

func addDumpManageFlags(cmd *cobra.Command) {
	fs := cmd.Flags()
	fs.BoolP("user", "u", false, "only processes running inside calling user slice")
	fs.BoolP("global", "g", false, "processes running inside any user slice")
	fs.BoolP("system", "s", false, "only processes running inside system slice")
	fs.BoolP("all", "a", false, "all running processes")
}

func bindFlags(cmd *cobra.Command, names ...string) {
	fs := cmd.Flags()
	for _, name := range names {
		viper.BindPFlag(name, fs.Lookup(name))
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.
	fs := rootCmd.PersistentFlags()
	fs.StringVar(&cfgFile, "config", "", "config `file`")

	fs.StringSlice("confdirs", []string{}, "user and system presets directories")
	fs.String("libdir", "", "read-only library directory")
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

// Commands

func init() {
	cobra.EnableCommandSorting = false

	rootCmd.AddCommand(buildCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(showCmd)
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(dumpCmd)
	rootCmd.AddCommand(manageCmd)
	rootCmd.AddCommand(installCmd)
}

func printErrln(a ...interface{}) (n int, err error){
	n, err = fmt.Fprintln(os.Stderr, a...)
	return
}

func printErrf(format string, a ...interface{}) (n int, err error){
	n, err = fmt.Fprintf(os.Stderr, format, a...)
	return
}

func createDirectoryIfNotExist(name string, perm os.FileMode) error {
	if _, err := os.Stat(name); os.IsNotExist(err) {
		return os.MkdirAll(name, perm)
	}
	return nil
}

func expandPath(path string) string {
	s, err := homedir.Expand(path)
	if err != nil {
		s = path
	}
	return os.ExpandEnv(s)
}

func mergeConfigFile(path string) error {
	if exists(path) {
		configBytes, err := readFile(path)
		if err != nil {
			return err
		}
		// Find and read the config file
		if err = viper.MergeConfig(bytes.NewBuffer(configBytes)); err != nil {
			return wrapError(err)
		}
	} else {
		return fmt.Errorf("%w: %v", ErrNotFound, path)
	}
	viper.SetConfigFile(path)
	return nil
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	var (
		path string
		err error
	)
	viper.SetDefault("PROG", prog)
	// Common configuration, per order of precedence in :
	// - /usr/local/etc/%prog%
	// - /etc/%prog%
	// Get UID
	uid := os.Getuid()
	viper.SetDefault("UID", uid)
	if uid != 0 {
		//  Configuration in %XDG_CONFIG_HOME%/%prog% will take precedence
		// Find HOME
		path, err = homedir.Dir() // Required
		checkErr(wrapError(err))
		viper.SetDefault("HOME", path)
		// Then XDG_CONFIG_HOME
		path, _ = os.UserConfigDir() // No error, HOME is defined
		viper.SetDefault("XDG_CONFIG_HOME", path)
	}
	if uid != 0 {
		// Set XDG_CACHE_HOME and cachedir
		path, _ = os.UserCacheDir() // No error, HOME is defined
		viper.SetDefault("XDG_CACHE_HOME", path)
		viper.SetDefault("cachedir", filepath.Join(path, prog))
	} else {
		// Set cachedir
		viper.SetDefault("cachedir", filepath.Join("/var/cache", prog))
	}
	// Set cgroups, types, rules and database paths
	for _, name := range []string{"cgroups", "types", "rules", "database"} {
		viper.SetDefault(name, filepath.Join(viper.GetString("cachedir"), name))
	}
	// Set XDG_RUNTIME_DIR and runtimedir
	path, ok := os.LookupEnv("XDG_RUNTIME_DIR")
	if !(ok) {
		path = filepath.Join("/run/user", strconv.Itoa(uid))
	}
	viper.SetDefault("XDG_RUNTIME_DIR", path)
	viper.SetDefault("runtimedir", filepath.Join(path, prog))
	// Create required directories
	checkErr(createDirectoryIfNotExist(viper.GetString("cachedir"), 0755))
	checkErr(createDirectoryIfNotExist(viper.GetString("runtimedir"), 0755))
	// Default configuration search paths (in order of precedence)
	var configPaths []string
	if uid != 0 {
		configPaths = append(configPaths, viper.GetString("XDG_CONFIG_HOME"))
	}
	configPaths = append(configPaths, "/usr/local/etc", "/etc")
	for i, path := range configPaths {
		configPaths[i] = filepath.Join(path, prog)
	}
	// Default install path for scripts
	if uid != 0 {
		path = filepath.Join(viper.GetString("HOME"), "bin", prog)
	} else {
		path = filepath.Join("/usr/local/bin", prog)
	}
	viper.SetDefault("scripts", path)
	// Main default values
	viper.SetDefault("confdirs", configPaths)
	viper.SetDefault("libdir", filepath.Join("/usr/lib", prog))
	viper.SetDefault("shell", "/bin/sh")
	// Config files
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	// First merge existing default config files
	for i := len(configPaths) - 1; i >= 0; i-- {
		path = filepath.Join(configPaths[i], "config.yaml")
		if err = mergeConfigFile(path); err != nil {
			if viper.GetBool("debug") {
				printErrln(err)
			}
		}
	}
	// Then merge config file that user set with the flag
	if cfgFile != "" {
		if err = mergeConfigFile(cfgFile); err != nil {
			if viper.GetBool("debug") {
				printErrln(err)
			}
		}
	}
	// Allow use of environment variables
	viper.SetEnvPrefix(prog)
	viper.AutomaticEnv() // read in environment variables that match
	// Provide SUDO environment variable to jq scripts.
	os.Setenv("SUDO", viper.GetString("sudo"))
	// Debug
	if viper.GetBool("debug") {
		// Display the capabilities of the running process
		printErrln("caps:", cap.GetProc())
		if viper.ConfigFileUsed() != "" {
			printErrln("config file:", viper.ConfigFileUsed())
		}
		viper.WriteConfigAs(filepath.Join(viper.GetString("runtimedir"), "viper.yaml"))
	}
}

func init() {
	// System utilities require CAP_SYS_NICE.
	// Use sudo only when this capability is not set.
	viper.SetDefault("sudo", "sudo")
	// Use kernel.org/pub/linux/libs/security/libcap/cap
}

// More functions

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

func debugOutput(cmd *cobra.Command) {
	if viper.GetBool("debug") {
		tw := getTabWriter(cmd.ErrOrStderr())
		fmt.Fprintf(cmd.ErrOrStderr(), "command:\n%#v\n", cmd.Name())
		tw.Write([]byte("key:\tvalue\n"))
		for _, k := range viper.AllKeys() {
			tw.Write([]byte(fmt.Sprintf("%s:\t%#v\n", k, viper.Get(k))))
		}
		tw.Flush()
	}
}

func getTabWriter(output io.Writer) *tabwriter.Writer {
	// output, minwidth, tabwidth, padding, padchar, flags
	return tabwriter.NewWriter(output, 8, 8, 0, '\t', 0)
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
