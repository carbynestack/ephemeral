// Copyright (c) 2021 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
//
// SPDX-License-Identifier: Apache-2.0
package fsm

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.uber.org/zap"
)

var _ = Describe("FSM", func() {

	It("generates a transition", func() {
		transition := WhenIn("Init").GotEvent("Register").GoTo("Registering")

		Expect(transition.Src).To(Equal("Init"))
		Expect(transition.Event).To(Equal("Register"))
		Expect(transition.Dst).To(Equal("Registering"))
	})
	var (
		respCh     chan string
		sideEffect func(e interface{}) error
		timeout    = 10 * time.Second
		errChan    = make(chan error)
		logger     = zap.NewNop().Sugar()
		ctx        = context.TODO()
	)

	BeforeEach(func() {
		respCh = make(chan string)
		sideEffect = func(e interface{}) error {
			ev := e.(*Event)
			respCh <- ev.Meta.FSM.current
			return nil
		}
	})
	Context("when running callbacks before and after state transition", func() {
		It("runs a callback after state transition", func() {
			cb := AfterEnter("Registering").Do(sideEffect)
			tr := WhenIn("Init").GotEvent("Register").GoTo("Registering")
			callbacks := map[string][]*Callback{}
			callbacks[cb.Src] = []*Callback{cb}
			transitions := map[TransitionID]*Transition{}
			transitions[tr.ID] = tr

			fsm, _ := NewFSM(ctx, "Init", transitions, callbacks, timeout, logger)
			go fsm.Run(errChan)
			event := Event{
				Name: "Register",
				Meta: &Metadata{FSM: fsm},
			}
			fsm.Write(&event)
			resp := <-respCh
			Expect(resp).To(Equal("Registering"))
			Expect(fsm.current).To(Equal("Registering"))
		})
		It("runs a callback before state transition", func() {
			cb := BeforeEnter("Registering").Do(sideEffect)
			tr := WhenIn("Init").GotEvent("Register").GoTo("Registering")
			callbacks := map[string][]*Callback{}
			callbacks[cb.Src] = []*Callback{cb}
			transitions := map[TransitionID]*Transition{}
			transitions[tr.ID] = tr

			fsm, _ := NewFSM(ctx, "Init", transitions, callbacks, timeout, logger)
			go fsm.Run(errChan)
			event := Event{
				Name: "Register",
				Meta: &Metadata{FSM: fsm},
			}
			fsm.Write(&event)
			res := <-respCh
			Expect(res).To(Equal("Init"))
			Expect(fsm.current).To(Equal("Registering"))
		})
	})

	Context("when state timeout is set", func() {
		It("transition to another state when the state timeout is reached", func() {
			respCh := make(chan string)
			timeoutCounter := int32(0)
			processTimeout := func(e interface{}) error {
				ev := e.(*Event)
				timeout := &Event{
					Name: "StateTimeout",
					Meta: &Metadata{FSM: ev.Meta.FSM},
				}
				ev.Meta.FSM.Write(timeout)
				atomic.AddInt32(&timeoutCounter, int32(1))
				return nil
			}
			respond := func(interface{}) error {
				respCh <- "timeout"
				return nil
			}
			trs := []*Transition{
				WhenIn("Init").GotEvent("StateTimeout").GoTo("Deadend"),
			}
			cbs := []*Callback{
				WhenStateTimeout().Do(processTimeout),
				AfterEnter("Deadend").Do(respond),
			}
			callbacks := map[string][]*Callback{}
			transitions := map[TransitionID]*Transition{}
			for _, c := range cbs {
				callbacks[c.Src] = []*Callback{c}
			}
			for _, t := range trs {
				transitions[t.ID] = t
			}
			timeout := 50 * time.Millisecond
			fsm, _ := NewFSM(ctx, "Init", transitions, callbacks, timeout, logger)
			go fsm.Run(errChan)
			resp := <-respCh
			Expect(resp).To(Equal("timeout"))
			time.Sleep(2 * timeout)
			// The second timeout hit makes sure the timer was reset.
			Expect(timeoutCounter).To(Equal(int32(2)))
			Expect(len(fsm.History().GetEvents())).To(Equal(2))
		})
	})

	Context("when staying the same state", func() {
		It("executes registered callbacks for the state", func() {
			respCh := make(chan string)
			sideEffect := func(e interface{}) error {
				ev := e.(*Event)
				respCh <- ev.Meta.FSM.current
				return nil
			}
			cb := AfterEnter("Init").Do(sideEffect)
			tr := WhenIn("Init").GotEvent("Register").Stay()
			callbacks := map[string][]*Callback{}
			callbacks[cb.Src] = []*Callback{cb}
			transitions := map[TransitionID]*Transition{}
			transitions[tr.ID] = tr

			fsm, _ := NewFSM(ctx, "Init", transitions, callbacks, timeout, logger)
			go fsm.Run(errChan)
			event := Event{
				Name: "Register",
				Meta: &Metadata{FSM: fsm},
			}
			fsm.Write(&event)
			res := <-respCh
			Expect(res).To(Equal("Init"))
			states := fsm.History().GetStates()
			Expect(len(states)).To(Equal(2))
			Expect(states[0]).To(Equal("Init"))
		})
	})
	Context("when several callbacks for a state are provided", func() {
		It("executes all of them", func() {
			afterInit := "AfterInit"
			respCh := make(chan string)
			callbacks := map[string][]*Callback{}
			transitions := map[TransitionID]*Transition{}
			sideEffect := func(e interface{}) error {
				ev := e.(*Event)
				respCh <- ev.Meta.FSM.current
				return nil
			}
			cb := []*Callback{
				AfterEnter(afterInit).Do(sideEffect),
				AfterEnter(afterInit).Do(sideEffect),
			}
			tr := WhenIn("Init").GotEvent("Next").GoTo(afterInit)
			callbacks[afterInit] = cb
			transitions[tr.ID] = tr
			fsm, _ := NewFSM(ctx, "Init", transitions, callbacks, timeout, logger)
			go fsm.Run(errChan)
			event := Event{
				Name: "Next",
				Meta: &Metadata{FSM: fsm},
			}
			fsm.Write(&event)
			fsm.Write(&event)
			res := <-respCh
			Expect(res).To(Equal(afterInit))
			res = <-respCh
			Expect(res).To(Equal(afterInit))
		})
	})
	Context("when an error in a callback happens", func() {
		It("propagates the error to the err channel", func() {
			afterInit := "AfterInit"
			callbacks := map[string][]*Callback{}
			transitions := map[TransitionID]*Transition{}
			faultyCallback := func(e interface{}) error {
				return errors.New("some error")
			}
			cb := []*Callback{
				AfterEnter(afterInit).Do(faultyCallback),
			}
			tr := WhenIn("Init").GotEvent("Next").GoTo(afterInit)
			callbacks[afterInit] = cb
			transitions[tr.ID] = tr

			errChan := make(chan error)
			fsm, _ := NewFSM(ctx, "Init", transitions, callbacks, timeout, logger)
			go fsm.Run(errChan)
			event := &Event{
				Name: "Next",
				Meta: &Metadata{FSM: fsm},
			}
			fsm.Write(event)
			err := <-errChan
			Expect(err.Error()).To(Equal("some error"))
			Expect(fsm.Current()).To(Equal(Stopped))
		})
	})
	Context("when context is cancelled", func() {
		It("stops the FSM", func() {
			pingCh := make(chan struct{})
			doneCh := make(chan struct{})
			errCh := make(chan error)
			timer := time.NewTimer(5 * time.Second)
			ctx := context.Background()
			ctx, cancel := context.WithTimeout(ctx, 5*time.Millisecond)
			fsm := &FSM{
				pingCh: pingCh,
				doneCh: doneCh,
				timer:  timer,
				ctx:    ctx,
			}
			cancel()
			fsm.Run(errCh)
			Expect(fsm.Current()).To(Equal(Stopped))
		})
	})
	Context("when stopping a FSM", func() {
		It("changes its state to Stopped", func() {
			pingCh := make(chan struct{})
			doneCh := make(chan struct{})
			errCh := make(chan error)
			timer := time.NewTimer(5 * time.Second)
			ctx := context.Background()
			fsm := &FSM{
				pingCh: pingCh,
				doneCh: doneCh,
				timer:  timer,
				ctx:    ctx,
			}
			go fsm.Run(errCh)
			fsm.Stop()
			Expect(fsm.Current()).To(Equal(Stopped))
		})
	})
	Context("when initializing callbacks and transitions", func() {
		It("converts slices to maps", func() {
			tState := "testState"
			tEvent := "testEvent"
			cbs := []*Callback{
				AfterEnter(tState),
			}
			trans := []*Transition{
				WhenInAnyState().GotEvent(tEvent),
			}
			callbacks, transitions := InitCallbacksAndTransitions(cbs, trans)
			Expect(len(callbacks)).To(Equal(1))
			Expect(len(transitions)).To(Equal(1))
			cb, ok := callbacks[tState]
			Expect(ok).To(BeTrue())
			Expect(len(cb)).To(Equal(1))
			Expect(cb[0].Src).To(Equal(tState))
			transitionID := TransitionID{
				Event:  tEvent,
				Source: "*",
			}
			tr, ok := transitions[transitionID]
			Expect(ok).To(BeTrue())
			Expect(tr).NotTo(BeNil())
			Expect(tr.Src).To(Equal("*"))
		})
	})
})
