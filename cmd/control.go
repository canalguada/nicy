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
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/sys/unix"
)

// controlCmd represents the control command
var controlCmd = &cobra.Command{
	Use:   "control [-n] [-u|-g|-s|-a] [-t SECONDS]",
	Short: "Control running processes",
	Long: `Control the running processes, applying rules, if any

The processes are selected when their group leader matches an existing rule.
The --user option is the implied default, when none is given.
Only superuser can fully run manage command with --system, --global or --all option.`,
	Args:                  cobra.MaximumNArgs(0),
	DisableFlagsInUseLine: true,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		fs := cmd.LocalNonPersistentFlags()
		// Bind shared flags
		err := viper.BindPFlags(fs)
		if tick := viper.GetDuration("tick"); tick.Seconds() < 2 || tick.Seconds() > 3600 {
			msg := fmt.Sprintf("must range from 2s to 1h, got %v", tick)
			return fmt.Errorf("%w: %s", ErrParse, msg)
		}
		return err
	},
	Run: func(cmd *cobra.Command, args []string) {
		viper.Set("tag", "control")
		// Debug output
		debugOutput(cmd)
		// Real job goes here
		presetCache = GetPresetCache() // get cache, once for all goroutines
		scope := GetStringFromFlags("user", viper.GetStringSlice("scopes")...)
		if err := setCapabilities(true); err != nil {
			cmd.PrintErrln(err)
		}
		defer func() {
			if err := setCapabilities(false); err != nil {
				cmd.PrintErrln(err)
			}
		}()
		filter := GetScopeOnlyFilterer(scope)
		err := doControlCmd("", filter, cmd.OutOrStdout(), cmd.ErrOrStderr())
		fatal(wrap(err))
	},
}

func init() {
	// Persistent flags
	// Local flags
	fs := controlCmd.Flags()
	fs.SortFlags = false
	fs.SetInterspersed(false)
	viper.Set("scopes", addScopeFlags(controlCmd))
	addDryRunFlag(controlCmd)
	fs.DurationP("tick", "t", 5*time.Second, "delay between consecutive runs in seconds")
	controlCmd.InheritedFlags().SortFlags = false
}

func doControlCmd(tag string, filter ProcFilterer, stdout, stderr io.Writer) (err error) {
	// prepare channels
	runjobs := make(chan *ProcGroupJob, 8)
	procs := make(chan []*Proc, 8)
	// and signal
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, unix.SIGUSR1)
	ticker := time.NewTicker(viper.GetDuration("tick"))
	defer func() {
		signal.Stop(signalChan)
		ticker.Stop()
		cancel()
	}()
	// spin up workers
	wg := getWaitGroup() // use a sync.WaitGroup to indicate completion
	for i := 0; i < (goMaxProcs + 1); i++ {
		wg.Add(1) // run jobs
		go func() {
			defer wg.Done()
			for job := range runjobs {
				err = job.Run(tag, stdout, stderr)
				if err != nil {
					return
				}
			}
		}()
	}
	wg.Add(1) // get jobs
	go presetCache.GenerateGroupJobs(procs, runjobs, &wg)
	// send input
	if viper.GetBool("dry-run") || viper.GetBool("verbose") {
		inform("", fmt.Sprintf("Setting %v every %v...", filter, viper.GetDuration("tick")))
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case s := <-signalChan:
				switch s {
				case unix.SIGUSR1:
					// TODO: Reload cache here
					if viper.GetBool("dry-run") || viper.GetBool("verbose") {
						inform("", "Reloading cache...")
						presetCache.LoadFromConfig()
						inform("", "Cache reloaded.")
					}
				case os.Interrupt:
					cancel()
					os.Exit(1)
				}
			case <-ctx.Done():
				if viper.GetBool("dry-run") || viper.GetBool("verbose") {
					inform("", "Done.")
				}
				break
				// os.Exit(0)
			}
		}
	}()
	procs <- FilteredProcs(filter)
	for {
		select {
		case <-ctx.Done():
			break
			// close(output)
			// return
		case <-ticker.C:
			procs <- FilteredProcs(filter)
		}
	}
	close(procs)
	wg.Wait() // wait on the workers to finish
	return
}

// vim: set ft=go fdm=indent ts=2 sw=2 tw=79 noet:
