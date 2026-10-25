package api

import (
	"context"
	"time"

	"connectrpc.com/connect"

	geminiv1 "github.com/wargasipil/san_remote/gen/san/remote/gemini/v1"
	"github.com/wargasipil/san_remote/internal/gemini"
)

// ---- Gemini --------------------------------------------------------------------------

type geminiAPI struct{ a *API }

func (s geminiAPI) GetStatus(ctx context.Context, req *connect.Request[geminiv1.GetStatusRequest]) (*connect.Response[geminiv1.GetStatusResponse], error) {
	st, err := s.a.Gemini.Status(ctx, req.Msg.Browser)
	if err != nil {
		return nil, s.a.fail(err)
	}
	return reply(&geminiv1.GetStatusResponse{Status: &geminiv1.Status{SignedIn: st.SignedIn, Name: st.Name}}, nil)
}

func (s geminiAPI) Ask(ctx context.Context, req *connect.Request[geminiv1.AskRequest]) (*connect.Response[geminiv1.AskResponse], error) {
	q := gemini.Ask{Browser: req.Msg.Browser, Prompt: req.Msg.Prompt, Keep: req.Msg.Keep, Wait: time.Duration(req.Msg.WaitSeconds) * time.Second, Stay: req.Msg.Stay}
	if err := q.Check(); err != nil {
		return nil, s.a.fail(err)
	}
	ans, err := s.a.Gemini.Ask(ctx, q)
	if err != nil {
		return nil, s.a.fail(err)
	}
	return reply(&geminiv1.AskResponse{Text: ans.Text, Chat: ans.Chat, Temporary: ans.Temporary}, nil)
}
