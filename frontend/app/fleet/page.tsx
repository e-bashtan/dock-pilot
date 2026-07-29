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

  return (
    <div>
      <div className="page-header">
        <div>
          <h1>{t("fleet.title")}</h1>
          <p className="muted" style={{ margin: "0.35rem 0 0" }}>
            {t("fleet.subtitle")}
          </p>
        </div>
        <div style={{ display: "flex", gap: "0.5rem", flexWrap: "wrap" }}>
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
          {overview && (
            <div className="server-status-grid" style={{ marginBottom: "1.25rem" }}>
              <div className="card">
                <div className="label">{t("fleet.summaryServers")}</div>
                <div style={{ fontSize: "1.35rem", fontWeight: 600 }}>
                  {overview.servers_online}/{overview.servers_total}
                </div>
                <div className="muted" style={{ fontSize: "0.8rem" }}>
                  {t("fleet.summaryServersHint", {
                    warning: overview.servers_warning,
                    offline: overview.servers_offline,
                  })}
                </div>
              </div>
              <div className="card">
                <div className="label">{t("fleet.summaryApps")}</div>
                <div style={{ fontSize: "1.35rem", fontWeight: 600 }}>
                  {overview.apps_running}/{overview.apps_total}
                </div>
                <div className="muted" style={{ fontSize: "0.8rem" }}>
                  {t("fleet.summaryAppsHint", {
                    unhealthy: overview.apps_unhealthy,
                  })}
                </div>
              </div>
              <div className="card">
                <div className="label">{t("fleet.summaryCost")}</div>
                <div style={{ fontSize: "1.35rem", fontWeight: 600 }}>
                  {formatMoneyMinor(
                    overview.monthly_cost_minor,
                    overview.currency,
                  )}
                </div>
                <div className="muted" style={{ fontSize: "0.8rem" }}>
                  {overview.open_incidents > 0
                    ? t("fleet.summaryIncidents", {
                        count: overview.open_incidents,
                      })
                    : overview.next_due_date
                      ? t("fleet.summaryNextDue", {
                          date: formatDateTime(overview.next_due_date),
                        })
                      : t("fleet.summaryNoIncidents")}
                </div>
              </div>
            </div>
          )}

          <div
            style={{
              display: "flex",
              gap: "0.5rem",
              flexWrap: "wrap",
              marginBottom: "1rem",
            }}
          >
            {FILTERS.map((f) => (
              <button
                key={f}
                type="button"
                className={`btn${filter === f ? "" : " btn-secondary"}`}
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
            <div className="stack-list">
              {visible.map((node) => {
                const href = fleetNodeHref(node);
                const external = fleetNodeExternal(node);
                const card = (
                  <div className="stack-item">
                    <div className="stack-item-main">
                      <div
                        style={{
                          display: "flex",
                          gap: "0.5rem",
                          alignItems: "center",
                          flexWrap: "wrap",
                        }}
                      >
                        <strong>{node.name}</strong>
                        <FleetStatusBadge status={node.status} />
                        <FleetNodeBadges
                          role={node.role}
                          connectionType={node.connection_type}
                        />
                      </div>
                      <div className="stack-item-meta">
                        {node.hostname || node.node_uid}
                        {node.last_seen_at
                          ? ` · ${t("fleet.lastSeen")} ${formatDateTime(node.last_seen_at)}`
                          : ""}
                      </div>
                      {node.metrics && (
                        <div className="stack-item-meta">
                          CPU {formatPercent(node.metrics.cpu_percent)}
                          {" · "}
                          {t("fleet.memory")}{" "}
                          {formatBytes(node.metrics.memory_used_bytes)} /{" "}
                          {formatBytes(node.metrics.memory_total_bytes)}
                          {" · "}
                          {t("fleet.disk")}{" "}
                          {formatPercent(node.metrics.disk_used_percent)}
                        </div>
                      )}
                      {node.applications && (
                        <div className="stack-item-meta">
                          {t("fleet.appsLine", {
                            running: node.applications.running,
                            total: node.applications.total,
                            unhealthy: node.applications.unhealthy,
                          })}
                        </div>
                      )}
                      {node.open_incidents > 0 && (
                        <div className="stack-item-meta" style={{ color: "var(--danger, #b91c1c)" }}>
                          {t("fleet.openIncidents", { count: node.open_incidents })}
                        </div>
                      )}
                    </div>
                    <div className="stack-item-actions">
                      {href &&
                        (external ? (
                          <a
                            href={href}
                            className="btn btn-secondary"
                            target="_blank"
                            rel="noopener noreferrer"
                          >
                            {t("fleet.openPanel")}
                          </a>
                        ) : (
                          <Link href={href} className="btn btn-secondary">
                            {node.connection_type === "local"
                              ? t("fleet.manageLocal")
                              : t("fleet.viewDetails")}
                          </Link>
                        ))}
                    </div>
                  </div>
                );
                return <div key={node.id}>{card}</div>;
              })}
            </div>
          )}
        </>
      )}
    </div>
  );
}
