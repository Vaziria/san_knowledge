package engine

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	shopeev1 "github.com/wargasipil/san_remote/gen/san/remote/shopee/v1"
	"github.com/wargasipil/san_remote/gen/san/remote/shopee/v1/shopeev1connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/store"
)

func TestShopeeID(t *testing.T) {
	for _, c := range []struct {
		in   string
		want int64
	}{
		{"57614536675", 57614536675},
		{" 57614536675 ", 57614536675},
		{"https://seller.shopee.co.id/portal/product/55071063221", 55071063221},
		{"https://shopee.co.id/Kaos-Polos-i.77987996.55071063221?sp_atk=x", 55071063221},
		{"https://shopee.co.id/product/77987996/55071063221", 55071063221},
		{"", 0},
		{"kaos", 0},
		{"-5", 0},
	} {
		got, err := shopeeID(c.in)
		if got != c.want || (c.want == 0) != (err != nil) {
			t.Errorf("shopeeID(%q) = %d, %v; want %d", c.in, got, err, c.want)
		}
	}
}

// fakeShopee is san_remote api's ShopeeService: it keeps the requests, and
// refuses to bump product 1 as Seller Centre's cooldown does.
type fakeShopee struct {
	shopeev1connect.UnimplementedShopeeServiceHandler
	list   *shopeev1.ListProductsRequest
	create *shopeev1.CreateProductRequest
	del    *shopeev1.DeleteProductRequest
	boost  *shopeev1.BoostProductRequest
}

func (f *fakeShopee) ListProducts(_ context.Context, req *connect.Request[shopeev1.ListProductsRequest]) (*connect.Response[shopeev1.ListProductsResponse], error) {
	f.list = req.Msg
	return connect.NewResponse(&shopeev1.ListProductsResponse{Total: 5, Products: []*shopeev1.Product{
		{Id: 11, Name: "Kaos Hitam", Status: "live", Models: []*shopeev1.Model{{Id: 1, Options: []string{"Hitam", "M"}, Price: 75000, Stock: 3}}},
		{Id: 12, Name: "Kaos Putih", Status: "live"},
	}}), nil
}

func (f *fakeShopee) CreateProduct(_ context.Context, req *connect.Request[shopeev1.CreateProductRequest]) (*connect.Response[shopeev1.CreateProductResponse], error) {
	f.create = req.Msg
	return connect.NewResponse(&shopeev1.CreateProductResponse{
		Product: &shopeev1.Product{Id: 55071063221, Name: "Kaos", Status: "archived", Url: "https://seller.shopee.co.id/portal/product/55071063221"},
		Form:    []string{"name: Kaos"},
	}), nil
}

func (f *fakeShopee) DeleteProduct(_ context.Context, req *connect.Request[shopeev1.DeleteProductRequest]) (*connect.Response[shopeev1.DeleteProductResponse], error) {
	f.del = req.Msg
	return connect.NewResponse(&shopeev1.DeleteProductResponse{Product: &shopeev1.Product{Id: req.Msg.Id, Name: "Kaos"}}), nil
}

func (f *fakeShopee) BoostProduct(_ context.Context, req *connect.Request[shopeev1.BoostProductRequest]) (*connect.Response[shopeev1.BoostProductResponse], error) {
	f.boost = req.Msg
	if req.Msg.Id == 1 {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errorString("shopee: the product was bumped less than 4 hours ago"))
	}
	ends := time.Date(2026, 11, 3, 6, 11, 11, 0, time.UTC)
	return connect.NewResponse(&shopeev1.BoostProductResponse{
		Boost:     &shopeev1.Boost{Id: req.Msg.Id, Name: "Kaos", EndsAt: timestamppb.New(ends), SecondsLeft: 14400},
		SlotsUsed: 2, SlotsTotal: 5,
	}), nil
}

// shopeeEnv serves f as san_remote api, at connection 1, which picks the
// browser "toko".
func shopeeEnv(t *testing.T, f *fakeShopee) Env {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle(shopeev1connect.NewShopeeServiceHandler(f))
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return Env{Connection: func(id uint32) (store.Connection, error) {
		if id != 1 {
			return store.Connection{}, store.ErrNotFound
		}
		return store.Connection{ID: 1, Name: "Toko", Type: store.SanRemote, Settings: &v1.ConnectionSettings{
			Type: &v1.ConnectionSettings_SanRemote{SanRemote: &v1.SanRemoteSettings{Url: ts.URL, Browser: "toko"}},
		}}, nil
	}}
}

// runOne runs one step of kind after a Topic Event, and returns its run.
func runOne(t *testing.T, env Env, kind string, values ...string) (*v1.StepRun, error) {
	t.Helper()
	g := &v1.Graph{
		Steps: []*v1.Step{step("ev", TopicEvent), step("s", kind, append([]string{"Connection", "1"}, values...)...)},
		Links: []*v1.Link{link("ev", "Event", "s", "Event")},
	}
	steps, err := Execute(context.Background(), g, "ev", nil, env)
	for _, s := range steps {
		if s.StepId == "s" {
			return s, err
		}
	}
	return nil, err
}

func TestShopeeProducts(t *testing.T) {
	f := &fakeShopee{}
	env := shopeeEnv(t, f)
	s, err := runOne(t, env, "shopee-products", "Filter", "Live", "Search", " kaos ", "Limit", "10")
	if err != nil {
		t.Fatal(err)
	}
	if f.list.GetFilter() != "live" || f.list.GetLimit() != 10 || f.list.GetSearch() != "kaos" || f.list.GetBrowser() != "toko" {
		t.Errorf("request = %v", f.list)
	}
	if strings.Join(s.Fired, ",") != "Done" || textOut(t, s, "Total") != "5" || textOut(t, s, "Count") != "2" {
		t.Errorf("fired %v, Total %s, Count %s", s.Fired, textOut(t, s, "Total"), textOut(t, s, "Count"))
	}
	products := objectOut(t, s, "Products").GetListValue().GetValues()
	if len(products) != 2 || products[0].GetStructValue().Fields["name"].GetStringValue() != "Kaos Hitam" ||
		len(products[0].GetStructValue().Fields["models"].GetListValue().GetValues()) != 1 {
		t.Errorf("Products = %v", products)
	}
	// Defaults: all, 50.
	if _, err := runOne(t, env, "shopee-products"); err != nil || f.list.GetFilter() != "all" || f.list.GetLimit() != 50 {
		t.Errorf("defaults: %v, %v", f.list, err)
	}
	for _, limit := range []string{"0", "501", "ten"} {
		if _, err := runOne(t, env, "shopee-products", "Limit", limit); err == nil || !strings.Contains(err.Error(), "Limit") {
			t.Errorf("Limit %s: %v", limit, err)
		}
	}
}

func TestShopeeUpload(t *testing.T) {
	f := &fakeShopee{}
	env := shopeeEnv(t, f)
	s, err := runOne(t, env, "shopee-upload", "Source", "File", "File", "D:/toko/kaos.md", "Publish", "Live")
	if err != nil {
		t.Fatal(err)
	}
	if f.create.GetFile() != "D:/toko/kaos.md" || f.create.GetPublish() != "live" || f.create.GetRehearse() || f.create.GetBrowser() != "toko" {
		t.Errorf("request = %v", f.create)
	}
	if textOut(t, s, "ID") != "55071063221" || textOut(t, s, "Link") != "https://seller.shopee.co.id/portal/product/55071063221" {
		t.Errorf("ID %s, Link %s", textOut(t, s, "ID"), textOut(t, s, "Link"))
	}
	// As Written leaves publish to the file; Text sends the markdown.
	md := "---\nname: Kaos\n---\nKaos polos."
	if _, err := runOne(t, env, "shopee-upload", "Source", "Text", "Product", md, "Rehearse", "Yes"); err != nil {
		t.Fatal(err)
	}
	if f.create.GetMarkdown() != md || f.create.GetFile() != "" || f.create.GetPublish() != "" || !f.create.GetRehearse() {
		t.Errorf("text request = %v", f.create)
	}
	f.create = nil
	if _, err := runOne(t, env, "shopee-upload", "File", "kaos.md"); err == nil || !strings.Contains(err.Error(), "absolute") || f.create != nil {
		t.Errorf("relative File: %v, sent %v", err, f.create)
	}
	if _, err := runOne(t, env, "shopee-upload", "Source", "Text", "Product", " "); err == nil || f.create != nil {
		t.Errorf("empty Product: %v, sent %v", err, f.create)
	}
}

func TestShopeeDelete(t *testing.T) {
	f := &fakeShopee{}
	env := shopeeEnv(t, f)
	s, err := runOne(t, env, "shopee-delete", "Product", "https://seller.shopee.co.id/portal/product/55071063221")
	if err != nil {
		t.Fatal(err)
	}
	if f.del.GetId() != 55071063221 || f.del.GetBrowser() != "toko" || strings.Join(s.Fired, ",") != "Done" {
		t.Errorf("request %v, fired %v", f.del, s.Fired)
	}
	f.del = nil
	if _, err := runOne(t, env, "shopee-delete", "Product", "kaos"); err == nil || f.del != nil {
		t.Errorf("bad Product: %v, sent %v", err, f.del)
	}
}

// Naikkan Produk fires Boosted with when the slot ends, or Refused with
// san_remote's reason when Seller Centre would not bump it; Done either way.
func TestShopeeBoost(t *testing.T) {
	f := &fakeShopee{}
	env := shopeeEnv(t, f)
	s, err := runOne(t, env, "shopee-boost", "Product", "55071063221")
	if err != nil {
		t.Fatal(err)
	}
	until, _ := time.Parse(time.RFC3339, textOut(t, s, "Until"))
	if strings.Join(s.Fired, ",") != "Boosted,Done" || !until.Equal(time.Date(2026, 11, 3, 6, 11, 11, 0, time.UTC)) ||
		textOut(t, s, "Slots") != "2 of 5" || textOut(t, s, "Reason") != "" || f.boost.GetBrowser() != "toko" {
		t.Errorf("fired %v, Until %q, Slots %q", s.Fired, textOut(t, s, "Until"), textOut(t, s, "Slots"))
	}
	s, err = runOne(t, env, "shopee-boost", "Product", "1")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(s.Fired, ",") != "Refused,Done" || !strings.Contains(textOut(t, s, "Reason"), "4 hours") || textOut(t, s, "Until") != "" {
		t.Errorf("refused: fired %v, Reason %q", s.Fired, textOut(t, s, "Reason"))
	}
}
