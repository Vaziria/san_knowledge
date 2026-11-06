package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/urfave/cli/v3"

	"github.com/wargasipil/san_remote/internal/gmaps"
	"github.com/wargasipil/san_remote/internal/page"
)

// gmapsService runs the Google Maps steps in this process, through the
// server's RemoteService, and prints each step to stderr.
func gmapsService(cmd *cli.Command) *gmaps.Service {
	return &gmaps.Service{
		B:        page.Remote{C: client(cmd)},
		Progress: func(step string) { fmt.Fprintln(os.Stderr, "== "+step) },
	}
}

// parsePoint reads "lat,lng", as Maps' own links and its "copy
// coordinates" give it.
func parsePoint(s string) (lat, lng float64, err error) {
	a, b, ok := strings.Cut(s, ",")
	if ok {
		if lat, err = strconv.ParseFloat(strings.TrimSpace(a), 64); err == nil {
			lng, err = strconv.ParseFloat(strings.TrimSpace(b), 64)
		}
	}
	if !ok || err != nil {
		return 0, 0, fmt.Errorf("--at %q: give latitude,longitude, such as -6.2,106.816", s)
	}
	return lat, lng, nil
}

func gmapsCommand() *cli.Command {
	return &cli.Command{
		Name:  "gmaps",
		Usage: "search Google Maps for places around a point: name, category, address, phone, website, social media, plus code",
		Description: "Reads Google Maps (in English) in a background tab, as a person does there: no Places API. It only reads.\n" +
			"Steps are printed to stderr as \"== ...\".",
		Commands: []*cli.Command{
			{
				Name:      "search",
				Usage:     "search for a keyword around a point, read every place listed (or --limit), and print them",
				ArgsUsage: "<keyword>",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "at", Usage: "the point to search around: latitude,longitude, such as -6.2,106.816"},
					&cli.StringFlag{Name: "plus-code", Usage: "the point as a full plus code, such as 6P58RR48+P4 (instead of --at)"},
					&cli.FloatFlag{Name: "zoom", Value: gmaps.DefaultZoom, Usage: "the map's zoom, 3 to 21 (15 is a few streets, 12 a city)"},
					&cli.IntFlag{Name: "limit", Usage: "at most this many places (0: all the list holds, about 120 at most)"},
					&cli.BoolFlag{Name: "visit-websites", Usage: "fetch each place's website for the social media links on it"},
					&cli.BoolFlag{Name: "json", Usage: "print the places as JSON"},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					q := gmaps.Search{
						Browser: cmd.String("browser"), Keyword: strings.Join(cmd.Args().Slice(), " "), PlusCode: cmd.String("plus-code"),
						Zoom: cmd.Float("zoom"), Limit: int(cmd.Int("limit")), VisitWebsites: cmd.Bool("visit-websites"),
					}
					if at := cmd.String("at"); at != "" {
						var err error
						if q.Lat, q.Lng, err = parsePoint(at); err != nil {
							return err
						}
					}
					res, err := gmapsService(cmd).Search(ctx, q)
					if err != nil {
						return clientError(cmd, err)
					}
					w := cmd.Root().Writer
					if cmd.Bool("json") {
						e := json.NewEncoder(w)
						e.SetIndent("", "  ")
						return e.Encode(res)
					}
					for i, p := range res.Places {
						fmt.Fprintf(w, "%d. %s", i+1, p.Name)
						if p.Category != "" {
							fmt.Fprintf(w, " (%s)", p.Category)
						}
						fmt.Fprintln(w)
						line := func(k, v string) {
							if v != "" {
								fmt.Fprintf(w, "   %-10s %s\n", k, v)
							}
						}
						line("address", p.Address)
						line("phone", p.Phone)
						line("website", p.Website)
						for _, s := range p.Social {
							line(s.Site, s.URL)
						}
						line("site error", p.WebsiteError)
						if p.PlusCode != "" {
							line("plus code", p.PlusCode+"  ("+p.PlusCodeLocal+")")
						}
						if p.Rating > 0 {
							line("rating", fmt.Sprintf("%.1f (%d reviews)", p.Rating, p.Reviews))
						}
						line("maps", p.URL)
					}
					end := "the end of the list"
					if !res.End {
						end = "not the end of the list"
					}
					if res.Limited {
						end += "; Maps showed its limited view (sign in to Google in this browser for the full list)"
					}
					_, err = fmt.Fprintf(w, "%d places (%s)\n", len(res.Places), end)
					return err
				},
			},
		},
	}
}
