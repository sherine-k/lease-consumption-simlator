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

		// For new template format: jobs with OnReleaseController set trigger on both cron and releases
		// For old format: jobs are either cron OR release-controller
		if len(job.OnReleaseController) > 0 {
			// Template format: generate both cron and release instances
			cronInstances := s.generateCronInstances(job)
			instances = append(instances, cronInstances...)
			releaseControllerJobs = append(releaseControllerJobs, job)
		} else {
			// Old format or template jobs without release controller
			switch job.TriggerType {
			case config.TriggerTypeCron:
				cronInstances := s.generateCronInstances(job)
				instances = append(instances, cronInstances...)
			case config.TriggerTypeReleaseController:
				releaseControllerJobs = append(releaseControllerJobs, job)
			}
		}
	}

	// Generate instances for all release controller jobs
	if len(releaseControllerJobs) > 0 {
		rcInstances := s.generateReleaseControllerInstances(releaseControllerJobs)
		instances = append(instances, rcInstances...)
	}

	// Generate developer session instances
	if s.config.DevLeaseBuffer > 0 {
		devSessions := s.generateDeveloperSessions()
		instances = append(instances, devSessions...)
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
// Frequency depends on version category:
// - Dev: 4-8 hours
// - Supported: 4-24 hours
// - EUS: 4 hours - 2 days
func (s *Simulator) generateReleaseEvents(category config.VersionCategory) []time.Time {
	releaseEvents := []time.Time{}

	// Determine frequency range based on version category
	var minHours, maxHours int
	switch category {
	case config.VersionCategoryDev:
		minHours = 4
		maxHours = 8
	case config.VersionCategorySupported:
		minHours = 4
		maxHours = 24
	case config.VersionCategoryEus:
		minHours = 4
		maxHours = 48 // 2 days
	default:
		// Fallback to dev timings
		minHours = 4
		maxHours = 8
	}

	// Start at a random offset from 0 to maxHours
	initialOffsetHours := rand.Intn(maxHours + 1)
	currentTime := s.simulationStart.Add(time.Duration(initialOffsetHours) * time.Hour)

	for currentTime.Before(s.simulationEnd) {
		releaseEvents = append(releaseEvents, currentTime)

		// Random interval within the range
		intervalHours := minHours + rand.Intn(maxHours-minHours+1)
		currentTime = currentTime.Add(time.Duration(intervalHours) * time.Hour)
	}

	return releaseEvents
}

// generateReleaseControllerInstances generates job instances for all release controller jobs
// Jobs are grouped by version, and each version has independent release events
func (s *Simulator) generateReleaseControllerInstances(jobs []*config.Job) []*config.JobInstance {
	instances := []*config.JobInstance{}

	// Group jobs by version
	type versionInfo struct {
		jobs     []*config.Job
		category config.VersionCategory
	}
	jobsByVersion := make(map[string]*versionInfo)

	for _, job := range jobs {
		if jobsByVersion[job.Version] == nil {
			jobsByVersion[job.Version] = &versionInfo{
				jobs:     []*config.Job{},
				category: job.VersionCategory,
			}
		}
		jobsByVersion[job.Version].jobs = append(jobsByVersion[job.Version].jobs, job)
	}

	// For each version, generate independent release events based on its category
	for _, info := range jobsByVersion {
		// Generate release event times for this version using its category
		releaseEvents := s.generateReleaseEvents(info.category)

		// For each release event, create instances for ALL jobs in this version
		for _, releaseTime := range releaseEvents {
			for _, job := range info.jobs {
				instances = append(instances, &config.JobInstance{
					Job:       job,
					StartTime: releaseTime,
					EndTime:   releaseTime.Add(job.Duration),
				})
			}
		}
	}

	return instances
}

// generateDeveloperSessions generates synthetic developer testing session instances
// These represent ad-hoc developer usage of leases for testing
func (s *Simulator) generateDeveloperSessions() []*config.JobInstance {
	instances := []*config.JobInstance{}

	if s.config.DevLeaseBuffer == 0 {
		return instances
	}

	// Calculate how many sessions to generate
	// Target: 40% average utilization of devLeaseBuffer
	targetUtilization := 0.4
	targetConcurrentSessions := float64(s.config.DevLeaseBuffer) * targetUtilization

	// Average session duration from mean
	avgSessionDuration := s.config.MeanDuration

	// Total dev-hours needed across simulation
	simulationHours := s.config.SimulationDuration.Hours()
	totalDevHours := simulationHours * targetConcurrentSessions

	// Number of sessions to generate
	numSessions := int(totalDevHours / avgSessionDuration.Hours())

	// Create a synthetic "developer" job for duration calculation
	devJob := &config.Job{
		Name:         "developer-session",
		Version:      "dev",
		Scenario:     "developer-testing",
		PayloadType:  "dev",
		MeanDuration: s.config.MeanDuration,
		StdDev:       s.config.JobDurationStandardDeviation,
		TriggerType:  config.TriggerTypeCron, // Doesn't matter, won't be used
	}

	// Create RNG for generating random values
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Generate random session start times across simulation period
	for i := 0; i < numSessions; i++ {
		// Random start time within simulation period
		randomOffsetSeconds := rng.Int63n(int64(s.config.SimulationDuration.Seconds()))
		startTime := s.simulationStart.Add(time.Duration(randomOffsetSeconds) * time.Second)

		// Calculate duration using Gaussian distribution
		duration := config.CalculateGaussianDuration(rng, s.config.MeanDuration, s.config.JobDurationStandardDeviation)
		endTime := startTime.Add(duration)

		// Only include if it starts before simulation end
		if startTime.Before(s.simulationEnd) {
			instances = append(instances, &config.JobInstance{
				Job:       devJob,
				StartTime: startTime,
				EndTime:   endTime,
			})
		}
	}

	return instances
}

// assignLeaseToWaitingJob assigns a freed lease to a waiting job
func (s *Simulator) assignLeaseToWaitingJob(waitingJob *config.JobInstance, currentTime time.Time, activeJobs *[]*config.JobInstance, activeLeases *int) {
	waitingJob.LeaseAcquired = true
	waitingJob.ActualStartTime = currentTime
	waitingJob.EndTime = currentTime.Add(waitingJob.Job.Duration)
	*activeLeases++
	*activeJobs = append(*activeJobs, waitingJob)

	s.addEvent(Event{
		Time:         currentTime,
		Type:         EventTypeLeaseAcquired,
		JobInstance:  waitingJob,
		ActiveLeases: *activeLeases,
		Message:      fmt.Sprintf("Job '%s' acquired lease after waiting %s", waitingJob.Job.Name, waitingJob.LeaseWaitTime),
		WasWaiting:   true,
	})
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
				job.ActualStartTime = currentTime // Track when job actually started running
				job.EndTime = currentTime.Add(job.Job.Duration) // Update EndTime based on actual start
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

		// Check for job execution timeouts FIRST (before completions)
		// Only timeout jobs whose Duration exceeds the timeout threshold
		// (jobs that complete naturally before timeout should not timeout)
		stillRunning := []*config.JobInstance{}
		for _, job := range activeJobs {
			// Only timeout if:
			// 1. Job has started (ActualStartTime is set)
			// 2. Job's calculated Duration exceeds timeout threshold
			// 3. Runtime has reached or exceeded the timeout
			// 4. Job hasn't already timed out
			runtime := currentTime.Sub(job.ActualStartTime)
			if !job.ActualStartTime.IsZero() &&
			   job.Job.Duration > s.config.JobTimeoutDuration &&
			   runtime >= s.config.JobTimeoutDuration &&
			   !job.TimedOut {
				job.TimedOut = true
				activeLeases--
				s.addEvent(Event{
					Time:         currentTime,
					Type:         EventTypeJobTimeout,
					JobInstance:  job,
					ActiveLeases: activeLeases,
					Message:      fmt.Sprintf("Job '%s' exceeded execution timeout (%s), Duration was %s", job.Job.Name, s.config.JobTimeoutDuration, job.Job.Duration),
					IsWarning:    true,
				})

				// Try to assign the released lease to a waiting job
				if len(waitingJobs) > 0 {
					waitingJob := waitingJobs[0]
					waitingJobs = waitingJobs[1:]
					s.assignLeaseToWaitingJob(waitingJob, currentTime, &stillRunning, &activeLeases)
				}
			} else {
				stillRunning = append(stillRunning, job)
			}
		}
		activeJobs = stillRunning

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
					s.assignLeaseToWaitingJob(waitingJob, currentTime, &remainingJobs, &activeLeases)
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

				// Find peak capacity during wait period to provide context
				peakLeases := s.getPeakLeasesDuringPeriod(job.StartTime, currentTime)

				s.addEvent(Event{
					Time:         currentTime,
					Type:         EventTypeJobTimeout,
					JobInstance:  job,
					ActiveLeases: activeLeases,
					Message:      fmt.Sprintf("Job '%s' timed out waiting for lease (waited %s, peak capacity during wait: %d/%d) - lease released", job.Job.Name, job.LeaseWaitTime, peakLeases, s.config.MaxActiveLeases),
					IsWarning:    true,
					WasWaiting:   true,
				})
			} else {
				remainingWaitingJobs = append(remainingWaitingJobs, job)
			}
		}
		waitingJobs = remainingWaitingJobs

		// Assign any remaining available leases to waiting jobs
		// This handles cases where capacity is freed but not all waiting jobs got leases
		for len(waitingJobs) > 0 && activeLeases < s.config.MaxActiveLeases {
			waitingJob := waitingJobs[0]
			waitingJobs = waitingJobs[1:]
			s.assignLeaseToWaitingJob(waitingJob, currentTime, &activeJobs, &activeLeases)
		}

		// Move to next time step (5 minute intervals)
		currentTime = currentTime.Add(5 * time.Minute)

		if jobIndex >= len(jobInstances) && len(activeJobs) == 0 && len(waitingJobs) == 0 {
			break
		}
	}
}

// getPeakLeasesDuringPeriod returns the peak active leases during a time period
func (s *Simulator) getPeakLeasesDuringPeriod(startTime, endTime time.Time) int {
	peakLeases := 0

	for _, event := range s.events {
		// Only consider events within the time period
		if (event.Time.After(startTime) || event.Time.Equal(startTime)) &&
			(event.Time.Before(endTime) || event.Time.Equal(endTime)) {
			if event.ActiveLeases > peakLeases {
				peakLeases = event.ActiveLeases
			}
		}
	}

	return peakLeases
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
			} else if event.WasWaiting {
				// Job was waiting and either got a lease or timed out - decrement waiting count
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

		currentTime = currentTime.Add(15 * time.Minute) // Sample every 15 minutes
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
	// Pre-allocate with estimated capacity (most events won't be warnings, so use smaller capacity)
	warnings := make([]Event, 0, len(s.events)/10)
	for _, event := range s.events {
		if event.IsWarning {
			warnings = append(warnings, event)
		}
	}
	return warnings
}
