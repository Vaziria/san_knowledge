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
	"github.com/wargasipil/san_tower/internal/moviemov"
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
	// The Telegram Command steps of active workflows, by workflow id, and
	// the chats to tell their forms were cut off, by bot connection id,
	// until it answers (telegramform.go).
	cmdSteps map[uint32][]cmdStep
	notices  map[uint32][]int64
	// The Telegram Forms' open conversations, one per person, guarded by
	// convMu; convSeq numbers them, for their buttons.
	convMu  sync.Mutex
	convs   map[convKey]*tgConv
	convSeq int64
	// The Log Event steps of active workflows, by workflow id.
	logSteps map[uint32][]logStep
	// The Tower Chat steps of active workflows, by workflow id (towerchat.go).
	chatSteps map[uint32][]string
	// The Folder Watch and File Watch steps watching, by the same key as the
	// timers, and the watch steps' states (watch.go).
	watchers    map[string]*watcher
	watchStates map[uint32][]*v1.TriggerState
	// The OpenAI API steps' servers, by the same key as the timers, and
	// their states; the requests waiting for their run's OpenAI Reply, by
	// request id, guarded by oaMu (openai.go).
	oaServers map[string]*openAIServer
	oaStates  map[uint32][]*v1.TriggerState
	oaMu      sync.Mutex
	oaWaiting map[string]*openAIWait
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
	// Also guarded by qmu (topicack.go): the turns that keep a one-at-a-time
	// workflow's runs, and an After Run trigger's, apart, by key; and the
	// events each After Run trigger's runs ended for, by subscriber key.
	turns    map[string]chan struct{}
	afterRan map[string][]eventStamp

	// ssh are the shells SSH Session steps keep open from run to run,
	// closed when the engine closes.
	ssh *SSHShells
	// movieMov are the MovieMov connections' tokens, kept in memory from
	// run to run (moviemov.go).
	movieMov *moviemov.Sessions
	// exness are the Exness connections' clients, kept from run to run
	// (exness.go).
	exness *ExnessClients

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
	// The trigger's Ack (After Run) and Ack Time (topicack.go).
	afterRun bool
	ackTime  time.Duration
	// done is closed when its goroutine has ended, its stream with it.
	done chan struct{}
}

func New(st *store.Store, b *broker.Broker, log *slog.Logger) *Engine {
	ctx, cancel := context.WithCancel(context.Background())
	return &Engine{
		store: st, broker: b, log: log, ctx: ctx, cancel: cancel,
		states: make(map[uint32][]*v1.TriggerState), listeners: make(map[string]*listener),
		timers: make(map[string]*timer), timerStates: make(map[uint32][]*v1.TriggerState),
		pollers: make(map[uint32]*poller), tgSteps: make(map[uint32][]tgStep),
		cmdSteps: make(map[uint32][]cmdStep), notices: make(map[uint32][]int64), convs: make(map[convKey]*tgConv),
		logSteps: make(map[uint32][]logStep), chatSteps: make(map[uint32][]string),
		watchers: make(map[string]*watcher), watchStates: make(map[uint32][]*v1.TriggerState),
		oaServers: make(map[string]*openAIServer), oaStates: make(map[uint32][]*v1.TriggerState), oaWaiting: make(map[string]*openAIWait),
		waiting: make(map[uint32]*waiter), readers: make(map[uint32]context.CancelFunc), queues: make(map[uint32]string),
		turns: make(map[string]chan struct{}), afterRan: make(map[string][]eventStamp),
		ssh: NewSSHShells(), movieMov: moviemov.NewSessions(), exness: NewExnessClients(nil),
	}
}

// MovieMov are the MovieMov connections' kept tokens, which TestConnection
// shares with the steps.
func (e *Engine) MovieMov() *moviemov.Sessions { return e.movieMov }

// Exness are the Exness connections' clients, which TestConnection shares
// with the steps.
func (e *Engine) Exness() *ExnessClients { return e.exness }

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
		MovieMov:         e.movieMov,
		Exness:           e.exness,
		SSH:              e.ssh,
		KeepHostKey:      e.store.KeepSSHHostKey,
		TelegramMessages: e.store.TelegramMessages,
		TelegramTalk:     e.talk,
		TelegramCommands: e.botCommands,
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
	e.takeNotices()
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
		e.syncOpenAI(w)
	}
	e.dropTimers(known)
	e.dropWatches(known)
	e.dropOpenAI(known)
	for id := range e.tgSteps {
		if !known[id] {
			delete(e.tgSteps, id)
		}
	}
	for id := range e.cmdSteps {
		if !known[id] {
			delete(e.cmdSteps, id)
		}
	}
	e.syncCommands()
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
	e.syncCommands()
	e.syncLogs(w)
	e.syncChats(w)
	e.syncWatches(w)
	e.syncOpenAI(w)
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

// trigger is a Topic Event step of an active workflow, as its subscriber,
// with its Ack and Ack Time (topicack.go).
type trigger struct {
	key, topic string
	state      *v1.TriggerState
	afterRun   bool
	ackTime    time.Duration // 0: the broker's own
}

// triggers are workflow w's Topic Event steps while it is active, each with
// its state; those without a topic, or with an Ack or Ack Time it cannot
// take, say so and are left out.
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
		afterRun, ackTime, err := topicAck(st.GetValues())
		if err != nil {
			ts.Error = err.Error()
			continue
		}
		key := fmt.Sprintf("%s%d-%s", subscriberPrefix, w.ID, st.GetId())
		t := trigger{key: key, topic: topic, state: ts, afterRun: afterRun, ackTime: ackTime}
		ts.Source = t.source()
		wanted[key] = t
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
		// A changed Ack or Ack Time opens the stream again, with its Hello.
		if l := e.listeners[key]; l != nil && l.url == url && l.topic == t.topic && l.afterRun == t.afterRun && l.ackTime == t.ackTime {
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
// closes. The stream it replaces ends first: an After Run run going on
// there ends, and its event is acked, before this one opens (stream).
// Called with e.mu held.
func (e *Engine) listen(url string, workflowID uint32, t trigger) {
	prev := e.listeners[t.key]
	if prev != nil {
		prev.cancel()
	}
	ctx, cancel := context.WithCancel(e.ctx)
	l := &listener{cancel: cancel, url: url, topic: t.topic, state: t.state, afterRun: t.afterRun, ackTime: t.ackTime, done: make(chan struct{})}
	e.listeners[t.key] = l
	stepID := strings.TrimPrefix(t.key, fmt.Sprintf("%s%d-", subscriberPrefix, workflowID))
	err := e.background(func() {
		defer close(l.done)
		if prev != nil {
			select {
			case <-prev.done:
			default:
				e.mu.Lock()
				if ctx.Err() == nil {
					l.state.Listening, l.state.Error = false, "waiting for the run going on to end: it started before the trigger changed"
				}
				e.mu.Unlock()
				select {
				case <-prev.done:
				case <-ctx.Done():
					return
				}
			}
		}
		wait := minRetry
		for {
			began := time.Now()
			err := e.stream(ctx, l, t, workflowID, stepID)
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
		close(l.done)
		delete(e.listeners, t.key)
	}
}

// stream reads trigger t's events from one Subscribe stream until it ends.
// With Ack On Start, each event is acked once its run is recorded as
// started; an event that cannot be recorded is not acked: the stream ends,
// and the broker sends the event again on the next. With After Run, each
// event's run goes on here, and the event is acked when it ends
// (topicack.go). The stream lives on the engine's context, not ctx: a
// trigger synced away or changed during an After Run run keeps its stream
// until the run's event is acked, so the broker cannot send the event to
// the next stream while it runs (it refuses a second stream of the
// subscriber until then).
func (e *Engine) stream(ctx context.Context, l *listener, t trigger, workflowID uint32, stepID string) error {
	sctx, cancel := context.WithCancel(e.ctx)
	defer cancel()
	s := broker.NewClient(l.url).Subscribe(sctx)
	defer s.CloseResponse()
	defer s.CloseRequest()
	// A Receive waiting for the next event does not see ctx end once an
	// event came (queue.go): end the request then, unless an After Run run
	// goes on, which ends the stream once its event is acked. The acks are
	// sent from this goroutine and an Ack step's, so sendMu keeps them apart.
	var sendMu sync.Mutex
	running := false // an After Run run goes on; guarded by sendMu
	send := func(m *messagev1.SubscribeRequest) error {
		sendMu.Lock()
		defer sendMu.Unlock()
		return s.Send(m)
	}
	stop := context.AfterFunc(ctx, func() {
		sendMu.Lock()
		defer sendMu.Unlock()
		if !running {
			_ = s.CloseRequest()
			cancel()
		}
	})
	defer stop()
	if err := send(t.hello()); err != nil {
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
		ackEvent := &messagev1.SubscribeRequest{Payload: &messagev1.SubscribeRequest_Ack{Ack: &messagev1.Ack{EventId: ev.GetId()}}}
		if !t.afterRun {
			if err := e.deliver(workflowID, stepID, ev); err != nil {
				return err
			}
			if err := send(ackEvent); err != nil {
				return err
			}
			continue
		}
		sendMu.Lock()
		if ctx.Err() != nil { // synced away as it came: the next stream gets it
			sendMu.Unlock()
			return ctx.Err()
		}
		running = true
		sendMu.Unlock()
		acked := false // guarded by sendMu
		ack := func() error {
			sendMu.Lock()
			defer sendMu.Unlock()
			if acked {
				return nil
			}
			err := s.Send(ackEvent)
			acked = err == nil
			return err
		}
		done, err := e.runAfter(ctx, t.key, workflowID, stepID, ev, ack)
		if err == nil && done {
			err = ack()
		}
		sendMu.Lock()
		running = false
		sendMu.Unlock()
		switch {
		case err != nil:
			return err
		case !done:
			return errors.New("the event is left to come again: the tower is stopping, or the trigger changed")
		case ctx.Err() != nil:
			return ctx.Err()
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
	for _, s := range slices.Concat(e.states[id], e.timerStates[id], e.telegramStates(id), e.whatsappStates(id), e.logStates(id), e.chatStates(id), e.watchStatesOf(id), e.openAIStatesOf(id)) {
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
	r, err := e.record(w, stepID, trigger)
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

// record checks that stepID is a trigger step of w, and records that a run
// of it started: begin, and an After Run trigger's run (topicack.go), which
// runs on its trigger's stream and never joins the workflow's queue.
func (e *Engine) record(w store.Workflow, stepID, trigger string) (store.Run, error) {
	var st *v1.Step
	for _, s := range w.Graph.GetSteps() {
		if s.GetId() == stepID {
			st = s
		}
	}
	if st == nil || !IsTrigger(st.GetKind()) {
		return store.Run{}, fmt.Errorf("%w: step %s is not a trigger of workflow %d", ErrBadStart, stepID, w.ID)
	}
	return e.store.StartRun(w.ID, trigger)
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
	env.OpenAIReply = e.openAIReplier(data)
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
