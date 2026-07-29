/** Format byte counts for disk / Docker usage display. */
export function formatBytes(n: number | undefined | null): string {
  const v = typeof n === "number" && Number.isFinite(n) ? Math.max(0, n) : 0;
  const units = ["B", "KiB", "MiB", "GiB", "TiB"];
  let x = v;
  let i = 0;
  while (x >= 1024 && i < units.length - 1) {
    x /= 1024;
    i++;
  }
  const digits = i === 0 ? 0 : x >= 10 ? 1 : 2;
  return `${x.toFixed(digits)} ${units[i]}`;
}

export function formatPercent(n: number | undefined | null): string {
  const v = typeof n === "number" && Number.isFinite(n) ? n : 0;
  return `${v.toFixed(1)}%`;
}

/** Format minor currency units (cents/kopecks) for display. */
export function formatMoneyMinor(
  minor: number | undefined | null,
  currency: string | undefined | null,
): string {
  const v = typeof minor === "number" && Number.isFinite(minor) ? minor : 0;
  const cur = (currency || "").trim() || "USD";
  const major = v / 100;
  try {
    return new Intl.NumberFormat(undefined, {
      style: "currency",
      currency: cur,
    }).format(major);
  } catch {
    return `${major.toFixed(2)} ${cur}`;
  }
}
