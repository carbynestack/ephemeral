// Copyright (c) 2021 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
//
// SPDX-License-Identifier: Apache-2.0
package discovery

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	//"network-controller/pkg/controller"
)

var _ = Describe("PortsState", func() {

	Context("wrong port range is provided", func() {
		It("returns an error", func() {
			_, err := NewPortsState("1000:", []int32{})
			Expect(err).To(HaveOccurred())
		})
	})

	Context("there are no released ports", func() {
		Context("within the range", func() {
			It("gets a port from the free pool", func() {
				mng, _ := NewPortsState("1000:1001", []int32{})
				port, _ := mng.GetFreePort()
				Expect(port).To(Equal(int32(1000)))
				Expect(mng.lastUsed).To(Equal(int32(1000)))
				port, _ = mng.GetFreePort()
				Expect(port).To(Equal(int32(1001)))
				Expect(mng.lastUsed).To(Equal(int32(1001)))
			})
		})
		Context("out of range", func() {
			It("returns an error", func() {
				mng, _ := NewPortsState("1000:1001", []int32{})
				port, _ := mng.GetFreePort()
				Expect(port).To(Equal(int32(1000)))
				port, _ = mng.GetFreePort()
				Expect(port).To(Equal(int32(1001)))
				port, err := mng.GetFreePort()
				Expect(err).To(HaveOccurred())
			})
		})
		Context("there are used ports", func() {
			It("assigns the next port after the highest port value", func() {
				ports, _ := NewPortsState("1000:1001", []int32{1000})
				port, _ := ports.GetFreePort()
				Expect(port).To(Equal(int32(1001)))
				Expect(ports.lastUsed).To(Equal(int32(1001)))
			})
		})
	})

	Context("there are released ports", func() {
		It("takes the released port", func() {
			ports, _ := NewPortsState("1000:1002", []int32{1000, 1002})
			port, _ := ports.GetFreePort()
			Expect(port).To(Equal(int32(1001)))
		})
	})

	It("gets a complementary set", func() {
		type input struct {
			params   []int32
			expected []int32
			text     string
		}
		inputs := []input{
			{[]int32{1, 3, 5}, []int32{2, 4}, "sorted with unused"},
			{[]int32{5, 3, 1}, []int32{2, 4}, "unsorted with unused"},
			{[]int32{1, 2, 3}, []int32{}, "without unused"},
			{[]int32{1}, []int32{}, "a single element"},
			{[]int32{2}, []int32{}, "a single element bigger then one"},
			{[]int32{}, []int32{}, "an empty set"},
			{[]int32{-1, -2}, []int32{}, "negative elements"},
			{[]int32{-2, -1}, []int32{}, "unsorted negative elements"},
		}
		for _, input := range inputs {
			mng := PortsState{}
			unused := mng.getComplementarySet(input.params)
			Expect(unused).To(Equal(input.expected))
		}
	})

	Context("syncing the ports", func() {

		Context("ports are in range", func() {
			It("populates the map based on the input ports", func() {
				usedPorts := []int32{int32(1000), int32(1002)}
				mng, _ := NewPortsState("1000:1002", []int32{})
				_ = mng.Sync(usedPorts)
				Expect(mng.lastUsed).To(Equal(int32(1002)))
				Expect(mng.released).To(Equal([]int32{1001}))
			})
			It("takes the released ports", func() {
				usedPorts := []int32{int32(1002), int32(1003)}
				mng, _ := NewPortsState("1000:1003", []int32{})
				_ = mng.Sync(usedPorts)
				port, err := mng.GetFreePort()
				Expect(err).NotTo(HaveOccurred())
				Expect(port).To(Equal(int32(1001)))
				port, err = mng.GetFreePort()
				Expect(err).NotTo(HaveOccurred())
				Expect(port).To(Equal(int32(1000)))
			})
		})

		Context("there are ports out of range", func() {
			It("ignores those ports", func() {
				state, _ := NewPortsState("1000:1001", []int32{})
				state.Sync([]int32{1001, 1005, 1007})
				Expect(len(state.released)).To(Equal(1))
			})
		})
		Context("all ports are released", func() {
			It("allows to reuse the released ports", func() {
				state, _ := NewPortsState("1000:1001", []int32{})
				state.Sync([]int32{1000})
				port, _ := state.GetFreePort()
				Expect(port).To(Equal(int32(1001)))
				state.Sync([]int32{})
				port, err := state.GetFreePort()
				Expect(err).NotTo(HaveOccurred())
				Expect(port).To(Equal(int32(1001)))
				port, err = state.GetFreePort()
				Expect(err).NotTo(HaveOccurred())
				Expect(port).To(Equal(int32(1000)))
			})
		})
	})
})
