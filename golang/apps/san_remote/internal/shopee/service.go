package shopee

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"

	"github.com/wargasipil/san_remote/internal/page"
)

//go:embed shopee.js
var shopeeJS string

var site = page.Site{Name: "shopee", Global: "__sanShopee", JS: shopeeJS, ErrPage: ErrPage}

const (
	origin  = "https://seller.shopee.co.id"
	listURL = origin + "/portal/product/list/all"
	newURL  = origin + "/portal/product/new"
	// piece is how many bytes of a file go into the page in one call.
	piece = 4 << 20
	// pageSize is how many products one read of the list asks for.
	pageSize = 48
	// maxPages bounds a scan of the list: 2000 products, Seller Centre's
	// "Batas Maks. Produk".
	maxPages = 42
)

// Service drives Shopee Seller Centre in the browser. Its calls run one at a
// time, since each one works in a tab of the user's browser. The CLI, the
// MCP server and the API each run one, in their own process, through
// page.Remote.
type Service struct {
	B page.Browser
	// Progress gets each step as it starts, such as "choosing the category".
	Progress func(step string)
	// Now is the clock, for tests; nil is time.Now.
	Now func() time.Time

	once sync.Once
	turn chan struct{}
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *Service) begin(ctx context.Context) (done func(), err error) {
	s.once.Do(func() { s.turn = make(chan struct{}, 1) })
	select {
	case s.turn <- struct{}{}:
		return func() { <-s.turn }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// run opens a tab at Produk Saya, shown or in the background, waits for it
// signed in, and runs f in it. A page error comes with a screenshot.
func (s *Service) run(ctx context.Context, browser string, shown bool, f func(t *session) error) error {
	done, err := s.begin(ctx)
	if err != nil {
		return err
	}
	defer done()
	p, err := page.Open(ctx, s.B, site, browser, listURL, shown, s.Progress)
	if err != nil {
		return err
	}
	t := &session{Session: p, now: s.now}
	defer t.Close()
	if err := t.home(ctx); err != nil {
		return t.Fail(err)
	}
	return t.Fail(f(t))
}

// session is the one tab a call works in.
type session struct {
	*page.Session
	now  func() time.Time
	shop shopInfo
}

// state is what shopee.js state() reports.
type state struct {
	URL       string `json:"url"`
	Lang      string `json:"lang"`
	Signin    bool   `json:"signin"`
	Portal    bool   `json:"portal"`
	List      bool   `json:"list"`
	ListReady bool   `json:"listReady"`
	Form      bool   `json:"form"`
	Full      bool   `json:"full"`
	Sections  bool   `json:"sections"`
	Modal     string `json:"modal"`
}

func (t *session) state(ctx context.Context) (state, error) {
	var st state
	err := t.Call(ctx, &st, "state")
	return st, err
}

// waitState waits until the page's state passes ok. A call that lands while
// a page loads can be refused by the browser; that is only a reason to ask
// again.
func (t *session) waitState(ctx context.Context, what string, timeout time.Duration, ok func(state) bool) (state, error) {
	var st state
	err := t.Until(ctx, what, timeout, func() (bool, error) {
		var err error
		st, err = t.state(ctx)
		if err != nil && ctx.Err() == nil && connect.CodeOf(err) == connect.CodeAborted {
			return false, nil
		}
		return err == nil && ok(st), err
	})
	return st, err
}

// shopInfo is what shopee.js shop() reports.
type shopInfo struct {
	Code     int    `json:"code"`
	Message  string `json:"message"`
	Shop     string `json:"shop"`
	ShopID   int64  `json:"shopId"`
	User     string `json:"user"`
	Language string `json:"language"`
}

// home waits for Produk Saya, signed in, and checks the account reads
// Seller Centre in Indonesian.
func (t *session) home(ctx context.Context) error {
	t.Log("opening Produk Saya in Seller Centre")
	st, err := t.waitState(ctx, "Seller Centre's Produk Saya", time.Minute, func(st state) bool {
		return st.ListReady || st.Signin
	})
	if err != nil {
		if st.URL != "" && !st.Portal {
			return fmt.Errorf("%w (the tab is at %s)", ErrNotSignedIn, st.URL)
		}
		return err
	}
	if st.Signin {
		return fmt.Errorf("%w (the tab is at %s)", ErrNotSignedIn, st.URL)
	}
	if err := t.CallFor(ctx, 30*time.Second, &t.shop, "shop"); err != nil {
		return err
	}
	if t.shop.Code != 0 {
		return fmt.Errorf("%w: the shop does not read (%d %s)", ErrNotSignedIn, t.shop.Code, t.shop.Message)
	}
	lang := t.shop.Language
	if lang == "" {
		lang = st.Lang
	}
	if lang != "" && !strings.HasPrefix(lang, "id") {
		return fmt.Errorf("%w: it is %q; switch it to Bahasa Indonesia in Seller Centre", ErrLanguage, lang)
	}
	return nil
}

// do is Do in a page kept in front, without a person's pause: for typing.
func (t *session) do(ctx context.Context, what, fn string, args ...any) error {
	if err := t.Front(ctx); err != nil {
		return err
	}
	return t.Do(ctx, what, fn, args...)
}

// ---- reads -----------------------------------------------------------------------------

// listedModel and listedProduct are what shopee.js products() reports.
type listedModel struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	SKU     string `json:"sku"`
	Tier    []int  `json:"tier"`
	Price   int64  `json:"price"`
	Stock   int64  `json:"stock"`
	Default bool   `json:"default"`
}

type listedProduct struct {
	ID       int64         `json:"id"`
	Name     string        `json:"name"`
	Status   int           `json:"status"`
	Unlist   bool          `json:"unlist"`
	PriceMin int64         `json:"priceMin"`
	PriceMax int64         `json:"priceMax"`
	Stock    int64         `json:"stock"`
	Sold     int64         `json:"sold"`
	SKU      string        `json:"sku"`
	Cover    string        `json:"cover"`
	Created  int64         `json:"created"`
	Modified int64         `json:"modified"`
	Models   []listedModel `json:"models"`
}

type productPage struct {
	Code     int             `json:"code"`
	Message  string          `json:"message"`
	Total    int             `json:"total"`
	Products []listedProduct `json:"products"`
}

func (l listedProduct) product() Product {
	p := Product{
		ID: l.ID, Name: l.Name, Status: statusOf(l.Status, l.Unlist), PriceMin: l.PriceMin, PriceMax: l.PriceMax,
		Stock: l.Stock, Sold: l.Sold, SKU: l.SKU, Cover: l.Cover,
	}
	if l.Created > 0 {
		p.Created = time.Unix(l.Created, 0)
	}
	if l.Modified > 0 {
		p.Updated = time.Unix(l.Modified, 0)
	}
	for _, m := range l.Models {
		p.Models = append(p.Models, Model{ID: m.ID, Options: modelOptions(m), Price: m.Price, Stock: m.Stock, SKU: m.SKU})
	}
	return p
}

// modelOptions reads a model's options from its name as the list gives it,
// "Merah,S"; a product without variations has one model with none.
func modelOptions(m listedModel) []string {
	if m.Default || strings.TrimSpace(m.Name) == "" {
		return nil
	}
	var out []string
	for _, o := range strings.Split(m.Name, ",") {
		out = append(out, strings.TrimSpace(o))
	}
	return out
}

func (t *session) page(ctx context.Context, listType string, number, size int, keyword string) (*productPage, error) {
	var pg productPage
	if err := t.CallFor(ctx, 30*time.Second, &pg, "products", listType, number, size, keyword); err != nil {
		return nil, err
	}
	if pg.Code != 0 {
		return nil, fmt.Errorf("%w: Produk Saya does not read (%d %s)", ErrPage, pg.Code, pg.Message)
	}
	return &pg, nil
}

// find looks a product up by its id: the list's search, then the whole
// list. It is nil when the shop has no such product.
func (t *session) find(ctx context.Context, id int64) (*Product, error) {
	pg, err := t.page(ctx, "all", 1, pageSize, strconv.FormatInt(id, 10))
	if err != nil {
		return nil, err
	}
	for _, l := range pg.Products {
		if l.ID == id {
			p := l.product()
			return &p, nil
		}
	}
	// The search can lag behind a product just made or changed.
	var hit *Product
	err = t.scan(ctx, "all", func(l listedProduct) bool {
		if l.ID == id {
			p := l.product()
			hit = &p
			return true
		}
		return false
	})
	return hit, err
}

// scan reads the list type's products page by page until stop says so or
// the list ends.
func (t *session) scan(ctx context.Context, listType string, stop func(listedProduct) bool) error {
	for n := 1; n <= maxPages; n++ {
		pg, err := t.page(ctx, listType, n, pageSize, "")
		if err != nil {
			return err
		}
		for _, l := range pg.Products {
			if stop(l) {
				return nil
			}
		}
		if len(pg.Products) < pageSize || n*pageSize >= pg.Total {
			return nil
		}
	}
	return nil
}

// ---- status ----------------------------------------------------------------------------

// Status says whether the browser is signed in to Seller Centre, and as
// which shop. It reads in a background tab.
func (s *Service) Status(ctx context.Context, browser string) (*Status, error) {
	var st *Status
	err := s.run(ctx, browser, false, func(t *session) error {
		st = &Status{SignedIn: true, Shop: t.shop.Shop, Username: t.shop.User, ShopID: t.shop.ShopID}
		return nil
	})
	if errors.Is(err, ErrNotSignedIn) {
		return &Status{}, nil
	}
	return st, err
}

// ---- products --------------------------------------------------------------------------

// Products lists the products of one of Produk Saya's tabs (Filters; ""
// is all), at most limit (0: 50), with the tab's total. search keeps those
// whose name, SKU, variation code or id holds it. It reads in a background
// tab.
func (s *Service) Products(ctx context.Context, browser, filter string, limit int, search string) ([]Product, int, error) {
	listType, err := listTypeOf(filter)
	if err != nil {
		return nil, 0, err
	}
	if limit <= 0 {
		limit = 50
	}
	var out []Product
	total := 0
	err = s.run(ctx, browser, false, func(t *session) error {
		t.Log("reading Produk Saya (" + listType + ")")
		if search = strings.TrimSpace(search); search != "" {
			pg, err := t.page(ctx, listType, 1, min(limit, pageSize), search)
			if err != nil {
				return err
			}
			total = pg.Total
			for _, l := range pg.Products {
				if len(out) < limit {
					out = append(out, l.product())
				}
			}
			return nil
		}
		for n := 1; n <= maxPages && len(out) < limit; n++ {
			pg, err := t.page(ctx, listType, n, pageSize, "")
			if err != nil {
				return err
			}
			total = pg.Total
			for _, l := range pg.Products {
				if len(out) < limit {
					out = append(out, l.product())
				}
			}
			if len(pg.Products) < pageSize || n*pageSize >= pg.Total {
				break
			}
		}
		return nil
	})
	return out, total, err
}

func listTypeOf(filter string) (string, error) {
	f := strings.ToLower(strings.TrimSpace(filter))
	if f == "" {
		f = "all"
	}
	lt, ok := Filters[f]
	if !ok {
		names := make([]string, 0, len(Filters))
		for k := range Filters {
			names = append(names, k)
		}
		slices.Sort(names)
		return "", fmt.Errorf("%w: no list %q; give one of %s", ErrInvalid, filter, strings.Join(names, ", "))
	}
	return lt, nil
}

// ---- the list's rows and their Lainnya menu --------------------------------------------

// menuItem is an item of a row's Lainnya menu.
type menuItem struct {
	Label    string `json:"label"`
	Disabled bool   `json:"disabled"`
	Tip      string `json:"tip"`
}

// countdown is a bumped product's Naikkan Produk item: "03:59:26".
var countdown = regexp.MustCompile(`^\d{1,2}:\d{2}:\d{2}$`)

// showRow searches the list for the product, as a person types its id in
// the search box, and waits for its row. Seller Centre's search can lag
// behind a product just made or changed, so the list is loaded again when
// the row does not come.
func (t *session) showRow(ctx context.Context, id int64) error {
	key := strconv.FormatInt(id, 10)
	for try := 1; ; try++ {
		t.Log("searching Produk Saya for " + key)
		if err := t.Press(ctx, "the list's search box", "search", key); err != nil {
			return err
		}
		err := t.Until(ctx, "the row of product "+key, 15*time.Second, func() (bool, error) {
			var r *struct {
				Text string `json:"text"`
			}
			err := t.Call(ctx, &r, "row", key)
			return err == nil && r != nil, err
		})
		if err == nil || !errors.Is(err, ErrPage) || try == 3 {
			return err
		}
		t.Log("the row did not show; loading the list again")
		if err := t.Navigate(ctx, listURL); err != nil {
			return err
		}
		if _, err := t.waitState(ctx, "Produk Saya", time.Minute, func(st state) bool { return st.ListReady }); err != nil {
			return err
		}
	}
}

// openMenu opens the row's Lainnya menu and reads its items.
func (t *session) openMenu(ctx context.Context, id int64) ([]menuItem, error) {
	key := strconv.FormatInt(id, 10)
	_ = t.Call(ctx, nil, "closeMenu")
	if err := t.Press(ctx, "the Lainnya of product "+key, "pressMore", key); err != nil {
		return nil, err
	}
	var items []menuItem
	err := t.Until(ctx, "the Lainnya menu", 10*time.Second, func() (bool, error) {
		err := t.Call(ctx, &items, "menu")
		return err == nil && len(items) > 0, err
	})
	return items, err
}

func itemOf(items []menuItem, label string) (menuItem, bool) {
	for _, it := range items {
		if sameName(it.Label, label) {
			return it, true
		}
	}
	return menuItem{}, false
}

func labels(items []menuItem) string {
	var l []string
	for _, it := range items {
		l = append(l, it.Label)
	}
	return strings.Join(l, ", ")
}

// modal is what shopee.js modal() reports.
type modal struct {
	Title   string   `json:"title"`
	Text    string   `json:"text"`
	Buttons []string `json:"buttons"`
}

// answer waits for the dialog that holds what and presses button in it.
func (t *session) answer(ctx context.Context, what, button string) error {
	t.Log("answering " + button + " to \"" + what + "\"")
	return t.Until(ctx, "Seller Centre's question \""+what+"\"", 15*time.Second, func() (bool, error) {
		var ok bool
		err := t.Call(ctx, &ok, "pressModal", what, button)
		return ok, err
	})
}

// waitStatus waits until the list reads the product in a state that ok
// passes, and returns it; nil when the product is gone.
func (t *session) waitStatus(ctx context.Context, id int64, what string, timeout time.Duration, ok func(*Product) bool) (*Product, error) {
	var p *Product
	err := t.Until(ctx, what, timeout, func() (bool, error) {
		var err error
		p, err = t.find(ctx, id)
		if err != nil || ok(p) {
			return err == nil, err
		}
		return false, page.Sleep(ctx, time.Second)
	})
	return p, err
}

// existing finds the product, or fails with ErrNotFound.
func (t *session) existing(ctx context.Context, id int64) (*Product, error) {
	p, err := t.find(ctx, id)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, fmt.Errorf("%w: the shop has no product %d", ErrNotFound, id)
	}
	return p, nil
}

// ---- publish and archive ---------------------------------------------------------------

// Publish shows an archived product to buyers: its row's Lainnya,
// Tampilkan. A product that is live already is left as it is. It brings
// Chrome to the front.
func (s *Service) Publish(ctx context.Context, browser string, id int64) (*Product, error) {
	if id <= 0 {
		return nil, fmt.Errorf("%w: no product id", ErrInvalid)
	}
	var out *Product
	err := s.run(ctx, browser, true, func(t *session) error {
		p, err := t.existing(ctx, id)
		if err != nil {
			return err
		}
		if p.Status == "live" {
			t.Log(fmt.Sprintf("%d is live already", id))
			out = p
			return nil
		}
		out, err = t.publish(ctx, p)
		return err
	})
	return out, err
}

func (t *session) publish(ctx context.Context, p *Product) (*Product, error) {
	t.Log(fmt.Sprintf("showing %d %q (Tampilkan)", p.ID, p.Name))
	if err := t.showRow(ctx, p.ID); err != nil {
		return nil, err
	}
	items, err := t.openMenu(ctx, p.ID)
	if err != nil {
		return nil, err
	}
	if _, ok := itemOf(items, "Tampilkan"); !ok {
		return nil, fmt.Errorf("%w: product %d's Lainnya has no Tampilkan (%s)", ErrPage, p.ID, labels(items))
	}
	if err := t.Press(ctx, "Tampilkan", "pressMenu", "Tampilkan"); err != nil {
		return nil, err
	}
	q, err := t.waitStatus(ctx, p.ID, "the product to leave the archive", 30*time.Second, func(q *Product) bool {
		return q != nil && q.Status != "archived"
	})
	if err != nil {
		return nil, t.refusal(ctx, err)
	}
	t.Log(fmt.Sprintf("%d is %s", p.ID, q.Status))
	return q, nil
}

// Archive hides a live product from buyers: its row's Lainnya, Arsipkan,
// then Konfirmasi when Seller Centre asks. An archived product is left as
// it is. It brings Chrome to the front.
func (s *Service) Archive(ctx context.Context, browser string, id int64) (*Product, error) {
	if id <= 0 {
		return nil, fmt.Errorf("%w: no product id", ErrInvalid)
	}
	var out *Product
	err := s.run(ctx, browser, true, func(t *session) error {
		p, err := t.existing(ctx, id)
		if err != nil {
			return err
		}
		if p.Status == "archived" {
			t.Log(fmt.Sprintf("%d is archived already", id))
			out = p
			return nil
		}
		out, err = t.archive(ctx, p)
		return err
	})
	return out, err
}

func (t *session) archive(ctx context.Context, p *Product) (*Product, error) {
	t.Log(fmt.Sprintf("archiving %d %q (Arsipkan)", p.ID, p.Name))
	if err := t.showRow(ctx, p.ID); err != nil {
		return nil, err
	}
	items, err := t.openMenu(ctx, p.ID)
	if err != nil {
		return nil, err
	}
	if _, ok := itemOf(items, "Arsipkan"); !ok {
		return nil, fmt.Errorf("%w: product %d's Lainnya has no Arsipkan (%s)", ErrPage, p.ID, labels(items))
	}
	if err := t.Press(ctx, "Arsipkan", "pressMenu", "Arsipkan"); err != nil {
		return nil, err
	}
	if err := t.answer(ctx, "mengarsipkan", "Konfirmasi"); err != nil {
		return nil, err
	}
	q, err := t.waitStatus(ctx, p.ID, "the product to be archived", 30*time.Second, func(q *Product) bool {
		return q != nil && q.Status == "archived"
	})
	if err != nil {
		return nil, t.refusal(ctx, err)
	}
	return q, nil
}

// refusal adds what Seller Centre says in a toast to a wait that ran out.
func (t *session) refusal(ctx context.Context, err error) error {
	var toasts []string
	if t.Call(ctx, &toasts, "toasts") == nil && len(toasts) > 0 {
		return fmt.Errorf("%w: Seller Centre says %q (%v)", ErrRefused, strings.Join(toasts, " / "), err)
	}
	return err
}

// ---- delete ----------------------------------------------------------------------------

// Delete deletes a product: its row's Lainnya, Hapus, then Hapus when
// Seller Centre asks "Yakin ingin menghapus produk ini?" about it by name.
// Seller Centre deletes only archived products, so a live one is archived
// first. The product must be the shop's; it is gone from the list
// afterwards. It cannot be undone. It brings Chrome to the front.
func (s *Service) Delete(ctx context.Context, browser string, id int64) (*Product, error) {
	if id <= 0 {
		return nil, fmt.Errorf("%w: no product id", ErrInvalid)
	}
	var out *Product
	err := s.run(ctx, browser, true, func(t *session) error {
		p, err := t.existing(ctx, id)
		if err != nil {
			return err
		}
		out = p
		return t.delete(ctx, p)
	})
	return out, err
}

func (t *session) delete(ctx context.Context, p *Product) error {
	t.Log(fmt.Sprintf("deleting %d %q (%s)", p.ID, p.Name, p.Status))
	if err := t.showRow(ctx, p.ID); err != nil {
		return err
	}
	items, err := t.openMenu(ctx, p.ID)
	if err != nil {
		return err
	}
	if _, ok := itemOf(items, "Hapus"); !ok {
		if _, ok := itemOf(items, "Arsipkan"); !ok {
			return fmt.Errorf("%w: product %d's Lainnya has neither Hapus nor Arsipkan (%s)", ErrPage, p.ID, labels(items))
		}
		t.Log("it is live: archiving it first, since Seller Centre deletes only archived products")
		if _, err := t.archive(ctx, p); err != nil {
			return err
		}
		// The list keeps the row as it was (its menu still that of a live
		// product) until it is loaded again.
		for try := 1; ; try++ {
			t.Log("loading Produk Saya again")
			if err := t.Navigate(ctx, listURL); err != nil {
				return err
			}
			if _, err := t.waitState(ctx, "Produk Saya", time.Minute, func(st state) bool { return st.ListReady }); err != nil {
				return err
			}
			if err := t.showRow(ctx, p.ID); err != nil {
				return err
			}
			if items, err = t.openMenu(ctx, p.ID); err != nil {
				return err
			}
			if _, ok := itemOf(items, "Hapus"); ok {
				break
			}
			if try == 3 {
				return fmt.Errorf("%w: archived, product %d's Lainnya still has no Hapus (%s)", ErrPage, p.ID, labels(items))
			}
			if err := page.Sleep(ctx, 3*time.Second); err != nil {
				return err
			}
		}
	}
	if err := t.Press(ctx, "Hapus", "pressMenu", "Hapus"); err != nil {
		return err
	}
	// The question names the product: Hapus is pressed only in that one.
	var m *modal
	if err := t.Until(ctx, "Seller Centre's question \"Hapus produk\"", 15*time.Second, func() (bool, error) {
		err := t.Call(ctx, &m, "modal")
		return err == nil && m != nil && strings.Contains(m.Text, "Hapus produk"), err
	}); err != nil {
		return err
	}
	if !strings.Contains(squash(m.Text), squash(p.Name)) {
		return fmt.Errorf("%w: Seller Centre asks %q, which does not name %q; not deleting", ErrPage, m.Text, p.Name)
	}
	if err := t.answer(ctx, p.Name, "Hapus"); err != nil {
		return err
	}
	// Seller Centre's search goes on finding a deleted product for a while:
	// the list itself says when it is gone.
	err = t.Until(ctx, "the product to leave Produk Saya", 2*time.Minute, func() (bool, error) {
		there := false
		err := t.scan(ctx, "all", func(l listedProduct) bool {
			there = l.ID == p.ID
			return there
		})
		if err != nil || !there {
			return err == nil, err
		}
		return false, page.Sleep(ctx, 2*time.Second)
	})
	if err != nil {
		return t.refusal(ctx, err)
	}
	t.Log(fmt.Sprintf("%d is gone from Produk Saya", p.ID))
	return nil
}

func squash(s string) string { return strings.ToLower(strings.Join(strings.Fields(s), "")) }

// ---- Naikkan Produk --------------------------------------------------------------------

// bumpedInfo is what shopee.js bumped() reports.
type bumpedInfo struct {
	Code     int    `json:"code"`
	Message  string `json:"message"`
	Total    int    `json:"total"`
	Cooldown int64  `json:"cooldown"`
	Products []struct {
		ID   int64 `json:"id"`
		Left int64 `json:"left"`
	} `json:"products"`
}

func (t *session) bumped(ctx context.Context) (*bumpedInfo, error) {
	var b bumpedInfo
	if err := t.CallFor(ctx, 30*time.Second, &b, "bumped"); err != nil {
		return nil, err
	}
	if b.Code != 0 {
		return nil, fmt.Errorf("%w: Naikkan Produk's slots do not read (%d %s)", ErrPage, b.Code, b.Message)
	}
	return &b, nil
}

// boosts turns the slots into Boosts, as of now, with the products' names
// where the shop still has them.
func (t *session) boosts(ctx context.Context, b *bumpedInfo, now time.Time) (*Boosts, error) {
	out := &Boosts{Total: b.Total, Cooldown: time.Duration(b.Cooldown) * time.Second}
	names := map[int64]string{}
	if len(b.Products) > 0 {
		left := len(b.Products)
		want := map[int64]bool{}
		for _, p := range b.Products {
			want[p.ID] = true
		}
		if err := t.scan(ctx, "all", func(l listedProduct) bool {
			if want[l.ID] {
				names[l.ID] = l.Name
				left--
			}
			return left == 0
		}); err != nil {
			return nil, err
		}
	}
	for _, p := range b.Products {
		out.Boosts = append(out.Boosts, Boost{ID: p.ID, Name: names[p.ID], EndsAt: now.Add(time.Duration(p.Left) * time.Second).Truncate(time.Second), SecondsLeft: p.Left})
	}
	return out, nil
}

// Boosted lists Naikkan Produk's slots in use: each bumped product, and when
// its slot is free again. A product deleted while bumped holds its slot all
// the same, with no name. It reads in a background tab.
func (s *Service) Boosted(ctx context.Context, browser string) (*Boosts, error) {
	var out *Boosts
	err := s.run(ctx, browser, false, func(t *session) error {
		t.Log("reading Naikkan Produk's slots")
		b, err := t.bumped(ctx)
		if err != nil {
			return err
		}
		out, err = t.boosts(ctx, b, s.now())
		return err
	})
	return out, err
}

// Boost bumps a live product to the top of the shop's products for buyers:
// its row's Lainnya, Naikkan Produk. What Seller Centre would refuse is
// checked first: a product that is not live (ErrNotLive), one bumped less
// than 4 hours ago (ErrCooldown), and all slots in use (ErrSlotsFull). It
// returns the product's slot, and the slots in use. It brings Chrome to the
// front.
func (s *Service) Boost(ctx context.Context, browser string, id int64) (*Boost, *Boosts, error) {
	if id <= 0 {
		return nil, nil, fmt.Errorf("%w: no product id", ErrInvalid)
	}
	var out *Boost
	var all *Boosts
	err := s.run(ctx, browser, true, func(t *session) error {
		var err error
		out, all, err = t.boost(ctx, id)
		return err
	})
	return out, all, err
}

func (t *session) boost(ctx context.Context, id int64) (*Boost, *Boosts, error) {
	p, err := t.existing(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	if p.Status != "live" {
		return nil, nil, fmt.Errorf("%w: %d %q is %s; Naikkan Produk is only for live products", ErrNotLive, p.ID, p.Name, p.Status)
	}
	b, err := t.bumped(ctx)
	if err != nil {
		return nil, nil, err
	}
	if err := checkSlots(b, id, t.now()); err != nil {
		return nil, nil, err
	}
	t.Log(fmt.Sprintf("bumping %d %q (Naikkan Produk); %d of %d slots in use", p.ID, p.Name, len(b.Products), b.Total))
	if err := t.showRow(ctx, id); err != nil {
		return nil, nil, err
	}
	items, err := t.openMenu(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	it, ok := itemOf(items, "Naikkan Produk")
	switch {
	case !ok:
		for _, x := range items {
			if countdown.MatchString(x.Label) {
				return nil, nil, fmt.Errorf("%w: Seller Centre shows %s left (%s)", ErrCooldown, x.Label, x.Tip)
			}
		}
		return nil, nil, fmt.Errorf("%w: product %d's Lainnya has no Naikkan Produk (%s)", ErrPage, id, labels(items))
	case it.Disabled:
		return nil, nil, fmt.Errorf("%w: Naikkan Produk is off: %s", ErrRefused, it.Tip)
	}
	if err := t.Press(ctx, "Naikkan Produk", "pressMenu", "Naikkan Produk"); err != nil {
		return nil, nil, err
	}
	var after *bumpedInfo
	if err := t.Until(ctx, "the product to take a Naikkan Produk slot", 20*time.Second, func() (bool, error) {
		var err error
		after, err = t.bumped(ctx)
		if err != nil {
			return false, err
		}
		for _, x := range after.Products {
			if x.ID == id {
				return true, nil
			}
		}
		return false, page.Sleep(ctx, time.Second)
	}); err != nil {
		return nil, nil, t.refusal(ctx, err)
	}
	all, err := t.boosts(ctx, after, t.now())
	if err != nil {
		return nil, nil, err
	}
	for _, x := range all.Boosts {
		if x.ID == id {
			x := x
			if x.Name == "" {
				x.Name = p.Name
			}
			t.Log(fmt.Sprintf("bumped until %s", x.EndsAt.Format("Mon 2 Jan 15:04:05")))
			return &x, all, nil
		}
	}
	return nil, nil, fmt.Errorf("%w: product %d left the slots at once", ErrPage, id)
}

// checkSlots refuses a bump Seller Centre would refuse: a product still on
// its cooldown, or no slot free.
func checkSlots(b *bumpedInfo, id int64, now time.Time) error {
	soonest := int64(-1)
	for _, x := range b.Products {
		if x.ID == id {
			at := now.Add(time.Duration(x.Left) * time.Second)
			return fmt.Errorf("%w: it can be bumped again at %s (in %s)", ErrCooldown, at.Format("Mon 2 Jan 15:04"), (time.Duration(x.Left) * time.Second).Round(time.Minute))
		}
		if soonest < 0 || x.Left < soonest {
			soonest = x.Left
		}
	}
	if b.Total > 0 && len(b.Products) >= b.Total {
		at := now.Add(time.Duration(soonest) * time.Second)
		return fmt.Errorf("%w: %d of %d; the first is free at %s (in %s)", ErrSlotsFull, len(b.Products), b.Total, at.Format("Mon 2 Jan 15:04"), (time.Duration(soonest) * time.Second).Round(time.Minute))
	}
	return nil
}
