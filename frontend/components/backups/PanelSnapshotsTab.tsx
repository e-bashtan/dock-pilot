"use client";

import { useState, useRef } from "react";
import { formatBytes, nextDailyRun } from "./format";
import { StatusBadge } from "./StatusBadge";
import { BackupErrorDetails } from "./BackupErrorDetails";
import { ConfirmDialog } from "@/components/ConfirmDialog";
import {
  BackupJobLog,
  appendBackupLog,
  resetBackupLogs,
  type BackupJobLogLine,
} from "@/components/BackupJobLog";
import type { FullPanelBackup, PanelBackupSettings } from "@/lib/types";
import { api } from "@/lib/api";

export function PanelSnapshotsTab({
  settings,
  fullBackups,
  t,
  formatDateTime,
  onCreateSnapshot,
  onEditSchedule,
  busy,
  reload,
}: {
  settings: PanelBackupSettings | null;
  fullBackups: FullPanelBackup[];
  t: (key: string, params?: Record<string, string>) => string;
  formatDateTime: (iso: string) => string;
  onCreateSnapshot: () => void;
  onEditSchedule: () => void;
  busy: boolean;
  reload: () => Promise<void>;
}) {
  const [restoreKey, setRestoreKey] = useState<string | null>(null);
  const [restoreJob, setRestoreJob] = useState<{ session: number } | null>(null);
  const [restoreLogs, setRestoreLogs] = useState<BackupJobLogLine[]>([]);
  const [restoreStatus, setRestoreStatus] = useState("running");
  const restoreLogsRef = useRef<BackupJobLogLine[]>([]);

  const nextRun = settings?.enabled
    ? nextDailyRun(settings.hour, settings.minute, settings.timezone)
    : null;

  return (
    <div>
      <h2 style={{ fontSize: "1.1rem", margin: "0 0 0.5rem" }}>
        {t("backups.tabPanel")}
      </h2>
      <p className="muted" style={{ marginTop: 0, marginBottom: "1.25rem" }}>
        {t("backups.panelSnapshotDesc")}
      </p>

      <div
        style={{
          display: "flex",
          justifyContent: "space-between",
          gap: "1rem",
          flexWrap: "wrap",
          marginBottom: "1.25rem",
        }}
      >
        <button
          type="button"
          className="btn"
          onClick={onCreateSnapshot}
          disabled={busy}
        >
          {busy ? t("common.loading") : t("backups.createSnapshot")}
        </button>
      </div>

      {settings && (
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
                {t("backups.scheduleCard")}
              </h3>
              <div style={{ fontSize: "0.875rem" }}>
                <div style={{ marginBottom: "0.25rem" }}>
                  <strong>{t("backups.scheduleStatus")}:</strong>{" "}
                  {settings.enabled ? t("common.enabled") : t("common.disabled")}
                </div>
                <div style={{ marginBottom: "0.25rem" }}>
                  <strong>{t("backups.overviewSchedule")}:</strong>{" "}
                  {settings.enabled
                    ? `${String(settings.hour).padStart(2, "0")}:${String(settings.minute).padStart(2, "0")} ${settings.timezone}`
                    : t("backups.scheduleNotConfigured")}
                </div>
                {nextRun && (
                  <div style={{ marginBottom: "0.25rem" }}>
                    <strong>{t("backups.overviewNextRun")}:</strong>{" "}
                    {formatDateTime(nextRun)}
                  </div>
                )}
                <div style={{ marginBottom: "0.25rem" }}>
                  <strong>{t("backups.scheduleRetention")}:</strong>{" "}
                  {t("backups.retentionCount", {
                    count: String(settings.retention_count),
                  })}
                </div>
                <div style={{ marginBottom: "0.25rem", overflowWrap: "anywhere" }}>
                  <strong>{t("backups.scheduleStorage")}:</strong>{" "}
                  <span style={{ color: "var(--muted)", fontSize: "0.8rem" }}>
                    {settings.s3_bucket
                      ? `${settings.s3_bucket}/${settings.s3_prefix}`
                      : t("backups.noData")}
                  </span>
                </div>
                {settings.last_run_at ? (
                  <div style={{ marginTop: "0.5rem", fontSize: "0.8rem" }}>
                    {t("backups.scheduleLastRun")}: {formatDateTime(settings.last_run_at)} —{" "}
                    <StatusBadge status={settings.last_status} t={t} />
                    {settings.last_error ? (
                      <BackupErrorDetails error={settings.last_error} t={t} />
                    ) : null}
                  </div>
                ) : (
                  <div style={{ marginTop: "0.5rem", fontSize: "0.8rem", color: "var(--muted)" }}>
                    {t("backups.neverRan")}
                  </div>
                )}
              </div>
            </div>
            <button
              type="button"
              className="btn btn-secondary"
              onClick={onEditSchedule}
              disabled={busy}
            >
              {t("backups.edit")}
            </button>
          </div>
        </div>
      )}

      {fullBackups.length === 0 ? (
        <div className="card">
          <h3 style={{ fontSize: "1rem", marginTop: 0 }}>{t("backups.noSnapshotsTitle")}</h3>
          <p style={{ margin: "0 0 1rem", color: "var(--muted)" }}>
            {t("backups.noSnapshotsDesc")}
          </p>
          <button
            type="button"
            className="btn"
            onClick={onCreateSnapshot}
            disabled={busy}
          >
            {t("backups.createSnapshot")}
          </button>
        </div>
      ) : (
        <div className="card" style={{ padding: 0, overflow: "hidden" }}>
          <div className="table-wrap">
            <table className="table">
              <thead>
                <tr>
                  <th>{t("backups.snapshotDate")}</th>
                  <th className="col-hide-mobile">{t("backups.snapshotSize")}</th>
                  <th>{t("common.actions")}</th>
                </tr>
              </thead>
              <tbody>
                {fullBackups.map((b) => (
                  <tr key={b.s3_key}>
                    <td>
                      <div>{formatDateTime(b.created_at)}</div>
                      <div
                        className="col-hide-desktop"
                        style={{
                          fontSize: "0.75rem",
                          color: "var(--muted)",
                          marginTop: "0.25rem",
                        }}
                      >
                        {formatBytes(b.size_bytes)}
                      </div>
                    </td>
                    <td className="col-hide-mobile">{formatBytes(b.size_bytes)}</td>
                    <td>
                      <button
                        type="button"
                        className="btn btn-secondary"
                        style={{ fontSize: "0.85rem" }}
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
        </div>
      )}

      {restoreJob && (
        <BackupJobLog
          key={restoreJob.session}
          embedded
          title={t("backups.restoreLog")}
          logs={restoreLogs}
          status={restoreStatus}
        />
      )}

      <ConfirmDialog
        open={!!restoreKey}
        title={t("backups.restoreSnapshot")}
        message={t("backups.restoreSnapshotConfirm")}
        busy={busy}
        danger
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
            void reload();
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
