package san_youtube

import "testing"

func TestParseAmount(t *testing.T) {
	tests := []struct {
		text     string
		value    float64
		currency string
	}{
		// What YouTube shows in English and in Indonesian.
		{"IDR 159,000", 159000, "IDR"},
		{"Rp 20.000", 20000, "IDR"},
		{"Rp20.000", 20000, "IDR"},
		{"Rp 1.500.000", 1500000, "IDR"},
		{"Rp 20.000,00", 20000, "IDR"},
		{"$5.00", 5, "USD"},
		{"US$5.00", 5, "USD"},
		{"$1,000.00", 1000, "USD"},
		{"$1,000", 1000, "USD"},
		{"CA$10.00", 10, "CAD"},
		{"A$2.99", 2.99, "AUD"},
		{"NT$75", 75, "TWD"},
		{"€5.00", 5, "EUR"},
		{"5,00 €", 5, "EUR"},
		{"1.234,56 €", 1234.56, "EUR"},
		{"£1,234.56", 1234.56, "GBP"},
		{"¥1,000", 1000, "JPY"},
		{"₩10,000", 10000, "KRW"},
		{"₹1,00,000.00", 100000, "INR"},
		{"₱100.00", 100, "PHP"},
		{"PHP 100.00", 100, "PHP"},
		{"RM5.00", 5, "MYR"},
		{"MYR 5.00", 5, "MYR"},
		{"SGD 2.00", 2, "SGD"},
		{"R$ 10,00", 10, "BRL"},
		{"CHF 5.00", 5, "CHF"},
		{"1\u00a0000,00\u00a0zł", 1000, "PLN"},
		{"BHD 1.500", 1.5, "BHD"},
		// Direction marks from right-to-left languages are invisible.
		{"\u200f1,500.00\u00a0₪", 1500, "ILS"},
		// Ambiguous symbols keep the value and leave the currency empty.
		{"kr 50", 50, ""},
		{"50,00 kr", 50, ""},
		// Nothing to read.
		{"", 0, ""},
		{"free", 0, ""},
	}
	for _, tt := range tests {
		got := ParseAmount(tt.text)
		if got.Text != tt.text || got.Value != tt.value || got.Currency != tt.currency {
			t.Errorf("ParseAmount(%q) = %+v, want value %v currency %q", tt.text, got, tt.value, tt.currency)
		}
	}
}
