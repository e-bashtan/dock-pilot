"use client";

import { useEffect, useRef, useState } from "react";
import { api } from "@/lib/api";
import { useI18n } from "@/lib/i18n/context";
import { useLogViewerScroll } from "@/lib/use-log-viewer-scroll";

type DeployLogLine = {
  id: number;
  level: string;
  message: string;
  at: string;
};

export function PostgresDeployLog({
  instanceId,
  onFinished,
}: {
  instanceId: string;
  onFinished?: (status: string) => void;
}) {
  const { t, formatTime } = useI18n();
  const [logs, setLogs] = useState<DeployLogLine[]>([]);
  const [status, setStatus] = useState("running");
  const viewerRef = useRef<HTMLDivElement>(null);
  const seqRef = useRef(0);
  const onFinishedRef = useRef(onFinished);
  onFinishedRef.current = onFinished;

  useEffect(() => {
    setLogs([]);
    setStatus("running");
    seqRef.current = 0;

    const es = api.streamPgDeploy(instanceId);

    es.addEventListener("log", (ev) => {
      try {
        const data = JSON.parse((ev as MessageEvent).data) as {
          level?: string;
          message?: string;
          at?: string;
        };
        seqRef.current += 1;
        setLogs((prev) => [
          ...prev,
          {
            id: seqRef.current,
            level: data.level ?? "info",
            message: data.message ?? "",
            at: data.at ?? new Date().toISOString(),
          },
        ]);
      } catch {
        /* ignore parse errors */
      }
    });

    es.addEventListener("done", (ev) => {
      try {
        const data = JSON.parse((ev as MessageEvent).data) as { status?: string };
        const next = data.status ?? "succeeded";
        setStatus(next);
        onFinishedRef.current?.(next);
      } catch {
        onFinishedRef.current?.("failed");
      }
      es.close();
    });

    es.onerror = () => {
      setStatus("failed");
      onFinishedRef.current?.("failed");
      es.close();
    };

    return () => es.close();
  }, [instanceId]);

  useLogViewerScroll(viewerRef, logs.length);

  const statusKey = status.toLowerCase();
  const statusLabel =
    statusKey === "active" ||
    statusKey === "running" ||
    statusKey === "succeeded" ||
    statusKey === "failed"
      ? t(`status.${statusKey}`)
      : status;

  return (
    <div style={{ marginBottom: "1.25rem" }}>
      <p style={{ marginBottom: "0.75rem" }}>
        {t("databases.deployLog")}: <strong>{statusLabel}</strong>
      </p>
      <div className="log-viewer" ref={viewerRef}>
        {logs.length === 0 && (
          <span style={{ color: "var(--muted)" }}>
            {t("databases.waitingDeployLog")}
          </span>
        )}
        {logs.map((log) => (
          <div key={log.id} className={`log-line-${log.level}`}>
            [{formatTime(log.at)}] {log.message}
          </div>
        ))}
      </div>
    </div>
  );
}
