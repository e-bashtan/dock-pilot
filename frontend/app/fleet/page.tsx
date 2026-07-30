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
  const hasBilling =
    !!node.billing &&
    ((node.billing.monthly_equiv_minor ?? node.billing.cost_minor ?? 0) > 0 ||
      !!node.billing.next_due_date);
  const dueShort = node.billing?.next_due_date
    ? formatDueDate(node.billing.next_due_date)
    : "";
  const provider =
    node.billing?.provider_name || node.billing?.server_ip || "";
  const alertDays =
    node.billing?.alert_days && node.billing.alert_days > 0
      ? node.billing.alert_days
      : 10;
  const billingDueSoon =
    typeof node.billing?.days_left === "number" &&
    node.billing.days_left <= alertDays;

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
          <span title={formatDateTime(node.last_seen_at)}>
            {t("fleet.lastSeen")} {formatShortDateTime(node.last_seen_at, formatDateTime)}
          </span>
        )}
      </div>

      <div className="fleet-node-metrics">
        {node.metrics ? (
          <>
            {node.status === "offline" && (
              <span className="fleet-metric fleet-metric-warn" title={t("fleet.metricsStale")}>
                <IconAlert />
                <span>{t("fleet.status.offline")}</span>
              </span>
            )}
            <span className="fleet-metric" title="CPU">
              <IconCpu />
              <span>{formatPercent(node.metrics.cpu_percent)}</span>
            </span>
            <span
              className="fleet-metric"
              title={t("fleet.memory")}
            >
              <IconMemory />
              <span>
                {formatBytes(node.metrics.memory_used_bytes)}/
                {formatBytes(node.metrics.memory_total_bytes)}
              </span>
            </span>
            <span className="fleet-metric" title={t("fleet.disk")}>
              <IconDisk />
              <span>{formatPercent(node.metrics.disk_used_percent)}</span>
            </span>
            {node.applications && (
              <span
                className={`fleet-metric${node.applications.unhealthy > 0 ? " fleet-metric-warn" : ""}`}
                title={t("fleet.appsLine", {
                  running: node.applications.running,
                  total: node.applications.total,
                  unhealthy: node.applications.unhealthy,
                })}
              >
                <IconApps />
                <span>
                  {node.applications.running}/{node.applications.total}
                  {node.applications.unhealthy > 0
                    ? ` · ${node.applications.unhealthy}!`
                    : ""}
                </span>
              </span>
            )}
            {node.open_incidents > 0 && (
              <span className="fleet-metric fleet-metric-warn">
                <IconAlert />
                <span>{node.open_incidents}</span>
              </span>
            )}
          </>
        ) : (
          <span className="fleet-metric fleet-metric-muted">
            {t("fleet.monitoringWaiting")}
          </span>
        )}

        <span className="fleet-metric-sep" aria-hidden />

        {hasBilling ? (
          <>
            {monthly > 0 && (
              <span className="fleet-metric" title={t("fleet.summaryCost")}>
                <IconMoney />
                <span>
                  {formatMoneyMinor(monthly, node.billing?.currency)}
                  <span className="muted">/{t("fleet.perMonth")}</span>
                </span>
              </span>
            )}
            {dueShort && (
              <span
                className={`fleet-metric${billingDueSoon ? " fleet-metric-warn" : ""}`}
                title={
                  node.billing?.next_due_date
                    ? `${t("fleet.nextDue")} ${formatDateTime(node.billing.next_due_date)}`
                    : t("fleet.nextDueDate")
                }
              >
                <IconCalendar />
                <span>
                  {t("fleet.dueShort", { date: dueShort })}
                </span>
              </span>
            )}
            {typeof node.billing?.days_left === "number" && (
              <span
                className={`fleet-metric${billingDueSoon ? " fleet-metric-warn" : ""}`}
                title={t("fleet.daysLeft", { days: node.billing.days_left })}
              >
                <IconClock />
                <span>{t("fleet.daysLeftShort", { days: node.billing.days_left })}</span>
              </span>
            )}
            {provider && (
              <span className="fleet-metric fleet-metric-muted" title={provider}>
                <span className="fleet-metric-text">{provider}</span>
              </span>
            )}
          </>
        ) : (
          <span className="fleet-metric fleet-metric-muted">
            <IconMoney />
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
              <Link href={detailHref} className="fleet-inline-link">
                {t("fleet.setBilling")}
              </Link>
            )}
          </span>
        )}
      </div>
    </article>
  );
}

function formatDueDate(value: string): string {
  const raw = value.trim().slice(0, 10);
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(raw);
  if (!m) return raw;
  return `${m[3]}.${m[2]}.${m[1]}`;
}

function formatShortDateTime(
  value: string,
  formatDateTime: (value: string) => string,
): string {
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return formatDateTime(value);
  const dd = String(d.getDate()).padStart(2, "0");
  const mm = String(d.getMonth() + 1).padStart(2, "0");
  const hh = String(d.getHours()).padStart(2, "0");
  const mi = String(d.getMinutes()).padStart(2, "0");
  return `${dd}.${mm} ${hh}:${mi}`;
}

function IconCpu() {
  return (
    <svg width="13" height="13" viewBox="0 0 24 24" fill="none" aria-hidden>
      <rect x="7" y="7" width="10" height="10" rx="1.5" stroke="currentColor" strokeWidth="1.8" />
      <path d="M9 3v2M12 3v2M15 3v2M9 19v2M12 19v2M15 19v2M3 9h2M3 12h2M3 15h2M19 9h2M19 12h2M19 15h2" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" />
    </svg>
  );
}

function IconMemory() {
  return (
    <svg width="13" height="13" viewBox="0 0 24 24" fill="none" aria-hidden>
      <rect x="3" y="7" width="18" height="10" rx="1.5" stroke="currentColor" strokeWidth="1.8" />
      <path d="M7 7v10M11 7v10M15 7v10M7 17v2M11 17v2M15 17v2" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" />
    </svg>
  );
}

function IconDisk() {
  return (
    <svg width="13" height="13" viewBox="0 0 24 24" fill="none" aria-hidden>
      <ellipse cx="12" cy="6" rx="7" ry="3" stroke="currentColor" strokeWidth="1.8" />
      <path d="M5 6v6c0 1.7 3.1 3 7 3s7-1.3 7-3V6M5 12v6c0 1.7 3.1 3 7 3s7-1.3 7-3v-6" stroke="currentColor" strokeWidth="1.8" />
    </svg>
  );
}

function IconApps() {
  return (
    <svg width="13" height="13" viewBox="0 0 24 24" fill="none" aria-hidden>
      <rect x="4" y="4" width="7" height="7" rx="1.2" stroke="currentColor" strokeWidth="1.8" />
      <rect x="13" y="4" width="7" height="7" rx="1.2" stroke="currentColor" strokeWidth="1.8" />
      <rect x="4" y="13" width="7" height="7" rx="1.2" stroke="currentColor" strokeWidth="1.8" />
      <rect x="13" y="13" width="7" height="7" rx="1.2" stroke="currentColor" strokeWidth="1.8" />
    </svg>
  );
}

function IconMoney() {
  return (
    <svg width="13" height="13" viewBox="0 0 24 24" fill="none" aria-hidden>
      <circle cx="12" cy="12" r="8" stroke="currentColor" strokeWidth="1.8" />
      <path d="M12 7v10M9.5 9.5c.6-1 1.5-1.5 2.5-1.5 1.5 0 2.5.8 2.5 2s-1 2-2.5 2h-1c-1.5 0-2.5.8-2.5 2s1 2 2.5 2c1 0 1.9-.5 2.5-1.5" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" />
    </svg>
  );
}

function IconCalendar() {
  return (
    <svg width="13" height="13" viewBox="0 0 24 24" fill="none" aria-hidden>
      <rect x="4" y="5" width="16" height="15" rx="2" stroke="currentColor" strokeWidth="1.8" />
      <path d="M8 3v4M16 3v4M4 10h16" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" />
    </svg>
  );
}

function IconClock() {
  return (
    <svg width="13" height="13" viewBox="0 0 24 24" fill="none" aria-hidden>
      <circle cx="12" cy="12" r="8" stroke="currentColor" strokeWidth="1.8" />
      <path d="M12 8v5l3 2" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

function IconAlert() {
  return (
    <svg width="13" height="13" viewBox="0 0 24 24" fill="none" aria-hidden>
      <path d="M12 4 3.5 19h17L12 4Z" stroke="currentColor" strokeWidth="1.8" strokeLinejoin="round" />
      <path d="M12 10v4M12 16.5v.5" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" />
    </svg>
  );
}
