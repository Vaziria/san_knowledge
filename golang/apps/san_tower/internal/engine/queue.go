package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"
	messagev1 "github.com/wargasipil/san_message/gen/san/message/v1"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/broker"
	"github.com/wargasipil/san_tower/internal/store"
)

// A one-at-a-time workflow's runs (Workflow.OneAtATime; the user,
// 2026-10-27: X links sent in a burst ran together and failed together)
// queue in the message broker, so the queue outlives the tower (the user:
// "Why we don't use send message broker and if success or fail, its ack").
//
//   - begin publishes each run, as its event came, to the workflow's queue
//     topic, tower-queue-<id>; the run waits as Running.
//   - The broker does not wait for a pull stream's ack, and the reader asks
//     to hold each event for up to queueAckTimeout, so a run of minutes
//     neither holds up the broker's other subscribers nor comes twice.
//   - The workflow's queue reader, a pull subscriber of that topic with one
//     Subscribe stream, takes the runs one at a time and runs each to its
//     end before it takes the next.
//   - A run's event is acked when the run ends, succeeded or failed, or
//     earlier by an Ack step (the user: "And we add ack node"). The broker
//     keeps an event that is not acked, so a run the tower stopped before
//     its ack runs again, as a new run, when the tower starts; one that ran
//     to its end, but whose ack was lost, is not run twice.

// queueAckTimeout is how long a queue's reader may hold a run's event, its
// run going, before the broker sends it again: longer than any run.
const queueAckTimeout = 6 * time.Hour

// queueKey is the topic of workflow id's queue, and its reader's subscriber.
func queueKey(id uint32) string { return fmt.Sprintf("tower-queue-%d", id) }

// queueWorkflow is the workflow a queue's subscriber key belongs to.
func queueWorkflow(key string) (uint32, bool) {
	rest, ok := strings.CutPrefix(key, "tower-queue-")
	if !ok {
		return 0, false
	}
	id, err := strconv.ParseUint(rest, 10, 32)
	return uint32(id), err == nil
}

// queued is a run as its queue holds it.
type queued struct {
	Run  uint32 `json:"run"`
	Step string `json:"step"`
	Data []byte `json:"data,omitempty"`
}

// waiter is a run this tower queued, waiting in finish for its end.
type waiter struct {
	workflow uint32
	started  bool           // the reader took it; guarded by Engine.qmu
	done     chan store.Run // gets the run as it ended
}

// queueURL is where the broker in use answers, for the queues.
func (e *Engine) queueURL() string {
	if uri := e.broker.Remote(); uri != "" {
		return uri
	}
	e.qmu.Lock()
	defer e.qmu.Unlock()
	return e.qbase
}

// enqueue publishes a begun run to its workflow's queue, and starts the
// queue's reader if none is reading.
func (e *Engine) enqueue(w store.Workflow, r store.Run, stepID string, data []byte) error {
	body, err := json.Marshal(queued{Run: r.ID, Step: stepID, Data: data})
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(e.ctx, callTimeout)
	defer cancel()
	if err := e.ensureQueue(ctx, w.ID); err != nil {
		return err
	}
	e.qmu.Lock()
	e.waiting[r.ID] = &waiter{workflow: w.ID, done: make(chan store.Run, 1)}
	e.qmu.Unlock()
	if _, err := e.broker.Publish(ctx, queueKey(w.ID), body); err != nil {
		e.qmu.Lock()
		delete(e.waiting, r.ID)
		e.qmu.Unlock()
		return err
	}
	e.startReader(w.ID)
	return nil
}

// ensureQueue makes workflow id's queue topic and its reader's subscriber,
// which starts at the start of the topic's log, so no run published before
// it is missed. Done once per broker.
func (e *Engine) ensureQueue(ctx context.Context, id uint32) error {
	url := e.queueURL()
	if url == "" {
		return errors.New("the tower has not started")
	}
	e.qmu.Lock()
	ready := e.queues[id] == url
	e.qmu.Unlock()
	if ready {
		return nil
	}
	c := broker.NewClient(url)
	key := queueKey(id)
	_, err := c.CreateTopic(ctx, connect.NewRequest(&messagev1.CreateTopicRequest{Topic: key}))
	if err != nil && connect.CodeOf(err) != connect.CodeAlreadyExists {
		return fmt.Errorf("make the queue's topic: %w", err)
	}
	_, err = c.CreateSubscriber(ctx, connect.NewRequest(&messagev1.CreateSubscriberRequest{
		Subscriber: key, Topic: key, Type: messagev1.SubscriberType_SUBSCRIBER_TYPE_PULL,
		Start: messagev1.SubscriberStart_SUBSCRIBER_START_EARLIEST,
	}))
	if err != nil && connect.CodeOf(err) != connect.CodeAlreadyExists {
		return fmt.Errorf("make the queue's reader: %w", err)
	}
	e.qmu.Lock()
	e.queues[id] = url
	e.qmu.Unlock()
	return nil
}

// startReader starts reading workflow id's queue, unless it is read.
func (e *Engine) startReader(id uint32) {
	e.qmu.Lock()
	defer e.qmu.Unlock()
	if e.readers[id] != nil {
		return
	}
	ctx, cancel := context.WithCancel(e.ctx)
	if err := e.background(func() { e.readQueue(ctx, id) }); err != nil {
		cancel()
		return
	}
	e.readers[id] = cancel
}

// stopReaders stops reading the queues of the workflows keep says no to,
// and forgets which queues are made, as when the broker changed.
func (e *Engine) stopReaders(keep func(id uint32) bool) {
	e.qmu.Lock()
	defer e.qmu.Unlock()
	for id, cancel := range e.readers {
		if !keep(id) {
			cancel()
			delete(e.readers, id)
			delete(e.queues, id)
		}
	}
}

// dropQueue removes a deleted workflow's queue: its reader, subscriber and
// topic, and lets the runs that waited in it go.
func (e *Engine) dropQueue(url string, id uint32) {
	e.stopReaders(func(r uint32) bool { return r != id })
	e.qmu.Lock()
	delete(e.queues, id)
	for runID, w := range e.waiting {
		if w.workflow == id {
			delete(e.waiting, runID)
			close(w.done)
		}
	}
	e.qmu.Unlock()
	ctx, cancel := context.WithTimeout(e.ctx, callTimeout)
	defer cancel()
	c := broker.NewClient(url)
	_, err := c.DeleteSubscriber(ctx, connect.NewRequest(&messagev1.DeleteSubscriberRequest{Subscriber: queueKey(id)}))
	if err == nil || connect.CodeOf(err) == connect.CodeNotFound {
		_, err = c.DeleteTopic(ctx, connect.NewRequest(&messagev1.DeleteTopicRequest{Topic: queueKey(id)}))
	}
	if err != nil && connect.CodeOf(err) != connect.CodeNotFound {
		e.log.Warn("delete a workflow's queue", "workflow", id, "err", err)
	}
}

// syncQueues reads the queues of the workflows that run one at a time, and
// of those whose queue still holds runs (the setting was switched off), and
// removes the queues of workflows that are gone. subs are the broker's
// subscribers. Called with e.mu held.
func (e *Engine) syncQueues(url string, list []store.Workflow, subs []*messagev1.Subscriber, known map[uint32]bool) {
	read := make(map[uint32]bool)
	for _, w := range list {
		if w.OneAtATime {
			read[w.ID] = true
		}
	}
	for _, s := range subs {
		if id, ok := queueWorkflow(s.Key); ok {
			if known[id] {
				read[id] = true
			} else {
				e.dropQueue(url, id)
			}
		}
	}
	// The broker may be another: read afresh.
	e.stopReaders(func(uint32) bool { return false })
	for id := range read {
		ctx, cancel := context.WithTimeout(e.ctx, callTimeout)
		err := e.ensureQueue(ctx, id)
		cancel()
		if err != nil {
			e.log.Error("make a workflow's queue", "workflow", id, "err", err)
			continue
		}
		e.startReader(id)
	}
}

// readQueue reads workflow id's queue until ctx ends, opening its stream
// again whenever it ends.
func (e *Engine) readQueue(ctx context.Context, id uint32) {
	wait := minRetry
	for {
		began := time.Now()
		err := e.queueStream(ctx, id)
		if ctx.Err() != nil {
			return
		}
		e.log.Warn("a workflow's queue stream ended", "workflow", id, "err", err)
		if time.Since(began) > maxRetry {
			wait = minRetry
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
		wait = min(2*wait, maxRetry)
	}
}

// queueStream takes a queue's runs from one Subscribe stream, one at a time:
// each runs to its end, and is acked then unless its Ack step acked it, or
// the tower is stopping (it runs again when the tower starts).
func (e *Engine) queueStream(ctx context.Context, id uint32) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	s := broker.NewClient(e.queueURL()).Subscribe(ctx)
	defer s.CloseResponse()
	defer s.CloseRequest()
	// A Receive waiting for the next event does not see ctx end once an
	// event came: end the request then, and the broker ends the stream. sendMu
	// keeps that from running beside a Send.
	var sendMu sync.Mutex
	send := func(m *messagev1.SubscribeRequest) error {
		sendMu.Lock()
		defer sendMu.Unlock()
		return s.Send(m)
	}
	stop := context.AfterFunc(ctx, func() {
		sendMu.Lock()
		defer sendMu.Unlock()
		_ = s.CloseRequest()
	})
	defer stop()
	hello := &messagev1.SubscribeRequest{Payload: &messagev1.SubscribeRequest_Hello{Hello: &messagev1.Hello{
		Subscriber: queueKey(id), AckTimeoutSeconds: uint32(queueAckTimeout / time.Second),
	}}}
	if err := send(hello); err != nil {
		return err
	}
	for {
		r, err := s.Receive()
		if err != nil {
			return err
		}
		ev := r.GetEvent()
		acked := false
		ack := func() error {
			if acked {
				return nil
			}
			err := send(&messagev1.SubscribeRequest{Payload: &messagev1.SubscribeRequest_Ack{Ack: &messagev1.Ack{EventId: ev.GetId()}}})
			acked = err == nil
			return err
		}
		done, err := e.take(id, ev.GetData(), ack)
		if err != nil {
			return err
		}
		if !done || e.ctx.Err() != nil {
			return errors.New("the tower is stopping")
		}
		if err := ack(); err != nil {
			return err
		}
	}
}

// take runs a run its queue gave, and says whether its event is done with.
// A run that ended already (its ack was lost) or is gone is done with; one
// the tower stopped (Interrupted) runs again as a new run.
func (e *Engine) take(workflowID uint32, data []byte, ack func() error) (bool, error) {
	var q queued
	if err := json.Unmarshal(data, &q); err != nil || q.Run == 0 {
		e.log.Warn("not a queued run: skipped", "workflow", workflowID)
		return true, nil
	}
	w, err := e.store.GetWorkflow(workflowID)
	if errors.Is(err, store.ErrNotFound) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	r, err := e.store.GetRun(q.Run)
	switch {
	case errors.Is(err, store.ErrNotFound):
		return true, nil
	case err != nil:
		return false, err
	case r.Status == v1.RunStatus_RUN_STATUS_SUCCEEDED || r.Status == v1.RunStatus_RUN_STATUS_FAILED:
		return true, nil
	case r.Status != v1.RunStatus_RUN_STATUS_RUNNING:
		trigger := strings.TrimSuffix(r.Trigger, againNote) + againNote
		if r, err = e.store.StartRun(w.ID, trigger); err != nil {
			return false, err
		}
	}
	e.qmu.Lock()
	wt := e.waiting[q.Run]
	if wt != nil {
		wt.started = true
	}
	e.qmu.Unlock()
	fr, _ := e.execute(w, r, q.Step, q.Data, ack)
	if wt != nil {
		e.qmu.Lock()
		delete(e.waiting, q.Run)
		e.qmu.Unlock()
		wt.done <- fr
	}
	return fr.Status != v1.RunStatus_RUN_STATUS_INTERRUPTED, nil
}

// againNote ends the trigger of a run that runs again: the tower stopped
// before its run was done with.
const againNote = " (again: the tower stopped before it was done)"

// await waits for a run this tower queued to end, and gives it as it ended.
// ok is false for a run that was not queued. A tower that stops before the
// run's turn marks it Interrupted: its event stays in the queue, and it runs
// when the tower starts again.
func (e *Engine) await(r store.Run) (store.Run, bool, error) {
	e.qmu.Lock()
	wt := e.waiting[r.ID]
	e.qmu.Unlock()
	if wt == nil {
		return r, false, nil
	}
	select {
	case fr, ok := <-wt.done:
		if !ok {
			return r, true, errors.New("the workflow was deleted")
		}
		return fr, true, nil
	case <-e.ctx.Done():
	}
	e.qmu.Lock()
	started := wt.started
	if !started {
		delete(e.waiting, r.ID)
	}
	e.qmu.Unlock()
	if started { // the reader is ending it
		fr, ok := <-wt.done
		if !ok {
			return r, true, errors.New("the workflow was deleted")
		}
		return fr, true, nil
	}
	fr, err := e.store.FinishRun(r, v1.RunStatus_RUN_STATUS_INTERRUPTED, "the tower stopped before the run's turn came; it runs when the tower starts again", nil)
	return fr, true, err
}

// noteWait tells, in the trigger step's log, how long a run waited for its
// turn.
func noteWait(steps []*v1.StepRun, waited time.Duration) {
	if waited < time.Second || len(steps) == 0 {
		return
	}
	st := steps[0]
	line := fmt.Sprintf("waited %s for the runs before it (one run at a time)", waited.Round(time.Second))
	if len(st.LogLevels) == len(st.Logs) {
		st.LogLevels = append([]v1.LogLevel{v1.LogLevel_LOG_LEVEL_INFO}, st.LogLevels...)
	}
	st.Logs = append([]string{line}, st.Logs...)
}
