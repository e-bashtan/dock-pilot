package billing

import (
	"time"

	"github.com/google/uuid"
)

type AccountResponse struct {
	ID               uuid.UUID  `json:"id"`
	Provider         string     `json:"provider"`
	ServerIP         string     `json:"server_ip"`
	Login            string     `json:"login"`
	BillmgrURL       string     `json:"billmgr_url"`
	AlertDays        int        `json:"alert_days"`
	Enabled          bool       `json:"enabled"`
	PasswordSet      bool       `json:"password_set"`
	ExpireDate       *string    `json:"expire_date,omitempty"`
	DaysLeft         *int       `json:"days_left,omitempty"`
	Status           string     `json:"status"`
	Name             string     `json:"name"`
	Cost             string     `json:"cost"`
	LastCheckedAt    *time.Time `json:"last_checked_at,omitempty"`
	LastCheckError   string     `json:"last_check_error"`
	LastAlertAt      *time.Time `json:"last_alert_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

type CreateAccountRequest struct {
	Provider   string `json:"provider"`
	ServerIP   string `json:"server_ip"`
	Login      string `json:"login"`
	Password   string `json:"password"`
	BillmgrURL string `json:"billmgr_url"`
	AlertDays  int    `json:"alert_days"`
	Enabled    *bool  `json:"enabled"`
}

type UpdateAccountRequest struct {
	Provider   *string `json:"provider"`
	ServerIP   *string `json:"server_ip"`
	Login      *string `json:"login"`
	Password   *string `json:"password"`
	BillmgrURL *string `json:"billmgr_url"`
	AlertDays  *int    `json:"alert_days"`
	Enabled    *bool   `json:"enabled"`
}
