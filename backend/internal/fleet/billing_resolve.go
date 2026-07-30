package fleet

import (
	"context"
	"encoding/json"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"

	"github.com/ebash/dock-pilot/backend/internal/db"
)

var costNumberRe = regexp.MustCompile(`(\d+(?:[.,]\d+)?)`)

// parseCachedCost turns Planetahost-style cost strings ("990", "1 500 RUB", "12.50 USD") into minor units.
func parseCachedCost(raw string) (minor int64, currency string) {
	currency = "RUB"
	s := strings.TrimSpace(raw)
	if s == "" {
		return 0, currency
	}
	upper := strings.ToUpper(s)
	switch {
	case strings.Contains(upper, "USD") || strings.Contains(s, "$"):
		currency = "USD"
	case strings.Contains(upper, "EUR") || strings.Contains(s, "€"):
		currency = "EUR"
	case strings.Contains(upper, "RUB") || strings.Contains(s, "₽") || strings.Contains(upper, "RUR"):
		currency = "RUB"
	}
	compact := strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, s)
	m := costNumberRe.FindString(compact)
	if m == "" {
		return 0, currency
	}
	m = strings.ReplaceAll(m, ",", ".")
	major, err := strconv.ParseFloat(m, 64)
	if err != nil || major < 0 {
		return 0, currency
	}
	return int64(math.Round(major * 100)), currency
}

func billingDTOFromAccount(acc db.BillingAccount) *BillingDTO {
	minor, currency := parseCachedCost(acc.CachedCost)
	dto := &BillingDTO{
		CostMinor:        minor,
		Currency:         currency,
		Period:           "monthly",
		Provider:         acc.Provider,
		ProviderURL:      acc.BillmgrUrl,
		MonthlyEquiv:     minor,
		Mode:             "planetahost",
		ServerIP:         acc.ServerIp,
		CostRaw:          acc.CachedCost,
		BillingAccountID: acc.ID.String(),
	}
	if name := strings.TrimSpace(acc.CachedName); name != "" {
		dto.Provider = name
	}
	if acc.CachedExpireDate.Valid {
		d := acc.CachedExpireDate.Time.Format("2006-01-02")
		dto.NextDueDate = &d
		days := int(acc.CachedExpireDate.Time.UTC().Truncate(24*time.Hour).Sub(time.Now().UTC().Truncate(24*time.Hour)).Hours() / 24)
		dto.DaysLeft = &days
	}
	return dto
}

func billingDTOFromManual(bill db.FleetNodeBilling) *BillingDTO {
	dto := &BillingDTO{
		CostMinor:    bill.CostMinor,
		Currency:     bill.Currency,
		Period:       bill.Period,
		AutoRenew:    bill.AutoRenew,
		Provider:     bill.ProviderName,
		ProviderURL:  bill.ProviderUrl,
		MonthlyEquiv: monthlyEquiv(bill.CostMinor, bill.Period),
		Mode:         "manual",
	}
	if bill.NextDueDate.Valid {
		d := bill.NextDueDate.Time.Format("2006-01-02")
		dto.NextDueDate = &d
	}
	return dto
}

func accountByID(accounts []db.BillingAccount, id uuid.UUID) (db.BillingAccount, bool) {
	for _, a := range accounts {
		if a.ID == id {
			return a, true
		}
	}
	return db.BillingAccount{}, false
}

func nodeMatchHaystack(row db.FleetNode, hostname, localIP string) string {
	parts := []string{row.BaseUrl, row.Name, hostname}
	if row.ConnectionType == ConnLocal && localIP != "" {
		parts = append(parts, localIP)
	}
	return strings.ToLower(strings.Join(parts, " "))
}

func matchBillingAccount(accounts []db.BillingAccount, claimed map[uuid.UUID]bool, haystack string) (db.BillingAccount, bool) {
	haystack = strings.ToLower(strings.TrimSpace(haystack))
	if haystack == "" {
		return db.BillingAccount{}, false
	}
	for _, a := range accounts {
		if !a.Enabled || claimed[a.ID] {
			continue
		}
		ip := strings.TrimSpace(a.ServerIp)
		if ip == "" {
			continue
		}
		if strings.Contains(haystack, strings.ToLower(ip)) {
			return a, true
		}
	}
	return db.BillingAccount{}, false
}

func (s *Service) listBillingAccounts(ctx context.Context) []db.BillingAccount {
	rows, err := s.q.ListBillingAccounts(ctx)
	if err != nil {
		return nil
	}
	return rows
}

func snapshotHostname(payload []byte) string {
	if len(payload) == 0 {
		return ""
	}
	var root map[string]any
	if json.Unmarshal(payload, &root) != nil {
		return ""
	}
	if v, ok := root["hostname"].(string); ok {
		return v
	}
	if metricsRaw, ok := root["metrics"].(map[string]any); ok {
		if v, ok := metricsRaw["hostname"].(string); ok {
			return v
		}
	}
	return ""
}

func (s *Service) resolveNodeBilling(
	ctx context.Context,
	row db.FleetNode,
	accounts []db.BillingAccount,
	claimed map[uuid.UUID]bool,
	localIP string,
) *BillingDTO {
	bill, billErr := s.q.GetFleetNodeBilling(ctx, row.ID)

	if billErr == nil && bill.BillingAccountID.Valid {
		id := uuid.UUID(bill.BillingAccountID.Bytes)
		if acc, ok := accountByID(accounts, id); ok {
			claimed[id] = true
			return billingDTOFromAccount(acc)
		}
	}

	hostname := ""
	if snap, err := s.q.GetLatestFleetSnapshot(ctx, row.ID); err == nil {
		hostname = snapshotHostname(snap.Payload)
	}

	if acc, ok := matchBillingAccount(accounts, claimed, nodeMatchHaystack(row, hostname, localIP)); ok {
		claimed[acc.ID] = true
		return billingDTOFromAccount(acc)
	}

	if billErr == nil && (bill.CostMinor > 0 || bill.NextDueDate.Valid) {
		return billingDTOFromManual(bill)
	}
	return nil
}
