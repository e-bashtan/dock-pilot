import {
  clearApiToken,
  getApiToken,
  notifyAuthLogout,
} from "./auth-token";
import { normalizeSecretMeta, normalizeSite } from "./normalize";
import type {
  ContainerActionResult,
  CreateSiteRequest,
  Deployment,
  NotificationSettings,
  TelegramTunnelConfig,
  TelegramTunnelStatus,
  SecretMeta,
  Site,
  SiteHealth,
  SiteListItem,
  SiteExportDocument,
  SystemStatus,
  SystemProcesses,
  SystemDockerDir,
  DockerPruneResult,
  SystemUpdateInfo,
  SystemUpgradeJob,
  SystemUpgradeStart,
  SystemHostInfo,
  UpdateNotificationSettings,
  PgInstance,
  PgDatabase,
  PgTableInfo,
  PgQueryResult,
  PgHealth,
  PgRole,
  PgBackupSchedule,
  PgBackup,
  CreatePgInstanceRequest,
  CreatePgDatabaseRequest,
  CreatePgRoleRequest,
  CreatePgScheduleRequest,
  ManualPgBackupRequest,
  RestorePgBackupRequest,
  PgGrant,
  PgConnectionInfo,
  PanelBackupSettings,
  UpdatePanelBackupSettings,
  TestPanelS3Request,
  TestPanelS3Response,
  BackupOperation,
  FullPanelBackup,
  BillingAccount,
  CreateBillingAccountRequest,
  UpdateBillingAccountRequest,
  ServersSettings,
  UpdateServersSettingsRequest,
  ServersOverview,
  ServerNode,
  ServerEvent,
  ServerIncident,
  ServersPairingCode,
  PairBarnRequest,
  CreateAgentInstallRequest,
  UpdateAgentRequest,
  UpdateServerNodeRequest,
  UpdateServerNodeBillingRequest,
  ServerInstallation,
  ServerInstallationLog,
} from "./types";

import { resolveApiBase } from "./api-base";

/** Browser: resolved at call time (supports auto/same-origin). SSR/build: env or localhost. */
export function getApiBase(): string {
  return resolveApiBase();
}

// Legacy export for modules that read once at module load (prefer getApiBase() in client code).
export const API_BASE = resolveApiBase();

export class ApiError extends Error {
  constructor(
    message: string,
    public status: number,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

function authHeaders(): HeadersInit {
  const token = getApiToken();
  if (!token) {
    return {};
  }
  return {
    Authorization: `Bearer ${token}`,
  };
}

async function request<T>(
  path: string,
  options: RequestInit = {},
): Promise<T> {
  const res = await fetch(`${getApiBase()}${path}`, {
    ...options,
    headers: {
      "Content-Type": "application/json",
      ...authHeaders(),
      ...options.headers,
    },
  });

  if (res.status === 401) {
    clearApiToken();
    notifyAuthLogout();
    throw new ApiError("Invalid or missing API token", 401);
  }

  if (!res.ok) {
    let message = res.statusText;
    try {
      const body = await res.json();
      if (body?.error) message = body.error;
    } catch {
      /* ignore */
    }
    throw new ApiError(message, res.status);
  }

  if (res.status === 204) {
    return undefined as T;
  }

  return res.json() as Promise<T>;
}

function streamURL(path: string): string {
  const url = new URL(`${getApiBase()}${path}`);
  const token = getApiToken();
  if (token) {
    url.searchParams.set("token", token);
  }
  return url.toString();
}

export type VerifyResult =
  | { ok: true }
  | { ok: false; reason: "invalid_token" }
  | { ok: false; reason: "network"; message: string };

/** Check token against the API before saving it in the browser. */
export async function verifyApiToken(token: string): Promise<VerifyResult> {
  try {
    const res = await fetch(`${getApiBase()}/api/sites`, {
      headers: {
        Authorization: `Bearer ${token.trim()}`,
      },
    });
    if (res.status === 401) {
      return { ok: false, reason: "invalid_token" };
    }
    if (!res.ok) {
      return { ok: false, reason: "network", message: `API returned ${res.status}` };
    }
    return { ok: true };
  } catch (err) {
    const message = err instanceof Error ? err.message : "Network error";
    return { ok: false, reason: "network", message };
  }
}

export const api = {
  listSites: () => request<SiteListItem[]>("/api/sites"),

  listSitesHealth: () => request<SiteHealth[]>("/api/sites/health"),

  getSiteHealth: (id: string) => request<SiteHealth>(`/api/sites/${id}/health`),

  streamSiteContainerLogs: (siteId: string, tail = 300) =>
    new EventSource(
      streamURL(`/api/sites/${siteId}/logs/stream?tail=${tail}`),
    ),

  getSite: (id: string) =>
    request<Site>(`/api/sites/${id}`).then(normalizeSite),

  exportSite: (id: string) =>
    request<SiteExportDocument>(`/api/sites/${id}/export`),

  createSite: (body: CreateSiteRequest) =>
    request<Site>("/api/sites", {
      method: "POST",
      body: JSON.stringify(body),
    }).then(normalizeSite),

  updateSite: (id: string, body: Partial<CreateSiteRequest>) =>
    request<Site>(`/api/sites/${id}`, {
      method: "PATCH",
      body: JSON.stringify(body),
    }).then(normalizeSite),

  deleteSite: (id: string) =>
    request<void>(`/api/sites/${id}`, { method: "DELETE" }),

  deploySite: (id: string) =>
    request<Deployment>(`/api/sites/${id}/deploy`, { method: "POST" }),

  listDeployments: (siteId: string) =>
    request<Deployment[]>(`/api/sites/${siteId}/deployments`),

  listSecrets: (siteId: string) =>
    request<SecretMeta[]>(`/api/sites/${siteId}/secrets`).then((rows) =>
      rows.map(normalizeSecretMeta),
    ),

  setSecrets: (siteId: string, secrets: Record<string, string>) =>
    request<SecretMeta[]>(`/api/sites/${siteId}/secrets`, {
      method: "POST",
      body: JSON.stringify({ secrets }),
    }),

  upsertSecret: (siteId: string, key: string, value: string) =>
    request<SecretMeta>(`/api/sites/${siteId}/secrets/${encodeURIComponent(key)}`, {
      method: "PUT",
      body: JSON.stringify({ value }),
    }),

  deleteSecret: (siteId: string, key: string) =>
    request<void>(
      `/api/sites/${siteId}/secrets/${encodeURIComponent(key)}`,
      { method: "DELETE" },
    ),

  streamDeploymentLogs: (deploymentId: string) =>
    new EventSource(
      streamURL(`/api/deployments/${deploymentId}/logs/stream`),
    ),

  getNotificationSettings: () =>
    request<NotificationSettings>("/api/notifications/settings"),

  updateNotificationSettings: (body: UpdateNotificationSettings) =>
    request<NotificationSettings>("/api/notifications/settings", {
      method: "PUT",
      body: JSON.stringify(body),
    }),

  sendNotificationTest: () =>
    request<{ status: string }>("/api/notifications/test", { method: "POST" }),

  getTelegramTunnel: () =>
    request<TelegramTunnelStatus>("/api/notifications/tunnel"),

  generateTelegramTunnelKey: (body: TelegramTunnelConfig) =>
    request<TelegramTunnelStatus>("/api/notifications/tunnel/key", {
      method: "POST",
      body: JSON.stringify(body),
    }),

  testTelegramTunnelSSH: () =>
    request<{ status: string }>("/api/notifications/tunnel/test-ssh", { method: "POST" }),

  startTelegramTunnel: () =>
    request<TelegramTunnelStatus>("/api/notifications/tunnel/start", { method: "POST" }),

  stopTelegramTunnel: () =>
    request<TelegramTunnelStatus>("/api/notifications/tunnel/stop", { method: "POST" }),

  restartTelegramTunnel: () =>
    request<TelegramTunnelStatus>("/api/notifications/tunnel/restart", { method: "POST" }),

  deleteTelegramTunnel: () =>
    request<void>("/api/notifications/tunnel", { method: "DELETE" }),

  getTelegramTunnelLogs: () =>
    request<{ logs: string }>("/api/notifications/tunnel/logs"),

  startSiteContainer: (id: string) =>
    request<ContainerActionResult>(`/api/sites/${id}/container/start`, { method: "POST" }),

  stopSiteContainer: (id: string) =>
    request<ContainerActionResult>(`/api/sites/${id}/container/stop`, { method: "POST" }),

  restartSiteContainer: (id: string) =>
    request<ContainerActionResult>(`/api/sites/${id}/container/restart`, { method: "POST" }),

  createQRSession: () =>
    request<{ code: string; expires_at: string }>("/api/auth/qr", { method: "POST" }),

  getSystemStatus: () => request<SystemStatus>("/api/system/status"),

  getSystemProcesses: () =>
    request<SystemProcesses>("/api/system/processes"),

  getSystemDockerDirs: () =>
    request<SystemDockerDir[]>("/api/system/docker-dirs"),

  pruneDocker: () =>
    request<DockerPruneResult>("/api/system/docker/prune", { method: "POST" }),

  getSystemHost: () => request<SystemHostInfo>("/api/system/host"),

  getSystemUpdate: () => request<SystemUpdateInfo>("/api/system/update"),

  startSystemUpdate: (target?: string) =>
    request<SystemUpgradeStart>("/api/system/update", {
      method: "POST",
      body: JSON.stringify({ target: target || "latest" }),
    }),

  getSystemUpdateJob: () =>
    request<SystemUpgradeJob>("/api/system/update/job"),

  listPgInstances: () => request<PgInstance[]>("/api/databases"),

  listPgHealth: () => request<PgHealth[]>("/api/databases/health"),

  getPgHealth: (id: string) => request<PgHealth>(`/api/databases/${id}/health`),

  createPgInstance: (body: CreatePgInstanceRequest) =>
    request<PgInstance>("/api/databases", {
      method: "POST",
      body: JSON.stringify(body),
    }),

  getPgInstance: (id: string) => request<PgInstance>(`/api/databases/${id}`),

  deployPgInstance: (id: string) =>
    request<PgInstance>(`/api/databases/${id}/deploy`, { method: "POST" }),

  streamPgDeploy: (id: string) =>
    new EventSource(streamURL(`/api/databases/${id}/deploy/stream`)),

  stopPgInstance: (id: string) =>
    request<PgInstance>(`/api/databases/${id}/stop`, { method: "POST" }),

  deletePgInstance: (id: string) =>
    request<void>(`/api/databases/${id}`, { method: "DELETE" }),

  listPgDatabases: (id: string) =>
    request<PgDatabase[]>(`/api/databases/${id}/databases`),

  createPgDatabase: (id: string, body: CreatePgDatabaseRequest) =>
    request<PgDatabase>(`/api/databases/${id}/databases`, {
      method: "POST",
      body: JSON.stringify(body),
    }),

  deletePgDatabase: (id: string, dbId: string) =>
    request<void>(`/api/databases/${id}/databases/${dbId}`, { method: "DELETE" }),

  listPgTables: (id: string, dbId: string) =>
    request<PgTableInfo[]>(`/api/databases/${id}/databases/${dbId}/tables`),

  selectPgTable: (
    id: string,
    dbId: string,
    body: { table: string; limit?: number },
  ) =>
    request<PgQueryResult>(`/api/databases/${id}/databases/${dbId}/select`, {
      method: "POST",
      body: JSON.stringify(body),
    }),

  listPgRoles: (id: string) => request<PgRole[]>(`/api/databases/${id}/roles`),

  createPgRole: (id: string, body: CreatePgRoleRequest) =>
    request<PgRole>(`/api/databases/${id}/roles`, {
      method: "POST",
      body: JSON.stringify(body),
    }),

  deletePgRole: (id: string, roleId: string) =>
    request<void>(`/api/databases/${id}/roles/${roleId}`, { method: "DELETE" }),

  grantPgRole: (
    id: string,
    roleId: string,
    body: { database_id: string; is_owner?: boolean },
  ) =>
    request<PgGrant>(`/api/databases/${id}/roles/${roleId}/grants`, {
      method: "POST",
      body: JSON.stringify(body),
    }),

  getPgConnection: (id: string, databaseId: string, roleId: string) =>
    request<PgConnectionInfo>(
      `/api/databases/${id}/connection?database_id=${encodeURIComponent(databaseId)}&role_id=${encodeURIComponent(roleId)}`,
    ),

  getPgAdminCredentials: (id: string) =>
    request<PgConnectionInfo>(`/api/databases/${id}/admin-credentials`),

  listPgSchedules: (id: string) =>
    request<PgBackupSchedule[]>(`/api/databases/${id}/schedules`),

  createPgSchedule: (id: string, body: CreatePgScheduleRequest) =>
    request<PgBackupSchedule>(`/api/databases/${id}/schedules`, {
      method: "POST",
      body: JSON.stringify(body),
    }),

  updatePgSchedule: (
    id: string,
    scheduleId: string,
    body: Partial<CreatePgScheduleRequest> & { clear_database_id?: boolean },
  ) =>
    request<PgBackupSchedule>(`/api/databases/${id}/schedules/${scheduleId}`, {
      method: "PATCH",
      body: JSON.stringify(body),
    }),

  deletePgSchedule: (id: string, scheduleId: string) =>
    request<void>(`/api/databases/${id}/schedules/${scheduleId}`, {
      method: "DELETE",
    }),

  listPgBackups: (id: string, scheduleId?: string) => {
    const q = scheduleId
      ? `?schedule_id=${encodeURIComponent(scheduleId)}`
      : "";
    return request<PgBackup[]>(`/api/databases/${id}/backups${q}`);
  },

  createPgBackup: (id: string, body: ManualPgBackupRequest) =>
    request<PgBackup>(`/api/databases/${id}/backups`, {
      method: "POST",
      body: JSON.stringify(body),
    }),

  restorePgBackup: (id: string, body: RestorePgBackupRequest) =>
    request<PgDatabase>(`/api/databases/${id}/backups/restore`, {
      method: "POST",
      body: JSON.stringify(body),
    }),

  streamPgBackupRestore: (
    id: string,
    params: {
      schedule_id: string;
      s3_key: string;
      target_database_name?: string;
      create_database?: boolean;
      drop_existing?: boolean;
    },
  ) => {
    const q = new URLSearchParams({
      schedule_id: params.schedule_id,
      s3_key: params.s3_key,
    });
    if (params.target_database_name) {
      q.set("target_database_name", params.target_database_name);
    }
    if (params.create_database === false) {
      q.set("create_database", "false");
    }
    if (params.drop_existing) {
      q.set("drop_existing", "true");
    }
    return new EventSource(
      streamURL(`/api/databases/${id}/backups/restore/stream?${q}`),
    );
  },

  restorePgBackupFromFile: async (
    id: string,
    form: {
      file: File;
      target_database_name: string;
      create_database?: boolean;
      drop_existing?: boolean;
    },
  ) => {
    const body = new FormData();
    body.append("file", form.file);
    body.append("target_database_name", form.target_database_name);
    body.append(
      "create_database",
      form.create_database === false ? "false" : "true",
    );
    body.append(
      "drop_existing",
      form.drop_existing ? "true" : "false",
    );
    const res = await fetch(
      `${getApiBase()}/api/databases/${id}/restore-upload`,
      {
        method: "POST",
        headers: authHeaders(),
        body,
      },
    );
    if (res.status === 401) {
      clearApiToken();
      notifyAuthLogout();
      throw new ApiError("Invalid or missing API token", 401);
    }
    if (!res.ok) {
      let message = res.statusText;
      try {
        const data = await res.json();
        if (data?.error) message = data.error;
      } catch {
        /* ignore */
      }
      throw new ApiError(message, res.status);
    }
    return res.json() as Promise<PgDatabase>;
  },

  streamPgBackupRestoreFromFile: (
    id: string,
    form: {
      file: File;
      target_database_name: string;
      create_database?: boolean;
      drop_existing?: boolean;
    },
    signal?: AbortSignal,
  ) => {
    const body = new FormData();
    body.append("file", form.file);
    body.append("target_database_name", form.target_database_name);
    body.append(
      "create_database",
      form.create_database === false ? "false" : "true",
    );
    body.append(
      "drop_existing",
      form.drop_existing ? "true" : "false",
    );
    return fetch(
      `${getApiBase()}/api/databases/${id}/restore-upload?stream=1`,
      {
        method: "POST",
        headers: {
          ...authHeaders(),
          Accept: "text/event-stream",
        },
        body,
        signal,
      },
    );
  },

  getPanelBackupSettings: () =>
    request<PanelBackupSettings>("/api/backups/settings"),

  updatePanelBackupSettings: (body: UpdatePanelBackupSettings) =>
    request<PanelBackupSettings>("/api/backups/settings", {
      method: "PUT",
      body: JSON.stringify(body),
    }),

  testPanelBackupS3: (body: TestPanelS3Request = {}) =>
    request<TestPanelS3Response>("/api/backups/settings/test", {
      method: "POST",
      body: JSON.stringify(body),
    }),

  listBackupOperations: (limit = 20) =>
    request<BackupOperation[]>(`/api/backups/operations?limit=${limit}`),

  listFullPanelBackups: () =>
    request<FullPanelBackup[]>("/api/backups/full"),

  createFullPanelBackup: () =>
    request<FullPanelBackup>("/api/backups/full", { method: "POST" }),

  restoreFullPanelBackup: (body: { s3_key: string }) =>
    request<{ status: string }>("/api/backups/full/restore", {
      method: "POST",
      body: JSON.stringify(body),
    }),

  streamFullPanelBackupRestore: (s3Key: string) =>
    new EventSource(
      streamURL(
        `/api/backups/full/restore/stream?s3_key=${encodeURIComponent(s3Key)}`,
      ),
    ),

  listBillingAccounts: () =>
    request<BillingAccount[]>("/api/billing/accounts"),

  createBillingAccount: (body: CreateBillingAccountRequest) =>
    request<BillingAccount>("/api/billing/accounts", {
      method: "POST",
      body: JSON.stringify(body),
    }),

  updateBillingAccount: (id: string, body: UpdateBillingAccountRequest) =>
    request<BillingAccount>(`/api/billing/accounts/${id}`, {
      method: "PATCH",
      body: JSON.stringify(body),
    }),

  deleteBillingAccount: (id: string) =>
    request<void>(`/api/billing/accounts/${id}`, { method: "DELETE" }),

  refreshBillingAccount: (id: string) =>
    request<BillingAccount>(`/api/billing/accounts/${id}/refresh`, {
      method: "POST",
    }),

  getServersSettings: () => request<ServersSettings>("/api/servers/settings"),

  updateServersSettings: (body: UpdateServersSettingsRequest) =>
    request<ServersSettings>("/api/servers/settings", {
      method: "PUT",
      body: JSON.stringify(body),
    }),

  getServersOverview: () => request<ServersOverview>("/api/servers/overview"),

  listServerNodes: () => request<ServerNode[]>("/api/servers/nodes"),

  getServerNode: (id: string) => request<ServerNode>(`/api/servers/nodes/${id}`),

  updateServerNode: (id: string, body: UpdateServerNodeRequest) =>
    request<ServerNode>(`/api/servers/nodes/${id}`, {
      method: "PATCH",
      body: JSON.stringify(body),
    }),

  updateServerNodeBilling: (id: string, body: UpdateServerNodeBillingRequest) =>
    request<ServerNode>(`/api/servers/nodes/${id}/billing`, {
      method: "PUT",
      body: JSON.stringify(body),
    }),

  deleteServerNode: (id: string) =>
    request<void>(`/api/servers/nodes/${id}`, { method: "DELETE" }),

  pairBarnNode: (body: PairBarnRequest) =>
    request<ServerNode>("/api/servers/nodes/barn", {
      method: "POST",
      body: JSON.stringify(body),
    }),

  createServersPairingCode: () =>
    request<ServersPairingCode>("/api/servers/pairing-code", { method: "POST" }),

  disconnectServersMaster: () =>
    request<void>("/api/servers/master", { method: "DELETE" }),

  listServerEvents: () => request<ServerEvent[]>("/api/servers/events"),

  listServerIncidents: () => request<ServerIncident[]>("/api/servers/incidents"),

  startAgentInstall: (body: CreateAgentInstallRequest) =>
    request<ServerInstallation>("/api/servers/installations/agent", {
      method: "POST",
      body: JSON.stringify(body),
    }),

  startAgentUpdate: (nodeId: string, body: UpdateAgentRequest) =>
    request<ServerInstallation>(`/api/servers/nodes/${nodeId}/update-agent`, {
      method: "POST",
      body: JSON.stringify(body),
    }),

  confirmAgentInstallHostKey: (id: string) =>
    request<ServerInstallation>(
      `/api/servers/installations/${id}/confirm-host-key`,
      { method: "POST" },
    ),

  getAgentInstall: (id: string) =>
    request<ServerInstallation>(`/api/servers/installations/${id}`),

  cancelAgentInstall: (id: string) =>
    request<void>(`/api/servers/installations/${id}`, { method: "DELETE" }),

  listAgentInstallLogs: (id: string) =>
    request<ServerInstallationLog[]>(`/api/servers/installations/${id}/logs`),
};

export async function exchangeQRCode(code: string): Promise<string> {
  const res = await fetch(`${getApiBase()}/api/auth/qr/exchange`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ code }),
  });

  if (!res.ok) {
    let message = res.statusText;
    try {
      const body = await res.json();
      if (body?.error) message = body.error;
    } catch {
      /* ignore */
    }
    throw new ApiError(message, res.status);
  }

  const body = (await res.json()) as { token: string };
  return body.token;
}

export async function createQRSessionWithToken(
  token: string,
): Promise<{ code: string; expires_at: string }> {
  const res = await fetch(`${getApiBase()}/api/auth/qr`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${token.trim()}`,
    },
  });

  if (!res.ok) {
    let message = res.statusText;
    try {
      const body = await res.json();
      if (body?.error) message = body.error;
    } catch {
      /* ignore */
    }
    throw new ApiError(message, res.status);
  }

  return res.json() as Promise<{ code: string; expires_at: string }>;
}
