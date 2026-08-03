"use client";

import { useState, useCallback, useEffect } from "react";
import { formatBytes } from "./format";
import { StatusBadge } from "./StatusBadge";
import { BackupErrorDetails } from "./BackupErrorDetails";
import { CreateDbBackupDialog } from "./CreateDbBackupDialog";
import { DbScheduleDialog } from "./DbScheduleDialog";
import { RestoreDatabaseWizard } from "./RestoreDatabaseWizard";
import { ConfirmDialog } from "@/components/ConfirmDialog";
import { api, ApiError } from "@/lib/api";
import type {
  PgDatabase,
  PgBackupSchedule,
  PgBackup,
  CreatePgScheduleRequest,
  PanelBackupSettings,
} from "@/lib/types";

export function DatabasesBackupsTab({
  instanceId,
  panelSettings,
  t,
  formatDateTime,
}: {
  instanceId: string;
  panelSettings: PanelBackupSettings | null;
  t: (key: string, params?: Record<string, string>) => string;
  formatDateTime: (iso: string) => string;
}) {
  const [databases, setDatabases] = useState<PgDatabase[]>([]);
  const [schedules, setSchedules] = useState<PgBackupSchedule[]>([]);
  const [backupsMap, setBackupsMap] = useState<Map<string, PgBackup[]>>(new Map());
  const [searchQuery, setSearchQuery] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);

  const [showCreateBackup, setShowCreateBackup] = useState(false);
  const [showCreateSchedule, setShowCreateSchedule] = useState(false);
  const [showRestore, setShowRestore] = useState(false);
  const [restoreBackup, setRestoreBackup] = useState<PgBackup | null>(null);
  const [deleteScheduleId, setDeleteScheduleId] = useState<string | null>(null);
  const [dbDetails, setDbDetails] = useState<string | null>(null);

  const load = useCallback(async () => {
    try {
      const [dbs, sched] = await Promise.all([
        api.listPgDatabases(instanceId),
        api.listPgSchedules(instanceId),
      ]);
      setDatabases(dbs);
      setSchedules(sched);
      setError(null);

      const map = new Map<string, PgBackup[]>();
      for (const schedule of sched) {
        try {
          const backups = await api.listPgBackups(instanceId, schedule.id);
          map.set(schedule.id, backups);
        } catch {
          /* ignore */
        }
      }
      setBackupsMap(map);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : t("databases.loadFailed"));
    } finally {
      setLoading(false);
    }
  }, [instanceId, t]);

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

  const filtered = databases.filter((db) =>
    db.name.toLowerCase().includes(searchQuery.toLowerCase()),
  );

  const getScheduleForDb = (dbId: string) => {
    return (
      schedules.find((s) => s.database_id === dbId) ||
      schedules.find((s) => !s.database_id)
    );
  };

  const getLastBackupForDb = (dbName: string): PgBackup | null => {
    let best: PgBackup | null = null;
    for (const backups of backupsMap.values()) {
      for (const b of backups) {
        if (b.database_name !== dbName) continue;
        if (!best || new Date(b.created_at) > new Date(best.created_at)) {
          best = b;
        }
      }
    }
    return best;
  };

  return (
    <div>
      <h2 style={{ fontSize: "1.1rem", margin: "0 0 0.5rem" }}>
        {t("backups.databasesTitle")}
      </h2>
      <p className="muted" style={{ marginTop: 0, marginBottom: "1.25rem" }}>
        {t("backups.databasesDesc")}
      </p>

      {error && (
        <div className="alert alert-error">
          <BackupErrorDetails error={error} t={t} />
          <button
            type="button"
            className="btn btn-secondary"
            style={{ marginTop: "0.75rem" }}
            onClick={() => {
              setLoading(true);
              void load();
            }}
          >
            {t("common.retry")}
          </button>
        </div>
      )}

      <div
        style={{
          display: "flex",
          justifyContent: "space-between",
          gap: "1rem",
          flexWrap: "wrap",
          marginBottom: "1.25rem",
        }}
      >
        <input
          type="search"
          className="input"
          style={{ maxWidth: "300px" }}
          placeholder={t("backups.searchDatabases")}
          aria-label={t("backups.searchDatabases")}
          value={searchQuery}
          onChange={(e) => setSearchQuery(e.target.value)}
        />
        <div style={{ display: "flex", gap: "0.75rem", flexWrap: "wrap" }}>
          <button
            type="button"
            className="btn"
            onClick={() => setShowCreateBackup(true)}
            disabled={busy || loading}
          >
            {t("backups.createBackup")}
          </button>
          <button
            type="button"
            className="btn btn-secondary"
            onClick={() => setShowCreateSchedule(true)}
            disabled={busy || loading}
          >
            {t("backups.createSchedule")}
          </button>
        </div>
      </div>

      {loading ? (
        <div className="card">
          <p style={{ margin: 0 }}>{t("common.loading")}</p>
        </div>
      ) : databases.length === 0 ? (
        <div className="card">
          <p style={{ margin: 0 }}>{t("databases.noDatabases")}</p>
        </div>
      ) : (
        <>
          <div className="card" style={{ padding: 0, overflow: "hidden" }}>
            <div className="table-wrap">
              <table className="table">
                <thead>
                  <tr>
                    <th>{t("databases.dbName")}</th>
                    <th className="col-hide-mobile">{t("backups.lastBackup")}</th>
                    <th className="col-hide-mobile">{t("backups.schedule")}</th>
                    <th className="col-hide-mobile">{t("backups.lastBackupSize")}</th>
                    <th>{t("common.status")}</th>
                    <th>{t("common.actions")}</th>
                  </tr>
                </thead>
                <tbody>
                  {filtered.map((db) => {
                    const schedule = getScheduleForDb(db.id);
                    const lastBackup = getLastBackupForDb(db.name);
                    return (
                      <tr key={db.id}>
                        <td>
                          <strong>{db.name}</strong>
                        </td>
                        <td className="col-hide-mobile">
                          {lastBackup ? (
                            <div style={{ fontSize: "0.8rem" }}>
                              {formatDateTime(lastBackup.created_at)}
                            </div>
                          ) : schedule?.last_run_at &&
                            (schedule.last_status === "failed" ||
                              !!schedule.last_error) ? (
                            <div style={{ fontSize: "0.8rem", color: "var(--muted)" }}>
                              {t("backups.lastAttempt")}:{" "}
                              {formatDateTime(schedule.last_run_at)}
                            </div>
                          ) : (
                            t("backups.neverRan")
                          )}
                        </td>
                        <td className="col-hide-mobile">
                          {schedule?.enabled ? (
                            <div style={{ fontSize: "0.8rem" }}>
                              {String(schedule.hour).padStart(2, "0")}:
                              {String(schedule.minute).padStart(2, "0")}{" "}
                              {schedule.timezone}
                            </div>
                          ) : (
                            t("backups.scheduleNotConfigured")
                          )}
                        </td>
                        <td className="col-hide-mobile">
                          {lastBackup
                            ? formatBytes(lastBackup.size_bytes)
                            : t("backups.noData")}
                        </td>
                        <td>
                          {schedule?.last_status ? (
                            <StatusBadge status={schedule.last_status} t={t} />
                          ) : (
                            t("backups.noData")
                          )}
                        </td>
                        <td>
                          <button
                            type="button"
                            className="btn btn-secondary"
                            style={{ fontSize: "0.85rem" }}
                            onClick={() => setDbDetails(db.id)}
                            disabled={busy}
                          >
                            {t("backups.details")}
                          </button>
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          </div>

          {schedules.length > 0 && (
            <div className="card" style={{ marginTop: "1.25rem" }}>
              <h3 style={{ fontSize: "1rem", marginBottom: "0.75rem" }}>
                {t("databases.schedules")}
              </h3>
              <div className="stack-list">
                {schedules.map((s) => (
                  <div key={s.id} className="stack-item">
                    <div className="stack-item-main">
                      <strong>
                        {String(s.hour).padStart(2, "0")}:
                        {String(s.minute).padStart(2, "0")} {s.timezone}
                      </strong>
                      <div className="stack-item-meta" style={{ fontSize: "0.8rem" }}>
                        {s.database_id
                          ? databases.find((d) => d.id === s.database_id)?.name ||
                            t("backups.unknownDatabase")
                          : t("databases.allDatabases")}
                        {" · "}
                        <span style={{ color: "var(--muted)" }}>
                          {s.use_panel_s3
                            ? t("backups.usePanelStorage")
                            : `${s.s3_bucket}/${s.s3_prefix}`}
                        </span>
                      </div>
                      {s.last_run_at && (
                        <div
                          style={{
                            fontSize: "0.75rem",
                            color: "var(--muted)",
                            marginTop: "0.25rem",
                          }}
                        >
                          {t("databases.lastRun")}: {formatDateTime(s.last_run_at)} —{" "}
                          <StatusBadge status={s.last_status} t={t} />
                        </div>
                      )}
                      {s.last_error ? (
                        <BackupErrorDetails error={s.last_error} t={t} />
                      ) : null}
                    </div>
                    <div className="stack-item-actions">
                      <button
                        type="button"
                        className="btn btn-secondary"
                        disabled={busy}
                        onClick={() => setDeleteScheduleId(s.id)}
                      >
                        {t("databases.delete")}
                      </button>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}
        </>
      )}

      {dbDetails && (
        <DbDetailsModal
          dbName={databases.find((d) => d.id === dbDetails)?.name || ""}
          backupsMap={backupsMap}
          t={t}
          formatDateTime={formatDateTime}
          onClose={() => setDbDetails(null)}
          onRestore={(backup) => {
            setRestoreBackup(backup);
            setShowRestore(true);
            setDbDetails(null);
          }}
        />
      )}

      <CreateDbBackupDialog
        open={showCreateBackup}
        databases={databases}
        schedules={schedules}
        t={t}
        onClose={() => setShowCreateBackup(false)}
        onCreate={(dbId, scheduleId) =>
          run(async () => {
            await api.createPgBackup(instanceId, {
              database_id: dbId,
              schedule_id: scheduleId,
            });
            setShowCreateBackup(false);
          })
        }
        busy={busy}
      />

      <DbScheduleDialog
        open={showCreateSchedule}
        databases={databases}
        panelSettings={panelSettings}
        t={t}
        onClose={() => setShowCreateSchedule(false)}
        onCreate={(data: CreatePgScheduleRequest) =>
          run(async () => {
            await api.createPgSchedule(instanceId, data);
            setShowCreateSchedule(false);
          })
        }
        busy={busy}
      />

      <RestoreDatabaseWizard
        open={showRestore}
        instanceId={instanceId}
        backup={restoreBackup}
        databases={databases}
        t={t}
        formatDateTime={formatDateTime}
        onClose={() => {
          setShowRestore(false);
          setRestoreBackup(null);
        }}
        onFinished={() => {
          void load();
        }}
      />

      <ConfirmDialog
        open={!!deleteScheduleId}
        title={t("backups.deleteScheduleTitle")}
        message={t("backups.deleteScheduleConfirm")}
        danger
        busy={busy}
        onCancel={() => setDeleteScheduleId(null)}
        onConfirm={() =>
          run(async () => {
            if (!deleteScheduleId) return;
            await api.deletePgSchedule(instanceId, deleteScheduleId);
            setDeleteScheduleId(null);
          })
        }
      />
    </div>
  );
}

function DbDetailsModal({
  dbName,
  backupsMap,
  t,
  formatDateTime,
  onClose,
  onRestore,
}: {
  dbName: string;
  backupsMap: Map<string, PgBackup[]>;
  t: (key: string) => string;
  formatDateTime: (iso: string) => string;
  onClose: () => void;
  onRestore: (backup: PgBackup) => void;
}) {
  const allBackups: PgBackup[] = [];
  for (const backups of backupsMap.values()) {
    allBackups.push(...backups.filter((b) => b.database_name === dbName));
  }
  allBackups.sort(
    (a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime(),
  );

  return (
    <div className="modal-backdrop" onClick={onClose} role="presentation">
      <div
        className="modal card modal-wide"
        onClick={(e) => e.stopPropagation()}
        role="dialog"
      >
        <h2 style={{ marginBottom: "1rem" }}>
          {t("backups.dbDetailsTitle")}: {dbName}
        </h2>

        {allBackups.length === 0 ? (
          <p style={{ margin: 0 }}>{t("databases.noBackups")}</p>
        ) : (
          <div style={{ maxHeight: "60vh", overflowY: "auto" }}>
            <div className="table-wrap">
              <table className="table">
                <thead>
                  <tr>
                    <th>{t("backups.snapshotDate")}</th>
                    <th className="col-hide-mobile">{t("databases.size")}</th>
                    <th>{t("common.actions")}</th>
                  </tr>
                </thead>
                <tbody>
                  {allBackups.map((b) => (
                    <tr key={b.s3_key}>
                      <td>
                        <div>{formatDateTime(b.created_at)}</div>
                        <div
                          style={{
                            fontSize: "0.75rem",
                            color: "var(--muted)",
                            marginTop: "0.25rem",
                            overflowWrap: "anywhere",
                          }}
                        >
                          {formatBytes(b.size_bytes)}
                          {b.status ? (
                            <>
                              {" · "}
                              <StatusBadge status={b.status} t={t} />
                            </>
                          ) : null}
                        </div>
                      </td>
                      <td className="col-hide-mobile">{formatBytes(b.size_bytes)}</td>
                      <td>
                        <button
                          type="button"
                          className="btn btn-secondary"
                          style={{ fontSize: "0.85rem" }}
                          onClick={() => onRestore(b)}
                        >
                          {t("databases.restore")}
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        )}

        <div style={{ marginTop: "1.25rem" }}>
          <button type="button" className="btn btn-secondary" onClick={onClose}>
            {t("common.close")}
          </button>
        </div>
      </div>
    </div>
  );
}
