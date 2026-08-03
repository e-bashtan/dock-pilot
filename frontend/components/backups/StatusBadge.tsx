"use client";

import { statusBadgeClass, isErrorStatusValue } from "./format";

export function StatusBadge({
  status,
  t,
}: {
  status: string;
  t?: (key: string) => string;
}) {
  if (!status) return null;
  const label = t ? friendlyStatus(status, t) : status;
  return <span className={`badge ${statusBadgeClass(status)}`}>{label}</span>;
}

function friendlyStatus(status: string, t: (key: string) => string): string {
  const s = status.toLowerCase().trim();
  if (s === "ok" || s === "succeeded" || s === "success") {
    return t("backups.statusSuccess");
  }
  if (s === "running" || s === "pending" || s === "in_progress") {
    return t("backups.statusRunning");
  }
  if (s === "cancelled" || s === "canceled") {
    return t("backups.statusCancelled");
  }
  if (s === "partial" || s === "partially_succeeded") {
    return t("backups.statusPartial");
  }
  if (s === "failed" || s === "error" || isErrorStatusValue(status)) {
    return t("backups.statusError");
  }
  // Short unknown tokens stay as-is; long backend blobs become "Error"
  if (status.length > 40 || status.includes(":") || status.includes(" ")) {
    return t("backups.statusError");
  }
  return status;
}
