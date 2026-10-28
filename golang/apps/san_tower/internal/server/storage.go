package server

import (
	"context"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/store"
)

// StorageService keeps the storages (named base folders) that workflow
// steps use.

func (s *Server) ListStorages(context.Context, *connect.Request[v1.ListStoragesRequest]) (*connect.Response[v1.ListStoragesResponse], error) {
	list, err := s.store.ListStorages()
	if err != nil {
		return nil, storeError(err)
	}
	res := &v1.ListStoragesResponse{Storages: make([]*v1.Storage, len(list)), DataDir: s.store.Dir()}
	for i, st := range list {
		res.Storages[i] = storageToProto(st)
	}
	return connect.NewResponse(res), nil
}

func (s *Server) CreateStorage(_ context.Context, req *connect.Request[v1.CreateStorageRequest]) (*connect.Response[v1.CreateStorageResponse], error) {
	st, err := s.store.CreateStorage(storageFromProto(req.Msg.GetStorage()))
	if err != nil {
		return nil, storeError(err)
	}
	return connect.NewResponse(&v1.CreateStorageResponse{Storage: storageToProto(st)}), nil
}

func (s *Server) UpdateStorage(_ context.Context, req *connect.Request[v1.UpdateStorageRequest]) (*connect.Response[v1.UpdateStorageResponse], error) {
	st, err := s.store.UpdateStorage(storageFromProto(req.Msg.GetStorage()))
	if err != nil {
		return nil, storeError(err)
	}
	return connect.NewResponse(&v1.UpdateStorageResponse{Storage: storageToProto(st)}), nil
}

func (s *Server) DeleteStorage(_ context.Context, req *connect.Request[v1.DeleteStorageRequest]) (*connect.Response[v1.DeleteStorageResponse], error) {
	if err := s.store.DeleteStorage(req.Msg.GetId()); err != nil {
		return nil, storeError(err)
	}
	return connect.NewResponse(&v1.DeleteStorageResponse{}), nil
}

func storageToProto(st store.Storage) *v1.Storage {
	return &v1.Storage{
		Id:           st.ID,
		Name:         st.Name,
		Path:         st.Path,
		AbsolutePath: st.Abs,
		Knowledge:    st.Knowledge,
		CreatedAt:    timestamppb.New(st.CreatedAt),
		UpdatedAt:    timestamppb.New(st.UpdatedAt),
	}
}

func storageFromProto(p *v1.Storage) store.Storage {
	return store.Storage{ID: p.GetId(), Name: p.GetName(), Path: p.GetPath()}
}
