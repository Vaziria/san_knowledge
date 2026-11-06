package gmaps

import (
	"fmt"
	"math"
	"strings"
)

// Plus codes (Open Location Code, github.com/google/open-location-code):
// a point as a short code, "6P58RR48+P4". Its first ten digits are five
// pairs, each a latitude digit then a longitude digit in base 20, from cells
// of 20 degrees down to 1/8000 of a degree (about 14 m); digits after them
// split that cell in a 4 by 5 grid, one digit each. A "+" follows the eighth
// digit. A code shorter than eight digits is padded with "0" to the "+".
//
// Written here rather than taken from the reference Go package, which has no
// release; the tests check it against codes the reference library gives.

const (
	olcAlphabet = "23456789CFGHJMPQRVWX"
	olcSep      = '+'
	olcPad      = '0'
	// Units of a degree at the 15th digit: 20^3 (the pairs) times 5^5 for
	// latitude and 4^5 for longitude (the grid).
	olcLatUnits = 8000 * 3125
	olcLngUnits = 8000 * 1024
)

// PlusCode is the plus code of a point, of length digits (10, about 14 m, to
// 15).
func PlusCode(lat, lng float64, length int) string {
	length = min(max(length, 10), 15)
	lat = min(max(lat, -90), 90)
	// Units, rounded to a millionth first so that 1.2 does not land in the
	// cell below 1.2 from a float that is a little less.
	latVal := int64(math.Floor(math.Round((lat+90)*olcLatUnits*1e6) / 1e6))
	lngVal := int64(math.Floor(math.Round((lng+180)*olcLngUnits*1e6) / 1e6))
	if latVal >= 180*olcLatUnits {
		latVal = 180*olcLatUnits - 1 // the north pole is in the top cell
	}
	lngVal %= 360 * olcLngUnits
	if lngVal < 0 {
		lngVal += 360 * olcLngUnits
	}
	code := make([]byte, 15)
	for i := 14; i >= 10; i-- {
		code[i] = olcAlphabet[(latVal%5)*4+lngVal%4]
		latVal, lngVal = latVal/5, lngVal/4
	}
	for i := 4; i >= 0; i-- {
		code[2*i] = olcAlphabet[latVal%20]
		code[2*i+1] = olcAlphabet[lngVal%20]
		latVal, lngVal = latVal/20, lngVal/20
	}
	return string(code[:8]) + string(olcSep) + string(code[8:length])
}

// DecodePlusCode gives the center of a full plus code's cell. A short code
// ("RR48+P4", or one with a place name after it) is refused: it is a point
// only near a place, which this does not look up.
func DecodePlusCode(s string) (lat, lng float64, err error) {
	code := strings.ToUpper(strings.TrimSpace(s))
	bad := func(why string) error {
		return fmt.Errorf("%w: plus code %q: %s", ErrInvalid, s, why)
	}
	if strings.ContainsAny(code, " ,") {
		return 0, 0, bad("a short code with a place name; give the full code, such as 6P58RR48+P4")
	}
	sep := strings.IndexByte(code, olcSep)
	switch {
	case sep < 0 || strings.Count(code, string(olcSep)) != 1:
		return 0, 0, bad("it needs one +, such as 6P58RR48+P4")
	case sep < 8:
		return 0, 0, bad("a short code; give the full code, such as 6P58RR48+P4")
	case sep > 8 || len(code)-sep-1 == 1:
		return 0, 0, bad("not a plus code")
	}
	digits := code[:sep]
	if p := strings.IndexByte(digits, olcPad); p >= 0 {
		if p == 0 || p%2 != 0 || strings.Trim(digits[p:], string(olcPad)) != "" || len(code) > sep+1 {
			return 0, 0, bad("its 0 padding is not a plus code's")
		}
		digits = digits[:p]
	}
	digits += code[sep+1:]
	idx := make([]int64, len(digits))
	for i := range digits {
		n := strings.IndexByte(olcAlphabet, digits[i])
		if n < 0 {
			return 0, 0, bad(fmt.Sprintf("%q is not a plus code digit", digits[i]))
		}
		idx[i] = int64(n)
	}
	if idx[0]*20 >= 180 || idx[1]*20 >= 360 {
		return 0, 0, bad("it is off the map")
	}
	var latVal, lngVal int64
	latPlace, lngPlace := int64(20*olcLatUnits), int64(20*olcLngUnits)
	for i := 0; i < len(idx) && i < 10; i += 2 {
		latVal += idx[i] * latPlace
		lngVal += idx[i+1] * lngPlace
		if i+2 < len(idx) && i+2 < 10 {
			latPlace, lngPlace = latPlace/20, lngPlace/20
		}
	}
	for i := 10; i < len(idx) && i < 15; i++ {
		latPlace, lngPlace = latPlace/5, lngPlace/4
		latVal += idx[i] / 4 * latPlace
		lngVal += idx[i] % 4 * lngPlace
	}
	lat = (float64(latVal)+float64(latPlace)/2)/olcLatUnits - 90
	lng = (float64(lngVal)+float64(lngPlace)/2)/olcLngUnits - 180
	return lat, lng, nil
}
