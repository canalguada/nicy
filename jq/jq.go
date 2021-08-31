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
	code *gojq.Code
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

type emptyError interface {
	IsEmptyError() bool
}

func (req *Request) Compile(force bool) (err error){
	if !(force) && req.code != nil {
		return
	}
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
	req.code = code
	return nil
}

func (req *Request) Code() (code *gojq.Code, err error) {
	if req.code == nil {
		if err = req.Compile(true); err != nil {
			return
		}
	}
	code = req.code
	return
}

func (req *Request) iterate(iter gojq.Iter) (result []interface{}, err error) {
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if err, ok = v.(error); ok {
			if e, ok := v.(emptyError); ok && e.IsEmptyError() {
				err = nil
				break
			} else if err != nil {
				return
			}
		}
		result = append(result, req.IterFunc(v))
	}
	return
}

func (req *Request) runQuery(input interface{}) (result []interface{}, err error) {
	if len(req.Script) == 0 {
		// Identity filter
		req.Script = "."
	}
	query, err := gojq.Parse(req.Script)
	if err != nil {
		err = fmt.Errorf("%w: %v", ErrUsage, err)
		return
	}
	return req.iterate(query.Run(interface{}(input)))
}

func (req *Request) runCode(input interface{}) (result []interface{}, err error) {
	if err = req.Compile(false); err != nil {
		return
	}
	return req.iterate(req.code.Run(interface{}(input), req.Values...))
}

func (req *Request) Result(input interface{}) ([]interface{}, error) {
	return req.runCode(interface{}(input))
}

func (req *Request) QueryOnlyResult(input interface{}) ([]interface{}, error) {
	return req.runQuery(interface{}(input))
}

// vim: set ft=go fdm=indent ts=2 sw=2 tw=79 noet:
