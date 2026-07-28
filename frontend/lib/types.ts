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
  top_cpu: SystemProcess[];
  top_mem: SystemProcess[];
  docker: SystemDockerUsage;
  docker_dirs?: SystemDockerDir[];
  checked_at: string;
}

export interface DockerPruneResult {
  containers_deleted: number;
  images_deleted: number;
  space_reclaimed: number;
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
}

export interface PgDatabase {
  id: string;
  instance_id: string;
  name: string;
  owner_role: string;
  created_at: string;
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
  retention_count: number;
  last_run_at?: string | null;
  last_status: string;
  created_at: string;
  updated_at: string;
}

export interface PgBackup {
  id: string;
  instance_id: string;
  database_id?: string | null;
  schedule_id?: string | null;
  database_name: string;
  status: string;
  s3_endpoint: string;
  s3_region: string;
  s3_bucket: string;
  s3_key: string;
  s3_force_path_style: boolean;
  size_bytes: number;
  message: string;
  created_at: string;
  finished_at?: string | null;
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
  s3_bucket: string;
  s3_prefix?: string;
  s3_access_key: string;
  s3_secret_key: string;
  s3_force_path_style?: boolean;
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
  target_database_name?: string;
  create_database?: boolean;
  drop_existing?: boolean;
  s3_access_key?: string;
  s3_secret_key?: string;
}
