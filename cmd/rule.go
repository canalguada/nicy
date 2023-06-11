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
	"strconv"
	"strings"

	"gopkg.in/yaml.v2"
)

type BaseCgroup struct {
	CPUQuota   string `yaml:"CPUQuota,omitempty" json:"CPUQuota,omitempty"`
	IOWeight   string `yaml:"IOWeight,omitempty" json:"IOWeight,omitempty"`
	MemoryHigh string `yaml:"MemoryHigh,omitempty" json:"MemoryHigh,omitempty"`
	MemoryMax  string `yaml:"MemoryMax,omitempty" json:"MemoryMax,omitempty"`
}

func (c *BaseCgroup) HasScopeProperties() bool {
	return c.CPUQuota != "" || c.IOWeight != "" ||
		c.MemoryHigh != "" || c.MemoryMax != ""
}

func (c *BaseCgroup) ScopeProperties() (result []string) {
	if c.CPUQuota != "" {
		if v, err := strconv.Atoi(strings.TrimSuffix(c.CPUQuota, `%`)); err == nil {
			value := fmt.Sprintf("%d%%", numCPU*v)
			result = append(result, fmt.Sprintf("CPUQuota=%s", value))
		}
	}
	if c.IOWeight != "" {
		result = append(result, fmt.Sprintf("IOWeight=%s", c.IOWeight))
	}
	if c.MemoryHigh != "" {
		result = append(result, fmt.Sprintf("MemoryHigh=%s", c.MemoryHigh))
	}
	if c.MemoryMax != "" {
		result = append(result, fmt.Sprintf("MemoryMax=%s", c.MemoryMax))
	}
	return
}

type Cgroup struct {
	BaseCgroup `yaml:"basecgroup,omitempty,flow"`
	CgroupKey  string `yaml:"cgroup,omitempty" json:"cgroup,omitempty"`
	Origin     string `yaml:"origin,omitempty" json:"origin,omitempty"`
}

func (c Cgroup) Keys() (string, string) {
	return c.CgroupKey, c.Origin
}

func (c Cgroup) String() string {
	c.CgroupKey, c.Origin = "", ""
	return ToJson(c)
}

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

type Struct interface {
	BaseCgroup | Cgroup | BaseProfile | Profile | BaseRule | Rule
}

func ToInterface[T Struct](st T) map[string]any {
	m := make(map[string]any)
	data, err := yaml.Marshal(st)
	nonfatal(wrap(err))
	if err := yaml.Unmarshal(data, &m); err != nil {
		nonfatal(wrap(err))
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
		f1 := s1.Field(i)
		if f1.IsValid() && !f1.IsZero() {
			if f2 := s2.FieldByName(typeOfT1.Field(i).Name); f2.IsValid() {
				f2.Set(reflect.Zero(f2.Type()))
			}
		}
	}
}

// UpdateRule iterates through st1 fields and, if set, updates
// matching fields in st2, if not set yet.
func UpdateRule[T1 BaseCgroup | BaseProfile](st T1, rule *Rule) {
	s := reflect.ValueOf(&st).Elem()
	r := reflect.ValueOf(rule).Elem()
	typeOfT1 := s.Type()
	for i := 0; i < s.NumField(); i++ {
		f1 := s.Field(i)
		if f1.IsValid() && !f1.IsZero() { // f1 is set
			// Find matching f2 field. Is it set ?
			if f2 := r.FieldByName(typeOfT1.Field(i).Name); f2.IsValid() && f2.IsZero() {
				f2.Set(f1)
			}
		}
	}
}

func Properties[T BaseCgroup | BaseProfile](st T) (result []string) {
	base := reflect.ValueOf(&st).Elem()
	typeOfBase := base.Type()
	for i := 0; i < base.NumField(); i++ {
		f := base.Field(i)
		if f.IsValid() && !f.IsZero() {
			name := typeOfBase.Field(i).Name
			result = append(result, fmt.Sprintf("%s=%s", name, f.Interface()))
		}
	}
	return
}

func Diff[T BaseCgroup | BaseProfile](preset T, runtime T) (T, int) {
	result := new(T)
	diff := reflect.ValueOf(result).Elem()
	count := 0
	rt := reflect.ValueOf(&runtime).Elem()
	base := reflect.ValueOf(&preset).Elem()
	for i := 0; i < base.NumField(); i++ {
		f := base.Field(i)
		if f.IsValid() && !f.IsZero() {
			if rt.Field(i).Interface() != f.Interface() {
				diff.Field(i).Set(f)
				count++
			}
		}
	}
	return *result, count
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
	if r.Nice < 0 {
		result = append(result, "nice")
	}
	if r.Sched == "fifo" || r.Sched == "rr" {
		result = append(result, "sched")
	}
	if r.IOClass == "realtime" {
		result = append(result, "ioclass")
	}
	return
}

func (r *Rule) SetCredentials() {
	if r.Nice < 0 {
		r.Credentials = append(r.Credentials, "nice")
	}
	if r.Sched == "fifo" || r.Sched == "rr" {
		r.Credentials = append(r.Credentials, "sched")
	}
	if r.IOClass == "realtime" {
		r.Credentials = append(r.Credentials, "ioclass")
	}
	return
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
