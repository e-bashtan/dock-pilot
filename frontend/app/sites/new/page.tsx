"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { EnvVarList } from "@/components/EnvVarList";
import { KeyValueEditor } from "@/components/KeyValueEditor";
import { SiteJsonActions } from "@/components/SiteJsonActions";
import { api, ApiError } from "@/lib/api";
import { useI18n } from "@/lib/i18n/context";
import type { WizardState } from "@/lib/types";

const WEB_STEP_KEYS = ["basic", "repository", "docker", "environment", "nginxSsl", "review"] as const;
const BOT_STEP_KEYS = ["basic", "repository", "docker", "environment", "review"] as const;

type WebStepKey = (typeof WEB_STEP_KEYS)[number];
type BotStepKey = (typeof BOT_STEP_KEYS)[number];
type StepKey = WebStepKey | BotStepKey;

const initialState: WizardState = {
  siteType: "web",
  name: "",
  slug: "",
  primaryUrl: "",
  gitRepoUrl: "",
  gitBranch: "main",
  dockerfilePath: "Dockerfile",
  buildContext: ".",
  containerPort: 3000,
  dockerNetworkHost: false,
  envVars: [],
  secrets: [],
  aliases: [],
  nginxSslEnabled: true,
  nginxForceHttps: true,
};

export default function NewSiteWizardPage() {
  const router = useRouter();
  const { t } = useI18n();
  const [step, setStep] = useState(0);
  const [data, setData] = useState<WizardState>(initialState);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const stepKeys = data.siteType === "telegram_bot" ? BOT_STEP_KEYS : WEB_STEP_KEYS;
  const stepKey = stepKeys[step] as StepKey;

  const update = (patch: Partial<WizardState>) =>
    setData((d) => {
      const next = { ...d, ...patch };
      if (patch.siteType === "telegram_bot") {
        next.nginxSslEnabled = false;
        next.nginxForceHttps = false;
      }
      return next;
    });

  const next = () => setStep((s) => Math.min(s + 1, stepKeys.length - 1));
  const back = () => setStep((s) => Math.max(s - 1, 0));

  const handleDeploy = async () => {
    setSubmitting(true);
    setError(null);
    try {
      const domains =
        data.siteType === "web"
          ? data.aliases
              .filter((a) => a.trim())
              .map((domain) => ({ domain: domain.trim(), is_primary: false }))
          : [];

      const site = await api.createSite({
        name: data.name,
        slug: data.slug || undefined,
        site_type: data.siteType,
        primary_url:
          data.siteType === "web" ? data.primaryUrl : data.primaryUrl || "",
        git_repo_url: data.gitRepoUrl,
        git_branch: data.gitBranch,
        dockerfile_path: data.dockerfilePath,
        build_context: data.buildContext,
        container_port:
          data.siteType === "telegram_bot" ? undefined : data.containerPort,
        docker_network_host: data.dockerNetworkHost,
        nginx_ssl_enabled:
          data.siteType === "web" ? data.nginxSslEnabled : false,
        nginx_force_https:
          data.siteType === "web" ? data.nginxForceHttps : false,
        domains,
        env_vars: data.envVars.filter((e) => e.key.trim()),
      });

      const secrets: Record<string, string> = {};
      for (const s of data.secrets) {
        const key = s.key.trim();
        if (key && s.value) secrets[key] = s.value;
      }
      if (Object.keys(secrets).length > 0) {
        await api.setSecrets(site.id, secrets);
      }

      await api.deploySite(site.id);
      router.push(`/sites/${site.id}/deployments`);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : t("wizard.createFailed"));
      setSubmitting(false);
    }
  };

  return (
    <div>
      <div className="page-header">
        <h1>{t("wizard.title")}</h1>
        <div className="page-actions">
          <SiteJsonActions />
        </div>
      </div>
      <div className="wizard-steps">
        {stepKeys.map((key, i) => (
          <span
            key={key}
            className={`wizard-step ${i === step ? "active" : ""} ${i < step ? "done" : ""}`}
          >
            {i + 1}. {t(`wizard.steps.${key}`)}
          </span>
        ))}
      </div>

      {error && <div className="alert alert-error">{error}</div>}

      <div className="card">
        {stepKey === "basic" && <StepBasic data={data} update={update} />}
        {stepKey === "repository" && <StepRepo data={data} update={update} />}
        {stepKey === "docker" && <StepDocker data={data} update={update} />}
        {stepKey === "environment" && <StepEnv data={data} update={update} />}
        {stepKey === "nginxSsl" && <StepNginx data={data} update={update} />}
        {stepKey === "review" && <StepReview data={data} />}
      </div>

      <div style={{ display: "flex", gap: "0.75rem", marginTop: "1.5rem" }}>
        {step > 0 && (
          <button type="button" className="btn btn-secondary" onClick={back}>
            {t("common.back")}
          </button>
        )}
        {step < stepKeys.length - 1 ? (
          <button type="button" className="btn" onClick={next}>
            {t("common.continue")}
          </button>
        ) : (
          <button
            type="button"
            className="btn"
            onClick={handleDeploy}
            disabled={submitting}
          >
            {submitting ? t("wizard.deploying") : t("wizard.createAndDeploy")}
          </button>
        )}
        <Link href="/sites" className="btn btn-secondary">
          {t("common.cancel")}
        </Link>
      </div>
    </div>
  );
}

function StepBasic({
  data,
  update,
}: {
  data: WizardState;
  update: (p: Partial<WizardState>) => void;
}) {
  const { t } = useI18n();

  return (
    <>
      <h2>{t("wizard.basic.heading")}</h2>
      <div className="field">
        <label className="label">{t("wizard.basic.type")}</label>
        <select
          className="input"
          value={data.siteType}
          onChange={(e) =>
            update({ siteType: e.target.value as WizardState["siteType"] })
          }
        >
          <option value="web">{t("wizard.basic.typeWebsite")}</option>
          <option value="telegram_bot">{t("wizard.basic.typeTelegramBot")}</option>
        </select>
      </div>
      <div className="field">
        <label className="label">{t("wizard.basic.name")}</label>
        <input
          className="input"
          value={data.name}
          onChange={(e) => update({ name: e.target.value })}
          placeholder={
            data.siteType === "telegram_bot"
              ? t("wizard.basic.namePlaceholderBot")
              : t("wizard.basic.namePlaceholderWeb")
          }
        />
      </div>
      <div className="field">
        <label className="label">{t("wizard.basic.slug")}</label>
        <input
          className="input"
          value={data.slug}
          onChange={(e) => update({ slug: e.target.value })}
          placeholder={t("wizard.basic.slugPlaceholder")}
        />
      </div>
      {data.siteType === "web" && (
        <div className="field">
          <label className="label">{t("wizard.basic.primaryUrl")}</label>
          <input
            className="input"
            value={data.primaryUrl}
            onChange={(e) => update({ primaryUrl: e.target.value })}
            placeholder={t("wizard.basic.primaryUrlPlaceholder")}
          />
        </div>
      )}
      {data.siteType === "telegram_bot" && (
        <p style={{ color: "var(--muted)", fontSize: "0.875rem", margin: 0 }}>
          {t("wizard.basic.telegramHint")}
        </p>
      )}
    </>
  );
}

function StepRepo({
  data,
  update,
}: {
  data: WizardState;
  update: (p: Partial<WizardState>) => void;
}) {
  const { t } = useI18n();

  return (
    <>
      <h2>{t("wizard.repo.heading")}</h2>
      <p style={{ color: "var(--muted)", fontSize: "0.875rem" }}>
        {t("wizard.repo.hint")}
      </p>
      <div className="field">
        <label className="label">{t("wizard.repo.gitUrl")}</label>
        <input
          className="input"
          value={data.gitRepoUrl}
          onChange={(e) => update({ gitRepoUrl: e.target.value })}
          placeholder={t("wizard.repo.gitUrlPlaceholder")}
        />
      </div>
      <div className="field">
        <label className="label">{t("wizard.repo.branch")}</label>
        <input
          className="input"
          value={data.gitBranch}
          onChange={(e) => update({ gitBranch: e.target.value })}
        />
      </div>
    </>
  );
}

function StepDocker({
  data,
  update,
}: {
  data: WizardState;
  update: (p: Partial<WizardState>) => void;
}) {
  const { t } = useI18n();

  return (
    <>
      <h2>{t("wizard.docker.heading")}</h2>
      <div className="grid-2">
        <div className="field">
          <label className="label">{t("wizard.docker.dockerfilePath")}</label>
          <input
            className="input"
            value={data.dockerfilePath}
            onChange={(e) => update({ dockerfilePath: e.target.value })}
          />
        </div>
        <div className="field">
          <label className="label">{t("wizard.docker.buildContext")}</label>
          <input
            className="input"
            value={data.buildContext}
            onChange={(e) => update({ buildContext: e.target.value })}
          />
        </div>
      </div>
      {data.siteType === "web" && (
        <div className="field">
          <label className="label">{t("wizard.docker.containerPort")}</label>
          <input
            className="input"
            type="number"
            value={data.containerPort}
            onChange={(e) =>
              update({ containerPort: parseInt(e.target.value, 10) || 3000 })
            }
          />
        </div>
      )}
      <label style={{ display: "flex", gap: "0.5rem", alignItems: "flex-start", marginTop: "0.5rem" }}>
        <input
          type="checkbox"
          checked={data.dockerNetworkHost}
          onChange={(e) => update({ dockerNetworkHost: e.target.checked })}
          style={{ marginTop: "0.2rem" }}
        />
        <span>{t("wizard.docker.hostNetwork")}</span>
      </label>
    </>
  );
}

function StepEnv({
  data,
  update,
}: {
  data: WizardState;
  update: (p: Partial<WizardState>) => void;
}) {
  const { t } = useI18n();

  return (
    <>
      <h2>{t("wizard.env.heading")}</h2>
      <p style={{ color: "var(--muted)", fontSize: "0.875rem" }}>
        {data.siteType === "telegram_bot"
          ? t("wizard.env.hintBot")
          : t("wizard.env.hintWeb")}
      </p>

      <h3 style={{ marginTop: "1.5rem" }}>{t("wizard.env.environmentVariables")}</h3>
      <EnvVarList
        envVars={data.envVars}
        onChange={(envVars) => update({ envVars })}
      />

      <h3 style={{ marginTop: "1.5rem" }}>{t("wizard.env.secrets")}</h3>
      <KeyValueEditor
        rows={data.secrets}
        onChange={(secrets) => update({ secrets })}
        valueInputType="password"
        keyPlaceholder={t("wizard.env.gitTokenPlaceholder")}
        valuePlaceholder={t("secrets.valuePlaceholder")}
        addLabel={t("wizard.env.addSecret")}
      />
    </>
  );
}

function StepNginx({
  data,
  update,
}: {
  data: WizardState;
  update: (p: Partial<WizardState>) => void;
}) {
  const { t } = useI18n();

  return (
    <>
      <h2>{t("wizard.nginx.heading")}</h2>
      <div className="field">
        <label className="label">{t("wizard.nginx.aliases")}</label>
        <textarea
          className="textarea"
          value={data.aliases.join("\n")}
          onChange={(e) =>
            update({
              aliases: e.target.value.split("\n").filter(Boolean),
            })
          }
          placeholder={t("wizard.nginx.aliasesPlaceholder")}
        />
      </div>
      <label style={{ display: "flex", gap: "0.5rem", alignItems: "center" }}>
        <input
          type="checkbox"
          checked={data.nginxSslEnabled}
          onChange={(e) => update({ nginxSslEnabled: e.target.checked })}
        />
        {t("wizard.nginx.enableSsl")}
      </label>
      <label
        style={{
          display: "flex",
          gap: "0.5rem",
          alignItems: "center",
          marginTop: "0.5rem",
        }}
      >
        <input
          type="checkbox"
          checked={data.nginxForceHttps}
          onChange={(e) => update({ nginxForceHttps: e.target.checked })}
        />
        {t("wizard.nginx.forceHttps")}
      </label>
    </>
  );
}

function StepReview({ data }: { data: WizardState }) {
  const { t } = useI18n();

  return (
    <>
      <h2>{t("wizard.review.heading")}</h2>
      <dl style={{ margin: 0 }}>
        <ReviewRow
          label={t("common.type")}
          value={
            data.siteType === "telegram_bot"
              ? t("sites.typeTelegramBot")
              : t("sites.typeWebsite")
          }
        />
        <ReviewRow label={t("wizard.basic.name")} value={data.name} />
        {data.siteType === "web" && (
          <ReviewRow label={t("wizard.review.primaryUrl")} value={data.primaryUrl} />
        )}
        <ReviewRow label={t("wizard.review.repository")} value={data.gitRepoUrl} />
        <ReviewRow label={t("wizard.review.branch")} value={data.gitBranch} />
        <ReviewRow
          label={t("wizard.review.docker")}
          value={
            data.siteType === "telegram_bot"
              ? data.dockerfilePath
              : `${data.dockerfilePath} (${t("wizard.review.portSuffix", { port: data.containerPort })})`
          }
        />
        <ReviewRow
          label={t("wizard.review.envVars")}
          value={String(data.envVars.filter((e) => e.key).length)}
        />
        <ReviewRow
          label={t("wizard.review.secrets")}
          value={String(data.secrets.filter((s) => s.key.trim()).length)}
        />
        <ReviewRow
          label={t("wizard.review.ssl")}
          value={data.nginxSslEnabled ? t("common.enabled") : t("common.disabled")}
        />
      </dl>
    </>
  );
}

function ReviewRow({ label, value }: { label: string; value: string }) {
  const { t } = useI18n();

  return (
    <div style={{ marginBottom: "0.5rem" }}>
      <span style={{ color: "var(--muted)", fontSize: "0.8rem" }}>{label}</span>
      <div>{value || t("common.emDash")}</div>
    </div>
  );
}
