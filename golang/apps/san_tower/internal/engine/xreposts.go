package engine

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"connectrpc.com/connect"
	xv1 "github.com/wargasipil/san_remote/gen/san/remote/x/v1"
	"google.golang.org/protobuf/types/known/structpb"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
)

// X Reposts and X Undo Repost (the user, 2026-11-08: "Can we have node to
// get reposted post, and cancel reposted post", then "get reposts and
// process to workflow, and after finish, it derepost"): a workflow lists the
// account's reposts, does what it does with each, and then undoes the
// repost, so the next run does not see it again.

// xRepostsDefault is how many reposts X Reposts lists when Limit is empty,
// as San Remote's default; xRepostsMax the most it takes.
const (
	xRepostsDefault = 20
	xRepostsMax     = 200
)

// xReposts runs X Reposts: San Remote reads the signed-in account's Reposts
// tab in a background tab, the latest repost first, up to Limit. Each fires
// once per repost with Item it and Link its link, then Done; Reposts is the
// list and Count how many.
func xReposts(c *call) error {
	limit := xRepostsDefault
	if s := strings.TrimSpace(c.setting("Limit")); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 || n > xRepostsMax {
			return fmt.Errorf("Limit %q: give how many reposts to list, 1 to %d (empty: %d)", s, xRepostsMax, xRepostsDefault)
		}
		limit = n
	}
	r, err := c.sanRemote()
	if err != nil {
		return err
	}
	res, err := r.x.ListReposts(c.ctx, connect.NewRequest(&xv1.ListRepostsRequest{Limit: uint32(limit)}))
	if err != nil {
		return remoteErr(err)
	}
	reposts := res.Msg.GetReposts()
	list := &structpb.ListValue{}
	for _, p := range reposts {
		list.Values = append(list.Values, toValue(p))
	}
	c.logf(v1.LogLevel_LOG_LEVEL_INFO, "@%s has %d reposts listed (up to %d)", res.Msg.GetAccount(), len(reposts), limit)
	c.outputs["Reposts"] = structpb.NewListValue(list)
	c.outputs["Count"] = stringValue(strconv.Itoa(len(reposts)))
	c.outputs["Item"] = structpb.NewNullValue()
	c.outputs["Link"] = stringValue("")
	for i, p := range reposts {
		if c.each == nil {
			break
		}
		c.outputs["Item"] = list.Values[i]
		c.outputs["Link"] = stringValue(p.GetLink())
		if err := c.each("Each"); err != nil {
			return err
		}
	}
	c.fire("Done")
	return nil
}

// xPostLink is a post's link as X and Twitter write it (x.com,
// twitter.com, www., mobile.; /photo/1, ?s=... after it), or a bare id: what
// San Remote takes.
var xPostLink = regexp.MustCompile(`^(?:(?:https?://)?(?:www\.|mobile\.)?(?:x|twitter)\.com/(?:[A-Za-z0-9_]+|i/web|i)/status(?:es)?/\d+(?:[/?#].*)?|\d+)$`)

// xUndoRepost runs X Undo Repost: San Remote opens the post at Link and, when
// the account reposted it, presses its green Repost button and Undo repost.
// Undone or Not Reposted fires, then Done either way.
func xUndoRepost(c *call) error {
	link := strings.TrimSpace(c.text("Link"))
	if link == "" {
		return fmt.Errorf("Link %w: give the post's link, such as https://x.com/user/status/123, or X Reposts' Link", errEmpty)
	}
	if !xPostLink.MatchString(link) {
		return fmt.Errorf("Link %q is not an X post's link, such as https://x.com/user/status/123, or its id", link)
	}
	r, err := c.sanRemote()
	if err != nil {
		return err
	}
	res, err := r.x.Unrepost(c.ctx, connect.NewRequest(&xv1.UnrepostRequest{Link: link}))
	if err != nil {
		return remoteErr(err)
	}
	if res.Msg.GetUndone() {
		c.logf(v1.LogLevel_LOG_LEVEL_SUCCESS, "undid the repost of %s", res.Msg.GetLink())
		c.fire("Undone")
	} else {
		c.logf(v1.LogLevel_LOG_LEVEL_INFO, "%s is not reposted by the account: nothing to undo", res.Msg.GetLink())
		c.fire("Not Reposted")
	}
	c.fire("Done")
	return nil
}
