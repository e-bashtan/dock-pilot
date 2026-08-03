"use client";

import { useState } from "react";
import { parseBackupError } from "./parseBackupError";

export function BackupErrorDetails({
  error,
  t,
}: {
  error: string;
  t: (key: string) => string;
}) {
  const [expanded, setExpanded] = useState(false);
  const parsed = parseBackupError(error);
  const isMulti = !!parsed && parsed.length > 1;

  return (
    <div style={{ marginTop: "0.5rem" }}>
      <div style={{ fontWeight: 600, marginBottom: "0.35rem" }}>
        {isMulti ? t("backups.errorPartialTitle") : t("backups.errorGenericTitle")}
      </div>
      <p style={{ margin: "0 0 0.5rem", fontSize: "0.875rem" }}>
        {isMulti ? t("backups.errorPartialDesc") : t("backups.errorGenericDesc")}
      </p>

      {parsed && parsed.length > 0 ? (
        <ul style={{ margin: "0.5rem 0", paddingLeft: "1.5rem", fontSize: "0.875rem" }}>
          {parsed.map((p, idx) => (
            <li key={idx}>
              <code>{p.db}</code> — {p.error}
            </li>
          ))}
        </ul>
      ) : null}

      <details open={expanded} onToggle={(e) => setExpanded(e.currentTarget.open)}>
        <summary
          style={{
            cursor: "pointer",
            color: "var(--muted)",
            fontSize: "0.85rem",
            marginTop: "0.5rem",
          }}
        >
          {t("backups.techDetails")}
        </summary>
        <div
          style={{
            marginTop: "0.5rem",
            padding: "0.75rem",
            background: "var(--bg)",
            border: "1px solid var(--border)",
            borderRadius: "var(--radius)",
            fontSize: "0.8rem",
            fontFamily: "monospace",
            whiteSpace: "pre-wrap",
            overflowWrap: "anywhere",
          }}
        >
          {error}
        </div>
        <button
          type="button"
          className="btn btn-secondary"
          style={{ marginTop: "0.5rem", fontSize: "0.8rem" }}
          onClick={() => {
            void navigator.clipboard.writeText(error);
          }}
        >
          {t("backups.copyLog")}
        </button>
      </details>
    </div>
  );
}
