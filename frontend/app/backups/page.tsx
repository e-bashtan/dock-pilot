"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import Link from "next/link";
import { ConfirmDialog } from "@/components/ConfirmDialog";
import {
  BackupJobLog,
  appendBackupLog,
  resetBackupLogs,
  type BackupJobLogLine,
} from "@/components/BackupJobLog";
import { PostgresBackupsPanel } from "@/components/PostgresBackupsPanel";
import { api, ApiError } from "@/lib/api";
import { browserTimezone, listTimezones } from "@/lib/timezone";
import { useI18n } from "@/lib/i18n/context";
import type {
  FullPanelBackup,
  PanelBackupSettings,
  PgInstance,
} from "@/lib/types";

function formatBytes(n: number): string {
  if (!n || n < 0) return "—";
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  return `${(n / (1024 * 1024)).toFixed(1)} MB`;
}

export default function BackupsPage() {
  const { t, formatDateTime } = useI18n();
  const [settings, setSettings] = useState<PanelBackupSettings | null>(null);
  const [fullBackups, setFullBackups] = useState<FullPanelBackup[]>([]);
  const [instance, setInstance] = useState<PgInstance | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [saved, setSaved] = useState(false);
  const [restoreKey, setRestoreKey] = useState<string | null>(null);
  const [restoreJob, setRestoreJob] = useState<{
    session: number;
  } | null>(null);
  const [restoreLogs, setRestoreLogs] = useState<BackupJobLogLine[]>([]);
  const [restoreStatus, setRestoreStatus] = useState("running");
  const restoreLogsRef = useRef<BackupJobLogLine[]>([]);

  const [enabled, setEnabled] = useState(false);
  const [hour, setHour] = useState(3);
  const [minute, setMinute] = useState(0);
  const [timezone, setTimezone] = useState(() => browserTimezone());
  const [s3Endpoint, setS3Endpoint] = useState("https://storage.yandexcloud.net");
  const [s3Region, setS3Region] = useState("ru-central1");
  const [s3Bucket, setS3Bucket] = useState("");
  const [s3Prefix, setS3Prefix] = useState("barn/backups");
  const [s3Access, setS3Access] = useState("");
  const [s3Secret, setS3Secret] = useState("");
  const [s3PathStyle, setS3PathStyle] = useState(false);
  const [clearKeys, setClearKeys] = useState(false);
  const [retention, setRetention] = useState(7);

  const tzOptions = useMemo(() => listTimezones(timezone), [timezone]);

  const load = useCallback(async () => {
    try {
      const [s, full, instances] = await Promise.all([
        api.getPanelBackupSettings(),
        api.listFullPanelBackups().catch(() => [] as FullPanelBackup[]),
        api.listPgInstances().catch(() => [] as PgInstance[]),
      ]);
      setSettings(s);
      setFullBackups(full);
      setInstance(instances[0] ?? null);
      setEnabled(s.enabled);
      setHour(s.hour);
      setMinute(s.minute);
      setTimezone(s.timezone || browserTimezone());
      setS3Endpoint(s.s3_endpoint || "https://storage.yandexcloud.net");
      setS3Region(s.s3_region || "ru-central1");
      setS3Bucket(s.s3_bucket);
      setS3Prefix(s.s3_prefix || "barn/backups");
      setS3PathStyle(s.s3_force_path_style);
      setRetention(s.retention_count || 7);
      setError(null);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : t("backups.loadFailed"));
    }
  }, [t]);

  useEffect(() => {
    void load();
  }, [load]);

  const run = async (fn: () => Promise<void>) => {
    setBusy(true);
    setError(null);
    setSaved(false);
    try {
      await fn();
      await load();
    } catch (e) {
      setError(e instanceof ApiError ? e.message : t("backups.loadFailed"));
    } finally {
      setBusy(false);
    }
  };

  return (
    <div>
      <div className="page-header">
        <div>
          <h1>{t("backups.title")}</h1>
          <p className="page-header-meta">{t("backups.subtitle")}</p>
        </div>
      </div>

      {error && <div className="alert alert-error">{error}</div>}
      {saved && <div className="alert alert-success">{t("backups.saved")}</div>}

      <form
        className="card"
        onSubmit={(e) => {
          e.preventDefault();
          run(async () => {
            await api.updatePanelBackupSettings({
              enabled,
              hour,
              minute,
              timezone,
              s3_endpoint: s3Endpoint.trim(),
              s3_region: s3Region.trim(),
              s3_bucket: s3Bucket.trim(),
              s3_prefix: s3Prefix.trim(),
              s3_access_key: s3Access.trim() || undefined,
              s3_secret_key: s3Secret.trim() || undefined,
              clear_s3_credentials: clearKeys,
              s3_force_path_style: s3PathStyle,
              retention_count: retention,
            });
            setS3Access("");
            setS3Secret("");
            setClearKeys(false);
            setSaved(true);
          });
        }}
      >
        <h2 className="section-title">{t("backups.destination")}</h2>
        <p className="muted" style={{ marginTop: 0 }}>
          {t("backups.destinationHint")}
        </p>
        <div className="field">
          <label className="label checkbox-row">
            <input
              type="checkbox"
              checked={enabled}
              onChange={(e) => setEnabled(e.target.checked)}
            />
            <span>{t("backups.scheduleEnabled")}</span>
          </label>
        </div>
        <div className="form-grid">
          <div className="field">
            <label className="label" htmlFor="full-bucket">
              {t("databases.s3Bucket")}
            </label>
            <input
              id="full-bucket"
              className="input"
              value={s3Bucket}
              onChange={(e) => setS3Bucket(e.target.value)}
              required={enabled}
            />
          </div>
          <div className="field">
            <label className="label" htmlFor="full-endpoint">
              {t("databases.s3Endpoint")}
            </label>
            <input
              id="full-endpoint"
              className="input"
              value={s3Endpoint}
              onChange={(e) => setS3Endpoint(e.target.value)}
            />
          </div>
          <div className="field">
            <label className="label" htmlFor="full-region">
              {t("databases.s3Region")}
            </label>
            <input
              id="full-region"
              className="input"
              value={s3Region}
              onChange={(e) => setS3Region(e.target.value)}
            />
          </div>
          <div className="field">
            <label className="label" htmlFor="full-prefix">
              {t("databases.s3Prefix")}
            </label>
            <input
              id="full-prefix"
              className="input"
              value={s3Prefix}
              onChange={(e) => setS3Prefix(e.target.value)}
            />
          </div>
          <div className="field">
            <label className="label" htmlFor="full-access">
              {t("databases.s3AccessKey")}
            </label>
            <input
              id="full-access"
              className="input"
              value={s3Access}
              onChange={(e) => setS3Access(e.target.value)}
              placeholder={
                settings?.s3_credentials_set
                  ? t("backups.keysSet")
                  : undefined
              }
            />
          </div>
          <div className="field">
            <label className="label" htmlFor="full-secret">
              {t("databases.s3SecretKey")}
            </label>
            <input
              id="full-secret"
              className="input"
              type="password"
              value={s3Secret}
              onChange={(e) => setS3Secret(e.target.value)}
            />
          </div>
          <div className="field">
            <label className="label" htmlFor="full-hour">
              {t("databases.hour")}
            </label>
            <input
              id="full-hour"
              className="input"
              type="number"
              min={0}
              max={23}
              value={hour}
              onChange={(e) => setHour(Number(e.target.value))}
            />
          </div>
          <div className="field">
            <label className="label" htmlFor="full-minute">
              {t("databases.minute")}
            </label>
            <input
              id="full-minute"
              className="input"
              type="number"
              min={0}
              max={59}
              value={minute}
              onChange={(e) => setMinute(Number(e.target.value))}
            />
          </div>
          <div className="field">
            <label className="label" htmlFor="full-tz">
              {t("databases.timezone")}
            </label>
            <select
              id="full-tz"
              className="select"
              value={timezone}
              onChange={(e) => setTimezone(e.target.value)}
            >
              {tzOptions.map((tz) => (
                <option key={tz} value={tz}>
                  {tz}
                </option>
              ))}
            </select>
          </div>
          <div className="field">
            <label className="label" htmlFor="full-retention">
              {t("databases.retention")}
            </label>
            <input
              id="full-retention"
              className="input"
              type="number"
              min={1}
              max={365}
              value={retention}
              onChange={(e) => setRetention(Number(e.target.value))}
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
        {settings?.s3_credentials_set ? (
          <div className="field">
            <label className="label checkbox-row">
              <input
                type="checkbox"
                checked={clearKeys}
                onChange={(e) => setClearKeys(e.target.checked)}
              />
              <span>{t("backups.clearKeys")}</span>
            </label>
          </div>
        ) : null}
        {settings?.last_run_at || settings?.last_status ? (
          <p className="muted" style={{ fontSize: "0.85rem" }}>
            {t("databases.lastRun")}:{" "}
            {settings.last_run_at ? formatDateTime(settings.last_run_at) : "—"}
            {settings.last_status ? ` · ${settings.last_status}` : ""}
          </p>
        ) : null}
        <div className="form-actions">
          <button type="submit" className="btn" disabled={busy}>
            {busy ? t("common.saving") : t("common.save")}
          </button>
        </div>
      </form>

      <div className="card" style={{ marginTop: "1.25rem" }}>
        <div
          style={{
            display: "flex",
            justifyContent: "space-between",
            gap: "1rem",
            flexWrap: "wrap",
            alignItems: "center",
          }}
        >
          <div>
            <h2 className="section-title" style={{ marginBottom: "0.25rem" }}>
              {t("backups.fullTitle")}
            </h2>
            <p className="muted" style={{ margin: 0, fontSize: "0.85rem" }}>
              {t("backups.fullHint")}
            </p>
          </div>
          <button
            type="button"
            className="btn"
            disabled={busy}
            onClick={() =>
              run(async () => {
                await api.createFullPanelBackup();
              })
            }
          >
            {t("backups.createFull")}
          </button>
        </div>
        {fullBackups.length === 0 ? (
          <p className="muted" style={{ marginTop: "1rem", marginBottom: 0 }}>
            {t("backups.noFull")}
          </p>
        ) : (
          <div className="table-wrap" style={{ marginTop: "1rem" }}>
            <table className="table">
              <thead>
                <tr>
                  <th>{t("databases.key")}</th>
                  <th className="col-hide-mobile">{t("databases.size")}</th>
                  <th>{t("databases.actions")}</th>
                </tr>
              </thead>
              <tbody>
                {fullBackups.map((b) => (
                  <tr key={b.s3_key}>
                    <td>
                      <code style={{ fontSize: "0.8rem", wordBreak: "break-all" }}>
                        {b.s3_key}
                      </code>
                      <div style={{ fontSize: "0.75rem", color: "var(--muted)" }}>
                        {formatDateTime(b.created_at)}
                      </div>
                    </td>
                    <td className="col-hide-mobile">{formatBytes(b.size_bytes)}</td>
                    <td>
                      <button
                        type="button"
                        className="btn btn-secondary"
                        disabled={busy}
                        onClick={() => setRestoreKey(b.s3_key)}
                      >
                        {t("databases.restore")}
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
        {restoreJob ? (
          <BackupJobLog
            key={restoreJob.session}
            embedded
            title={t("backups.restoreLog")}
            logs={restoreLogs}
            status={restoreStatus}
          />
        ) : null}
      </div>

      <div style={{ marginTop: "2rem" }}>
        <h2>{t("backups.postgresTitle")}</h2>
        <p className="muted">{t("backups.postgresHint")}</p>
        {instance ? (
          <PostgresBackupsPanel instanceId={instance.id} />
        ) : (
          <div className="card">
            <p style={{ margin: 0 }}>{t("backups.noPostgres")}</p>
            <Link href="/databases" className="btn" style={{ marginTop: "0.75rem" }}>
              {t("nav.databases")}
            </Link>
          </div>
        )}
      </div>

      <ConfirmDialog
        open={!!restoreKey}
        title={t("backups.restoreFull")}
        message={t("backups.restoreFullConfirm")}
        busy={busy}
        onCancel={() => setRestoreKey(null)}
        onConfirm={() => {
          if (!restoreKey) return;
          const key = restoreKey;
          setRestoreKey(null);
          resetBackupLogs(restoreLogsRef, setRestoreLogs);
          setRestoreStatus("running");
          setRestoreJob((prev) => ({
            session: (prev?.session ?? 0) + 1,
          }));

          const es = api.streamFullPanelBackupRestore(key);
          es.addEventListener("log", (ev) => {
            try {
              const data = JSON.parse((ev as MessageEvent).data) as {
                level?: string;
                message?: string;
                at?: string;
              };
              appendBackupLog(
                restoreLogsRef,
                setRestoreLogs,
                data.level ?? "info",
                data.message ?? "",
                data.at ?? new Date().toISOString(),
              );
            } catch {
              /* ignore */
            }
          });
          es.addEventListener("done", (ev) => {
            try {
              const data = JSON.parse((ev as MessageEvent).data) as {
                status?: string;
              };
              setRestoreStatus(data.status ?? "succeeded");
            } catch {
              setRestoreStatus("failed");
            }
            es.close();
            void load();
          });
          es.onerror = () => {
            setRestoreStatus("failed");
            es.close();
          };
        }}
      />
    </div>
  );
}
