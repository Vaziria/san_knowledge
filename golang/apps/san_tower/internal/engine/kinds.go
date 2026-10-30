package engine

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/store"
)

// Socket types. A link joins sockets of the same type; an event link says
// which step runs next, the others carry data.
const (
	Event  = "event"
	Object = "object" // a google.protobuf.Value
	Text   = "text"   // a google.protobuf.StringValue
	// Knowledge is a knowledge of the Knowledge page, a StringValue of its
	// id: Knowledge Base's output, which the knowledge steps look in
	// (knowledge.go).
	Knowledge = "knowledge"
	// Tool is a tool an AI step may use, a Value object a tool step gives
	// (Knowledge Tool, Telegram Tool; tools.go). An input of this type takes
	// any number of links, and gets their tools as a list.
	Tool = "tool"
)

// TopicEvent is the kind of the step that starts a run when an event comes
// on a topic of the tower's broker.
const TopicEvent = "topic-event"

// Manual is the kind of the trigger that starts a run only by hand (the
// user, 2026-10-27: "What if I need have workflow, but I just run it
// manually"): the editor's Run, or RunWorkflow. Nothing else fires it, so
// the engine's triggers leave it out, switched on or not.
const Manual = "manual"

// kind is a kind of step and how it runs. A trigger starts runs; a step with
// run is run when an event link reaches it, or, without event inputs (a
// "pure" step, such as Template), when a step needs one of its outputs.
type kind struct {
	*v1.StepKind
	trigger func(st *v1.Step, data []byte) (outputs map[string]proto.Message, fired []string, err error)
	run     func(c *call) error
	// back are a loop step's inputs that come back from a step in its own
	// loop (Knowledge Summarize's Answer, from the AI step its Each runs):
	// not read before the step runs, but by call.back after each item.
	back []string
}

func (k *kind) pure() bool {
	if k.trigger != nil {
		return false
	}
	for _, s := range k.Inputs {
		if s.Type == Event {
			return false
		}
	}
	return true
}

func sock(name, typ string) *v1.Socket { return &v1.Socket{Name: name, Type: typ} }

// kinds are the kinds of step, in the order the editor's Add menu shows them:
// the core ones, Log Event (logevent.go), Tower Restart (towerrestart.go),
// the datetime ones (datetime.go), the san_remote ones (social.go, then
// sites.go), then the Tower Chat ones (towerchat.go), the Telegram ones
// (telegram.go), the WhatsApp ones (whatsapp.go) and the knowledge ones
// (knowledge.go).
var kinds = slices.Concat(coreKinds, progressKinds, watchKinds, logKinds, towerKinds, datetimeKinds, socialKinds, siteKinds, claudeKinds, browserChatKinds, remoteShellKinds, towerChatKinds, telegramKinds, telegramWebKinds, whatsappKinds, financeKinds, bashKinds, knowledgeKinds, toolKinds, moreToolKinds, indicatorKinds, tradingKinds)

var coreKinds = []*kind{
	{
		StepKind: &v1.StepKind{
			Kind: Manual, Title: "Manual", Category: "Trigger",
			Description: "Starts the workflow only by hand: the editor's Run button, never by itself, switched on or not; Payload is the data typed in Run, JSON as it is or text.",
			Outputs:     []*v1.Socket{sock("Event", Event), sock("Payload", Object)},
		},
		trigger: func(_ *v1.Step, data []byte) (map[string]proto.Message, []string, error) {
			return map[string]proto.Message{"Payload": payload(data)}, []string{"Event"}, nil
		},
	},
	{
		StepKind: &v1.StepKind{
			Kind: TopicEvent, Title: "Topic Event", Category: "Trigger",
			Description: "Starts the workflow on each event published to a topic of the message broker; Payload is the event's data.",
			Outputs:     []*v1.Socket{sock("Event", Event), sock("Payload", Object)},
			Settings:    []*v1.Setting{{Name: "Topic", Placeholder: "orders"}},
		},
		trigger: func(_ *v1.Step, data []byte) (map[string]proto.Message, []string, error) {
			return map[string]proto.Message{"Payload": payload(data)}, []string{"Event"}, nil
		},
	},
	{
		StepKind: &v1.StepKind{
			Kind: Schedule, Title: "Schedule", Category: "Trigger",
			Description: "Starts the workflow on a schedule: every so often (\"5m\", \"1d\") or by a cron line (\"0 9 * * *\").",
			Outputs:     []*v1.Socket{sock("Event", Event), sock("Payload", Object)},
			Settings:    []*v1.Setting{{Name: "Every", Placeholder: "5m, 1d or 0 9 * * *"}},
		},
		trigger: scheduleFired,
	},
	{
		StepKind: &v1.StepKind{
			Kind: "filter", Title: "Filter", Category: "Logic",
			Description: "Goes on by Match when its If condition on the payload holds (\"payload.total > 100\"), else by Else.",
			Outputs:     []*v1.Socket{sock("Match", Event), sock("Else", Event)},
			Settings:    []*v1.Setting{{Name: "If", Placeholder: "payload.total > 100"}},
			Inputs:      []*v1.Socket{sock("Event", Event), sock("Payload", Object)},
		},
		run: func(c *call) error {
			ok, err := evalBool(c.setting("If"), c.object("Payload"))
			if err != nil {
				return err
			}
			c.branch(ok)
			return nil
		},
	},
	{
		StepKind: &v1.StepKind{
			Kind: "prefix-filter", Title: "Prefix Filter", Category: "Logic",
			Description: "Goes on by Match when the text starts with Prefix (\"/start\"), else by Else.",
			Outputs:     []*v1.Socket{sock("Match", Event), sock("Else", Event)},
			Settings:    []*v1.Setting{{Name: "Prefix", Placeholder: "/start"}},
			Inputs:      []*v1.Socket{sock("Event", Event), sock("Text", Text)},
		},
		run: func(c *call) error {
			// As typed: a space after the prefix can matter ("/say ").
			prefix := c.step.GetValues()["Prefix"]
			if strings.TrimSpace(prefix) == "" {
				return fmt.Errorf("Prefix %w", errEmpty)
			}
			c.branch(strings.HasPrefix(c.text("Text"), prefix))
			return nil
		},
	},
	{
		StepKind: &v1.StepKind{
			Kind: "regex-filter", Title: "Regex Filter", Category: "Logic",
			Description: "Goes on by Match when the text matches Pattern, a Go regular expression (\"^/(start|help)\\b\"), else by Else.",
			Outputs:     []*v1.Socket{sock("Match", Event), sock("Else", Event)},
			Settings:    []*v1.Setting{{Name: "Pattern", Placeholder: `^/(start|help)\b`}},
			Inputs:      []*v1.Socket{sock("Event", Event), sock("Text", Text)},
		},
		run: func(c *call) error {
			pattern := c.setting("Pattern")
			if pattern == "" {
				return fmt.Errorf("Pattern %w", errEmpty)
			}
			re, err := regexp.Compile(pattern)
			if err != nil {
				return fmt.Errorf("Pattern %q: %v", pattern, strings.TrimPrefix(err.Error(), "error parsing regexp: "))
			}
			c.branch(re.MatchString(c.text("Text")))
			return nil
		},
	},
	{
		StepKind: &v1.StepKind{
			Kind: "pick-greater", Title: "Pick Greater", Category: "Logic",
			Description: "Gives the greater of A and B as it came: two numbers (3 and 10 give 10) or two times, the later one (Datetime, Date or Time). An empty one gives the other.",
			Outputs:     []*v1.Socket{sock("Value", Text)},
			Inputs:      []*v1.Socket{field("A"), field("B")},
		},
		run: pickGreater,
	},
	{
		StepKind: &v1.StepKind{
			Kind: "template", Title: "Template", Category: "Data",
			Description: "Makes text from the payload, filling in each {{ expression }}, for a message, a post or a log line.",
			Outputs:     []*v1.Socket{sock("Text", Text)},
			Settings:    []*v1.Setting{{Name: "Text", Placeholder: "Order {{payload.id}}"}},
			Inputs:      []*v1.Socket{sock("Payload", Object)},
		},
		run: func(c *call) error {
			s, err := interpolate(c.setting("Text"), c.object("Payload"))
			if err != nil {
				return err
			}
			c.outputs["Text"] = wrapperspb.String(s)
			return nil
		},
	},
	{
		StepKind: &v1.StepKind{
			Kind: "format-text", Title: "Format Text", Category: "Data",
			Description: "Puts its Text into Format, where {{ text }} stands for it: pesan \"{{ text }}\".",
			Outputs:     []*v1.Socket{sock("Text", Text)},
			Settings:    []*v1.Setting{{Name: "Format", Placeholder: `Pesan: "{{ text }}"`}},
			Inputs:      []*v1.Socket{sock("Text", Text)},
		},
		run: func(c *call) error {
			f := c.setting("Format")
			if f == "" {
				return fmt.Errorf("Format %w", errEmpty)
			}
			s, err := interpolateVars(f, map[string]any{"text": c.text("Text")})
			if err != nil {
				return err
			}
			c.outputs["Text"] = wrapperspb.String(s)
			return nil
		},
	},
	{
		StepKind: &v1.StepKind{
			Kind: "storage", Title: "Storage", Category: "Storage",
			Description: "A storage from the Storage page: Path as set there, and Absolute Path, where it is on this computer (a relative path is in the tower's data folder).",
			Outputs:     []*v1.Socket{sock("Path", Text), sock("Absolute Path", Text)},
			Settings:    []*v1.Setting{{Name: "Storage", Storage: true}},
		},
		run: func(c *call) error {
			st, err := c.storage()
			if err != nil {
				return err
			}
			c.outputs["Path"] = stringValue(st.Path)
			c.outputs["Absolute Path"] = stringValue(st.Abs)
			return nil
		},
	},
	{
		StepKind: &v1.StepKind{
			Kind: "file-path", Title: "File Path", Category: "Storage",
			Description: "A file's path from its File Name, in a storage when one is picked, else in the tower's data folder: Path, and Absolute Path.",
			Outputs:     []*v1.Socket{sock("Path", Text), sock("Absolute Path", Text)},
			Settings:    []*v1.Setting{{Name: "Storage", Storage: true, Placeholder: "No storage: the data folder"}},
			Inputs:      []*v1.Socket{{Name: "File Name", Type: Text, Field: true}},
		},
		run: filePath,
	},
	{
		StepKind: &v1.StepKind{
			Kind: "read-file", Title: "Read File", Category: "Storage",
			Description: "Reads the text of the file at Path (UTF-8, up to 10 MB), in a storage when one is picked, else in the tower's data folder or an absolute path: Text.",
			Outputs:     []*v1.Socket{sock("Text", Text)},
			Settings:    []*v1.Setting{{Name: "Storage", Storage: true, Placeholder: "No storage: the data folder"}},
			Inputs:      []*v1.Socket{field("Path")},
		},
		run: readFile,
	},
	{
		StepKind: &v1.StepKind{
			Kind: "glob", Title: "Glob", Category: "Storage",
			Description: "Finds the files matching Pattern (articles/*/*.md, ** for any folders) in a storage, the data folder or an absolute folder: Each fires once per file with Item its path, then Done; Files, First and Count.",
			Outputs:     []*v1.Socket{sock("Done", Event), sock("Each", Event), sock("Item", Text), sock("Files", Object), sock("First", Text), sock("Count", Text)},
			Settings:    []*v1.Setting{{Name: "Storage", Storage: true, Placeholder: "No storage: the data folder"}},
			Inputs:      []*v1.Socket{sock("Event", Event), field("Pattern")},
		},
		run: glob,
	},
	{
		StepKind: &v1.StepKind{
			Kind: "download-file", Title: "Download File", Category: "Action",
			Description: "Downloads URL to Save To, a file or a folder (then named from the URL) in a storage, the data folder or an absolute path; File is where it went.",
			Outputs:     []*v1.Socket{sock("Done", Event), sock("File", Text)},
			Settings:    []*v1.Setting{{Name: "Storage", Storage: true, Placeholder: "No storage: the data folder"}},
			Inputs:      []*v1.Socket{sock("Event", Event), field("URL"), field("Save To")},
		},
		run: downloadFile,
	},
	{
		StepKind: &v1.StepKind{
			Kind: "cut-video", Title: "Cut Video", Category: "Action",
			Description: "Cuts the part of the video File from Start to End, in seconds (no End: to its end), with ffmpeg, exact to the frame; saved at Save To (a file or a folder) or beside File. File is the cut video.",
			Outputs:     []*v1.Socket{sock("Done", Event), sock("File", Text)},
			Settings:    []*v1.Setting{{Name: "Storage", Storage: true, Placeholder: "No storage: the data folder"}},
			Inputs:      []*v1.Socket{sock("Event", Event), field("File"), field("Start"), field("End"), field("Save To")},
		},
		run: cutVideo,
	},
	{
		StepKind: &v1.StepKind{
			Kind: "crop-video", Title: "Crop Video", Category: "Action",
			Description: "Crops the picture of the video File with ffmpeg: to the largest middle part of Aspect 9:16, 1:1, 4:5 or 16:9, or, with Custom, to Width x Height pixels at X, Y (empty: the middle); saved at Save To (a file or a folder) or beside File. File is the cropped video.",
			Outputs:     []*v1.Socket{sock("Done", Event), sock("File", Text)},
			Settings: []*v1.Setting{{Name: "Aspect", Options: cropAspects},
				{Name: "Storage", Storage: true, Placeholder: "No storage: the data folder"}},
			Inputs: []*v1.Socket{sock("Event", Event), field("File"),
				only("Aspect=Custom", field("Width")), only("Aspect=Custom", field("Height")),
				only("Aspect=Custom", field("X")), only("Aspect=Custom", field("Y")), field("Save To")},
		},
		run: cropVideo,
	},
	{
		StepKind: &v1.StepKind{
			Kind: "delete-file", Title: "Delete File", Category: "Action",
			Description: "Deletes the file at Path, such as File Path's Absolute Path (a relative path is in the tower's data folder); one not there counts as done, a folder is refused.",
			Outputs:     []*v1.Socket{sock("Done", Event)},
			Inputs:      []*v1.Socket{sock("Event", Event), {Name: "Path", Type: Text, Field: true}},
		},
		run: deleteFile,
	},
	{
		StepKind: &v1.StepKind{
			Kind: "delete-folder", Title: "Delete Folder", Category: "Action",
			Description: "Deletes the folder at Path with everything in it, in a storage when one is picked, else in the tower's data folder or an absolute path; one not there counts as done. A drive's root, the data folder and the storage itself are refused.",
			Outputs:     []*v1.Socket{sock("Done", Event)},
			Settings:    []*v1.Setting{{Name: "Storage", Storage: true, Placeholder: "No storage: the data folder"}},
			Inputs:      []*v1.Socket{sock("Event", Event), field("Path")},
		},
		run: deleteFolder,
	},
	{
		StepKind: &v1.StepKind{
			Kind: "uuid", Title: "UUID", Category: "Data",
			Description: "Makes a new random UUID (version 4), such as 3f2b8c1e-9d4a-4f6b-8e2a-1c5d7b9e0f3a: a new one in each run.",
			Outputs:     []*v1.Socket{sock("UUID", Text)},
		},
		run: func(c *call) error {
			c.outputs["UUID"] = stringValue(newUUID())
			return nil
		},
	},
	{
		StepKind: &v1.StepKind{
			Kind: "md5", Title: "MD5 Hash", Category: "Data",
			Description: "Hashes its Text with MD5: Hash is 32 hexadecimal digits. For checksums and keys, not for passwords.",
			Outputs:     []*v1.Socket{sock("Hash", Text)},
			Inputs:      []*v1.Socket{{Name: "Text", Type: Text, Field: true}},
		},
		run: func(c *call) error {
			sum := md5.Sum([]byte(c.text("Text")))
			c.outputs["Hash"] = stringValue(hex.EncodeToString(sum[:]))
			return nil
		},
	},
	{
		StepKind: &v1.StepKind{
			Kind: "stringify", Title: "Stringify", Category: "Data",
			Description: "Writes its Object as JSON text, compact on one line or pretty on several, e.g. an HTTP Request's Response for a message.",
			Outputs:     []*v1.Socket{sock("Text", Text)},
			Settings:    []*v1.Setting{{Name: "Format", Options: []string{"Compact", "Pretty"}}},
			Inputs:      []*v1.Socket{sock("Object", Object)},
		},
		run: func(c *call) error {
			s, err := stringify(c.object("Object"), c.setting("Format") == "Pretty")
			if err != nil {
				return err
			}
			c.outputs["Text"] = stringValue(s)
			return nil
		},
	},
	{
		StepKind: &v1.StepKind{
			Kind: "http-request", Title: "HTTP Request", Category: "Action",
			Description: "Calls a URL with GET, POST, PUT or DELETE, sending Body as JSON; Response is the answer.",
			Outputs:     []*v1.Socket{sock("Done", Event), sock("Response", Object)},
			Settings: []*v1.Setting{
				{Name: "Method", Options: []string{"POST", "GET", "PUT", "DELETE"}},
				{Name: "URL", Placeholder: "https://example.com/orders/{{ payload.id }}"},
			},
			Inputs: []*v1.Socket{sock("Event", Event), sock("Body", Object)},
		},
		run: httpRequest,
	},
	{
		StepKind: &v1.StepKind{
			Kind: "publish", Title: "Publish to Topic", Category: "Action",
			Description: "Publishes Payload as an event on a topic of the message broker, for apps or other workflows.",
			Outputs:     []*v1.Socket{sock("Done", Event), sock("Result", Object)},
			Settings:    []*v1.Setting{{Name: "Topic", Placeholder: "orders"}},
			Inputs:      []*v1.Socket{sock("Event", Event), sock("Payload", Object)},
		},
		run: publish,
	},
	{
		StepKind: &v1.StepKind{
			Kind: "log", Title: "Log", Category: "Action",
			Description: "Writes Message to the run's log as a line of its Type, to see what a run did.",
			Settings:    []*v1.Setting{{Name: "Type", Options: []string{"info", "success", "warning", "failed", "debug"}}},
			Inputs:      []*v1.Socket{sock("Event", Event), {Name: "Message", Type: Text, Field: true}},
		},
		run: func(c *call) error {
			level := v1.LogLevel(v1.LogLevel_value["LOG_LEVEL_"+strings.ToUpper(c.setting("Type"))])
			if level == v1.LogLevel_LOG_LEVEL_UNSPECIFIED {
				level = v1.LogLevel_LOG_LEVEL_INFO // a step saved before Log had a Type
			}
			c.logs = append(c.logs, c.text("Message"))
			c.levels = append(c.levels, level)
			return nil
		},
	},
	{
		// The user, 2026-10-27: a Failed branch that cleans up (deletes a
		// download) would otherwise turn a failed run into a succeeded one.
		StepKind: &v1.StepKind{
			Kind: "fail", Title: "Fail Run", Category: "Action",
			Description: "Ends the run as failed, with Message, or when it is empty with the error of the last step whose Failed output fired: put it at the end of a Failed branch that cleans up, so the run still counts as failed.",
			Inputs:      []*v1.Socket{sock("Event", Event), {Name: "Message", Type: Text, Field: true}},
		},
		run: func(c *call) error {
			switch msg := c.text("Message"); {
			case strings.TrimSpace(msg) != "":
				return errors.New(msg)
			case c.failed != nil:
				return c.failed
			}
			return errors.New("the run failed here")
		},
	},
	{
		// The user, 2026-10-27: "And we add ack node" (queue.go).
		StepKind: &v1.StepKind{
			Kind: "ack", Title: "Ack", Category: "Action",
			Description: "In a one-at-a-time workflow, tells its queue in the message broker that this run is done with now: a tower that stops after it does not run it again. Without it a run is acked when it ends, succeeded or failed. Put it after the step that must not happen twice (an X Post). The next run still waits for this one to end; outside a queued run it does nothing.",
			Outputs:     []*v1.Socket{sock("Done", Event)},
			Inputs:      []*v1.Socket{sock("Event", Event)},
		},
		run: func(c *call) error {
			switch {
			case c.env.Ack == nil:
				c.logs = append(c.logs, "not a queued run (one run at a time is off, or it ran by itself): nothing to ack")
				c.levels = append(c.levels, v1.LogLevel_LOG_LEVEL_INFO)
			case c.env.Ack() != nil:
				c.logs = append(c.logs, "the queue did not take the ack now; the run is acked when it ends")
				c.levels = append(c.levels, v1.LogLevel_LOG_LEVEL_WARNING)
			default:
				c.logs = append(c.logs, "acked: a tower that stops now does not run it again")
				c.levels = append(c.levels, v1.LogLevel_LOG_LEVEL_INFO)
			}
			c.fire(Done)
			return nil
		},
	},
}

// Done is the event output of a step that does something. Such a step also
// gets In Progress before it, Failed after it (event outputs), and Error, a
// text output, last (execution.call):
//   - In Progress fires as the step starts: the steps it links to run first,
//     then the step does its work.
//   - When the step fails and its Failed output is linked, Failed fires
//     instead of Done, with Error the error, and the run goes on, so a
//     workflow can handle the failure.
const (
	InProgress  = "In Progress"
	Done        = "Done"
	Failed      = "Failed"
	ErrorOutput = "Error"
)

var byKind = func() map[string]*kind {
	m := make(map[string]*kind, len(kinds))
	for _, k := range kinds {
		k.Runnable = k.trigger != nil || k.run != nil
		k.Trigger = k.trigger != nil
		if i := k.output(Done, Event); i >= 0 {
			k.Outputs = slices.Insert(k.Outputs, i, sock(InProgress, Event))
			k.Outputs = slices.Insert(k.Outputs, i+2, sock(Failed, Event))
			k.Outputs = append(k.Outputs, sock(ErrorOutput, Text))
		}
		m[k.Kind] = k
	}
	return m
}()

// output is the index of the kind's output socket name of type typ, or -1.
func (k *kind) output(name, typ string) int {
	return slices.IndexFunc(k.Outputs, func(s *v1.Socket) bool { return s.Name == name && s.Type == typ })
}

// Kinds lists the kinds of step, in the order the editor's Add menu shows
// them.
func Kinds() []*v1.StepKind {
	out := make([]*v1.StepKind, len(kinds))
	for i, k := range kinds {
		out[i] = k.StepKind
	}
	return out
}

// IsTrigger says whether steps of a kind start runs.
func IsTrigger(kindName string) bool {
	k := byKind[kindName]
	return k != nil && k.trigger != nil
}

// payload reads an event's data: JSON as it is, other text as a string, and
// nothing as null.
func payload(data []byte) *structpb.Value {
	if len(bytes.TrimSpace(data)) == 0 {
		return structpb.NewNullValue()
	}
	var v structpb.Value
	if err := protojson.Unmarshal(data, &v); err == nil {
		return &v
	}
	return structpb.NewStringValue(string(data))
}

// call is one step being run: its inputs, settings, and what it gives back.
// ctx is the run's: it ends when the tower stops.
type call struct {
	ctx     context.Context
	env     Env
	step    *v1.Step
	inputs  map[string]proto.Message
	outputs map[string]proto.Message
	fired   []string
	logs    []string
	levels  []v1.LogLevel // the type of each line in logs
	// each runs what an event output links to, to the end of its branch,
	// with the step's outputs as they are now: once per item of a loop
	// (Glob's Each). Nil outside a run.
	each func(output string) error
	// linked says whether an output of the step links to a step. Nil
	// outside a run.
	linked func(output string) bool
	// inLinked says whether an input of the step has a link, and back
	// reads one of the kind's back inputs, as the item each just ran left
	// it: nil when it has no link. Nil outside a run.
	inLinked func(input string) bool
	back     func(input string) (proto.Message, error)
	// failed is the error of the run's last step whose Failed output fired
	// (Fail Run).
	failed error
	// logMu guards logs and levels for log, which several goroutines may
	// call; dropped counts the earliest lines log let go.
	logMu   sync.Mutex
	dropped int
}

func (c *call) setting(name string) string { return strings.TrimSpace(c.step.GetValues()[name]) }

func (c *call) fire(output string) { c.fired = append(c.fired, output) }

// branch fires Match when ok, else Else, for the filters.
func (c *call) branch(ok bool) {
	if ok {
		c.fire("Match")
	} else {
		c.fire("Else")
	}
}

// object is an object input's value, null when nothing came.
func (c *call) object(input string) *structpb.Value {
	if v, ok := c.inputs[input].(*structpb.Value); ok {
		return v
	}
	return structpb.NewNullValue()
}

// text is a text input's value, empty when nothing came.
func (c *call) text(input string) string {
	if v, ok := c.inputs[input].(*wrapperspb.StringValue); ok {
		return v.GetValue()
	}
	return ""
}

// storage is the storage the step's Storage setting names.
func (c *call) storage() (store.Storage, error) {
	id := c.setting("Storage")
	if id == "" {
		return store.Storage{}, errors.New("Storage: pick a storage (Storage page)")
	}
	n, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		return store.Storage{}, fmt.Errorf("Storage %q is not a storage's id", id)
	}
	if c.env.Storage == nil {
		return store.Storage{}, errors.New("this run has no storages")
	}
	st, err := c.env.Storage(uint32(n))
	if errors.Is(err, store.ErrNotFound) {
		return st, fmt.Errorf("storage %d is gone: pick another", n)
	}
	return st, err
}

// stringify writes v as JSON: keys sorted, whole numbers without a decimal
// point, and < > & as they are (for messages, not for HTML); pretty indents
// it by two spaces. Nothing linked is null.
func stringify(v *structpb.Value, pretty bool) (string, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if pretty {
		enc.SetIndent("", "  ")
	}
	if err := enc.Encode(v.AsInterface()); err != nil {
		return "", err
	}
	return strings.TrimSuffix(b.String(), "\n"), nil
}

// newUUID is a random UUID, version 4 (RFC 9562).
func newUUID() string {
	var b [16]byte
	rand.Read(b[:])
	b[6] = b[6]&0x0f | 0x40 // version 4
	b[8] = b[8]&0x3f | 0x80 // variant 10
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

var errEmpty = errors.New("is empty")

func stringValue(s string) *wrapperspb.StringValue { return wrapperspb.String(s) }
