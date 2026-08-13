"use client";

import Link from "next/link";
import { useCallback, useEffect, useState } from "react";
import { BillingExpiryPanel } from "@/components/BillingExpiryPanel";
import { ConfirmDialog } from "@/components/ConfirmDialog";
import { HealthBadge } from "@/components/HealthBadge";
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

  return (
    <div>
      <ServerStatusPanel />
      <BillingExpiryPanel />
      <PostgresHealthSummary />

      <div className="page-header">
        <h1>{t("sites.title")}</h1>
        <div className="page-actions">
          <SiteJsonActions />
          <Link href="/sites/new" className="btn">
            {t("nav.newSite")}
          </Link>
        </div>
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
        <div className="card" style={{ padding: 0, overflow: "hidden" }}>
          <div className="table-wrap">
          <table className="table">
            <thead>
              <tr>
                <th>{t("sites.tableName")}</th>
                <th className="col-hide-mobile">{t("sites.tableType")}</th>
                <th>{t("sites.tableUrl")}</th>
                <th>{t("sites.tableHealth")}</th>
                <th className="col-hide-mobile">{t("sites.tableStatus")}</th>
                <th className="col-hide-mobile">{t("sites.tableUpdated")}</th>
                <th>{t("sites.delete")}</th>
              </tr>
            </thead>
            <tbody>
              {sites.map((site) => (
                <tr key={site.id}>
                  <td>
                    <Link href={`/sites/${site.id}`}>{site.name}</Link>
                    <div style={{ fontSize: "0.75rem", color: "var(--muted)" }}>
                      {site.slug}
                    </div>
                  </td>
                  <td className="col-hide-mobile">
                    {site.site_type === "telegram_bot"
                      ? t("sites.typeTelegramBot")
                      : t("sites.typeWebsite")}
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
                  <td className="col-hide-mobile">{formatDateTime(site.updated_at)}</td>
                  <td>
                    <button
                      type="button"
                      className="btn btn-secondary"
                      disabled={deletingId !== null}
                      onClick={() => setPendingDelete(site)}
                    >
                      {deletingId === site.id
                        ? t("sites.deleting")
                        : t("sites.delete")}
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          </div>
        </div>
      )}

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
