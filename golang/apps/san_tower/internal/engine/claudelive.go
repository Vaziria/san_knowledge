package engine

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/wargasipil/san_tower/internal/telegram"
)

// A chat session's questions (Claude Ask (CLI) with a Session ID) go to one
// Claude Code process while it answers them (the user, 2026-10-26: the
// Telegram chat as interactive as Claude Code itself). It is claude -p in
// stream-json both ways, which a few experiments with claude 2.1.306 showed
// to work as Claude Code does on screen:
//   - A question sent while it works is queued by Claude Code itself and
//     answered next, in the same process; several queued at once are read as
//     one turn, and answered once.
//   - --replay-user-messages echoes each question, with the uuid it was sent
//     with, as Claude Code takes it into a turn: the turn's answer (its
//     result) is for the last question taken; the others were read with it.
//   - A control request "interrupt" stops the turn, as Esc does: Claude Code
//     notes "[Request interrupted by user]", the turn ends without an answer
//     (error_during_execution), and the conversation and the queue go on.
//   - --json-schema still shapes each answer (structured_output), and each
//     tool call comes as it is made, for the live status.
//
// Once no question waits, its stdin is closed and the process ends, so
// nothing runs, or takes memory, between conversations; the next question
// starts it again with --resume.

// liveClaudes are the sessions' running processes, by session ID.
var liveClaudes = struct {
	sync.Mutex
	m map[string]*claudeLive
}{m: map[string]*claudeLive{}}

// claudeLive is a session's running claude process.
type claudeLive struct {
	session string
	cmd     *exec.Cmd
	done    chan struct{} // closed once it has ended and its questions are told

	mu       sync.Mutex
	stdin    io.WriteCloser
	waiting  []*liveAsk // sent, not yet taken into a turn
	turn     []*liveAsk // taken into the turn going on; the last gets its answer
	stopping bool       // an interrupt was sent for the turn going on
	closing  bool       // stdin closed: it takes no more questions
	began    bool       // it said anything: --resume found the conversation
	resume   bool       // started with --resume, not --session-id
	start    func(resume bool) error
	stderr   *strings.Builder
}

// liveAsk is a question sent to a session's process.
type liveAsk struct {
	id     string
	text   string
	reply  chan liveReply // of one
	taken  func()         // its turn began
	doing  func(string)   // a line of what Claude does in its turn
	finish func(liveReply)
}

// liveReply is how a question ended.
type liveReply struct {
	text     string
	question bool
	merged   bool // read together with a later question, whose run answers
	stopped  bool // its turn was interrupted
	err      error
}

// errClosing is a session process that takes no more questions: the next
// one waits for it to end, then starts another.
var errClosing = errors.New("the session's claude is ending")

// askLive sends text to session's process, started in dir with args when
// none runs, and returns the question to wait on.
func askLive(session, dir string, args []string, a *liveAsk) (*claudeLive, error) {
	for {
		liveClaudes.Lock()
		l := liveClaudes.m[session]
		if l == nil {
			var err error
			if l, err = startLive(session, dir, args); err != nil {
				liveClaudes.Unlock()
				return nil, err
			}
			liveClaudes.m[session] = l
		}
		liveClaudes.Unlock()
		err := l.send(a)
		if errors.Is(err, errClosing) {
			<-l.done
			continue
		}
		return l, err
	}
}

// startLive starts session's process: --resume, or, when Claude Code has no
// such conversation yet, --session-id.
func startLive(session, dir string, args []string) (*claudeLive, error) {
	l := &claudeLive{session: session, done: make(chan struct{})}
	l.start = func(resume bool) error {
		flag := "--session-id"
		if resume {
			flag = "--resume"
		}
		cmd := exec.Command(claudeCLI, slices.Concat(args, []string{flag, session})...)
		cmd.Dir = dir
		stdin, err := cmd.StdinPipe()
		if err != nil {
			return err
		}
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return err
		}
		l.stderr = &strings.Builder{}
		cmd.Stderr = l.stderr
		if err := cmd.Start(); err != nil {
			if errors.Is(err, exec.ErrNotFound) {
				return errors.New("the claude command is not on this computer's PATH: install Claude Code and sign in")
			}
			return err
		}
		l.cmd, l.stdin, l.resume, l.began = cmd, stdin, resume, false
		go l.read(cmd, stdout)
		return nil
	}
	if err := l.start(true); err != nil {
		return nil, err
	}
	return l, nil
}

// send writes question a to the process.
func (l *claudeLive) send(a *liveAsk) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closing {
		return errClosing
	}
	l.waiting = append(l.waiting, a)
	return l.write(userLine(a))
}

// userLine is question a as stream-json input, with its uuid, which the
// replay gives back.
func userLine(a *liveAsk) map[string]any {
	return map[string]any{"type": "user", "uuid": a.id, "session_id": "", "parent_tool_use_id": nil,
		"message": map[string]any{"role": "user", "content": a.text}}
}

// busy says whether it is answering or has questions waiting.
func (l *claudeLive) busy() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.turn)+len(l.waiting) > 0
}

// write writes one JSON line to stdin. Called with l.mu held.
func (l *claudeLive) write(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = l.stdin.Write(append(b, '\n'))
	return err
}

// interrupt stops the turn going on, as Esc does in Claude Code; the
// questions sent after it are still answered. It says whether there was
// one.
func (l *claudeLive) interrupt() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closing || len(l.turn)+len(l.waiting) == 0 {
		return false
	}
	l.stopping = true
	return l.write(map[string]any{"type": "control_request", "request_id": newUUID(),
		"request": map[string]any{"subtype": "interrupt"}}) == nil
}

// kill ends the process at once: the tower is stopping.
func (l *claudeLive) kill() {
	l.mu.Lock()
	cmd := l.cmd
	l.mu.Unlock()
	if cmd != nil && cmd.Process != nil {
		cmd.Process.Kill()
	}
}

// liveEvent is the part of a stream-json line the tower reads.
type liveEvent struct {
	Type     string   `json:"type"`
	Subtype  string   `json:"subtype"`
	UUID     string   `json:"uuid"`
	IsReplay bool     `json:"isReplay"`
	IsError  bool     `json:"is_error"`
	Errors   []string `json:"errors"`
	Result   string   `json:"result"`
	Message  struct {
		Content json.RawMessage `json:"content"`
	} `json:"message"`
	Structured *struct {
		NeedsFeedback bool   `json:"needs_feedback"`
		Text          string `json:"text"`
	} `json:"structured_output"`
}

// read reads the process cmd's events until it ends.
func (l *claudeLive) read(cmd *exec.Cmd, stdout io.Reader) {
	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 64<<10), 64<<20) // a tool's result can be long
	for sc.Scan() {
		var ev liveEvent
		if json.Unmarshal(sc.Bytes(), &ev) != nil {
			continue
		}
		l.event(ev)
	}
	l.ended(cmd.Wait())
}

func (l *claudeLive) event(ev liveEvent) {
	l.mu.Lock()
	if ev.Type == "system" && ev.Subtype == "init" {
		l.began = true
	}
	if ev.Type == "result" && !l.began {
		// It never began: with --resume of a conversation Claude Code does
		// not have yet, it says "No conversation found" here and ends;
		// ended starts it with --session-id.
		l.mu.Unlock()
		return
	}
	var taken *liveAsk
	var lines []string
	var owner *liveAsk
	var told []func()
	switch ev.Type {
	case "user":
		if !ev.IsReplay {
			break
		}
		for i, a := range l.waiting {
			if a.id == ev.UUID {
				l.waiting = slices.Delete(l.waiting, i, i+1)
				l.turn = append(l.turn, a)
				taken = a
				break
			}
		}
	case "assistant":
		if len(l.turn) > 0 {
			owner = l.turn[len(l.turn)-1]
			lines = describeContent(ev.Message.Content)
		}
	case "result":
		if len(l.turn) == 0 && len(l.waiting) > 0 { // no replays: the oldest
			l.turn, l.waiting = l.waiting[:1], l.waiting[1:]
		}
		for i, a := range l.turn {
			r := liveReply{merged: true}
			if i == len(l.turn)-1 {
				r = l.answer(ev)
			}
			told = append(told, func() { a.end(r) })
		}
		l.turn, l.stopping = nil, false
		if len(l.waiting) == 0 {
			// Nothing waits: the process may end. A question that comes now
			// waits for it, then starts another.
			l.closing = true
			l.stdin.Close()
		}
	}
	l.mu.Unlock()
	if taken != nil && taken.taken != nil {
		taken.taken()
	}
	for _, line := range lines {
		if owner.doing != nil {
			owner.doing(line)
		}
	}
	for _, f := range told {
		f()
	}
}

// answer reads the turn's result. Called with l.mu held.
func (l *claudeLive) answer(ev liveEvent) liveReply {
	switch {
	case l.stopping && (ev.IsError || ev.Subtype != "success"):
		return liveReply{stopped: true}
	case ev.IsError || ev.Subtype != "success":
		msg := strings.TrimSpace(ev.Result)
		if msg == "" {
			msg = strings.Join(ev.Errors, "; ")
		}
		if msg == "" {
			msg = ev.Subtype
		}
		return liveReply{err: fmt.Errorf("Claude: %s", cut(msg, 300))}
	case ev.Structured != nil:
		return liveReply{text: strings.TrimSpace(ev.Structured.Text), question: ev.Structured.NeedsFeedback}
	}
	return liveReply{text: strings.TrimSpace(ev.Result)} // not in the shape asked for: an answer
}

// end tells question a how it ended.
func (a *liveAsk) end(r liveReply) {
	if a.finish != nil {
		a.finish(r)
	}
	a.reply <- r
}

// ended: the process is gone. One started with --resume that said nothing
// and found no conversation starts it with --session-id, its questions sent
// again; otherwise the questions not answered fail.
func (l *claudeLive) ended(err error) {
	l.mu.Lock()
	msg := strings.TrimSpace(l.stderr.String())
	if l.resume && !l.began && strings.Contains(msg, "No conversation found") {
		asks := slices.Concat(l.turn, l.waiting)
		l.turn, l.waiting = nil, nil
		serr := l.start(false)
		if serr == nil {
			for _, a := range asks {
				l.waiting = append(l.waiting, a)
				_ = l.write(userLine(a))
			}
			l.mu.Unlock()
			return
		}
		l.waiting, msg = asks, serr.Error()
	}
	asks := slices.Concat(l.turn, l.waiting)
	l.turn, l.waiting, l.closing = nil, nil, true
	l.mu.Unlock()
	liveClaudes.Lock()
	if liveClaudes.m[l.session] == l {
		delete(liveClaudes.m, l.session)
	}
	liveClaudes.Unlock()
	close(l.done)
	if msg == "" && err != nil {
		msg = err.Error()
	}
	for _, a := range asks {
		a.end(liveReply{err: fmt.Errorf("claude ended before it answered: %s", cut(msg, 300))})
	}
}

// interruptClaude stops what the process of session is doing, and says
// whether it was doing anything.
func interruptClaude(session string) bool {
	liveClaudes.Lock()
	l := liveClaudes.m[session]
	liveClaudes.Unlock()
	return l != nil && l.interrupt()
}

// describeContent turns an assistant message's content into lines of what
// Claude does: each tool call, and the first line of what it says.
func describeContent(content json.RawMessage) []string {
	var blocks []struct {
		Type  string          `json:"type"`
		Text  string          `json:"text"`
		Name  string          `json:"name"`
		Input json.RawMessage `json:"input"`
	}
	if json.Unmarshal(content, &blocks) != nil {
		return nil
	}
	var out []string
	for _, b := range blocks {
		switch b.Type {
		case "text":
			if line := firstLine(b.Text); line != "" {
				out = append(out, "“"+cut(line, 90)+"”")
			}
		case "tool_use":
			if line := describeTool(b.Name, b.Input); line != "" {
				out = append(out, line)
			}
		}
	}
	return out
}

// describeTool is a tool call as a short line, as Claude Code shows it.
func describeTool(name string, input json.RawMessage) string {
	var in map[string]any
	_ = json.Unmarshal(input, &in)
	str := func(k string) string {
		s, _ := in[k].(string)
		return strings.TrimSpace(s)
	}
	switch name {
	case "StructuredOutput": // the answer itself
		return ""
	case "Read", "Write", "Edit", "MultiEdit", "NotebookEdit":
		return name + " " + filepath.Base(str("file_path"))
	case "Bash", "PowerShell":
		if d := str("description"); d != "" {
			return d
		}
		return cut(firstLine(str("command")), 70)
	case "Grep":
		return "Search " + cut(str("pattern"), 60)
	case "Glob":
		return "Find " + cut(str("pattern"), 60)
	case "WebFetch":
		if u, err := url.Parse(str("url")); err == nil && u.Host != "" {
			return "Fetch " + u.Host
		}
		return "Fetch a page"
	case "WebSearch":
		return "Search the web: " + cut(str("query"), 60)
	case "Agent", "Task":
		return "Agent: " + cut(str("description"), 60)
	case "TodoWrite":
		return "Update the plan"
	}
	if s, ok := strings.CutPrefix(name, "mcp__"); ok {
		server, tool, _ := strings.Cut(s, "__")
		return server + ": " + tool
	}
	return name
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

// askClaudeLive asks prompt in session, through its live process (started in
// dir with args when none runs), waits for its answer up to wait, and gives
// it as claudeAskCLI does. With Status Bot and Status Chat, a Telegram
// message shows what Claude is doing meanwhile. The steps Claude took are
// the step's log lines.
func (c *call) askClaudeLive(session, dir string, args []string, prompt string, wait time.Duration) error {
	if wait == 0 {
		wait = 10 * time.Minute
	}
	ctx, cancel := context.WithTimeout(c.ctx, wait)
	defer cancel()
	a := &liveAsk{id: newUUID(), text: prompt, reply: make(chan liveReply, 1)}
	var mu sync.Mutex
	var steps []string
	a.doing = func(line string) {
		mu.Lock()
		steps = append(steps, line)
		mu.Unlock()
	}
	if chat := strings.TrimSpace(c.text("Status Chat")); chat != "" && c.setting("Status Bot") != "" {
		cl, _, err := c.telegramOf("Status Bot")
		if err != nil {
			return err
		}
		liveClaudes.Lock()
		l := liveClaudes.m[session]
		liveClaudes.Unlock()
		status, err := newLiveStatus(ctx, cl, chat, l != nil && l.busy())
		if err != nil {
			c.logs = append(c.logs, "the live status could not be sent: "+err.Error())
		} else {
			doing := a.doing
			a.taken, a.finish = status.working, status.end
			a.doing = func(line string) {
				doing(line)
				status.add(line)
			}
		}
	}
	l, err := askLive(session, dir, args, a)
	if err != nil {
		if a.finish != nil {
			a.finish(liveReply{err: err})
		}
		return err
	}
	var r liveReply
	select {
	case r = <-a.reply:
	case <-ctx.Done():
		if c.ctx.Err() != nil { // the tower is stopping
			l.kill()
			return c.ctx.Err()
		}
		l.interrupt()
		return fmt.Errorf("Claude did not answer within %s: stopped it", wait)
	}
	mu.Lock()
	c.logs = append(c.logs, steps...)
	mu.Unlock()
	switch {
	case r.err != nil:
		return r.err
	case r.merged:
		c.logs = append(c.logs, "read together with the message after it, whose run answers both")
		return nil
	case r.stopped:
		c.fire("Stopped")
		return nil
	}
	c.claudeAnswered(r.text, r.question)
	return nil
}

// liveStatus is a Telegram message that shows what Claude is doing for a
// question, edited as it goes, as Claude Code shows it on screen: queued,
// then working with its last steps, then how it ended.
type liveStatus struct {
	cl   *telegram.Client
	chat string
	id   int64

	mu    sync.Mutex
	state string    // Queued or Working
	since time.Time // when it began working
	lines []string  // the last few steps
	steps int
	dirty bool
	quit  chan struct{}
	gone  chan struct{}
}

// statusEvery is how often the status is edited at most: Telegram allows
// about one message a second in a chat.
var statusEvery = 3 * time.Second

// newLiveStatus sends the status message, Queued or Working.
func newLiveStatus(ctx context.Context, cl *telegram.Client, chat string, queued bool) (*liveStatus, error) {
	s := &liveStatus{cl: cl, chat: chat, state: "Working", since: time.Now(), quit: make(chan struct{}), gone: make(chan struct{})}
	if queued {
		s.state = "Queued: Claude reads it once it is done with the message before"
	}
	sent, err := cl.SendMessage(ctx, telegram.Send{Chat: chat, Text: s.render()})
	if err != nil {
		return nil, err
	}
	s.id = telegram.MessageID(sent)
	go s.loop()
	return s, nil
}

// working: its turn began.
func (s *liveStatus) working() {
	s.mu.Lock()
	s.state, s.since, s.dirty = "Working", time.Now(), true
	s.mu.Unlock()
}

// add adds a step.
func (s *liveStatus) add(line string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !strings.HasPrefix(line, "“") {
		s.steps++
	}
	s.lines = append(s.lines, line)
	if len(s.lines) > 6 {
		s.lines = s.lines[len(s.lines)-6:]
	}
	s.dirty = true
}

func (s *liveStatus) render() string {
	head := s.state
	if s.state == "Working" {
		head = fmt.Sprintf("Working · %s · %d steps", time.Since(s.since).Round(time.Second), s.steps)
	}
	var b strings.Builder
	b.WriteString(head)
	for _, l := range s.lines {
		b.WriteString("\n› " + l)
	}
	return b.String()
}

// loop edits the message when it changed, every statusEvery, and keeps the
// working time going.
func (s *liveStatus) loop() {
	defer close(s.gone)
	t := time.NewTicker(statusEvery)
	defer t.Stop()
	for {
		select {
		case <-s.quit:
			return
		case <-t.C:
		}
		s.mu.Lock()
		edit := s.dirty || s.state == "Working"
		text := s.render()
		s.dirty = false
		s.mu.Unlock()
		if edit {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			_ = s.cl.EditMessageText(ctx, s.chat, s.id, text)
			cancel()
		}
	}
}

// end stops the edits and shows how it ended.
func (s *liveStatus) end(r liveReply) {
	close(s.quit)
	<-s.gone
	s.mu.Lock()
	took := time.Since(s.since).Round(time.Second)
	head := fmt.Sprintf("Done · %s · %d steps", took, s.steps)
	switch {
	case r.merged:
		head, s.lines = "Read together with your next message", nil
	case r.stopped:
		head = fmt.Sprintf("Stopped · %s · %d steps", took, s.steps)
	case r.err != nil:
		head = fmt.Sprintf("Failed · %s: %s", took, cut(r.err.Error(), 200))
	}
	s.state = head
	text := head
	for _, l := range s.lines {
		text += "\n› " + l
	}
	s.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	_ = s.cl.EditMessageText(ctx, s.chat, s.id, text)
	cancel()
}
