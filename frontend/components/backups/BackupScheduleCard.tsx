"use client";

import { nextDailyRun } from "./format";
import { StatusBadge } from "./StatusBadge";
import { BackupErrorDetails } from "./BackupErrorDetails";

export function BackupScheduleCard({
  title,
  enabled,
  hour,
  minute,
  timezone,
  retentionCount,
  storageLabel,
  scopeLabel,
  lastRunAt,
  lastStatus,
  lastError,
  t,
  formatDateTime,
  onEdit,
  onDelete,
  busy,
}: {
  title?: string;
  enabled: boolean;
  hour: number;
  minute: number;
  timezone: string;
  retentionCount: number;
  storageLabel: string;
  scopeLabel?: string;
  lastRunAt?: string | null;
  lastStatus?: string;
  lastError?: string;
  t: (key: string, params?: Record<string, string>) => string;
  formatDateTime: (iso: string) => string;
  onEdit?: () => void;
  onDelete?: () => void;
  busy?: boolean;
}) {
  const nextRun = enabled ? nextDailyRun(hour, minute, timezone || "UTC") : null;

  return (
    <div className="card" style={{ marginBottom: "1.25rem" }}>
      <div
        style={{
          display: "flex",
          justifyContent: "space-between",
          alignItems: "flex-start",
          gap: "1rem",
          flexWrap: "wrap",
        }}
      >
        <div>
          <h3 style={{ fontSize: "0.95rem", marginBottom: "0.5rem" }}>
            {title || t("backups.scheduleCard")}
          </h3>
          <div style={{ fontSize: "0.875rem" }}>
            <div style={{ marginBottom: "0.25rem" }}>
              <strong>{t("backups.scheduleStatus")}:</strong>{" "}
              {enabled ? t("common.enabled") : t("common.disabled")}
            </div>
            {scopeLabel ? (
              <div style={{ marginBottom: "0.25rem" }}>
                <strong>{t("databases.scheduleDb")}:</strong> {scopeLabel}
              </div>
            ) : null}
            <div style={{ marginBottom: "0.25rem" }}>
              <strong>{t("backups.overviewSchedule")}:</strong>{" "}
              {`${String(hour).padStart(2, "0")}:${String(minute).padStart(2, "0")} ${timezone || "UTC"}`}
            </div>
            <div style={{ marginBottom: "0.25rem" }}>
              <strong>{t("backups.overviewNextRun")}:</strong>{" "}
              {enabled
                ? nextRun
                  ? formatDateTime(nextRun)
                  : t("backups.unknownDatabase")
                : t("backups.scheduleDisabled")}
            </div>
            <div style={{ marginBottom: "0.25rem" }}>
              <strong>{t("backups.scheduleRetention")}:</strong>{" "}
              {t("backups.retentionCount", { count: String(retentionCount) })}
            </div>
            <div style={{ marginBottom: "0.25rem", overflowWrap: "anywhere" }}>
              <strong>{t("backups.scheduleStorage")}:</strong>{" "}
              <span style={{ color: "var(--muted)", fontSize: "0.8rem" }}>
                {storageLabel || t("backups.noData")}
              </span>
            </div>
            {lastRunAt ? (
              <div style={{ marginTop: "0.5rem", fontSize: "0.8rem" }}>
                {t("backups.scheduleLastRun")}: {formatDateTime(lastRunAt)} —{" "}
                {lastStatus ? <StatusBadge status={lastStatus} t={t} /> : null}
                {lastError ? <BackupErrorDetails error={lastError} t={t} /> : null}
              </div>
            ) : (
              <div style={{ marginTop: "0.5rem", fontSize: "0.8rem", color: "var(--muted)" }}>
                {t("backups.neverRan")}
              </div>
            )}
          </div>
        </div>
        <div style={{ display: "flex", gap: "0.5rem", flexWrap: "wrap" }}>
          {onEdit ? (
            <button
              type="button"
              className="btn btn-secondary"
              onClick={onEdit}
              disabled={busy}
            >
              {t("backups.edit")}
            </button>
          ) : null}
          {onDelete ? (
            <button
              type="button"
              className="btn btn-secondary"
              onClick={onDelete}
              disabled={busy}
            >
              {t("databases.delete")}
            </button>
          ) : null}
        </div>
      </div>
    </div>
  );
}
