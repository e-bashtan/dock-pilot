"use client";

import Link from "next/link";
import { useCallback, useEffect, useMemo, useState } from "react";
import { ServerNodeBadges, ServerStatusBadge } from "@/components/ServerBadges";
import { api, ApiError } from "@/lib/api";
import {
  filterServerNodes,
  serverNodeExternal,
  serverNodeHref,
  isBarnPanel,
  sortServerNodes,
  type ServerFilter,
} from "@/lib/servers-utils";
import { formatBytes, formatMoneyMinor, formatPercent } from "@/lib/format";
import { useI18n } from "@/lib/i18n/context";
import type { ServerNode, ServersOverview } from "@/lib/types";

const FILTERS: ServerFilter[] = [
  "all",
  "problems",
  "offline",
  "barn",
  "monitored",
  "billing_due",
];

export default function ServersOverviewPage() {
  const { t, formatDateTime } = useI18n();
  const [overview, setOverview] = useState<ServersOverview | null>(null);
  const [nodes, setNodes] = useState<ServerNode[]>([]);
  const [filter, setFilter] = useState<ServerFilter>("all");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  const load = useCallback(async () => {
    try {
      const [ov, list] = await Promise.all([
        api.getServersOverview(),
        api.listServerNodes(),
      ]);
      setOverview(ov);
      setNodes(list);
      setError(null);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : t("servers.loadFailed"));
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
    () => sortServerNodes(filterServerNodes(nodes, filter)),
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
    <div className="servers-page">
      <div className="page-header">
        <div>
          <h1>{t("servers.title")}</h1>
          <p className="muted" style={{ margin: "0.35rem 0 0" }}>
            {t("servers.subtitle")}
          </p>
        </div>
        <div className="page-actions">
          <Link href="/servers/events" className="btn btn-secondary">
            {t("nav.serverEvents")}
          </Link>
          <Link href="/servers/settings" className="btn btn-secondary">
            {t("servers.settingsLink")}
          </Link>
          <Link href="/servers/new" className="btn">
            {t("servers.addServer")}
          </Link>
        </div>
      </div>

      {error && <div className="alert alert-error">{error}</div>}

      {loading ? (
        <p className="muted">{t("common.loading")}</p>
      ) : (
        <>
          <p className="servers-legend muted">{t("servers.monitoringLegend")}</p>

          {overview && (
            <div className="servers-summary">
              <div className="servers-summary-card">
                <div className="server-status-label">{t("servers.summaryServers")}</div>
                <div className="server-status-value">
                  {overview.servers_online}/{overview.servers_total}
                </div>
                <div className="server-status-meta">
                  {t("servers.summaryServersHint", {
                    warning: overview.servers_warning,
                    offline: overview.servers_offline,
                  })}
                </div>
              </div>
              <div className="servers-summary-card">
                <div className="server-status-label">{t("servers.summaryApps")}</div>
                <div className="server-status-value">
                  {overview.apps_running}/{overview.apps_total}
                </div>
                <div className="server-status-meta">
                  {t("servers.summaryAppsHint", {
                    unhealthy: overview.apps_unhealthy,
                  })}
                  {overview.open_incidents > 0
                    ? ` · ${t("servers.summaryIncidents", { count: overview.open_incidents })}`
                    : ""}
                </div>
              </div>
              <div className="servers-summary-card">
                <div className="server-status-label">{t("servers.summaryCost")}</div>
                <div className="server-status-value">
                  {formatMoneyMinor(
                    overview.monthly_cost_minor,
                    overview.currency,
                  )}
                </div>
                <div className="server-status-meta">
                  {overview.next_due_date
                    ? t("servers.summaryNextDue", {
                        date: formatDueDate(overview.next_due_date),
                      })
                    : overview.monthly_cost_minor === 0
                      ? t("servers.summaryBillingEmpty")
                      : t("servers.summaryBillingSet", { count: billedCount })}
                </div>
              </div>
            </div>
          )}

          <div className="servers-filters" role="tablist" aria-label={t("servers.title")}>
            {FILTERS.map((f) => (
              <button
                key={f}
                type="button"
                role="tab"
                aria-selected={filter === f}
                className={`servers-filter${filter === f ? " servers-filter-active" : ""}`}
                onClick={() => setFilter(f)}
              >
                {t(`servers.filter.${f}`)}
              </button>
            ))}
          </div>

          {visible.length === 0 ? (
            <div className="card">
              <p>{t("servers.empty")}</p>
              <Link href="/servers/new" className="btn" style={{ marginTop: "1rem" }}>
                {t("servers.addServer")}
              </Link>
            </div>
          ) : (
            <div className="servers-node-list">
              {visible.map((node) => (
                <ServerNodeRow
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

function ServerNodeRow({
  node,
  formatDateTime,
}: {
  node: ServerNode;
  formatDateTime: (value: string) => string;
}) {
  const { t } = useI18n();
  const href = serverNodeHref(node);
  const external = serverNodeExternal(node);
  const detailHref = `/servers/${node.id}`;
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
    <article className="servers-node-card">
      <div className="servers-node-top">
        <div className="servers-node-heading">
          <Link href={detailHref} className="servers-node-name">
            {node.name}
          </Link>
          <div className="servers-node-badges">
            <ServerStatusBadge status={node.status} />
            <ServerNodeBadges
              role={node.role}
              connectionType={node.connection_type}
            />
          </div>
        </div>
        <div className="servers-node-actions">
          <Link href={detailHref} className="btn btn-secondary">
            {t("servers.viewDetails")}
          </Link>
          {href && external && (
            <a
              href={href}
              className="btn btn-secondary"
              target="_blank"
              rel="noopener noreferrer"
            >
              {t("servers.openPanel")}
            </a>
          )}
          {href && !external && node.connection_type === "local" && (
            <Link href={href} className="btn btn-secondary">
              {t("servers.manageLocal")}
            </Link>
          )}
        </div>
      </div>

      <div className="servers-node-meta">
        <span>{node.hostname || node.base_url || node.node_uid}</span>
        {node.last_seen_at && (
          <span title={formatDateTime(node.last_seen_at)}>
            {t("servers.lastSeen")} {formatShortDateTime(node.last_seen_at, formatDateTime)}
          </span>
        )}
      </div>

      <div className="servers-node-metrics">
        {node.metrics ? (
          <>
            {node.status === "offline" && (
              <span className="servers-metric servers-metric-warn" title={t("servers.metricsStale")}>
                <IconAlert />
                <span>{t("servers.status.offline")}</span>
              </span>
            )}
            <span className="servers-metric" title="CPU">
              <IconCpu />
              <span>{formatPercent(node.metrics.cpu_percent)}</span>
            </span>
            <span
              className="servers-metric"
              title={t("servers.memory")}
            >
              <IconMemory />
              <span>
                {formatBytes(node.metrics.memory_used_bytes)}/
                {formatBytes(node.metrics.memory_total_bytes)}
              </span>
            </span>
            <span className="servers-metric" title={t("servers.disk")}>
              <IconDisk />
              <span>{formatPercent(node.metrics.disk_used_percent)}</span>
            </span>
            {node.applications && (
              <span
                className={`servers-metric${node.applications.unhealthy > 0 ? " servers-metric-warn" : ""}`}
                title={t("servers.appsLine", {
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
              <span className="servers-metric servers-metric-warn">
                <IconAlert />
                <span>{node.open_incidents}</span>
              </span>
            )}
          </>
        ) : (
          <span className="servers-metric servers-metric-muted">
            {t("servers.monitoringWaiting")}
          </span>
        )}

        <span className="servers-metric-sep" aria-hidden />

        {hasBilling ? (
          <>
            {monthly > 0 && (
              <span className="servers-metric" title={t("servers.summaryCost")}>
                <IconMoney />
                <span>
                  {formatMoneyMinor(monthly, node.billing?.currency)}
                  <span className="muted">/{t("servers.perMonth")}</span>
                </span>
              </span>
            )}
            {dueShort && (
              <span
                className={`servers-metric${billingDueSoon ? " servers-metric-warn" : ""}`}
                title={
                  node.billing?.next_due_date
                    ? `${t("servers.nextDue")} ${formatDateTime(node.billing.next_due_date)}`
                    : t("servers.nextDueDate")
                }
              >
                <IconCalendar />
                <span>
                  {t("servers.dueShort", { date: dueShort })}
                </span>
              </span>
            )}
            {typeof node.billing?.days_left === "number" && (
              <span
                className={`servers-metric${billingDueSoon ? " servers-metric-warn" : ""}`}
                title={t("servers.daysLeft", { days: node.billing.days_left })}
              >
                <IconClock />
                <span>{t("servers.daysLeftShort", { days: node.billing.days_left })}</span>
              </span>
            )}
            {provider && (
              <span className="servers-metric servers-metric-muted" title={provider}>
                <span className="servers-metric-text">{provider}</span>
              </span>
            )}
          </>
        ) : (
          <span className="servers-metric servers-metric-muted">
            <IconMoney />
            {isBarnPanel(node) && node.base_url ? (
              <a
                href={`${node.base_url.replace(/\/$/, "")}/payments`}
                className="servers-inline-link"
                target="_blank"
                rel="noopener noreferrer"
              >
                {t("servers.openNodePayments")}
              </a>
            ) : (
              <Link href={detailHref} className="servers-inline-link">
                {t("servers.setBilling")}
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
