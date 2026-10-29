package mcp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/wargasipil/san_remote/internal/chatgpt"
	"github.com/wargasipil/san_remote/internal/claude"
	"github.com/wargasipil/san_remote/internal/gemini"
	"github.com/wargasipil/san_remote/internal/instagram"
	"github.com/wargasipil/san_remote/internal/linkedin"
	"github.com/wargasipil/san_remote/internal/medium"
	"github.com/wargasipil/san_remote/internal/tiktok"
	"github.com/wargasipil/san_remote/internal/youtube"
)

// The tools of the sites added on 2026-10-25: medium_*, linkedin_*, tiktok_*,
// chatgpt_*, gemini_* and claude_*; youtube_* on 2026-10-26, telegram_* on
// 2026-10-27, instagram_* on 2026-10-29.

const sitesInstructions = `
medium_* write the user's markdown articles into Medium (interface in English) through that browser, as a person does in Medium's editor: medium_draft makes a draft from an article file (title, subtitle, text, images with alt text and captions) and, with at, schedules it; medium_schedule schedules a draft, medium_unschedule cancels its time; medium_drafts, medium_scheduled, medium_get and medium_status read. Nothing is published at once. medium_delete deletes a draft (it cannot be undone: ask the user first).
linkedin_* do the same for the user's LinkedIn articles (interface in English), in LinkedIn's article editor: linkedin_draft (with a cover, and with at a schedule on a quarter hour, shared with text), linkedin_schedule, linkedin_unschedule (by title), linkedin_drafts, linkedin_scheduled, linkedin_get, linkedin_status; linkedin_delete deletes a draft (ask the user first). Each brings Chrome to the front for a minute or more.
tiktok_* post the user's videos to TikTok (interface in English) through TikTok Studio in that browser: tiktok_post schedules a video for Everyone (15 minutes to 30 days ahead, on a 5-minute step) or, with now, posts it for Only you; tiktok_posts lists Studio's posts; tiktok_delete deletes one (it cannot be undone: ask the user first); tiktok_status reads. Post only what the user asked for.
youtube_* upload the user's videos to YouTube (interface in English) through YouTube Studio in that browser: youtube_upload uploads a video (title, description, tags; short for a Short, a vertical or square video of at most 3 minutes) and schedules it at at (at least 15 minutes ahead), or without at saves it as Private: never public at once; youtube_videos lists Studio's Videos and Shorts; youtube_private makes a video Private (a scheduled one loses its schedule); youtube_delete deletes one for good (a Public one only with allow_public; it cannot be undone: ask the user first); youtube_status reads. Upload only what the user asked for. An upload brings Chrome to the front for as long as the file takes to go up.
instagram_* post the user's photos and videos to Instagram through that browser: instagram_post with at schedules photos or one video (a Reel) with a caption in Meta Business Suite (interface in Indonesian; up to 10 files, 15 minutes to 29 days ahead); with now it shares them at once on instagram.com (interface in English; up to 20 files), which cannot schedule, so use now only for a post the user asked to publish now. rehearse does every step but the last. instagram_scheduled lists what is scheduled in Business Suite, instagram_posts the profile's latest posts; instagram_delete deletes a published post (kept in Recently deleted for 30 days) and instagram_delete_scheduled a scheduled one (it cannot be undone): ask the user first; instagram_status reads.
telegram_* work in Telegram Web (interface in English) in that browser, as the user's own account: telegram_create_bot asks @BotFather for a new bot and returns its token (a secret: hand it to what needs it, never repeat it in chat); telegram_delete_bot deletes one of the account's bots; telegram_create_group makes a group with no other members yet and returns its id; telegram_delete_group deletes a group for all its members; telegram_add_member and telegram_remove_member add and remove a user by username; telegram_status reads. Deletes and removals cannot be undone, and adding someone to a group is seen by them: ask the user first. Each brings Chrome to the front with a tab of its own for a minute or so.
chatgpt_* ask ChatGPT (interface in English) through that browser: chatgpt_ask sends a prompt and returns the answer as markdown, in a Temporary chat unless keep is set; chat continues a kept chat by its link. chatgpt_status reads the account and plan.
gemini_* ask Gemini (gemini.google.com, interface in English) through that browser: gemini_ask sends a prompt and returns the answer, in a Temporary chat unless keep is set; it brings Chrome to the front while it asks, since Gemini sends only from a shown page. gemini_status reads the Google account's name.
claude_* ask Claude on claude.ai (interface in English) through that browser: claude_ask sends a prompt and returns the answer as markdown, in an incognito chat (kept out of the history) unless keep is set; claude_status reads the account and plan.`

func siteToolDefs() []any {
	storyProp := str("The story's id or its link (https://medium.com/p/<id>/edit)")
	return []any{
		tool("medium_status", "Medium: who is signed in",
			"Whether the browser is signed in to Medium, and as whom.",
			schema(map[string]any{"browser": browserProp}), true, false),
		tool("medium_draft", "Medium: write a draft",
			"Write a markdown article file into a new Medium draft: the first # heading is the title, an italic paragraph under it the subtitle, each image on a paragraph of its own is uploaded with its alt text, and an italic paragraph under an image is its caption. With at, the draft is then scheduled (with topics). Brings Chrome to the front for a few minutes.",
			schema(map[string]any{
				"file":    str("The article's markdown file"),
				"at":      timeProp("Schedule it for this time (at least 10 minutes ahead); leave out to keep it a draft"),
				"topics":  strs("Topics, as Medium suggests them (up to 5); only with at"),
				"browser": browserProp,
			}, "file"), false, false),
		tool("medium_drafts", "Medium: drafts",
			"The drafts on the stories page: id, title, when updated, words.",
			schema(map[string]any{"browser": browserProp}), true, false),
		tool("medium_scheduled", "Medium: scheduled stories",
			"The stories in the Scheduled tab, with when Medium publishes each.",
			schema(map[string]any{"browser": browserProp}), true, false),
		tool("medium_get", "Medium: read a draft",
			"A draft as the editor holds it: status, title, subtitle, images, and the text.",
			schema(map[string]any{"story": storyProp, "browser": browserProp}, "story"), true, false),
		tool("medium_schedule", "Medium: schedule a draft",
			"Have Medium publish a draft at a time, set on its publish page, with topics added (and remove_topics taken off first). Brings Chrome to the front.",
			schema(map[string]any{
				"story":         storyProp,
				"at":            timeProp("When Medium publishes it (at least 10 minutes ahead)"),
				"topics":        strs("Topics to add, as Medium suggests them (up to 5)"),
				"remove_topics": strs("Topics to take off first"),
				"browser":       browserProp,
			}, "story", "at"), false, false),
		tool("medium_unschedule", "Medium: cancel a schedule",
			"Cancel a scheduled story's time; it stays a draft. Brings Chrome to the front.",
			schema(map[string]any{"story": storyProp, "browser": browserProp}, "story"), false, false),
		tool("medium_delete", "Medium: delete a draft",
			"Delete a draft (only drafts). Medium asks first and the tool answers Delete. It cannot be undone: ask the user first.",
			schema(map[string]any{"story": storyProp, "browser": browserProp}, "story"), false, true),

		tool("linkedin_status", "LinkedIn: who is signed in",
			"Whether the browser is signed in to LinkedIn, and as whom.",
			schema(map[string]any{"browser": browserProp}), true, false),
		tool("linkedin_draft", "LinkedIn: write an article draft",
			"Write a markdown article file into a new LinkedIn article draft, read as for medium_draft, with an optional cover image. With at (a quarter hour, at least 15 minutes ahead) the draft is then scheduled, shared with text. Brings Chrome to the front for a few minutes.",
			schema(map[string]any{
				"file":    str("The article's markdown file"),
				"cover":   str("The cover image file"),
				"at":      timeProp("Schedule it for this time, on a quarter hour; leave out to keep it a draft"),
				"text":    str("The post that shares the article when it goes out; only with at"),
				"browser": browserProp,
			}, "file"), false, false),
		tool("linkedin_drafts", "LinkedIn: article drafts",
			"The article drafts, from the editor's Articles dialog: id, title, when modified. Brings Chrome to the front.",
			schema(map[string]any{"browser": browserProp}), true, false),
		tool("linkedin_scheduled", "LinkedIn: scheduled articles",
			"The scheduled articles, with when LinkedIn publishes each. The Scheduled tab shows no ids. Brings Chrome to the front.",
			schema(map[string]any{"browser": browserProp}), true, false),
		tool("linkedin_get", "LinkedIn: read a draft",
			"A draft as the editor holds it: status, title, cover, images, and the text. Brings Chrome to the front.",
			schema(map[string]any{"story": str("The draft's id or edit link"), "browser": browserProp}, "story"), true, false),
		tool("linkedin_schedule", "LinkedIn: schedule a draft",
			"Have LinkedIn publish a draft at a time (a quarter hour, at least 15 minutes ahead), shared with text, through the editor's Next. Brings Chrome to the front.",
			schema(map[string]any{
				"story":   str("The draft's id or edit link"),
				"at":      timeProp("When LinkedIn publishes it"),
				"text":    str("The post that shares the article"),
				"browser": browserProp,
			}, "story", "at"), false, false),
		tool("linkedin_unschedule", "LinkedIn: move a scheduled article to the drafts",
			"Move a scheduled article back to the drafts (it keeps its id), named by its title; when picks one of two with the same title.",
			schema(map[string]any{
				"match":   str("The article's title"),
				"when":    str(`Its time as linkedin_scheduled shows it, "Fri, Nov 20 at 9:00 AM"`),
				"browser": browserProp,
			}, "match"), false, false),
		tool("linkedin_delete", "LinkedIn: delete a draft",
			"Delete an article draft (only drafts). LinkedIn asks first and the tool answers Delete. It cannot be undone: ask the user first.",
			schema(map[string]any{"story": str("The draft's id or edit link"), "browser": browserProp}, "story"), false, true),

		tool("tiktok_status", "TikTok: who is signed in",
			"Whether the browser is signed in to TikTok, and as whom.",
			schema(map[string]any{"browser": browserProp}), true, false),
		tool("tiktok_post", "TikTok: schedule a video",
			"Upload a video through TikTok Studio, write its caption (#hashtags picked from TikTok's suggestions), and schedule it for Everyone at at; or with now, post it now for Only you. Give a post file (frontmatter video: and at:, then the caption) or video and caption. Brings Chrome to the front for a few minutes.",
			schema(map[string]any{
				"file":    str("A post markdown file: frontmatter video: and at:, then the caption"),
				"video":   str("The video file, instead of a post file"),
				"caption": str("The caption, with video"),
				"at":      timeProp("When TikTok publishes it, for Everyone: 15 minutes to 30 days ahead, on a 5-minute step"),
				"now":     flag("Post it now, seen by Only you, instead of scheduling it"),
				"browser": browserProp,
			}), false, false),
		tool("tiktok_posts", "TikTok: Studio's posts",
			"Studio's Posts list, newest first: when (a scheduled post's publish time), length, who can see it, caption.",
			schema(map[string]any{"browser": browserProp}), true, false),
		tool("tiktok_delete", "TikTok: delete a post",
			"Delete the one post of Studio's Posts list whose caption starts with caption and whose date is when, as tiktok_posts shows it. It cannot be undone: ask the user first.",
			schema(map[string]any{
				"caption": str("The post's caption, or its start"),
				"when":    str(`Its date as tiktok_posts shows it, "Nov 3, 7:00 PM"`),
				"browser": browserProp,
			}, "caption", "when"), false, true),

		tool("instagram_status", "Instagram: who is signed in",
			"Whether the browser is signed in to Instagram, and as whom (@handle and name).",
			schema(map[string]any{"browser": browserProp}), true, false),
		tool("instagram_post", "Instagram: schedule a post, or share it now",
			"With at, schedule photos (a carousel of up to 10) or one video (a Reel) with a caption in Meta Business Suite's composer, for the Instagram account only. With now, share them (up to 20) on instagram.com at once, in the Original crop, with the photos' alt text, Share to Threads and Facebook off: instagram.com cannot schedule, so give now only for a post the user asked to publish now. rehearse does every step but the last and discards it. Give a post file (frontmatter at: and ai_label:, the caption, and ![alt](file) for each photo or video, in order) or media and caption. Brings Chrome to the front for a few minutes.",
			schema(map[string]any{
				"file":     str("A post markdown file: the caption, and ![alt](file) for each photo or video, relative to it"),
				"media":    strs("The photo and video files, in order, instead of a post file"),
				"caption":  str("The caption, with media: plain text, up to 2200 characters and 30 #hashtags"),
				"at":       timeProp("Schedule it in Meta Business Suite for this time: a whole minute, 15 minutes to 29 days ahead"),
				"now":      flag("Share it now: Instagram publishes it at once"),
				"rehearse": flag("Do every step but Share, then discard the post"),
				"ai_label": flag(`Turn on "Add AI label", for realistic photos and videos made with AI`),
				"browser":  browserProp,
			}), false, false),
		tool("instagram_scheduled", "Instagram: scheduled posts",
			"The posts scheduled in Meta Business Suite (Konten, Dijadwalkan): when each goes out, post or reel, caption.",
			schema(map[string]any{"browser": browserProp}), true, false),
		tool("instagram_delete", "Instagram: delete a post",
			"Delete a published post of the account, by its link or code: its page's More Options, Delete. Instagram keeps it in Recently deleted for 30 days. Ask the user first.",
			schema(map[string]any{
				"post":    str("The post's link (https://www.instagram.com/p/<code>/) or its code"),
				"browser": browserProp,
			}, "post"), false, true),
		tool("instagram_delete_scheduled", "Instagram: delete a scheduled post",
			"Delete the one post scheduled in Meta Business Suite whose caption starts with caption and, when given, goes out at at, as instagram_scheduled shows it. It cannot be undone: ask the user first.",
			schema(map[string]any{
				"caption": str("The post's caption, or its start"),
				"at":      timeProp("When it goes out, as scheduled"),
				"browser": browserProp,
			}), false, true),
		tool("instagram_posts", "Instagram: the profile's posts",
			"The profile's latest posts as its grid shows them, pinned ones first: when, post or reel, link, caption. Each is read on its own page.",
			schema(map[string]any{"limit": intg("How many (default 10)"), "browser": browserProp}), true, false),

		tool("youtube_status", "YouTube: who is signed in",
			"Whether the browser is signed in to YouTube, and as whom (name and @handle).",
			schema(map[string]any{"browser": browserProp}), true, false),
		tool("youtube_upload", "YouTube: upload a video",
			"Upload a video through YouTube Studio with its title, description and tags, say it is not made for kids, and schedule it at at, or without at save it as Private. Never public at once. With short, the upload fails when Studio does not list it under Shorts (a Short is vertical or square and at most 3 minutes; it is saved all the same). Give a post file (frontmatter video:, tags:, at:, short:; then a `# ` title and the description) or video and title. Brings Chrome to the front for as long as the file takes to go up.",
			schema(map[string]any{
				"file":        str("A post markdown file: frontmatter video:, tags:, at:, short:, then a `# ` title and the description"),
				"video":       str("The video file, instead of a post file"),
				"title":       str("The title, with video: up to 100 characters, no < or >"),
				"description": str("The description, with video: plain text, up to 5000 bytes"),
				"tags":        strs("Tags"),
				"at":          timeProp("When YouTube makes it public: at least 15 minutes ahead; without it, it is saved as Private"),
				"short":       flag("It is meant as a Short"),
				"browser":     browserProp,
			}), false, false),
		tool("youtube_private", "YouTube: make a video Private",
			"Make a video Private in Studio's content list (its visibility, Private, Save); a scheduled one loses its schedule. Brings Chrome to the front.",
			schema(map[string]any{
				"video":   str("The video's id, or a link to it (youtu.be/<id>, youtube.com/shorts/<id>, studio link)"),
				"browser": browserProp,
			}, "video"), false, false),
		tool("youtube_delete", "YouTube: delete a video",
			"Delete a video for good from Studio's content list (Options, Delete forever). A Public video is refused unless allow_public. It cannot be undone: ask the user first.",
			schema(map[string]any{
				"video":        str("The video's id, or a link to it"),
				"allow_public": flag("Delete it even when it is Public"),
				"browser":      browserProp,
			}, "video"), false, true),
		tool("youtube_videos", "YouTube: Studio's videos",
			"Studio's content list, the Videos tab then Shorts, newest first: kind, visibility, date, link, title. Brings Chrome to the front briefly.",
			schema(map[string]any{"browser": browserProp}), true, false),

		tool("telegram_status", "Telegram: who is signed in",
			"Whether the browser is signed in to Telegram Web, and as whom (name and @username).",
			schema(map[string]any{"browser": browserProp}), true, false),
		tool("telegram_create_bot", "Telegram: create a bot",
			"Ask @BotFather (/newbot) for a new bot with name and username, and return its username, link and token. The token is a secret: never repeat it in chat. @BotFather's refusal (a username taken, too many bots) comes back with what it said. Brings Chrome to the front for a minute.",
			schema(map[string]any{
				"name":     str("The bot's name, up to 64 characters"),
				"username": str("Its username: 5 to 32 letters, digits and _, ending in bot"),
				"browser":  browserProp,
			}, "name", "username"), false, false),
		tool("telegram_delete_bot", "Telegram: delete a bot",
			"Delete one of the account's bots through @BotFather (/deletebot, with its confirmation). It cannot be undone: ask the user first.",
			schema(map[string]any{
				"username": str("The bot's username, with or without the @"),
				"browser":  browserProp,
			}, "username"), false, true),
		tool("telegram_create_group", "Telegram: create a group",
			"Make a group with title and no other members yet (New Group), and return its id (-5012345678). Brings Chrome to the front for a minute.",
			schema(map[string]any{
				"title":   str("The group's title, up to 128 characters"),
				"browser": browserProp,
			}, "title"), false, false),
		tool("telegram_delete_group", "Telegram: delete a group",
			"Delete a group of the account's for all its members (the chat's Delete Group, with Delete for all members). A group the account does not own is refused. It cannot be undone: ask the user first.",
			schema(map[string]any{
				"group":   str("The group's id (-5012345678), or its title when only one of the account's groups has it"),
				"browser": browserProp,
			}, "group"), false, true),
		tool("telegram_add_member", "Telegram: add a user to a group",
			"Add a user, by username, to a group of the account's, from the group's profile (Add Members). A user already in it is left as they are; one whose privacy settings keep them out of groups is refused. Ask the user first.",
			schema(map[string]any{
				"group":   str("The group's id, or its title"),
				"user":    str("The user's username, with or without the @"),
				"browser": browserProp,
			}, "group", "user"), false, false),
		tool("telegram_remove_member", "Telegram: remove a user from a group",
			"Remove a user, by username, from a group of the account's (the member's Remove from group in the group's profile). Ask the user first.",
			schema(map[string]any{
				"group":   str("The group's id, or its title"),
				"user":    str("The member's username, with or without the @"),
				"browser": browserProp,
			}, "group", "user"), false, true),

		tool("chatgpt_status", "ChatGPT: who is signed in",
			"Whether the browser is signed in to ChatGPT, as whom, and on which plan.",
			schema(map[string]any{"browser": browserProp}), true, false),
		tool("chatgpt_ask", "ChatGPT: ask",
			"Send a prompt to ChatGPT and return its answer as markdown. A new chat is a Temporary chat (not kept in the user's history) unless keep is set; chat continues a kept chat by its link. Works in a background tab.",
			schema(map[string]any{
				"prompt":       str("The prompt"),
				"keep":         flag("A normal chat, kept in the history, instead of a Temporary chat; its link comes back"),
				"chat":         str("Continue the kept chat at this link (https://chatgpt.com/c/<id>)"),
				"wait_seconds": intg("How long to wait for the answer (default 600)"),
				"browser":      browserProp,
			}, "prompt"), false, false),

		tool("claude_status", "Claude: who is signed in",
			"Whether the browser is signed in to claude.ai, as whom, and on which plan.",
			schema(map[string]any{"browser": browserProp}), true, false),
		tool("claude_ask", "Claude: ask",
			"Send a prompt to Claude on claude.ai and return its answer as markdown. A new chat is an incognito chat (kept out of the user's history) unless keep is set; chat continues a kept chat by its link. Works in a background tab.",
			schema(map[string]any{
				"prompt":       str("The prompt"),
				"keep":         flag("A normal chat, kept in the history, instead of an incognito chat; its link comes back"),
				"chat":         str("Continue the kept chat at this link (https://claude.ai/chat/<id>)"),
				"wait_seconds": intg("How long to wait for the answer (default 600)"),
				"browser":      browserProp,
			}, "prompt"), false, false),

		tool("gemini_status", "Gemini: who is signed in",
			"Whether the browser is signed in to Gemini, and as whom (the Google account's name).",
			schema(map[string]any{"browser": browserProp}), true, false),
		tool("gemini_ask", "Gemini: ask",
			"Send a prompt to Gemini and return its answer. A new chat is a Temporary chat (not kept in the user's history) unless keep is set; chat continues a kept chat by its link. Brings Chrome to the front while it asks.",
			schema(map[string]any{
				"prompt":       str("The prompt"),
				"keep":         flag("A normal chat, kept in the history, instead of a Temporary chat; its link comes back"),
				"chat":         str("Continue the kept chat at this link (https://gemini.google.com/app/<id>)"),
				"wait_seconds": intg("How long to wait for the answer (default 600)"),
				"browser":      browserProp,
			}, "prompt"), false, false),
	}
}

func (s *Server) medium(ctx context.Context, name string, a args) (string, error) {
	m := s.Medium
	switch name {
	case "medium_status":
		st, err := m.Status(ctx, a.Browser)
		if err != nil {
			return "", err
		}
		if !st.SignedIn {
			return "The browser is not signed in to Medium.", nil
		}
		return fmt.Sprintf("Signed in to Medium as %s (@%s).", st.Name, st.Handle), nil
	case "medium_draft":
		if a.File == "" {
			return "", errors.New("file is required")
		}
		art, err := medium.ParseFile(a.File)
		if err != nil {
			return "", err
		}
		at, err := optionalTime(a.At)
		if err != nil {
			return "", err
		}
		st, err := m.Draft(ctx, medium.Draft{Browser: a.Browser, Article: art, At: at, Topics: a.Topics})
		if err != nil {
			if st != nil {
				return "", fmt.Errorf("%w (the draft so far: %s)", err, st.URL)
			}
			return "", err
		}
		var b strings.Builder
		fmt.Fprintf(&b, "Draft: %s %q", st.URL, st.Title)
		if !st.At.IsZero() {
			fmt.Fprintf(&b, ", scheduled for %s", st.At.Format("2006-01-02 15:04"))
		}
		for _, w := range art.Warnings {
			fmt.Fprintf(&b, "\nwarning: %s", w)
		}
		return b.String(), nil
	case "medium_drafts", "medium_scheduled":
		var list []medium.Story
		var err error
		if name == "medium_drafts" {
			list, err = m.Drafts(ctx, a.Browser)
		} else {
			list, err = m.Scheduled(ctx, a.Browser)
		}
		if err != nil {
			return "", err
		}
		if len(list) == 0 {
			return "None.", nil
		}
		var b strings.Builder
		for _, st := range list {
			when := st.Updated
			if !st.At.IsZero() {
				when = "scheduled " + st.At.Format("2006-01-02 15:04")
			}
			fmt.Fprintf(&b, "%s %q, %s, %d words, %s\n", st.ID, st.Title, when, st.Words, st.URL)
		}
		return b.String(), nil
	case "medium_get":
		c, err := m.Get(ctx, a.Browser, a.Story)
		if err != nil {
			return "", err
		}
		var b strings.Builder
		fmt.Fprintf(&b, "status: %s\ntitle: %s\nsubtitle: %s\nlink: %s\n", c.Status, c.Title, c.Subtitle, c.URL)
		for i, im := range c.Images {
			fmt.Fprintf(&b, "image %d: %s alt=%q caption=%q\n", i+1, im.URL, im.Alt, im.Caption)
		}
		fmt.Fprintf(&b, "\n%s", c.Text)
		return b.String(), nil
	case "medium_schedule":
		at, err := medium.ParseTime(a.At)
		if err != nil {
			return "", err
		}
		st, err := m.Schedule(ctx, a.Browser, a.Story, at, a.Topics, a.RemoveTopics)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Scheduled %s %q for %s.", st.ID, st.Title, st.At.Format("2006-01-02 15:04")), nil
	case "medium_unschedule":
		st, err := m.Unschedule(ctx, a.Browser, a.Story)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Unscheduled %s %q; it is a draft again.", st.ID, st.Title), nil
	case "medium_delete":
		st, err := m.Delete(ctx, a.Browser, a.Story)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Deleted the draft %s %q.", st.ID, st.Title), nil
	}
	return "", fmt.Errorf("unknown tool %q", name)
}

func (s *Server) linkedin(ctx context.Context, name string, a args) (string, error) {
	li := s.LinkedIn
	switch name {
	case "linkedin_status":
		st, err := li.Status(ctx, a.Browser)
		if err != nil {
			return "", err
		}
		if !st.SignedIn {
			return "The browser is not signed in to LinkedIn.", nil
		}
		return "Signed in to LinkedIn as " + st.Name + ".", nil
	case "linkedin_draft":
		if a.File == "" {
			return "", errors.New("file is required")
		}
		art, err := linkedin.ParseFile(a.File)
		if err != nil {
			return "", err
		}
		if a.Cover != "" {
			if err := art.SetCover(a.Cover); err != nil {
				return "", err
			}
		}
		at, err := optionalTime(a.At)
		if err != nil {
			return "", err
		}
		st, err := li.Draft(ctx, linkedin.Draft{Browser: a.Browser, Article: art, At: at, Text: a.Text})
		if err != nil {
			if st != nil {
				return "", fmt.Errorf("%w (the draft so far: %s)", err, st.URL)
			}
			return "", err
		}
		out := fmt.Sprintf("Draft: %s %q", st.URL, st.Title)
		if st.Shown != "" {
			out += ", scheduled: " + st.Shown
		}
		return out, nil
	case "linkedin_drafts", "linkedin_scheduled":
		var list []linkedin.Story
		var err error
		if name == "linkedin_drafts" {
			list, err = li.Drafts(ctx, a.Browser)
		} else {
			list, err = li.Scheduled(ctx, a.Browser)
		}
		if err != nil {
			return "", err
		}
		if len(list) == 0 {
			return "None.", nil
		}
		var b strings.Builder
		for _, st := range list {
			if st.ID != "" {
				fmt.Fprintf(&b, "%s %q, %s, %s\n", st.ID, st.Title, st.Modified, st.URL)
			} else {
				fmt.Fprintf(&b, "%q, %s\n", st.Title, st.Shown)
			}
		}
		return b.String(), nil
	case "linkedin_get":
		c, err := li.Get(ctx, a.Browser, a.Story)
		if err != nil {
			return "", err
		}
		var b strings.Builder
		fmt.Fprintf(&b, "status: %s\ntitle: %s\ncover: %s\nlink: %s\n", c.Status, c.Title, c.Cover, c.URL)
		for i, im := range c.Images {
			fmt.Fprintf(&b, "image %d: %s alt=%q caption=%q\n", i+1, im.URL, im.Alt, im.Caption)
		}
		fmt.Fprintf(&b, "\n%s", c.Text)
		return b.String(), nil
	case "linkedin_schedule":
		at, err := linkedin.ParseTime(a.At)
		if err != nil {
			return "", err
		}
		st, err := li.Schedule(ctx, a.Browser, a.Story, at, a.Text)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Scheduled %q: %s.", st.Title, st.Shown), nil
	case "linkedin_unschedule":
		st, err := li.Unschedule(ctx, a.Browser, a.Match, a.When)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("A draft again: %s %q.", st.ID, st.Title), nil
	case "linkedin_delete":
		st, err := li.Delete(ctx, a.Browser, a.Story)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Deleted the draft %s %q.", st.ID, st.Title), nil
	}
	return "", fmt.Errorf("unknown tool %q", name)
}

func (s *Server) tiktok(ctx context.Context, name string, a args) (string, error) {
	tt := s.TikTok
	switch name {
	case "tiktok_status":
		st, err := tt.Status(ctx, a.Browser)
		if err != nil {
			return "", err
		}
		if !st.SignedIn {
			return "The browser is not signed in to TikTok.", nil
		}
		return "Signed in to TikTok as @" + st.Handle + ".", nil
	case "tiktok_post":
		var p *tiktok.Post
		switch {
		case a.File != "" && a.Video != "":
			return "", errors.New("give file or video, not both")
		case a.File != "":
			var err error
			if p, err = tiktok.ParseFile(a.File); err != nil {
				return "", err
			}
		case a.Video != "":
			p = &tiktok.Post{Video: a.Video, Caption: a.Caption}
		default:
			return "", errors.New("file or video is required")
		}
		if a.At != "" {
			at, err := tiktok.ParseTime(a.At)
			if err != nil {
				return "", err
			}
			p.At = at
		}
		p.Now, p.Browser = a.Now, a.Browser
		r, err := tt.Post(ctx, *p)
		if err != nil {
			return "", err
		}
		if r.Listed != "" {
			return "Taken by TikTok Studio: " + r.Listed, nil
		}
		return "Taken by TikTok Studio, for " + r.Visibility + ".", nil
	case "tiktok_posts":
		list, err := tt.Posts(ctx, a.Browser)
		if err != nil {
			return "", err
		}
		if len(list) == 0 {
			return "No posts.", nil
		}
		var b strings.Builder
		for _, l := range list {
			fmt.Fprintf(&b, "%s | %s | %s | %s\n", l.When, l.Duration, l.Privacy, l.Caption)
		}
		return b.String(), nil
	case "tiktok_delete":
		l, err := tt.Delete(ctx, a.Browser, a.Caption, a.When)
		if err != nil {
			return "", err
		}
		return "Deleted: " + l.Row, nil
	}
	return "", fmt.Errorf("unknown tool %q", name)
}

func (s *Server) instagram(ctx context.Context, name string, a args) (string, error) {
	ig := s.Instagram
	switch name {
	case "instagram_status":
		st, err := ig.Status(ctx, a.Browser)
		if err != nil {
			return "", err
		}
		if !st.SignedIn {
			return "The browser is not signed in to Instagram.", nil
		}
		who := "@" + st.Handle
		if st.Name != "" {
			who = st.Name + " (" + who + ")"
		}
		return "Signed in to Instagram as " + who + ".", nil
	case "instagram_post":
		var p *instagram.Post
		switch {
		case a.File != "" && len(a.Media) > 0:
			return "", errors.New("give file or media, not both")
		case a.File != "":
			var err error
			if p, err = instagram.ParseFile(a.File); err != nil {
				return "", err
			}
		case len(a.Media) > 0:
			p = &instagram.Post{Caption: a.Caption}
			for _, f := range a.Media {
				p.Media = append(p.Media, instagram.Media{File: f})
			}
		default:
			return "", errors.New("file or media is required")
		}
		if a.At != "" {
			at, err := instagram.ParseTime(a.At)
			if err != nil {
				return "", err
			}
			p.At = at
		}
		p.Now, p.Rehearse, p.Browser = a.Now, a.Rehearse, a.Browser
		p.AILabel = p.AILabel || a.AILabel
		r, err := ig.Post(ctx, *p)
		if err != nil {
			return "", err
		}
		switch {
		case p.Rehearse && r.Scheduled:
			return "Rehearsed a scheduled " + r.Kind + " in Meta Business Suite: every step but Jadwalkan; nothing was scheduled.", nil
		case r.Scheduled:
			return "Scheduled a " + r.Kind + " in Meta Business Suite for " + r.At.Format("Mon 2 Jan 2006 15:04 MST") + ".", nil
		case p.Rehearse:
			return "Rehearsed a " + r.Kind + ": every step but Share; nothing was shared.", nil
		case r.Link != "":
			return "Shared a " + r.Kind + ": " + r.Link, nil
		}
		return "Shared a " + r.Kind + "; the profile does not show it yet.", nil
	case "instagram_delete":
		l, err := ig.Delete(ctx, a.Browser, a.Post)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Deleted %s (%s): %s", l.Link, l.At.Format(time.RFC3339), l.Caption), nil
	case "instagram_delete_scheduled":
		var at time.Time
		if a.At != "" {
			var err error
			if at, err = instagram.ParseTime(a.At); err != nil {
				return "", err
			}
		}
		l, err := ig.DeleteScheduled(ctx, a.Browser, a.Caption, at)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Deleted the post scheduled %s: %s", l.When, l.Caption), nil
	case "instagram_scheduled":
		list, err := ig.Scheduled(ctx, a.Browser)
		if err != nil {
			return "", err
		}
		if len(list) == 0 {
			return "Nothing is scheduled.", nil
		}
		var b strings.Builder
		for _, l := range list {
			when := l.When
			if !l.At.IsZero() {
				when = l.At.Format(time.RFC3339)
			}
			fmt.Fprintf(&b, "%s | %s | %s\n", when, l.Kind, l.Caption)
		}
		return b.String(), nil
	case "instagram_posts":
		list, err := ig.Posts(ctx, a.Browser, int(a.Limit))
		if err != nil {
			return "", err
		}
		if len(list) == 0 {
			return "No posts.", nil
		}
		var b strings.Builder
		for _, l := range list {
			kind := l.Kind
			if l.Pinned {
				kind += " (pinned)"
			}
			fmt.Fprintf(&b, "%s | %s | %s | %s\n", l.At.Format(time.RFC3339), kind, l.Link, l.Caption)
		}
		return b.String(), nil
	}
	return "", fmt.Errorf("unknown tool %q", name)
}

func (s *Server) youtube(ctx context.Context, name string, a args) (string, error) {
	yt := s.YouTube
	switch name {
	case "youtube_status":
		st, err := yt.Status(ctx, a.Browser)
		if err != nil {
			return "", err
		}
		if !st.SignedIn {
			return "The browser is not signed in to YouTube.", nil
		}
		who := st.Name
		if st.Handle != "" {
			who += " (@" + st.Handle + ")"
		}
		return "Signed in to YouTube as " + strings.TrimSpace(who) + ".", nil
	case "youtube_upload":
		var p *youtube.Post
		switch {
		case a.File != "" && a.Video != "":
			return "", errors.New("give file or video, not both")
		case a.File != "":
			var err error
			if p, err = youtube.ParseFile(a.File); err != nil {
				return "", err
			}
		case a.Video != "":
			p = &youtube.Post{Video: a.Video, Title: a.Title, Description: a.Description}
		default:
			return "", errors.New("file or video is required")
		}
		if tags := youtube.SplitTags(a.Tags...); len(tags) > 0 {
			p.Tags = tags
		}
		if a.At != "" {
			at, err := youtube.ParseTime(a.At)
			if err != nil {
				return "", err
			}
			p.At = at
		}
		p.Short = p.Short || a.Short
		p.Browser = a.Browser
		v, err := yt.Upload(ctx, *p)
		if err != nil {
			if v != nil {
				return "", fmt.Errorf("%w (Studio made video %s: %s; finish or delete it there)", err, v.ID, v.Studio)
			}
			return "", err
		}
		return fmt.Sprintf("Uploaded: %s, %s %s (edit: %s)", v.URL, v.Visibility, v.Date, v.Studio), nil
	case "youtube_private":
		l, err := yt.MakePrivate(ctx, a.Browser, a.Video)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Private: %s (%s)", l.URL(), l.Title), nil
	case "youtube_delete":
		l, err := yt.Delete(ctx, a.Browser, a.Video, a.AllowPublic)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Deleted: %s (%s, %s)", l.URL(), l.Title, l.Visibility), nil
	case "youtube_videos":
		list, err := yt.Videos(ctx, a.Browser)
		if err != nil {
			return "", err
		}
		if len(list) == 0 {
			return "No videos.", nil
		}
		var b strings.Builder
		for _, l := range list {
			kind := "video"
			if l.Short {
				kind = "short"
			}
			fmt.Fprintf(&b, "%s | %s | %s | %s | %s\n", kind, l.Visibility, l.Date, l.URL(), l.Title)
		}
		return b.String(), nil
	}
	return "", fmt.Errorf("unknown tool %q", name)
}

func (s *Server) telegram(ctx context.Context, name string, a args) (string, error) {
	tg := s.Telegram
	switch name {
	case "telegram_status":
		st, err := tg.Status(ctx, a.Browser)
		if err != nil {
			return "", err
		}
		if !st.SignedIn {
			return "The browser is not signed in to Telegram Web.", nil
		}
		who := st.Name
		if st.Username != "" {
			who += " (@" + st.Username + ")"
		}
		return "Signed in to Telegram Web as " + strings.TrimSpace(who) + ".", nil
	case "telegram_create_bot":
		b, err := tg.CreateBot(ctx, a.Browser, a.Name, a.Username)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Made bot @%s (%s), %s. Its token (secret): %s", b.Username, b.Name, b.Link, b.Token), nil
	case "telegram_delete_bot":
		if err := tg.DeleteBot(ctx, a.Browser, a.Username); err != nil {
			return "", err
		}
		return "Deleted bot @" + strings.TrimPrefix(strings.TrimSpace(a.Username), "@") + ".", nil
	case "telegram_create_group":
		g, err := tg.CreateGroup(ctx, a.Browser, a.Title)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Made group %q, id %s.", g.Title, g.ID), nil
	case "telegram_delete_group":
		g, err := tg.DeleteGroup(ctx, a.Browser, a.Group)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Deleted group %q (%s) for all its members.", g.Title, g.ID), nil
	case "telegram_add_member":
		g, m, err := tg.AddMember(ctx, a.Browser, a.Group, a.User)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("@%s (%s) is a member of %q (%s).", m.Username, m.Name, g.Title, g.ID), nil
	case "telegram_remove_member":
		g, m, err := tg.RemoveMember(ctx, a.Browser, a.Group, a.User)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Removed @%s (%s) from %q (%s).", m.Username, m.Name, g.Title, g.ID), nil
	}
	return "", fmt.Errorf("unknown tool %q", name)
}

func (s *Server) chatgpt(ctx context.Context, name string, a args) (string, error) {
	switch name {
	case "chatgpt_status":
		st, err := s.ChatGPT.Status(ctx, a.Browser)
		if err != nil {
			return "", err
		}
		if !st.SignedIn {
			return "The browser is not signed in to ChatGPT.", nil
		}
		return fmt.Sprintf("Signed in to ChatGPT as %s (%s).", st.Name, st.Plan), nil
	case "chatgpt_ask":
		ans, err := s.ChatGPT.Ask(ctx, chatgpt.Ask{
			Browser: a.Browser, Prompt: a.Prompt, Chat: a.Chat, Keep: a.Keep, Wait: time.Duration(a.WaitSeconds) * time.Second,
		})
		if err != nil {
			return "", err
		}
		if ans.Chat != "" {
			return ans.Text + "\n\n(chat: " + ans.Chat + ")", nil
		}
		return ans.Text, nil
	}
	return "", fmt.Errorf("unknown tool %q", name)
}

func (s *Server) claude(ctx context.Context, name string, a args) (string, error) {
	switch name {
	case "claude_status":
		st, err := s.Claude.Status(ctx, a.Browser)
		if err != nil {
			return "", err
		}
		if !st.SignedIn {
			return "The browser is not signed in to claude.ai.", nil
		}
		return fmt.Sprintf("Signed in to claude.ai as %s (%s).", st.Name, st.Plan), nil
	case "claude_ask":
		ans, err := s.Claude.Ask(ctx, claude.Ask{
			Browser: a.Browser, Prompt: a.Prompt, Chat: a.Chat, Keep: a.Keep, Wait: time.Duration(a.WaitSeconds) * time.Second,
		})
		if err != nil {
			return "", err
		}
		if ans.Chat != "" {
			return ans.Text + "\n\n(chat: " + ans.Chat + ")", nil
		}
		return ans.Text, nil
	}
	return "", fmt.Errorf("unknown tool %q", name)
}

func (s *Server) gemini(ctx context.Context, name string, a args) (string, error) {
	switch name {
	case "gemini_status":
		st, err := s.Gemini.Status(ctx, a.Browser)
		if err != nil {
			return "", err
		}
		if !st.SignedIn {
			return "The browser is not signed in to Gemini.", nil
		}
		return "Signed in to Gemini as " + st.Name + ".", nil
	case "gemini_ask":
		ans, err := s.Gemini.Ask(ctx, gemini.Ask{
			Browser: a.Browser, Prompt: a.Prompt, Chat: a.Chat, Keep: a.Keep, Wait: time.Duration(a.WaitSeconds) * time.Second,
		})
		if err != nil {
			return "", err
		}
		if ans.Chat != "" {
			return ans.Text + "\n\n(chat: " + ans.Chat + ")", nil
		}
		return ans.Text, nil
	}
	return "", fmt.Errorf("unknown tool %q", name)
}
