"use client";

import { useState, useEffect } from "react";
import { ModalShell } from "./ModalShell";
import type { PgDatabase, PgBackupSchedule } from "@/lib/types";

export function CreateDbBackupDialog({
  open,
  databases,
  schedules,
  t,
  onClose,
  onCreate,
  busy,
}: {
  open: boolean;
  databases: PgDatabase[];
  schedules: PgBackupSchedule[];
  t: (key: string) => string;
  onClose: () => void;
  onCreate: (dbId: string, scheduleId: string) => Promise<void>;
  busy: boolean;
}) {
  const [dbId, setDbId] = useState("");
  const [scheduleId, setScheduleId] = useState("");

  useEffect(() => {
    if (open) {
      setDbId(databases[0]?.id || "");
      setScheduleId(schedules[0]?.id || "");
    }
  }, [open, databases, schedules]);

  return (
    <ModalShell open={open} title={t("backups.createBackup")} onClose={onClose}>
      <form
        onSubmit={(e) => {
          e.preventDefault();
          void onCreate(dbId, scheduleId);
        }}
      >
        <div className="form-grid">
          <div className="field">
            <label className="label" htmlFor="backup-db">
              {t("databases.dbName")}
            </label>
            <select
              id="backup-db"
              className="select"
              value={dbId}
              onChange={(e) => setDbId(e.target.value)}
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
            <label className="label" htmlFor="backup-schedule">
              {t("databases.useSchedule")}
            </label>
            <select
              id="backup-schedule"
              className="select"
              value={scheduleId}
              onChange={(e) => setScheduleId(e.target.value)}
              required
            >
              {schedules.map((s) => (
                <option key={s.id} value={s.id}>
                  {s.s3_bucket}/{s.s3_prefix}
                </option>
              ))}
            </select>
          </div>
        </div>

        <div className="form-actions">
          <button
            type="button"
            className="btn btn-secondary"
            onClick={onClose}
            disabled={busy}
          >
            {t("common.cancel")}
          </button>
          <button
            type="submit"
            className="btn"
            disabled={busy || !databases.length || !schedules.length}
          >
            {busy ? t("common.loading") : t("backups.createBackup")}
          </button>
        </div>
      </form>
    </ModalShell>
  );
}
