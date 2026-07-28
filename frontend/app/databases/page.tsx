"use client";

import { useCallback, useEffect, useState } from "react";
import { PostgresManager } from "@/components/PostgresManager";
import { api, ApiError } from "@/lib/api";
import { useI18n } from "@/lib/i18n/context";
import type { PgInstance } from "@/lib/types";

function formatError(message: string, migrationHint: string): string {
  if (/pdb_instances|pg_instances|42P01|does not exist/i.test(message)) {
    return migrationHint;
  }
  return message;
}

export default function DatabasesPage() {
  const { t } = useI18n();
  const [instance, setInstance] = useState<PgInstance | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [creating, setCreating] = useState(false);

  const [name, setName] = useState("Postgres");
  const [image, setImage] = useState("postgres:16-alpine");
  const [adminUser, setAdminUser] = useState("postgres");
  const [adminPassword, setAdminPassword] = useState("");
  const [hostPort, setHostPort] = useState("");
  const [networkHost, setNetworkHost] = useState(false);

  const load = useCallback(async () => {
    try {
      const list = await api.listPgInstances();
      setInstance(list[0] ?? null);
      setError(null);
    } catch (e) {
      const msg = e instanceof ApiError ? e.message : t("databases.loadFailed");
      setError(formatError(msg, t("databases.migrationNeeded")));
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    void load();
  }, [load]);

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    setCreating(true);
    setError(null);
    try {
      const created = await api.createPgInstance({
        name: name.trim() || "Postgres",
        image: image.trim() || undefined,
        admin_user: adminUser.trim() || undefined,
        admin_password: adminPassword || undefined,
        docker_network_host: networkHost,
        ...(hostPort.trim() ? { host_port: Number(hostPort.trim()) } : {}),
      });
      setInstance(created);
    } catch (err) {
      const msg = err instanceof ApiError ? err.message : t("databases.loadFailed");
      setError(formatError(msg, t("databases.migrationNeeded")));
    } finally {
      setCreating(false);
    }
  };

  if (loading) {
    return (
      <div>
        <h1>{t("databases.title")}</h1>
        <p className="muted">{t("common.loading")}</p>
      </div>
    );
  }

  if (instance) {
    return (
      <PostgresManager
        instanceId={instance.id}
        onDeleted={() => {
          setInstance(null);
          void load();
        }}
      />
    );
  }

  return (
    <div>
      <div className="page-header">
        <div>
          <h1>{t("databases.title")}</h1>
          <p className="page-header-meta">{t("databases.subtitle")}</p>
        </div>
      </div>

      {error && <div className="alert alert-error">{error}</div>}

      <div className="card" style={{ marginBottom: "1rem" }}>
        <p style={{ margin: 0 }}>{t("databases.empty")}</p>
        <p className="muted" style={{ marginTop: "0.5rem", marginBottom: 0 }}>
          {t("databases.emptyHint")}
        </p>
      </div>

      <form onSubmit={handleCreate} className="card">
        <h2 className="section-title">{t("databases.createTitle")}</h2>
        <div className="form-grid">
          <div className="field">
            <label className="label" htmlFor="pg-name">
              {t("databases.name")}
            </label>
            <input
              id="pg-name"
              className="input"
              value={name}
              onChange={(e) => setName(e.target.value)}
            />
          </div>
          <div className="field">
            <label className="label" htmlFor="pg-image">
              {t("databases.image")}
            </label>
            <input
              id="pg-image"
              className="input"
              value={image}
              onChange={(e) => setImage(e.target.value)}
            />
          </div>
          <div className="field">
            <label className="label" htmlFor="pg-admin">
              {t("databases.adminUser")}
            </label>
            <input
              id="pg-admin"
              className="input"
              value={adminUser}
              onChange={(e) => setAdminUser(e.target.value)}
            />
          </div>
          <div className="field">
            <label className="label" htmlFor="pg-password">
              {t("databases.adminPassword")}
            </label>
            <input
              id="pg-password"
              className="input"
              type="password"
              autoComplete="new-password"
              value={adminPassword}
              onChange={(e) => setAdminPassword(e.target.value)}
              placeholder={t("databases.adminPasswordHint")}
            />
          </div>
          {!networkHost && (
            <div className="field">
              <label className="label" htmlFor="pg-port">
                {t("databases.hostPort")}
              </label>
              <input
                id="pg-port"
                className="input"
                value={hostPort}
                onChange={(e) => setHostPort(e.target.value)}
                placeholder={t("databases.hostPortHint")}
              />
            </div>
          )}
        </div>
        <div className="field" style={{ marginTop: "1rem" }}>
          <label className="label checkbox-row">
            <input
              type="checkbox"
              checked={networkHost}
              onChange={(e) => setNetworkHost(e.target.checked)}
            />
            <span>{t("databases.networkHost")}</span>
          </label>
        </div>
        <div className="form-actions">
          <button type="submit" className="btn" disabled={creating}>
            {creating ? t("common.loading") : t("databases.create")}
          </button>
        </div>
      </form>
    </div>
  );
}
