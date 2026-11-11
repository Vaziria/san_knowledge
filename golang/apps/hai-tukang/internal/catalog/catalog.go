// Package catalog holds the kinds of work Hai Tukang knows, and the area
// rule shared by repairman profiles and repair requests.
package catalog

import (
	"errors"
	"strings"
	"unicode/utf8"
)

// Services are the kinds of work, by code. The frontend names each in the
// reader's language (src/i18n, "services"); a new code needs a name there.
var Services = []string{"listrik", "pipa", "atap", "cat", "ac", "bangunan", "kayu", "lainnya"}

var (
	ErrInvalidService = errors.New("unknown kind of work")
	ErrInvalidArea    = errors.New("area must be 2 to 80 characters")
)

func ValidService(code string) bool {
	for _, s := range Services {
		if s == code {
			return true
		}
	}
	return false
}

// NormalizeServices checks codes and returns them once each, in catalog order.
func NormalizeServices(codes []string) ([]string, error) {
	want := map[string]bool{}
	for _, code := range codes {
		if !ValidService(code) {
			return nil, ErrInvalidService
		}
		want[code] = true
	}
	if len(want) == 0 {
		return nil, ErrInvalidService
	}
	out := make([]string, 0, len(want))
	for _, s := range Services {
		if want[s] {
			out = append(out, s)
		}
	}
	return out, nil
}

// NormalizeArea trims an area and collapses its inner spaces.
func NormalizeArea(area string) (string, error) {
	area = strings.Join(strings.Fields(area), " ")
	if n := utf8.RuneCountInString(area); n < 2 || n > 80 {
		return "", ErrInvalidArea
	}
	return area, nil
}
