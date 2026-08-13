"use client";

import Link from "next/link";
import { useParams } from "next/navigation";
import { Fragment, useCallback, useEffect, useState } from "react";
import { DeploymentLogStream } from "@/components/DeploymentLogStream";
import { SiteTabs } from "@/components/SiteTabs";
import { StatusBadge } from "@/components/StatusBadge";
import { api, ApiError } from "@/lib/api";
import { useI18n } from "@/lib/i18n/context";
import { useIsMobile } from "@/lib/use-is-mobile";
import type { Deployment, Site } from "@/lib/types";

export default function SiteDeploymentsPage() {
  const { id } = useParams<{ id: string }>();
  const { t, formatDateTime } = useI18n();
  const isMobile = useIsMobile();
  const [site, setSite] = useState<Site | null>(null);
  const [deployments, setDeployments] = useState<Deployment[]>([]);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [deploying, setDeploying] = useState(false);

  const load = useCallback(async () => {
    try {
      const [s, deps] = await Promise.all([
        api.getSite(id),
        api.listDeployments(id),
      ]);
      setSite(s);
      setDeployments(deps);
      if (!selectedId && deps[0]) setSelectedId(deps[0].id);
      setError(null);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : t("siteDeployments.loadFailed"));
    }
  }, [id, selectedId, t]);

  useEffect(() => {
    load();
    const timer = setInterval(load, 3000);
    return () => clearInterval(timer);
  }, [load]);

  const handleDeploy = async () => {
    setDeploying(true);
    try {
      const dep = await api.deploySite(id);
      setSelectedId(dep.id);
      await load();
    } catch (e) {
      setError(e instanceof ApiError ? e.message : t("site.deployFailed"));
    } finally {
      setDeploying(false);
    }
  };

  const selected = deployments.find((d) => d.id === selectedId);

  const logsPanel = selected ? (
    <>
      <h3 style={{ marginTop: 0 }}>{t("siteDeployments.deploymentLogs")}</h3>
      <p style={{ fontSize: "0.875rem", color: "var(--muted)", marginBottom: "0.75rem" }}>
        {t("common.id")}: <code className="deployments-log-id">{selected.id}</code>
      </p>
      <DeploymentLogStream
        key={selected.id}
        deploymentId={selected.id}
        initialStatus={selected.status}
      />
    </>
  ) : (
    <p style={{ color: "var(--muted)" }}>{t("siteDeployments.selectDeployment")}</p>
  );

  return (
    <div>
      <div
        style={{
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
        }}
      >
        <h1>{site?.name ?? t("siteDeployments.title")}</h1>
        <button
          type="button"
          className="btn"
          onClick={handleDeploy}
          disabled={deploying}
        >
          {deploying ? t("site.starting") : t("siteDeployments.newDeployment")}
        </button>
      </div>

      <SiteTabs siteId={id} active="deployments" />
      {error && <div className="alert alert-error">{error}</div>}

      <div className="deployments-layout">
        <div className="card deployments-list" style={{ padding: 0 }}>
          <table className="table">
            <thead>
              <tr>
                <th>{t("common.status")}</th>
                <th>{t("common.created")}</th>
              </tr>
            </thead>
            <tbody>
              {deployments.map((d) => (
                <Fragment key={d.id}>
                  <tr
                    onClick={() => setSelectedId(d.id)}
                    style={{
                      cursor: "pointer",
                      background:
                        d.id === selectedId ? "var(--surface-hover)" : undefined,
                    }}
                  >
                    <td>
                      <StatusBadge status={d.status} />
                      <div style={{ fontSize: "0.75rem", color: "var(--muted)" }}>
                        {d.message}
                      </div>
                    </td>
                    <td>{formatDateTime(d.created_at)}</td>
                  </tr>
                  {isMobile && d.id === selectedId && selected && (
                    <tr className="deployments-inline-logs">
                      <td colSpan={2}>{logsPanel}</td>
                    </tr>
                  )}
                </Fragment>
              ))}
            </tbody>
          </table>
          {deployments.length === 0 && (
            <p style={{ padding: "1rem", color: "var(--muted)" }}>
              {t("siteDeployments.noDeployments")}
            </p>
          )}
        </div>

        {!isMobile && (
          <div className="card deployments-side-logs">{logsPanel}</div>
        )}
      </div>

      <Link href={`/sites/${id}`} style={{ marginTop: "1rem", display: "inline-block" }}>
        {t("siteDeployments.backToOverview")}
      </Link>
    </div>
  );
}
