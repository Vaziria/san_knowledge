package engine

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"connectrpc.com/connect"
	gmapsv1 "github.com/wargasipil/san_remote/gen/san/remote/gmaps/v1"
	"google.golang.org/protobuf/types/known/structpb"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
)

// The Google Maps step (the user, 2026-11-06: "can we remote google map ? and
// make it to node in san tower", from a point (latitude and longitude, or a
// plus code), a zoom level and a keyword, each place's name, address, social
// media links, phone number and plus code): through `san_remote api`'s
// GoogleMapsService at a San Remote connection, like the Shopee steps, so it
// reads Maps in the user's browser and needs no Places API key. Social media
// comes from each place's website (his choice: the website when it is a
// profile, and the links on it). Its own category, Maps.

var gmapsKinds = []*kind{
	{
		StepKind: &v1.StepKind{
			Kind: "gmaps-search", Title: "Google Maps Places", Category: "Maps",
			Group:       "Google Maps",
			Description: "Searches Google Maps for Keyword around a point, through San Remote in the connection's browser (no API key), and reads every place it lists (Maps lists about 120 at most), or at most Limit (1 to 500). Location says how the point is given: Coordinates (Latitude and Longitude, or both in Latitude as \"-6.2, 106.816\") or Plus Code (a full plus code, such as 6P58RR48+P4). Zoom is the map's zoom, 3 to 21 (default 15, a few streets; 12 is a city). Each place has its name, category, address, phone, website, social media links, plus code (full, and as Maps shows it), latitude and longitude, rating and reviews, and its Maps link. Social Links From Websites also reads each place's website for the links on it; Website Only takes the website when it is a profile. Reads in a background tab, a few seconds a place. Places is the list, Count how many came, End Of List Yes when the list was read to its end.",
			Outputs:     []*v1.Socket{sock("Done", Event), sock("Places", Object), sock("Count", Text), sock("End Of List", Text)},
			Settings: []*v1.Setting{
				connectionSetting(),
				{Name: "Location", Options: []string{"Coordinates", "Plus Code"}},
				{Name: "Social Links", Options: []string{"From Websites", "Website Only"}},
			},
			Inputs: []*v1.Socket{
				sock("Event", Event),
				only("Location=Coordinates", hint("-6.2, or -6.2, 106.816", field("Latitude"))),
				only("Location=Coordinates", hint("106.816", field("Longitude"))),
				only("Location=Plus Code", hint("6P58RR48+P4", field("Plus Code"))),
				hint("15 (3 to 21; 12 is a city)", field("Zoom")),
				hint("coffee, bengkel motor", field("Keyword")),
				hint("all (about 120), or 1 to 500", field("Limit")),
			},
		},
		run: gmapsSearch,
	},
}

func gmapsSearch(c *call) error {
	req, err := gmapsRequest(c)
	if err != nil {
		return err
	}
	r, err := c.sanRemote()
	if err != nil {
		return err
	}
	res, err := r.gmaps.SearchPlaces(c.ctx, connect.NewRequest(req))
	if err != nil {
		return remoteErr(err)
	}
	list := &structpb.ListValue{}
	for _, p := range res.Msg.GetPlaces() {
		list.Values = append(list.Values, toValue(p))
	}
	c.outputs["Places"] = structpb.NewListValue(list)
	c.outputs["Count"] = stringValue(strconv.Itoa(len(list.Values)))
	end := "No"
	if res.Msg.GetEndOfList() {
		end = "Yes"
	}
	c.outputs["End Of List"] = stringValue(end)
	c.fire("Done")
	return nil
}

// gmapsRequest reads the step's inputs. san_remote checks them again; these
// say it in the step's words.
func gmapsRequest(c *call) (*gmapsv1.SearchPlacesRequest, error) {
	req := &gmapsv1.SearchPlacesRequest{}
	switch c.setting("Social Links") {
	case "", "From Websites":
		req.VisitWebsites = true
	case "Website Only":
	default:
		return nil, fmt.Errorf("Social Links %q: give From Websites or Website Only", c.setting("Social Links"))
	}
	if req.Keyword = strings.Join(strings.Fields(c.text("Keyword")), " "); req.Keyword == "" {
		return nil, fmt.Errorf("Keyword %w: give what to search for, such as \"coffee\"", errEmpty)
	}
	switch c.setting("Location") {
	case "", "Coordinates":
		lat, lng, err := gmapsPoint(c.text("Latitude"), c.text("Longitude"))
		if err != nil {
			return nil, err
		}
		req.Latitude, req.Longitude = lat, lng
	case "Plus Code":
		if req.PlusCode = strings.TrimSpace(c.text("Plus Code")); req.PlusCode == "" {
			return nil, fmt.Errorf("Plus Code %w: give a full plus code, such as 6P58RR48+P4", errEmpty)
		}
	default:
		return nil, fmt.Errorf("Location %q: give Coordinates or Plus Code", c.setting("Location"))
	}
	if s := strings.TrimSpace(c.text("Zoom")); s != "" {
		z, err := strconv.ParseFloat(strings.TrimSuffix(s, "z"), 64)
		if err != nil || z < 3 || z > 21 {
			return nil, fmt.Errorf("Zoom %q: give 3 to 21 (15 is a few streets, 12 a city)", s)
		}
		req.Zoom = z
	}
	if s := strings.TrimSpace(c.text("Limit")); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 || n > 500 {
			return nil, fmt.Errorf("Limit %q: give a whole number from 1 to 500, or leave it empty for all", s)
		}
		req.Limit = uint32(n)
	}
	return req, nil
}

// gmapsPoint reads Latitude and Longitude; Latitude may hold both, as Maps
// copies a point: "-6.2, 106.816".
func gmapsPoint(latText, lngText string) (lat, lng float64, err error) {
	latText, lngText = strings.TrimSpace(latText), strings.TrimSpace(lngText)
	if a, b, ok := strings.Cut(latText, ","); ok && lngText == "" {
		latText, lngText = strings.TrimSpace(a), strings.TrimSpace(b)
	}
	if latText == "" || lngText == "" {
		return 0, 0, errors.New("Latitude and Longitude: give the point to search around, such as -6.2 and 106.816 (or Location Plus Code)")
	}
	lat, err = strconv.ParseFloat(latText, 64)
	if err != nil || lat < -90 || lat > 90 {
		return 0, 0, fmt.Errorf("Latitude %q: give a number from -90 to 90, such as -6.2", latText)
	}
	lng, err = strconv.ParseFloat(lngText, 64)
	if err != nil || lng < -180 || lng > 180 {
		return 0, 0, fmt.Errorf("Longitude %q: give a number from -180 to 180, such as 106.816", lngText)
	}
	return lat, lng, nil
}
