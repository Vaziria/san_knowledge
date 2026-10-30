package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/binance"
	"github.com/wargasipil/san_tower/internal/store"
)

// The other tool steps (the user, 2026-10-29: "Implement all rest"): a
// workflow to run, a WhatsApp chat, a Binance account to read, Medium and
// LinkedIn drafts, a storage's files and web pages. Most commands run the
// step that does the same in a workflow (runStep), so a tool checks and
// does what the step does.

var moreToolKinds = []*kind{
	{
		StepKind: &v1.StepKind{
			Kind: "workflow-tool", Title: "Workflow Tool", Category: "Action",
			Description: "A tool for an AI step: a workflow, by its name, which the AI may run with run [input]: its Manual trigger (else its first) gets the input as its payload (JSON, or text), and the AI gets back how the run ended and what its steps logged. The workflow's description tells the AI what it is for. A workflow cannot be its own tool. Name is the tool's name for the AI (workflow); two tools need two names.",
			Outputs:     []*v1.Socket{sock("Tool", Tool)},
			Settings:    []*v1.Setting{{Name: "Workflow", Placeholder: "the workflow's name"}, {Name: "Name", Placeholder: "workflow"}},
		},
		run: workflowToolStep,
	},
	{
		StepKind: &v1.StepKind{
			Kind: "whatsapp-tool", Title: "WhatsApp Tool", Category: "Messengers", Group: "WhatsApp",
			Description: "A tool for an AI step: a WhatsApp account the AI may send with, to Chat only: send [text], and send_file [path or link, caption]. Name is the tool's name for the AI (whatsapp); two tools need two names.",
			Outputs:     []*v1.Socket{sock("Tool", Tool)},
			Settings:    []*v1.Setting{whatsappSetting(), {Name: "Name", Placeholder: "whatsapp"}},
			Inputs:      []*v1.Socket{field("Chat")},
		},
		run: func(c *call) error { return chatToolStep(c, "whatsapp") },
	},
	{
		StepKind: &v1.StepKind{
			Kind: "binance-tool", Title: "Binance Tool", Category: "Finance", Group: "Binance",
			Description: "A tool for an AI step: a Binance futures account the AI may read, never trade: indicators [symbol, intervals] (Binance Indicators), candles [symbol, interval, limit], portfolio, positions [symbol] and orders [symbol, open, history or stops]. The market data is the connection's network's. Name is the tool's name for the AI (binance); two tools need two names.",
			Outputs:     []*v1.Socket{sock("Tool", Tool)},
			Settings:    []*v1.Setting{binanceSetting(), {Name: "Name", Placeholder: "binance"}},
		},
		run: binanceToolStep,
	},
	{
		StepKind: &v1.StepKind{
			Kind: "social-tool", Title: "Social Tool", Category: "Social",
			Description: "A tool for an AI step: drafts on Medium or LinkedIn (Site), through San Remote, never published: draft [article, in markdown, its first # line the title]. Name is the tool's name for the AI (social); two tools need two names.",
			Outputs:     []*v1.Socket{sock("Tool", Tool)},
			Settings:    []*v1.Setting{connectionSetting(), {Name: "Site", Options: []string{"Medium", "LinkedIn"}}, {Name: "Name", Placeholder: "social"}},
		},
		run: socialToolStep,
	},
	{
		StepKind: &v1.StepKind{
			Kind: "file-tool", Title: "File Tool", Category: "Storage",
			Description: "A tool for an AI step: a storage's files, which the AI may read but not change: list [pattern] (*, **/*.md; the first 200) and read [path] (text, up to 12,000 characters). Paths are inside the storage's folder. Name is the tool's name for the AI (files); two tools need two names.",
			Outputs:     []*v1.Socket{sock("Tool", Tool)},
			Settings:    []*v1.Setting{{Name: "Storage", Storage: true}, {Name: "Name", Placeholder: "files"}},
		},
		run: fileToolStep,
	},
	{
		StepKind: &v1.StepKind{
			Kind: "http-tool", Title: "HTTP Tool", Category: "Action",
			Description: "A tool for an AI step: get [url], a web page or an API's answer, as text (a page without its markup), up to 12,000 characters; only URLs that start with Allowed when it is set. Name is the tool's name for the AI (web); two tools need two names.",
			Outputs:     []*v1.Socket{sock("Tool", Tool)},
			Settings:    []*v1.Setting{{Name: "Allowed", Placeholder: "https://api.example.com/: empty for any URL"}, {Name: "Name", Placeholder: "web"}},
		},
		run: httpToolStep,
	},
}

func workflowToolStep(c *call) error {
	name, err := c.toolNameSetting("workflow")
	if err != nil {
		return err
	}
	title := c.setting("Workflow")
	if title == "" {
		return fmt.Errorf("Workflow %w: give the name of the workflow the AI may run", errEmpty)
	}
	if c.env.FindWorkflow == nil {
		return errors.New("this run cannot reach workflows")
	}
	w, err := c.env.FindWorkflow(title)
	if err != nil {
		return err
	}
	if w.ID == c.env.Workflow && c.env.Workflow != 0 {
		return fmt.Errorf("workflow %s cannot be its own tool", w.Name)
	}
	return c.giveTool(map[string]any{"type": "workflow", "tool_name": name, "workflow": strconv.FormatUint(uint64(w.ID), 10), "title": w.Name, "about": w.Description})
}

// chatToolStep gives a messenger's tool: its connection and its one chat.
func chatToolStep(c *call, typ string) error {
	name, err := c.toolNameSetting(typ)
	if err != nil {
		return err
	}
	conn := c.setting("Connection")
	if conn == "" {
		return errors.New("Connection: pick one (Connections page)")
	}
	chat := strings.TrimSpace(c.text("Chat"))
	if chat == "" {
		return fmt.Errorf("Chat %w: give the chat the AI may write to", errEmpty)
	}
	return c.giveTool(map[string]any{"type": typ, "tool_name": name, "connection": conn, "chat": chat})
}

func binanceToolStep(c *call) error {
	name, err := c.toolNameSetting("binance")
	if err != nil {
		return err
	}
	if _, _, err := c.binance(); err != nil {
		return err
	}
	return c.giveTool(map[string]any{"type": "binance", "tool_name": name, "connection": c.setting("Connection")})
}

func socialToolStep(c *call) error {
	name, err := c.toolNameSetting("social")
	if err != nil {
		return err
	}
	if c.setting("Connection") == "" {
		return errors.New("Connection: pick a San Remote connection (Connections page)")
	}
	site := c.setting("Site")
	if site == "" {
		site = "Medium"
	}
	return c.giveTool(map[string]any{"type": "social", "tool_name": name, "connection": c.setting("Connection"), "site": site})
}

func fileToolStep(c *call) error {
	name, err := c.toolNameSetting("files")
	if err != nil {
		return err
	}
	st, err := c.storage()
	if err != nil {
		return err
	}
	return c.giveTool(map[string]any{"type": "files", "tool_name": name, "storage": strconv.FormatUint(uint64(st.ID), 10)})
}

func httpToolStep(c *call) error {
	name, err := c.toolNameSetting("web")
	if err != nil {
		return err
	}
	allowed := c.setting("Allowed")
	if allowed != "" && !strings.HasPrefix(allowed, "http://") && !strings.HasPrefix(allowed, "https://") {
		return fmt.Errorf("Allowed %q: give the start of the URLs, http:// or https://", allowed)
	}
	return c.giveTool(map[string]any{"type": "web", "tool_name": name, "allowed": allowed})
}

// toolOf makes the tool a tool step gave, by its type (fields, the step's
// object).
func (c *call) toolOf(typ string, f map[string]*structpb.Value) (shellTool, error) {
	get := func(k string) string { return f[k].GetStringValue() }
	switch typ {
	case "workflow":
		id, err := strconv.ParseUint(get("workflow"), 10, 32)
		if err != nil {
			return nil, err
		}
		return &workflowTool{id: uint32(id), title: get("title"), about: get("about")}, nil
	case "whatsapp":
		return &whatsappTool{connection: get("connection"), chat: get("chat")}, nil
	case "binance":
		return &binanceTool{connection: get("connection")}, nil
	case "social":
		return &socialTool{connection: get("connection"), site: get("site")}, nil
	case "files":
		id, err := strconv.ParseUint(get("storage"), 10, 32)
		if err != nil {
			return nil, err
		}
		if c.env.Storage == nil {
			return nil, errors.New("this run has no storages")
		}
		st, err := c.env.Storage(uint32(id))
		if err != nil {
			return nil, err
		}
		return &fileTool{name: st.Name, root: filepath.FromSlash(st.Abs)}, nil
	case "web":
		return &webTool{allowed: get("allowed")}, nil
	}
	return nil, fmt.Errorf("Tools: a tool of a kind this tower does not know (%q)", typ)
}

// runStep runs a step of kind with values (its settings and its field
// inputs) as part of c, and gives its outputs: a tool's command doing what
// the step does in a workflow. Its lines go to c's log.
func (c *call) runStep(kind string, values map[string]string) (map[string]proto.Message, error) {
	k := byKind[kind]
	if k == nil || k.run == nil {
		return nil, fmt.Errorf("no step %s", kind)
	}
	child := &call{ctx: c.ctx, env: c.env, step: &v1.Step{Id: c.step.GetId(), Kind: kind, Values: values},
		inputs: map[string]proto.Message{}, outputs: map[string]proto.Message{}}
	for _, s := range k.Inputs {
		if s.Field {
			child.inputs[s.Name] = stringValue(values[s.Name])
		}
	}
	err := k.run(child)
	for _, line := range child.logs {
		c.logf(v1.LogLevel_LOG_LEVEL_DEBUG, "%s: %s", k.Title, line)
	}
	return child.outputs, err
}

// outputText is a step's output as a tool's result: text as it is, data as
// JSON.
func outputText(m proto.Message) string {
	switch v := m.(type) {
	case nil:
		return ""
	case *wrapperspb.StringValue:
		return v.GetValue()
	case *structpb.Value:
		b, err := json.Marshal(v.AsInterface())
		if err != nil {
			return ""
		}
		return string(b)
	}
	b, _ := protojson.Marshal(m)
	return string(b)
}

// toolPath is a file a tool sends: a link or an absolute path as it is, a
// relative one in c's folder (a Remote Shell's storage).
func toolPath(c *call, p string) string {
	if p == "" || strings.HasPrefix(p, "http://") || strings.HasPrefix(p, "https://") || filepath.IsAbs(p) {
		return p
	}
	if dir, err := c.workDir(); err == nil {
		return filepath.Join(dir, filepath.FromSlash(p))
	}
	return p
}

// arg is args' i-th, trimmed, or "".
func arg(args []string, i int) string {
	if i < len(args) {
		return strings.TrimSpace(args[i])
	}
	return ""
}

// workflowTool is a Workflow Tool's workflow.
type workflowTool struct {
	id           uint32
	title, about string
}

func (w *workflowTool) describe(name string) string {
	about := ""
	if w.about != "" {
		about = " (" + w.about + ")"
	}
	return fmt.Sprintf(`  - %s: runs the workflow %q%s and gives back how the run ended and what its steps logged. Command: run, arguments [input]: what its trigger gets, JSON or text.`, name, w.title, about)
}

func (w *workflowTool) brief(name string) string {
	return fmt.Sprintf("%s (runs the workflow %q): run [input]", name, w.title)
}

func (w *workflowTool) run(c *call, command string, args []string) string {
	if command != "run" {
		return fmt.Sprintf("error: no command %q: use run", command)
	}
	if c.env.RunWorkflow == nil {
		return "error: this run cannot reach workflows"
	}
	r, err := c.env.RunWorkflow(c.ctx, w.id, []byte(strings.Join(args, "\n")))
	if r.ID == 0 && err != nil {
		return "error: " + err.Error()
	}
	var b strings.Builder
	status := strings.ToLower(strings.TrimPrefix(r.Status.String(), "RUN_STATUS_"))
	fmt.Fprintf(&b, "Run %d of %q: %s", r.ID, w.title, status)
	if r.FinishedAt != nil {
		fmt.Fprintf(&b, " in %s", r.FinishedAt.Sub(r.StartedAt).Round(100*time.Millisecond))
	}
	if r.Error != "" {
		fmt.Fprintf(&b, ": %s", r.Error)
	} else if err != nil {
		fmt.Fprintf(&b, ": %v", err)
	}
	b.WriteString("\n")
	for _, st := range r.Steps.GetSteps() {
		title := st.GetKind()
		if k := byKind[st.GetKind()]; k != nil {
			title = k.Title
		}
		for _, line := range st.GetLogs() {
			fmt.Fprintf(&b, "%s: %s\n", title, line)
		}
		if st.GetError() != "" {
			fmt.Fprintf(&b, "%s failed: %s\n", title, st.GetError())
		}
	}
	return b.String()
}

// whatsappTool is a WhatsApp Tool's account and its one chat.
type whatsappTool struct {
	connection, chat string
}

func (t *whatsappTool) describe(name string) string {
	return fmt.Sprintf(`  - %s: sends WhatsApp messages to the chat %s, and only there. Commands: send, arguments [text]; send_file, arguments [path or link, caption] (a relative path is in your working folder). Write there only what the user asked to be sent or told there.`, name, t.chat)
}

func (t *whatsappTool) brief(name string) string {
	return fmt.Sprintf("%s (WhatsApp, to the chat %s only): send [text], send_file [path, caption]", name, t.chat)
}

func (t *whatsappTool) run(c *call, command string, args []string) string {
	values := map[string]string{"Connection": t.connection, "Chat": t.chat}
	switch command {
	case "send":
		values["Text"] = strings.TrimSpace(strings.Join(args, "\n"))
		if values["Text"] == "" {
			return "error: send needs the text in arguments"
		}
	case "send_file":
		if values["File"] = toolPath(c, arg(args, 0)); values["File"] == "" {
			return "error: send_file needs a path or a link in arguments"
		}
		values["Text"] = strings.Join(args[min(1, len(args)):], "\n")
	default:
		return fmt.Sprintf("error: no command %q: use send or send_file", command)
	}
	out, err := c.runStep("whatsapp-send", values)
	if err != nil {
		return "error: " + err.Error()
	}
	return "sent to " + t.chat + ", message " + outputText(out["Message ID"])
}

// binanceTool is a Binance Tool's account, read only.
type binanceTool struct {
	connection string
}

func (t *binanceTool) describe(name string) string {
	return fmt.Sprintf(`  - %s: reads a Binance USD-M futures account, never trades. Commands:
    - indicators, arguments [symbol, intervals]: on each interval (15m,1h,4h by default) the close, EMA 20/50/200 and the trend they make, RSI 14, MACD, ATR 14 (also as %% of the price), Bollinger 20/2, volume against its average, the prior candle's high and low, the nearest swing highs and lows; and the 24h high and low. Start here.
    - candles, arguments [symbol, interval, limit]: the latest candles, oldest first (interval %s, 1h by default; limit 50 by default, up to 500).
    - portfolio: the account's totals, assets and open positions.
    - positions, arguments [symbol]: open positions, of the symbol or of all.
    - orders, arguments [symbol, open, history or stops]: orders that can still fill (open, the default), the symbol's latest of any status (history), or the stop losses and take profits waiting (stops).`, name, strings.Join(binance.Intervals, ", "))
}

func (t *binanceTool) brief(name string) string {
	return fmt.Sprintf("%s (a Binance account, read only): indicators [symbol, intervals], candles [symbol, interval, limit], portfolio, positions [symbol], orders [symbol, open|history|stops]", name)
}

func (t *binanceTool) run(c *call, command string, args []string) string {
	var kind string
	var values map[string]string
	var outputs []string
	switch command {
	case "indicators":
		out, err := c.runStep("binance-indicators", map[string]string{"Connection": t.connection, "Symbol": strings.ToUpper(arg(args, 0)), "Intervals": strings.Join(args[min(1, len(args)):], ",")})
		if err != nil {
			return "error: " + err.Error()
		}
		return outputText(out["Text"])
	case "candles":
		interval, limit := arg(args, 1), arg(args, 2)
		if interval == "" {
			interval = "1h"
		}
		if limit == "" {
			limit = "50"
		}
		if !slices.Contains(binance.Intervals, interval) {
			return fmt.Sprintf("error: interval %q: one of %s", interval, strings.Join(binance.Intervals, ", "))
		}
		if n, err := strconv.Atoi(limit); err != nil || n < 1 || n > 500 {
			return fmt.Sprintf("error: limit %q: a number from 1 to 500", limit)
		}
		kind, outputs = "binance-candles", []string{"Candles"}
		values = map[string]string{"Network": t.network(c), "Interval": interval, "Limit": limit, "Symbol": strings.ToUpper(arg(args, 0))}
	case "portfolio":
		kind, outputs = "binance-portfolio", []string{"Account", "Assets", "Positions"}
		values = map[string]string{"Connection": t.connection}
	case "positions":
		kind, outputs = "binance-positions", []string{"Positions"}
		values = map[string]string{"Connection": t.connection, "Symbol": strings.ToUpper(arg(args, 0))}
	case "orders":
		show := map[string]string{"": "Open", "open": "Open", "history": "History", "stops": "Stop Orders"}[strings.ToLower(arg(args, 1))]
		if show == "" {
			return fmt.Sprintf("error: %q: open, history or stops", arg(args, 1))
		}
		kind, outputs = "binance-orders", []string{"Orders"}
		values = map[string]string{"Connection": t.connection, "Symbol": strings.ToUpper(arg(args, 0)), "Show": show}
	default:
		return fmt.Sprintf("error: no command %q: use candles, portfolio, positions or orders", command)
	}
	out, err := c.runStep(kind, values)
	if err != nil {
		return "error: " + err.Error()
	}
	var b strings.Builder
	for _, name := range outputs {
		fmt.Fprintf(&b, "%s: %s\n", name, outputText(out[name]))
	}
	return b.String()
}

// network is the connection's network as Binance Candles names it: from
// this PC only the test network answers (fapi.binance.com times out).
func (t *binanceTool) network(c *call) string {
	id, err := strconv.ParseUint(t.connection, 10, 32)
	if err != nil || c.env.Connection == nil {
		return "Testnet"
	}
	conn, err := c.env.Connection(uint32(id))
	if err != nil {
		return "Testnet"
	}
	switch conn.Settings.GetBinance().GetNetwork() {
	case store.BinanceLive:
		return "Live"
	case store.BinanceDemo:
		return "Demo"
	}
	return "Testnet"
}

// socialTool is a Social Tool's site, where it only makes drafts.
type socialTool struct {
	connection, site string
}

func (t *socialTool) describe(name string) string {
	return fmt.Sprintf(`  - %s: makes a draft on %s, never published: the user reviews and publishes it. Command: draft, arguments [article]: in markdown, its first "# " line the title; images need absolute paths or links.`, name, t.site)
}

func (t *socialTool) brief(name string) string {
	return fmt.Sprintf("%s (drafts on %s, never published): draft [markdown article]", name, t.site)
}

func (t *socialTool) run(c *call, command string, args []string) string {
	if command != "draft" {
		return fmt.Sprintf("error: no command %q: use draft", command)
	}
	article := strings.TrimSpace(strings.Join(args, "\n"))
	if article == "" {
		return "error: draft needs the article in arguments"
	}
	kind, output := "medium-draft", "Story"
	if t.site == "LinkedIn" {
		kind, output = "linkedin-draft", "Article"
	}
	out, err := c.runStep(kind, map[string]string{"Connection": t.connection, "Source": "Text", "Article": article})
	if err != nil {
		return "error: " + err.Error()
	}
	return fmt.Sprintf("draft made on %s: %s\n%s", t.site, outputText(out["Link"]), outputText(out[output]))
}

// fileTool is a File Tool's storage, read only.
type fileTool struct {
	name, root string
}

func (t *fileTool) describe(name string) string {
	return fmt.Sprintf(`  - %s: the files of the storage %q, read only, paths relative to it. Commands: list, arguments [pattern] (* for a folder's files, **/*.md for every markdown file; the first 200); read, arguments [path] (text, up to 12,000 characters).`, name, t.name)
}

func (t *fileTool) brief(name string) string {
	return fmt.Sprintf("%s (the storage %q, read only): list [pattern], read [path]", name, t.name)
}

// toolTextMax is how much of a file or a page a tool gives, in characters.
const toolTextMax = 12000

func (t *fileTool) run(_ *call, command string, args []string) string {
	switch command {
	case "list":
		pattern := arg(args, 0)
		if pattern == "" {
			pattern = "*"
		}
		if strings.Contains(pattern, "..") || filepath.IsAbs(pattern) {
			return "error: a pattern is relative to the storage, without .."
		}
		files, err := globFiles(filepath.ToSlash(t.root) + "/" + pattern)
		if err != nil {
			return "error: " + err.Error()
		}
		var b strings.Builder
		fmt.Fprintf(&b, "%d files match %s", len(files), pattern)
		if len(files) > 200 {
			b.WriteString(", the first 200")
			files = files[:200]
		}
		b.WriteString(":\n")
		for _, f := range files {
			rel, _ := filepath.Rel(t.root, filepath.FromSlash(f))
			fmt.Fprintf(&b, "%s\n", filepath.ToSlash(rel))
		}
		return b.String()
	case "read":
		p := arg(args, 0)
		if p == "" {
			return "error: read needs a path in arguments"
		}
		full := filepath.Join(t.root, filepath.FromSlash(p))
		if rel, err := filepath.Rel(t.root, full); err != nil || strings.HasPrefix(rel, "..") {
			return "error: the path is outside the storage"
		}
		data, err := os.ReadFile(full)
		if err != nil {
			return "error: " + err.Error()
		}
		text := strings.ToValidUTF8(string(data), "\uFFFD")
		if r := []rune(text); len(r) > toolTextMax {
			text = string(r[:toolTextMax]) + fmt.Sprintf("\n(cut: %d characters in all)", len(r))
		}
		return text
	}
	return fmt.Sprintf("error: no command %q: use list or read", command)
}

// webTool is an HTTP Tool: GET only, within allowed when it is set.
type webTool struct {
	allowed string
}

func (t *webTool) describe(name string) string {
	where := "any URL"
	if t.allowed != "" {
		where = "URLs that start with " + t.allowed
	}
	return fmt.Sprintf(`  - %s: gets a web page or an API's answer, %s. Command: get, arguments [url]: the status and the text (a page without its markup), up to 12,000 characters.`, name, where)
}

func (t *webTool) brief(name string) string {
	return fmt.Sprintf("%s (gets web pages): get [url]", name)
}

var (
	// htmlDrop are a page's parts that are no text.
	htmlDrop = regexp.MustCompile(`(?is)<(script|style|noscript|svg|head)\b.*?</(script|style|noscript|svg|head)>`)
	htmlTag  = regexp.MustCompile(`(?s)<[^>]*>`)
	// htmlBlock are the tags that end a line of a page's text.
	htmlBlock = regexp.MustCompile(`(?i)</?(p|div|h[1-6]|br|li|tr|ul|ol|table|section|article|header|footer|nav|main|blockquote|pre)\b[^>]*>`)
)

// webTimeout bounds a get.
const webTimeout = 30 * time.Second

func (t *webTool) run(c *call, command string, args []string) string {
	if command != "get" {
		return fmt.Sprintf("error: no command %q: use get", command)
	}
	url := arg(args, 0)
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return "error: get needs an http:// or https:// URL in arguments"
	}
	if t.allowed != "" && !strings.HasPrefix(url, t.allowed) {
		return "error: only URLs that start with " + t.allowed
	}
	ctx, cancel := context.WithTimeout(c.ctx, webTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "error: " + err.Error()
	}
	req.Header.Set("User-Agent", "san_tower")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "error: " + err.Error()
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		return "error: " + err.Error()
	}
	text := strings.ToValidUTF8(string(body), "\uFFFD")
	kind := res.Header.Get("Content-Type")
	if strings.Contains(kind, "html") {
		// A line per block, each without its extra spaces.
		text = htmlTag.ReplaceAllString(htmlBlock.ReplaceAllString(htmlDrop.ReplaceAllString(text, ""), "\n"), "")
		var lines []string
		for _, l := range strings.Split(html.UnescapeString(text), "\n") {
			if l = strings.Join(strings.Fields(l), " "); l != "" {
				lines = append(lines, l)
			}
		}
		text = strings.Join(lines, "\n")
	}
	if r := []rune(text); len(r) > toolTextMax {
		text = string(r[:toolTextMax]) + fmt.Sprintf("\n(cut: %d characters in all)", len(r))
	}
	return fmt.Sprintf("HTTP %d, %s:\n%s", res.StatusCode, kind, text)
}

// telegramMore are the Telegram Tool's commands besides send: a file or a
// photo, and the chat's latest messages.
func (t *telegramTool) more(c *call, command string, args []string) (string, bool) {
	switch command {
	case "send_file", "send_photo":
		file := toolPath(c, arg(args, 0))
		if file == "" {
			return "error: " + command + " needs a path or a link in arguments", true
		}
		media := map[string]string{"send_file": "Document", "send_photo": "Photo"}[command]
		_, err := c.runStep("telegram-send", map[string]string{"Connection": t.connection, "Chat": t.chat, "Media": media,
			"File": file, "Text": strings.Join(args[1:], "\n"), "Format": "Plain"})
		if err != nil {
			return "error: " + err.Error(), true
		}
		return "sent to " + t.chat, true
	case "read":
		limit := 10
		if s := arg(args, 0); s != "" {
			n, err := strconv.Atoi(s)
			if err != nil || n < 1 || n > 50 {
				return "error: read's argument is how many messages, 1 to 50", true
			}
			limit = n
		}
		id, err := strconv.ParseUint(t.connection, 10, 32)
		if err != nil || c.env.TelegramMessages == nil {
			return "error: this run cannot read Telegram messages", true
		}
		msgs, err := c.env.TelegramMessages(uint32(id), t.chat, limit)
		if err != nil {
			return "error: " + err.Error(), true
		}
		return telegramReadText(t.chat, msgs), true
	}
	return "", false
}

// telegramReadText is a chat's latest messages for a tool's result, oldest
// first.
func telegramReadText(chat string, msgs []store.TelegramMessage) string {
	if len(msgs) == 0 {
		return "No messages from " + chat + " yet (the bot keeps those it got since the tower watched it)."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "The latest %d messages of %s, oldest first:\n", len(msgs), chat)
	for i := len(msgs) - 1; i >= 0; i-- {
		var m struct {
			From struct {
				FirstName string `json:"first_name"`
				Username  string `json:"username"`
			} `json:"from"`
			Text    string `json:"text"`
			Caption string `json:"caption"`
		}
		_ = json.Unmarshal([]byte(msgs[i].Data), &m)
		who := m.From.FirstName
		if m.From.Username != "" {
			who = strings.TrimSpace(who + " @" + m.From.Username)
		}
		text := m.Text
		if text == "" {
			text = m.Caption
		}
		if text == "" {
			text = "(no text)"
		}
		fmt.Fprintf(&b, "%s %s: %s\n", msgs[i].Date.Format("2006-01-02 15:04"), who, text)
	}
	return b.String()
}
