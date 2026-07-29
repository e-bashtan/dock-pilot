"use client";

import Link from "next/link";
import { useCallback, useEffect, useState } from "react";
import { api, ApiError } from "@/lib/api";
import { useI18n } from "@/lib/i18n/context";
import type { BillingAccount } from "@/lib/types";

function toneFor(daysLeft: number | undefined, alertDays: number): string {
  if (daysLeft == null) return "var(--muted)";
  if (daysLeft <= 0) return "var(--danger, #b91c1c)";
  if (daysLeft <= alertDays) return "var(--warn, #b45309)";
  return "var(--ok, #15803d)";
}

export function BillingExpiryPanel() {
  const { t, formatDateTime } = useI18n();
  const [accounts, setAccounts] = useState<BillingAccount[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  const load = useCallback(async () => {
    try {
      const rows = await api.listBillingAccounts();
      setAccounts(rows);
      setError(null);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : t("payments.loadFailed"));
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    void load();
    const timer = setInterval(load, 60_000);
    return () => clearInterval(timer);
  }, [load]);

  if (loading && accounts.length === 0 && !error) {
    return null;
  }

  return (
    <div className="card" style={{ marginBottom: "1.25rem" }}>
      <div
        style={{
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
          gap: "0.75rem",
          flexWrap: "wrap",
        }}
      >
        <h2 style={{ margin: 0, fontSize: "1.1rem" }}>{t("payments.homeTitle")}</h2>
        <Link href="/payments" className="btn btn-secondary" style={{ fontSize: "0.85rem" }}>
          {t("payments.homeLink")}
        </Link>
      </div>

      {error && (
        <div className="alert alert-error" style={{ marginTop: "0.75rem" }}>
          {error}
        </div>
      )}

      {!error && accounts.length === 0 && (
        <p style={{ color: "var(--muted)", margin: "0.5rem 0 0" }}>
          {t("payments.homeNone")}
        </p>
      )}

      {accounts.length > 0 && (
        <ul style={{ listStyle: "none", margin: "0.75rem 0 0", padding: 0 }}>
          {accounts.map((a) => {
            const days = a.days_left;
            const label =
              days == null
                ? "—"
                : days <= a.alert_days
                  ? t("payments.homeExpiring")
                  : t("payments.homeOk");
            return (
              <li
                key={a.id}
                style={{
                  display: "flex",
                  flexWrap: "wrap",
                  gap: "0.5rem 1.25rem",
                  alignItems: "baseline",
                  padding: "0.4rem 0",
                  borderTop: "1px solid var(--border, #e5e7eb)",
                  fontSize: "0.9rem",
                }}
              >
                <span style={{ fontWeight: 600 }}>{a.server_ip}</span>
                <span style={{ color: "var(--muted)" }}>
                  {a.provider}
                  {a.name ? ` · ${a.name}` : ""}
                </span>
                <span style={{ color: toneFor(days, a.alert_days), fontWeight: 600 }}>
                  {a.expire_date
                    ? `${t("payments.expireDate")}: ${a.expire_date}`
                    : t("payments.expireDate") + ": —"}
                  {days != null ? ` · ${t("payments.daysLeft")}: ${days}` : ""}
                  {` · ${label}`}
                </span>
                {a.last_checked_at && (
                  <span style={{ color: "var(--muted)", fontSize: "0.8rem" }}>
                    {t("payments.lastChecked")}: {formatDateTime(a.last_checked_at)}
                  </span>
                )}
              </li>
            );
          })}
        </ul>
      )}
    </div>
  );
}
