"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { ConfirmDialog } from "@/components/ConfirmDialog";
import { StatusBadge } from "@/components/StatusBadge";
import { api, ApiError } from "@/lib/api";
import { browserTimezone, listTimezones } from "@/lib/timezone";
import { useI18n } from "@/lib/i18n/context";
import type {
  PgBackup,
  PgBackupSchedule,
  PgDatabase,
  PgInstance,
  PgRole,
} from "@/lib/types";

type Tab = "databases" | "roles" | "backups";

function formatBytes(n: number): string {
  if (!n || n < 0) return "—";
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  return `${(n / (1024 * 1024)).toFixed(1)} MB`;
}

export function PostgresManager({
  instanceId,
  onDeleted,
}: {
  instanceId: string;
  onDeleted?: () => void;
}) {
  const { t, formatDateTime } = useI18n();
  const id = instanceId;

  const [tab, setTab] = useState<Tab>("databases");
  const [instance, setInstance] = useState<PgInstance | null>(null);
  const [databases, setDatabases] = useState<PgDatabase[]>([]);
  const [roles, setRoles] = useState<PgRole[]>([]);
  const [schedules, setSchedules] = useState<PgBackupSchedule[]>([]);
  const [backups, setBackups] = useState<PgBackup[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [restoreBackup, setRestoreBackup] = useState<PgBackup | null>(null);

  const [dbName, setDbName] = useState("");
  const [roleName, setRoleName] = useState("");
  const [rolePassword, setRolePassword] = useState("");
  const [grantRoleId, setGrantRoleId] = useState("");
  const [grantDbId, setGrantDbId] = useState("");
  const [grantOwner, setGrantOwner] = useState(false);

  const [scheduleDbId, setScheduleDbId] = useState("");
  const [scheduleHour, setScheduleHour] = useState(3);
  const [scheduleMinute, setScheduleMinute] = useState(0);
  const [scheduleTz, setScheduleTz] = useState(() => browserTimezone());
  const [s3Endpoint, setS3Endpoint] = useState("");
  const [s3Region, setS3Region] = useState("us-east-1");
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

  const tzOptions = useMemo(() => listTimezones(scheduleTz), [scheduleTz]);

  const load = useCallback(async () => {
    if (!id) return;
    try {
      const [inst, dbs, roleRows, sched, bak] = await Promise.all([
        api.getPgInstance(id),
        api.listPgDatabases(id),
        api.listPgRoles(id),
        api.listPgSchedules(id),
        api.listPgBackups(id),
      ]);
      setInstance(inst);
      setDatabases(dbs);
      setRoles(roleRows);
      setSchedules(sched);
      setBackups(bak);
      setError(null);
      setBackupDbId((prev) => prev || dbs[0]?.id || "");
      setBackupScheduleId((prev) => prev || sched[0]?.id || "");
    } catch (e) {
      setError(e instanceof ApiError ? e.message : t("databases.loadFailed"));
    }
  }, [id, t]);

  useEffect(() => {
    void load();
  }, [load]);

  const run = async (fn: () => Promise<void>) => {
    setBusy(true);
    setError(null);
    try {
      await fn();
      await load();
    } catch (e) {
      setError(e instanceof ApiError ? e.message : t("databases.loadFailed"));
    } finally {
      setBusy(false);
    }
  };

  if (!instance && !error) {
    return (
      <div>
        <p className="muted">{t("common.loading")}</p>
      </div>
    );
  }

  return (
    <div>
      {instance && (
        <div className="page-header page-header-tight">
          <div>
            <h1>{t("databases.title")}</h1>
            <p className="page-header-meta">
              {instance.container_name}
              {instance.host_port ? ` · localhost:${instance.host_port}` : ""}
              {instance.docker_network_host ? ` · ${t("databases.networkHost")}` : ""}
              {" · "}
              {instance.image}
              {" · "}
              <StatusBadge
                status={
                  instance.status === "running" ? "active" : instance.status
                }
              />
            </p>
          </div>
          <div className="page-actions">
            <button
              type="button"
              className="btn"
              disabled={busy}
              onClick={() =>
                run(async () => {
                  await api.deployPgInstance(id);
                })
              }
            >
              {busy ? t("databases.deploying") : t("databases.deploy")}
            </button>
            <button
              type="button"
              className="btn btn-secondary"
              disabled={busy || instance.status === "stopped" || instance.status === "draft"}
              onClick={() =>
                run(async () => {
                  await api.stopPgInstance(id);
                })
              }
            >
              {t("databases.stop")}
            </button>
            <button
              type="button"
              className="btn btn-danger"
              disabled={busy}
              onClick={() => setConfirmDelete(true)}
            >
              {t("databases.delete")}
            </button>
          </div>
        </div>
      )}

      {instance?.message ? (
        <div className="alert alert-error" style={{ marginBottom: "1rem" }}>
          {instance.message}
        </div>
      ) : null}
      {error && <div className="alert alert-error">{error}</div>}

      <nav className="site-tabs" aria-label={t("databases.title")}>
        {(["databases", "roles", "backups"] as Tab[]).map((key) => (
          <button
            key={key}
            type="button"
            className={tab === key ? "site-tab site-tab-active" : "site-tab"}
            onClick={() => setTab(key)}
          >
            {t(`databases.${key}`)}
          </button>
        ))}
      </nav>

      {tab === "databases" && (
        <>
          <div className="card">
            <h2 className="section-title">{t("databases.databases")}</h2>
            {databases.length === 0 ? (
              <p className="muted" style={{ margin: 0 }}>
                {t("databases.noDatabases")}
              </p>
            ) : (
              <div className="stack-list">
                {databases.map((db) => (
                  <div key={db.id} className="stack-item">
                    <div className="stack-item-main">
                      <code>{db.name}</code>
                      <div className="stack-item-meta">
                        owner: {db.owner_role} · {formatDateTime(db.created_at)}
                      </div>
                    </div>
                    <div className="stack-item-actions">
                      <button
                        type="button"
                        className="btn btn-secondary"
                        disabled={busy}
                        onClick={() =>
                          run(async () => {
                            await api.deletePgDatabase(id, db.id);
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

          <form
            className="card"
            onSubmit={(e) => {
              e.preventDefault();
              run(async () => {
                await api.createPgDatabase(id, { name: dbName.trim() });
                setDbName("");
              });
            }}
          >
            <h2 className="section-title">{t("databases.createDatabase")}</h2>
            <div className="form-grid">
              <div className="field">
                <label className="label" htmlFor="db-name">
                  {t("databases.dbName")}
                </label>
                <input
                  id="db-name"
                  className="input"
                  value={dbName}
                  onChange={(e) => setDbName(e.target.value)}
                  required
                  pattern="[A-Za-z_][A-Za-z0-9_]*"
                />
              </div>
            </div>
            <div className="form-actions">
              <button type="submit" className="btn" disabled={busy}>
                {t("databases.createDatabase")}
              </button>
            </div>
          </form>
        </>
      )}

      {tab === "roles" && (
        <>
          <div className="card">
            <h2 className="section-title">{t("databases.roles")}</h2>
            {roles.length === 0 ? (
              <p className="muted" style={{ margin: 0 }}>
                {t("databases.noRoles")}
              </p>
            ) : (
              <div className="stack-list">
                {roles.map((role) => (
                  <div key={role.id} className="stack-item">
                    <div className="stack-item-main">
                      <code>{role.name}</code>
                      <div className="stack-item-meta">
                        {role.grants?.length
                          ? role.grants
                              .map(
                                (g) =>
                                  `${g.database_name}${g.is_owner ? " (owner)" : ""}`,
                              )
                              .join(", ")
                          : `${t("databases.grants")}: —`}
                      </div>
                    </div>
                    <div className="stack-item-actions">
                      <button
                        type="button"
                        className="btn btn-secondary"
                        disabled={busy}
                        onClick={() =>
                          run(async () => {
                            await api.deletePgRole(id, role.id);
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

          <form
            className="card"
            onSubmit={(e) => {
              e.preventDefault();
              run(async () => {
                await api.createPgRole(id, {
                  name: roleName.trim(),
                  password: rolePassword || undefined,
                });
                setRoleName("");
                setRolePassword("");
              });
            }}
          >
            <h2 className="section-title">{t("databases.createRole")}</h2>
            <div className="form-grid">
              <div className="field">
                <label className="label" htmlFor="role-name">
                  {t("databases.roleName")}
                </label>
                <input
                  id="role-name"
                  className="input"
                  value={roleName}
                  onChange={(e) => setRoleName(e.target.value)}
                  required
                />
              </div>
              <div className="field">
                <label className="label" htmlFor="role-pass">
                  {t("databases.rolePassword")}
                </label>
                <input
                  id="role-pass"
                  className="input"
                  type="password"
                  autoComplete="new-password"
                  value={rolePassword}
                  onChange={(e) => setRolePassword(e.target.value)}
                  placeholder={t("databases.adminPasswordHint")}
                />
              </div>
            </div>
            <div className="form-actions">
              <button type="submit" className="btn" disabled={busy}>
                {t("databases.createRole")}
              </button>
            </div>
          </form>

          <form
            className="card"
            onSubmit={(e) => {
              e.preventDefault();
              run(async () => {
                await api.grantPgRole(id, grantRoleId, {
                  database_id: grantDbId,
                  is_owner: grantOwner,
                });
              });
            }}
          >
            <h2 className="section-title">{t("databases.grant")}</h2>
            <div className="form-grid">
              <div className="field">
                <label className="label" htmlFor="grant-role">
                  {t("databases.roles")}
                </label>
                <select
                  id="grant-role"
                  className="select"
                  value={grantRoleId}
                  onChange={(e) => setGrantRoleId(e.target.value)}
                  required
                >
                  <option value="">—</option>
                  {roles.map((r) => (
                    <option key={r.id} value={r.id}>
                      {r.name}
                    </option>
                  ))}
                </select>
              </div>
              <div className="field">
                <label className="label" htmlFor="grant-db">
                  {t("databases.databases")}
                </label>
                <select
                  id="grant-db"
                  className="select"
                  value={grantDbId}
                  onChange={(e) => setGrantDbId(e.target.value)}
                  required
                >
                  <option value="">—</option>
                  {databases.map((d) => (
                    <option key={d.id} value={d.id}>
                      {d.name}
                    </option>
                  ))}
                </select>
              </div>
            </div>
            <div className="field" style={{ marginTop: "1rem" }}>
              <label className="label checkbox-row">
                <input
                  type="checkbox"
                  checked={grantOwner}
                  onChange={(e) => setGrantOwner(e.target.checked)}
                />
                <span>{t("databases.grantOwner")}</span>
              </label>
            </div>
            <div className="form-actions">
              <button type="submit" className="btn" disabled={busy}>
                {t("databases.grant")}
              </button>
            </div>
          </form>
        </>
      )}

      {tab === "backups" && (
        <>
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
                        {" · "}
                        {s.enabled ? t("databases.enabled") : t("common.disabled")}
                        <br />
                        {t("databases.lastRun")}:{" "}
                        {s.last_run_at ? formatDateTime(s.last_run_at) : "—"}
                        {s.last_status ? ` · ${s.last_status}` : ""}
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

          <div className="card" style={{ padding: 0, overflow: "hidden" }}>
            <div style={{ padding: "1.25rem 1.25rem 0" }}>
              <h2 className="section-title">{t("databases.backupsList")}</h2>
            </div>
            {backups.length === 0 ? (
              <p className="muted" style={{ padding: "0 1.25rem 1.25rem" }}>
                {t("databases.noBackups")}
              </p>
            ) : (
              <div className="table-wrap">
                <table className="table">
                  <thead>
                    <tr>
                      <th>{t("databases.dbName")}</th>
                      <th>{t("databases.status")}</th>
                      <th className="col-hide-mobile">{t("databases.size")}</th>
                      <th>{t("databases.actions")}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {backups.map((b) => (
                      <tr key={b.id}>
                        <td>
                          {b.database_name}
                          <div
                            style={{
                              fontSize: "0.75rem",
                              color: "var(--muted)",
                            }}
                          >
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
                        <td>
                          <StatusBadge status={b.status} />
                          {b.message ? (
                            <div
                              style={{
                                fontSize: "0.75rem",
                                color: "var(--muted)",
                                marginTop: "0.25rem",
                              }}
                            >
                              {b.message}
                            </div>
                          ) : null}
                        </td>
                        <td className="col-hide-mobile">
                          {formatBytes(b.size_bytes)}
                        </td>
                        <td>
                          {b.status === "success" && (
                            <button
                              type="button"
                              className="btn btn-secondary"
                              disabled={busy}
                              onClick={() => setRestoreBackup(b)}
                            >
                              {t("databases.restore")}
                            </button>
                          )}
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
            <p className="muted" style={{ marginTop: 0, fontSize: "0.875rem" }}>
              {t("databases.scheduleTime")}
            </p>
            <div className="form-grid">
              <div className="field">
                <label className="label" htmlFor="sched-db">
                  {t("databases.scheduleDb")}
                </label>
                <select
                  id="sched-db"
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
                <label className="label" htmlFor="sched-hour">
                  {t("databases.hour")}
                </label>
                <input
                  id="sched-hour"
                  className="input"
                  type="number"
                  min={0}
                  max={23}
                  value={scheduleHour}
                  onChange={(e) => setScheduleHour(Number(e.target.value))}
                />
              </div>
              <div className="field">
                <label className="label" htmlFor="sched-min">
                  {t("databases.minute")}
                </label>
                <input
                  id="sched-min"
                  className="input"
                  type="number"
                  min={0}
                  max={59}
                  value={scheduleMinute}
                  onChange={(e) => setScheduleMinute(Number(e.target.value))}
                />
              </div>
              <div className="field">
                <label className="label" htmlFor="sched-tz">
                  {t("databases.timezone")}
                </label>
                <select
                  id="sched-tz"
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
            </div>

            <h3 className="section-title" style={{ marginTop: "1.25rem" }}>
              {t("databases.s3Settings")}
            </h3>
            <div className="form-grid">
              <div className="field">
                <label className="label" htmlFor="s3-ep">
                  {t("databases.s3Endpoint")}
                </label>
                <input
                  id="s3-ep"
                  className="input"
                  value={s3Endpoint}
                  onChange={(e) => setS3Endpoint(e.target.value)}
                  placeholder="https://s3.amazonaws.com"
                />
              </div>
              <div className="field">
                <label className="label" htmlFor="s3-region">
                  {t("databases.s3Region")}
                </label>
                <input
                  id="s3-region"
                  className="input"
                  value={s3Region}
                  onChange={(e) => setS3Region(e.target.value)}
                />
              </div>
              <div className="field">
                <label className="label" htmlFor="s3-bucket">
                  {t("databases.s3Bucket")}
                </label>
                <input
                  id="s3-bucket"
                  className="input"
                  value={s3Bucket}
                  onChange={(e) => setS3Bucket(e.target.value)}
                  required
                />
              </div>
              <div className="field">
                <label className="label" htmlFor="s3-prefix">
                  {t("databases.s3Prefix")}
                </label>
                <input
                  id="s3-prefix"
                  className="input"
                  value={s3Prefix}
                  onChange={(e) => setS3Prefix(e.target.value)}
                />
              </div>
              <div className="field">
                <label className="label" htmlFor="s3-ak">
                  {t("databases.s3AccessKey")}
                </label>
                <input
                  id="s3-ak"
                  className="input"
                  value={s3Access}
                  onChange={(e) => setS3Access(e.target.value)}
                  required
                  autoComplete="off"
                />
              </div>
              <div className="field">
                <label className="label" htmlFor="s3-sk">
                  {t("databases.s3SecretKey")}
                </label>
                <input
                  id="s3-sk"
                  className="input"
                  type="password"
                  value={s3Secret}
                  onChange={(e) => setS3Secret(e.target.value)}
                  required
                  autoComplete="off"
                />
              </div>
              <div className="field">
                <label className="label" htmlFor="s3-ret">
                  {t("databases.retention")}
                </label>
                <input
                  id="s3-ret"
                  className="input"
                  type="number"
                  min={1}
                  max={365}
                  value={retention}
                  onChange={(e) => setRetention(Number(e.target.value))}
                />
              </div>
            </div>
            <div className="field" style={{ marginTop: "1rem" }}>
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
                <label className="label" htmlFor="bak-db">
                  {t("databases.databases")}
                </label>
                <select
                  id="bak-db"
                  className="select"
                  value={backupDbId}
                  onChange={(e) => setBackupDbId(e.target.value)}
                  required
                >
                  <option value="">—</option>
                  {databases.map((d) => (
                    <option key={d.id} value={d.id}>
                      {d.name}
                    </option>
                  ))}
                </select>
              </div>
              <div className="field">
                <label className="label" htmlFor="bak-sched">
                  {t("databases.useSchedule")}
                </label>
                <select
                  id="bak-sched"
                  className="select"
                  value={backupScheduleId}
                  onChange={(e) => setBackupScheduleId(e.target.value)}
                  required
                >
                  <option value="">—</option>
                  {schedules.map((s) => (
                    <option key={s.id} value={s.id}>
                      {s.s3_bucket} @ {String(s.hour).padStart(2, "0")}:
                      {String(s.minute).padStart(2, "0")}
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

          <div className="card">
            <h2 className="section-title">{t("databases.restoreOptions")}</h2>
            <div className="form-grid">
              <div className="field">
                <label className="label" htmlFor="restore-target">
                  {t("databases.restoreTarget")}
                </label>
                <input
                  id="restore-target"
                  className="input"
                  value={restoreTarget}
                  onChange={(e) => setRestoreTarget(e.target.value)}
                  placeholder="same as backup"
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
        </>
      )}

      <ConfirmDialog
        open={confirmDelete}
        title={t("databases.delete")}
        message={t("databases.deleteConfirm", {
          name: instance?.name ?? "",
        })}
        danger
        busy={busy}
        onCancel={() => setConfirmDelete(false)}
        onConfirm={() => {
          run(async () => {
            await api.deletePgInstance(id);
            setConfirmDelete(false);
            onDeleted?.();
          });
        }}
      />

      <ConfirmDialog
        open={!!restoreBackup}
        title={t("databases.restore")}
        message={t("databases.restoreConfirm", {
          name: restoreTarget.trim() || restoreBackup?.database_name || "",
        })}
        busy={busy}
        onCancel={() => setRestoreBackup(null)}
        onConfirm={() => {
          if (!restoreBackup) return;
          const target = restoreTarget.trim() || restoreBackup.database_name;
          run(async () => {
            await api.restorePgBackup(id, restoreBackup.id, {
              target_database_name: target,
              create_database: restoreCreate,
              drop_existing: restoreDrop,
            });
            setRestoreBackup(null);
            setTab("databases");
          });
        }}
      />
    </div>
  );
}
