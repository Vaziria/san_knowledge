package api

import (
	"context"

	"connectrpc.com/connect"

	gmapsv1 "github.com/wargasipil/san_remote/gen/san/remote/gmaps/v1"
	"github.com/wargasipil/san_remote/internal/gmaps"
)

// ---- Google Maps (2026-11-06) -------------------------------------------------------------

type gmapsAPI struct{ a *API }

// SearchPlaces checks the search (the point, the zoom, the keyword) before a
// tab opens.
func (s gmapsAPI) SearchPlaces(ctx context.Context, req *connect.Request[gmapsv1.SearchPlacesRequest]) (*connect.Response[gmapsv1.SearchPlacesResponse], error) {
	m := req.Msg
	q := gmaps.Search{
		Browser: m.Browser, Lat: m.Latitude, Lng: m.Longitude, PlusCode: m.PlusCode, Zoom: m.Zoom,
		Keyword: m.Keyword, Limit: int(m.Limit), VisitWebsites: m.VisitWebsites,
	}
	if err := q.Check(); err != nil {
		return nil, s.a.fail(err)
	}
	r, err := s.a.GoogleMaps.Search(ctx, q)
	if err != nil {
		return nil, s.a.fail(err)
	}
	res := &gmapsv1.SearchPlacesResponse{Latitude: r.Lat, Longitude: r.Lng, Zoom: r.Zoom, EndOfList: r.End, LimitedView: r.Limited}
	for _, p := range r.Places {
		res.Places = append(res.Places, gmapsPlace(p))
	}
	return reply(res, nil)
}

func gmapsPlace(p gmaps.Place) *gmapsv1.Place {
	out := &gmapsv1.Place{
		Name: p.Name, Category: p.Category, Address: p.Address, Phone: p.Phone, Tel: p.Tel, Website: p.Website,
		PlusCode: p.PlusCode, PlusCodeLocal: p.PlusCodeLocal, Latitude: p.Lat, Longitude: p.Lng,
		Rating: p.Rating, Reviews: uint32(p.Reviews), PlaceId: p.PlaceID, Url: p.URL, WebsiteError: p.WebsiteError,
	}
	for _, s := range p.Social {
		out.Social = append(out.Social, &gmapsv1.SocialLink{Site: s.Site, Url: s.URL})
	}
	return out
}
