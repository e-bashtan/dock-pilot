"use client";

import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { useCallback, useEffect, useState } from "react";
import { ConfirmDialog } from "@/components/ConfirmDialog";
import { FleetNodeBadges, FleetStatusBadge } from "@/components/FleetBadges";
import { api, ApiError } from "@/lib/api";
import { formatBytes, formatMoneyMinor, formatPercent } from "@/lib/format";
import { useI18n } from "@/lib/i18n/context";
import type { BillingAccount, FleetNode } from "@/lib/types";

function formatUptime(seconds: number | undefined): string {
  if (!seconds || seconds <= 0) return "—";
  const d = Math.floor(seconds / 86400);
  const h = Math.floor((seconds % 86400) / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  if (d > 0) return `${d}d ${h}h`;
  if (h > 0) return `${h}h ${m}m`;
  return `${m}m`;
}

export default function FleetServerDetailPage() {
  const params = useParams();
  const router = useRouter();
  const id = typeof params.id === "string" ? params.id : "";
  const { t, formatDateTime } = useI18n();
  const [node, setNode] = useState<FleetNode | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [busy, setBusy] = useState(false);
  const [accounts, setAccounts] = useState<BillingAccount[]>([]);
  const [billingAccountId, setBillingAccountId] = useState("");
  const [billingMsg, setBillingMsg] = useState<string | null>(null);
  const [editName, setEditName] = useState("");
  const [nameMsg, setNameMsg] = useState<string | null>(null);

  const load = useCallback(async () => {
    if (!id) return;
    try {
      const [row, list] = await Promise.all([
        api.getFleetNode(id),
        api.listBillingAccounts().catch(() => [] as BillingAccount[]),
      ]);
      setNode(row);
      setEditName(row.name);
      setAccounts(list);
      setBillingAccountId(row.billing?.billing_account_id || "");
      setError(null);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : t("fleet.nodeLoadFailed"));
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
      await api.deleteFleetNode(id);
      router.replace("/fleet");
    } catch (e) {
      setError(e instanceof ApiError ? e.message : t("fleet.nodeDeleteFailed"));
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
      const updated = await api.updateFleetNode(id, { name: editName.trim() });
      setNode(updated);
      setEditName(updated.name);
      setNameMsg(t("fleet.nameSaved"));
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("fleet.nameSaveFailed"));
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
      const updated = await api.updateFleetNodeBilling(id, {
        billing_account_id: billingAccountId,
      });
      setNode(updated);
      setBillingAccountId(updated.billing?.billing_account_id || "");
      setBillingMsg(t("fleet.billingSaved"));
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("fleet.billingSaveFailed"));
    } finally {
      setBusy(false);
    }
  };

  if (loading) {
    return <p className="muted">{t("common.loading")}</p>;
  }

  if (!node) {
    return (
      <div>
        <div className="alert alert-error">{error || t("fleet.nodeNotFound")}</div>
        <Link href="/fleet" className="btn btn-secondary">
          {t("common.back")}
        </Link>
      </div>
    );
  }

  return (
    <div>
      <div className="page-header">
        <div>
          <h1>{node.name}</h1>
          <div className="page-header-meta">
            <FleetStatusBadge status={node.status} />
            <FleetNodeBadges role={node.role} connectionType={node.connection_type} />
          </div>
        </div>
        <div style={{ display: "flex", gap: "0.5rem", flexWrap: "wrap" }}>
          <Link href="/fleet" className="btn btn-secondary">
            {t("common.back")}
          </Link>
          {node.connection_type === "dockpilot" && node.base_url && (
            <a
              href={node.base_url}
              className="btn btn-secondary"
              target="_blank"
              rel="noopener noreferrer"
            >
              {t("fleet.openPanel")}
            </a>
          )}
          {node.connection_type !== "local" && (
            <button
              type="button"
              className="btn btn-secondary"
              onClick={() => setConfirmDelete(true)}
            >
              {t("fleet.removeServer")}
            </button>
          )}
        </div>
      </div>

      {error && <div className="alert alert-error">{error}</div>}

      <form className="card" onSubmit={saveName}>
        <h2 className="section-title">{t("fleet.renameServer")}</h2>
        {nameMsg && <div className="alert alert-success">{nameMsg}</div>}
        <div className="field">
          <label className="label" htmlFor="node-name">
            {t("common.name")}
          </label>
          <input
            id="node-name"
            className="input"
            value={editName}
            onChange={(e) => setEditName(e.target.value)}
            required
          />
          {node.hostname && node.hostname !== node.name && (
            <p className="muted" style={{ margin: "0.35rem 0 0", fontSize: "0.85rem" }}>
              {t("fleet.hostnameHint", { hostname: node.hostname })}
            </p>
          )}
        </div>
        <button
          type="submit"
          className="btn"
          disabled={busy || editName.trim() === node.name}
          style={{ marginTop: "0.75rem" }}
        >
          {busy ? t("common.saving") : t("fleet.saveName")}
        </button>
      </form>

      <div className="grid-2">
        <div className="card">
          <h2 className="section-title">{t("fleet.nodeInfo")}</h2>
          <dl style={{ margin: 0, fontSize: "0.9rem" }}>
            <dt className="muted">{t("fleet.nodeUid")}</dt>
            <dd>{node.node_uid}</dd>
            {node.hostname && (
              <>
                <dt className="muted">{t("fleet.hostname")}</dt>
                <dd>{node.hostname}</dd>
              </>
            )}
            {node.version && (
              <>
                <dt className="muted">{t("fleet.version")}</dt>
                <dd>{node.version}</dd>
              </>
            )}
            {node.agent_version && (
              <>
                <dt className="muted">{t("fleet.agentVersion")}</dt>
                <dd>{node.agent_version}</dd>
              </>
            )}
            {node.last_seen_at && (
              <>
                <dt className="muted">{t("fleet.lastSeen")}</dt>
                <dd>{formatDateTime(node.last_seen_at)}</dd>
              </>
            )}
            {(node.os_name || node.os_version) && (
              <>
                <dt className="muted">{t("fleet.os")}</dt>
                <dd>
                  {[node.os_name, node.os_version].filter(Boolean).join(" ")}
                </dd>
              </>
            )}
            {node.kernel && (
              <>
                <dt className="muted">{t("fleet.kernel")}</dt>
                <dd>{node.kernel}</dd>
              </>
            )}
            {node.architecture && (
              <>
                <dt className="muted">{t("fleet.architecture")}</dt>
                <dd>{node.architecture}</dd>
              </>
            )}
          </dl>
        </div>

        {node.metrics && (
          <div className="card">
            <h2 className="section-title">{t("fleet.metrics")}</h2>
            <div className="server-status-grid">
              <div>
                <div className="label">CPU</div>
                <div>{formatPercent(node.metrics.cpu_percent)}</div>
              </div>
              <div>
                <div className="label">{t("fleet.memory")}</div>
                <div>
                  {formatBytes(node.metrics.memory_used_bytes)} /{" "}
                  {formatBytes(node.metrics.memory_total_bytes)}
                </div>
              </div>
              <div>
                <div className="label">{t("fleet.disk")}</div>
                <div>{formatPercent(node.metrics.disk_used_percent)}</div>
              </div>
            </div>
            <p className="muted" style={{ marginTop: "0.75rem", fontSize: "0.85rem" }}>
              {t("fleet.uptime")}: {formatUptime(node.metrics.uptime_seconds)}
            </p>
          </div>
        )}
      </div>

      {node.applications && (
        <div className="card">
          <h2 className="section-title">{t("fleet.applications")}</h2>
          <p>
            {t("fleet.appsLine", {
              running: node.applications.running,
              total: node.applications.total,
              unhealthy: node.applications.unhealthy,
            })}
          </p>
        </div>
      )}

      <form className="card" onSubmit={saveBilling}>
        <h2 className="section-title">{t("fleet.billing")}</h2>
        <p className="muted" style={{ marginTop: 0 }}>
          {t("fleet.billingFormHint")}{" "}
          <Link href="/payments">{t("fleet.openPayments")}</Link>
        </p>
        {billingMsg && <div className="alert alert-success">{billingMsg}</div>}

        {node.billing && (node.billing.cost_minor > 0 || node.billing.next_due_date) && (
          <div className="fleet-node-stats" style={{ marginBottom: "1rem" }}>
            {(node.billing.monthly_equiv_minor ?? node.billing.cost_minor) > 0 && (
              <span>
                {formatMoneyMinor(
                  node.billing.monthly_equiv_minor ?? node.billing.cost_minor,
                  node.billing.currency,
                )}
                <span className="muted"> / {t("fleet.perMonth")}</span>
              </span>
            )}
            {node.billing.cost_raw && (
              <span className="muted">{node.billing.cost_raw}</span>
            )}
            {node.billing.next_due_date && (
              <span>
                {t("fleet.nextDue")} {formatDateTime(node.billing.next_due_date)}
              </span>
            )}
            {typeof node.billing.days_left === "number" && (
              <span>
                {t("fleet.daysLeft", { days: node.billing.days_left })}
              </span>
            )}
            {node.billing.server_ip && <span>{node.billing.server_ip}</span>}
          </div>
        )}

        <div className="field">
          <label className="label" htmlFor="bill-account">
            {t("fleet.billingAccount")}
          </label>
          <select
            id="bill-account"
            className="input"
            value={billingAccountId}
            onChange={(e) => setBillingAccountId(e.target.value)}
          >
            <option value="">{t("fleet.billingAccountAuto")}</option>
            {accounts.map((a) => (
              <option key={a.id} value={a.id}>
                {a.server_ip}
                {a.name ? ` — ${a.name}` : ""}
                {a.cost ? ` (${a.cost})` : ""}
                {a.expire_date ? ` · ${a.expire_date}` : ""}
              </option>
            ))}
          </select>
        </div>
        {accounts.length === 0 && (
          <p className="muted">{t("fleet.billingAccountsEmpty")}</p>
        )}
        <button type="submit" className="btn" disabled={busy} style={{ marginTop: "0.75rem" }}>
          {busy ? t("common.saving") : t("fleet.saveBilling")}
        </button>
      </form>

      {node.services && node.services.length > 0 && (
        <div className="card" style={{ padding: 0, overflow: "hidden" }}>
          <h2 className="section-title" style={{ padding: "1rem 1rem 0" }}>
            {t("fleet.services")}
          </h2>
          <div className="table-wrap">
            <table className="table">
              <thead>
                <tr>
                  <th>{t("fleet.serviceUnit")}</th>
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
            {t("fleet.filesystems")}
          </h2>
          <div className="table-wrap">
            <table className="table">
              <thead>
                <tr>
                  <th>{t("fleet.mount")}</th>
                  <th>{t("fleet.device")}</th>
                  <th>{t("fleet.used")}</th>
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
        title={t("fleet.removeServer")}
        message={t("fleet.removeServerConfirm", { name: node.name })}
        confirmLabel={t("common.delete")}
        onConfirm={handleDelete}
        onCancel={() => setConfirmDelete(false)}
        busy={busy}
      />
    </div>
  );
}
