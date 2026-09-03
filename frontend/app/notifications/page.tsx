"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { api, ApiError } from "@/lib/api";
import { browserTimezone, listTimezones, resolveDigestTimezone } from "@/lib/timezone";
import { useI18n } from "@/lib/i18n/context";
import type { NotificationSettings, UpdateNotificationSettings } from "@/lib/types";
import TelegramTunnelCard from "@/components/TelegramTunnelCard";

export default function NotificationsPage() {
  const { t } = useI18n();
  const [settings, setSettings] = useState<NotificationSettings | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [saved, setSaved] = useState(false);
  const [saving, setSaving] = useState(false);
  const [testing, setTesting] = useState(false);
  const [testOk, setTestOk] = useState(false);

  const [panelName, setPanelName] = useState("");
  const [enabled, setEnabled] = useState(false);
  const [telegramChatID, setTelegramChatID] = useState("");
  const [telegramHTTPProxy, setTelegramHTTPProxy] = useState("");
  const [telegramBotToken, setTelegramBotToken] = useState("");
  const [tokenSet, setTokenSet] = useState(false);
  const [clearToken, setClearToken] = useState(false);
  const [dailyDigestEnabled, setDailyDigestEnabled] = useState(false);
  const [dailyDigestHour, setDailyDigestHour] = useState(9);
  const [dailyDigestMinute, setDailyDigestMinute] = useState(0);
  const [dailyDigestTimezone, setDailyDigestTimezone] = useState(() => browserTimezone());
  const [alertOnIncident, setAlertOnIncident] = useState(true);

  const browserTz = useMemo(() => browserTimezone(), []);
  const timezoneOptions = useMemo(
    () => listTimezones(dailyDigestTimezone, settings?.daily_digest_timezone ?? ""),
    [dailyDigestTimezone, settings?.daily_digest_timezone],
  );

  const load = useCallback(async () => {
    try {
      const s = await api.getNotificationSettings();
      setSettings(s);
      setPanelName(s.panel_name);
      setEnabled(s.enabled);
      setTelegramChatID(s.telegram_chat_id);
      setTelegramHTTPProxy(s.telegram_http_proxy ?? "");
      setTokenSet(s.telegram_bot_token_set);
      setDailyDigestEnabled(s.daily_digest_enabled);
      setDailyDigestHour(s.daily_digest_hour);
      setDailyDigestMinute(s.daily_digest_minute);
      setDailyDigestTimezone(resolveDigestTimezone(s.daily_digest_timezone));
      setAlertOnIncident(s.alert_on_incident_enabled);
      setTelegramBotToken("");
      setClearToken(false);
      setError(null);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : t("notifications.loadFailed"));
    }
  }, [t]);

  useEffect(() => {
    load();
  }, [load]);

  const buildPayload = (): UpdateNotificationSettings => ({
    panel_name: panelName.trim(),
    enabled,
    telegram_chat_id: telegramChatID.trim(),
    telegram_http_proxy: telegramHTTPProxy.trim(),
    daily_digest_enabled: dailyDigestEnabled,
    daily_digest_hour: dailyDigestHour,
    daily_digest_minute: dailyDigestMinute,
    daily_digest_timezone: dailyDigestTimezone,
    alert_on_incident_enabled: alertOnIncident,
    ...(telegramBotToken.trim()
      ? { telegram_bot_token: telegramBotToken.trim() }
      : {}),
    ...(clearToken ? { clear_telegram_bot_token: true } : {}),
  });

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    setSaved(false);
    setTestOk(false);
    setError(null);
    try {
      const updated = await api.updateNotificationSettings(buildPayload());
      setSettings(updated);
      setTokenSet(updated.telegram_bot_token_set);
      setDailyDigestTimezone(resolveDigestTimezone(updated.daily_digest_timezone));
      setTelegramBotToken("");
      setClearToken(false);
      setSaved(true);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("notifications.saveFailed"));
    } finally {
      setSaving(false);
    }
  };

  const handleTest = async () => {
    setTesting(true);
    setTestOk(false);
    setSaved(false);
    setError(null);
    try {
      if (
        settings &&
        (telegramBotToken.trim() ||
          panelName.trim() !== settings.panel_name ||
          telegramChatID.trim() !== settings.telegram_chat_id ||
          telegramHTTPProxy.trim() !== (settings.telegram_http_proxy ?? "") ||
          enabled !== settings.enabled ||
          dailyDigestEnabled !== settings.daily_digest_enabled ||
          dailyDigestHour !== settings.daily_digest_hour ||
          dailyDigestMinute !== settings.daily_digest_minute ||
          dailyDigestTimezone !== resolveDigestTimezone(settings.daily_digest_timezone) ||
          alertOnIncident !== settings.alert_on_incident_enabled)
      ) {
        const updated = await api.updateNotificationSettings(buildPayload());
        setSettings(updated);
        setTokenSet(updated.telegram_bot_token_set);
        setDailyDigestTimezone(resolveDigestTimezone(updated.daily_digest_timezone));
      }
      await api.sendNotificationTest();
      setTestOk(true);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("notifications.testFailed"));
    } finally {
      setTesting(false);
    }
  };

  return (
    <div>
      <h1>{t("notifications.title")}</h1>
      <p style={{ color: "var(--muted)", fontSize: "0.875rem" }}>
        {t("notifications.subtitle")}
      </p>

      {error && <div className="alert alert-error">{error}</div>}
      {saved && <div className="alert alert-success">{t("notifications.saved")}</div>}
      {testOk && (
        <div className="alert alert-success">{t("notifications.testSent")}</div>
      )}

      <TelegramTunnelCard onProxyConfigured={() => void load()} />

      <form onSubmit={handleSave} className="card" style={{ marginTop: "1rem" }}>
        <div className="field">
          <label className="label" htmlFor="panel-name">
            {t("notifications.panelName")}
          </label>
          <input
            id="panel-name"
            className="input"
            type="text"
            value={panelName}
            onChange={(e) => setPanelName(e.target.value)}
          />
          <p style={{ color: "var(--muted)", fontSize: "0.8125rem", margin: "0.35rem 0 0" }}>
            {t("notifications.panelNameHint")}
          </p>
        </div>

        <div className="field">
          <label className="label checkbox-row">
            <input
              type="checkbox"
              checked={enabled}
              onChange={(e) => setEnabled(e.target.checked)}
            />
            <span>{t("notifications.enable")}</span>
          </label>
        </div>

        <div className="field">
          <label className="label" htmlFor="telegram-token">
            {t("notifications.botToken")}
          </label>
          <input
            id="telegram-token"
            className="input"
            type="password"
            autoComplete="off"
            placeholder={
              tokenSet
                ? t("notifications.tokenPlaceholderSet")
                : t("notifications.tokenPlaceholderNew")
            }
            value={telegramBotToken}
            onChange={(e) => setTelegramBotToken(e.target.value)}
          />
          {tokenSet && (
            <label className="label checkbox-row" style={{ marginTop: "0.5rem" }}>
              <input
                type="checkbox"
                checked={clearToken}
                onChange={(e) => setClearToken(e.target.checked)}
              />
              <span>{t("notifications.clearToken")}</span>
            </label>
          )}
        </div>

        <div className="field">
          <label className="label" htmlFor="telegram-chat">
            {t("notifications.chatId")}
          </label>
          <input
            id="telegram-chat"
            className="input"
            type="text"
            placeholder="-1001234567890"
            value={telegramChatID}
            onChange={(e) => setTelegramChatID(e.target.value)}
          />
          <p style={{ color: "var(--muted)", fontSize: "0.8125rem", margin: "0.35rem 0 0" }}>
            {t("notifications.chatIdHint")}
          </p>
        </div>

        <div className="field">
          <label className="label" htmlFor="telegram-proxy">
            {t("notifications.proxy")}
          </label>
          <input
            id="telegram-proxy"
            className="input"
            type="url"
            autoComplete="off"
            placeholder="socks5://127.0.0.1:1080"
            value={telegramHTTPProxy}
            onChange={(e) => setTelegramHTTPProxy(e.target.value)}
          />
          <p style={{ color: "var(--muted)", fontSize: "0.8125rem", margin: "0.35rem 0 0" }}>
            {t("notifications.proxyHint")}
          </p>
        </div>

        <hr style={{ border: "none", borderTop: "1px solid var(--border)", margin: "1.25rem 0" }} />

        <div className="field">
          <label className="label checkbox-row">
            <input
              type="checkbox"
              checked={dailyDigestEnabled}
              onChange={(e) => setDailyDigestEnabled(e.target.checked)}
            />
            <span>{t("notifications.dailyDigest")}</span>
          </label>
        </div>

        <div className="field">
          <label className="label" htmlFor="digest-hour">
            {t("notifications.digestHour")}
          </label>
          <input
            id="digest-hour"
            className="input"
            type="time"
            step={300}
            value={`${String(dailyDigestHour).padStart(2, "0")}:${String(dailyDigestMinute).padStart(2, "0")}`}
            onChange={(e) => {
              const [hour, minute] = e.target.value.split(":").map(Number);
              setDailyDigestHour(hour);
              setDailyDigestMinute(minute);
            }}
            disabled={!dailyDigestEnabled}
          />
          <p style={{ color: "var(--muted)", fontSize: "0.8125rem", margin: "0.35rem 0 0" }}>
            {t("notifications.digestHourHint")}
          </p>
        </div>

        <div className="field">
          <label className="label" htmlFor="digest-timezone">
            {t("notifications.digestTimezone")}
          </label>
          <select
            id="digest-timezone"
            className="input"
            value={dailyDigestTimezone}
            onChange={(e) => setDailyDigestTimezone(e.target.value)}
            disabled={!dailyDigestEnabled}
          >
            {timezoneOptions.map((tz) => (
              <option key={tz} value={tz}>
                {tz}
              </option>
            ))}
          </select>
          <p style={{ color: "var(--muted)", fontSize: "0.8125rem", margin: "0.35rem 0 0" }}>
            {t("notifications.digestTimezoneHint")}
          </p>
          {browserTz !== dailyDigestTimezone && (
            <button
              type="button"
              className="btn btn-secondary"
              style={{ marginTop: "0.5rem" }}
              disabled={!dailyDigestEnabled}
              onClick={() => setDailyDigestTimezone(browserTz)}
            >
              {t("notifications.useBrowserTimezone", { tz: browserTz })}
            </button>
          )}
        </div>

        <div className="field">
          <label className="label checkbox-row">
            <input
              type="checkbox"
              checked={alertOnIncident}
              onChange={(e) => setAlertOnIncident(e.target.checked)}
            />
            <span>{t("notifications.alertOnIncident")}</span>
          </label>
        </div>

        <div style={{ display: "flex", gap: "0.75rem", marginTop: "1.25rem", flexWrap: "wrap" }}>
          <button type="submit" className="btn" disabled={saving}>
            {saving ? t("common.saving") : t("common.save")}
          </button>
          <button
            type="button"
            className="btn btn-secondary"
            disabled={testing || !enabled}
            onClick={handleTest}
          >
            {testing ? t("notifications.sending") : t("notifications.sendTest")}
          </button>
        </div>
      </form>
    </div>
  );
}
