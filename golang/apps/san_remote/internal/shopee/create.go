package shopee

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/wargasipil/san_remote/internal/page"
)

// Created is a product Create made: as the shop lists it afterwards, and
// what the form read before it was saved. A rehearsal's Product holds only
// what the form held.
type Created struct {
	Product Product
	Form    []string
}

// Create adds the product through Tambah Produk Baru, as a person fills the
// form: the photos (through Pembuat Foto AI's Terapkan ke Produk; the first
// becomes the cover), the name, then Selanjutnya; the category in its picker, the
// attributes, the description, the variations and their table (or one
// price and stock), the weight, Produk Berbahaya and the couriers. The
// whole form is read back, and only then is Simpan & Arsipkan (or, for a
// live product, Simpan & Tampilkan) pressed. The product is then looked up
// in Produk Saya and its models compared with what was asked. With
// Rehearse, it stops before Simpan and the form is left unsaved. It brings
// Chrome to the front with a tab of its own.
//
// Once Simpan is pressed and the product is made, an error comes with the
// product too.
func (s *Service) Create(ctx context.Context, n New) (*Created, error) {
	models, err := n.Check()
	if err != nil {
		return nil, err
	}
	var out *Created
	err = s.run(ctx, n.Browser, true, func(t *session) error {
		var err error
		out, err = t.create(ctx, n, models)
		return err
	})
	return out, err
}

func (t *session) create(ctx context.Context, n New, models []Model) (*Created, error) {
	if err := t.newForm(ctx); err != nil {
		return nil, err
	}
	if err := t.photos(ctx, n.Photos); err != nil {
		return nil, err
	}
	if err := t.cover(ctx); err != nil {
		return nil, err
	}
	t.Log("typing the name")
	if err := t.do(ctx, "the name box", "setName", n.Name); err != nil {
		return nil, err
	}
	if err := t.Until(ctx, "the name to read as given", 5*time.Second, func() (bool, error) {
		var v struct {
			Value string `json:"value"`
		}
		err := t.Call(ctx, &v, "name")
		return err == nil && v.Value == n.Name, err
	}); err != nil {
		return nil, err
	}
	t.Log("pressing Selanjutnya")
	if err := t.Press(ctx, "Selanjutnya", "pressNext"); err != nil {
		return nil, err
	}
	if _, err := t.waitState(ctx, "the rest of the form", time.Minute, func(st state) bool { return st.Full }); err != nil {
		return nil, t.formErrors(ctx, err)
	}
	t.guides(ctx)
	if err := t.category(ctx, n.Category); err != nil {
		return nil, err
	}
	t.guides(ctx)
	if err := t.attributes(ctx, n.Attributes); err != nil {
		return nil, err
	}
	if err := t.description(ctx, n.Description); err != nil {
		return nil, err
	}
	if len(n.Variations) > 0 {
		if err := t.variations(ctx, n.Variations, models); err != nil {
			return nil, err
		}
	} else {
		if err := t.single(ctx, models[0]); err != nil {
			return nil, err
		}
	}
	if n.SKU != "" {
		if err := t.typeField(ctx, "parentSku", n.SKU); err != nil {
			return nil, err
		}
	}
	if err := t.shipping(ctx, n); err != nil {
		return nil, err
	}
	if n.Used {
		if err := t.condition(ctx, "Pernah Dipakai"); err != nil {
			return nil, err
		}
	}
	form, err := t.ready(ctx, n, models)
	if err != nil {
		return nil, err
	}
	if n.Rehearse {
		t.Log("rehearsal: every step but Simpan; leaving the form unsaved")
		p := Product{Name: n.Name, Status: "rehearsed", SKU: n.SKU, Models: models}
		return &Created{Product: p, Form: form}, nil
	}
	p, err := t.save(ctx, n, models)
	if p == nil {
		return nil, err
	}
	return &Created{Product: *p, Form: form}, err
}

// newForm opens Tambah Produk Baru from Produk Saya, as a person does, and
// closes the onboarding over it.
func (t *session) newForm(ctx context.Context) error {
	t.Log("opening Tambah Produk Baru")
	if err := t.Press(ctx, "Tambah Produk Baru", "pressAddProduct"); err != nil {
		return err
	}
	if _, err := t.waitState(ctx, "the Tambah Produk Baru form", time.Minute, func(st state) bool { return st.Form }); err != nil {
		if !errors.Is(err, ErrPage) {
			return err
		}
		t.Log("the form did not open from the button; opening its page")
		if err := t.Navigate(ctx, newURL); err != nil {
			return err
		}
		if _, err := t.waitState(ctx, "the Tambah Produk Baru form", time.Minute, func(st state) bool { return st.Form }); err != nil {
			return err
		}
	}
	if err := t.Look(ctx, 2*time.Second); err != nil {
		return err
	}
	t.guides(ctx)
	return nil
}

// guides closes the onboarding Seller Centre shows over the form.
func (t *session) guides(ctx context.Context) {
	var closed []string
	if t.Call(ctx, &closed, "guides") == nil {
		for _, c := range closed {
			t.Log("closed Seller Centre's guide: " + c)
		}
	}
}

// photoState is what shopee.js photos() reports.
type photoState struct {
	Count  int      `json:"count"`
	Max    int      `json:"max"`
	Drawer string   `json:"drawer"`
	Ready  bool     `json:"ready"`
	Errors []string `json:"errors"`
}

// stageFile puts a file in the page in pieces, under id.
func (t *session) stageFile(ctx context.Context, id, path string) error {
	fh, err := os.Open(path)
	if err != nil {
		return err
	}
	defer fh.Close()
	buf := make([]byte, piece)
	for {
		n, err := io.ReadFull(fh, buf)
		if n > 0 {
			if err := t.CallFor(ctx, time.Minute, nil, "stage", id, base64.StdEncoding.EncodeToString(buf[:n])); err != nil {
				return err
			}
		}
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

type fileRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// give stages the files and hands them to a field's file input.
func (t *session) give(ctx context.Context, fieldID, prefix string, paths []string) error {
	var refs []fileRef
	for i, p := range paths {
		id := fmt.Sprintf("%s%d", prefix, i)
		if err := t.stageFile(ctx, id, p); err != nil {
			return err
		}
		refs = append(refs, fileRef{ID: id, Name: filepath.Base(p), Type: photoType(p)})
	}
	if err := t.Front(ctx); err != nil {
		return err
	}
	var msg string
	if err := t.Call(ctx, &msg, "give", fieldID, refs); err != nil {
		return err
	}
	if msg != "" {
		return fmt.Errorf("%w: %s", ErrPage, msg)
	}
	return nil
}

// photos gives Foto Produk the photos. Seller Centre opens them in
// Pembuat Foto AI, whose "Terapkan ke Produk (N)" puts them in the form.
func (t *session) photos(ctx context.Context, paths []string) error {
	t.Log(fmt.Sprintf("choosing %d photos", len(paths)))
	if err := t.give(ctx, "images", "photo", paths); err != nil {
		return err
	}
	var ps photoState
	if err := t.Until(ctx, "the photos to upload", 2*time.Minute, func() (bool, error) {
		if err := t.Call(ctx, &ps, "photos"); err != nil {
			return false, err
		}
		if len(ps.Errors) > 0 {
			return false, fmt.Errorf("%w: Foto Produk: %s", ErrRefused, strings.Join(ps.Errors, "; "))
		}
		return ps.Drawer != "" || ps.Count == len(paths), nil
	}); err != nil {
		return err
	}
	if ps.Drawer != "" {
		want := fmt.Sprintf("(%d)", len(paths))
		// The drawer takes the photos in for a few seconds: its button is
		// greyed and has no count until then.
		if err := t.Until(ctx, "Pembuat Foto AI to take the photos in", time.Minute, func() (bool, error) {
			err := t.Call(ctx, &ps, "photos")
			return err == nil && ps.Ready && strings.HasSuffix(ps.Drawer, ")"), err
		}); err != nil {
			return fmt.Errorf("%w (its button reads %q)", err, ps.Drawer)
		}
		t.guides(ctx)
		if !strings.HasSuffix(ps.Drawer, want) {
			return fmt.Errorf("%w: Pembuat Foto AI holds %q, not %d photos", ErrPage, ps.Drawer, len(paths))
		}
		t.Log("pressing " + ps.Drawer)
		if err := t.Press(ctx, ps.Drawer, "pressApplyPhotos"); err != nil {
			return err
		}
	}
	if err := t.Until(ctx, fmt.Sprintf("Foto Produk to hold %d photos", len(paths)), time.Minute, func() (bool, error) {
		err := t.Call(ctx, &ps, "photos")
		return err == nil && ps.Count == len(paths) && ps.Drawer == "", err
	}); err != nil {
		return fmt.Errorf("%w (it reads %d, %v)", err, ps.Count, ps.Errors)
	}
	t.guides(ctx)
	return nil
}

// cover waits for Seller Centre to fill Cover Produk with the first photo.
// It locks the cover there (the section's image manager is "image-lock",
// with only Preview on the photo), so no other photo is offered for it.
func (t *session) cover(ctx context.Context) error {
	var c string
	return t.Until(ctx, "Seller Centre to make the first photo the cover", 30*time.Second, func() (bool, error) {
		err := t.Call(ctx, &c, "cover")
		return err == nil && c != "", err
	})
}

// formErrors adds what the form says is wrong to an error.
func (t *session) formErrors(ctx context.Context, err error) error {
	var errs []string
	if t.Call(ctx, &errs, "errors") == nil && len(errs) > 0 {
		return fmt.Errorf("%w: the form says: %s (%v)", ErrRefused, strings.Join(errs, "; "), err)
	}
	return err
}

// ---- category --------------------------------------------------------------------------

type pickerState struct {
	Columns  [][]string `json:"columns"`
	Selected []string   `json:"selected"`
	Picked   string     `json:"picked"`
}

func (t *session) picker(ctx context.Context) (*pickerState, error) {
	var p *pickerState
	err := t.Call(ctx, &p, "picker")
	return p, err
}

// category picks the category in Ubah Kategori, level by level, checks it
// has no subcategories, and confirms it. The rest of the form (Spesifikasi
// on) appears once a category is set.
func (t *session) category(ctx context.Context, path []string) error {
	want := strings.Join(path, " > ")
	t.Log("choosing the category " + want)
	if err := t.Press(ctx, "the category box", "pressCategory"); err != nil {
		return err
	}
	var p *pickerState
	if err := t.Until(ctx, "the category picker", 15*time.Second, func() (bool, error) {
		var err error
		p, err = t.picker(ctx)
		return err == nil && p != nil && len(p.Columns) > 0 && len(p.Columns[0]) > 0, err
	}); err != nil {
		return err
	}
	for level, name := range path {
		if level >= len(p.Columns) || !slices.ContainsFunc(p.Columns[level], func(c string) bool { return sameName(c, name) }) {
			under := "at the top"
			if level > 0 {
				under = "under " + strings.Join(path[:level], " > ")
			}
			var offered []string
			if level < len(p.Columns) {
				offered = p.Columns[level]
			}
			return fmt.Errorf("%w: no category %q %s; it has: %s", ErrInvalid, name, under, strings.Join(offered, ", "))
		}
		if err := t.Press(ctx, "the category "+name, "pickCategory", level, name); err != nil {
			return err
		}
		if err := t.Until(ctx, "the category "+name+" to be picked", 10*time.Second, func() (bool, error) {
			var err error
			p, err = t.picker(ctx)
			return err == nil && p != nil && len(p.Selected) > level && sameName(p.Selected[level], name), err
		}); err != nil {
			return err
		}
	}
	if len(p.Columns) > len(path) && len(p.Columns[len(path)]) > 0 {
		return fmt.Errorf("%w: the category %s has subcategories; give one of: %s", ErrInvalid, want, strings.Join(p.Columns[len(path)], ", "))
	}
	if !sameName(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(p.Picked), "Dipilih :")), want) {
		return fmt.Errorf("%w: the picker reads %q, not %s", ErrPage, p.Picked, want)
	}
	if err := t.Press(ctx, "Confirm", "confirmCategory"); err != nil {
		return err
	}
	var shown string
	if _, err := t.waitState(ctx, "the category to be set and the form to open", time.Minute, func(st state) bool {
		_ = t.Call(ctx, &shown, "category")
		return st.Sections && sameName(shown, want)
	}); err != nil {
		return fmt.Errorf("%w (the category reads %q)", err, shown)
	}
	return t.Look(ctx, time.Second)
}

// ---- attributes ------------------------------------------------------------------------

type attribute struct {
	Name     string `json:"name"`
	Required bool   `json:"required"`
	Kind     string `json:"kind"`
	Value    string `json:"value"`
}

func (t *session) readAttributes(ctx context.Context) ([]attribute, error) {
	var as []attribute
	err := t.Call(ctx, &as, "attributes")
	return as, err
}

func findAttribute(as []attribute, name string) (attribute, bool) {
	for _, a := range as {
		if sameName(a.Name, name) {
			return a, true
		}
	}
	return attribute{}, false
}

type optionList struct {
	Filter bool     `json:"filter"`
	Items  []string `json:"items"`
}

// attributes fills Spesifikasi: each given attribute typed, or picked from
// its list (through the list's search box when it is not among the first
// options). Then no required attribute may be empty: Seller Centre fills
// some by itself, which are left as they are.
func (t *session) attributes(ctx context.Context, given map[string]string) error {
	as, err := t.readAttributes(ctx)
	if err != nil {
		return err
	}
	for _, name := range sortedKeys(given) {
		value := strings.TrimSpace(given[name])
		a, ok := findAttribute(as, name)
		if !ok {
			var names []string
			for _, a := range as {
				names = append(names, a.Name)
			}
			return fmt.Errorf("%w: the category has no attribute %q; it has: %s", ErrInvalid, name, strings.Join(names, ", "))
		}
		t.Log(fmt.Sprintf("attribute %s: %s", a.Name, value))
		switch a.Kind {
		case "text":
			if err := t.do(ctx, "the attribute "+a.Name, "setAttributeText", a.Name, value); err != nil {
				return err
			}
		case "list":
			if err := t.pick(ctx, a.Name, value); err != nil {
				return err
			}
		default:
			return fmt.Errorf("%w: the attribute %q takes several values (%s), which the tool does not fill", ErrInvalid, a.Name, a.Kind)
		}
		if err := t.Until(ctx, "the attribute "+a.Name+" to read "+value, 5*time.Second, func() (bool, error) {
			as, err = t.readAttributes(ctx)
			got, _ := findAttribute(as, a.Name)
			return err == nil && sameName(got.Value, value), err
		}); err != nil {
			return err
		}
	}
	var empty []string
	for _, a := range as {
		if a.Required && strings.TrimSpace(a.Value) == "" {
			empty = append(empty, a.Name)
		}
	}
	if len(empty) > 0 {
		hint := ""
		if slices.ContainsFunc(empty, func(s string) bool { return sameName(s, "Merek") }) {
			hint = " (Merek: \"Tidak ada merek\" for a product without a brand)"
		}
		return fmt.Errorf("%w: the category needs these attributes: %s%s", ErrInvalid, strings.Join(empty, ", "), hint)
	}
	return nil
}

// pick opens an attribute's list and picks value from it.
func (t *session) pick(ctx context.Context, name, value string) error {
	if err := t.Press(ctx, "the attribute "+name, "openAttribute", name); err != nil {
		return err
	}
	var ol *optionList
	if err := t.Until(ctx, "the list of "+name, 10*time.Second, func() (bool, error) {
		err := t.Call(ctx, &ol, "options")
		return err == nil && ol != nil && len(ol.Items) > 0, err
	}); err != nil {
		return err
	}
	has := func() bool { return slices.ContainsFunc(ol.Items, func(o string) bool { return sameName(o, value) }) }
	if !has() && ol.Filter && utf8.RuneCountInString(value) >= 2 {
		if err := t.do(ctx, "the search box of "+name, "filterOptions", value); err != nil {
			return err
		}
		_ = t.Until(ctx, "the list of "+name+" to offer "+value, 8*time.Second, func() (bool, error) {
			err := t.Call(ctx, &ol, "options")
			return err == nil && ol != nil && has(), err
		})
	}
	if ol == nil || !has() {
		var offered []string
		if ol != nil {
			offered = ol.Items
		}
		if len(offered) > 15 {
			offered = append(offered[:15], "…")
		}
		_ = t.Call(ctx, nil, "closeOptions")
		return fmt.Errorf("%w: %s offers no %q; it offers: %s", ErrInvalid, name, value, strings.Join(offered, ", "))
	}
	return t.Press(ctx, value, "pickOption", value)
}

// ---- description -----------------------------------------------------------------------

func (t *session) description(ctx context.Context, text string) error {
	t.Log(fmt.Sprintf("writing the description (%d characters)", utf8.RuneCountInString(text)))
	if err := t.do(ctx, "the description box", "setDescription", text); err != nil {
		return err
	}
	var d *struct {
		Text    string `json:"text"`
		Counter string `json:"counter"`
	}
	err := t.Until(ctx, "the description to read as given", 10*time.Second, func() (bool, error) {
		err := t.Call(ctx, &d, "description")
		return err == nil && d != nil && sameText(d.Text, text), err
	})
	if err != nil && d != nil {
		return fmt.Errorf("%w (it reads %q)", err, d.Text)
	}
	return err
}

// ---- variations and prices -------------------------------------------------------------

type tierState struct {
	Name    string   `json:"name"`
	Options []string `json:"options"`
	Errors  []string `json:"errors"`
}

type rowState struct {
	Options []string `json:"options"`
	Price   string   `json:"price"`
	Stock   string   `json:"stock"`
	SKU     string   `json:"sku"`
	Errors  []string `json:"errors"`
}

func (t *session) tiers(ctx context.Context) ([]tierState, error) {
	var ts []tierState
	err := t.Call(ctx, &ts, "tiers")
	return ts, err
}

func (t *session) rows(ctx context.Context) ([]rowState, error) {
	var rs []rowState
	err := t.Call(ctx, &rs, "models")
	return rs, err
}

// variations turns Variasi on, types each variation's name and options,
// and fills the table's rows, which come in the order of models. Seller
// Centre's "Isi Otomatis Variasi" may have turned Variasi on already, with
// variations it guessed from the photos and the name: those are retyped,
// and the ones (and options) beyond the product's are taken away.
func (t *session) variations(ctx context.Context, vs []Variation, models []Model) error {
	ts, err := t.tiers(ctx)
	if err != nil {
		return err
	}
	if len(ts) == 0 {
		t.Log("turning variations on (Aktifkan Variasi)")
		if err := t.Press(ctx, "Aktifkan Variasi", "pressEnableVariation"); err != nil {
			return err
		}
	} else {
		var guessed []string
		for _, x := range ts {
			guessed = append(guessed, x.Name+": "+strings.Join(x.Options, ", "))
		}
		t.Log("Seller Centre filled variations by itself (" + strings.Join(guessed, "; ") + "); retyping them")
	}
	for i := len(ts) - 1; i >= len(vs); i-- {
		t.Log(fmt.Sprintf("taking away variation %d (%s)", i+1, ts[i].Name))
		if err := t.Press(ctx, fmt.Sprintf("variation %d's delete button", i+1), "pressDeleteTier", i); err != nil {
			return err
		}
		if err := t.Until(ctx, fmt.Sprintf("variation %d to go", i+1), 10*time.Second, func() (bool, error) {
			ts, err := t.tiers(ctx)
			return err == nil && len(ts) == i, err
		}); err != nil {
			return err
		}
	}
	for i, v := range vs {
		ts, err := t.tiers(ctx)
		if err != nil {
			return err
		}
		if i >= len(ts) {
			if err := t.Press(ctx, "Tambah Variasi 2", "pressAddTier"); err != nil {
				return err
			}
		}
		if err := t.Until(ctx, fmt.Sprintf("variation %d's box", i+1), 10*time.Second, func() (bool, error) {
			ts, err = t.tiers(ctx)
			return err == nil && len(ts) > i, err
		}); err != nil {
			return err
		}
		for j := len(ts[i].Options) - 1; j >= len(v.Options); j-- {
			t.Log(fmt.Sprintf("taking away the option %s", ts[i].Options[j]))
			if err := t.Press(ctx, "the option "+ts[i].Options[j]+"'s delete button", "pressRemoveOption", i, j); err != nil {
				return err
			}
			if err := t.Until(ctx, "the option to go", 10*time.Second, func() (bool, error) {
				now, err := t.tiers(ctx)
				return err == nil && len(now) > i && len(now[i].Options) == j, err
			}); err != nil {
				return err
			}
		}
		t.Log(fmt.Sprintf("variation %s: %s", v.Name, strings.Join(v.Options, ", ")))
		if err := t.do(ctx, "variation "+v.Name+"'s name", "setTierName", i, strings.TrimSpace(v.Name)); err != nil {
			return err
		}
		for j, o := range v.Options {
			if err := t.do(ctx, "the option "+o, "setOption", i, j, strings.TrimSpace(o)); err != nil {
				return err
			}
			if err := page.Sleep(ctx, 300*time.Millisecond); err != nil {
				return err
			}
		}
		if err := t.Until(ctx, "variation "+v.Name+" to read as given", 10*time.Second, func() (bool, error) {
			var err error
			ts, err = t.tiers(ctx)
			if err != nil || len(ts) <= i {
				return false, err
			}
			if len(ts[i].Errors) > 0 {
				return false, fmt.Errorf("%w: variation %s: %s", ErrRefused, v.Name, strings.Join(ts[i].Errors, "; "))
			}
			return ts[i].Name == strings.TrimSpace(v.Name) && sameList(ts[i].Options, v.Options), nil
		}); err != nil {
			return err
		}
	}
	t.guides(ctx)
	var rs []rowState
	if err := t.Until(ctx, fmt.Sprintf("the table to have %d rows", len(models)), 10*time.Second, func() (bool, error) {
		var err error
		rs, err = t.rows(ctx)
		return err == nil && len(rs) == len(models), err
	}); err != nil {
		return err
	}
	for i, m := range models {
		if !sameList(rs[i].Options, m.Options) {
			return fmt.Errorf("%w: the table's row %d reads %v, not %v", ErrPage, i+1, rs[i].Options, m.Options)
		}
	}
	t.Log(fmt.Sprintf("filling the table's %d rows", len(models)))
	for i, m := range models {
		if err := t.do(ctx, fmt.Sprintf("row %d's price and stock", i+1), "setModel", i, m.Price, m.Stock, m.SKU); err != nil {
			return err
		}
	}
	return t.Until(ctx, "the table to read as given", 10*time.Second, func() (bool, error) {
		rs, err := t.rows(ctx)
		if err != nil {
			return false, err
		}
		for i, m := range models {
			if len(rs[i].Errors) > 0 {
				return false, fmt.Errorf("%w: %s: %s", ErrRefused, strings.Join(m.Options, ", "), strings.Join(rs[i].Errors, "; "))
			}
			if digits(rs[i].Price) != m.Price || digits(rs[i].Stock) != m.Stock || strings.TrimSpace(rs[i].SKU) != m.SKU {
				return false, nil
			}
		}
		return true, nil
	})
}

// single types the price and stock of a product without variations. When
// Seller Centre's "Isi Otomatis Variasi" turned Variasi on by itself, its
// variations are taken away first, which brings Harga and Stok back.
func (t *session) single(ctx context.Context, m Model) error {
	ts, err := t.tiers(ctx)
	if err != nil {
		return err
	}
	for i := len(ts) - 1; i >= 0; i-- {
		t.Log(fmt.Sprintf("taking away variation %s, which Seller Centre filled by itself", ts[i].Name))
		if err := t.Press(ctx, fmt.Sprintf("variation %d's delete button", i+1), "pressDeleteTier", i); err != nil {
			return err
		}
		if err := t.Until(ctx, fmt.Sprintf("variation %d to go", i+1), 10*time.Second, func() (bool, error) {
			ts, err := t.tiers(ctx)
			return err == nil && len(ts) == i, err
		}); err != nil {
			return err
		}
	}
	if len(ts) > 0 {
		if err := t.Until(ctx, "Harga and Stok to come back", 10*time.Second, func() (bool, error) {
			var v *string
			err := t.Call(ctx, &v, "fieldValue", "price")
			return err == nil && v != nil, err
		}); err != nil {
			return err
		}
	}
	t.Log(fmt.Sprintf("price %s, stock %d", Rupiah(m.Price), m.Stock))
	if err := t.typeField(ctx, "price", strconv.FormatInt(m.Price, 10)); err != nil {
		return err
	}
	return t.typeField(ctx, "stock", strconv.FormatInt(m.Stock, 10))
}

// typeField types v into a plain field and waits for it to read v (a
// number in any format).
func (t *session) typeField(ctx context.Context, id, v string) error {
	if err := t.do(ctx, id+" field", "setField", id, v); err != nil {
		return err
	}
	var got *string
	err := t.Until(ctx, id+" field to read "+v, 5*time.Second, func() (bool, error) {
		err := t.Call(ctx, &got, "fieldValue", id)
		return err == nil && got != nil && sameValue(*got, v), err
	})
	if err != nil && got != nil {
		return fmt.Errorf("%w (it reads %q)", err, *got)
	}
	return err
}

var nonDigit = regexp.MustCompile(`\D`)

// digits reads a number as a field shows it: "9.999.000", "Rp9.999.000".
func digits(s string) int64 {
	n, err := strconv.ParseInt(nonDigit.ReplaceAllString(s, ""), 10, 64)
	if err != nil {
		return -1
	}
	return n
}

func sameValue(shown, given string) bool {
	if shown == given {
		return true
	}
	if _, err := strconv.ParseInt(given, 10, 64); err == nil {
		return digits(shown) == digits(given)
	}
	return strings.TrimSpace(shown) == strings.TrimSpace(given)
}

func sameList(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !sameName(a[i], b[i]) {
			return false
		}
	}
	return true
}

// ---- shipping and the rest -------------------------------------------------------------

type courier struct {
	Name     string `json:"name"`
	On       bool   `json:"on"`
	Disabled bool   `json:"disabled"`
	Price    string `json:"price"`
}

type courierState struct {
	Items []courier `json:"items"`
	Note  string    `json:"note"`
}

func (t *session) couriers(ctx context.Context) (*courierState, error) {
	var c *courierState
	err := t.Call(ctx, &c, "couriers")
	if err == nil && c == nil {
		err = fmt.Errorf("%w: no Ongkos Kirim", ErrPage)
	}
	return c, err
}

// shipping types the weight and the parcel's size, answers Produk
// Berbahaya, and turns the couriers on. Seller Centre offers the couriers
// once it knows the weight.
func (t *session) shipping(ctx context.Context, n New) error {
	t.Log(fmt.Sprintf("weight %d g", n.Weight))
	if err := t.typeField(ctx, "weight", strconv.Itoa(n.Weight)); err != nil {
		return err
	}
	if d := n.Dimensions; d != (Dimensions{}) {
		t.Log(fmt.Sprintf("parcel %dx%dx%d cm", d.Length, d.Width, d.Height))
		for id, v := range map[string]int{"dimension.length": d.Length, "dimension.width": d.Width, "dimension.height": d.Height} {
			if err := t.typeField(ctx, id, strconv.Itoa(v)); err != nil {
				return err
			}
		}
	}
	danger := "0"
	if n.Dangerous {
		danger = "1"
	}
	if err := t.Press(ctx, "Produk Berbahaya", "setDanger", danger); err != nil {
		return err
	}
	if err := t.Until(ctx, "Produk Berbahaya to be answered", 5*time.Second, func() (bool, error) {
		var v string
		err := t.Call(ctx, &v, "danger")
		return err == nil && v == danger, err
	}); err != nil {
		return err
	}
	var cs *courierState
	if err := t.Until(ctx, "Seller Centre to offer the couriers", 30*time.Second, func() (bool, error) {
		var err error
		cs, err = t.couriers(ctx)
		return err == nil && slices.ContainsFunc(cs.Items, func(c courier) bool { return c.Price != "" && !c.Disabled }), err
	}); err != nil {
		return err
	}
	want, err := chooseCouriers(cs.Items, n.Couriers)
	if err != nil {
		return err
	}
	for _, c := range cs.Items {
		on := slices.Contains(want, c.Name)
		if c.On == on {
			continue
		}
		verb := "turning off"
		if on {
			verb = "turning on"
		}
		t.Log(verb + " the courier " + c.Name)
		if err := t.Press(ctx, "the courier "+c.Name, "toggleCourier", c.Name); err != nil {
			return err
		}
		if err := t.Until(ctx, "the courier "+c.Name+" to switch", 10*time.Second, func() (bool, error) {
			cs, err := t.couriers(ctx)
			if err != nil {
				return false, err
			}
			for _, x := range cs.Items {
				if x.Name == c.Name {
					return x.On == on, nil
				}
			}
			return false, nil
		}); err != nil {
			return err
		}
	}
	return nil
}

// chooseCouriers says which of the offered couriers are to be on: the
// given ones, each of which must be offered; without any, the ones already
// on, or else Reguler (Cashless), or else the first that can be turned on.
func chooseCouriers(offered []courier, given []string) ([]string, error) {
	var names []string
	for _, c := range offered {
		names = append(names, c.Name)
	}
	if len(given) > 0 {
		var want []string
		for _, g := range given {
			i := slices.IndexFunc(offered, func(c courier) bool { return sameName(c.Name, g) })
			if i < 0 {
				return nil, fmt.Errorf("%w: Seller Centre offers no courier %q for this product; it offers: %s", ErrInvalid, g, strings.Join(names, ", "))
			}
			if offered[i].Disabled && !offered[i].On {
				return nil, fmt.Errorf("%w: the courier %q cannot be turned on for this product", ErrInvalid, offered[i].Name)
			}
			want = append(want, offered[i].Name)
		}
		return want, nil
	}
	var on []string
	for _, c := range offered {
		if c.On {
			on = append(on, c.Name)
		}
	}
	if len(on) > 0 {
		return on, nil
	}
	for _, c := range offered {
		if sameName(c.Name, "Reguler (Cashless)") && !c.Disabled {
			return []string{c.Name}, nil
		}
	}
	for _, c := range offered {
		if !c.Disabled {
			return []string{c.Name}, nil
		}
	}
	return nil, fmt.Errorf("%w: Seller Centre offers no courier that can be turned on (%s)", ErrRefused, strings.Join(names, ", "))
}

// condition picks Kondisi.
func (t *session) condition(ctx context.Context, v string) error {
	t.Log("condition " + v)
	if err := t.Press(ctx, "Kondisi", "openCondition"); err != nil {
		return err
	}
	if err := t.Until(ctx, "the list of Kondisi", 10*time.Second, func() (bool, error) {
		var ol *optionList
		err := t.Call(ctx, &ol, "options")
		return err == nil && ol != nil && len(ol.Items) > 0, err
	}); err != nil {
		return err
	}
	if err := t.Press(ctx, v, "pickOption", v); err != nil {
		return err
	}
	return t.Until(ctx, "Kondisi to read "+v, 5*time.Second, func() (bool, error) {
		var got string
		err := t.Call(ctx, &got, "condition")
		return err == nil && sameName(got, v), err
	})
}

// ---- the check before Simpan, and Simpan -----------------------------------------------

// formView is the whole form as it reads before it is saved.
type formView struct {
	Photos      int
	Cover       string
	Name        string
	Category    string
	Attributes  []attribute
	Description string
	Tiers       []tierState
	Rows        []rowState
	Price       string
	Stock       string
	Weight      string
	SKU         string
	Parcel      [3]string
	Danger      string
	Couriers    []courier
	Ads         bool
	Condition   string
	Errors      []string
}

func (t *session) view(ctx context.Context, variations bool) (*formView, error) {
	v := &formView{}
	var ps photoState
	var name struct {
		Value string `json:"value"`
	}
	var desc *struct {
		Text string `json:"text"`
	}
	var price, stock, weight, sku *string
	var parcel [3]*string
	type call struct {
		out  any
		fn   string
		args []any
	}
	calls := []call{
		{&ps, "photos", nil}, {&v.Cover, "cover", nil}, {&name, "name", nil}, {&v.Category, "category", nil},
		{&v.Attributes, "attributes", nil}, {&desc, "description", nil}, {&weight, "fieldValue", []any{"weight"}},
		{&v.Danger, "danger", nil}, {&v.Ads, "ads", nil}, {&v.Condition, "condition", nil}, {&v.Errors, "errors", nil},
		{&sku, "fieldValue", []any{"parentSku"}}, {&parcel[0], "fieldValue", []any{"dimension.length"}},
		{&parcel[1], "fieldValue", []any{"dimension.width"}}, {&parcel[2], "fieldValue", []any{"dimension.height"}},
	}
	if variations {
		calls = append(calls, call{&v.Tiers, "tiers", nil}, call{&v.Rows, "models", nil})
	} else {
		calls = append(calls, call{&price, "fieldValue", []any{"price"}}, call{&stock, "fieldValue", []any{"stock"}})
	}
	for _, c := range calls {
		if err := t.Call(ctx, c.out, c.fn, c.args...); err != nil {
			return nil, err
		}
	}
	cs, err := t.couriers(ctx)
	if err != nil {
		return nil, err
	}
	v.Couriers = cs.Items
	v.Photos, v.Name = ps.Count, name.Value
	if desc != nil {
		v.Description = desc.Text
	}
	for _, p := range []struct {
		dst *string
		src *string
	}{{&v.Price, price}, {&v.Stock, stock}, {&v.Weight, weight}, {&v.SKU, sku}, {&v.Parcel[0], parcel[0]}, {&v.Parcel[1], parcel[1]}, {&v.Parcel[2], parcel[2]}} {
		if p.src != nil {
			*p.dst = *p.src
		}
	}
	return v, nil
}

// ready reads the whole form back right before Simpan, so that nothing is
// saved other than meant: every field the tool filled, the ads switch off,
// and no error on the form. It returns the form, line by line.
func (t *session) ready(ctx context.Context, n New, models []Model) ([]string, error) {
	v, err := t.view(ctx, len(n.Variations) > 0)
	if err != nil {
		return nil, err
	}
	lines, problems := v.check(n, models)
	if len(problems) > 0 {
		return lines, fmt.Errorf("%w: the form reads %s; not saving", ErrPage, strings.Join(problems, "; "))
	}
	t.Log(fmt.Sprintf("checked the form: %d photos, %s, %d models, %s g, couriers %s", v.Photos, v.Category, len(models), v.Weight, strings.Join(onCouriers(v.Couriers), ", ")))
	return lines, nil
}

func onCouriers(cs []courier) []string {
	var on []string
	for _, c := range cs {
		if c.On {
			on = append(on, c.Name)
		}
	}
	return on
}

// check compares the form with the product, and returns the form line by
// line and what differs.
func (v *formView) check(n New, models []Model) (lines, problems []string) {
	bad := func(format string, a ...any) { problems = append(problems, fmt.Sprintf(format, a...)) }
	lines = append(lines, fmt.Sprintf("photos: %d", v.Photos))
	if v.Photos != len(n.Photos) {
		bad("%d photos, not %d", v.Photos, len(n.Photos))
	}
	if v.Cover == "" {
		bad("no cover")
	} else {
		lines = append(lines, "cover: "+v.Cover)
	}
	lines = append(lines, "name: "+v.Name)
	if v.Name != n.Name {
		bad("the name %q", v.Name)
	}
	lines = append(lines, "category: "+v.Category)
	if want := strings.Join(n.Category, " > "); !sameName(v.Category, want) {
		bad("the category %q, not %q", v.Category, want)
	}
	for _, a := range v.Attributes {
		if a.Value != "" {
			lines = append(lines, fmt.Sprintf("attribute %s: %s", a.Name, a.Value))
		} else if a.Required {
			bad("the required attribute %s empty", a.Name)
		}
	}
	for name, want := range n.Attributes {
		if a, ok := findAttribute(v.Attributes, name); !ok || !sameName(a.Value, want) {
			bad("the attribute %s %q, not %q", name, a.Value, want)
		}
	}
	lines = append(lines, fmt.Sprintf("description: %d characters", utf8.RuneCountInString(v.Description)))
	if !sameText(v.Description, n.Description) {
		bad("a description other than given (%q)", clip(v.Description, 60))
	}
	if len(n.Variations) > 0 {
		if len(v.Tiers) != len(n.Variations) {
			bad("%d variations, not %d", len(v.Tiers), len(n.Variations))
		}
		for i, tier := range v.Tiers {
			lines = append(lines, fmt.Sprintf("variation %s: %s", tier.Name, strings.Join(tier.Options, ", ")))
			if i < len(n.Variations) && (tier.Name != strings.TrimSpace(n.Variations[i].Name) || !sameList(tier.Options, n.Variations[i].Options)) {
				bad("variation %d as %s: %v", i+1, tier.Name, tier.Options)
			}
		}
		if len(v.Rows) != len(models) {
			bad("%d rows in the table, not %d", len(v.Rows), len(models))
		}
		for i, r := range v.Rows {
			lines = append(lines, fmt.Sprintf("%s: %s, stock %s, SKU %s", strings.Join(r.Options, ", "), r.Price, r.Stock, r.SKU))
			if i >= len(models) {
				continue
			}
			m := models[i]
			if !sameList(r.Options, m.Options) || digits(r.Price) != m.Price || digits(r.Stock) != m.Stock || strings.TrimSpace(r.SKU) != m.SKU {
				bad("the row %v as %s, stock %s, SKU %q", m.Options, r.Price, r.Stock, r.SKU)
			}
		}
	} else {
		lines = append(lines, fmt.Sprintf("price: %s, stock %s", v.Price, v.Stock))
		if digits(v.Price) != models[0].Price || digits(v.Stock) != models[0].Stock {
			bad("price %s and stock %s", v.Price, v.Stock)
		}
	}
	if n.SKU != "" || v.SKU != "" {
		lines = append(lines, "SKU Induk: "+v.SKU)
		if strings.TrimSpace(v.SKU) != n.SKU {
			bad("SKU Induk %q", v.SKU)
		}
	}
	if d := n.Dimensions; d != (Dimensions{}) {
		lines = append(lines, fmt.Sprintf("parcel: %s x %s x %s cm", v.Parcel[0], v.Parcel[1], v.Parcel[2]))
		if digits(v.Parcel[0]) != int64(d.Length) || digits(v.Parcel[1]) != int64(d.Width) || digits(v.Parcel[2]) != int64(d.Height) {
			bad("a parcel of %v", v.Parcel)
		}
	}
	lines = append(lines, "weight: "+v.Weight+" g")
	if digits(v.Weight) != int64(n.Weight) {
		bad("a weight of %s", v.Weight)
	}
	want := "0"
	if n.Dangerous {
		want = "1"
	}
	lines = append(lines, "dangerous goods: "+map[string]string{"0": "Tidak", "1": "Mengandung"}[v.Danger])
	if v.Danger != want {
		bad("Produk Berbahaya %q", v.Danger)
	}
	on := onCouriers(v.Couriers)
	lines = append(lines, "couriers: "+strings.Join(on, ", "))
	if len(on) == 0 {
		bad("no courier on")
	}
	for _, c := range n.Couriers {
		if !slices.ContainsFunc(on, func(o string) bool { return sameName(o, c) }) {
			bad("the courier %s off", c)
		}
	}
	if v.Ads {
		bad("\"Iklankan Produk dan Dapatkan Voucher\" on")
	}
	lines = append(lines, "condition: "+v.Condition)
	if wantCond := map[bool]string{false: "Baru", true: "Pernah Dipakai"}[n.Used]; !sameName(v.Condition, wantCond) {
		bad("the condition %q", v.Condition)
	}
	if len(v.Errors) > 0 {
		bad("errors: %s", strings.Join(v.Errors, "; "))
	}
	return lines, problems
}

func clip(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n-1]) + "…"
	}
	return s
}

// save presses Simpan & Arsipkan (or Simpan & Tampilkan), waits for Seller
// Centre to go back to Produk Saya, finds the new product there, and
// compares its models with what was asked.
func (t *session) save(ctx context.Context, n New, models []Model) (*Product, error) {
	label := "Simpan & Arsipkan"
	if n.Live {
		label = "Simpan & Tampilkan"
	}
	start := t.now()
	t.Log("pressing " + label)
	if err := t.Press(ctx, label, "pressSave", label); err != nil {
		return nil, err
	}
	var errs []string
	st, err := t.waitState(ctx, "Seller Centre to save the product", 2*time.Minute, func(st state) bool {
		if st.List {
			return true
		}
		_ = t.Call(ctx, &errs, "errors")
		return len(errs) > 0 || st.Modal != ""
	})
	if err != nil {
		return nil, err
	}
	if !st.List {
		if len(errs) > 0 {
			return nil, fmt.Errorf("%w: after %s the form says: %s", ErrRefused, label, strings.Join(errs, "; "))
		}
		return nil, fmt.Errorf("%w: after %s Seller Centre asks: %q; it is left to you in the tab", ErrRefused, label, st.Modal)
	}
	t.Log("saved; finding it in Produk Saya")
	var hit *listedProduct
	if err := t.Until(ctx, "the new product in Produk Saya", time.Minute, func() (bool, error) {
		pg, err := t.page(ctx, "all", 1, pageSize, "")
		if err != nil {
			return false, err
		}
		hit = newest(pg.Products, n.Name, start.Add(-2*time.Minute))
		if hit == nil {
			return false, page.Sleep(ctx, 2*time.Second)
		}
		return true, nil
	}); err != nil {
		return nil, fmt.Errorf("%w (it was saved; look for %q in Produk Saya)", err, n.Name)
	}
	p := hit.product()
	t.Log(fmt.Sprintf("made product %d, %s", p.ID, p.Status))
	if problems := compareModels(p.Models, models); len(problems) > 0 {
		return &p, fmt.Errorf("%w: product %d was saved, but Produk Saya reads %s", ErrPage, p.ID, strings.Join(problems, "; "))
	}
	return &p, nil
}

// newest is the product named name made since, the newest one.
func newest(ps []listedProduct, name string, since time.Time) *listedProduct {
	var hit *listedProduct
	for i, l := range ps {
		if l.Name != name || time.Unix(l.Created, 0).Before(since) {
			continue
		}
		if hit == nil || l.Created > hit.Created || (l.Created == hit.Created && l.ID > hit.ID) {
			hit = &ps[i]
		}
	}
	return hit
}

// compareModels says how the listed models differ from the asked ones.
func compareModels(got, want []Model) []string {
	var problems []string
	if len(got) != len(want) {
		problems = append(problems, fmt.Sprintf("%d models, not %d", len(got), len(want)))
	}
	for _, w := range want {
		i := slices.IndexFunc(got, func(g Model) bool { return sameList(g.Options, w.Options) })
		label := strings.Join(w.Options, ", ")
		if label == "" {
			label = "the product"
		}
		if i < 0 {
			problems = append(problems, "no model "+label)
			continue
		}
		g := got[i]
		if g.Price != w.Price || g.Stock != w.Stock || (len(w.Options) > 0 && g.SKU != w.SKU) {
			problems = append(problems, fmt.Sprintf("%s as %s, stock %d, SKU %q", label, Rupiah(g.Price), g.Stock, g.SKU))
		}
	}
	return problems
}
