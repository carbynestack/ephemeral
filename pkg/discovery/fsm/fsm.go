// Copyright (c) 2021 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
//
// SPDX-License-Identifier: Apache-2.0
package fsm

import (
	"context"
	"errors"
	"fmt"
	proto "github.com/carbynestack/ephemeral/pkg/discovery/transport/proto"
	"sync"
	"time"

	"go.uber.org/zap"
)

const (
	Stopped = "_Stopped"
)

// NewFSM returns a new finate state machine.
func NewFSM(ctx context.Context, initState string, trn map[TransitionID]*Transition, cb map[string][]*Callback, timeout time.Duration, logger *zap.SugaredLogger) (*FSM, error) {
	var stateTimeoutCb *Callback
	timer := time.NewTimer(timeout)
	beforeCallbacks := make(map[string][]*Callback)
	afterCallbacks := make(map[string][]*Callback)
	for k, c := range cb {
		for _, i := range c {
			switch i.Type {
			case CallbackWhenStateTimeout:
				stateTimeoutCb = i
			case CallbackBeforeEnter:
				appendCallback(beforeCallbacks, k, i)
			case CallbackAfterEnter:
				appendCallback(afterCallbacks, k, i)
			default:
				return nil, errors.New("unsupported callback type")
			}
		}
	}
	if stateTimeoutCb == nil {
		stateTimeoutCb = noopCallback()
	}
	history := NewHistory()
	history.AddState(initState)
	return &FSM{
		afterCallbacks:       afterCallbacks,
		beforeCallbacks:      beforeCallbacks,
		transitions:          trn,
		current:              initState,
		history:              history,
		stateTimeoutCallback: stateTimeoutCb,
		timer:                timer,
		timeout:              timeout,
		pingCh:               make(chan struct{}),
		doneCh:               make(chan struct{}, 1),
		queue:                []*Event{},
		logger:               logger,
		ctx:                  ctx,
	}, nil
}

// FSM is a finate state machine.
// Before and after callbacks for the same source state can be defined.
// If several callbacks are provided for each type, all of them are executed in order.
type FSM struct {
	afterCallbacks       map[string][]*Callback
	beforeCallbacks      map[string][]*Callback
	transitions          map[TransitionID]*Transition
	stateTimeoutCallback *Callback
	current              string
	history              *History
	pingCh               chan struct{}
	doneCh               chan struct{}
	timer                *time.Timer
	timeout              time.Duration
	queue                []*Event
	logger               *zap.SugaredLogger
	mux                  sync.Mutex
	ctx                  context.Context
}

// Write sends an event to the FSM FIFO queue and notifies the processor that new event arrived.
func (f *FSM) Write(event *Event) {
	f.mux.Lock()
	defer f.mux.Unlock()
	f.queue = append(f.queue, event)
	go func() {
		f.pingCh <- struct{}{}
	}()
}

// History returns the state transition history.
func (f *FSM) History() *History {
	return f.history
}

// Current returns the current state of FSM.
func (f *FSM) Current() string {
	f.mux.Lock()
	defer f.mux.Unlock()
	return f.current
}

// Run consumes events from the queue until an error occurs or the FSM has been stopped.
// The error is caused either by an unregistered event or by the callback itself.
// If the FSM was stopped its state is updated, the timer is stopped and the error channel is closed.
// The method is blocking and must be started exactly once.
func (f *FSM) Run(errChan chan error) {
	for {
		select {
		case <-f.pingCh:
			if err := f.process(); err != nil {
				f.current = Stopped
				errChan <- err
				return
			}
		case <-f.timer.C:
			f.stateTimeoutCallback.Action(f.stateTimeoutEvent())
		case <-f.ctx.Done():
			f.current = Stopped
			f.timer.Stop()
			return
		case <-f.doneCh:
			f.current = Stopped
			f.timer.Stop()
			return
		}

	}
}

// Stop stops the FSM. No other state transition is possible after the call.
// This method must be called only once, subsequent calls might be blocked infinitely.
func (f *FSM) Stop() {
	f.doneCh <- struct{}{}
}

// process reads in event from the queue, updates the state history and executes the transition.
func (f *FSM) process() error {
	f.mux.Lock()
	defer f.mux.Unlock()
	if len(f.queue) < 1 {
		// This should never happen with the current implementation.
		return errors.New("the number of events is out of sync with received pings")
	}
	event := f.queue[0]
	f.queue = f.queue[1:]
	f.history.AddEvent(event)
	trID := TransitionID{
		Source: f.current,
		Event:  event.Name,
	}
	// Specific state transition superceeds the general one, e.g.
	// if there is a transition with a specified source state it is followed,
	// otherwise a transition mathing any state "*" is specified.
	tr, ok := f.transitions[trID]
	if !ok {
		trID = TransitionID{
			Source: "*",
			Event:  event.Name,
		}
		tr, ok = f.transitions[trID]
		if !ok {
			return errors.New("unregistered event received")
		}
	}
	err := f.doTransition(tr, event)
	if err != nil {
		return err
	}
	return nil
}

// doTransition executes the transition to the next state.
// If there are any before- and after- callbacks, they are executed. The state timeout counter is reset as well.
func (f *FSM) doTransition(tr *Transition, event *Event) error {
	f.logger.Debugf("FSM Transition %v\n", tr)
	// Run possible callbacks before state transition.
	err := f.runCallbackIfExists(f.beforeCallbacks, tr.Dst, event)
	if err != nil {
		return err
	}
	// Transition to the next state.
	f.current = tr.Dst
	f.history.AddState(f.current)
	// Reset state timeout.
	// See the description of time.Reset for the reasoning of the complicated setup.
	f.timer.Stop()
	if !f.timer.Stop() && len(f.timer.C) > 0 {
		<-f.timer.C
	}
	f.timer.Reset(f.timeout)
	// Run callbacks after state transition.
	err = f.runCallbackIfExists(f.afterCallbacks, f.current, event)
	if err != nil {
		return err
	}
	return nil
}

// runCallbackIfExists executes a callback for a given state if it exists, does nothing otherwise.
// It returns an error if user callback fails.
func (f *FSM) runCallbackIfExists(callbacks map[string][]*Callback, state string, event *Event) error {
	callbacksBySource, ok := callbacks[state]
	if ok {
		for _, cb := range callbacksBySource {
			err := cb.Action(event)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// stateTimeoutEvent returns an event containing only an fsm reference.
func (f *FSM) stateTimeoutEvent() *Event {
	return &Event{
		Name: "_StateTimeout",
		Meta: &Metadata{FSM: f},
	}
}

func noopCallback() *Callback {
	return &Callback{
		Action: func(interface{}) error {
			fmt.Println("Noop callback was executed.")
			return nil
		},
	}
}

func appendCallback(mp map[string][]*Callback, k string, i *Callback) {
	cb, ok := mp[k]
	if !ok {
		cb = []*Callback{}
	}
	mp[k] = append(cb, i)
}

// NewHistory returns an empty fsm history.
func NewHistory() *History {
	return &History{
		received: []*Event{},
		states:   []string{},
	}
}

// History contains all received events and passed states including the current one.
type History struct {
	received  []*Event
	states    []string
	eventLock sync.Mutex
	stateLock sync.Mutex
}

// AddEvent writes a new event to the history.
func (h *History) AddEvent(ev *Event) {
	h.eventLock.Lock()
	defer h.eventLock.Unlock()
	h.received = append(h.received, ev)
}

// GetEvents returns a list of all events.
func (h *History) GetEvents() []*Event {
	h.eventLock.Lock()
	defer h.eventLock.Unlock()
	return h.received
}

// AddState saves the state to the history.
func (h *History) AddState(st string) {
	h.stateLock.Lock()
	defer h.stateLock.Unlock()
	h.states = append(h.states, st)
}

// GetStates returns passed states of FSM including the current one.
func (h *History) GetStates() []string {
	h.stateLock.Lock()
	defer h.stateLock.Unlock()
	return h.states
}

// Event is an event consumed by FSM.
type Event struct {
	Name   string
	GameID string
	Meta   *Metadata
}

// Metadata contains metada of an FSM event.
type Metadata struct {
	FSM          *FSM
	TargetTopic  string
	SrcTopics    []string
	TransportMsg *proto.Event
}

// TransitionID is a tuple containing external Event and source State.
type TransitionID struct {
	Event, Source string
}

// Transition defines a transition between FSM states.
type Transition struct {
	ID              TransitionID
	Event, Src, Dst string
}

// WhenIn specifies the source state of the transition.
func WhenIn(state string) *Transition {
	return &Transition{Src: state}
}

// WhenInAnyState targets transition from all states.
func WhenInAnyState() *Transition {
	return &Transition{Src: "*"}
}

// GotEvent specifies the triggering event for the transition.
func (i *Transition) GotEvent(event string) *Transition {
	i.Event = event
	i.ID = TransitionID{
		Event:  event,
		Source: i.Src,
	}
	return i
}

// GoTo specifies the destination State.
func (i *Transition) GoTo(dst string) *Transition {
	i.Dst = dst
	return i
}

// Stay forces the transition to stay in the source state.
func (i *Transition) Stay() *Transition {
	i.Dst = i.Src
	return i
}

// Action is a user defined function executed in the callback.
type Action func(interface{}) error

const (
	// CallbackAfterEnter is a callback type which is triggered when a new state just entered.
	CallbackAfterEnter = "AfterEnter"
	// CallbackBeforeEnter is a callback type which is triggered when a new state just entered.
	CallbackBeforeEnter = "BeforeEnter"
	// CallbackWhenStateTimeout is a type of callback which is triggered when state timeout is reached.
	CallbackWhenStateTimeout = "WhenStateTimeout"
)

// Callback is a function which is executed as a response to event during state transition.
type Callback struct {
	Type   string
	Src    string
	Action Action
}

// AfterEnter defines state this callback is bound to.
func AfterEnter(state string) *Callback {
	return &Callback{
		Type: CallbackAfterEnter,
		Src:  state,
	}
}

// BeforeEnter defines callback which is executed before entering the state.
func BeforeEnter(state string) *Callback {
	return &Callback{
		Type: CallbackBeforeEnter,
		Src:  state,
	}
}

// WhenStateTimeout defines a callback which is called when state timeout is reached.
func WhenStateTimeout() *Callback {
	return &Callback{
		Type: CallbackWhenStateTimeout,
	}
}

// Do defines a function to execute in the callback.
func (c *Callback) Do(a Action) *Callback {
	c.Action = a
	return c
}

// InitCallbacksAndTransitions converts slices to maps.
func InitCallbacksAndTransitions(cbs []*Callback, trs []*Transition) (map[string][]*Callback, map[TransitionID]*Transition) {
	callbacks := map[string][]*Callback{}
	transitions := map[TransitionID]*Transition{}
	for _, c := range cbs {
		callbacksBySource, ok := callbacks[c.Src]
		if !ok {
			callbacksBySource = []*Callback{}
		}
		callbacks[c.Src] = append(callbacksBySource, c)
	}
	for _, t := range trs {
		transitions[t.ID] = t
	}
	return callbacks, transitions
}
