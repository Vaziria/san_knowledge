package shopee

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	v1 "github.com/wargasipil/san_remote/gen/san/remote/v1"
	"github.com/wargasipil/san_remote/internal/page"
	"github.com/wargasipil/san_remote/internal/pagetest"
)

// A photo over several pieces reaches the page whole.
func TestStageWholeFile(t *testing.T) {
	ctx := context.Background()
	b := &pagetest.Pieces{}
	p, err := page.Open(ctx, b, site, "", "about:blank", false, nil)
	if err != nil {
		t.Fatal(err)
	}
	s := &session{Session: p}
	path, want := pagetest.File(t, "1.jpg", pagetest.Size)
	if err := s.give(ctx, "images", "photo", []string{path}); !errors.Is(err, pagetest.ErrNotStage) {
		t.Fatalf("give: %v", err)
	}
	// The page makes a Blob of each piece.
	b.Each(t, "photo0", want)
}

// fake is a Seller Centre tab: each helper of shopee.js answers from a Go
// function, and every call is kept, to see what was pressed.
type fake struct {
	page.Browser
	mu      sync.Mutex
	answers map[string]func(args []json.RawMessage) any
	calls   []string
	// onNavigate runs when the tab loads a page.
	onNavigate func()
}

var helperCall = regexp.MustCompile(`globalThis\.__sanShopee\.(\w+)\(([^\n]*)\);$`)

func (f *fake) OpenTab(context.Context, *v1.OpenTabRequest) (*v1.OpenTabResponse, error) {
	return &v1.OpenTabResponse{Tab: &v1.Tab{Id: 1}}, nil
}

func (f *fake) CloseTab(context.Context, *v1.CloseTabRequest) (*v1.CloseTabResponse, error) {
	return &v1.CloseTabResponse{}, nil
}

func (f *fake) Navigate(context.Context, *v1.NavigateRequest) (*v1.NavigateResponse, error) {
	f.mu.Lock()
	on := f.onNavigate
	f.mu.Unlock()
	if on != nil {
		on()
	}
	return &v1.NavigateResponse{}, nil
}

func (f *fake) FocusTab(context.Context, *v1.FocusTabRequest) (*v1.FocusTabResponse, error) {
	return &v1.FocusTabResponse{}, nil
}

func (f *fake) Screenshot(context.Context, *v1.ScreenshotRequest) (*v1.ScreenshotResponse, error) {
	return nil, errors.New("no screen in a test")
}

func (f *fake) Exec(_ context.Context, r *v1.ExecRequest) (*v1.ExecResponse, error) {
	if strings.Contains(r.GetCode(), "document.visibilityState") {
		return &v1.ExecResponse{ResultJson: "true"}, nil
	}
	m := helperCall.FindStringSubmatch(r.GetCode())
	if m == nil {
		return nil, fmt.Errorf("fake: not a helper call: %.100s", r.GetCode())
	}
	var args []json.RawMessage
	if err := json.Unmarshal([]byte("["+m[2]+"]"), &args); err != nil {
		return nil, err
	}
	f.mu.Lock()
	f.calls = append(f.calls, m[1]+"("+m[2]+")")
	answer := f.answers[m[1]]
	f.mu.Unlock()
	var out any
	if answer != nil {
		out = answer(args)
	}
	b, err := json.Marshal(out)
	if err != nil {
		return nil, err
	}
	return &v1.ExecResponse{ResultJson: string(b)}, nil
}

func (f *fake) called(prefix string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.ContainsFunc(f.calls, func(c string) bool { return strings.HasPrefix(c, prefix) })
}

type slot struct {
	ID   int64 `json:"id"`
	Left int64 `json:"left"`
}

// shop is a fake Seller Centre with these products and bumped slots.
func shop(products []listedProduct, slots []slot, menu []menuItem) *fake {
	f := &fake{}
	f.answers = map[string]func([]json.RawMessage) any{
		"state": func([]json.RawMessage) any {
			return state{URL: listURL, Lang: "id", Portal: true, List: true, ListReady: true}
		},
		"shop": func([]json.RawMessage) any {
			return shopInfo{Shop: "Get Resolved ID", ShopID: 77987996, User: "getresolved.id", Language: "id"}
		},
		"products": func(args []json.RawMessage) any {
			var keyword string
			_ = json.Unmarshal(args[3], &keyword)
			out := productPage{Total: len(products)}
			for _, p := range products {
				if keyword == "" || strings.Contains(fmt.Sprint(p.ID), keyword) {
					out.Products = append(out.Products, p)
				}
			}
			return out
		},
		"bumped": func([]json.RawMessage) any {
			b := map[string]any{"total": 5, "cooldown": 14400, "products": slots}
			return b
		},
		"search": func([]json.RawMessage) any { return true },
		"row":    func([]json.RawMessage) any { return map[string]any{"text": "row"} },
		"closeMenu": func([]json.RawMessage) any {
			return true
		},
		"pressMore": func([]json.RawMessage) any { return true },
		"menu":      func([]json.RawMessage) any { return menu },
		"pressMenu": func([]json.RawMessage) any { return true },
		"toasts":    func([]json.RawMessage) any { return []string{} },
	}
	return f
}

var (
	live     = listedProduct{ID: 55471067707, Name: "TES san_remote - akan dihapus", Status: 1, PriceMin: 9999000, PriceMax: 9999000, Stock: 4}
	archived = listedProduct{ID: 57614536675, Name: "Software Aplikasi Kasir", Status: 8, Unlist: true}
	liveMenu = []menuItem{{Label: "Salin"}, {Label: "Tampilan Produk"}, {Label: "Arsipkan"}, {Label: "Naikkan Produk"}, {Label: "Atur Komisi Affiliate"}}
)

// What Seller Centre would refuse is refused before anything is pressed.
func TestBoostRefused(t *testing.T) {
	ctx := context.Background()
	full := []slot{{1, 100}, {2, 200}, {3, 300}, {4, 400}, {5, 500}}
	for name, c := range map[string]struct {
		products []listedProduct
		slots    []slot
		id       int64
		want     error
	}{
		"archived":     {[]listedProduct{live, archived}, nil, archived.ID, ErrNotLive},
		"no such":      {[]listedProduct{live}, nil, 42, ErrNotFound},
		"on cooldown":  {[]listedProduct{live}, []slot{{live.ID, 14000}}, live.ID, ErrCooldown},
		"no slot free": {[]listedProduct{live}, full, live.ID, ErrSlotsFull},
	} {
		f := shop(c.products, c.slots, liveMenu)
		s := &Service{B: f}
		_, _, err := s.Boost(ctx, "", c.id)
		if !errors.Is(err, c.want) {
			t.Errorf("%s: %v, want %v", name, err, c.want)
		}
		if f.called("pressMore") || f.called("pressMenu") {
			t.Errorf("%s: something was pressed: %v", name, f.calls)
		}
	}
}

// A bump presses Naikkan Produk in the product's own menu, and reads its
// slot back.
func TestBoost(t *testing.T) {
	ctx := context.Background()
	var slots []slot
	f := shop([]listedProduct{live}, nil, liveMenu)
	f.answers["bumped"] = func([]json.RawMessage) any {
		return map[string]any{"total": 5, "cooldown": 14400, "products": slots}
	}
	f.answers["pressMenu"] = func(args []json.RawMessage) any {
		var label string
		_ = json.Unmarshal(args[0], &label)
		if label == "Naikkan Produk" {
			slots = append(slots, slot{live.ID, 14400}, slot{7, 60})
		}
		return true
	}
	now := time.Date(2026, 11, 3, 1, 27, 0, 0, time.Local)
	s := &Service{B: f, Now: func() time.Time { return now }}
	b, all, err := s.Boost(ctx, "", live.ID)
	if err != nil {
		t.Fatal(err)
	}
	if b.ID != live.ID || b.Name != live.Name || !b.EndsAt.Equal(now.Add(4*time.Hour)) || len(all.Boosts) != 2 || all.Total != 5 {
		t.Errorf("boost %+v, slots %+v", b, all)
	}
	if !f.called(`pressMore("55471067707")`) || !f.called(`pressMenu("Naikkan Produk")`) {
		t.Errorf("calls: %v", f.calls)
	}
	// A product gone from the shop holds its slot, with no name.
	if all.Boosts[1].Name != "" {
		t.Errorf("the other slot: %+v", all.Boosts[1])
	}
}

// A menu that shows the countdown instead of Naikkan Produk is the
// cooldown, even when the slots read free.
func TestBoostCountdown(t *testing.T) {
	f := shop([]listedProduct{live}, nil, []menuItem{{Label: "Salin"}, {Label: "Arsipkan"}, {Label: "03:59:26", Tip: "Setiap slot dapat digunakan kembali setiap 4 jam sekali"}})
	_, _, err := (&Service{B: f}).Boost(context.Background(), "", live.ID)
	if !errors.Is(err, ErrCooldown) || !strings.Contains(err.Error(), "03:59:26") {
		t.Errorf("err = %v", err)
	}
	if f.called(`pressMenu(`) {
		t.Errorf("pressed: %v", f.calls)
	}
}

// Delete answers Hapus only in the question that names the product.
func TestDeleteAsksAboutIt(t *testing.T) {
	f := shop([]listedProduct{archived}, nil, []menuItem{{Label: "Salin"}, {Label: "Tampilkan"}, {Label: "Hapus"}})
	f.answers["modal"] = func([]json.RawMessage) any {
		return modal{Title: "Hapus produk", Text: "Hapus produk Yakin ingin menghapus produk ini? TES lain Batal Hapus", Buttons: []string{"Batal", "Hapus"}}
	}
	_, err := (&Service{B: f}).Delete(context.Background(), "", archived.ID)
	if !errors.Is(err, ErrPage) || !strings.Contains(err.Error(), "not deleting") {
		t.Errorf("err = %v", err)
	}
	if f.called("pressModal") {
		t.Errorf("answered: %v", f.calls)
	}
	if _, err := (&Service{B: shop(nil, nil, nil)}).Delete(context.Background(), "", 42); !errors.Is(err, ErrNotFound) {
		t.Errorf("no such product: %v", err)
	}
}

// Seller Centre in another language is refused before anything is read.
func TestLanguage(t *testing.T) {
	f := shop([]listedProduct{live}, nil, liveMenu)
	f.answers["shop"] = func([]json.RawMessage) any { return shopInfo{Shop: "x", Language: "en"} }
	if _, err := (&Service{B: f}).Status(context.Background(), ""); !errors.Is(err, ErrLanguage) {
		t.Errorf("err = %v", err)
	}
	f.answers["state"] = func([]json.RawMessage) any {
		return state{URL: "https://seller.shopee.co.id/account/signin", Signin: true}
	}
	if st, err := (&Service{B: f}).Status(context.Background(), ""); err != nil || st.SignedIn {
		t.Errorf("signed out: %+v %v", st, err)
	}
}

// A live product is archived first, and the list loaded again before its
// Lainnya is read: Seller Centre keeps the row's live menu until then.
// Hapus is answered in the question that names the product, and the
// product must then be gone from the list itself.
func TestDeleteLive(t *testing.T) {
	ctx := context.Background()
	var mu sync.Mutex
	p := live
	products := []listedProduct{p}
	stale, gone := false, false
	f := shop(nil, nil, nil)
	f.onNavigate = func() { mu.Lock(); stale = false; mu.Unlock() }
	f.answers["products"] = func(args []json.RawMessage) any {
		mu.Lock()
		defer mu.Unlock()
		return productPage{Total: len(products), Products: products}
	}
	f.answers["menu"] = func([]json.RawMessage) any {
		mu.Lock()
		defer mu.Unlock()
		if products[0].Status == 1 || stale {
			return liveMenu
		}
		return []menuItem{{Label: "Salin"}, {Label: "Tampilkan"}, {Label: "Hapus"}}
	}
	f.answers["modal"] = func([]json.RawMessage) any {
		return modal{Title: "Hapus produk", Text: "Hapus produk Yakin ingin menghapus produk ini? " + p.Name + " Batal Hapus", Buttons: []string{"Batal", "Hapus"}}
	}
	f.answers["pressModal"] = func(args []json.RawMessage) any {
		var button string
		_ = json.Unmarshal(args[1], &button)
		mu.Lock()
		defer mu.Unlock()
		switch button {
		case "Konfirmasi":
			products[0].Status, products[0].Unlist, stale = 8, true, true
		case "Hapus":
			products, gone = nil, true
		}
		return true
	}
	got, err := (&Service{B: f}).Delete(ctx, "", p.ID)
	if err != nil {
		t.Fatalf("%v (calls %v)", err, f.calls)
	}
	if got.ID != p.ID || got.Status != "live" || !gone {
		t.Errorf("deleted %+v, gone %v", got, gone)
	}
	if !f.called(`pressMenu("Arsipkan")`) || !f.called(`pressModal("mengarsipkan", "Konfirmasi")`) || !f.called(`pressMenu("Hapus")`) {
		t.Errorf("calls: %v", f.calls)
	}
}
