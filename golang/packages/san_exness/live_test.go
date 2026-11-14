package san_exness

import (
	"os"
	"testing"
)

// TestLiveReadOnly reads a real account, never trades. It runs only with
// EXN_LIVE=1 and the account's EXN_API_KEY, EXN_PRIVATE_KEY (or
// EXN_PRIVATE_KEY_FILE), EXN_ACCOUNT_ID and EXN_DISCOVERY_BASE_URL set:
//
//	EXN_LIVE=1 go test -run Live -v .
func TestLiveReadOnly(t *testing.T) {
	if os.Getenv("EXN_LIVE") != "1" {
		t.Skip("set EXN_LIVE=1 and the EXN_* variables to read a real account")
	}
	key := os.Getenv("EXN_PRIVATE_KEY")
	if path := os.Getenv("EXN_PRIVATE_KEY_FILE"); path != "" {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		key = string(b)
	}
	c, err := New(Config{
		APIKey:      os.Getenv("EXN_API_KEY"),
		PrivateKey:  key,
		AccountID:   os.Getenv("EXN_ACCOUNT_ID"),
		Gateway:     os.Getenv("EXN_DISCOVERY_BASE_URL"),
		AccessPoint: os.Getenv("EXN_API_BASE_URL"),
	})
	if err != nil {
		t.Fatal(err)
	}
	ap, err := c.AccessPoint(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("access point %s", ap)
	a, err := c.Account(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("account %s: %s, leverage %s, %s", a.AccountID, a.Settings.Currency, a.Settings.Leverage, a.Settings.AccountStatus)
	names, err := c.Instruments(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%d instruments", len(names))
	if _, err := c.Limits(ctx); err != nil {
		t.Fatal(err)
	}
	s, err := c.Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%d open positions, %d pending orders, equity %s", len(s.Positions), len(s.Orders), s.AccountState.Equity)
}
