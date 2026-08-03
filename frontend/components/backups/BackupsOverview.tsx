"use client";

import { formatBytes, nextDailyRun } from "./format";
import { StatusBadge } from "./StatusBadge";
import { BackupErrorDetails } from "./BackupErrorDetails";
import type {
  BackupOperation,
  PanelBackupSettings,
  PgBackupSchedule,
} from "@/lib/types";

function scheduleDbLabel(
  schedule: PgBackupSchedule,
  dbNamesById: Record<string, string>,
  t: (key: string) => string,
): string {
  if (!schedule.database_id) return t("databases.allDatabases");
  return dbNamesById[schedule.database_id] || t("backups.unknownDatabase");
}

function operationKindLabel(
  kind: string,
  t: (key: string) => string,
): string {
  if (kind === "panel_snapshot") return t("backups.overviewPanelSnapshot");
  if (kind === "pg_backup") return t("backups.overviewDbBackupOp");
  if (kind === "pg_restore") return t("backups.opPgRestore");
  if (kind === "panel_restore") return t("backups.opPanelRestore");
  return kind;
}

export function BackupsOverview({
  settings,
  schedules,
  operations,
  dbNamesById,
  dbCount,
  t,
  formatDateTime,
  onCreateSnapshot,
  onOpenDatabases,
  onOpenSettings,
  busy,
}: {
  settings: PanelBackupSettings | null;
  schedules: PgBackupSchedule[];
  operations: BackupOperation[];
  dbNamesById: Record<string, string>;
  dbCount: number;
  t: (key: string, params?: Record<string, string>) => string;
  formatDateTime: (iso: string) => string;
  onCreateSnapshot: () => void;
  onOpenDatabases: () => void;
  onOpenSettings: () => void;
  busy: boolean;
}) {
  const nextRun = settings?.enabled
    ? nextDailyRun(settings.hour, settings.minute, settings.timezone)
    : null;

  const schedulesWithSuccess = schedules.filter((s) => {
    const st = (s.last_status || "").toLowerCase();
    return st === "ok" || st === "succeeded" || st === "success";
  });
  const schedulesWithError = schedules.filter((s) => {
    const st = (s.last_status || "").toLowerCase();
    return st === "failed" || st === "error" || !!s.last_error;
  });

  const nearestDbRun = schedules
    .filter((s) => s.enabled)
    .map((s) => nextDailyRun(s.hour, s.minute, s.timezone))
    .filter((iso): iso is string => !!iso)
    .sort()[0];

  return (
    <div>
      <h2 style={{ fontSize: "1.05rem", margin: "0 0 1rem" }}>
        {t("backups.overviewStateTitle")}
      </h2>

      <div className="backup-status-grid">
        <div className="card">
          <h3 style={{ fontSize: "0.95rem", marginBottom: "0.75rem" }}>
            {t("backups.overviewPanelSnapshot")}
          </h3>
          {settings ? (
            <div style={{ fontSize: "0.875rem" }}>
              <div style={{ marginBottom: "0.5rem" }}>
                <strong>{t("backups.overviewLastSuccess")}:</strong>{" "}
                {settings.last_run_at && isSuccessStatus(settings.last_status)
                  ? formatDateTime(settings.last_run_at)
                  : t("backups.neverRan")}
              </div>
              <div style={{ marginBottom: "0.5rem" }}>
                <strong>{t("backups.overviewStatus")}:</strong>{" "}
                {settings.last_status ? (
                  <StatusBadge status={settings.last_status} t={t} />
                ) : (
                  t("backups.noData")
                )}
              </div>
              {settings.last_error ? (
                <BackupErrorDetails error={settings.last_error} t={t} />
              ) : null}
              <div style={{ marginBottom: "0.5rem" }}>
                <strong>{t("backups.overviewSchedule")}:</strong>{" "}
                {settings.enabled
                  ? `${String(settings.hour).padStart(2, "0")}:${String(settings.minute).padStart(2, "0")} ${settings.timezone}`
                  : t("backups.scheduleNotConfigured")}
              </div>
              <div style={{ marginBottom: "0.5rem" }}>
                <strong>{t("backups.overviewNextRun")}:</strong>{" "}
                {nextRun ? formatDateTime(nextRun) : t("backups.scheduleNotConfigured")}
              </div>
              <button
                type="button"
                className="btn btn-secondary"
                style={{ marginTop: "0.75rem", fontSize: "0.85rem" }}
                onClick={onCreateSnapshot}
                disabled={busy}
              >
                {busy ? t("common.loading") : t("backups.createSnapshot")}
              </button>
            </div>
          ) : (
            <p style={{ margin: 0, fontSize: "0.85rem", color: "var(--muted)" }}>
              {t("backups.noData")}
            </p>
          )}
        </div>

        <div className="card">
          <h3 style={{ fontSize: "0.95rem", marginBottom: "0.75rem" }}>
            {t("backups.overviewDatabases")}
          </h3>
          <div style={{ fontSize: "0.875rem" }}>
            <div style={{ marginBottom: "0.5rem" }}>
              <strong>{t("backups.overviewDbCount")}:</strong> {dbCount}
            </div>
            <div style={{ marginBottom: "0.5rem" }}>
              <strong>{t("backups.overviewDbSuccess")}:</strong>{" "}
              {schedules.length > 0 ? schedulesWithSuccess.length : t("backups.noData")}
            </div>
            <div style={{ marginBottom: "0.5rem" }}>
              <strong>{t("backups.overviewDbErrors")}:</strong>{" "}
              {schedules.length > 0 ? schedulesWithError.length : t("backups.noData")}
            </div>
            <div style={{ marginBottom: "0.5rem" }}>
              <strong>{t("backups.overviewNextRun")}:</strong>{" "}
              {nearestDbRun
                ? formatDateTime(nearestDbRun)
                : t("backups.scheduleNotConfigured")}
            </div>
            <button
              type="button"
              className="btn btn-secondary"
              style={{ marginTop: "0.75rem", fontSize: "0.85rem" }}
              onClick={onOpenDatabases}
            >
              {t("backups.overviewGoToDatabases")}
            </button>
          </div>
        </div>

        <div className="card">
          <h3 style={{ fontSize: "0.95rem", marginBottom: "0.75rem" }}>
            {t("backups.overviewStorage")}
          </h3>
          {settings ? (
            <div style={{ fontSize: "0.875rem" }}>
              <div style={{ marginBottom: "0.5rem" }}>
                <strong>{t("backups.overviewBucket")}:</strong>{" "}
                {settings.s3_bucket || t("backups.noData")}
              </div>
              <div
                style={{
                  marginBottom: "0.5rem",
                  maxWidth: "100%",
                  overflowWrap: "anywhere",
                }}
              >
                <strong>{t("backups.overviewEndpoint")}:</strong>{" "}
                <span style={{ fontSize: "0.8rem", color: "var(--muted)" }}>
                  {settings.s3_endpoint || t("backups.noData")}
                </span>
              </div>
              <div style={{ marginBottom: "0.5rem" }}>
                <strong>{t("backups.overviewCredentials")}:</strong>{" "}
                {settings.s3_credentials_set
                  ? t("backups.overviewCredsSet")
                  : t("backups.overviewCredsNotSet")}
              </div>
              <button
                type="button"
                className="btn btn-secondary"
                style={{ marginTop: "0.75rem", fontSize: "0.85rem" }}
                onClick={onOpenSettings}
              >
                {t("backups.overviewGoToSettings")}
              </button>
            </div>
          ) : (
            <p style={{ margin: 0, fontSize: "0.85rem", color: "var(--muted)" }}>
              {t("backups.noData")}
            </p>
          )}
        </div>
      </div>

      <div className="card" style={{ marginTop: "1.25rem" }}>
        <h3 style={{ fontSize: "1rem", marginBottom: "0.75rem" }}>
          {t("backups.overviewRecentOps")}
        </h3>
        <div style={{ fontSize: "0.875rem" }}>
          {operations.length > 0 ? (
            <ul style={{ paddingLeft: "1.25rem", margin: 0 }}>
              {operations.slice(0, 10).map((op) => (
                <li key={op.id} style={{ marginBottom: "0.65rem" }}>
                  <strong>{operationKindLabel(op.kind, t)}</strong>
                  {op.database_name ? (
                    <>
                      {" · "}
                      {op.database_name}
                    </>
                  ) : null}
                  {" · "}
                  {formatDateTime(op.started_at)}
                  {" · "}
                  <StatusBadge status={op.status} t={t} />
                  {op.size_bytes > 0 ? (
                    <span style={{ color: "var(--muted)" }}>
                      {" · "}
                      {formatBytes(op.size_bytes)}
                    </span>
                  ) : null}
                  {op.message && op.status === "failed" ? (
                    <BackupErrorDetails error={op.message} t={t} />
                  ) : null}
                </li>
              ))}
            </ul>
          ) : (
            <FallbackRecentOps
              settings={settings}
              schedules={schedules}
              dbNamesById={dbNamesById}
              t={t}
              formatDateTime={formatDateTime}
            />
          )}
        </div>
      </div>
    </div>
  );
}

function FallbackRecentOps({
  settings,
  schedules,
  dbNamesById,
  t,
  formatDateTime,
}: {
  settings: PanelBackupSettings | null;
  schedules: PgBackupSchedule[];
  dbNamesById: Record<string, string>;
  t: (key: string) => string;
  formatDateTime: (iso: string) => string;
}) {
  const recentScheduleOps = schedules
    .filter((s) => s.last_run_at)
    .slice()
    .sort(
      (a, b) =>
        new Date(b.last_run_at!).getTime() - new Date(a.last_run_at!).getTime(),
    )
    .slice(0, 5);

  if (!settings?.last_run_at && recentScheduleOps.length === 0) {
    return (
      <p style={{ margin: 0, color: "var(--muted)" }}>
        {t("backups.overviewNoRecentOps")}
      </p>
    );
  }

  return (
    <ul style={{ paddingLeft: "1.25rem", margin: 0 }}>
      {settings?.last_run_at && (
        <li style={{ marginBottom: "0.5rem" }}>
          <strong>{t("backups.overviewPanelSnapshot")}</strong>
          {" · "}
          {formatDateTime(settings.last_run_at)}
          {" · "}
          <StatusBadge status={settings.last_status} t={t} />
        </li>
      )}
      {recentScheduleOps.map((s) => (
        <li key={s.id} style={{ marginBottom: "0.5rem" }}>
          <strong>{t("backups.overviewDbBackupOp")}</strong>
          {" · "}
          {scheduleDbLabel(s, dbNamesById, t)}
          {" · "}
          {s.last_run_at ? formatDateTime(s.last_run_at) : t("backups.noData")}
          {" · "}
          <StatusBadge status={s.last_status} t={t} />
        </li>
      ))}
    </ul>
  );
}

function isSuccessStatus(status: string): boolean {
  const s = (status || "").toLowerCase();
  return s === "ok" || s === "succeeded" || s === "success";
}
