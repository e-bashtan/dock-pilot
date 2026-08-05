"use client";

import Link from "next/link";
import { useCallback, useEffect, useState } from "react";
import { api, ApiError } from "@/lib/api";
import { useI18n } from "@/lib/i18n/context";
import type { ServerEvent } from "@/lib/types";

export default function ServerEventsPage() {
  const { t, formatDateTime } = useI18n();
  const [events, setEvents] = useState<ServerEvent[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  const load = useCallback(async () => {
    try {
      const rows = await api.listServerEvents();
      setEvents(rows);
      setError(null);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : t("servers.eventsLoadFailed"));
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    void load();
    const timer = setInterval(() => void load(), 30_000);
    return () => clearInterval(timer);
  }, [load]);

  return (
    <div>
      <div className="page-header">
        <div>
          <h1>{t("servers.eventsTitle")}</h1>
          <p className="muted" style={{ margin: "0.35rem 0 0" }}>
            {t("servers.eventsSubtitle")}
          </p>
        </div>
        <Link href="/servers" className="btn btn-secondary">
          {t("nav.servers")}
        </Link>
      </div>

      {error && <div className="alert alert-error">{error}</div>}

      {loading ? (
        <p className="muted">{t("common.loading")}</p>
      ) : events.length === 0 ? (
        <div className="card">
          <p>{t("servers.eventsEmpty")}</p>
        </div>
      ) : (
        <div className="card" style={{ padding: 0, overflow: "hidden" }}>
          <div className="table-wrap">
            <table className="table">
              <thead>
                <tr>
                  <th>{t("servers.eventTime")}</th>
                  <th>{t("servers.eventSeverity")}</th>
                  <th>{t("servers.eventTitle")}</th>
                  <th className="col-hide-mobile">{t("servers.eventNode")}</th>
                  <th className="col-hide-mobile">{t("servers.eventType")}</th>
                </tr>
              </thead>
              <tbody>
                {events.map((ev) => (
                  <tr key={ev.id}>
                    <td>{formatDateTime(ev.occurred_at)}</td>
                    <td>
                      <span className={`badge badge-${ev.severity === "error" ? "failed" : ev.severity === "warning" ? "deploying" : "active"}`}>
                        {ev.severity}
                      </span>
                    </td>
                    <td>
                      <div>{ev.title}</div>
                      {ev.message && (
                        <div className="muted" style={{ fontSize: "0.8rem" }}>
                          {ev.message}
                        </div>
                      )}
                    </td>
                    <td className="col-hide-mobile">{ev.node_uid || t("common.emDash")}</td>
                    <td className="col-hide-mobile">{ev.event_type}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  );
}
