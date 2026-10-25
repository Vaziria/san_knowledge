package engine

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/store"
)

// MaxSteps is the most steps one run runs, against a loop of links.
const MaxSteps = 1000

// ErrBadStart is returned when a run cannot start from the step it was given.
var ErrBadStart = errors.New("cannot start there")

// Env is what steps reach outside their graph.
type Env struct {
	// Connection looks up a connection by id: a step's Connection setting.
	Connection func(id uint32) (store.Connection, error)
	// Publish publishes data on a topic of the tower's broker and returns
	// the event's id (Publish to Topic).
	Publish func(ctx context.Context, topic string, data []byte) (uint64, error)
	// TelegramAPI is where the Bot API answers, telegram.API when empty.
	TelegramAPI string
	// TelegramMessages lists the messages a Telegram bot connection got,
	// newest first: of every chat, or of one (Telegram Messages).
	TelegramMessages func(connectionID uint32, chat string, limit int) ([]store.TelegramMessage, error)
	// Storage looks up a storage by id, with its absolute path: a step's
	// Storage setting.
	Storage func(id uint32) (store.Storage, error)
	// DataDir is the tower's data folder, absolute, with /: where File
	// Path's relative names are when no storage is picked.
	DataDir string
}

// Execute runs a graph from its trigger step trigger, as if the trigger got
// data, and returns what each step did, in order. The trigger fires its
// event outputs; a step runs when an event link from a fired output reaches
// it, then fires its own. A data input takes the value of the output linked
// to it: from a step that already ran, from a pure step run then, or, for an
// unlinked field input, the value typed in the step. The first step that
// fails ends the run with its error, unless its Failed output is linked: then
// that fires, with the error as its Error output, and the run goes on. A
// step's In Progress output fires as it starts: what it links to runs first.
// Steps reach connections through env.
func Execute(ctx context.Context, g *v1.Graph, trigger string, data []byte, env Env) ([]*v1.StepRun, error) {
	x := newExecution(ctx, g, env)
	st := x.steps[trigger]
	if st == nil {
		return nil, fmt.Errorf("%w: no step %s", ErrBadStart, trigger)
	}
	k := byKind[st.GetKind()]
	if k == nil || k.trigger == nil {
		return nil, fmt.Errorf("%w: step %s is not a trigger", ErrBadStart, trigger)
	}
	start := time.Now()
	outputs, fired, err := k.trigger(st, data)
	x.record(st, &call{outputs: outputs, fired: fired}, start, err)
	if err != nil {
		return x.runs, fmt.Errorf("%s: %w", k.Title, err)
	}
	x.values[trigger] = outputs
	return x.runs, x.drain(x.next(trigger, fired))
}

// drain runs the steps in queue, and those their fired outputs link to, until
// none is left or one fails.
func (x *execution) drain(queue []string) error {
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		if x.budget--; x.budget < 0 {
			return fmt.Errorf("the run went past %d steps: do the links make a loop?", MaxSteps)
		}
		if err := x.ctx.Err(); err != nil {
			return err
		}
		fired, err := x.call(id)
		if err != nil {
			return err
		}
		queue = append(queue, x.next(id, fired)...)
	}
	return nil
}

type execution struct {
	ctx    context.Context
	env    Env
	steps  map[string]*v1.Step
	in     map[string]map[string]*v1.Link   // target step, input socket: the link into it
	out    map[string]map[string][]*v1.Link // source step, output socket: the links from it
	values map[string]map[string]proto.Message
	busy   map[string]bool // pure steps being run, against a loop of data links
	runs   []*v1.StepRun
	budget int
}

func newExecution(ctx context.Context, g *v1.Graph, env Env) *execution {
	x := &execution{
		ctx:    ctx,
		env:    env,
		steps:  make(map[string]*v1.Step),
		in:     make(map[string]map[string]*v1.Link),
		out:    make(map[string]map[string][]*v1.Link),
		values: make(map[string]map[string]proto.Message),
		busy:   make(map[string]bool),
		budget: MaxSteps,
	}
	for _, st := range g.GetSteps() {
		x.steps[st.GetId()] = st
	}
	for _, l := range g.GetLinks() {
		if x.in[l.GetTarget()] == nil {
			x.in[l.GetTarget()] = make(map[string]*v1.Link)
		}
		x.in[l.GetTarget()][l.GetTargetSocket()] = l
		if x.out[l.GetSource()] == nil {
			x.out[l.GetSource()] = make(map[string][]*v1.Link)
		}
		x.out[l.GetSource()][l.GetSourceSocket()] = append(x.out[l.GetSource()][l.GetSourceSocket()], l)
	}
	return x
}

// next lists the steps the fired outputs of step id link to.
func (x *execution) next(id string, fired []string) []string {
	var ids []string
	for _, socket := range fired {
		for _, l := range x.out[id][socket] {
			if x.steps[l.GetTarget()] != nil {
				ids = append(ids, l.GetTarget())
			}
		}
	}
	return ids
}

// call runs a step and returns the event outputs it fired. A linked In
// Progress output fires first: the steps it links to run, to the end of
// their branch, before the step does. A step that fails with its Failed
// output linked fires Failed, with Error the error, and is recorded with its
// error; one without ends the run.
func (x *execution) call(id string) ([]string, error) {
	st := x.steps[id]
	k := byKind[st.GetKind()]
	if k == nil {
		return nil, fmt.Errorf("step %s: unknown kind %q", id, st.GetKind())
	}
	progress := k.output(InProgress, Event) >= 0 && len(x.out[id][InProgress]) > 0
	if progress {
		if err := x.drain(x.next(id, []string{InProgress})); err != nil {
			return nil, err
		}
	}
	c := &call{ctx: x.ctx, env: x.env, step: st, inputs: make(map[string]proto.Message), outputs: make(map[string]proto.Message)}
	// A loop's outputs, and the error of a step in its branch, which ends
	// the run as it would anywhere else: it is not this step's.
	var looped []string
	var branchErr error
	c.each = func(output string) error {
		if !slices.Contains(looped, output) {
			looped = append(looped, output)
		}
		x.values[id] = c.outputs
		x.forget(id)
		if err := x.drain(x.next(id, []string{output})); err != nil {
			branchErr = err
			return err
		}
		return nil
	}
	start := time.Now()
	err := x.run(k, st, c)
	var r *v1.StepRun
	switch {
	case branchErr != nil:
		r = x.record(st, c, start, nil)
		r.Fired = append(slices.Clone(looped), r.Fired...)
		return nil, branchErr
	case err == nil:
		r = x.record(st, c, start, nil)
	case k.output(Failed, Event) >= 0 && len(x.out[id][Failed]) > 0:
		c.fired, c.outputs = []string{Failed}, map[string]proto.Message{ErrorOutput: stringValue(err.Error())}
		r = x.record(st, c, start, err)
		r.Fired = c.fired
	default:
		x.record(st, c, start, err)
		return nil, fmt.Errorf("%s: %w", k.Title, err)
	}
	// Shown as fired; their steps already ran.
	r.Fired = append(slices.Clone(looped), r.Fired...)
	if progress {
		r.Fired = append([]string{InProgress}, r.Fired...)
	}
	x.values[id] = c.outputs
	return c.fired, nil
}

// forget drops the values of the pure steps that read, through data links,
// from step id, and of those that read from them, so that each item of a
// loop works them out again from its own outputs.
func (x *execution) forget(id string) {
	for target, ins := range x.in {
		for _, l := range ins {
			if l.GetSource() != id {
				continue
			}
			k := byKind[x.steps[target].GetKind()]
			if _, ran := x.values[target]; ran && k != nil && k.pure() {
				delete(x.values, target)
				x.forget(target)
			}
		}
	}
}

// run gets a step's data inputs into c and runs it.
func (x *execution) run(k *kind, st *v1.Step, c *call) error {
	if k.run == nil {
		return errors.New("this kind of step cannot run yet")
	}
	for _, s := range k.Inputs {
		if s.Type == Event {
			continue
		}
		v, err := x.input(st, s)
		if err != nil {
			return err
		}
		if v != nil {
			c.inputs[s.Name] = v
		}
	}
	return k.run(c)
}

// input is the value of one data input of st, nil when nothing comes.
func (x *execution) input(st *v1.Step, s *v1.Socket) (proto.Message, error) {
	l := x.in[st.GetId()][s.Name]
	if l == nil {
		if s.Field {
			return stringValue(st.GetValues()[s.Name]), nil
		}
		return nil, nil
	}
	src := x.steps[l.GetSource()]
	if src == nil {
		return nil, nil
	}
	if _, ran := x.values[src.GetId()]; !ran {
		k := byKind[src.GetKind()]
		if k == nil || !k.pure() {
			name := src.GetKind()
			if k != nil {
				name = k.Title
			}
			return nil, fmt.Errorf("input %s comes from %s, which has not run before this step", s.Name, name)
		}
		if x.busy[src.GetId()] {
			return nil, fmt.Errorf("input %s: the data links make a loop", s.Name)
		}
		x.busy[src.GetId()] = true
		_, err := x.call(src.GetId())
		delete(x.busy, src.GetId())
		if err != nil {
			return nil, err
		}
	}
	return x.values[src.GetId()][l.GetSourceSocket()], nil
}

// record adds what a step did to the run, a failed one without the outputs
// it fired, and returns it.
func (x *execution) record(st *v1.Step, c *call, start time.Time, err error) *v1.StepRun {
	r := &v1.StepRun{
		StepId:    st.GetId(),
		Kind:      st.GetKind(),
		Fired:     c.fired,
		Logs:      c.logs,
		LogLevels: c.levels,
		Duration:  durationpb.New(time.Since(start)),
	}
	if err != nil {
		r.Error = err.Error()
		r.Fired = nil
	}
	for name, v := range c.outputs {
		a, aerr := anypb.New(v)
		if aerr != nil {
			continue
		}
		if r.Outputs == nil {
			r.Outputs = make(map[string]*anypb.Any)
		}
		r.Outputs[name] = a
	}
	x.runs = append(x.runs, r)
	return r
}
