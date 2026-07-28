"use client";

import { useI18n } from "@/lib/i18n/context";

export function StatusBadge({ status }: { status: string }) {
  const { t } = useI18n();
  const key = status.toLowerCase();
  const known =
    key === "active" ||
    key === "stopped" ||
    key === "pending" ||
    key === "running" ||
    key === "succeeded" ||
    key === "success" ||
    key === "failed" ||
    key === "cancelled" ||
    key === "draft" ||
    key === "deploying" ||
    key === "error";
  const label = known
    ? key === "success"
      ? t("status.succeeded")
      : t(`status.${key}`)
    : status;
  const badgeClass = key === "success" ? "succeeded" : key;
  return <span className={`badge badge-${badgeClass}`}>{label}</span>;
}
