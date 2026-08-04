"use client";

import type { ReactNode } from "react";
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

function formatScheduleTime(hour: number, minute: number, timezone: string) {
  return `${String(hour).padStart(2, "0")}:${String(minute).padStart(2, "0")} ${timezone || "UTC"}`;
}

function isSuccessStatus(status: string): boolean {
  const s = (status || "").toLowerCase();
  return s === "ok" || s === "succeeded" || s === "success";
}

function isErrorStatus(status: string, lastError?: string): boolean {
  const s = (status || "").toLowerCase();
  return s === "failed" || s === "error" || !!lastError;
}

function OverviewRow({
  label,
  children,
}: {
  label: string;
  children: ReactNode;
}) {
  return (
    <div className="backup-status-card-row">
      <strong>{label}</strong>
      <span>{children}</span>
    </div>
  );
}

function OverviewCard({
  title,
  children,
  action,
}: {
  title: string;
  children: ReactNode;
  action: ReactNode;
}) {
  return (
    <div className="card backup-status-card">
      <h3>{title}</h3>
      <div className="backup-status-card-body">{children}</div>
      <div className="backup-status-card-actions">{action}</div>
    </div>
  );
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
  const panelNextRun = settings?.enabled
    ? nextDailyRun(settings.hour, settings.minute, settings.timezone)
    : null;

  const enabledSchedules = schedules.filter((s) => s.enabled);
  const primarySchedule =
    enabledSchedules[0] ||
    schedules
      .slice()
      .sort(
        (a, b) =>
          new Date(b.last_run_at || 0).getTime() -
          new Date(a.last_run_at || 0).getTime(),
      )[0];

  const latestDbRun = schedules
    .filter((s) => s.last_run_at)
    .slice()
    .sort(
      (a, b) =>
        new Date(b.last_run_at!).getTime() - new Date(a.last_run_at!).getTime(),
    )[0];

  const dbAggregateStatus = (() => {
    if (schedules.some((s) => isErrorStatus(s.last_status, s.last_error))) {
      return "failed";
    }
    if (schedules.some((s) => isSuccessStatus(s.last_status))) {
      return "ok";
    }
    return latestDbRun?.last_status || "";
  })();

  const nearestDbRun = enabledSchedules
    .map((s) => nextDailyRun(s.hour, s.minute, s.timezone))
    .filter((iso): iso is string => !!iso)
    .sort()[0];

  const dbScheduleLabel = (() => {
    if (schedules.length === 0) return t("backups.scheduleNotConfigured");
    if (schedules.length === 1 && primarySchedule) {
      const enabledLabel = primarySchedule.enabled
        ? t("common.enabled")
        : t("common.disabled");
      return `${enabledLabel} · ${formatScheduleTime(primarySchedule.hour, primarySchedule.minute, primarySchedule.timezone)}`;
    }
    const enabledCount = enabledSchedules.length;
    const sample = primarySchedule
      ? formatScheduleTime(
          primarySchedule.hour,
          primarySchedule.minute,
          primarySchedule.timezone,
        )
      : "";
    return t("backups.overviewSchedulesSummary", {
      total: String(schedules.length),
      enabled: String(enabledCount),
      time: sample,
    });
  })();

  const storageReady = !!(
    settings?.s3_bucket &&
    settings?.s3_endpoint &&
    settings?.s3_credentials_set
  );

  return (
    <div>
      <h2 style={{ fontSize: "1rem", margin: "0 0 0.75rem" }}>
        {t("backups.overviewStateTitle")}
      </h2>

      <div className="backup-status-grid">
        <OverviewCard
          title={t("backups.overviewPanelSnapshot")}
          action={
            <button
              type="button"
              className="btn btn-secondary"
              onClick={onCreateSnapshot}
              disabled={busy || !settings}
            >
              {busy ? t("common.loading") : t("backups.createSnapshot")}
            </button>
          }
        >
          {settings ? (
            <>
              <OverviewRow label={t("backups.overviewLastRun")}>
                {settings.last_run_at
                  ? formatDateTime(settings.last_run_at)
                  : t("backups.neverRan")}
              </OverviewRow>
              <OverviewRow label={t("backups.overviewStatus")}>
                {settings.last_status ? (
                  <StatusBadge status={settings.last_status} t={t} />
                ) : (
                  t("backups.noData")
                )}
              </OverviewRow>
              <OverviewRow label={t("backups.overviewSchedule")}>
                {settings.enabled ? t("common.enabled") : t("common.disabled")}
                {" · "}
                {formatScheduleTime(
                  settings.hour,
                  settings.minute,
                  settings.timezone,
                )}
              </OverviewRow>
              <OverviewRow label={t("backups.overviewNextRun")}>
                {settings.enabled
                  ? panelNextRun
                    ? formatDateTime(panelNextRun)
                    : t("backups.noData")
                  : t("backups.scheduleDisabled")}
              </OverviewRow>
            </>
          ) : (
            <p style={{ margin: 0, color: "var(--muted)" }}>{t("backups.noData")}</p>
          )}
        </OverviewCard>

        <OverviewCard
          title={t("backups.overviewDatabases")}
          action={
            <button
              type="button"
              className="btn btn-secondary"
              onClick={onOpenDatabases}
            >
              {t("backups.overviewGoToDatabases")}
            </button>
          }
        >
          <OverviewRow label={t("backups.overviewLastRun")}>
            {latestDbRun?.last_run_at
              ? formatDateTime(latestDbRun.last_run_at)
              : t("backups.neverRan")}
          </OverviewRow>
          <OverviewRow label={t("backups.overviewStatus")}>
            {dbAggregateStatus ? (
              <StatusBadge status={dbAggregateStatus} t={t} />
            ) : (
              t("backups.noData")
            )}
          </OverviewRow>
          <OverviewRow label={t("backups.overviewSchedule")}>
            {dbScheduleLabel}
          </OverviewRow>
          <OverviewRow label={t("backups.overviewNextRun")}>
            {enabledSchedules.length > 0
              ? nearestDbRun
                ? formatDateTime(nearestDbRun)
                : t("backups.noData")
              : schedules.length > 0
                ? t("backups.scheduleDisabled")
                : t("backups.scheduleNotConfigured")}
          </OverviewRow>
          <OverviewRow label={t("backups.overviewDbCount")}>
            {dbCount}
          </OverviewRow>
        </OverviewCard>

        <OverviewCard
          title={t("backups.overviewStorage")}
          action={
            <button
              type="button"
              className="btn btn-secondary"
              onClick={onOpenSettings}
            >
              {t("backups.overviewGoToSettings")}
            </button>
          }
        >
          {settings ? (
            <>
              <OverviewRow label={t("backups.overviewStatus")}>
                {storageReady
                  ? t("backups.overviewStorageReady")
                  : t("backups.overviewStorageIncomplete")}
              </OverviewRow>
              <OverviewRow label={t("backups.overviewBucket")}>
                {settings.s3_bucket || t("backups.noData")}
              </OverviewRow>
              <OverviewRow label={t("backups.overviewEndpoint")}>
                <span style={{ color: "var(--muted)", fontSize: "0.8rem" }}>
                  {settings.s3_endpoint || t("backups.noData")}
                </span>
              </OverviewRow>
              <OverviewRow label={t("backups.overviewCredentials")}>
                {settings.s3_credentials_set
                  ? t("backups.overviewCredsSet")
                  : t("backups.overviewCredsNotSet")}
              </OverviewRow>
            </>
          ) : (
            <p style={{ margin: 0, color: "var(--muted)" }}>{t("backups.noData")}</p>
          )}
        </OverviewCard>
      </div>

      <div className="card" style={{ marginTop: "1.25rem" }}>
        <h3 style={{ fontSize: "1rem", margin: "0 0 0.75rem" }}>
          {t("backups.overviewRecentOps")}
        </h3>
        <div style={{ fontSize: "0.875rem" }}>
          {operations.length > 0 ? (
            <ul style={{ listStyle: "none", padding: 0, margin: 0 }}>
              {operations.slice(0, 10).map((op) => (
                <li
                  key={op.id}
                  style={{
                    marginBottom: "0.75rem",
                    paddingBottom: "0.75rem",
                    borderBottom: "1px solid var(--border)",
                  }}
                >
                  <div
                    style={{
                      display: "flex",
                      flexWrap: "wrap",
                      gap: "0.5rem",
                      alignItems: "center",
                    }}
                  >
                    <strong>{operationKindLabel(op.kind, t)}</strong>
                    {op.database_name ? (
                      <span style={{ color: "var(--muted)" }}>
                        {op.database_name}
                      </span>
                    ) : null}
                    <span style={{ color: "var(--muted)" }}>
                      {formatDateTime(op.started_at)}
                    </span>
                    <StatusBadge status={op.status} t={t} />
                    {op.size_bytes > 0 ? (
                      <span style={{ color: "var(--muted)" }}>
                        {formatBytes(op.size_bytes)}
                      </span>
                    ) : null}
                  </div>
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
    <ul style={{ listStyle: "none", padding: 0, margin: 0 }}>
      {settings?.last_run_at && (
        <li
          style={{
            marginBottom: "0.75rem",
            paddingBottom: "0.75rem",
            borderBottom: "1px solid var(--border)",
            display: "flex",
            flexWrap: "wrap",
            gap: "0.5rem",
            alignItems: "center",
          }}
        >
          <strong>{t("backups.overviewPanelSnapshot")}</strong>
          <span style={{ color: "var(--muted)" }}>
            {formatDateTime(settings.last_run_at)}
          </span>
          <StatusBadge status={settings.last_status} t={t} />
        </li>
      )}
      {recentScheduleOps.map((s) => (
        <li
          key={s.id}
          style={{
            marginBottom: "0.75rem",
            paddingBottom: "0.75rem",
            borderBottom: "1px solid var(--border)",
            display: "flex",
            flexWrap: "wrap",
            gap: "0.5rem",
            alignItems: "center",
          }}
        >
          <strong>{t("backups.overviewDbBackupOp")}</strong>
          <span style={{ color: "var(--muted)" }}>
            {scheduleDbLabel(s, dbNamesById, t)}
          </span>
          <span style={{ color: "var(--muted)" }}>
            {s.last_run_at ? formatDateTime(s.last_run_at) : t("backups.noData")}
          </span>
          <StatusBadge status={s.last_status} t={t} />
        </li>
      ))}
    </ul>
  );
}
