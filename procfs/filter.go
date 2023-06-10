// build +linux

/*
Copyright © 2022 David Guadalupe <guadalupe.david@gmail.com>

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
package procfs

import (
	"os"
	"strings"
)

type Filterer[P Proc | ProcStat] interface {
	Filter(p *P, err error) bool
	String() string
}

type FilterAny[T any] struct {
	Filter  func(p *T, err error) bool
	Message string
}
type FilterProc = FilterAny[Proc]

type ProcFilter struct {
	FilterProc
	Scope string
}

func (pf ProcFilter) Filter(p *Proc, err error) bool {
	return pf.FilterProc.Filter(p, err)
}

func (pf ProcFilter) String() string {
	return pf.Message
}

func NewProcScopeFilter(scope string) Filterer[Proc] {
	var inner FilterAny[Proc]
	switch strings.ToLower(scope) {
	case "global":
		inner = FilterAny[Proc]{
			Filter: func(p *Proc, err error) bool {
				return err == nil && p.InUserSlice()
			},
			Message: "processes inside any user slice",
		}
	case "system":
		inner = FilterAny[Proc]{
			Filter: func(p *Proc, err error) bool {
				return err == nil && p.InSystemSlice()
			},
			Message: "processes inside system slice",
		}
	case "all":
		inner = FilterAny[Proc]{
			Filter: func(p *Proc, err error) bool {
				return err == nil
			},
			Message: "all processes",
		}
	case "user":
		inner = FilterAny[Proc]{
			Filter: func(p *Proc, err error) bool {
				return err == nil && p.Uid == os.Getuid() && p.InUserSlice()
			},
			Message: "calling user processes",
		}
	}
	return Filterer[Proc](ProcFilter{FilterProc: inner, Scope: scope})
}

func GetFilterer(scope string) Filterer[Proc] {
	scope = strings.ToLower(scope)
	switch scope {
	case "global", "system", "all":
		return NewProcScopeFilter(scope)
	default:
		return NewProcScopeFilter("user")
	}
}

// vim: set ft=go fdm=indent ts=2 sw=2 tw=79 noet:
