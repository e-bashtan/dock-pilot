"use client";

import { useCallback, useEffect, useState } from "react";
import { ConfirmDialog } from "@/components/ConfirmDialog";
import { api, ApiError } from "@/lib/api";
import { useI18n } from "@/lib/i18n/context";
import type { BillingAccount } from "@/lib/types";

const DEFAULT_BILLMGR = "https://bill.planetahost.ru/billmgr";

type FormState = {
  provider: string;
  serverIp: string;
  login: string;
  password: string;
  billmgrUrl: string;
  alertDays: number;
  enabled: boolean;
};

const emptyForm = (): FormState => ({
  provider: "planetahost",
  serverIp: "",
  login: "",
  password: "",
  billmgrUrl: DEFAULT_BILLMGR,
  alertDays: 10,
  enabled: true,
});

export default function PaymentsPage() {
  const { t, formatDateTime } = useI18n();
  const [accounts, setAccounts] = useState<BillingAccount[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [formOpen, setFormOpen] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [form, setForm] = useState<FormState>(emptyForm);
  const [passwordSet, setPasswordSet] = useState(false);
  const [deleteId, setDeleteId] = useState<string | null>(null);

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
  }, [load]);

  const openCreate = () => {
    setEditingId(null);
    setForm(emptyForm());
    setPasswordSet(false);
    setFormOpen(true);
    setError(null);
  };

  const openEdit = (a: BillingAccount) => {
    setEditingId(a.id);
    setForm({
      provider: a.provider || "planetahost",
      serverIp: a.server_ip,
      login: a.login,
      password: "",
      billmgrUrl: a.billmgr_url || DEFAULT_BILLMGR,
      alertDays: a.alert_days || 10,
      enabled: a.enabled,
    });
    setPasswordSet(a.password_set);
    setFormOpen(true);
    setError(null);
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      if (editingId) {
        const body: {
          provider: string;
          server_ip: string;
          login: string;
          billmgr_url: string;
          alert_days: number;
          enabled: boolean;
          password?: string;
        } = {
          provider: form.provider,
          server_ip: form.serverIp.trim(),
          login: form.login.trim(),
          billmgr_url: form.billmgrUrl.trim(),
          alert_days: form.alertDays,
          enabled: form.enabled,
        };
        if (form.password.trim()) {
          body.password = form.password;
        }
        await api.updateBillingAccount(editingId, body);
      } else {
        if (!form.password.trim()) {
          throw new ApiError(t("payments.password"), 400);
        }
        await api.createBillingAccount({
          provider: form.provider,
          server_ip: form.serverIp.trim(),
          login: form.login.trim(),
          password: form.password,
          billmgr_url: form.billmgrUrl.trim() || undefined,
          alert_days: form.alertDays,
          enabled: form.enabled,
        });
      }
      setFormOpen(false);
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("payments.loadFailed"));
    } finally {
      setBusy(false);
    }
  };

  const handleRefresh = async (id: string) => {
    setBusy(true);
    setError(null);
    try {
      await api.refreshBillingAccount(id);
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("payments.loadFailed"));
    } finally {
      setBusy(false);
    }
  };

  const handleDelete = async () => {
    if (!deleteId) return;
    setBusy(true);
    setError(null);
    try {
      await api.deleteBillingAccount(deleteId);
      setDeleteId(null);
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("payments.loadFailed"));
    } finally {
      setBusy(false);
    }
  };

  const deleteTarget = accounts.find((a) => a.id === deleteId);

  return (
    <div>
      <div className="page-header">
        <div>
          <h1>{t("payments.title")}</h1>
          <p style={{ color: "var(--muted)", fontSize: "0.875rem", margin: "0.35rem 0 0" }}>
            {t("payments.subtitle")}
          </p>
        </div>
        <button type="button" className="btn" onClick={openCreate} disabled={busy}>
          {t("payments.add")}
        </button>
      </div>

      {error && <div className="alert alert-error">{error}</div>}

      {formOpen && (
        <form onSubmit={handleSubmit} className="card" style={{ marginBottom: "1.25rem" }}>
          <h2 style={{ marginTop: 0, fontSize: "1.05rem" }}>
            {editingId ? t("payments.edit") : t("payments.add")}
          </h2>

          <div className="form-grid">
            <div className="field">
              <label className="label" htmlFor="billing-provider">
                {t("payments.provider")}
              </label>
              <select
                id="billing-provider"
                className="input"
                value={form.provider}
                onChange={(e) => setForm((f) => ({ ...f, provider: e.target.value }))}
                required
              >
                <option value="planetahost">{t("payments.providerPlanetahost")}</option>
              </select>
            </div>

            <div className="field">
              <label className="label" htmlFor="billing-ip">
                {t("payments.serverIp")}
              </label>
              <input
                id="billing-ip"
                className="input"
                type="text"
                value={form.serverIp}
                onChange={(e) => setForm((f) => ({ ...f, serverIp: e.target.value }))}
                placeholder="1.2.3.4"
                required
                autoComplete="off"
              />
            </div>

            <div className="field">
              <label className="label" htmlFor="billing-login">
                {t("payments.login")}
              </label>
              <input
                id="billing-login"
                className="input"
                type="text"
                value={form.login}
                onChange={(e) => setForm((f) => ({ ...f, login: e.target.value }))}
                required
                autoComplete="username"
              />
            </div>

            <div className="field">
              <label className="label" htmlFor="billing-password">
                {t("payments.password")}
              </label>
              <input
                id="billing-password"
                className="input"
                type="password"
                value={form.password}
                onChange={(e) => setForm((f) => ({ ...f, password: e.target.value }))}
                required={!editingId}
                placeholder={passwordSet ? t("payments.passwordSet") : undefined}
                autoComplete="new-password"
              />
            </div>

            <div className="field">
              <label className="label" htmlFor="billing-url">
                {t("payments.billmgrUrl")}
              </label>
              <input
                id="billing-url"
                className="input"
                type="url"
                value={form.billmgrUrl}
                onChange={(e) => setForm((f) => ({ ...f, billmgrUrl: e.target.value }))}
              />
            </div>

            <div className="field">
              <label className="label" htmlFor="billing-alert">
                {t("payments.alertDays")}
              </label>
              <input
                id="billing-alert"
                className="input"
                type="number"
                min={1}
                max={90}
                value={form.alertDays}
                onChange={(e) =>
                  setForm((f) => ({ ...f, alertDays: Number(e.target.value) || 10 }))
                }
              />
            </div>
          </div>

          <div className="field">
            <label className="label checkbox-row" htmlFor="billing-enabled">
              <input
                id="billing-enabled"
                type="checkbox"
                checked={form.enabled}
                onChange={(e) => setForm((f) => ({ ...f, enabled: e.target.checked }))}
              />
              <span>{t("payments.enabled")}</span>
            </label>
          </div>

          <div className="form-actions">
            <button type="submit" className="btn" disabled={busy}>
              {editingId ? t("payments.save") : t("payments.create")}
            </button>
            <button
              type="button"
              className="btn btn-secondary"
              disabled={busy}
              onClick={() => setFormOpen(false)}
            >
              {t("common.cancel")}
            </button>
          </div>
        </form>
      )}

      {loading ? (
        <p style={{ color: "var(--muted)" }}>{t("common.loading")}</p>
      ) : accounts.length === 0 && !formOpen ? (
        <div className="card">
          <p>{t("payments.empty")}</p>
        </div>
      ) : (
        <div className="card" style={{ padding: 0, overflow: "hidden" }}>
          <div className="table-wrap">
            <table className="table">
              <thead>
                <tr>
                  <th>{t("payments.serverIp")}</th>
                  <th className="col-hide-mobile">{t("payments.provider")}</th>
                  <th>{t("payments.expireDate")}</th>
                  <th>{t("payments.daysLeft")}</th>
                  <th className="col-hide-mobile">{t("payments.status")}</th>
                  <th className="col-hide-mobile">{t("payments.lastChecked")}</th>
                  <th />
                </tr>
              </thead>
              <tbody>
                {accounts.map((a) => (
                  <tr key={a.id}>
                    <td>
                      <div style={{ fontWeight: 600 }}>{a.server_ip}</div>
                      <div style={{ color: "var(--muted)", fontSize: "0.8rem" }}>
                        {a.login}
                        {a.name ? ` · ${a.name}` : ""}
                      </div>
                      {a.last_check_error ? (
                        <div
                          style={{
                            color: "var(--danger, #b91c1c)",
                            fontSize: "0.75rem",
                            marginTop: "0.25rem",
                          }}
                        >
                          {a.last_check_error}
                        </div>
                      ) : null}
                    </td>
                    <td className="col-hide-mobile">{a.provider}</td>
                    <td>{a.expire_date || "—"}</td>
                    <td>
                      {a.days_left == null ? (
                        "—"
                      ) : (
                        <span
                          style={{
                            fontWeight: 600,
                            color:
                              a.days_left <= a.alert_days
                                ? "var(--warn, #b45309)"
                                : "var(--ok, #15803d)",
                          }}
                        >
                          {a.days_left}
                        </span>
                      )}
                    </td>
                    <td className="col-hide-mobile">
                      {a.status || "—"}
                      {a.cost ? (
                        <div style={{ color: "var(--muted)", fontSize: "0.8rem" }}>{a.cost}</div>
                      ) : null}
                    </td>
                    <td className="col-hide-mobile">
                      {a.last_checked_at ? formatDateTime(a.last_checked_at) : "—"}
                    </td>
                    <td>
                      <div style={{ display: "flex", gap: "0.35rem", flexWrap: "wrap" }}>
                        <button
                          type="button"
                          className="btn btn-secondary"
                          style={{ fontSize: "0.8rem" }}
                          disabled={busy}
                          onClick={() => void handleRefresh(a.id)}
                        >
                          {t("payments.refresh")}
                        </button>
                        <button
                          type="button"
                          className="btn btn-secondary"
                          style={{ fontSize: "0.8rem" }}
                          disabled={busy}
                          onClick={() => openEdit(a)}
                        >
                          {t("payments.edit")}
                        </button>
                        <button
                          type="button"
                          className="btn btn-secondary"
                          style={{ fontSize: "0.8rem" }}
                          disabled={busy}
                          onClick={() => setDeleteId(a.id)}
                        >
                          {t("common.delete")}
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      <ConfirmDialog
        open={deleteId != null}
        title={t("common.delete")}
        message={t("payments.deleteConfirm", { ip: deleteTarget?.server_ip ?? "" })}
        confirmLabel={t("common.delete")}
        danger
        onConfirm={() => void handleDelete()}
        onCancel={() => setDeleteId(null)}
      />
    </div>
  );
}
