package engine

import (
	"cmp"
	"fmt"
	"path/filepath"
	"strings"

	"connectrpc.com/connect"
	xv1 "github.com/wargasipil/san_remote/gen/san/remote/x/v1"
	"google.golang.org/protobuf/types/known/structpb"
)

// xDownload runs X Post Download (the user's ask, 2026-10-25): San Remote
// reads the post at Link in a background tab and saves its images and
// videos, and an X Article's cover and images, in Save To, a folder placed
// as File Path places a name (empty: the storage or the data folder
// itself). Caption is the post's text, Text an article as markdown (its
// images the saved files) or else the caption, Tags the hashtags as
// "#a #b", Media the saved files and First the first of them.
func xDownload(c *call) error {
	r, err := c.sanRemote()
	if err != nil {
		return err
	}
	link := strings.TrimSpace(c.text("Link"))
	if link == "" {
		return fmt.Errorf("Link %w: give the post's link, such as https://x.com/user/status/123", errEmpty)
	}
	out, err := place(c, "Save To", cmp.Or(strings.TrimSpace(c.text("Save To")), "."))
	if err != nil {
		return err
	}
	res, err := r.x.DownloadPost(c.ctx, connect.NewRequest(&xv1.DownloadPostRequest{Link: link, Out: filepath.FromSlash(out)}))
	if err != nil {
		return remoteErr(err)
	}
	p := res.Msg.GetPost()
	files := &structpb.ListValue{}
	first := ""
	for _, m := range p.GetMedia() {
		f := filepath.ToSlash(m.GetFile())
		files.Values = append(files.Values, structpb.NewStringValue(f))
		if first == "" {
			first = f
		}
	}
	tags := make([]string, len(p.GetTags()))
	for i, t := range p.GetTags() {
		tags[i] = "#" + t
	}
	c.outputs["Caption"] = stringValue(p.GetCaption())
	c.outputs["Text"] = stringValue(p.GetText())
	c.outputs["Markdown"] = stringValue(postMarkdown(p))
	c.outputs["Title"] = stringValue(p.GetTitle())
	c.outputs["Tags"] = stringValue(strings.Join(tags, " "))
	c.outputs["Media"] = structpb.NewListValue(files)
	c.outputs["First"] = stringValue(first)
	c.outputs["Post"] = toValue(p)
	c.fire("Done")
	return nil
}

// postMarkdown is the post as markdown (the user's ask, 2026-10-25), as a
// post step's Format Markdown and a Medium or LinkedIn article read it: an X
// Article's Text, which is markdown already, or the caption with each saved
// image and video under it as ![alt](file), by its absolute path.
func postMarkdown(p *xv1.DownloadedPost) string {
	if p.GetTitle() != "" {
		return p.GetText()
	}
	var parts []string
	if caption := strings.TrimSpace(p.GetCaption()); caption != "" {
		parts = append(parts, caption)
	}
	for _, m := range p.GetMedia() {
		parts = append(parts, mdImage(m.GetAlt(), filepath.ToSlash(m.GetFile())))
	}
	return strings.Join(parts, "\n\n")
}

// mdImage is a markdown image: its alt text on one line, without brackets
// that would end it, and its path in <> when it has a space.
func mdImage(alt, file string) string {
	alt = strings.NewReplacer("\r", " ", "\n", " ", "[", "(", "]", ")").Replace(alt)
	if strings.Contains(file, " ") {
		file = "<" + file + ">"
	}
	return "![" + alt + "](" + file + ")"
}
