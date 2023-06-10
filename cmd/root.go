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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strconv"

	"github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	prog     = "nicy"
	version  = "0.2.0"
	confName = "v" + version
	confType = "yaml"
)

var (
	logger      = log.New(os.Stderr, prog+": ", 0)
	cfgFile     string      // user configuration file
	presetCache PresetCache // content cache
)

func debug(v ...any) {
	if viper.GetBool("debug") {
		lv := []any{"debug:"}
		lv = append(lv, v...)
		notify(lv...)
	}
}

func warn(v ...any) {
	lv := []any{"error:"}
	lv = append(lv, v...)
	notify(lv...)
}

func notify(v ...any) {
	logger.Println(v...)
}

func inform(tag string, v ...any) {
	var lv []any
	if len(viper.GetString("tag")) > 0 {
		lv = append(lv, viper.GetString("tag")+":")
	}
	if viper.GetBool("dry-run") {
		lv = append(lv, "dry-run:")
	}
	if len(tag) > 0 {
		lv = append(lv, tag+":")
	}
	if len(v) > 0 {
		lv = append(lv, v...)
	}
	notify(lv...)
}

func trace(tag string, subkey string, arg any) {
	if viper.GetBool("verbose") && viper.GetBool("debug") {
		s := []any{"debug:"}
		if len(subkey) > 0 {
			s = append(s, subkey+`:`)
		}
		s = append(s, fmt.Sprintf("%v", arg))
		if len(tag) > 0 {
			s = append(s, fmt.Sprintf("(%s)", tag))
		}
		notify(s...)
	}
}

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

func addCacheFlag(cmd *cobra.Command) {
	fs := cmd.Flags()
	fs.BoolP("force", "f", false, "ignore old cache or forcefully build new")
}

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

// func bindFlags(cmd *cobra.Command, names ...string) {
//   fs := cmd.Flags()
//   for _, name := range names {
//     viper.BindPFlag(name, fs.Lookup(name))
//   }
// }

func addVerboseFlag() (names []string) {
	fs := rootCmd.PersistentFlags()
	fs.BoolP("verbose", "v", false, "be verbose")
	fs.BoolP("quiet", "q", false, "suppress additional output")
	names = append(names, "quiet", "verbose")
	rootCmd.MarkFlagsMutuallyExclusive(names...)
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

func createDirectoryIfNotExist(dirname string, perm os.FileMode) error {
	if !(exists(dirname)) {
		return os.MkdirAll(dirname, perm)
	}
	return nil
}

func expandPath(path string) string {
	if s, err := homedir.Expand(path); err == nil {
		path = s
	}
	return os.ExpandEnv(path)
}

func mergeConfigFile(path string) (err error) {
	if exists(path) {
		var configBytes []byte
		if configBytes, err = ioutil.ReadFile(path); err == nil {
			// Find and read the config file
			if err = viper.MergeConfig(bytes.NewBuffer(configBytes)); err == nil {
				viper.SetConfigFile(path)
				return
				// // Read presets
				// presetConfig := Config{
				//   Cgroups:   make(map[string]BaseCgroup),
				//   AppGroups: make(map[string]AppGroup),
				//   Rules:     make(map[string]AppRule),
				// }
				// if err = viper.UnmarshalKey("presets", &presetConfig); err == nil {
				//   // Build cache
				//   presetConfig.SetOrigin(path)
				//   // TODO: Reverse all slices created here ?
				//   presetCache.Load(presetConfig)
				//   // Empty presets since moved to cache
				//   viper.Set("presets", make(map[string]any))
				//   return
				// }
				// return wrap(err)
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
		// Find HOME
		home, err = homedir.Dir() // Required
		fatal(wrap(err))
		// Then XDG_CONFIG_HOME
		config, _ = os.UserConfigDir() // No error, HOME is defined
		// Then XDG_CACHE_HOME
		cache, _ = os.UserCacheDir() // No error, HOME is defined
	}
	// Set XDG_RUNTIME_DIR
	runtime, ok := os.LookupEnv("XDG_RUNTIME_DIR")
	if !(ok) {
		runtime = filepath.Join("/run/user", strconv.Itoa(uid))
		fatal(createDirectoryIfNotExist(runtime, 0700))
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
	fatal(createDirectoryIfNotExist(viper.GetString("runtimedir"), 0755))
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
	// presetCache = NewPresetCache()
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
	// Debug
	// Display the capabilities of the running process
	debug(getCapabilities())
	if viper.ConfigFileUsed() != "" {
		debug("config file:", viper.ConfigFileUsed())
	}
	viper.WriteConfigAs(filepath.Join(viper.GetString("runtimedir"), confName+"."+confType))
}

// More functions

func debugOutput(cmd *cobra.Command) {
	if viper.GetBool("debug") {
		config := viper.AllSettings()
		config["command"] = cmd.Name()
		data, err := json.Marshal(config)
		if err != nil {
			warn(err)
		} else {
			debug(string(data))
		}
	}
}

func GetStringFromFlags(fallback string, names ...string) string {
	for _, name := range names {
		if viper.GetBool(name) {
			return name
		}
	}
	return fallback
}

// vim: set ft=go fdm=indent ts=2 sw=2 tw=79 noet:
