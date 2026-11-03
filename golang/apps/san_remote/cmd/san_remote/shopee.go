package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/urfave/cli/v3"

	"github.com/wargasipil/san_remote/internal/page"
	"github.com/wargasipil/san_remote/internal/shopee"
)

// shopeeService runs the Shopee steps in this process, through the server's
// RemoteService, and prints each step to stderr.
func shopeeService(cmd *cli.Command) *shopee.Service {
	return &shopee.Service{
		B:        page.Remote{C: client(cmd)},
		Progress: func(step string) { fmt.Fprintln(os.Stderr, "== "+step) },
	}
}

// productID reads the command's one argument: a product id or link.
func productID(cmd *cli.Command) (int64, error) {
	if cmd.Args().Len() != 1 {
		return 0, fmt.Errorf("%s needs one product id (or its Seller Centre link)", cmd.Name)
	}
	return shopee.ParseID(cmd.Args().First())
}

func shopeeCommand() *cli.Command {
	return &cli.Command{
		Name:  "shopee",
		Usage: "add, list, show, archive and delete products in Shopee Seller Centre, replace their photos and video, and bump them (Naikkan Produk)",
		Description: "Seller Centre's interface must be in Indonesian. upload, media, delete, publish, archive and boost bring\n" +
			"Chrome to the front with a tab of their own; leave it alone until the command is done. status,\n" +
			"products and boosted read in a background tab. Steps are printed to stderr as \"== ...\".",
		Commands: []*cli.Command{
			{
				Name:  "status",
				Usage: "say whether the browser is signed in to Seller Centre, and as which shop",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					st, err := shopeeService(cmd).Status(ctx, cmd.String("browser"))
					if err != nil {
						return clientError(cmd, err)
					}
					if !st.SignedIn {
						fmt.Fprintln(cmd.Root().Writer, "not signed in to Shopee Seller Centre")
						return shopee.ErrNotSignedIn
					}
					_, err = fmt.Fprintf(cmd.Root().Writer, "signed in as %s (shop %s, id %d)\n", st.Username, st.Shop, st.ShopID)
					return err
				},
			},
			{
				Name:  "products",
				Usage: "list the shop's products (Produk Saya): ID, STATUS, PRICE, STOCK, SOLD, MODELS, NAME",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "filter", Value: "all", Usage: "which tab: all, live, archived, review, violation or draft"},
					&cli.IntFlag{Name: "limit", Value: 50, Usage: "at most this many"},
					&cli.StringFlag{Name: "search", Usage: "only those whose name, SKU, variation code or id holds this"},
					&cli.BoolFlag{Name: "models", Usage: "also list each product's models (options, price, stock, SKU)"},
					&cli.BoolFlag{Name: "media", Usage: "also list each product's photos and video (read from each product's page: one request a product)"},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					list, total, err := shopeeService(cmd).Products(ctx, cmd.String("browser"), cmd.String("filter"), int(cmd.Int("limit")), cmd.String("search"), cmd.Bool("media"))
					if err != nil {
						return clientError(cmd, err)
					}
					w := cmd.Root().Writer
					tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
					fmt.Fprintln(tw, "ID\tSTATUS\tPRICE\tSTOCK\tSOLD\tMODELS\tNAME")
					for _, p := range list {
						fmt.Fprintf(tw, "%d\t%s\t%s\t%d\t%d\t%d\t%s\n", p.ID, p.Status, priceRange(p), p.Stock, p.Sold, len(p.Models), clip(p.Name, 70))
						if cmd.Bool("models") {
							for _, m := range p.Models {
								fmt.Fprintf(tw, "  %d\t\t%s\t%d\t\t%s\t%s\n", m.ID, shopee.Rupiah(m.Price), m.Stock, strings.Join(m.Options, ", "), m.SKU)
							}
						}
						if p.MediaRead {
							for _, line := range mediaLines(&p) {
								fmt.Fprintln(tw, strings.Repeat("\t", 6)+line)
							}
						}
					}
					if err := tw.Flush(); err != nil {
						return err
					}
					_, err = fmt.Fprintf(w, "%d of %d\n", len(list), total)
					return err
				},
			},
			{
				Name:      "upload",
				Usage:     "add a product through Tambah Produk Baru, with its variations; saved archived unless --publish live",
				ArgsUsage: "PRODUCT.md",
				Description: "PRODUCT.md: frontmatter name:, category: (\"A > B\"), photos: (relative to the file; the first\n" +
					"is the cover), video: (an MP4 of 10-60 s and up to 30 MB, relative to the file), attributes:, price:,\n" +
					"stock:, sku:, variations: (name and options, up to 2), models: (options, price, stock, sku), weight:\n" +
					"(grams), dimensions: (\"20x10x5\" cm), condition: (new or used), couriers:, dangerous:, publish:\n" +
					"(archived or live); then the description. See docs/readme.md.\n" +
					"The whole form is read back before Simpan is pressed. --rehearse does every step but Simpan.",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "publish", Usage: "archived (Simpan & Arsipkan) or live (Simpan & Tampilkan); overrides the file's"},
					&cli.StringFlag{Name: "video", Usage: "the product's video (Video Produk), an MP4 of 10 to 60 seconds and up to 30 MB; overrides the file's"},
					&cli.BoolFlag{Name: "rehearse", Usage: "fill the whole form and check it, but do not save it"},
					&cli.BoolFlag{Name: "dry-run", Usage: "show the product as it will be made; open nothing"},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					if cmd.Args().Len() != 1 {
						return errors.New("upload needs a PRODUCT.md")
					}
					n, err := shopee.ParseFile(cmd.Args().First())
					if err != nil {
						return err
					}
					if v := cmd.String("publish"); v != "" {
						if n.Live, err = shopee.ParsePublish(v); err != nil {
							return err
						}
					}
					if v := cmd.String("video"); v != "" {
						n.Video = v
					}
					n.Browser, n.Rehearse = cmd.String("browser"), cmd.Bool("rehearse")
					w := cmd.Root().Writer
					if cmd.Bool("dry-run") {
						models, err := n.Check()
						if err != nil {
							return err
						}
						_, err = fmt.Fprintln(w, strings.Join(n.Summary(models), "\n"))
						return err
					}
					c, err := shopeeService(cmd).Create(ctx, *n)
					if c != nil {
						printProduct(w, &c.Product)
					}
					if err != nil {
						return clientError(cmd, err)
					}
					if n.Rehearse {
						_, err = fmt.Fprintln(w, "rehearsed: the form read\n  "+strings.Join(c.Form, "\n  ")+"\nnothing was saved")
					}
					return err
				},
			},
			{
				Name:      "media",
				Usage:     "replace the photos and/or the video of a product on its page in Seller Centre, or take its video away; saved with Update, which keeps it live or archived as it is",
				ArgsUsage: "ID",
				Description: "--photo FILE (repeated, in order) is the whole new gallery: 1 to 9 square jpg/png of up to 2 MB, the first\n" +
					"is the cover; the old photos are taken away. --video FILE replaces the video (an MP4 of 10-60 s, up to\n" +
					"30 MB); --remove-video takes it away. What is not given is kept. The page is read back before Update\n" +
					"is pressed, and the product's photos and video are then read from Seller Centre. --rehearse does every\n" +
					"step but Update; --dry-run checks the files and opens nothing.",
				Flags: []cli.Flag{
					&cli.StringSliceFlag{Name: "photo", Usage: "a photo of the new gallery, in order (repeat the flag)"},
					&cli.StringFlag{Name: "video", Usage: "a new video, in place of the product's"},
					&cli.BoolFlag{Name: "remove-video", Usage: "take the product's video away"},
					&cli.BoolFlag{Name: "rehearse", Usage: "change the page and check it, but do not press Update"},
					&cli.BoolFlag{Name: "dry-run", Usage: "check the files and show the change; open nothing"},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					id, err := productID(cmd)
					if err != nil {
						return err
					}
					m := shopee.Media{
						Browser: cmd.String("browser"), ID: id, Photos: cmd.StringSlice("photo"), Video: cmd.String("video"),
						RemoveVideo: cmd.Bool("remove-video"), Rehearse: cmd.Bool("rehearse"),
					}
					if err := m.Check(); err != nil {
						return err
					}
					w := cmd.Root().Writer
					if cmd.Bool("dry-run") {
						_, err := fmt.Fprintln(w, strings.Join(m.Summary(), "\n"))
						return err
					}
					u, err := shopeeService(cmd).UpdateMedia(ctx, m)
					if err != nil {
						return clientError(cmd, err)
					}
					fmt.Fprintf(w, "%d %s (%s) %s\n", u.Before.ID, u.Before.Name, u.Before.Status, u.Before.URL())
					fmt.Fprintln(w, "before:\n  "+strings.Join(mediaLines(&u.Before), "\n  "))
					if m.Rehearse {
						_, err = fmt.Fprintln(w, "rehearsed: the page read\n  "+strings.Join(u.Form, "\n  ")+"\nnothing was saved")
						return err
					}
					_, err = fmt.Fprintf(w, "after (%s):\n  %s\n", u.Product.Status, strings.Join(mediaLines(&u.Product), "\n  "))
					return err
				},
			},
			{
				Name:      "delete",
				Usage:     "delete a product (Lainnya, Hapus); a live one is archived first. It cannot be undone",
				ArgsUsage: "ID",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					id, err := productID(cmd)
					if err != nil {
						return err
					}
					p, err := shopeeService(cmd).Delete(ctx, cmd.String("browser"), id)
					if err != nil {
						return clientError(cmd, err)
					}
					_, err = fmt.Fprintf(cmd.Root().Writer, "deleted %d %s (%s)\n", p.ID, p.Name, p.Status)
					return err
				},
			},
			{
				Name:      "publish",
				Usage:     "show an archived product to buyers (Lainnya, Tampilkan)",
				ArgsUsage: "ID",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					id, err := productID(cmd)
					if err != nil {
						return err
					}
					p, err := shopeeService(cmd).Publish(ctx, cmd.String("browser"), id)
					if err != nil {
						return clientError(cmd, err)
					}
					_, err = fmt.Fprintf(cmd.Root().Writer, "%d %s: %s\n", p.ID, p.Name, p.Status)
					return err
				},
			},
			{
				Name:      "archive",
				Usage:     "hide a live product from buyers (Lainnya, Arsipkan)",
				ArgsUsage: "ID",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					id, err := productID(cmd)
					if err != nil {
						return err
					}
					p, err := shopeeService(cmd).Archive(ctx, cmd.String("browser"), id)
					if err != nil {
						return clientError(cmd, err)
					}
					_, err = fmt.Fprintf(cmd.Root().Writer, "%d %s: %s\n", p.ID, p.Name, p.Status)
					return err
				},
			},
			{
				Name:      "boost",
				Usage:     "bump a live product to the top (Naikkan Produk): 5 slots, 4 hours each",
				ArgsUsage: "ID",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					id, err := productID(cmd)
					if err != nil {
						return err
					}
					b, all, err := shopeeService(cmd).Boost(ctx, cmd.String("browser"), id)
					if err != nil {
						return clientError(cmd, err)
					}
					_, err = fmt.Fprintf(cmd.Root().Writer, "bumped %d %s until %s; %d of %d slots in use\n",
						b.ID, b.Name, b.EndsAt.Local().Format("Mon 2 Jan 2006 15:04:05 MST"), len(all.Boosts), all.Total)
					return err
				},
			},
			{
				Name:  "boosted",
				Usage: "list the bumped products (Naikkan Produk): ID, UNTIL, LEFT, NAME",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					all, err := shopeeService(cmd).Boosted(ctx, cmd.String("browser"))
					if err != nil {
						return clientError(cmd, err)
					}
					w := cmd.Root().Writer
					tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
					fmt.Fprintln(tw, "ID\tUNTIL\tLEFT\tNAME")
					for _, b := range all.Boosts {
						name := b.Name
						if name == "" {
							name = "(no longer in the shop)"
						}
						fmt.Fprintf(tw, "%d\t%s\t%s\t%s\n", b.ID, b.EndsAt.Local().Format("Mon 2 Jan 15:04"), (time.Duration(b.SecondsLeft) * time.Second).Round(time.Minute), clip(name, 70))
					}
					if err := tw.Flush(); err != nil {
						return err
					}
					_, err = fmt.Fprintf(w, "%d of %d slots in use; a slot holds its product for %s\n", len(all.Boosts), all.Total, all.Cooldown)
					return err
				},
			},
		},
	}
}

func priceRange(p shopee.Product) string {
	if p.PriceMin == p.PriceMax {
		return shopee.Rupiah(p.PriceMin)
	}
	return shopee.Rupiah(p.PriceMin) + "-" + shopee.Rupiah(p.PriceMax)
}

// printProduct writes a product and its models, and its photos and video
// when they were read.
func printProduct(w interface{ Write([]byte) (int, error) }, p *shopee.Product) {
	if p.ID != 0 {
		fmt.Fprintf(w, "%d %s (%s) %s\n", p.ID, p.Name, p.Status, p.URL())
	} else {
		fmt.Fprintf(w, "%s (%s)\n", p.Name, p.Status)
	}
	for _, m := range p.Models {
		opts := strings.Join(m.Options, ", ")
		if opts == "" {
			opts = "price"
		}
		line := fmt.Sprintf("  %s: %s, stock %d", opts, shopee.Rupiah(m.Price), m.Stock)
		if m.SKU != "" {
			line += ", SKU " + m.SKU
		}
		if m.ID != 0 {
			line += fmt.Sprintf(" (model %d)", m.ID)
		}
		fmt.Fprintln(w, line)
	}
	if p.MediaRead {
		for _, line := range mediaLines(p) {
			fmt.Fprintln(w, "  "+line)
		}
	}
}

// mediaLines is a product's photos and video, a line each: "photo 1:
// <link>", "video: <id>, 15s" or "video: none".
func mediaLines(p *shopee.Product) []string {
	var out []string
	for i, l := range p.Photos {
		out = append(out, fmt.Sprintf("photo %d: %s", i+1, l))
	}
	if v := p.Video; v != nil {
		out = append(out, fmt.Sprintf("video: %s, %s", v.ID, v.Duration))
	} else {
		out = append(out, "video: none")
	}
	return out
}
