"use client";

import Link from "next/link";
import { useCallback, useEffect, useRef, useState } from "react";
import { api, ApiError } from "@/lib/api";
import { useI18n } from "@/lib/i18n/context";
import type { FleetInstallation, FleetInstallationLog } from "@/lib/types";

type WizardKind = "choose" | "form" | "pair" | "install";
type InstallKind = "barn" | "agent";

const TERMINAL_INSTALL = new Set(["completed", "failed", "cancelled"]);

export default function FleetNewServerPage() {
  const { t } = useI18n();
  const [step, setStep] = useState<WizardKind>("choose");
  const [kind, setKind] = useState<InstallKind>("barn");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const [name, setName] = useState("");
  const [host, setHost] = useState("");
  const [port, setPort] = useState("22");
  const [username, setUsername] = useState("root");
  const [password, setPassword] = useState("");
  const [panelUrl, setPanelUrl] = useState("");
  const [email, setEmail] = useState("");

  const [pairName, setPairName] = useState("");
  const [pairUrl, setPairUrl] = useState("");
  const [pairCode, setPairCode] = useState("");

  const [install, setInstall] = useState<FleetInstallation | null>(null);
  const [logs, setLogs] = useState<FleetInstallationLog[]>([]);
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const clearPassword = useCallback(() => setPassword(""), []);

  const stopPolling = useCallback(() => {
    if (pollRef.current) {
      clearInterval(pollRef.current);
      pollRef.current = null;
    }
  }, []);

  const pollInstall = useCallback(
    async (id: string) => {
      try {
        const [inst, logRows] = await Promise.all([
          api.getAgentInstall(id),
          api.listAgentInstallLogs(id).catch(() => [] as FleetInstallationLog[]),
        ]);
        setInstall(inst);
        setLogs(logRows);
        if (TERMINAL_INSTALL.has(inst.status)) {
          stopPolling();
        }
        if (inst.status === "completed" && inst.node_id) {
          window.location.href = `/fleet/servers/${inst.node_id}`;
        }
      } catch (e) {
        setError(e instanceof ApiError ? e.message : t("fleet.installPollFailed"));
        stopPolling();
      }
    },
    [stopPolling, t],
  );

  const startPolling = useCallback(
    (id: string) => {
      stopPolling();
      void pollInstall(id);
      pollRef.current = setInterval(() => void pollInstall(id), 3000);
    },
    [pollInstall, stopPolling],
  );

  useEffect(() => () => stopPolling(), [stopPolling]);

  const choose = (next: InstallKind) => {
    setKind(next);
    setError(null);
    setStep("form");
  };

  const submitInstall = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    const pass = password;
    try {
      const body: Parameters<typeof api.startAgentInstall>[0] = {
        kind,
        name: name.trim(),
        host: host.trim(),
        password: pass,
      };
      const portNum = parseInt(port, 10);
      if (Number.isFinite(portNum) && portNum > 0) body.port = portNum;
      if (username.trim()) body.username = username.trim();
      if (kind === "barn") {
        body.panel_url = panelUrl.trim();
        if (email.trim()) body.email = email.trim();
      }

      const inst = await api.startAgentInstall(body);
      setInstall(inst);
      setStep("install");
      startPolling(inst.id);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("fleet.installStartFailed"));
    } finally {
      clearPassword();
      setBusy(false);
    }
  };

  const submitPair = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      await api.pairBarnNode({
        name: pairName.trim(),
        base_url: pairUrl.trim(),
        pairing_code: pairCode.trim(),
      });
      window.location.href = "/fleet";
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("fleet.pairFailed"));
    } finally {
      setBusy(false);
    }
  };

  const confirmHostKey = async () => {
    if (!install) return;
    setBusy(true);
    setError(null);
    try {
      const inst = await api.confirmAgentInstallHostKey(install.id);
      setInstall(inst);
      startPolling(inst.id);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("fleet.hostKeyFailed"));
    } finally {
      setBusy(false);
    }
  };

  const cancelInstall = async () => {
    if (!install) return;
    setBusy(true);
    try {
      await api.cancelAgentInstall(install.id);
      stopPolling();
      setInstall(null);
      setStep("choose");
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("fleet.installCancelFailed"));
    } finally {
      setBusy(false);
    }
  };

  return (
    <div>
      <div className="page-header">
        <div>
          <h1>{t("fleet.newServerTitle")}</h1>
          <p className="muted" style={{ margin: "0.35rem 0 0" }}>
            {t("fleet.newServerSubtitle")}
          </p>
        </div>
        <Link href="/fleet" className="btn btn-secondary">
          {t("common.back")}
        </Link>
      </div>

      {error && <div className="alert alert-error">{error}</div>}

      {step === "choose" && (
        <div className="choice-list">
          <button
            type="button"
            className="choice-option"
            onClick={() => choose("barn")}
          >
            <span className="choice-option-title">{t("fleet.kindBarn")}</span>
            <span className="choice-option-hint">{t("fleet.kindBarnHint")}</span>
          </button>
          <button
            type="button"
            className="choice-option"
            onClick={() => {
              setError(null);
              setStep("pair");
            }}
          >
            <span className="choice-option-title">{t("fleet.kindPairExisting")}</span>
            <span className="choice-option-hint">{t("fleet.kindPairExistingHint")}</span>
          </button>
          <button
            type="button"
            className="choice-option"
            onClick={() => choose("agent")}
          >
            <span className="choice-option-title">{t("fleet.kindAgent")}</span>
            <span className="choice-option-hint">{t("fleet.kindAgentHint")}</span>
          </button>
        </div>
      )}

      {step === "pair" && (
        <form className="card" onSubmit={submitPair}>
          <h2 className="section-title">{t("fleet.kindPairExisting")}</h2>
          <p className="muted" style={{ marginTop: 0 }}>
            {t("fleet.kindPairExistingHint")}
          </p>
          <div className="field">
            <label className="label" htmlFor="pair-name">
              {t("common.name")}
            </label>
            <input
              id="pair-name"
              className="input"
              value={pairName}
              onChange={(e) => setPairName(e.target.value)}
              required
            />
          </div>
          <div className="field">
            <label className="label" htmlFor="pair-url">
              {t("fleet.panelUrl")}
            </label>
            <input
              id="pair-url"
              className="input"
              type="url"
              placeholder="https://slave.example.com"
              value={pairUrl}
              onChange={(e) => setPairUrl(e.target.value)}
              required
            />
          </div>
          <div className="field">
            <label className="label" htmlFor="pair-code">
              {t("fleet.pairingCode")}
            </label>
            <input
              id="pair-code"
              className="input"
              value={pairCode}
              onChange={(e) => setPairCode(e.target.value)}
              required
              autoComplete="off"
            />
          </div>
          <div className="form-actions">
            <button type="button" className="btn btn-secondary" onClick={() => setStep("choose")}>
              {t("common.back")}
            </button>
            <button type="submit" className="btn" disabled={busy}>
              {busy ? t("common.loading") : t("fleet.pairServer")}
            </button>
          </div>
        </form>
      )}

      {step === "form" && (
        <form className="card" onSubmit={submitInstall}>
          <h2 className="section-title">
            {kind === "barn" ? t("fleet.kindBarn") : t("fleet.kindAgent")}
          </h2>
          <p className="muted" style={{ marginTop: 0 }}>
            {kind === "barn" ? t("fleet.sshFormHintBarn") : t("fleet.sshFormHintAgent")}
          </p>

          <div className="field">
            <label className="label" htmlFor="slave-name">
              {t("common.name")}
            </label>
            <input
              id="slave-name"
              className="input"
              value={name}
              onChange={(e) => setName(e.target.value)}
              required
            />
          </div>

          <div className="form-grid">
            <div className="field">
              <label className="label" htmlFor="slave-host">
                {t("fleet.sshHost")}
              </label>
              <input
                id="slave-host"
                className="input"
                placeholder="203.0.113.10"
                value={host}
                onChange={(e) => setHost(e.target.value)}
                required
                autoComplete="off"
              />
            </div>
            <div className="field">
              <label className="label" htmlFor="slave-port">
                {t("fleet.port")}
              </label>
              <input
                id="slave-port"
                className="input"
                value={port}
                onChange={(e) => setPort(e.target.value)}
                inputMode="numeric"
              />
            </div>
            <div className="field">
              <label className="label" htmlFor="slave-user">
                {t("fleet.sshUser")}
              </label>
              <input
                id="slave-user"
                className="input"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                autoComplete="username"
              />
            </div>
            <div className="field">
              <label className="label" htmlFor="slave-pass">
                {t("fleet.sshPassword")}
              </label>
              <input
                id="slave-pass"
                className="input"
                type="password"
                autoComplete="new-password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                required
              />
            </div>
          </div>

          {kind === "barn" && (
            <>
              <div className="field">
                <label className="label" htmlFor="slave-panel-url">
                  {t("fleet.panelUrl")}
                </label>
                <input
                  id="slave-panel-url"
                  className="input"
                  type="url"
                  placeholder="https://pilot.example.com"
                  value={panelUrl}
                  onChange={(e) => setPanelUrl(e.target.value)}
                  required
                />
                <p className="muted" style={{ margin: "0.35rem 0 0", fontSize: "0.85rem" }}>
                  {t("fleet.panelUrlHint")}
                </p>
              </div>
              <div className="field">
                <label className="label" htmlFor="slave-email">
                  {t("fleet.certEmail")}
                </label>
                <input
                  id="slave-email"
                  className="input"
                  type="email"
                  placeholder="admin@example.com"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                />
                <p className="muted" style={{ margin: "0.35rem 0 0", fontSize: "0.85rem" }}>
                  {t("fleet.certEmailHint")}
                </p>
              </div>
            </>
          )}

          <div className="form-actions">
            <button
              type="button"
              className="btn btn-secondary"
              onClick={() => {
                clearPassword();
                setStep("choose");
              }}
            >
              {t("common.back")}
            </button>
            <button type="submit" className="btn" disabled={busy}>
              {busy ? t("common.loading") : t("fleet.startInstall")}
            </button>
          </div>
        </form>
      )}

      {step === "install" && install && (
        <div className="card">
          <h2 className="section-title">{t("fleet.installProgress")}</h2>
          <p>
            <strong>{install.host}</strong>
            {install.port ? `:${install.port}` : ""}
            {install.install_kind ? (
              <span className="muted"> · {install.install_kind}</span>
            ) : null}
          </p>
          {install.panel_url ? (
            <p className="muted">
              {t("fleet.panelUrl")}: {install.panel_url}
            </p>
          ) : null}
          <p className="muted">{install.current_step}</p>
          <p>
            {t("common.status")}: <code>{install.status}</code>
          </p>
          {install.status === "awaiting_host_key_confirmation" && install.ssh_fingerprint && (
            <div style={{ margin: "1rem 0" }}>
              <p>{t("fleet.hostKeyPrompt")}</p>
              <pre
                style={{
                  padding: "0.75rem",
                  background: "var(--bg)",
                  borderRadius: "var(--radius)",
                  overflow: "auto",
                }}
              >
                {install.ssh_fingerprint}
              </pre>
              <div className="form-actions">
                <button type="button" className="btn" disabled={busy} onClick={confirmHostKey}>
                  {t("fleet.confirmHostKey")}
                </button>
                <button
                  type="button"
                  className="btn btn-secondary"
                  disabled={busy}
                  onClick={cancelInstall}
                >
                  {t("common.cancel")}
                </button>
              </div>
            </div>
          )}
          {install.error_message && (
            <div className="alert alert-error">{install.error_message}</div>
          )}
          {logs.length > 0 && (
            <div style={{ marginTop: "1rem" }}>
              <h3 className="section-title">{t("fleet.installLogs")}</h3>
              <pre
                style={{
                  maxHeight: "240px",
                  overflow: "auto",
                  padding: "0.75rem",
                  background: "var(--bg)",
                  borderRadius: "var(--radius)",
                  fontSize: "0.8rem",
                }}
              >
                {logs.map((row) => `[${row.level}] ${row.message}`).join("\n")}
              </pre>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
