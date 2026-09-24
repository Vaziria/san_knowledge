package san_youtube

import (
	"encoding/json"
	"errors"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// The chat endpoint answers with "actions", each an object with one key that
// names what to do: add an item, mark one deleted, replace one, and many more
// that only matter to the web UI. The structs below name only the fields this
// package reads. Anything else, and any action or item it does not know, is
// skipped rather than treated as an error: this is an internal API with no
// compatibility promise, and an unfamiliar renderer must not stop a chat that
// is otherwise working.

type rawAction struct {
	AddChatItem *struct {
		Item rawItem `json:"item"`
	} `json:"addChatItemAction"`
	MarkDeleted *struct {
		TargetItemID        string  `json:"targetItemId"`
		DeletedStateMessage rawText `json:"deletedStateMessage"`
	} `json:"markChatItemAsDeletedAction"`
	MarkAuthorDeleted *struct {
		ExternalChannelID   string  `json:"externalChannelId"`
		DeletedStateMessage rawText `json:"deletedStateMessage"`
	} `json:"markChatItemsByAuthorAsDeletedAction"`
	Remove *struct {
		TargetItemID string `json:"targetItemId"`
	} `json:"removeChatItemAction"`
	RemoveByAuthor *struct {
		ExternalChannelID string `json:"externalChannelId"`
	} `json:"removeChatItemByAuthorAction"`
	Replace *struct {
		TargetItemID    string  `json:"targetItemId"`
		ReplacementItem rawItem `json:"replacementItem"`
	} `json:"replaceChatItemAction"`
}

// rawItem is a chat item. Exactly one field is set for the kinds this package
// models; placeholders, system notices and the rest leave them all nil.
type rawItem struct {
	Text           *rawRenderer `json:"liveChatTextMessageRenderer"`
	Paid           *rawRenderer `json:"liveChatPaidMessageRenderer"`
	Sticker        *rawRenderer `json:"liveChatPaidStickerRenderer"`
	Membership     *rawRenderer `json:"liveChatMembershipItemRenderer"`
	GiftPurchase   *rawRenderer `json:"liveChatSponsorshipsGiftPurchaseAnnouncementRenderer"`
	GiftRedemption *rawRenderer `json:"liveChatSponsorshipsGiftRedemptionAnnouncementRenderer"`
}

// rawRenderer is the union of the item renderers' fields. They share the
// author and id fields and differ in the rest.
type rawRenderer struct {
	ID              string     `json:"id"`
	TimestampUsec   string     `json:"timestampUsec"`
	AuthorName      rawText    `json:"authorName"`
	AuthorChannelID string     `json:"authorExternalChannelId"`
	AuthorPhoto     rawImage   `json:"authorPhoto"`
	AuthorBadges    []rawBadge `json:"authorBadges"`
	Message         rawText    `json:"message"`

	PurchaseAmountText rawText  `json:"purchaseAmountText"`
	Sticker            rawImage `json:"sticker"`

	HeaderPrimaryText rawText `json:"headerPrimaryText"`
	HeaderSubtext     rawText `json:"headerSubtext"`

	// A gift purchase keeps its author and text one level down, in a header
	// renderer.
	Header struct {
		Sponsorships *rawRenderer `json:"liveChatSponsorshipsHeaderRenderer"`
	} `json:"header"`
	PrimaryText rawText `json:"primaryText"`
}

type rawText struct {
	SimpleText string   `json:"simpleText"`
	Runs       []rawRun `json:"runs"`
}

type rawRun struct {
	Text               string    `json:"text"`
	Bold               bool      `json:"bold"`
	Italics            bool      `json:"italics"`
	Emoji              *rawEmoji `json:"emoji"`
	NavigationEndpoint *struct {
		CommandMetadata struct {
			WebCommandMetadata struct {
				URL string `json:"url"`
			} `json:"webCommandMetadata"`
		} `json:"commandMetadata"`
		URLEndpoint *struct {
			URL string `json:"url"`
		} `json:"urlEndpoint"`
	} `json:"navigationEndpoint"`
}

type rawEmoji struct {
	EmojiID       string   `json:"emojiId"`
	Shortcuts     []string `json:"shortcuts"`
	IsCustomEmoji bool     `json:"isCustomEmoji"`
	Image         rawImage `json:"image"`
}

type rawImage struct {
	Thumbnails []struct {
		URL   string `json:"url"`
		Width int    `json:"width"`
	} `json:"thumbnails"`
	Accessibility struct {
		AccessibilityData struct {
			Label string `json:"label"`
		} `json:"accessibilityData"`
	} `json:"accessibility"`
}

type rawBadge struct {
	Renderer struct {
		Icon struct {
			IconType string `json:"iconType"`
		} `json:"icon"`
		CustomThumbnail rawImage `json:"customThumbnail"`
		Tooltip         string   `json:"tooltip"`
	} `json:"liveChatAuthorBadgeRenderer"`
}

// parseActions turns one response's actions into messages, in order. now is
// the time the response arrived, which is the only time deletions and
// replacements have.
func parseActions(actions []json.RawMessage, now time.Time) []Message {
	var out []Message
	for _, raw := range actions {
		var a rawAction
		if decodeLenient(raw, &a) != nil {
			continue
		}
		if m := a.message(raw, now); m != nil {
			out = append(out, m)
		}
	}
	return out
}

func (a *rawAction) message(raw json.RawMessage, now time.Time) Message {
	event := Base{Time: now, Raw: raw}
	switch {
	case a.AddChatItem != nil:
		return a.AddChatItem.Item.message(raw)

	case a.MarkDeleted != nil:
		return &Deletion{Base: event, TargetID: a.MarkDeleted.TargetItemID, Text: a.MarkDeleted.DeletedStateMessage.String()}

	case a.Remove != nil:
		return &Deletion{Base: event, TargetID: a.Remove.TargetItemID}

	case a.MarkAuthorDeleted != nil:
		event.Author.ChannelID = a.MarkAuthorDeleted.ExternalChannelID
		return &AuthorDeletion{Base: event, Text: a.MarkAuthorDeleted.DeletedStateMessage.String()}

	case a.RemoveByAuthor != nil:
		event.Author.ChannelID = a.RemoveByAuthor.ExternalChannelID
		return &AuthorDeletion{Base: event}

	case a.Replace != nil:
		m := a.Replace.ReplacementItem.message(raw)
		if m == nil {
			// Replaced by something this package does not model, such as
			// another placeholder.
			return nil
		}
		return &Replacement{Base: event, TargetID: a.Replace.TargetItemID, Message: m}
	}
	return nil
}

func (it *rawItem) message(raw json.RawMessage) Message {
	switch {
	case it.Text != nil:
		r := it.Text
		text, runs := r.Message.parse()
		return &TextMessage{Base: r.base(raw), Text: text, Runs: runs}

	case it.Paid != nil:
		r := it.Paid
		text, runs := r.Message.parse()
		return &SuperChat{Base: r.base(raw), Amount: ParseAmount(r.PurchaseAmountText.String()), Text: text, Runs: runs}

	case it.Sticker != nil:
		r := it.Sticker
		return &SuperSticker{
			Base:       r.base(raw),
			Amount:     ParseAmount(r.PurchaseAmountText.String()),
			Sticker:    r.Sticker.Accessibility.AccessibilityData.Label,
			StickerURL: r.Sticker.largest(),
		}

	case it.Membership != nil:
		r := it.Membership
		text, runs := r.Message.parse()
		header := r.HeaderPrimaryText.String()
		return &Membership{
			Base:      r.base(raw),
			Milestone: header != "",
			Months:    milestoneMonths(header),
			Header:    header,
			Subtext:   r.HeaderSubtext.String(),
			Text:      text,
			Runs:      runs,
		}

	case it.GiftPurchase != nil:
		r := it.GiftPurchase
		// The author and the announcement live in the header renderer; the
		// outer one has only the id, the time and the channel id.
		b := r.base(raw)
		if h := r.Header.Sponsorships; h != nil {
			hb := h.base(raw)
			hb.ID, hb.Time = b.ID, b.Time
			if hb.Author.ChannelID == "" {
				hb.Author.ChannelID = b.Author.ChannelID
			}
			text := h.PrimaryText.String()
			return &GiftPurchase{Base: hb, Count: firstNumber(text), Text: text}
		}
		text := r.PrimaryText.String()
		return &GiftPurchase{Base: b, Count: firstNumber(text), Text: text}

	case it.GiftRedemption != nil:
		r := it.GiftRedemption
		return &GiftRedemption{Base: r.base(raw), Gifter: r.Message.bold(), Text: r.Message.String()}
	}
	return nil
}

func (r *rawRenderer) base(raw json.RawMessage) Base {
	b := Base{
		ID:  r.ID,
		Raw: raw,
		Author: Author{
			Name:      r.AuthorName.String(),
			ChannelID: r.AuthorChannelID,
			PhotoURL:  r.AuthorPhoto.largest(),
		},
	}
	if us, err := strconv.ParseInt(r.TimestampUsec, 10, 64); err == nil {
		b.Time = time.UnixMicro(us)
	}
	for _, rb := range r.AuthorBadges {
		badge := rb.Renderer
		bd := Badge{Label: badge.Tooltip}
		switch strings.ToUpper(badge.Icon.IconType) {
		case "OWNER":
			bd.Kind = "owner"
			b.Author.Owner = true
		case "MODERATOR":
			bd.Kind = "moderator"
			b.Author.Moderator = true
		case "VERIFIED", "VERIFIED_ARTIST", "OFFICIAL_ARTIST_BADGE":
			bd.Kind = "verified"
			b.Author.Verified = true
		case "":
			// Members' badges are the channel's own image rather than an
			// icon, and that is the only badge drawn that way.
			if len(badge.CustomThumbnail.Thumbnails) == 0 {
				continue
			}
			bd.Kind = "member"
			bd.IconURL = badge.CustomThumbnail.largest()
			b.Author.Member = true
		default:
			bd.Kind = strings.ToLower(badge.Icon.IconType)
		}
		b.Author.Badges = append(b.Author.Badges, bd)
	}
	return b
}

// String flattens the text the way it reads.
func (t rawText) String() string {
	s, _ := t.parse()
	return s
}

// parse flattens the text and returns its runs. A simpleText is one run.
func (t rawText) parse() (string, []Run) {
	if len(t.Runs) == 0 {
		if t.SimpleText == "" {
			return "", nil
		}
		return t.SimpleText, []Run{{Text: t.SimpleText}}
	}
	var b strings.Builder
	runs := make([]Run, 0, len(t.Runs))
	for _, rr := range t.Runs {
		run := Run{Text: rr.Text, Bold: rr.Bold, Italic: rr.Italics}
		if e := rr.Emoji; e != nil {
			run.Emoji = &Emoji{
				ID:        e.EmojiID,
				Shortcuts: e.Shortcuts,
				Custom:    e.IsCustomEmoji,
				ImageURL:  e.Image.largest(),
			}
			run.Text = e.text()
		}
		if ne := rr.NavigationEndpoint; ne != nil {
			switch {
			case ne.URLEndpoint != nil:
				run.URL = unwrapRedirect(ne.URLEndpoint.URL)
			case ne.CommandMetadata.WebCommandMetadata.URL != "":
				run.URL = absoluteURL(ne.CommandMetadata.WebCommandMetadata.URL)
			}
		}
		b.WriteString(run.Text)
		runs = append(runs, run)
	}
	return b.String(), runs
}

// bold returns the first bold run, which is how a gift redemption names the
// gifter.
func (t rawText) bold() string {
	for _, r := range t.Runs {
		if r.Bold && strings.TrimSpace(r.Text) != "" {
			return strings.TrimSpace(r.Text)
		}
	}
	return ""
}

// text is how an emoji reads in a message: Unicode emoji as themselves, custom
// ones as their shortcut.
func (e *rawEmoji) text() string {
	if e.IsCustomEmoji || strings.Contains(e.EmojiID, "/") {
		if len(e.Shortcuts) > 0 {
			return e.Shortcuts[0]
		}
		if l := e.Image.Accessibility.AccessibilityData.Label; l != "" {
			return ":" + l + ":"
		}
		return e.EmojiID
	}
	if e.EmojiID != "" {
		return e.EmojiID
	}
	if l := e.Image.Accessibility.AccessibilityData.Label; l != "" {
		return l
	}
	if len(e.Shortcuts) > 0 {
		return e.Shortcuts[0]
	}
	return ""
}

// largest returns the URL of the widest thumbnail, made absolute: stickers
// come as protocol-relative "//lh3.googleusercontent.com/…" URLs.
func (im rawImage) largest() string {
	best, width := "", -1
	for _, t := range im.Thumbnails {
		if t.Width > width || best == "" {
			best, width = t.URL, t.Width
		}
	}
	if strings.HasPrefix(best, "//") {
		best = "https:" + best
	}
	return best
}

// unwrapRedirect turns YouTube's click-tracking redirect back into the link the
// author posted.
func unwrapRedirect(u string) string {
	p, err := url.Parse(u)
	if err != nil || p.Path != "/redirect" {
		return absoluteURL(u)
	}
	if q := p.Query().Get("q"); q != "" {
		return q
	}
	return u
}

func absoluteURL(u string) string {
	if strings.HasPrefix(u, "/") && !strings.HasPrefix(u, "//") {
		return "https://www.youtube.com" + u
	}
	return u
}

var (
	reNumber    = regexp.MustCompile(`\d[\d.,]*`)
	reMilestone = regexp.MustCompile(`(?i)(\d+)\s*(month|year)`)
)

// firstNumber returns the first whole number in s, 0 if there is none.
func firstNumber(s string) int {
	m := reNumber.FindString(s)
	n, _ := strconv.Atoi(strings.NewReplacer(".", "", ",", "").Replace(m))
	return n
}

// milestoneMonths reads "Member for 6 months" or "Member for 2 years". Only
// English is understood; other languages give 0.
func milestoneMonths(header string) int {
	m := reMilestone.FindStringSubmatch(header)
	if m == nil {
		return 0
	}
	n, _ := strconv.Atoi(m[1])
	if strings.EqualFold(m[2], "year") {
		n *= 12
	}
	return n
}

// decodeLenient unmarshals, tolerating fields whose JSON type differs from the
// Go one. encoding/json fills in everything else before reporting such a
// mismatch, and a renamed or retyped field YouTube does not promise to keep
// should cost that field, not the whole message.
func decodeLenient(data []byte, v any) error {
	err := json.Unmarshal(data, v)
	if _, ok := errors.AsType[*json.UnmarshalTypeError](err); ok {
		return nil
	}
	return err
}
