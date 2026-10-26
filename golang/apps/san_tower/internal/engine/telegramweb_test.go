package engine

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"connectrpc.com/connect"
	telegramv1 "github.com/wargasipil/san_remote/gen/san/remote/telegram/v1"
	"github.com/wargasipil/san_remote/gen/san/remote/telegram/v1/telegramv1connect"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/store"
)

const fakeBotToken = "1234567890:AAH-made_up_token_for_the_test_0123456789"

type fakeTelegramWeb struct {
	telegramv1connect.UnimplementedTelegramServiceHandler
	reqs []any
}

func (f *fakeTelegramWeb) CreateBot(_ context.Context, req *connect.Request[telegramv1.CreateBotRequest]) (*connect.Response[telegramv1.CreateBotResponse], error) {
	f.reqs = append(f.reqs, req.Msg)
	if req.Msg.Username == "taken_bot" {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("telegram: refused: @BotFather: Sorry, this username is already taken."))
	}
	u := strings.TrimPrefix(req.Msg.Username, "@")
	return connect.NewResponse(&telegramv1.CreateBotResponse{Bot: &telegramv1.Bot{Name: req.Msg.Name, Username: u, Token: fakeBotToken, Link: "https://t.me/" + u}}), nil
}

func (f *fakeTelegramWeb) DeleteBot(_ context.Context, req *connect.Request[telegramv1.DeleteBotRequest]) (*connect.Response[telegramv1.DeleteBotResponse], error) {
	f.reqs = append(f.reqs, req.Msg)
	return connect.NewResponse(&telegramv1.DeleteBotResponse{}), nil
}

func (f *fakeTelegramWeb) CreateGroup(_ context.Context, req *connect.Request[telegramv1.CreateGroupRequest]) (*connect.Response[telegramv1.CreateGroupResponse], error) {
	f.reqs = append(f.reqs, req.Msg)
	return connect.NewResponse(&telegramv1.CreateGroupResponse{Group: &telegramv1.Group{Id: "-5012345678", Title: req.Msg.Title}}), nil
}

func (f *fakeTelegramWeb) DeleteGroup(_ context.Context, req *connect.Request[telegramv1.DeleteGroupRequest]) (*connect.Response[telegramv1.DeleteGroupResponse], error) {
	f.reqs = append(f.reqs, req.Msg)
	return connect.NewResponse(&telegramv1.DeleteGroupResponse{Group: &telegramv1.Group{Id: "-5012345678", Title: "Uji"}}), nil
}

func (f *fakeTelegramWeb) AddMember(_ context.Context, req *connect.Request[telegramv1.AddMemberRequest]) (*connect.Response[telegramv1.AddMemberResponse], error) {
	f.reqs = append(f.reqs, req.Msg)
	return connect.NewResponse(&telegramv1.AddMemberResponse{Group: &telegramv1.Group{Id: req.Msg.Group, Title: "Uji"}, Member: &telegramv1.Member{Id: "42", Name: "Budi", Username: req.Msg.User}}), nil
}

func (f *fakeTelegramWeb) RemoveMember(_ context.Context, req *connect.Request[telegramv1.RemoveMemberRequest]) (*connect.Response[telegramv1.RemoveMemberResponse], error) {
	f.reqs = append(f.reqs, req.Msg)
	return connect.NewResponse(&telegramv1.RemoveMemberResponse{Group: &telegramv1.Group{Id: req.Msg.Group, Title: "Uji"}, Member: &telegramv1.Member{Id: "42", Name: "Budi", Username: req.Msg.User}}), nil
}

// telegramWebEnv is a San Remote connection (1) at a fake Telegram service,
// and connections that AddConnection adds to added. "Taken" is a name
// already in use.
func telegramWebEnv(t *testing.T) (*fakeTelegramWeb, Env, *[]store.Connection) {
	t.Helper()
	f := &fakeTelegramWeb{}
	mux := http.NewServeMux()
	mux.Handle(telegramv1connect.NewTelegramServiceHandler(f))
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	var added []store.Connection
	env := Env{
		Connection: func(id uint32) (store.Connection, error) {
			if id != 1 {
				return store.Connection{}, store.ErrNotFound
			}
			return store.Connection{ID: 1, Name: "Chrome", Type: store.SanRemote, Settings: &v1.ConnectionSettings{
				Type: &v1.ConnectionSettings_SanRemote{SanRemote: &v1.SanRemoteSettings{Url: ts.URL}},
			}}, nil
		},
		AddConnection: func(c store.Connection) (store.Connection, error) {
			if c.Name == "Taken" {
				return c, store.ErrExists
			}
			c.ID = uint32(10 + len(added))
			added = append(added, c)
			return c, nil
		},
	}
	return f, env, &added
}

// Create Bot adds the bot as a Telegram Bot connection named after it (or
// after it and its username when the name is taken), and never outputs
// the token.
func TestTelegramCreateBot(t *testing.T) {
	f, env, added := telegramWebEnv(t)
	steps, err := run(t, env, "telegram-create-bot", "Connection", "1", "Name", "San Test", "Username", "@san_test_bot")
	if err != nil {
		t.Fatal(err)
	}
	if r := f.reqs[0].(*telegramv1.CreateBotRequest); r.Name != "San Test" || r.Username != "@san_test_bot" {
		t.Errorf("request = %v", r)
	}
	if len(*added) != 1 || (*added)[0].Name != "San Test" || (*added)[0].Settings.GetTelegramBot().GetToken() != fakeBotToken {
		t.Fatalf("added = %v", *added)
	}
	if got := textOut(t, steps[1], "Connection ID"); got != "10" {
		t.Errorf("Connection ID = %q", got)
	}
	if got := textOut(t, steps[1], "Link"); got != "https://t.me/san_test_bot" {
		t.Errorf("Link = %q", got)
	}
	for name, out := range steps[1].Outputs {
		if strings.Contains(string(out.GetValue()), "made_up_token") {
			t.Errorf("output %s has the token", name)
		}
	}
	if _, err := run(t, env, "telegram-create-bot", "Connection", "1", "Name", "Taken", "Username", "taken2_bot"); err != nil {
		t.Fatal(err)
	}
	if got := (*added)[1].Name; got != "Taken (@taken2_bot)" {
		t.Errorf("second name = %q", got)
	}
	if _, err := run(t, env, "telegram-create-bot", "Connection", "1", "Name", "X", "Username", "taken_bot"); err == nil || !strings.Contains(err.Error(), "already taken") {
		t.Errorf("taken: %v", err)
	}
	if _, err := run(t, env, "telegram-create-bot", "Connection", "1", "Name", "X"); err == nil || !strings.Contains(err.Error(), "Username") {
		t.Errorf("no username: %v", err)
	}
}

// The group and member steps send their inputs.
func TestTelegramGroupSteps(t *testing.T) {
	f, env, _ := telegramWebEnv(t)
	steps, err := run(t, env, "telegram-create-group", "Connection", "1", "Title", " Uji ")
	if err != nil {
		t.Fatal(err)
	}
	if got := textOut(t, steps[1], "Group ID"); got != "-5012345678" {
		t.Errorf("Group ID = %q", got)
	}
	if _, err := run(t, env, "telegram-add-member", "Connection", "1", "Group", "-5012345678", "User", "@budi"); err != nil {
		t.Fatal(err)
	}
	if _, err := run(t, env, "telegram-remove-member", "Connection", "1", "Group", "Uji", "User", "budi"); err != nil {
		t.Fatal(err)
	}
	if _, err := run(t, env, "telegram-delete-group", "Connection", "1", "Group", "-5012345678"); err != nil {
		t.Fatal(err)
	}
	if _, err := run(t, env, "telegram-delete-bot", "Connection", "1", "Username", "san_test_bot"); err != nil {
		t.Fatal(err)
	}
	if r := f.reqs[0].(*telegramv1.CreateGroupRequest); r.Title != "Uji" {
		t.Errorf("create = %v", r)
	}
	if r := f.reqs[1].(*telegramv1.AddMemberRequest); r.Group != "-5012345678" || r.User != "@budi" {
		t.Errorf("add = %v", r)
	}
	if r := f.reqs[2].(*telegramv1.RemoveMemberRequest); r.Group != "Uji" || r.User != "budi" {
		t.Errorf("remove = %v", r)
	}
	if r := f.reqs[3].(*telegramv1.DeleteGroupRequest); r.Group != "-5012345678" {
		t.Errorf("delete group = %v", r)
	}
	if r := f.reqs[4].(*telegramv1.DeleteBotRequest); r.Username != "san_test_bot" {
		t.Errorf("delete bot = %v", r)
	}
	if _, err := run(t, env, "telegram-add-member", "Connection", "1", "Group", "Uji"); err == nil || !strings.Contains(err.Error(), "User") {
		t.Errorf("no user: %v", err)
	}
	if _, err := run(t, env, "telegram-delete-group", "Connection", "1"); err == nil || !strings.Contains(err.Error(), "Group") {
		t.Errorf("no group: %v", err)
	}
}
