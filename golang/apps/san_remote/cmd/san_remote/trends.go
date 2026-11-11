package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/urfave/cli/v3"

	"github.com/wargasipil/san_remote/internal/page"
	"github.com/wargasipil/san_remote/internal/trends"
)

// trendsService runs the Google Trends steps in this process, through the
// server's RemoteService, and prints each step to stderr.
func trendsService(cmd *cli.Command) *trends.Service {
	return &trends.Service{
		B:        page.Remote{C: client(cmd)},
		Progress: func(step string) { fmt.Fprintln(os.Stderr, "== "+step) },
	}
}

// trendsTypes are --type's words for the page's search types.
var trendsTypes = map[string]string{"web": "", "images": "images", "news": "news", "shopping": "froogle", "froogle": "froogle", "youtube": "youtube"}

func printJSON(cmd *cli.Command, v any) error {
	e := json.NewEncoder(cmd.Root().Writer)
	e.SetIndent("", "  ")
	return e.Encode(v)
}

func trendsCommand() *cli.Command {
	return &cli.Command{
		Name:  "trends",
		Usage: "read Google Trends' new Explore: a keyword's related queries (Top and Rising), and the page's filter lists",
		Description: "Reads Google Trends' new Explore page (trends.google.com/explore) in a tab, as a person does there: no API. It only reads.\n" +
			"The page loads its related queries only while they are on screen, so a lookup shows its tab for a few seconds, then shows your tab again.\n" +
			"A page that opens in the classic Explore is switched to the new one. Steps are printed to stderr as \"== ...\".",
		Commands: []*cli.Command{
			{
				Name:      "related",
				Usage:     "read a keyword's related queries: Top (by search interest) and Rising (by change)",
				ArgsUsage: "<keyword>",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "geo", Value: "ID", Usage: "the location: a country (ID), a region (ID-JK), or worldwide"},
					&cli.StringFlag{Name: "time", Value: trends.DefaultTime, Usage: "the time range as the page writes it: \"now 1-H\", \"now 4-H\", \"now 1-d\", \"now 7-d\", \"today 1-m\", \"today 3-m\", \"today 12-m\", \"today 5-y\", \"all\", or \"YYYY-MM-DD YYYY-MM-DD\""},
					&cli.IntFlag{Name: "cat", Usage: "the category's id (trends pickers); 0 is All Categories"},
					&cli.StringFlag{Name: "type", Value: "web", Usage: "the search type: web, images, news, shopping or youtube"},
					&cli.BoolFlag{Name: "json", Usage: "print the result as JSON"},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					prop, ok := trendsTypes[strings.ToLower(cmd.String("type"))]
					if !ok {
						return fmt.Errorf("--type %q: give web, images, news, shopping or youtube", cmd.String("type"))
					}
					q := trends.Query{
						Browser: cmd.String("browser"), Keyword: strings.Join(cmd.Args().Slice(), " "), Geo: cmd.String("geo"),
						Time: cmd.String("time"), Category: int(cmd.Int("cat")), Property: prop,
					}
					res, err := trendsService(cmd).Related(ctx, q)
					if err != nil {
						return clientError(cmd, err)
					}
					if cmd.Bool("json") {
						return printJSON(cmd, res)
					}
					w := cmd.Root().Writer
					fmt.Fprintf(w, "%s\n\nTop queries (search interest, change)\n", res.URL)
					for i, it := range res.Top {
						fmt.Fprintf(w, "%3d. %-50s %3d  %s\n", i+1, it.Query, it.SearchInterest, it.ChangeText())
					}
					fmt.Fprintf(w, "\nRising queries (change, search interest)\n")
					for i, it := range res.Rising {
						fmt.Fprintf(w, "%3d. %-50s %9s  %3d\n", i+1, it.Query, it.ChangeText(), it.SearchInterest)
					}
					_, err = fmt.Fprintf(w, "\n%d top, %d rising\n", len(res.Top), len(res.Rising))
					return err
				},
			},
			{
				Name:  "pickers",
				Usage: "read the Explore page's locations, categories, time ranges and search types",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "country", Usage: "also read this country's regions (ID); the tab is shown for it"},
					&cli.BoolFlag{Name: "json", Usage: "print them as JSON"},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					p, err := trendsService(cmd).Pickers(ctx, cmd.String("browser"), cmd.String("country"))
					if err != nil {
						return clientError(cmd, err)
					}
					if cmd.Bool("json") {
						return printJSON(cmd, p)
					}
					w := cmd.Root().Writer
					fmt.Fprintf(w, "Locations (%d)\n", len(p.Geos))
					for _, g := range p.Geos {
						code := g.Code
						if code == "" {
							code = "-"
						}
						more := ""
						if g.HasRegions {
							more = " +regions"
						}
						fmt.Fprintf(w, "  %-4s %s%s\n", code, g.Name, more)
						for _, r := range g.Regions {
							fmt.Fprintf(w, "       %-7s %s\n", r.Code, r.Name)
						}
					}
					fmt.Fprintln(w, "Categories (the top ones; --json gives them all)")
					for _, c := range p.Categories {
						fmt.Fprintf(w, "  %5d %s (%d inside)\n", c.ID, c.Name, countCategories(c.Children))
					}
					fmt.Fprintln(w, "Time ranges")
					for _, o := range p.Times {
						fmt.Fprintf(w, "  %-12s %s\n", o.Value, o.Label)
					}
					fmt.Fprintln(w, "Search types")
					for _, o := range p.Properties {
						v := o.Value
						if v == "" {
							v = "web"
						}
						fmt.Fprintf(w, "  %-12s %s\n", v, o.Label)
					}
					return nil
				},
			},
			{
				Name:  "status",
				Usage: "open the Explore page and say which Explore it shows and whether the browser is signed in to Google",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					st, err := trendsService(cmd).Status(ctx, cmd.String("browser"))
					if err != nil {
						return clientError(cmd, err)
					}
					explore := "the new Explore"
					if st.Switched {
						explore += " (it opened the classic one first: switched)"
					}
					signed := "not signed in to Google"
					if st.SignedIn {
						signed = "signed in to Google"
					}
					_, err = fmt.Fprintf(cmd.Root().Writer, "%s, %s\n%s\n", explore, signed, st.URL)
					return err
				},
			},
		},
	}
}

func countCategories(list []trends.Category) int {
	n := 0
	for _, c := range list {
		n += 1 + countCategories(c.Children)
	}
	return n
}
