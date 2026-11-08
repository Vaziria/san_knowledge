package api

import (
	"context"
	"time"

	"connectrpc.com/connect"

	grokv1 "github.com/wargasipil/san_remote/gen/san/remote/grok/v1"
	"github.com/wargasipil/san_remote/internal/grok"
)

// ---- Grok ----------------------------------------------------------------------------

type grokAPI struct{ a *API }

func (s grokAPI) GetStatus(ctx context.Context, req *connect.Request[grokv1.GetStatusRequest]) (*connect.Response[grokv1.GetStatusResponse], error) {
	st, err := s.a.Grok.Status(ctx, req.Msg.Browser)
	if err != nil {
		return nil, s.a.fail(err)
	}
	return reply(&grokv1.GetStatusResponse{Status: &grokv1.Status{SignedIn: st.SignedIn, Name: st.Name}}, nil)
}

func (s grokAPI) Ask(ctx context.Context, req *connect.Request[grokv1.AskRequest]) (*connect.Response[grokv1.AskResponse], error) {
	q := grok.Ask{Browser: req.Msg.Browser, Prompt: req.Msg.Prompt, Keep: req.Msg.Keep, Chat: req.Msg.Chat, Wait: time.Duration(req.Msg.WaitSeconds) * time.Second, Stay: req.Msg.Stay, Idle: time.Duration(req.Msg.IdleSeconds) * time.Second, Files: req.Msg.Files}
	if err := q.Check(); err != nil {
		return nil, s.a.fail(err)
	}
	ans, err := s.a.Grok.Ask(ctx, q)
	if err != nil {
		return nil, s.a.fail(err)
	}
	return reply(&grokv1.AskResponse{Text: ans.Text, Chat: ans.Chat, Private: ans.Private}, nil)
}
