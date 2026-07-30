package billing

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ebash/barn/backend/internal/db"
	notifpkg "github.com/ebash/barn/backend/internal/notifications"
	"github.com/ebash/barn/backend/internal/secrets"
)

type Service struct {
	queries *db.Queries
	cipher  *secrets.Cipher
	notif   *notifpkg.Service
	logger  *slog.Logger
}

func NewService(queries *db.Queries, cipher *secrets.Cipher, notif *notifpkg.Service, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{queries: queries, cipher: cipher, notif: notif, logger: logger}
}

func (s *Service) List(ctx context.Context) ([]AccountResponse, error) {
	rows, err := s.queries.ListBillingAccounts(ctx)
	if err != nil {
		return nil, mapDBErr(err)
	}
	out := make([]AccountResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, toAccountResponse(row))
	}
	return out, nil
}

func (s *Service) Create(ctx context.Context, req CreateAccountRequest) (AccountResponse, error) {
	provider, err := normalizeProvider(req.Provider)
	if err != nil {
		return AccountResponse{}, err
	}
	ip := strings.TrimSpace(req.ServerIP)
	if net.ParseIP(ip) == nil {
		return AccountResponse{}, wrapInvalid("server_ip must be a valid IP")
	}
	login := strings.TrimSpace(req.Login)
	password := strings.TrimSpace(req.Password)
	if login == "" || password == "" {
		return AccountResponse{}, wrapInvalid("login and password are required")
	}
	billmgrURL := strings.TrimSpace(req.BillmgrURL)
	if billmgrURL == "" {
		billmgrURL = defaultBillmgrURL(provider)
	}
	alertDays := req.AlertDays
	if alertDays <= 0 {
		alertDays = 10
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	enc, err := s.cipher.Encrypt(password)
	if err != nil {
		return AccountResponse{}, err
	}
	row, err := s.queries.CreateBillingAccount(ctx, db.CreateBillingAccountParams{
		Provider:          provider,
		ServerIp:          ip,
		Login:             login,
		EncryptedPassword: enc,
		BillmgrUrl:        billmgrURL,
		AlertDays:         int32(alertDays),
		Enabled:           enabled,
	})
	if err != nil {
		return AccountResponse{}, mapDBErr(err)
	}
	_, _ = s.RefreshAccount(ctx, row.ID)
	row, err = s.queries.GetBillingAccount(ctx, row.ID)
	if err != nil {
		return AccountResponse{}, mapDBErr(err)
	}
	return toAccountResponse(row), nil
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, req UpdateAccountRequest) (AccountResponse, error) {
	row, err := s.queries.GetBillingAccount(ctx, id)
	if err != nil {
		return AccountResponse{}, mapDBErr(err)
	}
	provider := row.Provider
	if req.Provider != nil {
		p, err := normalizeProvider(*req.Provider)
		if err != nil {
			return AccountResponse{}, err
		}
		provider = p
	}
	ip := row.ServerIp
	if req.ServerIP != nil {
		ip = strings.TrimSpace(*req.ServerIP)
		if net.ParseIP(ip) == nil {
			return AccountResponse{}, wrapInvalid("server_ip must be a valid IP")
		}
	}
	login := row.Login
	if req.Login != nil {
		login = strings.TrimSpace(*req.Login)
		if login == "" {
			return AccountResponse{}, wrapInvalid("login is required")
		}
	}
	billmgrURL := row.BillmgrUrl
	if req.BillmgrURL != nil {
		billmgrURL = strings.TrimSpace(*req.BillmgrURL)
		if billmgrURL == "" {
			billmgrURL = defaultBillmgrURL(provider)
		}
	}
	alertDays := row.AlertDays
	if req.AlertDays != nil {
		if *req.AlertDays < 1 || *req.AlertDays > 90 {
			return AccountResponse{}, wrapInvalid("alert_days must be 1–90")
		}
		alertDays = int32(*req.AlertDays)
	}
	enabled := row.Enabled
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	row, err = s.queries.UpdateBillingAccount(ctx, db.UpdateBillingAccountParams{
		ID:         id,
		Provider:   provider,
		ServerIp:   ip,
		Login:      login,
		BillmgrUrl: billmgrURL,
		AlertDays:  alertDays,
		Enabled:    enabled,
	})
	if err != nil {
		return AccountResponse{}, mapDBErr(err)
	}
	if req.Password != nil && strings.TrimSpace(*req.Password) != "" {
		enc, err := s.cipher.Encrypt(strings.TrimSpace(*req.Password))
		if err != nil {
			return AccountResponse{}, err
		}
		row, err = s.queries.UpdateBillingAccountPassword(ctx, db.UpdateBillingAccountPasswordParams{
			ID:                id,
			EncryptedPassword: enc,
		})
		if err != nil {
			return AccountResponse{}, mapDBErr(err)
		}
	}
	_, _ = s.RefreshAccount(ctx, id)
	row, err = s.queries.GetBillingAccount(ctx, id)
	if err != nil {
		return AccountResponse{}, mapDBErr(err)
	}
	return toAccountResponse(row), nil
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	if err := s.queries.DeleteBillingAccount(ctx, id); err != nil {
		return mapDBErr(err)
	}
	return nil
}

func (s *Service) RefreshAccount(ctx context.Context, id uuid.UUID) (AccountResponse, error) {
	row, err := s.queries.GetBillingAccount(ctx, id)
	if err != nil {
		return AccountResponse{}, mapDBErr(err)
	}
	if err := s.refreshOne(ctx, row, false); err != nil {
		return AccountResponse{}, err
	}
	row, err = s.queries.GetBillingAccount(ctx, id)
	if err != nil {
		return AccountResponse{}, mapDBErr(err)
	}
	return toAccountResponse(row), nil
}

func (s *Service) RunDue(ctx context.Context) error {
	rows, err := s.queries.ListEnabledBillingAccounts(ctx)
	if err != nil {
		if mapped := mapDBErr(err); mapped != nil {
			if errors.Is(mapped, ErrMigration) {
				return nil
			}
			return mapped
		}
		return err
	}
	var first error
	for _, row := range rows {
		if err := s.refreshOne(ctx, row, true); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func (s *Service) refreshOne(ctx context.Context, row db.BillingAccount, sendAlerts bool) error {
	password, err := s.cipher.Decrypt(row.EncryptedPassword)
	if err != nil {
		_, _ = s.queries.UpdateBillingAccountCache(ctx, db.UpdateBillingAccountCacheParams{
			ID:               row.ID,
			CachedExpireDate: row.CachedExpireDate,
			CachedStatus:     row.CachedStatus,
			CachedName:       row.CachedName,
			CachedCost:       row.CachedCost,
			LastCheckError:   "decrypt password: " + err.Error(),
		})
		return err
	}
	list, err := fetchVDSList(ctx, row.BillmgrUrl, row.Login, password)
	if err != nil {
		_, _ = s.queries.UpdateBillingAccountCache(ctx, db.UpdateBillingAccountCacheParams{
			ID:               row.ID,
			CachedExpireDate: row.CachedExpireDate,
			CachedStatus:     row.CachedStatus,
			CachedName:       row.CachedName,
			CachedCost:       row.CachedCost,
			LastCheckError:   err.Error(),
		})
		return err
	}
	vds, ok := findByIP(list, row.ServerIp)
	if !ok {
		msg := fmt.Sprintf("VPS with IP %s not found in billmgr", row.ServerIp)
		_, _ = s.queries.UpdateBillingAccountCache(ctx, db.UpdateBillingAccountCacheParams{
			ID:               row.ID,
			CachedExpireDate: pgtype.Date{},
			CachedStatus:     "",
			CachedName:       "",
			CachedCost:       "",
			LastCheckError:   msg,
		})
		return fmt.Errorf("%s", msg)
	}
	expire := pgtype.Date{}
	if vds.ExpireDate != "" {
		t, err := time.Parse("2006-01-02", vds.ExpireDate)
		if err == nil {
			expire = pgtype.Date{Time: t, Valid: true}
		}
	}
	_, err = s.queries.UpdateBillingAccountCache(ctx, db.UpdateBillingAccountCacheParams{
		ID:               row.ID,
		CachedExpireDate: expire,
		CachedStatus:     vds.Status,
		CachedName:       vds.Name,
		CachedCost:       vds.Cost,
		LastCheckError:   "",
	})
	if err != nil {
		return err
	}
	if !sendAlerts || !expire.Valid || s.notif == nil {
		return nil
	}
	daysLeft := daysUntil(expire.Time)
	if daysLeft > int(row.AlertDays) {
		return nil
	}
	if row.LastAlertExpireDate.Valid &&
		row.LastAlertExpireDate.Time.Equal(expire.Time) {
		return nil
	}
	msg := fmt.Sprintf(
		"<b>Barn — оплата VPS</b>\nIP: <code>%s</code>\n%s\nИстекает: <b>%s</b> (через %d дн.)\n%s",
		row.ServerIp,
		escapeHTML(vds.Name),
		expire.Time.Format("2006-01-02"),
		daysLeft,
		escapeHTML(vds.Cost),
	)
	if err := s.notif.SendText(ctx, msg); err != nil {
		s.logger.Warn("billing telegram alert failed", "error", err, "ip", row.ServerIp)
		return nil
	}
	_, _ = s.queries.MarkBillingAccountAlerted(ctx, db.MarkBillingAccountAlertedParams{
		ID:                  row.ID,
		LastAlertExpireDate: expire,
	})
	return nil
}

func daysUntil(expire time.Time) int {
	today := time.Now().UTC().Truncate(24 * time.Hour)
	exp := expire.UTC().Truncate(24 * time.Hour)
	return int(exp.Sub(today).Hours() / 24)
}

func toAccountResponse(row db.BillingAccount) AccountResponse {
	resp := AccountResponse{
		ID:             row.ID,
		Provider:       row.Provider,
		ServerIP:       row.ServerIp,
		Login:          row.Login,
		BillmgrURL:     row.BillmgrUrl,
		AlertDays:      int(row.AlertDays),
		Enabled:        row.Enabled,
		PasswordSet:    len(row.EncryptedPassword) > 0,
		Status:         row.CachedStatus,
		Name:           row.CachedName,
		Cost:           row.CachedCost,
		LastCheckError: row.LastCheckError,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
	}
	if row.CachedExpireDate.Valid {
		s := row.CachedExpireDate.Time.Format("2006-01-02")
		resp.ExpireDate = &s
		d := daysUntil(row.CachedExpireDate.Time)
		resp.DaysLeft = &d
	}
	if row.LastCheckedAt.Valid {
		t := row.LastCheckedAt.Time
		resp.LastCheckedAt = &t
	}
	if row.LastAlertAt.Valid {
		t := row.LastAlertAt.Time
		resp.LastAlertAt = &t
	}
	return resp
}

func mapDBErr(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		if pgErr.Code == "42P01" {
			return ErrMigration
		}
		if pgErr.Code == "23505" {
			return wrapInvalid("account for this IP already exists")
		}
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func escapeHTML(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
	return r.Replace(s)
}

func defaultBillmgrURL(provider string) string {
	_ = provider
	return "https://bill.planetahost.ru/billmgr"
}

func normalizeProvider(provider string) (string, error) {
	provider = strings.TrimSpace(provider)
	if provider == "" {
		provider = "planetahost"
	}
	switch provider {
	case "planetahost":
		return provider, nil
	default:
		return "", wrapInvalid("unsupported provider")
	}
}
