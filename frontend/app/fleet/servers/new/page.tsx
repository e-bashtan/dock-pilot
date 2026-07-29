"use client";

import Link from "next/link";
import { useCallback, useEffect, useRef, useState } from "react";
import { api, ApiError } from "@/lib/api";
import { useI18n } from "@/lib/i18n/context";
import type { FleetInstallation, FleetInstallationLog } from "@/lib/types";

type WizardKind = "choose" | "dockpilot" | "agent" | "install";

const TERMINAL_INSTALL = new Set(["completed", "failed", "cancelled"]);

export default function FleetNewServerPage() {
  const { t } = useI18n();
  const [kind, setKind] = useState<WizardKind>("choose");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const [dockName, setDockName] = useState("");
  const [dockUrl, setDockUrl] = useState("");
  const [dockCode, setDockCode] = useState("");

  const [agentName, setAgentName] = useState("");
  const [agentHost, setAgentHost] = useState("");
  const [agentPort, setAgentPort] = useState("22");
  const [agentUser, setAgentUser] = useState("root");
  const [agentPassword, setAgentPassword] = useState("");
  const [agentPurpose, setAgentPurpose] = useState("");
  const [agentCost, setAgentCost] = useState("");
  const [agentCurrency, setAgentCurrency] = useState("USD");
  const [agentDue, setAgentDue] = useState("");
  const [agentAutoRenew, setAgentAutoRenew] = useState(false);
  const [agentProvider, setAgentProvider] = useState("");
  const [agentProviderUrl, setAgentProviderUrl] = useState("");

  const [install, setInstall] = useState<FleetInstallation | null>(null);
  const [logs, setLogs] = useState<FleetInstallationLog[]>([]);
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const clearPassword = useCallback(() => setAgentPassword(""), []);

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

  const submitDockpilot = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      await api.pairDockpilotNode({
        name: dockName.trim(),
        base_url: dockUrl.trim(),
        pairing_code: dockCode.trim(),
      });
      window.location.href = "/fleet";
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("fleet.pairFailed"));
    } finally {
      setBusy(false);
    }
  };

  const submitAgent = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    const password = agentPassword;
    try {
      const body: Parameters<typeof api.startAgentInstall>[0] = {
        name: agentName.trim(),
        host: agentHost.trim(),
        password,
      };
      const port = parseInt(agentPort, 10);
      if (Number.isFinite(port) && port > 0) body.port = port;
      if (agentUser.trim()) body.username = agentUser.trim();
      if (agentPurpose.trim()) body.purpose = agentPurpose.trim();
      if (agentCost.trim()) {
        const minor = Math.round(parseFloat(agentCost) * 100);
        if (Number.isFinite(minor)) body.cost_minor = minor;
      }
      if (agentCurrency.trim()) body.currency = agentCurrency.trim();
      if (agentDue.trim()) body.next_due_date = agentDue.trim();
      body.auto_renew = agentAutoRenew;
      if (agentProvider.trim()) body.provider_name = agentProvider.trim();
      if (agentProviderUrl.trim()) body.provider_url = agentProviderUrl.trim();

      const inst = await api.startAgentInstall(body);
      setInstall(inst);
      setKind("install");
      startPolling(inst.id);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("fleet.installStartFailed"));
    } finally {
      clearPassword();
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
      setKind("choose");
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

      {kind === "choose" && (
        <div className="grid-2">
          <button
            type="button"
            className="card"
            style={{ textAlign: "left", cursor: "pointer" }}
            onClick={() => setKind("dockpilot")}
          >
            <h2 className="section-title">{t("fleet.kindDockpilot")}</h2>
            <p className="muted">{t("fleet.kindDockpilotHint")}</p>
          </button>
          <button
            type="button"
            className="card"
            style={{ textAlign: "left", cursor: "pointer" }}
            onClick={() => setKind("agent")}
          >
            <h2 className="section-title">{t("fleet.kindAgent")}</h2>
            <p className="muted">{t("fleet.kindAgentHint")}</p>
          </button>
        </div>
      )}

      {kind === "dockpilot" && (
        <form className="card" onSubmit={submitDockpilot}>
          <h2 className="section-title">{t("fleet.kindDockpilot")}</h2>
          <div className="field">
            <label className="label" htmlFor="dock-name">
              {t("common.name")}
            </label>
            <input
              id="dock-name"
              className="input"
              value={dockName}
              onChange={(e) => setDockName(e.target.value)}
              required
            />
          </div>
          <div className="field">
            <label className="label" htmlFor="dock-url">
              {t("fleet.baseUrl")}
            </label>
            <input
              id="dock-url"
              className="input"
              type="url"
              placeholder="https://pilot.example.com"
              value={dockUrl}
              onChange={(e) => setDockUrl(e.target.value)}
              required
            />
          </div>
          <div className="field">
            <label className="label" htmlFor="dock-code">
              {t("fleet.pairingCode")}
            </label>
            <input
              id="dock-code"
              className="input"
              value={dockCode}
              onChange={(e) => setDockCode(e.target.value)}
              required
            />
          </div>
          <div className="form-actions">
            <button type="button" className="btn btn-secondary" onClick={() => setKind("choose")}>
              {t("common.back")}
            </button>
            <button type="submit" className="btn" disabled={busy}>
              {busy ? t("common.loading") : t("fleet.pairServer")}
            </button>
          </div>
        </form>
      )}

      {kind === "agent" && (
        <form className="card" onSubmit={submitAgent}>
          <h2 className="section-title">{t("fleet.kindAgent")}</h2>
          <div className="form-grid">
            <div className="field">
              <label className="label" htmlFor="agent-name">
                {t("common.name")}
              </label>
              <input
                id="agent-name"
                className="input"
                value={agentName}
                onChange={(e) => setAgentName(e.target.value)}
                required
              />
            </div>
            <div className="field">
              <label className="label" htmlFor="agent-host">
                {t("fleet.host")}
              </label>
              <input
                id="agent-host"
                className="input"
                value={agentHost}
                onChange={(e) => setAgentHost(e.target.value)}
                required
              />
            </div>
            <div className="field">
              <label className="label" htmlFor="agent-port">
                {t("fleet.port")}
              </label>
              <input
                id="agent-port"
                className="input"
                value={agentPort}
                onChange={(e) => setAgentPort(e.target.value)}
              />
            </div>
            <div className="field">
              <label className="label" htmlFor="agent-user">
                {t("fleet.sshUser")}
              </label>
              <input
                id="agent-user"
                className="input"
                value={agentUser}
                onChange={(e) => setAgentUser(e.target.value)}
              />
            </div>
          </div>
          <div className="field">
            <label className="label" htmlFor="agent-pass">
              {t("fleet.sshPassword")}
            </label>
            <input
              id="agent-pass"
              className="input"
              type="password"
              autoComplete="new-password"
              value={agentPassword}
              onChange={(e) => setAgentPassword(e.target.value)}
              required
            />
          </div>
          <details style={{ marginBottom: "1rem" }}>
            <summary className="muted">{t("fleet.billingOptional")}</summary>
            <div className="form-grid" style={{ marginTop: "1rem" }}>
              <div className="field">
                <label className="label" htmlFor="agent-purpose">
                  {t("fleet.purpose")}
                </label>
                <input
                  id="agent-purpose"
                  className="input"
                  value={agentPurpose}
                  onChange={(e) => setAgentPurpose(e.target.value)}
                />
              </div>
              <div className="field">
                <label className="label" htmlFor="agent-cost">
                  {t("fleet.costMajor")}
                </label>
                <input
                  id="agent-cost"
                  className="input"
                  inputMode="decimal"
                  value={agentCost}
                  onChange={(e) => setAgentCost(e.target.value)}
                />
              </div>
              <div className="field">
                <label className="label" htmlFor="agent-currency">
                  {t("fleet.currency")}
                </label>
                <input
                  id="agent-currency"
                  className="input"
                  value={agentCurrency}
                  onChange={(e) => setAgentCurrency(e.target.value)}
                />
              </div>
              <div className="field">
                <label className="label" htmlFor="agent-due">
                  {t("fleet.nextDueDate")}
                </label>
                <input
                  id="agent-due"
                  className="input"
                  type="date"
                  value={agentDue}
                  onChange={(e) => setAgentDue(e.target.value)}
                />
              </div>
              <div className="field">
                <label className="label" htmlFor="agent-provider">
                  {t("fleet.providerName")}
                </label>
                <input
                  id="agent-provider"
                  className="input"
                  value={agentProvider}
                  onChange={(e) => setAgentProvider(e.target.value)}
                />
              </div>
              <div className="field">
                <label className="label" htmlFor="agent-provider-url">
                  {t("fleet.providerUrl")}
                </label>
                <input
                  id="agent-provider-url"
                  className="input"
                  value={agentProviderUrl}
                  onChange={(e) => setAgentProviderUrl(e.target.value)}
                />
              </div>
            </div>
            <label style={{ display: "flex", gap: "0.5rem", alignItems: "center" }}>
              <input
                type="checkbox"
                checked={agentAutoRenew}
                onChange={(e) => setAgentAutoRenew(e.target.checked)}
              />
              {t("fleet.autoRenew")}
            </label>
          </details>
          <div className="form-actions">
            <button type="button" className="btn btn-secondary" onClick={() => setKind("choose")}>
              {t("common.back")}
            </button>
            <button type="submit" className="btn" disabled={busy}>
              {busy ? t("common.loading") : t("fleet.startInstall")}
            </button>
          </div>
        </form>
      )}

      {kind === "install" && install && (
        <div className="card">
          <h2 className="section-title">{t("fleet.installProgress")}</h2>
          <p>
            <strong>{install.host}</strong>
            {install.port ? `:${install.port}` : ""}
          </p>
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
                <button type="button" className="btn btn-secondary" disabled={busy} onClick={cancelInstall}>
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
