"use client";

import Link from "next/link";
import { useCallback, useEffect, useState } from "react";
import { api, ApiError } from "@/lib/api";
import { useI18n } from "@/lib/i18n/context";
import type { PgInstance } from "@/lib/types";

export default function DatabasesPage() {
  const { t } = useI18n();
  const [rows, setRows] = useState<PgInstance[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [creating, setCreating] = useState(false);
  const [name, setName] = useState("");
  const [slug, setSlug] = useState("");
  const [image, setImage] = useState("postgres:16-alpine");
  const [adminUser, setAdminUser] = useState("postgres");
  const [adminPassword, setAdminPassword] = useState("");
  const [hostPort, setHostPort] = useState("");
  const [networkHost, setNetworkHost] = useState(false);

  const load = useCallback(async () => {
    try {
      const list = await api.listPgInstances();
      setRows(list);
      setError(null);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : t("databases.loadFailed"));
    }
  }, [t]);

  useEffect(() => {
    load();
  }, [load]);

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    setCreating(true);
    setError(null);
    try {
      await api.createPgInstance({
        name: name.trim(),
        slug: slug.trim() || undefined,
        image: image.trim() || undefined,
        admin_user: adminUser.trim() || undefined,
        admin_password: adminPassword || undefined,
        docker_network_host: networkHost,
        ...(hostPort.trim()
          ? { host_port: Number(hostPort.trim()) }
          : {}),
      });
      setName("");
      setSlug("");
      setAdminPassword("");
      setHostPort("");
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("databases.loadFailed"));
    } finally {
      setCreating(false);
    }
  };

  return (
    <div className="page">
      <div className="page-header">
        <div>
          <h1>{t("databases.title")}</h1>
          <p className="muted">{t("databases.subtitle")}</p>
        </div>
      </div>

      {error && <p className="error">{error}</p>}

      <form className="card-form" onSubmit={handleCreate} style={{ marginBottom: "1.5rem" }}>
        <h2 style={{ marginTop: 0 }}>{t("databases.create")}</h2>
        <div className="form-grid">
          <label>
            {t("databases.name")}
            <input value={name} onChange={(e) => setName(e.target.value)} required />
          </label>
          <label>
            {t("databases.slug")}
            <input value={slug} onChange={(e) => setSlug(e.target.value)} placeholder="auto" />
          </label>
          <label>
            {t("databases.image")}
            <input value={image} onChange={(e) => setImage(e.target.value)} />
          </label>
          <label>
            {t("databases.adminUser")}
            <input value={adminUser} onChange={(e) => setAdminUser(e.target.value)} />
          </label>
          <label>
            {t("databases.adminPassword")}
            <input
              type="password"
              value={adminPassword}
              onChange={(e) => setAdminPassword(e.target.value)}
              placeholder={t("databases.adminPasswordHint")}
            />
          </label>
          {!networkHost && (
            <label>
              {t("databases.hostPort")}
              <input
                value={hostPort}
                onChange={(e) => setHostPort(e.target.value)}
                placeholder={t("databases.hostPortHint")}
              />
            </label>
          )}
          <label className="checkbox-row">
            <input
              type="checkbox"
              checked={networkHost}
              onChange={(e) => setNetworkHost(e.target.checked)}
            />
            {t("databases.networkHost")}
          </label>
        </div>
        <button type="submit" className="btn" disabled={creating}>
          {t("databases.create")}
        </button>
      </form>

      {rows.length === 0 ? (
        <p className="muted">{t("databases.empty")}</p>
      ) : (
        <div className="table-wrap">
          <table className="table">
            <thead>
              <tr>
                <th>{t("databases.name")}</th>
                <th>{t("databases.status")}</th>
                <th className="col-hide-mobile">{t("databases.port")}</th>
                <th>{t("databases.actions")}</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((row) => (
                <tr key={row.id}>
                  <td>
                    <Link href={`/databases/${row.id}`}>{row.name}</Link>
                    <div className="muted" style={{ fontSize: "0.85rem" }}>
                      {row.slug} · {row.image}
                    </div>
                  </td>
                  <td>
                    <span className="badge">{row.status}</span>
                    {row.message ? (
                      <div className="muted" style={{ fontSize: "0.8rem" }}>
                        {row.message}
                      </div>
                    ) : null}
                  </td>
                  <td className="col-hide-mobile">
                    {row.docker_network_host
                      ? "host"
                      : row.host_port ?? "—"}
                  </td>
                  <td>
                    <Link href={`/databases/${row.id}`} className="btn btn-secondary">
                      {t("databases.open")}
                    </Link>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
