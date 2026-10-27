package engine

import (
	"cmp"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"google.golang.org/protobuf/encoding/protojson"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
)

// Remote Shell (the user, 2026-10-27: "Implement remote_shell_protocol.md.
// it to be node"): the Web Machine Shell Protocol of
// golang/apps/san_remote/docs/remote_shell_protocol.md. A web chat
// (ChatGPT, Gemini or Claude on claude.ai, through San Remote) talks with
// the user and the machine, a bash shell on this computer, in one chat.
// Every message to it is a WebchatRequest and every answer a
// WebchatResponse (remote_shell.proto), as JSON. An answer's chat goes to
// the user, its execute is run by the shell and the result goes back to the
// chat from the machine, until an answer has nothing to execute: then the
// chat waits for the user's next message, which a later run brings (a
// Browser Chat Session keeps the chat).

var remoteShellKinds = []*kind{{
	StepKind: &v1.StepKind{
		Kind: "remote-shell", Title: "Remote Shell", Category: "AI",
		Description: "The Web Machine Shell Protocol: the user's Message goes to a web chat (ChatGPT, Gemini or Claude on claude.ai, through San Remote), which answers in JSON with a chat for the user and a bash script to execute on this computer. Chat fires for each chat, with Reply its text; each script runs in a storage's folder when one is picked, else in the tower's data folder, and what it gave goes back to the chat, until an answer has nothing to execute. The chat is a kept one: Chat Link goes on in a chat, a chat's link or a Browser Chat Session's, and the output is its link. Commands counts the scripts run; there is no limit unless Max Commands sets one, past which the step fails. The step fails too when the chat looks stuck: the same script with the same result 3 times in a row. The web chat runs whatever it wants with the tower's rights: give it a storage to work in.",
		Outputs:     []*v1.Socket{sock("Done", Event), sock("Chat", Event), sock("Reply", Text), sock("Chat Link", Text), sock("Commands", Text)},
		Settings: []*v1.Setting{
			connectionSetting(),
			{Name: "Site", Options: []string{"ChatGPT", "Gemini", "Claude"}},
			{Name: "Wait", Placeholder: "10m: how long each answer may take"},
			{Name: "Storage", Storage: true, Placeholder: "No storage: the data folder"},
			{Name: "Timeout", Placeholder: "5m: how long each script may run"},
			{Name: "Max Commands", Placeholder: "No limit"},
		},
		Inputs: []*v1.Socket{sock("Event", Event), field("Message"), field("Chat Link")},
	},
	run: remoteShell,
}}

// What the machine tells the chat of each of a script's stdout and
// stderr: past it, the middle is left out.
const shellResultMax = 8000

// shellStuck is how many times in a row the chat may run the same script
// with the same result before the step takes it as stuck and fails (the
// user, 2026-10-27: long jobs, such as making a video or setting up a
// project, need no limit on the scripts, but a chat retrying a broken
// script forever must end). A script whose result changes, such as one
// polling a render, goes on.
const shellStuck = 3

func remoteShell(c *call) error {
	message := strings.TrimSpace(c.text("Message"))
	if message == "" {
		return fmt.Errorf("Message %w", errEmpty)
	}
	site := cmp.Or(c.setting("Site"), "ChatGPT")
	ask := browserAsks[site]
	if ask == nil {
		return fmt.Errorf("Site %q: pick ChatGPT, Gemini or Claude", site)
	}
	most := 0 // no limit
	if s := c.setting("Max Commands"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 {
			return fmt.Errorf("Max Commands %q: give a number, 1 or more", s)
		}
		most = n
	}
	wait, err := c.wait()
	if err != nil {
		return err
	}
	timeout, err := c.timeout()
	if err != nil {
		return err
	}
	dir, err := c.workDir()
	if err != nil {
		return err
	}
	r, err := c.sanRemote()
	if err != nil {
		return err
	}
	chat, session, err := c.chatLink(site)
	if err != nil {
		return err
	}
	sc := &shellChat{c: c, r: r, ask: ask, site: site, session: session, chat: chat, wait: wait}

	// A new chat learns the protocol first, in the same message; a chat that
	// goes on is reminded of it with the user's message (the user, 2026-10-27:
	// a chat picked up hours later may have drifted from it), not with a
	// script's result, which comes seconds after the last answer.
	prompt := shellReminder + "\n"
	if chat == "" {
		prompt = shellProtocol(dir, timeout) + "\n\n"
	}
	req := &v1.WebchatRequest{Chat: message, From: v1.FromType_FROM_TYPE_USER}
	commands, repeats, last := 0, 0, ""
	c.outputs["Reply"] = stringValue("")
	c.outputs["Commands"] = stringValue("0")
	for {
		res, err := sc.send(prompt + webchatJSON(req))
		c.outputs["Chat Link"] = stringValue(sc.chat)
		if err != nil {
			return err
		}
		prompt = ""
		if text := strings.TrimSpace(res.GetChat()); text != "" {
			c.logf(v1.LogLevel_LOG_LEVEL_INFO, "%s: %s", site, text)
			c.outputs["Reply"] = stringValue(text)
			if c.each != nil {
				if err := c.each("Chat"); err != nil {
					return err
				}
			}
		}
		script := res.GetExecute()
		if strings.TrimSpace(script) == "" {
			break
		}
		if most > 0 && commands == most {
			return fmt.Errorf("the chat asks for a script after Max Commands, %d: %s", most, clip(script, 300))
		}
		if sc.chat == "" {
			return errors.New("the chat has no link, so the script's result cannot go back to it")
		}
		commands++
		c.outputs["Commands"] = stringValue(strconv.Itoa(commands))
		// The script, each line it prints as it comes (the Logs pages show
		// them while the run goes on; live.go), then how it ended.
		c.logf(v1.LogLevel_LOG_LEVEL_INFO, "script %d:\n%s", commands, clip(strings.TrimSpace(script), 2000))
		out, err := runScript(c.ctx, script, "", dir, timeout, func(line string) { c.log(v1.LogLevel_LOG_LEVEL_DEBUG, line) })
		if err != nil {
			return err
		}
		status, level := fmt.Sprintf("exit %d", out.code), v1.LogLevel_LOG_LEVEL_INFO
		if out.timedOut {
			status = "stopped after the Timeout, " + timeout.String()
		}
		if out.timedOut || out.code != 0 {
			level = v1.LogLevel_LOG_LEVEL_WARNING
		}
		c.logf(level, "script %d: %s after %s", commands, status, out.took)
		result := executeResult(out, status)
		if ran := strings.TrimSpace(script) + "\x00" + result; ran == last {
			repeats++
		} else {
			repeats, last = 1, ran
		}
		if repeats == shellStuck {
			return fmt.Errorf("the chat looks stuck: it ran the same script %d times in a row, with the same result (%s): %s", shellStuck, status, clip(strings.TrimSpace(script), 300))
		}
		req = &v1.WebchatRequest{ExecuteResult: result, From: v1.FromType_FROM_TYPE_EXECUTE}
	}
	c.fire("Done")
	return nil
}

// shellChat is a Remote Shell step's web chat: chat is its link, empty
// until the first answer of a new chat; a Browser Chat Session's session
// keeps it.
type shellChat struct {
	c       *call
	r       remoteClients
	ask     browserAsk
	site    string
	session string
	chat    string
	wait    time.Duration
}

// shellTabIdle is how long San Remote keeps a Remote Shell chat's tab open
// after an ask: each turn goes on in the same tab, and the tab is closed
// once no ask has come for this long (the user, 2026-10-27: "stay open and
// use same tab, and if there is no activity in 5 minute, close it").
const shellTabIdle = 5 * time.Minute

// say sends prompt to the chat, a kept one, and gives its answer.
func (s *shellChat) say(prompt string) (string, error) {
	s.c.logf(v1.LogLevel_LOG_LEVEL_DEBUG, "waiting for %s's answer", s.site)
	answer, link, err := s.ask(s.c, s.r, prompt, s.wait, s.chat, true, false, shellTabIdle)
	if err != nil {
		return "", remoteErr(err)
	}
	if err := s.c.keepChat(s.session, s.site, s.chat, link); err != nil {
		return "", err
	}
	if link != "" {
		s.chat = link
	}
	return answer, nil
}

// send sends prompt and reads the answer as a WebchatResponse. An answer
// that is not one is asked for once more.
func (s *shellChat) send(prompt string) (*v1.WebchatResponse, error) {
	answer, err := s.say(prompt)
	if err != nil {
		return nil, err
	}
	res, perr := parseWebchat(answer)
	if perr == nil {
		return res, nil
	}
	if s.chat == "" {
		return nil, fmt.Errorf("the answer is not a WebchatResponse (%v): %s", perr, clip(answer, 300))
	}
	s.c.logf(v1.LogLevel_LOG_LEVEL_WARNING, "the answer is not a WebchatResponse (%v): asked again", perr)
	if answer, err = s.say(notWebchatResponse); err != nil {
		return nil, err
	}
	if res, perr = parseWebchat(answer); perr != nil {
		return nil, fmt.Errorf("the answer is not a WebchatResponse (%v): %s", perr, clip(answer, 300))
	}
	return res, nil
}

const shellReminder = `(Web Machine Shell Protocol: answer with only one JSON {"execute": "...", "chat": "..."}, nothing else.)`

const notWebchatResponse = `Your last answer was not one JSON WebchatResponse. Answer it again as {"execute": "...", "chat": "..."}, with nothing before or after it.`

// shellProtocol is the start of a new chat's first message: the protocol,
// with the shell, the folder it works in, and how long a script may run.
// Each script is a new bash in that folder (2026-10-27: the chat should
// not count on a cd from the last one).
func shellProtocol(dir string, timeout time.Duration) string {
	shell := "bash"
	if runtime.GOOS == "windows" {
		shell = "Git Bash on Windows"
	}
	return fmt.Sprintf(`This chat follows the Web Machine Shell Protocol. Two parties talk with you in it: a user, and a machine, which is a %s shell on the user's computer, working in the folder %s.

Every message you get is one JSON WebchatRequest:
{"execute_result": "...", "chat": "...", "from": "FROM_TYPE_USER" or "FROM_TYPE_EXECUTE"}
- from FROM_TYPE_USER: chat is the user's message.
- from FROM_TYPE_EXECUTE: execute_result is what the machine's shell gave for your last execute: its exit code, stdout and stderr.

Answer every message with one JSON WebchatResponse, with nothing before or after it:
{"execute": "...", "chat": "..."}
- execute: a bash script for the machine to run now. What it gives comes back to you in the next message. Leave it empty when nothing is to be run. Each script runs in a new shell that starts in %[2]s: a cd or a variable from the last one does not carry over. A script may run up to %[3]s; start a longer job in the background, writing to a log file, and check on it with later scripts.
- chat: what to tell the user. Leave it empty when there is nothing to say.
When both are set, the user gets chat, then the machine runs execute. When execute is empty, your turn is over until the user writes again: ask the user in chat for what you need, and before running anything that deletes or changes what cannot be undone.

The first message:`, shell, filepath.ToSlash(dir), timeout)
}

// webchatJSON is req as the chat reads it, with the protocol's field names
// and every field, empty or not.
func webchatJSON(req *v1.WebchatRequest) string {
	b, _ := protojson.MarshalOptions{UseProtoNames: true, EmitUnpopulated: true}.Marshal(req)
	return string(b)
}

// parseWebchat reads a web chat's answer as a WebchatResponse: the JSON
// object in it, from its first { to its last }, so that a code fence or a
// line around it does no harm. Fields it does not know are left out. An
// object that is not JSON as it is is read again repaired (repairJSON).
func parseWebchat(answer string) (*v1.WebchatResponse, error) {
	i, j := strings.Index(answer, "{"), strings.LastIndex(answer, "}")
	if i < 0 || j < i {
		return nil, errors.New("no JSON object in it")
	}
	obj := answer[i : j+1]
	res := &v1.WebchatResponse{}
	opts := protojson.UnmarshalOptions{DiscardUnknown: true}
	if err := opts.Unmarshal([]byte(obj), res); err != nil {
		if opts.Unmarshal([]byte(repairJSON(obj)), res) != nil {
			return nil, err
		}
	}
	return res, nil
}

// repairJSON undoes what an answer goes through on its way: San Remote
// reads it with the site's Copy button, which gives markdown, and JSON
// written as text comes out with markdown's escapes, a backslash before
// * _ : [ and such (2026-10-27, Gemini: "\*\*bold\*\*"), which JSON does
// not have. In the object's strings, a backslash before a character JSON
// does not escape is dropped, and a raw line break or tab, which a chat
// may write, is escaped.
func repairJSON(s string) string {
	var b strings.Builder
	in := false
	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch {
		case !in:
			in = ch == '"'
			b.WriteByte(ch)
		case ch == '"':
			in = false
			b.WriteByte(ch)
		case ch == '\\':
			if i+1 < len(s) && strings.IndexByte(`"\/bfnrtu`, s[i+1]) >= 0 {
				b.WriteString(s[i : i+2])
				i++
			}
		case ch == '\n':
			b.WriteString(`\n`)
		case ch == '\r':
			b.WriteString(`\r`)
		case ch == '\t':
			b.WriteString(`\t`)
		default:
			b.WriteByte(ch)
		}
	}
	return b.String()
}

// executeResult is what the machine tells the chat a script gave.
func executeResult(out scriptResult, status string) string {
	return fmt.Sprintf("%s\nstdout:\n%s\nstderr:\n%s", status,
		clipMiddle(trimNewline(out.stdout), shellResultMax), clipMiddle(trimNewline(out.stderr), shellResultMax))
}

// clip is s cut to n characters, with … when it was longer.
func clip(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}

// clipMiddle is s with its middle left out when it is longer than n
// characters: a listing's start and an error's end both stay.
func clipMiddle(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	head, tail := n/2, n-n/2
	return fmt.Sprintf("%s\n… (%d characters left out) …\n%s", string(r[:head]), len(r)-n, string(r[len(r)-tail:]))
}
