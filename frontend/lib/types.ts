export interface SiteListItem {
  id: string;
  name: string;
  slug: string;
  site_type: string;
  primary_url: string;
  status: string;
  updated_at: string;
}

export interface SiteHealthContainer {
  found: boolean;
  running: boolean;
  state: string;
  health: string;
  container?: string;
}

export interface SiteHealthHTTP {
  url: string;
  status_code?: number;
  ok: boolean;
  error?: string;
}

export interface ContainerActionResult {
  action: string;
  container: {
    found: boolean;
    running: boolean;
    state: string;
    health: string;
    container?: string;
  };
}

export interface ContainerLogLine {
  seq: number;
  stream: string;
  line: string;
  time: string;
}

export interface SiteHealth {
  site_id: string;
  site_type: string;
  overall: string;
  message: string;
  container?: SiteHealthContainer;
  http?: SiteHealthHTTP;
  checked_at: string;
}

export interface PgHealth {
  instance_id: string;
  name: string;
  overall: string;
  message: string;
  container?: SiteHealthContainer;
  ready?: boolean;
  checked_at: string;
}

export interface SystemDisk {
  path: string;
  total_bytes: number;
  used_bytes: number;
  available_bytes: number;
  used_percent: number;
}

export interface SystemMemory {
  total_bytes: number;
  available_bytes: number;
  used_bytes: number;
  used_percent: number;
}

export interface SystemProcess {
  pid: number;
  user: string;
  cpu_percent: number;
  mem_percent: number;
  rss_bytes: number;
  command: string;
}

export interface SystemDockerImage {
  id: string;
  tags: string[];
  size_bytes: number;
  total_bytes: number;
  in_use: boolean;
  dangling: boolean;
}

export interface SystemDockerUsage {
  images_bytes: number;
  containers_bytes: number;
  volumes_bytes: number;
  build_cache_bytes: number;
  reclaimable_bytes: number;
  image_count?: number;
  unused_image_count?: number;
  top_images?: SystemDockerImage[];
}

export interface SystemDockerDir {
  path: string;
  size_bytes: number;
}

export interface SystemStatus {
  disk?: SystemDisk[] | null;
  memory: SystemMemory;
  top_cpu?: SystemProcess[];
  top_mem?: SystemProcess[];
  docker: SystemDockerUsage;
  docker_dirs?: SystemDockerDir[];
  checked_at: string;
}

export interface SystemProcesses {
  top_cpu: SystemProcess[];
  top_mem: SystemProcess[];
  checked_at: string;
}

export interface DockerPruneResult {
  containers_deleted: number;
  images_deleted: number;
  space_reclaimed: number;
}

export interface SystemUpdateInfo {
  current: string;
  latest: string;
  update_available: boolean;
  can_update: boolean;
  reason?: string;
  install_dir: string;
  upgrade_status: string;
  upgrade_target?: string;
  checked_at: string;
}

export interface SystemUpgradeJob {
  status: string;
  target?: string;
  log: string;
  updated_at?: string;
}

export interface SystemUpgradeStart {
  status: string;
  message: string;
}

export interface SystemHostInfo {
  ip: string;
  hostname?: string;
  checked_at: string;
}

export interface NotificationSettings {
  enabled: boolean;
  telegram_chat_id: string;
  telegram_http_proxy: string;
  telegram_bot_token_set: boolean;
  daily_digest_enabled: boolean;
  daily_digest_hour: number;
  daily_digest_timezone: string;
  alert_on_incident_enabled: boolean;
}

export interface UpdateNotificationSettings {
  enabled: boolean;
  telegram_chat_id: string;
  telegram_http_proxy: string;
  telegram_bot_token?: string;
  clear_telegram_bot_token?: boolean;
  daily_digest_enabled: boolean;
  daily_digest_hour: number;
  daily_digest_timezone: string;
  alert_on_incident_enabled: boolean;
}

export interface Domain {
  id?: string;
  domain: string;
  is_primary: boolean;
}

export interface EnvVar {
  key: string;
  value: string;
}

export type SiteType = "web" | "telegram_bot";

export interface Site {
  id: string;
  name: string;
  slug: string;
  site_type: SiteType;
  primary_url: string;
  git_repo_url: string;
  git_branch: string;
  dockerfile_path: string;
  build_context: string;
  container_port: number;
  host_port?: number;
  nginx_ssl_enabled: boolean;
  nginx_force_https: boolean;
  docker_volume_mounts: string[];
  docker_named_volumes: string[];
  docker_network_host: boolean;
  health_check_path: string;
  status: string;
  domains: Domain[];
  env_vars: EnvVar[];
  created_at: string;
  updated_at: string;
}

export interface CreateSiteRequest {
  name: string;
  slug?: string;
  site_type?: SiteType;
  primary_url: string;
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
  domains?: { domain: string; is_primary: boolean }[];
  env_vars?: EnvVar[];
}

export interface SecretMeta {
  key: string;
  created_at: string;
  updated_at: string;
}

export interface Deployment {
  id: string;
  site_id: string;
  status: string;
  message: string;
  started_at?: string;
  finished_at?: string;
  created_at: string;
}

export interface DeploymentLog {
  id: number;
  level: string;
  message: string;
  created_at: string;
}

export interface WizardState {
  siteType: SiteType;
  name: string;
  slug: string;
  primaryUrl: string;
  gitRepoUrl: string;
  gitBranch: string;
  dockerfilePath: string;
  buildContext: string;
  containerPort: number;
  dockerNetworkHost: boolean;
  envVars: EnvVar[];
  secrets: EnvVar[];
  aliases: string[];
  nginxSslEnabled: boolean;
  nginxForceHttps: boolean;
}

export interface PgInstance {
  id: string;
  name: string;
  slug: string;
  image: string;
  container_port: number;
  host_port?: number | null;
  docker_network_host: boolean;
  admin_user: string;
  status: string;
  message: string;
  container_name: string;
  created_at: string;
  updated_at: string;
  password?: string;
}

export interface PgDatabase {
  id: string;
  instance_id: string;
  name: string;
  owner_role: string;
  created_at: string;
}

export interface PgTableInfo {
  name: string;
  approx_rows: number;
}

export interface PgQueryResult {
  columns: string[];
  rows: string[][];
  sql: string;
  limit: number;
}

export interface PgGrant {
  id: string;
  role_id: string;
  database_id: string;
  database_name: string;
  is_owner: boolean;
}

export interface PgRole {
  id: string;
  instance_id: string;
  name: string;
  created_at: string;
  grants: PgGrant[];
  password?: string;
}

export interface PgConnectionInfo {
  host: string;
  port: number;
  database: string;
  user: string;
  password: string;
  url: string;
}

export interface PgBackupSchedule {
  id: string;
  instance_id: string;
  database_id?: string | null;
  enabled: boolean;
  hour: number;
  minute: number;
  timezone: string;
  s3_endpoint: string;
  s3_region: string;
  s3_bucket: string;
  s3_prefix: string;
  s3_force_path_style: boolean;
  use_panel_s3: boolean;
  retention_count: number;
  last_run_at?: string | null;
  last_status: string;
  last_error?: string;
  created_at: string;
  updated_at: string;
}

export interface PgBackup {
  s3_key: string;
  database_name: string;
  status: string;
  s3_endpoint: string;
  s3_region: string;
  s3_bucket: string;
  size_bytes: number;
  created_at: string;
  schedule_id?: string | null;
}

export interface CreatePgInstanceRequest {
  name: string;
  slug?: string;
  image?: string;
  container_port?: number;
  host_port?: number;
  docker_network_host?: boolean;
  admin_user?: string;
  admin_password?: string;
}

export interface CreatePgDatabaseRequest {
  name: string;
  owner_role?: string;
}

export interface CreatePgRoleRequest {
  name: string;
  password?: string;
}

export interface CreatePgScheduleRequest {
  database_id?: string | null;
  enabled?: boolean;
  hour: number;
  minute: number;
  timezone: string;
  s3_endpoint?: string;
  s3_region?: string;
  s3_bucket?: string;
  s3_prefix?: string;
  s3_access_key?: string;
  s3_secret_key?: string;
  s3_force_path_style?: boolean;
  use_panel_s3?: boolean;
  retention_count?: number;
}

export interface ManualPgBackupRequest {
  database_id: string;
  schedule_id?: string;
  s3_endpoint?: string;
  s3_region?: string;
  s3_bucket?: string;
  s3_prefix?: string;
  s3_access_key?: string;
  s3_secret_key?: string;
  s3_force_path_style?: boolean;
}

export interface RestorePgBackupRequest {
  schedule_id: string;
  s3_key: string;
  target_database_name?: string;
  create_database?: boolean;
  drop_existing?: boolean;
}

export interface PanelBackupSettings {
  enabled: boolean;
  hour: number;
  minute: number;
  timezone: string;
  s3_endpoint: string;
  s3_region: string;
  s3_bucket: string;
  s3_prefix: string;
  s3_force_path_style: boolean;
  s3_credentials_set: boolean;
  retention_count: number;
  last_run_at?: string | null;
  last_status: string;
  last_error?: string;
  updated_at: string;
}

export interface UpdatePanelBackupSettings {
  enabled: boolean;
  hour: number;
  minute: number;
  timezone: string;
  s3_endpoint: string;
  s3_region: string;
  s3_bucket: string;
  s3_prefix: string;
  s3_access_key?: string;
  s3_secret_key?: string;
  clear_s3_credentials?: boolean;
  s3_force_path_style: boolean;
  retention_count: number;
}

export interface TestPanelS3Request {
  s3_endpoint?: string;
  s3_region?: string;
  s3_bucket?: string;
  s3_access_key?: string;
  s3_secret_key?: string;
  s3_force_path_style?: boolean;
}

export interface TestPanelS3Response {
  ok: boolean;
  message: string;
}

export interface BackupOperation {
  id: string;
  kind: "panel_snapshot" | "pg_backup" | "pg_restore" | "panel_restore" | string;
  status: "running" | "ok" | "failed" | string;
  database_name: string;
  instance_id?: string | null;
  schedule_id?: string | null;
  s3_key: string;
  size_bytes: number;
  message: string;
  started_at: string;
  finished_at?: string | null;
}

export interface FullPanelBackup {
  s3_key: string;
  size_bytes: number;
  created_at: string;
}

export interface BillingAccount {
  id: string;
  provider: string;
  server_ip: string;
  login: string;
  billmgr_url: string;
  alert_days: number;
  enabled: boolean;
  password_set: boolean;
  expire_date?: string;
  days_left?: number;
  status: string;
  name: string;
  cost: string;
  last_checked_at?: string;
  last_check_error: string;
  last_alert_at?: string;
  created_at: string;
  updated_at: string;
}

export interface CreateBillingAccountRequest {
  provider: string;
  server_ip: string;
  login: string;
  password: string;
  billmgr_url?: string;
  alert_days?: number;
  enabled?: boolean;
}

export interface UpdateBillingAccountRequest {
  provider?: string;
  server_ip?: string;
  login?: string;
  password?: string;
  billmgr_url?: string;
  alert_days?: number;
  enabled?: boolean;
}

export type ServersMode = "standalone" | "master" | "managed_node";
export type ServersNotificationMode = "local" | "master" | "disabled";
export type ServerNodeRole = "master" | "node" | "agent";
export type ServerConnectionType = "local" | "barn" | "dockpilot" | "agent";
export type ServerNodeStatus = "online" | "warning" | "offline";

export interface ServersSettings {
  mode: ServersMode;
  node_uid: string;
  node_name: string;
  public_url: string;
  master_url: string;
  notification_mode: ServersNotificationMode;
  has_master_token: boolean;
}

export interface UpdateServersSettingsRequest {
  mode?: ServersMode;
  node_name?: string;
  public_url?: string;
  master_url?: string;
  notification_mode?: ServersNotificationMode;
  enable_master?: boolean;
  disable_master?: boolean;
}

export interface ServerMetrics {
  cpu_percent: number;
  memory_used_bytes: number;
  memory_total_bytes: number;
  disk_used_percent: number;
  uptime_seconds: number;
  load_1?: number;
  load_5?: number;
  load_15?: number;
}

export interface ServerApplications {
  total: number;
  running: number;
  unhealthy: number;
}

export interface ServerBilling {
  cost_minor: number;
  currency: string;
  period?: string;
  next_due_date?: string;
  auto_renew?: boolean;
  provider_name?: string;
  provider_url?: string;
  monthly_equiv_minor?: number;
  mode?: string;
  billing_account_id?: string;
  server_ip?: string;
  cost_raw?: string;
  days_left?: number;
  alert_days?: number;
}

export interface UpdateServerNodeRequest {
  name: string;
}

export interface UpdateServerNodeBillingRequest {
  billing_account_id?: string;
  cost_minor?: number;
  currency?: string;
  period?: string;
  next_due_date?: string;
  auto_renew?: boolean;
  alert_days?: number;
  provider_name?: string;
  provider_url?: string;
  comment?: string;
}

export interface ServerFilesystem {
  mountpoint: string;
  device: string;
  filesystem: string;
  used_bytes: number;
  total_bytes: number;
  used_percent: number;
}

export interface ServerServiceStatus {
  unit_name: string;
  state: string;
}

export interface ServerNode {
  id: string;
  node_uid: string;
  name: string;
  role: ServerNodeRole;
  connection_type: ServerConnectionType;
  base_url: string;
  status: ServerNodeStatus;
  version: string;
  agent_version?: string;
  last_seen_at?: string;
  capabilities: string[];
  metrics?: ServerMetrics;
  applications?: ServerApplications;
  billing?: ServerBilling;
  open_incidents: number;
  hostname?: string;
  os_name?: string;
  os_version?: string;
  kernel?: string;
  architecture?: string;
  filesystems?: ServerFilesystem[];
  services?: ServerServiceStatus[];
}

export interface ServersOverview {
  servers_total: number;
  servers_online: number;
  servers_warning: number;
  servers_offline: number;
  apps_total: number;
  apps_running: number;
  apps_unhealthy: number;
  monthly_cost_minor: number;
  currency: string;
  next_due_date?: string;
  open_incidents: number;
}

export interface ServerEvent {
  id: string;
  event_id: string;
  node_id?: string;
  node_uid?: string;
  event_type: string;
  severity: string;
  resource_type: string;
  resource_id: string;
  title: string;
  message: string;
  occurred_at: string;
}

export interface ServerIncident {
  id: string;
  node_id?: string;
  event_type: string;
  title: string;
  status: string;
  count: number;
  first_seen_at: string;
  last_seen_at: string;
  resolved_at?: string;
}

export interface ServersPairingCode {
  code: string;
  expires_at: string;
}

export interface PairBarnRequest {
  name: string;
  base_url: string;
  pairing_code: string;
}

export interface CreateAgentInstallRequest {
  kind?: "agent" | "barn";
  name: string;
  host: string;
  port?: number;
  username?: string;
  password: string;
  panel_url?: string;
  email?: string;
  purpose?: string;
  tags?: string[];
  cost_minor?: number;
  currency?: string;
  next_due_date?: string;
  auto_renew?: boolean;
  provider_name?: string;
  provider_url?: string;
}

export interface UpdateAgentRequest {
  host: string;
  port?: number;
  username?: string;
  password: string;
}

export interface ServerInstallation {
  id: string;
  status: string;
  current_step: string;
  ssh_fingerprint: string;
  host: string;
  port: number;
  username: string;
  install_kind?: string;
  panel_url?: string;
  error_code?: string;
  error_message?: string;
  node_id?: string;
}

export interface ServerInstallationLog {
  id: number;
  level: string;
  message: string;
  created_at: string;
}

