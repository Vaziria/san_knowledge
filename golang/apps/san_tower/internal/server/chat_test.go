package server

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/gen/san/tower/v1/towerv1connect"
)

// ChatService: a chat with a workflow that has a Tower Chat step, a message
// that starts its run and the answer it sends, the run's status, and the
// list, rename and delete.
func TestChatService(t *testing.T) {
	ts := tower(t)
	ctx := context.Background()
	wf := towerv1connect.NewWorkflowServiceClient(ts.Client(), ts.URL)
	c := towerv1connect.NewChatServiceClient(ts.Client(), ts.URL)

	plain, err := wf.CreateWorkflow(ctx, connect.NewRequest(&v1.CreateWorkflowRequest{Name: "Plain"}))
	if err != nil {
		t.Fatal(err)
	}
	cr, err := wf.CreateWorkflow(ctx, connect.NewRequest(&v1.CreateWorkflowRequest{Name: "Echo"}))
	if err != nil {
		t.Fatal(err)
	}
	w := cr.Msg.Workflow
	w.Steps = []*v1.Step{{Id: "in", Kind: "tower-chat"}, {Id: "out", Kind: "tower-chat-send", Values: map[string]string{"Format": "Plain"}}}
	w.Links = []*v1.Link{
		{Source: "in", SourceSocket: "Event", Target: "out", TargetSocket: "Event"},
		{Source: "in", SourceSocket: "Text", Target: "out", TargetSocket: "Text"},
		{Source: "in", SourceSocket: "Chat", Target: "out", TargetSocket: "Chat"},
	}
	if _, err := wf.UpdateWorkflow(ctx, connect.NewRequest(&v1.UpdateWorkflowRequest{Workflow: w})); err != nil {
		t.Fatal(err)
	}

	ws, err := c.ListChatWorkflows(ctx, connect.NewRequest(&v1.ListChatWorkflowsRequest{}))
	if err != nil || len(ws.Msg.Workflows) != 1 || ws.Msg.Workflows[0].Name != "Echo" || ws.Msg.Workflows[0].Active {
		t.Fatalf("chat workflows: %v, %v", ws, err)
	}
	if _, err := c.CreateChat(ctx, connect.NewRequest(&v1.CreateChatRequest{WorkflowId: plain.Msg.Workflow.Id})); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Errorf("a chat with a workflow without Tower Chat: %v", err)
	}
	made, err := c.CreateChat(ctx, connect.NewRequest(&v1.CreateChatRequest{WorkflowId: w.Id}))
	if err != nil {
		t.Fatal(err)
	}
	chat := made.Msg.Chat
	if chat.Name != "Echo" || chat.WorkflowName != "Echo" || chat.WorkflowActive {
		t.Errorf("chat: %v", chat)
	}

	send := func(text string) (*connect.Response[v1.SendChatMessageResponse], error) {
		return c.SendChatMessage(ctx, connect.NewRequest(&v1.SendChatMessageRequest{ChatId: chat.Id, Text: text}))
	}
	if _, err := send("hi"); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Errorf("to a workflow switched off: %v", err)
	}
	if _, err := wf.SetWorkflowActive(ctx, connect.NewRequest(&v1.SetWorkflowActiveRequest{Id: w.Id, Active: true})); err != nil {
		t.Fatal(err)
	}
	if _, err := send("  "); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("empty: %v", err)
	}
	sent, err := send("ping")
	if err != nil {
		t.Fatal(err)
	}
	m := sent.Msg.Message
	if m.From != v1.ChatFrom_CHAT_FROM_USER || m.Text != "ping" || len(sent.Msg.Runs) != 1 || len(m.RunIds) != 1 {
		t.Fatalf("sent: %v", sent.Msg)
	}

	// The page asks for what came after its message, watching the run.
	var got *v1.ListChatMessagesResponse
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		r, err := c.ListChatMessages(ctx, connect.NewRequest(&v1.ListChatMessagesRequest{ChatId: chat.Id, AfterId: m.Id, WatchRuns: m.RunIds}))
		if err != nil {
			t.Fatal(err)
		}
		if got = r.Msg; len(got.Runs) == 1 && got.Runs[0].Status != v1.RunStatus_RUN_STATUS_RUNNING {
			break
		}
	}
	if len(got.Messages) != 1 || got.Messages[0].Text != "ping" || got.Messages[0].From != v1.ChatFrom_CHAT_FROM_WORKFLOW ||
		got.Messages[0].Markdown || got.Runs[0].Status != v1.RunStatus_RUN_STATUS_SUCCEEDED || !got.Chat.WorkflowActive {
		t.Fatalf("after: %v", got)
	}
	all, err := c.ListChatMessages(ctx, connect.NewRequest(&v1.ListChatMessagesRequest{ChatId: chat.Id, Limit: 1}))
	if err != nil || len(all.Msg.Messages) != 1 || !all.Msg.More || all.Msg.Messages[0].From != v1.ChatFrom_CHAT_FROM_WORKFLOW {
		t.Errorf("newest 1: %v, %v", all, err)
	}

	if _, err := c.UpdateChat(ctx, connect.NewRequest(&v1.UpdateChatRequest{Id: chat.Id, Name: "Ping pong"})); err != nil {
		t.Fatal(err)
	}
	list, err := c.ListChats(ctx, connect.NewRequest(&v1.ListChatsRequest{}))
	if err != nil || len(list.Msg.Chats) != 1 || list.Msg.Chats[0].Name != "Ping pong" || list.Msg.Chats[0].Last.GetText() != "ping" {
		t.Fatalf("chats: %v, %v", list, err)
	}
	if _, err := c.DeleteChat(ctx, connect.NewRequest(&v1.DeleteChatRequest{Id: chat.Id})); err != nil {
		t.Fatal(err)
	}
	if _, err := c.ListChatMessages(ctx, connect.NewRequest(&v1.ListChatMessagesRequest{ChatId: chat.Id})); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("a deleted chat: %v", err)
	}
}
