package secrets

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/ebash/barn/backend/internal/db"
	sitesvc "github.com/ebash/barn/backend/internal/sites"
)

type Service struct {
	queries *db.Queries
	cipher  *Cipher
}

func NewService(queries *db.Queries, cipher *Cipher) *Service {
	return &Service{queries: queries, cipher: cipher}
}

func (s *Service) List(ctx context.Context, siteID uuid.UUID) ([]SecretResponse, error) {
	if err := s.ensureSite(ctx, siteID); err != nil {
		return nil, err
	}

	rows, err := s.queries.ListSiteSecrets(ctx, siteID)
	if err != nil {
		return nil, fmt.Errorf("list secrets: %w", err)
	}

	out := make([]SecretResponse, 0, len(rows))
	for _, row := range rows {
		key := strings.TrimSpace(row.Key)
		if key == "" {
			// Ignore malformed legacy rows with empty keys.
			continue
		}
		out = append(out, SecretResponse{
			Key:       key,
			CreatedAt: row.CreatedAt,
			UpdatedAt: row.UpdatedAt,
		})
	}
	return out, nil
}

func (s *Service) Set(ctx context.Context, siteID uuid.UUID, key, value string) (SecretResponse, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return SecretResponse{}, fmt.Errorf("%w: key is required", ErrInvalidInput)
	}
	if strings.TrimSpace(value) == "" {
		return SecretResponse{}, fmt.Errorf("%w: value is required", ErrInvalidInput)
	}
	if err := s.ensureSite(ctx, siteID); err != nil {
		return SecretResponse{}, err
	}

	encrypted, err := s.cipher.Encrypt(value)
	if err != nil {
		return SecretResponse{}, fmt.Errorf("encrypt secret: %w", err)
	}

	row, err := s.queries.UpsertSiteSecret(ctx, db.UpsertSiteSecretParams{
		SiteID:         siteID,
		Key:            key,
		EncryptedValue: encrypted,
	})
	if err != nil {
		return SecretResponse{}, fmt.Errorf("upsert secret: %w", err)
	}

	return SecretResponse{
		Key:       row.Key,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}, nil
}

func (s *Service) SetMany(ctx context.Context, siteID uuid.UUID, secrets map[string]string) ([]SecretResponse, error) {
	if err := s.ensureSite(ctx, siteID); err != nil {
		return nil, err
	}

	var out []SecretResponse
	for key, value := range secrets {
		resp, err := s.Set(ctx, siteID, key, value)
		if err != nil {
			return nil, err
		}
		out = append(out, resp)
	}
	return out, nil
}

func (s *Service) Delete(ctx context.Context, siteID uuid.UUID, key string) error {
	if err := s.ensureSite(ctx, siteID); err != nil {
		return err
	}
	if err := s.queries.DeleteSiteSecret(ctx, db.DeleteSiteSecretParams{
		SiteID: siteID,
		Key:    strings.TrimSpace(key),
	}); err != nil {
		return fmt.Errorf("delete secret: %w", err)
	}
	return nil
}

func (s *Service) DecryptForDeploy(ctx context.Context, siteID uuid.UUID) (map[string]string, error) {
	rows, err := s.queries.ListSiteSecrets(ctx, siteID)
	if err != nil {
		return nil, fmt.Errorf("list secrets: %w", err)
	}

	out := make(map[string]string, len(rows))
	for _, meta := range rows {
		key := strings.TrimSpace(meta.Key)
		if key == "" {
			continue
		}
		full, err := s.queries.GetSiteSecret(ctx, db.GetSiteSecretParams{
			SiteID: siteID,
			Key:    key,
		})
		if err != nil {
			return nil, fmt.Errorf("get secret %s: %w", key, err)
		}
		plain, err := s.cipher.Decrypt(full.EncryptedValue)
		if err != nil {
			return nil, fmt.Errorf("decrypt secret %s: %w", key, err)
		}
		out[key] = plain
	}
	return out, nil
}

func (s *Service) ensureSite(ctx context.Context, siteID uuid.UUID) error {
	if _, err := s.queries.GetSite(ctx, siteID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return sitesvc.ErrNotFound
		}
		return fmt.Errorf("get site: %w", err)
	}
	return nil
}
