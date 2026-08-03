"use client";

import { useState, useEffect } from "react";
import { ConfirmDialog } from "@/components/ConfirmDialog";
import type { PanelBackupSettings, UpdatePanelBackupSettings } from "@/lib/types";

export function StorageSettingsTab({
  settings,
  t,
  onSave,
  busy,
  saved,
}: {
  settings: PanelBackupSettings | null;
  t: (key: string) => string;
  onSave: (data: UpdatePanelBackupSettings) => Promise<void>;
  busy: boolean;
  saved: boolean;
}) {
  const [editing, setEditing] = useState(false);
  const [replacingKeys, setReplacingKeys] = useState(false);
  const [confirmClear, setConfirmClear] = useState(false);
  const [showAdvanced, setShowAdvanced] = useState(false);

  const [s3Endpoint, setS3Endpoint] = useState("https://storage.yandexcloud.net");
  const [s3Region, setS3Region] = useState("ru-central1");
  const [s3Bucket, setS3Bucket] = useState("");
  const [s3Prefix, setS3Prefix] = useState("barn/backups");
  const [s3Access, setS3Access] = useState("");
  const [s3Secret, setS3Secret] = useState("");
  const [s3PathStyle, setS3PathStyle] = useState(false);

  useEffect(() => {
    if (settings) {
      setS3Endpoint(settings.s3_endpoint || "https://storage.yandexcloud.net");
      setS3Region(settings.s3_region || "ru-central1");
      setS3Bucket(settings.s3_bucket);
      setS3Prefix(settings.s3_prefix || "barn/backups");
      setS3PathStyle(settings.s3_force_path_style);
    }
  }, [settings]);

  const basePayload = (): UpdatePanelBackupSettings => ({
    enabled: settings?.enabled ?? false,
    hour: settings?.hour ?? 3,
    minute: settings?.minute ?? 0,
    timezone: settings?.timezone ?? "UTC",
    s3_endpoint: s3Endpoint.trim(),
    s3_region: s3Region.trim(),
    s3_bucket: s3Bucket.trim(),
    s3_prefix: s3Prefix.trim(),
    s3_force_path_style: s3PathStyle,
    retention_count: settings?.retention_count ?? 7,
  });

  if (!settings) {
    return (
      <div className="card">
        <p style={{ margin: 0 }}>{t("backups.noData")}</p>
      </div>
    );
  }

  return (
    <div>
      <h2 style={{ fontSize: "1.1rem", marginBottom: "1rem" }}>
        {t("backups.storageTitle")}
      </h2>

      {saved && !editing && (
        <div className="alert alert-success" style={{ marginBottom: "1rem" }}>
          {t("backups.saved")}
        </div>
      )}

      {!editing ? (
        <div className="card" style={{ marginBottom: "1.25rem" }}>
          <h3 style={{ fontSize: "0.95rem", marginBottom: "0.75rem" }}>
            {t("backups.overviewStorage")}
          </h3>
          <div style={{ fontSize: "0.875rem" }}>
            <div style={{ marginBottom: "0.5rem" }}>
              <strong>{t("backups.overviewBucket")}:</strong>{" "}
              {settings.s3_bucket || t("backups.noData")}
            </div>
            <div style={{ marginBottom: "0.5rem", overflowWrap: "anywhere" }}>
              <strong>{t("backups.overviewEndpoint")}:</strong>{" "}
              <span style={{ color: "var(--muted)", fontSize: "0.8rem" }}>
                {settings.s3_endpoint || t("backups.noData")}
              </span>
            </div>
            <div style={{ marginBottom: "0.5rem" }}>
              <strong>{t("databases.s3Region")}:</strong>{" "}
              {settings.s3_region || t("backups.noData")}
            </div>
            <div style={{ marginBottom: "0.5rem", overflowWrap: "anywhere" }}>
              <strong>{t("databases.s3Prefix")}:</strong>{" "}
              <span style={{ color: "var(--muted)", fontSize: "0.8rem" }}>
                {settings.s3_prefix || t("backups.noData")}
              </span>
            </div>
            <div style={{ marginBottom: "0.5rem" }}>
              <strong>{t("backups.overviewCredentials")}:</strong>{" "}
              {settings.s3_credentials_set
                ? t("backups.overviewCredsSet")
                : t("backups.overviewCredsNotSet")}
            </div>
          </div>
          <button
            type="button"
            className="btn btn-secondary"
            style={{ marginTop: "0.75rem" }}
            onClick={() => setEditing(true)}
          >
            {t("backups.edit")}
          </button>
        </div>
      ) : (
        <form
          className="card"
          style={{ marginBottom: "1.25rem" }}
          onSubmit={(e) => {
            e.preventDefault();
            void onSave({
              ...basePayload(),
              s3_access_key: s3Access.trim() || undefined,
              s3_secret_key: s3Secret.trim() || undefined,
            }).then(() => {
              setS3Access("");
              setS3Secret("");
              setReplacingKeys(false);
              setEditing(false);
            });
          }}
        >
          <div className="form-grid">
            <div className="field">
              <label className="label" htmlFor="storage-bucket">
                {t("databases.s3Bucket")}
              </label>
              <input
                id="storage-bucket"
                className="input"
                value={s3Bucket}
                onChange={(e) => setS3Bucket(e.target.value)}
                required
              />
            </div>
            <div className="field">
              <label className="label" htmlFor="storage-endpoint">
                {t("databases.s3Endpoint")}
              </label>
              <input
                id="storage-endpoint"
                className="input"
                value={s3Endpoint}
                onChange={(e) => setS3Endpoint(e.target.value)}
              />
            </div>
            <div className="field">
              <label className="label" htmlFor="storage-prefix">
                {t("databases.s3Prefix")}
              </label>
              <input
                id="storage-prefix"
                className="input"
                value={s3Prefix}
                onChange={(e) => setS3Prefix(e.target.value)}
              />
            </div>
          </div>

          <details
            open={showAdvanced}
            onToggle={(e) => setShowAdvanced(e.currentTarget.open)}
            style={{ marginTop: "1rem" }}
          >
            <summary style={{ cursor: "pointer", fontWeight: 500 }}>
              {t("backups.storageAdvanced")}
            </summary>
            <div className="form-grid" style={{ marginTop: "1rem" }}>
              <div className="field">
                <label className="label" htmlFor="storage-region">
                  {t("databases.s3Region")}
                </label>
                <input
                  id="storage-region"
                  className="input"
                  value={s3Region}
                  onChange={(e) => setS3Region(e.target.value)}
                />
              </div>
            </div>
            <div className="field" style={{ marginTop: "0.75rem" }}>
              <label className="label checkbox-row">
                <input
                  type="checkbox"
                  checked={s3PathStyle}
                  onChange={(e) => setS3PathStyle(e.target.checked)}
                />
                <span>{t("databases.s3PathStyle")}</span>
              </label>
            </div>
          </details>

          <div className="form-actions">
            <button
              type="button"
              className="btn btn-secondary"
              onClick={() => {
                setEditing(false);
                setS3Access("");
                setS3Secret("");
                setReplacingKeys(false);
              }}
              disabled={busy}
            >
              {t("common.cancel")}
            </button>
            <button type="submit" className="btn" disabled={busy}>
              {busy ? t("common.saving") : t("common.save")}
            </button>
          </div>
        </form>
      )}

      <div className="card">
        <h3 style={{ fontSize: "0.95rem", marginBottom: "0.75rem" }}>
          {t("backups.storageCredentials")}
        </h3>
        <div style={{ fontSize: "0.875rem", marginBottom: "1rem" }}>
          <div style={{ marginBottom: "0.35rem" }}>
            <strong>{t("databases.s3AccessKey")}:</strong>{" "}
            {settings.s3_credentials_set
              ? t("backups.accessKeyMasked")
              : t("backups.overviewCredsNotSet")}
          </div>
          <div>
            <strong>{t("databases.s3SecretKey")}:</strong>{" "}
            {settings.s3_credentials_set
              ? t("backups.secretKeySaved")
              : t("backups.overviewCredsNotSet")}
          </div>
        </div>

        {!replacingKeys ? (
          <div style={{ display: "flex", gap: "0.75rem", flexWrap: "wrap" }}>
            <button
              type="button"
              className="btn btn-secondary"
              onClick={() => setReplacingKeys(true)}
              disabled={busy}
            >
              {t("backups.replaceKeys")}
            </button>
            {settings.s3_credentials_set && (
              <button
                type="button"
                className="btn btn-secondary"
                onClick={() => setConfirmClear(true)}
                disabled={busy}
              >
                {t("backups.clearKeys")}
              </button>
            )}
          </div>
        ) : (
          <form
            onSubmit={(e) => {
              e.preventDefault();
              void onSave({
                ...basePayload(),
                s3_access_key: s3Access.trim(),
                s3_secret_key: s3Secret.trim(),
              }).then(() => {
                setS3Access("");
                setS3Secret("");
                setReplacingKeys(false);
              });
            }}
          >
            <p className="muted" style={{ marginTop: 0, fontSize: "0.85rem" }}>
              {t("backups.storageCredentialsHint")}
            </p>
            <div className="form-grid">
              <div className="field">
                <label className="label" htmlFor="replace-access">
                  {t("databases.s3AccessKey")}
                </label>
                <input
                  id="replace-access"
                  className="input"
                  value={s3Access}
                  onChange={(e) => setS3Access(e.target.value)}
                  required
                  autoComplete="off"
                />
              </div>
              <div className="field">
                <label className="label" htmlFor="replace-secret">
                  {t("databases.s3SecretKey")}
                </label>
                <input
                  id="replace-secret"
                  className="input"
                  type="password"
                  value={s3Secret}
                  onChange={(e) => setS3Secret(e.target.value)}
                  required
                  autoComplete="new-password"
                />
              </div>
            </div>
            <div className="form-actions">
              <button
                type="button"
                className="btn btn-secondary"
                onClick={() => {
                  setReplacingKeys(false);
                  setS3Access("");
                  setS3Secret("");
                }}
                disabled={busy}
              >
                {t("common.cancel")}
              </button>
              <button
                type="submit"
                className="btn"
                disabled={busy || !s3Access.trim() || !s3Secret.trim()}
              >
                {busy ? t("common.saving") : t("common.save")}
              </button>
            </div>
          </form>
        )}
      </div>

      <ConfirmDialog
        open={confirmClear}
        title={t("backups.clearKeys")}
        message={t("backups.clearKeysConfirm")}
        danger
        busy={busy}
        onCancel={() => setConfirmClear(false)}
        onConfirm={() => {
          void onSave({
            ...basePayload(),
            clear_s3_credentials: true,
          }).then(() => setConfirmClear(false));
        }}
      />
    </div>
  );
}
