"use client";

import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { useCallback, useEffect, useState } from "react";
import { ConfirmDialog } from "@/components/ConfirmDialog";
import { FleetNodeBadges, FleetStatusBadge } from "@/components/FleetBadges";
import { api, ApiError } from "@/lib/api";
import { formatBytes, formatMoneyMinor, formatPercent } from "@/lib/format";
import { useI18n } from "@/lib/i18n/context";
import type { FleetNode } from "@/lib/types";

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

  const load = useCallback(async () => {
    if (!id) return;
    try {
      const row = await api.getFleetNode(id);
      setNode(row);
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

      {node.billing && (
        <div className="card">
          <h2 className="section-title">{t("fleet.billing")}</h2>
          <p>
            {formatMoneyMinor(node.billing.cost_minor, node.billing.currency)}
            {node.billing.next_due_date
              ? ` · ${t("fleet.nextDue")} ${formatDateTime(node.billing.next_due_date)}`
              : ""}
          </p>
          {node.billing.provider_name && (
            <p className="muted">
              {node.billing.provider_name}
              {node.billing.provider_url && (
                <>
                  {" · "}
                  <a href={node.billing.provider_url} target="_blank" rel="noopener noreferrer">
                    {t("fleet.providerLink")}
                  </a>
                </>
              )}
            </p>
          )}
        </div>
      )}

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
