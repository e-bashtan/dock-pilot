"use client";

import { useState, useEffect, useMemo } from "react";
import { ModalShell } from "./ModalShell";
import { browserTimezone, listTimezones } from "@/lib/timezone";
import type { PanelBackupSettings, UpdatePanelBackupSettings } from "@/lib/types";

export function PanelScheduleDialog({
  open,
  settings,
  t,
  onClose,
  onSave,
  busy,
}: {
  open: boolean;
  settings: PanelBackupSettings | null;
  t: (key: string) => string;
  onClose: () => void;
  onSave: (data: UpdatePanelBackupSettings) => Promise<void>;
  busy: boolean;
}) {
  const [enabled, setEnabled] = useState(false);
  const [hour, setHour] = useState(3);
  const [minute, setMinute] = useState(0);
  const [timezone, setTimezone] = useState(() => browserTimezone());
  const [retention, setRetention] = useState(7);

  const tzOptions = useMemo(() => listTimezones(timezone), [timezone]);

  useEffect(() => {
    if (settings) {
      setEnabled(settings.enabled);
      setHour(settings.hour);
      setMinute(settings.minute);
      setTimezone(settings.timezone || browserTimezone());
      setRetention(settings.retention_count || 7);
    }
  }, [settings, open]);

  return (
    <ModalShell open={open} title={t("backups.editSchedule")} onClose={onClose}>
      <form
        onSubmit={(e) => {
          e.preventDefault();
          void onSave({
            enabled,
            hour,
            minute,
            timezone,
            s3_endpoint: settings?.s3_endpoint || "",
            s3_region: settings?.s3_region || "",
            s3_bucket: settings?.s3_bucket || "",
            s3_prefix: settings?.s3_prefix || "",
            s3_force_path_style: settings?.s3_force_path_style || false,
            retention_count: retention,
          });
        }}
      >
        <div className="field">
          <label className="label checkbox-row">
            <input
              type="checkbox"
              checked={enabled}
              onChange={(e) => setEnabled(e.target.checked)}
            />
            <span>{t("backups.scheduleEnabled")}</span>
          </label>
        </div>

        <div className="form-grid" style={{ marginTop: "1rem" }}>
          <div className="field">
            <label className="label" htmlFor="panel-sched-hour">
              {t("databases.hour")}
            </label>
            <input
              id="panel-sched-hour"
              className="input"
              type="number"
              min={0}
              max={23}
              value={hour}
              onChange={(e) => setHour(Number(e.target.value))}
            />
          </div>
          <div className="field">
            <label className="label" htmlFor="panel-sched-minute">
              {t("databases.minute")}
            </label>
            <input
              id="panel-sched-minute"
              className="input"
              type="number"
              min={0}
              max={59}
              value={minute}
              onChange={(e) => setMinute(Number(e.target.value))}
            />
          </div>
          <div className="field">
            <label className="label" htmlFor="panel-sched-tz">
              {t("databases.timezone")}
            </label>
            <select
              id="panel-sched-tz"
              className="select"
              value={timezone}
              onChange={(e) => setTimezone(e.target.value)}
            >
              {tzOptions.map((tz) => (
                <option key={tz} value={tz}>
                  {tz}
                </option>
              ))}
            </select>
          </div>
          <div className="field">
            <label className="label" htmlFor="panel-retention">
              {t("databases.retention")}
            </label>
            <input
              id="panel-retention"
              className="input"
              type="number"
              min={1}
              max={365}
              value={retention}
              onChange={(e) => setRetention(Number(e.target.value))}
            />
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
          <button type="submit" className="btn" disabled={busy}>
            {busy ? t("common.saving") : t("common.save")}
          </button>
        </div>
      </form>
    </ModalShell>
  );
}
