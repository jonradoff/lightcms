package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ImportSource is a configured recurring RSS/Atom feed source
type ImportSource struct {
	ID            primitive.ObjectID  `bson:"_id,omitempty" json:"id,omitempty"`
	Name          string              `bson:"name" json:"name"`
	URL           string              `bson:"url" json:"url"`           // RSS feed URL
	TemplateID    primitive.ObjectID  `bson:"template_id,omitempty" json:"template_id,omitempty"`
	TemplateName  string              `bson:"template_name,omitempty" json:"template_name,omitempty"`
	FolderPath    string              `bson:"folder_path,omitempty" json:"folder_path,omitempty"`
	AutoPublish   bool                `bson:"auto_publish" json:"auto_publish"`
	Schedule      string              `bson:"schedule" json:"schedule"` // "hourly", "daily", "weekly"
	Active        bool                `bson:"active" json:"active"`
	LastRunAt     *time.Time          `bson:"last_run_at,omitempty" json:"last_run_at,omitempty"`
	LastRunStatus string              `bson:"last_run_status,omitempty" json:"last_run_status,omitempty"` // "ok", "failed"
	LastJobID     primitive.ObjectID  `bson:"last_job_id,omitempty" json:"last_job_id,omitempty"`
	CreatedAt     time.Time           `bson:"created_at" json:"created_at"`
	UpdatedAt     time.Time           `bson:"updated_at" json:"updated_at"`
	CreatedBy     primitive.ObjectID  `bson:"created_by,omitempty" json:"created_by,omitempty"`
}

// ImportType defines the type of import
type ImportType string

const (
	ImportTypeRSS      ImportType = "rss"
	ImportTypeCSV      ImportType = "csv"
	ImportTypeMarkdown ImportType = "markdown"
)

// ImportStatus defines the status of an import job
type ImportStatus string

const (
	ImportStatusRunning   ImportStatus = "running"
	ImportStatusDone      ImportStatus = "done"
	ImportStatusFailed    ImportStatus = "failed"
	ImportStatusCancelled ImportStatus = "cancelled"
)

// ImportJob represents one execution of an import
type ImportJob struct {
	ID         primitive.ObjectID  `bson:"_id,omitempty" json:"id,omitempty"`
	SourceID   *primitive.ObjectID `bson:"source_id,omitempty" json:"source_id,omitempty"` // nil for manual uploads
	SourceName string              `bson:"source_name,omitempty" json:"source_name,omitempty"`
	Type       ImportType          `bson:"type" json:"type"`
	Status     ImportStatus        `bson:"status" json:"status"`
	TotalPages int                 `bson:"total_pages" json:"total_pages"`
	Created    int                 `bson:"created" json:"created"`
	Updated    int                 `bson:"updated" json:"updated"`
	Failed     int                 `bson:"failed" json:"failed"`
	Skipped    int                 `bson:"skipped" json:"skipped"`
	ErrorMsg   string              `bson:"error_msg,omitempty" json:"error_msg,omitempty"`
	StartedAt  time.Time           `bson:"started_at" json:"started_at"`
	FinishedAt *time.Time          `bson:"finished_at,omitempty" json:"finished_at,omitempty"`
	CreatedBy  string              `bson:"created_by,omitempty" json:"created_by,omitempty"` // email or "scheduler"
}

// ImportLogLevel is the severity of a log line
type ImportLogLevel string

const (
	ImportLogInfo  ImportLogLevel = "info"
	ImportLogWarn  ImportLogLevel = "warn"
	ImportLogError ImportLogLevel = "error"
)

// ImportLog is one log line for an import job
type ImportLog struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	JobID     primitive.ObjectID `bson:"job_id" json:"job_id"`
	Seq       int                `bson:"seq" json:"seq"`
	Level     ImportLogLevel     `bson:"level" json:"level"`
	Message   string             `bson:"message" json:"message"`
	Path      string             `bson:"path,omitempty" json:"path,omitempty"` // content path affected
	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
}
