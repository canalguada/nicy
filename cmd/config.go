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
	"os"
	"strings"

	"gopkg.in/yaml.v2"
)

// Build command

type Group struct {
	CgroupKey   string `yaml:"cgroup,omitempty" json:"cgroup,omitempty"`
	CPUQuota    string `yaml:"CPUQuota,omitempty" json:"CPUQuota,omitempty"`
	IOWeight    string `yaml:"IOWeight,omitempty" json:"IOWeight,omitempty"`
	MemoryHigh  string `yaml:"MemoryHigh,omitempty" json:"MemoryHigh,omitempty"`
	MemoryMax   string `yaml:"MemoryMax,omitempty" json:"MemoryMax,omitempty"`
	Nice        int    `yaml:"nice,omitempty" json:"nice,omitempty"`
	Sched       string `yaml:"sched,omitempty" json:"sched,omitempty"`
	Rtprio      int    `yaml:"rtprio,omitempty" json:"rtprio,omitempty"`
	Ioclass     string `yaml:"ioclass,omitempty" json:"ioclass,omitempty"`
	Ionice      int    `yaml:"ionice,omitempty" json:"ionice,omitempty"`
	OomScoreAdj int    `yaml:"oom_score_adj,omitempty" json:"oom_score_adj,omitempty"`
}

func (g *Group) ToProfile(key, origin string) Profile {
	return Profile{
		CgroupKey: g.CgroupKey,
		BaseCgroup: BaseCgroup{
			CPUQuota:   g.CPUQuota,
			IOWeight:   g.IOWeight,
			MemoryHigh: g.MemoryHigh,
			MemoryMax:  g.MemoryMax,
		},
		ProfileKey: key,
		BaseProfile: BaseProfile{
			Nice:        g.Nice,
			Sched:       g.Sched,
			RTPrio:      g.Rtprio,
			IOClass:     g.Ioclass,
			IONice:      g.Ionice,
			OomScoreAdj: g.OomScoreAdj,
		},
		Origin: origin,
	}
}

type AppGroup struct {
	Profile     Group    `yaml:"profile,omitempty,flow" json:"profile,omitempty"`
	Assignments []string `yaml:"assignments,omitempty,flow" json:"assignments,omitempty"`
}

type AppRule struct {
	ProfileKey  string            `yaml:"profile,omitempty" json:"profile,omitempty"`
	Nice        int               `yaml:"nice,omitempty" json:"nice,omitempty"`
	Sched       string            `yaml:"sched,omitempty" json:"sched,omitempty"`
	RTPrio      int               `yaml:"rtprio,omitempty" json:"rtprio,omitempty"`
	IOClass     string            `yaml:"ioclass,omitempty" json:"ioclass,omitempty"`
	IONice      int               `yaml:"ionice,omitempty" json:"ionice,omitempty"`
	OomScoreAdj int               `yaml:"oom_score_adj,omitempty" json:"oom_score_adj,omitempty"`
	CgroupKey   string            `yaml:"cgroup,omitempty" json:"cgroup,omitempty"`
	CPUQuota    string            `yaml:"CPUQuota,omitempty" json:"CPUQuota,omitempty"`
	IOWeight    string            `yaml:"IOWeight,omitempty" json:"IOWeight,omitempty"`
	MemoryHigh  string            `yaml:"MemoryHigh,omitempty" json:"MemoryHigh,omitempty"`
	MemoryMax   string            `yaml:"MemoryMax,omitempty" json:"MemoryMax,omitempty"`
	CmdArgs     []string          `yaml:"cmdargs,omitempty,flow" json:"cmdargs,omitempty"`
	Env         map[string]string `yaml:"env,omitempty,flow" json:"env,omitempty"`
}

func (a AppRule) ToRule(key, origin string) Rule {
	return Rule{
		BaseProfile: BaseProfile{
			Nice:        a.Nice,
			Sched:       a.Sched,
			RTPrio:      a.RTPrio,
			IOClass:     a.IOClass,
			IONice:      a.IONice,
			OomScoreAdj: a.OomScoreAdj,
		},
		BaseCgroup: BaseCgroup{
			CPUQuota:   a.CPUQuota,
			IOWeight:   a.IOWeight,
			MemoryHigh: a.MemoryHigh,
			MemoryMax:  a.MemoryMax,
		},
		BaseRule: BaseRule{
			ProfileKey: a.ProfileKey,
			CgroupKey:  a.CgroupKey,
			CmdArgs:    a.CmdArgs,
			Env:        a.Env,
		},
		RuleKey: key,
		Origin:  origin,
	}
}

type Config struct {
	Path      string
	Origin    string
	Cgroups   map[string]BaseCgroup `yaml:"cgroups,flow" json:"cgroups"`
	AppGroups map[string]AppGroup   `yaml:"appgroups,flow" json:"appgroups"`
	Rules     map[string]AppRule    `yaml:"rules,flow" json:"rules"`
}

func NewConfig(path string) (Config, error) {
	var err error
	cfg := struct {
		Presets Config
	}{
		Presets: Config{
			Cgroups:   make(map[string]BaseCgroup),
			AppGroups: make(map[string]AppGroup),
			Rules:     make(map[string]AppRule),
		},
	}
	if content, err := os.ReadFile(path); err == nil {
		if err := yaml.Unmarshal([]byte(content), &cfg); err == nil {
			cfg.Presets.SetOrigin(path)
			return cfg.Presets, nil
		}
		return cfg.Presets, err
	}
	return cfg.Presets, err
}

func (c *Config) SetOrigin(path string) {
	c.Path = path
	if strings.HasPrefix(path, "/home") {
		c.Origin = "user"
	} else if strings.HasPrefix(path, "/usr/local/etc") {
		c.Origin = "site"
	} else if strings.HasPrefix(path, "/etc") {
		c.Origin = "vendor"
	} else {
		c.Origin = "other"
	}
}

func (c *Config) IterCgroups() chan Cgroup {
	ch := make(chan Cgroup)
	go func(ch chan Cgroup) {
		for tag, cgroup := range c.Cgroups {
			ch <- Cgroup{
				BaseCgroup: cgroup,
				CgroupKey:  tag,
				Origin:     c.Origin,
			}
		}
		close(ch)
	}(ch)
	return ch
}

func (c *Config) IterProfiles() chan Profile {
	ch := make(chan Profile)
	go func(ch chan Profile) {
		for tag, group := range c.AppGroups {
			ch <- group.Profile.ToProfile(tag, c.Origin)
		}
		close(ch)
	}(ch)
	return ch
}

func (c *Config) IterRules() chan Rule {
	ch := make(chan Rule)
	go func(ch chan Rule) {
		tags := make(map[string]string)
		for tag, group := range c.AppGroups {
			for _, app := range group.Assignments {
				var rule Rule
				_, found := c.Rules[app]
				if found {
					rule = c.Rules[app].ToRule(app, c.Origin)
					if !rule.HasProfileKey() {
						rule.ProfileKey = tag
					}
				} else {
					rule = Rule{
						BaseRule: BaseRule{
							ProfileKey: tag,
						},
						RuleKey: app,
						Origin:  c.Origin,
					}
				}
				tags[app] = tag
				ch <- rule
			}
		}
		for tag, item := range c.Rules {
			if _, found := tags[tag]; !found {
				rule := item.ToRule(tag, c.Origin)
				if !rule.HasProfileKey() {
					rule.ProfileKey = "none"
				}
				tags[tag] = "none"
				ch <- rule
			}
		}
		close(ch)
	}(ch)
	return ch
}

// vim: set ft=go fdm=indent ts=2 sw=2 tw=79 noet:
