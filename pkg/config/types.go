package config

import (
	"time"
)

// Config represents the entire configuration for the lease simulator
type Config struct {
	MaxActiveLeases    int           `yaml:"maxActiveLeases"`
	JobTimeoutDuration time.Duration `yaml:"jobTimeoutDuration"`
	LeaseWaitTimeout   time.Duration `yaml:"leaseWaitTimeout"`
	SimulationDuration time.Duration `yaml:"simulationDuration"`

	// Template-based job configuration (new format)
	DevVersions                  int           `yaml:"devVersions,omitempty"`
	SupportedVersions            int           `yaml:"supportedVersions,omitempty"`
	EusVersions                  int           `yaml:"eusVersions,omitempty"`
	DevLeaseBuffer               int           `yaml:"devLeaseBuffer,omitempty"`
	MeanDuration                 time.Duration `yaml:"meanDuration,omitempty"`
	JobDurationStandardDeviation time.Duration `yaml:"jobDurationStdDev,omitempty"`

	// Release controller interval configuration (Gaussian distribution)
	DevReleaseIntervalMean       time.Duration `yaml:"devReleaseIntervalMean,omitempty"`
	DevReleaseIntervalStdDev     time.Duration `yaml:"devReleaseIntervalStdDev,omitempty"`
	SupportedReleaseIntervalMean time.Duration `yaml:"supportedReleaseIntervalMean,omitempty"`
	SupportedReleaseIntervalStdDev time.Duration `yaml:"supportedReleaseIntervalStdDev,omitempty"`
	EusReleaseIntervalMean       time.Duration `yaml:"eusReleaseIntervalMean,omitempty"`
	EusReleaseIntervalStdDev     time.Duration `yaml:"eusReleaseIntervalStdDev,omitempty"`

	Jobs []Job `yaml:"jobs"`
}

// Job represents a single CI job
type Job struct {
	Name        string      `yaml:"name"`
	Version     string      `yaml:"version,omitempty"`
	Scenario    string      `yaml:"scenario,omitempty"`
	PayloadType string      `yaml:"payloadType,omitempty"`
	TriggerType TriggerType `yaml:"triggerType,omitempty"`

	// For cron-based jobs
	CronSchedule string `yaml:"cronSchedule,omitempty"`

	// Template-based configuration (new format)
	OnReleaseController []VersionCategory `yaml:"onReleaseController,omitempty"`
	VersionCategory     VersionCategory   `yaml:"-"` // Set during job expansion (dev/supported/eus)
}

// TriggerType defines how a job is triggered
type TriggerType string

const (
	TriggerTypeCron              TriggerType = "cron"
	TriggerTypeReleaseController TriggerType = "release-controller"
)

// VersionCategory defines the category of an OpenShift version
type VersionCategory string

const (
	VersionCategoryDev       VersionCategory = "dev"
	VersionCategorySupported VersionCategory = "supported"
	VersionCategoryEus       VersionCategory = "eus"
)

// JobInstance represents a specific execution of a job
type JobInstance struct {
	Job             *Job
	StartTime       time.Time // Original scheduled start time (never changes)
	ActualStartTime time.Time // When the job actually started running (after acquiring lease)
	EndTime         time.Time
	Duration        time.Duration // Expected duration of the job (calculated once at instance creation)
	LeaseAcquired   bool
	LeaseWaitTime   time.Duration
	TimedOut        bool
}
