package api

import (
	"context"
	"testing"

	"connectrpc.com/connect"

	gmapsv1 "github.com/wargasipil/san_remote/gen/san/remote/gmaps/v1"
	"github.com/wargasipil/san_remote/gen/san/remote/gmaps/v1/gmapsv1connect"
	"github.com/wargasipil/san_remote/internal/hubtest"
)

// SearchPlaces refuses a search Maps cannot run before a tab opens, and a
// good one gets as far as the browser (none is attached here).
func TestSearchPlacesChecks(t *testing.T) {
	h, server := hubtest.Start(t)
	h.AttachWait = 0
	ts := start(t, server)
	gm := gmapsv1connect.NewGoogleMapsServiceClient(ts.Client(), ts.URL)
	ctx := context.Background()
	search := func(r *gmapsv1.SearchPlacesRequest) error {
		_, err := gm.SearchPlaces(ctx, connect.NewRequest(r))
		return err
	}
	for name, r := range map[string]*gmapsv1.SearchPlacesRequest{
		"no keyword":      {Latitude: -6.2, Longitude: 106.816},
		"no point":        {Keyword: "kopi"},
		"both":            {Latitude: -6.2, Longitude: 106.816, PlusCode: "6P58RR48+P4", Keyword: "kopi"},
		"short plus code": {PlusCode: "RR48+P4 Jakarta", Keyword: "kopi"},
		"off the map":     {Latitude: 95, Longitude: 106.816, Keyword: "kopi"},
		"zoom":            {Latitude: -6.2, Longitude: 106.816, Keyword: "kopi", Zoom: 30},
		"limit":           {Latitude: -6.2, Longitude: 106.816, Keyword: "kopi", Limit: 1000},
	} {
		if err := search(r); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s: %v", name, err)
		}
	}
	for name, r := range map[string]*gmapsv1.SearchPlacesRequest{
		"point":     {Latitude: -6.2, Longitude: 106.816, Keyword: "kopi"},
		"plus code": {PlusCode: "6P58RR48+P4", Keyword: "kopi", Zoom: 13},
	} {
		if err := search(r); err == nil || connect.CodeOf(err) == connect.CodeInvalidArgument {
			t.Errorf("%s: %v", name, err)
		}
	}
}
