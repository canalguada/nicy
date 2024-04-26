/*
Copyright © 2024 David Guadalupe <guadalupe.david@gmail.com>

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

func Clone[S ~[]E, E any](s S) S {
	result := make(S, len(s))
	_ = copy(result, s)
	return result
}

func Reverse[S ~[]E, E any](s S) S {
	first := 0
	last := len(s) - 1
	for first < last {
		s[first], s[last] = s[last], s[first]
		first++
		last--
	}
	return s
}

func Map[S1 ~[]E1, E1, E2 any](s S1, f func(E1) E2) []E2 {
	r := make([]E2, len(s))
	for i, v := range s {
		r[i] = f(v)
	}
	return r
}

func Filter[S ~[]E, E any](s S, f func(E) bool) S {
	var r S
	for _, v := range s {
		if f(v) {
			r = append(r, v)
		}
	}
	return r
}

func Reduce[S ~[]E, E, T any](s S, init T, f func(T, E) T) T {
	r := init
	for _, v := range s {
		r = f(r, v)
	}
	return r
}

func ChanFirst[C chan T, T any](ch C, f func(T) bool) T {
	for v := range ch {
		if f(v) {
			return v
		}
	}
	return *new(T)
}

func ChanMapFilter[C1 chan T1, C2 chan T2, T1, T2 any](in C1, out C2, f func(T1) (T2, bool)) {
	for v := range in {
		if result, ok := f(v); ok {
			out <- result
		}
	}
}
