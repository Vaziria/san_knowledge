package api

import (
	"context"
	"fmt"
	"strings"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	shopeev1 "github.com/wargasipil/san_remote/gen/san/remote/shopee/v1"
	"github.com/wargasipil/san_remote/internal/shopee"
)

// ---- Shopee (2026-11-03) ----------------------------------------------------------------

type shopeeAPI struct{ a *API }

func (s shopeeAPI) GetStatus(ctx context.Context, req *connect.Request[shopeev1.GetStatusRequest]) (*connect.Response[shopeev1.GetStatusResponse], error) {
	st, err := s.a.Shopee.Status(ctx, req.Msg.Browser)
	if err != nil {
		return nil, s.a.fail(err)
	}
	return reply(&shopeev1.GetStatusResponse{Status: &shopeev1.Status{SignedIn: st.SignedIn, Shop: st.Shop, Username: st.Username, ShopId: st.ShopID}}, nil)
}

func (s shopeeAPI) ListProducts(ctx context.Context, req *connect.Request[shopeev1.ListProductsRequest]) (*connect.Response[shopeev1.ListProductsResponse], error) {
	if _, ok := shopee.Filters[strings.ToLower(strings.TrimSpace(req.Msg.Filter))]; !ok && strings.TrimSpace(req.Msg.Filter) != "" {
		return nil, invalid("filter %q: give all, live, archived, review, violation or draft", req.Msg.Filter)
	}
	list, total, err := s.a.Shopee.Products(ctx, req.Msg.Browser, req.Msg.Filter, int(req.Msg.Limit), req.Msg.Search)
	if err != nil {
		return nil, s.a.fail(err)
	}
	res := &shopeev1.ListProductsResponse{Total: uint32(total)}
	for i := range list {
		res.Products = append(res.Products, shopeeProduct(&list[i]))
	}
	return reply(res, nil)
}

// CreateProduct checks the product, its photos and its variations before a
// tab opens.
func (s shopeeAPI) CreateProduct(ctx context.Context, req *connect.Request[shopeev1.CreateProductRequest]) (*connect.Response[shopeev1.CreateProductResponse], error) {
	n, err := shopeeNewOf(req.Msg)
	if err != nil {
		return nil, err
	}
	if _, err := n.Check(); err != nil {
		return nil, s.a.fail(err)
	}
	c, err := s.a.Shopee.Create(ctx, *n)
	if err != nil {
		if c != nil && c.Product.ID != 0 {
			// Seller Centre made it: say which, to check or delete it there.
			err = fmt.Errorf("%w (product %d was made: %s)", err, c.Product.ID, c.Product.URL())
		}
		return nil, s.a.fail(err)
	}
	return reply(&shopeev1.CreateProductResponse{Product: shopeeProduct(&c.Product), Form: c.Form}, nil)
}

func (s shopeeAPI) DeleteProduct(ctx context.Context, req *connect.Request[shopeev1.DeleteProductRequest]) (*connect.Response[shopeev1.DeleteProductResponse], error) {
	if req.Msg.Id <= 0 {
		return nil, invalid("id is required")
	}
	p, err := s.a.Shopee.Delete(ctx, req.Msg.Browser, req.Msg.Id)
	if err != nil {
		return nil, s.a.fail(err)
	}
	return reply(&shopeev1.DeleteProductResponse{Product: shopeeProduct(p)}, nil)
}

func (s shopeeAPI) PublishProduct(ctx context.Context, req *connect.Request[shopeev1.PublishProductRequest]) (*connect.Response[shopeev1.PublishProductResponse], error) {
	if req.Msg.Id <= 0 {
		return nil, invalid("id is required")
	}
	p, err := s.a.Shopee.Publish(ctx, req.Msg.Browser, req.Msg.Id)
	if err != nil {
		return nil, s.a.fail(err)
	}
	return reply(&shopeev1.PublishProductResponse{Product: shopeeProduct(p)}, nil)
}

func (s shopeeAPI) ArchiveProduct(ctx context.Context, req *connect.Request[shopeev1.ArchiveProductRequest]) (*connect.Response[shopeev1.ArchiveProductResponse], error) {
	if req.Msg.Id <= 0 {
		return nil, invalid("id is required")
	}
	p, err := s.a.Shopee.Archive(ctx, req.Msg.Browser, req.Msg.Id)
	if err != nil {
		return nil, s.a.fail(err)
	}
	return reply(&shopeev1.ArchiveProductResponse{Product: shopeeProduct(p)}, nil)
}

func (s shopeeAPI) BoostProduct(ctx context.Context, req *connect.Request[shopeev1.BoostProductRequest]) (*connect.Response[shopeev1.BoostProductResponse], error) {
	if req.Msg.Id <= 0 {
		return nil, invalid("id is required")
	}
	b, all, err := s.a.Shopee.Boost(ctx, req.Msg.Browser, req.Msg.Id)
	if err != nil {
		return nil, s.a.fail(err)
	}
	return reply(&shopeev1.BoostProductResponse{Boost: shopeeBoost(*b), SlotsUsed: uint32(len(all.Boosts)), SlotsTotal: uint32(all.Total)}, nil)
}

func (s shopeeAPI) ListBoosted(ctx context.Context, req *connect.Request[shopeev1.ListBoostedRequest]) (*connect.Response[shopeev1.ListBoostedResponse], error) {
	all, err := s.a.Shopee.Boosted(ctx, req.Msg.Browser)
	if err != nil {
		return nil, s.a.fail(err)
	}
	res := &shopeev1.ListBoostedResponse{SlotsUsed: uint32(len(all.Boosts)), SlotsTotal: uint32(all.Total), CooldownSeconds: int64(all.Cooldown.Seconds())}
	for _, b := range all.Boosts {
		res.Boosts = append(res.Boosts, shopeeBoost(b))
	}
	return reply(res, nil)
}

// shopeeNewOf reads a CreateProduct request: a markdown file or text, with
// what the request gives besides winning, or the request's fields alone.
// Files are absolute paths.
func shopeeNewOf(m *shopeev1.CreateProductRequest) (*shopee.New, error) {
	n := &shopee.New{}
	file, md := strings.TrimSpace(m.File), strings.TrimSpace(m.Markdown)
	switch {
	case file != "" && md != "":
		return nil, invalid("give file or markdown, not both")
	case file != "":
		if err := absolute("file", file); err != nil {
			return nil, err
		}
		var err error
		if n, err = shopee.ParseFile(file); err != nil {
			return nil, parsed(err)
		}
	case md != "":
		var err error
		if n, err = shopee.Parse([]byte(md)); err != nil {
			return nil, parsed(err)
		}
		if err := absoluteIn("photo", n.Photos...); err != nil {
			return nil, err
		}
	}
	n.Browser, n.Rehearse = m.Browser, m.Rehearse
	if v := strings.TrimSpace(m.Name); v != "" {
		n.Name = v
	}
	if strings.TrimSpace(m.Description) != "" {
		n.Description = strings.TrimSpace(m.Description)
	}
	if len(m.Photos) > 0 {
		if err := absoluteAll("photo", m.Photos); err != nil {
			return nil, err
		}
		n.Photos = m.Photos
	}
	if len(m.Category) > 0 {
		n.Category = shopee.SplitCategory(strings.Join(m.Category, ">"))
	}
	if len(m.Attributes) > 0 {
		if n.Attributes == nil {
			n.Attributes = map[string]string{}
		}
		for k, v := range m.Attributes {
			n.Attributes[k] = v
		}
	}
	if len(m.Variations) > 0 {
		n.Variations = nil
		for _, v := range m.Variations {
			n.Variations = append(n.Variations, shopee.Variation{Name: v.Name, Options: v.Options})
		}
	}
	if len(m.Models) > 0 {
		n.Models = nil
		for _, x := range m.Models {
			n.Models = append(n.Models, shopee.NewModel{Options: x.Options, Price: x.Price, Stock: x.Stock, SKU: x.Sku})
		}
	}
	if m.Price != 0 {
		n.Price = m.Price
	}
	if m.Stock != nil {
		v := *m.Stock
		n.Stock = &v
	}
	if v := strings.TrimSpace(m.Sku); v != "" {
		n.SKU = v
	}
	if m.WeightGrams != 0 {
		n.Weight = int(m.WeightGrams)
	}
	if d := m.Dimensions; d != nil && (d.Length != 0 || d.Width != 0 || d.Height != 0) {
		n.Dimensions = shopee.Dimensions{Length: int(d.Length), Width: int(d.Width), Height: int(d.Height)}
	}
	if strings.TrimSpace(m.Condition) != "" {
		used, err := shopee.ParseCondition(m.Condition)
		if err != nil {
			return nil, parsed(err)
		}
		n.Used = used
	}
	if len(m.Couriers) > 0 {
		n.Couriers = m.Couriers
	}
	n.Dangerous = n.Dangerous || m.DangerousGoods
	if strings.TrimSpace(m.Publish) != "" {
		live, err := shopee.ParsePublish(m.Publish)
		if err != nil {
			return nil, parsed(err)
		}
		n.Live = live
	}
	return n, nil
}

func absoluteAll(what string, paths []string) error {
	for _, p := range paths {
		if err := absolute(what, p); err != nil {
			return err
		}
	}
	return nil
}

func shopeeProduct(p *shopee.Product) *shopeev1.Product {
	out := &shopeev1.Product{
		Id: p.ID, Name: p.Name, Status: p.Status, PriceMin: p.PriceMin, PriceMax: p.PriceMax, Stock: p.Stock, Sold: p.Sold,
		Sku: p.SKU, Cover: p.Cover, Created: stamp(p.Created), Updated: stamp(p.Updated),
	}
	if p.ID != 0 {
		out.Url = p.URL()
	}
	for _, m := range p.Models {
		out.Models = append(out.Models, &shopeev1.Model{Id: m.ID, Options: m.Options, Price: m.Price, Stock: m.Stock, Sku: m.SKU})
	}
	if p.PriceMin == 0 && p.PriceMax == 0 && len(p.Models) > 0 {
		// A rehearsal's product: its range from the models it would have.
		out.PriceMin, out.PriceMax = p.Models[0].Price, p.Models[0].Price
		for _, m := range p.Models {
			out.PriceMin, out.PriceMax = min(out.PriceMin, m.Price), max(out.PriceMax, m.Price)
			out.Stock += m.Stock
		}
	}
	return out
}

func shopeeBoost(b shopee.Boost) *shopeev1.Boost {
	return &shopeev1.Boost{Id: b.ID, Name: b.Name, EndsAt: timestamppb.New(b.EndsAt), SecondsLeft: b.SecondsLeft}
}
