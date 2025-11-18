package simulation

import (
	"fmt"
	"math/rand"
	"sort"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/sherine-k/leases/pkg/config"
)

// Simulator runs the lease simulation
type Simulator struct {
	config          *config.Config
	events          []Event
	timePoints      []TimePoint
	currentTime     time.Time
	simulationStart time.Time
	simulationEnd   time.Time
}

// NewSimulator creates a new simulator
func NewSimulator(cfg *config.Config) *Simulator {
	// Calculate last Monday at midnight
	now := time.Now()
	weekday := now.Weekday()
	var daysBack int
	if weekday == time.Sunday {
		daysBack = 6 // Sunday is 6 days after Monday
	} else {
		daysBack = int(weekday) - 1 // Days since Monday
	}
	lastMondayDate := now.AddDate(0, 0, -daysBack)
	lastMonday := time.Date(lastMondayDate.Year(), lastMondayDate.Month(), lastMondayDate.Day(), 0, 0, 0, 0, time.Local)

	return &Simulator{
		config:          cfg,
		events:          []Event{},
		timePoints:      []TimePoint{},
		currentTime:     lastMonday,
		simulationStart: lastMonday,
		simulationEnd:   lastMonday.Add(cfg.SimulationDuration),
	}
}

// Run executes the simulation
func (s *Simulator) Run() error {
	// Generate all job instances for the simulation period
	jobInstances := s.generateJobInstances()

	// Sort job instances by start time
	sort.Slice(jobInstances, func(i, j int) bool {
		return jobInstances[i].StartTime.Before(jobInstances[j].StartTime)
	})

	// Run the simulation
	s.simulateLeaseUsage(jobInstances)

	// Generate time points for charting
	s.generateTimePoints()

	return nil
}

// generateJobInstances generates all job instances for the simulation period
func (s *Simulator) generateJobInstances() []*config.JobInstance {
	instances := []*config.JobInstance{}
	releaseControllerJobs := []*config.Job{}

	for i := range s.config.Jobs {
		job := &s.config.Jobs[i]

		switch job.TriggerType {
		case config.TriggerTypeCron:
			// Parse cron schedule and generate instances
			cronInstances := s.generateCronInstances(job)
			instances = append(instances, cronInstances...)
		case config.TriggerTypeReleaseController:
			// Collect all release controller jobs to process together
			releaseControllerJobs = append(releaseControllerJobs, job)
		}
	}

	// Generate instances for all release controller jobs at the same release events
	if len(releaseControllerJobs) > 0 {
		rcInstances := s.generateReleaseControllerInstances(releaseControllerJobs)
		instances = append(instances, rcInstances...)
	}

	return instances
}

// generateCronInstances generates job instances based on cron schedule
func (s *Simulator) generateCronInstances(job *config.Job) []*config.JobInstance {
	instances := []*config.JobInstance{}

	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	schedule, err := parser.Parse(job.CronSchedule)
	if err != nil {
		fmt.Printf("Warning: failed to parse cron schedule for job %s: %v\n", job.Name, err)
		return instances
	}

	currentTime := s.simulationStart
	for currentTime.Before(s.simulationEnd) {
		nextRun := schedule.Next(currentTime)
		if nextRun.After(s.simulationEnd) {
			break
		}

		instances = append(instances, &config.JobInstance{
			Job:       job,
			StartTime: nextRun,
			EndTime:   nextRun.Add(job.Duration),
		})

		currentTime = nextRun.Add(time.Minute) // Move forward to find next occurrence
	}

	return instances
}

// generateReleaseEvents generates release trigger times for a specific version
func (s *Simulator) generateReleaseEvents() []time.Time {
	releaseEvents := []time.Time{}

	// Generate release events at somewhat random intervals
	// Average of one release every 6 hours
	currentTime := s.simulationStart

	for currentTime.Before(s.simulationEnd) {
		releaseEvents = append(releaseEvents, currentTime)

		// Next release in 4-8 hours (random interval, averaging ~6 hours)
		currentTime = currentTime.Add(4*time.Hour + time.Duration(rand.Intn(5))*time.Hour)
	}

	return releaseEvents
}

// generateReleaseControllerInstances generates job instances for all release controller jobs
// Jobs are grouped by version, and each version has independent release events
func (s *Simulator) generateReleaseControllerInstances(jobs []*config.Job) []*config.JobInstance {
	instances := []*config.JobInstance{}

	// Group jobs by version
	jobsByVersion := make(map[string][]*config.Job)
	for _, job := range jobs {
		jobsByVersion[job.Version] = append(jobsByVersion[job.Version], job)
	}

	// For each version, generate independent release events
	for version, versionJobs := range jobsByVersion {
		// Generate release event times for this version
		releaseEvents := s.generateReleaseEvents()

		// For each release event, create instances for ALL jobs in this version
		for _, releaseTime := range releaseEvents {
			for _, job := range versionJobs {
				instances = append(instances, &config.JobInstance{
					Job:       job,
					StartTime: releaseTime,
					EndTime:   releaseTime.Add(job.Duration),
				})
			}
		}

		// Optional: log the number of release events generated for this version
		_ = version // Use version if needed for debugging
	}

	return instances
}

// simulateLeaseUsage simulates the lease usage over time
func (s *Simulator) simulateLeaseUsage(jobInstances []*config.JobInstance) {
	activeLeases := 0
	activeJobs := []*config.JobInstance{}
	waitingJobs := []*config.JobInstance{}

	// Process all job instances
	jobIndex := 0
	currentTime := s.simulationStart

	for currentTime.Before(s.simulationEnd) || len(activeJobs) > 0 || len(waitingJobs) > 0 {
		// Check for jobs that should start
		for jobIndex < len(jobInstances) && (jobInstances[jobIndex].StartTime.Before(currentTime) || jobInstances[jobIndex].StartTime.Equal(currentTime)) {
			job := jobInstances[jobIndex]
			jobIndex++

			// Try to acquire a lease
			availableLeases := s.config.MaxActiveLeases - activeLeases

			if availableLeases > 0 {
				// Lease acquired
				activeLeases++
				job.LeaseAcquired = true
				activeJobs = append(activeJobs, job)

				s.addEvent(Event{
					Time:         currentTime,
					Type:         EventTypeLeaseAcquired,
					JobInstance:  job,
					ActiveLeases: activeLeases,
					Message:      fmt.Sprintf("Job '%s' acquired lease", job.Job.Name),
				})

				// Check if max exceeded
				if activeLeases > s.config.MaxActiveLeases {
					s.addEvent(Event{
						Time:         currentTime,
						Type:         EventTypeMaxExceeded,
						JobInstance:  job,
						ActiveLeases: activeLeases,
						Message:      fmt.Sprintf("Max active leases exceeded: %d/%d", activeLeases, s.config.MaxActiveLeases),
						IsWarning:    true,
					})
				}
			} else {
				// No lease available, job must wait
				waitingJobs = append(waitingJobs, job)
				job.LeaseWaitTime = 0

				s.addEvent(Event{
					Time:         currentTime,
					Type:         EventTypeJobWaiting,
					JobInstance:  job,
					ActiveLeases: activeLeases,
					Message:      fmt.Sprintf("Job '%s' waiting for lease", job.Job.Name),
					IsWarning:    true,
				})
			}
		}

		// Check for jobs that should finish
		remainingJobs := []*config.JobInstance{}
		for _, job := range activeJobs {
			if currentTime.After(job.EndTime) || currentTime.Equal(job.EndTime) {
				// Job completed, release lease
				activeLeases--

				s.addEvent(Event{
					Time:         currentTime,
					Type:         EventTypeLeaseReleased,
					JobInstance:  job,
					ActiveLeases: activeLeases,
					Message:      fmt.Sprintf("Job '%s' completed and released lease", job.Job.Name),
				})

				// Try to assign the released lease to a waiting job
				if len(waitingJobs) > 0 {
					waitingJob := waitingJobs[0]
					waitingJobs = waitingJobs[1:]

					waitingJob.LeaseAcquired = true
					// waitingJob.StartTime = currentTime
					waitingJob.EndTime = currentTime.Add(waitingJob.Job.Duration)
					activeLeases++
					remainingJobs = append(remainingJobs, waitingJob)

					s.addEvent(Event{
						Time:         currentTime,
						Type:         EventTypeLeaseAcquired,
						JobInstance:  waitingJob,
						ActiveLeases: activeLeases,
						Message:      fmt.Sprintf("Job '%s' acquired lease after waiting %s", waitingJob.Job.Name, waitingJob.LeaseWaitTime),
					})
				}
			} else {
				remainingJobs = append(remainingJobs, job)
			}
		}
		activeJobs = remainingJobs

		// Check for waiting job timeouts
		remainingWaitingJobs := []*config.JobInstance{}
		for _, job := range waitingJobs {
			job.LeaseWaitTime += 5 * time.Minute

			if job.LeaseWaitTime >= s.config.LeaseWaitTimeout {
				job.TimedOut = true

				s.addEvent(Event{
					Time:         currentTime,
					Type:         EventTypeJobTimeout,
					JobInstance:  job,
					ActiveLeases: activeLeases,
					Message:      fmt.Sprintf("Job '%s' timed out waiting for lease (waited %s) - lease released", job.Job.Name, job.LeaseWaitTime),
					IsWarning:    true,
				})
			} else {
				remainingWaitingJobs = append(remainingWaitingJobs, job)
			}
		}
		waitingJobs = remainingWaitingJobs

		// Check for job execution timeouts
		stillRunning := []*config.JobInstance{}
		for _, job := range activeJobs {
			if currentTime.Sub(job.StartTime) >= s.config.JobTimeoutDuration && !job.TimedOut {
				job.TimedOut = true
				activeLeases--
				s.addEvent(Event{
					Time:         currentTime,
					Type:         EventTypeJobTimeout,
					JobInstance:  job,
					ActiveLeases: activeLeases,
					Message:      fmt.Sprintf("Job '%s' exceeded execution timeout (%s)", job.Job.Name, s.config.JobTimeoutDuration),
					IsWarning:    true,
				})
			} else {
				stillRunning = append(stillRunning, job)
			}
		}
		activeJobs = stillRunning

		// Move to next time step (5 minute intervals)
		currentTime = currentTime.Add(5 * time.Minute)

		if jobIndex >= len(jobInstances) && len(activeJobs) == 0 && len(waitingJobs) == 0 {
			break
		}
	}
}

// generateTimePoints generates time points for charting
func (s *Simulator) generateTimePoints() {
	if len(s.events) == 0 {
		return
	}

	// Create time points at regular intervals
	currentTime := s.simulationStart
	activeLeases := 0
	waitingJobs := 0

	eventIndex := 0

	for currentTime.Before(s.simulationEnd) || currentTime.Equal(s.simulationEnd) {
		// Process all events up to current time
		for eventIndex < len(s.events) && (s.events[eventIndex].Time.Before(currentTime) || s.events[eventIndex].Time.Equal(currentTime)) {
			event := s.events[eventIndex]
			activeLeases = event.ActiveLeases

			if event.Type == EventTypeJobWaiting {
				waitingJobs++
			} else if event.Type == EventTypeLeaseAcquired {
				if waitingJobs > 0 {
					waitingJobs--
				}
			}

			eventIndex++
		}

		s.timePoints = append(s.timePoints, TimePoint{
			Time:         currentTime,
			ActiveLeases: activeLeases,
			WaitingJobs:  waitingJobs,
		})

		currentTime = currentTime.Add(30 * time.Minute) // Sample every 30 minutes
	}
}

// addEvent adds an event to the event list
func (s *Simulator) addEvent(event Event) {
	s.events = append(s.events, event)
}

// GetEvents returns all events
func (s *Simulator) GetEvents() []Event {
	return s.events
}

// GetTimePoints returns all time points
func (s *Simulator) GetTimePoints() []TimePoint {
	return s.timePoints
}

// GetWarnings returns all warning events
func (s *Simulator) GetWarnings() []Event {
	warnings := []Event{}
	for _, event := range s.events {
		if event.IsWarning {
			warnings = append(warnings, event)
		}
	}
	return warnings
}
