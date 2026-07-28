"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { ConfirmDialog } from "@/components/ConfirmDialog";
import { HealthBadge } from "@/components/HealthBadge";
import { PostgresDeployLog } from "@/components/PostgresDeployLog";
import { PostgresHealthPanel } from "@/components/PostgresHealthPanel";
import { StatusBadge } from "@/components/StatusBadge";
import { api, ApiError } from "@/lib/api";
import { useI18n } from "@/lib/i18n/context";
import type {
  PgConnectionInfo,
  PgDatabase,
  PgHealth,
  PgInstance,
  PgQueryResult,
  PgRole,
  PgTableInfo,
} from "@/lib/types";

type Tab = "databases" | "roles" | "deployments";

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
  const [health, setHealth] = useState<PgHealth | null>(null);
  const [databases, setDatabases] = useState<PgDatabase[]>([]);
  const [roles, setRoles] = useState<PgRole[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [deploying, setDeploying] = useState(false);
  const [deploySession, setDeploySession] = useState(0);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [connRole, setConnRole] = useState<PgRole | null>(null);
  const [connDbId, setConnDbId] = useState("");
  const [connInfo, setConnInfo] = useState<PgConnectionInfo | null>(null);
  const [adminInfo, setAdminInfo] = useState<PgConnectionInfo | null>(null);
  const [createdPassword, setCreatedPassword] = useState<string | null>(null);
  const [copied, setCopied] = useState<string | null>(null);

  const [dbName, setDbName] = useState("");
  const [roleName, setRoleName] = useState("");
  const [rolePassword, setRolePassword] = useState("");
  const [grantRoleId, setGrantRoleId] = useState("");
  const [grantDbId, setGrantDbId] = useState("");
  const [grantOwner, setGrantOwner] = useState(false);

  const [expandedDbId, setExpandedDbId] = useState<string | null>(null);
  const [tablesByDb, setTablesByDb] = useState<Record<string, PgTableInfo[]>>({});
  const [tablesLoadingDb, setTablesLoadingDb] = useState<string | null>(null);
  const [queryResult, setQueryResult] = useState<{
    database: string;
    table: string;
    result: PgQueryResult;
  } | null>(null);
  const [selectLimit, setSelectLimit] = useState(100);

  const load = useCallback(async () => {
    if (!id) return;
    try {
      const [inst, dbs, roleRows, h] = await Promise.all([
        api.getPgInstance(id),
        api.listPgDatabases(id),
        api.listPgRoles(id),
        api.getPgHealth(id).catch(() => null),
      ]);
      setInstance(inst);
      setDatabases(dbs);
      setRoles(roleRows);
      setHealth(h);
      setError(null);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : t("databases.loadFailed"));
    }
  }, [id, t]);

  useEffect(() => {
    void load();
    const timer = setInterval(() => {
      void api.getPgHealth(id).then(setHealth).catch(() => null);
    }, 30_000);
    return () => clearInterval(timer);
  }, [load, id]);

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

  const copyText = async (key: string, value: string) => {
    try {
      await navigator.clipboard.writeText(value);
      setCopied(key);
      setTimeout(() => setCopied(null), 1500);
    } catch {
      /* ignore */
    }
  };

  const loadRoleConnection = async (role: PgRole, databaseId: string) => {
    if (!databaseId) {
      setConnInfo(null);
      return;
    }
    setBusy(true);
    setError(null);
    try {
      const info = await api.getPgConnection(id, databaseId, role.id);
      setConnInfo(info);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : t("databases.loadFailed"));
      setConnInfo(null);
    } finally {
      setBusy(false);
    }
  };

  const openAdminCredentials = async () => {
    setBusy(true);
    setError(null);
    try {
      const info = await api.getPgAdminCredentials(id);
      setAdminInfo(info);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : t("databases.loadFailed"));
    } finally {
      setBusy(false);
    }
  };

  const loadTables = async (dbId: string) => {
    setTablesLoadingDb(dbId);
    setError(null);
    try {
      const tables = await api.listPgTables(id, dbId);
      setTablesByDb((prev) => ({ ...prev, [dbId]: tables }));
    } catch (e) {
      setError(e instanceof ApiError ? e.message : t("databases.loadFailed"));
      setTablesByDb((prev) => ({ ...prev, [dbId]: [] }));
    } finally {
      setTablesLoadingDb(null);
    }
  };

  const toggleDatabase = (dbId: string) => {
    if (expandedDbId === dbId) {
      setExpandedDbId(null);
      return;
    }
    setExpandedDbId(dbId);
    if (!tablesByDb[dbId]) {
      void loadTables(dbId);
    }
  };

  const runSelect = async (db: PgDatabase, table: string) => {
    setBusy(true);
    setError(null);
    try {
      const result = await api.selectPgTable(id, db.id, {
        table,
        limit: selectLimit,
      });
      setQueryResult({ database: db.name, table, result });
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
              {health ? (
                <>
                  {" · "}
                  <HealthBadge overall={health.overall} />
                </>
              ) : null}
            </p>
          </div>
          <div className="page-actions">
            <button
              type="button"
              className="btn btn-secondary"
              disabled={busy}
              onClick={() => void openAdminCredentials()}
            >
              {t("databases.showAdmin")}
            </button>
            <button
              type="button"
              className="btn"
              disabled={busy || deploying}
              onClick={() => {
                setError(null);
                setTab("deployments");
                setDeploySession((n) => n + 1);
                setDeploying(true);
              }}
            >
              {deploying ? t("databases.deploying") : t("databases.deploy")}
            </button>
            <button
              type="button"
              className="btn btn-secondary"
              disabled={busy || deploying || instance.status === "stopped" || instance.status === "draft"}
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
              disabled={busy || deploying}
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

      <div style={{ marginBottom: "1rem" }}>
        <PostgresHealthPanel instanceId={id} />
      </div>

      {createdPassword && (
        <div className="alert alert-success">
          <strong>{t("databases.createdPassword")}:</strong>{" "}
          <code>{createdPassword}</code>
          <button
            type="button"
            className="btn btn-secondary"
            style={{ marginLeft: "0.75rem" }}
            onClick={() => void copyText("created", createdPassword)}
          >
            {copied === "created" ? t("databases.copied") : t("databases.copy")}
          </button>
          <p className="muted" style={{ margin: "0.5rem 0 0", fontSize: "0.85rem" }}>
            {t("databases.passwordOnce")}
          </p>
        </div>
      )}

      <nav className="site-tabs" aria-label={t("databases.title")}>
        {(["databases", "roles", "deployments"] as Tab[]).map((key) => (
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

      <p className="muted" style={{ marginBottom: "1rem", fontSize: "0.9rem" }}>
        {t("databases.backupsMoved")}{" "}
        <Link href="/backups">{t("nav.backups")}</Link>
      </p>

      {deploySession > 0 ? (
        <div hidden={tab !== "deployments"}>
          <div className="card">
            <h2 className="section-title">{t("databases.deployLog")}</h2>
            <PostgresDeployLog
              key={deploySession}
              instanceId={id}
              onFinished={() => {
                setDeploying(false);
                void load();
              }}
            />
          </div>
        </div>
      ) : null}

      {tab === "deployments" && deploySession === 0 ? (
        <div className="card">
          <h2 className="section-title">{t("databases.deployLog")}</h2>
          <p className="muted" style={{ margin: 0 }}>
            {t("databases.noDeployYet")}
          </p>
        </div>
      ) : null}

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
                {databases.map((db) => {
                  const expanded = expandedDbId === db.id;
                  const tables = tablesByDb[db.id];
                  const tablesLoading = tablesLoadingDb === db.id;
                  return (
                    <div key={db.id}>
                      <div className="stack-item">
                        <div className="stack-item-main">
                          <button
                            type="button"
                            className="linkish"
                            onClick={() => toggleDatabase(db.id)}
                            aria-expanded={expanded}
                          >
                            <code>{db.name}</code>
                            <span className="muted" style={{ marginLeft: "0.5rem" }}>
                              {expanded ? "▾" : "▸"} {t("databases.tables")}
                            </span>
                          </button>
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
                                setTablesByDb((prev) => {
                                  const next = { ...prev };
                                  delete next[db.id];
                                  return next;
                                });
                                if (expandedDbId === db.id) setExpandedDbId(null);
                              })
                            }
                          >
                            {t("databases.delete")}
                          </button>
                        </div>
                      </div>
                      {expanded ? (
                        <div className="db-tables">
                          {tablesLoading ? (
                            <p className="muted" style={{ margin: 0 }}>
                              {t("common.loading")}
                            </p>
                          ) : !tables || tables.length === 0 ? (
                            <p className="muted" style={{ margin: 0 }}>
                              {t("databases.noTables")}
                            </p>
                          ) : (
                            <>
                              <div className="field" style={{ marginBottom: "0.75rem" }}>
                                <label className="label" htmlFor={`select-limit-${db.id}`}>
                                  {t("databases.selectLimit")}
                                </label>
                                <select
                                  id={`select-limit-${db.id}`}
                                  className="select"
                                  value={selectLimit}
                                  onChange={(e) => setSelectLimit(Number(e.target.value))}
                                  style={{ maxWidth: "8rem" }}
                                >
                                  <option value={50}>50</option>
                                  <option value={100}>100</option>
                                  <option value={200}>200</option>
                                  <option value={500}>500</option>
                                </select>
                              </div>
                              <div className="table-wrap">
                                <table className="table table-compact">
                                  <thead>
                                    <tr>
                                      <th>{t("databases.tableName")}</th>
                                      <th>{t("databases.approxRows")}</th>
                                      <th />
                                    </tr>
                                  </thead>
                                  <tbody>
                                    {tables.map((table) => (
                                      <tr key={table.name}>
                                        <td>
                                          <code>{table.name}</code>
                                        </td>
                                        <td>{table.approx_rows}</td>
                                        <td>
                                          <button
                                            type="button"
                                            className="btn btn-secondary"
                                            disabled={busy}
                                            onClick={() => void runSelect(db, table.name)}
                                          >
                                            SELECT
                                          </button>
                                        </td>
                                      </tr>
                                    ))}
                                  </tbody>
                                </table>
                              </div>
                              <button
                                type="button"
                                className="btn btn-secondary"
                                style={{ marginTop: "0.5rem" }}
                                disabled={busy || tablesLoading}
                                onClick={() => void loadTables(db.id)}
                              >
                                {t("databases.refreshTables")}
                              </button>
                            </>
                          )}
                        </div>
                      ) : null}
                    </div>
                  );
                })}
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
                        disabled={busy || databases.length === 0}
                        onClick={() => {
                          const dbId = role.grants?.[0]?.database_id || databases[0]?.id || "";
                          setConnRole(role);
                          setConnDbId(dbId);
                          setConnInfo(null);
                          if (dbId) void loadRoleConnection(role, dbId);
                        }}
                      >
                        {t("databases.showConnection")}
                      </button>
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
                const created = await api.createPgRole(id, {
                  name: roleName.trim(),
                  password: rolePassword || undefined,
                });
                if (created.password) {
                  setCreatedPassword(created.password);
                }
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



      {queryResult && (
        <div
          className="modal-backdrop"
          onClick={() => setQueryResult(null)}
          role="presentation"
        >
          <div
            className="modal modal-wide card"
            onClick={(e) => e.stopPropagation()}
            role="dialog"
            aria-labelledby="pg-query-title"
          >
            <h2 id="pg-query-title" className="section-title">
              {t("databases.selectResult")}: {queryResult.database}.{queryResult.table}
            </h2>
            <p className="muted" style={{ marginTop: 0 }}>
              <code>{queryResult.result.sql}</code>
              {" · "}
              {t("databases.rowsReturned", {
                count: String(queryResult.result.rows.length),
              })}
            </p>
            {queryResult.result.columns.length === 0 ? (
              <p className="muted">{t("databases.emptyResult")}</p>
            ) : (
              <div className="table-wrap query-result-wrap">
                <table className="table table-compact">
                  <thead>
                    <tr>
                      {queryResult.result.columns.map((col) => (
                        <th key={col}>{col}</th>
                      ))}
                    </tr>
                  </thead>
                  <tbody>
                    {queryResult.result.rows.map((row, i) => (
                      <tr key={i}>
                        {row.map((cell, j) => (
                          <td key={j}>
                            <code className="query-cell">{cell}</code>
                          </td>
                        ))}
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
            <div className="form-actions">
              <button
                type="button"
                className="btn btn-secondary"
                onClick={() => setQueryResult(null)}
              >
                {t("common.cancel")}
              </button>
            </div>
          </div>
        </div>
      )}

      {connRole && (
        <div
          className="modal-backdrop"
          onClick={() => {
            setConnRole(null);
            setConnInfo(null);
          }}
          role="presentation"
        >
          <div
            className="modal card"
            onClick={(e) => e.stopPropagation()}
            role="dialog"
            aria-labelledby="pg-conn-title"
          >
            <h2 id="pg-conn-title" className="section-title">
              {t("databases.connectionTitle")}: {connRole.name}
            </h2>
            <div className="field">
              <label className="label" htmlFor="conn-db">
                {t("databases.selectDatabase")}
              </label>
              <select
                id="conn-db"
                className="select"
                value={connDbId}
                onChange={(e) => {
                  const next = e.target.value;
                  setConnDbId(next);
                  void loadRoleConnection(connRole, next);
                }}
              >
                <option value="">—</option>
                {databases.map((d) => (
                  <option key={d.id} value={d.id}>
                    {d.name}
                  </option>
                ))}
              </select>
            </div>
            {connInfo && (
              <ConnectionFields
                info={connInfo}
                copied={copied}
                onCopy={copyText}
                t={t}
              />
            )}
            <div className="form-actions">
              <button
                type="button"
                className="btn btn-secondary"
                onClick={() => {
                  setConnRole(null);
                  setConnInfo(null);
                }}
              >
                {t("common.cancel")}
              </button>
            </div>
          </div>
        </div>
      )}

      {adminInfo && (
        <div
          className="modal-backdrop"
          onClick={() => setAdminInfo(null)}
          role="presentation"
        >
          <div
            className="modal card"
            onClick={(e) => e.stopPropagation()}
            role="dialog"
            aria-labelledby="pg-admin-title"
          >
            <h2 id="pg-admin-title" className="section-title">
              {t("databases.adminTitle")}
            </h2>
            <ConnectionFields
              info={adminInfo}
              copied={copied}
              onCopy={copyText}
              t={t}
            />
            <div className="form-actions">
              <button
                type="button"
                className="btn btn-secondary"
                onClick={() => setAdminInfo(null)}
              >
                {t("common.cancel")}
              </button>
            </div>
          </div>
        </div>
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

    </div>
  );
}

function ConnectionFields({
  info,
  copied,
  onCopy,
  t,
}: {
  info: PgConnectionInfo;
  copied: string | null;
  onCopy: (key: string, value: string) => void;
  t: (key: string, params?: Record<string, string>) => string;
}) {
  return (
    <div className="stack-list" style={{ marginTop: "1rem" }}>
      <div className="field">
        <div className="label">{t("databases.connUrl")}</div>
        <code style={{ display: "block", wordBreak: "break-all", fontSize: "0.8rem" }}>
          {info.url}
        </code>
        <button
          type="button"
          className="btn btn-secondary"
          style={{ marginTop: "0.5rem" }}
          onClick={() => onCopy("url", info.url)}
        >
          {copied === "url" ? t("databases.copied") : t("databases.copy")}
        </button>
      </div>
      <div className="form-grid">
        <div className="field">
          <div className="label">Host</div>
          <code>{info.host}</code>
        </div>
        <div className="field">
          <div className="label">{t("databases.port")}</div>
          <code>{info.port}</code>
        </div>
        <div className="field">
          <div className="label">{t("databases.dbName")}</div>
          <code>{info.database}</code>
        </div>
        <div className="field">
          <div className="label">{t("databases.roleName")}</div>
          <code>{info.user}</code>
        </div>
      </div>
      <div className="field">
        <div className="label">{t("databases.password")}</div>
        <code style={{ wordBreak: "break-all" }}>{info.password}</code>
        <button
          type="button"
          className="btn btn-secondary"
          style={{ marginTop: "0.5rem", marginLeft: "0.5rem" }}
          onClick={() => onCopy("password", info.password)}
        >
          {copied === "password" ? t("databases.copied") : t("databases.copy")}
        </button>
      </div>
    </div>
  );
}
