package keywordresearch

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"path/filepath"
	"slices"
	"strconv"
	"sync"
	"time"

	"connectrpc.com/connect"

	"github.com/wargasipil/san_tower/internal/store"
)

// Filters are Trends' filters of a lookup.
type Filters struct {
	Geo      string
	Time     string
	Category int32
	Property string
}

// Trends looks up a keyword's related queries on Google Trends through a
// San Remote connection (0: the first one). Its errors carry San Remote's
// Connect codes: ResourceExhausted is Google limiting the browser,
// InvalidArgument a keyword Trends cannot take.
type Trends interface {
	Related(ctx context.Context, connection uint32, keyword string, f Filters) (top, rising []Found, err error)
}

// ErrSettingsChanged is Run on a graph made with other filters (the user,
// 2026-11-11): the page asks before it clears it, since one graph would mix
// the numbers of two time ranges or places. A changed seed is not asked
// about (the user, 2026-11-12: "when i change seed keyword, dont ask reset,
// just add it, no reset"): Run adds it to the graph.
var ErrSettingsChanged = errors.New("The graph was made with other filters: clear it and start over?")

// ErrBusy is a change refused while the research's crawl runs or waits.
var ErrBusy = errors.New("the research is running or waiting: stop it first")

// Progress is what a queued research is doing (KeywordProgress).
type Progress struct {
	Keyword  string
	Depth    int
	Lookups  int
	Todo     int
	Note     string
	Position int // 1 is the one running
}

// Crawler runs the researches' crawls, one at a time in the whole tower
// (the user, 2026-11-11: they share his Chrome); the others wait their
// turn in a queue. A crawl goes on when the page closes.
type Crawler struct {
	Store  *store.Store
	Trends Trends
	// Root is the folder of the researches' graphs, <data>/keyword_research.
	Root string
	Log  *slog.Logger
	// Pause, plus up to Jitter more, goes between two lookups (the user,
	// 2026-11-11: "Never hammer it").
	Pause, Jitter time.Duration
	// Backoff is how long to wait before each try again when Google limits
	// the browser; past the last, the crawl fails.
	Backoff []time.Duration
	// Watch hears of each change of a research the crawler makes, right
	// after it is made: its pages follow it (watch.go). Nil: no one.
	Watch *Watch

	mu      sync.Mutex
	queue   []*job // queue[0] runs
	wake    chan struct{}
	ctx     context.Context
	cancel  context.CancelFunc
	done    chan struct{}
	started bool
}

// job is a Run (keyword "") or an Explore from here of one keyword.
type job struct {
	id      uint32
	keyword string
	// stop ends it after its lookup; stopWait ends its pauses at once.
	stop     bool
	stopWait context.CancelFunc
	waitCtx  context.Context
	// deleted: the research is gone; its graph goes when the job ends.
	deleted  bool
	progress Progress
	// prior is the research's status before an Explore, which it goes
	// back to.
	prior, priorMessage string
	// gone: an Explore's keyword was taken out of the graph (blacklisted)
	// before its lookup was written.
	gone bool
}

// The pace's defaults: 3 to 7 seconds between lookups, on top of the
// lookup's own few seconds.
var (
	DefaultPause  = 3 * time.Second
	DefaultJitter = 4 * time.Second
)

// DefaultBackoff waits 1, 3, then 10 minutes before trying again.
var DefaultBackoff = []time.Duration{time.Minute, 3 * time.Minute, 10 * time.Minute}

// Dir is research id's graph folder.
func (c *Crawler) Dir(id uint32) string {
	return filepath.Join(c.Root, strconv.FormatUint(uint64(id), 10))
}

// Start marks the researches a stopped tower left running as stopped, and
// starts the crawls' worker; Close ends it.
func (c *Crawler) Start() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.started {
		return
	}
	if c.Log == nil {
		c.Log = slog.New(slog.DiscardHandler)
	}
	if err := c.Store.StopCutResearches(); err != nil {
		c.Log.Warn("keyword research: mark the cut crawls stopped", "err", err)
	}
	c.started = true
	c.wake = make(chan struct{}, 1)
	c.done = make(chan struct{})
	c.ctx, c.cancel = context.WithCancel(context.Background())
	go c.work()
}

// Close stops the crawl that runs, records it as stopped, and ends the
// worker, waiting up to 10 seconds.
func (c *Crawler) Close() {
	c.mu.Lock()
	if !c.started {
		c.mu.Unlock()
		return
	}
	c.started = false
	c.cancel()
	c.mu.Unlock()
	select {
	case <-c.done:
	case <-time.After(10 * time.Second):
	}
}

// Progress is research id's progress while it runs or waits.
func (c *Crawler) Progress(id uint32) (Progress, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i, j := range c.queue {
		if j.id == id && !j.deleted {
			p := j.progress
			p.Position = i + 1
			return p, true
		}
	}
	return Progress{}, false
}

// Busy says whether research id runs or waits.
func (c *Crawler) Busy(id uint32) bool {
	_, ok := c.Progress(id)
	return ok
}

// Run queues research id's crawl: from the seed when the graph is empty,
// else on from where it was, with the research's seed keyword added to the
// graph as a seed when it is a new one (Graph.Seed): looked up first, then
// its Top keywords to the deep limit, the keywords looked up before not
// again. A graph made with other filters gives ErrSettingsChanged, unless
// clear, which empties it first. A research already queued stays as it
// is.
func (c *Crawler) Run(id uint32, clear bool) error {
	k, err := c.Store.GetResearch(id)
	if err != nil {
		return err
	}
	if store.NormalizeKeyword(k.Seed) == "" {
		return fmt.Errorf("%w: give the research a seed keyword first", store.ErrInvalid)
	}
	if c.Busy(id) {
		return nil
	}
	if k.GraphKey != "" && k.GraphKey != k.SettingsKey() {
		// A graph with nothing looked up yet (a first Run that failed) is
		// only its seed: it goes without asking.
		if !clear && SettingsChanged(k) {
			return ErrSettingsChanged
		}
		if err := c.Clear(id); err != nil {
			return err
		}
	}
	return c.enqueue(&job{id: id}, k)
}

// SettingsChanged says whether k's graph, with keywords looked up, was made
// with other filters than k has now (its seeds do not count).
func SettingsChanged(k store.KeywordResearch) bool {
	return k.GraphKey != "" && k.ExploredCount > 0 && k.GraphKey != k.SettingsKey()
}

// Explore queues one lookup of keyword, a keyword of research id's graph not
// looked up yet: past the deep limit, or a Rising one, too.
func (c *Crawler) Explore(id uint32, keyword string) error {
	k, err := c.Store.GetResearch(id)
	if err != nil {
		return err
	}
	if SettingsChanged(k) {
		return fmt.Errorf("%w: the graph was made with other settings: Run asks to clear it", store.ErrInvalid)
	}
	keyword = store.NormalizeKeyword(keyword)
	var n Node
	var ok bool
	if Exists(c.Dir(id)) {
		err = With(c.Dir(id), func(g *Graph) error {
			n, ok, err = g.Find(keyword)
			return err
		})
		if err != nil {
			return err
		}
	}
	switch {
	case !ok:
		return fmt.Errorf("%w: %q is not in the graph", store.ErrNotFound, keyword)
	case n.Explored:
		return fmt.Errorf("%w: %q was looked up already", store.ErrInvalid, keyword)
	}
	c.mu.Lock()
	for _, j := range c.queue {
		if j.id == id && j.keyword == keyword && !j.stop {
			c.mu.Unlock()
			return nil
		}
	}
	c.mu.Unlock()
	return c.enqueue(&job{id: id, keyword: keyword}, k)
}

func (c *Crawler) enqueue(j *job, k store.KeywordResearch) error {
	c.mu.Lock()
	if !c.started {
		c.mu.Unlock()
		return errors.New("the crawler is not running")
	}
	j.waitCtx, j.stopWait = context.WithCancel(c.ctx)
	waiting, busySelf := false, false
	for _, q := range c.queue {
		if q.id != j.id {
			waiting = true
		} else {
			busySelf = true
		}
	}
	// An Explore goes back to the status the research had: now, or, behind
	// its own Run, the one that Run ends with (read when it starts).
	if j.keyword != "" && !busySelf {
		j.prior, j.priorMessage = k.Status, k.Message
	}
	c.queue = append(c.queue, j)
	c.mu.Unlock()
	if !busySelf {
		status := store.ResearchRunning
		if waiting {
			status = store.ResearchWaiting
		}
		if err := c.Store.SetResearchState(j.id, map[string]any{"status": status, "message": ""}); err != nil {
			return err
		}
		c.changed(j.id)
	}
	select {
	case c.wake <- struct{}{}:
	default:
	}
	return nil
}

// Stop ends research id's crawl after the lookup it is doing (the user,
// 2026-11-11), or takes it out of the queue. What was found stays.
func (c *Crawler) Stop(id uint32) error {
	c.mu.Lock()
	var rest []*job
	stopped := false
	for i, j := range c.queue {
		if j.id != id {
			rest = append(rest, j)
			continue
		}
		stopped = true
		if i == 0 {
			j.stop = true
			j.stopWait()
			rest = append(rest, j)
		}
	}
	c.queue = rest
	running := len(rest) > 0 && rest[0].id == id
	c.mu.Unlock()
	if stopped && !running {
		if err := c.Store.SetResearchState(id, map[string]any{"status": store.ResearchStopped, "message": "Stopped before its turn came."}); err != nil {
			return err
		}
	}
	// The ones behind it moved up the queue.
	c.changed(append(c.queued(), id)...)
	return nil
}

// queued are the researches in the queue, the running one first.
func (c *Crawler) queued() []uint32 {
	c.mu.Lock()
	defer c.mu.Unlock()
	var ids []uint32
	for _, j := range c.queue {
		if !slices.Contains(ids, j.id) {
			ids = append(ids, j.id)
		}
	}
	return ids
}

// changed tells the pages that researches' status, progress or counts
// changed.
func (c *Crawler) changed(ids ...uint32) {
	for _, id := range ids {
		c.Watch.Publish(Change{ID: id, Kind: Changed})
	}
}

// Forget stops research id for good, its research deleted: its graph is
// deleted now, or by its crawl once the lookup it is doing ends.
func (c *Crawler) Forget(id uint32) error {
	c.mu.Lock()
	var rest []*job
	later := false
	for i, j := range c.queue {
		if j.id != id {
			rest = append(rest, j)
			continue
		}
		if i == 0 {
			j.stop, j.deleted, later = true, true, true
			j.stopWait()
			rest = append(rest, j)
		}
	}
	c.queue = rest
	c.mu.Unlock()
	if later {
		return nil
	}
	return Remove(c.Dir(id))
}

// Clear empties research id's graph; refused while it runs or waits.
func (c *Crawler) Clear(id uint32) error {
	if c.Busy(id) {
		return ErrBusy
	}
	if err := Remove(c.Dir(id)); err != nil {
		return err
	}
	err := c.Store.SetResearchState(id, map[string]any{"node_count": 0, "edge_count": 0, "explored_count": 0, "graph_key": "", "status": store.ResearchIdle, "message": ""})
	c.Watch.Publish(Change{ID: id, Kind: Reset})
	c.changed(id)
	return err
}

func (c *Crawler) work() {
	defer close(c.done)
	for {
		c.mu.Lock()
		var j *job
		if len(c.queue) > 0 {
			j = c.queue[0]
		}
		c.mu.Unlock()
		if j == nil {
			select {
			case <-c.wake:
				continue
			case <-c.ctx.Done():
				return
			}
		}
		c.run(j)
		c.mu.Lock()
		if len(c.queue) > 0 && c.queue[0] == j {
			c.queue = c.queue[1:]
		}
		var next *job
		if len(c.queue) > 0 {
			next = c.queue[0]
		}
		c.mu.Unlock()
		if j.deleted {
			if err := Remove(c.Dir(j.id)); err != nil {
				c.Log.Warn("keyword research: delete its graph", "research", j.id, "err", err)
			}
		}
		if c.ctx.Err() != nil {
			return
		}
		if next != nil && next.id != j.id {
			if err := c.Store.SetResearchState(next.id, map[string]any{"status": store.ResearchRunning}); err != nil {
				c.Log.Warn("keyword research: set running", "research", next.id, "err", err)
			}
		}
		// Its progress is gone, and the ones waiting moved up.
		if !j.deleted {
			c.changed(j.id)
		}
		c.changed(c.queued()...)
	}
}

func (c *Crawler) setProgress(j *job, fn func(p *Progress)) {
	c.mu.Lock()
	fn(&j.progress)
	c.mu.Unlock()
	c.changed(j.id)
}

// errStopped ends a crawl that was stopped during a pause.
var errStopped = errors.New("stopped")

// run runs a job and records how it ended.
func (c *Crawler) run(j *job) {
	if j.keyword != "" && j.prior == "" {
		if k, err := c.Store.GetResearch(j.id); err == nil {
			j.prior, j.priorMessage = k.Status, k.Message
		}
	}
	now := time.Now()
	if err := c.Store.SetResearchState(j.id, map[string]any{"status": store.ResearchRunning, "message": "", "last_run_at": &now}); err != nil {
		c.Log.Warn("keyword research: set running", "research", j.id, "err", err)
	}
	c.changed(j.id)
	var status, msg string
	lookups, err := c.crawl(j)
	switch {
	case j.deleted:
		return
	case c.ctx.Err() != nil:
		status, msg = store.ResearchStopped, "The tower stopped during the crawl. Run goes on from where it was."
	case errors.Is(err, errStopped) || (err != nil && j.stop):
		status, msg = store.ResearchStopped, fmt.Sprintf("Stopped after %d lookups. Run goes on from there.", lookups)
	case err != nil:
		status, msg = store.ResearchFailed, err.Error()
	case j.keyword != "":
		status, msg = j.prior, j.priorMessage
		if status == "" || status == store.ResearchIdle || status == store.ResearchWaiting || status == store.ResearchRunning {
			status = store.ResearchStopped
		}
		msg = fmt.Sprintf("Looked up %q.", j.keyword)
		if j.gone {
			msg = fmt.Sprintf("%q was blacklisted: its lookup was dropped.", j.keyword)
		}
	case j.stop:
		status, msg = store.ResearchStopped, fmt.Sprintf("Stopped after %d lookups. Run goes on from there.", lookups)
	default:
		status, msg = store.ResearchDone, fmt.Sprintf("Done: %d lookups this run.", lookups)
	}
	c.Log.Info("keyword research: crawl ended", "research", j.id, "status", status, "lookups", lookups, "message", msg)
	if err := c.Store.SetResearchState(j.id, map[string]any{"status": status, "message": msg}); err != nil {
		c.Log.Warn("keyword research: set status", "research", j.id, "err", err)
	}
	c.changed(j.id)
}

// crawl does a job's lookups and gives how many it made: for a Run, the
// explorable keywords, breadth first, until none is left under the deep
// limit; for an Explore, its keyword once.
func (c *Crawler) crawl(j *job) (lookups int, err error) {
	k, err := c.Store.GetResearch(j.id)
	if err != nil {
		return 0, err
	}
	dir := c.Dir(j.id)
	f := Filters{Geo: k.Geo, Time: k.Time, Category: k.Category, Property: k.Property}
	key := k.SettingsKey()
	if j.keyword == "" {
		var n Counts
		var d *Delta
		err := With(dir, func(g *Graph) error {
			changed, err := g.Seed(k.Seed, time.Now())
			if err != nil {
				return err
			}
			n, d, err = c.delta(g, j.id, "", append(changed, k.Seed))
			return err
		})
		if err != nil {
			return 0, err
		}
		if err := c.Store.SetResearchState(j.id, map[string]any{"graph_key": key}); err != nil {
			return 0, err
		}
		if err := c.wrote(j.id, n, d); err != nil {
			return 0, err
		}
	}
	for {
		if j.stop {
			return lookups, errStopped
		}
		// The deep limit is read again each time: raising it while it
		// runs goes deeper.
		if cur, err := c.Store.GetResearch(j.id); err == nil {
			k.DeepLimit = cur.DeepLimit
		}
		var next Node
		var todo int
		var ok bool
		err := With(dir, func(g *Graph) error {
			nodes, err := g.Nodes()
			if err != nil {
				return err
			}
			if j.keyword != "" {
				if lookups > 0 {
					return nil
				}
				for _, n := range nodes {
					if n.Name == j.keyword && !n.Explored {
						next, todo, ok = n, 1, true
					}
				}
				j.gone = !ok && !slices.ContainsFunc(nodes, func(n Node) bool { return n.Name == j.keyword })
				return nil
			}
			next, todo, ok = Next(nodes, int(k.DeepLimit))
			return nil
		})
		if err != nil {
			return lookups, err
		}
		if !ok {
			return lookups, nil
		}
		if lookups > 0 {
			pause := c.Pause
			if c.Jitter > 0 {
				pause += rand.N(c.Jitter)
			}
			c.setProgress(j, func(p *Progress) { p.Todo, p.Note = todo, "pausing "+pause.Round(time.Second).String() })
			if !sleep(j.waitCtx, pause) {
				return lookups, errStopped
			}
		}
		c.setProgress(j, func(p *Progress) { p.Keyword, p.Depth, p.Todo, p.Note = next.Name, next.Depth, todo, "" })
		top, rising, err := c.lookup(j, k.ConnectionID, next.Name, f)
		skipped := false
		var invalid *skipError
		switch {
		case errors.As(err, &invalid):
			// A keyword Trends cannot take (too long for its box) is
			// passed over, not the end of the crawl.
			c.Log.Warn("keyword research: a keyword passed over", "research", j.id, "keyword", next.Name, "err", invalid.err)
			skipped = true
		case err != nil:
			return lookups, err
		}
		lookups++
		at := time.Now()
		var n Counts
		var d *Delta
		gone := false
		err = With(dir, func(g *Graph) error {
			// A blacklist entry added during the lookup may have taken the
			// keyword out (the user, 2026-11-12: "remove the node and
			// child"): what it found is not written then.
			if _, ok, err := g.Find(next.Name); err != nil || !ok {
				gone = !ok
				return err
			}
			if !skipped {
				// The blacklist is read here, in the graph's turn: an entry
				// added while this lookup ran counts already, and one being
				// added waits for this write and then takes out what it
				// matches, so no blacklisted keyword stays.
				entries, err := c.blacklist(j.id)
				if err != nil {
					return err
				}
				top, rising = allowed(entries, top), allowed(entries, rising)
				if _, err := g.Add(next.Name, top, rising, at); err != nil {
					return err
				}
			}
			if err := g.MarkExplored(next.Name, at); err != nil {
				return err
			}
			// A keyword found nearer a seed than before (a new seed's
			// lookup) brings those found through it nearer too.
			moved, err := g.Relevel()
			if err != nil {
				return err
			}
			n, d, err = c.delta(g, j.id, next.Name, append(queries(top, rising), moved...))
			return err
		})
		if err != nil {
			return lookups, err
		}
		c.mu.Lock()
		j.progress.Lookups = lookups
		c.mu.Unlock()
		if gone {
			c.Log.Info("keyword research: a keyword blacklisted during its lookup", "research", j.id, "keyword", next.Name)
			j.gone = j.keyword != ""
			c.changed(j.id)
			continue
		}
		// The pages get what it wrote right away, then the counts.
		if err := c.wrote(j.id, n, d); err != nil {
			return lookups, err
		}
		if k2, err := c.Store.GetResearch(j.id); err == nil && k2.SettingsKey() != key {
			return lookups, fmt.Errorf("the filters changed during the crawl: Run again, which asks to clear the graph")
		}
	}
}

// blacklist is research id's blacklist entries, as the crawl reads them
// before each write.
func (c *Crawler) blacklist(id uint32) ([]string, error) {
	list, err := c.Store.ListBlacklist(id)
	if err != nil {
		return nil, err
	}
	out := make([]string, len(list))
	for i, e := range list {
		out[i] = e.Entry
	}
	return out, nil
}

// allowed are the queries of a lookup's list that no blacklist entry
// matches: the crawl never adds the others, so they are never looked up.
func allowed(entries []string, list []Found) []Found {
	if len(entries) == 0 {
		return list
	}
	var out []Found
	for _, f := range list {
		if matchesAny(entries, store.NormalizeKeyword(f.Query)) == "" {
			out = append(out, f)
		}
	}
	return out
}

// queries are the names of the queries a lookup found.
func queries(lists ...[]Found) []string {
	var out []string
	for _, l := range lists {
		for _, f := range l {
			out = append(out, f.Query)
		}
	}
	return out
}

// delta reads the graph after a write of keyword (the one looked up, "" for
// a seed put in) that wrote names (what it found, and the keywords whose
// depth changed): its counts, and, while a page watches research id, the
// Delta its pages get: the keywords written, keyword's relationships, and
// the communities of the whole graph.
func (c *Crawler) delta(g *Graph, id uint32, keyword string, names []string) (Counts, *Delta, error) {
	nodes, err := g.Nodes()
	if err != nil {
		return Counts{}, nil, err
	}
	n := g.Counts(nodes)
	if !c.Watch.Watched(id) {
		return n, nil, nil
	}
	edges, err := g.Edges(nodes)
	if err != nil {
		return n, nil, err
	}
	written := map[string]bool{keyword: true}
	for _, name := range names {
		written[store.NormalizeKeyword(name)] = true
	}
	d := &Delta{Keyword: keyword}
	for _, x := range nodes {
		if written[x.Name] {
			d.Nodes = append(d.Nodes, x)
		}
	}
	for _, e := range edges {
		if keyword != "" && e.From == keyword {
			d.Edges = append(d.Edges, e)
		}
	}
	d.Community, d.Communities = Communities(nodes, edges)
	return n, d, nil
}

// wrote saves the graph's counts for the list and tells the pages what was
// written (d nil: no page watched, so they read the graph again), then that
// the counts changed.
func (c *Crawler) wrote(id uint32, n Counts, d *Delta) error {
	err := c.Store.SetResearchState(id, map[string]any{"node_count": n.Nodes, "edge_count": n.Edges, "explored_count": n.Explored})
	c.Watch.Publish(Change{ID: id, Kind: Wrote, Delta: d})
	c.changed(id)
	return err
}

// skipError is a keyword Trends refused as a search term.
type skipError struct{ err error }

func (e *skipError) Error() string { return e.err.Error() }

// lookup looks keyword up, waiting and trying again while Google limits the
// browser (Backoff), and once more after another failure.
func (c *Crawler) lookup(j *job, connection uint32, keyword string, f Filters) (top, rising []Found, err error) {
	again := false
	for try := 0; ; try++ {
		top, rising, err = c.Trends.Related(c.ctx, connection, keyword, f)
		if err == nil {
			c.setProgress(j, func(p *Progress) { p.Note = "" })
			return top, rising, nil
		}
		if c.ctx.Err() != nil {
			return nil, nil, c.ctx.Err()
		}
		switch connect.CodeOf(err) {
		case connect.CodeFailedPrecondition, connect.CodeUnimplemented, connect.CodeUnauthenticated, connect.CodePermissionDenied:
			// Nothing a second try mends: no connection, a San Remote
			// without Trends, a browser not attached.
			return nil, nil, fmt.Errorf("looking up %q: %s", keyword, message(err))
		case connect.CodeResourceExhausted:
			if try >= len(c.Backoff) {
				return nil, nil, fmt.Errorf("Google kept limiting the Trends lookups of the browser (%d tries): wait an hour or so, answer its check in Chrome if it shows one, then Run goes on from there (%s)", try+1, message(err))
			}
			wait := c.Backoff[try]
			c.Log.Warn("keyword research: Google limits the lookups; waiting", "research", j.id, "wait", wait, "try", try+1)
			c.setProgress(j, func(p *Progress) {
				p.Note = fmt.Sprintf("Google asked to slow down: trying again in %s (%d of %d)", wait, try+1, len(c.Backoff))
			})
			if !sleep(j.waitCtx, wait) {
				return nil, nil, errStopped
			}
		case connect.CodeInvalidArgument:
			return nil, nil, &skipError{err}
		default:
			if again || j.stop {
				return nil, nil, fmt.Errorf("looking up %q: %s", keyword, message(err))
			}
			again = true
			c.Log.Warn("keyword research: a lookup failed; trying once more", "research", j.id, "keyword", keyword, "err", err)
			c.setProgress(j, func(p *Progress) { p.Note = "the lookup failed; trying once more" })
			if !sleep(j.waitCtx, 2*c.Pause+time.Second) {
				return nil, nil, errStopped
			}
		}
	}
}

// message is err's text without Connect's code in front.
func message(err error) string {
	var ce *connect.Error
	if errors.As(err, &ce) {
		return ce.Message()
	}
	return err.Error()
}

// sleep waits d, or less when ctx ends: then it says false.
func sleep(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-ctx.Done():
		return false
	}
}
