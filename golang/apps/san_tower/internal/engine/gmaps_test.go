package engine

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"connectrpc.com/connect"
	gmapsv1 "github.com/wargasipil/san_remote/gen/san/remote/gmaps/v1"
	"github.com/wargasipil/san_remote/gen/san/remote/gmaps/v1/gmapsv1connect"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/store"
)

// fakeGmaps is san_remote api's GoogleMapsService: it keeps the request and
// answers with two places.
type fakeGmaps struct {
	gmapsv1connect.UnimplementedGoogleMapsServiceHandler
	req *gmapsv1.SearchPlacesRequest
}

func (f *fakeGmaps) SearchPlaces(_ context.Context, req *connect.Request[gmapsv1.SearchPlacesRequest]) (*connect.Response[gmapsv1.SearchPlacesResponse], error) {
	f.req = req.Msg
	return connect.NewResponse(&gmapsv1.SearchPlacesResponse{EndOfList: true, Places: []*gmapsv1.Place{
		{
			Name: "Toko Kopi Style", Category: "Coffee shop", Address: "Jl. Kebon Kacang Raya No.17, Jakarta", Phone: "0813-2345-6387", Tel: "081323456387",
			Website: "https://tr.ee/MrnAIt", Social: []*gmapsv1.SocialLink{{Site: "instagram", Url: "https://www.instagram.com/tokokopistyle"}},
			PlusCode: "6P58RR48+P4", PlusCodeLocal: "RR48+P4 Kebon Kacang, Central Jakarta City, Jakarta", Latitude: -6.1931299, Longitude: 106.8152684,
		},
		{Name: "Shan Coffee", PlusCode: "6P58RRQP+R9"},
	}}), nil
}

func gmapsEnv(t *testing.T, f *fakeGmaps) Env {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle(gmapsv1connect.NewGoogleMapsServiceHandler(f))
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return Env{Connection: func(id uint32) (store.Connection, error) {
		if id != 1 {
			return store.Connection{}, store.ErrNotFound
		}
		return store.Connection{ID: 1, Name: "Peta", Type: store.SanRemote, Settings: &v1.ConnectionSettings{
			Type: &v1.ConnectionSettings_SanRemote{SanRemote: &v1.SanRemoteSettings{Url: ts.URL, Browser: "peta"}},
		}}, nil
	}}
}

// Every field of the step shows an example in its empty box.
func TestGmapsHints(t *testing.T) {
	for _, k := range gmapsKinds {
		for _, in := range k.Inputs {
			if in.Field && in.Placeholder == "" {
				t.Errorf("%s: field %s has no placeholder", k.Kind, in.Name)
			}
		}
	}
}

func TestGmapsSearch(t *testing.T) {
	f := &fakeGmaps{}
	env := gmapsEnv(t, f)
	s, err := runOne(t, env, "gmaps-search", "Latitude", " -6.2 ", "Longitude", "106.816", "Keyword", "  kopi   susu ", "Zoom", "14", "Limit", "5")
	if err != nil {
		t.Fatal(err)
	}
	if r := f.req; r.GetLatitude() != -6.2 || r.GetLongitude() != 106.816 || r.GetKeyword() != "kopi susu" || r.GetZoom() != 14 ||
		r.GetLimit() != 5 || !r.GetVisitWebsites() || r.GetPlusCode() != "" || r.GetBrowser() != "peta" {
		t.Errorf("request = %v", r)
	}
	if strings.Join(s.Fired, ",") != "Done" || textOut(t, s, "Count") != "2" || textOut(t, s, "End Of List") != "Yes" {
		t.Errorf("fired %v, Count %s, End Of List %s", s.Fired, textOut(t, s, "Count"), textOut(t, s, "End Of List"))
	}
	places := objectOut(t, s, "Places").GetListValue().GetValues()
	if len(places) != 2 {
		t.Fatalf("Places = %v", places)
	}
	p := places[0].GetStructValue().Fields
	if p["name"].GetStringValue() != "Toko Kopi Style" || p["phone"].GetStringValue() != "0813-2345-6387" || p["plusCode"].GetStringValue() != "6P58RR48+P4" ||
		p["address"].GetStringValue() == "" || len(p["social"].GetListValue().GetValues()) != 1 || p["latitude"].GetNumberValue() != -6.1931299 {
		t.Errorf("place 1 = %v", p)
	}

	// Both in Latitude, as Maps copies a point; Website Only; the defaults.
	if _, err := runOne(t, env, "gmaps-search", "Latitude", "-6.2, 106.816", "Keyword", "kopi", "Social Links", "Website Only"); err != nil {
		t.Fatal(err)
	}
	if r := f.req; r.GetLatitude() != -6.2 || r.GetLongitude() != 106.816 || r.GetVisitWebsites() || r.GetZoom() != 0 || r.GetLimit() != 0 {
		t.Errorf("one field, Website Only, defaults: %v", r)
	}
	if _, err := runOne(t, env, "gmaps-search", "Location", "Plus Code", "Plus Code", " 6P58RR48+P4 ", "Keyword", "kopi", "Zoom", "13.5z"); err != nil {
		t.Fatal(err)
	}
	if r := f.req; r.GetPlusCode() != "6P58RR48+P4" || r.GetLatitude() != 0 || r.GetZoom() != 13.5 {
		t.Errorf("plus code: %v", r)
	}

	for name, c := range map[string]struct {
		values []string
		want   string
	}{
		"no keyword":    {[]string{"Latitude", "-6.2", "Longitude", "106.8"}, "Keyword"},
		"no point":      {[]string{"Keyword", "kopi"}, "Latitude and Longitude"},
		"bad latitude":  {[]string{"Latitude", "95", "Longitude", "106.8", "Keyword", "kopi"}, "Latitude"},
		"bad longitude": {[]string{"Latitude", "-6.2", "Longitude", "east", "Keyword", "kopi"}, "Longitude"},
		"no plus code":  {[]string{"Location", "Plus Code", "Keyword", "kopi"}, "Plus Code"},
		"zoom":          {[]string{"Latitude", "-6.2", "Longitude", "106.8", "Keyword", "kopi", "Zoom", "25"}, "Zoom"},
		"limit":         {[]string{"Latitude", "-6.2", "Longitude", "106.8", "Keyword", "kopi", "Limit", "0"}, "Limit"},
		"limit words":   {[]string{"Latitude", "-6.2", "Longitude", "106.8", "Keyword", "kopi", "Limit", "all"}, "Limit"},
	} {
		f.req = nil
		if _, err := runOne(t, env, "gmaps-search", c.values...); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v, want an error about %s", name, err, c.want)
		}
		if f.req != nil {
			t.Errorf("%s: asked san_remote anyway", name)
		}
	}
}
