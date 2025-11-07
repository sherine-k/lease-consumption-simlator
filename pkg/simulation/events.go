package simulation

import (
	"time"

	"github.com/sherine-k/leases/pkg/config"
)

// EventType defines the type of event in the simulation
type EventType string

const (
	EventTypeLeaseAcquired EventType = "lease-acquired"
	EventTypeLeaseReleased EventType = "lease-released"
	EventTypeJobWaiting    EventType = "job-waiting"
	EventTypeJobTimeout    EventType = "job-timeout"
	EventTypeMaxExceeded   EventType = "max-exceeded"
)

// Event represents a point-in-time event in the simulation
type Event struct {
	Time         time.Time
	Type         EventType
	JobInstance  *config.JobInstance
	ActiveLeases int
	Message      string
	IsWarning    bool
}

// TimePoint represents the state at a specific point in time
type TimePoint struct {
	Time         time.Time
	ActiveLeases int
	WaitingJobs  int
}
