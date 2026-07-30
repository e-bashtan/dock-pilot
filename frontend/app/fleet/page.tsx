"use client";

import Link from "next/link";
import { useCallback, useEffect, useMemo, useState } from "react";
import { FleetNodeBadges, FleetStatusBadge } from "@/components/FleetBadges";
import { api, ApiError } from "@/lib/api";
import {
  filterFleetNodes,
  fleetNodeExternal,
  fleetNodeHref,
  sortFleetNodes,
  type FleetServerFilter,
} from "@/lib/fleet-utils";
import { formatBytes, formatMoneyMinor, formatPercent } from "@/lib/format";
import { useI18n } from "@/lib/i18n/context";
import type { FleetNode, FleetOverview } from "@/lib/types";

const FILTERS: FleetServerFilter[] = [
  "all",
  "problems",
  "offline",
  "dockpilot",
  "monitored",
  "billing_due",
];

export default function FleetOverviewPage() {
  const { t, formatDateTime } = useI18n();
  const [overview, setOverview] = useState<FleetOverview | null>(null);
  const [nodes, setNodes] = useState<FleetNode[]>([]);
  const [filter, setFilter] = useState<FleetServerFilter>("all");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  const load = useCallback(async () => {
    try {
      const [ov, list] = await Promise.all([
        api.getFleetOverview(),
        api.listFleetNodes(),
      ]);
      setOverview(ov);
      setNodes(list);
      setError(null);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : t("fleet.loadFailed"));
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    void load();
    const timer = setInterval(() => void load(), 30_000);
    return () => clearInterval(timer);
  }, [load]);

  const visible = useMemo(
    () => sortFleetNodes(filterFleetNodes(nodes, filter)),
    [nodes, filter],
  );

  const billedCount = useMemo(
    () =>
      nodes.filter(
        (n) =>
          n.billing &&
          ((n.billing.cost_minor ?? 0) > 0 || !!n.billing.next_due_date),
      ).length,
    [nodes],
  );

  return (
    <div className="fleet-page">
      <div className="page-header">
        <div>
          <h1>{t("fleet.title")}</h1>
          <p className="muted" style={{ margin: "0.35rem 0 0" }}>
            {t("fleet.subtitle")}
          </p>
        </div>
        <div className="page-actions">
          <Link href="/fleet/settings" className="btn btn-secondary">
            {t("fleet.settingsLink")}
          </Link>
          <Link href="/fleet/servers/new" className="btn">
            {t("fleet.addServer")}
          </Link>
        </div>
      </div>

      {error && <div className="alert alert-error">{error}</div>}

      {loading ? (
        <p className="muted">{t("common.loading")}</p>
      ) : (
        <>
          <p className="fleet-legend muted">{t("fleet.monitoringLegend")}</p>

          {overview && (
            <div className="fleet-summary">
              <div className="fleet-summary-card">
                <div className="server-status-label">{t("fleet.summaryServers")}</div>
                <div className="server-status-value">
                  {overview.servers_online}/{overview.servers_total}
                </div>
                <div className="server-status-meta">
                  {t("fleet.summaryServersHint", {
                    warning: overview.servers_warning,
                    offline: overview.servers_offline,
                  })}
                </div>
              </div>
              <div className="fleet-summary-card">
                <div className="server-status-label">{t("fleet.summaryApps")}</div>
                <div className="server-status-value">
                  {overview.apps_running}/{overview.apps_total}
                </div>
                <div className="server-status-meta">
                  {t("fleet.summaryAppsHint", {
                    unhealthy: overview.apps_unhealthy,
                  })}
                  {overview.open_incidents > 0
                    ? ` · ${t("fleet.summaryIncidents", { count: overview.open_incidents })}`
                    : ""}
                </div>
              </div>
              <div className="fleet-summary-card">
                <div className="server-status-label">{t("fleet.summaryCost")}</div>
                <div className="server-status-value">
                  {formatMoneyMinor(
                    overview.monthly_cost_minor,
                    overview.currency,
                  )}
                </div>
                <div className="server-status-meta">
                  {overview.next_due_date
                    ? t("fleet.summaryNextDue", {
                        date: formatDateTime(overview.next_due_date),
                      })
                    : overview.monthly_cost_minor === 0
                      ? t("fleet.summaryBillingEmpty")
                      : t("fleet.summaryBillingSet", { count: billedCount })}
                </div>
              </div>
            </div>
          )}

          <div className="fleet-filters" role="tablist" aria-label={t("fleet.title")}>
            {FILTERS.map((f) => (
              <button
                key={f}
                type="button"
                role="tab"
                aria-selected={filter === f}
                className={`fleet-filter${filter === f ? " fleet-filter-active" : ""}`}
                onClick={() => setFilter(f)}
              >
                {t(`fleet.filter.${f}`)}
              </button>
            ))}
          </div>

          {visible.length === 0 ? (
            <div className="card">
              <p>{t("fleet.empty")}</p>
              <Link href="/fleet/servers/new" className="btn" style={{ marginTop: "1rem" }}>
                {t("fleet.addServer")}
              </Link>
            </div>
          ) : (
            <div className="fleet-node-list">
              {visible.map((node) => (
                <FleetNodeRow
                  key={node.id}
                  node={node}
                  formatDateTime={formatDateTime}
                />
              ))}
            </div>
          )}
        </>
      )}
    </div>
  );
}

function FleetNodeRow({
  node,
  formatDateTime,
}: {
  node: FleetNode;
  formatDateTime: (value: string) => string;
}) {
  const { t } = useI18n();
  const href = fleetNodeHref(node);
  const external = fleetNodeExternal(node);
  const detailHref = `/fleet/servers/${node.id}`;
  const monthly =
    node.billing?.monthly_equiv_minor ?? node.billing?.cost_minor ?? 0;

  return (
    <article className="fleet-node-card">
      <div className="fleet-node-top">
        <div className="fleet-node-heading">
          <Link href={detailHref} className="fleet-node-name">
            {node.name}
          </Link>
          <div className="fleet-node-badges">
            <FleetStatusBadge status={node.status} />
            <FleetNodeBadges
              role={node.role}
              connectionType={node.connection_type}
            />
          </div>
        </div>
        <div className="fleet-node-actions">
          <Link href={detailHref} className="btn btn-secondary">
            {t("fleet.viewDetails")}
          </Link>
          {href && external && (
            <a
              href={href}
              className="btn btn-secondary"
              target="_blank"
              rel="noopener noreferrer"
            >
              {t("fleet.openPanel")}
            </a>
          )}
          {href && !external && node.connection_type === "local" && (
            <Link href={href} className="btn btn-secondary">
              {t("fleet.manageLocal")}
            </Link>
          )}
        </div>
      </div>

      <div className="fleet-node-meta">
        <span>{node.hostname || node.base_url || node.node_uid}</span>
        {node.last_seen_at && (
          <span>
            {t("fleet.lastSeen")} {formatDateTime(node.last_seen_at)}
          </span>
        )}
      </div>

      <div className="fleet-node-sections">
        <div className="fleet-node-section">
          <div className="fleet-node-section-label">{t("fleet.monitoringSection")}</div>
          {node.metrics ? (
            <div className="fleet-node-stats">
              <span>CPU {formatPercent(node.metrics.cpu_percent)}</span>
              <span>
                {t("fleet.memory")}{" "}
                {formatBytes(node.metrics.memory_used_bytes)} /{" "}
                {formatBytes(node.metrics.memory_total_bytes)}
              </span>
              <span>
                {t("fleet.disk")} {formatPercent(node.metrics.disk_used_percent)}
              </span>
              {node.applications && (
                <span>
                  {t("fleet.appsLine", {
                    running: node.applications.running,
                    total: node.applications.total,
                    unhealthy: node.applications.unhealthy,
                  })}
                </span>
              )}
              {node.open_incidents > 0 && (
                <span className="fleet-node-incidents">
                  {t("fleet.openIncidents", { count: node.open_incidents })}
                </span>
              )}
            </div>
          ) : (
            <div className="fleet-node-stats muted">{t("fleet.monitoringWaiting")}</div>
          )}
        </div>

        <div className="fleet-node-section">
          <div className="fleet-node-section-label">{t("fleet.billingSection")}</div>
          {node.billing &&
          ((node.billing.monthly_equiv_minor ?? node.billing.cost_minor ?? 0) >
            0 ||
            node.billing.next_due_date) ? (
            <div className="fleet-node-stats">
              {(node.billing.monthly_equiv_minor ??
                node.billing.cost_minor ??
                0) > 0 && (
                <span>
                  {formatMoneyMinor(monthly, node.billing.currency)}
                  <span className="muted"> / {t("fleet.perMonth")}</span>
                </span>
              )}
              {node.billing.next_due_date && (
                <span>
                  {t("fleet.nextDue")}{" "}
                  {formatDateTime(node.billing.next_due_date)}
                </span>
              )}
              {typeof node.billing.days_left === "number" && (
                <span>
                  {t("fleet.daysLeft", { days: node.billing.days_left })}
                </span>
              )}
              {(node.billing.provider_name || node.billing.server_ip) && (
                <span>
                  {node.billing.provider_name || node.billing.server_ip}
                </span>
              )}
              <Link href="/payments" className="fleet-inline-link">
                {t("fleet.openPayments")}
              </Link>
            </div>
          ) : (
            <div className="fleet-node-stats">
              <span className="muted">{t("fleet.billingNotSet")}</span>
              {node.connection_type === "dockpilot" && node.base_url ? (
                <a
                  href={`${node.base_url.replace(/\/$/, "")}/payments`}
                  className="fleet-inline-link"
                  target="_blank"
                  rel="noopener noreferrer"
                >
                  {t("fleet.openNodePayments")}
                </a>
              ) : (
                <Link href="/payments" className="fleet-inline-link">
                  {t("fleet.configurePayments")}
                </Link>
              )}
            </div>
          )}
        </div>
      </div>
    </article>
  );
}
