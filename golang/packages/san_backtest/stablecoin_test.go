package san_backtest

import "testing"

// BTCUSDT on Exness makes profit in "TET" (Tether); a USD account takes it
// 1:1 instead of refusing the instrument.
func TestUSDStableQuote(t *testing.T) {
	for _, q := range []string{"USDT", "TET", "USDC"} {
		if !usdStable[q] {
			t.Fatalf("%s is not taken as USD", q)
		}
	}
	if usdStable["EUR"] {
		t.Fatal("EUR taken as USD")
	}
}
