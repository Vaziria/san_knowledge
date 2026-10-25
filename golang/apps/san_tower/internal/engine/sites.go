package engine

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"connectrpc.com/connect"
	chatgptv1 "github.com/wargasipil/san_remote/gen/san/remote/chatgpt/v1"
	claudev1 "github.com/wargasipil/san_remote/gen/san/remote/claude/v1"
	geminiv1 "github.com/wargasipil/san_remote/gen/san/remote/gemini/v1"
	linkedinv1 "github.com/wargasipil/san_remote/gen/san/remote/linkedin/v1"
	mediumv1 "github.com/wargasipil/san_remote/gen/san/remote/medium/v1"
	tiktokv1 "github.com/wargasipil/san_remote/gen/san/remote/tiktok/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
)

// The san_remote steps for Medium, LinkedIn, TikTok and ChatGPT (the user,
// 2026-10-25: "make san_medium, san_linkedin, san_tiktok have browser remote
// version in san_remote and have node in tower", then "san_gpt too"). Like
// the others they go through `san_remote api` at a San Remote connection.
// Articles become drafts, and are scheduled only when At is given; a TikTok
// video is only scheduled. ChatGPT asks in a Temporary chat (the user's
// choice), which ChatGPT does not keep; Gemini too, in a tab brought to the
// front, since Gemini sends only from a shown page; Claude (claude.ai) in an
// incognito chat. Claude Ask (CLI) is claude.go's.

var siteKinds = []*kind{
	{
		StepKind: &v1.StepKind{
			Kind: "medium-draft", Title: "Medium Article", Category: "Social",
			Group:       "Medium",
			Description: "Writes a markdown article, a File or the Article text (its images by absolute paths), into a Medium draft through San Remote, images included; with At it is scheduled too, with Topics. Link is the draft's.",
			Outputs:     []*v1.Socket{sock("Done", Event), sock("Story", Object), sock("Link", Text)},
			Settings:    []*v1.Setting{connectionSetting(), {Name: "Topics", Placeholder: "Programming, Golang"}, articleSource()},
			Inputs:      []*v1.Socket{sock("Event", Event), only("Source=File", field("File")), only("Source=Text", field("Article")), atPicker()},
		},
		run: func(c *call) error {
			r, err := c.sanRemote()
			if err != nil {
				return err
			}
			file, md, err := c.articleOf()
			if err != nil {
				return err
			}
			at, err := c.optionalAt(minuteGrid)
			if err != nil {
				return err
			}
			res, err := r.medium.CreateDraft(c.ctx, connect.NewRequest(&mediumv1.CreateDraftRequest{File: file, Markdown: md, At: at, Topics: list(c.setting("Topics"))}))
			if err != nil {
				return remoteErr(err)
			}
			c.outputs["Story"] = toValue(res.Msg.Story)
			c.outputs["Link"] = stringValue(res.Msg.Story.GetUrl())
			c.fire("Done")
			return nil
		},
	},
	{
		StepKind: &v1.StepKind{
			Kind: "linkedin-draft", Title: "LinkedIn Article", Category: "Social",
			Group:       "LinkedIn",
			Description: "Writes a markdown article, a File or the Article text (its images by absolute paths), into a LinkedIn article draft through San Remote, with an optional Cover; with At it is scheduled too, shared with Text. Link is the draft's.",
			Outputs:     []*v1.Socket{sock("Done", Event), sock("Article", Object), sock("Link", Text)},
			Settings:    []*v1.Setting{connectionSetting(), articleSource()},
			Inputs:      []*v1.Socket{sock("Event", Event), only("Source=File", field("File")), only("Source=Text", field("Article")), field("Cover"), field("Text"), atPicker()},
		},
		run: func(c *call) error {
			r, err := c.sanRemote()
			if err != nil {
				return err
			}
			file, md, err := c.articleOf()
			if err != nil {
				return err
			}
			cover := strings.TrimSpace(c.text("Cover"))
			if cover != "" && !filepath.IsAbs(cover) {
				return fmt.Errorf("Cover %q: give the image's absolute path, such as File Path's Absolute Path", cover)
			}
			at, err := c.optionalAt(quarterHour)
			if err != nil {
				return err
			}
			res, err := r.linkedin.CreateDraft(c.ctx, connect.NewRequest(&linkedinv1.CreateDraftRequest{File: file, Markdown: md, Cover: cover, At: at, Text: strings.TrimSpace(c.text("Text"))}))
			if err != nil {
				return remoteErr(err)
			}
			c.outputs["Article"] = toValue(res.Msg.Article)
			c.outputs["Link"] = stringValue(res.Msg.Article.GetUrl())
			c.fire("Done")
			return nil
		},
	},
	{
		StepKind: &v1.StepKind{
			Kind: "tiktok-post", Title: "TikTok Post", Category: "Social",
			Group:       "TikTok",
			Description: "Schedules a video file on TikTok for Everyone through San Remote: Video with Caption or a File at At (on a 5-minute step, up to 30 days ahead), as Plain text or Markdown (a post file: frontmatter video: and at:, the markup written as plain text; Video over the frontmatter's).",
			Outputs:     []*v1.Socket{sock("Done", Event), sock("Result", Object)},
			Settings:    append([]*v1.Setting{connectionSetting()}, formatSettings()...),
			Inputs:      slices.Insert(postInputs("Caption"), 1, field("Video")),
		},
		run: func(c *call) error {
			r, at, p, err := c.postArgs(fiveMinutes, "Caption", false)
			if err != nil {
				return err
			}
			// Markdown may name the video in its frontmatter (video:).
			video := strings.TrimSpace(c.text("Video"))
			if video != "" || !p.isMarkdown() {
				if video, err = c.absolute("Video", "the video file"); err != nil {
					return err
				}
			}
			res, err := r.tiktok.SchedulePost(c.ctx, connect.NewRequest(&tiktokv1.SchedulePostRequest{
				Video: video, Caption: strings.TrimSpace(p.plain), File: p.file, Markdown: p.markdown, At: at,
			}))
			if err != nil {
				return remoteErr(err)
			}
			c.outputs["Result"] = toValue(res.Msg)
			c.fire("Done")
			return nil
		},
	},
	{
		StepKind: &v1.StepKind{
			Kind: "chatgpt-ask", Title: "ChatGPT Ask", Category: "AI",
			Group:       "ChatGPT",
			Description: "Asks ChatGPT the Prompt through San Remote, in a Temporary chat that ChatGPT does not keep, or with Temporary No a normal chat kept in the history; Answer is its reply as markdown. Chat: Leave closes the tab afterwards, Stay leaves it open on the chat.",
			Outputs:     []*v1.Socket{sock("Done", Event), sock("Answer", Text)},
			Settings:    askSettings(),
			Inputs:      []*v1.Socket{sock("Event", Event), field("Prompt")},
		},
		run: func(c *call) error {
			r, err := c.sanRemote()
			if err != nil {
				return err
			}
			prompt, wait, err := c.askArgs()
			if err != nil {
				return err
			}
			res, err := r.chatgpt.Ask(c.ctx, connect.NewRequest(&chatgptv1.AskRequest{Prompt: prompt, WaitSeconds: uint32(wait / time.Second), Keep: c.setting("Temporary") == "No", Stay: c.setting("Chat") == "Stay"}))
			if err != nil {
				return remoteErr(err)
			}
			c.outputs["Answer"] = stringValue(res.Msg.Text)
			c.fire("Done")
			return nil
		},
	},
	{
		StepKind: &v1.StepKind{
			Kind: "gemini-ask", Title: "Gemini Ask", Category: "AI",
			Group:       "Gemini",
			Description: "Asks Gemini the Prompt through San Remote, in a Temporary chat that Gemini does not keep, or with Temporary No a normal chat kept in the history; Answer is its reply. Brings Chrome to the front while it asks. Chat: Leave closes the tab afterwards, Stay leaves it open on the chat.",
			Outputs:     []*v1.Socket{sock("Done", Event), sock("Answer", Text)},
			Settings:    askSettings(),
			Inputs:      []*v1.Socket{sock("Event", Event), field("Prompt")},
		},
		run: func(c *call) error {
			r, err := c.sanRemote()
			if err != nil {
				return err
			}
			prompt, wait, err := c.askArgs()
			if err != nil {
				return err
			}
			res, err := r.gemini.Ask(c.ctx, connect.NewRequest(&geminiv1.AskRequest{Prompt: prompt, WaitSeconds: uint32(wait / time.Second), Keep: c.setting("Temporary") == "No", Stay: c.setting("Chat") == "Stay"}))
			if err != nil {
				return remoteErr(err)
			}
			c.outputs["Answer"] = stringValue(res.Msg.Text)
			c.fire("Done")
			return nil
		},
	},
	{
		StepKind: &v1.StepKind{
			Kind: "claude-ask-browser", Title: "Claude Ask (Browser)", Category: "AI",
			Group:       "Claude",
			Description: "Asks Claude on claude.ai the Prompt through San Remote, in an incognito chat kept out of the history, or with Temporary No a normal chat kept in it; Answer is its reply as markdown. Chat: Leave closes the tab afterwards, Stay leaves it open on the chat.",
			Outputs:     []*v1.Socket{sock("Done", Event), sock("Answer", Text)},
			Settings:    askSettings(),
			Inputs:      []*v1.Socket{sock("Event", Event), field("Prompt")},
		},
		run: func(c *call) error {
			r, err := c.sanRemote()
			if err != nil {
				return err
			}
			prompt, wait, err := c.askArgs()
			if err != nil {
				return err
			}
			res, err := r.claude.Ask(c.ctx, connect.NewRequest(&claudev1.AskRequest{Prompt: prompt, WaitSeconds: uint32(wait / time.Second), Keep: c.setting("Temporary") == "No", Stay: c.setting("Chat") == "Stay"}))
			if err != nil {
				return remoteErr(err)
			}
			c.outputs["Answer"] = stringValue(res.Msg.Text)
			c.fire("Done")
			return nil
		},
	},
}

// statusSites are the sites a Social Status step reads: the posting sites,
// ChatGPT, Gemini and Claude (claude.ai).
var statusSites = append(slices.Clone(sites), "ChatGPT", "Gemini", "Claude")

// askSettings are a browser ask step's: its San Remote connection, Wait,
// Temporary, whether the chat is a Temporary (incognito on claude.ai) one
// the site does not keep (Yes) or a normal one kept in the history (No),
// and Chat, whether the tab is closed when the ask is over (Leave) or left
// open on the chat, to read on or go on in it (Stay; the user, 2026-10-25).
func askSettings() []*v1.Setting {
	return []*v1.Setting{
		connectionSetting(),
		{Name: "Wait", Placeholder: "10m: how long the answer may take"},
		{Name: "Temporary", Options: []string{"Yes", "No"}},
		{Name: "Chat", Options: []string{"Leave", "Stay"}},
	}
}

// askArgs reads an ask step's Prompt and its Wait (0 when not set: the
// service's own default).
func (c *call) askArgs() (string, time.Duration, error) {
	prompt := strings.TrimSpace(c.text("Prompt"))
	if prompt == "" {
		return "", 0, fmt.Errorf("Prompt %w", errEmpty)
	}
	var wait time.Duration
	if w := c.setting("Wait"); w != "" {
		var err error
		if wait, err = time.ParseDuration(w); err != nil || wait <= 0 {
			return "", 0, fmt.Errorf("Wait %q: give a duration such as 10m or 90s", w)
		}
	}
	return prompt, wait, nil
}

// absolute is a file input that must be an absolute path: `san_remote api`
// may run in another folder than the tower.
func (c *call) absolute(input, what string) (string, error) {
	path := strings.TrimSpace(c.text(input))
	if path == "" {
		return "", fmt.Errorf("%s %w: give %s's path on this computer", input, errEmpty, what)
	}
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("%s %q: give %s's absolute path, such as File Path's Absolute Path", input, path, what)
	}
	return path, nil
}

// articleSource is an article step's Source: its markdown File, or the
// Article text (the user, 2026-10-25).
func articleSource() *v1.Setting {
	return &v1.Setting{Name: "Source", Options: []string{"File", "Text"}}
}

// articleOf reads an article step's markdown by its Source: File, an
// absolute path, or the Article text, whose images need absolute paths.
func (c *call) articleOf() (file, markdown string, err error) {
	if c.setting("Source") == "Text" {
		md := c.text("Article")
		if strings.TrimSpace(md) == "" {
			return "", "", fmt.Errorf("Article %w: give the article as markdown", errEmpty)
		}
		return "", md, nil
	}
	file, err = c.absolute("File", "the article's markdown file")
	return file, "", err
}

// optionalAt reads an At that may be empty, which keeps an article a draft,
// on the site's grid.
func (c *call) optionalAt(grid time.Duration) (*timestamppb.Timestamp, error) {
	if strings.TrimSpace(c.text("At")) == "" {
		return nil, nil
	}
	t, err := when(c.text("At"), time.Now(), grid)
	if err != nil {
		return nil, err
	}
	return timestamppb.New(t), nil
}

// list splits a setting of comma-separated values.
func list(s string) []string {
	var out []string
	for _, v := range strings.Split(s, ",") {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// siteStatus reads who is signed in on one of the sites of this file.
func (c *call) siteStatus(r remoteClients, site string) (proto.Message, error) {
	switch site {
	case "Medium":
		res, err := r.medium.GetStatus(c.ctx, connect.NewRequest(&mediumv1.GetStatusRequest{}))
		if err != nil {
			return nil, err
		}
		return res.Msg.GetStatus(), nil
	case "LinkedIn":
		res, err := r.linkedin.GetStatus(c.ctx, connect.NewRequest(&linkedinv1.GetStatusRequest{}))
		if err != nil {
			return nil, err
		}
		return res.Msg.GetStatus(), nil
	case "TikTok":
		res, err := r.tiktok.GetStatus(c.ctx, connect.NewRequest(&tiktokv1.GetStatusRequest{}))
		if err != nil {
			return nil, err
		}
		return res.Msg.GetStatus(), nil
	case "ChatGPT":
		res, err := r.chatgpt.GetStatus(c.ctx, connect.NewRequest(&chatgptv1.GetStatusRequest{}))
		if err != nil {
			return nil, err
		}
		return res.Msg.GetStatus(), nil
	case "Gemini":
		res, err := r.gemini.GetStatus(c.ctx, connect.NewRequest(&geminiv1.GetStatusRequest{}))
		if err != nil {
			return nil, err
		}
		return res.Msg.GetStatus(), nil
	case "Claude":
		res, err := r.claude.GetStatus(c.ctx, connect.NewRequest(&claudev1.GetStatusRequest{}))
		if err != nil {
			return nil, err
		}
		return res.Msg.GetStatus(), nil
	}
	return nil, fmt.Errorf("Site %q: give one of %s", site, strings.Join(statusSites, ", "))
}

// errNoList is a list a site does not have.
var errNoList = errors.New("has no such list here")

// listSitePosts lists one of the sites of this file: Medium's and LinkedIn's
// scheduled stories or drafts, and TikTok's scheduled or published posts
// (Studio lists both together; the scheduled ones are those still ahead).
func (c *call) listSitePosts(r remoteClients, site, kind string) ([]proto.Message, func(i int) time.Time, error) {
	var items []proto.Message
	var at []time.Time
	switch site + "/" + kind {
	case "Medium/Scheduled", "Medium/Drafts":
		var stories []*mediumv1.Story
		if kind == "Scheduled" {
			res, err := r.medium.ListScheduled(c.ctx, connect.NewRequest(&mediumv1.ListScheduledRequest{}))
			if err != nil {
				return nil, nil, err
			}
			stories = res.Msg.Stories
		} else {
			res, err := r.medium.ListDrafts(c.ctx, connect.NewRequest(&mediumv1.ListDraftsRequest{}))
			if err != nil {
				return nil, nil, err
			}
			stories = res.Msg.Stories
		}
		for _, s := range stories {
			items, at = append(items, s), append(at, s.GetAt().AsTime())
		}
	case "LinkedIn/Scheduled", "LinkedIn/Drafts":
		var arts []*linkedinv1.Article
		if kind == "Scheduled" {
			res, err := r.linkedin.ListScheduled(c.ctx, connect.NewRequest(&linkedinv1.ListScheduledRequest{}))
			if err != nil {
				return nil, nil, err
			}
			arts = res.Msg.Articles
		} else {
			res, err := r.linkedin.ListDrafts(c.ctx, connect.NewRequest(&linkedinv1.ListDraftsRequest{}))
			if err != nil {
				return nil, nil, err
			}
			arts = res.Msg.Articles
		}
		for _, a := range arts {
			items, at = append(items, a), append(at, a.GetAt().AsTime())
		}
	case "TikTok/Scheduled", "TikTok/Published":
		res, err := r.tiktok.ListPosts(c.ctx, connect.NewRequest(&tiktokv1.ListPostsRequest{}))
		if err != nil {
			return nil, nil, err
		}
		now := time.Now()
		for _, p := range res.Msg.Posts {
			if t := p.GetAt().AsTime(); (kind == "Scheduled") == t.After(now) {
				items, at = append(items, p), append(at, t)
			}
		}
	default:
		return nil, nil, fmt.Errorf("%s %w (%s)", site, errNoList, kind)
	}
	if kind == "Drafts" {
		// Drafts have no time: the first listed is the newest.
		return items, nil, nil
	}
	return items, func(i int) time.Time { return at[i] }, nil
}

// postText is a listed post's text: its text, an article's title, or a
// video's caption.
func postText(v *structpb.Value) string {
	f := v.GetStructValue().GetFields()
	for _, k := range []string{"text", "title", "caption"} {
		if s := f[k].GetStringValue(); s != "" {
			return s
		}
	}
	return ""
}
