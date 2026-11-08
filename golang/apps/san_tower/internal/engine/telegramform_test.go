package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/broker"
	"github.com/wargasipil/san_tower/internal/moviemov/moviemovtest"
	"github.com/wargasipil/san_tower/internal/store"
	"github.com/wargasipil/san_tower/internal/telegram/telegramtest"
)

// formTower is a tower whose bot is a fake Bot API and whose MovieMov is a
// fake site, its workflows' runs started by the real poller from what the
// test pushes, as a person would send it.
type formTower struct {
	t     *testing.T
	f     *telegramtest.Fake
	st    *store.Store
	e     *Engine
	bot   string // the Telegram Bot connection's id
	botID uint32
	site  *moviemovtest.Server
	movie string       // the MovieMov connection's id
	used  map[int]bool // the calls in f.Sent() the test has read
	on    bool
}

// heri and budi are listed people, each in their private chat; tamu is not
// listed.
const (
	heri int64 = 42
	budi int64 = 43
	tamu int64 = 777
)

func newFormTower(t *testing.T, allowed ...int64) *formTower {
	t.Helper()
	defer func(n int) { pollTimeout = n }(pollTimeout)
	pollTimeout = 1
	ft := &formTower{t: t, f: telegramtest.New(t), used: map[int]bool{}}
	log := slog.New(slog.DiscardHandler)
	var err error
	if ft.st, err = store.Open(t.TempDir(), log); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ft.st.Close() })
	b, err := broker.Open(t.TempDir(), log)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { b.Close() })
	ft.e = New(ft.st, b, log)
	ft.e.TelegramAPI = ft.f.URL
	t.Cleanup(func() { ft.e.Close() })
	ft.site = moviemovtest.New("admin@moviemov.test", "rahasia")
	t.Cleanup(ft.site.Close)
	conn, err := ft.st.CreateConnection(store.Connection{Name: "Bot", Settings: &v1.ConnectionSettings{
		Type: &v1.ConnectionSettings_TelegramBot{TelegramBot: &v1.TelegramBotSettings{Token: telegramtest.Token, AllowedUsers: allowed}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	ft.botID, ft.bot = conn.ID, strconv.FormatUint(uint64(conn.ID), 10)
	mm, err := ft.st.CreateConnection(store.Connection{Name: "MovieMov", Settings: &v1.ConnectionSettings{
		Type: &v1.ConnectionSettings_Moviemov{Moviemov: &v1.MovieMovSettings{BaseUri: ft.site.API(), Username: "admin@moviemov.test", Password: "rahasia"}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	ft.movie = strconv.FormatUint(uint64(mm.ID), 10)
	return ft
}

// workflow makes an active workflow of g and starts the tower, or syncs
// it once started.
func (ft *formTower) workflow(name string, g *v1.Graph) store.Workflow {
	ft.t.Helper()
	w, err := ft.st.CreateWorkflow(name)
	if err != nil {
		ft.t.Fatal(err)
	}
	w.Graph = g
	if w, err = ft.st.UpdateWorkflow(w); err != nil {
		ft.t.Fatal(err)
	}
	if w, err = ft.st.SetWorkflowActive(w.ID, true); err != nil {
		ft.t.Fatal(err)
	}
	if !ft.on {
		ft.on = true
		pollTimeout = 1
		_ = ft.e.Start("http://127.0.0.1:1") // no broker: the bot needs none
		ft.t.Cleanup(func() { pollTimeout = 50 })
		for deadline := time.Now().Add(5 * time.Second); len(ft.f.Offsets()) == 0 && time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		}
	} else {
		ft.e.Sync(w.ID)
	}
	return w
}

// sent waits for the first call of method the test has not read whose
// text has want, and returns it. Calls of other methods may come before
// or after it: the bot answers a tap, edits a message and sends the next
// from different goroutines.
func (ft *formTower) sent(method, want string) map[string]any {
	ft.t.Helper()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		for i, s := range ft.f.Sent() {
			text, _ := s["text"].(string)
			if !ft.used[i] && s["method"] == method && strings.Contains(text, want) {
				ft.used[i] = true
				return s
			}
		}
	}
	var got []string
	for i, s := range ft.f.Sent() {
		if !ft.used[i] {
			got = append(got, fmt.Sprintf("%s %q", s["method"], s["text"]))
		}
	}
	ft.t.Fatalf("no %s with %q; not read: %s", method, want, strings.Join(got, ", "))
	return nil
}

// say sends the bot's next message with want in it.
func (ft *formTower) say(want string) map[string]any {
	ft.t.Helper()
	return ft.sent("sendMessage", want)
}

// quiet checks the bot sent no message for a moment.
func (ft *formTower) quiet() {
	ft.t.Helper()
	time.Sleep(300 * time.Millisecond)
	for i, s := range ft.f.Sent() {
		if !ft.used[i] && s["method"] == "sendMessage" {
			ft.t.Fatalf("the bot sent %q", s["text"])
		}
	}
}

// buttons are a sent message's buttons' texts, row by row.
func buttons(msg map[string]any) [][]string {
	rm, _ := msg["reply_markup"].(map[string]any)
	rows, _ := rm["inline_keyboard"].([]any)
	var out [][]string
	for _, r := range rows {
		var row []string
		for _, b := range r.([]any) {
			row = append(row, b.(map[string]any)["text"].(string))
		}
		out = append(out, row)
	}
	return out
}

// tap taps the button labelled text of the bot's message msg, as user in
// chat.
func (ft *formTower) tap(chat, user int64, msg map[string]any, text string) {
	ft.t.Helper()
	rm, _ := msg["reply_markup"].(map[string]any)
	rows, _ := rm["inline_keyboard"].([]any)
	for _, r := range rows {
		for _, b := range r.([]any) {
			if btn := b.(map[string]any); btn["text"] == text {
				data := btn["callback_data"].(string)
				if len(data) > 64 {
					ft.t.Fatalf("button %s's data has %d bytes: %s", text, len(data), data)
				}
				ft.f.Tap(chat, user, msg["sent_id"].(int64), data)
				return
			}
		}
	}
	ft.t.Fatalf("no button %q in %v", text, buttons(msg))
}

// runs waits until workflow w has n runs, all ended, and returns them, the
// oldest first, with their steps.
func (ft *formTower) runs(w store.Workflow, n int) []store.Run {
	ft.t.Helper()
	var list []store.Run
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		list, _ = ft.st.ListRuns(w.ID, 50)
		if len(list) >= n && !slices.ContainsFunc(list, func(r store.Run) bool { return r.Status == v1.RunStatus_RUN_STATUS_RUNNING }) {
			break
		}
	}
	if len(list) != n {
		ft.t.Fatalf("workflow %s has %d runs, want %d: %+v", w.Name, len(list), n, list)
	}
	slices.Reverse(list)
	for i, r := range list {
		full, err := ft.st.GetRun(r.ID)
		if err != nil {
			ft.t.Fatal(err)
		}
		list[i] = full
	}
	return list
}

// stepRun is step id's last StepRun in run r.
func stepRun(t *testing.T, r store.Run, id string) *v1.StepRun {
	t.Helper()
	var last *v1.StepRun
	for _, s := range r.Steps.GetSteps() {
		if s.StepId == id {
			last = s
		}
	}
	if last == nil {
		t.Fatalf("run %d has no step %s: %s", r.ID, id, ran(r.Steps.GetSteps()))
	}
	return last
}

// table is a table setting's value of rows.
func table(rows ...map[string]string) string {
	b, _ := json.Marshal(rows)
	return string(b)
}

// pngData is a PNG-looking picture of size bytes.
func pngData(size int) []byte {
	data := make([]byte, size)
	copy(data, "\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")
	return data
}

// movieForm is the MovieMov upload form of the user's design (2026-11-08):
// judul text required; link link or iframe required, https; kategori
// multiple choice optional; tag list optional; thumbnail photo optional, 2
// MB at most; status Draft or Published, default Draft.
func movieForm() string {
	return table(
		map[string]string{"Name": "judul", "Question": "Judul filmnya apa?", "Type": "Text", "Required": "Yes", "Max": "40", "Field": "title, slug"},
		map[string]string{"Name": "link", "Question": "Link film (https) atau kode iframe-nya?", "Type": "Link or Iframe", "Required": "Yes", "Starts With": "https://", "Field": "embed_url"},
		map[string]string{"Name": "kategori", "Question": "Kategorinya?", "Type": "Multiple Choice", "Required": "No", "Choices": "Drama, Thriller, Komedi", "Field": "categories"},
		map[string]string{"Name": "tag", "Question": "Tag-nya?", "Type": "List", "Required": "No", "Field": "tags"},
		map[string]string{"Name": "thumbnail", "Question": "Thumbnail-nya?", "Type": "Photo", "Required": "No", "Max": "2", "Field": "thumbnail"},
		map[string]string{"Name": "status", "Question": "Statusnya?", "Type": "Choice", "Required": "Yes", "Choices": "Draft, Published", "Default": "Draft", "Field": "status"},
	)
}

// uploadGraph is /start's menu and /upload's form, to MovieMov Upload
// Movie, whose refusal goes back to the form's Fix.
func (ft *formTower) uploadGraph() *v1.Graph {
	return &v1.Graph{
		Steps: []*v1.Step{
			step("cmd", TelegramCommand, "Connection", ft.bot, "Commands", table(
				map[string]string{"Command": "start", "Description": "Menu MovieMov"},
				map[string]string{"Command": "/upload", "Description": "Upload film"},
				map[string]string{"Command": "hapus", "Description": "Hapus film"})),
			step("menu", TelegramMenu, "Connection", ft.bot, "Text", "MovieMov: mau ngapain?", "Per Row", "2", "Buttons", table(
				map[string]string{"Button": "Upload film", "Command": "upload"},
				map[string]string{"Button": "Hapus film", "Command": "hapus"})),
			step("form", TelegramForm, "Connection", ft.bot, "Questions", movieForm()),
			step("up", "moviemov-upload", "Connection", ft.movie, "Status", "From Input"),
			step("url", "format-text", "Format", "Sudah diupload: {{ text }}"),
			step("done", "telegram-send", "Connection", ft.bot),
			step("bye", "log", "Message", "form cancelled"),
		},
		Links: []*v1.Link{
			link("cmd", "/start", "menu", "Event"), link("cmd", "Chat", "menu", "Chat"),
			link("cmd", "/upload", "form", "Event"), link("cmd", "Chat", "form", "Chat"), link("cmd", "User", "form", "User"),
			link("cmd", "Text", "form", "Prefill"),
			link("form", "Done", "up", "Event"), link("form", "judul", "up", "Title"), link("form", "link", "up", "Embed URL"),
			link("form", "kategori", "up", "Categories"), link("form", "tag", "up", "Tags"), link("form", "thumbnail", "up", "Thumbnail"),
			link("form", "status", "up", "Status Value"),
			link("up", "Refused", "form", "Fix"), link("up", "Errors", "form", "Errors"),
			link("up", "Done", "done", "Event"), link("up", "URL", "url", "Text"), link("url", "Text", "done", "Text"), link("cmd", "Chat", "done", "Chat"),
			link("form", "Cancelled", "bye", "Event"),
		},
	}
}

// The user's MovieMov upload, end to end: /start's menu, its Upload film
// button leading to the same form as /upload, each question in order with
// its checks and re-asks, Lewati, a multiple choice, a photo too big, a
// file that is no picture and an image sent as a file, Pakai Draft, the
// summary's Ubah and Kirim, then the movie on the site and the bot saying
// where.
func TestTelegramFormUpload(t *testing.T) {
	ft := newFormTower(t, heri)
	w := ft.workflow("upload", ft.uploadGraph())

	// The bot's / menu is the commands, with /cancel.
	cmds := ft.sent("setMyCommands", "")
	if got := jsonOf(cmds["commands"]); got != `[{"command":"start","description":"Menu MovieMov"},{"command":"upload","description":"Upload film"},{"command":"hapus","description":"Hapus film"},{"command":"cancel","description":"Batalkan form yang sedang diisi"}]` {
		t.Errorf("commands: %s", got)
	}

	ft.f.Message(heri, "heri", "/start")
	menu := ft.say("MovieMov: mau ngapain?")
	if got := buttons(menu); jsonOf(got) != `[["Upload film","Hapus film"]]` {
		t.Errorf("menu buttons: %v", got)
	}
	ft.tap(heri, heri, menu, "Upload film")
	if a := ft.sent("answerCallbackQuery", ""); a["text"] != nil {
		t.Errorf("the tap's answer: %v", a)
	}
	if e := ft.sent("editMessageText", "→ Upload film"); e["text"] != "MovieMov: mau ngapain?\n\n→ Upload film" || e["reply_markup"] != nil {
		t.Errorf("the menu shows the pick: %v", e)
	}

	ft.say("Judul filmnya apa?")
	ft.f.Message(heri, "heri", strings.Repeat("x", 41))
	ft.say("Maksimal 40 karakter (ini 41).\n\nJudul filmnya apa?")
	ft.f.Message(heri, "heri", "Night Train")
	ft.say("Link film (https) atau kode iframe-nya?")
	ft.f.Message(heri, "heri", "http://vidara.to/e/abc")
	ft.say("Link harus diawali https://")
	ft.f.Message(heri, "heri", "https://vidara.to/e/abc")
	q := ft.say("Kategorinya?\n(pilih satu atau lebih, lalu Selesai)")
	if got := jsonOf(buttons(q)); got != `[["Drama","Thriller"],["Komedi"],["Selesai"],["Lewati"]]` {
		t.Errorf("kategori buttons: %s", got)
	}
	ft.tap(heri, heri, q, "Drama")
	ft.sent("editMessageReplyMarkup", "")
	if got := jsonOf(buttons(ft.f.SentMessage(q["sent_id"].(int64)))); got != `[["✅ Drama","Thriller"],["Komedi"],["Selesai"],["Lewati"]]` {
		t.Errorf("ticked: %s", got)
	}
	ft.tap(heri, heri, q, "Thriller")
	ft.sent("editMessageReplyMarkup", "")
	ft.tap(heri, heri, q, "Selesai")
	ft.sent("editMessageText", "→ Drama, Thriller")
	tag := ft.say("Tag-nya?\n(pisahkan dengan koma)")
	ft.tap(heri, heri, tag, "Lewati")
	ft.say("Thumbnail-nya?\n(kirim foto atau file gambar, maks 2 MB)")
	ft.f.Photo(heri, heri, pngData(5<<19)) // 2.5 MB
	ft.say("Foto maksimal 2 MB (ini 2.5 MB).")
	ft.f.Document(heri, heri, "notes.txt", "text/plain", []byte("not a picture"))
	ft.say("File ini bukan gambar (jpg, png, webp atau gif).")
	ft.f.Document(heri, heri, "poster.png", "image/png", pngData(3000))
	status := ft.say("Statusnya?")
	if got := jsonOf(buttons(status)); got != `[["Draft","Published"],["Pakai Draft"]]` {
		t.Errorf("status buttons: %s", got)
	}
	ft.tap(heri, heri, status, "Pakai Draft")

	sum := ft.say("Cek dulu ya:")
	if want := "Cek dulu ya:\nJudul: Night Train\nLink: https://vidara.to/e/abc\nKategori: Drama, Thriller\nTag: -\nThumbnail: poster.png (3 KB)\nStatus: Draft"; sum["text"] != want {
		t.Errorf("summary:\n%s\nwant\n%s", sum["text"], want)
	}
	if got := jsonOf(buttons(sum)); got != `[["Kirim","Ubah","Batal"]]` {
		t.Errorf("summary buttons: %s", got)
	}
	ft.tap(heri, heri, sum, "Ubah")
	which := ft.say("Mau ubah yang mana?")
	ft.tap(heri, heri, which, "Judul")
	ft.say("Judul filmnya apa?")
	ft.f.Message(heri, "heri", "Night Train 2")
	sum = ft.say("Judul: Night Train 2")
	// A button of an earlier message of the form does nothing.
	ft.tap(heri, heri, which, "Link")
	ft.quiet()
	ft.tap(heri, heri, sum, "Kirim")
	ft.sent("editMessageText", "→ Kirim")
	ft.say("Sudah diupload: " + ft.site.URL + "/movies/night-train-2")

	movie := ft.site.Movie("night-train-2")
	if movie == nil || movie["title"] != "Night Train 2" || movie["embed_url"] != "https://vidara.to/e/abc" || movie["status"] != "draft" ||
		jsonOf(movie["categories"]) != `[{"name":"Drama","slug":"drama"},{"name":"Thriller","slug":"thriller"}]` {
		t.Errorf("movie: %v", movie)
	}
	if last := ft.site.Last(); last.File != "thumbnail-poster.png" || last.FileSize != 3000 {
		t.Errorf("thumbnail sent: %+v", last)
	}

	r := ft.runs(w, 2)[1]
	if r.Status != v1.RunStatus_RUN_STATUS_SUCCEEDED || r.Trigger != "telegram, chat @heri, /upload" {
		t.Fatalf("run: %v %s %s; %s", r.Status, r.Trigger, r.Error, ran(r.Steps.GetSteps()))
	}
	form := stepRun(t, r, "form")
	if textOut(t, form, "judul") != "Night Train 2" || textOut(t, form, "status") != "Draft" ||
		jsonOf(objectOut(t, form, "kategori").AsInterface()) != `["Drama","Thriller"]` || jsonOf(objectOut(t, form, "tag").AsInterface()) != `[]` {
		t.Errorf("outputs: %v", form.Outputs)
	}
	thumb := textOut(t, form, "thumbnail")
	if !strings.HasSuffix(thumb, "thumbnail-poster.png") {
		t.Errorf("thumbnail: %s", thumb)
	}
	if _, err := os.Stat(thumb); !os.IsNotExist(err) {
		t.Errorf("the run's folder is still there: %v", err)
	}
	if a := objectOut(t, form, "Answers").GetStructValue().GetFields(); a["judul"].GetStringValue() != "Night Train 2" || len(a) != 6 {
		t.Errorf("Answers: %v", a)
	}
}

// The site refusing the answers (422) asks the questions of those fields
// again, with its reason, and sends them again, up to Tries times; then
// the form fails, saying so. A fix that does is uploaded.
func TestTelegramFormRefused(t *testing.T) {
	ft := newFormTower(t, heri)
	ft.site.AddMovie("night-train", "Night Train", "published")
	w := ft.workflow("upload", ft.uploadGraph())
	fill := func(title string) {
		ft.f.Message(heri, "heri", "/upload")
		ft.say("Judul filmnya apa?")
		ft.f.Message(heri, "heri", title)
		ft.say("Link film")
		ft.f.Message(heri, "heri", `<iframe src="https://vidara.to/e/abc"></iframe>`)
		ft.tap(heri, heri, ft.say("Kategorinya?"), "Lewati")
		ft.say("Tag-nya?")
		ft.f.Message(heri, "heri", "korea, zombie")
		ft.tap(heri, heri, ft.say("Thumbnail-nya?"), "Lewati")
		ft.tap(heri, heri, ft.say("Statusnya?"), "Published")
		ft.tap(heri, heri, ft.say("Cek dulu ya:"), "Kirim")
	}
	fill("Night Train")
	for range 2 {
		ft.say("Ditolak: The slug has already been taken.\n\nJudul filmnya apa?")
		ft.f.Message(heri, "heri", "Night Train")
	}
	ft.say("Masih ditolak setelah 3 kali coba: slug: The slug has already been taken.\nForm dibatalkan.")
	r := ft.runs(w, 1)[0]
	if r.Status != v1.RunStatus_RUN_STATUS_FAILED || !strings.Contains(r.Error, "the site refused the answers 3 times: slug: The slug has already been taken.") {
		t.Errorf("run: %v %s", r.Status, r.Error)
	}
	if got := ran(r.Steps.GetSteps()); strings.Count(got, "up:Refused") != 3 {
		t.Errorf("steps: %s", got)
	}

	fill("Night Train")
	ft.say("Ditolak: The slug has already been taken.")
	ft.f.Message(heri, "heri", "Night Train Returns")
	ft.say("Sudah diupload: " + ft.site.URL + "/movies/night-train-returns")
	r = ft.runs(w, 2)[1]
	if r.Status != v1.RunStatus_RUN_STATUS_SUCCEEDED {
		t.Errorf("run: %v %s", r.Status, r.Error)
	}
	if m := ft.site.Movie("night-train-returns"); m == nil || m["status"] != "published" || jsonOf(m["tags"]) != `[{"name":"korea","slug":"korea"},{"name":"zombie","slug":"zombie"}]` {
		t.Errorf("movie: %v", m)
	}
	// The form's outputs are its last answers: the upload read them.
	if got := textOut(t, stepRun(t, r, "form"), "judul"); got != "Night Train Returns" {
		t.Errorf("judul: %s", got)
	}
}

// Only the Allowed Users: someone else is told their id, and nothing runs;
// a menu's button too. The Telegram Message trigger takes everyone while
// the list is empty, as before it, then only the listed; Command, Menu and
// Form refuse to run on an empty list and say so.
func TestTelegramFormAccess(t *testing.T) {
	ft := newFormTower(t, heri)
	w := ft.workflow("upload", ft.uploadGraph())
	echo := ft.workflow("echo", &v1.Graph{
		Steps: []*v1.Step{step("in", TelegramMessage, "Connection", ft.bot), step("log", "log", "Message", "heard")},
		Links: []*v1.Link{link("in", "Event", "log", "Event")},
	})
	ft.f.MessageFrom(-100, tamu, "tamu", "/upload")
	if r := ft.say("Maaf, tidak punya akses (ID kamu: 777)"); r["chat_id"] != float64(-100) || r["reply_parameters"] == nil {
		t.Errorf("refusal: %v", r)
	}
	ft.f.MessageFrom(-100, tamu, "tamu", "halo semua") // a group's chat: not a word
	ft.quiet()
	ft.f.Message(tamu, "tamu", "halo") // a private one: told their id
	ft.say("Maaf, tidak punya akses (ID kamu: 777)")
	ft.f.Message(heri, "heri", "/start")
	menu := ft.say("MovieMov: mau ngapain?")
	ft.tap(heri, tamu, menu, "Upload film")
	ft.sent("answerCallbackQuery", "Maaf, tidak punya akses (ID kamu: 777)")
	ft.quiet()
	ft.runs(w, 1) // /start
	if runs := ft.runs(echo, 1); !strings.HasPrefix(runs[0].Trigger, "telegram, chat @heri, message ") {
		t.Errorf("echo: %s", runs[0].Trigger) // heri's /start only
	}

	// An empty list: commands are refused, and the trigger says why.
	c, err := ft.st.GetConnection(ft.botID)
	if err != nil {
		t.Fatal(err)
	}
	c.Settings.GetTelegramBot().AllowedUsers = nil
	if _, err := ft.st.UpdateConnection(c); err != nil {
		t.Fatal(err)
	}
	ft.e.SyncConnections()
	ft.f.Message(heri, "heri", "/upload")
	ft.say("Maaf, tidak punya akses (ID kamu: 42)")
	ft.f.Message(tamu, "tamu", "halo lagi")
	ft.quiet()
	ft.runs(w, 1)
	ft.runs(echo, 3) // the Message trigger takes everyone again
	states := ft.e.Triggers(w.ID)
	if len(states) != 1 || states[0].Listening || !strings.Contains(states[0].Error, "Allowed Users is empty") || states[0].Source != "telegram @testbot /start /upload /hapus" {
		t.Errorf("states: %v", states)
	}
	g := &v1.Graph{Steps: []*v1.Step{step("go", Manual), step("menu", TelegramMenu, "Connection", ft.bot, "Chat", "42", "Text", "Menu",
		"Buttons", table(map[string]string{"Button": "Upload", "Command": "upload"}))}, Links: []*v1.Link{link("go", "Event", "menu", "Event")}}
	if _, err := Execute(context.Background(), g, "go", nil, ft.e.env()); err == nil || !strings.Contains(err.Error(), "Allowed Users is empty") {
		t.Errorf("menu with no list: %v", err)
	}
	g.Steps[1] = step("menu", TelegramForm, "Connection", ft.bot, "Chat", "42", "Questions", movieForm())
	if _, err := Execute(context.Background(), g, "go", nil, ft.e.env()); err == nil || !strings.Contains(err.Error(), "Allowed Users is empty") {
		t.Errorf("form with no list: %v", err)
	}
}

// A command's text prefills the form's first question (/hapus
// night-train), and a menu's button and the command lead to the same
// steps; a yes/no question with a filter on its answer; a command for
// another bot is not this bot's.
func TestTelegramFormDelete(t *testing.T) {
	ft := newFormTower(t, heri)
	ft.site.AddMovie("night-train", "Night Train", "published")
	ft.site.AddMovie("old-movie", "Old Movie", "draft")
	g := ft.uploadGraph()
	g.Steps = append(g.Steps,
		step("del", TelegramForm, "Connection", ft.bot, "Questions", table(
			map[string]string{"Name": "slug", "Question": "Slug filmnya?", "Type": "Text", "Field": "slug"},
			map[string]string{"Name": "yakin", "Question": "Yakin hapus film ini?", "Type": "Yes/No"})),
		step("sure", "prefix-filter", "Prefix", "Yes"),
		step("rm", "moviemov-delete", "Connection", ft.movie),
		step("gone", "format-text", "Format", "Sudah dihapus: {{ text }}"),
		step("said", "telegram-send", "Connection", ft.bot),
		step("kept", "telegram-send", "Connection", ft.bot, "Text", "Tidak jadi dihapus."))
	g.Links = append(g.Links,
		link("cmd", "/hapus", "del", "Event"), link("cmd", "Chat", "del", "Chat"), link("cmd", "User", "del", "User"), link("cmd", "Text", "del", "Prefill"),
		link("del", "Done", "sure", "Event"), link("del", "yakin", "sure", "Text"),
		link("sure", "Match", "rm", "Event"), link("del", "slug", "rm", "Slug"),
		link("rm", "Deleted", "said", "Event"), link("del", "slug", "gone", "Text"), link("gone", "Text", "said", "Text"), link("cmd", "Chat", "said", "Chat"),
		link("sure", "Else", "kept", "Event"), link("cmd", "Chat", "kept", "Chat"))
	w := ft.workflow("hapus", g)

	ft.f.Message(heri, "heri", "/hapus@testbot night-train")
	yes := ft.say("Yakin hapus film ini?")
	ft.tap(heri, heri, yes, "Ya")
	ft.tap(heri, heri, ft.say("Cek dulu ya:\nSlug: night-train\nYakin: Ya"), "Kirim")
	ft.say("Sudah dihapus: night-train")
	if ft.site.Movie("night-train") != nil {
		t.Error("night-train is still there")
	}

	ft.f.Message(heri, "heri", "/hapus@otherbot old-movie")
	ft.quiet()

	ft.f.Message(heri, "heri", "/start")
	ft.tap(heri, heri, ft.say("MovieMov: mau ngapain?"), "Hapus film")
	ft.say("Slug filmnya?")
	ft.f.Message(heri, "heri", "old-movie")
	ft.f.Message(heri, "heri", "tidak")
	ft.tap(heri, heri, ft.say("Cek dulu ya:\nSlug: old-movie\nYakin: Tidak"), "Kirim")
	ft.say("Tidak jadi dihapus.")
	if ft.site.Movie("old-movie") == nil {
		t.Error("old-movie was deleted")
	}
	runs := ft.runs(w, 3)
	if runs[0].Trigger != "telegram, chat @heri, /hapus" || runs[2].Trigger != "telegram, chat @heri, /hapus" {
		t.Errorf("triggers: %s, %s", runs[0].Trigger, runs[2].Trigger)
	}
}

// /cancel and Batal end a form with Cancelled; a new command asks first
// (Tidak goes on, Ya starts it); no answer within Timeout ends it with
// Timed Out; a button of an ended form starts nothing; another person's
// messages start their own runs while a form waits.
func TestTelegramFormCancel(t *testing.T) {
	ft := newFormTower(t, heri, budi)
	g := ft.uploadGraph()
	g.Steps = append(g.Steps, step("late", "log", "Message", "timed out"))
	g.Links = append(g.Links, link("form", "Timed Out", "late", "Event"))
	w := ft.workflow("upload", g)

	ft.f.Message(heri, "heri", "/upload")
	ft.say("Judul filmnya apa?")
	ft.f.Message(heri, "heri", "/cancel")
	ft.say("Dibatalkan")
	r := ft.runs(w, 1)[0]
	if got := ran(r.Steps.GetSteps()); !strings.Contains(got, "form:Cancelled:") || !strings.Contains(got, "bye::form cancelled") {
		t.Errorf("cancelled: %s", got)
	}

	ft.f.Message(heri, "heri", "/upload")
	ft.say("Judul filmnya apa?")
	// Another person, and the same person in another chat, are not the form's.
	ft.f.Message(budi, "budi", "/upload")
	budiForm := ft.say("Judul filmnya apa?")
	if budiForm["chat_id"] != float64(budi) {
		t.Errorf("budi's form went to %v", budiForm["chat_id"])
	}
	ft.f.Message(budi, "budi", "/cancel")
	ft.say("Dibatalkan")
	ft.f.Message(heri, "heri", "/hapus")
	ask := ft.say("Form yang tadi belum selesai. Batalkan dan mulai baru?")
	ft.tap(heri, heri, ask, "Tidak")
	ft.say("Oke, lanjut.")
	ft.say("Judul filmnya apa?")
	ft.f.Message(heri, "heri", "Night Train")
	ft.say("Link film")
	ft.f.Message(heri, "heri", "/start")
	ft.tap(heri, heri, ft.say("Batalkan dan mulai baru?"), "Ya")
	ft.say("Dibatalkan")
	ft.say("MovieMov: mau ngapain?")
	runs := ft.runs(w, 4)
	if runs[3].Trigger != "telegram, chat @heri, /start" {
		t.Errorf("the new command's run: %s", runs[3].Trigger)
	}

	// Batal on the summary, and then its buttons start nothing.
	g2 := &v1.Graph{Steps: []*v1.Step{
		step("cmd", TelegramCommand, "Connection", ft.bot, "Commands", table(map[string]string{"Command": "tanya"})),
		step("form", TelegramForm, "Connection", ft.bot, "Timeout", "1s", "Questions", table(map[string]string{"Name": "umur", "Question": "Umur?", "Type": "Number", "Min": "1", "Max": "120"})),
		step("late", "log", "Message", "timed out"),
	}, Links: []*v1.Link{link("cmd", "/tanya", "form", "Event"), link("cmd", "Chat", "form", "Chat"), link("form", "Timed Out", "late", "Event")}}
	w2 := ft.workflow("tanya", g2)
	ft.f.Message(heri, "heri", "/tanya")
	ft.say("Umur?\n(angka)")
	ft.f.Message(heri, "heri", "200")
	ft.say("Maksimal 120.")
	ft.f.Message(heri, "heri", "30")
	sum := ft.say("Cek dulu ya:\nUmur: 30")
	ft.tap(heri, heri, sum, "Batal")
	ft.say("Dibatalkan")
	ft.tap(heri, heri, sum, "Kirim")
	ft.sent("answerCallbackQuery", "Form ini sudah selesai")
	ft.sent("editMessageReplyMarkup", "")
	ft.quiet()

	// The timeout.
	ft.f.Message(heri, "heri", "/tanya")
	ft.say("Umur?")
	ft.say("Waktu habis, form dibatalkan")
	runs = ft.runs(w2, 2)
	if got := ran(runs[1].Steps.GetSteps()); !strings.Contains(got, "form:Timed Out:") || !strings.Contains(got, "late::timed out") {
		t.Errorf("timed out: %s", got)
	}
}

// A tower that stops during a form loses it, and once back its bot tells
// that chat.
func TestTelegramFormRestart(t *testing.T) {
	ft := newFormTower(t, heri)
	w := ft.workflow("upload", ft.uploadGraph())
	ft.f.Message(heri, "heri", "/upload")
	ft.say("Judul filmnya apa?")
	ft.e.Close()
	if r := ft.runs(w, 1)[0]; r.Status != v1.RunStatus_RUN_STATUS_INTERRUPTED {
		t.Errorf("run: %v", r.Status)
	}

	log := slog.New(slog.DiscardHandler)
	b, err := broker.Open(t.TempDir(), log)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	e := New(ft.st, b, log)
	e.TelegramAPI = ft.f.URL
	defer e.Close()
	_ = e.Start("http://127.0.0.1:1")
	if m := ft.say("Maaf, form terputus, silakan mulai lagi"); m["chat_id"] != float64(heri) {
		t.Errorf("notice: %v", m)
	}
	if left, _ := ft.st.TakeTelegramConversations(); len(left) != 0 {
		t.Errorf("records left: %v", left)
	}
}

// The other types, in a talk: a file of File Types, a date and a date time
// in WIB, a number, a choice typed, a list; and a form in a group asks the
// one person who sent the command.
func TestTelegramFormTypes(t *testing.T) {
	ft := newFormTower(t, heri, budi)
	w := ft.workflow("types", &v1.Graph{Steps: []*v1.Step{
		step("cmd", TelegramCommand, "Connection", ft.bot, "Commands", table(map[string]string{"Command": "isi"})),
		step("form", TelegramForm, "Connection", ft.bot, "Questions", table(
			map[string]string{"Name": "naskah", "Question": "Naskahnya?", "Type": "File", "File Types": "pdf, docx", "Max": "1"},
			map[string]string{"Name": "tanggal", "Question": "Tanggal rilis?", "Type": "Date"},
			map[string]string{"Name": "tayang", "Question": "Tayang kapan?", "Type": "Date Time"},
			map[string]string{"Name": "durasi", "Question": "Durasi (jam)?", "Type": "Number", "Min": "0.5"},
			map[string]string{"Name": "rating", "Question": "Rating?", "Type": "Choice", "Choices": "SU, 13+, 17+"},
			map[string]string{"Name": "pemain", "Question": "Pemain?", "Type": "List", "Max": "3"})),
	}, Links: []*v1.Link{link("cmd", "/isi", "form", "Event"), link("cmd", "Chat", "form", "Chat"), link("cmd", "User", "form", "User")}})

	ft.f.MessageFrom(-100, heri, "heri", "/isi")
	ft.say("Naskahnya?\n(kirim file pdf/docx, maks 1 MB)")
	ft.f.MessageFrom(-100, budi, "budi", "bukan untuk form") // another person in the group
	ft.f.Document(-100, heri, "naskah.txt", "text/plain", []byte("teks"))
	ft.say("File harus pdf atau docx.")
	ft.f.Document(-100, heri, "naskah.pdf", "application/pdf", make([]byte, 2<<20))
	ft.say("File maksimal 1 MB (ini 2 MB).")
	ft.f.Document(-100, heri, "naskah.pdf", "application/pdf", []byte("%PDF-1.7"))
	ft.say("Tanggal rilis?\n(contoh: 2026-11-09)")
	ft.f.MessageFrom(-100, heri, "heri", "besok")
	ft.say("Tanggal tidak terbaca.")
	ft.f.MessageFrom(-100, heri, "heri", "17 Agustus 2026")
	ft.say("Tayang kapan?\n(contoh: 2026-11-09 20:00, WIB)")
	ft.f.MessageFrom(-100, heri, "heri", "9/11/2026 20.30")
	ft.say("Durasi (jam)?")
	ft.f.MessageFrom(-100, heri, "heri", "1,5")
	ft.say("Rating?")
	ft.f.MessageFrom(-100, heri, "heri", "17+")
	ft.say("Pemain?")
	ft.f.MessageFrom(-100, heri, "heri", "A, B, C, D")
	ft.say("Maksimal 3.")
	ft.f.MessageFrom(-100, heri, "heri", "Reza, Dian")
	sum := ft.say("Cek dulu ya:")
	if want := "Cek dulu ya:\nNaskah: naskah.pdf (1 KB)\nTanggal: 2026-08-17\nTayang: 2026-11-09 20:30 WIB\nDurasi: 1.5\nRating: 17+\nPemain: Reza, Dian"; sum["text"] != want {
		t.Errorf("summary:\n%s", sum["text"])
	}
	ft.tap(-100, budi, sum, "Kirim") // not budi's form
	ft.sent("answerCallbackQuery", "Form ini bukan untukmu")
	ft.tap(-100, heri, sum, "Kirim")
	r := ft.runs(w, 1)[0]
	form := stepRun(t, r, "form")
	if r.Status != v1.RunStatus_RUN_STATUS_SUCCEEDED || textOut(t, form, "tanggal") != "2026-08-17" || textOut(t, form, "tayang") != "2026-11-09T20:30:00+07:00" ||
		textOut(t, form, "durasi") != "1.5" || textOut(t, form, "rating") != "17+" || jsonOf(objectOut(t, form, "pemain").AsInterface()) != `["Reza","Dian"]` ||
		!strings.HasSuffix(textOut(t, form, "naskah"), "naskah-naskah.pdf") {
		t.Errorf("run %v %s: %v", r.Status, r.Error, form.Outputs)
	}
}

// Every type of question, typed: its checks, and the value it gives.
func TestFormParse(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	for _, c := range []struct {
		q       question
		in      string
		want    string // the answer, or the problem
		problem bool
	}{
		{question{Type: qText, Required: true}, "", "Wajib diisi.", true},
		{question{Type: qText}, "", "-", false},
		{question{Type: qText, Min: f(3)}, "ab", "Minimal 3 karakter.", true},
		{question{Type: qText, Max: f(3)}, "abcd", "Maksimal 3 karakter (ini 4).", true},
		{question{Type: qText, StartsWith: "#"}, "abc", "Jawaban harus diawali #", true},
		{question{Type: qNumber}, "dua", "Harus angka, misalnya 2024.", true},
		{question{Type: qNumber}, "1,5", "1.5", false},
		{question{Type: qNumber, Min: f(1870)}, "1500", "Minimal 1870.", true},
		{question{Type: qNumber, Max: f(2100)}, "2200", "Maksimal 2100.", true},
		{question{Type: qLink}, "vidara.to/e/x", "Kirim link yang benar, misalnya https://contoh.com", true},
		{question{Type: qLink}, "http://a.b/c", "http://a.b/c", false},
		{question{Type: qLink, StartsWith: "https://"}, "http://a.b/c", "Link harus diawali https://", true},
		{question{Type: qEmbed}, "http://a.b/c", "Kirim link https:// atau kode <iframe>.", true},
		{question{Type: qEmbed}, `<IFRAME src="x">`, `<IFRAME src="x">`, false},
		{question{Type: qChoice, Choices: []string{"Draft", "Published"}}, "published", "Published", false},
		{question{Type: qChoice, Choices: []string{"Draft", "Published"}}, "Live", "Pilih dari tombol: Draft, Published", true},
		{question{Type: qMulti, Required: true, Choices: []string{"Drama", "Thriller"}}, "drama, Horor", "Pilih dari tombol, lalu Selesai.", true},
		{question{Type: qMulti, Choices: []string{"Drama", "Thriller"}, Max: f(1)}, "drama, thriller", "Maksimal 1.", true},
		{question{Type: qMulti, Choices: []string{"Drama", "Thriller"}}, "thriller,drama", "Thriller, Drama", false},
		{question{Type: qList, Min: f(2)}, "korea", "Minimal 2.", true},
		{question{Type: qList}, "korea, zombie, korea", "korea, zombie", false},
		{question{Type: qYesNo}, "iya", "Ya", false},
		{question{Type: qYesNo}, "nggak", "Tidak", false},
		{question{Type: qYesNo}, "mungkin", "Jawab Ya atau Tidak.", true},
		{question{Type: qDate}, "9/11/2026", "2026-11-09", false},
		{question{Type: qDate}, "31/2/2026", "Tanggal tidak terbaca. Contoh: 2026-11-09 atau 9/11/2026.", true},
		{question{Type: qDateTime}, "9 Nov 2026 jam 20.30", "2026-11-09 20:30 WIB", false},
		{question{Type: qDateTime}, "besok", "Waktu tidak terbaca. Contoh: 2026-11-09 20:00 (WIB).", true},
		{question{Type: qPhoto}, "foto.jpg", "Kirim foto (atau file gambar).", true},
		{question{Type: qFile}, "a.pdf", "Kirim file-nya.", true},
	} {
		q := c.q
		a, problem := (&talk{}).parse(&q, c.in)
		got := a.shown
		if c.problem {
			got = problem
		} else if problem != "" {
			got = "problem: " + problem
		}
		if got != c.want || (c.problem && a.set) {
			t.Errorf("%s %q: %q, want %q", q.Type, c.in, got, c.want)
		}
	}
	if a, _ := (&talk{}).parse(&question{Type: qDateTime}, "2026-11-09 20:00"); a.text != "2026-11-09T20:00:00+07:00" {
		t.Errorf("date time: %s", a.text)
	}
	if a, _ := (&talk{}).parse(&question{Type: qYesNo}, "ya"); a.text != "Yes" {
		t.Errorf("yes: %s", a.text)
	}
}

func TestParseDateWIB(t *testing.T) {
	for in, want := range map[string]string{
		"2026-11-09": "2026-11-09", "9/11/2026": "2026-11-09", "09-11-2026": "2026-11-09", "9.11.2026": "2026-11-09",
		"9 Nov 2026": "2026-11-09", "9 november 2026": "2026-11-09", "17 Agustus 2026": "2026-08-17", "1 Mei 2026": "2026-05-01",
		"29 Feb 2028": "2028-02-29", "29 Feb 2026": "", "9 Novem 2026": "", "11/09": "", "": "",
	} {
		d, err := parseDateWIB(in)
		got := ""
		if err == nil {
			got = d.Format(time.DateOnly)
			if d.Location() != wib {
				t.Errorf("%q: not WIB", in)
			}
		}
		if got != want {
			t.Errorf("%q: %q, want %q", in, got, want)
		}
	}
	for in, want := range map[string]string{
		"2026-11-09 20:00": "2026-11-09T20:00:00+07:00", "9/11/2026 20.00": "2026-11-09T20:00:00+07:00",
		"9 Nov 2026, pukul 07:05 WIB": "2026-11-09T07:05:00+07:00", "2026-11-09T13:00:00Z": "2026-11-09T20:00:00+07:00",
		"2026-11-09": "", "2026-11-09 25:00": "",
	} {
		d, err := parseDateTimeWIB(in)
		got := ""
		if err == nil {
			got = d.Format(time.RFC3339)
		}
		if got != want {
			t.Errorf("%q: %q, want %q", in, got, want)
		}
	}
}

func TestSplitCommand(t *testing.T) {
	for _, c := range []struct{ text, bot, cmd, args string }{
		{"/hapus night-train", "", "hapus", "night-train"},
		{"/Upload@MovieBot", "moviebot", "upload", ""},
		{"/upload@other_bot", "moviebot", "", ""},
		{"/start\nhalo", "", "start", "halo"},
		{"halo /start", "", "", ""},
		{"/", "", "", ""},
	} {
		cmd, args, _ := splitCommand(c.text, c.bot)
		if cmd != c.cmd || args != c.args {
			t.Errorf("%q: %q %q, want %q %q", c.text, cmd, args, c.cmd, c.args)
		}
	}
}

// A form's outputs follow its questions, by name and type, before Error;
// a command trigger's are its commands, first; other kinds are as they
// were.
func TestStepOutputs(t *testing.T) {
	names := func(st *v1.Step) string {
		var out []string
		for _, s := range StepOutputs(st) {
			out = append(out, s.Name+":"+s.Type)
		}
		return strings.Join(out, " ")
	}
	form := step("f", TelegramForm, "Questions", movieForm())
	if got := names(form); got != "In Progress:event Done:event Failed:event Cancelled:event Timed Out:event Answers:object judul:text link:text kategori:object tag:object thumbnail:text status:text Error:text" {
		t.Errorf("form: %s", got)
	}
	cmd := step("c", TelegramCommand, "Commands", `[{"Command":"start"},{"Command":"/upload"},{"Command":""},{"Command":"upload"}]`)
	if got := names(cmd); got != "/start:event /upload:event Message:object Text:text Chat:text User:text Command:text" {
		t.Errorf("command: %s", got)
	}
	if got := names(step("p", "publish")); got != "In Progress:event Done:event Failed:event Result:object Error:text" {
		t.Errorf("publish: %s", got)
	}
	if got := names(step("f", TelegramForm, "Questions", "not json")); !strings.HasPrefix(got, "In Progress:event") || strings.Contains(got, "judul") {
		t.Errorf("bad questions: %s", got)
	}
}

// A form's Questions are checked before it talks.
func TestFormQuestions(t *testing.T) {
	for raw, want := range map[string]string{
		"":                                  "give at least one",
		"[{]":                               "not a table",
		`[{"Name":"a b-c","Question":"?"}]`: "a name is letters",
		`[{"Name":"Done","Question":"?"}]`:  "one of the form's own outputs",
		`[{"Name":"a","Question":"?"},{"Name":"A","Question":"?"}]`: "two questions are named A",
		`[{"Name":"a"}]`: "give the Question",
		`[{"Name":"a","Question":"?","Type":"Video"}]`:                                 `Type "Video" is none of`,
		`[{"Name":"a","Question":"?","Type":"Choice"}]`:                                "needs its Choices",
		`[{"Name":"a","Question":"?","Max":"lots"}]`:                                   `Max "lots" is not a number`,
		`[{"Name":"a","Question":"?","Type":"Photo","Default":"x"}]`:                   "a Photo has no Default",
		`[{"Name":"a","Question":"?","Type":"Choice","Choices":"A, B","Default":"C"}]`: "its Default \"C\" is not an answer",
	} {
		if _, err := formQuestions(raw); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: %v, want %q", raw, err, want)
		}
	}
	qs, err := formQuestions(movieForm())
	if err != nil || len(qs) != 6 || qs[0].Fields[1] != "slug" || qs[2].Required || qs[4].maxMB(10) != 2 {
		t.Errorf("movie form: %v %+v", err, qs)
	}
}

// A Telegram Command run by hand takes typed text as the message.
func TestTelegramCommandFired(t *testing.T) {
	st := step("c", TelegramCommand, "Commands", `[{"Command":"hapus"}]`)
	out, fired, err := telegramCommandFired(st, []byte("/hapus night-train"))
	if err != nil || jsonOf(fired) != `["/hapus"]` || out["Text"].(interface{ GetValue() string }).GetValue() != "night-train" {
		t.Errorf("%v %v %v", out, fired, err)
	}
	if _, _, err := telegramCommandFired(st, []byte(`{"text": "/upload", "chat": {"id": 1}}`)); err == nil || !strings.Contains(err.Error(), "none of its commands (/hapus)") {
		t.Errorf("another command: %v", err)
	}
}
