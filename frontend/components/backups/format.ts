export function formatBytes(n: number): string {
  if (!n || n < 0) return "—";
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  return `${(n / (1024 * 1024)).toFixed(1)} MB`;
}

export function isErrorStatusValue(status: string): boolean {
  const s = (status || "").toLowerCase().trim();
  if (!s) return false;
  if (s === "ok" || s === "succeeded" || s === "success") return false;
  if (s === "running" || s === "pending" || s === "in_progress") return false;
  if (s === "cancelled" || s === "canceled" || s === "partial") return false;
  return s === "failed" || s === "error" || s.length > 40 || s.includes(":");
}

export function statusBadgeClass(status: string): string {
  const s = status.toLowerCase();
  if (s === "succeeded" || s === "success" || s === "ok") return "badge-succeeded";
  if (s === "failed" || s === "error" || isErrorStatusValue(status)) return "badge-failed";
  if (s === "running" || s === "pending" || s === "in_progress") return "badge-running";
  if (s === "partial" || s === "partially_succeeded") return "badge-pending";
  return "badge";
}

/**
 * Computes next daily run time in user timezone given hour, minute, and timezone.
 * Returns ISO string or null if cannot compute.
 */
export function nextDailyRun(hour: number, minute: number, tz: string): string | null {
  try {
    const now = new Date();
    const formatter = new Intl.DateTimeFormat("en-US", {
      timeZone: tz,
      year: "numeric",
      month: "2-digit",
      day: "2-digit",
      hour: "2-digit",
      minute: "2-digit",
      second: "2-digit",
      hour12: false,
    });
    const parts = formatter.formatToParts(now);
    const get = (type: string) => parts.find((p) => p.type === type)?.value ?? "0";
    const y = Number(get("year"));
    const m = Number(get("month"));
    const d = Number(get("day"));
    const hh = Number(get("hour"));
    const mm = Number(get("minute"));

    let targetDay = d;
    if (hh > hour || (hh === hour && mm >= minute)) {
      targetDay = d + 1;
    }

    const targetStr = `${y}-${String(m).padStart(2, "0")}-${String(targetDay).padStart(2, "0")}T${String(hour).padStart(2, "0")}:${String(minute).padStart(2, "0")}:00`;
    const targetDate = new Date(
      new Date(targetStr).toLocaleString("en-US", { timeZone: tz }),
    );
    return targetDate.toISOString();
  } catch {
    return null;
  }
}
