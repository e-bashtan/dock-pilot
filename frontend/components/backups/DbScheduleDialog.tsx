"use client";

import { useState, useEffect, useMemo } from "react";
import { ModalShell } from "./ModalShell";
import { browserTimezone, listTimezones } from "@/lib/timezone";
import type {
  PgDatabase,
  CreatePgScheduleRequest,
  PanelBackupSettings,
  PgBackupSchedule,
} from "@/lib/types";

export function DbScheduleDialog({
  open,
  databases,
  panelSettings,
  schedule,
  t,
  onClose,
  onCreate,
  onUpdate,
  busy,
}: {
  open: boolean;
  databases: PgDatabase[];
  panelSettings?: PanelBackupSettings | null;
  schedule?: PgBackupSchedule | null;
  t: (key: string) => string;
  onClose: () => void;
  onCreate: (data: CreatePgScheduleRequest) => Promise<void>;
  onUpdate?: (
    scheduleId: string,
    data: Partial<CreatePgScheduleRequest> & { clear_database_id?: boolean },
  ) => Promise<void>;
  busy: boolean;
}) {
  const editing = !!schedule;
  const [dbId, setDbId] = useState("");
  const [enabled, setEnabled] = useState(true);
  const [hour, setHour] = useState(3);
  const [minute, setMinute] = useState(0);
  const [tz, setTz] = useState(() => browserTimezone());
  const [s3Endpoint, setS3Endpoint] = useState("");
  const [s3Region, setS3Region] = useState("ru-central1");
  const [s3Bucket, setS3Bucket] = useState("");
  const [s3Prefix, setS3Prefix] = useState("barn/pg-backups");
  const [s3Access, setS3Access] = useState("");
  const [s3Secret, setS3Secret] = useState("");
  const [s3PathStyle, setS3PathStyle] = useState(false);
  const [retention, setRetention] = useState(7);
  const [usePanelS3, setUsePanelS3] = useState(true);
  const [showStorage, setShowStorage] = useState(false);

  const tzOptions = useMemo(() => listTimezones(tz), [tz]);
  const canUsePanelS3 = !!panelSettings?.s3_credentials_set;

  useEffect(() => {
    if (!open) return;

    if (schedule) {
      setDbId(schedule.database_id || "");
      setEnabled(schedule.enabled);
      setHour(schedule.hour);
      setMinute(schedule.minute);
      setTz(schedule.timezone || browserTimezone());
      setRetention(schedule.retention_count || 7);
      setUsePanelS3(schedule.use_panel_s3 && canUsePanelS3);
      setShowStorage(!schedule.use_panel_s3);
      setS3Endpoint(schedule.s3_endpoint || panelSettings?.s3_endpoint || "");
      setS3Region(schedule.s3_region || panelSettings?.s3_region || "ru-central1");
      setS3Bucket(schedule.s3_bucket || panelSettings?.s3_bucket || "");
      setS3Prefix(schedule.s3_prefix || "barn/pg-backups");
      setS3PathStyle(schedule.s3_force_path_style);
      setS3Access("");
      setS3Secret("");
      return;
    }

    setDbId(databases[0]?.id || "");
    setEnabled(true);
    setHour(3);
    setMinute(0);
    setTz(browserTimezone());
    setRetention(7);
    const preferPanel = !!panelSettings?.s3_credentials_set;
    setUsePanelS3(preferPanel);
    setShowStorage(!preferPanel);
    if (panelSettings) {
      setS3Endpoint(panelSettings.s3_endpoint || "");
      setS3Region(panelSettings.s3_region || "ru-central1");
      setS3Bucket(panelSettings.s3_bucket || "");
      setS3Prefix(
        panelSettings.s3_prefix
          ? `${panelSettings.s3_prefix}/pg`
          : "barn/pg-backups",
      );
      setS3PathStyle(panelSettings.s3_force_path_style);
    }
    setS3Access("");
    setS3Secret("");
  }, [open, databases, panelSettings, schedule, canUsePanelS3]);

  return (
    <ModalShell
      open={open}
      title={editing ? t("backups.editSchedule") : t("databases.createSchedule")}
      onClose={onClose}
      wide
    >
      <form
        onSubmit={(e) => {
          e.preventDefault();
          if (editing && schedule && onUpdate) {
            void onUpdate(schedule.id, {
              database_id: dbId || null,
              clear_database_id: !dbId,
              enabled,
              hour,
              minute,
              timezone: tz,
              use_panel_s3: usePanelS3 && canUsePanelS3,
              s3_endpoint: usePanelS3 ? undefined : s3Endpoint.trim(),
              s3_region: usePanelS3 ? undefined : s3Region.trim(),
              s3_bucket: usePanelS3 ? undefined : s3Bucket.trim(),
              s3_prefix: s3Prefix.trim() || undefined,
              s3_access_key:
                usePanelS3 || !s3Access.trim() ? undefined : s3Access.trim(),
              s3_secret_key:
                usePanelS3 || !s3Secret.trim() ? undefined : s3Secret.trim(),
              s3_force_path_style: usePanelS3 ? undefined : s3PathStyle,
              retention_count: retention,
            });
            return;
          }
          void onCreate({
            database_id: dbId || null,
            hour,
            minute,
            timezone: tz,
            use_panel_s3: usePanelS3 && canUsePanelS3,
            s3_endpoint: usePanelS3 ? undefined : s3Endpoint.trim(),
            s3_region: usePanelS3 ? undefined : s3Region.trim(),
            s3_bucket: usePanelS3 ? undefined : s3Bucket.trim(),
            s3_prefix: s3Prefix.trim() || undefined,
            s3_access_key: usePanelS3 ? undefined : s3Access.trim(),
            s3_secret_key: usePanelS3 ? undefined : s3Secret.trim(),
            s3_force_path_style: usePanelS3 ? undefined : s3PathStyle,
            retention_count: retention,
            enabled: true,
          }).then(() => {
            setS3Access("");
            setS3Secret("");
          });
        }}
      >
        {editing && (
          <div className="field" style={{ marginBottom: "1rem" }}>
            <label className="label checkbox-row">
              <input
                type="checkbox"
                checked={enabled}
                onChange={(e) => setEnabled(e.target.checked)}
              />
              <span>{t("backups.scheduleEnabled")}</span>
            </label>
          </div>
        )}

        <div className="form-grid">
          <div className="field">
            <label className="label" htmlFor="sched-db">
              {t("databases.scheduleDb")}
            </label>
            <select
              id="sched-db"
              className="select"
              value={dbId}
              onChange={(e) => setDbId(e.target.value)}
            >
              <option value="">{t("databases.allDatabases")}</option>
              {databases.map((d) => (
                <option key={d.id} value={d.id}>
                  {d.name}
                </option>
              ))}
            </select>
          </div>
          <div className="field">
            <label className="label" htmlFor="sched-hour">
              {t("databases.hour")}
            </label>
            <input
              id="sched-hour"
              className="input"
              type="number"
              min={0}
              max={23}
              value={hour}
              onChange={(e) => setHour(Number(e.target.value))}
            />
          </div>
          <div className="field">
            <label className="label" htmlFor="sched-minute">
              {t("databases.minute")}
            </label>
            <input
              id="sched-minute"
              className="input"
              type="number"
              min={0}
              max={59}
              value={minute}
              onChange={(e) => setMinute(Number(e.target.value))}
            />
          </div>
          <div className="field">
            <label className="label" htmlFor="sched-tz">
              {t("databases.timezone")}
            </label>
            <select
              id="sched-tz"
              className="select"
              value={tz}
              onChange={(e) => setTz(e.target.value)}
            >
              {tzOptions.map((tzOpt) => (
                <option key={tzOpt} value={tzOpt}>
                  {tzOpt}
                </option>
              ))}
            </select>
          </div>
          <div className="field">
            <label className="label" htmlFor="sched-retention">
              {t("databases.retention")}
            </label>
            <input
              id="sched-retention"
              className="input"
              type="number"
              min={1}
              max={365}
              value={retention}
              onChange={(e) => setRetention(Number(e.target.value))}
            />
          </div>
          <div className="field">
            <label className="label" htmlFor="sched-prefix">
              {t("databases.s3Prefix")}
            </label>
            <input
              id="sched-prefix"
              className="input"
              value={s3Prefix}
              onChange={(e) => setS3Prefix(e.target.value)}
            />
          </div>
        </div>

        {canUsePanelS3 && (
          <div className="field" style={{ marginTop: "1rem" }}>
            <label className="label checkbox-row">
              <input
                type="checkbox"
                checked={usePanelS3}
                onChange={(e) => {
                  const next = e.target.checked;
                  setUsePanelS3(next);
                  setShowStorage(!next);
                }}
              />
              <span>{t("backups.usePanelStorage")}</span>
            </label>
            <p className="muted" style={{ margin: "0.35rem 0 0", fontSize: "0.8rem" }}>
              {t("backups.usePanelStorageHint")}
            </p>
          </div>
        )}

        {!usePanelS3 && (
          <details
            open={showStorage}
            onToggle={(e) => setShowStorage(e.currentTarget.open)}
            style={{ marginTop: "1rem" }}
          >
            <summary
              style={{
                cursor: "pointer",
                fontWeight: 500,
                marginBottom: showStorage ? "1rem" : 0,
              }}
            >
              {t("backups.storageParams")}
            </summary>

            <div className="form-grid">
              <div className="field">
                <label className="label" htmlFor="sched-bucket">
                  {t("databases.s3Bucket")}
                </label>
                <input
                  id="sched-bucket"
                  className="input"
                  value={s3Bucket}
                  onChange={(e) => setS3Bucket(e.target.value)}
                  required={!editing}
                />
              </div>
              <div className="field">
                <label className="label" htmlFor="sched-endpoint">
                  {t("databases.s3Endpoint")}
                </label>
                <input
                  id="sched-endpoint"
                  className="input"
                  value={s3Endpoint}
                  onChange={(e) => setS3Endpoint(e.target.value)}
                />
              </div>
              <div className="field">
                <label className="label" htmlFor="sched-region">
                  {t("databases.s3Region")}
                </label>
                <input
                  id="sched-region"
                  className="input"
                  value={s3Region}
                  onChange={(e) => setS3Region(e.target.value)}
                />
              </div>
              <div className="field">
                <label className="label" htmlFor="sched-access">
                  {t("databases.s3AccessKey")}
                </label>
                <input
                  id="sched-access"
                  className="input"
                  value={s3Access}
                  onChange={(e) => setS3Access(e.target.value)}
                  required={!editing}
                  placeholder={editing ? t("backups.keysSet") : undefined}
                />
              </div>
              <div className="field">
                <label className="label" htmlFor="sched-secret">
                  {t("databases.s3SecretKey")}
                </label>
                <input
                  id="sched-secret"
                  className="input"
                  type="password"
                  value={s3Secret}
                  onChange={(e) => setS3Secret(e.target.value)}
                  required={!editing}
                />
              </div>
            </div>
            <div className="field" style={{ marginTop: "0.75rem" }}>
              <label className="label checkbox-row">
                <input
                  type="checkbox"
                  checked={s3PathStyle}
                  onChange={(e) => setS3PathStyle(e.target.checked)}
                />
                <span>{t("databases.s3PathStyle")}</span>
              </label>
            </div>
          </details>
        )}

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
            disabled={
              busy ||
              (!usePanelS3 &&
                !editing &&
                (!s3Access.trim() || !s3Secret.trim() || !s3Bucket.trim()))
            }
          >
            {busy ? t("common.saving") : t("common.save")}
          </button>
        </div>
      </form>
    </ModalShell>
  );
}
