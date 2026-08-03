package pgdb

import (
	"time"

	"github.com/google/uuid"
)

type InstanceResponse struct {
	ID                uuid.UUID `json:"id"`
	Name              string    `json:"name"`
	Slug              string    `json:"slug"`
	Image             string    `json:"image"`
	ContainerPort     int       `json:"container_port"`
	HostPort          *int      `json:"host_port"`
	DockerNetworkHost bool      `json:"docker_network_host"`
	AdminUser         string    `json:"admin_user"`
	Status            string    `json:"status"`
	Message           string    `json:"message"`
	ContainerName     string    `json:"container_name"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	// Password is set only on create when generated/returned once.
	Password string `json:"password,omitempty"`
}

type CreateInstanceRequest struct {
	Name              string `json:"name"`
	Slug              string `json:"slug"`
	Image             string `json:"image"`
	ContainerPort     int    `json:"container_port"`
	HostPort          *int   `json:"host_port"`
	DockerNetworkHost bool   `json:"docker_network_host"`
	AdminUser         string `json:"admin_user"`
	AdminPassword     string `json:"admin_password"`
}

type DatabaseResponse struct {
	ID         uuid.UUID `json:"id"`
	InstanceID uuid.UUID `json:"instance_id"`
	Name       string    `json:"name"`
	OwnerRole  string    `json:"owner_role"`
	CreatedAt  time.Time `json:"created_at"`
}

type CreateDatabaseRequest struct {
	Name      string `json:"name"`
	OwnerRole string `json:"owner_role"`
}

type RoleResponse struct {
	ID         uuid.UUID       `json:"id"`
	InstanceID uuid.UUID       `json:"instance_id"`
	Name       string          `json:"name"`
	CreatedAt  time.Time       `json:"created_at"`
	Grants     []GrantResponse `json:"grants"`
	// Password is set only on create when returned once.
	Password string `json:"password,omitempty"`
}

type CreateRoleRequest struct {
	Name     string `json:"name"`
	Password string `json:"password"`
}

type GrantResponse struct {
	ID           uuid.UUID `json:"id"`
	RoleID       uuid.UUID `json:"role_id"`
	DatabaseID   uuid.UUID `json:"database_id"`
	DatabaseName string    `json:"database_name"`
	IsOwner      bool      `json:"is_owner"`
}

type GrantRequest struct {
	DatabaseID uuid.UUID `json:"database_id"`
	IsOwner    bool      `json:"is_owner"`
}

type ScheduleResponse struct {
	ID               uuid.UUID  `json:"id"`
	InstanceID       uuid.UUID  `json:"instance_id"`
	DatabaseID       *uuid.UUID `json:"database_id"`
	Enabled          bool       `json:"enabled"`
	Hour             int        `json:"hour"`
	Minute           int        `json:"minute"`
	Timezone         string     `json:"timezone"`
	S3Endpoint       string     `json:"s3_endpoint"`
	S3Region         string     `json:"s3_region"`
	S3Bucket         string     `json:"s3_bucket"`
	S3Prefix         string     `json:"s3_prefix"`
	S3ForcePathStyle bool       `json:"s3_force_path_style"`
	UsePanelS3       bool       `json:"use_panel_s3"`
	RetentionCount   int        `json:"retention_count"`
	LastRunAt        *time.Time `json:"last_run_at"`
	LastStatus       string     `json:"last_status"`
	LastError        string     `json:"last_error"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

type CreateScheduleRequest struct {
	DatabaseID       *uuid.UUID `json:"database_id"`
	Enabled          *bool      `json:"enabled"`
	Hour             int        `json:"hour"`
	Minute           int        `json:"minute"`
	Timezone         string     `json:"timezone"`
	S3Endpoint       string     `json:"s3_endpoint"`
	S3Region         string     `json:"s3_region"`
	S3Bucket         string     `json:"s3_bucket"`
	S3Prefix         string     `json:"s3_prefix"`
	S3AccessKey      string     `json:"s3_access_key"`
	S3SecretKey      string     `json:"s3_secret_key"`
	S3ForcePathStyle bool       `json:"s3_force_path_style"`
	UsePanelS3       bool       `json:"use_panel_s3"`
	RetentionCount   int        `json:"retention_count"`
}

type UpdateScheduleRequest struct {
	DatabaseID       *uuid.UUID `json:"database_id"`
	ClearDatabaseID  bool       `json:"clear_database_id"`
	Enabled          *bool      `json:"enabled"`
	Hour             *int       `json:"hour"`
	Minute           *int       `json:"minute"`
	Timezone         *string    `json:"timezone"`
	S3Endpoint       *string    `json:"s3_endpoint"`
	S3Region         *string    `json:"s3_region"`
	S3Bucket         *string    `json:"s3_bucket"`
	S3Prefix         *string    `json:"s3_prefix"`
	S3AccessKey      *string    `json:"s3_access_key"`
	S3SecretKey      *string    `json:"s3_secret_key"`
	S3ForcePathStyle *bool      `json:"s3_force_path_style"`
	UsePanelS3       *bool      `json:"use_panel_s3"`
	RetentionCount   *int       `json:"retention_count"`
}

type BackupResponse struct {
	S3Key      string     `json:"s3_key"`
	DatabaseName string   `json:"database_name"`
	Status     string     `json:"status"`
	S3Endpoint string     `json:"s3_endpoint"`
	S3Region   string     `json:"s3_region"`
	S3Bucket   string     `json:"s3_bucket"`
	SizeBytes  int64      `json:"size_bytes"`
	CreatedAt  time.Time  `json:"created_at"`
	ScheduleID *uuid.UUID `json:"schedule_id,omitempty"`
}

type ManualBackupRequest struct {
	DatabaseID       uuid.UUID  `json:"database_id"`
	ScheduleID       *uuid.UUID `json:"schedule_id"`
	S3Endpoint       string     `json:"s3_endpoint"`
	S3Region         string     `json:"s3_region"`
	S3Bucket         string     `json:"s3_bucket"`
	S3Prefix         string     `json:"s3_prefix"`
	S3AccessKey      string     `json:"s3_access_key"`
	S3SecretKey      string     `json:"s3_secret_key"`
	S3ForcePathStyle bool       `json:"s3_force_path_style"`
}

type RestoreRequest struct {
	ScheduleID         uuid.UUID `json:"schedule_id"`
	S3Key              string    `json:"s3_key"`
	TargetDatabaseName string    `json:"target_database_name"`
	CreateDatabase     bool      `json:"create_database"`
	DropExisting       bool      `json:"drop_existing"`
}

// RestoreUploadOptions are form fields for restoring a dump uploaded from the browser.
type RestoreUploadOptions struct {
	TargetDatabaseName string
	CreateDatabase     bool
	DropExisting       bool
}

type ConnectionInfoResponse struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Database string `json:"database"`
	User     string `json:"user"`
	Password string `json:"password"`
	URL      string `json:"url"`
}
