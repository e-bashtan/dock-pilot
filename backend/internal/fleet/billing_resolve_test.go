package fleet

import "testing"

func TestParseCachedCost(t *testing.T) {
	cases := []struct {
		raw      string
		minor    int64
		currency string
	}{
		{"", 0, "RUB"},
		{"990", 99000, "RUB"},
		{"1 500 RUB", 150000, "RUB"},
		{"12.50 USD", 1250, "USD"},
		{"99,99 €", 9999, "EUR"},
		{"₽ 2 000", 200000, "RUB"},
	}
	for _, tc := range cases {
		minor, currency := parseCachedCost(tc.raw)
		if minor != tc.minor || currency != tc.currency {
			t.Fatalf("%q: got %d %s, want %d %s", tc.raw, minor, currency, tc.minor, tc.currency)
		}
	}
}
