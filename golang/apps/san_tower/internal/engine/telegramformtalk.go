package engine

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/telegram"
)

// A Telegram Form's talk (telegramform.go has the kinds and the routing):
// it asks its questions in order, checks each answer and asks again with a
// short reason when it is wrong, then shows them for Kirim, Ubah or Batal.

// What the form says, in Indonesian.
const (
	txtSummary   = "Cek dulu ya:"
	txtWhich     = "Mau ubah yang mana?"
	txtPickOne   = "Pilih Kirim, Ubah atau Batal."
	txtRefused   = "Ditolak: %s"
	txtGaveUp    = "Masih ditolak setelah %d kali coba: %s\nForm dibatalkan."
	btnSend      = "Kirim"
	btnChange    = "Ubah"
	btnCancel    = "Batal"
	btnSkip      = "Lewati"
	btnDefault   = "Pakai %s"
	btnYes       = "Ya"
	btnNo        = "Tidak"
	btnDone      = "Selesai"
	txtNeedText  = "Jawab dengan teks ya."
	txtRequired  = "Wajib diisi."
	txtTooLong   = "Maksimal %d karakter (ini %d)."
	txtTooShort  = "Minimal %d karakter."
	txtNumber    = "Harus angka, misalnya 2024."
	txtNumberLo  = "Minimal %s."
	txtNumberHi  = "Maksimal %s."
	txtLink      = "Kirim link yang benar, misalnya https://contoh.com"
	txtStarts    = "%s harus diawali %s"
	txtEmbed     = "Kirim link https:// atau kode <iframe>."
	txtPick      = "Pilih dari tombol: %s"
	txtPickSome  = "Pilih dari tombol, lalu Selesai."
	txtFewest    = "Pilih minimal %d."
	txtMost      = "Maksimal %d."
	txtListFew   = "Minimal %d."
	txtYesNo     = "Jawab Ya atau Tidak."
	txtPhoto     = "Kirim foto (atau file gambar)."
	txtNotImage  = "File ini bukan gambar (jpg, png, webp atau gif)."
	txtPhotoMax  = "Foto maksimal %s MB (ini %s MB)."
	txtFile      = "Kirim file-nya."
	txtFileTypes = "File harus %s."
	txtFileMax   = "File maksimal %s MB (ini %s MB)."
	txtTooBig    = "File terlalu besar untuk bot Telegram (maks 20 MB)."
	txtFetch     = "File tidak bisa diambil, kirim lagi ya."
	txtDate      = "Tanggal tidak terbaca. Contoh: 2026-11-09 atau 9/11/2026."
	txtDateTime  = "Waktu tidak terbaca. Contoh: 2026-11-09 20:00 (WIB)."
)

// question is a question of a form, read from its Questions.
type question struct {
	Name, Question, Type string
	Required             bool
	Default              string
	Choices              []string
	Min, Max             *float64
	StartsWith           string
	FileTypes            []string // lower case, without the dot
	Fields               []string // lower case
}

// object says whether its answer is a list (an object output).
func (q *question) object() bool { return q.Type == qMulti || q.Type == qList }

// label is how the summary and Ubah name it: its name, _ as spaces, the
// first letter upper case.
func (q *question) label() string { return capitalize(strings.ReplaceAll(q.Name, "_", " ")) }

// questionName is a question's name: its output's.
var questionName = regexp.MustCompile(`^[\pL\pN_ ]{1,40}$`)

// formQuestions reads and checks a form's Questions.
func formQuestions(raw string) ([]*question, error) {
	rows, err := tableRows("Questions", raw)
	if err != nil {
		return nil, err
	}
	var qs []*question
	for i, r := range rows {
		q := &question{Name: r["Name"], Question: r["Question"], Type: r["Type"], Required: r["Required"] != "No",
			Default: r["Default"], StartsWith: r["Starts With"]}
		if q.Name == "" && q.Question == "" {
			continue
		}
		where := fmt.Sprintf("Questions, row %d (%s)", i+1, q.Name)
		if q.Type == "" {
			q.Type = qText
		}
		switch {
		case !questionName.MatchString(q.Name):
			return nil, fmt.Errorf("%s: a name is letters, digits, _ and spaces, up to 40 (judul)", where)
		case slices.Contains([]string{Done, Failed, InProgress, ErrorOutput, "Cancelled", "Timed Out", "Answers"}, q.Name):
			return nil, fmt.Errorf("%s: %s is the name of one of the form's own outputs", where, q.Name)
		case slices.ContainsFunc(qs, func(o *question) bool { return strings.EqualFold(o.Name, q.Name) }):
			return nil, fmt.Errorf("%s: two questions are named %s", where, q.Name)
		case q.Question == "":
			return nil, fmt.Errorf("%s: give the Question to ask", where)
		case !slices.Contains(questionTypes, q.Type):
			return nil, fmt.Errorf("%s: Type %q is none of %s", where, q.Type, strings.Join(questionTypes, ", "))
		}
		for _, c := range strings.Split(r["Choices"], ",") {
			if c = strings.TrimSpace(c); c != "" && !slices.Contains(q.Choices, c) {
				q.Choices = append(q.Choices, c)
			}
		}
		if (q.Type == qChoice || q.Type == qMulti) && len(q.Choices) == 0 {
			return nil, fmt.Errorf("%s: a %s needs its Choices, separated by commas (Draft, Published)", where, q.Type)
		}
		for _, f := range []struct {
			name string
			to   **float64
		}{{"Min", &q.Min}, {"Max", &q.Max}} {
			s := strings.TrimSpace(r[f.name])
			if s == "" {
				continue
			}
			n, err := strconv.ParseFloat(strings.ReplaceAll(s, ",", "."), 64)
			if err != nil {
				return nil, fmt.Errorf("%s: %s %q is not a number", where, f.name, s)
			}
			*f.to = &n
		}
		for _, t := range strings.FieldsFunc(strings.ToLower(r["File Types"]), func(r rune) bool { return r == ',' || r == ' ' }) {
			q.FileTypes = append(q.FileTypes, strings.TrimPrefix(t, "."))
		}
		for _, f := range strings.Split(r["Field"], ",") {
			if f = strings.ToLower(strings.TrimSpace(f)); f != "" {
				q.Fields = append(q.Fields, f)
			}
		}
		if q.Default != "" {
			if q.Type == qPhoto || q.Type == qFile {
				return nil, fmt.Errorf("%s: a %s has no Default", where, q.Type)
			}
			if _, problem := (&talk{}).parse(q, q.Default); problem != "" {
				return nil, fmt.Errorf("%s: its Default %q is not an answer to it: %s", where, q.Default, problem)
			}
		}
		qs = append(qs, q)
	}
	if len(qs) == 0 {
		return nil, errors.New("Questions: give at least one")
	}
	return qs, nil
}

// answer is a question's answer: text, or a list, and what the summary
// shows of it. A skipped question's is set, and empty.
type answer struct {
	set   bool
	text  string
	list  []string
	shown string
}

// formState is what a form keeps from one of its runs to the next in a run
// (call.memo): the answers, and how often the site refused them.
type formState struct {
	answers map[string]answer
	tries   int
}

var (
	errFormCancelled = errors.New("cancelled")
	errFormTimedOut  = errors.New("timed out")
)

func telegramForm(c *call) error {
	qs, err := formQuestions(c.setting("Questions"))
	if err != nil {
		return err
	}
	timeout := 10 * time.Minute
	if s := c.setting("Timeout"); s != "" {
		d, err := time.ParseDuration(s)
		if n, nerr := strconv.Atoi(s); nerr == nil {
			d, err = time.Duration(n)*time.Minute, nil
		}
		if err != nil || d < time.Second || d > 24*time.Hour {
			return fmt.Errorf("Timeout %q: how long to wait for an answer, such as 10m (minutes when a number), up to 24h", s)
		}
		timeout = d
	}
	tries := 3
	if s := c.setting("Tries"); s != "" {
		if tries, err = strconv.Atoi(s); err != nil || tries < 1 || tries > 10 {
			return fmt.Errorf("Tries %q: how many times the site may be sent the answers, 1 to 10", s)
		}
	}
	cl, id, err := c.telegram()
	if err != nil {
		return err
	}
	allowed, err := c.telegramAllowed(id)
	if err != nil {
		return err
	}
	chatText := strings.TrimSpace(c.text("Chat"))
	chat, err := strconv.ParseInt(chatText, 10, 64)
	if err != nil {
		return fmt.Errorf("Chat %q: link the Telegram Command's Chat (a chat's id)", chatText)
	}
	user := chat // a private chat's id is its person's
	if s := strings.TrimSpace(c.text("User")); s != "" {
		if user, err = strconv.ParseInt(s, 10, 64); err != nil {
			return fmt.Errorf("User %q: link the Telegram Command's User (a person's id)", s)
		}
	} else if chat < 0 {
		return fmt.Errorf("User %w: in a group, link the Telegram Command's User, the person to ask", errEmpty)
	}
	switch {
	case len(allowed) == 0:
		return errNoAllowed("Telegram Form")
	case !slices.Contains(allowed, user):
		return fmt.Errorf("user %d is not on the bot connection's Allowed Users: a form talks only with them", user)
	case c.env.TelegramTalk == nil:
		return errors.New("this run cannot talk in Telegram: it does not run in the tower")
	}

	st, _ := c.memo["form"].(*formState)
	fix := c.via == "Fix" && st != nil
	var refused map[string][]string
	if fix {
		refused = refusedFields(c)
		if st.tries++; st.tries >= tries {
			reason := refusalText(refused)
			if _, err := cl.SendMessage(c.ctx, telegram.Send{Chat: chatText, Text: fmt.Sprintf(txtGaveUp, st.tries, reason)}); err != nil {
				c.logf(v1.LogLevel_LOG_LEVEL_WARNING, "could not tell the person: %v", err)
			}
			return fmt.Errorf("the site refused the answers %d times: %s", st.tries, reason)
		}
	} else {
		st = &formState{answers: map[string]answer{}}
		c.memo["form"] = st
	}
	conv, err := c.env.TelegramTalk(id, chat, user, c.env.Run, c.step.GetId())
	if err != nil {
		return err
	}
	closed := false
	end := func() {
		if !closed {
			closed = true
			conv.close(c.ctx.Err() != nil) // a stopping tower keeps the record: it tells the chat once back
		}
	}
	defer end()
	t := &talk{c: c, cl: cl, conv: conv, chat: chatText, qs: qs, st: st, timeout: timeout, timer: time.NewTimer(timeout)}
	defer t.timer.Stop()
	if fix {
		c.logf(v1.LogLevel_LOG_LEVEL_INFO, "the site refused the answers (%s): asking again, try %d of %d", refusalText(refused), st.tries+1, tries)
	} else {
		c.logf(v1.LogLevel_LOG_LEVEL_INFO, "asking user %d in chat %d %s", user, chat, plural(len(qs), "question", "questions"))
	}
	err = t.run(fix, refused)
	t.outputs()
	switch {
	case errors.Is(err, errFormCancelled):
		t.unlive("")
		_, _ = t.send(txtCancelled, nil)
		end()
		if t.replay != nil {
			conv.replay(*t.replay)
		}
		c.log(v1.LogLevel_LOG_LEVEL_INFO, "cancelled")
		c.fire("Cancelled")
	case errors.Is(err, errFormTimedOut):
		t.unlive("")
		_, _ = t.send(txtTimedOut, nil)
		c.logf(v1.LogLevel_LOG_LEVEL_WARNING, "no answer within %s: cancelled", short(timeout))
		c.fire("Timed Out")
	case err != nil:
		return err
	default:
		c.logf(v1.LogLevel_LOG_LEVEL_SUCCESS, "answered: %s", strings.ReplaceAll(t.summaryLines(), "\n", "; "))
		c.fire(Done)
	}
	return nil
}

// refusedFields reads Errors, a site's refusal (MovieMov's: {"slug": "The
// slug has already been taken."}, a message or a list of them per field),
// as messages by field, lower case.
func refusedFields(c *call) map[string][]string {
	out := map[string][]string{}
	if c.back == nil {
		return out
	}
	v, err := c.back("Errors")
	val, ok := v.(*structpb.Value)
	if err != nil || !ok {
		return out
	}
	switch k := val.GetKind().(type) {
	case *structpb.Value_StructValue:
		for f, m := range k.StructValue.GetFields() {
			f = strings.ToLower(f)
			if l := m.GetListValue(); l != nil {
				for _, x := range l.GetValues() {
					out[f] = append(out[f], itemText(x))
				}
			} else {
				out[f] = append(out[f], itemText(m))
			}
		}
	case *structpb.Value_StringValue:
		out[""] = []string{k.StringValue}
	}
	return out
}

// refusalText is a refusal as one line: "slug: The slug has already been
// taken.", fields in order.
func refusalText(refused map[string][]string) string {
	var parts []string
	for _, f := range slices.Sorted(func(yield func(string) bool) {
		for k := range refused {
			if !yield(k) {
				return
			}
		}
	}) {
		msg := strings.Join(refused[f], " ")
		if f != "" {
			msg = f + ": " + msg
		}
		parts = append(parts, msg)
	}
	if len(parts) == 0 {
		return "situs tidak memberi alasan" // shown to the person
	}
	return strings.Join(parts, "; ")
}

// talk is a form talking with its person.
type talk struct {
	c       *call
	cl      *telegram.Client
	conv    *tgConv
	chat    string
	qs      []*question
	st      *formState
	timeout time.Duration
	timer   *time.Timer
	// live is the last message sent with buttons that still work, with its
	// text; prompt sends the question or summary waiting for its answer
	// again (after Tidak to a new command).
	live     int64
	liveText string
	prompt   func() error
	// replay is a new command the person chose to start (Ya), handed on
	// once the form has ended.
	replay *tgIn
}

// run asks the questions (on a fix, only those the site refused) and the
// summary.
func (t *talk) run(fix bool, refused map[string][]string) error {
	if fix {
		var again []*question
		for _, q := range t.qs {
			if len(t.reasons(q, refused)) > 0 {
				again = append(again, q)
			}
		}
		if len(again) > 0 {
			for _, q := range again {
				if err := t.ask(q, fmt.Sprintf(txtRefused, strings.Join(t.reasons(q, refused), " "))); err != nil {
					return err
				}
			}
			return nil // sent again, without the summary
		}
		// A refusal of no question's field: the summary, to change any.
		if _, err := t.send(fmt.Sprintf(txtRefused, refusalText(refused)), nil); err != nil {
			return err
		}
	} else {
		notes := t.prefill()
		for _, q := range t.qs {
			if a := t.st.answers[q.Name]; a.set {
				continue
			}
			if err := t.ask(q, notes[q.Name]); err != nil {
				return err
			}
		}
	}
	return t.summary()
}

// reasons are the site's messages about question q's fields (Field, or its
// name): "categories.0" is categories'.
func (t *talk) reasons(q *question, refused map[string][]string) []string {
	var out []string
	for f, msgs := range refused {
		base, _, _ := strings.Cut(f, ".")
		if slices.Contains(q.Fields, f) || slices.Contains(q.Fields, base) || strings.EqualFold(q.Name, base) {
			out = append(out, msgs...)
		}
	}
	slices.Sort(out)
	return out
}

// prefill answers questions from Prefill: a JSON object by question name,
// or text, the first question's answer (a command's text: /hapus
// night-train). An answer that does not do is asked, with why.
func (t *talk) prefill() map[string]string {
	notes := map[string]string{}
	raw := strings.TrimSpace(t.c.text("Prefill"))
	if raw == "" {
		return notes
	}
	given := map[string]string{}
	var obj structpb.Struct
	if strings.HasPrefix(raw, "{") && protojson.Unmarshal([]byte(raw), &obj) == nil {
		for k, v := range obj.GetFields() {
			given[strings.ToLower(k)] = itemText(v)
		}
	} else {
		given[strings.ToLower(t.qs[0].Name)] = raw
	}
	for _, q := range t.qs {
		s, ok := given[strings.ToLower(q.Name)]
		if !ok || q.Type == qPhoto || q.Type == qFile {
			continue
		}
		a, problem := t.parse(q, s)
		if problem != "" {
			notes[q.Name] = problem
			continue
		}
		t.st.answers[q.Name] = a
	}
	return notes
}

// send sends text with buttons, the previous live buttons first taken off.
func (t *talk) send(text string, buttons [][]telegram.Button) (int64, error) {
	t.unlive("")
	sent, err := t.cl.SendMessage(t.c.ctx, telegram.Send{Chat: t.chat, Text: text, Buttons: buttons})
	if err != nil {
		return 0, err
	}
	id := telegram.MessageID(sent)
	if buttons != nil {
		t.live, t.liveText = id, text
	}
	return id, nil
}

// unlive takes the buttons off the live message: with pick, it shows what
// was picked under its text.
func (t *talk) unlive(pick string) {
	if t.live == 0 {
		return
	}
	id, text := t.live, t.liveText
	t.live, t.liveText = 0, ""
	ctx, cancel := context.WithTimeout(context.WithoutCancel(t.c.ctx), callTimeout)
	defer cancel()
	var err error
	if pick != "" {
		err = t.cl.EditMessage(ctx, t.chat, id, text+"\n→ "+pick, nil)
	} else {
		err = t.cl.EditButtons(ctx, t.chat, id, nil)
	}
	if err != nil {
		t.c.logf(v1.LogLevel_LOG_LEVEL_DEBUG, "could not take a message's buttons off: %v", err)
	}
}

// tap answers a tap, as Telegram wants each answered.
func (t *talk) tap(cb *telegram.CallbackQuery, text string) {
	if err := t.cl.AnswerCallback(t.c.ctx, cb.ID, text); err != nil {
		t.c.logf(v1.LogLevel_LOG_LEVEL_DEBUG, "could not answer a tap: %v", err)
	}
}

// wait is the person's next input: a message, or a tap on one of this
// form's buttons, with what the button does. /cancel ends the form; a new
// command asks to end it first; a button of an ended form is told so; no
// input within the timeout ends it.
func (t *talk) wait() (tgIn, string, error) {
	for {
		select {
		case <-t.c.ctx.Done():
			return tgIn{}, "", t.c.ctx.Err()
		case <-t.timer.C:
			return tgIn{}, "", errFormTimedOut
		case in := <-t.conv.in:
			t.timer.Reset(t.timeout)
			if cb := in.cb; cb != nil {
				switch {
				case strings.HasPrefix(cb.Data, t.conv.session):
					if cb.Message != nil && cb.Message.MessageID != t.live {
						t.tap(cb, "") // an older message of this form
						continue
					}
					t.tap(cb, "")
					return in, strings.TrimPrefix(cb.Data, t.conv.session), nil
				case strings.HasPrefix(cb.Data, menuPrefix):
					t.tap(cb, "")
					if err := t.newCommand(in); err != nil {
						return tgIn{}, "", err
					}
				default:
					t.tap(cb, txtFormDone)
					if cb.Message != nil {
						_ = t.cl.EditButtons(t.c.ctx, t.chat, cb.Message.MessageID, nil)
					}
				}
				continue
			}
			if in.msg == nil {
				continue
			}
			if cmd, _, ok := splitCommand(in.msg.Text, t.conv.e.botName(t.conv.key.conn)); ok && cmd == "cancel" {
				return tgIn{}, "", errFormCancelled
			}
			if t.conv.isCommand(in.msg.Text) {
				if err := t.newCommand(in); err != nil {
					return tgIn{}, "", err
				}
				continue
			}
			return in, "", nil
		}
	}
}

// newCommand asks whether to end the form for the new command in: Ya ends
// it and starts the command once it has ended; Tidak sends the question
// waiting again.
func (t *talk) newCommand(in tgIn) error {
	prompt := t.prompt
	if _, err := t.send(txtNewCommand, [][]telegram.Button{{{Text: btnYes, Data: t.conv.session + "Y"}, {Text: btnNo, Data: t.conv.session + "N"}}}); err != nil {
		return err
	}
	for {
		next, action, err := t.wait()
		if err != nil {
			return err
		}
		yes, no := action == "Y", action == "N"
		if next.msg != nil {
			switch strings.ToLower(strings.TrimSpace(next.msg.Text)) {
			case "ya", "y", "yes", "iya":
				yes = true
			case "tidak", "t", "no", "n", "gak", "nggak":
				no = true
			}
		}
		switch {
		case yes:
			t.unlive(btnYes)
			t.replay = &in
			return errFormCancelled
		case no:
			t.unlive(btnNo)
			if _, err := t.send(txtGoOn, nil); err != nil {
				return err
			}
			if prompt != nil {
				return prompt()
			}
			return nil
		}
	}
}

// questionText is a question as sent: its text, a hint of what to send,
// and a note before it (why it is asked again).
func questionText(q *question, note string) string {
	text := q.Question
	switch q.Type {
	case qNumber:
		text += "\n(angka)"
	case qDate:
		text += "\n(contoh: 2026-11-09)"
	case qDateTime:
		text += "\n(contoh: 2026-11-09 20:00, WIB)"
	case qPhoto:
		text += fmt.Sprintf("\n(kirim foto atau file gambar, maks %s MB)", mbText(q.maxMB(10)))
	case qFile:
		if len(q.FileTypes) > 0 {
			text += fmt.Sprintf("\n(kirim file %s, maks %s MB)", strings.Join(q.FileTypes, "/"), mbText(q.maxMB(20)))
		} else {
			text += fmt.Sprintf("\n(kirim file, maks %s MB)", mbText(q.maxMB(20)))
		}
	case qMulti:
		text += "\n(pilih satu atau lebih, lalu Selesai)"
	case qList:
		text += "\n(pisahkan dengan koma)"
	}
	if note != "" {
		text = note + "\n\n" + text
	}
	return text
}

// maxMB is a photo or file question's Max, in MB, or def.
func (q *question) maxMB(def float64) float64 {
	if q.Max != nil && *q.Max > 0 {
		return *q.Max
	}
	return def
}

// mbText writes MB as people do: 2, 2.5, 0.3.
func mbText(mb float64) string { return strconv.FormatFloat(float64(int(mb*10+0.5))/10, 'f', -1, 64) }

// buttons are question q's buttons: its choices (multiple ones ticked as
// picked), Ya and Tidak, its Default as Pakai, or Lewati.
func (t *talk) buttons(q *question, picked map[int]bool) [][]telegram.Button {
	s := t.conv.session
	var rows [][]telegram.Button
	switch q.Type {
	case qChoice:
		var bs []telegram.Button
		for i, c := range q.Choices {
			bs = append(bs, telegram.Button{Text: c, Data: s + "c" + strconv.Itoa(i)})
		}
		rows = rowsOf(bs, 2)
	case qMulti:
		var bs []telegram.Button
		for i, c := range q.Choices {
			if picked[i] {
				c = "✅ " + c
			}
			bs = append(bs, telegram.Button{Text: c, Data: s + "t" + strconv.Itoa(i)})
		}
		rows = append(rowsOf(bs, 2), []telegram.Button{{Text: btnDone, Data: s + "d"}})
	case qYesNo:
		rows = [][]telegram.Button{{{Text: btnYes, Data: s + "y"}, {Text: btnNo, Data: s + "n"}}}
	}
	switch {
	case q.Default != "":
		rows = append(rows, []telegram.Button{{Text: fmt.Sprintf(btnDefault, q.Default), Data: s + "D"}})
	case !q.Required:
		rows = append(rows, []telegram.Button{{Text: btnSkip, Data: s + "x"}})
	}
	return rows
}

// ask asks question q, after note, until it has an answer that does.
func (t *talk) ask(q *question, note string) error {
	picked := map[int]bool{} // a multiple choice's ticks, kept when it is asked again
	for {
		text := questionText(q, note)
		send := func() error {
			_, err := t.send(text, t.buttons(q, picked))
			return err
		}
		t.prompt = send
		if err := send(); err != nil {
			return err
		}
		note = ""
		for note == "" {
			in, action, err := t.wait()
			if err != nil {
				return err
			}
			a, problem, pick, done := t.read(q, in, action, picked)
			switch {
			case problem != "":
				note = problem
			case !done:
				if t.live != 0 {
					_ = t.cl.EditButtons(t.c.ctx, t.chat, t.live, t.buttons(q, picked))
				}
			default:
				t.unlive(pick)
				t.st.answers[q.Name] = a
				return nil
			}
		}
	}
}

// read reads an input as question q's answer: problem says why it does
// not do, done false that more is to come (a multiple choice's tick); pick
// is what a button picked, to show under the question.
func (t *talk) read(q *question, in tgIn, action string, picked map[int]bool) (a answer, problem, pick string, done bool) {
	if action != "" {
		switch {
		case action == "x" && !q.Required:
			return answer{set: true, shown: "-"}, "", btnSkip, true
		case action == "D" && q.Default != "":
			a, problem := t.parse(q, q.Default)
			return a, problem, fmt.Sprintf(btnDefault, q.Default), true
		case action == "y" || action == "n":
			s := map[string]string{"y": "Yes", "n": "No"}[action]
			return answer{set: true, text: s, shown: map[string]string{"y": btnYes, "n": btnNo}[action]}, "", map[string]string{"y": btnYes, "n": btnNo}[action], true
		case strings.HasPrefix(action, "c"):
			if i, err := strconv.Atoi(action[1:]); err == nil && i >= 0 && i < len(q.Choices) {
				return answer{set: true, text: q.Choices[i], shown: q.Choices[i]}, "", q.Choices[i], true
			}
		case strings.HasPrefix(action, "t"):
			if i, err := strconv.Atoi(action[1:]); err == nil && i >= 0 && i < len(q.Choices) {
				picked[i] = !picked[i]
			}
			return answer{}, "", "", false
		case action == "d":
			var list []string
			for i, c := range q.Choices {
				if picked[i] {
					list = append(list, c)
				}
			}
			if problem := q.count(list); problem != "" {
				return answer{}, problem, "", false
			}
			return listAnswer(list), "", listAnswer(list).shown, true
		}
		return answer{}, "", "", false
	}
	m := in.msg
	if m == nil {
		return answer{}, "", "", false
	}
	switch q.Type {
	case qPhoto, qFile:
		a, problem, err := t.fetch(q, m)
		if err != nil {
			return answer{}, txtFetch, "", false
		}
		return a, problem, "", true
	}
	if m.Text == "" {
		return answer{}, txtNeedText, "", false
	}
	if !q.Required && strings.EqualFold(strings.TrimSpace(m.Text), btnSkip) {
		return answer{set: true, shown: "-"}, "", "", true
	}
	a, problem = t.parse(q, m.Text)
	return a, problem, "", true
}

func listAnswer(list []string) answer {
	shown := strings.Join(list, ", ")
	if shown == "" {
		shown = "-"
	}
	return answer{set: true, list: list, shown: shown}
}

// count checks a list's length against Min and Max (and Required).
func (q *question) count(list []string) string {
	switch {
	case len(list) == 0 && q.Required && q.Type == qMulti:
		return txtPickSome
	case len(list) == 0 && q.Required:
		return txtRequired
	case q.Min != nil && len(list) < int(*q.Min):
		if q.Type == qMulti {
			return fmt.Sprintf(txtFewest, int(*q.Min))
		}
		return fmt.Sprintf(txtListFew, int(*q.Min))
	case q.Max != nil && len(list) > int(*q.Max):
		return fmt.Sprintf(txtMost, int(*q.Max))
	}
	return ""
}

// parse reads typed text as question q's answer, or says why it does not
// do (in Indonesian, for the person).
func (t *talk) parse(q *question, s string) (answer, string) {
	s = strings.TrimSpace(s)
	ok := func(v string) (answer, string) { return answer{set: true, text: v, shown: v}, "" }
	if s == "" && q.Type != qMulti && q.Type != qList {
		if q.Required {
			return answer{}, txtRequired
		}
		return answer{set: true, shown: "-"}, ""
	}
	starts := func(what string) string {
		if q.StartsWith != "" && !strings.HasPrefix(strings.ToLower(s), strings.ToLower(q.StartsWith)) {
			return fmt.Sprintf(txtStarts, what, q.StartsWith)
		}
		return ""
	}
	long := func() string {
		n := utf8.RuneCountInString(s)
		switch {
		case q.Max != nil && n > int(*q.Max):
			return fmt.Sprintf(txtTooLong, int(*q.Max), n)
		case q.Min != nil && n < int(*q.Min) && q.Type == qText:
			return fmt.Sprintf(txtTooShort, int(*q.Min))
		}
		return ""
	}
	switch q.Type {
	case qText:
		if p := long(); p != "" {
			return answer{}, p
		}
		if p := starts("Jawaban"); p != "" {
			return answer{}, p
		}
		return ok(s)
	case qNumber:
		v := strings.ReplaceAll(s, " ", "")
		if !strings.Contains(v, ".") {
			v = strings.ReplaceAll(v, ",", ".")
		}
		n, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return answer{}, txtNumber
		}
		switch {
		case q.Min != nil && n < *q.Min:
			return answer{}, fmt.Sprintf(txtNumberLo, strconv.FormatFloat(*q.Min, 'f', -1, 64))
		case q.Max != nil && n > *q.Max:
			return answer{}, fmt.Sprintf(txtNumberHi, strconv.FormatFloat(*q.Max, 'f', -1, 64))
		}
		return ok(strconv.FormatFloat(n, 'f', -1, 64))
	case qLink, qEmbed:
		if q.Type == qEmbed && strings.Contains(strings.ToLower(s), "<iframe") {
			return ok(s)
		}
		if p := starts("Link"); p != "" {
			return answer{}, p
		}
		u, err := url.Parse(s)
		switch {
		case q.Type == qEmbed && (err != nil || u.Scheme != "https" || u.Host == ""):
			return answer{}, txtEmbed
		case err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || strings.ContainsAny(s, " \n"):
			return answer{}, txtLink
		}
		if p := long(); p != "" {
			return answer{}, p
		}
		return ok(s)
	case qChoice:
		for _, c := range q.Choices {
			if strings.EqualFold(c, s) {
				return ok(c)
			}
		}
		return answer{}, fmt.Sprintf(txtPick, strings.Join(q.Choices, ", "))
	case qMulti, qList:
		list := []string{}
		for _, item := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == '\n' }) {
			if item = strings.TrimSpace(item); item == "" {
				continue
			}
			if q.Type == qMulti {
				i := slices.IndexFunc(q.Choices, func(c string) bool { return strings.EqualFold(c, item) })
				if i < 0 {
					return answer{}, txtPickSome
				}
				item = q.Choices[i]
			}
			if !slices.Contains(list, item) {
				list = append(list, item)
			}
		}
		if p := q.count(list); p != "" {
			if len(list) == 0 && !q.Required {
				return listAnswer(nil), ""
			}
			return answer{}, p
		}
		return listAnswer(list), ""
	case qYesNo:
		switch strings.ToLower(s) {
		case "ya", "y", "yes", "iya", "ok", "oke":
			return answer{set: true, text: "Yes", shown: btnYes}, ""
		case "tidak", "t", "no", "n", "gak", "nggak", "enggak":
			return answer{set: true, text: "No", shown: btnNo}, ""
		}
		return answer{}, txtYesNo
	case qDate:
		d, err := parseDateWIB(s)
		if err != nil {
			return answer{}, txtDate
		}
		return ok(d.Format(time.DateOnly))
	case qDateTime:
		d, err := parseDateTimeWIB(s)
		if err != nil {
			return answer{}, txtDateTime
		}
		return answer{set: true, text: d.Format(time.RFC3339), shown: d.Format("2006-01-02 15:04") + " WIB"}, ""
	case qPhoto:
		return answer{}, txtPhoto
	case qFile:
		return answer{}, txtFile
	}
	return ok(s)
}

// months are the month names a date may be written with, Indonesian and
// English, full and short.
var months = map[string]time.Month{
	"januari": 1, "jan": 1, "january": 1, "februari": 2, "feb": 2, "february": 2, "maret": 3, "mar": 3, "march": 3,
	"april": 4, "apr": 4, "mei": 5, "may": 5, "juni": 6, "jun": 6, "june": 6, "juli": 7, "jul": 7, "july": 7,
	"agustus": 8, "agu": 8, "agt": 8, "aug": 8, "august": 8, "september": 9, "sep": 9, "sept": 9,
	"oktober": 10, "okt": 10, "oct": 10, "october": 10, "november": 11, "nov": 11, "desember": 12, "des": 12, "dec": 12, "december": 12,
}

var (
	isoDate   = regexp.MustCompile(`^(\d{4})-(\d{1,2})-(\d{1,2})$`)
	dmyDate   = regexp.MustCompile(`^(\d{1,2})[/.-](\d{1,2})[/.-](\d{4})$`)
	namedDate = regexp.MustCompile(`^(\d{1,2})\s+([\pL]+)\.?\s+(\d{4})$`)
	clockPart = regexp.MustCompile(`^(.*?)[\st,]+(?:jam\s+|pukul\s+)?(\d{1,2})[:.](\d{2})(?:\s*wib)?$`)
)

// parseDateWIB reads a date as people write it in Indonesia: 2026-11-09,
// 9/11/2026 (day first), 9-11-2026, 9 Nov 2026 or 9 November 2026.
func parseDateWIB(s string) (time.Time, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	var y, m, d int
	switch {
	case isoDate.MatchString(s):
		p := isoDate.FindStringSubmatch(s)
		y, m, d = atoi(p[1]), atoi(p[2]), atoi(p[3])
	case dmyDate.MatchString(s):
		p := dmyDate.FindStringSubmatch(s)
		d, m, y = atoi(p[1]), atoi(p[2]), atoi(p[3])
	case namedDate.MatchString(s):
		p := namedDate.FindStringSubmatch(s)
		mo, ok := months[p[2]]
		if !ok {
			return time.Time{}, fmt.Errorf("%q: no such month", p[2])
		}
		d, m, y = atoi(p[1]), int(mo), atoi(p[3])
	default:
		return time.Time{}, fmt.Errorf("%q is not a date", s)
	}
	t := time.Date(y, time.Month(m), d, 0, 0, 0, 0, wib)
	if t.Day() != d || int(t.Month()) != m || y < 1900 || y > 2200 {
		return time.Time{}, fmt.Errorf("%q is no such date", s)
	}
	return t, nil
}

// parseDateTimeWIB reads a date and a time, in WIB: 2026-11-09 20:00,
// 9/11/2026 20.00, 9 Nov 2026 jam 20:00, or RFC 3339 as given.
func parseDateTimeWIB(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.In(wib), nil
	}
	p := clockPart.FindStringSubmatch(strings.ToLower(s))
	if p == nil {
		return time.Time{}, fmt.Errorf("%q has no time (20:00)", s)
	}
	day, err := parseDateWIB(p[1])
	if err != nil {
		return time.Time{}, err
	}
	h, mi := atoi(p[2]), atoi(p[3])
	if h > 23 || mi > 59 {
		return time.Time{}, fmt.Errorf("%q: no such time", s)
	}
	return day.Add(time.Duration(h)*time.Hour + time.Duration(mi)*time.Minute), nil
}

func atoi(s string) int { n, _ := strconv.Atoi(s); return n }

// imageTypes are the pictures a photo question takes, by their MIME type,
// with their file extensions.
var imageTypes = map[string]string{"image/jpeg": ".jpg", "image/png": ".png", "image/webp": ".webp", "image/gif": ".gif"}

// fetch downloads a photo or file message as question q's answer into the
// run's folder: a photo as Telegram sent it (its largest size) or an image
// sent as a file (full quality); any file for a file question, of its File
// Types. err is a download that failed, to be sent again.
func (t *talk) fetch(q *question, m *telegram.Message) (answer, string, error) {
	need := map[bool]string{true: txtPhoto, false: txtFile}[q.Type == qPhoto]
	var fileID, name string
	var size int64
	switch {
	case len(m.Photo) > 0:
		p := m.Photo[len(m.Photo)-1]
		fileID, size, name = p.FileID, p.FileSize, "foto" // its type's extension is added
	case m.Document != nil:
		fileID, size, name = m.Document.FileID, m.Document.FileSize, m.Document.FileName
		if q.Type == qPhoto && !strings.HasPrefix(m.Document.MimeType, "image/") {
			return answer{}, txtNotImage, nil
		}
	default:
		return answer{}, need, nil
	}
	if name == "" {
		name = "file"
	}
	max := int64(q.maxMB(map[bool]float64{true: 10, false: 20}[q.Type == qPhoto]) * (1 << 20))
	tooBig := func(n int64) string {
		if q.Type == qPhoto {
			return fmt.Sprintf(txtPhotoMax, mbText(q.maxMB(10)), mbText(float64(n)/(1<<20)))
		}
		return fmt.Sprintf(txtFileMax, mbText(q.maxMB(20)), mbText(float64(n)/(1<<20)))
	}
	ext := strings.TrimPrefix(strings.ToLower(path.Ext(name)), ".")
	switch {
	case size > telegram.MaxDownload:
		return answer{}, txtTooBig, nil
	case size > max:
		return answer{}, tooBig(size), nil
	case q.Type == qFile && len(q.FileTypes) > 0 && !slices.Contains(q.FileTypes, ext):
		return answer{}, fmt.Sprintf(txtFileTypes, orList(q.FileTypes)), nil
	}
	info, err := t.cl.GetFile(t.c.ctx, fileID)
	if err != nil {
		return answer{}, "", err
	}
	if info.FileSize > max {
		return answer{}, tooBig(info.FileSize), nil
	}
	dir, err := t.c.tempDir()
	if err != nil {
		return answer{}, "", err
	}
	dst := filepath.Join(dir, safeFileName(q.Name)+"-"+safeFileName(name))
	n, err := t.cl.Download(t.c.ctx, info.FilePath, dst, max)
	if err != nil {
		if fi, serr := os.Stat(dst); serr == nil && fi.Size() > max {
			return answer{}, tooBig(fi.Size()), nil
		}
		return answer{}, "", err
	}
	if q.Type == qPhoto {
		head := make([]byte, 512)
		f, err := os.Open(dst)
		if err != nil {
			return answer{}, "", err
		}
		k, _ := f.Read(head)
		f.Close()
		e, ok := imageTypes[http.DetectContentType(head[:k])]
		if !ok {
			os.Remove(dst)
			return answer{}, txtNotImage, nil
		}
		if !strings.EqualFold(filepath.Ext(dst), e) && !(e == ".jpg" && strings.EqualFold(filepath.Ext(dst), ".jpeg")) {
			if err := os.Rename(dst, dst+e); err == nil {
				dst += e
				if path.Ext(name) == "" {
					name += e
				}
			}
		}
	}
	t.c.logf(v1.LogLevel_LOG_LEVEL_INFO, "%s: got %s (%s)", q.Name, name, sizeText(n))
	return answer{set: true, text: dst, shown: fmt.Sprintf("%s (%s)", name, sizeText(n))}, "", nil
}

// sizeText is a file's size as people say it: 3 KB, 1.4 MB.
func sizeText(n int64) string {
	if n < 1<<20 {
		return fmt.Sprintf("%d KB", (n+1023)/1024)
	}
	return mbText(float64(n)/(1<<20)) + " MB"
}

// orList is "pdf", "pdf atau zip", "pdf, zip atau png".
func orList(items []string) string {
	if len(items) < 2 {
		return strings.Join(items, "")
	}
	return strings.Join(items[:len(items)-1], ", ") + " atau " + items[len(items)-1]
}

// safeFileName keeps a name's letters, digits, - _ and ., the rest as _.
func safeFileName(s string) string {
	s = strings.Map(func(r rune) rune {
		if r == '-' || r == '_' || r == '.' || ('a' <= r && r <= 'z') || ('A' <= r && r <= 'Z') || ('0' <= r && r <= '9') {
			return r
		}
		return '_'
	}, s)
	s = strings.Trim(s, ".")
	if s == "" {
		return "file"
	}
	if len(s) > 80 {
		s = s[len(s)-80:]
	}
	return s
}

// summaryLines are the answers, a line each, as the summary shows them.
func (t *talk) summaryLines() string {
	var lines []string
	for _, q := range t.qs {
		shown := t.st.answers[q.Name].shown
		if shown == "" {
			shown = "-"
		}
		lines = append(lines, q.label()+": "+shown)
	}
	return strings.Join(lines, "\n")
}

// summary shows the answers with Kirim, Ubah and Batal: Kirim ends the
// form, Ubah asks one question again and shows the summary again, Batal
// cancels.
func (t *talk) summary() error {
	note := ""
	for {
		text := txtSummary + "\n" + t.summaryLines()
		if note != "" {
			text = note + "\n\n" + text
		}
		s := t.conv.session
		buttons := [][]telegram.Button{{{Text: btnSend, Data: s + "k"}, {Text: btnChange, Data: s + "u"}, {Text: btnCancel, Data: s + "b"}}}
		send := func() error {
			_, err := t.send(text, buttons)
			return err
		}
		t.prompt = send
		if err := send(); err != nil {
			return err
		}
		note = ""
		for note == "" {
			in, action, err := t.wait()
			if err != nil {
				return err
			}
			typed := ""
			if in.msg != nil {
				typed = strings.ToLower(strings.TrimSpace(in.msg.Text))
			}
			switch {
			case action == "k" || typed == "kirim":
				t.unlive(btnSend)
				return nil
			case action == "b" || typed == "batal":
				t.unlive(btnCancel)
				return errFormCancelled
			case action == "u" || typed == "ubah":
				t.unlive(btnChange)
				q, err := t.pick()
				if err != nil {
					return err
				}
				if err := t.ask(q, ""); err != nil {
					return err
				}
				note = "-" // the summary again
			case action == "":
				note = txtPickOne
			}
		}
		if note == "-" {
			note = ""
		}
	}
}

// pick asks which question to answer again.
func (t *talk) pick() (*question, error) {
	var bs []telegram.Button
	for i, q := range t.qs {
		bs = append(bs, telegram.Button{Text: q.label(), Data: t.conv.session + "e" + strconv.Itoa(i)})
	}
	send := func() error {
		_, err := t.send(txtWhich, rowsOf(bs, 2))
		return err
	}
	t.prompt = send
	if err := send(); err != nil {
		return nil, err
	}
	for {
		in, action, err := t.wait()
		if err != nil {
			return nil, err
		}
		if i, err := strconv.Atoi(strings.TrimPrefix(action, "e")); err == nil && strings.HasPrefix(action, "e") && i >= 0 && i < len(t.qs) {
			t.unlive(t.qs[i].label())
			return t.qs[i], nil
		}
		if in.msg != nil {
			typed := strings.TrimSpace(in.msg.Text)
			for _, q := range t.qs {
				if strings.EqualFold(typed, q.Name) || strings.EqualFold(typed, q.label()) {
					t.unlive(q.label())
					return q, nil
				}
			}
		}
	}
}

// outputs gives each question's answer as its output (a list for Multiple
// Choice and List, else text; nothing yet is empty), and Answers all.
func (t *talk) outputs() {
	all := &structpb.Struct{Fields: map[string]*structpb.Value{}}
	for _, q := range t.qs {
		a := t.st.answers[q.Name]
		if q.object() {
			l := &structpb.ListValue{}
			for _, s := range a.list {
				l.Values = append(l.Values, structpb.NewStringValue(s))
			}
			t.c.outputs[q.Name] = structpb.NewListValue(l)
			all.Fields[q.Name] = structpb.NewListValue(l)
			continue
		}
		t.c.outputs[q.Name] = stringValue(a.text)
		all.Fields[q.Name] = structpb.NewStringValue(a.text)
	}
	t.c.outputs["Answers"] = structpb.NewStructValue(all)
}
