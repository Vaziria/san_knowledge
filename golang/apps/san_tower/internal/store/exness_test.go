package store

import (
	"errors"
	"testing"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
)

// rfc8032Key is RFC 8032's public test key (Test 1): not a credential.
const rfc8032Key = "9d61b19deffd5a60ba844af492ec2cc44449c5697b326919703bac031cae7f60" +
	"d75a980182b10ab7d54bfed3c964073a0ee172f3daa62325af021a68f707511a"

func exnessSettings(e *v1.ExnessSettings) *v1.ConnectionSettings {
	return &v1.ConnectionSettings{Type: &v1.ConnectionSettings_Exness{Exness: e}}
}

// An Exness connection is an API key, a private key that parses, an
// account number and a Gateway (or an Access Point), all trimmed; trading
// stays off until asked for. An empty private key on an edit keeps the
// saved one.
func TestExnessConnection(t *testing.T) {
	s := open(t)
	c, err := s.CreateConnection(Connection{Name: "Exness", Settings: exnessSettings(&v1.ExnessSettings{
		ApiKey: " exnsk-abc ", PrivateKey: " " + rfc8032Key + "\n", AccountId: " 123456 ", Gateway: " https://gateway.example/ ", PrivateKeySet: true,
	})})
	if err != nil {
		t.Fatal(err)
	}
	e := c.Settings.GetExness()
	if c.Type != Exness || e.ApiKey != "exnsk-abc" || e.PrivateKey != rfc8032Key || e.AccountId != "123456" || e.Gateway != "https://gateway.example" ||
		e.PrivateKeySet || e.AllowTrading {
		t.Fatalf("connection = %+v", e)
	}
	// An edit without a private key keeps the saved one.
	c.Settings = exnessSettings(&v1.ExnessSettings{ApiKey: "exnsk-abc", AccountId: "123456", AccessPoint: "https://ap.example", AllowTrading: true})
	c, err = s.UpdateConnection(c)
	if err != nil {
		t.Fatal(err)
	}
	if e := c.Settings.GetExness(); e.PrivateKey != rfc8032Key || e.AccessPoint != "https://ap.example" || e.Gateway != "" || !e.AllowTrading {
		t.Fatalf("after the edit = %+v", e)
	}

	good := func() *v1.ExnessSettings {
		return &v1.ExnessSettings{ApiKey: "exnsk-abc", PrivateKey: rfc8032Key, AccountId: "123456", Gateway: "https://gateway.example"}
	}
	for name, change := range map[string]func(*v1.ExnessSettings){
		"no api key":             func(e *v1.ExnessSettings) { e.ApiKey = "" },
		"no private key":         func(e *v1.ExnessSettings) { e.PrivateKey = "" },
		"private key not a key":  func(e *v1.ExnessSettings) { e.PrivateKey = "exnsk-abc" },
		"no account":             func(e *v1.ExnessSettings) { e.AccountId = "" },
		"account not a number":   func(e *v1.ExnessSettings) { e.AccountId = "12a" },
		"no gateway or ap":       func(e *v1.ExnessSettings) { e.Gateway = "" },
		"gateway over http":      func(e *v1.ExnessSettings) { e.Gateway = "http://gateway.example" },
		"gateway with a path":    func(e *v1.ExnessSettings) { e.Gateway = "https://gateway.example/v1" },
		"access point with user": func(e *v1.ExnessSettings) { e.AccessPoint = "https://u:p@ap.example" },
	} {
		set := good()
		change(set)
		if _, err := s.CreateConnection(Connection{Name: "bad " + name, Settings: exnessSettings(set)}); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: %v, want ErrInvalid", name, err)
		}
	}
	// An edit cannot drop the key when none was saved: another type's
	// connection becoming Exness needs one.
	other, err := s.CreateConnection(Connection{Name: "Remote", Settings: sanRemote("http://127.0.0.1:7478")})
	if err != nil {
		t.Fatal(err)
	}
	other.Settings = exnessSettings(&v1.ExnessSettings{ApiKey: "exnsk-abc", AccountId: "123456", Gateway: "https://gateway.example"})
	if _, err := s.UpdateConnection(other); !errors.Is(err, ErrInvalid) {
		t.Fatalf("becoming Exness without a key: %v", err)
	}
}
