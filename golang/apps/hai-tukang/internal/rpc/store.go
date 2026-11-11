package rpc

import (
	"context"

	"connectrpc.com/connect"
	v1 "github.com/wargasipil/hai-tukang/gen/haitukang/v1"
	"github.com/wargasipil/hai-tukang/internal/accounts"
	"github.com/wargasipil/hai-tukang/internal/photos"
	"github.com/wargasipil/hai-tukang/internal/stores"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Stores is what the services need of stores.Service.
type Stores interface {
	SaveProfile(ctx context.Context, a accounts.Account, p stores.Profile) (stores.Profile, error)
	Mine(ctx context.Context, a accounts.Account) (*stores.Profile, []stores.Product, error)
	StoreOf(ctx context.Context, storeID int64) (*stores.Profile, []stores.Product, error)
	SaveProduct(ctx context.Context, a accounts.Account, productID int64, in stores.ProductInput) (stores.Product, error)
	DeleteProduct(ctx context.Context, a accounts.Account, productID int64) error
	AddPhoto(ctx context.Context, a accounts.Account, productID int64, data []byte) (stores.Product, error)
	DeletePhoto(ctx context.Context, a accounts.Account, productID, photoID int64) (stores.Product, error)
	Search(ctx context.Context, f stores.Filter) ([]stores.Listing, error)
	Get(ctx context.Context, productID int64) (stores.Listing, error)
}

type storeServer struct {
	deps     Deps
	sessions sessions
}

func (s *storeServer) GetMyStore(ctx context.Context, req *connect.Request[v1.GetMyStoreRequest]) (*connect.Response[v1.GetMyStoreResponse], error) {
	a, err := s.sessions.account(ctx, req.Header())
	if err != nil {
		return nil, err
	}
	profile, list, err := s.deps.Stores.Mine(ctx, a)
	if err != nil {
		return nil, rpcError(err)
	}
	return connect.NewResponse(myStoreToProto(profile, list)), nil
}

func myStoreToProto(profile *stores.Profile, list []stores.Product) *v1.GetMyStoreResponse {
	out := &v1.GetMyStoreResponse{Products: make([]*v1.Product, 0, len(list))}
	if profile != nil {
		out.Profile = storeProfileToProto(*profile)
	}
	for _, p := range list {
		out.Products = append(out.Products, productToProto(p))
	}
	return out
}

func (s *storeServer) SaveStoreProfile(ctx context.Context, req *connect.Request[v1.SaveStoreProfileRequest]) (*connect.Response[v1.SaveStoreProfileResponse], error) {
	a, err := s.sessions.account(ctx, req.Header())
	if err != nil {
		return nil, err
	}
	in := req.Msg.GetProfile()
	saved, err := s.deps.Stores.SaveProfile(ctx, a, stores.Profile{
		Name: in.GetName(), Area: in.GetArea(), Address: in.GetAddress(), Pickup: in.GetPickup(), Delivers: in.GetDelivers(),
		DeliveryFee: in.GetDeliveryFee(), DeliveryAreas: in.GetDeliveryAreas(),
	})
	if err != nil {
		return nil, rpcError(err)
	}
	return connect.NewResponse(&v1.SaveStoreProfileResponse{Profile: storeProfileToProto(saved)}), nil
}

func (s *storeServer) SaveProduct(ctx context.Context, req *connect.Request[v1.SaveProductRequest]) (*connect.Response[v1.SaveProductResponse], error) {
	a, err := s.sessions.account(ctx, req.Header())
	if err != nil {
		return nil, err
	}
	d := req.Msg.GetDetails()
	saved, err := s.deps.Stores.SaveProduct(ctx, a, req.Msg.GetProductId(), stores.ProductInput{
		Name: d.GetName(), Service: d.GetService(), Price: d.GetPrice(), Unit: d.GetUnit(), Available: d.GetAvailable(), Description: d.GetDescription(),
	})
	if err != nil {
		return nil, rpcError(err)
	}
	return connect.NewResponse(&v1.SaveProductResponse{Product: productToProto(saved)}), nil
}

func (s *storeServer) DeleteProduct(ctx context.Context, req *connect.Request[v1.DeleteProductRequest]) (*connect.Response[v1.DeleteProductResponse], error) {
	a, err := s.sessions.account(ctx, req.Header())
	if err != nil {
		return nil, err
	}
	if err := s.deps.Stores.DeleteProduct(ctx, a, req.Msg.GetProductId()); err != nil {
		return nil, rpcError(err)
	}
	return connect.NewResponse(&v1.DeleteProductResponse{}), nil
}

func (s *storeServer) AddProductPhoto(ctx context.Context, req *connect.Request[v1.AddProductPhotoRequest]) (*connect.Response[v1.AddProductPhotoResponse], error) {
	a, err := s.sessions.account(ctx, req.Header())
	if err != nil {
		return nil, err
	}
	saved, err := s.deps.Stores.AddPhoto(ctx, a, req.Msg.GetProductId(), req.Msg.GetData())
	if err != nil {
		return nil, rpcError(err)
	}
	return connect.NewResponse(&v1.AddProductPhotoResponse{Product: productToProto(saved)}), nil
}

func (s *storeServer) DeleteProductPhoto(ctx context.Context, req *connect.Request[v1.DeleteProductPhotoRequest]) (*connect.Response[v1.DeleteProductPhotoResponse], error) {
	a, err := s.sessions.account(ctx, req.Header())
	if err != nil {
		return nil, err
	}
	saved, err := s.deps.Stores.DeletePhoto(ctx, a, req.Msg.GetProductId(), req.Msg.GetPhotoId())
	if err != nil {
		return nil, rpcError(err)
	}
	return connect.NewResponse(&v1.DeleteProductPhotoResponse{Product: productToProto(saved)}), nil
}

func (s *storeServer) SearchProducts(ctx context.Context, req *connect.Request[v1.SearchProductsRequest]) (*connect.Response[v1.SearchProductsResponse], error) {
	if _, err := s.sessions.account(ctx, req.Header()); err != nil {
		return nil, err
	}
	list, err := s.deps.Stores.Search(ctx, stores.Filter{Service: req.Msg.GetService(), Area: req.Msg.GetArea(), Query: req.Msg.GetQuery()})
	if err != nil {
		return nil, rpcError(err)
	}
	out := &v1.SearchProductsResponse{Listings: make([]*v1.ProductListing, 0, len(list))}
	for _, l := range list {
		out.Listings = append(out.Listings, listingToProto(l))
	}
	return connect.NewResponse(out), nil
}

func (s *storeServer) GetProduct(ctx context.Context, req *connect.Request[v1.GetProductRequest]) (*connect.Response[v1.GetProductResponse], error) {
	if _, err := s.sessions.account(ctx, req.Header()); err != nil {
		return nil, err
	}
	l, err := s.deps.Stores.Get(ctx, req.Msg.GetProductId())
	if err != nil {
		return nil, rpcError(err)
	}
	return connect.NewResponse(&v1.GetProductResponse{Listing: listingToProto(l)}), nil
}

func storeProfileToProto(p stores.Profile) *v1.StoreProfile {
	return &v1.StoreProfile{
		Name: p.Name, Area: p.Area, Address: p.Address, Pickup: p.Pickup, Delivers: p.Delivers,
		DeliveryFee: p.DeliveryFee, DeliveryAreas: p.DeliveryAreas,
	}
}

func productToProto(p stores.Product) *v1.Product {
	out := &v1.Product{
		Id: p.ID, StoreId: p.StoreID, Hidden: p.Hidden, UpdatedAt: timestamppb.New(p.UpdatedAt),
		Details: &v1.ProductDetails{Name: p.Name, Service: p.Service, Price: p.Price, Unit: p.Unit, Available: p.Available, Description: p.Description},
		Photos:  make([]*v1.ProductPhoto, 0, len(p.Photos)),
	}
	for _, ph := range p.Photos {
		out.Photos = append(out.Photos, &v1.ProductPhoto{Id: ph.ID, Url: photos.URL(ph.Key), Width: int32(ph.Width), Height: int32(ph.Height)})
	}
	return out
}

func listingToProto(l stores.Listing) *v1.ProductListing {
	return &v1.ProductListing{
		Product: productToProto(l.Product),
		Store:   &v1.Store{Id: l.Store.AccountID, Phone: l.Store.Phone, Profile: storeProfileToProto(l.Store.Profile)},
	}
}
