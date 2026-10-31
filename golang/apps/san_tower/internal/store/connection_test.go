package store

import (
	"errors"
	"strings"
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

// A Binance connection needs a key and its secret; its network is testnet
// unless it says demo or live.
func TestBinanceConnection(t *testing.T) {
	s := open(t)
	binance := func(key, secret, network string) *v1.ConnectionSettings {
		return &v1.ConnectionSettings{Type: &v1.ConnectionSettings_Binance{Binance: &v1.BinanceSettings{ApiKey: key, ApiSecret: secret, Network: network}}}
	}
	c, err := s.CreateConnection(Connection{Name: "Binance test", Settings: binance(" key ", "secret", "")})
	if err != nil {
		t.Fatal(err)
	}
	if b := c.Settings.GetBinance(); c.Type != Binance || b.ApiKey != "key" || b.Network != BinanceTestnet {
		t.Errorf("connection = %+v", c)
	}
	if c, err := s.CreateConnection(Connection{Name: "Binance live", Settings: binance("k", "s", "Live")}); err != nil || c.Settings.GetBinance().Network != BinanceLive {
		t.Errorf("live: %+v, %v", c, err)
	}
	for _, bad := range []*v1.ConnectionSettings{binance("key", "", "testnet"), binance("", "secret", ""), binance("key", "secret", "mainnet")} {
		if _, err := s.CreateConnection(Connection{Name: "bad", Settings: bad}); !errors.Is(err, ErrInvalid) {
			t.Errorf("%v: %v, want ErrInvalid", bad, err)
		}
	}
}

// A GitHub connection is a token in GitHub's shape, trimmed.
func TestGitHubConnection(t *testing.T) {
	s := open(t)
	github := func(token string) *v1.ConnectionSettings {
		return &v1.ConnectionSettings{Type: &v1.ConnectionSettings_Github{Github: &v1.GitHubSettings{Token: token}}}
	}
	classic := "ghp_" + strings.Repeat("a1B2", 9)
	c, err := s.CreateConnection(Connection{Name: "GitHub", Settings: github(" " + classic + "\n")})
	if err != nil {
		t.Fatal(err)
	}
	if c.Type != GitHub || c.Settings.GetGithub().Token != classic {
		t.Errorf("connection = %+v", c)
	}
	if list, err := s.ListConnections(GitHub); err != nil || len(list) != 1 {
		t.Errorf("list: %v, %v", list, err)
	}
	if _, err := s.CreateConnection(Connection{Name: "fine", Settings: github("github_pat_" + strings.Repeat("x", 82))}); err != nil {
		t.Errorf("fine-grained: %v", err)
	}
	for _, bad := range []string{"", "ghp_short", "ghp_" + strings.Repeat("a", 30) + " b", "Bearer ghp_" + strings.Repeat("a", 36)} {
		if _, err := s.CreateConnection(Connection{Name: "bad", Settings: github(bad)}); !errors.Is(err, ErrInvalid) {
			t.Errorf("%q: %v, want ErrInvalid", bad, err)
		}
	}
}

// An SSH connection needs a host, a user, and a key that reads or a
// password; its port is 22 when not given. The host key is kept once, and
// another host clears it.
func TestSSHConnection(t *testing.T) {
	s := open(t)
	sshc := func(m *v1.SSHSettings) *v1.ConnectionSettings {
		return &v1.ConnectionSettings{Type: &v1.ConnectionSettings_Ssh{Ssh: m}}
	}
	c, err := s.CreateConnection(Connection{Name: "Pi", Settings: sshc(&v1.SSHSettings{Host: " 192.168.1.5 ", User: "pi", Password: "secret"})})
	if err != nil {
		t.Fatal(err)
	}
	if m := c.Settings.GetSsh(); c.Type != SSH || m.Host != "192.168.1.5" || m.Port != 22 {
		t.Errorf("connection = %+v", c)
	}
	const key = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOMqqnkVzrm0SdG6UOoqKLsabgH5C9okWi0dh2l9GKJl"
	if err := s.KeepSSHHostKey(c.ID, key); err != nil {
		t.Fatal(err)
	}
	if err := s.KeepSSHHostKey(c.ID, "ssh-ed25519 other"); err != nil {
		t.Fatal(err)
	}
	c, _ = s.GetConnection(c.ID)
	if got := c.Settings.GetSsh().HostKey; got != key {
		t.Errorf("kept %q", got)
	}
	// A new name keeps it; a new host does not.
	c.Name = "Raspberry Pi"
	if c, err = s.UpdateConnection(c); err != nil || c.Settings.GetSsh().HostKey != key {
		t.Errorf("renamed: %v, %v", c.Settings.GetSsh(), err)
	}
	c.Settings.GetSsh().Host = "raspi.local"
	if c, err = s.UpdateConnection(c); err != nil || c.Settings.GetSsh().HostKey != "" {
		t.Errorf("new host: %v, %v", c.Settings.GetSsh(), err)
	}

	for _, bad := range []*v1.SSHSettings{
		{User: "pi", Password: "x"},
		{Host: "pi@raspi", User: "pi", Password: "x"},
		{Host: "raspi", Password: "x"},
		{Host: "raspi", User: "pi"},
		{Host: "raspi", User: "pi", Port: 70000, Password: "x"},
		{Host: "raspi", User: "pi", PrivateKey: "not a key"},
		{Host: "raspi", User: "pi", Password: "x", HostKey: "nonsense"},
	} {
		if _, err := s.CreateConnection(Connection{Name: "bad", Settings: sshc(bad)}); !errors.Is(err, ErrInvalid) {
			t.Errorf("%v: %v, want ErrInvalid", bad, err)
		}
	}
}
