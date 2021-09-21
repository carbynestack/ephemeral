//
// Copyright (c) 2021 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
//
// SPDX-License-Identifier: Apache-2.0
//
package discovery

import (
	"errors"
	"sort"
	"strconv"
	"strings"
)

func NewPortsState(rng string, used []int32) (*PortsState, error) {

	ports := strings.Split(rng, ":")

	if len(ports) != 2 {
		return &PortsState{}, errors.New("the range must contain a port range in the form start_port:end_port, e.g. 1:65535")
	}
	start, err := strconv.Atoi(ports[0])
	if err != nil {
		return &PortsState{}, err
	}
	end, err := strconv.Atoi(ports[1])
	if err != nil {
		return &PortsState{}, err
	}

	state := &PortsState{start: int32(start), end: int32(end), lastUsed: -1}

	if len(used) > 0 {
		state.Sync(used)
	}
	return state, nil

}

type PortsState struct {
	start, end, lastUsed int32
	released             []int32
}

// GetFreePort returns a port that can be assigned or an error if there are no free ports.
func (m *PortsState) GetFreePort() (int32, error) {
	var port int32
	// Look up the released ports first.
	if len(m.released) > 0 {
		// Take the last port in the slice and reslice.
		port = m.released[len(m.released)-1]
		m.released = m.released[:len(m.released)-1]
		return port, nil
	}
	if m.lastUsed < m.start {
		// Initial port assignment, lastUsed is outside of the range.
		m.lastUsed = m.start
		return m.start, nil
	} else if m.lastUsed+1 >= m.start && m.lastUsed+1 <= m.end {
		// Ports were assigned previously, lastUsed is within the range.
		m.lastUsed++
		port = m.lastUsed
		return port, nil
	} else {
		return 0, errors.New("no free ports")
	}
}

// Sync updates the state based on the currently used ports.
// It populates the list of released ports and updates the lastUsed pointer.
// Not thead safe, an external lock must be hold to execute this method.
func (m *PortsState) Sync(used []int32) error {
	// Filter out the elements which are out of range.
	var usedInRange []int32
	for _, i := range used {
		if i >= m.start && i <= m.end {
			usedInRange = append(usedInRange, i)
		}
	}
	// Get the largest element and set it as a new lastUsed.
	// Reset the lastUsed if there are no used ports.
	sort.Slice(usedInRange, func(i, j int) bool {
		return usedInRange[i] < usedInRange[j]
	})
	if len(usedInRange) > 0 {
		m.lastUsed = usedInRange[len(usedInRange)-1]
	}
	// The ports were used in the past.
	if m.lastUsed > 0 {
		// We need to add the lower and upper bounds to make sure we include the released ports.
		usedInRange = append(usedInRange, m.start-1, m.lastUsed+1)
	}

	// Save released ports.
	m.released = m.getComplementarySet(usedInRange)
	return nil
}

// getComplementarySet returns a complement set to the universal set consisting of all natural numbers
// capped by the highest number in the given slice.
// For example, if a slice {1,3,5} is given, its universal set is {1,2,3,4,5},
// the complementary set would be {2,4}.
func (m *PortsState) getComplementarySet(set []int32) []int32 {
	var unusedPorts []int32
	sort.Slice(set, func(i, j int) bool {
		return set[i] < set[j]
	})
	for i, port := range set {
		//	Get the delta between two elements.
		if i != len(set)-1 {
			first := port
			second := set[i+1]
			delta := second - first
			if delta > 1 {
				//	Save the complementary element.
				for j := first + 1; j < second; j++ {
					unusedPorts = append(unusedPorts, j)
				}
			}
		}
	}
	if unusedPorts == nil {
		return make([]int32, 0)
	}
	return unusedPorts
}
