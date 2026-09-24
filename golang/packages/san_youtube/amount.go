package san_youtube

import (
	"strconv"
	"strings"
	"unicode"
)

// Amount is what a Super Chat or Super Sticker cost.
type Amount struct {
	// Text is the amount as YouTube shows it, e.g. "Rp 20.000", "IDR 159,000"
	// or "$5.00". Its format follows the language the chat was read in.
	Text string `json:"text"`
	// Value is the number in Text, 0 if it could not be read. It is a float
	// because the amounts are display values, not ledger entries.
	Value float64 `json:"value,omitempty"`
	// Currency is the ISO 4217 code, e.g. "IDR" or "USD", or empty when the
	// symbol is ambiguous or unknown ("kr" could be four currencies).
	Currency string `json:"currency,omitempty"`
}

// currencySymbols maps what YouTube writes before or after the number to an
// ISO 4217 code. Three-letter codes ("IDR 159,000") need no entry. A plain "$"
// is USD because YouTube prefixes every other dollar ("CA$", "A$", "NT$").
var currencySymbols = map[string]string{
	"$": "USD", "US$": "USD",
	"CA$": "CAD", "C$": "CAD",
	"A$": "AUD", "AU$": "AUD",
	"NZ$": "NZD", "HK$": "HKD", "NT$": "TWD", "MX$": "MXN", "S$": "SGD", "R$": "BRL",
	"€": "EUR", "£": "GBP", "CHF": "CHF",
	"¥": "JPY", "JP¥": "JPY", "￥": "JPY", "CN¥": "CNY",
	"₩": "KRW", "₹": "INR", "₱": "PHP", "₫": "VND", "₽": "RUB", "₺": "TRY",
	"₪": "ILS", "₴": "UAH", "₦": "NGN", "₸": "KZT", "₡": "CRC", "฿": "THB",
	"Rp": "IDR", "RM": "MYR", "zł": "PLN", "Kč": "CZK", "Ft": "HUF",
	"R": "ZAR", "E£": "EGP", "S/": "PEN", "lei": "RON", "лв.": "BGN",
}

// threeDecimals are the currencies whose minor unit is a thousandth, where
// "1.500" is one and a half rather than fifteen hundred.
var threeDecimals = map[string]bool{"BHD": true, "IQD": true, "JOD": true, "KWD": true, "LYD": true, "OMR": true, "TND": true}

// ParseAmount reads the value and currency out of an amount as YouTube
// displays it. It understands both separator conventions ("1,234.56" and
// "1.234,56") and treats a lone separator followed by exactly three digits as
// grouping, which is what "Rp 20.000" and "IDR 159,000" need.
func ParseAmount(text string) Amount {
	a := Amount{Text: text}

	// Spaces of every width become one kind, and invisible marks go: YouTube
	// puts direction marks around amounts in right-to-left languages.
	s := strings.TrimSpace(strings.Map(func(r rune) rune {
		switch {
		case unicode.Is(unicode.Cf, r):
			return -1
		case unicode.IsSpace(r):
			return ' '
		}
		return r
	}, text))

	first := strings.IndexFunc(s, isDigit)
	last := strings.LastIndexFunc(s, isDigit)
	if first < 0 {
		return a
	}
	number := s[first : last+1]
	sym := strings.TrimSpace(s[:first])
	if sym == "" {
		sym = strings.TrimSpace(s[last+1:])
	}
	a.Currency = currencyCode(sym)

	if v, ok := parseNumber(number, threeDecimals[a.Currency]); ok {
		a.Value = v
	}
	return a
}

func currencyCode(sym string) string {
	if code, ok := currencySymbols[sym]; ok {
		return code
	}
	if len(sym) == 3 && strings.IndexFunc(sym, func(r rune) bool { return r < 'A' || r > 'Z' }) < 0 {
		return sym
	}
	return ""
}

// parseNumber turns a displayed number into a float. The decimal separator is
// whichever of '.' and ',' comes last when both appear; when only one kind
// appears it is grouping if it repeats or is followed by exactly three digits.
func parseNumber(s string, thousandths bool) (float64, bool) {
	s = strings.NewReplacer(" ", "", "'", "", "’", "").Replace(s)
	for _, r := range s {
		if !isDigit(r) && r != '.' && r != ',' {
			return 0, false
		}
	}

	dot, comma := strings.LastIndexByte(s, '.'), strings.LastIndexByte(s, ',')
	decimal := -1
	switch {
	case dot >= 0 && comma >= 0:
		decimal = max(dot, comma)
	case dot >= 0 || comma >= 0:
		sep := max(dot, comma)
		if strings.Count(s, s[sep:sep+1]) == 1 && (len(s)-sep-1 != 3 || thousandths) {
			decimal = sep
		}
	}

	var b strings.Builder
	for i, r := range s {
		switch {
		case i == decimal:
			b.WriteByte('.')
		case isDigit(r):
			b.WriteRune(r)
		}
	}
	v, err := strconv.ParseFloat(b.String(), 64)
	return v, err == nil
}

func isDigit(r rune) bool { return r >= '0' && r <= '9' }
