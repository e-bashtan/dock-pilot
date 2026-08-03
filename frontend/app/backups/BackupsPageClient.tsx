"use client";

import { useCallback, useEffect, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import Link from "next/link";
import { useI18n } from "@/lib/i18n/context";
import { api, ApiError } from "@/lib/api";
import type {
  BackupOperation,
  FullPanelBackup,
  PanelBackupSettings,
  PgInstance,
  PgBackupSchedule,
  UpdatePanelBackupSettings,
} from "@/lib/types";
import { BackupsOverview } from "@/components/backups/BackupsOverview";
import { PanelSnapshotsTab } from "@/components/backups/PanelSnapshotsTab";
import { DatabasesBackupsTab } from "@/components/backups/DatabasesBackupsTab";
import { StorageSettingsTab } from "@/components/backups/StorageSettingsTab";
import { PanelScheduleDialog } from "@/components/backups/PanelScheduleDialog";
import { BackupErrorDetails } from "@/components/backups/BackupErrorDetails";

type Tab = "overview" | "panel" | "databases" | "settings";

function isTab(value: string | null): value is Tab {
  return (
    value === "overview" ||
    value === "panel" ||
    value === "databases" ||
    value === "settings"
  );
}

export function BackupsPageClient() {
  const { t, formatDateTime } = useI18n();
  const router = useRouter();
  const searchParams = useSearchParams();
  const tabParam = searchParams.get("tab");
  const activeTab: Tab = isTab(tabParam) ? tabParam : "overview";

  const [settings, setSettings] = useState<PanelBackupSettings | null>(null);
  const [fullBackups, setFullBackups] = useState<FullPanelBackup[]>([]);
  const [instance, setInstance] = useState<PgInstance | null>(null);
  const [schedules, setSchedules] = useState<PgBackupSchedule[]>([]);
  const [dbNamesById, setDbNamesById] = useState<Record<string, string>>({});
  const [dbCount, setDbCount] = useState(0);
  const [operations, setOperations] = useState<BackupOperation[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [saved, setSaved] = useState(false);
  const [showScheduleDialog, setShowScheduleDialog] = useState(false);

  const load = useCallback(async () => {
    try {
      const [s, full, instances, ops] = await Promise.all([
        api.getPanelBackupSettings(),
        api.listFullPanelBackups().catch(() => [] as FullPanelBackup[]),
        api.listPgInstances().catch(() => [] as PgInstance[]),
        api.listBackupOperations(20).catch(() => [] as BackupOperation[]),
      ]);
      setSettings(s);
      setFullBackups(full);
      setOperations(ops);
      const inst = instances[0] ?? null;
      setInstance(inst);
      setError(null);

      if (inst) {
        const [sched, dbs] = await Promise.all([
          api.listPgSchedules(inst.id).catch(() => []),
          api.listPgDatabases(inst.id).catch(() => []),
        ]);
        setSchedules(sched);
        setDbCount(dbs.length);
        const names: Record<string, string> = {};
        for (const db of dbs) names[db.id] = db.name;
        setDbNamesById(names);
      } else {
        setSchedules([]);
        setDbCount(0);
        setDbNamesById({});
      }
    } catch (e) {
      setError(e instanceof ApiError ? e.message : t("backups.loadFailed"));
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    void load();
  }, [load]);

  const run = async (fn: () => Promise<void>) => {
    setBusy(true);
    setError(null);
    setSaved(false);
    try {
      await fn();
      await load();
    } catch (e) {
      setError(e instanceof ApiError ? e.message : t("backups.loadFailed"));
    } finally {
      setBusy(false);
    }
  };

  const setTab = (tab: Tab) => {
    const params = new URLSearchParams(searchParams.toString());
    if (tab === "overview") {
      params.delete("tab");
    } else {
      params.set("tab", tab);
    }
    const qs = params.toString();
    router.push(qs ? `?${qs}` : "/backups");
  };

  const handleSaveSettings = async (data: UpdatePanelBackupSettings) => {
    setBusy(true);
    setError(null);
    setSaved(false);
    try {
      await api.updatePanelBackupSettings(data);
      await load();
      setSaved(true);
      setShowScheduleDialog(false);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : t("backups.loadFailed"));
    } finally {
      setBusy(false);
    }
  };

  return (
    <div>
      <div className="page-header">
        <div>
          <h1>{t("backups.title")}</h1>
          <p className="page-header-meta">{t("backups.subtitle")}</p>
        </div>
      </div>

      {error && (
        <div className="alert alert-error">
          <div>{t("backups.operationFailed")}</div>
          <BackupErrorDetails error={error} t={t} />
        </div>
      )}

      <div className="site-tabs">
        <button
          type="button"
          className={`site-tab${activeTab === "overview" ? " site-tab-active" : ""}`}
          onClick={() => setTab("overview")}
        >
          {t("backups.tabOverview")}
        </button>
        <button
          type="button"
          className={`site-tab${activeTab === "panel" ? " site-tab-active" : ""}`}
          onClick={() => setTab("panel")}
        >
          {t("backups.tabPanel")}
        </button>
        <button
          type="button"
          className={`site-tab${activeTab === "databases" ? " site-tab-active" : ""}`}
          onClick={() => setTab("databases")}
        >
          {t("backups.tabDatabases")}
        </button>
        <button
          type="button"
          className={`site-tab${activeTab === "settings" ? " site-tab-active" : ""}`}
          onClick={() => setTab("settings")}
        >
          {t("backups.tabSettings")}
        </button>
      </div>

      {loading ? (
        <div className="card">
          <p style={{ margin: 0 }}>{t("common.loading")}</p>
        </div>
      ) : (
        <>
          {activeTab === "overview" && (
            <BackupsOverview
              settings={settings}
              schedules={schedules}
              operations={operations}
              dbNamesById={dbNamesById}
              dbCount={dbCount}
              t={t}
              formatDateTime={formatDateTime}
              onCreateSnapshot={() =>
                run(async () => {
                  await api.createFullPanelBackup();
                })
              }
              onOpenDatabases={() => setTab("databases")}
              onOpenSettings={() => setTab("settings")}
              busy={busy}
            />
          )}

          {activeTab === "panel" && (
            <PanelSnapshotsTab
              settings={settings}
              fullBackups={fullBackups}
              t={t}
              formatDateTime={formatDateTime}
              onCreateSnapshot={() =>
                run(async () => {
                  await api.createFullPanelBackup();
                })
              }
              onEditSchedule={() => setShowScheduleDialog(true)}
              busy={busy}
              reload={load}
            />
          )}

          {activeTab === "databases" && (
            <div>
              {instance ? (
                <DatabasesBackupsTab
                  instanceId={instance.id}
                  panelSettings={settings}
                  t={t}
                  formatDateTime={formatDateTime}
                />

              ) : (
                <div className="card">
                  <p style={{ margin: 0 }}>{t("backups.noPostgres")}</p>
                  <Link href="/databases" className="btn" style={{ marginTop: "0.75rem" }}>
                    {t("nav.databases")}
                  </Link>
                </div>
              )}
            </div>
          )}

          {activeTab === "settings" && (
            <StorageSettingsTab
              settings={settings}
              t={t}
              onSave={handleSaveSettings}
              busy={busy}
              saved={saved}
            />
          )}
        </>
      )}

      <PanelScheduleDialog
        open={showScheduleDialog}
        settings={settings}
        t={t}
        onClose={() => setShowScheduleDialog(false)}
        onSave={handleSaveSettings}
        busy={busy}
      />
    </div>
  );
}
