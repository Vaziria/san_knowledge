package engine

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/wargasipil/san_message"
	messagev1 "github.com/wargasipil/san_message/gen/san/message/v1"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/store"
)

// A Topic Event step's Ack says when its events are acked (the user,
// 2026-11-08, for "upload twitter to vidara": the waiting events stay in
// the topic instead of becoming runs that wait):
//   - On Start (the default, as before): an event is acked as soon as its
//     run is recorded, so the broker sends the next at once and runs go side
//     by side.
//   - After Run: the trigger's stream takes one event, runs the workflow on
//     it there and then, and acks it when the run ends, succeeded or failed,
//     or earlier at its Ack step. San Message holds one unacked event per
//     pull stream and one stream per subscriber (packages/san_message,
//     pull.go), so the next event stays in the topic, its backlog, until
//     then: strictly one run at a time per trigger. A tower that stops during
//     the run leaves the event unacked: the run is Interrupted, and the
//     broker sends the event again after the restart, which runs it again as
//     a new run (its trigger ends with againNote), as a one-at-a-time queue
//     does (queue.go).
//
// Ack Time is how long the broker waits for the ack before it sends the
// event again (Hello.ack_timeout_seconds): empty is the broker's own (30 s)
// for On Start and 6 h for After Run, at most 24 h (the broker's limit). An
// event sent again while the tower still runs, after its run ended (the ack
// was late or lost), is acked without a run (afterDone).
//
// One at a time with After Run: the trigger's events do not go through the
// workflow's queue too (they would wait in two queues), but every run of a
// one-at-a-time workflow takes the workflow's turn (takeTurn) before it
// runs, the queue's reader too: a manual run or a Telegram message that
// comes during a topic run waits in the queue, and the topic event waits
// for the queue's run going on, so two runs of the workflow never overlap.

// The Topic Event step's Ack choices.
const (
	AckOnStart  = "On Start"
	AckAfterRun = "After Run"
)

// afterRunAckTime is an After Run trigger's Ack Time when it is empty.
const afterRunAckTime = 6 * time.Hour

// topicAck reads a Topic Event step's Ack and Ack Time: whether its events
// are acked after their runs, and how long the broker waits for an ack (0:
// the broker's own).
func topicAck(values map[string]string) (afterRun bool, ackTime time.Duration, err error) {
	switch a := strings.TrimSpace(values["Ack"]); a {
	case "", AckOnStart:
	case AckAfterRun:
		afterRun = true
	default:
		return false, 0, fmt.Errorf("Ack %q: pick %s or %s", a, AckOnStart, AckAfterRun)
	}
	s := strings.TrimSpace(values["Ack Time"])
	if s == "" {
		if afterRun {
			return true, afterRunAckTime, nil
		}
		return false, 0, nil
	}
	d, err := days(s)
	// The broker counts whole seconds: a part of one counts as one.
	if r := d % time.Second; err == nil && r > 0 {
		d += time.Second - r
	}
	if err != nil || d <= 0 || d > san_message.MaxAckTimeout {
		return afterRun, 0, fmt.Errorf(`Ack Time %q: give a time such as "30s", "6h" or "1d", at most 24h`, s)
	}
	return afterRun, d, nil
}

// hello is the first message of trigger t's Subscribe stream.
func (t trigger) hello() *messagev1.SubscribeRequest {
	return &messagev1.SubscribeRequest{Payload: &messagev1.SubscribeRequest_Hello{Hello: &messagev1.Hello{
		Subscriber: t.key, AckTimeoutSeconds: uint32(t.ackTime / time.Second),
	}}}
}

// source is what a Topic Event trigger's state says it waits for.
func (t trigger) source() string {
	if t.afterRun {
		return fmt.Sprintf("topic %s, ack after run (%s)", t.topic, ackTimeText(t.ackTime))
	}
	return "topic " + t.topic
}

// ackTimeText is d as Ack Time is typed: "6h", "1h30m", "45s".
func ackTimeText(d time.Duration) string {
	s := d.String() // "6h0m0s"
	if strings.HasSuffix(s, "m0s") {
		s = strings.TrimSuffix(s, "0s")
	}
	if strings.HasSuffix(s, "h0m") {
		s = strings.TrimSuffix(s, "0m")
	}
	return s
}

// workflowTurn is the turn key of workflow id's runs (takeTurn).
func workflowTurn(id uint32) string { return fmt.Sprintf("workflow %d", id) }

// takeTurn waits until no other holder of key's turn runs, or ctx ends
// (false). release gives the turn to the next who waits, in the order they
// came. The keys are a one-at-a-time workflow's (workflowTurn) and an After
// Run trigger's subscriber key.
func (e *Engine) takeTurn(ctx context.Context, key string) (release func(), ok bool) {
	e.qmu.Lock()
	t := e.turns[key]
	if t == nil {
		t = make(chan struct{}, 1)
		e.turns[key] = t
	}
	e.qmu.Unlock()
	select {
	case t <- struct{}{}:
		return func() { <-t }, true
	case <-ctx.Done():
		return nil, false
	}
}

// eventStamp tells one event of a topic from another, even a topic made
// again whose ids start over.
type eventStamp struct {
	id      uint64
	created int64
}

// afterDoneKept is how many events an After Run trigger remembers as run.
const afterDoneKept = 16

// afterDone says whether trigger key's run of ev ended in this tower: an
// event the broker sends again after that is acked without a run.
func (e *Engine) afterDone(key string, ev *messagev1.Event) bool {
	e.qmu.Lock()
	defer e.qmu.Unlock()
	for _, s := range e.afterRan[key] {
		if s == (eventStamp{ev.GetId(), ev.GetCreatedMs()}) {
			return true
		}
	}
	return false
}

// ranAfter remembers that trigger key's run of ev ended.
func (e *Engine) ranAfter(key string, ev *messagev1.Event) {
	e.qmu.Lock()
	defer e.qmu.Unlock()
	list := append(e.afterRan[key], eventStamp{ev.GetId(), ev.GetCreatedMs()})
	if len(list) > afterDoneKept {
		list = list[len(list)-afterDoneKept:]
	}
	e.afterRan[key] = list
}

// runAfter runs workflow workflowID from its After Run trigger stepID on
// event ev, on the trigger's stream, and says whether the event is done
// with (to ack). ack acks it early (the Ack step). It waits for the
// trigger's turn, and a one-at-a-time workflow's, first; ctx ending while
// it waits, or the tower stopping during the run, leaves the event to come
// again.
func (e *Engine) runAfter(ctx context.Context, key string, workflowID uint32, stepID string, ev *messagev1.Event, ack func() error) (bool, error) {
	if e.afterDone(key, ev) {
		e.log.Info("a trigger's event came again after its run ended: acked, not run again", "subscriber", key, "event", ev.GetId())
		return true, nil
	}
	release, ok := e.takeTurn(ctx, key)
	if !ok {
		return false, nil
	}
	defer release()
	w, err := e.store.GetWorkflow(workflowID)
	if errors.Is(err, store.ErrNotFound) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	if !w.Active {
		return true, nil
	}
	if w.OneAtATime {
		release, ok := e.takeTurn(ctx, workflowTurn(w.ID))
		if !ok {
			return false, nil
		}
		defer release()
		// It may have changed while it waited.
		if w, err = e.store.GetWorkflow(workflowID); errors.Is(err, store.ErrNotFound) {
			return true, nil
		} else if err != nil {
			return false, err
		}
	}
	trigger := fmt.Sprintf("topic %s, event %d", ev.GetTopic(), ev.GetId())
	if last, found, err := e.store.LastRunByTrigger(w.ID, trigger, trigger+againNote); err == nil && found && last.Status == v1.RunStatus_RUN_STATUS_INTERRUPTED {
		trigger += againNote
	}
	r, err := e.record(w, stepID, trigger)
	switch {
	case errors.Is(err, ErrBadStart):
		return true, nil
	case err != nil:
		e.log.Error("record a workflow run", "workflow", w.ID, "err", err)
		return false, err
	}
	fr, _ := e.execute(w, r, stepID, ev.GetData(), ack)
	if fr.Status == v1.RunStatus_RUN_STATUS_INTERRUPTED {
		return false, nil
	}
	e.ranAfter(key, ev)
	return true, nil
}
