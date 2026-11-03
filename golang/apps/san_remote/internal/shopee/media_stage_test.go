package shopee

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// editPage is a fake product page in Seller Centre (/portal/product/<id>):
// Foto Produk, Video Produk with its Edit Video trimmer, and the page's
// buttons; Update saves what the page holds into the product's detail.
type editPage struct {
	*fake
	mu      sync.Mutex
	product listedProduct
	// The product as saved, and the page as it reads.
	saved, shown []string
	savedVideo   string
	video        videoState
	pending      int  // photos in Pembuat Foto AI
	trimmer      bool // Edit Video is open
	added        int
	buttons      []string
}

func newEditPage(p listedProduct, photos []string, video string) *editPage {
	e := &editPage{fake: shop([]listedProduct{p}, nil, nil), product: p, saved: photos, savedVideo: video, buttons: []string{"Kembali", "Hapus", "Tampilkan", "Update"}}
	e.onNavigate = func() {
		e.mu.Lock()
		defer e.mu.Unlock()
		e.shown = slices.Clone(e.saved)
		e.video = videoState{State: "empty"}
		if e.savedVideo != "" {
			e.video = videoState{State: "ready", ID: e.savedVideo, Duration: "00:44"}
		}
	}
	link := func(id string) string { return "https://down-id.img.susercontent.com/file/" + id }
	answer := func(fn string, f func(args []json.RawMessage) any) {
		e.answers[fn] = func(args []json.RawMessage) any {
			e.mu.Lock()
			defer e.mu.Unlock()
			return f(args)
		}
	}
	e.answers["state"] = func([]json.RawMessage) any {
		return state{URL: listURL, Lang: "id", Portal: true, List: true, ListReady: true, Edit: true, Form: true}
	}
	answer("detail", func([]json.RawMessage) any {
		d := map[string]any{"code": 0, "found": true, "name": e.product.Name, "status": e.product.Status, "images": e.saved, "videos": []any{}}
		if e.savedVideo != "" {
			d["videos"] = []any{map[string]any{"id": e.savedVideo, "duration": 15, "cover": e.savedVideo + "_cover"}}
		}
		return d
	})
	answer("editButtons", func([]json.RawMessage) any {
		var out []map[string]any
		for _, b := range e.buttons {
			out = append(out, map[string]any{"label": b})
		}
		return out
	})
	answer("gallery", func([]json.RawMessage) any {
		var out []string
		for _, id := range e.shown {
			out = append(out, link(id))
		}
		return out
	})
	answer("cover", func([]json.RawMessage) any {
		if len(e.shown) == 0 {
			return ""
		}
		return link(e.shown[0])
	})
	answer("pressDeletePhoto", func(args []json.RawMessage) any {
		var i int
		_ = json.Unmarshal(args[0], &i)
		if i >= len(e.shown) || len(e.shown) == 1 {
			return false // Seller Centre keeps one photo
		}
		e.shown = slices.Delete(e.shown, i, i+1)
		return true
	})
	answer("give", func(args []json.RawMessage) any {
		var field string
		var refs []fileRef
		_ = json.Unmarshal(args[0], &field)
		_ = json.Unmarshal(args[1], &refs)
		switch field {
		case "images":
			e.pending = len(refs)
		case "video":
			e.trimmer = true
			e.video = videoState{State: "uploading", Progress: "0%"}
		}
		return ""
	})
	answer("photos", func([]json.RawMessage) any {
		ps := photoState{Count: len(e.shown), Max: MaxPhotos, Ready: true}
		if e.pending > 0 {
			ps.Drawer = fmt.Sprintf("Terapkan ke Produk (%d)", e.pending)
		}
		return ps
	})
	answer("pressApplyPhotos", func([]json.RawMessage) any {
		for range e.pending {
			e.added++
			e.shown = append(e.shown, fmt.Sprintf("new%d", e.added))
		}
		e.pending = 0
		return true
	})
	answer("modal", func([]json.RawMessage) any {
		if e.trimmer {
			return modal{Title: "Edit Video", Text: "Edit Video 15 detik ditentukan Batal Konfirmasi", Buttons: []string{"Batal", "Konfirmasi"}}
		}
		return nil
	})
	answer("pressModal", func(args []json.RawMessage) any {
		var what, button string
		_ = json.Unmarshal(args[0], &what)
		_ = json.Unmarshal(args[1], &button)
		if e.trimmer && what == "Edit Video" && button == "Konfirmasi" {
			e.trimmer = false
			e.video = videoState{State: "processing", ID: "newvid"}
			return true
		}
		return false
	})
	answer("video", func([]json.RawMessage) any {
		v := e.video
		if v.State == "processing" {
			// Processed by the next look.
			e.video = videoState{State: "ready", ID: v.ID, Duration: "00:15"}
		}
		return v
	})
	answer("pressRemoveVideo", func([]json.RawMessage) any {
		if e.video.State != "ready" && e.video.State != "processing" {
			return false
		}
		e.video = videoState{State: "empty"}
		return true
	})
	answer("name", func([]json.RawMessage) any { return map[string]string{"value": e.product.Name} })
	answer("errors", func([]json.RawMessage) any { return []string{} })
	answer("guides", func([]json.RawMessage) any { return []string{} })
	answer("pressEditButton", func(args []json.RawMessage) any {
		var label string
		_ = json.Unmarshal(args[0], &label)
		if !slices.Contains(e.buttons, label) {
			return false
		}
		if label == "Update" {
			e.saved = slices.Clone(e.shown)
			e.savedVideo = ""
			if e.video.State != "empty" {
				e.savedVideo = e.video.ID
			}
		}
		return true
	})
	return e
}

var tes = listedProduct{ID: 49871140785, Name: "TES san_remote - akan dihapus", Status: 8, Unlist: true, PriceMin: 9999000, PriceMax: 9999000, Stock: 1}

// The whole gallery and the video are replaced on the product's page, the
// page is read back, Update (never Tampilkan) saves it, and the product
// reads back with the new photos and video, archived as it was.
func TestUpdateMedia(t *testing.T) {
	dir := t.TempDir()
	m := Media{
		ID:     tes.ID,
		Photos: []string{photo(t, dir, "b1.png", 500, 500), photo(t, dir, "b2.png", 500, 500), photo(t, dir, "b3.png", 500, 500)},
		Video:  mp4File(t, dir, "tes2.mp4", 15*time.Second),
	}
	e := newEditPage(tes, []string{"old1", "old2"}, "oldvid")
	u, err := (&Service{B: e}).UpdateMedia(context.Background(), m)
	if err != nil {
		t.Fatalf("%v (calls %v)", err, e.calls)
	}
	wantPhotos := []string{photoLink("new1"), photoLink("new2"), photoLink("new3")}
	if !slices.Equal(u.Product.Photos, wantPhotos) || u.Product.Video == nil || u.Product.Video.ID != "newvid" || u.Product.Status != "archived" {
		t.Errorf("after: %+v %+v", u.Product, u.Product.Video)
	}
	if !slices.Equal(u.Before.Photos, []string{photoLink("old1"), photoLink("old2")}) || u.Before.Video == nil || u.Before.Video.ID != "oldvid" {
		t.Errorf("before: %+v", u.Before)
	}
	if !slices.Contains(u.Form, "photo 1: new1") || !slices.Contains(u.Form, "cover: new1") || !slices.Contains(u.Form, "video: ready 00:15 (newvid)") {
		t.Errorf("form: %v", u.Form)
	}
	// Photo 2 first, the new ones after photo 1, then photo 1.
	var order []string
	for _, c := range e.calls {
		if strings.HasPrefix(c, "press") {
			order = append(order, c)
		}
	}
	want := []string{"pressDeletePhoto(1)", "pressApplyPhotos()", "pressDeletePhoto(0)", "pressRemoveVideo()", `pressModal("Edit Video", "Konfirmasi")`, `pressEditButton("Update")`}
	if !slices.Equal(order, want) {
		t.Errorf("presses %v, want %v", order, want)
	}
}

// A rehearsal does all but Update: the product keeps its photos and video.
func TestUpdateMediaRehearse(t *testing.T) {
	dir := t.TempDir()
	e := newEditPage(tes, []string{"old1"}, "oldvid")
	m := Media{ID: tes.ID, Photos: []string{photo(t, dir, "b1.png", 500, 500)}, RemoveVideo: true, Rehearse: true}
	u, err := (&Service{B: e}).UpdateMedia(context.Background(), m)
	if err != nil {
		t.Fatalf("%v (calls %v)", err, e.calls)
	}
	if e.called("pressEditButton") || !slices.Equal(e.saved, []string{"old1"}) || e.savedVideo != "oldvid" {
		t.Errorf("saved: %v %v (calls %v)", e.saved, e.savedVideo, e.calls)
	}
	if !slices.Contains(u.Form, "photo 1: new1") || !slices.Contains(u.Form, "video: empty") || len(u.Form) != 5 {
		t.Errorf("form: %v", u.Form)
	}
	if !e.called("pressRemoveVideo") {
		t.Errorf("calls: %v", e.calls)
	}
}

// Only the video is taken away: the photos are not touched.
func TestUpdateMediaRemoveVideo(t *testing.T) {
	e := newEditPage(tes, []string{"old1", "old2"}, "oldvid")
	u, err := (&Service{B: e}).UpdateMedia(context.Background(), Media{ID: tes.ID, RemoveVideo: true})
	if err != nil {
		t.Fatalf("%v (calls %v)", err, e.calls)
	}
	if u.Product.Video != nil || !slices.Equal(e.saved, []string{"old1", "old2"}) || e.called("pressDeletePhoto") || e.called("give") {
		t.Errorf("after: %+v, saved %v (calls %v)", u.Product, e.saved, e.calls)
	}
	// A product with no video has nothing to take away: its page is not
	// opened.
	e = newEditPage(tes, []string{"old1"}, "")
	u, err = (&Service{B: e}).UpdateMedia(context.Background(), Media{ID: tes.ID, RemoveVideo: true})
	if err != nil || e.called("editButtons") || e.called("press") || len(u.Form) != 1 {
		t.Errorf("no video: %v %v (calls %v)", u, err, e.calls)
	}
}

// What would change the product's visibility, or another shop's product,
// is refused before anything on the page is touched.
func TestUpdateMediaRefused(t *testing.T) {
	dir := t.TempDir()
	m := Media{ID: tes.ID, Photos: []string{photo(t, dir, "b1.png", 500, 500)}}
	// A page that offers only Tampilkan (which shows the product) and no
	// Update.
	e := newEditPage(tes, []string{"old1"}, "")
	e.buttons = []string{"Kembali", "Hapus", "Tampilkan"}
	_, err := (&Service{B: e}).UpdateMedia(context.Background(), m)
	if !errors.Is(err, ErrRefused) || !strings.Contains(err.Error(), "Tampilkan") {
		t.Errorf("no Update: %v", err)
	}
	if e.called("press") || e.called("give") {
		t.Errorf("touched: %v", e.calls)
	}
	m.ID = 42
	e = newEditPage(tes, []string{"old1"}, "")
	if _, err := (&Service{B: e}).UpdateMedia(context.Background(), m); !errors.Is(err, ErrNotFound) || e.called("detail") || e.called("editButtons") {
		t.Errorf("no such product: %v (calls %v)", err, e.calls)
	}
	// A short video is refused before a tab opens.
	m = Media{ID: tes.ID, Video: mp4File(t, dir, "short.mp4", 5*time.Second)}
	e = newEditPage(tes, []string{"old1"}, "")
	if _, err := (&Service{B: e}).UpdateMedia(context.Background(), m); !errors.Is(err, ErrInvalid) || len(e.calls) > 0 {
		t.Errorf("a short video: %v (calls %v)", err, e.calls)
	}
}

// What Video Produk says against a video is Seller Centre's refusal.
func TestPutVideoRefused(t *testing.T) {
	dir := t.TempDir()
	e := newEditPage(tes, []string{"old1"}, "")
	e.onNavigate()
	e.answers["video"] = func([]json.RawMessage) any {
		e.mu.Lock()
		defer e.mu.Unlock()
		if e.trimmer || e.video.State == "empty" {
			return e.video
		}
		return videoState{State: "other", Errors: []string{"Durasi video harus 10-60 detik"}}
	}
	s := (&Service{B: e})
	video := mp4File(t, dir, "tes.mp4", 15*time.Second)
	err := s.run(context.Background(), "", true, func(st *session) error {
		_, err := st.putVideo(context.Background(), video)
		return err
	})
	if !errors.Is(err, ErrRefused) || !strings.Contains(err.Error(), "Durasi video") {
		t.Errorf("err = %v", err)
	}
}
