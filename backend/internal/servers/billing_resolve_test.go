package servers

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
			Name:       "MASTER SAS 40 #221728 (62.173.140.62, bashtan1.e.a.unassigned.planetahost.ru)",
			Cost:       "288.75 ₽ / Месяц",
			ExpireDate: &expire,
			DaysLeft:   &days,
			AlertDays:  30,
			Enabled:    true,
		},
	}
	dto := pickRemoteBilling(accounts, "62.173.140.62", "")
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
	if dto.AlertDays != 30 {
		t.Fatalf("alert_days=%d", dto.AlertDays)
	}
	if dto.DaysLeft == nil || *dto.DaysLeft != 16 {
		t.Fatalf("days_left=%v", dto.DaysLeft)
	}

	byHost := pickRemoteBilling(accounts, "", "bashtan1.e.a.unassigned.planetahost.ru")
	if byHost == nil || byHost.NextDueDate == nil {
		t.Fatal("expected match by hostname in account name")
	}

	byBase := pickRemoteBilling(accounts, "", hostnameFromBaseURL("https://bashtan1.e.a.unassigned.planetahost.ru"))
	if byBase == nil || byBase.NextDueDate == nil {
		t.Fatal("expected match via panel base URL hostname")
	}
}

func TestIsBarnPanelLegacy(t *testing.T) {
	if !IsBarnPanel("barn") || !IsBarnPanel("dockpilot") {
		t.Fatal("barn/dockpilot should be panel")
	}
	if IsBarnPanel("agent") || IsBarnPanel("local") {
		t.Fatal("agent/local should not be panel")
	}
}

func TestMergeKeepsPreviousBilling(t *testing.T) {
	expire := "2026-08-15"
	prev := []RemoteBillingAccount{{
		ServerIP:   "62.173.140.62",
		Cost:       "288.75",
		ExpireDate: &expire,
		Enabled:    true,
		AlertDays:  30,
	}}
	// Empty incoming billing should not be considered useful.
	if billingDTOUseful(pickRemoteBilling(nil, "62.173.140.62", "")) {
		t.Fatal("empty should not be useful")
	}
	if !billingDTOUseful(pickRemoteBilling(prev, "62.173.140.62", "")) {
		t.Fatal("prev should be useful")
	}
}
