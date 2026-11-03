package shopee

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// mp4Box is an MP4 box: a 32-bit size, the type, then the body.
func mp4Box(kind string, body ...[]byte) []byte {
	b := binary.BigEndian.AppendUint32(nil, uint32(8+len(bytes.Join(body, nil))))
	return append(append(b, kind...), bytes.Join(body, nil)...)
}

// mvhd is a movie header of version 0 (32-bit times and duration) or 1
// (64-bit), with the rest of its fields zero.
func mvhd(version byte, timescale uint32, units uint64) []byte {
	b := []byte{version, 0, 0, 0}
	if version == 1 {
		b = append(b, make([]byte, 16)...)
		b = binary.BigEndian.AppendUint32(b, timescale)
		b = binary.BigEndian.AppendUint64(b, units)
	} else {
		b = append(b, make([]byte, 8)...)
		b = binary.BigEndian.AppendUint32(b, timescale)
		b = binary.BigEndian.AppendUint32(b, uint32(units))
	}
	return mp4Box("mvhd", append(b, make([]byte, 80)...))
}

var ftyp = mp4Box("ftyp", []byte("isom\x00\x00\x02\x00isomiso2avc1mp41"))

// mp4File writes a small MP4 that plays d (no media in it: only its boxes)
// and returns its path.
func mp4File(t *testing.T, dir, name string, d time.Duration) string {
	t.Helper()
	b := append(slices.Clone(ftyp), mp4Box("moov", mvhd(0, 1000, uint64(d/time.Millisecond)), mp4Box("trak"))...)
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestMP4Duration(t *testing.T) {
	mdat := mp4Box("mdat", make([]byte, 1000))
	// A 64-bit size: size 1, the type, then the size.
	big := binary.BigEndian.AppendUint32(nil, 1)
	big = append(big, "free"...)
	big = binary.BigEndian.AppendUint64(big, 16+5)
	big = append(big, "12345"...)
	mehd := func(units uint32) []byte {
		return mp4Box("mvex", mp4Box("mehd", binary.BigEndian.AppendUint32([]byte{0, 0, 0, 0}, units)))
	}
	for name, c := range map[string]struct {
		file []byte
		want time.Duration
	}{
		"version 0":           {bytes.Join([][]byte{ftyp, mp4Box("moov", mvhd(0, 1000, 15000))}, nil), 15 * time.Second},
		"version 1":           {bytes.Join([][]byte{ftyp, mp4Box("moov", mvhd(1, 90000, 90000*44+45000))}, nil), 44500 * time.Millisecond},
		"moov after the data": {bytes.Join([][]byte{ftyp, mdat, mp4Box("moov", mp4Box("trak"), mvhd(0, 600, 600*12))}, nil), 12 * time.Second},
		"a 64-bit box before": {bytes.Join([][]byte{ftyp, big, mp4Box("moov", mvhd(0, 25, 25*30))}, nil), 30 * time.Second},
		"fragmented":          {bytes.Join([][]byte{ftyp, mp4Box("moov", mvhd(0, 1000, 0), mehd(20000)), mp4Box("moof")}, nil), 20 * time.Second},
		"a 1/3 second":        {bytes.Join([][]byte{ftyp, mp4Box("moov", mvhd(0, 3, 31))}, nil), 10333 * time.Millisecond},
	} {
		got, err := mp4Duration(bytes.NewReader(c.file), int64(len(c.file)))
		if err != nil || got != c.want {
			t.Errorf("%s: %v %v, want %v", name, got, err, c.want)
		}
	}
	whole := bytes.Join([][]byte{ftyp, mp4Box("moov", mvhd(0, 1000, 15000))}, nil)
	for name, file := range map[string][]byte{
		"empty":            nil,
		"no moov":          bytes.Join([][]byte{ftyp, mdat}, nil),
		"no mvhd":          bytes.Join([][]byte{ftyp, mp4Box("moov", mp4Box("trak"))}, nil),
		"cut short":        whole[:len(whole)-40],
		"a jpg":            {0xff, 0xd8, 0xff, 0xe0, 0, 0x10, 'J', 'F', 'I', 'F', 0, 1, 1, 0, 0, 1},
		"no duration":      bytes.Join([][]byte{ftyp, mp4Box("moov", mvhd(0, 1000, 0))}, nil),
		"timescale 0":      bytes.Join([][]byte{ftyp, mp4Box("moov", mvhd(0, 0, 15000))}, nil),
		"unknown version":  bytes.Join([][]byte{ftyp, mp4Box("moov", append(mvhd(0, 1000, 15000)[:8], append([]byte{7}, mvhd(0, 1000, 15000)[9:]...)...))}, nil),
		"a box too small":  append(slices.Clone(ftyp), 0, 0, 0, 3, 'm', 'o', 'o', 'v'),
		"a box too big":    append(slices.Clone(ftyp), 0, 0, 1, 0, 'm', 'o', 'o', 'v'),
		"a 64-bit too big": append(slices.Clone(ftyp), 0, 0, 0, 1, 'm', 'o', 'o', 'v', 0, 0, 0, 0, 0, 0, 1, 0),
	} {
		if d, err := mp4Duration(bytes.NewReader(file), int64(len(file))); !errors.Is(err, errNoDuration) {
			t.Errorf("%s: %v %v", name, d, err)
		}
	}
}

// The file's own length is read, and a video Seller Centre would refuse
// (or trim) is refused before a browser opens.
func TestCheckVideo(t *testing.T) {
	dir := t.TempDir()
	good := mp4File(t, dir, "tes.mp4", 15*time.Second)
	if d, err := CheckVideo(good); err != nil || d != 15*time.Second {
		t.Errorf("good: %v %v", d, err)
	}
	if d, err := CheckVideo(mp4File(t, dir, "UPPER.MP4", 60*time.Second)); err != nil || d != time.Minute {
		t.Errorf("60 s: %v %v", d, err)
	}
	big := filepath.Join(dir, "big.mp4")
	if err := os.WriteFile(big, append(must(os.ReadFile(good)), mp4Box("free", make([]byte, MaxVideoBytes))...), 0o644); err != nil {
		t.Fatal(err)
	}
	mov := filepath.Join(dir, "tes.mov")
	if err := os.WriteFile(mov, must(os.ReadFile(good)), 0o644); err != nil {
		t.Fatal(err)
	}
	notMP4 := filepath.Join(dir, "photo.mp4")
	if err := os.WriteFile(notMP4, []byte("not a video at all"), 0o644); err != nil {
		t.Fatal(err)
	}
	for name, c := range map[string]struct{ path, says string }{
		"5 s":       {mp4File(t, dir, "short.mp4", 5*time.Second), "plays 5s"},
		"61 s":      {mp4File(t, dir, "long.mp4", 61*time.Second), "plays 1m1s"},
		"too big":   {big, "30 MB"},
		"a .mov":    {mov, "not an .mp4"},
		"not mp4":   {notMP4, "photo.mp4"},
		"missing":   {filepath.Join(dir, "none.mp4"), "none.mp4"},
		"a folder":  {mkdir(t, dir, "dir.mp4"), "not a file"},
		"just 9.9s": {mp4File(t, dir, "almost.mp4", 9900*time.Millisecond), "plays 9.9s"},
	} {
		_, err := CheckVideo(c.path)
		if !errors.Is(err, ErrInvalid) || !strings.Contains(err.Error(), c.says) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

func mkdir(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// The video in a product file is relative to the file, checked with the
// rest, and in the plan.
func TestParseFileVideo(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "img"), 0o755); err != nil {
		t.Fatal(err)
	}
	photo(t, dir, "img/1.png", 500, 500)
	photo(t, dir, "img/2.png", 500, 500)
	mp4File(t, dir, "img/tes.mp4", 15*time.Second)
	md := strings.Replace(productMD, "photos: [img/1.png, img/2.png]\n", "photos: [img/1.png, img/2.png]\nvideo: img/tes.mp4\n", 1)
	path := filepath.Join(dir, "p.md")
	if err := os.WriteFile(path, []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}
	n, err := ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if n.Video != filepath.Join(dir, "img", "tes.mp4") {
		t.Errorf("video %q", n.Video)
	}
	models, err := n.Check()
	if err != nil {
		t.Fatal(err)
	}
	if plan := n.Summary(models); !slices.Contains(plan, "video: "+n.Video+" (15s)") {
		t.Errorf("plan: %v", plan)
	}
	mp4File(t, dir, "img/short.mp4", 4*time.Second)
	n.Video = filepath.Join(dir, "img", "short.mp4")
	if _, err := n.Check(); !errors.Is(err, ErrInvalid) || !strings.Contains(err.Error(), "10 to 60 seconds") {
		t.Errorf("a short video: %v", err)
	}
}

func TestMediaCheck(t *testing.T) {
	dir := t.TempDir()
	p1, p2 := photo(t, dir, "1.png", 500, 500), photo(t, dir, "2.png", 500, 500)
	wide := photo(t, dir, "wide.png", 800, 500)
	video := mp4File(t, dir, "tes.mp4", 15*time.Second)
	short := mp4File(t, dir, "short.mp4", 5*time.Second)
	ten := slices.Repeat([]string{p1}, 10)
	for name, m := range map[string]Media{
		"photos":       {ID: 1, Photos: []string{p1, p2}},
		"a video":      {ID: 1, Video: video},
		"both":         {ID: 1, Photos: []string{p2}, Video: video},
		"no video":     {ID: 1, RemoveVideo: true},
		"photos, none": {ID: 1, Photos: []string{p1}, RemoveVideo: true},
		"nine":         {ID: 1, Photos: ten[:9]},
	} {
		if err := m.Check(); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	for name, c := range map[string]struct {
		m    Media
		says string
	}{
		"no id":         {Media{Photos: []string{p1}}, "no product id"},
		"nothing":       {Media{ID: 1}, "nothing to change"},
		"video and not": {Media{ID: 1, Video: video, RemoveVideo: true}, "not both"},
		"ten photos":    {Media{ID: 1, Photos: ten}, "10 photos"},
		"not square":    {Media{ID: 1, Photos: []string{p1, wide}}, "square"},
		"a short video": {Media{ID: 1, Video: short}, "plays 5s"},
		"no such photo": {Media{ID: 1, Photos: []string{filepath.Join(dir, "none.png")}}, "none.png"},
	} {
		if err := c.m.Check(); !errors.Is(err, ErrInvalid) || !strings.Contains(err.Error(), c.says) {
			t.Errorf("%s: %v", name, err)
		}
	}
	m := Media{ID: 7, Photos: []string{p1, p2}, Video: video}
	if got := strings.Join(m.Summary(), "\n"); !strings.Contains(got, "photo 2: "+p2) || !strings.Contains(got, "(15s), in place of the product's") || !strings.Contains(got, "save: Update") {
		t.Errorf("summary:\n%s", got)
	}
	if got := (&Media{ID: 7, RemoveVideo: true}).Summary(); !slices.Contains(got, "photos: kept") || !slices.Contains(got, "video: taken away") {
		t.Errorf("summary: %v", got)
	}
}

// A product's detail gives its photos' links and its video.
func TestDetail(t *testing.T) {
	d := &detail{Found: true, Images: []string{"a", "b"}}
	var p Product
	d.apply(&p)
	if !p.MediaRead || p.Video != nil || !slices.Equal(p.Photos, []string{photoLink("a"), photoLink("b")}) {
		t.Errorf("no video: %+v", p)
	}
	d.Videos = append(d.Videos, struct {
		ID       string `json:"id"`
		Duration int64  `json:"duration"`
		Cover    string `json:"cover"`
	}{"vid", 44, "vid_cover"})
	d.apply(&p)
	if p.Video == nil || p.Video.ID != "vid" || p.Video.Duration != 44*time.Second || p.Video.Cover != photoLink("vid_cover") {
		t.Errorf("a video: %+v", p.Video)
	}
	// The saved product has the page's video: none, the very one, or, when
	// the page showed no id yet, some video.
	for name, c := range map[string]struct {
		v    *videoState
		want bool
	}{
		"the same":        {&videoState{State: "ready", ID: "vid"}, true},
		"another":         {&videoState{State: "ready", ID: "new"}, false},
		"processing":      {&videoState{State: "processing"}, true},
		"empty, has one":  {&videoState{State: "empty"}, false},
		"no field at all": {nil, false},
	} {
		if got := savedVideo(d, c.v); got != c.want {
			t.Errorf("%s: %v", name, got)
		}
	}
	if !savedVideo(&detail{}, &videoState{State: "empty"}) {
		t.Error("none saved as none")
	}
}

// The page is saved only as it was asked: the new photos in their order
// with photo 1 the cover, the new video taken (or none), nothing else.
func TestMediaPageCheck(t *testing.T) {
	before := &detail{Found: true, Images: []string{"old1", "old2"}}
	before.Videos = append(before.Videos, struct {
		ID       string `json:"id"`
		Duration int64  `json:"duration"`
		Cover    string `json:"cover"`
	}{"oldvid", 15, ""})
	newPhotos := []string{"n1", "n2", "n3"}
	m := Media{ID: 1, Photos: []string{"a.png", "b.png", "c.png"}, Video: "tes.mp4"}
	good := func() mediaPage {
		return mediaPage{Name: "TES", Photos: slices.Clone(newPhotos), Cover: "n1", Video: &videoState{State: "ready", ID: "newvid", Duration: "00:15"}}
	}
	f := good()
	lines, problems := f.check(m, newPhotos, before, "TES")
	if len(problems) > 0 {
		t.Fatalf("a good page: %v", problems)
	}
	if !slices.Contains(lines, "photo 3: n3") || !slices.Contains(lines, "video: ready 00:15 (newvid)") {
		t.Errorf("lines: %v", lines)
	}
	for name, change := range map[string]func(*mediaPage){
		"an old photo left": func(f *mediaPage) { f.Photos = append(f.Photos, "old1") },
		"a photo short":     func(f *mediaPage) { f.Photos = f.Photos[:2] },
		"photos swapped":    func(f *mediaPage) { f.Photos[0], f.Photos[1] = f.Photos[1], f.Photos[0] },
		"the cover":         func(f *mediaPage) { f.Cover = "n2" },
		"the old video":     func(f *mediaPage) { f.Video.ID = "oldvid" },
		"the video not in":  func(f *mediaPage) { f.Video = &videoState{State: "uploading", Progress: "40%"} },
		"no Video Produk":   func(f *mediaPage) { f.Video = nil },
		"Video Produk says": func(f *mediaPage) { f.Video.Errors = []string{"Gagal mengunggah"} },
		"an error":          func(f *mediaPage) { f.Errors = []string{"Nama produk: wajib diisi"} },
		"another product":   func(f *mediaPage) { f.Name = "Software Aplikasi Kasir" },
	} {
		f := good()
		change(&f)
		if _, problems := f.check(m, newPhotos, before, "TES"); len(problems) == 0 {
			t.Errorf("%s: passed", name)
		}
	}
	// Photos and video kept: the page must read as the product does.
	keep := Media{ID: 1, RemoveVideo: true}
	f = mediaPage{Name: "TES", Photos: []string{"old1", "old2"}, Cover: "old1", Video: &videoState{State: "empty"}}
	if _, problems := f.check(keep, before.Images, before, "TES"); len(problems) > 0 {
		t.Errorf("video taken away: %v", problems)
	}
	f.Video = &videoState{State: "ready", ID: "oldvid"}
	if _, problems := f.check(keep, before.Images, before, "TES"); len(problems) == 0 {
		t.Error("the video still there: passed")
	}
	f.Photos = []string{"old2", "old1"}
	if _, problems := f.check(Media{ID: 1, Video: "tes.mp4"}, before.Images, before, "TES"); len(problems) == 0 {
		t.Error("kept photos out of order: passed")
	}
}

// The upload form's check knows the video: taken when given, none when
// not.
func TestFormCheckVideo(t *testing.T) {
	one := int64(1)
	n := New{
		Name: "TES", Description: "Baris.", Photos: []string{"a.png"}, Category: []string{"Komputer & Aksesoris", "Software"},
		Attributes: map[string]string{"Merek": "Tidak ada merek"}, Price: 9999000, Stock: &one, Weight: 100, Video: "tes.mp4",
	}
	models := []Model{{Price: 9999000, Stock: 1}}
	good := func() formView {
		return formView{
			Photos: 1, Cover: "https://down-id.img.susercontent.com/file/x", Name: "TES", Category: "Komputer & Aksesoris > Software",
			Attributes:  []attribute{{Name: "Merek", Required: true, Kind: "list", Value: "Tidak ada merek"}},
			Description: "Baris.", Price: "9999000", Stock: "1",
			Weight: "100", Danger: "0", Couriers: []courier{{Name: "Reguler (Cashless)", On: true}}, Condition: "Baru",
			Video: &videoState{State: "processing", ID: "vid"},
		}
	}
	v := good()
	lines, problems := v.check(n, models)
	if len(problems) > 0 {
		t.Fatalf("a good form: %v", problems)
	}
	if !slices.Contains(lines, "video: processing (vid)") {
		t.Errorf("lines: %v", lines)
	}
	for name, change := range map[string]func(*formView){
		"not taken":   func(v *formView) { v.Video.State = "empty" },
		"uploading":   func(v *formView) { v.Video.State = "uploading" },
		"no field":    func(v *formView) { v.Video = nil },
		"it says":     func(v *formView) { v.Video.Errors = []string{"Durasi video tidak sesuai"} },
		"none wanted": func(v *formView) { n.Video = "" },
	} {
		v := good()
		change(&v)
		if _, problems := v.check(n, models); len(problems) == 0 {
			t.Errorf("%s: passed", name)
		}
		n.Video = "tes.mp4"
	}
}
