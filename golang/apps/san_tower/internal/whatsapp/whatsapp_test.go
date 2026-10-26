package whatsapp

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The devices' database opens (whatsmeow's tables on the pure-Go SQLite),
// and a device that is not linked does not connect.
func TestStore(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), FileName("My WhatsApp"))
	s, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Connect(ctx, "6281234567890:3@s.whatsapp.net"); err == nil || !strings.Contains(err.Error(), "no longer linked") {
		t.Errorf("an unknown device: %v", err)
	}
	if err := s.Forget(ctx, "6281234567890:3@s.whatsapp.net"); err != nil {
		t.Errorf("forget an unknown device: %v", err)
	}
	s.Close()
	if err := RemoveFiles(path); err != nil {
		t.Fatal(err)
	}
	if left, _ := filepath.Glob(path + "*"); len(left) != 0 {
		t.Errorf("left: %v", left)
	}
}

// A connection's database is named by the SHA-256 of its name.
func TestFileName(t *testing.T) {
	if got := FileName(" abc "); got != "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad.db" {
		t.Errorf("FileName(abc) = %s", got)
	}
	if FileName("My WhatsApp") == FileName("My Whatsapp") {
		t.Error("two names, one file")
	}
}

// TestPairLive asks WhatsApp for a QR code, then stops: nothing is linked.
// It reaches WhatsApp's servers, so it runs only with
// SAN_TOWER_WHATSAPP_LIVE=1.
func TestPairLive(t *testing.T) {
	if os.Getenv("SAN_TOWER_WHATSAPP_LIVE") == "" {
		t.Skip("set SAN_TOWER_WHATSAPP_LIVE=1 to ask WhatsApp for a QR code")
	}
	s, err := Open(context.Background(), filepath.Join(t.TempDir(), FileName("Live")))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var code string
	_, err = s.Pair(ctx, func(c string) {
		code = c
		cancel() // one is enough
	})
	if code == "" || !errors.Is(err, context.Canceled) {
		t.Fatalf("code %q, err %v", code, err)
	}
	t.Logf("WhatsApp gave a QR code of %d characters", len(code))
}

func TestParseChat(t *testing.T) {
	for in, want := range map[string]string{
		"6281234567890":                "6281234567890@s.whatsapp.net",
		"+62 812-3456-7890":            "6281234567890@s.whatsapp.net",
		"(62) 812.3456.7890":           "6281234567890@s.whatsapp.net",
		"120363012345678901@g.us":      "120363012345678901@g.us",
		"6281234567890@s.whatsapp.net": "6281234567890@s.whatsapp.net",
	} {
		if j, err := ParseChat(in); err != nil || j.String() != want {
			t.Errorf("ParseChat(%q) = %v, %v; want %s", in, j, err, want)
		}
	}
	for _, in := range []string{"", "  ", "budi", "0812 abc"} {
		if _, err := ParseChat(in); err == nil {
			t.Errorf("ParseChat(%q) took it", in)
		}
	}
}
