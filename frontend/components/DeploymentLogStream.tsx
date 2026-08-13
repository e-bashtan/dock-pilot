"use client";

import { useEffect, useRef, useState } from "react";
import { api } from "@/lib/api";
import { useI18n } from "@/lib/i18n/context";
import { useLogViewerScroll } from "@/lib/use-log-viewer-scroll";
import type { DeploymentLog } from "@/lib/types";

export function DeploymentLogStream({
  deploymentId,
  initialStatus,
}: {
  deploymentId: string;
  initialStatus?: string;
}) {
  const { t, formatTime } = useI18n();
  const [logs, setLogs] = useState<DeploymentLog[]>([]);
  const [status, setStatus] = useState(initialStatus ?? "pending");
  const viewerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    setLogs([]);
    setStatus(initialStatus ?? "pending");
    const es = api.streamDeploymentLogs(deploymentId);

    es.addEventListener("log", (ev) => {
      try {
        const data = JSON.parse((ev as MessageEvent).data) as DeploymentLog;
        setLogs((prev) => {
          if (prev.some((l) => l.id === data.id)) return prev;
          return [...prev, data];
        });
      } catch {
        /* ignore parse errors */
      }
    });

    es.addEventListener("done", (ev) => {
      try {
        const data = JSON.parse((ev as MessageEvent).data) as { status: string };
        setStatus(data.status);
      } catch {
        /* ignore */
      }
      es.close();
    });

    es.onerror = () => es.close();

    return () => es.close();
  }, [deploymentId]);

  useLogViewerScroll(viewerRef, logs.length);

  const statusKey = status.toLowerCase();
  const statusLabel =
    statusKey === "active" ||
    statusKey === "pending" ||
    statusKey === "running" ||
    statusKey === "succeeded" ||
    statusKey === "failed" ||
    statusKey === "cancelled"
      ? t(`status.${statusKey}`)
      : status;

  return (
    <div>
      <p style={{ marginBottom: "0.75rem" }}>
        {t("logs.status")}: <strong>{statusLabel}</strong>
      </p>
      <div className="log-viewer" ref={viewerRef}>
        {logs.length === 0 && (
          <span style={{ color: "var(--muted)" }}>{t("logs.waitingDeployment")}</span>
        )}
        {logs.map((log) => (
          <div key={log.id} className={`log-line-${log.level}`}>
            [{formatTime(log.created_at)}] {log.message}
          </div>
        ))}
      </div>
    </div>
  );
}
