"use client";

import Link from "next/link";
import { useCallback, useEffect, useState } from "react";
import { HealthBadge } from "@/components/HealthBadge";
import { api } from "@/lib/api";
import { useI18n } from "@/lib/i18n/context";
import type { PgHealth } from "@/lib/types";

/** Compact Postgres health strip for the sites monitoring overview. */
export function PostgresHealthSummary({ autoRefreshMs = 30_000 }: { autoRefreshMs?: number }) {
  const { t } = useI18n();
  const [rows, setRows] = useState<PgHealth[] | null>(null);

  const load = useCallback(async () => {
    try {
      const list = await api.listPgHealth();
      setRows(list);
    } catch {
      /* optional on overview */
    }
  }, []);

  useEffect(() => {
    void load();
    if (!autoRefreshMs) return;
    const timer = setInterval(() => void load(), autoRefreshMs);
    return () => clearInterval(timer);
  }, [load, autoRefreshMs]);

  if (!rows || rows.length === 0) {
    return null;
  }

  return (
    <div className="card" style={{ marginBottom: "1rem" }}>
      <div
        style={{
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
          gap: "1rem",
          flexWrap: "wrap",
        }}
      >
        <div>
          <strong>{t("databases.title")}</strong>
          <div className="muted" style={{ fontSize: "0.875rem", marginTop: "0.25rem" }}>
            {rows.map((h) => (
              <span key={h.instance_id} style={{ marginRight: "0.75rem" }}>
                <HealthBadge overall={h.overall} />{" "}
                {h.message}
              </span>
            ))}
          </div>
        </div>
        <Link href="/databases" className="btn btn-secondary">
          {t("databases.open")}
        </Link>
      </div>
    </div>
  );
}
