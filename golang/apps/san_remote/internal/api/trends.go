package api

import (
	"context"
	"strconv"
	"time"

	"connectrpc.com/connect"

	trendsv1 "github.com/wargasipil/san_remote/gen/san/remote/trends/v1"
	"github.com/wargasipil/san_remote/internal/trends"
)

// ---- Google Trends (2026-11-11) -------------------------------------------------------------

type trendsAPI struct{ a *API }

func (s trendsAPI) GetStatus(ctx context.Context, req *connect.Request[trendsv1.GetStatusRequest]) (*connect.Response[trendsv1.GetStatusResponse], error) {
	st, err := s.a.Trends.Status(ctx, req.Msg.Browser)
	if err != nil {
		return nil, s.a.fail(err)
	}
	return reply(&trendsv1.GetStatusResponse{NewExplore: st.NewExplore, Switched: st.Switched, SignedIn: st.SignedIn, Url: st.URL}, nil)
}

// Related checks the lookup (keyword, location, time range, search type)
// before a tab opens.
func (s trendsAPI) Related(ctx context.Context, req *connect.Request[trendsv1.RelatedRequest]) (*connect.Response[trendsv1.RelatedResponse], error) {
	m := req.Msg
	q := trends.Query{
		Browser: m.Browser, Keyword: m.Keyword, Geo: m.Geo, Time: m.Time, Category: int(m.Category), Property: m.Property,
		Wait: time.Duration(m.WaitSeconds) * time.Second,
	}
	if err := q.Check(); err != nil {
		return nil, s.a.fail(err)
	}
	r, err := s.a.Trends.Related(ctx, q)
	if err != nil {
		return nil, s.a.fail(err)
	}
	res := &trendsv1.RelatedResponse{Keyword: r.Keyword, Url: r.URL, Switched: r.Switched}
	for _, it := range r.Top {
		res.Top = append(res.Top, trendsQuery(it, it.SearchInterest, strconv.Itoa(it.SearchInterest)))
	}
	for _, it := range r.Rising {
		res.Rising = append(res.Rising, trendsQuery(it, it.Change, it.ChangeText()))
	}
	return reply(res, nil)
}

func trendsQuery(it trends.Item, value int, formatted string) *trendsv1.RelatedQuery {
	return &trendsv1.RelatedQuery{
		Query: it.Query, Value: int32(value), FormattedValue: formatted, Breakout: it.Breakout, Link: it.Link,
		SearchInterest: int32(it.SearchInterest), Change: int32(it.Change),
	}
}

func (s trendsAPI) Pickers(ctx context.Context, req *connect.Request[trendsv1.PickersRequest]) (*connect.Response[trendsv1.PickersResponse], error) {
	p, err := s.a.Trends.Pickers(ctx, req.Msg.Browser, req.Msg.Country)
	if err != nil {
		return nil, s.a.fail(err)
	}
	res := &trendsv1.PickersResponse{}
	for _, g := range p.Geos {
		res.Geos = append(res.Geos, trendsGeo(g))
	}
	for _, c := range p.Categories {
		res.Categories = append(res.Categories, trendsCategory(c))
	}
	for _, o := range p.Times {
		res.Times = append(res.Times, &trendsv1.Option{Value: o.Value, Label: o.Label})
	}
	for _, o := range p.Properties {
		res.Properties = append(res.Properties, &trendsv1.Option{Value: o.Value, Label: o.Label})
	}
	return reply(res, nil)
}

func trendsGeo(g trends.Geo) *trendsv1.Geo {
	out := &trendsv1.Geo{Code: g.Code, Name: g.Name, HasRegions: g.HasRegions}
	for _, r := range g.Regions {
		out.Regions = append(out.Regions, trendsGeo(r))
	}
	return out
}

func trendsCategory(c trends.Category) *trendsv1.Category {
	out := &trendsv1.Category{Id: int32(c.ID), Name: c.Name}
	for _, ch := range c.Children {
		out.Children = append(out.Children, trendsCategory(ch))
	}
	return out
}
