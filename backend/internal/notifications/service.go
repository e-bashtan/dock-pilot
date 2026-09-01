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

	"github.com/ebash/barn/backend/internal/db"
	"github.com/ebash/barn/backend/internal/pgdb"
	"github.com/ebash/barn/backend/internal/secrets"
	sitesvc "github.com/ebash/barn/backend/internal/sites"
)

// LocalAlertGate optionally suppresses local Telegram user alerts
// (e.g. when servers notification_mode is "master" on a managed node).
type LocalAlertGate interface {
	SuppressLocalAlerts(ctx context.Context) bool
}

type Service struct {
	queries       *db.Queries
	cipher        *secrets.Cipher
	sites         *sitesvc.Service
	pgdb          *pgdb.Service
	telegram      *TelegramClient
	alertGate     LocalAlertGate
	serversEvents ServersEventSink
	defaultName   func(context.Context) string
	serverSummary func(context.Context) ([]ServerSummaryItem, bool, error)
}

// ServersEventSink forwards local incidents to the master outbox when notifications are centralized.
type ServersEventSink interface {
	OnLocalIncident(ctx context.Context, kind, resourceID, name, overall, message string) error
}

type ServerSummaryItem struct {
	Name     string
	Status   string
	DaysLeft *int
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

func (s *Service) SetLocalAlertGate(g LocalAlertGate) {
	s.alertGate = g
}

func (s *Service) SetServersEventSink(sink ServersEventSink) {
	s.serversEvents = sink
}

func (s *Service) SetDefaultPanelNameProvider(provider func(context.Context) string) {
	s.defaultName = provider
}

func (s *Service) SetServerSummaryProvider(provider func(context.Context) ([]ServerSummaryItem, bool, error)) {
	s.serverSummary = provider
}

func (s *Service) panelName(ctx context.Context, configured string) string {
	if name := strings.TrimSpace(configured); name != "" {
		return name
	}
	if s.defaultName != nil {
		if name := strings.TrimSpace(s.defaultName(ctx)); name != "" {
			return name
		}
	}
	return "panel"
}

func (s *Service) suppressLocal(ctx context.Context) bool {
	return s.alertGate != nil && s.alertGate.SuppressLocalAlerts(ctx)
}

func (s *Service) GetSettings(ctx context.Context) (SettingsResponse, error) {
	row, err := s.getSettingsRow(ctx)
	if err != nil {
		return SettingsResponse{}, err
	}
	return s.toSettingsResponse(ctx, row), nil
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
		PanelName:              strings.TrimSpace(req.PanelName),
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
	return s.toSettingsResponse(ctx, row), nil
}

// SetTelegramProxy updates only the Telegram proxy while preserving all other settings.
func (s *Service) SetTelegramProxy(ctx context.Context, proxyURL string) (SettingsResponse, error) {
	row, err := s.getSettingsRow(ctx)
	if err != nil {
		return SettingsResponse{}, err
	}
	if err := validateProxyURL(proxyURL); err != nil {
		return SettingsResponse{}, err
	}
	row, err = s.queries.UpdateNotificationSettings(ctx, db.UpdateNotificationSettingsParams{
		PanelName:              row.PanelName,
		Enabled:                row.Enabled,
		TelegramChatID:         row.TelegramChatID,
		TelegramHttpProxy:      strings.TrimSpace(proxyURL),
		DailyDigestEnabled:     row.DailyDigestEnabled,
		DailyDigestHour:        row.DailyDigestHour,
		DailyDigestTimezone:    row.DailyDigestTimezone,
		AlertOnIncidentEnabled: row.AlertOnIncidentEnabled,
	})
	if err != nil {
		return SettingsResponse{}, err
	}
	return s.toSettingsResponse(ctx, row), nil
}

func (s *Service) SendTest(ctx context.Context) error {
	settings, token, err := s.loadTelegramConfig(ctx)
	if err != nil {
		return err
	}
	items, names, err := s.collectDigestItems(ctx)
	if err != nil {
		return err
	}
	servers, err := s.collectServerSummary(ctx)
	if err != nil {
		return err
	}
	text := formatDailyDigest(s.panelName(ctx, settings.PanelName), items, names, servers, time.Now().UTC(), settings.DailyDigestTimezone)
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
	suppress := s.suppressLocal(ctx)

	settings, token, err := s.loadTelegramConfig(ctx)
	if err != nil {
		if !errors.Is(err, ErrNotConfigured) {
			return err
		}
		if !suppress {
			return nil
		}
		// settings still holds the row when Telegram is merely unconfigured
	}

	prev := decodeOverallMap(settings.LastOverallBySite)
	now := time.Now().UTC()
	digestItems, names, err := s.collectDigestItems(ctx)
	if err != nil {
		return err
	}

	if !suppress && settings.DailyDigestEnabled && shouldSendDaily(settings.LastDailySentAt, settings.DailyDigestHour, settings.DailyDigestTimezone, now) {
		servers, err := s.collectServerSummary(ctx)
		if err != nil {
			return err
		}
		msg := formatDailyDigest(s.panelName(ctx, settings.PanelName), digestItems, names, servers, now, settings.DailyDigestTimezone)
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
			if suppress {
				if s.serversEvents != nil {
					_ = s.serversEvents.OnLocalIncident(ctx, item.Kind, item.Key, name, item.Overall, item.Message)
				}
				continue
			}
			msg := formatIncident(s.panelName(ctx, settings.PanelName), name, item)
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

func (s *Service) collectServerSummary(ctx context.Context) ([]ServerSummaryItem, error) {
	if s.serverSummary == nil {
		return nil, nil
	}
	items, master, err := s.serverSummary(ctx)
	if err != nil {
		return nil, err
	}
	if !master {
		return nil, nil
	}
	return items, nil
}

func pgHealthKey(instanceID string) string {
	return "pg:" + instanceID
}

func pgDatabaseHealthKey(instanceID, database string) string {
	return pgHealthKey(instanceID) + ":" + database
}

func pgDisplayName(h pgdb.HealthResult) string {
	if strings.TrimSpace(h.Name) != "" {
		return h.Name
	}
	return "Postgres"
}

type digestItem struct {
	Key          string
	Kind         string // site | postgres
	Overall      string
	Message      string
	LastActivity *time.Time
}

func (s *Service) collectDigestItems(ctx context.Context) ([]digestItem, map[string]string, error) {
	healthRows, err := s.sites.HealthAll(ctx)
	if err != nil {
		return nil, nil, err
	}

	var pgRows []pgdb.HealthResult
	if s.pgdb != nil {
		pgRows, err = s.pgdb.HealthAll(ctx)
		if err != nil {
			return nil, nil, err
		}
	}

	siteRows, err := s.queries.ListSites(ctx)
	if err != nil {
		return nil, nil, err
	}
	names := make(map[string]string, len(siteRows)+len(pgRows))
	for _, site := range siteRows {
		names[site.ID.String()] = site.Name
	}
	items := make([]digestItem, 0, len(healthRows)+len(pgRows))
	for _, h := range healthRows {
		items = append(items, digestItem{
			Key:     h.SiteID.String(),
			Kind:    "site",
			Overall: h.Overall,
			Message: h.Message,
		})
	}
	for _, h := range pgRows {
		if len(h.Databases) == 0 {
			key := pgHealthKey(h.InstanceID.String())
			names[key] = pgDisplayName(h)
			items = append(items, digestItem{Key: key, Kind: "postgres", Overall: h.Overall, Message: h.Message})
			continue
		}
		for _, database := range h.Databases {
			key := pgDatabaseHealthKey(h.InstanceID.String(), database.Name)
			names[key] = pgDisplayName(h) + " / " + database.Name
			items = append(items, digestItem{
				Key: key, Kind: "postgres", Overall: database.Overall,
				Message: database.Message, LastActivity: database.LastDMLAt,
			})
		}
	}
	return items, names, nil
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

func (s *Service) toSettingsResponse(ctx context.Context, row db.NotificationSetting) SettingsResponse {
	return SettingsResponse{
		PanelName:              s.panelName(ctx, row.PanelName),
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

func formatDailyDigest(panelName string, rows []digestItem, names map[string]string, servers []ServerSummaryItem, now time.Time, tz string) string {
	localNow := now.In(digestLocation(tz))
	var b strings.Builder
	fmt.Fprintf(&b, "<b>Barn — %s — ежедневный отчёт</b>\n%s\n\n", escapeHTML(panelName), localNow.Format("2006-01-02 15:04 MST"))
	if len(rows) == 0 && len(servers) == 0 {
		b.WriteString("Нет сайтов и баз в панели.")
		return b.String()
	}
	counts := map[string]int{}
	for _, h := range rows {
		counts[h.Overall]++
	}
	b.WriteString("<b>Сводка</b>\n")
	fmt.Fprintf(&b, "✅ В норме: %d\n", counts["healthy"])
	fmt.Fprintf(&b, "⚠️ Есть проблемы: %d\n", counts["degraded"])
	fmt.Fprintf(&b, "❌ Недоступны: %d\n", counts["unhealthy"])
	if counts["unknown"] > 0 {
		fmt.Fprintf(&b, "❔ Статус неизвестен: %d\n", counts["unknown"])
	}

	appendDigestGroup(&b, "Сайты", "site", rows, names, localNow.Location())
	appendDigestGroup(&b, "Базы данных", "postgres", rows, names, localNow.Location())
	appendServerDigest(&b, servers)
	return b.String()
}

func appendServerDigest(b *strings.Builder, servers []ServerSummaryItem) {
	if len(servers) == 0 {
		return
	}

	b.WriteString("\n<b>Серверы</b>\n")
	online, warning, offline := 0, 0, 0
	for _, server := range servers {
		switch server.Status {
		case "online":
			online++
		case "warning":
			warning++
		default:
			offline++
		}
	}
	fmt.Fprintf(b, "✅ Онлайн: %d · ⚠️ Нестабильно: %d · ❌ Не в сети: %d\n\n", online, warning, offline)
	for _, server := range servers {
		fmt.Fprintf(b, "%s %s\n", serverStatusEmoji(server.Status), escapeHTML(server.Name))
		switch {
		case server.DaysLeft == nil:
			b.WriteString("   └ 💳 Срок оплаты не указан\n")
		case *server.DaysLeft < 0:
			fmt.Fprintf(b, "   └ 💳 Оплата просрочена на %d дн.\n", -*server.DaysLeft)
		case *server.DaysLeft == 0:
			b.WriteString("   └ 💳 Оплата истекает сегодня\n")
		default:
			fmt.Fprintf(b, "   └ 💳 До оплаты: %d дн.\n", *server.DaysLeft)
		}
	}
}

func serverStatusEmoji(status string) string {
	switch status {
	case "online":
		return "✅"
	case "warning":
		return "⚠️"
	default:
		return "❌"
	}
}

func appendDigestGroup(b *strings.Builder, title, kind string, rows []digestItem, names map[string]string, loc *time.Location) {
	group := make([]digestItem, 0)
	for _, item := range rows {
		if item.Kind == kind {
			group = append(group, item)
		}
	}
	if len(group) == 0 {
		return
	}

	fmt.Fprintf(b, "\n<b>%s</b>\n", title)
	for _, item := range group {
		name := names[item.Key]
		if name == "" {
			name = item.Key
		}
		activity := ""
		if kind == "postgres" {
			activity = " (нет данных об изменениях)"
			if item.LastActivity != nil {
				activity = " (" + item.LastActivity.In(loc).Format("02.01.2006 15:04") + ")"
			}
		}
		fmt.Fprintf(b, "%s %s%s\n", statusEmoji(item.Overall), escapeHTML(name), activity)
		if item.Overall != "healthy" && strings.TrimSpace(item.Message) != "" {
			fmt.Fprintf(b, "   └ %s\n", escapeHTML(item.Message))
		}
	}
}

func statusEmoji(overall string) string {
	switch overall {
	case "healthy":
		return "✅"
	case "degraded":
		return "⚠️"
	case "unhealthy":
		return "❌"
	default:
		return "❔"
	}
}

func formatIncident(panelName, name string, h digestItem) string {
	title := "авария"
	if h.Overall == "degraded" {
		title = "проблема"
	}
	label := "Сайт"
	if h.Kind == "postgres" {
		label = "Postgres"
	}
	return fmt.Sprintf(
		"<b>Barn — %s — %s</b>\n%s: %s\nСтатус: <b>%s</b>\n%s",
		escapeHTML(panelName),
		title,
		label,
		escapeHTML(name),
		h.Overall,
		escapeHTML(h.Message),
	)
}
