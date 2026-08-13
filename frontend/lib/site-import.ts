import type { CreateSiteRequest, EnvVar, SiteType } from "./types";

export const SITE_IMPORT_FORMAT = "barn.site";
export const SITE_IMPORT_FORMAT_VERSION = 1;

export type SiteImportSecrets = EnvVar[];

/** Full JSON document for export/import (template + filled configs). */
export type SiteImportDocument = {
  format: typeof SITE_IMPORT_FORMAT;
  format_version: number;
  /** Human/LLM instructions — ignored on import. */
  instructions?: string;
  site_type: SiteType;
  name: string;
  slug?: string;
  primary_url?: string;
  git_repo_url: string;
  git_branch?: string;
  dockerfile_path?: string;
  build_context?: string;
  container_port?: number;
  nginx_ssl_enabled?: boolean;
  nginx_force_https?: boolean;
  docker_volume_mounts?: string[];
  docker_named_volumes?: string[];
  docker_network_host?: boolean;
  health_check_path?: string;
  domains?: { domain: string; is_primary?: boolean }[];
  env_vars?: EnvVar[];
  /** Encrypted after save; fill real values before import. */
  secrets?: SiteImportSecrets;
  /** If true (default), start deploy after create. */
  deploy?: boolean;
};

export type ParsedSiteImport = {
  request: CreateSiteRequest;
  secrets: Record<string, string>;
  deploy: boolean;
  siteType: SiteType;
};

export class SiteImportError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "SiteImportError";
  }
}

const WEB_INSTRUCTIONS = [
  "Barn site import template (website).",
  "Fill every placeholder, then import the file in the Barn panel (Sites → Import JSON).",
  "site_type must stay \"web\".",
  "primary_url is required (https://your-domain).",
  "domains: extra hostnames (aliases), is_primary false unless it is the main host.",
  "env_vars: non-secret KEY=VALUE pairs for the container.",
  "secrets: sensitive values (GIT_TOKEN / GIT_SSH_KEY for private repos). Never commit real secrets to git.",
  "Set deploy to true to create and deploy immediately after import.",
].join(" ");

const BOT_INSTRUCTIONS = [
  "Barn site import template (Telegram bot).",
  "Fill every placeholder, then import the file in the Barn panel (Sites → Import JSON).",
  "site_type must stay \"telegram_bot\".",
  "primary_url is optional (panel may synthesize telegram://<slug>).",
  "nginx/ssl/domains/container_port are ignored for bots.",
  "Put BOT_TOKEN in secrets (recommended), not in env_vars.",
  "env_vars: non-secret KEY=VALUE pairs for the container.",
  "Set deploy to true to create and deploy immediately after import.",
].join(" ");

export function buildSiteImportTemplate(siteType: SiteType): SiteImportDocument {
  if (siteType === "telegram_bot") {
    return {
      format: SITE_IMPORT_FORMAT,
      format_version: SITE_IMPORT_FORMAT_VERSION,
      instructions: BOT_INSTRUCTIONS,
      site_type: "telegram_bot",
      name: "My Telegram Bot",
      slug: "my-telegram-bot",
      primary_url: "",
      git_repo_url: "https://github.com/org/my-bot.git",
      git_branch: "main",
      dockerfile_path: "Dockerfile",
      build_context: ".",
      docker_network_host: false,
      docker_volume_mounts: [],
      docker_named_volumes: [],
      env_vars: [{ key: "LOG_LEVEL", value: "info" }],
      secrets: [{ key: "BOT_TOKEN", value: "REPLACE_WITH_TELEGRAM_BOT_TOKEN" }],
      deploy: true,
    };
  }

  return {
    format: SITE_IMPORT_FORMAT,
    format_version: SITE_IMPORT_FORMAT_VERSION,
    instructions: WEB_INSTRUCTIONS,
    site_type: "web",
    name: "My Website",
    slug: "my-website",
    primary_url: "https://app.example.com",
    git_repo_url: "https://github.com/org/my-site.git",
    git_branch: "main",
    dockerfile_path: "Dockerfile",
    build_context: ".",
    container_port: 3000,
    docker_network_host: false,
    nginx_ssl_enabled: true,
    nginx_force_https: true,
    health_check_path: "",
    docker_volume_mounts: [],
    docker_named_volumes: [],
    domains: [{ domain: "www.example.com", is_primary: false }],
    env_vars: [{ key: "NODE_ENV", value: "production" }],
    secrets: [{ key: "GIT_TOKEN", value: "REPLACE_WITH_GITHUB_PAT_IF_PRIVATE" }],
    deploy: true,
  };
}

export function siteImportFilename(siteType: SiteType): string {
  return siteType === "telegram_bot"
    ? "barn-telegram-bot-template.json"
    : "barn-website-template.json";
}

export function downloadSiteImportTemplate(siteType: SiteType): void {
  const doc = buildSiteImportTemplate(siteType);
  const blob = new Blob([JSON.stringify(doc, null, 2) + "\n"], {
    type: "application/json",
  });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = siteImportFilename(siteType);
  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(url);
}

function asObject(value: unknown, label: string): Record<string, unknown> {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    throw new SiteImportError(`${label} must be a JSON object`);
  }
  return value as Record<string, unknown>;
}

function asString(value: unknown, field: string, required = true): string {
  if (value === undefined || value === null) {
    if (required) throw new SiteImportError(`Missing field: ${field}`);
    return "";
  }
  if (typeof value !== "string") {
    throw new SiteImportError(`Field ${field} must be a string`);
  }
  return value.trim();
}

function asOptionalString(value: unknown, field: string): string | undefined {
  if (value === undefined || value === null || value === "") return undefined;
  if (typeof value !== "string") {
    throw new SiteImportError(`Field ${field} must be a string`);
  }
  return value.trim();
}

function asBool(value: unknown, field: string, fallback: boolean): boolean {
  if (value === undefined || value === null) return fallback;
  if (typeof value !== "boolean") {
    throw new SiteImportError(`Field ${field} must be a boolean`);
  }
  return value;
}

function asOptionalInt(value: unknown, field: string): number | undefined {
  if (value === undefined || value === null || value === "") return undefined;
  if (typeof value !== "number" || !Number.isFinite(value) || !Number.isInteger(value)) {
    throw new SiteImportError(`Field ${field} must be an integer`);
  }
  return value;
}

function asStringArray(value: unknown, field: string): string[] | undefined {
  if (value === undefined || value === null) return undefined;
  if (!Array.isArray(value)) {
    throw new SiteImportError(`Field ${field} must be an array of strings`);
  }
  return value.map((item, i) => {
    if (typeof item !== "string") {
      throw new SiteImportError(`Field ${field}[${i}] must be a string`);
    }
    return item.trim();
  }).filter(Boolean);
}

function parseEnvList(value: unknown, field: string): EnvVar[] {
  if (value === undefined || value === null) return [];
  if (!Array.isArray(value)) {
    throw new SiteImportError(`Field ${field} must be an array`);
  }
  const out: EnvVar[] = [];
  for (let i = 0; i < value.length; i++) {
    const row = asObject(value[i], `${field}[${i}]`);
    const key = asString(row.key, `${field}[${i}].key`);
    if (!key) continue;
    const raw = row.value;
    if (raw !== undefined && raw !== null && typeof raw !== "string") {
      throw new SiteImportError(`Field ${field}[${i}].value must be a string`);
    }
    out.push({ key, value: typeof raw === "string" ? raw : "" });
  }
  return out;
}

function parseDomains(
  value: unknown,
): { domain: string; is_primary: boolean }[] | undefined {
  if (value === undefined || value === null) return undefined;
  if (!Array.isArray(value)) {
    throw new SiteImportError("Field domains must be an array");
  }
  return value.map((item, i) => {
    const row = asObject(item, `domains[${i}]`);
    const domain = asString(row.domain, `domains[${i}].domain`);
    if (!domain) {
      throw new SiteImportError(`Field domains[${i}].domain is required`);
    }
    return {
      domain,
      is_primary: asBool(row.is_primary, `domains[${i}].is_primary`, false),
    };
  });
}

function normalizeSiteType(value: unknown): SiteType {
  const raw = typeof value === "string" ? value.trim().toLowerCase() : "";
  if (raw === "telegram_bot" || raw === "bot" || raw === "telegram") {
    return "telegram_bot";
  }
  if (raw === "web" || raw === "website" || raw === "site" || raw === "") {
    return "web";
  }
  throw new SiteImportError(
    `Unknown site_type "${String(value)}". Use "web" or "telegram_bot".`,
  );
}

function isPlaceholderSecret(value: string): boolean {
  const v = value.trim();
  if (!v) return true;
  return (
    v.startsWith("REPLACE_") ||
    v.startsWith("YOUR_") ||
    v === "changeme" ||
    v === "change-me"
  );
}

/**
 * Parse a Barn site import JSON string into create/deploy payload.
 * Accepts barn.site documents or a bare CreateSite-like object.
 */
export function parseSiteImportJson(text: string): ParsedSiteImport {
  let raw: unknown;
  try {
    raw = JSON.parse(text);
  } catch {
    throw new SiteImportError("Invalid JSON");
  }

  const obj = asObject(raw, "Root");

  if (obj.format !== undefined && obj.format !== SITE_IMPORT_FORMAT) {
    throw new SiteImportError(
      `Unsupported format "${String(obj.format)}". Expected "${SITE_IMPORT_FORMAT}".`,
    );
  }
  if (obj.format_version !== undefined) {
    if (
      typeof obj.format_version !== "number" ||
      !Number.isInteger(obj.format_version) ||
      obj.format_version < 1 ||
      obj.format_version > SITE_IMPORT_FORMAT_VERSION
    ) {
      throw new SiteImportError(
        `Unsupported format_version. Expected 1..${SITE_IMPORT_FORMAT_VERSION}.`,
      );
    }
  }

  const siteType = normalizeSiteType(obj.site_type);
  const name = asString(obj.name, "name");
  if (!name) throw new SiteImportError("Field name is required");

  const gitRepoUrl = asString(obj.git_repo_url, "git_repo_url");
  if (!gitRepoUrl) throw new SiteImportError("Field git_repo_url is required");

  const primaryUrl =
    asOptionalString(obj.primary_url, "primary_url") ?? "";
  if (siteType === "web" && !primaryUrl) {
    throw new SiteImportError("Field primary_url is required for website");
  }

  const envVars = parseEnvList(obj.env_vars, "env_vars");
  const secretRows = parseEnvList(obj.secrets, "secrets");
  const secrets: Record<string, string> = {};
  for (const s of secretRows) {
    if (!s.key) continue;
    if (isPlaceholderSecret(s.value)) continue;
    secrets[s.key] = s.value;
  }

  const request: CreateSiteRequest = {
    name,
    slug: asOptionalString(obj.slug, "slug"),
    site_type: siteType,
    primary_url: primaryUrl,
    git_repo_url: gitRepoUrl,
    git_branch: asOptionalString(obj.git_branch, "git_branch"),
    dockerfile_path: asOptionalString(obj.dockerfile_path, "dockerfile_path"),
    build_context: asOptionalString(obj.build_context, "build_context"),
    docker_network_host: asBool(obj.docker_network_host, "docker_network_host", false),
    docker_volume_mounts: asStringArray(obj.docker_volume_mounts, "docker_volume_mounts"),
    docker_named_volumes: asStringArray(obj.docker_named_volumes, "docker_named_volumes"),
    env_vars: envVars.filter((e) => e.key),
  };

  if (siteType === "web") {
    request.container_port = asOptionalInt(obj.container_port, "container_port");
    request.nginx_ssl_enabled = asBool(obj.nginx_ssl_enabled, "nginx_ssl_enabled", true);
    request.nginx_force_https = asBool(obj.nginx_force_https, "nginx_force_https", true);
    request.health_check_path = asOptionalString(obj.health_check_path, "health_check_path");
    request.domains = parseDomains(obj.domains);
  } else {
    request.nginx_ssl_enabled = false;
    request.nginx_force_https = false;
  }

  const deploy = asBool(obj.deploy, "deploy", true);

  return { request, secrets, deploy, siteType };
}
