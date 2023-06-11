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
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	logger = log.New(os.Stderr, prog+": ", 0)
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

// vim: set ft=go fdm=indent ts=2 sw=2 tw=79 noet:
