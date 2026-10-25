package server

import (
	"context"
	"testing"

	"connectrpc.com/connect"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/gen/san/tower/v1/towerv1connect"
)

// The Browser Remote Server page: off at first, with the URIs it will use;
// switched on, it runs (no browser attached yet); switched off, it stops.
func TestBrowserRemote(t *testing.T) {
	ts := tower(t)
	ctx := context.Background()
	c := towerv1connect.NewBrowserRemoteServiceClient(ts.Client(), ts.URL)

	got, err := c.GetBrowserRemote(ctx, connect.NewRequest(&v1.GetBrowserRemoteRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	if r := got.Msg.Remote; r.Active || r.Running || r.ServerUri == "" || r.ApiUri == "" {
		t.Fatalf("at first = %v", r)
	}

	on, err := c.SetBrowserRemoteActive(ctx, connect.NewRequest(&v1.SetBrowserRemoteActiveRequest{Active: true}))
	if err != nil {
		t.Fatal(err)
	}
	if r := on.Msg.Remote; !r.Active || !r.Running || r.StartedAt == nil || r.Error != "" || len(r.Browsers) != 0 {
		t.Fatalf("on = %v", r)
	}

	off, err := c.SetBrowserRemoteActive(ctx, connect.NewRequest(&v1.SetBrowserRemoteActiveRequest{Active: false}))
	if err != nil {
		t.Fatal(err)
	}
	if r := off.Msg.Remote; r.Active || r.Running || r.StartedAt != nil {
		t.Fatalf("off = %v", r)
	}
}
