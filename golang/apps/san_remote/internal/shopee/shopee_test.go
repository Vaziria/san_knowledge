package shopee

import (
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"
)

// photo writes a w x h png and returns its path.
func photo(t *testing.T, dir, name string, w, h int) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := 0; x < w; x++ {
		img.Set(x, x%h, color.RGBA{0x33, 0x66, 0xcc, 0xff})
	}
	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	return path
}

const productMD = `---
name: TES san_remote - akan dihapus
category: Komputer & Aksesoris > Software
photos: [img/1.png, img/2.png]
attributes:
  Merek: Tidak ada merek
price: 9999000
stock: 1
sku: TES-INDUK
weight: 100
dimensions: 20 x 10 x 5
condition: new
couriers: [Reguler (Cashless)]
publish: archived
variations:
  - name: Warna
    options: [Merah, Biru]
  - name: Ukuran
    options: [S, M]
models:
  - options: [Merah, S]
    sku: TES-MERAH-S
  - options: [biru, m]
    price: 9998000
    stock: 2
    sku: TES-BIRU-M
---

Produk **uji** otomatis dari san_remote.
Jangan dibeli: produk ini akan segera dihapus.
`

func TestParseFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "img"), 0o755); err != nil {
		t.Fatal(err)
	}
	photo(t, dir, "img/1.png", 64, 64)
	photo(t, dir, "img/2.png", 64, 64)
	path := filepath.Join(dir, "p.md")
	if err := os.WriteFile(path, []byte(strings.ReplaceAll(productMD, "\n", "\r\n")), 0o644); err != nil {
		t.Fatal(err)
	}
	n, err := ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if n.Name != "TES san_remote - akan dihapus" || !slices.Equal(n.Category, []string{"Komputer & Aksesoris", "Software"}) {
		t.Errorf("name %q, category %q", n.Name, n.Category)
	}
	if n.Photos[0] != filepath.Join(dir, "img", "1.png") {
		t.Errorf("photo %q: not relative to the file", n.Photos[0])
	}
	if n.Description != "Produk uji otomatis dari san_remote.\nJangan dibeli: produk ini akan segera dihapus." {
		t.Errorf("description %q", n.Description)
	}
	if n.Dimensions != (Dimensions{20, 10, 5}) || n.Weight != 100 || n.Used || n.Live || n.SKU != "TES-INDUK" {
		t.Errorf("shipping and the rest: %+v", n)
	}
	if n.Attributes["Merek"] != "Tidak ada merek" || !slices.Equal(n.Couriers, []string{"Reguler (Cashless)"}) {
		t.Errorf("attributes %v, couriers %v", n.Attributes, n.Couriers)
	}
	models, err := n.Check()
	if err != nil {
		t.Fatal(err)
	}
	// Every combination, the first variation outermost, with the product's
	// price and stock unless the model gives its own.
	want := []Model{
		{Options: []string{"Merah", "S"}, Price: 9999000, Stock: 1, SKU: "TES-MERAH-S"},
		{Options: []string{"Merah", "M"}, Price: 9999000, Stock: 1},
		{Options: []string{"Biru", "S"}, Price: 9999000, Stock: 1},
		{Options: []string{"Biru", "M"}, Price: 9998000, Stock: 2, SKU: "TES-BIRU-M"},
	}
	if len(models) != len(want) {
		t.Fatalf("%d models: %+v", len(models), models)
	}
	for i := range want {
		g, w := models[i], want[i]
		if !slices.Equal(g.Options, w.Options) || g.Price != w.Price || g.Stock != w.Stock || g.SKU != w.SKU {
			t.Errorf("model %d = %+v, want %+v", i, g, w)
		}
	}
	sum := strings.Join(n.Summary(models), "\n")
	for _, s := range []string{"variation Warna: Merah, Biru", "Biru, M: Rp9.998.000, stock 2, SKU TES-BIRU-M", "save: Simpan & Arsipkan (archived)", "parcel"} {
		if s == "parcel" {
			s = "weight: 100 g, 20x10x5 cm"
		}
		if !strings.Contains(sum, s) {
			t.Errorf("summary lacks %q:\n%s", s, sum)
		}
	}
}

func TestParse(t *testing.T) {
	n, err := Parse([]byte("---\nname: Kaos\ncategory: [Pakaian Pria, Atasan, Kaos]\nphotos: [a.jpg]\nprice: 75000\nstock: 0\nweight: 200\ncondition: bekas\npublish: live\ndangerous: true\n---\nKaos katun.\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(n.Category, []string{"Pakaian Pria", "Atasan", "Kaos"}) || !n.Used || !n.Live || !n.Dangerous {
		t.Errorf("%+v", n)
	}
	if n.Stock == nil || *n.Stock != 0 {
		t.Errorf("stock: 0 is a stock of 0, not none: %v", n.Stock)
	}
	for _, bad := range []string{
		"---\nname: x\n",                 // no closing ---
		"---\ncondition: rusak\n---\nx",  // condition
		"---\npublish: now\n---\nx",      // publish
		"---\ndimensions: 20x10\n---\nx", // dimensions
		"---\ncategory: {a: b}\n---\nx",  // category
		"---\nname: [\n---\nx",           // yaml
	} {
		if _, err := Parse([]byte(bad)); !errors.Is(err, ErrInvalid) {
			t.Errorf("%q: %v", bad, err)
		}
	}
}

func TestCheck(t *testing.T) {
	dir := t.TempDir()
	sq := photo(t, dir, "sq.png", 32, 32)
	wide := photo(t, dir, "wide.png", 40, 30)
	big := filepath.Join(dir, "big.jpg")
	if err := os.WriteFile(big, make([]byte, MaxPhotoBytes+1), 0o644); err != nil {
		t.Fatal(err)
	}
	gif := filepath.Join(dir, "a.gif")
	if err := os.WriteFile(gif, []byte("GIF89a"), 0o644); err != nil {
		t.Fatal(err)
	}
	one := int64(1)
	low := int64(50)
	base := func() New {
		return New{
			Name: "Kaos", Description: "Kaos katun.", Photos: []string{sq}, Category: []string{"Pakaian Pria", "Kaos"},
			Price: 75000, Stock: &one, Weight: 200,
		}
	}
	tiers := []Variation{{Name: "Warna", Options: []string{"Merah", "Biru"}}, {Name: "Ukuran", Options: []string{"S"}}}
	valid := base()
	if m, err := valid.Check(); err != nil || len(m) != 1 || m[0].Price != 75000 {
		t.Fatalf("a valid product: %v %+v", err, m)
	}
	for name, change := range map[string]func(*New){
		"no name":           func(n *New) { n.Name = " " },
		"long name":         func(n *New) { n.Name = strings.Repeat("a", MaxName+1) },
		"no description":    func(n *New) { n.Description = "" },
		"long description":  func(n *New) { n.Description = strings.Repeat("é", MaxDescription+1) },
		"no photos":         func(n *New) { n.Photos = nil },
		"ten photos":        func(n *New) { n.Photos = slices.Repeat([]string{sq}, 10) },
		"a missing photo":   func(n *New) { n.Photos = []string{filepath.Join(dir, "nope.png")} },
		"a wide photo":      func(n *New) { n.Photos = []string{wide} },
		"a photo over 2 MB": func(n *New) { n.Photos = []string{big} },
		"a gif":             func(n *New) { n.Photos = []string{gif} },
		"no category":       func(n *New) { n.Category = nil },
		"no weight":         func(n *New) { n.Weight = 0 },
		"half dimensions":   func(n *New) { n.Dimensions = Dimensions{Length: 10} },
		"no price":          func(n *New) { n.Price = 0 },
		"a low price":       func(n *New) { n.Price = 98 },
		"a high price":      func(n *New) { n.Price = MaxPrice + 1 },
		"no stock":          func(n *New) { n.Stock = nil },
		"models, no tiers":  func(n *New) { n.Models = []NewModel{{Options: []string{"Merah"}}} },
		"three variations":  func(n *New) { n.Variations = append(tiers, Variation{Name: "Bahan", Options: []string{"Katun"}}) },
		"a long tier name":  func(n *New) { n.Variations = []Variation{{Name: "ABCDEFGHIJKLMNO", Options: []string{"a"}}} },
		"a long option":     func(n *New) { n.Variations = []Variation{{Name: "Warna", Options: []string{strings.Repeat("b", 21)}}} },
		"a comma":           func(n *New) { n.Variations = []Variation{{Name: "Warna", Options: []string{"Merah, tua"}}} },
		"an option twice":   func(n *New) { n.Variations = []Variation{{Name: "Warna", Options: []string{"Merah", "merah"}}} },
		"a variation twice": func(n *New) {
			n.Variations = []Variation{{Name: "Warna", Options: []string{"a"}}, {Name: "warna", Options: []string{"b"}}}
		},
		"no options":          func(n *New) { n.Variations = []Variation{{Name: "Warna"}} },
		"a model off the map": func(n *New) { n.Variations = tiers; n.Models = []NewModel{{Options: []string{"Hijau", "S"}}} },
		"a short model":       func(n *New) { n.Variations = tiers; n.Models = []NewModel{{Options: []string{"Merah"}}} },
		"a model twice": func(n *New) {
			n.Variations = tiers
			n.Models = []NewModel{{Options: []string{"Merah", "S"}}, {Options: []string{"merah", "s"}}}
		},
		"a model's low price": func(n *New) {
			n.Variations = tiers
			n.Models = []NewModel{{Options: []string{"Merah", "S"}, Price: &low}}
		},
		"an SKU twice": func(n *New) {
			n.Variations = tiers
			n.Models = []NewModel{{Options: []string{"Merah", "S"}, SKU: "A"}, {Options: []string{"Biru", "S"}, SKU: "a"}}
		},
		"a model without a price": func(n *New) { n.Variations = tiers; n.Price = 0 },
		"a model without a stock": func(n *New) { n.Variations = tiers; n.Stock = nil },
	} {
		n := base()
		change(&n)
		if _, err := n.Check(); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: %v", name, err)
		}
	}
	// A model's own price and stock serve where the product has none.
	n := base()
	n.Variations, n.Price, n.Stock = []Variation{{Name: "Warna", Options: []string{"Merah"}}}, 0, nil
	p, s := int64(120000), int64(3)
	n.Models = []NewModel{{Options: []string{"Merah"}, Price: &p, Stock: &s}}
	if m, err := n.Check(); err != nil || m[0].Price != p || m[0].Stock != s {
		t.Errorf("own price: %v %+v", err, m)
	}
}

func TestParseID(t *testing.T) {
	for in, want := range map[string]int64{
		"57614536675":   57614536675,
		" 55471067707 ": 55471067707,
		"https://seller.shopee.co.id/portal/product/57614536675":                       57614536675,
		"https://shopee.co.id/Software-Aplikasi-Kasir-i.77987996.57614536675?sp_atk=x": 57614536675,
	} {
		if got, err := ParseID(in); err != nil || got != want {
			t.Errorf("ParseID(%q) = %d, %v", in, got, err)
		}
	}
	for _, bad := range []string{"", "abc", "-5", "0"} {
		if _, err := ParseID(bad); !errors.Is(err, ErrInvalid) {
			t.Errorf("ParseID(%q): %v", bad, err)
		}
	}
}

func TestRupiah(t *testing.T) {
	for in, want := range map[int64]string{99: "Rp99", 1000: "Rp1.000", 9999000: "Rp9.999.000", 150000000: "Rp150.000.000"} {
		if got := Rupiah(in); got != want {
			t.Errorf("Rupiah(%d) = %s", in, got)
		}
	}
}

// The list's product, as Seller Centre's get_product_list gives it
// (2026-11-03): status 8 with tag.unlist is archived; a model's name holds
// its options, "Merah,S".
func TestListedProduct(t *testing.T) {
	l := listedProduct{ID: 55471067707, Name: "TES", Status: 8, Unlist: true, PriceMin: 9999000, PriceMax: 9999000, Stock: 4, Created: 1793642000, Models: []listedModel{
		{ID: 273066084074, Name: "Merah,S", SKU: "TES-MERAH-S", Tier: []int{0, 0}, Price: 9999000, Stock: 1},
	}}
	p := l.product()
	if p.Status != "archived" || !slices.Equal(p.Models[0].Options, []string{"Merah", "S"}) || p.Created.Unix() != 1793642000 {
		t.Errorf("%+v", p)
	}
	single := listedProduct{Status: 1, Models: []listedModel{{ID: 1, Default: true, Price: 3500000}}}.product()
	if single.Status != "live" || single.Models[0].Options != nil {
		t.Errorf("a product without variations: %+v", single)
	}
	if got := statusOf(2, false); got != "status 2" {
		t.Errorf("statusOf(2) = %q", got)
	}
	if p.URL() != "https://seller.shopee.co.id/portal/product/55471067707" {
		t.Errorf("URL %s", p.URL())
	}
}

func TestChooseCouriers(t *testing.T) {
	offered := []courier{
		{Name: "Reguler (Cashless)", Price: "Rp8.000"},
		{Name: "Instant", Price: "Rp10.000"},
		{Name: "Hemat Kargo", Price: "Rp5.000", Disabled: true},
	}
	if got, err := chooseCouriers(offered, nil); err != nil || !slices.Equal(got, []string{"Reguler (Cashless)"}) {
		t.Errorf("default: %v %v", got, err)
	}
	if got, err := chooseCouriers(offered, []string{"instant", "reguler (cashless)"}); err != nil || !slices.Equal(got, []string{"Instant", "Reguler (Cashless)"}) {
		t.Errorf("given: %v %v", got, err)
	}
	for _, bad := range [][]string{{"JNE"}, {"Hemat Kargo"}} {
		if _, err := chooseCouriers(offered, bad); !errors.Is(err, ErrInvalid) {
			t.Errorf("%v: %v", bad, err)
		}
	}
	on := []courier{{Name: "Reguler (Cashless)"}, {Name: "Instant", On: true}}
	if got, _ := chooseCouriers(on, nil); !slices.Equal(got, []string{"Instant"}) {
		t.Errorf("one already on is kept: %v", got)
	}
	if _, err := chooseCouriers([]courier{{Name: "Instant", Disabled: true}}, nil); !errors.Is(err, ErrRefused) {
		t.Errorf("none can be turned on: %v", err)
	}
}

// Naikkan Produk's checks, from get_bumped_product_list as it read on
// 2026-11-03: {total_slots: 5, cooldown_seconds: 14400}.
func TestCheckSlots(t *testing.T) {
	now := time.Date(2026, 11, 3, 1, 30, 0, 0, time.Local)
	b := &bumpedInfo{Total: 5, Cooldown: 14400}
	add := func(id, left int64) {
		b.Products = append(b.Products, struct {
			ID   int64 `json:"id"`
			Left int64 `json:"left"`
		}{id, left})
	}
	if err := checkSlots(b, 1, now); err != nil {
		t.Errorf("free slots: %v", err)
	}
	add(1, 14378)
	if err := checkSlots(b, 1, now); !errors.Is(err, ErrCooldown) || !strings.Contains(err.Error(), "05:29") {
		t.Errorf("on its cooldown: %v", err)
	}
	for id := int64(2); id <= 5; id++ {
		add(id, 3600*id)
	}
	if err := checkSlots(b, 9, now); !errors.Is(err, ErrSlotsFull) || !strings.Contains(err.Error(), "5 of 5") || !strings.Contains(err.Error(), "03:30") {
		t.Errorf("full: %v", err)
	}
}

func TestCompareModels(t *testing.T) {
	want := []Model{{Options: []string{"Merah", "S"}, Price: 9999000, Stock: 1, SKU: "A"}, {Options: []string{"Biru", "S"}, Price: 9999000, Stock: 1}}
	got := []Model{{ID: 2, Options: []string{"Biru", "S"}, Price: 9999000, Stock: 1}, {ID: 1, Options: []string{"Merah", "S"}, Price: 9999000, Stock: 1, SKU: "A"}}
	if p := compareModels(got, want); len(p) > 0 {
		t.Errorf("the same models, in another order: %v", p)
	}
	got[0].Stock = 0
	if p := compareModels(got, want); len(p) != 1 || !strings.Contains(p[0], "Biru, S") {
		t.Errorf("a stock off: %v", p)
	}
	if p := compareModels(got[:1], want); len(p) != 3 {
		t.Errorf("a model missing: %v", p)
	}
}

func TestNewest(t *testing.T) {
	since := time.Unix(1000, 0)
	ps := []listedProduct{{ID: 1, Name: "TES", Created: 900}, {ID: 2, Name: "TES", Created: 1100}, {ID: 3, Name: "TES", Created: 1200}, {ID: 4, Name: "Lain", Created: 1300}}
	if got := newest(ps, "TES", since); got == nil || got.ID != 3 {
		t.Errorf("newest = %+v", got)
	}
	if got := newest(ps, "TES", time.Unix(1250, 0)); got != nil {
		t.Errorf("none since: %+v", got)
	}
}

// The check before Simpan passes a form that reads as given, and stops on
// anything else: the ads switch on, a row off, an error on the form.
func TestFormCheck(t *testing.T) {
	one := int64(1)
	n := New{
		Name: "TES", Description: "Baris satu.\nBaris dua.", Photos: []string{"a.png", "b.png"}, Category: []string{"Komputer & Aksesoris", "Software"},
		Attributes: map[string]string{"Merek": "Tidak ada merek"}, Price: 9999000, Stock: &one, Weight: 100,
		Variations: []Variation{{Name: "Warna", Options: []string{"Merah", "Biru"}}},
	}
	models := []Model{{Options: []string{"Merah"}, Price: 9999000, Stock: 1}, {Options: []string{"Biru"}, Price: 9999000, Stock: 1}}
	good := func() formView {
		return formView{
			Photos: 2, Cover: "https://down-id.img.susercontent.com/file/x", Name: "TES", Category: "Komputer & Aksesoris > Software",
			Attributes:  []attribute{{Name: "Merek", Required: true, Kind: "list", Value: "Tidak ada merek"}, {Name: "Tipe Software", Required: true, Value: "Edukasi & Produktivitas"}},
			Description: "Baris satu.\nBaris dua.",
			Tiers:       []tierState{{Name: "Warna", Options: []string{"Merah", "Biru"}}},
			Rows:        []rowState{{Options: []string{"Merah"}, Price: "9999000", Stock: "1"}, {Options: []string{"Biru"}, Price: "9.999.000", Stock: "1"}},
			Weight:      "100", Danger: "0", Couriers: []courier{{Name: "Reguler (Cashless)", On: true}}, Condition: "Baru",
		}
	}
	v := good()
	lines, problems := v.check(n, models)
	if len(problems) > 0 {
		t.Fatalf("a good form: %v", problems)
	}
	if !slices.Contains(lines, "couriers: Reguler (Cashless)") || !slices.Contains(lines, "attribute Tipe Software: Edukasi & Produktivitas") {
		t.Errorf("lines: %v", lines)
	}
	for name, change := range map[string]func(*formView){
		"ads on":             func(v *formView) { v.Ads = true },
		"a row's stock":      func(v *formView) { v.Rows[1].Stock = "2" },
		"rows swapped":       func(v *formView) { v.Rows[0], v.Rows[1] = v.Rows[1], v.Rows[0] },
		"an error":           func(v *formView) { v.Errors = []string{"Berat: wajib diisi"} },
		"no courier":         func(v *formView) { v.Couriers[0].On = false },
		"a photo short":      func(v *formView) { v.Photos = 1 },
		"no cover":           func(v *formView) { v.Cover = "" },
		"another category":   func(v *formView) { v.Category = "Elektronik > Remot Kontrol" },
		"a brand":            func(v *formView) { v.Attributes[0].Value = "Tesoro" },
		"an empty required":  func(v *formView) { v.Attributes[1].Value = "" },
		"the description":    func(v *formView) { v.Description = "Baris satu." },
		"dangerous":          func(v *formView) { v.Danger = "1" },
		"no danger answered": func(v *formView) { v.Danger = "" },
		"used":               func(v *formView) { v.Condition = "Pernah Dipakai" },
		"the weight":         func(v *formView) { v.Weight = "1000" },
		"an SKU Induk":       func(v *formView) { v.SKU = "LAIN" },
	} {
		v := good()
		change(&v)
		if _, problems := v.check(n, models); len(problems) == 0 {
			t.Errorf("%s: passed", name)
		}
	}
}

// Each helper of shopee.js is named once, and every one the Go steps call
// is there.
func TestScript(t *testing.T) {
	helper := regexp.MustCompile(`(?m)^    (?:async )?([A-Za-z]\w*)(?:\(|: )`)
	// The helpers are the members of the object the script returns.
	_, helpers, ok := strings.Cut(shopeeJS, "\n  return {\n")
	if !ok {
		t.Fatal("shopee.js returns no object of helpers")
	}
	seen := map[string]bool{}
	for _, m := range helper.FindAllStringSubmatch(helpers, -1) {
		if seen[m[1]] {
			t.Errorf("shopee.js defines %s twice", m[1])
		}
		seen[m[1]] = true
	}
	called := regexp.MustCompile(`(?:Call|CallFor|Do|do|Press)\(ctx, (?:[^,]+, )?(?:&?\w+(?:\.\w+)?, )?"(\w+)"`)
	for _, f := range []string{"service.go", "create.go"} {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		n := 0
		for _, m := range called.FindAllStringSubmatch(string(b), -1) {
			n++
			if !seen[m[1]] {
				t.Errorf("%s calls %s, which shopee.js does not have", f, m[1])
			}
		}
		if n < 10 {
			t.Errorf("%s: found only %d calls; the pattern no longer matches", f, n)
		}
	}
}
