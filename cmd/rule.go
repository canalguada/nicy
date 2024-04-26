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

type BaseRule struct {
	// Profile: assign to profile with `ProfileKey`
	// and eventually adjust process properties in Rule
	// Cgroup: assign to cgroup slice with `CgroupKey`
	// and eventually adjust scope properties in Rule
	ProfileKey      string            `yaml:"profile,omitempty" json:"profile,omitempty"`
	CgroupKey       string            `yaml:"cgroup,omitempty" json:"cgroup,omitempty"`
	CmdArgs         []string          `yaml:"cmdargs,omitempty,flow" json:"cmdargs,omitempty"`
	Env             map[string]string `yaml:"env,omitempty,flow" json:"env,omitempty"`
	SliceProperties []string          `yaml:"slice_properties,omitempty,flow" json:"slice_properties,omitempty"`
	Credentials     []string          `yaml:"cred,omitempty,flow" json:"cred,omitempty"`
}

type Rule struct {
	BaseProfile `yaml:"baseprofile,omitempty,flow"`
	BaseCgroup  `yaml:"basecgroup,omitempty,flow"`
	BaseRule    `yaml:"baserule,omitempty,flow"`
	RuleKey     string `yaml:"name,omitempty" json:"name,omitempty"`
	Origin      string `yaml:"origin,omitempty" json:"origin,omitempty"`
}

func (r Rule) Keys() (string, string) {
	return r.RuleKey, r.Origin
}

func (r Rule) Path() string {
	return LookPath(r.RuleKey)
}

func (r Rule) String() string {
	r.RuleKey, r.Origin = "", ""
	return ToJson(r)
}

func (r *Rule) HasProfileKey() bool {
	return r.ProfileKey != ""
}

func (r *Rule) HasNice() bool {
	return r.Nice != 0
}

func (r *Rule) HasSched() bool {
	return r.Sched != ""
}

func (r *Rule) HasRtprio() bool {
	return r.RTPrio != 0
}

func (r *Rule) HasIoclass() bool {
	return r.IOClass != ""
}

func (r *Rule) HasIonice() bool {
	return r.IONice != 0
}

func (r *Rule) HasOomScoreAdj() bool {
	return r.OomScoreAdj != 0
}

func (r *Rule) HasCgroupKey() bool {
	return r.CgroupKey != ""
}

func (r *Rule) HasCPUQuota() bool {
	return r.CPUQuota != ""
}

func (r *Rule) HasIOWeight() bool {
	return r.IOWeight != ""
}

func (r *Rule) HasMemoryHigh() bool {
	return r.MemoryHigh != ""
}

func (r *Rule) HasMemoryMax() bool {
	return r.MemoryMax != ""
}

func (r *Rule) CgroupOnly() {
	r.ProfileKey = ""
	r.BaseProfile = BaseProfile{}
}

func (r *Rule) SetCgroup(cgroup string) {
	r.CgroupKey = cgroup
}

func (r *Rule) CgroupEntries() (BaseCgroup, bool) {
	trace("CgroupEntries", r.RuleKey, r.BaseCgroup)
	return r.BaseCgroup, r.HasScopeProperties()
}

func (r *Rule) NeedScope() bool {
	return r.HasCgroupKey() || r.HasScopeProperties()
}

func (r *Rule) GetCredentials() (result []string) {
	result = append(result, "nice")
	if r.Sched == "fifo" || r.Sched == "rr" {
		result = append(result, "sched")
	}
	if r.IOClass == "realtime" {
		result = append(result, "ioclass")
	}
	return
}

func (r *Rule) SetCredentials() {
	r.Credentials = nil
	r.Credentials = append(r.Credentials, r.GetCredentials()...)
}

func (r *Rule) SetSliceProperties(properties []string) {
	r.SliceProperties = properties
}

func (r *Rule) Copy() Rule {
	result := Rule{
		BaseProfile: r.BaseProfile,
		BaseCgroup:  r.BaseCgroup,
		RuleKey:     r.RuleKey,
		BaseRule:    r.BaseRule,
		Origin:      r.Origin,
	}
	if len(r.CmdArgs) > 0 {
		_ = copy(result.CmdArgs, r.CmdArgs)
	}
	if len(r.Env) > 0 {
		for k, v := range r.Env {
			result.Env[k] = v
		}
	}
	if len(r.SliceProperties) > 0 {
		_ = copy(result.SliceProperties, r.SliceProperties)
	}
	if len(r.Credentials) > 0 {
		_ = copy(result.Credentials, r.Credentials)
	}
	return result
}

func (r *Rule) GetDiff(runtime Rule) (Rule, int) {
	var count, n int
	diffCgroup, n := Diff(r.BaseCgroup, runtime.BaseCgroup)
	count += n
	diffProfile, n := Diff(r.BaseProfile, runtime.BaseProfile)
	count += n
	diff := Rule{
		BaseProfile: diffProfile,
		BaseCgroup:  diffCgroup,
	}
	if r.HasCgroupKey() {
		diff.CgroupKey = r.CgroupKey
		count++
	}
	return diff, count
}

// vim: set ft=go fdm=indent ts=2 sw=2 tw=79 noet:
