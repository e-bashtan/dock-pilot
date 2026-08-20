"use client";

import { useCallback, useEffect, useState } from "react";
import { HealthBadge } from "@/components/HealthBadge";
import { api, ApiError } from "@/lib/api";
import { useI18n } from "@/lib/i18n/context";
import type { SiteHealth } from "@/lib/types";

export function SiteHealthPanel({
  siteId,
  autoRefreshMs = 30_000,
}: {
  siteId: string;
  autoRefreshMs?: number;
}) {
  const { t, formatDateTime } = useI18n();
  const [health, setHealth] = useState<SiteHealth | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    try {
      const h = await api.getSiteHealth(siteId);
      setHealth(h);
      setError(null);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : t("health.checkFailed"));
    } finally {
      setLoading(false);
    }
  }, [siteId, t]);

  useEffect(() => {
    setLoading(true);
    load();
    if (!autoRefreshMs) return;
    const timer = setInterval(load, autoRefreshMs);
    return () => clearInterval(timer);
  }, [load, autoRefreshMs]);

  if (loading && !health) {
    return (
      <div className="card site-health-summary">
        <span className="site-metric-label">{t("health.title")}</span>
        <p style={{ color: "var(--muted)", margin: 0 }}>{t("health.checking")}</p>
      </div>
    );
  }

  return (
    <div className="card site-health-summary">
      <div className="site-health-summary-head">
        <span className="site-metric-label">{t("health.title")}</span>
        <button type="button" className="site-icon-button" onClick={() => load()} title={t("common.refresh")} aria-label={t("common.refresh")}>
          ↻
        </button>
      </div>

      {error && <div className="alert alert-error">{error}</div>}

      {health && (
        <>
          <div className="site-health-state">
            <HealthBadge overall={health.overall} />{" "}
            <span className="site-metric-meta">
              {health.message}
            </span>
          </div>
          <dl className="site-health-metrics">
            {health.container && (
              <HealthRow
                label={t("health.container")}
                value={
                  health.container.found
                    ? `${health.container.container || t("common.emDash")} · ${health.container.state}${
                        health.container.health && health.container.health !== "none"
                          ? ` · HEALTH ${health.container.health}`
                          : ""
                      }`
                    : t("common.notFound")
                }
              />
            )}
            {health.http && (
              <HealthRow
                label={t("health.http")}
                value={
                  health.http.ok
                    ? `${health.http.url} → ${health.http.status_code}`
                    : health.http.error
                      ? `${health.http.url}: ${health.http.error}`
                      : `${health.http.url} → ${health.http.status_code}`
                }
              />
            )}
            <HealthRow
              label={t("common.checked")}
              value={formatDateTime(health.checked_at)}
            />
          </dl>
        </>
      )}
    </div>
  );
}

function HealthRow({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <dt>{label}</dt>
      <dd>{value}</dd>
    </div>
  );
}
