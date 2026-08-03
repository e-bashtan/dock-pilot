package panelbackup

import "time"

type SettingsResponse struct {
	Enabled           bool       `json:"enabled"`
	Hour              int        `json:"hour"`
	Minute            int        `json:"minute"`
	Timezone          string     `json:"timezone"`
	S3Endpoint        string     `json:"s3_endpoint"`
	S3Region          string     `json:"s3_region"`
	S3Bucket          string     `json:"s3_bucket"`
	S3Prefix          string     `json:"s3_prefix"`
	S3ForcePathStyle  bool       `json:"s3_force_path_style"`
	S3CredentialsSet  bool       `json:"s3_credentials_set"`
	RetentionCount    int        `json:"retention_count"`
	LastRunAt         *time.Time `json:"last_run_at"`
	LastStatus        string     `json:"last_status"`
	LastError         string     `json:"last_error"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

type UpdateSettingsRequest struct {
	Enabled            bool   `json:"enabled"`
	Hour               int    `json:"hour"`
	Minute             int    `json:"minute"`
	Timezone           string `json:"timezone"`
	S3Endpoint         string `json:"s3_endpoint"`
	S3Region           string `json:"s3_region"`
	S3Bucket           string `json:"s3_bucket"`
	S3Prefix           string `json:"s3_prefix"`
	S3AccessKey        string `json:"s3_access_key"`
	S3SecretKey        string `json:"s3_secret_key"`
	ClearS3Credentials bool   `json:"clear_s3_credentials"`
	S3ForcePathStyle   bool   `json:"s3_force_path_style"`
	RetentionCount     int    `json:"retention_count"`
}

type FullBackupInfo struct {
	S3Key     string    `json:"s3_key"`
	SizeBytes int64     `json:"size_bytes"`
	CreatedAt time.Time `json:"created_at"`
}

type RestoreFullRequest struct {
	S3Key string `json:"s3_key"`
}

type Manifest struct {
	Version      string   `json:"version"`
	CreatedAt    string   `json:"created_at"`
	Hostname     string   `json:"hostname"`
	SiteSlugs    []string `json:"site_slugs"`
	ManagedDBs   []string `json:"managed_databases"`
	PanelDBName  string   `json:"panel_database"`
}

type TestS3Request struct {
	S3Endpoint       string `json:"s3_endpoint"`
	S3Region         string `json:"s3_region"`
	S3Bucket         string `json:"s3_bucket"`
	S3AccessKey      string `json:"s3_access_key"`
	S3SecretKey      string `json:"s3_secret_key"`
	S3ForcePathStyle bool   `json:"s3_force_path_style"`
}

type TestS3Response struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

type OperationResponse struct {
	ID           string     `json:"id"`
	Kind         string     `json:"kind"`
	Status       string     `json:"status"`
	DatabaseName string     `json:"database_name"`
	InstanceID   *string    `json:"instance_id,omitempty"`
	ScheduleID   *string    `json:"schedule_id,omitempty"`
	S3Key        string     `json:"s3_key"`
	SizeBytes    int64      `json:"size_bytes"`
	Message      string     `json:"message"`
	StartedAt    time.Time  `json:"started_at"`
	FinishedAt   *time.Time `json:"finished_at"`
}
