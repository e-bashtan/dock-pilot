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
		{"288.75 ₽ / Месяц", 28875, "RUB"},
	}
	for _, tc := range cases {
		minor, currency := parseCachedCost(tc.raw)
		if minor != tc.minor || currency != tc.currency {
			t.Fatalf("%q: got %d %s, want %d %s", tc.raw, minor, currency, tc.minor, tc.currency)
		}
	}
}

func TestPickRemoteBilling(t *testing.T) {
	expire := "2026-08-15"
	days := 16
	accounts := []RemoteBillingAccount{
		{
			ServerIP:   "62.173.140.62",
			Provider:   "planetahost",
			Name:       "bashtan.e.a · MASTER SAS 40 #221728",
			Cost:       "288.75 ₽ / Месяц",
			ExpireDate: &expire,
			DaysLeft:   &days,
			Enabled:    true,
		},
	}
	dto := pickRemoteBilling(accounts, "62.173.140.62")
	if dto == nil {
		t.Fatal("expected remote billing")
	}
	if dto.CostMinor != 28875 {
		t.Fatalf("cost_minor=%d", dto.CostMinor)
	}
	if dto.NextDueDate == nil || *dto.NextDueDate != expire {
		t.Fatalf("due=%v", dto.NextDueDate)
	}
	if dto.Mode != "remote" {
		t.Fatalf("mode=%s", dto.Mode)
	}
}
