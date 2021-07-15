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
package jq

import (
	"fmt"
	"os"
	"errors"
	"github.com/itchyny/gojq"
)

var (
	ErrUsage = errors.New("gojq usage")
	ErrCompile = errors.New("gojq compile")
)

type Request struct {
	Script string
	LibDirs []string
	Variables []string
	Values []interface{}
	IterFunc func(value interface{}) interface{}
}

func NewRequest(script string, vars []string, values ...interface{}) *Request {
	return &Request{
		Script: script,
		LibDirs: []string{},
		Variables: vars,
		Values: values,
		IterFunc: func (v interface{}) interface{} {
			return v
		},
	}
}

func (req *Request) Output(input []interface{}) (output []interface{}, err error) {
	if len(req.Script) == 0 {
		// Identity filter
		req.Script = "."
	}
	query, err := gojq.Parse(req.Script)
	if err != nil {
		err = fmt.Errorf("%w: %v", ErrUsage, err)
		return
	}
	options := []gojq.CompilerOption{gojq.WithEnvironLoader(os.Environ)}
	if len(req.LibDirs) > 0 {
		moduleLoader := gojq.NewModuleLoader(req.LibDirs)
		options = append(options, gojq.WithModuleLoader(moduleLoader))
	}
	if len(req.Variables) > 0 {
		options = append(options, gojq.WithVariables(req.Variables))
	}
	code, err := gojq.Compile(query, options...)
	if err != nil {
		err = fmt.Errorf("%w: %v", ErrCompile, err)
		return
	}
	iter := code.Run(input, req.Values...)
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if err, ok = v.(error); ok {
			if err != nil {
				return
			}
		}
		output = append(output, req.IterFunc(v))
	}
	return
}

// vim: set ft=go fdm=indent ts=2 sw=2 tw=79 noet:
