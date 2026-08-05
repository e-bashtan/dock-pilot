"use client";

import { useEffect, useState } from "react";
import { useI18n } from "@/lib/i18n/context";

type ConfirmDialogProps = {
  open: boolean;
  title: string;
  message: string;
  confirmLabel?: string;
  danger?: boolean;
  busy?: boolean;
  /** If set, confirm stays disabled until the user types this exact string. */
  confirmText?: string;
  confirmTextLabel?: string;
  onConfirm: () => void;
  onCancel: () => void;
};

export function ConfirmDialog({
  open,
  title,
  message,
  confirmLabel,
  danger = false,
  busy = false,
  confirmText,
  confirmTextLabel,
  onConfirm,
  onCancel,
}: ConfirmDialogProps) {
  const { t } = useI18n();
  const [typed, setTyped] = useState("");

  useEffect(() => {
    if (open) setTyped("");
  }, [open, confirmText]);

  if (!open) return null;

  const needsType = !!confirmText;
  const typedOk = !needsType || typed === confirmText;

  return (
    <div className="modal-backdrop" onClick={onCancel} role="presentation">
      <div
        className="modal card confirm-dialog"
        onClick={(e) => e.stopPropagation()}
        role="alertdialog"
        aria-labelledby="confirm-dialog-title"
        aria-describedby="confirm-dialog-message"
      >
        <h2 id="confirm-dialog-title">{title}</h2>
        <p id="confirm-dialog-message" className="confirm-dialog-message">
          {message}
        </p>
        {needsType ? (
          <div className="field" style={{ marginTop: "1rem" }}>
            <label className="label" htmlFor="confirm-dialog-type">
              {confirmTextLabel || t("common.typeToConfirm", { name: confirmText! })}
            </label>
            <input
              id="confirm-dialog-type"
              className="input"
              value={typed}
              onChange={(e) => setTyped(e.target.value)}
              autoComplete="off"
              autoFocus
              disabled={busy}
            />
          </div>
        ) : null}
        <div className="confirm-dialog-actions">
          <button
            type="button"
            className="btn btn-secondary"
            onClick={onCancel}
            disabled={busy}
          >
            {t("common.cancel")}
          </button>
          <button
            type="button"
            className={`btn${danger ? " btn-danger" : ""}`}
            onClick={onConfirm}
            disabled={busy || !typedOk}
          >
            {confirmLabel ?? t("common.continue")}
          </button>
        </div>
      </div>
    </div>
  );
}
