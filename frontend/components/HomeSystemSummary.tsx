"use client";

import Link from "next/link";
import { useCallback, useEffect, useState } from "react";
import { api } from "@/lib/api";
import { formatBytes, formatPercent } from "@/lib/format";
import { useI18n } from "@/lib/i18n/context";
import type { BillingAccount, PgHealth, SystemHostInfo, SystemStatus } from "@/lib/types";

export function HomeSystemSummary() {
  const { t } = useI18n();
  const [status, setStatus] = useState<SystemStatus | null>(null);
  const [host, setHost] = useState<SystemHostInfo | null>(null);
  const [billing, setBilling] = useState<BillingAccount[]>([]);
  const [postgres, setPostgres] = useState<PgHealth[]>([]);

  const load = useCallback(async () => {
    const [nextStatus, nextHost, nextBilling, nextPostgres] = await Promise.all([
      api.getSystemStatus().catch(() => null),
      api.getSystemHost().catch(() => null),
      api.listBillingAccounts().catch(() => []),
      api.listPgHealth().catch(() => []),
    ]);
    setStatus(nextStatus);
    setHost(nextHost);
    setBilling(nextBilling);
    setPostgres(nextPostgres);
  }, []);

  useEffect(() => {
    void load();
    const timer = window.setInterval(() => void load(), 30_000);
    return () => window.clearInterval(timer);
  }, [load]);

  const disk = status?.disk?.[0];
  const memory = status?.memory;
  const minDays = billing.reduce<number | null>((min, account) => {
    if (account.days_left == null) return min;
    return min == null ? account.days_left : Math.min(min, account.days_left);
  }, null);
  const billingWarning = billing.some(
    (account) => account.days_left != null && account.days_left <= account.alert_days,
  );
  const pgHealthy = postgres.filter((row) => row.overall === "healthy").length;
  const dockerBytes = status
    ? status.docker.images_bytes + status.docker.containers_bytes + status.docker.volumes_bytes + status.docker.build_cache_bytes
    : null;

  return (
    <section className="home-system-summary" aria-label={t("sites.systemSummary")}>
      <div className="home-system-primary">
        <div className="home-system-server">
          <span className="home-system-dot" aria-hidden />
          <strong>{host?.hostname || host?.ip || t("system.title")}</strong>
          {host?.hostname && host.ip && <span>{host.ip}</span>}
        </div>
        <Link href="/payments" className={billingWarning ? "home-system-alert" : ""}>
          <span>{t("sites.billingShort")}</span>
          <strong>{minDays == null ? t("common.emDash") : t("sites.daysShort", { count: minDays })}</strong>
        </Link>
      </div>
      <div className="home-system-metrics">
        <Metric label={t("system.disk")} value={disk ? formatPercent(disk.used_percent) : "—"} meta={disk ? t("sites.freeSpace", { value: formatBytes(disk.available_bytes) }) : undefined} warn={Boolean(disk && disk.used_percent >= 80)} />
        <Metric label={t("system.memory")} value={memory ? formatPercent(memory.used_percent) : "—"} meta={memory ? t("sites.freeSpace", { value: formatBytes(memory.available_bytes) }) : undefined} warn={Boolean(memory && memory.used_percent >= 80)} />
        <Metric label={t("system.docker")} value={dockerBytes == null ? "—" : formatBytes(dockerBytes)} meta={status ? t("sites.reclaimableSpace", { value: formatBytes(status.docker.reclaimable_bytes) }) : undefined} />
        <Metric label="PostgreSQL" value={postgres.length ? `${pgHealthy}/${postgres.length}` : "—"} meta={postgres.length ? t("sites.databasesHealthy") : undefined} warn={postgres.length > 0 && pgHealthy !== postgres.length} />
      </div>
    </section>
  );
}

function Metric({ label, value, meta, warn = false }: { label: string; value: string; meta?: string; warn?: boolean }) {
  return (
    <div className={warn ? "home-system-metric home-system-metric-warn" : "home-system-metric"}>
      <span>{label}</span>
      <strong>{value}</strong>
      {meta && <small>{meta}</small>}
    </div>
  );
}
