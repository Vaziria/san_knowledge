package engine

import (
	"fmt"
	"strings"
	"testing"
	"time"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
)

// Custom choices and the choices of a question's input: the user,
// 2026-11-08, "In ask category can it optional to send custom text and
// create automatically", then the tag question's buttons from the tags
// already on moviemov.id.

// buttonsSet waits for the bot to set message msg's buttons to want (their
// texts row by row, as JSON), and marks that edit read.
func (ft *formTower) buttonsSet(msg map[string]any, want string) {
	ft.t.Helper()
	id := msg["sent_id"].(int64)
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		for i, s := range ft.f.Sent() {
			if !ft.used[i] && s["method"] == "editMessageReplyMarkup" && s["message_id"] == float64(id) && jsonOf(buttons(s)) == want {
				ft.used[i] = true
				return
			}
		}
	}
	ft.t.Fatalf("message %d's buttons did not become %s", id, want)
}

// customForm is a form of kategori, a multiple choice with Custom (2 to 4
// of Action, Drama, Sci-Fi or names typed), and negara, a choice with
// Custom (Indonesia, Korea, or typed), then extra.
func customForm(extra ...map[string]string) string {
	return table(append([]map[string]string{
		{"Name": "kategori", "Question": "Kategorinya?", "Type": "Multiple Choice", "Choices": "Action, Drama, Sci-Fi", "Custom": "Yes", "Min": "2", "Max": "4"},
		{"Name": "negara", "Question": "Negaranya?", "Type": "Choice", "Choices": "Indonesia, Korea", "Custom": "Yes"},
	}, extra...)...)
}

// isiGraph is /isi's form of questions, Prefill being the command's text.
func (ft *formTower) isiGraph(questions string) *v1.Graph {
	return &v1.Graph{Steps: []*v1.Step{
		step("cmd", TelegramCommand, "Connection", ft.bot, "Commands", table(map[string]string{"Command": "isi"})),
		step("form", TelegramForm, "Connection", ft.bot, "Questions", questions),
	}, Links: []*v1.Link{link("cmd", "/isi", "form", "Event"), link("cmd", "Chat", "form", "Chat"), link("cmd", "User", "form", "User"),
		link("cmd", "Text", "form", "Prefill")}}
}

// A multiple choice with Custom: names typed (one, or several by commas and
// lines) tick a choice by its slug or become ticked buttons of their own,
// each once; such a button unticks too; Max and Min count every tick; a
// name too long is asked again; Selesai, tapped or typed, ends it. A choice
// with Custom takes a name typed as its answer. The summary shows them, and
// Ubah asks again with the typed names still ticked.
func TestTelegramFormCustom(t *testing.T) {
	ft := newFormTower(t, heri)
	w := ft.workflow("isi", ft.isiGraph(customForm()))

	ft.f.Message(heri, "heri", "/isi")
	q := ft.say("Kategorinya?\n(pilih satu atau lebih, atau ketik sendiri dipisah koma, lalu Selesai)")
	if got := jsonOf(buttons(q)); got != `[["Action","Drama"],["Sci-Fi"],["Selesai"]]` {
		t.Errorf("buttons: %s", got)
	}
	ft.tap(heri, heri, q, "Drama")
	ft.buttonsSet(q, `[["Action","✅ Drama"],["Sci-Fi"],["Selesai"]]`)
	ft.f.Message(heri, "heri", "Mecha")
	ft.buttonsSet(q, `[["Action","✅ Drama"],["Sci-Fi","✅ Mecha"],["Selesai"]]`)
	// sci fi is Sci-Fi; drama, ticked already, stays so; mecha is Mecha's;
	// the empty one is left out; ISEKAI is the Isekai typed before it.
	ft.f.Message(heri, "heri", "sci fi, drama\nIsekai, mecha, , ISEKAI")
	ft.buttonsSet(q, `[["Action","✅ Drama"],["✅ Sci-Fi","✅ Mecha"],["✅ Isekai"],["Selesai"]]`)
	ft.quiet() // typing sends nothing new

	ft.f.Message(heri, "heri", "Horor")
	q = ft.say("Maksimal 4 (sudah 4 dipilih).\n\nKategorinya?")
	if got := jsonOf(buttons(q)); got != `[["Action","✅ Drama"],["✅ Sci-Fi","✅ Mecha"],["✅ Isekai"],["Selesai"]]` {
		t.Errorf("asked again: %s", got)
	}
	ft.f.Message(heri, "heri", strings.Repeat("x", 51))
	q = ft.say("Maksimal 50 karakter per nama (ini 51: " + strings.Repeat("x", 20) + "…).\n\nKategorinya?")
	ft.tap(heri, heri, q, "✅ Mecha")
	ft.buttonsSet(q, `[["Action","✅ Drama"],["✅ Sci-Fi","Mecha"],["✅ Isekai"],["Selesai"]]`)
	ft.tap(heri, heri, q, "✅ Drama")
	ft.buttonsSet(q, `[["Action","Drama"],["✅ Sci-Fi","Mecha"],["✅ Isekai"],["Selesai"]]`)
	ft.tap(heri, heri, q, "✅ Sci-Fi")
	ft.buttonsSet(q, `[["Action","Drama"],["Sci-Fi","Mecha"],["✅ Isekai"],["Selesai"]]`)
	ft.tap(heri, heri, q, "Selesai")
	// Asked again, the typed name no longer ticked is gone.
	q = ft.say("Pilih minimal 2.\n\nKategorinya?")
	if got := jsonOf(buttons(q)); got != `[["Action","Drama"],["Sci-Fi","✅ Isekai"],["Selesai"]]` {
		t.Errorf("asked again: %s", got)
	}
	ft.f.Message(heri, "heri", "action")
	ft.buttonsSet(q, `[["✅ Action","Drama"],["Sci-Fi","✅ Isekai"],["Selesai"]]`)
	ft.f.Message(heri, "heri", "selesai")
	ft.sent("editMessageText", "→ Action, Isekai")

	q = ft.say("Negaranya?\n(pilih atau ketik sendiri)")
	if got := jsonOf(buttons(q)); got != `[["Indonesia","Korea"]]` {
		t.Errorf("negara buttons: %s", got)
	}
	ft.f.Message(heri, "heri", "Jepang")
	sum := ft.say("Cek dulu ya:")
	if want := "Cek dulu ya:\nKategori: Action, Isekai\nNegara: Jepang"; sum["text"] != want {
		t.Errorf("summary:\n%s", sum["text"])
	}

	ft.tap(heri, heri, sum, "Ubah")
	ft.tap(heri, heri, ft.say("Mau ubah yang mana?"), "Kategori")
	q = ft.say("Kategorinya?")
	if got := jsonOf(buttons(q)); got != `[["✅ Action","Drama"],["Sci-Fi","✅ Isekai"],["Selesai"]]` {
		t.Errorf("Ubah: %s", got)
	}
	ft.f.Message(heri, "heri", "Mecha")
	ft.buttonsSet(q, `[["✅ Action","Drama"],["Sci-Fi","✅ Isekai"],["✅ Mecha"],["Selesai"]]`)
	ft.tap(heri, heri, q, "Selesai")
	sum = ft.say("Cek dulu ya:")
	if want := "Cek dulu ya:\nKategori: Action, Isekai, Mecha\nNegara: Jepang"; sum["text"] != want {
		t.Errorf("summary:\n%s", sum["text"])
	}
	ft.tap(heri, heri, sum, "Kirim")
	r := ft.runs(w, 1)[0]
	form := stepRun(t, r, "form")
	if r.Status != v1.RunStatus_RUN_STATUS_SUCCEEDED || jsonOf(objectOut(t, form, "kategori").AsInterface()) != `["Action","Isekai","Mecha"]` ||
		textOut(t, form, "negara") != "Jepang" {
		t.Errorf("run %v %s: %v", r.Status, r.Error, form.Outputs)
	}
}

// Custom No (or none) is as before: a choice typed that is none of the
// buttons is asked again, a multiple choice's typed list must be of its
// choices (and answers it, as typed), and Ubah starts with nothing ticked.
func TestTelegramFormCustomOff(t *testing.T) {
	ft := newFormTower(t, heri)
	w := ft.workflow("isi", ft.isiGraph(table(
		map[string]string{"Name": "status", "Question": "Statusnya?", "Type": "Choice", "Choices": "Draft, Published", "Custom": "No"},
		map[string]string{"Name": "genre", "Question": "Genre?", "Type": "Multiple Choice", "Choices": "Drama, Thriller"})))
	ft.f.Message(heri, "heri", "/isi")
	if q := ft.say("Statusnya?"); q["text"] != "Statusnya?" {
		t.Errorf("question: %q", q["text"])
	}
	ft.f.Message(heri, "heri", "Live")
	ft.say("Pilih dari tombol: Draft, Published\n\nStatusnya?")
	ft.f.Message(heri, "heri", "draft")
	q := ft.say("Genre?\n(pilih satu atau lebih, lalu Selesai)")
	ft.tap(heri, heri, q, "Drama")
	ft.buttonsSet(q, `[["✅ Drama","Thriller"],["Selesai"]]`)
	ft.f.Message(heri, "heri", "Mecha")
	ft.say("Pilih dari tombol, lalu Selesai.\n\nGenre?")
	ft.f.Message(heri, "heri", "thriller, drama")
	sum := ft.say("Cek dulu ya:")
	if want := "Cek dulu ya:\nStatus: Draft\nGenre: Thriller, Drama"; sum["text"] != want {
		t.Errorf("summary:\n%s", sum["text"])
	}
	ft.tap(heri, heri, sum, "Ubah")
	ft.tap(heri, heri, ft.say("Mau ubah yang mana?"), "Genre")
	if got := jsonOf(buttons(ft.say("Genre?"))); got != `[["Drama","Thriller"],["Selesai"]]` {
		t.Errorf("Ubah: %s", got)
	}
	ft.f.Message(heri, "heri", "/cancel")
	ft.say("Dibatalkan")
	ft.runs(w, 1)
}

// With Custom, Prefill and Default may be names that are none of the
// choices: JSON by name (a list as a list), or text, the first question's.
func TestTelegramFormCustomPrefill(t *testing.T) {
	ft := newFormTower(t, heri)
	w := ft.workflow("isi", ft.isiGraph(customForm(
		map[string]string{"Name": "asal", "Question": "Asalnya?", "Type": "Choice", "Choices": "Lokal", "Custom": "Yes", "Default": "Impor"})))
	ft.f.Message(heri, "heri", `/isi {"kategori": ["Mecha", "drama"], "negara": "Jepang"}`)
	q := ft.say("Asalnya?\n(pilih atau ketik sendiri)")
	if got := jsonOf(buttons(q)); got != `[["Lokal"],["Pakai Impor"]]` {
		t.Errorf("asal buttons: %s", got)
	}
	ft.tap(heri, heri, q, "Pakai Impor")
	sum := ft.say("Cek dulu ya:")
	if want := "Cek dulu ya:\nKategori: Drama, Mecha\nNegara: Jepang\nAsal: Impor"; sum["text"] != want {
		t.Errorf("summary:\n%s", sum["text"])
	}
	ft.tap(heri, heri, sum, "Kirim")
	form := stepRun(t, ft.runs(w, 1)[0], "form")
	if jsonOf(objectOut(t, form, "kategori").AsInterface()) != `["Drama","Mecha"]` || textOut(t, form, "asal") != "Impor" {
		t.Errorf("outputs: %v", form.Outputs)
	}

	ft.f.Message(heri, "heri", "/isi Mecha, sci fi")
	ft.say("Negaranya?")
	ft.f.Message(heri, "heri", "korea")
	ft.tap(heri, heri, ft.say("Asalnya?"), "Lokal")
	if sum := ft.say("Cek dulu ya:"); sum["text"] != "Cek dulu ya:\nKategori: Sci-Fi, Mecha\nNegara: Korea\nAsal: Lokal" {
		t.Errorf("summary:\n%s", sum["text"])
	}
	ft.f.Message(heri, "heri", "/cancel")
	ft.say("Dibatalkan")
}

// Custom, typed: a choice by its slug or a name of its own, a multiple
// choice's choices first and then the names, each once, each up to 50
// characters.
func TestFormParseCustom(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	choice := question{Type: qChoice, Required: true, Choices: []string{"Korea", "Sci-Fi"}, Custom: true}
	multi := question{Type: qMulti, Choices: []string{"Drama", "Sci-Fi"}, Custom: true}
	for _, c := range []struct {
		q       question
		in      string
		want    string
		problem bool
	}{
		{choice, "korea", "Korea", false},
		{choice, "sci fi", "Sci-Fi", false},
		{choice, " Jepang ", "Jepang", false},
		{choice, strings.Repeat("x", 51), "Maksimal 50 karakter (ini 51).", true},
		{choice, "", "Wajib diisi.", true},
		{question{Type: qChoice, Custom: true}, "apa saja", "apa saja", false},
		{multi, "mecha, sci fi, Drama, MECHA", "Drama, Sci-Fi, mecha", false},
		{multi, "", "-", false},
		{multi, ", ,\n", "-", false},
		{multi, "ok, " + strings.Repeat("y", 51), "Maksimal 50 karakter per nama (ini 51: " + strings.Repeat("y", 20) + "…).", true},
		{question{Type: qMulti, Required: true, Choices: []string{"Drama"}, Custom: true}, " , ", "Pilih atau ketik sendiri (dipisah koma), lalu Selesai.", true},
		{question{Type: qMulti, Choices: []string{"Drama"}, Custom: true, Max: f(2)}, "drama, a, b", "Maksimal 2.", true},
		{question{Type: qMulti, Custom: true}, "b, a", "b, a", false},
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
}

// A choice question's buttons are its Choices, then those its input brings
// that are none of them by slug; with none at all it needs Custom; a Default
// may be one of the input's, or any name with Custom.
func TestFormQuestionsChoices(t *testing.T) {
	linked := func(m map[string][]string) func(*question) ([]string, error) {
		return func(q *question) ([]string, error) { return m[q.Name], nil }
	}
	qs, err := readQuestions(table(
		map[string]string{"Name": "kategori", "Question": "?", "Type": "Multiple Choice", "Choices": "Action, Sci-Fi"},
		map[string]string{"Name": "tag", "Question": "?", "Type": "Multiple Choice", "Custom": "Yes"},
		map[string]string{"Name": "status", "Question": "?", "Type": "Choice", "Default": "Indo"},
		map[string]string{"Name": "judul", "Question": "?", "Custom": "Yes"}),
		linked(map[string][]string{"kategori": {"sci fi", "Horor", " ", "action", "Mecha", "HOROR"}, "status": {"Indo", "Luar"}}))
	if err != nil {
		t.Fatal(err)
	}
	got := fmt.Sprintf("%q %q %v %q %q %v", qs[0].Choices, qs[1].Choices, qs[1].Custom, qs[2].Choices, qs[2].Default, qs[3].Custom)
	if want := `["Action" "Sci-Fi" "Horor" "Mecha"] [] true ["Indo" "Luar"] "Indo" false`; got != want {
		t.Errorf("questions: %s, want %s", got, want)
	}
	many := make([]string, 70)
	for i := range many {
		many[i] = fmt.Sprintf("tag %d", i)
	}
	qs, err = readQuestions(table(map[string]string{"Name": "tag", "Question": "?", "Type": "Multiple Choice"}), linked(map[string][]string{"tag": many}))
	if err != nil || len(qs[0].Choices) != maxChoices || qs[0].dropped != 10 {
		t.Errorf("70 linked: %v, %d choices, %d dropped", err, len(qs[0].Choices), qs[0].dropped)
	}

	for _, c := range []struct {
		row    map[string]string
		linked []string
		want   string // the error, or "" for none
	}{
		{map[string]string{"Type": "Choice"}, nil, "a Choice needs its Choices, separated by commas (Draft, Published), a list linked to its kategori Choices input, or Custom Yes to type them"},
		{map[string]string{"Type": "Multiple Choice", "Custom": "Yes"}, nil, ""},
		{map[string]string{"Type": "Choice"}, []string{"Drama"}, ""},
		{map[string]string{"Type": "Choice", "Choices": "Lokal", "Default": "Impor"}, nil, `its Default "Impor" is not an answer to it: Pilih dari tombol: Lokal`},
		{map[string]string{"Type": "Choice", "Choices": "Lokal", "Default": "Impor", "Custom": "Yes"}, nil, ""},
		{map[string]string{"Type": "Multiple Choice", "Choices": "Drama", "Default": "Drama, Mecha"}, nil, "is not an answer to it: Pilih dari tombol, lalu Selesai."},
		{map[string]string{"Type": "Multiple Choice", "Choices": "Drama", "Default": "Drama, Mecha", "Custom": "Yes"}, nil, ""},
		{map[string]string{"Type": "Multiple Choice", "Default": "Mecha"}, []string{"mecha"}, ""},
	} {
		c.row["Name"], c.row["Question"] = "kategori", "?"
		_, err := readQuestions(table(c.row), linked(map[string][]string{"kategori": c.linked}))
		if (c.want == "") != (err == nil) || (err != nil && !strings.Contains(err.Error(), c.want)) {
			t.Errorf("%v with %q: %v, want %q", c.row, c.linked, err, c.want)
		}
	}
	_, err = readQuestions(table(map[string]string{"Name": "kategori", "Question": "?", "Type": "Choice"}),
		func(*question) ([]string, error) {
			return nil, fmt.Errorf("kategori Choices: link a list, or type the items")
		})
	if err == nil || err.Error() != "Questions, row 1 (kategori): kategori Choices: link a list, or type the items" {
		t.Errorf("a bad input: %v", err)
	}
}

// A form has an input of more choices for each choice and multiple choice
// question, named after it, which follows its name; other kinds' inputs
// are their own.
func TestStepInputs(t *testing.T) {
	names := func(st *v1.Step) string {
		var out []string
		for _, s := range StepInputs(st) {
			out = append(out, s.Name+":"+s.Type)
		}
		return strings.Join(out, " ")
	}
	form := step("f", TelegramForm, "Questions", table(
		map[string]string{"Name": "judul", "Question": "?"},
		map[string]string{"Name": "kategori", "Question": "?", "Type": "Multiple Choice"},
		map[string]string{"Name": "status", "Question": "?", "Type": "Choice"},
		map[string]string{"Name": "tag", "Question": "?", "Type": "List"},
		map[string]string{"Name": "", "Question": "?", "Type": "Choice"}))
	const own = "Event:event Fix:event Errors:object Chat:text User:text Prefill:text"
	if got := names(form); got != own+" kategori Choices:object status Choices:object" {
		t.Errorf("form: %s", got)
	}
	form.Values["Questions"] = table(map[string]string{"Name": "genre", "Question": "?", "Type": "Multiple Choice"},
		map[string]string{"Name": "status", "Question": "?", "Type": "Text"})
	if got := names(form); got != own+" genre Choices:object" {
		t.Errorf("renamed: %s", got)
	}
	if got := StepInputs(step("u", "moviemov-upload")); len(got) != len(byKind["moviemov-upload"].Inputs) {
		t.Errorf("upload: %d inputs", len(got))
	}
	if StepInputs(step("x", "no-such-kind")) != nil {
		t.Error("a kind the tower does not have has inputs")
	}
}

// MovieMov Categories and Tags reads the site's public lists without
// logging in: the most used first (or by name), Limit of each, names and
// objects.
func TestMovieMovCategoriesTags(t *testing.T) {
	site, env := movieMovEnv(t)
	site.AddMovie("a", "A", "published")
	site.SetTerms("a", "categories", "Drama", "Sci-Fi")
	site.SetTerms("a", "tags", "Korea", "Zombie")
	site.AddMovie("b", "B", "published")
	site.SetTerms("b", "categories", "Drama")
	site.SetTerms("b", "tags", "korea")
	site.AddMovie("c", "C", "draft")
	site.SetTerms("c", "categories", "Horor")
	site.SetTerms("c", "tags", "Rahasia") // a draft's only: not listed
	site.AddTerm("categories", "Action")

	s, err := runOne(t, env, movieMovTermsKind, "Limit", "2")
	if err != nil {
		t.Fatal(err)
	}
	items := objectOut(t, s, "Category Items").GetListValue().GetValues()
	if got := jsonOf(objectOut(t, s, "Categories").AsInterface()) + jsonOf(objectOut(t, s, "Tags").AsInterface()); got != `["Drama","Sci-Fi"]["Korea","Zombie"]` ||
		len(items) != 2 || jsonOf(items[0].AsInterface()) != `{"description":null,"id":2,"movies_count":2,"name":"Drama","slug":"drama","url":"`+site.URL+`/category/drama"}` ||
		len(objectOut(t, s, "Tag Items").GetListValue().GetValues()) != 2 || strings.Join(s.Fired, ",") != "Done" {
		t.Errorf("got %s; items %s; fired %v", got, jsonOf(objectOut(t, s, "Category Items").AsInterface()), s.Fired)
	}
	if logs := logsOf(s); !strings.Contains(logs, "read 2 of 4 categories and 2 of 2 tags from "+site.API()+", the most used first") {
		t.Errorf("logs: %s", logs)
	}
	s, err = runOne(t, env, movieMovTermsKind, "Order", "Name", "Limit", "0")
	if err != nil || jsonOf(objectOut(t, s, "Categories").AsInterface()) != `["Action","Drama","Horor","Sci-Fi"]` {
		t.Errorf("by name: %v %v", err, s.GetOutputs())
	}
	for _, r := range site.Requests() {
		if r.Auth != "" || (r.Path != "/api/categories" && r.Path != "/api/tags") {
			t.Errorf("request %+v", r)
		}
	}
	if n := site.Logins(); n != 0 {
		t.Errorf("logged in %d times", n)
	}
	sent := len(site.Requests())
	if _, err := runOne(t, env, movieMovTermsKind, "Limit", "lots"); err == nil || !strings.Contains(err.Error(), `Limit "lots"`) || len(site.Requests()) != sent {
		t.Errorf("bad Limit: %v, %d requests", err, len(site.Requests())-sent)
	}
}

// The upload form with its kategori and tag buttons from the site's
// categories and tags (MovieMov Categories and Tags), merged with its own
// and typed ones; a refusal of a category asks it again with the picks
// still ticked, and the fix goes up.
func TestTelegramFormChoicesFromSite(t *testing.T) {
	ft := newFormTower(t, heri)
	ft.site.AddMovie("old", "Old", "published")
	ft.site.SetTerms("old", "categories", "Drama", "Sci-Fi")
	ft.site.SetTerms("old", "tags", "Korea", "Zombie")
	ft.site.AddTerm("categories", "action")
	ft.site.AddMovie("draft", "Draft", "draft")
	ft.site.SetTerms("draft", "tags", "Rahasia")
	w := ft.workflow("upload", &v1.Graph{
		Steps: []*v1.Step{
			step("cmd", TelegramCommand, "Connection", ft.bot, "Commands", table(map[string]string{"Command": "upload"})),
			step("terms", movieMovTermsKind, "Connection", ft.movie),
			step("form", TelegramForm, "Connection", ft.bot, "Questions", table(
				map[string]string{"Name": "judul", "Question": "Judul filmnya apa?", "Field": "title, slug"},
				map[string]string{"Name": "link", "Question": "Link filmnya?", "Type": "Link or Iframe"},
				map[string]string{"Name": "kategori", "Question": "Kategorinya?", "Type": "Multiple Choice", "Choices": "Action", "Custom": "Yes", "Required": "No", "Field": "categories"},
				map[string]string{"Name": "tag", "Question": "Tag-nya?", "Type": "Multiple Choice", "Custom": "Yes", "Required": "No", "Field": "tags"})),
			step("up", "moviemov-upload", "Connection", ft.movie),
			step("url", "format-text", "Format", "Sudah diupload: {{ text }}"),
			step("done", "telegram-send", "Connection", ft.bot),
		},
		Links: []*v1.Link{
			link("cmd", "/upload", "terms", "Event"), link("terms", "Done", "form", "Event"),
			link("cmd", "Chat", "form", "Chat"), link("cmd", "User", "form", "User"),
			link("terms", "Categories", "form", "kategori Choices"), link("terms", "Tags", "form", "tag Choices"),
			link("form", "Done", "up", "Event"), link("form", "judul", "up", "Title"), link("form", "link", "up", "Embed URL"),
			link("form", "kategori", "up", "Categories"), link("form", "tag", "up", "Tags"),
			link("up", "Refused", "form", "Fix"), link("up", "Errors", "form", "Errors"),
			link("up", "Done", "done", "Event"), link("up", "URL", "url", "Text"), link("url", "Text", "done", "Text"), link("cmd", "Chat", "done", "Chat"),
		},
	})

	ft.f.Message(heri, "heri", "/upload")
	ft.say("Judul filmnya apa?")
	ft.f.Message(heri, "heri", "Night Train")
	ft.say("Link filmnya?")
	ft.f.Message(heri, "heri", "https://vidara.to/e/abc")
	q := ft.say("Kategorinya?\n(pilih satu atau lebih, atau ketik sendiri dipisah koma, lalu Selesai)")
	if got := jsonOf(buttons(q)); got != `[["Action","Drama"],["Sci-Fi"],["Selesai"],["Lewati"]]` { // the site's action is Action
		t.Errorf("kategori buttons: %s", got)
	}
	ft.tap(heri, heri, q, "Drama")
	ft.buttonsSet(q, `[["Action","✅ Drama"],["Sci-Fi"],["Selesai"],["Lewati"]]`)
	ft.f.Message(heri, "heri", "Mecha")
	ft.buttonsSet(q, `[["Action","✅ Drama"],["Sci-Fi","✅ Mecha"],["Selesai"],["Lewati"]]`)
	ft.tap(heri, heri, q, "Selesai")
	q = ft.say("Tag-nya?\n(pilih satu atau lebih, atau ketik sendiri dipisah koma, lalu Selesai)")
	if got := jsonOf(buttons(q)); got != `[["Korea","Zombie"],["Selesai"],["Lewati"]]` {
		t.Errorf("tag buttons: %s", got)
	}
	ft.tap(heri, heri, q, "Korea")
	ft.buttonsSet(q, `[["✅ Korea","Zombie"],["Selesai"],["Lewati"]]`)
	ft.f.Message(heri, "heri", "kpop")
	ft.buttonsSet(q, `[["✅ Korea","Zombie"],["✅ kpop"],["Selesai"],["Lewati"]]`)
	ft.tap(heri, heri, q, "Selesai")
	sum := ft.say("Cek dulu ya:")
	if want := "Cek dulu ya:\nJudul: Night Train\nLink: https://vidara.to/e/abc\nKategori: Drama, Mecha\nTag: Korea, kpop"; sum["text"] != want {
		t.Errorf("summary:\n%s", sum["text"])
	}
	ft.site.RefuseNext("categories.1", "The categories.1 is not allowed.")
	ft.tap(heri, heri, sum, "Kirim")
	q = ft.say("Ditolak: The categories.1 is not allowed.\n\nKategorinya?")
	if got := jsonOf(buttons(q)); got != `[["Action","✅ Drama"],["Sci-Fi","✅ Mecha"],["Selesai"],["Lewati"]]` {
		t.Errorf("asked again: %s", got)
	}
	ft.tap(heri, heri, q, "✅ Mecha")
	ft.buttonsSet(q, `[["Action","✅ Drama"],["Sci-Fi","Mecha"],["Selesai"],["Lewati"]]`)
	ft.tap(heri, heri, q, "Selesai")
	ft.say("Sudah diupload: " + ft.site.URL + "/movies/night-train")
	m := ft.site.Movie("night-train")
	if m == nil || jsonOf(m["categories"]) != `[{"name":"Drama","slug":"drama"}]` || jsonOf(m["tags"]) != `[{"name":"Korea","slug":"korea"},{"name":"kpop","slug":"kpop"}]` {
		t.Errorf("movie: %v", m)
	}
	if r := ft.runs(w, 1)[0]; r.Status != v1.RunStatus_RUN_STATUS_SUCCEEDED || strings.Count(ran(r.Steps.GetSteps()), "terms:") != 1 {
		t.Errorf("run %v %s: %s", r.Status, r.Error, ran(r.Steps.GetSteps()))
	}
}
