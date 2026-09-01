package notifications

import (
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

func TestNotificationTitlesIncludePanelName(t *testing.T) {
	t.Parallel()

	item := digestItem{Key: "site-1", Kind: "site", Overall: "unhealthy", Message: "down"}
	digest := formatDailyDigest("panel & one", []digestItem{item}, map[string]string{"site-1": "Site"}, nil, time.Now(), "UTC")
	incident := formatIncident("panel & one", "Site", item)

	for _, message := range []string{digest, incident} {
		if !strings.HasPrefix(message, "<b>Barn — panel &amp; one — ") {
			t.Fatalf("notification title does not contain escaped panel name: %q", message)
		}
	}
}

func TestDailyDigestUsesReadableStatusLists(t *testing.T) {
	t.Parallel()

	rows := []digestItem{
		{Key: "site-ok", Kind: "site", Overall: "healthy", Message: "HTTP 200"},
		{Key: "site-down", Kind: "site", Overall: "unhealthy", Message: "connection refused"},
		{Key: "pg-main", Kind: "postgres", Overall: "healthy", Message: "Postgres is ready"},
	}
	names := map[string]string{
		"site-ok":   "Магазин",
		"site-down": "API",
		"pg-main":   "Основная база",
	}

	message := formatDailyDigest("panel", rows, names, nil, time.Now(), "UTC")
	for _, want := range []string{
		"<b>Сайты</b>\n✅ Магазин\n❌ API\n   └ connection refused",
		"<b>Базы данных</b>\n✅ Основная база",
	} {
		if !strings.Contains(message, want) {
			t.Fatalf("digest does not contain %q:\n%s", want, message)
		}
	}
	if strings.Contains(message, "HTTP 200") || strings.Contains(message, "Postgres is ready") {
		t.Fatalf("healthy technical details should be hidden:\n%s", message)
	}
}

func TestDailyDigestIncludesMasterServersAndBilling(t *testing.T) {
	t.Parallel()

	ten := 10
	overdue := -2
	servers := []ServerSummaryItem{
		{Name: "Master", Status: "online", DaysLeft: &ten},
		{Name: "Worker", Status: "warning", DaysLeft: nil},
		{Name: "Backup", Status: "offline", DaysLeft: &overdue},
	}
	message := formatDailyDigest("panel", nil, nil, servers, time.Now(), "UTC")

	for _, want := range []string{
		"<b>Серверы</b>",
		"✅ Онлайн: 1 · ⚠️ Нестабильно: 1 · ❌ Не в сети: 1",
		"✅ Master\n   └ 💳 До оплаты: 10 дн.",
		"⚠️ Worker\n   └ 💳 Срок оплаты не указан",
		"❌ Backup\n   └ 💳 Оплата просрочена на 2 дн.",
	} {
		if !strings.Contains(message, want) {
			t.Fatalf("digest does not contain %q:\n%s", want, message)
		}
	}
}

func TestDailyDigestShowsDatabaseLastActivity(t *testing.T) {
	t.Parallel()

	last := time.Date(2026, 9, 1, 12, 34, 0, 0, time.UTC)
	rows := []digestItem{
		{Key: "db-active", Kind: "postgres", Overall: "healthy", LastActivity: &last},
		{Key: "db-idle", Kind: "postgres", Overall: "healthy"},
	}
	names := map[string]string{"db-active": "Postgres / app", "db-idle": "Postgres / archive"}
	message := formatDailyDigest("panel", rows, names, nil, time.Now(), "Europe/Moscow")

	for _, want := range []string{
		"✅ Postgres / app (01.09.2026 15:34)",
		"✅ Postgres / archive (нет данных об изменениях)",
	} {
		if !strings.Contains(message, want) {
			t.Fatalf("digest does not contain %q:\n%s", want, message)
		}
	}
}

func TestIsIncidentTransition(t *testing.T) {
	t.Parallel()
	cases := []struct {
		prev, current string
		want          bool
	}{
		{"healthy", "unhealthy", true},
		{"healthy", "degraded", true},
		{"degraded", "unhealthy", true},
		{"unhealthy", "unhealthy", false},
		{"healthy", "healthy", false},
		{"degraded", "degraded", false},
		{"", "unhealthy", true},
		{"", "healthy", false},
	}
	for _, tc := range cases {
		if got := isIncidentTransition(tc.prev, tc.current); got != tc.want {
			t.Fatalf("isIncidentTransition(%q,%q)=%v want %v", tc.prev, tc.current, got, tc.want)
		}
	}
}

func TestDigestLocationAsiaBarnaul(t *testing.T) {
	t.Parallel()
	loc := digestLocation("Asia/Barnaul")
	if loc.String() != "Asia/Barnaul" {
		t.Fatalf("digestLocation(Asia/Barnaul) = %q", loc.String())
	}
}

func TestShouldSendDaily(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 6, 15, 9, 30, 0, 0, time.UTC)
	last := pgtype.Timestamptz{Time: time.Date(2026, 6, 14, 9, 0, 0, 0, time.UTC), Valid: true}

	if !shouldSendDaily(last, 9, "UTC", now) {
		t.Fatal("expected daily send on new UTC day at matching hour")
	}
	if shouldSendDaily(last, 10, "UTC", now) {
		t.Fatal("expected no send when hour mismatches")
	}
	if shouldSendDaily(last, 9, "UTC", now.Add(-24*time.Hour)) {
		t.Fatal("expected no send when already sent today")
	}

	// 06:30 UTC = 09:30 Europe/Moscow
	moscowNow := time.Date(2026, 6, 15, 6, 30, 0, 0, time.UTC)
	if !shouldSendDaily(last, 9, "Europe/Moscow", moscowNow) {
		t.Fatal("expected daily send when local Moscow hour matches")
	}
	if shouldSendDaily(last, 9, "Europe/Moscow", now) {
		t.Fatal("expected no send when Moscow local hour is 12, not 9")
	}
}
