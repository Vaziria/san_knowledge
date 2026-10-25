package store

import (
	"errors"
	"testing"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
)

func sanRemote(url string) *v1.ConnectionSettings {
	return &v1.ConnectionSettings{Type: &v1.ConnectionSettings_SanRemote{SanRemote: &v1.SanRemoteSettings{Url: url}}}
}

func TestConnections(t *testing.T) {
	s := open(t)
	c, err := s.CreateConnection(Connection{Name: " My Chrome ", Settings: sanRemote(" http://127.0.0.1:7478/ ")})
	if err != nil {
		t.Fatal(err)
	}
	if c.ID == 0 || c.Name != "My Chrome" || c.Type != SanRemote || c.Settings.GetSanRemote().Url != "http://127.0.0.1:7478" {
		t.Errorf("created %+v", c)
	}

	for name, bad := range map[string]Connection{
		"no name":   {Settings: sanRemote("http://127.0.0.1:7478")},
		"no type":   {Name: "x", Settings: &v1.ConnectionSettings{}},
		"bad url":   {Name: "x", Settings: sanRemote("127.0.0.1:7478")},
		"empty url": {Name: "x", Settings: sanRemote("")},
	} {
		if _, err := s.CreateConnection(bad); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if _, err := s.CreateConnection(Connection{Name: "MY CHROME", Settings: sanRemote("http://127.0.0.1:1")}); !errors.Is(err, ErrExists) {
		t.Errorf("same name: %v", err)
	}

	c.Settings = sanRemote("http://localhost:9000")
	got, err := s.UpdateConnection(c)
	if err != nil || got.Settings.GetSanRemote().Url != "http://localhost:9000" {
		t.Errorf("updated %+v, %v", got, err)
	}
	if list, err := s.ListConnections(SanRemote); err != nil || len(list) != 1 {
		t.Errorf("san_remote list = %+v, %v", list, err)
	}
	if list, _ := s.ListConnections("other"); len(list) != 0 {
		t.Errorf("other type: %+v", list)
	}

	if err := s.DeleteConnection(c.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetConnection(c.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("get deleted: %v", err)
	}
	if _, err := s.UpdateConnection(c); !errors.Is(err, ErrNotFound) {
		t.Errorf("update deleted: %v", err)
	}
}
