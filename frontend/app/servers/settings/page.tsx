"use client";

import Link from "next/link";
import { useCallback, useEffect, useState } from "react";
import { ConfirmDialog } from "@/components/ConfirmDialog";
import { api, ApiError } from "@/lib/api";
import { useServersMode } from "@/lib/servers-mode";
import { useI18n } from "@/lib/i18n/context";
import type { ServersNotificationMode, ServersPairingCode } from "@/lib/types";

export default function ServersSettingsPage() {
  const { t, formatDateTime } = useI18n();
  const { settings, refresh, isMaster } = useServersMode();
  const [nodeName, setNodeName] = useState("");
  const [publicUrl, setPublicUrl] = useState("");
  const [masterUrl, setMasterUrl] = useState("");
  const [notificationMode, setNotificationMode] =
    useState<ServersNotificationMode>("local");
  const [error, setError] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [pairing, setPairing] = useState<ServersPairingCode | null>(null);
  const [confirmDisable, setConfirmDisable] = useState(false);
  const [confirmDisconnect, setConfirmDisconnect] = useState(false);

  useEffect(() => {
    setNodeName(settings.node_name);
    setPublicUrl(
      settings.public_url ||
        (typeof window !== "undefined" ? window.location.origin : ""),
    );
    setMasterUrl(settings.master_url);
    setNotificationMode(settings.notification_mode);
  }, [settings]);

  const save = useCallback(
    async (body: Parameters<typeof api.updateServersSettings>[0]) => {
      setBusy(true);
      setError(null);
      setMessage(null);
      try {
        await api.updateServersSettings(body);
        await refresh();
        setMessage(t("servers.settingsSaved"));
      } catch (e) {
        setError(e instanceof ApiError ? e.message : t("servers.settingsSaveFailed"));
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

  const enableMaster = async (e: React.FormEvent) => {
    e.preventDefault();
    const url = publicUrl.trim() || (typeof window !== "undefined" ? window.location.origin : "");
    if (!url) {
      setError(t("servers.publicUrlRequired"));
      return;
    }
    setPublicUrl(url);
    await save({
      enable_master: true,
      node_name: nodeName.trim() || "Master",
      public_url: url,
    });
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
      await api.disconnectServersMaster();
      await refresh();
      setMessage(t("servers.masterDisconnected"));
    } catch (e) {
      setError(e instanceof ApiError ? e.message : t("servers.masterDisconnectFailed"));
    } finally {
      setBusy(false);
    }
  };

  const generatePairingCode = async () => {
    setBusy(true);
    setError(null);
    try {
      const code = await api.createServersPairingCode();
      setPairing(code);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : t("servers.pairingCodeFailed"));
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
          <h1>{t("servers.settingsTitle")}</h1>
          <p className="muted" style={{ margin: "0.35rem 0 0" }}>
            {t("servers.settingsSubtitle")}
          </p>
        </div>
        <Link href="/servers" className="btn btn-secondary">
          {t("common.back")}
        </Link>
      </div>

      {error && <div className="alert alert-error">{error}</div>}
      {message && <div className="alert alert-success">{message}</div>}

      <div className="card" style={{ marginBottom: "1rem" }}>
        <p>
          {t("servers.currentMode")}: <strong>{t(`servers.mode.${settings.mode}`)}</strong>
        </p>
        {settings.node_uid && (
          <p className="muted" style={{ fontSize: "0.85rem" }}>
            {t("servers.nodeUid")}: {settings.node_uid}
          </p>
        )}
      </div>

      {settings.mode === "standalone" && (
        <>
          <form className="card" style={{ marginBottom: "1rem" }} onSubmit={enableMaster}>
            <h2 className="section-title">{t("servers.enableMasterTitle")}</h2>
            <p className="muted">{t("servers.enableMasterHint")}</p>
            <div className="field">
              <label className="label" htmlFor="enable-master-name">
                {t("servers.masterName")}
              </label>
              <input
                id="enable-master-name"
                className="input"
                value={nodeName}
                onChange={(e) => setNodeName(e.target.value)}
                placeholder="Master"
              />
            </div>
            <div className="field">
              <label className="label" htmlFor="enable-master-url">
                {t("servers.publicUrl")}
              </label>
              <input
                id="enable-master-url"
                className="input"
                type="url"
                placeholder="https://pilot.example.com"
                value={publicUrl}
                onChange={(e) => setPublicUrl(e.target.value)}
                required
              />
              <p className="muted" style={{ fontSize: "0.8rem", marginTop: "0.35rem" }}>
                {t("servers.publicUrlHint")}
              </p>
            </div>
            <button type="submit" className="btn" disabled={busy}>
              {busy ? t("common.saving") : t("servers.enableMaster")}
            </button>
          </form>

          <div className="card">
            <h2 className="section-title">{t("servers.joinAsSlaveTitle")}</h2>
            <p className="muted">{t("servers.joinAsSlaveHint")}</p>
            <ol className="muted" style={{ margin: "0.75rem 0 1rem", paddingLeft: "1.25rem" }}>
              <li>{t("servers.joinAsSlaveStep1")}</li>
              <li>{t("servers.joinAsSlaveStep2")}</li>
              <li>{t("servers.joinAsSlaveStep3")}</li>
            </ol>
            {pairing ? (
              <div>
                <p>
                  <code style={{ fontSize: "1.25rem", letterSpacing: "0.04em" }}>
                    {pairing.code}
                  </code>
                </p>
                <p className="muted" style={{ fontSize: "0.85rem" }}>
                  {t("servers.pairingCodeExpires", {
                    time: formatDateTime(pairing.expires_at),
                  })}
                </p>
                <button
                  type="button"
                  className="btn btn-secondary"
                  disabled={busy}
                  onClick={generatePairingCode}
                  style={{ marginTop: "0.75rem" }}
                >
                  {t("servers.generatePairingCode")}
                </button>
              </div>
            ) : (
              <button
                type="button"
                className="btn btn-secondary"
                disabled={busy}
                onClick={generatePairingCode}
              >
                {t("servers.generatePairingCode")}
              </button>
            )}
          </div>
        </>
      )}

      {isMaster && (
        <form className="card" onSubmit={handleSave}>
          <h2 className="section-title">{t("servers.masterSettings")}</h2>
          <div className="field">
            <label className="label" htmlFor="master-name">
              {t("servers.masterName")}
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
              {t("servers.publicUrl")}
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
              {t("servers.publicUrlHint")}
            </p>
          </div>
          <div className="field">
            <label className="label" htmlFor="notify-mode">
              {t("servers.notificationMode")}
            </label>
            <select
              id="notify-mode"
              className="input"
              value={notificationMode}
              onChange={(e) =>
                setNotificationMode(e.target.value as ServersNotificationMode)
              }
            >
              <option value="local">{t("servers.notification.local")}</option>
              <option value="master">{t("servers.notification.master")}</option>
              <option value="disabled">{t("servers.notification.disabled")}</option>
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
              {t("servers.disableMaster")}
            </button>
          </div>
        </form>
      )}

      {settings.mode === "managed_node" && (
        <>
          <form className="card" onSubmit={saveManaged}>
            <h2 className="section-title">{t("servers.managedSettings")}</h2>
            <div className="field">
              <label className="label" htmlFor="managed-name">
                {t("servers.nodeDisplayName")}
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
                {t("servers.masterUrl")}
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
                {t("servers.notificationMode")}
              </label>
              <select
                id="managed-notify"
                className="input"
                value={notificationMode}
                onChange={(e) =>
                  setNotificationMode(e.target.value as ServersNotificationMode)
                }
              >
                <option value="local">{t("servers.notification.local")}</option>
                <option value="master">{t("servers.notification.master")}</option>
                <option value="disabled">{t("servers.notification.disabled")}</option>
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
                  {t("servers.disconnectMaster")}
                </button>
              )}
            </div>
          </form>

          <div className="card">
            <h2 className="section-title">{t("servers.pairingCodeTitle")}</h2>
            <p className="muted">{t("servers.pairingCodeHint")}</p>
            {pairing ? (
              <div style={{ marginTop: "1rem" }}>
                <p>
                  <code style={{ fontSize: "1.1rem" }}>{pairing.code}</code>
                </p>
                <p className="muted" style={{ fontSize: "0.85rem" }}>
                  {t("servers.pairingCodeExpires", {
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
                {t("servers.generatePairingCode")}
              </button>
            )}
          </div>
        </>
      )}

      <ConfirmDialog
        open={confirmDisable}
        title={t("servers.disableMaster")}
        message={t("servers.disableMasterConfirm")}
        confirmLabel={t("servers.disableMaster")}
        onConfirm={disableMaster}
        onCancel={() => setConfirmDisable(false)}
        busy={busy}
      />

      <ConfirmDialog
        open={confirmDisconnect}
        title={t("servers.disconnectMaster")}
        message={t("servers.disconnectMasterConfirm")}
        confirmLabel={t("servers.disconnectMaster")}
        onConfirm={disconnectMaster}
        onCancel={() => setConfirmDisconnect(false)}
        busy={busy}
      />
    </div>
  );
}
