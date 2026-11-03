// Package shopee works on the user's Shopee shop through Shopee Seller
// Centre (seller.shopee.co.id) in the user's own browser, the way a person
// does there: no Shopee Open Platform API. It adds a product with its
// photos, video, category, attributes, description and variations (Tambah
// Produk Baru), replaces the photos and the video of a product it has
// (its page in Seller Centre), deletes, archives and shows products, bumps
// one to the top (Naikkan Produk), and lists the products and the bumped
// ones.
//
// Seller Centre's interface is read in Indonesian (the account's language,
// "id"): the steps press Lainnya, Hapus, Naikkan Produk, Simpan & Arsipkan.
// The page knowledge is in shopee.js; the steps, waits and pace are in Go
// (service.go, create.go), on internal/page. What the steps read (the
// product list, the bumped slots, the shop) comes from the requests Seller
// Centre's own pages make, sent from the page; every change is made by
// pressing the page's buttons.
package shopee

import (
	"errors"
	"fmt"
	"image"
	_ "image/jpeg" // photos are checked for their size
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/wargasipil/san_remote/internal/plaintext"
	"gopkg.in/yaml.v3"
)

var (
	// ErrInvalid is a product Seller Centre would refuse, or one the tool
	// cannot make: no photos, a name too long, a price under Rp 99, a model
	// without a price.
	ErrInvalid = errors.New("shopee: the product is not valid")
	// ErrNotSignedIn means the browser has no Seller Centre session.
	ErrNotSignedIn = errors.New("shopee: the browser is not signed in to Shopee Seller Centre")
	// ErrLanguage means Seller Centre's interface is not in Indonesian,
	// which the steps read.
	ErrLanguage = errors.New("shopee: Seller Centre's interface is not in Indonesian")
	// ErrNotFound means the shop has no product with that id.
	ErrNotFound = errors.New("shopee: no such product")
	// ErrNotLive means the product is not live (archived, under review),
	// and Naikkan Produk is only for live ones.
	ErrNotLive = errors.New("shopee: the product is not live")
	// ErrCooldown means the product was bumped less than 4 hours ago.
	ErrCooldown = errors.New("shopee: the product was bumped less than 4 hours ago")
	// ErrSlotsFull means all 5 Naikkan Produk slots are in use.
	ErrSlotsFull = errors.New("shopee: all Naikkan Produk slots are in use")
	// ErrRefused means Seller Centre refused what was asked, with its own
	// words: a form error after Simpan, a toast.
	ErrRefused = errors.New("shopee: Seller Centre refused")
	// ErrPage means Seller Centre did not do what a step expected: an
	// element never appeared, or a change did not take. The page may have
	// been redesigned.
	ErrPage = errors.New("shopee: Seller Centre did not respond as expected")
)

// Seller Centre's limits, as Tambah Produk Baru applies them (2026-11-03).
const (
	MaxName        = 255
	MaxDescription = 3000
	MaxPhotos      = 9
	MaxPhotoBytes  = 2 << 20 // "masing-masing maksimal 2.0 MB"
	MaxVariations  = 2
	MaxTierName    = 14 // "The variation name is too long" at 15
	MaxOption      = 20 // "nilai antara 1-20 karakter"
	MaxOptions     = 50
	MinPrice       = 99 // "Nilai yang harus diisi setidaknya 99"
	MaxPrice       = 150_000_000
	MaxStock       = 9_999_999
	MaxWeight      = 1_000_000 // grams
	MaxSKU         = 100
	// Video Produk: "Size: Max 30MB", "Durasi: 10-60detik", "Format: MP4".
	MaxVideoBytes = 30 << 20
	MinVideo      = 10 * time.Second
	MaxVideo      = 60 * time.Second
)

// photoTypes are what the tool hands Seller Centre's file inputs, by
// extension: "Format foto yang diperbolehkan: jpg,jpeg,png".
var photoTypes = map[string]string{".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".png": "image/png"}

func photoType(name string) string { return photoTypes[strings.ToLower(filepath.Ext(name))] }

// fileType is a photo's or a video's type, as the file inputs are given it.
func fileType(name string) string {
	if strings.EqualFold(filepath.Ext(name), ".mp4") {
		return "video/mp4"
	}
	return photoType(name)
}

// Variation is a variation of a product (Variasi) and its options, in order.
type Variation struct {
	Name    string   `yaml:"name"`
	Options []string `yaml:"options"`
}

// NewModel gives one combination of the options its price, stock and SKU;
// a nil Price or Stock takes the product's.
type NewModel struct {
	Options []string `yaml:"options"`
	Price   *int64   `yaml:"price"`
	Stock   *int64   `yaml:"stock"`
	SKU     string   `yaml:"sku"`
}

// Dimensions is the parcel's size in centimetres.
type Dimensions struct{ Length, Width, Height int }

// New is a product to add.
type New struct {
	Browser     string
	Name        string
	Description string
	// Photos are files, in order. The first is the cover (Cover Produk):
	// Seller Centre fills the cover with it and locks it there.
	Photos []string
	// Video is an MP4 file of 10 to 60 seconds and up to 30 MB (Video
	// Produk), or "" for none.
	Video    string
	Category []string
	// Attributes by their names on the form, as typed or picked.
	Attributes map[string]string
	Variations []Variation
	Models     []NewModel
	// Price and Stock are the product's, or the models' default; Stock nil
	// is none given.
	Price      int64
	Stock      *int64
	SKU        string
	Weight     int // grams
	Dimensions Dimensions
	Used       bool // Kondisi Pernah Dipakai; Baru otherwise
	Couriers   []string
	Dangerous  bool
	// Live saves it with Simpan & Tampilkan; Simpan & Arsipkan otherwise.
	Live bool
	// Rehearse does every step but Simpan.
	Rehearse bool
}

// Media is what to change in the photos and the video of a product the
// shop has (UpdateMedia), on its page in Seller Centre.
type Media struct {
	Browser string
	ID      int64
	// Photos are the whole new gallery, in order (1 to 9; the first is the
	// cover, as Seller Centre locks it to photo 1); none keeps the photos.
	Photos []string
	// Video replaces the product's video, or adds one; "" keeps it.
	Video string
	// RemoveVideo takes the product's video away.
	RemoveVideo bool
	// Rehearse does every step but Update, and leaves the page unsaved.
	Rehearse bool
}

// Check reports what Seller Centre would refuse, or what cannot be asked,
// before any browser is opened.
func (m *Media) Check() error {
	bad := func(format string, a ...any) error {
		return fmt.Errorf("%w: "+format, append([]any{ErrInvalid}, a...)...)
	}
	switch {
	case m.ID <= 0:
		return bad("no product id")
	case len(m.Photos) == 0 && m.Video == "" && !m.RemoveVideo:
		return bad("nothing to change: give the photos, a video, or remove the video")
	case m.Video != "" && m.RemoveVideo:
		return bad("give a video or remove the video, not both")
	case len(m.Photos) > MaxPhotos:
		return bad("%d photos; Seller Centre takes up to %d", len(m.Photos), MaxPhotos)
	}
	for _, p := range m.Photos {
		if err := checkPhoto(p); err != nil {
			return err
		}
	}
	if m.Video != "" {
		if _, err := CheckVideo(m.Video); err != nil {
			return err
		}
	}
	return nil
}

// Summary is the change as it will be made, line by line, for --dry-run.
func (m *Media) Summary() []string {
	out := []string{fmt.Sprintf("product: %d", m.ID)}
	if len(m.Photos) == 0 {
		out = append(out, "photos: kept")
	}
	for i, p := range m.Photos {
		out = append(out, fmt.Sprintf("photo %d: %s", i+1, p))
	}
	if len(m.Photos) > 0 {
		out = append(out, "cover: photo 1 (Seller Centre's choice)")
	}
	switch {
	case m.Video != "":
		line := "video: " + m.Video
		if d, err := MP4Duration(m.Video); err == nil {
			line += fmt.Sprintf(" (%s)", d.Round(100*time.Millisecond))
		}
		out = append(out, line+", in place of the product's")
	case m.RemoveVideo:
		out = append(out, "video: taken away")
	default:
		out = append(out, "video: kept")
	}
	return append(out, "save: Update (the product stays live or archived as it is)")
}

// photoLink is a photo's or picture's link from its id in Seller Centre.
func photoLink(id string) string {
	if id == "" || strings.Contains(id, "://") {
		return id
	}
	return "https://down-id.img.susercontent.com/file/" + id
}

// photoID is the id in a photo's link: the link's last part.
func photoID(link string) string {
	link, _, _ = strings.Cut(link, "?")
	return link[strings.LastIndex(link, "/")+1:]
}

// Model is one row of the variations' table as it will be filled: a
// combination of the options with its price, stock and SKU.
type Model struct {
	ID      int64
	Options []string
	Price   int64
	Stock   int64
	SKU     string
}

// Product is a product as Seller Centre lists it.
type Product struct {
	ID       int64
	Name     string
	Status   string // live, archived, or "status N"
	PriceMin int64
	PriceMax int64
	Stock    int64
	Sold     int64
	SKU      string
	Models   []Model
	Cover    string
	Created  time.Time
	Updated  time.Time
	// Photos are the photos' links, in order, and Video the product's
	// video (nil for none): read from the product's own page (its detail),
	// so only where MediaRead says they were read.
	Photos    []string
	Video     *Video
	MediaRead bool
}

// Video is a product's video as Seller Centre keeps it.
type Video struct {
	// ID is Seller Centre's, "id-11110105-6vfes-mqyi4q5mk4xsf0".
	ID       string
	Duration time.Duration
	// Cover is its picture's link.
	Cover string
}

// URL is the product's page in Seller Centre.
func (p *Product) URL() string { return fmt.Sprintf("%s/portal/product/%d", origin, p.ID) }

// Status is the browser's Seller Centre session.
type Status struct {
	SignedIn bool
	Shop     string
	Username string
	ShopID   int64
}

// Boost is a bumped product's slot.
type Boost struct {
	ID   int64
	Name string // empty for a product that is gone
	// EndsAt is when the slot is free again.
	EndsAt      time.Time
	SecondsLeft int64
}

// Boosts are Naikkan Produk's slots.
type Boosts struct {
	Boosts   []Boost
	Total    int
	Cooldown time.Duration
}

// Filters are ListProducts' tabs, by the list type Seller Centre's own
// requests name them.
var Filters = map[string]string{
	"all":       "all",       // Semua
	"live":      "live_all",  // Live
	"archived":  "delisted",  // Belum Ditampilkan
	"review":    "reviewing", // Sedang Ditinjau Shopee
	"violation": "banned",    // Perlu Tindakan
	"draft":     "draft",
}

// file is a product file's frontmatter.
type file struct {
	Name       string            `yaml:"name"`
	Category   yaml.Node         `yaml:"category"`
	Photos     []string          `yaml:"photos"`
	Video      string            `yaml:"video"`
	Attributes map[string]string `yaml:"attributes"`
	Variations []Variation       `yaml:"variations"`
	Models     []NewModel        `yaml:"models"`
	Price      int64             `yaml:"price"`
	Stock      *int64            `yaml:"stock"`
	SKU        string            `yaml:"sku"`
	Weight     int               `yaml:"weight"`
	Dimensions string            `yaml:"dimensions"`
	Condition  string            `yaml:"condition"`
	Couriers   []string          `yaml:"couriers"`
	Dangerous  bool              `yaml:"dangerous"`
	Publish    string            `yaml:"publish"`
}

// ParseFile reads a product from a markdown file:
//
//	---
//	name: Kaos Polos Katun
//	category: Pakaian Pria > Atasan > Kaos
//	photos: [photos/1.jpg, photos/2.jpg]
//	video: photos/kaos.mp4
//	attributes:
//	  Merek: Tidak ada merek
//	weight: 200
//	price: 75000
//	stock: 10
//	variations:
//	  - name: Warna
//	    options: [Merah, Biru]
//	  - name: Ukuran
//	    options: [S, M]
//	models:
//	  - options: [Merah, S]
//	    sku: KAOS-MERAH-S
//	---
//
//	The description, as plain text.
//
// Photos and the video are relative to the file.
func ParseFile(path string) (*New, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("shopee: %w", err)
	}
	n, err := Parse(b)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	dir := filepath.Dir(path)
	for i, p := range n.Photos {
		n.Photos[i] = relTo(dir, p)
	}
	n.Video = relTo(dir, n.Video)
	return n, nil
}

func relTo(dir, p string) string {
	if p == "" || filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(dir, filepath.FromSlash(p))
}

// Parse reads a product from markdown, as ParseFile does, leaving its
// photos and video as the frontmatter gives them.
func Parse(md []byte) (*New, error) {
	text := strings.ReplaceAll(string(md), "\r\n", "\n")
	text = strings.TrimPrefix(text, string(rune(0xFEFF)))
	var f file
	if rest, ok := strings.CutPrefix(text, "---\n"); ok {
		head, body, found := strings.Cut("\n"+rest, "\n---\n")
		if !found {
			head, found = strings.CutSuffix("\n"+rest, "\n---")
			body = ""
		}
		if !found {
			return nil, fmt.Errorf("%w: the frontmatter has no closing ---", ErrInvalid)
		}
		if err := yaml.NewDecoder(strings.NewReader(head)).Decode(&f); err != nil && !errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("%w: frontmatter: %v", ErrInvalid, err)
		}
		text = body
	}
	n := &New{
		Name: strings.TrimSpace(f.Name), Description: plaintext.From(text), Photos: f.Photos, Video: strings.TrimSpace(f.Video),
		Attributes: f.Attributes, Variations: f.Variations, Models: f.Models, Price: f.Price, Stock: f.Stock,
		SKU: f.SKU, Weight: f.Weight, Couriers: f.Couriers, Dangerous: f.Dangerous,
	}
	switch f.Category.Kind {
	case 0:
	case yaml.ScalarNode:
		n.Category = SplitCategory(f.Category.Value)
	case yaml.SequenceNode:
		var path []string
		if err := f.Category.Decode(&path); err != nil {
			return nil, fmt.Errorf("%w: category: %v", ErrInvalid, err)
		}
		n.Category = SplitCategory(strings.Join(path, ">"))
	default:
		return nil, fmt.Errorf("%w: category: give \"A > B > C\" or a list", ErrInvalid)
	}
	if f.Dimensions != "" {
		d, err := ParseDimensions(f.Dimensions)
		if err != nil {
			return nil, err
		}
		n.Dimensions = d
	}
	var err error
	if n.Used, err = ParseCondition(f.Condition); err != nil {
		return nil, err
	}
	if n.Live, err = ParsePublish(f.Publish); err != nil {
		return nil, err
	}
	return n, nil
}

// SplitCategory reads a category path: "Komputer & Aksesoris > Software".
func SplitCategory(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ">") {
		if p = strings.Join(strings.Fields(p), " "); p != "" {
			out = append(out, p)
		}
	}
	return out
}

var dims = regexp.MustCompile(`^\s*(\d+)\s*[xX×*]\s*(\d+)\s*[xX×*]\s*(\d+)\s*(?:cm)?\s*$`)

// ParseDimensions reads a parcel's size, "length x width x height" in
// centimetres: "20x10x5".
func ParseDimensions(s string) (Dimensions, error) {
	m := dims.FindStringSubmatch(s)
	if m == nil {
		return Dimensions{}, fmt.Errorf("%w: dimensions %q: give \"length x width x height\" in cm, such as 20x10x5", ErrInvalid, s)
	}
	l, _ := strconv.Atoi(m[1])
	w, _ := strconv.Atoi(m[2])
	h, _ := strconv.Atoi(m[3])
	return Dimensions{Length: l, Width: w, Height: h}, nil
}

// ParseCondition reads a condition: new (baru, the default) or used (bekas);
// it reports whether it is used.
func ParseCondition(s string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "new", "baru":
		return false, nil
	case "used", "bekas", "pernah dipakai":
		return true, nil
	}
	return false, fmt.Errorf("%w: condition %q: give new or used", ErrInvalid, s)
}

// ParsePublish reads how a product is saved: archived (the default) or
// live; it reports whether it is live.
func ParsePublish(s string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "archived", "archive", "arsip", "arsipkan":
		return false, nil
	case "live", "tampilkan":
		return true, nil
	}
	return false, fmt.Errorf("%w: publish %q: give archived or live", ErrInvalid, s)
}

// ParseID reads a product id, or a link to its page in Seller Centre
// (https://seller.shopee.co.id/portal/product/57614536675), or its page in
// the shop (…-i.77987996.57614536675).
func ParseID(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if m := regexp.MustCompile(`/portal/product/(\d+)`).FindStringSubmatch(s); m != nil {
		s = m[1]
	} else if m := regexp.MustCompile(`[-.]i\.\d+\.(\d+)`).FindStringSubmatch(s); m != nil {
		s = m[1]
	}
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("%w: %q is not a product id or link", ErrInvalid, s)
	}
	return id, nil
}

// Check reports what Seller Centre would refuse, or what the tool cannot
// make, before any browser is opened, and returns the variations' table
// as it will be filled: every combination of the options, in the table's
// order (the first variation's options outermost), each with its price,
// stock and SKU. A product without variations is one model.
func (n *New) Check() ([]Model, error) {
	bad := func(format string, a ...any) error {
		return fmt.Errorf("%w: "+format, append([]any{ErrInvalid}, a...)...)
	}
	switch c := utf8.RuneCountInString(n.Name); {
	case strings.TrimSpace(n.Name) == "":
		return nil, bad("no name")
	case c > MaxName:
		return nil, bad("the name has %d characters, more than Seller Centre's %d", c, MaxName)
	case strings.ContainsAny(n.Name, "\n\r"):
		return nil, bad("the name holds a line break")
	}
	switch c := utf8.RuneCountInString(n.Description); {
	case strings.TrimSpace(n.Description) == "":
		return nil, bad("no description (Deskripsi Produk is required)")
	case c > MaxDescription:
		return nil, bad("the description has %d characters, more than Seller Centre's %d", c, MaxDescription)
	}
	if len(n.Photos) == 0 {
		return nil, bad("no photos")
	}
	if len(n.Photos) > MaxPhotos {
		return nil, bad("%d photos; Seller Centre takes up to %d", len(n.Photos), MaxPhotos)
	}
	for _, p := range n.Photos {
		if err := checkPhoto(p); err != nil {
			return nil, err
		}
	}
	if n.Video != "" {
		if _, err := CheckVideo(n.Video); err != nil {
			return nil, err
		}
	}
	if len(n.Category) == 0 {
		return nil, bad("no category: give its path as the picker names it, such as \"Komputer & Aksesoris > Software\"")
	}
	for name, v := range n.Attributes {
		if strings.TrimSpace(name) == "" || strings.TrimSpace(v) == "" {
			return nil, bad("attribute %q: give a name and a value", name)
		}
	}
	switch {
	case n.Weight <= 0:
		return nil, bad("no weight: give the parcel's weight in grams")
	case n.Weight > MaxWeight:
		return nil, bad("a weight of %d g is more than %d", n.Weight, MaxWeight)
	}
	if d := n.Dimensions; d != (Dimensions{}) && (d.Length <= 0 || d.Width <= 0 || d.Height <= 0) {
		return nil, bad("dimensions %dx%dx%d: give all three, in whole cm", d.Length, d.Width, d.Height)
	}
	if c := utf8.RuneCountInString(n.SKU); c > MaxSKU {
		return nil, bad("the SKU has %d characters, more than %d", c, MaxSKU)
	}
	for _, c := range n.Couriers {
		if strings.TrimSpace(c) == "" {
			return nil, bad("an empty courier name")
		}
	}
	if n.Price != 0 {
		if err := checkPrice(n.Price, "the price"); err != nil {
			return nil, err
		}
	}
	if n.Stock != nil {
		if err := checkStock(*n.Stock, "the stock"); err != nil {
			return nil, err
		}
	}
	if len(n.Variations) == 0 {
		if len(n.Models) > 0 {
			return nil, bad("models are given, but no variations")
		}
		if n.Price == 0 {
			return nil, bad("no price")
		}
		if n.Stock == nil {
			return nil, bad("no stock")
		}
		return []Model{{Price: n.Price, Stock: *n.Stock, SKU: n.SKU}}, nil
	}
	return n.models()
}

func checkPrice(p int64, what string) error {
	if p < MinPrice || p > MaxPrice {
		return fmt.Errorf("%w: %s is Rp %d; Seller Centre takes Rp %d to Rp %d", ErrInvalid, what, p, MinPrice, MaxPrice)
	}
	return nil
}

func checkStock(s int64, what string) error {
	if s < 0 || s > MaxStock {
		return fmt.Errorf("%w: %s is %d; give 0 to %d", ErrInvalid, what, s, MaxStock)
	}
	return nil
}

// checkPhoto checks a photo is a square jpg or png of up to 2 MB: Seller
// Centre's cover is 1:1, and a photo of another shape opens a cropper the
// tool does not drive.
func checkPhoto(path string) error {
	if photoType(path) == "" {
		return fmt.Errorf("%w: %s is not a jpg or png", ErrInvalid, path)
	}
	fi, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	if !fi.Mode().IsRegular() {
		return fmt.Errorf("%w: %s is not a file", ErrInvalid, path)
	}
	if fi.Size() > MaxPhotoBytes {
		return fmt.Errorf("%w: %s is %.1f MB; Seller Centre takes up to 2.0 MB a photo", ErrInvalid, path, float64(fi.Size())/(1<<20))
	}
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	defer f.Close()
	c, _, err := image.DecodeConfig(f)
	if err != nil {
		return fmt.Errorf("%w: %s: %v", ErrInvalid, path, err)
	}
	if c.Width != c.Height {
		return fmt.Errorf("%w: %s is %dx%d; give a square photo (1:1, such as 1024x1024)", ErrInvalid, path, c.Width, c.Height)
	}
	return nil
}

// CheckVideo checks a video is one Video Produk takes: an MP4 of up to 30
// MB that plays 10 to 60 seconds (Seller Centre opens a longer one in a
// trimmer the tool does not drive). It returns how long it plays, read
// from the file's own boxes.
func CheckVideo(path string) (time.Duration, error) {
	if !strings.EqualFold(filepath.Ext(path), ".mp4") {
		return 0, fmt.Errorf("%w: %s is not an .mp4; Video Produk takes MP4 only", ErrInvalid, path)
	}
	fi, err := os.Stat(path)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	if !fi.Mode().IsRegular() {
		return 0, fmt.Errorf("%w: %s is not a file", ErrInvalid, path)
	}
	if fi.Size() > MaxVideoBytes {
		return 0, fmt.Errorf("%w: %s is %.1f MB; Video Produk takes up to 30 MB", ErrInvalid, path, float64(fi.Size())/(1<<20))
	}
	d, err := MP4Duration(path)
	if err != nil {
		return 0, fmt.Errorf("%w: %s: %v", ErrInvalid, path, err)
	}
	if d < MinVideo || d > MaxVideo {
		return d, fmt.Errorf("%w: %s plays %s; Video Produk takes 10 to 60 seconds", ErrInvalid, path, d)
	}
	return d, nil
}

// models checks the variations and works out every combination's model.
func (n *New) models() ([]Model, error) {
	bad := func(format string, a ...any) error {
		return fmt.Errorf("%w: "+format, append([]any{ErrInvalid}, a...)...)
	}
	if len(n.Variations) > MaxVariations {
		return nil, bad("%d variations; Seller Centre takes up to %d", len(n.Variations), MaxVariations)
	}
	names := map[string]bool{}
	for i, v := range n.Variations {
		name := strings.TrimSpace(v.Name)
		switch c := utf8.RuneCountInString(name); {
		case name == "":
			return nil, bad("variation %d has no name", i+1)
		case c > MaxTierName:
			return nil, bad("the variation name %q has %d characters; Seller Centre takes up to %d", name, c, MaxTierName)
		case names[strings.ToLower(name)]:
			return nil, bad("two variations are named %q", name)
		}
		names[strings.ToLower(name)] = true
		if len(v.Options) == 0 {
			return nil, bad("the variation %q has no options", name)
		}
		if len(v.Options) > MaxOptions {
			return nil, bad("the variation %q has %d options; Seller Centre takes up to %d", name, len(v.Options), MaxOptions)
		}
		seen := map[string]bool{}
		for _, o := range v.Options {
			o = strings.TrimSpace(o)
			switch c := utf8.RuneCountInString(o); {
			case o == "":
				return nil, bad("the variation %q has an empty option", name)
			case c > MaxOption:
				return nil, bad("the option %q has %d characters; Seller Centre takes 1 to %d", o, c, MaxOption)
			case strings.Contains(o, ","):
				return nil, bad("the option %q holds a comma, which Seller Centre's list uses between options", o)
			case seen[strings.ToLower(o)]:
				return nil, bad("the variation %q has the option %q twice", name, o)
			}
			seen[strings.ToLower(o)] = true
		}
	}
	// Every combination, the first variation's options outermost, as the
	// table lists them.
	combos := [][]string{{}}
	for _, v := range n.Variations {
		var next [][]string
		for _, c := range combos {
			for _, o := range v.Options {
				next = append(next, append(append([]string{}, c...), strings.TrimSpace(o)))
			}
		}
		combos = next
	}
	key := func(opts []string) string {
		k := make([]string, len(opts))
		for i, o := range opts {
			k[i] = strings.ToLower(strings.TrimSpace(o))
		}
		return strings.Join(k, "\x00")
	}
	given := map[string]NewModel{}
	for _, m := range n.Models {
		if len(m.Options) != len(n.Variations) {
			return nil, bad("the model %v names %d options; give one per variation (%d)", m.Options, len(m.Options), len(n.Variations))
		}
		k := key(m.Options)
		if _, dup := given[k]; dup {
			return nil, bad("the model %v is given twice", m.Options)
		}
		given[k] = m
	}
	out := make([]Model, 0, len(combos))
	skus := map[string]bool{}
	for _, c := range combos {
		m := Model{Options: c, Price: n.Price, Stock: -1}
		if n.Stock != nil {
			m.Stock = *n.Stock
		}
		if g, ok := given[key(c)]; ok {
			delete(given, key(c))
			if g.Price != nil {
				m.Price = *g.Price
			}
			if g.Stock != nil {
				m.Stock = *g.Stock
			}
			m.SKU = strings.TrimSpace(g.SKU)
		}
		label := strings.Join(c, ", ")
		if m.Price == 0 {
			return nil, bad("%s has no price: give the product's price or the model's", label)
		}
		if err := checkPrice(m.Price, label+"'s price"); err != nil {
			return nil, err
		}
		if m.Stock < 0 {
			return nil, bad("%s has no stock: give the product's stock or the model's", label)
		}
		if err := checkStock(m.Stock, label+"'s stock"); err != nil {
			return nil, err
		}
		if cnt := utf8.RuneCountInString(m.SKU); cnt > MaxSKU {
			return nil, bad("%s's SKU has %d characters, more than %d", label, cnt, MaxSKU)
		}
		if m.SKU != "" {
			if skus[strings.ToLower(m.SKU)] {
				return nil, bad("the SKU %q is given twice", m.SKU)
			}
			skus[strings.ToLower(m.SKU)] = true
		}
		out = append(out, m)
	}
	for _, g := range given {
		return nil, bad("the model %v is not a combination of the variations' options", g.Options)
	}
	return out, nil
}

// Summary is the product as it will be made, line by line, for --dry-run.
func (n *New) Summary(models []Model) []string {
	out := []string{
		"name: " + n.Name,
		"category: " + strings.Join(n.Category, " > "),
	}
	for i, p := range n.Photos {
		out = append(out, fmt.Sprintf("photo %d: %s", i+1, p))
	}
	out = append(out, "cover: photo 1 (Seller Centre's choice)")
	if n.Video != "" {
		line := "video: " + n.Video
		if d, err := MP4Duration(n.Video); err == nil {
			line += fmt.Sprintf(" (%s)", d.Round(100*time.Millisecond))
		}
		out = append(out, line)
	}
	for _, k := range sortedKeys(n.Attributes) {
		out = append(out, fmt.Sprintf("attribute %s: %s", k, n.Attributes[k]))
	}
	for _, v := range n.Variations {
		out = append(out, fmt.Sprintf("variation %s: %s", v.Name, strings.Join(v.Options, ", ")))
	}
	for _, m := range models {
		label := strings.Join(m.Options, ", ")
		if label == "" {
			label = "price"
		}
		line := fmt.Sprintf("%s: %s, stock %d", label, Rupiah(m.Price), m.Stock)
		if m.SKU != "" {
			line += ", SKU " + m.SKU
		}
		out = append(out, line)
	}
	if n.SKU != "" && len(n.Variations) > 0 {
		out = append(out, "parent SKU: "+n.SKU)
	}
	ship := fmt.Sprintf("weight: %d g", n.Weight)
	if d := n.Dimensions; d != (Dimensions{}) {
		ship += fmt.Sprintf(", %dx%dx%d cm", d.Length, d.Width, d.Height)
	}
	out = append(out, ship)
	couriers := "Reguler (Cashless), or the first offered"
	if len(n.Couriers) > 0 {
		couriers = strings.Join(n.Couriers, ", ")
	}
	out = append(out, "couriers: "+couriers)
	cond := "Baru"
	if n.Used {
		cond = "Pernah Dipakai"
	}
	danger := "Tidak"
	if n.Dangerous {
		danger = "Mengandung baterai/magnet/cairan/bahan mudah terbakar"
	}
	out = append(out, "condition: "+cond, "dangerous goods: "+danger)
	save := "Simpan & Arsipkan (archived)"
	if n.Live {
		save = "Simpan & Tampilkan (live)"
	}
	out = append(out, "save: "+save, fmt.Sprintf("description (%d of %d characters):", utf8.RuneCountInString(n.Description), MaxDescription), n.Description)
	return out
}

// Rupiah writes a price as Seller Centre does: Rp9.999.000.
func Rupiah(p int64) string {
	s := strconv.FormatInt(p, 10)
	var b strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(r)
	}
	return "Rp" + b.String()
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return keys
}

// statusOf names a product's state from Seller Centre's list: status 1 is
// live; an unlisted one (tag unlist) is archived.
func statusOf(status int, unlist bool) string {
	switch {
	case unlist:
		return "archived"
	case status == 1:
		return "live"
	}
	return fmt.Sprintf("status %d", status)
}

// sameText compares text as given with a field as it reads: line ends,
// no-break spaces and runs of spaces may differ.
func sameText(shown, given string) bool {
	norm := func(s string) string {
		s = strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\u00a0", " ")
		var lines []string
		for _, l := range strings.Split(s, "\n") {
			lines = append(lines, strings.Join(strings.Fields(l), " "))
		}
		return strings.TrimSpace(strings.Join(lines, "\n"))
	}
	return norm(shown) == norm(given)
}

// sameName compares two names on the page (a category, an option, an
// attribute): case and runs of spaces aside.
func sameName(a, b string) bool {
	return strings.EqualFold(strings.Join(strings.Fields(a), " "), strings.Join(strings.Fields(b), " "))
}
