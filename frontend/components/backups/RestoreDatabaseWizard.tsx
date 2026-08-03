"use client";

import { useState, useEffect, useRef } from "react";
import { ModalShell } from "./ModalShell";
import {
  BackupJobLog,
  appendBackupLog,
  resetBackupLogs,
  consumeFetchSSE,
  type BackupJobLogLine,
} from "@/components/BackupJobLog";
import type { PgBackup, PgDatabase } from "@/lib/types";
import { api } from "@/lib/api";

const CUSTOM_TARGET = "__custom__";

function RestoreTargetPicker({
  id,
  databases,
  value,
  onChange,
  disabled,
  required,
  t,
}: {
  id: string;
  databases: PgDatabase[];
  value: string;
  onChange: (name: string) => void;
  disabled?: boolean;
  required?: boolean;
  t: (key: string, params?: Record<string, string>) => string;
}) {
  const inList = databases.some((d) => d.name === value);
  const selectValue = inList ? value : CUSTOM_TARGET;

  return (
    <div className="field">
      <label className="label" htmlFor={id}>
        {t("databases.restoreTarget")}
      </label>
      <select
        id={id}
        className="select"
        value={databases.length === 0 ? CUSTOM_TARGET : selectValue}
        disabled={disabled}
        onChange={(e) => {
          const next = e.target.value;
          if (next === CUSTOM_TARGET) {
            onChange("");
            return;
          }
          onChange(next);
        }}
      >
        {databases.map((d) => (
          <option key={d.id} value={d.name}>
            {d.name}
          </option>
        ))}
        <option value={CUSTOM_TARGET}>{t("databases.restoreTargetCustom")}</option>
      </select>
      {!inList || databases.length === 0 ? (
        <input
          className="input"
          style={{ marginTop: "0.5rem" }}
          value={value}
          onChange={(e) => onChange(e.target.value)}
          placeholder={t("databases.restoreTargetCustomPlaceholder")}
          required={required}
          pattern="[A-Za-z_][A-Za-z0-9_]*"
          disabled={disabled}
        />
      ) : null}
    </div>
  );
}

export function RestoreDatabaseWizard({
  open,
  instanceId,
  backup,
  databases,
  t,
  formatDateTime,
  onClose,
  onFinished,
}: {
  open: boolean;
  instanceId: string;
  backup: PgBackup | null;
  databases: PgDatabase[];
  t: (key: string, params?: Record<string, string>) => string;
  formatDateTime: (iso: string) => string;
  onClose: () => void;
  onFinished: () => void;
}) {
  const [step, setStep] = useState<"source" | "target" | "confirm" | "running">("source");
  const [sourceType, setSourceType] = useState<"s3" | "file">("s3");
  const [uploadFile, setUploadFile] = useState<File | null>(null);
  const [target, setTarget] = useState("");
  const [createDb, setCreateDb] = useState(true);
  const [dropDb, setDropDb] = useState(false);
  const [confirmTyped, setConfirmTyped] = useState("");
  const [restoreRunning, setRestoreRunning] = useState(false);
  const [restoreLogs, setRestoreLogs] = useState<BackupJobLogLine[]>([]);
  const [restoreStatus, setRestoreStatus] = useState("running");
  const [restoreJob, setRestoreJob] = useState<{ session: number } | null>(null);
  const restoreLogsRef = useRef<BackupJobLogLine[]>([]);

  useEffect(() => {
    if (open) {
      setStep("source");
      setSourceType("s3");
      setUploadFile(null);
      setTarget(backup?.database_name || databases[0]?.name || "");
      setCreateDb(true);
      setDropDb(false);
      setConfirmTyped("");
      setRestoreRunning(false);
    }
  }, [open, backup, databases]);

  const handleRestore = () => {
    if (restoreRunning) return;
    setStep("running");
    setRestoreRunning(true);
    resetBackupLogs(restoreLogsRef, setRestoreLogs);
    setRestoreStatus("running");
    setRestoreJob((prev) => ({
      session: (prev?.session ?? 0) + 1,
    }));

    if (sourceType === "s3" && backup) {
      const es = api.streamPgBackupRestore(instanceId, {
        schedule_id: backup.schedule_id!,
        s3_key: backup.s3_key,
        target_database_name: target.trim(),
        create_database: createDb,
        drop_existing: dropDb,
      });
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
        setRestoreRunning(false);
        es.close();
        onFinished();
      });
      es.onerror = () => {
        setRestoreStatus("failed");
        setRestoreRunning(false);
        es.close();
      };
    } else if (sourceType === "file" && uploadFile) {
      void (async () => {
        try {
          const res = await api.streamPgBackupRestoreFromFile(instanceId, {
            file: uploadFile,
            target_database_name: target.trim(),
            create_database: createDb,
            drop_existing: dropDb,
          });
          const status = await consumeFetchSSE(res, (level, message, at) => {
            appendBackupLog(restoreLogsRef, setRestoreLogs, level, message, at);
          });
          setRestoreStatus(status);
        } catch (err) {
          appendBackupLog(
            restoreLogsRef,
            setRestoreLogs,
            "error",
            err instanceof Error ? err.message : "restore failed",
          );
          setRestoreStatus("failed");
        } finally {
          setRestoreRunning(false);
          onFinished();
        }
      })();
    }
  };

  const canProceedToTarget = sourceType === "s3" ? !!backup : !!uploadFile;
  const needsConfirm = dropDb && target.trim();

  return (
    <ModalShell
      open={open}
      title={t("backups.restoreWizardTitle")}
      onClose={onClose}
      wide
      closeOnBackdrop={!restoreRunning}
    >
      {step === "source" && (
        <div>
          <p style={{ marginTop: 0, fontSize: "0.875rem" }}>
            {t("backups.restoreWizardSourceHint")}
          </p>
          <div className="field" style={{ marginBottom: "1rem" }}>
            <label className="label">{t("backups.restoreWizardSource")}</label>
            <div style={{ display: "flex", gap: "1rem", marginTop: "0.5rem" }}>
              <label className="label checkbox-row">
                <input
                  type="radio"
                  name="source-type"
                  checked={sourceType === "s3"}
                  onChange={() => setSourceType("s3")}
                />
                <span>{t("backups.restoreWizardSourceS3")}</span>
              </label>
              <label className="label checkbox-row">
                <input
                  type="radio"
                  name="source-type"
                  checked={sourceType === "file"}
                  onChange={() => setSourceType("file")}
                />
                <span>{t("backups.restoreWizardSourceFile")}</span>
              </label>
            </div>
          </div>

          {sourceType === "s3" && backup && (
            <div className="card" style={{ background: "var(--bg)", padding: "1rem" }}>
              <div style={{ fontSize: "0.875rem" }}>
                <div>
                  <strong>{t("databases.dbName")}:</strong> {backup.database_name || "—"}
                </div>
                <div>
                  <strong>{t("backups.snapshotDate")}:</strong>{" "}
                  {formatDateTime(backup.created_at)}
                </div>
                <div style={{ fontSize: "0.75rem", color: "var(--muted)", marginTop: "0.5rem" }}>
                  {backup.s3_key}
                </div>
              </div>
            </div>
          )}

          {sourceType === "file" && (
            <div className="field">
              <label className="label" htmlFor="restore-file-input">
                {t("databases.restoreFile")}
              </label>
              <input
                id="restore-file-input"
                className="input"
                type="file"
                accept=".sql,.sql.gz,.gz"
                onChange={(e) => setUploadFile(e.target.files?.[0] ?? null)}
                required
              />
              <p className="muted" style={{ margin: "0.5rem 0 0", fontSize: "0.8rem" }}>
                {t("databases.restoreFromFileHint")}
              </p>
            </div>
          )}

          <div className="form-actions" style={{ marginTop: "1.5rem" }}>
            <button type="button" className="btn btn-secondary" onClick={onClose}>
              {t("common.cancel")}
            </button>
            <button
              type="button"
              className="btn"
              onClick={() => setStep("target")}
              disabled={!canProceedToTarget}
            >
              {t("common.continue")}
            </button>
          </div>
        </div>
      )}

      {step === "target" && (
        <div>
          <p style={{ marginTop: 0, fontSize: "0.875rem" }}>
            {t("backups.restoreWizardTargetHint")}
          </p>
          <div className="form-grid">
            <RestoreTargetPicker
              id="restore-target"
              databases={databases}
              value={target}
              onChange={setTarget}
              required
              t={t}
            />
          </div>

          <div className="field" style={{ marginTop: "1rem" }}>
            <label className="label checkbox-row">
              <input
                type="checkbox"
                checked={createDb}
                onChange={(e) => setCreateDb(e.target.checked)}
              />
              <span>{t("databases.createOnRestore")}</span>
            </label>
          </div>
          <div className="field">
            <label
              className="label checkbox-row"
              style={dropDb ? { color: "var(--danger)" } : undefined}
            >
              <input
                type="checkbox"
                checked={dropDb}
                onChange={(e) => setDropDb(e.target.checked)}
              />
              <span>{t("databases.dropOnRestore")}</span>
            </label>
          </div>

          {dropDb && (
            <div className="alert alert-error" style={{ marginTop: "1rem" }}>
              {t("backups.restoreWizardDropWarning")}
            </div>
          )}

          <div className="form-actions" style={{ marginTop: "1.5rem" }}>
            <button
              type="button"
              className="btn btn-secondary"
              onClick={() => setStep("source")}
            >
              {t("common.back")}
            </button>
            <button
              type="button"
              className="btn"
              onClick={() => {
                if (needsConfirm) {
                  setStep("confirm");
                } else {
                  handleRestore();
                }
              }}
              disabled={!target.trim()}
            >
              {needsConfirm ? t("common.continue") : t("backups.startRestore")}
            </button>
          </div>
        </div>
      )}

      {step === "confirm" && (
        <div>
          <p style={{ marginTop: 0, fontSize: "0.875rem", color: "var(--danger)" }}>
            {t("backups.restoreWizardConfirmHint", { name: target.trim() })}
          </p>
          <div className="field">
            <label className="label" htmlFor="restore-confirm-input">
              {t("backups.restoreWizardConfirmLabel", { name: target.trim() })}
            </label>
            <input
              id="restore-confirm-input"
              className="input"
              value={confirmTyped}
              onChange={(e) => setConfirmTyped(e.target.value)}
              autoComplete="off"
            />
          </div>

          <div className="form-actions" style={{ marginTop: "1.5rem" }}>
            <button
              type="button"
              className="btn btn-secondary"
              onClick={() => setStep("target")}
            >
              {t("common.back")}
            </button>
            <button
              type="button"
              className="btn btn-danger"
              onClick={handleRestore}
              disabled={confirmTyped !== target.trim() || restoreRunning}
            >
              {t("backups.startRestore")}
            </button>
          </div>
        </div>
      )}

      {step === "running" && restoreJob && (
        <div>
          <BackupJobLog
            key={restoreJob.session}
            embedded
            title={t("backups.restoreLog")}
            logs={restoreLogs}
            status={restoreStatus}
          />
          <div className="form-actions" style={{ marginTop: "1.5rem" }}>
            <button
              type="button"
              className="btn"
              onClick={onClose}
              disabled={restoreRunning}
            >
              {restoreRunning ? t("backups.restoreWizardRestoring") : t("common.close")}
            </button>
          </div>
        </div>
      )}
    </ModalShell>
  );
}
