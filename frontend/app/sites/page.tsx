"use client";

import Link from "next/link";
import { useCallback, useEffect, useState } from "react";
import { BillingExpiryPanel } from "@/components/BillingExpiryPanel";
import { ConfirmDialog } from "@/components/ConfirmDialog";
import { HealthBadge } from "@/components/HealthBadge";
import { HomeSystemSummary } from "@/components/HomeSystemSummary";
import { PostgresHealthSummary } from "@/components/PostgresHealthSummary";
import { ServerStatusPanel } from "@/components/ServerStatusPanel";
import { SiteJsonActions } from "@/components/SiteJsonActions";
import { StatusBadge } from "@/components/StatusBadge";
import { api, ApiError } from "@/lib/api";
import { useI18n } from "@/lib/i18n/context";
import { siteUrlHref } from "@/lib/site-url";
import type { SiteHealth, SiteListItem } from "@/lib/types";

export default function SitesPage() {
  const { t, formatDateTime } = useI18n();
  const [sites, setSites] = useState<SiteListItem[]>([]);
  const [healthBySite, setHealthBySite] = useState<Record<string, SiteHealth>>({});
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [deletingId, setDeletingId] = useState<string | null>(null);
  const [pendingDelete, setPendingDelete] = useState<SiteListItem | null>(null);
  const [query, setQuery] = useState("");

  const loadSites = useCallback(async () => {
    try {
      const rows = await api.listSites();
      setSites(rows);
      setError(null);
    } catch (e: unknown) {
      setError(e instanceof ApiError ? e.message : t("sites.loadFailed"));
    } finally {
      setLoading(false);
    }
  }, [t]);

  const loadHealth = useCallback(async () => {
    try {
      const rows = await api.listSitesHealth();
      const map: Record<string, SiteHealth> = {};
      for (const h of rows) {
        map[h.site_id] = h;
      }
      setHealthBySite(map);
    } catch {
      /* health is optional on list */
    }
  }, []);

  useEffect(() => {
    void loadSites();
    void loadHealth();
    const timer = setInterval(loadHealth, 30_000);
    return () => clearInterval(timer);
  }, [loadHealth, loadSites]);

  const runDelete = async (site: SiteListItem) => {
    setDeletingId(site.id);
    setError(null);
    try {
      await api.deleteSite(site.id);
      setPendingDelete(null);
      setSites((prev) => prev.filter((s) => s.id !== site.id));
    } catch (e: unknown) {
      setError(e instanceof ApiError ? e.message : t("sites.deleteFailed"));
    } finally {
      setDeletingId(null);
    }
  };

  const normalizedQuery = query.trim().toLocaleLowerCase();
  const visibleSites = normalizedQuery
    ? sites.filter((site) =>
        [site.name, site.slug, site.primary_url, site.site_type]
          .join(" ")
          .toLocaleLowerCase()
          .includes(normalizedQuery),
      )
    : sites;
  const healthyCount = sites.filter((site) => healthBySite[site.id]?.overall === "healthy").length;
  const attentionCount = sites.filter((site) => {
    const overall = healthBySite[site.id]?.overall;
    return overall && overall !== "healthy";
  }).length;

  return (
    <div className="sites-dashboard">
      <HomeSystemSummary />
      <div className="page-header sites-dashboard-header">
        <div>
          <h1>{t("sites.title")}</h1>
          <p className="muted sites-dashboard-subtitle">{t("sites.dashboardSubtitle")}</p>
        </div>
        <div className="page-actions">
          <SiteJsonActions />
          <Link href="/sites/new" className="btn">
            {t("nav.newSite")}
          </Link>
        </div>
      </div>

      <div className="sites-summary-strip" aria-label={t("sites.summaryLabel")}>
        <div><span>{t("sites.summaryTotal")}</span><strong>{sites.length}</strong></div>
        <div><span>{t("sites.summaryHealthy")}</span><strong className="sites-summary-good">{healthyCount}</strong></div>
        <div><span>{t("sites.summaryAttention")}</span><strong className={attentionCount ? "sites-summary-warn" : ""}>{attentionCount}</strong></div>
        <div><span>{t("sites.summaryWeb")}</span><strong>{sites.filter((site) => site.site_type === "web").length}</strong></div>
        <div><span>{t("sites.summaryBots")}</span><strong>{sites.filter((site) => site.site_type === "telegram_bot").length}</strong></div>
      </div>

      {error && <div className="alert alert-error">{error}</div>}

      {loading ? (
        <p style={{ color: "var(--muted)" }}>{t("common.loading")}</p>
      ) : sites.length === 0 ? (
        <div className="card">
          <p>{t("sites.empty")}</p>
          <div className="page-actions" style={{ marginTop: "1rem", justifyContent: "flex-start" }}>
            <SiteJsonActions />
            <Link href="/sites/new" className="btn">
              {t("sites.createSite")}
            </Link>
          </div>
        </div>
      ) : (
        <div className="card sites-table-card">
          <div className="sites-table-toolbar">
            <div>
              <h2>{t("sites.allSites")}</h2>
              <span className="muted">{t("sites.itemsCount", { count: visibleSites.length })}</span>
            </div>
            <input
              className="input sites-search"
              type="search"
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              placeholder={t("sites.searchPlaceholder")}
              aria-label={t("sites.searchPlaceholder")}
            />
          </div>
          <div className="table-wrap">
          <table className="table">
            <thead>
              <tr>
                <th>{t("sites.tableName")}</th>
                <th>{t("sites.tableUrl")}</th>
                <th>{t("sites.tableHealth")}</th>
                <th className="col-hide-mobile">{t("sites.tableStatus")}</th>
                <th className="col-hide-mobile">{t("sites.tableUpdated")}</th>
                <th aria-label={t("common.actions")}></th>
              </tr>
            </thead>
            <tbody>
              {visibleSites.map((site) => (
                <tr key={site.id}>
                  <td>
                    <div className="site-name-cell">
                      <span className={`site-type-icon site-type-icon-${site.site_type}`} aria-hidden>
                        {site.site_type === "telegram_bot" ? "B" : "W"}
                      </span>
                      <div>
                        <Link href={`/sites/${site.id}`}>{site.name}</Link>
                        <div className="site-row-meta">
                          {site.slug} · {site.site_type === "telegram_bot" ? t("sites.typeTelegramBot") : t("sites.typeWebsite")}
                        </div>
                      </div>
                    </div>
                  </td>
                  <td>
                    {site.site_type === "telegram_bot" ? (
                      t("common.emDash")
                    ) : site.primary_url ? (
                      <a
                        href={siteUrlHref(site.primary_url)}
                        target="_blank"
                        rel="noopener noreferrer"
                      >
                        {site.primary_url}
                      </a>
                    ) : (
                      t("common.emDash")
                    )}
                  </td>
                  <td>
                    {healthBySite[site.id] ? (
                      <span title={healthBySite[site.id].message}>
                        <HealthBadge overall={healthBySite[site.id].overall} />
                      </span>
                    ) : (
                      <span style={{ color: "var(--muted)", fontSize: "0.875rem" }}>
                        …
                      </span>
                    )}
                  </td>
                  <td className="col-hide-mobile">
                    <StatusBadge status={site.status} />
                  </td>
                  <td className="col-hide-mobile">
                    {site.last_deployed_at
                      ? formatDateTime(site.last_deployed_at)
                      : t("common.emDash")}
                  </td>
                  <td>
                    <details className="site-row-menu">
                      <summary aria-label={t("common.actions")}>•••</summary>
                      <div>
                        <Link href={`/sites/${site.id}`}>{t("sites.openSite")}</Link>
                        <button type="button" disabled={deletingId !== null} onClick={() => setPendingDelete(site)}>
                          {deletingId === site.id ? t("sites.deleting") : t("sites.delete")}
                        </button>
                      </div>
                    </details>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          </div>
        </div>
      )}

      <details className="dashboard-system-details">
        <summary>{t("sites.infrastructure")}</summary>
        <div className="dashboard-system-content">
          <ServerStatusPanel showUpdate={false} />
          <BillingExpiryPanel />
          <PostgresHealthSummary />
        </div>
      </details>

      <ConfirmDialog
        open={pendingDelete !== null}
        title={t("sites.deleteConfirmTitle")}
        message={
          pendingDelete
            ? t("sites.deleteConfirm", { name: pendingDelete.name })
            : ""
        }
        danger
        busy={deletingId !== null}
        onCancel={() => {
          if (deletingId === null) setPendingDelete(null);
        }}
        onConfirm={() => {
          if (pendingDelete) void runDelete(pendingDelete);
        }}
      />
    </div>
  );
}
