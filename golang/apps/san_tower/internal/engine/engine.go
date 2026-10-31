// Package engine runs the tower's workflows. kinds.go defines the kinds of
// step, run.go runs a workflow's graph, and this file records the runs and
// keeps the triggers of active workflows listening.
//
// A Topic Event step of an active workflow is a pull subscriber on its
// topic, in the broker the tower uses, named workflow-<id>-<step id>. The
// engine keeps a Subscribe stream open for each: to the tower's own address
// when the broker runs inside it, or to the separate server, so a broker on
// another machine needs no way back to the tower. An event is acked once
// its run is recorded as started; the steps run after. Since the
// subscribers live in the broker, events that come while the tower is down
// run once it is back.
//
// A Schedule step of an active workflow is a timer in the engine
// (schedule.go), which needs no broker: it starts a run each time its Every
// comes. A time missed while the tower was stopped is skipped.
//
// Runs go on in the background, under the engine's own context: a step that
// posts takes a minute or two, and a run by hand should not stop halfway
// when the page that asked goes away. Close ends them; one cut short is
// recorded as interrupted.
package engine

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"
	"github.com/wargasipil/san_message"
	messagev1 "github.com/wargasipil/san_message/gen/san/message/v1"
	"google.golang.org/protobuf/proto"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/broker"
	"github.com/wargasipil/san_tower/internal/store"
)

const subscriberPrefix = "workflow-"

// callTimeout bounds each call the engine makes to the broker for the
// triggers' subscribers.
const callTimeout = 10 * time.Second

// A trigger's stream that ends is opened again after a wait that doubles
// from minRetry up to maxRetry, back to minRetry once a stream lasted.
const (
	minRetry = time.Second
	maxRetry = 30 * time.Second
)

type Engine struct {
	store  *store.Store
	broker *broker.Broker
	log    *slog.Logger

	ctx    context.Context // the runs' and the streams'; Close cancels it
	cancel context.CancelFunc
	runs   sync.WaitGroup // the runs and the streams
	runMu  sync.Mutex     // guards closed, so nothing starts once Close waits
	closed bool

	// The lines running runs log as they go, by run id (live.go).
	liveMu sync.Mutex
	live   map[uint32]*liveLog

	mu        sync.Mutex // serializes changes to the triggers, and guards their states
	base      string     // the tower's own address; empty until Start
	states    map[uint32][]*v1.TriggerState
	listeners map[string]*listener // by subscriber key
	// The Schedule steps' timers, by the same key, and their states.
	timers      map[string]*timer
	timerStates map[uint32][]*v1.TriggerState
	// The Telegram bots' pollers, by connection id, and the Telegram
	// Message steps of active workflows, by workflow id.
	pollers map[uint32]*poller
	tgSteps map[uint32][]tgStep
	// The Log Event steps of active workflows, by workflow id.
	logSteps map[uint32][]logStep
	// The Tower Chat steps of active workflows, by workflow id (towerchat.go).
	chatSteps map[uint32][]string
	// The Folder Watch and File Watch steps watching, by the same key as the
	// timers, and the watch steps' states (watch.go).
	watchers    map[string]*watcher
	watchStates map[uint32][]*v1.TriggerState
	// The WhatsApp connections' devices, by connection id (whatsapp.go).
	wa map[uint32]*waConn
	// The one-at-a-time workflows' queues (queue.go), guarded by qmu: the
	// runs this tower queued, by run id; each queue's reader, by workflow
	// id; the broker each queue was made in; and the tower's own address.
	qmu     sync.Mutex
	waiting map[uint32]*waiter
	readers map[uint32]context.CancelFunc
	queues  map[uint32]string
	qbase   string

	// ssh are the shells SSH Session steps keep open from run to run,
	// closed when the engine closes.
	ssh *SSHShells

	// TelegramAPI is where the Bot API answers, telegram.API when empty;
	// tests set a fake's before Start.
	TelegramAPI string
	// GitHubAPI is where GitHub's REST API answers, github.API when empty;
	// tests set a fake's before Start.
	GitHubAPI string
	// Version is the tower's, for the Tower Restart steps (towerrestart.go).
	Version string
	// WhatsApp links the WhatsApp connections' devices (whatsapp.go); set
	// before Start. Without it the WhatsApp steps fail, saying so.
	WhatsApp WhatsAppLink

	// claudeVersion is what `claude --version` printed on first ask, with
	// its minimum (ClaudeCLIMinVersion) and the probe error if any; the UI's
	// startup banner reads it (claude.go).
	claudeVersionOnce sync.Once
	claudeVersion     ClaudeCLIProbe
}

// listener is a trigger's Subscribe stream, opened again when it ends.
type listener struct {
	cancel context.CancelFunc
	url    string // the broker's
	topic  string
	state  *v1.TriggerState // guarded by Engine.mu
}

func New(st *store.Store, b *broker.Broker, log *slog.Logger) *Engine {
	ctx, cancel := context.WithCancel(context.Background())
	return &Engine{
		store: st, broker: b, log: log, ctx: ctx, cancel: cancel,
		states: make(map[uint32][]*v1.TriggerState), listeners: make(map[string]*listener),
		timers: make(map[string]*timer), timerStates: make(map[uint32][]*v1.TriggerState),
		pollers: make(map[uint32]*poller), tgSteps: make(map[uint32][]tgStep),
		logSteps: make(map[uint32][]logStep), chatSteps: make(map[uint32][]string),
		watchers: make(map[string]*watcher), watchStates: make(map[uint32][]*v1.TriggerState),
		waiting: make(map[uint32]*waiter), readers: make(map[uint32]context.CancelFunc), queues: make(map[uint32]string),
		ssh: NewSSHShells(),
	}
}

// Close closes the triggers' streams, cuts the runs still going short, and
// waits until each is recorded. Calling it again does nothing more.
func (e *Engine) Close() {
	e.runMu.Lock()
	e.closed = true
	e.runMu.Unlock()
	e.cancel()
	e.runs.Wait()
	e.ssh.Close()
}

// background runs f as a run of the engine, unless it is closing.
func (e *Engine) background(f func()) error {
	e.runMu.Lock()
	defer e.runMu.Unlock()
	if e.closed {
		return errors.New("the tower is stopping")
	}
	e.runs.Add(1)
	go func() {
		defer e.runs.Done()
		f()
	}()
	return nil
}

// env is what the runs' steps reach: the tower's connections.
func (e *Engine) env() Env {
	return Env{
		Connection:       e.store.GetConnection,
		AddConnection:    e.addConnection,
		Publish:          e.publish,
		TelegramAPI:      e.TelegramAPI,
		GitHubAPI:        e.GitHubAPI,
		SSH:              e.ssh,
		KeepHostKey:      e.store.KeepSSHHostKey,
		TelegramMessages: e.store.TelegramMessages,
		WhatsAppSend:     e.sendWhatsApp,
		Storage:          e.store.GetStorage,
		Knowledge:        e.store.GetKnowledge,
		ChatLink:         e.store.ChatLink,
		RunBatch:         e.store.RunBatch,
		SetChatLink:      e.store.SetChatLink,
		DataDir:          e.store.Dir(),
		FindWorkflow:     e.store.GetWorkflowByName,
		RunWorkflow:      e.runForTool,
		ServeTools:       e.serveTools,
	}
}

// runForTool runs workflow id from its trigger, a Manual one first, as a
// Workflow Tool's run command asks, and waits for it.
func (e *Engine) runForTool(ctx context.Context, id uint32, data []byte) (store.Run, error) {
	w, err := e.store.GetWorkflow(id)
	if err != nil {
		return store.Run{}, err
	}
	start := ""
	for _, st := range w.Graph.GetSteps() {
		if st.GetKind() == Manual {
			start = st.GetId()
			break
		}
		if start == "" && IsTrigger(st.GetKind()) {
			start = st.GetId()
		}
	}
	if start == "" {
		return store.Run{}, fmt.Errorf("workflow %s has no trigger to run it from: give it a Manual one", w.Name)
	}
	return e.Run(ctx, id, start, data, "tool")
}

// addConnection adds a connection and starts what it needs, such as a
// Telegram bot's poller, as the Connections page's Create does.
func (e *Engine) addConnection(c store.Connection) (store.Connection, error) {
	c, err := e.store.CreateConnection(c)
	if err != nil {
		return c, err
	}
	e.SyncConnections()
	return c, nil
}

// Start marks the runs the tower stopped during as interrupted, then
// switches on the triggers of the active workflows. baseURL is the tower's
// own address, where the broker inside it answers.
func (e *Engine) Start(baseURL string) error {
	n, err := e.store.InterruptRuns()
	if err != nil {
		return err
	}
	if n > 0 {
		e.log.Warn("workflow runs interrupted by the last stop", "runs", n)
	}
	e.mu.Lock()
	e.base = strings.TrimRight(baseURL, "/")
	e.mu.Unlock()
	e.qmu.Lock()
	e.qbase = strings.TrimRight(baseURL, "/")
	e.qmu.Unlock()
	err = e.SyncAll()
	e.SyncConnections()
	// Back from a restart: the Tower Restart steps' After runs.
	e.afterRestart()
	return err
}

// brokerURL is where the broker in use answers MessageService.
func (e *Engine) brokerURL() string {
	if uri := e.broker.Remote(); uri != "" {
		return uri
	}
	return e.base
}

// SyncAll brings every workflow's triggers in line with the database, opens
// their streams afresh, and removes the subscribers of workflows that are
// gone: on start, and when the tower switched to another broker.
func (e *Engine) SyncAll() error {
	list, err := e.store.Workflows()
	if err != nil {
		return err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.base == "" {
		return nil
	}
	known := make(map[uint32]bool, len(list))
	for _, w := range list {
		known[w.ID] = true
		e.syncTimers(w)
		e.syncTelegram(w)
		e.syncLogs(w)
		e.syncChats(w)
		e.syncWatches(w)
	}
	e.dropTimers(known)
	e.dropWatches(known)
	for id := range e.tgSteps {
		if !known[id] {
			delete(e.tgSteps, id)
		}
	}
	for id := range e.logSteps {
		if !known[id] {
			delete(e.logSteps, id)
		}
	}
	for id := range e.chatSteps {
		if !known[id] {
			delete(e.chatSteps, id)
		}
	}
	for key, l := range e.listeners {
		l.cancel()
		delete(e.listeners, key)
	}
	clear(e.states)
	url := e.brokerURL()
	subs, err := e.subscribers(url)
	if err != nil {
		for _, w := range list {
			e.unreachable(w, err)
		}
		return err
	}
	for _, w := range list {
		e.sync(url, w, subs)
	}
	for _, s := range subs {
		if id, ok := subscriberWorkflow(s.Key); ok && !known[id] {
			e.deleteSubscriber(url, s.Key)
		}
	}
	e.syncQueues(url, list, subs, known)
	return nil
}

// Sync brings one workflow's triggers in line with the database, after it
// changed, was switched on or off, or was deleted.
func (e *Engine) Sync(id uint32) {
	w, err := e.store.GetWorkflow(id)
	if errors.Is(err, store.ErrNotFound) {
		w, err = store.Workflow{ID: id}, nil // gone: no triggers
	}
	if err != nil {
		e.log.Error("sync a workflow's triggers", "workflow", id, "err", err)
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.base == "" {
		return
	}
	e.syncTimers(w)
	e.syncTelegram(w)
	e.syncLogs(w)
	e.syncChats(w)
	e.syncWatches(w)
	url := e.brokerURL()
	subs, err := e.subscribers(url)
	if err != nil {
		e.log.Error("sync a workflow's triggers", "workflow", id, "err", err)
		e.unreachable(w, err)
		return
	}
	e.sync(url, w, subs)
	switch {
	case w.Graph == nil && w.Name == "": // gone
		e.dropQueue(url, id)
	case w.OneAtATime:
		ctx, cancel := context.WithTimeout(e.ctx, callTimeout)
		err := e.ensureQueue(ctx, id)
		cancel()
		if err != nil {
			e.log.Error("make a workflow's queue", "workflow", id, "err", err)
			return
		}
		e.startReader(id)
	}
}

// trigger is a Topic Event step of an active workflow, as its subscriber.
type trigger struct {
	key, topic string
	state      *v1.TriggerState
}

// triggers are workflow w's Topic Event steps while it is active, each with
// its state; those without a topic say so and are left out.
func triggers(w store.Workflow) (map[string]trigger, []*v1.TriggerState) {
	wanted := make(map[string]trigger)
	var states []*v1.TriggerState
	if !w.Active {
		return wanted, nil
	}
	for _, st := range w.Graph.GetSteps() {
		if st.GetKind() != TopicEvent {
			continue
		}
		topic := strings.TrimSpace(st.GetValues()["Topic"])
		ts := &v1.TriggerState{StepId: st.GetId(), Source: "topic " + topic}
		states = append(states, ts)
		if topic == "" {
			ts.Source, ts.Error = "topic", "no topic set"
			continue
		}
		key := fmt.Sprintf("%s%d-%s", subscriberPrefix, w.ID, st.GetId())
		wanted[key] = trigger{key, topic, ts}
	}
	return wanted, states
}

// unreachable records that workflow w's triggers cannot listen: the broker
// does not answer. Their streams stay closed until the next sync.
func (e *Engine) unreachable(w store.Workflow, err error) {
	wanted, states := triggers(w)
	for _, t := range wanted {
		t.state.Error = "the broker does not answer: " + err.Error()
		if l := e.listeners[t.key]; l != nil {
			l.cancel()
			delete(e.listeners, t.key)
		}
	}
	e.setStates(w.ID, states)
}

func (e *Engine) setStates(id uint32, states []*v1.TriggerState) {
	if len(states) == 0 {
		delete(e.states, id)
	} else {
		e.states[id] = states
	}
}

// sync makes the subscribers of workflow w, in the broker at url, those its
// triggers need: a pull subscriber per Topic Event step with a topic, while
// it is active, each with a stream open. A subscriber that already has the
// right topic is kept, so it keeps its place in the topic's log, and so is
// its stream.
func (e *Engine) sync(url string, w store.Workflow, subs []*messagev1.Subscriber) {
	prefix := fmt.Sprintf("%s%d-", subscriberPrefix, w.ID)
	wanted, states := triggers(w)
	have := make(map[string]bool)
	for _, s := range subs {
		if !strings.HasPrefix(s.Key, prefix) {
			continue
		}
		if t, ok := wanted[s.Key]; ok && s.Type == messagev1.SubscriberType_SUBSCRIBER_TYPE_PULL && s.Topic == t.topic {
			have[s.Key] = true
			continue
		}
		e.deleteSubscriber(url, s.Key)
	}
	c := broker.NewClient(url)
	for key, t := range wanted {
		if !have[key] {
			ctx, cancel := context.WithTimeout(e.ctx, callTimeout)
			_, err := c.CreateSubscriber(ctx, connect.NewRequest(&messagev1.CreateSubscriberRequest{
				Subscriber: key, Topic: t.topic, Type: messagev1.SubscriberType_SUBSCRIBER_TYPE_PULL,
			}))
			cancel()
			switch {
			case connect.CodeOf(err) == connect.CodeNotFound:
				t.state.Error = fmt.Sprintf("topic %s does not exist", t.topic)
				continue
			case err != nil:
				t.state.Error = err.Error()
				continue
			}
		}
		t.state.Listening = true
		if l := e.listeners[key]; l != nil && l.url == url && l.topic == t.topic {
			l.state = t.state
			continue
		}
		e.listen(url, w.ID, t)
	}
	// The streams of triggers no longer wanted, or no longer listening.
	for key, l := range e.listeners {
		if t, ok := wanted[key]; strings.HasPrefix(key, prefix) && (!ok || !t.state.Listening) {
			l.cancel()
			delete(e.listeners, key)
		}
	}
	e.setStates(w.ID, states)
}

// subscribers lists the subscribers of the broker at url.
func (e *Engine) subscribers(url string) ([]*messagev1.Subscriber, error) {
	ctx, cancel := context.WithTimeout(e.ctx, callTimeout)
	defer cancel()
	r, err := broker.NewClient(url).ListSubscribers(ctx, connect.NewRequest(&messagev1.ListSubscribersRequest{}))
	if err != nil {
		return nil, err
	}
	return r.Msg.Subscribers, nil
}

func (e *Engine) deleteSubscriber(url, key string) {
	if l := e.listeners[key]; l != nil {
		l.cancel()
		delete(e.listeners, key)
	}
	ctx, cancel := context.WithTimeout(e.ctx, callTimeout)
	defer cancel()
	_, err := broker.NewClient(url).DeleteSubscriber(ctx, connect.NewRequest(&messagev1.DeleteSubscriberRequest{Subscriber: key}))
	if err != nil && connect.CodeOf(err) != connect.CodeNotFound {
		e.log.Warn("delete a trigger's subscriber", "subscriber", key, "err", err)
	}
}

// listen opens trigger t's stream from the broker at url, and opens it
// again whenever it ends, until the trigger is synced away or the engine
// closes. Called with e.mu held.
func (e *Engine) listen(url string, workflowID uint32, t trigger) {
	if l := e.listeners[t.key]; l != nil {
		l.cancel()
	}
	ctx, cancel := context.WithCancel(e.ctx)
	l := &listener{cancel: cancel, url: url, topic: t.topic, state: t.state}
	e.listeners[t.key] = l
	stepID := strings.TrimPrefix(t.key, fmt.Sprintf("%s%d-", subscriberPrefix, workflowID))
	err := e.background(func() {
		wait := minRetry
		for {
			began := time.Now()
			err := e.stream(ctx, l, t.key, workflowID, stepID)
			if ctx.Err() != nil {
				return
			}
			e.mu.Lock()
			if ctx.Err() == nil {
				l.state.Listening = false
				l.state.Error = "not connected to the broker: " + err.Error()
			}
			e.mu.Unlock()
			e.log.Warn("a trigger's stream ended", "subscriber", t.key, "err", err)
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
	})
	if err != nil {
		cancel()
		delete(e.listeners, t.key)
	}
}

// stream reads trigger events from one Subscribe stream until it ends. Each
// event is acked once its run is recorded as started. An event that cannot
// be recorded is not acked: the stream ends, and the broker sends the event
// again on the next.
func (e *Engine) stream(ctx context.Context, l *listener, key string, workflowID uint32, stepID string) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	s := broker.NewClient(l.url).Subscribe(ctx)
	defer s.CloseResponse()
	defer s.CloseRequest()
	// A Receive waiting for the next event does not see ctx end once an
	// event came (queue.go): end the request then. The acks are sent from
	// this goroutine, so sendMu keeps the two apart.
	var sendMu sync.Mutex
	stop := context.AfterFunc(ctx, func() {
		sendMu.Lock()
		defer sendMu.Unlock()
		_ = s.CloseRequest()
	})
	defer stop()
	hello := &messagev1.SubscribeRequest{Payload: &messagev1.SubscribeRequest_Hello{Hello: &messagev1.Hello{Subscriber: key}}}
	sendMu.Lock()
	err := s.Send(hello)
	sendMu.Unlock()
	if err != nil {
		return err
	}
	e.mu.Lock()
	if ctx.Err() == nil {
		l.state.Listening, l.state.Error = true, ""
	}
	e.mu.Unlock()
	for {
		r, err := s.Receive()
		if err != nil {
			return err
		}
		ev := r.GetEvent()
		if err := e.deliver(workflowID, stepID, ev); err != nil {
			return err
		}
		ack := &messagev1.SubscribeRequest{Payload: &messagev1.SubscribeRequest_Ack{Ack: &messagev1.Ack{EventId: ev.GetId()}}}
		sendMu.Lock()
		err = s.Send(ack)
		sendMu.Unlock()
		if err != nil {
			return err
		}
	}
}

// deliver starts the run an event of a trigger asks for, and says whether
// the event is done with. An event for a workflow, or a step, that is gone or
// switched off is skipped. Only a failure to record the run, or a tower that
// is stopping, keeps it for later.
func (e *Engine) deliver(workflowID uint32, stepID string, ev *messagev1.Event) error {
	wf, err := e.store.GetWorkflow(workflowID)
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if !wf.Active {
		return nil
	}
	run, err := e.begin(wf, stepID, fmt.Sprintf("topic %s, event %d", ev.GetTopic(), ev.GetId()), ev.GetData())
	switch {
	case errors.Is(err, ErrBadStart):
		return nil
	case err != nil && run.ID != 0: // recorded as failed
		return nil
	case err != nil:
		e.log.Error("record a workflow run", "workflow", wf.ID, "err", err)
		return err
	}
	data := ev.GetData()
	if err := e.background(func() { _, _ = e.finish(wf, run, stepID, data) }); err != nil {
		// Not run: mark it, and leave the event to come again.
		_, _ = e.store.FinishRun(run, v1.RunStatus_RUN_STATUS_INTERRUPTED, err.Error(), nil)
		return err
	}
	return nil
}

// Triggers says what an active workflow's trigger steps are doing.
func (e *Engine) Triggers(id uint32) []*v1.TriggerState {
	e.mu.Lock()
	defer e.mu.Unlock()
	var out []*v1.TriggerState
	for _, s := range slices.Concat(e.states[id], e.timerStates[id], e.telegramStates(id), e.whatsappStates(id), e.logStates(id), e.chatStates(id), e.watchStatesOf(id)) {
		out = append(out, proto.Clone(s).(*v1.TriggerState))
	}
	return out
}

// subscriberWorkflow is the workflow a trigger's subscriber belongs to.
func subscriberWorkflow(key string) (uint32, bool) {
	rest, ok := strings.CutPrefix(key, subscriberPrefix)
	if !ok {
		return 0, false
	}
	idText, _, ok := strings.Cut(rest, "-")
	if !ok {
		return 0, false
	}
	id, err := strconv.ParseUint(idText, 10, 32)
	return uint32(id), err == nil
}

// Run runs a workflow once from its trigger step stepID, as if the trigger
// got data, and records the run. trigger says what started it. A step that
// is not a trigger of the workflow is ErrBadStart, and no run is recorded.
// It waits for the run, or until ctx ends: the run goes on then, and Run
// returns it as it stands.
func (e *Engine) Run(ctx context.Context, workflowID uint32, stepID string, data []byte, trigger string) (store.Run, error) {
	w, err := e.store.GetWorkflow(workflowID)
	if err != nil {
		return store.Run{}, err
	}
	r, err := e.begin(w, stepID, trigger, data)
	if err != nil {
		return r, err
	}
	type result struct {
		run store.Run
		err error
	}
	done := make(chan result, 1)
	if err := e.background(func() {
		fr, err := e.finish(w, r, stepID, data)
		done <- result{fr, err}
	}); err != nil {
		return r, err
	}
	select {
	case res := <-done:
		return res.run, res.err
	case <-ctx.Done():
		return r, ctx.Err()
	}
}

// begin checks that stepID is a trigger step of w, and records that a run
// of it started with data. A run of a one-at-a-time workflow joins its queue
// in the message broker (queue.go) in the order begin is called; one that
// cannot is recorded as failed.
func (e *Engine) begin(w store.Workflow, stepID, trigger string, data []byte) (store.Run, error) {
	var st *v1.Step
	for _, s := range w.Graph.GetSteps() {
		if s.GetId() == stepID {
			st = s
		}
	}
	if st == nil || !IsTrigger(st.GetKind()) {
		return store.Run{}, fmt.Errorf("%w: step %s is not a trigger of workflow %d", ErrBadStart, stepID, w.ID)
	}
	r, err := e.store.StartRun(w.ID, trigger)
	if err != nil || !w.OneAtATime {
		return r, err
	}
	if err := e.enqueue(w, r, stepID, data); err != nil {
		err = fmt.Errorf("the run could not join the workflow's queue (one run at a time): %w", err)
		r, _ = e.store.FinishRun(r, v1.RunStatus_RUN_STATUS_FAILED, err.Error(), nil)
		return r, err
	}
	return r, nil
}

// finish runs a begun run's steps under the engine's context, and records
// how it ended; a queued run is run by its queue's reader, and finish waits
// for it to end.
func (e *Engine) finish(w store.Workflow, r store.Run, stepID string, data []byte) (store.Run, error) {
	if fr, ok, err := e.await(r); ok {
		return fr, err
	}
	return e.execute(w, r, stepID, data, nil)
}

// execute runs a run's steps and records how it ended. ack, for a queued
// run, acks its event (the Ack step). The lines its steps log, and its error
// when it fails, go to the other workflows' Log Event steps (logevent.go).
func (e *Engine) execute(w store.Workflow, r store.Run, stepID string, data []byte, ack func() error) (store.Run, error) {
	env := e.env()
	env.Logged = e.logLines(w, r, stepID)
	env.Live = func(_ *v1.Step, line string, level v1.LogLevel) { e.addLive(r.ID, line, level) }
	defer e.dropLive(r.ID) // once the run's record has its lines
	env.Workflow = w.ID
	env.Run = r.ID
	env.Ack = ack
	env.ChatSend = func(chat, text string, markdown bool) (chatMessage, error) {
		return e.chatSend(w.ID, r.ID, chat, text, markdown)
	}
	waited := time.Since(r.StartedAt)
	steps, runErr := Execute(e.ctx, w.Graph, stepID, data, env)
	noteWait(steps, waited)
	status, errText := v1.RunStatus_RUN_STATUS_SUCCEEDED, ""
	switch {
	case runErr != nil && e.ctx.Err() != nil:
		status, errText = v1.RunStatus_RUN_STATUS_INTERRUPTED, "the tower stopped during the run"
	case runErr != nil:
		status, errText = v1.RunStatus_RUN_STATUS_FAILED, runErr.Error()
	}
	fr, err := e.store.FinishRun(r, status, errText, steps)
	if err != nil {
		e.log.Error("record a workflow run", "workflow", w.ID, "run", r.ID, "err", err)
	}
	if status == v1.RunStatus_RUN_STATUS_FAILED && env.Logged != nil {
		e.logged(w, logLine{Message: errText, Type: "failed", RunID: r.ID})
	}
	return fr, err
}

// publish publishes data on topic in the broker in use, for the Publish to
// Topic step.
func (e *Engine) publish(ctx context.Context, topic string, data []byte) (uint64, error) {
	id, err := e.broker.Publish(ctx, topic, data)
	if errors.Is(err, san_message.ErrNotFound) {
		return 0, fmt.Errorf("topic %s does not exist", topic)
	}
	return id, err
}
