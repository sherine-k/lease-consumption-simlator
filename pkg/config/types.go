package config

import (
	"time"
)

// Config represents the entire configuration for the lease simulator
type Config struct {
	MaxActiveLeases      int           `yaml:"maxActiveLeases"`
	JobTimeoutDuration   time.Duration `yaml:"jobTimeoutDuration"`
	LeaseWaitTimeout     time.Duration `yaml:"leaseWaitTimeout"`
	SimulationDuration   time.Duration `yaml:"simulationDuration"`
	Jobs                 []Job         `yaml:"jobs"`
}

// Job represents a single CI job
type Job struct {
	Name        string        `yaml:"name"`
	Version     string        `yaml:"version"`
	Scenario    string        `yaml:"scenario"`
	PayloadType string        `yaml:"payloadType"`
	Duration    time.Duration `yaml:"duration"`
	TriggerType TriggerType   `yaml:"triggerType"`

	// For cron-based jobs
	CronSchedule string `yaml:"cronSchedule,omitempty"`

	// For release controller jobs
	// These are considered as "always reserved" leases
	IsReleaseController bool `yaml:"isReleaseController,omitempty"`
}

// TriggerType defines how a job is triggered
type TriggerType string

const (
	TriggerTypeCron              TriggerType = "cron"
	TriggerTypeReleaseController TriggerType = "release-controller"
)

// JobInstance represents a specific execution of a job
type JobInstance struct {
	Job       *Job
	StartTime time.Time
	EndTime   time.Time
	LeaseAcquired bool
	LeaseWaitTime time.Duration
	TimedOut   bool
}
