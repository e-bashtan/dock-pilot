package notifications

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ebash/dock-pilot/backend/internal/db"
	"github.com/ebash/dock-pilot/backend/internal/pgdb"
	"github.com/ebash/dock-pilot/backend/internal/secrets"
	sitesvc "github.com/ebash/dock-pilot/backend/internal/sites"
)

type Service struct {
	queries  *db.Queries
	cipher   *secrets.Cipher
	sites    *sitesvc.Service
	pgdb     *pgdb.Service
	telegram *TelegramClient
}

func NewService(queries *db.Queries, cipher *secrets.Cipher, sites *sitesvc.Service, pgdbSvc *pgdb.Service) *Service {
	return &Service{
		queries:  queries,
		cipher:   cipher,
		sites:    sites,
		pgdb:     pgdbSvc,
		telegram: NewTelegramClient(),
	}
}

func (s *Service) GetSettings(ctx context.Context) (SettingsResponse, error) {
	row, err := s.getSettingsRow(ctx)
	if err != nil {
		return SettingsResponse{}, err
	}
	return toSettingsResponse(row), nil
}

func (s *Service) UpdateSettings(ctx context.Context, req UpdateSettingsRequest) (SettingsResponse, error) {
	current, err := s.getSettingsRow(ctx)
	if err != nil {
		return SettingsResponse{}, err
	}

	if err := validateUpdate(req, current); err != nil {
		return SettingsResponse{}, err
	}

	if req.ClearTelegramBotToken {
		if err := s.queries.ClearNotificationToken(ctx); err != nil {
			return SettingsResponse{}, fmt.Errorf("clear telegram token: %w", err)
		}
	} else if token := strings.TrimSpace(req.TelegramBotToken); token != "" {
		encrypted, err := s.cipher.Encrypt(token)
		if err != nil {
			return SettingsResponse{}, fmt.Errorf("encrypt telegram token: %w", err)
		}
		if err := s.queries.UpdateNotificationToken(ctx, encrypted); err != nil {
			return SettingsResponse{}, fmt.Errorf("save telegram token: %w", err)
		}
	}

	row, err := s.queries.UpdateNotificationSettings(ctx, db.UpdateNotificationSettingsParams{
		Enabled:                req.Enabled,
		TelegramChatID:         strings.TrimSpace(req.TelegramChatID),
		TelegramHttpProxy:      strings.TrimSpace(req.TelegramHTTPProxy),
		DailyDigestEnabled:     req.DailyDigestEnabled,
		DailyDigestHour:        int32(req.DailyDigestHour),
		DailyDigestTimezone:    strings.TrimSpace(req.DailyDigestTimezone),
		AlertOnIncidentEnabled: req.AlertOnIncidentEnabled,
	})
	if err != nil {
		return SettingsResponse{}, err
	}
	return toSettingsResponse(row), nil
}

func (s *Service) SendTest(ctx context.Context) error {
	settings, token, err := s.loadTelegramConfig(ctx)
	if err != nil {
		return err
	}
	text := "<b>DockPilot</b>\nТестовое уведомление — Telegram настроен."
	if err := s.telegram.SendMessage(ctx, token, settings.TelegramChatID, text, settings.TelegramHttpProxy); err != nil {
		return fmt.Errorf("telegram: %w", err)
	}
	return nil
}

// SendText sends an arbitrary message using the configured Telegram bot (if enabled).
func (s *Service) SendText(ctx context.Context, text string) error {
	settings, token, err := s.loadTelegramConfig(ctx)
	if err != nil {
		return err
	}
	if !settings.Enabled {
		return fmt.Errorf("telegram notifications are disabled")
	}
	return s.telegram.SendMessage(ctx, token, settings.TelegramChatID, text, settings.TelegramHttpProxy)
}

func (s *Service) RunCheck(ctx context.Context) error {
	settings, token, err := s.loadTelegramConfig(ctx)
	if err != nil {
		if errors.Is(err, ErrNotConfigured) {
			return nil
		}
		return err
	}

	healthRows, err := s.sites.HealthAll(ctx)
	if err != nil {
		return err
	}

	var pgRows []pgdb.HealthResult
	if s.pgdb != nil {
		pgRows, err = s.pgdb.HealthAll(ctx)
		if err != nil {
			return err
		}
	}

	siteRows, err := s.queries.ListSites(ctx)
	if err != nil {
		return err
	}
	names := make(map[string]string, len(siteRows)+len(pgRows))
	for _, site := range siteRows {
		names[site.ID.String()] = site.Name
	}
	for _, pg := range pgRows {
		names[pgHealthKey(pg.InstanceID.String())] = pgDisplayName(pg)
	}

	prev := decodeOverallMap(settings.LastOverallBySite)
	now := time.Now().UTC()

	digestItems := make([]digestItem, 0, len(healthRows)+len(pgRows))
	for _, h := range healthRows {
		digestItems = append(digestItems, digestItem{
			Key:     h.SiteID.String(),
			Kind:    "site",
			Overall: h.Overall,
			Message: h.Message,
		})
	}
	for _, h := range pgRows {
		digestItems = append(digestItems, digestItem{
			Key:     pgHealthKey(h.InstanceID.String()),
			Kind:    "postgres",
			Overall: h.Overall,
			Message: h.Message,
		})
	}

	if settings.DailyDigestEnabled && shouldSendDaily(settings.LastDailySentAt, settings.DailyDigestHour, settings.DailyDigestTimezone, now) {
		msg := formatDailyDigest(digestItems, names, now, settings.DailyDigestTimezone)
		if err := s.telegram.SendMessage(ctx, token, settings.TelegramChatID, msg, settings.TelegramHttpProxy); err != nil {
			return fmt.Errorf("daily digest: %w", err)
		}
		if err := s.queries.UpdateNotificationLastDailySent(ctx, pgtype.Timestamptz{Time: now, Valid: true}); err != nil {
			return err
		}
	}

	if settings.AlertOnIncidentEnabled {
		for _, item := range digestItems {
			prevOverall := prev[item.Key]
			if !isIncidentTransition(prevOverall, item.Overall) {
				continue
			}
			name := names[item.Key]
			if name == "" {
				name = item.Key
			}
			msg := formatIncident(name, item)
			if err := s.telegram.SendMessage(ctx, token, settings.TelegramChatID, msg, settings.TelegramHttpProxy); err != nil {
				return fmt.Errorf("incident alert: %w", err)
			}
		}
	}

	next := make(map[string]string, len(digestItems))
	for _, item := range digestItems {
		next[item.Key] = item.Overall
	}
	raw, err := json.Marshal(next)
	if err != nil {
		return err
	}
	return s.queries.UpdateNotificationLastOverall(ctx, raw)
}

func pgHealthKey(instanceID string) string {
	return "pg:" + instanceID
}

func pgDisplayName(h pgdb.HealthResult) string {
	if strings.TrimSpace(h.Name) != "" {
		return "Postgres: " + h.Name
	}
	return "Postgres"
}

type digestItem struct {
	Key     string
	Kind    string // site | postgres
	Overall string
	Message string
}

func (s *Service) loadTelegramConfig(ctx context.Context) (db.NotificationSetting, string, error) {
	row, err := s.queries.GetNotificationSettings(ctx)
	if err != nil {
		return db.NotificationSetting{}, "", err
	}
	if !row.Enabled {
		return row, "", ErrNotConfigured
	}
	if len(row.EncryptedTelegramBotToken) == 0 {
		return row, "", ErrNotConfigured
	}
	if strings.TrimSpace(row.TelegramChatID) == "" {
		return row, "", ErrNotConfigured
	}
	token, err := s.cipher.Decrypt(row.EncryptedTelegramBotToken)
	if err != nil {
		return row, "", fmt.Errorf("decrypt telegram token: %w", err)
	}
	return row, token, nil
}

func (s *Service) getSettingsRow(ctx context.Context) (db.NotificationSetting, error) {
	row, err := s.queries.GetNotificationSettings(ctx)
	if err == nil {
		return row, nil
	}
	if mapped := mapDBErr(err); mapped != nil {
		return db.NotificationSetting{}, mapped
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return db.NotificationSetting{}, err
	}
	row, err = s.queries.EnsureNotificationSettings(ctx)
	if err != nil {
		if mapped := mapDBErr(err); mapped != nil {
			return db.NotificationSetting{}, mapped
		}
		return db.NotificationSetting{}, err
	}
	return row, nil
}

func mapDBErr(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "42P01" {
		return fmt.Errorf("%w: apply migration 00006_notification_settings", ErrMigration)
	}
	return nil
}

func validateUpdate(req UpdateSettingsRequest, current db.NotificationSetting) error {
	if err := validateProxyURL(req.TelegramHTTPProxy); err != nil {
		return err
	}
	if req.DailyDigestHour < 0 || req.DailyDigestHour > 23 {
		return ErrInvalidInput
	}
	tz := strings.TrimSpace(req.DailyDigestTimezone)
	if tz == "" {
		tz = "UTC"
	}
	if _, err := time.LoadLocation(tz); err != nil {
		return fmt.Errorf("%w: daily_digest_timezone must be a valid IANA timezone (e.g. Europe/Moscow)", ErrInvalidInput)
	}
	if !req.Enabled {
		return nil
	}
	if strings.TrimSpace(req.TelegramChatID) == "" {
		return fmt.Errorf("%w: telegram_chat_id is required when enabled", ErrInvalidInput)
	}
	hasToken := len(current.EncryptedTelegramBotToken) > 0 && !req.ClearTelegramBotToken
	if strings.TrimSpace(req.TelegramBotToken) != "" {
		hasToken = true
	}
	if !hasToken {
		return fmt.Errorf("%w: telegram_bot_token is required when enabled", ErrInvalidInput)
	}
	return nil
}

func toSettingsResponse(row db.NotificationSetting) SettingsResponse {
	return SettingsResponse{
		Enabled:                row.Enabled,
		TelegramChatID:         row.TelegramChatID,
		TelegramHTTPProxy:      row.TelegramHttpProxy,
		TelegramBotTokenSet:    len(row.EncryptedTelegramBotToken) > 0,
		DailyDigestEnabled:     row.DailyDigestEnabled,
		DailyDigestHour:        int(row.DailyDigestHour),
		DailyDigestTimezone:    row.DailyDigestTimezone,
		AlertOnIncidentEnabled: row.AlertOnIncidentEnabled,
	}
}

func decodeOverallMap(raw []byte) map[string]string {
	out := map[string]string{}
	if len(raw) == 0 {
		return out
	}
	_ = json.Unmarshal(raw, &out)
	return out
}

func digestLocation(tz string) *time.Location {
	tz = strings.TrimSpace(tz)
	if tz == "" {
		tz = "UTC"
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return time.UTC
	}
	return loc
}

func shouldSendDaily(last pgtype.Timestamptz, hour int32, tz string, now time.Time) bool {
	localNow := now.In(digestLocation(tz))
	if localNow.Hour() != int(hour) {
		return false
	}
	if !last.Valid {
		return true
	}
	lastLocal := last.Time.In(digestLocation(tz))
	y1, m1, d1 := lastLocal.Date()
	y2, m2, d2 := localNow.Date()
	return y1 != y2 || m1 != m2 || d1 != d2
}

func isIncidentTransition(prev, current string) bool {
	if current != "unhealthy" && current != "degraded" {
		return false
	}
	if prev == "" {
		// First observation after restart — alert only if already bad.
		return current == "unhealthy" || current == "degraded"
	}
	if prev == current {
		return false
	}
	if current == "unhealthy" {
		return prev != "unhealthy"
	}
	// degraded: alert when worsening from healthy only
	return prev == "healthy"
}

func formatDailyDigest(rows []digestItem, names map[string]string, now time.Time, tz string) string {
	localNow := now.In(digestLocation(tz))
	var b strings.Builder
	fmt.Fprintf(&b, "<b>DockPilot — ежедневный отчёт</b>\n%s\n\n", localNow.Format("2006-01-02 15:04 MST"))
	if len(rows) == 0 {
		b.WriteString("Нет сайтов и баз в панели.")
		return b.String()
	}
	counts := map[string]int{}
	for _, h := range rows {
		counts[h.Overall]++
	}
	fmt.Fprintf(&b, "Всего: %d\n", len(rows))
	fmt.Fprintf(&b, "Здоровые: %d\n", counts["healthy"])
	fmt.Fprintf(&b, "Проблемы: %d\n", counts["degraded"])
	fmt.Fprintf(&b, "Авария: %d\n", counts["unhealthy"])
	fmt.Fprintf(&b, "Неизвестно: %d\n\n", counts["unknown"])
	for _, h := range rows {
		name := names[h.Key]
		if name == "" {
			name = h.Key
		}
		fmt.Fprintf(&b, "• %s — <b>%s</b>\n  %s\n", escapeHTML(name), h.Overall, escapeHTML(h.Message))
	}
	return b.String()
}

func formatIncident(name string, h digestItem) string {
	title := "авария"
	if h.Overall == "degraded" {
		title = "проблема"
	}
	label := "Сайт"
	if h.Kind == "postgres" {
		label = "Postgres"
	}
	return fmt.Sprintf(
		"<b>DockPilot — %s</b>\n%s: %s\nСтатус: <b>%s</b>\n%s",
		title,
		label,
		escapeHTML(name),
		h.Overall,
		escapeHTML(h.Message),
	)
}
