"use client";

import { useI18n } from "@/lib/i18n/context";
import type { ServerNodeStatus } from "@/lib/types";

export function ServerStatusBadge({ status }: { status: ServerNodeStatus | string }) {
  const { t } = useI18n();
  const key = status.toLowerCase();
  const known = key === "online" || key === "warning" || key === "offline";
  const label = known ? t(`servers.status.${key}`) : status;
  const cls =
    key === "online"
      ? "badge-health-healthy"
      : key === "warning"
        ? "badge-health-degraded"
        : key === "offline"
          ? "badge-health-unhealthy"
          : "badge-health-unknown";
  return <span className={`badge ${cls}`}>{label}</span>;
}

export function ServerNodeBadges({
  role,
  connectionType,
}: {
  role: string;
  connectionType: string;
}) {
  const { t } = useI18n();
  const badges: string[] = [];
  if (role === "master" || connectionType === "local") {
    badges.push(t("servers.badgeMaster"));
  }
  if (connectionType === "barn" || connectionType === "dockpilot") {
    badges.push(t("servers.badgeBarn"));
  }
  if (connectionType === "agent") {
    badges.push(t("servers.badgeMonitored"));
  }
  if (badges.length === 0) return null;
  return (
    <span style={{ display: "inline-flex", gap: "0.35rem", flexWrap: "wrap" }}>
      {badges.map((label) => (
        <span key={label} className="badge badge-draft">
          {label}
        </span>
      ))}
    </span>
  );
}
