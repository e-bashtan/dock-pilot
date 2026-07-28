"use client";

import Link from "next/link";
import { useParams } from "next/navigation";
import { useCallback, useEffect, useMemo, useState } from "react";
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

export default function DatabaseInstancePage() {
  const { t } = useI18n();
  const params = useParams();
  const id = String(params.id ?? "");

  const [tab, setTab] = useState<Tab>("databases");
  const [instance, setInstance] = useState<PgInstance | null>(null);
  const [databases, setDatabases] = useState<PgDatabase[]>([]);
  const [roles, setRoles] = useState<PgRole[]>([]);
  const [schedules, setSchedules] = useState<PgBackupSchedule[]>([]);
  const [backups, setBackups] = useState<PgBackup[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

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

  const tzOptions = useMemo(
    () => listTimezones(scheduleTz),
    [scheduleTz],
  );

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
    } catch (e) {
      setError(e instanceof ApiError ? e.message : t("databases.loadFailed"));
    }
  }, [id, t]);

  useEffect(() => {
    load();
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
    return <div className="page"><p className="muted">…</p></div>;
  }

  return (
    <div className="page">
      <p>
        <Link href="/databases">{t("databases.back")}</Link>
      </p>
      {instance && (
        <div className="page-header">
          <div>
            <h1>{instance.name}</h1>
            <p className="muted">
              {instance.slug} · {instance.image} · {instance.status}
              {instance.host_port ? ` · :${instance.host_port}` : ""}
            </p>
            {instance.message ? <p className="error">{instance.message}</p> : null}
          </div>
          <div style={{ display: "flex", gap: "0.5rem", flexWrap: "wrap" }}>
            <button
              type="button"
              className="btn"
              disabled={busy}
              onClick={() => run(async () => { await api.deployPgInstance(id); })}
            >
              {busy ? t("databases.deploying") : t("databases.deploy")}
            </button>
            <button
              type="button"
              className="btn btn-secondary"
              disabled={busy}
              onClick={() => run(async () => { await api.stopPgInstance(id); })}
            >
              {t("databases.stop")}
            </button>
            <button
              type="button"
              className="btn btn-secondary"
              disabled={busy}
              onClick={() => {
                if (!confirm(t("databases.deleteConfirm", { name: instance.name }))) return;
                run(async () => {
                  await api.deletePgInstance(id);
                  window.location.href = "/databases";
                });
              }}
            >
              {t("databases.delete")}
            </button>
          </div>
        </div>
      )}

      {error && <p className="error">{error}</p>}

      <div className="tabs" style={{ marginBottom: "1rem" }}>
        {(["databases", "roles", "backups"] as Tab[]).map((key) => (
          <button
            key={key}
            type="button"
            className={`btn btn-secondary${tab === key ? " active" : ""}`}
            onClick={() => setTab(key)}
          >
            {t(`databases.${key}`)}
          </button>
        ))}
      </div>

      {tab === "databases" && (
        <section>
          <form
            className="card-form"
            onSubmit={(e) => {
              e.preventDefault();
              run(async () => {
                await api.createPgDatabase(id, { name: dbName.trim() });
                setDbName("");
              });
            }}
          >
            <h2 style={{ marginTop: 0 }}>{t("databases.createDatabase")}</h2>
            <label>
              {t("databases.dbName")}
              <input value={dbName} onChange={(e) => setDbName(e.target.value)} required />
            </label>
            <button type="submit" className="btn" disabled={busy}>{t("databases.createDatabase")}</button>
          </form>
          {databases.length === 0 ? (
            <p className="muted">{t("databases.noDatabases")}</p>
          ) : (
            <ul>
              {databases.map((db) => (
                <li key={db.id} style={{ display: "flex", gap: "0.75rem", alignItems: "center", marginBottom: "0.5rem" }}>
                  <code>{db.name}</code>
                  <span className="muted">owner: {db.owner_role}</span>
                  <button
                    type="button"
                    className="btn btn-secondary"
                    disabled={busy}
                    onClick={() => run(async () => { await api.deletePgDatabase(id, db.id); })}
                  >
                    {t("databases.delete")}
                  </button>
                </li>
              ))}
            </ul>
          )}
        </section>
      )}

      {tab === "roles" && (
        <section>
          <form
            className="card-form"
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
            <h2 style={{ marginTop: 0 }}>{t("databases.createRole")}</h2>
            <div className="form-grid">
              <label>
                {t("databases.roleName")}
                <input value={roleName} onChange={(e) => setRoleName(e.target.value)} required />
              </label>
              <label>
                {t("databases.rolePassword")}
                <input
                  type="password"
                  value={rolePassword}
                  onChange={(e) => setRolePassword(e.target.value)}
                  placeholder={t("databases.adminPasswordHint")}
                />
              </label>
            </div>
            <button type="submit" className="btn" disabled={busy}>{t("databases.createRole")}</button>
          </form>

          <form
            className="card-form"
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
            <h2 style={{ marginTop: 0 }}>{t("databases.grant")}</h2>
            <div className="form-grid">
              <label>
                {t("databases.roles")}
                <select value={grantRoleId} onChange={(e) => setGrantRoleId(e.target.value)} required>
                  <option value="">—</option>
                  {roles.map((r) => (
                    <option key={r.id} value={r.id}>{r.name}</option>
                  ))}
                </select>
              </label>
              <label>
                {t("databases.databases")}
                <select value={grantDbId} onChange={(e) => setGrantDbId(e.target.value)} required>
                  <option value="">—</option>
                  {databases.map((d) => (
                    <option key={d.id} value={d.id}>{d.name}</option>
                  ))}
                </select>
              </label>
              <label className="checkbox-row">
                <input type="checkbox" checked={grantOwner} onChange={(e) => setGrantOwner(e.target.checked)} />
                {t("databases.grantOwner")}
              </label>
            </div>
            <button type="submit" className="btn" disabled={busy}>{t("databases.grant")}</button>
          </form>

          {roles.length === 0 ? (
            <p className="muted">{t("databases.noRoles")}</p>
          ) : (
            <ul>
              {roles.map((role) => (
                <li key={role.id} style={{ marginBottom: "0.75rem" }}>
                  <div style={{ display: "flex", gap: "0.75rem", alignItems: "center" }}>
                    <code>{role.name}</code>
                    <button
                      type="button"
                      className="btn btn-secondary"
                      disabled={busy}
                      onClick={() => run(async () => { await api.deletePgRole(id, role.id); })}
                    >
                      {t("databases.delete")}
                    </button>
                  </div>
                  {role.grants?.length ? (
                    <ul className="muted">
                      {role.grants.map((g) => (
                        <li key={g.id}>
                          {g.database_name}{g.is_owner ? " (owner)" : ""}
                        </li>
                      ))}
                    </ul>
                  ) : null}
                </li>
              ))}
            </ul>
          )}
        </section>
      )}

      {tab === "backups" && (
        <section>
          <form
            className="card-form"
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
            <h2 style={{ marginTop: 0 }}>{t("databases.createSchedule")}</h2>
            <div className="form-grid">
              <label>
                {t("databases.scheduleDb")}
                <select value={scheduleDbId} onChange={(e) => setScheduleDbId(e.target.value)}>
                  <option value="">{t("databases.allDatabases")}</option>
                  {databases.map((d) => (
                    <option key={d.id} value={d.id}>{d.name}</option>
                  ))}
                </select>
              </label>
              <label>
                {t("databases.hour")}
                <input type="number" min={0} max={23} value={scheduleHour} onChange={(e) => setScheduleHour(Number(e.target.value))} />
              </label>
              <label>
                {t("databases.minute")}
                <input type="number" min={0} max={59} value={scheduleMinute} onChange={(e) => setScheduleMinute(Number(e.target.value))} />
              </label>
              <label>
                {t("databases.timezone")}
                <select value={scheduleTz} onChange={(e) => setScheduleTz(e.target.value)}>
                  {tzOptions.map((tz) => (
                    <option key={tz} value={tz}>{tz}</option>
                  ))}
                </select>
              </label>
              <label>
                {t("databases.s3Endpoint")}
                <input value={s3Endpoint} onChange={(e) => setS3Endpoint(e.target.value)} placeholder="https://s3.amazonaws.com" />
              </label>
              <label>
                {t("databases.s3Region")}
                <input value={s3Region} onChange={(e) => setS3Region(e.target.value)} />
              </label>
              <label>
                {t("databases.s3Bucket")}
                <input value={s3Bucket} onChange={(e) => setS3Bucket(e.target.value)} required />
              </label>
              <label>
                {t("databases.s3Prefix")}
                <input value={s3Prefix} onChange={(e) => setS3Prefix(e.target.value)} />
              </label>
              <label>
                {t("databases.s3AccessKey")}
                <input value={s3Access} onChange={(e) => setS3Access(e.target.value)} required />
              </label>
              <label>
                {t("databases.s3SecretKey")}
                <input type="password" value={s3Secret} onChange={(e) => setS3Secret(e.target.value)} required />
              </label>
              <label>
                {t("databases.retention")}
                <input type="number" min={1} max={365} value={retention} onChange={(e) => setRetention(Number(e.target.value))} />
              </label>
              <label className="checkbox-row">
                <input type="checkbox" checked={s3PathStyle} onChange={(e) => setS3PathStyle(e.target.checked)} />
                {t("databases.s3PathStyle")}
              </label>
            </div>
            <button type="submit" className="btn" disabled={busy}>{t("databases.createSchedule")}</button>
          </form>

          <h2>{t("databases.schedules")}</h2>
          {schedules.length === 0 ? (
            <p className="muted">—</p>
          ) : (
            <ul>
              {schedules.map((s) => (
                <li key={s.id} style={{ marginBottom: "0.75rem" }}>
                  <div>
                    {String(s.hour).padStart(2, "0")}:{String(s.minute).padStart(2, "0")} {s.timezone}
                    {" · "}
                    {s.s3_bucket}/{s.s3_prefix}
                    {" · "}
                    {s.enabled ? t("databases.enabled") : "off"}
                  </div>
                  <div className="muted" style={{ fontSize: "0.85rem" }}>
                    {t("databases.lastRun")}: {s.last_run_at ?? "—"} {s.last_status}
                  </div>
                  <button
                    type="button"
                    className="btn btn-secondary"
                    disabled={busy}
                    onClick={() => run(async () => { await api.deletePgSchedule(id, s.id); })}
                  >
                    {t("databases.delete")}
                  </button>
                </li>
              ))}
            </ul>
          )}

          <form
            className="card-form"
            onSubmit={(e) => {
              e.preventDefault();
              run(async () => {
                await api.createPgBackup(id, {
                  database_id: backupDbId,
                  ...(backupScheduleId ? { schedule_id: backupScheduleId } : {}),
                });
              });
            }}
          >
            <h2 style={{ marginTop: 0 }}>{t("databases.runBackup")}</h2>
            <div className="form-grid">
              <label>
                {t("databases.databases")}
                <select value={backupDbId} onChange={(e) => setBackupDbId(e.target.value)} required>
                  <option value="">—</option>
                  {databases.map((d) => (
                    <option key={d.id} value={d.id}>{d.name}</option>
                  ))}
                </select>
              </label>
              <label>
                {t("databases.useSchedule")}
                <select value={backupScheduleId} onChange={(e) => setBackupScheduleId(e.target.value)} required>
                  <option value="">—</option>
                  {schedules.map((s) => (
                    <option key={s.id} value={s.id}>
                      {s.s3_bucket} @ {String(s.hour).padStart(2, "0")}:{String(s.minute).padStart(2, "0")}
                    </option>
                  ))}
                </select>
              </label>
            </div>
            <button type="submit" className="btn" disabled={busy || !schedules.length}>
              {t("databases.runBackup")}
            </button>
          </form>

          <h2>{t("databases.backupsList")}</h2>
          {backups.length === 0 ? (
            <p className="muted">—</p>
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
                        <div className="muted" style={{ fontSize: "0.8rem" }}>{b.created_at}</div>
                        <div className="muted" style={{ fontSize: "0.75rem" }}>{b.s3_key}</div>
                      </td>
                      <td>
                        {b.status}
                        {b.message ? <div className="muted">{b.message}</div> : null}
                      </td>
                      <td className="col-hide-mobile">{b.size_bytes}</td>
                      <td>
                        {b.status === "success" && (
                          <button
                            type="button"
                            className="btn btn-secondary"
                            disabled={busy}
                            onClick={() => {
                              const target = restoreTarget.trim() || b.database_name;
                              if (!confirm(t("databases.restoreConfirm", { name: target }))) return;
                              run(async () => {
                                await api.restorePgBackup(id, b.id, {
                                  target_database_name: target,
                                  create_database: restoreCreate,
                                  drop_existing: restoreDrop,
                                });
                              });
                            }}
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

          <div className="card-form" style={{ marginTop: "1rem" }}>
            <label>
              {t("databases.restoreTarget")}
              <input value={restoreTarget} onChange={(e) => setRestoreTarget(e.target.value)} placeholder="same as backup" />
            </label>
            <label className="checkbox-row">
              <input type="checkbox" checked={restoreCreate} onChange={(e) => setRestoreCreate(e.target.checked)} />
              {t("databases.createOnRestore")}
            </label>
            <label className="checkbox-row">
              <input type="checkbox" checked={restoreDrop} onChange={(e) => setRestoreDrop(e.target.checked)} />
              {t("databases.dropOnRestore")}
            </label>
          </div>
        </section>
      )}
    </div>
  );
}
