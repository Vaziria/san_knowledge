package mcp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/wargasipil/san_remote/internal/shopee"
)

// The shopee_* tools (2026-11-03).

const shopeeInstructions = `
shopee_* work on the user's Shopee shop in Shopee Seller Centre (seller.shopee.co.id, interface in Indonesian) in that browser, as a person does there (no Shopee API): shopee_upload adds a product from a product markdown file (photos, a video, category, attributes, description, up to 2 variations with a price, stock and SKU per combination), saved archived unless publish is live, and reads the whole form back before Simpan; rehearse fills and checks the form without saving. shopee_media replaces the photos (the whole gallery) and/or the video of a product the shop has, or takes its video away, and saves with Update, which keeps the product live or archived as it is; rehearse does all but Update. shopee_products lists the products (filter all, live, archived, review, violation, draft; with_media adds each one's photos and video), shopee_boosted the bumped ones; shopee_status reads the shop. shopee_publish shows an archived product to buyers and shopee_archive hides a live one. shopee_boost bumps a live product (Naikkan Produk: 5 slots, each held 4 hours; a product on its cooldown or a full set of slots is refused). shopee_delete deletes a product for good (a live one is archived first): ask the user first. Make live, bump, change or delete only the products the user named; upload only what the user asked for. Each change brings Chrome to the front with a tab of its own.`

func shopeeToolDefs() []any {
	productProp := str("The product's id (ID Produk, as shopee_products lists it) or its Seller Centre link")
	return []any{
		tool("shopee_status", "Shopee: which shop is signed in",
			"Whether the browser is signed in to Shopee Seller Centre, and as which shop (name, username, id).",
			schema(map[string]any{"browser": browserProp}), true, false),
		tool("shopee_products", "Shopee: the shop's products",
			"The shop's products from Produk Saya: id, status (live, archived), price range, stock, sold, and each model (options, price, stock, SKU); with with_media, each one's photos and video too.",
			schema(map[string]any{
				"filter":     enum("Which tab (default all)", "all", "live", "archived", "review", "violation", "draft"),
				"search":     str("Only products whose name, SKU, variation code or id holds this"),
				"limit":      intg("At most this many (default 50)"),
				"with_media": flag("Also read each product's photos and video (one request a product)"),
				"browser":    browserProp,
			}), true, false),
		tool("shopee_upload", "Shopee: add a product",
			"Add a product through Tambah Produk Baru from a product markdown file: frontmatter name, category (\"A > B\"), photos (square jpg/png up to 2 MB, relative to the file; the first is the cover), video (an MP4 of 10 to 60 seconds and up to 30 MB, relative to the file; optional), attributes, price, stock, sku, variations (up to 2: name and options), models (options, price, stock, sku per combination), weight (grams), dimensions, condition, couriers, dangerous, publish; the description after it. Saved archived (Simpan & Arsipkan) unless publish is live. The form is read back before Simpan. rehearse does every step but Simpan. Brings Chrome to the front for a minute or two.",
			schema(map[string]any{
				"file":     str("The product's markdown file"),
				"publish":  enum("archived (the default) or live; overrides the file's", "archived", "live"),
				"rehearse": flag("Fill and check the whole form, but do not save it"),
				"browser":  browserProp,
			}, "file"), false, false),
		tool("shopee_media", "Shopee: replace a product's photos or video",
			"Replace the photos and/or the video of a product the shop has, on its page in Seller Centre: photos is the whole new gallery in order (1 to 9 square jpg/png up to 2 MB; the first is the cover), video a new MP4 (10 to 60 seconds, up to 30 MB) in place of the product's, remove_video takes its video away. Only what is given changes; the page is read back, then saved with Update, which keeps the product live or archived as it is. rehearse does every step but Update. Brings Chrome to the front for a minute or two.",
			schema(map[string]any{
				"product":      productProp,
				"photos":       strs("The new photos, absolute paths, in order (the first is the cover); omit to keep the product's"),
				"video":        str("A new video, an absolute path to an MP4; omit to keep the product's"),
				"remove_video": flag("Take the product's video away"),
				"rehearse":     flag("Do every step but Update, and save nothing"),
				"browser":      browserProp,
			}, "product"), false, false),
		tool("shopee_publish", "Shopee: show a product to buyers",
			"Show an archived product to buyers (Lainnya, Tampilkan). Shopee may review it first. Brings Chrome to the front.",
			schema(map[string]any{"product": productProp, "browser": browserProp}, "product"), false, false),
		tool("shopee_archive", "Shopee: hide a product from buyers",
			"Archive a live product (Lainnya, Arsipkan, Konfirmasi): buyers no longer see it. Brings Chrome to the front.",
			schema(map[string]any{"product": productProp, "browser": browserProp}, "product"), false, false),
		tool("shopee_boost", "Shopee: bump a product (Naikkan Produk)",
			"Bump a live product to the top (Lainnya, Naikkan Produk). Seller Centre has 5 slots; each holds its product for 4 hours, and the product cannot be bumped again until then. Refused when the product is not live, is on its cooldown, or all slots are in use. Returns when the slot is free again. Brings Chrome to the front.",
			schema(map[string]any{"product": productProp, "browser": browserProp}, "product"), false, false),
		tool("shopee_boosted", "Shopee: bumped products",
			"Naikkan Produk's slots in use: each bumped product and when its slot is free again, and how many of the 5 are in use.",
			schema(map[string]any{"browser": browserProp}), true, false),
		tool("shopee_delete", "Shopee: delete a product",
			"Delete a product for good (Lainnya, Hapus, then Hapus when Seller Centre asks about it by name); a live one is archived first. It cannot be undone: ask the user first.",
			schema(map[string]any{"product": productProp, "browser": browserProp}, "product"), false, true),
	}
}

func (s *Server) shopee(ctx context.Context, name string, a args) (string, error) {
	sh := s.Shopee
	id := func() (int64, error) { return shopee.ParseID(a.Product) }
	switch name {
	case "shopee_status":
		st, err := sh.Status(ctx, a.Browser)
		if err != nil {
			return "", err
		}
		if !st.SignedIn {
			return "The browser is not signed in to Shopee Seller Centre.", nil
		}
		return fmt.Sprintf("Signed in to Shopee Seller Centre as %s, shop %q (id %d).", st.Username, st.Shop, st.ShopID), nil
	case "shopee_products":
		list, total, err := sh.Products(ctx, a.Browser, a.Filter, int(a.Limit), a.Search, a.WithMedia)
		if err != nil {
			return "", err
		}
		if len(list) == 0 {
			return "No products.", nil
		}
		var b strings.Builder
		for _, p := range list {
			fmt.Fprintf(&b, "%d | %s | %s-%s | stock %d | sold %d | %s\n", p.ID, p.Status, shopee.Rupiah(p.PriceMin), shopee.Rupiah(p.PriceMax), p.Stock, p.Sold, p.Name)
			for _, m := range p.Models {
				if len(m.Options) > 0 {
					fmt.Fprintf(&b, "    %s | %s | stock %d | SKU %s\n", strings.Join(m.Options, ", "), shopee.Rupiah(m.Price), m.Stock, m.SKU)
				}
			}
			if p.MediaRead {
				b.WriteString(shopeeMedia(&p))
			}
		}
		fmt.Fprintf(&b, "%d of %d.", len(list), total)
		return b.String(), nil
	case "shopee_media":
		pid, err := id()
		if err != nil {
			return "", err
		}
		m := shopee.Media{Browser: a.Browser, ID: pid, Photos: a.Photos, Video: a.Video, RemoveVideo: a.RemoveVideo, Rehearse: a.Rehearse}
		u, err := sh.UpdateMedia(ctx, m)
		if err != nil {
			return "", err
		}
		var b strings.Builder
		if m.Rehearse {
			fmt.Fprintf(&b, "Rehearsed on product %d %q: the page was changed and checked, and not saved. It read:\n%s\n", pid, u.Before.Name, strings.Join(u.Form, "\n"))
			b.WriteString("The product is still:\n" + shopeeMedia(&u.Before))
			return b.String(), nil
		}
		fmt.Fprintf(&b, "Updated product %d %q (%s). Before:\n%sNow:\n%s", pid, u.Product.Name, u.Product.Status, shopeeMedia(&u.Before), shopeeMedia(&u.Product))
		return b.String(), nil
	case "shopee_upload":
		if a.File == "" {
			return "", fmt.Errorf("file is required")
		}
		n, err := shopee.ParseFile(a.File)
		if err != nil {
			return "", err
		}
		if a.Publish != "" {
			if n.Live, err = shopee.ParsePublish(a.Publish); err != nil {
				return "", err
			}
		}
		n.Browser, n.Rehearse = a.Browser, a.Rehearse
		c, err := sh.Create(ctx, *n)
		if err != nil {
			if c != nil && c.Product.ID != 0 {
				return "", fmt.Errorf("%w (product %d was made: %s)", err, c.Product.ID, c.Product.URL())
			}
			return "", err
		}
		var b strings.Builder
		if n.Rehearse {
			b.WriteString("Rehearsed: the form was filled and checked, and not saved. It read:\n")
		} else {
			fmt.Fprintf(&b, "Made product %d (%s): %s\n", c.Product.ID, c.Product.Status, c.Product.URL())
			for _, m := range c.Product.Models {
				fmt.Fprintf(&b, "    %s | %s | stock %d | SKU %s | model %d\n", strings.Join(m.Options, ", "), shopee.Rupiah(m.Price), m.Stock, m.SKU, m.ID)
			}
			if c.Product.MediaRead {
				b.WriteString(shopeeMedia(&c.Product))
			}
			b.WriteString("The form read:\n")
		}
		b.WriteString(strings.Join(c.Form, "\n"))
		return b.String(), nil
	case "shopee_publish", "shopee_archive", "shopee_delete":
		pid, err := id()
		if err != nil {
			return "", err
		}
		var p *shopee.Product
		switch name {
		case "shopee_publish":
			p, err = sh.Publish(ctx, a.Browser, pid)
		case "shopee_archive":
			p, err = sh.Archive(ctx, a.Browser, pid)
		default:
			p, err = sh.Delete(ctx, a.Browser, pid)
		}
		if err != nil {
			return "", err
		}
		if name == "shopee_delete" {
			return fmt.Sprintf("Deleted product %d %q (it was %s).", p.ID, p.Name, p.Status), nil
		}
		return fmt.Sprintf("Product %d %q is %s.", p.ID, p.Name, p.Status), nil
	case "shopee_boost":
		pid, err := id()
		if err != nil {
			return "", err
		}
		bst, all, err := sh.Boost(ctx, a.Browser, pid)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Bumped product %d %q until %s; %d of %d slots in use.", bst.ID, bst.Name, bst.EndsAt.Local().Format(time.RFC3339), len(all.Boosts), all.Total), nil
	case "shopee_boosted":
		all, err := sh.Boosted(ctx, a.Browser)
		if err != nil {
			return "", err
		}
		var b strings.Builder
		for _, x := range all.Boosts {
			n := x.Name
			if n == "" {
				n = "(no longer in the shop)"
			}
			fmt.Fprintf(&b, "%d | until %s | %s\n", x.ID, x.EndsAt.Local().Format(time.RFC3339), n)
		}
		fmt.Fprintf(&b, "%d of %d slots in use; a slot holds its product for %s.", len(all.Boosts), all.Total, all.Cooldown)
		return b.String(), nil
	}
	return "", fmt.Errorf("unknown tool %q", name)
}

// shopeeMedia is a product's photos and video, a line each.
func shopeeMedia(p *shopee.Product) string {
	var b strings.Builder
	for i, l := range p.Photos {
		fmt.Fprintf(&b, "    photo %d: %s\n", i+1, l)
	}
	if v := p.Video; v != nil {
		fmt.Fprintf(&b, "    video: %s, %s\n", v.ID, v.Duration)
	} else {
		b.WriteString("    video: none\n")
	}
	return b.String()
}
