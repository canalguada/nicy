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
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"

	"github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	prog     = "nicy"
	version  = "0.2.1"
	confName = "v" + version
	confType = "yaml"
)

var (
	cfgFile     string      // user configuration file
	presetCache PresetCache // content cache
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   fmt.Sprintf("%s [command [arguments]]", prog),
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
	fatal(rootCmd.Execute())
}

// Flags

func addVerboseFlag() (names []string) {
	fs := rootCmd.PersistentFlags()
	fs.BoolP("verbose", "v", false, "be verbose")
	fs.BoolP("quiet", "q", false, "suppress additional output")
	names = append(names, "quiet", "verbose")
	rootCmd.MarkFlagsMutuallyExclusive(names...)
	return
}

func addCacheFlag() (names []string) {
	fs := rootCmd.PersistentFlags()
	fs.BoolP("force", "f", false, "ignore existing cache and forcefully build new")
	names = append(names, "force")
	return
}

func init() {
	cobra.OnInitialize(initConfig)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.
	fs := rootCmd.PersistentFlags()
	fs.StringVar(&cfgFile, "config", "", "config `file`")
	addVerboseFlag()
	addCacheFlag()

	fs.StringSlice("confdirs", []string{}, "user and system presets directories")
	fs.BoolP("debug", "D", false, "show debug output")
	fs.MarkHidden("confdirs")
	fs.MarkHidden("debug")

	fs.SortFlags = false

	viper.BindPFlags(fs)

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	// rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	rootCmd.Flags().SortFlags = false

	// Commands
	cobra.EnableCommandSorting = false

	// Move instructions here to preserve order
	rootCmd.AddCommand(buildCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(showCmd)
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(dumpCmd)
	rootCmd.AddCommand(setCmd)
	rootCmd.AddCommand(controlCmd)
	rootCmd.AddCommand(installCmd)
}

// Functions

func exists(path string) bool {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false
	}
	return true
}

func expandPath(path string) string {
	path = os.ExpandEnv(path)
	if s, err := homedir.Expand(path); err == nil {
		return s
	}
	return path
}

func mergeConfigFile(path string) (err error) {
	if exists(path) {
		var configBytes []byte
		if configBytes, err = ioutil.ReadFile(path); err == nil {
			// Find and read the config file
			if err = viper.MergeConfig(bytes.NewBuffer(configBytes)); err == nil {
				viper.SetConfigFile(path)
				return
			}
			return wrap(err)
		}
		return wrap(err)
	}
	return fmt.Errorf("%w: %v", ErrNotFound, path)
}

func getUserDefaults() (int, string, string, string, string) {
	var (
		err     error
		uid     int
		home    string
		config  string
		cache   string
		runtime string
	)
	// Get UID
	uid = os.Getuid()
	if uid != 0 {
		//  Configuration in %XDG_CONFIG_HOME%/%prog% will take precedence
		home, err = homedir.Dir() // Find HOME
		fatal(wrap(err))
		config, _ = os.UserConfigDir() // Then XDG_CONFIG_HOME
		cache, _ = os.UserCacheDir()   // Then XDG_CACHE_HOME
	}
	runtime, ok := os.LookupEnv("XDG_RUNTIME_DIR") // Set XDG_RUNTIME_DIR
	if !(ok) {
		runtime = filepath.Join("/run/user", strconv.Itoa(uid))
		fatal(os.MkdirAll(runtime, 0700))
	}
	return uid, home, config, runtime, cache
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	uid, home, configHome, runtimePath, _ := getUserDefaults()
	// Common configuration, per order of precedence in :
	// - $XDG_CONFIG_HOME/%prog%
	// - /usr/local/etc/%prog%
	// - /etc/%prog%
	viper.SetDefault("runtimedir", filepath.Join(runtimePath, prog))
	viper.SetDefault("cache", filepath.Join(viper.GetString("runtimedir"), "cache.yaml"))
	// Create required directories
	fatal(os.MkdirAll(viper.GetString("runtimedir"), 0755))
	// Default configuration search paths (in order of precedence)
	configPaths := []string{configHome, "/usr/local/etc", "/etc"}
	if uid == 0 {
		configPaths = configPaths[1:]
	}
	for i, path := range configPaths {
		configPaths[i] = filepath.Join(path, prog)
	}
	// Default install path for scripts
	if uid != 0 {
		viper.SetDefault("scripts.location", filepath.Join(home, "bin", prog))
	} else {
		viper.SetDefault("scripts.location", filepath.Join("/usr/local/bin", prog))
	}
	// System utilities require capabilities.
	// Use sudo only when not available.
	viper.SetDefault("sudo", "sudo")
	// Use kernel.org/pub/linux/libs/security/libcap/cap
	// Main default values
	viper.SetDefault("confdirs", configPaths)
	// Default shell
	viper.SetDefault("shell", "/bin/bash")
	// Config files
	viper.Set("version", version)
	viper.SetConfigName(confName)
	viper.SetConfigType(confType)
	// First merge existing default config files
	for i := len(configPaths) - 1; i >= 0; i-- {
		if err := mergeConfigFile(filepath.Join(configPaths[i], confName+"."+confType)); err != nil {
			debug(err)
		}
	}
	// Then merge config file that user set with the flag
	if cfgFile != "" {
		if err := mergeConfigFile(cfgFile); err != nil {
			fatal(err)
		}
	}
	// Allow use of environment variables
	viper.SetEnvPrefix(prog)
	viper.AutomaticEnv() // read in environment variables that match
	// Provide SUDO environment variable.
	os.Setenv("SUDO", viper.GetString("sudo"))
	// Update values
	viper.Set("scripts.location", expandPath(viper.GetString("scripts.location")))
	viper.Set("uid", uid)
	// Debug
	// Display the capabilities of the running process
	debug(getCapabilities())
	if viper.ConfigFileUsed() != "" {
		debug("config file:", viper.ConfigFileUsed())
	}
	viper.WriteConfigAs(filepath.Join(viper.GetString("runtimedir"), confName+"."+confType))
}

// vim: set ft=go fdm=indent ts=2 sw=2 tw=79 noet:
