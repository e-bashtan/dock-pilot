"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { ConfirmDialog } from "@/components/ConfirmDialog";
import { api, ApiError } from "@/lib/api";
import { browserTimezone, listTimezones } from "@/lib/timezone";
import { useI18n } from "@/lib/i18n/context";
import type { PgBackup, PgBackupSchedule, PgDatabase } from "@/lib/types";

function formatBytes(n: number): string {
  if (!n || n < 0) return "—";
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  return `${(n / (1024 * 1024)).toFixed(1)} MB`;
}

export function PostgresBackupsPanel({ instanceId }: { instanceId: string }) {
  const { t, formatDateTime } = useI18n();
  const id = instanceId;

  const [databases, setDatabases] = useState<PgDatabase[]>([]);
  const [schedules, setSchedules] = useState<PgBackupSchedule[]>([]);
  const [backups, setBackups] = useState<PgBackup[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [restoreBackup, setRestoreBackup] = useState<PgBackup | null>(null);

  const [scheduleDbId, setScheduleDbId] = useState("");
  const [scheduleHour, setScheduleHour] = useState(3);
  const [scheduleMinute, setScheduleMinute] = useState(0);
  const [scheduleTz, setScheduleTz] = useState(() => browserTimezone());
  const [s3Endpoint, setS3Endpoint] = useState("");
  const [s3Region, setS3Region] = useState("ru-central1");
  const [s3Bucket, setS3Bucket] = useState("");
  const [s3Prefix, setS3Prefix] = useState("dock-pilot/pg-backups");
  const [s3Access, setS3Access] = useState("");
  const [s3Secret, setS3Secret] = useState("");
  const [s3PathStyle, setS3PathStyle] = useState(false);
  const [retention, setRetention] = useState(7);

  const [backupDbId, setBackupDbId] = useState("");
  const [backupScheduleId, setBackupScheduleId] = useState("");
  const [restoreTarget, setRestoreTarget] = useState("");
  const [restoreCreate, setRestoreCreate] = useState(true);
  const [restoreDrop, setRestoreDrop] = useState(false);
  const [uploadFile, setUploadFile] = useState<File | null>(null);
  const [uploadTarget, setUploadTarget] = useState("");
  const [uploadCreate, setUploadCreate] = useState(true);
  const [uploadDrop, setUploadDrop] = useState(false);

  const tzOptions = useMemo(() => listTimezones(scheduleTz), [scheduleTz]);

  const load = useCallback(async () => {
    try {
      const [dbs, sched] = await Promise.all([
        api.listPgDatabases(id),
        api.listPgSchedules(id),
      ]);
      setDatabases(dbs);
      setSchedules(sched);
      setBackupDbId((prev) => prev || dbs[0]?.id || "");
      setBackupScheduleId((prev) => prev || sched[0]?.id || "");
      setError(null);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : t("databases.loadFailed"));
    }
  }, [id, t]);

  const refreshBackups = useCallback(async () => {
    if (!backupScheduleId) {
      setBackups([]);
      return;
    }
    try {
      setBackups(await api.listPgBackups(id, backupScheduleId));
    } catch (e) {
      setError(e instanceof ApiError ? e.message : t("databases.loadFailed"));
    }
  }, [id, backupScheduleId, t]);

  useEffect(() => {
    void load();
  }, [load]);

  useEffect(() => {
    void refreshBackups();
  }, [refreshBackups]);

  const run = async (fn: () => Promise<void>) => {
    setBusy(true);
    setError(null);
    try {
      await fn();
      await load();
      await refreshBackups();
    } catch (e) {
      setError(e instanceof ApiError ? e.message : t("databases.loadFailed"));
    } finally {
      setBusy(false);
    }
  };

  return (
    <div>
      {error && <div className="alert alert-error">{error}</div>}

      <div className="card">
        <h2 className="section-title">{t("databases.schedules")}</h2>
        {schedules.length === 0 ? (
          <p className="muted" style={{ margin: 0 }}>
            {t("databases.noSchedules")}
          </p>
        ) : (
          <div className="stack-list">
            {schedules.map((s) => (
              <div key={s.id} className="stack-item">
                <div className="stack-item-main">
                  <strong>
                    {String(s.hour).padStart(2, "0")}:
                    {String(s.minute).padStart(2, "0")} {s.timezone}
                  </strong>
                  <div className="stack-item-meta">
                    {s.s3_bucket}/{s.s3_prefix}
                    {s.s3_endpoint ? ` · ${s.s3_endpoint}` : ""}
                  </div>
                </div>
                <div className="stack-item-actions">
                  <button
                    type="button"
                    className="btn btn-secondary"
                    disabled={busy}
                    onClick={() =>
                      run(async () => {
                        await api.deletePgSchedule(id, s.id);
                      })
                    }
                  >
                    {t("databases.delete")}
                  </button>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      <div className="card" style={{ padding: 0, overflow: "hidden", marginTop: "1rem" }}>
        <div style={{ padding: "1.25rem 1.25rem 0" }}>
          <h2 className="section-title">{t("databases.backupsList")}</h2>
          <p className="muted" style={{ marginTop: 0, fontSize: "0.85rem" }}>
            {t("databases.backupsFromS3")}
          </p>
          <div className="field" style={{ maxWidth: "16rem" }}>
            <label className="label" htmlFor="pg-list-schedule">
              {t("databases.s3Source")}
            </label>
            <select
              id="pg-list-schedule"
              className="select"
              value={backupScheduleId}
              onChange={(e) => setBackupScheduleId(e.target.value)}
            >
              {schedules.length === 0 ? (
                <option value="">{t("databases.noSchedules")}</option>
              ) : (
                schedules.map((s) => (
                  <option key={s.id} value={s.id}>
                    {s.s3_bucket}/{s.s3_prefix}
                  </option>
                ))
              )}
            </select>
          </div>
        </div>
        {!schedules.length ? (
          <p className="muted" style={{ padding: "0 1.25rem 1.25rem" }}>
            {t("databases.backupsNeedSchedule")}
          </p>
        ) : backups.length === 0 ? (
          <p className="muted" style={{ padding: "0 1.25rem 1.25rem" }}>
            {t("databases.noBackups")}
          </p>
        ) : (
          <div className="table-wrap">
            <table className="table">
              <thead>
                <tr>
                  <th>{t("databases.dbName")}</th>
                  <th className="col-hide-mobile">{t("databases.size")}</th>
                  <th>{t("databases.actions")}</th>
                </tr>
              </thead>
              <tbody>
                {backups.map((b) => (
                  <tr key={b.s3_key}>
                    <td>
                      {b.database_name || "—"}
                      <div style={{ fontSize: "0.75rem", color: "var(--muted)" }}>
                        {formatDateTime(b.created_at)}
                      </div>
                      <div
                        style={{
                          fontSize: "0.7rem",
                          color: "var(--muted)",
                          wordBreak: "break-all",
                        }}
                      >
                        {b.s3_key}
                      </div>
                    </td>
                    <td className="col-hide-mobile">{formatBytes(b.size_bytes)}</td>
                    <td>
                      <button
                        type="button"
                        className="btn btn-secondary"
                        disabled={busy}
                        onClick={() => setRestoreBackup(b)}
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
      </div>

      <form
        className="card"
        style={{ marginTop: "1rem" }}
        onSubmit={(e) => {
          e.preventDefault();
          run(async () => {
            await api.createPgSchedule(id, {
              database_id: scheduleDbId || null,
              hour: scheduleHour,
              minute: scheduleMinute,
              timezone: scheduleTz,
              s3_endpoint: s3Endpoint.trim(),
              s3_region: s3Region.trim(),
              s3_bucket: s3Bucket.trim(),
              s3_prefix: s3Prefix.trim(),
              s3_access_key: s3Access.trim(),
              s3_secret_key: s3Secret.trim(),
              s3_force_path_style: s3PathStyle,
              retention_count: retention,
              enabled: true,
            });
            setS3Access("");
            setS3Secret("");
          });
        }}
      >
        <h2 className="section-title">{t("databases.createSchedule")}</h2>
        <div className="form-grid">
          <div className="field">
            <label className="label" htmlFor="pg-sched-db">
              {t("databases.scheduleDb")}
            </label>
            <select
              id="pg-sched-db"
              className="select"
              value={scheduleDbId}
              onChange={(e) => setScheduleDbId(e.target.value)}
            >
              <option value="">{t("databases.allDatabases")}</option>
              {databases.map((d) => (
                <option key={d.id} value={d.id}>
                  {d.name}
                </option>
              ))}
            </select>
          </div>
          <div className="field">
            <label className="label" htmlFor="pg-s3-bucket">
              {t("databases.s3Bucket")}
            </label>
            <input
              id="pg-s3-bucket"
              className="input"
              value={s3Bucket}
              onChange={(e) => setS3Bucket(e.target.value)}
              required
            />
          </div>
          <div className="field">
            <label className="label" htmlFor="pg-s3-endpoint">
              {t("databases.s3Endpoint")}
            </label>
            <input
              id="pg-s3-endpoint"
              className="input"
              value={s3Endpoint}
              onChange={(e) => setS3Endpoint(e.target.value)}
              placeholder="https://storage.yandexcloud.net"
            />
          </div>
          <div className="field">
            <label className="label" htmlFor="pg-s3-prefix">
              {t("databases.s3Prefix")}
            </label>
            <input
              id="pg-s3-prefix"
              className="input"
              value={s3Prefix}
              onChange={(e) => setS3Prefix(e.target.value)}
            />
          </div>
          <div className="field">
            <label className="label" htmlFor="pg-s3-access">
              {t("databases.s3AccessKey")}
            </label>
            <input
              id="pg-s3-access"
              className="input"
              value={s3Access}
              onChange={(e) => setS3Access(e.target.value)}
              required
            />
          </div>
          <div className="field">
            <label className="label" htmlFor="pg-s3-secret">
              {t("databases.s3SecretKey")}
            </label>
            <input
              id="pg-s3-secret"
              className="input"
              type="password"
              value={s3Secret}
              onChange={(e) => setS3Secret(e.target.value)}
              required
            />
          </div>
          <div className="field">
            <label className="label" htmlFor="pg-sched-hour">
              {t("databases.hour")}
            </label>
            <input
              id="pg-sched-hour"
              className="input"
              type="number"
              min={0}
              max={23}
              value={scheduleHour}
              onChange={(e) => setScheduleHour(Number(e.target.value))}
            />
          </div>
          <div className="field">
            <label className="label" htmlFor="pg-sched-min">
              {t("databases.minute")}
            </label>
            <input
              id="pg-sched-min"
              className="input"
              type="number"
              min={0}
              max={59}
              value={scheduleMinute}
              onChange={(e) => setScheduleMinute(Number(e.target.value))}
            />
          </div>
          <div className="field">
            <label className="label" htmlFor="pg-sched-tz">
              {t("databases.timezone")}
            </label>
            <select
              id="pg-sched-tz"
              className="select"
              value={scheduleTz}
              onChange={(e) => setScheduleTz(e.target.value)}
            >
              {tzOptions.map((tz) => (
                <option key={tz} value={tz}>
                  {tz}
                </option>
              ))}
            </select>
          </div>
          <div className="field">
            <label className="label" htmlFor="pg-retention">
              {t("databases.retention")}
            </label>
            <input
              id="pg-retention"
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
        <div className="form-actions">
          <button type="submit" className="btn" disabled={busy}>
            {t("databases.createSchedule")}
          </button>
        </div>
      </form>

      <form
        className="card"
        style={{ marginTop: "1rem" }}
        onSubmit={(e) => {
          e.preventDefault();
          run(async () => {
            await api.createPgBackup(id, {
              database_id: backupDbId,
              schedule_id: backupScheduleId,
            });
          });
        }}
      >
        <h2 className="section-title">{t("databases.runBackup")}</h2>
        <div className="form-grid">
          <div className="field">
            <label className="label" htmlFor="pg-bak-db">
              {t("databases.dbName")}
            </label>
            <select
              id="pg-bak-db"
              className="select"
              value={backupDbId}
              onChange={(e) => setBackupDbId(e.target.value)}
              required
            >
              {databases.map((d) => (
                <option key={d.id} value={d.id}>
                  {d.name}
                </option>
              ))}
            </select>
          </div>
          <div className="field">
            <label className="label" htmlFor="pg-bak-sched">
              {t("databases.useSchedule")}
            </label>
            <select
              id="pg-bak-sched"
              className="select"
              value={backupScheduleId}
              onChange={(e) => setBackupScheduleId(e.target.value)}
              required
            >
              {schedules.map((s) => (
                <option key={s.id} value={s.id}>
                  {s.s3_bucket}
                </option>
              ))}
            </select>
          </div>
        </div>
        <div className="form-actions">
          <button
            type="submit"
            className="btn"
            disabled={busy || !schedules.length || !databases.length}
          >
            {t("databases.runBackup")}
          </button>
        </div>
      </form>

      <div className="card" style={{ marginTop: "1rem" }}>
        <h2 className="section-title">{t("databases.restoreOptions")}</h2>
        <div className="form-grid">
          <div className="field">
            <label className="label" htmlFor="pg-restore-target">
              {t("databases.restoreTarget")}
            </label>
            <input
              id="pg-restore-target"
              className="input"
              value={restoreTarget}
              onChange={(e) => setRestoreTarget(e.target.value)}
            />
          </div>
        </div>
        <div className="field" style={{ marginTop: "0.75rem" }}>
          <label className="label checkbox-row">
            <input
              type="checkbox"
              checked={restoreCreate}
              onChange={(e) => setRestoreCreate(e.target.checked)}
            />
            <span>{t("databases.createOnRestore")}</span>
          </label>
        </div>
        <div className="field">
          <label className="label checkbox-row">
            <input
              type="checkbox"
              checked={restoreDrop}
              onChange={(e) => setRestoreDrop(e.target.checked)}
            />
            <span>{t("databases.dropOnRestore")}</span>
          </label>
        </div>
      </div>

      <form
        className="card"
        style={{ marginTop: "1rem" }}
        onSubmit={(e) => {
          e.preventDefault();
          if (!uploadFile || !uploadTarget.trim()) return;
          run(async () => {
            await api.restorePgBackupFromFile(id, {
              file: uploadFile,
              target_database_name: uploadTarget.trim(),
              create_database: uploadCreate,
              drop_existing: uploadDrop,
            });
            setUploadFile(null);
            setUploadTarget("");
          });
        }}
      >
        <h2 className="section-title">{t("databases.restoreFromFile")}</h2>
        <p className="muted" style={{ marginTop: 0 }}>
          {t("databases.restoreFromFileHint")}
        </p>
        <div className="form-grid">
          <div className="field">
            <label className="label" htmlFor="pg-restore-file">
              {t("databases.restoreFile")}
            </label>
            <input
              id="pg-restore-file"
              className="input"
              type="file"
              accept=".sql,.sql.gz,.gz"
              onChange={(e) => setUploadFile(e.target.files?.[0] ?? null)}
              required
            />
          </div>
          <div className="field">
            <label className="label" htmlFor="pg-upload-target">
              {t("databases.restoreTarget")}
            </label>
            <input
              id="pg-upload-target"
              className="input"
              value={uploadTarget}
              onChange={(e) => setUploadTarget(e.target.value)}
              required
              pattern="[A-Za-z_][A-Za-z0-9_]*"
            />
          </div>
        </div>
        <div className="field" style={{ marginTop: "0.75rem" }}>
          <label className="label checkbox-row">
            <input
              type="checkbox"
              checked={uploadCreate}
              onChange={(e) => setUploadCreate(e.target.checked)}
            />
            <span>{t("databases.createOnRestore")}</span>
          </label>
        </div>
        <div className="field">
          <label className="label checkbox-row">
            <input
              type="checkbox"
              checked={uploadDrop}
              onChange={(e) => setUploadDrop(e.target.checked)}
            />
            <span>{t("databases.dropOnRestore")}</span>
          </label>
        </div>
        <div className="form-actions">
          <button
            type="submit"
            className="btn"
            disabled={busy || !uploadFile || !uploadTarget.trim()}
          >
            {t("databases.restoreUpload")}
          </button>
        </div>
      </form>

      <ConfirmDialog
        open={!!restoreBackup}
        title={t("databases.restore")}
        message={t("databases.restoreConfirm", {
          name: restoreTarget.trim() || restoreBackup?.database_name || "",
        })}
        busy={busy}
        onCancel={() => setRestoreBackup(null)}
        onConfirm={() => {
          if (!restoreBackup?.s3_key || !restoreBackup.schedule_id) return;
          const target = restoreTarget.trim() || restoreBackup.database_name;
          run(async () => {
            await api.restorePgBackup(id, {
              schedule_id: restoreBackup.schedule_id!,
              s3_key: restoreBackup.s3_key,
              target_database_name: target,
              create_database: restoreCreate,
              drop_existing: restoreDrop,
            });
            setRestoreBackup(null);
          });
        }}
      />
    </div>
  );
}
