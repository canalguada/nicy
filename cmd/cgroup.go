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
	"strconv"
	"strings"
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
