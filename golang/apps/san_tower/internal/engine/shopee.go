package engine

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect"
	shopeev1 "github.com/wargasipil/san_remote/gen/san/remote/shopee/v1"
	"google.golang.org/protobuf/types/known/structpb"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
)

// The Shopee steps (the user, 2026-11-03: "Upload product shopee, delete
// produk shopee and up product shopee", then "Add node to get list product
// too"): the shop's products in Shopee Seller Centre, through `san_remote
// api` at a San Remote connection (his choice over the Shopee Open Platform
// API), like the Social steps. "Up product" is Seller Centre's Naikkan
// Produk. They are in their own category, Shop.

var shopeeKinds = []*kind{
	{
		StepKind: &v1.StepKind{
			Kind: "shopee-products", Title: "Shopee Products", Category: "Shop",
			Group:       "Shopee",
			Description: "Lists the shop's products in Shopee Seller Centre (Produk Saya) through San Remote, newest change first: Filter picks the tab (All, Live, Archived, Review, Violation, Draft), Search keeps those whose name, SKU or id holds it, Limit is at most how many (default 50, up to 500). Products is the list, each with its models (options, price, stock, SKU); Total is how many the tab holds, Count how many came.",
			Outputs:     []*v1.Socket{sock("Done", Event), sock("Products", Object), sock("Total", Text), sock("Count", Text)},
			Settings:    []*v1.Setting{connectionSetting(), {Name: "Filter", Options: []string{"All", "Live", "Archived", "Review", "Violation", "Draft"}}},
			Inputs:      []*v1.Socket{sock("Event", Event), field("Search"), field("Limit")},
		},
		run: shopeeProducts,
	},
	{
		StepKind: &v1.StepKind{
			Kind: "shopee-upload", Title: "Shopee Upload Product", Category: "Shop",
			Group:       "Shopee",
			Description: "Adds a product to the shop through San Remote, in Seller Centre's Tambah Produk Baru: a product file, markdown with frontmatter (name, category, photos, attributes, price, stock, weight, up to 2 variations with a price, stock and SKU per combination; san_remote's readme), as a File (photos relative to it) or the Product text (photos by absolute paths). The form is read back before it is saved. Publish: As Written follows the file (archived when it says nothing), Archived saves it hidden (Simpan & Arsipkan), Live shows it to buyers (Simpan & Tampilkan). Rehearse Yes fills and checks the form but saves nothing. Brings Chrome to the front for a minute or two. ID and Link are the new product's.",
			Outputs:     []*v1.Socket{sock("Done", Event), sock("Product", Object), sock("ID", Text), sock("Link", Text)},
			Settings: []*v1.Setting{
				connectionSetting(),
				{Name: "Source", Options: []string{"File", "Text"}},
				{Name: "Publish", Options: []string{"As Written", "Archived", "Live"}},
				{Name: "Rehearse", Options: []string{"No", "Yes"}},
			},
			Inputs: []*v1.Socket{sock("Event", Event), only("Source=File", field("File")), only("Source=Text", field("Product"))},
		},
		run: shopeeUpload,
	},
	{
		StepKind: &v1.StepKind{
			Kind: "shopee-delete", Title: "Shopee Delete Product", Category: "Shop",
			Group:       "Shopee",
			Description: "Deletes a product from the shop through San Remote (Lainnya, Hapus): Product is its id or its Seller Centre or shop link (a Shopee Upload Product's ID or Link). A live product is archived first, since Seller Centre deletes only archived ones; a product the shop does not have fails. It cannot be undone. Product out is the product as it was.",
			Outputs:     []*v1.Socket{sock("Done", Event), sock("Product", Object)},
			Settings:    []*v1.Setting{connectionSetting()},
			Inputs:      []*v1.Socket{sock("Event", Event), field("Product")},
		},
		run: func(c *call) error {
			r, id, err := c.shopeeProduct()
			if err != nil {
				return err
			}
			res, err := r.shopee.DeleteProduct(c.ctx, connect.NewRequest(&shopeev1.DeleteProductRequest{Id: id}))
			if err != nil {
				return remoteErr(err)
			}
			c.outputs["Product"] = toValue(res.Msg.GetProduct())
			c.fire("Done")
			return nil
		},
	},
	{
		StepKind: &v1.StepKind{
			Kind: "shopee-boost", Title: "Shopee Naikkan Produk", Category: "Shop",
			Group:       "Shopee",
			Description: "Bumps a live product to the top through San Remote (Lainnya, Naikkan Produk): Product is its id or link. Seller Centre has 5 slots, each holding its product for 4 hours, and a product can be bumped again only after its 4 hours. Boosted fires with Until, when its slot is free again (an RFC 3339 datetime, as the datetime steps give one), and Slots, the slots in use. A product that is not live, one bumped less than 4 hours ago, or all 5 slots in use fire Refused with the Reason instead, before anything is pressed; Done either way.",
			Outputs: []*v1.Socket{sock("Done", Event), sock("Boosted", Event), sock("Refused", Event),
				sock("Boost", Object), sock("Until", Text), sock("Slots", Text), sock("Reason", Text)},
			Settings: []*v1.Setting{connectionSetting()},
			Inputs:   []*v1.Socket{sock("Event", Event), field("Product")},
		},
		run: shopeeBoost,
	},
}

// shopeeFilters are Shopee Products' Filter options as san_remote names the
// tabs.
var shopeeFilters = map[string]string{
	"All": "all", "Live": "live", "Archived": "archived", "Review": "review", "Violation": "violation", "Draft": "draft",
}

func shopeeProducts(c *call) error {
	filter := "all"
	if f := c.setting("Filter"); f != "" {
		var ok bool
		if filter, ok = shopeeFilters[f]; !ok {
			return fmt.Errorf("Filter %q: give All, Live, Archived, Review, Violation or Draft", f)
		}
	}
	limit := 50
	if s := strings.TrimSpace(c.text("Limit")); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 || n > 500 {
			return fmt.Errorf("Limit %q: give a whole number from 1 to 500", s)
		}
		limit = n
	}
	r, err := c.sanRemote()
	if err != nil {
		return err
	}
	res, err := r.shopee.ListProducts(c.ctx, connect.NewRequest(&shopeev1.ListProductsRequest{
		Filter: filter, Limit: uint32(limit), Search: strings.TrimSpace(c.text("Search")),
	}))
	if err != nil {
		return remoteErr(err)
	}
	list := &structpb.ListValue{}
	for _, p := range res.Msg.GetProducts() {
		list.Values = append(list.Values, toValue(p))
	}
	c.outputs["Products"] = structpb.NewListValue(list)
	c.outputs["Total"] = stringValue(strconv.Itoa(int(res.Msg.GetTotal())))
	c.outputs["Count"] = stringValue(strconv.Itoa(len(list.Values)))
	c.fire("Done")
	return nil
}

func shopeeUpload(c *call) error {
	req := &shopeev1.CreateProductRequest{Rehearse: c.setting("Rehearse") == "Yes"}
	switch c.setting("Publish") {
	case "", "As Written":
	case "Archived":
		req.Publish = "archived"
	case "Live":
		req.Publish = "live"
	default:
		return fmt.Errorf("Publish %q: give As Written, Archived or Live", c.setting("Publish"))
	}
	if c.setting("Source") == "Text" {
		req.Markdown = c.text("Product")
		if strings.TrimSpace(req.Markdown) == "" {
			return fmt.Errorf("Product %w: give the product file's text, markdown with frontmatter", errEmpty)
		}
	} else {
		file, err := c.absolute("File", "the product's markdown file")
		if err != nil {
			return err
		}
		req.File = file
	}
	r, err := c.sanRemote()
	if err != nil {
		return err
	}
	res, err := r.shopee.CreateProduct(c.ctx, connect.NewRequest(req))
	if err != nil {
		return remoteErr(err)
	}
	p := res.Msg.GetProduct()
	c.outputs["Product"] = toValue(res.Msg)
	id := ""
	if p.GetId() != 0 {
		id = strconv.FormatInt(p.GetId(), 10)
	}
	c.outputs["ID"] = stringValue(id)
	c.outputs["Link"] = stringValue(p.GetUrl())
	c.fire("Done")
	return nil
}

func shopeeBoost(c *call) error {
	r, id, err := c.shopeeProduct()
	if err != nil {
		return err
	}
	res, err := r.shopee.BoostProduct(c.ctx, connect.NewRequest(&shopeev1.BoostProductRequest{Id: id}))
	var ce *connect.Error
	if errors.As(err, &ce) && ce.Code() == connect.CodeFailedPrecondition {
		// Refused before anything was pressed: not live, on its 4 hours, or
		// all slots in use. A workflow that bumps on a schedule goes on.
		c.outputs["Boost"] = structpb.NewNullValue()
		c.outputs["Until"] = stringValue("")
		c.outputs["Slots"] = stringValue("")
		c.outputs["Reason"] = stringValue(ce.Message())
		c.fire("Refused")
		c.fire("Done")
		return nil
	}
	if err != nil {
		return remoteErr(err)
	}
	b := res.Msg.GetBoost()
	until := ""
	if b.GetEndsAt() != nil {
		// As the datetime steps give a time: RFC 3339, local.
		until = b.GetEndsAt().AsTime().Local().Format(time.RFC3339)
	}
	c.outputs["Boost"] = toValue(res.Msg)
	c.outputs["Until"] = stringValue(until)
	c.outputs["Slots"] = stringValue(fmt.Sprintf("%d of %d", res.Msg.GetSlotsUsed(), res.Msg.GetSlotsTotal()))
	c.outputs["Reason"] = stringValue("")
	c.fire("Boosted")
	c.fire("Done")
	return nil
}

// shopeeProduct reads the Product input of the steps on one product, after
// the connection.
func (c *call) shopeeProduct() (remoteClients, int64, error) {
	id, err := shopeeID(c.text("Product"))
	if err != nil {
		return remoteClients{}, 0, err
	}
	r, err := c.sanRemote()
	return r, id, err
}

// shopeeLink are the links that name a product: Seller Centre's
// (seller.shopee.co.id/portal/product/<id>) and the shop's
// (shopee.co.id/<name>-i.<shop>.<id>, shopee.co.id/product/<shop>/<id>).
var shopeeLink = regexp.MustCompile(`/portal/product/(\d+)|-i\.\d+\.(\d+)|/product/\d+/(\d+)`)

// shopeeID is a product's id from its id or a link to it.
func shopeeID(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("Product %w: give the product's id or link (a Shopee Upload Product's ID)", errEmpty)
	}
	digits := s
	if m := shopeeLink.FindStringSubmatch(s); m != nil {
		digits = m[1] + m[2] + m[3]
	}
	id, err := strconv.ParseInt(digits, 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("Product %q: give the product's id (ID Produk, such as 57614536675) or its link", s)
	}
	return id, nil
}
