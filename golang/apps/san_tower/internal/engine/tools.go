package engine

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"google.golang.org/protobuf/types/known/structpb"
	"san_knowledge/graph"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/telegram"
)

// Tools (the user, 2026-10-29: "we have Tool meta, like if telegram we have
// Telegram Tool Meta, and in ai remote shell have input tool ... tool meta
// that connected to remote shell mean remote shell can use that tools").
// A tool step gives a tool: what it is and what it may touch (a knowledge,
// a bot and its one chat). Remote Shell's Tools input takes any number of
// them, and its chat calls one with its answer's tools, {"tool_name":
// "knowledge", "command": "explain", "arguments": ["..."]}; the result goes
// back as the next request's tools_result. A tool that is not linked does
// not exist for the chat.

var toolKinds = []*kind{
	{
		StepKind: &v1.StepKind{
			Kind: "knowledge-tool", Title: "Knowledge Tool", Category: "Knowledge",
			Description: "A tool for an AI step (Remote Shell's Tools): the knowledge a Knowledge Base step links to Knowledge, which the AI may look in with explain (what it says about a question), search (the nodes holding every word) and get (one doc or section in full). Name is the tool's name for the AI (knowledge); two tools need two names.",
			Outputs:     []*v1.Socket{sock("Tool", Tool)},
			Settings:    []*v1.Setting{{Name: "Name", Placeholder: "knowledge"}},
			Inputs:      []*v1.Socket{sock("Knowledge", Knowledge)},
		},
		run: knowledgeToolStep,
	},
	{
		StepKind: &v1.StepKind{
			Kind: "telegram-tool", Title: "Telegram Tool", Category: "Messengers", Group: "Telegram",
			Description: "A tool for an AI step (Remote Shell's Tools): a Telegram bot the AI may send messages with, to Chat only (a chat's id or a channel's @username): its command send, with the text, plain; a long one goes as several messages. Name is the tool's name for the AI (telegram); two tools need two names.",
			Outputs:     []*v1.Socket{sock("Tool", Tool)},
			Settings:    []*v1.Setting{telegramSetting(), {Name: "Name", Placeholder: "telegram"}},
			Inputs:      []*v1.Socket{field("Chat")},
		},
		run: telegramToolStep,
	},
}

// toolName is the name an AI calls a tool by.
var toolName = regexp.MustCompile(`^[a-z][a-z0-9_]{0,31}$`)

// toolNameSetting is a tool step's Name, or def.
func (c *call) toolNameSetting(def string) (string, error) {
	name := cmp.Or(c.setting("Name"), def)
	if !toolName.MatchString(name) {
		return "", fmt.Errorf("Name %q: lowercase letters, digits and _, starting with a letter", name)
	}
	return name, nil
}

func knowledgeToolStep(c *call) error {
	name, err := c.toolNameSetting("knowledge")
	if err != nil {
		return err
	}
	k, _, err := c.linkedKnowledge()
	if err != nil {
		return err
	}
	return c.giveTool(map[string]any{"type": "knowledge", "tool_name": name, "knowledge": strconv.FormatUint(uint64(k.ID), 10)})
}

func telegramToolStep(c *call) error {
	name, err := c.toolNameSetting("telegram")
	if err != nil {
		return err
	}
	_, id, err := c.telegram()
	if err != nil {
		return err
	}
	chat := strings.TrimSpace(c.text("Chat"))
	if chat == "" {
		return fmt.Errorf("Chat %w: give the chat's id or a channel's @username the AI may write to", errEmpty)
	}
	return c.giveTool(map[string]any{"type": "telegram", "tool_name": name, "connection": strconv.FormatUint(uint64(id), 10), "chat": chat})
}

func (c *call) giveTool(tool map[string]any) error {
	v, err := structpb.NewValue(tool)
	if err != nil {
		return err
	}
	c.outputs["Tool"] = v
	return nil
}

// shellTool is a tool as an AI step uses it.
type shellTool interface {
	// describe is the tool in the protocol's list, as the chat calls it.
	describe(name string) string
	// brief is the tool in a line, for the reminder of a chat that goes on.
	brief(name string) string
	// run runs a command and gives what goes back to the chat: its result,
	// or what is wrong with the call, for the chat to correct.
	run(c *call, command string, args []string) string
}

// shellTools are the tools linked to a step's Tools input, by name, in the
// order of their links.
type shellTools struct {
	names  []string
	byName map[string]shellTool
}

// linkedTools are the tools linked to input, nil when none is.
func (c *call) linkedTools(input string) (*shellTools, error) {
	list := c.object(input).GetListValue().GetValues()
	if len(list) == 0 {
		return nil, nil
	}
	ts := &shellTools{byName: map[string]shellTool{}}
	for _, v := range list {
		f := v.GetStructValue().GetFields()
		name := f["tool_name"].GetStringValue()
		if ts.byName[name] != nil {
			return nil, fmt.Errorf("Tools: two tools are named %s: give one another Name", name)
		}
		var tool shellTool
		switch typ := f["type"].GetStringValue(); typ {
		case "knowledge":
			k, root, err := c.knowledge(f["knowledge"].GetStringValue())
			if err != nil {
				return nil, fmt.Errorf("tool %s: %w", name, err)
			}
			tool = &knowledgeTool{name: k.Name, root: root}
		case "telegram":
			cl, _, err := c.telegramByID(f["connection"].GetStringValue())
			if err != nil {
				return nil, fmt.Errorf("tool %s: %w", name, err)
			}
			tool = &telegramTool{cl: cl, connection: f["connection"].GetStringValue(), chat: f["chat"].GetStringValue()}
		default:
			t, err := c.toolOf(typ, f)
			if err != nil {
				return nil, fmt.Errorf("tool %s: %w", name, err)
			}
			tool = t
		}
		ts.names = append(ts.names, name)
		ts.byName[name] = tool
	}
	return ts, nil
}

// protocol is the protocol's part on the tools, for a new chat's first
// message.
func (ts *shellTools) protocol() string {
	var b strings.Builder
	b.WriteString(`- tools: a call to one of the machine's tools, or null when you need none: {"tool_name": "...", "command": "...", "arguments": ["..."]}, one call per answer. Its result comes back in the next message as tools_result. The tools:`)
	for _, name := range ts.names {
		b.WriteString("\n" + ts.byName[name].describe(name))
	}
	return b.String()
}

// brief is the tools in a sentence, for the reminder.
func (ts *shellTools) brief() string {
	parts := make([]string, len(ts.names))
	for i, name := range ts.names {
		parts[i] = ts.byName[name].brief(name)
	}
	return "The machine's tools: " + strings.Join(parts, "; ") + "."
}

// toolsResultMax is how much of a tool's result goes back to the chat.
const toolsResultMax = 12000

// run runs a tools call.
func (ts *shellTools) run(c *call, call *v1.ToolCall) string {
	tool := ts.byName[call.GetToolName()]
	if tool == nil {
		return fmt.Sprintf("error: the machine has no tool %q: its tools are %s", call.GetToolName(), strings.Join(ts.names, ", "))
	}
	return clip(tool.run(c, call.GetCommand(), call.GetArguments()), toolsResultMax)
}

// knowledgeTool is a Knowledge Tool's knowledge.
type knowledgeTool struct {
	name, root string
}

func (k *knowledgeTool) describe(name string) string {
	return fmt.Sprintf(`  - %s: the knowledge base %q: notes and documents as a graph of docs and their sections, each with a key (its file path, then #heading/subheading for a section), a summary, keywords, domains, and loc (file:line). Commands:
    - explain, arguments [question]: what it says about the question: the docs and sections that match best, with their summaries, domains, the nodes linked to them, and the loc to cite.
    - search, arguments [words]: the docs and sections that hold every one of the words, best first: give two or three key words (such as "tower release"), not a sentence.
    - get, arguments [key]: one doc or section by its key, as explain and search give them: its summary, links and full text.
    Look in it before you answer about what it holds, or start work that depends on it, and cite the loc of what you use.`, name, k.name)
}

func (k *knowledgeTool) brief(name string) string {
	return fmt.Sprintf("%s (the knowledge base %q; look in it before answering about what it holds, and cite the loc): explain [question], search [two or three key words, every one must match], get [key]", name, k.name)
}

func (k *knowledgeTool) run(_ *call, command string, args []string) string {
	arg := strings.TrimSpace(strings.Join(args, " "))
	var out string
	err := graph.With(k.root, func(g *graph.Store) error {
		var err error
		switch command {
		case "explain":
			if arg == "" {
				return errors.New("explain needs a question in arguments")
			}
			var ex *graph.Explanation
			if ex, err = g.Explain(arg, 5, 1); err == nil {
				out = fmt.Sprintf("From the knowledge base %q. Cite a node by its loc.\n\n%s", k.name, ex.Markdown())
			}
		case "search":
			if arg == "" {
				return errors.New("search needs words in arguments")
			}
			out, err = k.search(g, arg)
		case "get":
			if arg == "" {
				return errors.New("get needs a key in arguments")
			}
			out, err = getText(g, arg)
		default:
			return fmt.Errorf("no command %q: use explain, search or get", command)
		}
		return err
	})
	if errors.Is(err, graph.ErrNotFound) {
		return fmt.Sprintf("error: there is no node with the key %q: explain and search give the keys", arg)
	}
	if err != nil {
		return "error: " + err.Error()
	}
	return out
}

func (k *knowledgeTool) search(g *graph.Store, words string) (string, error) {
	hits, err := g.Search(words, graph.Filter{})
	if err != nil {
		return "", err
	}
	var found []knowledgeNode
	for i, h := range hits {
		if i == 10 {
			break
		}
		n := nodeOf(h.Node, h.Score)
		if n.Summary == "" {
			n.Excerpt = excerpt(g, h.Node)
		}
		found = append(found, n)
	}
	text := searchText(k.name, words, found, len(hits), true)
	if len(hits) == 0 {
		// A chat tends to search with a whole sentence (2026-10-29: "San
		// Tower release process how San Tower is released" found nothing,
		// "san tower release" finds the README's Release section).
		text += "\nNothing holds all of these words. Search again with two or three key words, or use explain, which matches any of them.\n"
	}
	return text, nil
}

// maxToolText is how much of a node's own text get gives, in characters.
const maxToolText = 8000

// getText is one node for the chat: what it is, its summary and links, and
// its text.
func getText(g *graph.Store, key string) (string, error) {
	n, err := g.Get(key)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# %s (%s `%s`)\n", n.Title, n.NodeType, n.Key)
	if node := nodeOf(n, 0); node.File != "" {
		fmt.Fprintf(&b, "- loc: %s\n", node.place())
	}
	if len(n.Keyword) > 0 {
		fmt.Fprintf(&b, "- keyword: %s\n", strings.Join(n.Keyword, ", "))
	}
	if n.Summary != "" {
		fmt.Fprintf(&b, "- summary: %s\n", n.Summary)
	}
	out, in, err := g.Links(key)
	if err != nil {
		return "", err
	}
	for _, l := range out {
		fmt.Fprintf(&b, "- %s → %s\n", l.Rel, l.To)
	}
	for _, l := range in {
		fmt.Fprintf(&b, "- %s ← %s\n", l.Rel, l.From)
	}
	if n.Loc != "" {
		if d, err := g.ParseDoc(n.Loc); err == nil {
			text := strings.TrimSpace(d.SectionText(n.Key))
			if utf8.RuneCountInString(text) > maxToolText {
				text = string([]rune(text)[:maxToolText]) + "…"
			}
			if text != "" {
				fmt.Fprintf(&b, "\n%s\n", text)
			}
		}
	}
	return b.String(), nil
}

// telegramTool is a Telegram Tool's bot and the one chat it writes to.
type telegramTool struct {
	cl         *telegram.Client
	connection string
	chat       string
}

func (t *telegramTool) describe(name string) string {
	return fmt.Sprintf(`  - %s: a Telegram bot in the chat %s, and only there. Commands: send, arguments [text]: plain text, a long one goes as several messages; send_file and send_photo, arguments [path or link, caption] (a relative path is in your working folder); read, arguments [how many]: the chat's latest messages (10 by default). Write there only what the user asked to be sent or told there.`, name, t.chat)
}

func (t *telegramTool) brief(name string) string {
	return fmt.Sprintf("%s (a Telegram bot, in the chat %s only): send [text], send_file [path, caption], send_photo [path, caption], read [how many]", name, t.chat)
}

func (t *telegramTool) run(c *call, command string, args []string) string {
	if out, ok := t.more(c, command, args); ok {
		return out
	}
	if command != "send" {
		return fmt.Sprintf("error: no command %q: use send, send_file, send_photo or read", command)
	}
	text := strings.TrimSpace(strings.Join(args, "\n"))
	if text == "" {
		return "error: send needs the text in arguments"
	}
	parts := splitText(text, telegramTextMax)
	for i, part := range parts {
		if _, err := c.telegramSendOne(t.cl, telegram.Send{Chat: t.chat, Text: part}, ""); err != nil {
			return fmt.Sprintf("error: message %d of %d not sent: %v", i+1, len(parts), err)
		}
	}
	if len(parts) == 1 {
		return "sent to " + t.chat
	}
	return fmt.Sprintf("sent to %s as %d messages", t.chat, len(parts))
}

// looseTools makes an answer's tools what the protocol reads, when a chat
// wrote it another way: "", [] or {} for none, an argument as a string
// rather than a list. obj is returned as it is when it is not a JSON object.
func looseTools(obj string) string {
	var m map[string]json.RawMessage
	if json.Unmarshal([]byte(obj), &m) != nil {
		return obj
	}
	raw, ok := m["tools"]
	if !ok {
		return obj
	}
	var call map[string]json.RawMessage
	switch t := strings.TrimSpace(string(raw)); {
	case t == "null", t == `""`, t == "[]", t == "{}":
		delete(m, "tools")
	case json.Unmarshal(raw, &call) == nil:
		var one string
		if json.Unmarshal(call["arguments"], &one) == nil {
			call["arguments"], _ = json.Marshal([]string{one})
		}
		m["tools"], _ = json.Marshal(call)
	default:
		return obj
	}
	b, err := json.Marshal(m)
	if err != nil {
		return obj
	}
	return string(b)
}
