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
	"fmt"
	"strings"
	"io/ioutil"
	"encoding/json"
	"github.com/spf13/viper"
)

// Cache
//     rule       nvim     []Entry
// map[string]map[string][]Preset{
//   "rules": {
//     "nvim": []Preset{
//       Preset{},
//       ...
//     },
//   },
//   ...
// }

type Preset CStringMap

func (p Preset) StringMap() CStringMap {
	return CStringMap(p)
}

func NewPreset(obj CStringMap) Preset {
	p := make(Preset)
	p.Update(obj)
	return p
}

func (p Preset) Keep(keys ...string) {
	Loop:
		for _, key := range p.Keys() {
			for _, k := range keys {
				if key == k {
					continue Loop
				}
			}
			delete(p, key)
		}
}

func (p Preset) Remove(keys ...string) {
	for _, k := range keys {
		if _, ok := p[k]; ok {
			delete(p, k)
		}
	}
}

func (p Preset) CgroupOnly() {
	p.Keep(cfgMap["cgroup-only"]...)
}

func (p Preset) SetCgroup(cgroup string) {
	p["cgroup"] = interface{}(cgroup)
}

func (p Preset) CgroupEntries() Preset {
	result := make(CStringMap)
	for key, value := range p {
		for _, k := range cfgMap["systemd"] {
			if key == k {
				result[key] = value
				break
			}
		}
	}
	return Preset(result)
}

func (p Preset) Credentials()  (result []string) {
	for key, value := range p {
		switch key {
		case "nice":
			var v int
			switch value.(type) {
			case int:
				v, _ = value.(int)
			case float64:
				r, _ := value.(float64)
				v = int(r)
			}
			if v < 0 {
				result = append(result, "nice")
			}
		case "sched":
			if v, ok := value.(string); ok {
				if v == "fifo" || v == "rr" {
					result = append(result, "sched")
				}
			}
		case "ioclass":
			if v, ok := value.(string); ok {
				if v == "realtime" {
					result = append(result, "ioclass")
				}
			}
		}
	}
	return
}

func (p Preset) Keys() (keys []string) {
	for key, _ := range p {
		keys = append(keys, key)
	}
	return
}

func (p Preset) Append(key string, value interface{}) {
	p[key] = value
}

func (p Preset) Update(obj CStringMap) {
	for key, value := range obj {
		p[key] = value
	}
}

func (p Preset) Copy() Preset {
	return NewPreset(CStringMap(p))
}

func (p Preset) String() string {
	result := []string{`Preset[`}
	for key, value := range p {
		result = append(result, fmt.Sprintf("[%s:%v]", key, value))
	}
	result = append(result, `]`)
	return strings.Join(result, ``)
}

func (p Preset) Trace(tag string, subkey string) {
	if viper.GetBool("verbose") && viper.GetBool("debug") {
		s := []interface{}{"debug:"}
		if len(subkey) > 0 {
			s = append(s, subkey + `:`)
		}
		s = append(s, fmt.Sprintf("%v", p))
		if len(tag) > 0 {
			s = append(s, fmt.Sprintf("(%s)", tag))
		}
		notify(s...)
	}
}

func (p Preset) Diff(runtime CStringMap) Preset {
	diff := make(Preset)
	resources := append(cfgMap["resource"], "cgroup")
	for _, resource := range resources {
		if value, found := p[resource]; found {
			if resource == "cgroup" {
				diff["cgroup"] = value
			} else if value != runtime[resource] {
				diff[resource] = value
			}
		}
	}
	return diff
}

type Cache struct {
	Date string															`json:"date"`
	Content map[string]map[string][]Preset	`json:"content"`
}

func NewCache() Cache {
	c := Cache{}
	c.Date = timestamp()
	c.Content = make(map[string]map[string][]Preset)
	return c
}

func contentToPreset(key string, obj CStringMap) (string, Preset) {
	var (
		flag bool
		subkey string
	)
	p := make(Preset)
	if len(key) > 0 {
		if _, found := cfgMap[key]; found {
			flag = true
		}
	}
	for k, v := range obj {
		if flag && k == cfgMap[key][0] { // skip and extract subkey
			if s, ok := v.(string); ok {
				subkey = s
			}
			continue
		} else {
			p[k] = v
		}
	}
	return subkey, p
}

func (c *Cache) AppendContent(key string, maps CSlice) (err error) {
	switch key {
	case "cgroup", "type", "rule":
		if _, found := c.Content[key]; !(found) {
			c.Content[key] = make(map[string][]Preset)
		}
		for _, obj := range maps {
			if obj, ok := obj.(CStringMap); ok {
				subkey, p := contentToPreset(key, obj)
				if len(subkey) > 0 {
					if _, found := c.Content[key][subkey]; found { // append
						c.Content[key][subkey] = append(c.Content[key][subkey], p)
					} else { // create
						c.Content[key][subkey] = []Preset{p}
					}
				}
			}
		}
	default:
		err = fmt.Errorf("%w: bad category: %s", ErrInvalid, key)
	}
	return
}

func ReadFile() (c Cache, err error) {
	if data, e := ioutil.ReadFile(viper.GetString("cache")); e != nil {
		err = e
	} else {
		err = json.Unmarshal(data, &c)
	}
	return
}

func (c *Cache) HasPreset(key string, subkey string) (found bool) {
	if _, ok := c.Content[key]; ok {
		if slice, ok := c.Content[key][subkey]; ok && len(slice) > 0 {
			found = true
		}
	}
	return
}

func (c *Cache) getPreset(key string, subkey string) (p Preset, err error) {
	if c.HasPreset(key, subkey) {
		p = c.Content[key][subkey][0]
	} else {
		err = fmt.Errorf("%s not found: %s", key, subkey)
		if viper.GetBool("verbose") {
			notify(err)
		}
	}
	return
}

func (c *Cache) Rule(subkey string) (p Preset, err error) {
	if preset, e := c.getPreset("rule", subkey); e == nil {
		p = preset.Copy()
	} else {
		err = e
	}
	return
}

func (c *Cache) Type(subkey string) (p Preset, err error) {
	if preset, e := c.getPreset("type", subkey); e == nil {
		p = preset.Copy()
	} else {
		err = e
	}
	return
}

func (c *Cache) Cgroup(subkey string) (p Preset, err error) {
	if preset, e := c.getPreset("cgroup", subkey); e == nil {
		p = preset.Copy()
	} else {
		err = e
	}
	return
}

func (c *Cache) Keys(key string, subkey string) (keys []string) {
	if p, err := c.getPreset(key, subkey); err == nil {
		keys = append(keys, p.Keys()...)
	}
	return
}

func (c *Cache) ruleOrType(input *RunInput) (p Preset, err error) {
	switch input.Preset {
	case "auto", "cgroup-only":
		p, err = c.Rule(input.Name)
		if err != nil {
			p, err = c.Type("default")
		}
	default:
		p, err = c.Type(input.Preset)
	}
	return
}

func (c *Cache) expandPreset(p Preset) (err error) {
	buffer := make(CStringMap)
	// expand first type, then cgroup
	for _, key := range []string{"type", "cgroup"} {
		if value, ok := p[key]; ok {
			subkey, _ := value.(string)
			buffer[key] = subkey
			if preset, e := c.getPreset(key, subkey); e == nil {
				for k, v := range preset {
					buffer[k] = v
				}
			} else {
				err = e
				return
			}
		}
	}
	// add other pairs
	for key, value := range p {
		if key != "type" && key != "cgroup" {
			buffer[key] = value
		}
	}
	// finally
	p.Update(buffer)
	return
}

func (c *Cache) getCandidate(key string, preset Preset) (subkey string, found bool) {
	counter := make(map[string]int)
	// count matching entries for each category key
	// c.Content[key] is map[string][]Preset
	for candidate, values := range c.Content[key] {
		counter[candidate] = 0
		// do not iterate over []Preset and all duplicates, but over first Preset
		for key, value := range values[0] {
			for k, v := range preset { // compare key and value
				if key == k && value == v {
					counter[candidate] = counter[candidate] + 1
					break
				}
			}
		}
	}
	// pick up the best candidate, if any
	var best int
	for candidate, count := range counter {
		if count > best {
			subkey = candidate
			best = count
		}
	}
	found = best > 0
	return
}

func (c *Cache) GetRunPreset(input *RunInput) (p Preset, err error) {
	p, err = c.ruleOrType(input)
	p.Trace("ruleOrType", input.Name)
	if err != nil {
		p = make(Preset) // empty preset
	}
	fatal(c.expandPreset(p))
	p.Trace("expandPreset", input.Name)
	p.Remove("origin")
	if input.Preset == "cgroup-only" {
		p.CgroupOnly()
	}
	if len(input.Cgroup) > 0 {
		p.SetCgroup(input.Cgroup)
	}
	if input.ForceCgroup {
		if _, ok := p["cgroup"]; !(ok) { // move to matching cgroup
			if entries := p.CgroupEntries(); len(entries) > 0 {
				if cgroup, found := c.getCandidate("cgroup", entries); found {
					keys := c.Keys("cgroup", cgroup)
					p.Remove(keys...) // delete entries belonging to cgroup preset
					p.SetCgroup(cgroup) // set cgroup
				}
			}
		}
	}
	p.Remove("name", "type")
	if value, found := p["cgroup"]; found {
		if cgroup, ok := value.(string); ok {
			preset, _ := c.Cgroup(cgroup)
			preset.Keep(cfgMap["systemd"]...) // prepare slice properties
			var properties []string
			for k, v := range preset { // add slice properties entry
				if s, ok := v.(string); ok {
					properties = append(properties, fmt.Sprintf("%s=%s", k, s))
				} else {
					warn(fmt.Errorf("%w: bad %s: %v", ErrInvalid, k, v))
				}
			}
			s := strings.Join(properties, ` `)
			p.Append("slice_properties", s)
			p.Remove(preset.Keys()...) // remove cgroup preset keys from current
		} else {
			warn(fmt.Errorf("%w: bad cgroup: %v", ErrInvalid, value))
		}
	}
	// finally check where credentials could be required
	p.Append("cred", p.Credentials())
	p.Trace("GetRunPreset", input.Name)
	return
}

func formatSubkeyPreset(key string, subkey string, preset Preset) string {
	preset.Keep(cfgMap[key]...)
	data, _ := json.Marshal(preset)
	return fmt.Sprintf("%s\t%s", subkey, string(data))
}

func (c *Cache) List(key string) (result []string, err error) {
	switch key {
	case "cgroup", "type", "rule":
		for subkey, presets := range c.Content[key] {
			if len(presets) > 0 {
				result = append(result, formatSubkeyPreset(key, subkey, presets[0]))
			}
		}
	default:
		err = fmt.Errorf("%w: bad category: %s", ErrInvalid, key)
	}
	return
}

func selectPreset(presets []Preset, filter func(p Preset) bool) (preset Preset){
	for _, p := range presets {
		if ok := filter(p); ok {
			preset = p
			break
		}
	}
	return
}

func (c *Cache) ListFrom(key string, path string) (result []string, err error) {
	switch key {
	case "cgroup", "type", "rule":
		if len(path) > 0 {
			for subkey, presets := range c.Content[key] {
				preset := selectPreset(presets, func(p Preset) bool {
					var flag bool
					if value, found := p["origin"]; found {
						if s, ok := value.(string); ok {
							if s == path {
								flag = true
							}
						}
					}
					return flag
				})
				if preset != nil {
					result = append(result, formatSubkeyPreset(key, subkey, preset))
				}
			}
		}
	default:
		err = fmt.Errorf("%w: bad category: %s", ErrInvalid, key)
	}
	return
}

// vim: set ft=go fdm=indent ts=2 sw=2 tw=79 noet:
