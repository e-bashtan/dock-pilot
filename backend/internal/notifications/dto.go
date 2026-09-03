package notifications

type SettingsResponse struct {
	PanelName              string `json:"panel_name"`
	Enabled                bool   `json:"enabled"`
	TelegramChatID         string `json:"telegram_chat_id"`
	TelegramHTTPProxy      string `json:"telegram_http_proxy"`
	TelegramBotTokenSet    bool   `json:"telegram_bot_token_set"`
	DailyDigestEnabled     bool   `json:"daily_digest_enabled"`
	DailyDigestHour        int    `json:"daily_digest_hour"`
	DailyDigestMinute      int    `json:"daily_digest_minute"`
	DailyDigestTimezone    string `json:"daily_digest_timezone"`
	AlertOnIncidentEnabled bool   `json:"alert_on_incident_enabled"`
}

type UpdateSettingsRequest struct {
	PanelName              string `json:"panel_name"`
	Enabled                bool   `json:"enabled"`
	TelegramChatID         string `json:"telegram_chat_id"`
	TelegramHTTPProxy      string `json:"telegram_http_proxy"`
	TelegramBotToken       string `json:"telegram_bot_token,omitempty"`
	ClearTelegramBotToken  bool   `json:"clear_telegram_bot_token,omitempty"`
	DailyDigestEnabled     bool   `json:"daily_digest_enabled"`
	DailyDigestHour        int    `json:"daily_digest_hour"`
	DailyDigestMinute      int    `json:"daily_digest_minute"`
	DailyDigestTimezone    string `json:"daily_digest_timezone"`
	AlertOnIncidentEnabled bool   `json:"alert_on_incident_enabled"`
}
