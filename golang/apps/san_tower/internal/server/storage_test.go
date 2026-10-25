package server

import (
	"context"
	"path/filepath"
	"testing"

	"connectrpc.com/connect"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/gen/san/tower/v1/towerv1connect"
)

func TestStorageService(t *testing.T) {
	ts := tower(t)
	ctx := context.Background()
	c := towerv1connect.NewStorageServiceClient(ts.Client(), ts.URL)

	cr, err := c.CreateStorage(ctx, connect.NewRequest(&v1.CreateStorageRequest{Storage: &v1.Storage{Name: "Videos", Path: "videos", AbsolutePath: "ignored"}}))
	if err != nil {
		t.Fatal(err)
	}
	st := cr.Msg.Storage
	list, err := c.ListStorages(ctx, connect.NewRequest(&v1.ListStoragesRequest{}))
	if err != nil || !filepath.IsAbs(list.Msg.DataDir) || len(list.Msg.Storages) != 1 {
		t.Fatalf("list: %v, %v", list, err)
	}
	if want := list.Msg.DataDir + "/videos"; st.AbsolutePath != want || list.Msg.Storages[0].AbsolutePath != want {
		t.Errorf("absolute path %q, listed %q; want %q", st.AbsolutePath, list.Msg.Storages[0].AbsolutePath, want)
	}

	if _, err := c.CreateStorage(ctx, connect.NewRequest(&v1.CreateStorageRequest{Storage: &v1.Storage{Name: "videos", Path: "x"}})); connect.CodeOf(err) != connect.CodeAlreadyExists {
		t.Errorf("same name: %v", err)
	}
	if _, err := c.CreateStorage(ctx, connect.NewRequest(&v1.CreateStorageRequest{Storage: &v1.Storage{Name: "x"}})); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("no path: %v", err)
	}

	abs := filepath.ToSlash(filepath.Join(t.TempDir(), "out"))
	st.Path = abs
	up, err := c.UpdateStorage(ctx, connect.NewRequest(&v1.UpdateStorageRequest{Storage: st}))
	if err != nil || up.Msg.Storage.Path != abs || up.Msg.Storage.AbsolutePath != abs {
		t.Fatalf("update: %v, %v", up, err)
	}
	if _, err := c.DeleteStorage(ctx, connect.NewRequest(&v1.DeleteStorageRequest{Id: st.Id})); err != nil {
		t.Fatal(err)
	}
	if _, err := c.DeleteStorage(ctx, connect.NewRequest(&v1.DeleteStorageRequest{Id: st.Id})); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("delete twice: %v", err)
	}
}
