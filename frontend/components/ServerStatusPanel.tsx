"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { ConfirmDialog } from "@/components/ConfirmDialog";
import { api, ApiError } from "@/lib/api";
import { formatBytes, formatPercent } from "@/lib/format";
import { useI18n } from "@/lib/i18n/context";
import type {
  SystemDockerDir,
  SystemProcess,
  SystemProcesses,
  SystemStatus,
  SystemUpdateInfo,
  SystemUpgradeJob,
} from "@/lib/types";

function diskTone(pct: number): string {
  if (pct >= 90) return "var(--danger, #b91c1c)";
  if (pct >= 80) return "var(--warn, #b45309)";
  return "var(--ok, #15803d)";
}

export function ServerStatusPanel() {
  const { t, formatDateTime } = useI18n();
  const [status, setStatus] = useState<SystemStatus | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [pruning, setPruning] = useState(false);
  const [pruneMsg, setPruneMsg] = useState<string | null>(null);

  const [showDocker, setShowDocker] = useState(false);
  const [dockerDirs, setDockerDirs] = useState<SystemDockerDir[] | null>(null);
  const [dockerDirsLoading, setDockerDirsLoading] = useState(false);
  const [dockerDirsError, setDockerDirsError] = useState<string | null>(null);

  const [showProcs, setShowProcs] = useState(false);
  const [procs, setProcs] = useState<SystemProcesses | null>(null);
  const [procsLoading, setProcsLoading] = useState(false);
  const [procsError, setProcsError] = useState<string | null>(null);

  const [updateInfo, setUpdateInfo] = useState<SystemUpdateInfo | null>(null);
  const [updateJob, setUpdateJob] = useState<SystemUpgradeJob | null>(null);
  const [updateConfirm, setUpdateConfirm] = useState(false);
  const [updateStarting, setUpdateStarting] = useState(false);
  const [updateMsg, setUpdateMsg] = useState<string | null>(null);
  const watchingUpgrade = useRef(false);

  const load = useCallback(async () => {
    try {
      const s = await api.getSystemStatus();
      setStatus(s);
      setError(null);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : t("system.loadFailed"));
    } finally {
      setLoading(false);
    }
  }, [t]);

  const loadUpdate = useCallback(async () => {
    try {
      const info = await api.getSystemUpdate();
      setUpdateInfo(info);
      if (info.upgrade_status === "running" || info.upgrade_status === "ok" || info.upgrade_status === "failed") {
        const job = await api.getSystemUpdateJob();
        setUpdateJob(job);
        if (info.upgrade_status === "running") {
          watchingUpgrade.current = true;
        }
      }
    } catch {
      /* optional on home */
    }
  }, []);

  useEffect(() => {
    load();
    void loadUpdate();
    const timer = setInterval(load, 30_000);
    return () => clearInterval(timer);
  }, [load, loadUpdate]);

  useEffect(() => {
    if (!watchingUpgrade.current && updateInfo?.upgrade_status !== "running") {
      return;
    }
    const timer = setInterval(async () => {
      try {
        const job = await api.getSystemUpdateJob();
        setUpdateJob(job);
        if (job.status === "ok") {
          watchingUpgrade.current = false;
          setUpdateMsg(t("system.updateDone"));
          void loadUpdate();
        } else if (job.status === "failed") {
          watchingUpgrade.current = false;
          setUpdateMsg(t("system.updateFailed"));
          void loadUpdate();
        } else if (job.status === "running") {
          watchingUpgrade.current = true;
        }
      } catch {
        // API may be down mid-upgrade — keep polling.
      }
    }, 3000);
    return () => clearInterval(timer);
  }, [updateInfo?.upgrade_status, t, loadUpdate]);

  const loadProcesses = useCallback(async () => {
    setProcsLoading(true);
    setProcsError(null);
    try {
      const p = await api.getSystemProcesses();
      setProcs(p);
    } catch (e) {
      setProcsError(e instanceof ApiError ? e.message : t("system.loadFailed"));
    } finally {
      setProcsLoading(false);
    }
  }, [t]);

  const loadDockerDirs = useCallback(async () => {
    setDockerDirsLoading(true);
    setDockerDirsError(null);
    try {
      const rows = await api.getSystemDockerDirs();
      setDockerDirs(rows);
    } catch (e) {
      setDockerDirsError(e instanceof ApiError ? e.message : t("system.loadFailed"));
    } finally {
      setDockerDirsLoading(false);
    }
  }, [t]);

  const toggleProcesses = () => {
    setShowProcs((open) => {
      const next = !open;
      if (next) {
        void loadProcesses();
      }
      return next;
    });
  };

  const toggleDocker = () => {
    setShowDocker((open) => {
      const next = !open;
      if (next && dockerDirs == null && !dockerDirsLoading) {
        void loadDockerDirs();
      }
      return next;
    });
  };

  const handlePrune = async () => {
    setPruning(true);
    setPruneMsg(null);
    setError(null);
    try {
      const r = await api.pruneDocker();
      setPruneMsg(
        t("system.pruneDone", {
          images: String(r.images_deleted),
          containers: String(r.containers_deleted),
          freed: formatBytes(r.space_reclaimed),
        }),
      );
      await load();
      if (showDocker) {
        setDockerDirs(null);
        void loadDockerDirs();
      }
    } catch (e) {
      setError(e instanceof ApiError ? e.message : t("system.pruneFailed"));
    } finally {
      setPruning(false);
    }
  };

  const handleStartUpdate = async () => {
    setUpdateConfirm(false);
    setUpdateStarting(true);
    setUpdateMsg(null);
    setError(null);
    try {
      await api.startSystemUpdate("latest");
      watchingUpgrade.current = true;
      setUpdateMsg(t("system.updateStarted"));
      const job = await api.getSystemUpdateJob();
      setUpdateJob(job);
      void loadUpdate();
    } catch (e) {
      setError(e instanceof ApiError ? e.message : t("system.updateFailed"));
    } finally {
      setUpdateStarting(false);
    }
  };

  if (loading && !status) {
    return (
      <div className="card" style={{ marginBottom: "1.25rem" }}>
        <h2 style={{ margin: 0, fontSize: "1.1rem" }}>{t("system.title")}</h2>
        <p style={{ color: "var(--muted)", margin: "0.5rem 0 0" }}>{t("common.loading")}</p>
      </div>
    );
  }

  if (error && !status) {
    return (
      <div className="card" style={{ marginBottom: "1.25rem" }}>
        <h2 style={{ margin: 0, fontSize: "1.1rem" }}>{t("system.title")}</h2>
        <div className="alert alert-error" style={{ marginTop: "0.75rem" }}>
          {error}
        </div>
      </div>
    );
  }

  if (!status) return null;

  const disks = status.disk ?? [];
  const root = disks[0];
  const mem = status.memory;
  const docker = status.docker;
  const topImages = docker?.top_images ?? [];
  const dockerTotal =
    (docker?.images_bytes ?? 0) +
    (docker?.build_cache_bytes ?? 0) +
    (docker?.volumes_bytes ?? 0) +
    (docker?.containers_bytes ?? 0);
  const memPct = mem?.used_percent ?? 0;

  return (
    <div className="card server-status" style={{ marginBottom: "1.25rem" }}>
      <div
        style={{
          display: "flex",
          justifyContent: "space-between",
          alignItems: "flex-start",
          gap: "0.75rem",
          flexWrap: "wrap",
        }}
      >
        <div>
          <h2 style={{ margin: 0, fontSize: "1.1rem" }}>{t("system.title")}</h2>
          <p style={{ color: "var(--muted)", fontSize: "0.8125rem", margin: "0.25rem 0 0" }}>
            {t("common.checked")}: {formatDateTime(status.checked_at)}
          </p>
        </div>
        <div style={{ display: "flex", gap: "0.5rem", flexWrap: "wrap" }}>
          <button type="button" className="btn btn-secondary" onClick={load} disabled={loading || pruning}>
            {t("common.refresh")}
          </button>
          <button
            type="button"
            className="btn btn-secondary"
            onClick={handlePrune}
            disabled={pruning}
            title={t("system.pruneHint")}
          >
            {pruning ? t("system.pruning") : t("system.pruneDocker")}
          </button>
        </div>
      </div>

      {error && (
        <div className="alert alert-error" style={{ marginTop: "0.75rem" }}>
          {error}
        </div>
      )}
      {pruneMsg && (
        <div className="alert alert-success" style={{ marginTop: "0.75rem" }}>
          {pruneMsg}
        </div>
      )}
      {updateMsg && (
        <div
          className={`alert ${updateJob?.status === "failed" ? "alert-error" : "alert-success"}`}
          style={{ marginTop: "0.75rem" }}
        >
          {updateMsg}
          {updateJob?.status === "ok" && (
            <button
              type="button"
              className="btn btn-secondary"
              style={{ marginLeft: "0.75rem", fontSize: "0.8125rem" }}
              onClick={() => window.location.reload()}
            >
              {t("system.reloadPage")}
            </button>
          )}
        </div>
      )}

      {updateInfo && (
        <div className="server-status-update">
          <div>
            <div className="server-status-label">{t("system.updateTitle")}</div>
            <div className="server-status-meta" style={{ marginTop: "0.15rem" }}>
              {t("system.updateCurrent")}:{" "}
              <strong>{updateInfo.current || "—"}</strong>
              {" → "}
              {t("system.updateLatest")}:{" "}
              <strong>
                {updateInfo.latest || t("system.updateUnknownLatest")}
              </strong>
              {" · "}
              <span
                style={{
                  color:
                    updateInfo.upgrade_status === "running" || updateJob?.status === "running"
                      ? "var(--warn, #b45309)"
                      : updateInfo.update_available
                        ? "var(--warn, #b45309)"
                        : updateInfo.latest
                          ? "var(--ok, #15803d)"
                          : "var(--muted)",
                  fontWeight: 600,
                }}
              >
                {updateInfo.upgrade_status === "running" || updateJob?.status === "running"
                  ? t("system.updateRunning")
                  : updateInfo.update_available && updateInfo.latest
                    ? t("system.updateAvailable", { version: updateInfo.latest })
                    : updateInfo.latest
                      ? t("system.updateUpToDate")
                      : t("system.updateUnknownLatest")}
              </span>
              {!updateInfo.can_update && updateInfo.reason ? (
                <span style={{ display: "block", marginTop: "0.2rem" }}>
                  {t("system.updateUnavailable")}: {updateInfo.reason}
                </span>
              ) : null}
            </div>
          </div>
          <div style={{ display: "flex", gap: "0.5rem", flexWrap: "wrap" }}>
            <button
              type="button"
              className="btn btn-secondary"
              style={{ fontSize: "0.8125rem" }}
              onClick={() => void loadUpdate()}
              disabled={updateStarting}
            >
              {t("system.updateCheck")}
            </button>
            <button
              type="button"
              className="btn"
              style={{ fontSize: "0.8125rem" }}
              disabled={
                updateStarting ||
                !updateInfo.can_update ||
                !updateInfo.latest ||
                (!updateInfo.update_available && updateInfo.upgrade_status !== "failed") ||
                updateInfo.upgrade_status === "running" ||
                updateJob?.status === "running"
              }
              onClick={() => setUpdateConfirm(true)}
            >
              {updateStarting ||
              updateInfo.upgrade_status === "running" ||
              updateJob?.status === "running"
                ? t("system.updateRunning")
                : updateInfo.latest
                  ? t("system.updateTo", { version: updateInfo.latest })
                  : t("system.updateNow")}
            </button>
          </div>
        </div>
      )}

      {(updateJob?.log || updateJob?.status === "running") && (
        <details open={updateJob?.status === "running" || updateJob?.status === "failed"} style={{ marginTop: "0.75rem" }}>
          <summary style={{ cursor: "pointer", color: "var(--muted)", fontSize: "0.8125rem" }}>
            {t("system.updateLog")}
          </summary>
          <pre className="server-status-update-log">{updateJob?.log || t("system.loadingDetails")}</pre>
        </details>
      )}

      <div className="server-status-grid">
        {root && (
          <div>
            <div className="server-status-label">{t("system.disk")}</div>
            <div className="server-status-value" style={{ color: diskTone(root.used_percent) }}>
              {formatPercent(root.used_percent)}
            </div>
            <div className="server-status-meta">
              {formatBytes(root.available_bytes)} {t("system.free")} ·{" "}
              {formatBytes(root.used_bytes)} / {formatBytes(root.total_bytes)}
            </div>
            <div className="meter" aria-hidden>
              <div
                className="meter-fill"
                style={{
                  width: `${Math.min(100, root.used_percent)}%`,
                  background: diskTone(root.used_percent),
                }}
              />
            </div>
          </div>
        )}

        <div>
          <div className="server-status-label">{t("system.memory")}</div>
          <div className="server-status-value">{formatPercent(memPct)}</div>
          <div className="server-status-meta">
            {formatBytes(mem?.available_bytes ?? 0)} {t("system.free")} ·{" "}
            {formatBytes(mem?.used_bytes ?? 0)} / {formatBytes(mem?.total_bytes ?? 0)}
          </div>
          <div className="meter" aria-hidden>
            <div
              className="meter-fill"
              style={{ width: `${Math.min(100, memPct)}%` }}
            />
          </div>
        </div>

        <div>
          <div className="server-status-label">{t("system.docker")}</div>
          <div className="server-status-value">{formatBytes(dockerTotal)}</div>
          <div className="server-status-meta">
            {t("system.images")}: {formatBytes(docker?.images_bytes)} (
            {docker?.image_count ?? 0}
            {(docker?.unused_image_count ?? 0) > 0
              ? `, ${t("system.unusedImages", { n: String(docker.unused_image_count) })}`
              : ""}
            )
            <br />
            {t("system.volumes")}: {formatBytes(docker?.volumes_bytes)} ·{" "}
            {t("system.buildCache")}: {formatBytes(docker?.build_cache_bytes)}
            {(docker?.reclaimable_bytes ?? 0) > 0 ? (
              <>
                <br />
                {t("system.reclaimable")}: {formatBytes(docker.reclaimable_bytes)}
              </>
            ) : null}
          </div>
        </div>
      </div>

      <div className="server-status-toggles">
        <button
          type="button"
          className="btn btn-secondary"
          style={{ fontSize: "0.8125rem" }}
          onClick={toggleDocker}
          aria-expanded={showDocker}
        >
          {showDocker ? t("system.hideDockerDetails") : t("system.showDockerDetails")}
        </button>
        <button
          type="button"
          className="btn btn-secondary"
          style={{ fontSize: "0.8125rem" }}
          onClick={toggleProcesses}
          aria-expanded={showProcs}
        >
          {showProcs ? t("system.hideProcesses") : t("system.showProcesses")}
        </button>
      </div>

      {showDocker && (
        <div className="server-status-procs" style={{ marginTop: "1rem" }}>
          {topImages.length > 0 && (
            <div>
              <h3 className="server-status-label">{t("system.topImages")}</h3>
              <p style={{ color: "var(--muted)", fontSize: "0.75rem", margin: "0 0 0.5rem" }}>
                {t("system.topImagesHint")}
              </p>
              <div className="table-wrap">
                <table className="table table-compact">
                  <thead>
                    <tr>
                      <th>{t("system.image")}</th>
                      <th>{t("system.size")}</th>
                      <th>{t("system.status")}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {topImages.map((img) => (
                      <tr key={img.id}>
                        <td className="cmd-cell" title={img.tags.join(", ")}>
                          {img.tags[0] || img.id}
                        </td>
                        <td>{formatBytes(img.size_bytes)}</td>
                        <td>
                          {img.in_use
                            ? t("system.inUse")
                            : img.dangling
                              ? t("system.dangling")
                              : t("system.unused")}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          )}

          <div>
            <h3 className="server-status-label">{t("system.dockerDirs")}</h3>
            <p style={{ color: "var(--muted)", fontSize: "0.75rem", margin: "0 0 0.5rem" }}>
              {t("system.dockerDirsHint")}
            </p>
            {dockerDirsLoading && !dockerDirs ? (
              <p style={{ color: "var(--muted)", fontSize: "0.8125rem" }}>
                {t("system.loadingDetails")}
              </p>
            ) : dockerDirsError ? (
              <div className="alert alert-error">{dockerDirsError}</div>
            ) : dockerDirs && dockerDirs.length > 0 ? (
              <div className="table-wrap">
                <table className="table table-compact">
                  <thead>
                    <tr>
                      <th>{t("system.path")}</th>
                      <th>{t("system.size")}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {dockerDirs.map((d) => (
                      <tr key={d.path}>
                        <td className="cmd-cell" title={d.path}>
                          {d.path.replace("/var/lib/docker/", "")}
                        </td>
                        <td>{formatBytes(d.size_bytes)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            ) : (
              <p style={{ color: "var(--muted)", fontSize: "0.8125rem" }}>—</p>
            )}
          </div>
        </div>
      )}

      {showProcs && (
        <div style={{ marginTop: "1rem" }}>
          {procsLoading && !procs ? (
            <p style={{ color: "var(--muted)", fontSize: "0.8125rem" }}>
              {t("system.loadingDetails")}
            </p>
          ) : procsError ? (
            <div className="alert alert-error">{procsError}</div>
          ) : (
            <div className="server-status-procs" style={{ marginTop: 0 }}>
              <div>
                <div
                  style={{
                    display: "flex",
                    justifyContent: "space-between",
                    alignItems: "center",
                    gap: "0.5rem",
                    marginBottom: "0.35rem",
                  }}
                >
                  <h3 className="server-status-label" style={{ margin: 0 }}>
                    {t("system.topCpu")}
                  </h3>
                  <button
                    type="button"
                    className="btn btn-secondary"
                    style={{ fontSize: "0.75rem", padding: "0.25rem 0.5rem" }}
                    onClick={() => void loadProcesses()}
                    disabled={procsLoading}
                  >
                    {t("common.refresh")}
                  </button>
                </div>
                <ProcessTable rows={procs?.top_cpu} empty={t("system.noProcesses")} />
              </div>
              <div>
                <h3 className="server-status-label">{t("system.topMem")}</h3>
                <ProcessTable rows={procs?.top_mem} empty={t("system.noProcesses")} mem />
              </div>
            </div>
          )}
        </div>
      )}

      <ConfirmDialog
        open={updateConfirm}
        title={t("system.updateTitle")}
        message={
          updateInfo?.latest
            ? t("system.updateConfirm", { version: updateInfo.latest })
            : t("system.updateConfirmLatest")
        }
        confirmLabel={
          updateInfo?.latest
            ? t("system.updateTo", { version: updateInfo.latest })
            : t("system.updateNow")
        }
        busy={updateStarting}
        onConfirm={() => void handleStartUpdate()}
        onCancel={() => setUpdateConfirm(false)}
      />
    </div>
  );
}

function ProcessTable({
  rows,
  empty,
  mem,
}: {
  rows: SystemProcess[] | undefined;
  empty: string;
  mem?: boolean;
}) {
  if (!rows?.length) {
    return <p style={{ color: "var(--muted)", fontSize: "0.8125rem" }}>{empty}</p>;
  }
  return (
    <div className="table-wrap">
      <table className="table table-compact">
        <thead>
          <tr>
            <th>PID</th>
            <th>{mem ? "MEM" : "CPU"}</th>
            <th>RSS</th>
            <th>CMD</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((p) => (
            <tr key={`${p.pid}-${p.command}`}>
              <td>{p.pid}</td>
              <td>{mem ? formatPercent(p.mem_percent) : formatPercent(p.cpu_percent)}</td>
              <td>{formatBytes(p.rss_bytes)}</td>
              <td className="cmd-cell" title={p.command}>
                {p.command}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
