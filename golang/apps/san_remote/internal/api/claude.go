package api

import (
	"context"
	"time"

	"connectrpc.com/connect"

	claudev1 "github.com/wargasipil/san_remote/gen/san/remote/claude/v1"
	"github.com/wargasipil/san_remote/internal/claude"
)

// ---- Claude --------------------------------------------------------------------------

type claudeAPI struct{ a *API }

func (s claudeAPI) GetStatus(ctx context.Context, req *connect.Request[claudev1.GetStatusRequest]) (*connect.Response[claudev1.GetStatusResponse], error) {
	st, err := s.a.Claude.Status(ctx, req.Msg.Browser)
	if err != nil {
		return nil, s.a.fail(err)
	}
	return reply(&claudev1.GetStatusResponse{Status: &claudev1.Status{SignedIn: st.SignedIn, Name: st.Name, Plan: st.Plan}}, nil)
}

func (s claudeAPI) Ask(ctx context.Context, req *connect.Request[claudev1.AskRequest]) (*connect.Response[claudev1.AskResponse], error) {
	q := claude.Ask{Browser: req.Msg.Browser, Prompt: req.Msg.Prompt, Keep: req.Msg.Keep, Chat: req.Msg.Chat, Wait: time.Duration(req.Msg.WaitSeconds) * time.Second, Stay: req.Msg.Stay, Idle: time.Duration(req.Msg.IdleSeconds) * time.Second}
	if err := q.Check(); err != nil {
		return nil, s.a.fail(err)
	}
	ans, err := s.a.Claude.Ask(ctx, q)
	if err != nil {
		return nil, s.a.fail(err)
	}
	return reply(&claudev1.AskResponse{Text: ans.Text, Chat: ans.Chat, Incognito: ans.Incognito}, nil)
}
