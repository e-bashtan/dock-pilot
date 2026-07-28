"use client";

import { useEffect, useRef, useState } from "react";
import { useI18n } from "@/lib/i18n/context";
import { useLogViewerScroll } from "@/lib/use-log-viewer-scroll";

export type BackupJobLogLine = {
  id: string;
  level: string;
  message: string;
  at: string;
};

/** Append one log line without Strict Mode double-updater duplicates. */
export function appendBackupLog(
  logsRef: { current: BackupJobLogLine[] },
  setLogs: (logs: BackupJobLogLine[]) => void,
  level: string,
  message: string,
  at = new Date().toISOString(),
) {
  logsRef.current = [
    ...logsRef.current,
    {
      id: crypto.randomUUID(),
      level,
      message,
      at,
    },
  ];
  setLogs(logsRef.current);
}

export function resetBackupLogs(
  logsRef: { current: BackupJobLogLine[] },
  setLogs: (logs: BackupJobLogLine[]) => void,
) {
  logsRef.current = [];
  setLogs([]);
}

export async function consumeFetchSSE(
  res: Response,
  onLog: (level: string, message: string, at: string) => void,
  signal?: AbortSignal,
): Promise<string> {
  if (!res.ok || !res.body) {
    let message = res.statusText || `HTTP ${res.status}`;
    try {
      const body = await res.json();
      if (body?.error) message = body.error;
    } catch {
      /* ignore */
    }
    throw new Error(message);
  }

  const reader = res.body.getReader();
  const decoder = new TextDecoder();
  let buf = "";
  let eventName = "message";
  let status = "succeeded";

  const flushBlock = (block: string) => {
    const lines = block.split("\n");
    let data = "";
    for (const line of lines) {
      if (line.startsWith("event:")) {
        eventName = line.slice(6).trim();
      } else if (line.startsWith("data:")) {
        data += (data ? "\n" : "") + line.slice(5).trimStart();
      }
    }
    if (!data) return;
    if (eventName === "log") {
      try {
        const parsed = JSON.parse(data) as {
          level?: string;
          message?: string;
          at?: string;
        };
        onLog(
          parsed.level ?? "info",
          parsed.message ?? "",
          parsed.at ?? new Date().toISOString(),
        );
      } catch {
        /* ignore */
      }
    } else if (eventName === "done") {
      try {
        const parsed = JSON.parse(data) as { status?: string };
        status = parsed.status ?? "succeeded";
      } catch {
        status = "failed";
      }
    }
    eventName = "message";
  };

  try {
    while (true) {
      if (signal?.aborted) {
        await reader.cancel();
        throw new DOMException("Aborted", "AbortError");
      }
      const { done, value } = await reader.read();
      if (done) break;
      buf += decoder.decode(value, { stream: true });
      let idx: number;
      while ((idx = buf.indexOf("\n\n")) >= 0) {
        const block = buf.slice(0, idx);
        buf = buf.slice(idx + 2);
        flushBlock(block);
      }
    }
    if (buf.trim()) flushBlock(buf);
    return status;
  } finally {
    reader.releaseLock();
  }
}

export function BackupJobLog({
  title,
  openStream,
  openFetchStream,
  logs: controlledLogs,
  status: controlledStatus,
  embedded,
  onFinished,
}: {
  title: string;
  openStream?: () => EventSource;
  /** Prefer calling restore from the submit handler; use controlled logs instead. */
  openFetchStream?: (signal: AbortSignal) => Promise<Response>;
  logs?: BackupJobLogLine[];
  status?: string;
  embedded?: boolean;
  onFinished?: (status: string) => void;
}) {
  const { t, formatTime } = useI18n();
  const [internalLogs, setInternalLogs] = useState<BackupJobLogLine[]>([]);
  const [internalStatus, setInternalStatus] = useState("running");
  const viewerRef = useRef<HTMLDivElement>(null);
  const logsRef = useRef<BackupJobLogLine[]>([]);
  const onFinishedRef = useRef(onFinished);
  onFinishedRef.current = onFinished;

  const isControlled = controlledLogs !== undefined;
  const logs = isControlled ? controlledLogs : internalLogs;
  const status = isControlled ? (controlledStatus ?? "running") : internalStatus;

  useEffect(() => {
    if (isControlled) return;

    resetBackupLogs(logsRef, setInternalLogs);
    setInternalStatus("running");
    let closed = false;
    const ac = new AbortController();

    const pushLog = (level: string, message: string, at: string) => {
      if (closed) return;
      appendBackupLog(logsRef, setInternalLogs, level, message, at);
    };

    const finish = (next: string) => {
      if (closed) return;
      closed = true;
      setInternalStatus(next);
      onFinishedRef.current?.(next);
    };

    if (openFetchStream) {
      void (async () => {
        try {
          const res = await openFetchStream(ac.signal);
          if (closed) return;
          const next = await consumeFetchSSE(res, pushLog, ac.signal);
          finish(next);
        } catch (e) {
          if (closed || ac.signal.aborted) return;
          if (e instanceof DOMException && e.name === "AbortError") return;
          pushLog(
            "error",
            e instanceof Error ? e.message : "restore failed",
            new Date().toISOString(),
          );
          finish("failed");
        }
      })();
      return () => {
        closed = true;
        ac.abort();
      };
    }

    if (!openStream) return;

    const es = openStream();
    es.addEventListener("log", (ev) => {
      if (closed) return;
      try {
        const data = JSON.parse((ev as MessageEvent).data) as {
          level?: string;
          message?: string;
          at?: string;
        };
        pushLog(
          data.level ?? "info",
          data.message ?? "",
          data.at ?? new Date().toISOString(),
        );
      } catch {
        /* ignore */
      }
    });
    es.addEventListener("done", (ev) => {
      try {
        const data = JSON.parse((ev as MessageEvent).data) as { status?: string };
        finish(data.status ?? "succeeded");
      } catch {
        finish("failed");
      }
      es.close();
    });
    es.onerror = () => {
      if (closed) return;
      finish("failed");
      es.close();
    };
    return () => {
      closed = true;
      es.close();
    };
    // session remount via key=
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isControlled]);

  useLogViewerScroll(viewerRef, logs.length);

  const statusKey = status.toLowerCase();
  const statusLabel =
    statusKey === "running" ||
    statusKey === "succeeded" ||
    statusKey === "failed"
      ? t(`status.${statusKey}`)
      : status;

  const body = (
    <>
      <p style={{ marginBottom: "0.75rem", marginTop: embedded ? "1rem" : 0 }}>
        {title}: <strong>{statusLabel}</strong>
      </p>
      <div className="log-viewer" ref={viewerRef}>
        {logs.length === 0 && (
          <span style={{ color: "var(--muted)" }}>{t("backups.waitingRestoreLog")}</span>
        )}
        {logs.map((log) => (
          <div key={log.id} className={`log-line-${log.level}`}>
            [{formatTime(log.at)}] {log.message}
          </div>
        ))}
      </div>
    </>
  );

  if (embedded) return <div>{body}</div>;
  return <div className="card" style={{ marginBottom: "1.25rem" }}>{body}</div>;
}
