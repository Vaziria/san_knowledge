package engine

import (
	"os"
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
// fake bot, site and Vidara: a photo sent is the thumbnail; a skipped one
// is the Vidara video's own, downloaded and sent as a file; another
// player's link goes up without one; Vidara not answering is said in the
// chat, and nothing is uploaded.
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
	thumbStep := func(r interface{ GetSteps() []*v1.StepRun }) *v1.StepRun {
		for _, s := range r.GetSteps() {
			if s.StepId == "upload_thumb" {
				return s
			}
		}
		t.Fatal("upload_thumb did not run")
		return nil
	}

	// Skipped: Vidara's thumbnail, as a file.
	upload("Night Train", "https://vidara.to/v/"+sampleCode, nil)
	ft.say("Sudah diupload: " + ft.site.URL + "/movies/night-train")
	if last := ft.site.Last(); last.File != "vidara-"+sampleCode+".jpg" || last.FileSize != len(pic) || last.Form.Get("embed_url") != "https://vidara.to/v/"+sampleCode {
		t.Errorf("sent: %+v", last)
	}
	r := ft.runs(w, 1)[0]
	if r.Status != v1.RunStatus_RUN_STATUS_SUCCEEDED {
		t.Fatalf("run: %v %s: %s", r.Status, r.Error, ran(r.Steps.GetSteps()))
	}
	th := thumbStep(r.Steps)
	if got := strings.Join(th.Fired, ","); got != "Found,Done" {
		t.Errorf("upload_thumb fired %s", got)
	}
	if file := textOut(t, th, "Thumbnail"); !strings.HasSuffix(file, "/vidara-"+sampleCode+".jpg") {
		t.Errorf("Thumbnail %s", file)
	} else if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Errorf("the run's file is still there: %v", err)
	}
	asked := len(vid.asked())

	// A photo sent: it is the thumbnail, and Vidara is not read.
	upload("Night Train 2", "https://vidara.to/v/"+sampleCode, pngData(3000))
	ft.say("Sudah diupload: " + ft.site.URL + "/movies/night-train-2")
	if last := ft.site.Last(); last.File != "thumbnail-poster.png" || last.FileSize != 3000 {
		t.Errorf("sent: %+v", last)
	}
	if got := strings.Join(thumbStep(ft.runs(w, 2)[1].Steps).Fired, ","); got != "Done" || len(vid.asked()) != asked {
		t.Errorf("upload_thumb fired %s; Vidara asked %v", got, vid.asked()[asked:])
	}

	// Another player's link, skipped: no Vidara thumbnail, uploaded without.
	upload("Night Train 3", "https://player.example/embed/123", nil)
	ft.say("Sudah diupload: " + ft.site.URL + "/movies/night-train-3")
	if last := ft.site.Last(); last.File != "" || last.JSON["thumbnail_url"] != nil || last.JSON["embed_url"] != "https://player.example/embed/123" {
		t.Errorf("sent: %+v", last)
	}
	if got := strings.Join(thumbStep(ft.runs(w, 3)[2].Steps).Fired, ","); got != "Not Found,Done" {
		t.Errorf("upload_thumb fired %s", got)
	}

	// Vidara not answering: said in the chat, nothing uploaded.
	vid.mu.Lock()
	vid.down["vidara.to"] = true
	vid.mu.Unlock()
	sent := len(ft.site.Requests())
	upload("Night Train 4", "https://vidara.to/e/"+sampleCode, nil)
	ft.say("Gagal ambil thumbnail dari Vidara: Vidara's page of " + sampleCode + " did not load: ")
	r = ft.runs(w, 4)[3]
	if got := strings.Join(thumbStep(r.Steps).Fired, ","); got != "Failed" || len(ft.site.Requests()) != sent {
		t.Errorf("upload_thumb fired %s; the site got %d more requests: %s", got, len(ft.site.Requests())-sent, ran(r.Steps.GetSteps()))
	}
}
