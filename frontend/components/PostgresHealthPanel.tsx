"use client";

import { useCallback, useEffect, useState } from "react";
import { HealthBadge } from "@/components/HealthBadge";
import { api, ApiError } from "@/lib/api";
import { useI18n } from "@/lib/i18n/context";
import type { PgHealth } from "@/lib/types";

export function PostgresHealthPanel({
  instanceId,
  autoRefreshMs = 30_000,
}: {
  instanceId: string;
  autoRefreshMs?: number;
}) {
  const { t, formatDateTime } = useI18n();
  const [health, setHealth] = useState<PgHealth | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    try {
      const h = await api.getPgHealth(instanceId);
      setHealth(h);
      setError(null);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : t("health.checkFailed"));
    } finally {
      setLoading(false);
    }
  }, [instanceId, t]);

  useEffect(() => {
    setLoading(true);
    void load();
    if (!autoRefreshMs) return;
    const timer = setInterval(() => void load(), autoRefreshMs);
    return () => clearInterval(timer);
  }, [load, autoRefreshMs]);

  if (loading && !health) {
    return (
      <div className="card">
        <h3>{t("health.title")}</h3>
        <p style={{ color: "var(--muted)", margin: 0 }}>{t("health.checking")}</p>
      </div>
    );
  }

  return (
    <div className="card">
      <div
        style={{
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
          marginBottom: "0.75rem",
        }}
      >
        <h3 style={{ margin: 0 }}>{t("health.title")}</h3>
        <button type="button" className="btn btn-secondary" onClick={() => void load()}>
          {t("common.refresh")}
        </button>
      </div>

      {error && <div className="alert alert-error">{error}</div>}

      {health && (
        <>
          <p style={{ margin: "0 0 0.75rem" }}>
            <HealthBadge overall={health.overall} />{" "}
            <span style={{ color: "var(--muted)", fontSize: "0.875rem" }}>
              {health.message}
            </span>
          </p>
          <dl style={{ margin: 0, fontSize: "0.875rem" }}>
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
            {typeof health.ready === "boolean" && (
              <HealthRow
                label={t("health.pgReady")}
                value={health.ready ? t("health.pgReadyOk") : t("health.pgReadyFail")}
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
    <div style={{ marginBottom: "0.35rem" }}>
      <span style={{ color: "var(--muted)" }}>{label}: </span>
      {value}
    </div>
  );
}
