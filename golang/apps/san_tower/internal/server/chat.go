package server

import (
	"context"
	"errors"
	"strings"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/engine"
	"github.com/wargasipil/san_tower/internal/store"
)

// ChatService is the Chat page: chats with workflows that have a Tower Chat
// step (engine/towerchat.go).

func (s *Server) ListChats(context.Context, *connect.Request[v1.ListChatsRequest]) (*connect.Response[v1.ListChatsResponse], error) {
	list, err := s.store.ListTowerChats()
	if err != nil {
		return nil, storeError(err)
	}
	heads, err := s.store.WorkflowHeads()
	if err != nil {
		return nil, storeError(err)
	}
	last, err := s.store.LastTowerChatMessages()
	if err != nil {
		return nil, storeError(err)
	}
	res := &v1.ListChatsResponse{Chats: make([]*v1.Chat, len(list))}
	for i, c := range list {
		p := chatToProto(c, heads[c.WorkflowID])
		if m, ok := last[c.ID]; ok {
			p.Last = chatMessageToProto(m)
		}
		res.Chats[i] = p
	}
	return connect.NewResponse(res), nil
}

func (s *Server) ListChatWorkflows(context.Context, *connect.Request[v1.ListChatWorkflowsRequest]) (*connect.Response[v1.ListChatWorkflowsResponse], error) {
	list, err := s.store.Workflows()
	if err != nil {
		return nil, storeError(err)
	}
	res := &v1.ListChatWorkflowsResponse{}
	for _, w := range list {
		if len(engine.TowerChatSteps(w)) > 0 {
			res.Workflows = append(res.Workflows, &v1.ChatWorkflow{Id: w.ID, Name: w.Name, Description: w.Description, Active: w.Active})
		}
	}
	return connect.NewResponse(res), nil
}

func (s *Server) CreateChat(_ context.Context, req *connect.Request[v1.CreateChatRequest]) (*connect.Response[v1.CreateChatResponse], error) {
	w, err := s.store.GetWorkflow(req.Msg.GetWorkflowId())
	if err != nil {
		return nil, storeError(err)
	}
	if len(engine.TowerChatSteps(w)) == 0 {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New(w.Name+" has no Tower Chat step: add one in its editor first"))
	}
	name := strings.TrimSpace(req.Msg.GetName())
	if name == "" {
		name = w.Name
	}
	c, err := s.store.CreateTowerChat(w.ID, name)
	if err != nil {
		return nil, storeError(err)
	}
	return connect.NewResponse(&v1.CreateChatResponse{Chat: chatToProto(c, store.WorkflowHead{ID: w.ID, Name: w.Name, Active: w.Active})}), nil
}

func (s *Server) UpdateChat(_ context.Context, req *connect.Request[v1.UpdateChatRequest]) (*connect.Response[v1.UpdateChatResponse], error) {
	c, err := s.store.RenameTowerChat(req.Msg.GetId(), req.Msg.GetName())
	if err != nil {
		return nil, storeError(err)
	}
	p, err := s.chat(c)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&v1.UpdateChatResponse{Chat: p}), nil
}

func (s *Server) DeleteChat(_ context.Context, req *connect.Request[v1.DeleteChatRequest]) (*connect.Response[v1.DeleteChatResponse], error) {
	if err := s.store.DeleteTowerChat(req.Msg.GetId()); err != nil {
		return nil, storeError(err)
	}
	return connect.NewResponse(&v1.DeleteChatResponse{}), nil
}

// maxChatPage is the most messages ListChatMessages gives at once.
const maxChatPage = 500

func (s *Server) ListChatMessages(_ context.Context, req *connect.Request[v1.ListChatMessagesRequest]) (*connect.Response[v1.ListChatMessagesResponse], error) {
	c, err := s.store.GetTowerChat(req.Msg.GetChatId())
	if err != nil {
		return nil, storeError(err)
	}
	limit := int(req.Msg.GetLimit())
	if limit == 0 {
		limit = 100
	}
	limit = min(limit, maxChatPage)
	list, more, err := s.store.TowerChatMessages(c.ID, req.Msg.GetAfterId(), req.Msg.GetBeforeId(), limit)
	if err != nil {
		return nil, storeError(err)
	}
	res := &v1.ListChatMessagesResponse{More: more, Messages: make([]*v1.ChatMessage, len(list))}
	for i, m := range list {
		res.Messages[i] = chatMessageToProto(m)
	}
	if res.Chat, err = s.chat(c); err != nil {
		return nil, err
	}
	watch := req.Msg.GetWatchRuns()
	if len(watch) > maxChatPage {
		watch = watch[len(watch)-maxChatPage:]
	}
	runs, err := s.store.RunHeads(watch)
	if err != nil {
		return nil, storeError(err)
	}
	for _, r := range runs {
		res.Runs = append(res.Runs, s.chatRunOf(r))
	}
	return connect.NewResponse(res), nil
}

func (s *Server) SendChatMessage(_ context.Context, req *connect.Request[v1.SendChatMessageRequest]) (*connect.Response[v1.SendChatMessageResponse], error) {
	m, runs, err := s.engine.Chat(req.Msg.GetChatId(), req.Msg.GetText())
	if errors.Is(err, engine.ErrChatClosed) {
		return nil, connect.NewError(connect.CodeFailedPrecondition, err)
	}
	if err != nil {
		return nil, storeError(err)
	}
	res := &v1.SendChatMessageResponse{Message: chatMessageToProto(m)}
	for _, r := range runs {
		res.Runs = append(res.Runs, s.chatRunOf(r))
	}
	return connect.NewResponse(res), nil
}

// chat is chat c as the page shows it, with its workflow's name and switch.
func (s *Server) chat(c store.TowerChat) (*v1.Chat, error) {
	w, err := s.store.GetWorkflow(c.WorkflowID)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return nil, storeError(err)
	}
	return chatToProto(c, store.WorkflowHead{ID: w.ID, Name: w.Name, Active: w.Active}), nil
}

// chatRunOf is run r as a chat shows it: while it runs, with the last line
// its steps logged as they went (engine.LiveLogs).
func (s *Server) chatRunOf(r store.Run) *v1.ChatRun {
	p := &v1.ChatRun{Id: r.ID, Status: r.Status, Error: r.Error, StartedAt: timestamppb.New(r.StartedAt)}
	if r.FinishedAt != nil {
		p.FinishedAt = timestamppb.New(*r.FinishedAt)
	}
	if r.Status == v1.RunStatus_RUN_STATUS_RUNNING {
		if lines, _, _, ok := s.engine.LiveLogs(r.ID); ok && len(lines) > 0 {
			p.Line = lines[len(lines)-1]
		}
	}
	return p
}

func chatToProto(c store.TowerChat, w store.WorkflowHead) *v1.Chat {
	return &v1.Chat{
		Id:             c.ID,
		Name:           c.Name,
		WorkflowId:     c.WorkflowID,
		WorkflowName:   w.Name,
		WorkflowActive: w.Active,
		CreatedAt:      timestamppb.New(c.CreatedAt),
		UpdatedAt:      timestamppb.New(c.UpdatedAt),
	}
}

func chatMessageToProto(m store.TowerChatMessage) *v1.ChatMessage {
	return &v1.ChatMessage{
		Id:         m.ID,
		ChatId:     m.ChatID,
		From:       m.From,
		Text:       m.Text,
		Markdown:   m.Markdown,
		RunIds:     m.RunIDs,
		WorkflowId: m.WorkflowID,
		CreatedAt:  timestamppb.New(m.CreatedAt),
	}
}
