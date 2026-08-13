"use client";

import { useId, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { api, ApiError } from "@/lib/api";
import { useI18n } from "@/lib/i18n/context";
import {
  downloadSiteImportTemplate,
  parseSiteImportJson,
  SiteImportError,
} from "@/lib/site-import";
import type { SiteType } from "@/lib/types";

type Mode = "closed" | "template" | "import";

export function SiteJsonActions() {
  const { t } = useI18n();
  const [mode, setMode] = useState<Mode>("closed");

  return (
    <>
      <button
        type="button"
        className="btn btn-secondary"
        onClick={() => setMode("template")}
      >
        {t("siteImport.downloadTemplate")}
      </button>
      <button
        type="button"
        className="btn btn-secondary"
        onClick={() => setMode("import")}
      >
        {t("siteImport.importJson")}
      </button>
      {mode === "template" && (
        <TemplateDialog onClose={() => setMode("closed")} />
      )}
      {mode === "import" && (
        <ImportDialog onClose={() => setMode("closed")} />
      )}
    </>
  );
}

function TemplateDialog({ onClose }: { onClose: () => void }) {
  const { t } = useI18n();

  const download = (siteType: SiteType) => {
    downloadSiteImportTemplate(siteType);
    onClose();
  };

  return (
    <div className="modal-backdrop" onClick={onClose} role="presentation">
      <div
        className="modal card"
        style={{ width: "min(100%, 28rem)" }}
        onClick={(e) => e.stopPropagation()}
        role="dialog"
        aria-labelledby="site-template-dialog-title"
      >
        <h2 id="site-template-dialog-title" style={{ marginTop: 0 }}>
          {t("siteImport.templateTitle")}
        </h2>
        <p style={{ color: "var(--muted)", fontSize: "0.875rem", marginTop: 0 }}>
          {t("siteImport.templateHint")}
        </p>
        <div style={{ display: "flex", flexDirection: "column", gap: "0.75rem" }}>
          <button
            type="button"
            className="choice-option"
            onClick={() => download("web")}
          >
            <span className="choice-option-title">
              {t("siteImport.templateWebsite")}
            </span>
            <span className="choice-option-hint">
              {t("siteImport.templateWebsiteHint")}
            </span>
          </button>
          <button
            type="button"
            className="choice-option"
            onClick={() => download("telegram_bot")}
          >
            <span className="choice-option-title">
              {t("siteImport.templateBot")}
            </span>
            <span className="choice-option-hint">
              {t("siteImport.templateBotHint")}
            </span>
          </button>
        </div>
        <div className="confirm-dialog-actions" style={{ marginTop: "1.25rem" }}>
          <button type="button" className="btn btn-secondary" onClick={onClose}>
            {t("common.cancel")}
          </button>
        </div>
      </div>
    </div>
  );
}

function ImportDialog({ onClose }: { onClose: () => void }) {
  const { t } = useI18n();
  const router = useRouter();
  const fileInputId = useId();
  const fileRef = useRef<HTMLInputElement>(null);
  const [text, setText] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const onFile = async (file: File | null) => {
    if (!file) return;
    setError(null);
    try {
      const content = await file.text();
      setText(content);
    } catch {
      setError(t("siteImport.readFileFailed"));
    }
  };

  const handleImport = async () => {
    setSubmitting(true);
    setError(null);
    try {
      const parsed = parseSiteImportJson(text);
      const site = await api.createSite(parsed.request);
      if (Object.keys(parsed.secrets).length > 0) {
        await api.setSecrets(site.id, parsed.secrets);
      }
      if (parsed.deploy) {
        await api.deploySite(site.id);
        router.push(`/sites/${site.id}/deployments`);
      } else {
        router.push(`/sites/${site.id}`);
      }
    } catch (e) {
      if (e instanceof SiteImportError) {
        setError(e.message);
      } else if (e instanceof ApiError) {
        setError(e.message);
      } else {
        setError(t("siteImport.importFailed"));
      }
      setSubmitting(false);
    }
  };

  return (
    <div className="modal-backdrop" onClick={onClose} role="presentation">
      <div
        className="modal card modal-wide"
        onClick={(e) => e.stopPropagation()}
        role="dialog"
        aria-labelledby="site-import-dialog-title"
      >
        <h2 id="site-import-dialog-title" style={{ marginTop: 0 }}>
          {t("siteImport.importTitle")}
        </h2>
        <p style={{ color: "var(--muted)", fontSize: "0.875rem", marginTop: 0 }}>
          {t("siteImport.importHint")}
        </p>

        {error && <div className="alert alert-error">{error}</div>}

        <div className="field">
          <label className="label" htmlFor={fileInputId}>
            {t("siteImport.chooseFile")}
          </label>
          <input
            id={fileInputId}
            ref={fileRef}
            type="file"
            accept="application/json,.json,text/json"
            onChange={(e) => void onFile(e.target.files?.[0] ?? null)}
            disabled={submitting}
          />
        </div>

        <div className="field">
          <label className="label" htmlFor="site-import-json">
            {t("siteImport.jsonLabel")}
          </label>
          <textarea
            id="site-import-json"
            className="textarea"
            rows={14}
            value={text}
            onChange={(e) => setText(e.target.value)}
            placeholder={t("siteImport.jsonPlaceholder")}
            disabled={submitting}
            spellCheck={false}
            style={{ fontFamily: "ui-monospace, SFMono-Regular, Menlo, monospace", fontSize: "0.8125rem" }}
          />
        </div>

        <div className="confirm-dialog-actions">
          <button
            type="button"
            className="btn btn-secondary"
            onClick={onClose}
            disabled={submitting}
          >
            {t("common.cancel")}
          </button>
          <button
            type="button"
            className="btn"
            onClick={() => void handleImport()}
            disabled={submitting || !text.trim()}
          >
            {submitting ? t("siteImport.importing") : t("siteImport.importAndCreate")}
          </button>
        </div>
      </div>
    </div>
  );
}
