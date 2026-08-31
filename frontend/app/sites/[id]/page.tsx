"use client";

import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { useCallback, useEffect, useState } from "react";
import { ConfirmDialog } from "@/components/ConfirmDialog";
import { SiteHealthPanel } from "@/components/SiteHealthPanel";
import { SiteTabs } from "@/components/SiteTabs";
import { StatusBadge } from "@/components/StatusBadge";
import { api, ApiError } from "@/lib/api";
import { useI18n } from "@/lib/i18n/context";
import { downloadSiteExport } from "@/lib/site-import";
import type { Deployment, Site, SiteHealthContainer } from "@/lib/types";

type PendingAction = "start" | "stop" | "restart" | "delete";

export default function SiteDetailPage() {
  const { id } = useParams<{ id: string }>();
  const router = useRouter();
  const { t, formatDateTime } = useI18n();
  const [site, setSite] = useState<Site | null>(null);
  const [latestDep, setLatestDep] = useState<Deployment | null>(null);
  const [containerState, setContainerState] = useState<SiteHealthContainer | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [deploying, setDeploying] = useState(false);
  const [containerBusy, setContainerBusy] = useState<PendingAction | null>(null);
  const [healthRefreshKey, setHealthRefreshKey] = useState(0);
  const [pendingAction, setPendingAction] = useState<PendingAction | null>(null);
  const [exporting, setExporting] = useState(false);

  const load = useCallback(async () => {
    try {
      const [s, deps, health] = await Promise.all([
        api.getSite(id),
        api.listDeployments(id),
        api.getSiteHealth(id).catch(() => null),
      ]);
      setSite(s);
      setLatestDep(deps[0] ?? null);
      setContainerState(health?.container ?? null);
      setError(null);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : t("site.loadFailed"));
    }
  }, [id, t]);

  useEffect(() => {
    void load();
  }, [load]);

  const deploymentActive =
    latestDep?.status === "pending" || latestDep?.status === "running";

  useEffect(() => {
    if (!deploymentActive) return;
    const timer = window.setInterval(() => void load(), 2_000);
    return () => window.clearInterval(timer);
  }, [deploymentActive, load]);

  const refreshAfterAction = useCallback(async () => {
    await load();
    setHealthRefreshKey((k) => k + 1);
  }, [load]);

  const handleDeploy = async () => {
    setDeploying(true);
    try {
      const dep = await api.deploySite(id);
      setLatestDep(dep);
      setError(null);
      await refreshAfterAction();
    } catch (e) {
      setError(e instanceof ApiError ? e.message : t("site.deployFailed"));
    } finally {
      setDeploying(false);
    }
  };

  const handleExport = async () => {
    setExporting(true);
    setError(null);
    try {
      const doc = await api.exportSite(id);
      downloadSiteExport(doc);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : t("siteImport.exportFailed"));
    } finally {
      setExporting(false);
    }
  };

  const runContainerAction = async (action: "start" | "stop" | "restart") => {
    setContainerBusy(action);
    setError(null);
    try {
      if (action === "start") await api.startSiteContainer(id);
      else if (action === "stop") await api.stopSiteContainer(id);
      else await api.restartSiteContainer(id);
      await refreshAfterAction();
    } catch (e) {
      setError(e instanceof ApiError ? e.message : t("site.containerActionFailed"));
    } finally {
      setContainerBusy(null);
    }
  };

  const runDelete = async () => {
    setContainerBusy("delete");
    setError(null);
    try {
      await api.deleteSite(id);
      router.push("/sites");
    } catch (e) {
      setError(e instanceof ApiError ? e.message : t("site.loadFailed"));
      setContainerBusy(null);
    }
  };

  const handleConfirm = async () => {
    if (!pendingAction) return;
    const action = pendingAction;
    setPendingAction(null);
    if (action === "delete") {
      await runDelete();
      return;
    }
    await runContainerAction(action);
  };

  const confirmCopy = (action: PendingAction, siteName: string) => {
    switch (action) {
      case "start":
        return {
          title: t("site.confirmStartTitle"),
          message: t("site.startConfirm", { name: siteName }),
        };
      case "stop":
        return {
          title: t("site.confirmStopTitle"),
          message: t("site.stopConfirm", { name: siteName }),
        };
      case "restart":
        return {
          title: t("site.confirmRestartTitle"),
          message: t("site.restartConfirm", { name: siteName }),
        };
      case "delete":
        return {
          title: t("site.confirmDeleteTitle"),
          message: t("site.deleteConfirm", { name: siteName }),
        };
    }
  };

  if (!site && !error) {
    return <p style={{ color: "var(--muted)" }}>{t("common.loading")}</p>;
  }

  if (error && !site) {
    return <div className="alert alert-error">{error}</div>;
  }

  if (!site) return null;

  const typeLabel =
    site.site_type === "telegram_bot" ? t("sites.typeTelegramBot") : t("sites.typeWebsite");
  const canControlContainer = site.status !== "draft";
  const containerRunning = containerState?.found === true && containerState.running === true;
  const containerFound = containerState?.found === true;
  const stateKnown = containerState !== null;
  const busy = deploying || deploymentActive || containerBusy !== null;
  const pendingCopy = pendingAction ? confirmCopy(pendingAction, site.name) : null;

  return (
    <div className="site-overview-page">
      <div className="site-overview-header">
        <div className="site-overview-identity">
          <h1>{site.name}</h1>
          <p className="page-header-meta">
            {site.site_type === "telegram_bot" ? (
              typeLabel
            ) : (
              <a href={site.primary_url} target="_blank" rel="noreferrer">
                {site.primary_url} ↗
              </a>
            )}{" "}
            <span aria-hidden>·</span>{" "}
            <StatusBadge status={deploymentActive ? "deploying" : site.status} />
          </p>
        </div>
        <div className="site-overview-actions">
          <button
            type="button"
            className="btn"
            onClick={() => void handleDeploy()}
            disabled={busy}
          >
            {deploying || deploymentActive ? t("status.deploying") : t("site.deploy")}
          </button>
          {containerRunning ? (
            <button type="button" className="btn btn-secondary" onClick={() => setPendingAction("stop")} disabled={busy || !canControlContainer}>
              {containerBusy === "stop" ? t("site.stopping") : t("site.stop")}
            </button>
          ) : (
            <button type="button" className="btn btn-secondary" onClick={() => setPendingAction("start")} disabled={busy || !canControlContainer || !stateKnown}>
              {containerBusy === "start" ? t("site.starting") : t("site.start")}
            </button>
          )}
          <button
            type="button"
            className="btn btn-secondary"
            onClick={() => setPendingAction("restart")}
            disabled={busy || !canControlContainer || !stateKnown || !containerFound || !containerRunning}
            title={!containerRunning ? t("site.restartNotRunning") : undefined}
          >
            {containerBusy === "restart" ? t("site.restarting") : t("site.restart")}
          </button>
          <details className="site-overview-more">
            <summary className="btn btn-secondary" aria-label={t("common.actions")}>•••</summary>
            <div className="site-overview-more-menu">
              <Link href={`/sites/${id}/settings`}>{t("site.editSettings")}</Link>
              <button type="button" onClick={() => void handleExport()} disabled={exporting}>
                {exporting ? t("siteImport.exporting") : t("siteImport.exportJson")}
              </button>
              <button type="button" onClick={() => setPendingAction("delete")} disabled={busy}>
                {containerBusy === "delete" ? t("common.loading") : t("common.delete")}
              </button>
            </div>
          </details>
        </div>
      </div>

      <SiteTabs siteId={id} active="overview" />

      {error && <div className="alert alert-error">{error}</div>}

      <div className="site-overview-status-grid">
        <SiteHealthPanel key={healthRefreshKey} siteId={id} />
        <div className="card site-deployment-summary">
          <div>
            <span className="site-metric-label">{t("site.latestDeployment")}</span>
            {latestDep ? (
              <div className="site-metric-value"><StatusBadge status={latestDep.status} /> {latestDep.message}</div>
            ) : (
              <div className="site-metric-value">{t("common.emDash")}</div>
            )}
          </div>
          {latestDep && <span className="site-metric-meta">{formatDateTime(latestDep.created_at)}</span>}
          <Link href={`/sites/${id}/deployments`}>{t("site.allDeployments")}</Link>
        </div>
      </div>

      <div className="grid-2 site-overview-details">
        <div className="card site-configuration-card">
          <h3>{t("site.configuration")}</h3>
          <dl>
            <Info label={t("common.type")} value={typeLabel} />
            <Info label={t("site.slug")} value={site.slug} />
            <Info label={t("site.git")} value={site.git_repo_url} />
            <Info label={t("site.branch")} value={site.git_branch} />
            {site.site_type === "web" && (
              <>
                <Info
                  label={t("site.docker")}
                  value={`${site.dockerfile_path} → :${site.container_port}`}
                />
                <Info
                  label={t("site.hostPort")}
                  value={
                    site.docker_network_host
                      ? t("site.hostNetwork")
                      : site.host_port
                        ? String(site.host_port)
                        : t("common.emDash")
                  }
                />
                <Info
                  label={t("site.healthCheckPath")}
                  value={site.health_check_path?.trim() || t("site.healthCheckPathDefault")}
                />
              </>
            )}
            {site.site_type === "telegram_bot" && (
              <Info label={t("site.dockerfile")} value={site.dockerfile_path} />
            )}
          </dl>
        </div>
        {site.site_type === "web" && (
          <div className="card">
            <h3>{t("site.domains")}</h3>
            <ul className="site-domain-list">
              {site.domains.map((d) => (
                <li key={d.id ?? d.domain}>
                  {d.domain}
                  {d.is_primary ? ` ${t("common.primary")}` : ""}
                </li>
              ))}
            </ul>
            <Link
              href={`/sites/${id}/settings`}
              style={{ fontSize: "0.875rem", marginTop: "0.75rem", display: "inline-block" }}
            >
              {t("site.editSettings")}
            </Link>
          </div>
        )}
        {site.site_type === "telegram_bot" && (
          <div className="card">
            <h3>{t("site.bot")}</h3>
            <p style={{ margin: 0, color: "var(--muted)", fontSize: "0.875rem" }}>
              {t("site.botHintBefore")}{" "}
              <Link href={`/sites/${id}/secrets`}>{t("site.secretsLink")}</Link>{" "}
              {t("site.botHintAfter")}
            </p>
            <Link
              href={`/sites/${id}/settings`}
              style={{ fontSize: "0.875rem", marginTop: "0.75rem", display: "inline-block" }}
            >
              {t("site.editSettings")}
            </Link>
          </div>
        )}
      </div>

      {pendingCopy && (
        <ConfirmDialog
          open
          title={pendingCopy.title}
          message={pendingCopy.message}
          confirmLabel={
            pendingAction === "delete"
              ? t("common.delete")
              : pendingAction === "start"
                ? t("site.start")
                : pendingAction === "stop"
                  ? t("site.stop")
                  : t("site.restart")
          }
          danger={pendingAction === "delete" || pendingAction === "stop"}
          busy={containerBusy !== null}
          onConfirm={() => void handleConfirm()}
          onCancel={() => setPendingAction(null)}
        />
      )}
    </div>
  );
}

function Info({ label, value }: { label: string; value: string }) {
  return (
    <div className="site-config-row">
      <dt>{label}</dt>
      <dd>{value}</dd>
    </div>
  );
}
