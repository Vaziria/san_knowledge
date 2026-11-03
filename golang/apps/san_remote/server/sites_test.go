package server

import (
	"log/slog"
	"reflect"
	"testing"
)

// The API the tower embeds (its Browser Remote Server) has every site
// service `san_remote api` has: a site added to internal/api but not here
// would answer the tower's steps with a nil service (Shopee, 2026-11-03,
// was missing until the v1.5.0 release check).
func TestAPIHasEverySite(t *testing.T) {
	a := reflect.ValueOf(newAPI("http://127.0.0.1:1", slog.New(slog.DiscardHandler))).Elem()
	for i := 0; i < a.NumField(); i++ {
		f := a.Field(i)
		if f.Kind() == reflect.Pointer && f.IsNil() {
			t.Errorf("API.%s is nil", a.Type().Field(i).Name)
		}
	}
}
