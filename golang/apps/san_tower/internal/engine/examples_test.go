package engine

import (
	"os"
	"slices"
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
)

// exampleGraph reads the first workflow of an example file (examples/, as
// ExportWorkflows wrote it), its steps' connections made the ones of this
// test by type: byType maps a connection type to its id here.
func exampleGraph(t *testing.T, file string, byType map[string]string) *v1.Graph {
	t.Helper()
	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var f v1.WorkflowFile
	if err := protojson.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	w := f.GetWorkflows()[0]
	for _, st := range w.GetSteps() {
		if id, ok := st.GetValues()["Connection"]; ok {
			ref := f.GetConnections()[id]
			if ref == nil || byType[ref.GetType()] == "" {
				t.Fatalf("step %s: connection %s (%v) has no stand-in", st.GetId(), id, ref)
			}
			st.Values["Connection"] = byType[ref.GetType()]
		}
	}
	return &v1.Graph{Steps: w.GetSteps(), Links: w.GetLinks()}
}

// examples/telegram-moviemov's /upload, as the file has it, against the
// fake bot, site and Vidara: a skipped thumbnail is the Vidara video's own,
// downloaded and sent as a file; a photo sent is the thumbnail; either way
// the movie's duration_minutes is Vidara's. Another player's link goes up
// without either; Vidara not answering is said in the chat and nothing is
// uploaded, unless a photo was sent, which then goes up without a
// duration.
func TestExampleTelegramMovieMov(t *testing.T) {
	ft := newFormTower(t, heri)
	vid := newVidaraFake(t)
	pic := vid.sample(t)
	w := ft.workflow("Telegram MovieMov", exampleGraph(t, "../../examples/telegram-moviemov/telegram-moviemov.workflow.json",
		map[string]string{"telegram_bot": ft.bot, "moviemov": ft.movie}))

	upload := func(title, link string, photo []byte) {
		t.Helper()
		ft.f.Message(heri, "heri", "/upload")
		ft.say("Judul filmnya apa?")
		ft.f.Message(heri, "heri", title)
		ft.say("Link film (https) atau kode iframe-nya?")
		ft.f.Message(heri, "heri", link)
		ft.tap(heri, heri, ft.say("Kategorinya?"), "Lewati")
		ft.tap(heri, heri, ft.say("Tag-nya?"), "Lewati")
		q := ft.say("Kirim thumbnail-nya (Lewati: pakai thumbnail dari Vidara)")
		if photo == nil {
			ft.tap(heri, heri, q, "Lewati")
		} else {
			ft.f.Document(heri, heri, "poster.png", "image/png", photo)
		}
		ft.tap(heri, heri, ft.say("Statusnya?"), "Pakai Draft")
		ft.tap(heri, heri, ft.say("Cek dulu ya:"), "Kirim")
	}
	vidaraStep := func(r interface{ GetSteps() []*v1.StepRun }) *v1.StepRun {
		for _, s := range r.GetSteps() {
			if s.StepId == "upload_vidara" {
				return s
			}
		}
		t.Fatal("upload_vidara did not run")
		return nil
	}

	// Skipped: Vidara's thumbnail, as a file, and its duration.
	upload("Night Train", "https://vidara.to/v/"+sampleCode, nil)
	ft.say("Sudah diupload: " + ft.site.URL + "/movies/night-train")
	if last := ft.site.Last(); last.File != "vidara-"+sampleCode+".jpg" || last.FileSize != len(pic) || last.Form.Get("embed_url") != "https://vidara.to/v/"+sampleCode ||
		last.Form.Get("duration_minutes") != "4" {
		t.Errorf("sent: %+v", last)
	}
	if got := ft.site.Movie("night-train")["duration_minutes"]; got != 4 {
		t.Errorf("night-train's duration_minutes %v", got)
	}
	r := ft.runs(w, 1)[0]
	if r.Status != v1.RunStatus_RUN_STATUS_SUCCEEDED {
		t.Fatalf("run: %v %s: %s", r.Status, r.Error, ran(r.Steps.GetSteps()))
	}
	vs := vidaraStep(r.Steps)
	if got := strings.Join(vs.Fired, ","); got != "Found,Done" || textOut(t, vs, "Duration") != "224.84" {
		t.Errorf("upload_vidara fired %s, Duration %q", got, textOut(t, vs, "Duration"))
	}
	if file := textOut(t, vs, "Thumbnail"); !strings.HasSuffix(file, "/vidara-"+sampleCode+".jpg") {
		t.Errorf("Thumbnail %s", file)
	} else if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Errorf("the run's file is still there: %v", err)
	}
	asked := len(vid.asked())

	// A photo sent: it is the thumbnail, Vidara's picture is not fetched,
	// and the duration is still Vidara's.
	upload("Night Train 2", "https://vidara.to/v/"+sampleCode, pngData(3000))
	ft.say("Sudah diupload: " + ft.site.URL + "/movies/night-train-2")
	if last := ft.site.Last(); last.File != "thumbnail-poster.png" || last.FileSize != 3000 || last.Form.Get("duration_minutes") != "4" {
		t.Errorf("sent: %+v", last)
	}
	if got := strings.Join(vidaraStep(ft.runs(w, 2)[1].Steps).Fired, ","); got != "Found,Done" ||
		slices.ContainsFunc(vid.asked()[asked:], func(k string) bool { return strings.HasSuffix(k, ".jpg") }) {
		t.Errorf("upload_vidara fired %s; Vidara asked %v", got, vid.asked()[asked:])
	}

	// Another player's link, skipped: no Vidara thumbnail nor duration.
	upload("Night Train 3", "https://player.example/embed/123", nil)
	ft.say("Sudah diupload: " + ft.site.URL + "/movies/night-train-3")
	if last := ft.site.Last(); last.File != "" || last.JSON["thumbnail_url"] != nil || last.JSON["duration_minutes"] != nil || last.JSON["embed_url"] != "https://player.example/embed/123" {
		t.Errorf("sent: %+v", last)
	}
	if got := strings.Join(vidaraStep(ft.runs(w, 3)[2].Steps).Fired, ","); got != "Not Found,Done" {
		t.Errorf("upload_vidara fired %s", got)
	}

	// Vidara not answering: said in the chat, nothing uploaded.
	vid.setDown("vidara.to", true)
	sent := len(ft.site.Requests())
	upload("Night Train 4", "https://vidara.to/e/"+sampleCode, nil)
	ft.say("Gagal ambil thumbnail dari Vidara: Vidara's page of " + sampleCode + " did not load: ")
	r = ft.runs(w, 4)[3]
	if got := strings.Join(vidaraStep(r.Steps).Fired, ","); got != "Failed" || len(ft.site.Requests()) != sent {
		t.Errorf("upload_vidara fired %s; the site got %d more requests: %s", got, len(ft.site.Requests())-sent, ran(r.Steps.GetSteps()))
	}

	// Vidara not answering with a photo sent: it goes up, without a
	// duration.
	upload("Night Train 5", "https://vidara.to/e/"+sampleCode, pngData(2000))
	ft.say("Sudah diupload: " + ft.site.URL + "/movies/night-train-5")
	if last := ft.site.Last(); last.File != "thumbnail-poster.png" || last.FileSize != 2000 || last.Form.Has("duration_minutes") {
		t.Errorf("sent: %+v", last)
	}
	if got := strings.Join(vidaraStep(ft.runs(w, 5)[4].Steps).Fired, ","); got != "Done" {
		t.Errorf("upload_vidara fired %s", got)
	}
}
