package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	DatabaseURL          string
	HTTPAddr             string
	SecretsEncryptionKey string
	APIToken             string
	CORSAllowedOrigins   []string
	Deploy               DeployConfig
}

type DeployConfig struct {
	Mode                string // stub | real
	WorkDir             string
	DockerHost          string
	NginxSitesAvailable string
	NginxSitesEnabled   string
	HostRoot            string // e.g. /host when API runs in Docker with / mounted
	CertbotEmail        string
	PortRangeStart      int
	PortRangeEnd        int
}

func Load() (Config, error) {
	deployMode := envOr("DEPLOY_MODE", "stub")
	if deployMode != "stub" && deployMode != "real" {
		return Config{}, fmt.Errorf("DEPLOY_MODE must be stub or real")
	}

	cfg := Config{
		DatabaseURL:          envOr("DATABASE_URL", "postgres://barn:barn@localhost:5432/barn?sslmode=disable"),
		HTTPAddr:             envOr("HTTP_ADDR", ":8080"),
		SecretsEncryptionKey: os.Getenv("SECRETS_ENCRYPTION_KEY"),
		APIToken:             os.Getenv("API_TOKEN"),
		CORSAllowedOrigins:   parseCORSOrigins(os.Getenv("CORS_ALLOWED_ORIGINS")),
		Deploy: DeployConfig{
			Mode:                deployMode,
			WorkDir:             envOr("DEPLOY_WORK_DIR", "/var/lib/barn"),
			DockerHost:          os.Getenv("DOCKER_HOST"),
			NginxSitesAvailable: envOr("NGINX_SITES_AVAILABLE", "/etc/nginx/sites-available"),
			NginxSitesEnabled:   envOr("NGINX_SITES_ENABLED", "/etc/nginx/sites-enabled"),
			HostRoot:            strings.TrimSpace(os.Getenv("HOST_ROOT")),
			CertbotEmail:        strings.TrimSpace(os.Getenv("CERTBOT_EMAIL")),
			PortRangeStart:      envInt("DEPLOY_PORT_START", 18080),
			PortRangeEnd:        envInt("DEPLOY_PORT_END", 18999),
		},
	}

	if cfg.SecretsEncryptionKey == "" {
		return Config{}, fmt.Errorf("SECRETS_ENCRYPTION_KEY is required")
	}
	if len(cfg.SecretsEncryptionKey) < 32 {
		return Config{}, fmt.Errorf("SECRETS_ENCRYPTION_KEY must be at least 32 bytes")
	}
	if cfg.APIToken == "" {
		return Config{}, fmt.Errorf("API_TOKEN is required")
	}
	if len(cfg.APIToken) < 16 {
		return Config{}, fmt.Errorf("API_TOKEN must be at least 16 characters")
	}

	return cfg, nil
}

func parseCORSOrigins(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return []string{
			"http://localhost:3000",
			"http://127.0.0.1:3000",
		}
	}

	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	var n int
	if _, err := fmt.Sscanf(v, "%d", &n); err != nil || n <= 0 {
		return fallback
	}
	return n
}
