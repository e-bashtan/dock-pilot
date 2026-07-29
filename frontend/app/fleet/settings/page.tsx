"use client";

import Link from "next/link";
import { useCallback, useEffect, useState } from "react";
import { ConfirmDialog } from "@/components/ConfirmDialog";
import { api, ApiError } from "@/lib/api";
import { useFleetMode } from "@/lib/fleet-mode";
import { useI18n } from "@/lib/i18n/context";
import type { FleetNotificationMode, FleetPairingCode } from "@/lib/types";

export default function FleetSettingsPage() {
  const { t, formatDateTime } = useI18n();
  const { settings, refresh, isMaster } = useFleetMode();
  const [nodeName, setNodeName] = useState("");
  const [publicUrl, setPublicUrl] = useState("");
  const [masterUrl, setMasterUrl] = useState("");
  const [notificationMode, setNotificationMode] =
    useState<FleetNotificationMode>("local");
  const [error, setError] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [pairing, setPairing] = useState<FleetPairingCode | null>(null);
  const [confirmDisable, setConfirmDisable] = useState(false);
  const [confirmDisconnect, setConfirmDisconnect] = useState(false);

  useEffect(() => {
    setNodeName(settings.node_name);
    setPublicUrl(settings.public_url);
    setMasterUrl(settings.master_url);
    setNotificationMode(settings.notification_mode);
  }, [settings]);

  const save = useCallback(
    async (body: Parameters<typeof api.updateFleetSettings>[0]) => {
      setBusy(true);
      setError(null);
      setMessage(null);
      try {
        await api.updateFleetSettings(body);
        await refresh();
        setMessage(t("fleet.settingsSaved"));
      } catch (e) {
        setError(e instanceof ApiError ? e.message : t("fleet.settingsSaveFailed"));
      } finally {
        setBusy(false);
      }
    },
    [refresh, t],
  );

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault();
    await save({
      node_name: nodeName.trim(),
      public_url: publicUrl.trim(),
      notification_mode: notificationMode,
    });
  };

  const enableMaster = async () => {
    await save({ enable_master: true });
  };

  const disableMaster = async () => {
    setConfirmDisable(false);
    await save({ disable_master: true });
  };

  const disconnectMaster = async () => {
    setConfirmDisconnect(false);
    setBusy(true);
    setError(null);
    try {
      await api.disconnectFleetMaster();
      await refresh();
      setMessage(t("fleet.masterDisconnected"));
    } catch (e) {
      setError(e instanceof ApiError ? e.message : t("fleet.masterDisconnectFailed"));
    } finally {
      setBusy(false);
    }
  };

  const generatePairingCode = async () => {
    setBusy(true);
    setError(null);
    try {
      const code = await api.createFleetPairingCode();
      setPairing(code);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : t("fleet.pairingCodeFailed"));
    } finally {
      setBusy(false);
    }
  };

  const saveManaged = async (e: React.FormEvent) => {
    e.preventDefault();
    await save({
      node_name: nodeName.trim(),
      master_url: masterUrl.trim(),
      notification_mode: notificationMode,
    });
  };

  return (
    <div>
      <div className="page-header">
        <div>
          <h1>{t("fleet.settingsTitle")}</h1>
          <p className="muted" style={{ margin: "0.35rem 0 0" }}>
            {t("fleet.settingsSubtitle")}
          </p>
        </div>
        <Link href="/fleet" className="btn btn-secondary">
          {t("common.back")}
        </Link>
      </div>

      {error && <div className="alert alert-error">{error}</div>}
      {message && <div className="alert alert-success">{message}</div>}

      <div className="card" style={{ marginBottom: "1rem" }}>
        <p>
          {t("fleet.currentMode")}: <strong>{t(`fleet.mode.${settings.mode}`)}</strong>
        </p>
        {settings.node_uid && (
          <p className="muted" style={{ fontSize: "0.85rem" }}>
            {t("fleet.nodeUid")}: {settings.node_uid}
          </p>
        )}
      </div>

      {settings.mode === "standalone" && (
        <div className="card">
          <h2 className="section-title">{t("fleet.enableMasterTitle")}</h2>
          <p className="muted">{t("fleet.enableMasterHint")}</p>
          <button type="button" className="btn" disabled={busy} onClick={enableMaster}>
            {t("fleet.enableMaster")}
          </button>
        </div>
      )}

      {isMaster && (
        <form className="card" onSubmit={handleSave}>
          <h2 className="section-title">{t("fleet.masterSettings")}</h2>
          <div className="field">
            <label className="label" htmlFor="master-name">
              {t("fleet.masterName")}
            </label>
            <input
              id="master-name"
              className="input"
              value={nodeName}
              onChange={(e) => setNodeName(e.target.value)}
            />
          </div>
          <div className="field">
            <label className="label" htmlFor="master-url">
              {t("fleet.publicUrl")}
            </label>
            <input
              id="master-url"
              className="input"
              type="url"
              placeholder="https://pilot.example.com"
              value={publicUrl}
              onChange={(e) => setPublicUrl(e.target.value)}
            />
            <p className="muted" style={{ fontSize: "0.8rem", marginTop: "0.35rem" }}>
              {t("fleet.publicUrlHint")}
            </p>
          </div>
          <div className="field">
            <label className="label" htmlFor="notify-mode">
              {t("fleet.notificationMode")}
            </label>
            <select
              id="notify-mode"
              className="input"
              value={notificationMode}
              onChange={(e) =>
                setNotificationMode(e.target.value as FleetNotificationMode)
              }
            >
              <option value="local">{t("fleet.notification.local")}</option>
              <option value="master">{t("fleet.notification.master")}</option>
              <option value="disabled">{t("fleet.notification.disabled")}</option>
            </select>
          </div>
          <div className="form-actions">
            <button type="submit" className="btn" disabled={busy}>
              {busy ? t("common.saving") : t("common.save")}
            </button>
            <button
              type="button"
              className="btn btn-secondary"
              disabled={busy}
              onClick={() => setConfirmDisable(true)}
            >
              {t("fleet.disableMaster")}
            </button>
          </div>
        </form>
      )}

      {settings.mode === "managed_node" && (
        <>
          <form className="card" onSubmit={saveManaged}>
            <h2 className="section-title">{t("fleet.managedSettings")}</h2>
            <div className="field">
              <label className="label" htmlFor="managed-name">
                {t("fleet.nodeDisplayName")}
              </label>
              <input
                id="managed-name"
                className="input"
                value={nodeName}
                onChange={(e) => setNodeName(e.target.value)}
              />
            </div>
            <div className="field">
              <label className="label" htmlFor="managed-master">
                {t("fleet.masterUrl")}
              </label>
              <input
                id="managed-master"
                className="input"
                type="url"
                value={masterUrl}
                onChange={(e) => setMasterUrl(e.target.value)}
              />
            </div>
            <div className="field">
              <label className="label" htmlFor="managed-notify">
                {t("fleet.notificationMode")}
              </label>
              <select
                id="managed-notify"
                className="input"
                value={notificationMode}
                onChange={(e) =>
                  setNotificationMode(e.target.value as FleetNotificationMode)
                }
              >
                <option value="local">{t("fleet.notification.local")}</option>
                <option value="master">{t("fleet.notification.master")}</option>
                <option value="disabled">{t("fleet.notification.disabled")}</option>
              </select>
            </div>
            <div className="form-actions">
              <button type="submit" className="btn" disabled={busy}>
                {busy ? t("common.saving") : t("common.save")}
              </button>
              {settings.has_master_token && (
                <button
                  type="button"
                  className="btn btn-secondary"
                  disabled={busy}
                  onClick={() => setConfirmDisconnect(true)}
                >
                  {t("fleet.disconnectMaster")}
                </button>
              )}
            </div>
          </form>

          <div className="card">
            <h2 className="section-title">{t("fleet.pairingCodeTitle")}</h2>
            <p className="muted">{t("fleet.pairingCodeHint")}</p>
            {pairing ? (
              <div style={{ marginTop: "1rem" }}>
                <p>
                  <code style={{ fontSize: "1.1rem" }}>{pairing.code}</code>
                </p>
                <p className="muted" style={{ fontSize: "0.85rem" }}>
                  {t("fleet.pairingCodeExpires", {
                    time: formatDateTime(pairing.expires_at),
                  })}
                </p>
              </div>
            ) : (
              <button
                type="button"
                className="btn btn-secondary"
                disabled={busy}
                onClick={generatePairingCode}
                style={{ marginTop: "0.75rem" }}
              >
                {t("fleet.generatePairingCode")}
              </button>
            )}
          </div>
        </>
      )}

      <ConfirmDialog
        open={confirmDisable}
        title={t("fleet.disableMaster")}
        message={t("fleet.disableMasterConfirm")}
        confirmLabel={t("fleet.disableMaster")}
        onConfirm={disableMaster}
        onCancel={() => setConfirmDisable(false)}
        busy={busy}
      />

      <ConfirmDialog
        open={confirmDisconnect}
        title={t("fleet.disconnectMaster")}
        message={t("fleet.disconnectMasterConfirm")}
        confirmLabel={t("fleet.disconnectMaster")}
        onConfirm={disconnectMaster}
        onCancel={() => setConfirmDisconnect(false)}
        busy={busy}
      />
    </div>
  );
}
