package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	chatgptv1 "github.com/wargasipil/san_remote/gen/san/remote/chatgpt/v1"
	"github.com/wargasipil/san_remote/gen/san/remote/chatgpt/v1/chatgptv1connect"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/gen/san/tower/v1/towerv1connect"
)

// slowChat is ChatGPT asking for one slow script, then done.
type slowChat struct {
	chatgptv1connect.UnimplementedChatGPTServiceHandler
	mu    sync.Mutex
	asked int
}

func (f *slowChat) Ask(context.Context, *connect.Request[chatgptv1.AskRequest]) (*connect.Response[chatgptv1.AskResponse], error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.asked++
	answer := `{"execute": "echo started; sleep 3; echo ended"}`
	if f.asked > 1 {
		answer = `{"chat": "done"}`
	}
	return connect.NewResponse(&chatgptv1.AskResponse{Text: answer, Chat: "https://chatgpt.com/c/live"}), nil
}

// While a Remote Shell run goes on, the Logs pages get its lines as they
// come, a script's output too; once it ends, its record's.
func TestListRunsLive(t *testing.T) {
	mux := http.NewServeMux()
	mux.Handle(chatgptv1connect.NewChatGPTServiceHandler(&slowChat{}))
	remote := httptest.NewServer(mux)
	defer remote.Close()
	ts := tower(t)
	ctx := context.Background()
	conns := towerv1connect.NewConnectionServiceClient(ts.Client(), ts.URL)
	cr, err := conns.CreateConnection(ctx, connect.NewRequest(&v1.CreateConnectionRequest{Connection: &v1.Connection{
		Name: "remote", Settings: &v1.ConnectionSettings{Type: &v1.ConnectionSettings_SanRemote{SanRemote: &v1.SanRemoteSettings{Url: remote.URL}}},
	}}))
	if err != nil {
		t.Fatal(err)
	}
	c := towerv1connect.NewWorkflowServiceClient(ts.Client(), ts.URL)
	w, err := c.CreateWorkflow(ctx, connect.NewRequest(&v1.CreateWorkflowRequest{Name: "shell"}))
	if err != nil {
		t.Fatal(err)
	}
	wf := w.Msg.Workflow
	wf.Steps = []*v1.Step{
		{Id: "go", Kind: "manual"},
		{Id: "shell", Kind: "remote-shell", Values: map[string]string{"Connection": strconv.FormatUint(uint64(cr.Msg.Connection.Id), 10), "Message": "go"}},
	}
	wf.Links = []*v1.Link{{Source: "go", SourceSocket: "Event", Target: "shell", TargetSocket: "Event"}}
	if _, err := c.UpdateWorkflow(ctx, connect.NewRequest(&v1.UpdateWorkflowRequest{Workflow: wf})); err != nil {
		t.Fatal(err)
	}

	ran := make(chan error, 1)
	go func() {
		r, err := c.RunWorkflow(ctx, connect.NewRequest(&v1.RunWorkflowRequest{WorkflowId: wf.Id, StepId: "go"}))
		if err == nil && r.Msg.Run.Status != v1.RunStatus_RUN_STATUS_SUCCEEDED {
			err = &runError{r.Msg.Run.Error}
		}
		ran <- err
	}()
	var seen []string
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(100 * time.Millisecond) {
		res, err := c.ListRuns(ctx, connect.NewRequest(&v1.ListRunsRequest{WorkflowId: wf.Id, Logs: true}))
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Msg.Runs) == 1 && res.Msg.Runs[0].Status == v1.RunStatus_RUN_STATUS_RUNNING {
			seen = res.Msg.Runs[0].Logs
			if slices.Contains(seen, "started") {
				break
			}
		}
	}
	if !slices.Contains(seen, "started") || slices.Contains(seen, "ended") {
		t.Errorf("a running run's lines: %q", seen)
	}
	if err := <-ran; err != nil {
		if strings.Contains(err.Error(), "Bash is not") || strings.Contains(err.Error(), "bash is not") {
			t.Skip(err)
		}
		t.Fatal(err)
	}
	res, err := c.ListRuns(ctx, connect.NewRequest(&v1.ListRunsRequest{WorkflowId: wf.Id, Logs: true}))
	if err != nil || len(res.Msg.Runs) != 1 || !slices.Contains(res.Msg.Runs[0].Logs, "ended") {
		t.Errorf("the finished run's lines: %v, %v", res.Msg.GetRuns(), err)
	}
}

type runError struct{ msg string }

func (e *runError) Error() string { return e.msg }
