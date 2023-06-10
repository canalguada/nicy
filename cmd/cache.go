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
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
)

type PresetCache struct {
	Date     string               `yaml:"date" json:"date"`
	Cgroups  map[string][]Cgroup  `yaml:"cgroups,flow" json:"cgroups"`
	Profiles map[string][]Profile `yaml:"profiles,flow" json:"profiles"`
	Rules    map[string][]Rule    `yaml:"rules,flow" json:"rules"`
	Origin   string               `yaml:"-" json:"-"`
}

func NewPresetCache() PresetCache {
	return PresetCache{
		Date:     timestamp(),
		Cgroups:  make(map[string][]Cgroup),
		Profiles: make(map[string][]Profile),
		Rules:    make(map[string][]Rule),
	}
}

type Preset interface {
	Cgroup | Profile | Rule
	Keys() (string, string)
	String() string
}

func ActivePreset[T Preset](s []T) (elem T) {
	if len(s) > 0 {
		elem = s[len(s)-1]
		// elem = s[0]
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
	slice, found := m[key]
	return found && len(slice) > 0
}

func GetPreset[T Preset](m map[string][]T, key string, kind string) (T, error) {
	if slice, found := m[key]; found && len(slice) > 0 {
		return ActivePreset(slice), nil
	}
	return *new(T), notFound(kind, key)
}

func Reverse[T any](s []T) []T {
	first := 0
	last := len(s) - 1
	for first < last {
		s[first], s[last] = s[last], s[first]
		first++
		last--
	}
	return s
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

func (pc *PresetCache) LoadFromCache(cacheFile string) (err error) {
	var data []byte
	if exists(cacheFile) {
		if data, err = os.ReadFile(cacheFile); err == nil {
			return yaml.Unmarshal(data, &pc)
		}
		return
	}
	return fmt.Errorf("%w: %v", ErrNotFound, cacheFile)
}

func (pc *PresetCache) GetContent() (data []byte, err error) {
	return yaml.Marshal(*pc)
}

func (pc *PresetCache) WriteCache(cacheFile string) (err error) {
	var data []byte
	if data, err = pc.GetContent(); err == nil {
		err = os.WriteFile(cacheFile, data, 0600)
	}
	return
}

func (pc *PresetCache) LoadFromConfig() (err error) {
	var cfg Config
	confdirs := viper.GetStringSlice("confdirs")
	for _, root := range Reverse(confdirs) {
		path := filepath.Join(root, confName+"."+confType)
		if exists(path) {
			inform("cache", "reading configuration file", path+"...")
			if cfg, err = NewConfig(path); err == nil {
				wg := getWaitGroup()
				wg.Add(3)
				go func() {
					defer wg.Done()
					LoadConfig(pc.Cgroups, cfg.IterCgroups)
				}()
				go func() {
					defer wg.Done()
					LoadConfig(pc.Profiles, cfg.IterProfiles)
				}()
				go func() {
					defer wg.Done()
					LoadConfig(pc.Rules, cfg.IterRules)
				}()
				wg.Wait()
				continue
			}
			nonfatal(err)
		}
	}
	pc.Origin = "config"
	// if viper.GetBool("force") {
	//   pc.WriteCache(viper.GetString("cache"))
	// }
	return
}

func GetPresetCache() PresetCache {
	var err error
	pc := NewPresetCache()
	if !viper.GetBool("force") {
		if err = pc.LoadFromCache(viper.GetString("cache")); err == nil {
			pc.Origin = "file"
			return pc
		}
	}
	if viper.GetBool("verbose") && err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			inform("warning", err.Error())
		default:
			nonfatal(err)
		}
	}
	pc.LoadFromConfig()
	return pc
}

func (pc *PresetCache) List(category string) (result []string, err error) {
	switch category {
	case "cgroup":
		result = List(pc.Cgroups)
	case "profile":
		result = List(pc.Profiles)
	case "rule":
		result = List(pc.Rules)
	default:
		err = fmt.Errorf("%w: bad category: %s", ErrInvalid, category)
	}
	return
}

func (pc *PresetCache) ListFrom(category, origin string) (result []string, err error) {
	switch category {
	case "cgroup":
		result = ListFrom(pc.Cgroups, origin)
	case "profile":
		result = ListFrom(pc.Profiles, origin)
	case "rule":
		result = ListFrom(pc.Rules, origin)
	default:
		err = fmt.Errorf("%w: bad category: %s", ErrInvalid, category)
	}
	return
}

func (pc *PresetCache) HasPreset(category string, key string) bool {
	switch category {
	case "cgroup":
		return HasPreset(pc.Cgroups, key)
	case "profile":
		return HasPreset(pc.Profiles, key)
	case "rule":
		return HasPreset(pc.Rules, key)
	}
	return false
}

func notFound(category string, key string) error {
	err := fmt.Errorf("%s not found: %s", category, key)
	if viper.GetBool("debug") && viper.GetBool("verbose") {
		inform("warning", err.Error())
	}
	return err
}

func (pc *PresetCache) Rule(key string) (Rule, error) {
	return GetPreset(pc.Rules, key, "rule")
}

func (pc *PresetCache) Profile(key string) (Profile, error) {
	return GetPreset(pc.Profiles, key, "profile")
}

func (pc *PresetCache) Cgroup(key string) (Cgroup, error) {
	return GetPreset(pc.Cgroups, key, "cgroup")
}

func (pc *PresetCache) GetRawRule(input *RunInput) (Rule, error) {
	switch input.Preset {
	case "auto", "cgroup-only":
		if rule, err := pc.Rule(input.Name); err == nil {
			return rule, nil
		} else {
			if profile, err := pc.Profile("default"); err == nil {
				return profile.ToRule(), nil
			} else {
				return Rule{}, err
			}
		}
	default:
		if profile, err := pc.Profile(input.Preset); err == nil {
			return profile.ToRule(), nil
		} else {
			return Rule{}, err
		}
	}
}

func (pc *PresetCache) ExpandRule(rule *Rule) error {
	if rule.HasProfileKey() {
		profile, err := pc.Profile(rule.ProfileKey)
		if err != nil {
			return err
		}
		UpdateRule(profile.BaseProfile, rule)
		if !rule.HasCgroupKey() && profile.CgroupKey != "" {
			rule.CgroupKey = profile.CgroupKey
			cgroup, err := pc.Cgroup(profile.CgroupKey)
			if err != nil {
				return err
			}
			UpdateRule(cgroup.BaseCgroup, rule)
		}
	}
	if rule.HasCgroupKey() {
		cgroup, err := pc.Cgroup(rule.CgroupKey)
		if err != nil {
			return err
		}
		UpdateRule(cgroup.BaseCgroup, rule)
	}
	return nil
}

func (pc *PresetCache) getCgroupCandidate(base BaseCgroup) (string, bool) {
	counter := make(map[string]int)
	// count matching entries for each cgroup
	// pc.Cgroups is map[string][]Cgroup
	ch := make(chan Cgroup)
	go func() {
		// do not iterate over []Cgroup and all duplicates,
		// but over first Cgroup
		for _, cgroups := range pc.Cgroups {
			ch <- ActivePreset(cgroups)
		}
		close(ch)
	}()
	preset := ToInterface(base)
	for cgroup := range ch { // iter through cgroups
		candidate := cgroup.CgroupKey
		counter[candidate] = 0 // count matching pairs
		p := ToInterface(cgroup.BaseCgroup)
		if len(p) > len(preset) { // obvious mismatch
			continue
		}
		for pk, pv := range p {
			for k, v := range preset { // compare key and value
				if pk == k && pv == v {
					counter[candidate] = counter[candidate] + 1
					break
				}
			}
		}
	}
	trace("getCgroupCandidate", "counter", counter)
	// pick up the best candidate, if any
	var best int
	for _, count := range counter {
		if count > best {
			best = count
		}
	}
	// found if preset is a superset of candidate properties
	var subkey string
	var found bool
	if best > 0 {
	Loop:
		for count := best; count > 0; count-- {
			for candidate, value := range counter {
				if value != count {
					continue
				}
				entries := ToInterface(pc.Cgroups[candidate][0].BaseCgroup)
				if len(entries) == count {
					found = true
					subkey = candidate
					trace("getCgroupCandidate", "entries", entries)
					break Loop
				}
			}
		}
		trace("getCgroupCandidate", "cgroup", subkey)
	}
	return subkey, found
}

func (pc *PresetCache) GetSliceProperties(rule Rule) (result []string) {
	if rule.CgroupKey != "" {
		if cgroup, err := pc.Cgroup(rule.CgroupKey); err == nil {
			result = Properties(cgroup.BaseCgroup)
		}
	}
	return
}

func (pc *PresetCache) GetRunInputRule(input *RunInput) Rule {
	rule, err := pc.GetRawRule(input)
	if err != nil {
		nonfatal(err)
		trace("GetRawRule", input.Name, "empty rule")
		return Rule{}
	}
	trace("GetRawRule", input.Name, rule)
	fatal(pc.ExpandRule(&rule))
	trace("ExpandRule", input.Name, rule)
	if input.Preset == "cgroup-only" {
		rule.CgroupOnly()
	}
	if len(input.Cgroup) > 0 {
		rule.SetCgroup(input.Cgroup)
	}
	if input.ForceCgroup && rule.CgroupKey == "" {
		if base, ok := rule.CgroupEntries(); ok {
			if cgroup, found := pc.getCgroupCandidate(base); found {
				rule.SetCgroup(cgroup) // set cgroup
			}
		}
	}
	if rule.CgroupKey != "" { // reset entries belonging to cgroup
		if cgroup, err := pc.Cgroup(rule.CgroupKey); err == nil {
			ResetMatching(&cgroup.BaseCgroup, &rule)
		}
	}
	rule.SetSliceProperties(pc.GetSliceProperties(rule))
	rule.SetCredentials()
	rule.RuleKey = ""
	rule.ProfileKey = ""
	rule.Origin = ""
	trace("GetRunInputRule", input.Name, rule)
	return rule
}

// vim: set ft=go fdm=indent ts=2 sw=2 tw=79 noet:
