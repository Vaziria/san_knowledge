package main

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	san_youtube "github.com/wargasipil/san_youtube"
)

func TestLine(t *testing.T) {
	at := time.Date(2026, 9, 24, 1, 2, 3, 0, time.Local)
	base := func(name string) san_youtube.Base {
		return san_youtube.Base{ID: "x", Time: at, Author: san_youtube.Author{Name: name}}
	}
	mod := base("@mod")
	mod.Author.Moderator, mod.Author.Member = true, true

	tests := []struct {
		msg  san_youtube.Message
		want string
	}{
		{&san_youtube.TextMessage{Base: base("@budi"), Text: "halo"}, "01:02:03 @budi: halo"},
		{&san_youtube.TextMessage{Base: mod, Text: "jaga bahasa"}, "01:02:03 @mod [mod, member]: jaga bahasa"},
		{&san_youtube.SuperChat{Base: base("@budi"), Amount: san_youtube.ParseAmount("Rp 20.000"), Text: "semangat"}, "01:02:03 [Super Chat Rp 20.000] @budi: semangat"},
		{&san_youtube.SuperSticker{Base: base("@budi"), Amount: san_youtube.ParseAmount("$2.00"), Sticker: "Cat waving"}, "01:02:03 [Super Sticker $2.00] @budi: Cat waving"},
		{&san_youtube.Membership{Base: base("@ani"), Subtext: "Welcome to Kenalan!"}, "01:02:03 [New member] @ani: Welcome to Kenalan!"},
		{&san_youtube.Membership{Base: base("@ani"), Milestone: true, Header: "Member for 6 months", Text: "setengah tahun"}, "01:02:03 [Member for 6 months] @ani: setengah tahun"},
		{&san_youtube.GiftPurchase{Base: base("@ani"), Count: 5, Text: "Sent 5 Kenalan gift memberships"}, "01:02:03 [Gift] @ani: Sent 5 Kenalan gift memberships"},
		{&san_youtube.GiftRedemption{Base: base("@budi"), Text: "received a gift membership by @ani"}, "01:02:03 [Gift] @budi received a gift membership by @ani"},
		{&san_youtube.Deletion{Base: san_youtube.Base{Time: at}, TargetID: "m1", Text: "[message deleted]"}, "01:02:03 [Deleted] message m1 [message deleted]"},
		{&san_youtube.Deletion{Base: san_youtube.Base{Time: at}, TargetID: "m1"}, "01:02:03 [Deleted] message m1"},
		{&san_youtube.AuthorDeletion{Base: san_youtube.Base{Time: at, Author: san_youtube.Author{ChannelID: "UCx"}}}, "01:02:03 [Deleted] all messages by UCx"},
		{&san_youtube.Replacement{Base: san_youtube.Base{Time: at}, TargetID: "p1", Message: &san_youtube.TextMessage{Base: base("@budi"), Text: "ditahan"}}, "01:02:03 [Replaces p1] @budi: ditahan"},
	}
	for _, tt := range tests {
		if got := line(tt.msg); got != tt.want {
			t.Errorf("line(%s) = %q, want %q", tt.msg.Kind(), got, tt.want)
		}
	}
}

func TestRunNeedsOneStream(t *testing.T) {
	var out, errOut bytes.Buffer
	if err := run(t.Context(), nil, &out, &errOut); err == nil {
		t.Error("run with no stream succeeded")
	}
	if !strings.Contains(errOut.String(), "usage: san_youtube") {
		t.Errorf("stderr = %q, want the usage", errOut.String())
	}
	if err := run(t.Context(), []string{"-h"}, &out, &errOut); err != nil {
		t.Errorf("run -h = %v", err)
	}
}

func TestRunRejectsBadInputWithoutNetwork(t *testing.T) {
	var out, errOut bytes.Buffer
	err := run(t.Context(), []string{"https://example.com/not-youtube"}, &out, &errOut)
	if err == nil || !strings.Contains(err.Error(), "not a YouTube video or channel") {
		t.Errorf("run = %v", err)
	}
}

// Ctrl+C and -for end quietly; an HTTP client timeout, which also reads as
// context.DeadlineExceeded, is still an error.
func TestQuiet(t *testing.T) {
	timeout := fmt.Errorf("san_youtube: GET https://www.youtube.com/live_chat: %w", context.DeadlineExceeded)
	if err := quiet(t.Context(), timeout); err == nil {
		t.Error("a client timeout with the context still live was hidden")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := quiet(ctx, ctx.Err()); err != nil {
		t.Errorf("quiet(cancelled) = %v, want nil", err)
	}
	if err := quiet(ctx, san_youtube.ErrChatDisabled); err == nil {
		t.Error("a real error was hidden because the context was done")
	}
}
