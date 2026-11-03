package shopee

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"connectrpc.com/connect"

	"github.com/wargasipil/san_remote/internal/page"
)

// ---- a product's detail ----------------------------------------------------------------

// detail is what shopee.js detail() reports: the product as its own page
// reads it (get_product_info).
type detail struct {
	Code    int      `json:"code"`
	Message string   `json:"message"`
	Found   bool     `json:"found"`
	Name    string   `json:"name"`
	Status  int      `json:"status"`
	Unlist  bool     `json:"unlist"`
	Images  []string `json:"images"`
	Videos  []struct {
		ID       string `json:"id"`
		Duration int64  `json:"duration"`
		Cover    string `json:"cover"`
	} `json:"videos"`
}

// video is the product's video, or nil.
func (d *detail) video() *Video {
	if len(d.Videos) == 0 || d.Videos[0].ID == "" {
		return nil
	}
	v := d.Videos[0]
	return &Video{ID: v.ID, Duration: time.Duration(v.Duration) * time.Second, Cover: photoLink(v.Cover)}
}

// videoID is the product's video's id, "" for none.
func (d *detail) videoID() string {
	if v := d.video(); v != nil {
		return v.ID
	}
	return ""
}

// apply puts the detail's photos and video on p.
func (d *detail) apply(p *Product) {
	p.Photos = nil
	for _, id := range d.Images {
		p.Photos = append(p.Photos, photoLink(id))
	}
	p.Video, p.MediaRead = d.video(), true
}

// detail reads the product's detail. A navigation in the tab can refuse a
// call; that is only a reason to ask again.
func (t *session) detail(ctx context.Context, id int64) (*detail, error) {
	var d detail
	err := t.Until(ctx, fmt.Sprintf("product %d's detail", id), 30*time.Second, func() (bool, error) {
		err := t.CallFor(ctx, 30*time.Second, &d, "detail", id)
		if err != nil && ctx.Err() == nil && connect.CodeOf(err) == connect.CodeAborted {
			return false, page.Sleep(ctx, time.Second)
		}
		return err == nil, err
	})
	if err != nil {
		return nil, err
	}
	if d.Code != 0 {
		return nil, fmt.Errorf("%w: product %d's detail does not read (%d %s)", ErrPage, id, d.Code, d.Message)
	}
	if !d.Found {
		return nil, fmt.Errorf("%w: product %d has no detail", ErrNotFound, id)
	}
	return &d, nil
}

// ---- Video Produk ----------------------------------------------------------------------

// videoState is what shopee.js video() reports: empty, uploading,
// processing or ready, with the video's id once Seller Centre shows its
// cover.
type videoState struct {
	State    string   `json:"state"`
	ID       string   `json:"id"`
	Duration string   `json:"duration"`
	Progress string   `json:"progress"`
	Text     string   `json:"text"`
	Errors   []string `json:"errors"`
}

// accepted says Seller Centre has taken the video: it is processing it or
// shows it.
func (v *videoState) accepted() bool {
	return v != nil && (v.State == "ready" || v.State == "processing")
}

func (v *videoState) String() string {
	if v == nil {
		return "no Video Produk"
	}
	s := v.State
	if v.Duration != "" {
		s += " " + v.Duration
	}
	if v.ID != "" {
		s += " (" + v.ID + ")"
	}
	return s
}

func (t *session) video(ctx context.Context) (*videoState, error) {
	var v *videoState
	err := t.Call(ctx, &v, "video")
	return v, err
}

// waitVideoField waits for Video Produk, which comes with the rest of the
// form.
func (t *session) waitVideoField(ctx context.Context) (*videoState, error) {
	var v *videoState
	err := t.Until(ctx, "Video Produk", 30*time.Second, func() (bool, error) {
		var err error
		v, err = t.video(ctx)
		return err == nil && v != nil, err
	})
	return v, err
}

// putVideo gives Video Produk the video: Seller Centre opens it in Edit
// Video (a trimmer: "15 detik ditentukan"), where Konfirmasi keeps it
// whole; it then uploads and processes it ("Diproses") and shows its
// cover and length. A video still processing after a few minutes is taken
// as it is: Seller Centre saves a product while its video is processed.
// Video Produk must be empty.
func (t *session) putVideo(ctx context.Context, path string) (*videoState, error) {
	v, err := t.waitVideoField(ctx)
	if err != nil {
		return nil, err
	}
	if v.State != "empty" {
		return nil, fmt.Errorf("%w: Video Produk is not empty (%s)", ErrPage, v)
	}
	t.Log("choosing the video " + path)
	if err := t.give(ctx, "video", "video", []string{path}); err != nil {
		return nil, err
	}
	confirmed := false
	if err := t.Until(ctx, "Seller Centre to take the video", 5*time.Minute, func() (bool, error) {
		var m *modal
		if err := t.Call(ctx, &m, "modal"); err != nil {
			return false, err
		}
		if m != nil && strings.Contains(m.Text, "Edit Video") {
			if confirmed {
				return false, nil
			}
			// Konfirmasi keeps the video whole.
			if err := t.answer(ctx, "Edit Video", "Konfirmasi"); err != nil {
				return false, err
			}
			confirmed = true
			return false, nil
		}
		var err error
		if v, err = t.video(ctx); err != nil || v == nil {
			return false, err
		}
		if len(v.Errors) > 0 {
			return false, fmt.Errorf("%w: Video Produk: %s", ErrRefused, strings.Join(v.Errors, "; "))
		}
		return v.accepted(), nil
	}); err != nil {
		return nil, t.refusal(ctx, fmt.Errorf("%w (Video Produk reads %s)", err, v))
	}
	if v.State == "processing" {
		t.Log("Seller Centre is processing the video")
		err := t.Until(ctx, "Seller Centre to process the video", 3*time.Minute, func() (bool, error) {
			var err error
			v, err = t.video(ctx)
			if err == nil && v != nil && len(v.Errors) > 0 {
				return false, fmt.Errorf("%w: Video Produk: %s", ErrRefused, strings.Join(v.Errors, "; "))
			}
			return err == nil && v != nil && v.State == "ready", err
		})
		if err != nil && !errors.Is(err, ErrPage) {
			return nil, err
		}
		if err != nil {
			// "Kamu dapat menampilkan produk saat video sedang diproses."
			t.Log("the video is still processing; Seller Centre saves the product all the same")
		}
	}
	if !v.accepted() {
		return nil, fmt.Errorf("%w: Video Produk reads %s", ErrPage, v)
	}
	t.Log("Video Produk: " + v.String())
	return v, nil
}

// removeVideo takes the video out of Video Produk.
func (t *session) removeVideo(ctx context.Context) error {
	v, err := t.video(ctx)
	if err != nil {
		return err
	}
	t.Log("taking the video away (" + v.String() + ")")
	if err := t.Press(ctx, "the video's delete", "pressRemoveVideo"); err != nil {
		return err
	}
	return t.Until(ctx, "Video Produk to be empty", 15*time.Second, func() (bool, error) {
		v, err := t.video(ctx)
		return err == nil && v != nil && v.State == "empty", err
	})
}

// ---- photos and video of a product the shop has ----------------------------------------

// Updated is a product UpdateMedia changed: as before, and as its page
// reads afterwards (for a rehearsal, as before), and what the form read
// before it was saved.
type Updated struct {
	Before  Product
	Product Product
	Form    []string
}

// UpdateMedia replaces the photos and the video of a product the shop
// has, on its own page in Seller Centre (/portal/product/<id>), as a person
// does there: the whole gallery for the photos given (the old ones taken
// away, the new ones added through Pembuat Foto AI; the first is the cover,
// which Seller Centre locks to photo 1), the video taken away and the new
// one given to Video Produk, or only taken away. Only what is asked is
// touched. The page is read back, and only then is Update pressed: the
// button that keeps a live product live and an archived one archived (the
// page's others, Tampilkan and Arsipkan, change that, and are never
// pressed). The product's detail is then read until it has the new photos
// and video, and the list says it is still as visible as it was. With
// Rehearse, it stops before Update. It brings Chrome to the front with a
// tab of its own.
func (s *Service) UpdateMedia(ctx context.Context, m Media) (*Updated, error) {
	if err := m.Check(); err != nil {
		return nil, err
	}
	var out *Updated
	err := s.run(ctx, m.Browser, true, func(t *session) error {
		var err error
		out, err = t.updateMedia(ctx, m)
		return err
	})
	return out, err
}

func (t *session) updateMedia(ctx context.Context, m Media) (*Updated, error) {
	p, err := t.existing(ctx, m.ID)
	if err != nil {
		return nil, err
	}
	before, err := t.detail(ctx, m.ID)
	if err != nil {
		return nil, err
	}
	before.apply(p)
	out := &Updated{Before: *p}
	if m.RemoveVideo && len(m.Photos) == 0 && before.videoID() == "" {
		t.Log(fmt.Sprintf("%d has no video: nothing to change", m.ID))
		out.Product = *p
		out.Form = []string{"(not opened: the product has no video to take away)"}
		return out, nil
	}
	t.Log(fmt.Sprintf("opening %d %q (%s, %d photos, video %s)", p.ID, p.Name, p.Status, len(before.Images), orNone(before.videoID())))
	if err := t.Navigate(ctx, p.URL()); err != nil {
		return nil, err
	}
	if _, err := t.waitState(ctx, "the product's page", time.Minute, func(st state) bool { return st.Edit && st.Form }); err != nil {
		return nil, err
	}
	if err := t.Look(ctx, 2*time.Second); err != nil {
		return nil, err
	}
	t.guides(ctx)
	// The button that saves is Update; check it is there before anything
	// is changed.
	if _, err := t.updateButton(ctx); err != nil {
		return nil, err
	}
	// The page must read as the detail does, so that only what is asked
	// changes.
	var shown []string
	if err := t.Until(ctx, "the product's photos on its page", 30*time.Second, func() (bool, error) {
		var err error
		shown, err = t.gallery(ctx)
		return err == nil && slices.Equal(shown, before.Images), err
	}); err != nil {
		return nil, fmt.Errorf("%w (the page shows %v, the product has %v)", err, shown, before.Images)
	}
	want := before.Images
	if len(m.Photos) > 0 {
		if want, err = t.replacePhotos(ctx, before.Images, m.Photos); err != nil {
			return nil, err
		}
	}
	if m.Video != "" || m.RemoveVideo {
		v, err := t.waitVideoField(ctx)
		if err != nil {
			return nil, err
		}
		if v.State != "empty" {
			if err := t.removeVideo(ctx); err != nil {
				return nil, err
			}
		}
		if m.Video != "" {
			if _, err := t.putVideo(ctx, m.Video); err != nil {
				return nil, err
			}
		}
	}
	f, err := t.mediaView(ctx)
	if err != nil {
		return nil, err
	}
	lines, problems := f.check(m, want, before, p.Name)
	if len(problems) > 0 {
		return nil, fmt.Errorf("%w: the page reads %s; not saving", ErrPage, strings.Join(problems, "; "))
	}
	out.Form = lines
	t.Log(fmt.Sprintf("checked the page: %d photos, video %s", len(f.Photos), f.Video))
	if m.Rehearse {
		t.Log("rehearsal: every step but Update; leaving the page unsaved")
		out.Product = *p
		return out, nil
	}
	after, err := t.updateSave(ctx, m.ID, f)
	if err != nil {
		return nil, err
	}
	// The list says whether it is as visible as it was.
	q, err := t.waitStatus(ctx, m.ID, "the product in Produk Saya", 30*time.Second, func(q *Product) bool { return q != nil })
	if err != nil {
		return nil, err
	}
	after.apply(q)
	out.Product = *q
	if q.Status != p.Status {
		return out, fmt.Errorf("%w: product %d was %s and is %s after Update", ErrPage, m.ID, p.Status, q.Status)
	}
	return out, nil
}

func orNone(s string) string {
	if s == "" {
		return "none"
	}
	return s
}

// updateButton finds Update among the page's buttons.
func (t *session) updateButton(ctx context.Context) (string, error) {
	var bs []struct {
		Label    string `json:"label"`
		Disabled bool   `json:"disabled"`
	}
	if err := t.Until(ctx, "the page's buttons", 30*time.Second, func() (bool, error) {
		err := t.Call(ctx, &bs, "editButtons")
		return err == nil && len(bs) > 0, err
	}); err != nil {
		return "", err
	}
	var labels []string
	for _, b := range bs {
		labels = append(labels, b.Label)
		if b.Label == "Update" && !b.Disabled {
			return b.Label, nil
		}
	}
	return "", fmt.Errorf("%w: the page offers %s, but no Update, which keeps the product as visible as it is; not changing it", ErrRefused, strings.Join(labels, ", "))
}

// gallery reads Foto Produk's photos, by id, in order.
func (t *session) gallery(ctx context.Context) ([]string, error) {
	var links []string
	if err := t.Call(ctx, &links, "gallery"); err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(links))
	for _, l := range links {
		ids = append(ids, photoID(l))
	}
	return ids, nil
}

// replacePhotos turns the gallery old into the photos paths, and returns
// the new photos' ids as the gallery reads them. Seller Centre holds 9
// photos and needs 1, so the old ones after the first go, the new ones are
// added after it (8 at most), the old first one goes, and a 9th new one is
// added last. The new first photo becomes the cover. Seller Centre gives a
// photo it has seen before its old id, so the photos are followed by
// position, not by id.
func (t *session) replacePhotos(ctx context.Context, old []string, paths []string) ([]string, error) {
	t.Log(fmt.Sprintf("replacing the %d photos with %d", len(old), len(paths)))
	for i := len(old) - 1; i >= 1; i-- {
		if err := t.deletePhoto(ctx, i, old[:i+1]); err != nil {
			return nil, err
		}
	}
	have := min(len(old), 1)
	first := paths[:min(len(paths), MaxPhotos-have)]
	if err := t.addPhotos(ctx, first, have, "photo"); err != nil {
		return nil, err
	}
	g, err := t.gallery(ctx)
	if err != nil {
		return nil, err
	}
	if len(g) != have+len(first) {
		return nil, fmt.Errorf("%w: Foto Produk holds %d photos, not %d", ErrPage, len(g), have+len(first))
	}
	added := slices.Clone(g[have:])
	if have == 1 {
		if err := t.deletePhoto(ctx, 0, g); err != nil {
			return nil, err
		}
	}
	if rest := paths[len(first):]; len(rest) > 0 {
		if err := t.addPhotos(ctx, rest, len(first), "more"); err != nil {
			return nil, err
		}
		if g, err = t.gallery(ctx); err != nil {
			return nil, err
		}
		if len(g) != len(paths) {
			return nil, fmt.Errorf("%w: Foto Produk holds %d photos, not %d", ErrPage, len(g), len(paths))
		}
		added = append(added, g[len(first):]...)
	}
	if err := t.cover(ctx); err != nil {
		return nil, err
	}
	t.guides(ctx)
	return added, nil
}

// deletePhoto takes photo i of the gallery (which reads now) away, and
// waits for the gallery to read without it.
func (t *session) deletePhoto(ctx context.Context, i int, now []string) error {
	want := slices.Delete(slices.Clone(now), i, i+1)
	t.Log(fmt.Sprintf("taking photo %d away (%s)", i+1, now[i]))
	if err := t.Press(ctx, fmt.Sprintf("photo %d's Delete", i+1), "pressDeletePhoto", i); err != nil {
		return err
	}
	var ids []string
	err := t.Until(ctx, fmt.Sprintf("photo %d to go", i+1), 15*time.Second, func() (bool, error) {
		var err error
		ids, err = t.gallery(ctx)
		return err == nil && slices.Equal(ids, want), err
	})
	if err != nil {
		return fmt.Errorf("%w (Foto Produk reads %v)", err, ids)
	}
	return nil
}

// mediaPage is a product's page as it reads before Update.
type mediaPage struct {
	Name   string
	Photos []string // ids, in order
	Cover  string   // id
	Video  *videoState
	Errors []string
}

func (t *session) mediaView(ctx context.Context) (*mediaPage, error) {
	f := &mediaPage{}
	var err error
	if f.Photos, err = t.gallery(ctx); err != nil {
		return nil, err
	}
	var name struct {
		Value string `json:"value"`
	}
	var cover string
	for _, c := range []struct {
		out any
		fn  string
	}{{&name, "name"}, {&cover, "cover"}, {&f.Video, "video"}, {&f.Errors, "errors"}} {
		if err := t.Call(ctx, c.out, c.fn); err != nil {
			return nil, err
		}
	}
	f.Name, f.Cover = name.Value, photoID(cover)
	return f, nil
}

// check compares the page with what was asked, and returns the page line
// by line and what differs: the photos as want (the new ones as they were
// added, or the product's very own), the cover photo 1, the video as asked,
// and no error on the page.
func (f *mediaPage) check(m Media, want []string, before *detail, name string) (lines, problems []string) {
	bad := func(format string, a ...any) { problems = append(problems, fmt.Sprintf(format, a...)) }
	lines = append(lines, "name: "+f.Name)
	if f.Name != name {
		bad("the name %q, not %q", f.Name, name)
	}
	lines = append(lines, fmt.Sprintf("photos: %d", len(f.Photos)))
	for i, id := range f.Photos {
		lines = append(lines, fmt.Sprintf("photo %d: %s", i+1, id))
	}
	switch {
	case len(m.Photos) > 0 && len(f.Photos) != len(m.Photos):
		bad("%d photos, not %d", len(f.Photos), len(m.Photos))
	case !slices.Equal(f.Photos, want):
		bad("photos %v, not %v", f.Photos, want)
	}
	lines = append(lines, "cover: "+f.Cover)
	if len(f.Photos) > 0 && f.Cover != f.Photos[0] {
		bad("the cover %s, not photo 1 (%s)", f.Cover, f.Photos[0])
	}
	lines = append(lines, "video: "+f.Video.String())
	switch {
	case f.Video == nil:
		bad("no Video Produk")
	case m.RemoveVideo:
		if f.Video.State != "empty" {
			bad("a video still there (%s)", f.Video)
		}
	case m.Video != "":
		if !f.Video.accepted() {
			bad("the new video not taken (%s)", f.Video)
		} else if f.Video.ID != "" && f.Video.ID == before.videoID() {
			bad("the old video still there (%s)", f.Video.ID)
		}
	default:
		if f.Video.ID != before.videoID() {
			bad("the video %s, not the product's %s", orNone(f.Video.ID), orNone(before.videoID()))
		}
	}
	if len(f.Video.errorsOrNil()) > 0 {
		bad("Video Produk says: %s", strings.Join(f.Video.Errors, "; "))
	}
	if len(f.Errors) > 0 {
		bad("errors: %s", strings.Join(f.Errors, "; "))
	}
	return lines, problems
}

func (v *videoState) errorsOrNil() []string {
	if v == nil {
		return nil
	}
	return v.Errors
}

// updateSave presses Update, answers Seller Centre's offer to nominate the
// product for a programme with Update Saja (update only), and waits until
// the product's detail has the page's photos and video.
func (t *session) updateSave(ctx context.Context, id int64, f *mediaPage) (*detail, error) {
	if _, err := t.updateButton(ctx); err != nil {
		return nil, err
	}
	t.Log("pressing Update")
	if err := t.Press(ctx, "Update", "pressEditButton", "Update"); err != nil {
		return nil, err
	}
	var d *detail
	answered := false
	err := t.Until(ctx, "Seller Centre to save the product", 3*time.Minute, func() (bool, error) {
		var m *modal
		if err := t.Call(ctx, &m, "modal"); err != nil {
			if ctx.Err() == nil && connect.CodeOf(err) == connect.CodeAborted {
				return false, nil
			}
			return false, err
		}
		if m != nil && strings.TrimSpace(m.Text) != "" {
			switch {
			case slices.Contains(m.Buttons, "Update Saja"):
				if !answered {
					t.Log("Seller Centre offers to nominate the product (" + clip(m.Title, 60) + "): answering Update Saja")
					if err := t.answer(ctx, "Update Saja", "Update Saja"); err != nil {
						return false, err
					}
					answered = true
				}
				return false, nil
			default:
				return false, fmt.Errorf("%w: after Update Seller Centre asks: %q; it is left to you in the tab", ErrRefused, clip(m.Text, 300))
			}
		}
		var errs []string
		if err := t.Call(ctx, &errs, "errors"); err == nil && len(errs) > 0 {
			return false, fmt.Errorf("%w: after Update the page says: %s", ErrRefused, strings.Join(errs, "; "))
		}
		var err error
		if d, err = t.detail(ctx, id); err != nil {
			return false, err
		}
		if slices.Equal(d.Images, f.Photos) && savedVideo(d, f.Video) {
			return true, nil
		}
		return false, page.Sleep(ctx, 2*time.Second)
	})
	if err != nil {
		if d != nil {
			err = fmt.Errorf("%w (the product has photos %v, video %s)", err, d.Images, orNone(d.videoID()))
		}
		return nil, t.refusal(ctx, err)
	}
	t.Log(fmt.Sprintf("saved: %d photos, video %s", len(d.Images), orNone(d.videoID())))
	return d, nil
}

// savedVideo says whether the product's detail has the video the page
// had: none, or the page's (by id, once Seller Centre showed it).
func savedVideo(d *detail, v *videoState) bool {
	switch {
	case v == nil || v.State == "empty":
		return d.videoID() == ""
	case v.ID != "":
		return d.videoID() == v.ID
	}
	return d.videoID() != ""
}
