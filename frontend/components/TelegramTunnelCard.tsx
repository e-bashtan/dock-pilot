"use client";

import { useCallback, useEffect, useState } from "react";
import { api, ApiError } from "@/lib/api";
import { useI18n } from "@/lib/i18n/context";
import type { TelegramTunnelConfig, TelegramTunnelStatus } from "@/lib/types";

const defaults: TelegramTunnelConfig = { host: "", ssh_port: 22, ssh_user: "tunnel", local_port: 1080 };

export default function TelegramTunnelCard({ onProxyConfigured }: { onProxyConfigured: () => void }) {
  const { t } = useI18n();
  const [status, setStatus] = useState<TelegramTunnelStatus | null>(null);
  const [config, setConfig] = useState(defaults);
  const [busy, setBusy] = useState("");
  const [error, setError] = useState("");
  const [sshOk, setSSHOk] = useState(false);
  const [copied, setCopied] = useState(false);
  const [logs, setLogs] = useState("");

  const load = useCallback(async () => {
    try {
      const next = await api.getTelegramTunnel();
      setStatus(next);
      if (next.configured) setConfig(next.config);
      setError("");
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("notifications.tunnel.loadFailed"));
    }
  }, [t]);

  useEffect(() => { void load(); }, [load]);

  const run = async (name: string, action: () => Promise<TelegramTunnelStatus | void>) => {
    setBusy(name); setError("");
    try {
      const next = await action();
      if (next) setStatus(next);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("notifications.tunnel.actionFailed"));
    } finally { setBusy(""); }
  };

  const generate = () => run("generate", async () => {
    const next = await api.generateTelegramTunnelKey(config);
    setConfig(next.config); setSSHOk(false); return next;
  });

  const testSSH = async () => {
    setBusy("test"); setError(""); setSSHOk(false);
    try { await api.testTelegramTunnelSSH(); setSSHOk(true); }
    catch (err) { setError(err instanceof ApiError ? err.message : t("notifications.tunnel.sshFailed")); }
    finally { setBusy(""); }
  };

  const start = () => run("start", async () => {
    const next = await api.startTelegramTunnel();
    onProxyConfigured(); return next;
  });

  const showLogs = async () => {
    setBusy("logs"); setError("");
    try { setLogs((await api.getTelegramTunnelLogs()).logs); }
    catch (err) { setError(err instanceof ApiError ? err.message : t("notifications.tunnel.logsFailed")); }
    finally { setBusy(""); }
  };

  const remove = async () => {
    if (!window.confirm(t("notifications.tunnel.deleteConfirm"))) return;
    await run("delete", async () => { await api.deleteTelegramTunnel(); setStatus(null); setConfig(defaults); setLogs(""); setSSHOk(false); await load(); });
  };

  const copy = async (value: string) => {
    await navigator.clipboard.writeText(value); setCopied(true); window.setTimeout(() => setCopied(false), 1500);
  };

  const active = status?.service === "active";

  return (
    <section className="card" style={{ marginTop: "1rem" }}>
      <h2 style={{ marginTop: 0 }}>{t("notifications.tunnel.title")}</h2>
      <p style={{ color: "var(--muted)", fontSize: "0.875rem" }}>{t("notifications.tunnel.subtitle")}</p>
      {error && <div className="alert alert-error">{error}</div>}

      <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(180px, 1fr))", gap: "0.75rem" }}>
        <div className="field"><label className="label" htmlFor="tunnel-host">{t("notifications.tunnel.host")}</label><input id="tunnel-host" className="input" value={config.host} disabled={busy !== "" || active} placeholder="foreign-vps.example.com" onChange={(e) => setConfig({ ...config, host: e.target.value })} /></div>
        <div className="field"><label className="label" htmlFor="tunnel-user">{t("notifications.tunnel.user")}</label><input id="tunnel-user" className="input" value={config.ssh_user} disabled={busy !== "" || active} onChange={(e) => setConfig({ ...config, ssh_user: e.target.value })} /></div>
        <div className="field"><label className="label" htmlFor="tunnel-ssh-port">{t("notifications.tunnel.sshPort")}</label><input id="tunnel-ssh-port" className="input" type="number" min={1} max={65535} value={config.ssh_port} disabled={busy !== "" || active} onChange={(e) => setConfig({ ...config, ssh_port: Number(e.target.value) })} /></div>
        <div className="field"><label className="label" htmlFor="tunnel-local-port">{t("notifications.tunnel.localPort")}</label><input id="tunnel-local-port" className="input" type="number" min={1024} max={65535} value={config.local_port} disabled={busy !== "" || active} onChange={(e) => setConfig({ ...config, local_port: Number(e.target.value) })} /></div>
      </div>

      <div style={{ display: "flex", gap: "0.75rem", flexWrap: "wrap", marginTop: "0.75rem" }}>
        <button className="btn btn-secondary" type="button" disabled={busy !== "" || active || !config.host || !config.ssh_user} onClick={generate}>{busy === "generate" ? t("common.saving") : status?.key_created ? t("notifications.tunnel.saveConfig") : t("notifications.tunnel.generate")}</button>
        {status?.key_created && <button className="btn btn-secondary" type="button" disabled={busy !== ""} onClick={testSSH}>{busy === "test" ? t("notifications.tunnel.testing") : t("notifications.tunnel.testSSH")}</button>}
        {status?.key_created && !active && <button className="btn" type="button" disabled={busy !== "" || !sshOk} onClick={start}>{t("notifications.tunnel.start")}</button>}
        {active && <button className="btn btn-secondary" type="button" disabled={busy !== ""} onClick={() => run("restart", () => api.restartTelegramTunnel())}>{t("notifications.tunnel.restart")}</button>}
        {active && <button className="btn btn-secondary" type="button" disabled={busy !== ""} onClick={() => run("stop", () => api.stopTelegramTunnel())}>{t("notifications.tunnel.stop")}</button>}
        {status?.key_created && <button className="btn btn-secondary" type="button" disabled={busy !== ""} onClick={showLogs}>{t("notifications.tunnel.logs")}</button>}
        {status?.key_created && <button className="btn btn-danger" type="button" disabled={busy !== ""} onClick={remove}>{t("notifications.tunnel.delete")}</button>}
      </div>

      {status?.key_created && status.public_key && (
        <div style={{ marginTop: "1rem" }}>
          <h3>{t("notifications.tunnel.installKey")}</h3>
          <p style={{ color: "var(--muted)", fontSize: "0.875rem" }}>{t("notifications.tunnel.installKeyHint")}</p>
          <pre style={{ whiteSpace: "pre-wrap", overflowWrap: "anywhere" }}>{status.public_key}</pre>
          <button className="btn btn-secondary" type="button" onClick={() => copy(status.public_key || "")}>{copied ? t("notifications.tunnel.copied") : t("notifications.tunnel.copyKey")}</button>
          {status.install_hint && <details style={{ marginTop: "0.75rem" }}><summary>{t("notifications.tunnel.showCommand")}</summary><pre style={{ whiteSpace: "pre-wrap", overflowWrap: "anywhere" }}>{status.install_hint}</pre></details>}
        </div>
      )}

      {status && <div style={{ display: "flex", gap: "1rem", flexWrap: "wrap", marginTop: "1rem", fontSize: "0.875rem" }}><span>{t("notifications.tunnel.sshStatus")}: {sshOk ? "✓" : "—"}</span><span>{t("notifications.tunnel.serviceStatus")}: {status.service}</span><span>{t("notifications.tunnel.socksStatus")}: {status.socks_ready ? "✓" : "—"}</span></div>}
      {logs && <pre style={{ marginTop: "1rem", maxHeight: "22rem", overflow: "auto", whiteSpace: "pre-wrap" }}>{logs}</pre>}
    </section>
  );
}
