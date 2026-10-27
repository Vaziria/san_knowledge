package engine

import (
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/wargasipil/san_remote/gen/san/remote/chatgpt/v1/chatgptv1connect"
	"github.com/wargasipil/san_remote/gen/san/remote/claude/v1/claudev1connect"
	facebookv1 "github.com/wargasipil/san_remote/gen/san/remote/facebook/v1"
	"github.com/wargasipil/san_remote/gen/san/remote/facebook/v1/facebookv1connect"
	"github.com/wargasipil/san_remote/gen/san/remote/gemini/v1/geminiv1connect"
	"github.com/wargasipil/san_remote/gen/san/remote/linkedin/v1/linkedinv1connect"
	"github.com/wargasipil/san_remote/gen/san/remote/medium/v1/mediumv1connect"
	"github.com/wargasipil/san_remote/gen/san/remote/telegram/v1/telegramv1connect"
	threadsv1 "github.com/wargasipil/san_remote/gen/san/remote/threads/v1"
	"github.com/wargasipil/san_remote/gen/san/remote/threads/v1/threadsv1connect"
	"github.com/wargasipil/san_remote/gen/san/remote/tiktok/v1/tiktokv1connect"
	vidarav1 "github.com/wargasipil/san_remote/gen/san/remote/vidara/v1"
	"github.com/wargasipil/san_remote/gen/san/remote/vidara/v1/vidarav1connect"
	xv1 "github.com/wargasipil/san_remote/gen/san/remote/x/v1"
	"github.com/wargasipil/san_remote/gen/san/remote/x/v1/xv1connect"
	"github.com/wargasipil/san_remote/gen/san/remote/youtube/v1/youtubev1connect"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/store"
)

// The san_remote steps post, upload and read on the user's Threads, X,
// Facebook and Vidara (and Medium, LinkedIn, TikTok and ChatGPT: sites.go)
// through `san_remote api`, at the URL of the connection
// they name (a san_remote connection). Posts are only scheduled (the user's
// choice, 2026-10-24): At is a time, picked or linked (Now, Datetime
// Operation), or a delay from the run linked as text ("+1d"). A post or an
// upload works in a tab of the user's browser brought to the front, for a
// minute or two.

// Sites a Social Posts step reads; a Social Status step reads them and
// ChatGPT (statusSites, sites.go).
var sites = []string{"Threads", "X", "Facebook", "Vidara", "Medium", "LinkedIn", "TikTok", "YouTube"}

// The times each site's scheduler offers, onto which At is rounded up:
// Threads, X and Medium take any minute, Facebook and TikTok 5-minute marks,
// LinkedIn quarter hours.
const (
	minuteGrid  = time.Minute
	fiveMinutes = 5 * time.Minute
	quarterHour = 15 * time.Minute
)

func connectionSetting() *v1.Setting {
	return &v1.Setting{Name: "Connection", ConnectionType: store.SanRemote}
}

// atPicker is a post's At: a datetime picker in the editor (the user,
// 2026-10-25), a text socket all the same.
func atPicker() *v1.Socket { return picker("At", "datetime") }

var socialKinds = []*kind{
	{
		StepKind: &v1.StepKind{
			Kind: "threads-post", Title: "Threads Post", Category: "Social",
			Group:       "Threads",
			Description: "Schedules a thread on Threads through San Remote at At: Text or a File, posts split by ---, as Plain text or Markdown (" + markdownNote + ", Topic over the frontmatter's).",
			Outputs:     []*v1.Socket{sock("Done", Event), sock("Result", Object)},
			Settings:    append([]*v1.Setting{connectionSetting(), {Name: "Topic", Placeholder: "one of Threads' suggestions"}}, formatSettings()...),
			Inputs:      postInputs("Text"),
		},
		run: func(c *call) error {
			r, at, p, err := c.postArgs(minuteGrid, "Text", true)
			if err != nil {
				return err
			}
			req := &threadsv1.SchedulePostRequest{At: at, File: p.file, Markdown: p.markdown}
			if p.isMarkdown() {
				req.Topic = c.setting("Topic")
			}
			for i, t := range posts(p.plain) {
				p := &threadsv1.Post{Text: t}
				if i == 0 {
					p.Topic = c.setting("Topic")
				}
				req.Posts = append(req.Posts, p)
			}
			res, err := r.threads.SchedulePost(c.ctx, connect.NewRequest(req))
			if err != nil {
				return remoteErr(err)
			}
			c.outputs["Result"] = toValue(res.Msg.Result)
			c.fire("Done")
			return nil
		},
	},
	{
		StepKind: &v1.StepKind{
			Kind: "x-post", Title: "X Post", Category: "Social",
			Group:       "X",
			Description: "Schedules a post or thread on X through San Remote at At, posts split by ---: as Plain text, a Caption (or a File) with up to 4 Images or a Video on its first post, or as Markdown, Text or a File (" + markdownNote + ").",
			Outputs:     []*v1.Socket{sock("Done", Event), sock("Result", Object)},
			Settings:    append([]*v1.Setting{connectionSetting()}, formatSettings()...),
			Inputs:      xPostInputs(),
		},
		run: xPost,
	},
	{
		StepKind: &v1.StepKind{
			Kind: "x-download", Title: "X Post Download", Category: "Social",
			Group:       "X",
			Description: "Reads any X post from its Link through San Remote and saves its images and videos (an X Article's cover and images too) in Save To: Caption, Text (an article as markdown), Markdown (the post with its saved media as ![](file), for a post step's Format Markdown or an article), Title, Tags, Media and First.",
			Outputs: []*v1.Socket{sock("Done", Event), sock("Caption", Text), sock("Text", Text), sock("Markdown", Text), sock("Title", Text), sock("Tags", Text),
				sock("Media", Object), sock("First", Text), sock("Post", Object)},
			Settings: []*v1.Setting{connectionSetting(), {Name: "Storage", Storage: true, Placeholder: "No storage: the data folder"}},
			Inputs:   []*v1.Socket{sock("Event", Event), field("Link"), field("Save To")},
		},
		run: xDownload,
	},
	{
		StepKind: &v1.StepKind{
			Kind: "facebook-post", Title: "Facebook Post", Category: "Social",
			Group:       "Facebook",
			Description: "Schedules a post on the Facebook profile or a Page through San Remote, with an audience, at At: Text or a File, as Plain text or Markdown (" + markdownNote + ", To and Audience over the frontmatter's).",
			Outputs:     []*v1.Socket{sock("Done", Event), sock("Result", Object)},
			Settings: append([]*v1.Setting{
				connectionSetting(),
				{Name: "To", Placeholder: "profile, or page: <name>"},
				{Name: "Audience", Options: []string{"default", "public", "friends", "only_me"}},
			}, formatSettings()...),
			Inputs: postInputs("Text"),
		},
		run: func(c *call) error {
			r, at, p, err := c.postArgs(fiveMinutes, "Text", true)
			if err != nil {
				return err
			}
			audience := c.setting("Audience")
			if audience == "default" {
				audience = ""
			}
			res, err := r.facebook.SchedulePost(c.ctx, connect.NewRequest(&facebookv1.SchedulePostRequest{
				To: c.setting("To"), Audience: audience, Text: strings.TrimSpace(p.plain), File: p.file, Markdown: p.markdown, At: at,
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
			Kind: "vidara-upload", Title: "Vidara Upload", Category: "Social",
			Group:       "Vidara",
			Description: "Uploads a video file to Vidara through San Remote, into Folder (a folder's name, or Promo/2026 for one in a folder; a top-level one that is not there is made) or the root; Link is the uploaded video's link (https://vidara.to/v/<code>), Video what My videos lists of it.",
			Outputs:     []*v1.Socket{sock("Done", Event), sock("Link", Text), sock("Video", Object)},
			Settings:    []*v1.Setting{connectionSetting()},
			Inputs:      []*v1.Socket{sock("Event", Event), field("File"), field("Title"), field("Folder")},
		},
		run: func(c *call) error {
			r, err := c.sanRemote()
			if err != nil {
				return err
			}
			file := strings.TrimSpace(c.text("File"))
			if file == "" {
				return fmt.Errorf("File %w: give the video's path on this computer", errEmpty)
			}
			res, err := r.vidara.UploadVideo(c.ctx, connect.NewRequest(&vidarav1.UploadVideoRequest{
				File: file, Title: strings.TrimSpace(c.text("Title")), Folder: strings.TrimSpace(c.text("Folder")),
			}))
			if err != nil {
				return remoteErr(err)
			}
			// The link is the one My videos shows; a row without one has
			// the video's code.
			v := res.Msg.Video
			link := v.GetUrl()
			if link == "" {
				link = "https://vidara.to/v/" + v.GetCode()
			}
			c.outputs["Link"] = stringValue(link)
			c.outputs["Video"] = toValue(v)
			c.fire("Done")
			return nil
		},
	},
	{
		StepKind: &v1.StepKind{
			Kind: "vidara-download", Title: "Vidara Download", Category: "Social",
			Group:       "Vidara",
			Description: "Downloads a Vidara video, by its link, to a folder on this computer through San Remote; File is where it went.",
			Outputs:     []*v1.Socket{sock("Done", Event), sock("File", Text), sock("Result", Object)},
			Settings:    []*v1.Setting{connectionSetting()},
			Inputs:      []*v1.Socket{sock("Event", Event), field("Link"), field("Save To")},
		},
		run: func(c *call) error {
			r, err := c.sanRemote()
			if err != nil {
				return err
			}
			link := strings.TrimSpace(c.text("Link"))
			if link == "" {
				return fmt.Errorf("Link %w: give the video's link (https://vidara.to/v/...) or its code", errEmpty)
			}
			out := strings.TrimSpace(c.text("Save To"))
			if out == "" {
				return fmt.Errorf(`Save To %w: give a folder on this computer, such as D:\videos, or a file`, errEmpty)
			}
			res, err := r.vidara.DownloadVideo(c.ctx, connect.NewRequest(&vidarav1.DownloadVideoRequest{Link: link, Out: out}))
			if err != nil {
				return remoteErr(err)
			}
			c.outputs["File"] = stringValue(res.Msg.Path)
			c.outputs["Result"] = toValue(res.Msg)
			c.fire("Done")
			return nil
		},
	},
	{
		StepKind: &v1.StepKind{
			Kind: "social-status", Title: "Social Status", Category: "Social",
			Description: "Reads who is signed in on a site through San Remote, and their Pages and groups.",
			Outputs:     []*v1.Socket{sock("Done", Event), sock("Status", Object)},
			Settings:    []*v1.Setting{connectionSetting(), {Name: "Site", Options: statusSites}},
			Inputs:      []*v1.Socket{sock("Event", Event)},
		},
		run: func(c *call) error {
			r, err := c.sanRemote()
			if err != nil {
				return err
			}
			var st proto.Message
			switch site := c.setting("Site"); site {
			case "Threads":
				res, e := r.threads.GetStatus(c.ctx, connect.NewRequest(&threadsv1.GetStatusRequest{}))
				if err = e; err == nil {
					st = res.Msg.GetStatus()
				}
			case "X":
				res, e := r.x.GetStatus(c.ctx, connect.NewRequest(&xv1.GetStatusRequest{}))
				if err = e; err == nil {
					st = res.Msg.GetStatus()
				}
			case "Facebook":
				res, e := r.facebook.GetStatus(c.ctx, connect.NewRequest(&facebookv1.GetStatusRequest{}))
				if err = e; err == nil {
					st = res.Msg.GetStatus()
				}
			case "Vidara":
				res, e := r.vidara.GetStatus(c.ctx, connect.NewRequest(&vidarav1.GetStatusRequest{}))
				if err = e; err == nil {
					st = res.Msg.GetStatus()
				}
			default:
				if !slices.Contains(statusSites, site) {
					return fmt.Errorf("Site %q: give one of %s", site, strings.Join(statusSites, ", "))
				}
				st, err = c.siteStatus(r, site)
			}
			if err != nil {
				return remoteErr(err)
			}
			c.outputs["Status"] = toValue(st)
			c.fire("Done")
			return nil
		},
	},
	{
		StepKind: &v1.StepKind{
			Kind: "social-posts", Title: "Social Posts", Category: "Social",
			Description: "Lists a site's scheduled or published posts (Medium's and LinkedIn's scheduled stories or drafts) through San Remote; Last is the latest.",
			Outputs:     []*v1.Socket{sock("Done", Event), sock("Posts", Object), sock("Last", Object)},
			Settings: []*v1.Setting{
				connectionSetting(),
				{Name: "Site", Options: sites},
				{Name: "List", Options: []string{"Scheduled", "Published", "Drafts"}},
			},
			Inputs: []*v1.Socket{sock("Event", Event)},
		},
		run: socialPosts,
	},
	{
		StepKind: &v1.StepKind{
			Kind: "last-scheduled-post", Title: "Last Scheduled Post", Category: "Social",
			Description: "Finds a site's last scheduled post through San Remote (the latest time): Found with it and its Datetime, Date and Time, or None when nothing is scheduled; Done either way.",
			Outputs: append([]*v1.Socket{sock("Done", Event), sock("Found", Event), sock("None", Event), sock("Post", Object),
				sock("Text", Text)}, timeOutputs()...),
			Settings: []*v1.Setting{connectionSetting(), {Name: "Site", Options: scheduleSites}},
			Inputs:   []*v1.Socket{sock("Event", Event)},
		},
		run: lastScheduledPost,
	},
	{
		// The user, 2026-10-27: "add node for getting post count at input
		// date in X" (postcount.go).
		StepKind: &v1.StepKind{
			Kind: "post-count", Title: "Post Count", Category: "Social",
			Description: "Counts a site's posts on Date through San Remote: a day in the tower's local time (2026-10-28; a datetime counts its day; empty: today). List picks Scheduled, Published or Both. Count is the total, Scheduled and Published each list's count, Posts the posts counted, each with its list. Published reads the latest 100 posts.",
			Outputs: []*v1.Socket{sock("Done", Event), sock("Count", Text), sock("Scheduled", Text), sock("Published", Text),
				sock("Posts", Object), sock("Date", Text)},
			Settings: []*v1.Setting{connectionSetting(), {Name: "Site", Options: countSites}, {Name: "List", Options: []string{"Both", "Scheduled", "Published"}}},
			Inputs:   []*v1.Socket{sock("Event", Event), {Name: "Date", Type: Text, Field: true}},
		},
		run: postCount,
	},
}

// scheduleSites are the sites with scheduled posts.
var scheduleSites = []string{"Threads", "X", "Facebook", "Medium", "LinkedIn", "TikTok", "YouTube"}

// socialPosts lists a site's scheduled or published posts, or drafts
// (Vidara: the account's videos). Last is the last scheduled one (the latest
// time) or the newest published one; for Facebook, whose list has no times,
// drafts and Vidara, it is the first listed.
func socialPosts(c *call) error {
	kind := c.setting("List")
	if kind == "" {
		kind = "Scheduled"
	}
	items, at, err := c.listPosts(c.setting("Site"), kind)
	if err != nil {
		return err
	}
	list := &structpb.ListValue{}
	for _, it := range items {
		list.Values = append(list.Values, toValue(it))
	}
	c.outputs["Posts"] = structpb.NewListValue(list)
	c.outputs["Last"] = structpb.NewNullValue()
	if last := lastOf(len(items), at); last >= 0 {
		c.outputs["Last"] = list.Values[last]
	}
	c.fire("Done")
	return nil
}

// lastScheduledPost finds a site's last scheduled post: the latest time
// (Facebook's list has no times: the first listed). Text is its text, and
// Datetime, Date and Time its time, as the datetime steps give it (all ""
// for Facebook, and when nothing is scheduled). It fires Found or None,
// then Done either way.
func lastScheduledPost(c *call) error {
	site := c.setting("Site")
	if !slices.Contains(scheduleSites, site) {
		return fmt.Errorf("Site %q: give one of %s", site, strings.Join(scheduleSites, ", "))
	}
	items, at, err := c.listPosts(site, "Scheduled")
	if err != nil {
		return err
	}
	last := lastOf(len(items), at)
	if last < 0 {
		c.outputs["Post"] = structpb.NewNullValue()
		c.outputs["Text"] = stringValue("")
		c.noTime()
		c.fire("None")
		c.fire("Done")
		return nil
	}
	post := toValue(items[last])
	c.outputs["Post"] = post
	c.outputs["Text"] = stringValue(postText(post))
	if at != nil {
		c.putTime(at(last))
	} else {
		c.noTime()
	}
	c.fire("Found")
	c.fire("Done")
	return nil
}

// lastOf is the index of the latest of n items by at, the first when at is
// nil, and -1 when there are none.
func lastOf(n int, at func(i int) time.Time) int {
	last := -1
	for i := range n {
		if last < 0 || (at != nil && at(i).After(at(last))) {
			last = i
		}
	}
	return last
}

// listPosts lists a site's posts through San Remote, and when each is (at is
// nil when the list has no times). kind is "Scheduled", "Published" or
// "Drafts"; Vidara lists the account's videos whatever it is.
func (c *call) listPosts(site, kind string) ([]proto.Message, func(i int) time.Time, error) {
	return c.listPostsUpTo(site, kind, 0)
}

// listPostsUpTo is listPosts reading up to limit published posts on X and
// Threads (0: San Remote's default, 10).
func (c *call) listPostsUpTo(site, kind string, limit int) ([]proto.Message, func(i int) time.Time, error) {
	r, err := c.sanRemote()
	if err != nil {
		return nil, nil, err
	}
	scheduled := kind == "Scheduled"
	if kind == "Drafts" && site != "Facebook" && site != "Vidara" && !slices.Contains([]string{"Medium", "LinkedIn"}, site) {
		return nil, nil, fmt.Errorf("List Drafts: %s has no drafts here; Facebook, Medium and LinkedIn do", site)
	}
	var items []proto.Message
	var at func(i int) time.Time
	switch site {
	case "Threads":
		if scheduled {
			res, e := r.threads.ListScheduled(c.ctx, connect.NewRequest(&threadsv1.ListScheduledRequest{}))
			if err = e; err == nil {
				for _, p := range res.Msg.Posts {
					items = append(items, p)
				}
				at = func(i int) time.Time { return res.Msg.Posts[i].GetAt().AsTime() }
			}
		} else {
			res, e := r.threads.ListPosts(c.ctx, connect.NewRequest(&threadsv1.ListPostsRequest{Limit: uint32(limit)}))
			if err = e; err == nil {
				for _, p := range res.Msg.Posts {
					items = append(items, p)
				}
				at = func(i int) time.Time { return res.Msg.Posts[i].GetAt().AsTime() }
			}
		}
	case "X":
		if scheduled {
			res, e := r.x.ListScheduled(c.ctx, connect.NewRequest(&xv1.ListScheduledRequest{}))
			if err = e; err == nil {
				for _, p := range res.Msg.Threads {
					items = append(items, p)
				}
				at = func(i int) time.Time { return res.Msg.Threads[i].GetAt().AsTime() }
			}
		} else {
			res, e := r.x.ListPosts(c.ctx, connect.NewRequest(&xv1.ListPostsRequest{Limit: uint32(limit)}))
			if err = e; err == nil {
				for _, p := range res.Msg.Posts {
					items = append(items, p)
				}
				at = func(i int) time.Time { return res.Msg.Posts[i].GetAt().AsTime() }
			}
		}
	case "Facebook":
		tab := map[string]string{"Scheduled": "scheduled", "Published": "published", "Drafts": "draft"}[kind]
		res, e := r.facebook.ListPosts(c.ctx, connect.NewRequest(&facebookv1.ListPostsRequest{Tab: tab}))
		if err = e; err == nil {
			for _, p := range res.Msg.Posts {
				items = append(items, p)
			}
		}
	case "Vidara":
		res, e := r.vidara.ListVideos(c.ctx, connect.NewRequest(&vidarav1.ListVideosRequest{}))
		if err = e; err == nil {
			for _, v := range res.Msg.Videos {
				items = append(items, v)
			}
		}
	case "Medium", "LinkedIn", "TikTok", "YouTube":
		items, at, err = c.listSitePosts(r, site, kind)
		if errors.Is(err, errNoList) {
			return nil, nil, err
		}
	default:
		return nil, nil, fmt.Errorf("Site %q: give one of %s", site, strings.Join(sites, ", "))
	}
	if err != nil {
		return nil, nil, remoteErr(err)
	}
	return items, at, nil
}

func field(name string) *v1.Socket { return &v1.Socket{Name: name, Type: Text, Field: true} }

// only has the editor show an input when a setting has a value, as
// "Source=File".
func only(when string, s *v1.Socket) *v1.Socket {
	s.When = when
	return s
}

// formatSettings are a post step's Format, Plain text or Markdown, and its
// Source, the text input or a File (the user, 2026-10-25).
func formatSettings() []*v1.Setting {
	return []*v1.Setting{
		{Name: "Format", Options: []string{"Plain", "Markdown"}},
		{Name: "Source", Options: []string{"Text", "File"}},
	}
}

// postInputs are a post step's inputs: its text (Text, or TikTok's Caption)
// or File, by its Source, and At.
func postInputs(text string) []*v1.Socket {
	return []*v1.Socket{sock("Event", Event), only("Source=Text", field(text)), only("Source=File", field("File")), atPicker()}
}

// xPostInputs are X Post's inputs (the user, 2026-10-25: a plain post takes
// a caption, images and a video): with Plain, Caption (or File) and Image or
// Video; with Markdown, Text (or File). Text was a plain post's text too
// before then, and a step saved so that links it still works.
func xPostInputs() []*v1.Socket {
	return []*v1.Socket{sock("Event", Event),
		only("Format=Plain,Source=Text", field("Caption")),
		only("Format=Markdown,Source=Text", field("Text")),
		only("Source=File", field("File")),
		only("Format=Plain", field("Image")),
		only("Format=Plain", field("Video")),
		atPicker()}
}

// xPost runs X Post. A plain post's text is Caption, or Text in a step saved
// before Caption was there, and may be empty when Image or Video is given;
// they go on its first post.
func xPost(c *call) error {
	plain := c.setting("Format") != "Markdown"
	text, media := "Text", []string(nil)
	if plain {
		var err error
		if media, err = c.xMedia(); err != nil {
			return err
		}
		if strings.TrimSpace(c.text("Caption")) != "" || strings.TrimSpace(c.text("Text")) == "" {
			text = "Caption"
		}
	}
	r, at, p, err := c.postArgs(minuteGrid, text, !plain)
	if err != nil {
		return err
	}
	req := &xv1.SchedulePostRequest{At: at, File: p.file, Markdown: p.markdown}
	for _, t := range posts(p.plain) {
		req.Posts = append(req.Posts, &xv1.Post{Text: t})
	}
	switch {
	case len(media) > 0:
		if len(req.Posts) == 0 {
			req.Posts = append(req.Posts, &xv1.Post{})
		}
		req.Posts[0].Media = media
	case plain && len(req.Posts) == 0 && c.setting("Source") == "File":
		return fmt.Errorf("File %w: give a caption in it, an Image or a Video", errEmpty)
	case plain && len(req.Posts) == 0:
		return fmt.Errorf("%s %w: give a caption, an Image or a Video", text, errEmpty)
	}
	res, err := r.x.SchedulePost(c.ctx, connect.NewRequest(req))
	if err != nil {
		return remoteErr(err)
	}
	c.outputs["Result"] = toValue(res.Msg)
	c.fire("Done")
	return nil
}

// xMedia is X Post's Image, up to 4 files one per line, or its Video, one
// video or GIF: X takes one or the other in a post. A relative path is in
// the tower's data folder; the files must be there.
func (c *call) xMedia() ([]string, error) {
	var media []string
	add := func(input, raw string) error {
		full, err := place(c, input, raw)
		if err != nil {
			return err
		}
		switch fi, err := os.Stat(filepath.FromSlash(full)); {
		case errors.Is(err, fs.ErrNotExist):
			return fmt.Errorf("%s %s is not there", input, full)
		case err != nil:
			return err
		case fi.IsDir():
			return fmt.Errorf("%s %s is a folder: give a file", input, full)
		}
		media = append(media, filepath.FromSlash(full))
		return nil
	}
	for _, l := range strings.Split(c.text("Image"), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			if err := add("Image", l); err != nil {
				return nil, err
			}
		}
	}
	if len(media) > 4 {
		return nil, fmt.Errorf("Image has %d files: X takes up to 4 images in a post", len(media))
	}
	if v := strings.TrimSpace(c.text("Video")); v != "" {
		if len(media) > 0 {
			return nil, errors.New("Image and Video: X takes up to 4 images or one video in a post, not both")
		}
		if err := add("Video", v); err != nil {
			return nil, err
		}
	}
	return media, nil
}

// markdownNote is what Markdown is, in a post step's description.
const markdownNote = "a post file: frontmatter such as at:, ![](image) attached, the markup written as plain text"

// post is a post step's words: plain text, or markdown, as text or a file,
// which San Remote reads as it reads a post file (frontmatter, images, the
// markup written as plain text, since the sites have none).
type post struct {
	plain, markdown, file string
}

func (p post) isMarkdown() bool { return p.markdown != "" || p.file != "" }

// postOf reads a post step's words by its Format and Source: the text input
// or File, an absolute path. A plain file is read here; a markdown one is
// San Remote's, as its images are relative to it. With required, they
// cannot be empty.
func (c *call) postOf(text string, required bool) (post, error) {
	markdown := c.setting("Format") == "Markdown"
	if c.setting("Source") == "File" {
		file, err := c.absolute("File", "the post's file")
		if err != nil {
			return post{}, err
		}
		if markdown {
			return post{file: file}, nil
		}
		b, err := os.ReadFile(file)
		if err != nil {
			return post{}, fmt.Errorf("File: %w", err)
		}
		if required && len(posts(string(b))) == 0 {
			return post{}, fmt.Errorf("File %q %w", file, errEmpty)
		}
		return post{plain: string(b)}, nil
	}
	s := c.text(text)
	switch {
	case required && len(posts(s)) == 0:
		return post{}, fmt.Errorf("%s %w", text, errEmpty)
	case markdown && strings.TrimSpace(s) != "":
		return post{markdown: s}, nil
	}
	return post{plain: s}, nil
}

// remoteClients are `san_remote api`'s services at one connection's URL.
type remoteClients struct {
	threads  threadsv1connect.ThreadsServiceClient
	x        xv1connect.XServiceClient
	facebook facebookv1connect.FacebookServiceClient
	vidara   vidarav1connect.VidaraServiceClient
	medium   mediumv1connect.MediumServiceClient
	linkedin linkedinv1connect.LinkedInServiceClient
	tiktok   tiktokv1connect.TikTokServiceClient
	youtube  youtubev1connect.YouTubeServiceClient
	telegram telegramv1connect.TelegramServiceClient
	chatgpt  chatgptv1connect.ChatGPTServiceClient
	gemini   geminiv1connect.GeminiServiceClient
	claude   claudev1connect.ClaudeServiceClient
}

// sanRemote is the san_remote connection the step names.
func (c *call) sanRemote() (remoteClients, error) {
	id := c.setting("Connection")
	if id == "" {
		return remoteClients{}, errors.New("Connection: pick a san_remote connection (Connections page)")
	}
	n, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		return remoteClients{}, fmt.Errorf("Connection %q is not a connection's id", id)
	}
	if c.env.Connection == nil {
		return remoteClients{}, errors.New("this run has no connections")
	}
	conn, err := c.env.Connection(uint32(n))
	if errors.Is(err, store.ErrNotFound) {
		return remoteClients{}, fmt.Errorf("connection %d is gone: pick another", n)
	}
	if err != nil {
		return remoteClients{}, err
	}
	if conn.Type != store.SanRemote {
		return remoteClients{}, fmt.Errorf("connection %s is not a san_remote connection", conn.Name)
	}
	// No timeout: a post takes a minute or two, and the run's context ends
	// it when the tower stops.
	hc, url := http.DefaultClient, conn.Settings.GetSanRemote().GetUrl()
	return remoteClients{
		threads:  threadsv1connect.NewThreadsServiceClient(hc, url),
		x:        xv1connect.NewXServiceClient(hc, url),
		facebook: facebookv1connect.NewFacebookServiceClient(hc, url),
		vidara:   vidarav1connect.NewVidaraServiceClient(hc, url),
		medium:   mediumv1connect.NewMediumServiceClient(hc, url),
		linkedin: linkedinv1connect.NewLinkedInServiceClient(hc, url),
		tiktok:   tiktokv1connect.NewTikTokServiceClient(hc, url),
		youtube:  youtubev1connect.NewYouTubeServiceClient(hc, url),
		telegram: telegramv1connect.NewTelegramServiceClient(hc, url),
		chatgpt:  chatgptv1connect.NewChatGPTServiceClient(hc, url),
		gemini:   geminiv1connect.NewGeminiServiceClient(hc, url),
		claude:   claudev1connect.NewClaudeServiceClient(hc, url),
	}, nil
}

// postArgs reads what every post step needs: its connection, its time on
// the site's grid and its words (postOf), from the input named text. A
// Text is required; a TikTok Caption may be empty.
func (c *call) postArgs(grid time.Duration, text string, required bool) (remoteClients, *timestamppb.Timestamp, post, error) {
	r, err := c.sanRemote()
	if err != nil {
		return r, nil, post{}, err
	}
	p, err := c.postOf(text, required)
	if err != nil {
		return r, nil, p, err
	}
	// Markdown may say when in its frontmatter (at:); San Remote says so
	// when neither does.
	if p.isMarkdown() && strings.TrimSpace(c.text("At")) == "" {
		return r, nil, p, nil
	}
	at, err := when(c.text("At"), time.Now(), grid)
	if err != nil {
		return r, nil, p, err
	}
	return r, timestamppb.New(at), p, nil
}

// when reads a post's At: a datetime as the picker and the datetime steps
// give it (2026-11-20T09:00, "2026-11-20 09:00", RFC 3339; local time unless
// it says otherwise), or a delay from now, "+" then days and a Go duration
// ("+90m", "+2h", "+1d", "+1d12h"). It is rounded up onto grid, the times
// the site offers: a time from Now carries seconds, which no site takes
// (2026-10-25). Grids of up to an hour match the clock in zones whose
// offset is on the grid, as Indonesia's are. It is required: posts are
// only scheduled.
func when(s string, now time.Time, grid time.Duration) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, errors.New(`At is empty: posts are only scheduled; pick a time, or link one (Now, then Datetime Operation)`)
	}
	var t time.Time
	if rest, ok := strings.CutPrefix(s, "+"); ok {
		d, err := days(rest)
		if err != nil {
			return time.Time{}, fmt.Errorf("At %q: give a delay such as +90m, +2h or +1d12h", s)
		}
		t = now.Add(d)
	} else {
		var err error
		if t, err = parseDatetime(s); err != nil {
			return time.Time{}, fmt.Errorf(`At %q: pick a time, or give one such as 2026-11-20T09:00, RFC 3339, or a delay such as +2h`, s)
		}
	}
	if r := t.Truncate(grid); r.Before(t) {
		t = r.Add(grid)
	}
	return t, nil
}

// posts splits a thread's text into its posts at lines that are only "---",
// as in a thread file.
func posts(text string) []string {
	var out []string
	var cur []string
	flush := func() {
		if p := strings.TrimSpace(strings.Join(cur, "\n")); p != "" {
			out = append(out, p)
		}
		cur = nil
	}
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		if strings.TrimSpace(line) == "---" {
			flush()
			continue
		}
		cur = append(cur, line)
	}
	flush()
	return out
}

// toValue is a message as an object input reads it: its protojson, so
// field names are lowerCamelCase and times RFC 3339 strings.
func toValue(m proto.Message) *structpb.Value {
	b, err := protojson.Marshal(m)
	if err != nil {
		return structpb.NewNullValue()
	}
	var v structpb.Value
	if err := protojson.Unmarshal(b, &v); err != nil {
		return structpb.NewNullValue()
	}
	return &v
}

// remoteErr is san_remote's own message: "x: the browser is not signed in
// to X", or that the api does not answer.
func remoteErr(err error) error {
	var ce *connect.Error
	if errors.As(err, &ce) {
		if ce.Code() == connect.CodeUnavailable && ce.Message() == "" {
			return fmt.Errorf("san_remote api does not answer: %v", err)
		}
		return errors.New(ce.Message())
	}
	return err
}
