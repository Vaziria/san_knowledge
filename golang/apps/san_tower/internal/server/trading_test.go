package server

import (
	"context"
	"os"
	"slices"
	"testing"

	"connectrpc.com/connect"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/gen/san/tower/v1/towerv1connect"
)

// The trading workflows (workflows/trading) import into a tower that has
// what setup.ps1 makes and the connections they name: nothing is left to
// pick, and each Workflow Tool's workflow is there.
func TestImportTradingWorkflows(t *testing.T) {
	ctx := context.Background()
	ts := tower(t)
	conns := towerv1connect.NewConnectionServiceClient(ts.Client(), ts.URL)
	for _, c := range []*v1.Connection{
		{Name: "Binance Tesnet", Settings: &v1.ConnectionSettings{Type: &v1.ConnectionSettings_Binance{Binance: &v1.BinanceSettings{ApiKey: "k", ApiSecret: "s", Network: "testnet"}}}},
		{Name: "Developer Bot", Settings: &v1.ConnectionSettings{Type: &v1.ConnectionSettings_TelegramBot{TelegramBot: &v1.TelegramBotSettings{Token: "123456:ABC-def"}}}},
		{Name: "Telegram Bot", Settings: &v1.ConnectionSettings{Type: &v1.ConnectionSettings_TelegramBot{TelegramBot: &v1.TelegramBotSettings{Token: "654321:ABC-def"}}}},
		{Name: "Tower Local", Settings: sanRemote("http://127.0.0.1:7478")},
	} {
		if _, err := conns.CreateConnection(ctx, connect.NewRequest(&v1.CreateConnectionRequest{Connection: c})); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := towerv1connect.NewKnowledgeServiceClient(ts.Client(), ts.URL).CreateKnowledge(ctx, connect.NewRequest(&v1.CreateKnowledgeRequest{Name: "Trading Journal", Path: "trading-journal"})); err != nil {
		t.Fatal(err)
	}
	file, err := os.ReadFile("../../workflows/trading/trading.workflows.json")
	if err != nil {
		t.Fatal(err)
	}
	wf := towerv1connect.NewWorkflowServiceClient(ts.Client(), ts.URL)
	im, err := wf.ImportWorkflows(ctx, connect.NewRequest(&v1.ImportWorkflowsRequest{Json: string(file)}))
	if err != nil {
		t.Fatal(err)
	}
	if len(im.Msg.Notes) > 0 {
		t.Errorf("notes: %q", im.Msg.Notes)
	}
	var names []string
	for _, s := range im.Msg.Workflows {
		names = append(names, s.Name)
		if s.Active {
			t.Errorf("%s is on", s.Name)
		}
		got, err := wf.GetWorkflow(ctx, connect.NewRequest(&v1.GetWorkflowRequest{Id: s.Id}))
		if err != nil {
			t.Fatal(err)
		}
		for _, st := range got.Msg.Workflow.Steps {
			for _, setting := range []string{"Connection", "Storage", "Knowledge"} {
				if v, ok := st.Values[setting]; ok && (v == "" || v[0] < '1' || v[0] > '9') {
					t.Errorf("%s: %s's %s is %q", s.Name, st.Id, setting, v)
				}
			}
		}
		if !got.Msg.Workflow.GetOneAtATime() {
			t.Errorf("%s does not run one at a time", s.Name)
		}
	}
	if want := []string{"trade open", "trade close", "trade protect", "trade lesson", "trade approach", "trade tick"}; !slices.Equal(names, want) {
		t.Errorf("imported %v", names)
	}
}
