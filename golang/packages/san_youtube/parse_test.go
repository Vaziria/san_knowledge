package san_youtube

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

var testNow = time.Date(2026, 9, 24, 2, 0, 0, 0, time.UTC)

// loadActions parses testdata/actions.json, which holds one action of every
// kind this package models, then the kinds it skips.
func loadActions(t *testing.T) []Message {
	t.Helper()
	b, err := os.ReadFile("testdata/actions.json")
	if err != nil {
		t.Fatal(err)
	}
	var actions []json.RawMessage
	if err := json.Unmarshal(b, &actions); err != nil {
		t.Fatal(err)
	}
	return parseActions(actions, testNow)
}

func at(us int64) time.Time { return time.UnixMicro(us) }

func TestParseKinds(t *testing.T) {
	msgs := loadActions(t)

	var got []string
	for _, m := range msgs {
		got = append(got, string(m.Kind())+":"+m.Meta().ID)
	}
	// The placeholder, the system notice, the ticker, the banner and the
	// unknown action come last in the fixture and must all be skipped.
	want := []string{
		"text:text-1", "text:text-2", "text:text-3",
		"super_chat:paid-1", "super_chat:paid-2",
		"super_sticker:sticker-1",
		"membership:member-1", "membership:milestone-1",
		"gift_purchase:gift-1", "gift_redemption:redeem-1",
		"deletion:", "author_deletion:", "deletion:", "author_deletion:",
		"replacement:",
	}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("kinds:\n got %v\nwant %v", got, want)
	}
	for _, m := range msgs {
		if !json.Valid(m.Meta().Raw) {
			t.Errorf("%s %s: Raw is not the action JSON: %q", m.Kind(), m.Meta().ID, m.Meta().Raw)
		}
	}
}

func TestParseTextMessage(t *testing.T) {
	m := loadActions(t)[0].(*TextMessage)

	if m.Text != "Cek di google 🧡 halo semua" {
		t.Errorf("Text = %q", m.Text)
	}
	if !m.Time.Equal(at(1790188400000000)) {
		t.Errorf("Time = %v", m.Time)
	}
	want := Author{
		Name:      "@viewer-one",
		ChannelID: "UCfakeViewer000000000001",
		PhotoURL:  "https://yt4.ggpht.com/fake-viewer-01=s64-c-k-c0x00ffffff-no-rj",
	}
	if a := m.Author; a.Name != want.Name || a.ChannelID != want.ChannelID || a.PhotoURL != want.PhotoURL || len(a.Badges) != 0 {
		t.Errorf("Author = %+v, want %+v", a, want)
	}
	if len(m.Runs) != 3 {
		t.Fatalf("Runs = %+v", m.Runs)
	}
	e := m.Runs[1].Emoji
	if e == nil || e.ID != "🧡" || e.Custom || m.Runs[1].Text != "🧡" || e.Shortcuts[0] != ":orange_heart:" {
		t.Errorf("emoji run = %+v %+v", m.Runs[1], e)
	}
}

// Links come back unwrapped from YouTube's redirect, custom emoji read as
// their shortcut, and the owner's badges set the flags.
func TestParseLinksCustomEmojiAndBadges(t *testing.T) {
	msgs := loadActions(t)
	m := msgs[1].(*TextMessage)

	if m.Text != "Join here: https://bit.ly/fake-join :yt:" {
		t.Errorf("Text = %q", m.Text)
	}
	if m.Runs[1].URL != "https://bit.ly/fake-join" {
		t.Errorf("link URL = %q", m.Runs[1].URL)
	}
	e := m.Runs[3].Emoji
	if e == nil || !e.Custom || m.Runs[3].Text != ":yt:" || e.ImageURL != "https://yt3.ggpht.com/fake-emoji-yt=w48-h48-c-k-nd" {
		t.Errorf("custom emoji run = %+v %+v", m.Runs[3], e)
	}
	a := m.Author
	if !a.Owner || !a.Verified || a.Moderator || a.Member {
		t.Errorf("owner flags = %+v", a)
	}
	if len(a.Badges) != 2 || a.Badges[0].Kind != "verified" || a.Badges[1].Kind != "owner" {
		t.Errorf("owner badges = %+v", a.Badges)
	}

	mod := msgs[2].(*TextMessage).Author
	if !mod.Moderator || !mod.Member || mod.Owner || mod.Verified {
		t.Errorf("moderator flags = %+v", mod)
	}
	member := mod.Badges[1]
	if member.Kind != "member" || member.Label != "Member (6 months)" || member.IconURL != "https://yt3.ggpht.com/fake-member-badge=s32-c-k" {
		t.Errorf("member badge = %+v", member)
	}
}

func TestParseSuperChatAndSticker(t *testing.T) {
	msgs := loadActions(t)

	sc := msgs[3].(*SuperChat)
	if sc.Amount != (Amount{Text: "IDR 159,000", Value: 159000, Currency: "IDR"}) {
		t.Errorf("Amount = %+v", sc.Amount)
	}
	if sc.Text != "semangat terus" || sc.Author.Name != "@viewer-three" || !sc.Time.Equal(at(1790188403000000)) {
		t.Errorf("super chat = %+v", sc)
	}

	bare := msgs[4].(*SuperChat)
	if bare.Amount != (Amount{Text: "Rp 20.000", Value: 20000, Currency: "IDR"}) || bare.Text != "" || bare.Runs != nil {
		t.Errorf("super chat without a message = %+v", bare)
	}

	st := msgs[5].(*SuperSticker)
	if st.Amount != (Amount{Text: "$2.00", Value: 2, Currency: "USD"}) {
		t.Errorf("sticker Amount = %+v", st.Amount)
	}
	if st.Sticker != "Cat waving hello" || st.StickerURL != "https://lh3.googleusercontent.com/fake-sticker=s80-rp" {
		t.Errorf("sticker = %q %q", st.Sticker, st.StickerURL)
	}
	if st.Author.ChannelID != "UCfakeViewer000000000005" {
		t.Errorf("sticker author = %+v", st.Author)
	}
}

func TestParseMemberships(t *testing.T) {
	msgs := loadActions(t)

	joined := msgs[6].(*Membership)
	if joined.Milestone || joined.Months != 0 || joined.Header != "" || joined.Subtext != "Welcome to Kenalan!" || joined.Text != "" {
		t.Errorf("new member = %+v", joined)
	}
	if !joined.Author.Member || joined.Author.Badges[0].Label != "New member" {
		t.Errorf("new member badges = %+v", joined.Author)
	}

	ms := msgs[7].(*Membership)
	if !ms.Milestone || ms.Months != 24 || ms.Header != "Member for 2 years" || ms.Subtext != "Kenalan" {
		t.Errorf("milestone = %+v", ms)
	}
	if ms.Text != "dua tahun nonton 🎉" || len(ms.Runs) != 2 {
		t.Errorf("milestone text = %q, runs %+v", ms.Text, ms.Runs)
	}
}

func TestParseGifts(t *testing.T) {
	msgs := loadActions(t)

	gp := msgs[8].(*GiftPurchase)
	if gp.ID != "gift-1" || !gp.Time.Equal(at(1790188408000000)) {
		t.Errorf("gift purchase id/time = %q %v", gp.ID, gp.Time)
	}
	if gp.Count != 5 || gp.Text != "Sent 5 Kenalan gift memberships" {
		t.Errorf("gift purchase = %d %q", gp.Count, gp.Text)
	}
	// The buyer is in the header renderer, not on the outer one.
	if gp.Author.Name != "@viewer-eight" || gp.Author.ChannelID != "UCfakeViewer000000000008" ||
		gp.Author.PhotoURL == "" || !gp.Author.Member {
		t.Errorf("gift purchase author = %+v", gp.Author)
	}

	gr := msgs[9].(*GiftRedemption)
	if gr.Author.Name != "@viewer-nine" || gr.Gifter != "@viewer-eight" || gr.Text != "received a gift membership by @viewer-eight" {
		t.Errorf("gift redemption = %+v", gr)
	}
}

func TestParseDeletionsAndReplacement(t *testing.T) {
	msgs := loadActions(t)

	del := msgs[10].(*Deletion)
	if del.TargetID != "text-3" || del.Text != "[message retracted]" || del.ID != "" || !del.Time.Equal(testNow) {
		t.Errorf("mark deleted = %+v", del)
	}
	byAuthor := msgs[11].(*AuthorDeletion)
	if byAuthor.Author.ChannelID != "UCfakeViewer000000000004" || byAuthor.Text != "[message deleted]" {
		t.Errorf("mark deleted by author = %+v", byAuthor)
	}
	removed := msgs[12].(*Deletion)
	if removed.TargetID != "text-1" || removed.Text != "" {
		t.Errorf("remove = %+v", removed)
	}
	removedByAuthor := msgs[13].(*AuthorDeletion)
	if removedByAuthor.Author.ChannelID != "UCfakeViewer000000000005" {
		t.Errorf("remove by author = %+v", removedByAuthor)
	}

	rep := msgs[14].(*Replacement)
	if rep.TargetID != "placeholder-1" {
		t.Errorf("replacement target = %q", rep.TargetID)
	}
	inner, ok := rep.Message.(*TextMessage)
	if !ok || inner.ID != "text-4" || inner.Text != "pesan yang ditahan tadi" || inner.Author.Name != "@viewer-ten" {
		t.Errorf("replacement message = %#v", rep.Message)
	}
}

// A field YouTube changes the type of costs that field, not the message.
func TestParseToleratesRetypedFields(t *testing.T) {
	raw := json.RawMessage(`{"addChatItemAction":{"item":{"liveChatTextMessageRenderer":{
		"message":{"runs":[{"text":"still here"}]},
		"authorName":{"simpleText":"@viewer-one"},
		"authorBadges":"not a list any more",
		"timestampUsec":1790188400000000,
		"id":"retyped-1"}}}}`)
	msgs := parseActions([]json.RawMessage{raw}, testNow)
	if len(msgs) != 1 {
		t.Fatalf("got %d messages, want 1", len(msgs))
	}
	m := msgs[0].(*TextMessage)
	if m.ID != "retyped-1" || m.Text != "still here" || m.Author.Name != "@viewer-one" {
		t.Errorf("message = %+v", m)
	}
}

func TestParseSkipsGarbage(t *testing.T) {
	msgs := parseActions([]json.RawMessage{
		json.RawMessage(`not json`),
		json.RawMessage(`[]`),
		json.RawMessage(`{}`),
		json.RawMessage(`{"addChatItemAction":{}}`),
		json.RawMessage(`{"replaceChatItemAction":{"targetItemId":"x","replacementItem":{"liveChatPlaceholderItemRenderer":{"id":"y"}}}}`),
	}, testNow)
	if len(msgs) != 0 {
		t.Errorf("got %d messages from garbage, want 0: %+v", len(msgs), msgs)
	}
}

func TestMessageJSON(t *testing.T) {
	msgs := loadActions(t)

	b, err := json.Marshal(msgs[3])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(b), `{"type":"super_chat","id":"paid-1",`) {
		t.Errorf("super chat JSON starts %s", b[:min(len(b), 60)])
	}
	var sc struct {
		Amount Amount `json:"amount"`
		Author Author `json:"author"`
		Raw    any    `json:"raw"`
	}
	if err := json.Unmarshal(b, &sc); err != nil {
		t.Fatal(err)
	}
	if sc.Amount.Value != 159000 || sc.Amount.Currency != "IDR" || sc.Author.ChannelID != "UCfakeViewer000000000003" {
		t.Errorf("super chat JSON = %s", b)
	}
	if sc.Raw != nil {
		t.Errorf("Raw is in the JSON; it should be left out")
	}

	// A replacement nests its message, type and all.
	b, err = json.Marshal(msgs[14])
	if err != nil {
		t.Fatal(err)
	}
	var rep struct {
		Type     string `json:"type"`
		TargetID string `json:"target_id"`
		Message  struct {
			Type string `json:"type"`
			ID   string `json:"id"`
		} `json:"message"`
	}
	if err := json.Unmarshal(b, &rep); err != nil {
		t.Fatal(err)
	}
	if rep.Type != "replacement" || rep.TargetID != "placeholder-1" || rep.Message.Type != "text" || rep.Message.ID != "text-4" {
		t.Errorf("replacement JSON = %s", b)
	}

	// Every kind marshals, and says what it is.
	for _, m := range msgs {
		b, err := json.Marshal(m)
		if err != nil {
			t.Fatalf("%s: %v", m.Kind(), err)
		}
		var head struct{ Type Kind }
		json.Unmarshal(b, &head)
		if head.Type != m.Kind() {
			t.Errorf("%s marshals as type %q", m.Kind(), head.Type)
		}
	}
}
