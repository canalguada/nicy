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
	"reflect"

	"gopkg.in/yaml.v2"
)

type Preset interface {
	Cgroup | Profile | Rule
	Keys() (string, string)
	String() string
}

func ActivePreset[T Preset](s []T) (elem T) {
	if len(s) > 0 {
		elem = s[len(s)-1]
	}
	return
}

func IterCache[T Preset](m map[string][]T) chan T {
	ch := make(chan T)
	go func(ch chan T) {
		for _, slice := range m {
			ch <- ActivePreset(slice)
		}
		close(ch)
	}(ch)
	return ch
}

func List[T Preset](m map[string][]T) (result []string) {
	for item := range IterCache(m) {
		tag, origin := item.Keys()
		result = append(result, fmt.Sprintf("%s\t%s\t%s", tag, origin, item.String()))
	}
	return
}

func ListFrom[T Preset](m map[string][]T, required string) (result []string) {
	for item := range IterCache(m) {
		tag, origin := item.Keys()
		if origin != required {
			continue
		}
		result = append(result, fmt.Sprintf("%s\t%s\t%s", tag, origin, item.String()))
	}
	return
}

func HasPreset[T Preset](m map[string][]T, key string) bool {
	if slice, found := m[key]; found {
		return len(slice) > 0
	}
	return false
}

func GetPreset[T Preset](m map[string][]T, key string, kind string) (T, error) {
	if slice, found := m[key]; found && len(slice) > 0 {
		return ActivePreset(slice), nil
	}
	return *new(T), notFound(kind, key)
}

func LoadConfig[T Preset](m map[string][]T, f func() chan T) {
	for item := range f() {
		tag, _ := item.Keys()
		if _, found := m[tag]; found {
			m[tag] = append(m[tag], item)
		} else {
			m[tag] = []T{item}
		}
	}
}

type Struct interface {
	BaseCgroup | Cgroup | BaseProfile | Profile | BaseRule | Rule
}

func ToInterface[T Struct](st T) map[string]any {
	m := make(map[string]any)
	data, err := yaml.Marshal(st)
	if err := failed(err); err == nil {
		nonfatal(failed(yaml.Unmarshal(data, &m)))
	}
	return m
}

func ToJson[T Struct](st T) string {
	data, err := json.Marshal(st)
	nonfatal(wrap(err))
	return string(data)
}

// ResetMatching iterate through st1 fields and, if set, resets
// matching fields in st2.
func ResetMatching[T1, T2 Struct](st1 *T1, st2 *T2) {
	s1 := reflect.ValueOf(st1).Elem()
	s2 := reflect.ValueOf(st2).Elem()
	typeOfT1 := s1.Type()
	for i := 0; i < s1.NumField(); i++ {
		if f1 := s1.Field(i); f1.IsValid() && !f1.IsZero() {
			if f2 := s2.FieldByName(typeOfT1.Field(i).Name); f2.IsValid() {
				f2.Set(reflect.Zero(f2.Type()))
			}
		}
	}
}

type BaseStruct interface {
	BaseCgroup | BaseProfile | BaseRule
}

// UpdateRule iterates through st fields and, if set, updates
// matching fields in rule, if not set yet.
func UpdateRule[T BaseStruct](st T, rule *Rule) {
	s := reflect.ValueOf(&st).Elem()
	r := reflect.ValueOf(rule).Elem()
	typeOfT := s.Type()
	for i := 0; i < s.NumField(); i++ {
		if f1 := s.Field(i); f1.IsValid() && !f1.IsZero() { // f1 is set
			// Find matching f2 field. Is it set ?
			if f2 := r.FieldByName(typeOfT.Field(i).Name); f2.IsValid() && f2.IsZero() {
				f2.Set(f1)
			}
		}
	}
}

func Properties[T BaseStruct](st T) (result []string) {
	base := reflect.ValueOf(&st).Elem()
	typeOfBase := base.Type()
	for i := 0; i < base.NumField(); i++ {
		if f := base.Field(i); f.IsValid() && !f.IsZero() {
			name := typeOfBase.Field(i).Name
			result = append(result, fmt.Sprintf("%s=%s", name, f.Interface()))
		}
	}
	return
}

func Diff[T BaseStruct](preset T, runtime T) (T, int) {
	result := new(T)
	diff := reflect.ValueOf(result).Elem()
	count := 0
	effective := reflect.ValueOf(&runtime).Elem()
	required := reflect.ValueOf(&preset).Elem()
	for i := 0; i < required.NumField(); i++ {
		if f := required.Field(i); f.IsValid() && !f.IsZero() {
			if effective.Field(i).Interface() != f.Interface() {
				diff.Field(i).Set(f)
				count++
			}
		}
	}
	return *result, count
}
