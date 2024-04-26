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

type BaseProfile struct {
	Nice        int    `yaml:"nice,omitempty" json:"nice,omitempty"`
	Sched       string `yaml:"sched,omitempty" json:"sched,omitempty"`
	RTPrio      int    `yaml:"rtprio,omitempty" json:"rtprio,omitempty"`
	IOClass     string `yaml:"ioclass,omitempty" json:"ioclass,omitempty"`
	IONice      int    `yaml:"ionice,omitempty" json:"ionice,omitempty"`
	OomScoreAdj int    `yaml:"oom_score_adj,omitempty" json:"oom_score_adj,omitempty"`
}

type Profile struct {
	// Cgroup: assign to cgroup slice with `CgroupKey`
	// and eventually adjust scope properties in BaseCgroup
	BaseCgroup  `yaml:"basecgroup,omitempty,flow"`
	BaseProfile `yaml:"baseprofile,omitempty,flow"`
	CgroupKey   string `yaml:"cgroup,omitempty" json:"cgroup,omitempty"`
	ProfileKey  string `yaml:"profile,omitempty" json:"profile,omitempty"`
	Origin      string `yaml:"origin,omitempty" json:"origin,omitempty"`
}

func (p Profile) Keys() (string, string) {
	return p.ProfileKey, p.Origin
}

func (p Profile) String() string {
	p.ProfileKey, p.Origin = "", ""
	return ToJson(p)
}

func (p *Profile) ToRule() Rule {
	return Rule{
		BaseProfile: p.BaseProfile,
		BaseCgroup:  p.BaseCgroup,
		BaseRule: BaseRule{
			ProfileKey: p.ProfileKey,
			CgroupKey:  p.CgroupKey,
		},
	}
}
