package san_youtube

import (
	"encoding/json"
	"time"
)

// Kind names a type of chat message. It is also the "type" field of every
// message's JSON form.
type Kind string

const (
	KindText           Kind = "text"
	KindSuperChat      Kind = "super_chat"
	KindSuperSticker   Kind = "super_sticker"
	KindMembership     Kind = "membership"
	KindGiftPurchase   Kind = "gift_purchase"
	KindGiftRedemption Kind = "gift_redemption"
	KindDeletion       Kind = "deletion"
	KindAuthorDeletion Kind = "author_deletion"
	KindReplacement    Kind = "replacement"
)

// Message is one event from a live chat. The concrete types are the pointer
// types in this file, so a caller switches on them:
//
//	switch m := msg.(type) {
//	case *san_youtube.TextMessage:
//		fmt.Println(m.Author.Name, m.Text)
//	case *san_youtube.SuperChat:
//		fmt.Println(m.Author.Name, m.Amount.Text, m.Text)
//	}
//
// Kinds of chat item this package does not model (system notices, polls,
// banners, the ticker) are skipped; Meta().Raw keeps the whole action of every
// message that is returned.
type Message interface {
	Kind() Kind
	// Meta returns the fields every message has.
	Meta() Base
}

// Base is what every message carries.
//
// Deletions and replacements act on another message and have neither an ID
// nor a timestamp of their own: their ID is empty, their Time is when the
// action arrived, and TargetID names the message they act on.
type Base struct {
	ID     string    `json:"id,omitempty"`
	Time   time.Time `json:"time"`
	Author Author    `json:"author,omitzero"`
	// Raw is the chat action exactly as YouTube sent it, for anything this
	// package does not model.
	Raw json.RawMessage `json:"-"`
}

// Meta returns b. Every message type embeds Base, so this is how code that
// does not care about the type reaches the author and time.
func (b Base) Meta() Base { return b }

// Author is who wrote a message.
type Author struct {
	Name      string `json:"name,omitempty"`
	ChannelID string `json:"channel_id,omitempty"`
	// PhotoURL is the largest avatar YouTube offered.
	PhotoURL string `json:"photo_url,omitempty"`

	Owner     bool `json:"owner,omitempty"`
	Moderator bool `json:"moderator,omitempty"`
	Verified  bool `json:"verified,omitempty"`
	Member    bool `json:"member,omitempty"`

	// Badges are all the badges as sent, including any kind the flags above
	// do not cover.
	Badges []Badge `json:"badges,omitempty"`
}

// Badge is one badge shown next to an author's name.
type Badge struct {
	// Kind is "owner", "moderator", "verified" or "member", or the icon name
	// YouTube used, lowercased, for anything else.
	Kind string `json:"kind"`
	// Label is the tooltip, e.g. "Moderator" or "Member (6 months)".
	Label string `json:"label,omitempty"`
	// IconURL is the channel's own badge image, which members' badges have.
	IconURL string `json:"icon_url,omitempty"`
}

// Run is one piece of a message as YouTube sent it: plain text, an emoji, or
// a link. A message's Text is its runs' Text joined.
type Run struct {
	// Text is how the run reads in Text: the text itself, a Unicode emoji,
	// or a custom emoji's shortcut such as ":yt:".
	Text  string `json:"text"`
	Emoji *Emoji `json:"emoji,omitempty"`
	// URL is the link target, already unwrapped from YouTube's redirect.
	URL    string `json:"url,omitempty"`
	Bold   bool   `json:"bold,omitempty"`
	Italic bool   `json:"italic,omitempty"`
}

// Emoji describes an emoji run.
type Emoji struct {
	// ID is the emoji itself for Unicode emoji, and a "<channel>/<id>" key
	// for custom ones.
	ID        string   `json:"id"`
	Shortcuts []string `json:"shortcuts,omitempty"`
	// Custom is set for channel and YouTube emoji that are images rather than
	// Unicode characters.
	Custom   bool   `json:"custom,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
}

// TextMessage is an ordinary chat message.
type TextMessage struct {
	Base
	Text string `json:"text"`
	Runs []Run  `json:"runs,omitempty"`
}

// SuperChat is a paid message. Text is empty when the buyer wrote nothing.
type SuperChat struct {
	Base
	Amount Amount `json:"amount"`
	Text   string `json:"text"`
	Runs   []Run  `json:"runs,omitempty"`
}

// SuperSticker is a paid sticker.
type SuperSticker struct {
	Base
	Amount Amount `json:"amount"`
	// Sticker is the sticker's description, e.g. "Cat waving hello".
	Sticker    string `json:"sticker,omitempty"`
	StickerURL string `json:"sticker_url,omitempty"`
}

// Membership is a new member joining, or a member celebrating a milestone.
type Membership struct {
	Base
	// Milestone is set for "Member for N months" messages, and unset for a
	// new member.
	Milestone bool `json:"milestone,omitempty"`
	// Months is the milestone length when it could be read from Header,
	// which is English unless another language was asked for.
	Months int `json:"months,omitempty"`
	// Header is the milestone line, e.g. "Member for 6 months". Empty for a
	// new member.
	Header string `json:"header,omitempty"`
	// Subtext is the smaller line: "Welcome to <level>!" for a new member,
	// or the membership level for a milestone.
	Subtext string `json:"subtext,omitempty"`
	// Text is what a member wrote with a milestone.
	Text string `json:"text,omitempty"`
	Runs []Run  `json:"runs,omitempty"`
}

// GiftPurchase is someone gifting memberships to other viewers. Author is the
// buyer.
type GiftPurchase struct {
	Base
	// Count is the number of memberships gifted, 0 if it could not be read.
	Count int `json:"count,omitempty"`
	// Text is the announcement, e.g. "Sent 5 Kenalan gift memberships".
	Text string `json:"text"`
}

// GiftRedemption is a viewer receiving a gifted membership. Author is the
// viewer who received it.
type GiftRedemption struct {
	Base
	// Gifter is the name of who gave it, when the message names them.
	Gifter string `json:"gifter,omitempty"`
	// Text is the announcement, e.g. "received a gift membership by @name".
	Text string `json:"text"`
}

// Deletion removes one message from the chat, either leaving a note in its
// place (Text, e.g. "[message deleted]" or "[message retracted]") or removing
// it outright (Text empty).
type Deletion struct {
	Base
	TargetID string `json:"target_id"`
	Text     string `json:"text,omitempty"`
}

// AuthorDeletion removes every message by one author, typically when they are
// banned or timed out. Author.ChannelID is that author; no other author field
// is known.
type AuthorDeletion struct {
	Base
	Text string `json:"text,omitempty"`
}

// Replacement swaps the message TargetID for Message. YouTube uses it to put a
// message in place of the placeholder shown while it was held for review.
// Placeholders are not returned, so Message is a message the reader has not
// been given before: a replacement bringing a message already returned (with
// the same ID) is dropped as a repeat.
type Replacement struct {
	Base
	TargetID string  `json:"target_id"`
	Message  Message `json:"message"`
}

func (TextMessage) Kind() Kind    { return KindText }
func (SuperChat) Kind() Kind      { return KindSuperChat }
func (SuperSticker) Kind() Kind   { return KindSuperSticker }
func (Membership) Kind() Kind     { return KindMembership }
func (GiftPurchase) Kind() Kind   { return KindGiftPurchase }
func (GiftRedemption) Kind() Kind { return KindGiftRedemption }
func (Deletion) Kind() Kind       { return KindDeletion }
func (AuthorDeletion) Kind() Kind { return KindAuthorDeletion }
func (Replacement) Kind() Kind    { return KindReplacement }

// The MarshalJSON methods add the "type" field, so a stream of JSON lines can
// be told apart without knowing the Go types.

func (m TextMessage) MarshalJSON() ([]byte, error) {
	type plain TextMessage
	return marshalKind(KindText, plain(m))
}

func (m SuperChat) MarshalJSON() ([]byte, error) {
	type plain SuperChat
	return marshalKind(KindSuperChat, plain(m))
}

func (m SuperSticker) MarshalJSON() ([]byte, error) {
	type plain SuperSticker
	return marshalKind(KindSuperSticker, plain(m))
}

func (m Membership) MarshalJSON() ([]byte, error) {
	type plain Membership
	return marshalKind(KindMembership, plain(m))
}

func (m GiftPurchase) MarshalJSON() ([]byte, error) {
	type plain GiftPurchase
	return marshalKind(KindGiftPurchase, plain(m))
}

func (m GiftRedemption) MarshalJSON() ([]byte, error) {
	type plain GiftRedemption
	return marshalKind(KindGiftRedemption, plain(m))
}

func (m Deletion) MarshalJSON() ([]byte, error) {
	type plain Deletion
	return marshalKind(KindDeletion, plain(m))
}

func (m AuthorDeletion) MarshalJSON() ([]byte, error) {
	type plain AuthorDeletion
	return marshalKind(KindAuthorDeletion, plain(m))
}

func (m Replacement) MarshalJSON() ([]byte, error) {
	type plain Replacement
	return marshalKind(KindReplacement, plain(m))
}

// marshalKind marshals v, an object, with "type" as its first field. The
// kinds are the constants above, so they need no escaping.
func marshalKind(k Kind, v any) ([]byte, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	out := []byte(`{"type":"` + string(k) + `"`)
	if len(b) > 2 {
		out = append(out, ',')
	}
	return append(out, b[1:]...), nil
}
