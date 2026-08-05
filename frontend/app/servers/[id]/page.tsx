"use client";

import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { useCallback, useEffect, useState } from "react";
import { ConfirmDialog } from "@/components/ConfirmDialog";
import { ServerNodeBadges, ServerStatusBadge } from "@/components/ServerBadges";
import { api, ApiError } from "@/lib/api";
import { formatBytes, formatPercent } from "@/lib/format";
import { isBarnPanel } from "@/lib/servers-utils";
import { useI18n } from "@/lib/i18n/context";
import type { BillingAccount, ServerNode } from "@/lib/types";

function formatUptime(seconds: number | undefined): string {
  if (!seconds || seconds <= 0) return "—";
  const d = Math.floor(seconds / 86400);
  const h = Math.floor((seconds % 86400) / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  if (d > 0) return `${d}d ${h}h`;
  if (h > 0) return `${h}h ${m}m`;
  return `${m}m`;
}

export default function ServerDetailPage() {
  const params = useParams();
  const router = useRouter();
  const id = typeof params.id === "string" ? params.id : "";
  const { t, formatDateTime } = useI18n();
  const [node, setNode] = useState<ServerNode | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [busy, setBusy] = useState(false);
  const [accounts, setAccounts] = useState<BillingAccount[]>([]);
  const [billingAccountId, setBillingAccountId] = useState("");
  const [billingCost, setBillingCost] = useState("");
  const [billingCurrency, setBillingCurrency] = useState("RUB");
  const [billingPeriod, setBillingPeriod] = useState("monthly");
  const [billingDue, setBillingDue] = useState("");
  const [billingProvider, setBillingProvider] = useState("");
  const [billingProviderUrl, setBillingProviderUrl] = useState("");
  const [billingAutoRenew, setBillingAutoRenew] = useState(false);
  const [billingAlertDays, setBillingAlertDays] = useState(10);
  const [billingMsg, setBillingMsg] = useState<string | null>(null);
  const [editName, setEditName] = useState("");
  const [nameMsg, setNameMsg] = useState<string | null>(null);

  const applyBillingForm = (row: ServerNode) => {
    setBillingAccountId(row.billing?.billing_account_id || "");
    if (row.billing) {
      setBillingCost(
        row.billing.cost_minor ? String(row.billing.cost_minor / 100) : "",
      );
      setBillingCurrency(row.billing.currency || "RUB");
      setBillingPeriod(row.billing.period || "monthly");
      setBillingDue(row.billing.next_due_date?.slice(0, 10) || "");
      setBillingProvider(row.billing.provider_name || "");
      setBillingProviderUrl(row.billing.provider_url || "");
      setBillingAutoRenew(!!row.billing.auto_renew);
      setBillingAlertDays(row.billing.alert_days && row.billing.alert_days > 0 ? row.billing.alert_days : 10);
    } else {
      setBillingCost("");
      setBillingCurrency("RUB");
      setBillingPeriod("monthly");
      setBillingDue("");
      setBillingProvider("");
      setBillingProviderUrl("");
      setBillingAutoRenew(false);
      setBillingAlertDays(10);
    }
  };

  const load = useCallback(async () => {
    if (!id) return;
    try {
      const [row, list] = await Promise.all([
        api.getServerNode(id),
        api.listBillingAccounts().catch(() => [] as BillingAccount[]),
      ]);
      setNode(row);
      setEditName(row.name);
      setAccounts(list);
      applyBillingForm(row);
      setError(null);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : t("servers.nodeLoadFailed"));
    } finally {
      setLoading(false);
    }
  }, [id, t]);

  useEffect(() => {
    void load();
    const timer = setInterval(() => void load(), 30_000);
    return () => clearInterval(timer);
  }, [load]);

  const handleDelete = async () => {
    if (!id) return;
    setBusy(true);
    try {
      await api.deleteServerNode(id);
      router.replace("/servers");
    } catch (e) {
      setError(e instanceof ApiError ? e.message : t("servers.nodeDeleteFailed"));
      setConfirmDelete(false);
    } finally {
      setBusy(false);
    }
  };

  const saveName = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!id) return;
    setBusy(true);
    setNameMsg(null);
    setError(null);
    try {
      const updated = await api.updateServerNode(id, { name: editName.trim() });
      setNode(updated);
      setEditName(updated.name);
      setNameMsg(t("servers.nameSaved"));
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("servers.nameSaveFailed"));
    } finally {
      setBusy(false);
    }
  };

  const saveBilling = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!id) return;
    setBusy(true);
    setBillingMsg(null);
    setError(null);
    try {
      const major = parseFloat(billingCost.replace(",", "."));
      const costMinor = Number.isFinite(major) ? Math.round(major * 100) : 0;
      const updated = await api.updateServerNodeBilling(id, {
        billing_account_id: billingAccountId,
        cost_minor: costMinor,
        currency: billingCurrency.trim() || "RUB",
        period: billingPeriod,
        next_due_date: billingDue.trim() || undefined,
        auto_renew: billingAutoRenew,
        alert_days: billingAlertDays,
        provider_name: billingProvider.trim() || undefined,
        provider_url: billingProviderUrl.trim() || undefined,
      });
      setNode(updated);
      applyBillingForm(updated);
      setBillingMsg(t("servers.billingSaved"));
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("servers.billingSaveFailed"));
    } finally {
      setBusy(false);
    }
  };

  const linkedAccount = Boolean(billingAccountId);

  if (loading) {
    return <p className="muted">{t("common.loading")}</p>;
  }

  if (!node) {
    return (
      <div>
        <div className="alert alert-error">{error || t("servers.nodeNotFound")}</div>
        <Link href="/servers" className="btn btn-secondary">
          {t("common.back")}
        </Link>
      </div>
    );
  }

  return (
    <div className="servers-detail-page">
      <div className="page-header">
        <div>
          <h1>{node.name}</h1>
          <div className="page-header-meta">
            <ServerStatusBadge status={node.status} />
            <ServerNodeBadges role={node.role} connectionType={node.connection_type} />
          </div>
        </div>
        <div className="page-actions">
          <Link href="/servers" className="btn btn-secondary">
            {t("common.back")}
          </Link>
          {isBarnPanel(node) && node.base_url && (
            <a
              href={node.base_url}
              className="btn btn-secondary"
              target="_blank"
              rel="noopener noreferrer"
            >
              {t("servers.openPanel")}
            </a>
          )}
          {node.connection_type !== "local" && (
            <button
              type="button"
              className="btn btn-secondary"
              onClick={() => setConfirmDelete(true)}
            >
              {t("servers.removeServer")}
            </button>
          )}
        </div>
      </div>

      {error && <div className="alert alert-error">{error}</div>}

      <form className="card" onSubmit={saveName}>
        {nameMsg && <div className="alert alert-success">{nameMsg}</div>}
        <div className="servers-rename-row">
          <div className="field">
            <label className="label" htmlFor="node-name">
              {t("servers.renameServer")}
            </label>
            <input
              id="node-name"
              className="input"
              value={editName}
              onChange={(e) => setEditName(e.target.value)}
              required
            />
            {node.hostname && node.hostname !== node.name && (
              <p className="muted" style={{ margin: "0.3rem 0 0", fontSize: "0.8rem" }}>
                {t("servers.hostnameHint", { hostname: node.hostname })}
              </p>
            )}
          </div>
          <button
            type="submit"
            className="btn"
            disabled={busy || editName.trim() === node.name}
          >
            {busy ? t("common.saving") : t("servers.saveName")}
          </button>
        </div>
      </form>

      {node.metrics && (
        <div className="card">
          <h2 className="section-title">{t("servers.metrics")}</h2>
          {node.status === "offline" && (
            <div className="alert alert-error" style={{ marginBottom: "0.75rem" }}>
              {t("servers.metricsStale")}
            </div>
          )}
          <div className="servers-detail-metrics">
            <div className="servers-detail-metric">
              <div className="servers-detail-metric-label">CPU</div>
              <div className="servers-detail-metric-value">
                {formatPercent(node.metrics.cpu_percent)}
              </div>
            </div>
            <div className="servers-detail-metric">
              <div className="servers-detail-metric-label">{t("servers.memory")}</div>
              <div
                className="servers-detail-metric-value"
                title={`${formatBytes(node.metrics.memory_used_bytes)} / ${formatBytes(node.metrics.memory_total_bytes)}`}
              >
                {formatBytes(node.metrics.memory_used_bytes)}/{formatBytes(node.metrics.memory_total_bytes)}
              </div>
            </div>
            <div className="servers-detail-metric">
              <div className="servers-detail-metric-label">{t("servers.disk")}</div>
              <div className="servers-detail-metric-value">
                {formatPercent(node.metrics.disk_used_percent)}
              </div>
            </div>
            <div className="servers-detail-metric">
              <div className="servers-detail-metric-label">{t("servers.uptime")}</div>
              <div className="servers-detail-metric-value">
                {formatUptime(node.metrics.uptime_seconds)}
              </div>
            </div>
          </div>
        </div>
      )}

      <div className="card">
        <h2 className="section-title">{t("servers.nodeInfo")}</h2>
        <dl className="servers-kv">
          <dt>{t("servers.nodeUid")}</dt>
          <dd>{node.node_uid}</dd>
          {node.hostname && (
            <>
              <dt>{t("servers.hostname")}</dt>
              <dd>{node.hostname}</dd>
            </>
          )}
          {node.version && (
            <>
              <dt>{t("servers.version")}</dt>
              <dd>{node.version}</dd>
            </>
          )}
          {node.agent_version && (
            <>
              <dt>{t("servers.agentVersion")}</dt>
              <dd>{node.agent_version}</dd>
            </>
          )}
          {node.last_seen_at && (
            <>
              <dt>{t("servers.lastSeen")}</dt>
              <dd>{formatDateTime(node.last_seen_at)}</dd>
            </>
          )}
          {(node.os_name || node.os_version) && (
            <>
              <dt>{t("servers.os")}</dt>
              <dd>{[node.os_name, node.os_version].filter(Boolean).join(" ")}</dd>
            </>
          )}
          {node.kernel && (
            <>
              <dt>{t("servers.kernel")}</dt>
              <dd>{node.kernel}</dd>
            </>
          )}
          {node.architecture && (
            <>
              <dt>{t("servers.architecture")}</dt>
              <dd>{node.architecture}</dd>
            </>
          )}
          {node.applications && (
            <>
              <dt>{t("servers.applications")}</dt>
              <dd>
                {t("servers.appsLine", {
                  running: node.applications.running,
                  total: node.applications.total,
                  unhealthy: node.applications.unhealthy,
                })}
              </dd>
            </>
          )}
        </dl>
      </div>

      <form className="card" onSubmit={saveBilling}>
        <h2 className="section-title">{t("servers.billing")}</h2>
        <p className="muted" style={{ marginTop: 0 }}>
          {t("servers.billingFormHint")}
        </p>
        {billingMsg && <div className="alert alert-success">{billingMsg}</div>}

        <div className="field">
          <label className="label" htmlFor="bill-account">
            {t("servers.billingAccount")}
          </label>
          <select
            id="bill-account"
            className="input"
            value={billingAccountId}
            onChange={(e) => setBillingAccountId(e.target.value)}
          >
            <option value="">{t("servers.billingAccountManual")}</option>
            {accounts.map((a) => (
              <option key={a.id} value={a.id}>
                {a.server_ip}
                {a.name ? ` — ${a.name}` : ""}
                {a.cost ? ` (${a.cost})` : ""}
                {a.expire_date ? ` · ${a.expire_date}` : ""}
              </option>
            ))}
          </select>
          <p className="muted" style={{ marginTop: "0.35rem", fontSize: "0.85rem" }}>
            {t("servers.billingAccountHint")}{" "}
            <Link href="/payments">{t("servers.openPayments")}</Link>
          </p>
        </div>

        <div className="servers-billing-grid">
          <div className="field">
            <label className="label" htmlFor="bill-cost">
              {t("servers.costMajor")}
            </label>
            <input
              id="bill-cost"
              className="input"
              inputMode="decimal"
              value={billingCost}
              onChange={(e) => setBillingCost(e.target.value)}
              placeholder="990"
              disabled={linkedAccount}
            />
          </div>
          <div className="field">
            <label className="label" htmlFor="bill-currency">
              {t("servers.currency")}
            </label>
            <input
              id="bill-currency"
              className="input"
              value={billingCurrency}
              onChange={(e) => setBillingCurrency(e.target.value)}
              disabled={linkedAccount}
            />
          </div>
          <div className="field">
            <label className="label" htmlFor="bill-period">
              {t("servers.billingPeriod")}
            </label>
            <select
              id="bill-period"
              className="input"
              value={billingPeriod}
              onChange={(e) => setBillingPeriod(e.target.value)}
              disabled={linkedAccount}
            >
              <option value="monthly">{t("servers.period.monthly")}</option>
              <option value="quarterly">{t("servers.period.quarterly")}</option>
              <option value="yearly">{t("servers.period.yearly")}</option>
              <option value="custom">{t("servers.period.custom")}</option>
            </select>
          </div>
          <div className="field">
            <label className="label" htmlFor="bill-due">
              {t("servers.nextDueDate")}
            </label>
            <input
              id="bill-due"
              className="input"
              type="date"
              value={billingDue}
              onChange={(e) => setBillingDue(e.target.value)}
              disabled={linkedAccount}
            />
          </div>
          <div className="field">
            <label className="label" htmlFor="bill-alert-days">
              {t("servers.alertDays")}
            </label>
            <input
              id="bill-alert-days"
              className="input"
              type="number"
              min={1}
              max={90}
              value={billingAlertDays}
              onChange={(e) => setBillingAlertDays(Number(e.target.value) || 10)}
              disabled={linkedAccount}
            />
          </div>
          <div className="field">
            <label className="label" htmlFor="bill-provider">
              {t("servers.providerName")}
            </label>
            <input
              id="bill-provider"
              className="input"
              value={billingProvider}
              onChange={(e) => setBillingProvider(e.target.value)}
              disabled={linkedAccount}
            />
          </div>
          <div className="field">
            <label className="label" htmlFor="bill-provider-url">
              {t("servers.providerUrl")}
            </label>
            <input
              id="bill-provider-url"
              className="input"
              value={billingProviderUrl}
              onChange={(e) => setBillingProviderUrl(e.target.value)}
              disabled={linkedAccount}
            />
          </div>
        </div>
        <div className="servers-billing-actions">
          <label className="checkbox-row" htmlFor="bill-auto-renew">
            <input
              id="bill-auto-renew"
              type="checkbox"
              checked={billingAutoRenew}
              onChange={(e) => setBillingAutoRenew(e.target.checked)}
              disabled={linkedAccount}
            />
            <span>{t("servers.autoRenew")}</span>
          </label>
          <button type="submit" className="btn" disabled={busy}>
            {busy ? t("common.saving") : t("servers.saveBilling")}
          </button>
        </div>
      </form>

      {node.services && node.services.length > 0 && (
        <div className="card" style={{ padding: 0, overflow: "hidden" }}>
          <h2 className="section-title" style={{ padding: "1rem 1rem 0" }}>
            {t("servers.services")}
          </h2>
          <div className="table-wrap">
            <table className="table">
              <thead>
                <tr>
                  <th>{t("servers.serviceUnit")}</th>
                  <th>{t("common.status")}</th>
                </tr>
              </thead>
              <tbody>
                {node.services.map((svc) => (
                  <tr key={svc.unit_name}>
                    <td>{svc.unit_name}</td>
                    <td>{svc.state}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {node.filesystems && node.filesystems.length > 0 && (
        <div className="card" style={{ padding: 0, overflow: "hidden" }}>
          <h2 className="section-title" style={{ padding: "1rem 1rem 0" }}>
            {t("servers.filesystems")}
          </h2>
          <div className="table-wrap">
            <table className="table">
              <thead>
                <tr>
                  <th>{t("servers.mount")}</th>
                  <th>{t("servers.device")}</th>
                  <th>{t("servers.used")}</th>
                </tr>
              </thead>
              <tbody>
                {node.filesystems.map((fs) => (
                  <tr key={fs.mountpoint}>
                    <td>{fs.mountpoint}</td>
                    <td>{fs.device}</td>
                    <td>
                      {formatPercent(fs.used_percent)} ({formatBytes(fs.used_bytes)} /{" "}
                      {formatBytes(fs.total_bytes)})
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      <ConfirmDialog
        open={confirmDelete}
        title={t("servers.removeServer")}
        message={t("servers.removeServerConfirm", { name: node.name })}
        confirmLabel={t("common.delete")}
        onConfirm={handleDelete}
        onCancel={() => setConfirmDelete(false)}
        busy={busy}
      />
    </div>
  );
}
