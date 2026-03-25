package simulation

import (
	"testing"
	"time"

	"github.com/sherine-k/leases/pkg/config"
)

// TestCalculateGaussianDuration tests the Gaussian duration calculation
func TestCalculateGaussianDuration(t *testing.T) {
	meanDuration := 5 * time.Hour
	stdDev := 30 * time.Minute

	// Run multiple times to ensure it's always positive and reasonable
	for i := 0; i < 100; i++ {
		duration := calculateGaussianDuration(meanDuration, stdDev)
		if duration <= 0 {
			t.Errorf("iteration %d: expected positive duration, got %v", i, duration)
		}
		// Duration should be at least 10% of mean (the minimum enforced in the function)
		minDuration := time.Duration(meanDuration.Seconds() * 0.1 * float64(time.Second))
		if duration < minDuration {
			t.Errorf("iteration %d: expected duration >= %v, got %v", i, minDuration, duration)
		}
	}
}

// TestSimulatorBasicScheduling tests basic job scheduling with a simple config
func TestSimulatorBasicScheduling(t *testing.T) {
	// Create config matching conf_test.yaml expectations
	cfg := &config.Config{
		MaxActiveLeases:              2,
		JobTimeoutDuration:           5*time.Hour + 15*time.Minute,
		LeaseWaitTimeout:             5 * time.Hour,
		SimulationDuration:           24 * time.Hour,
		DevVersions:                  1,
		SupportedVersions:            0,
		EusVersions:                  0,
		DevLeaseBuffer:               0,
		MeanDuration:                 3*time.Hour + 30*time.Minute,
		JobDurationStandardDeviation: 1 * time.Hour,
		// Release intervals should get defaults
		DevReleaseIntervalMean:       6 * time.Hour,
		DevReleaseIntervalStdDev:       2 * time.Hour,
		SupportedReleaseIntervalMean: 8 * time.Hour,
		SupportedReleaseIntervalStdDev: 4 * time.Hour,
		EusReleaseIntervalMean:       24 * time.Hour,
		EusReleaseIntervalStdDev:       8 * time.Hour,
	}

	// Add job templates (these will be expanded during parsing)
	// Note: In a real scenario, expandJobTemplates would be called by LoadConfig
	// Here we manually create the expanded jobs
	// NOTE: The cron library's Next() returns times AFTER the current time.
	// Since simulation starts at Monday midnight (00:00), a cron scheduled for
	// "0 0 * * *" won't trigger until Tuesday. We use offset times to ensure
	// jobs run within the 24h simulation period.
	cfg.Jobs = []config.Job{
		{
			Name:            "e2e-conformance-dev-1",
			Version:         "dev-1",
			Scenario:        "e2e-conformance",
			PayloadType:     "dev",
			CronSchedule:    "0 6 * * *", // 6 AM daily
			TriggerType:     config.TriggerTypeCron,
			VersionCategory: config.VersionCategoryDev,
		},
		{
			Name:            "e2e-upgrade-dev-1",
			Version:         "dev-1",
			Scenario:        "e2e-upgrade",
			PayloadType:     "dev",
			CronSchedule:    "0 18 * * *", // 6 PM daily
			TriggerType:     config.TriggerTypeCron,
			VersionCategory: config.VersionCategoryDev,
		},
	}

	// Create simulator
	sim := NewSimulator(cfg)

	// Run simulation
	err := sim.Run()
	if err != nil {
		t.Fatalf("simulation failed: %v", err)
	}

	// Verify events were generated
	events := sim.GetEvents()
	if len(events) == 0 {
		t.Fatal("expected events to be generated, got 0")
	}

	// Count lease-acquired events (each job should acquire a lease)
	acquiredCount := 0
	releasedCount := 0
	for _, event := range events {
		switch event.Type {
		case EventTypeLeaseAcquired:
			acquiredCount++
		case EventTypeLeaseReleased:
			releasedCount++
		}
	}

	// We expect exactly 2 jobs to be scheduled in a 24-hour period
	// (one at midnight, one at noon on Monday)
	if acquiredCount != 2 {
		t.Errorf("expected 2 lease acquisitions, got %d", acquiredCount)
	}

	if releasedCount != 2 {
		t.Errorf("expected 2 lease releases, got %d", releasedCount)
	}

	// Verify timing of jobs (6 AM and 6 PM)
	var morningJob, eveningJob *Event
	for i := range events {
		event := &events[i]
		if event.Type == EventTypeLeaseAcquired {
			hour := event.Time.Hour()
			if hour == 6 {
				morningJob = event
			} else if hour == 18 {
				eveningJob = event
			}
		}
	}

	if morningJob == nil {
		t.Error("expected a job to start at 6 AM (hour 6)")
	} else {
		if morningJob.Time.Hour() != 6 || morningJob.Time.Minute() != 0 {
			t.Errorf("morning job should start at 06:00, got %02d:%02d",
				morningJob.Time.Hour(), morningJob.Time.Minute())
		}
	}

	if eveningJob == nil {
		t.Error("expected a job to start at 6 PM (hour 18)")
	} else {
		if eveningJob.Time.Hour() != 18 || eveningJob.Time.Minute() != 0 {
			t.Errorf("evening job should start at 18:00, got %02d:%02d",
				eveningJob.Time.Hour(), eveningJob.Time.Minute())
		}
	}

	// Verify no timeouts occurred (capacity is sufficient for 2 jobs)
	timeoutCount := 0
	for _, event := range events {
		if event.Type == EventTypeJobTimeout || event.Type == EventTypeLeaseWaitTimeout {
			timeoutCount++
		}
	}

	if timeoutCount != 0 {
		t.Errorf("expected no timeouts with sufficient capacity, got %d", timeoutCount)
	}

	// Verify time points were generated
	timePoints := sim.GetTimePoints()
	if len(timePoints) == 0 {
		t.Error("expected time points to be generated")
	}

	// Verify peak usage doesn't exceed max leases
	peakUsage := 0
	for _, tp := range timePoints {
		if tp.ActiveLeases > peakUsage {
			peakUsage = tp.ActiveLeases
		}
		if tp.ActiveLeases > cfg.MaxActiveLeases {
			t.Errorf("time point at %s shows %d active leases, exceeds max of %d",
				tp.Time.Format(time.RFC3339), tp.ActiveLeases, cfg.MaxActiveLeases)
		}
	}

	// Peak should be at most 2 (both jobs running simultaneously)
	if peakUsage > 2 {
		t.Errorf("expected peak usage <= 2, got %d", peakUsage)
	}
}

// TestSimulatorLeaseContention tests behavior when jobs exceed available leases
func TestSimulatorLeaseContention(t *testing.T) {
	// Create config with insufficient capacity
	cfg := &config.Config{
		MaxActiveLeases:              1, // Only 1 lease but 2 jobs will run
		JobTimeoutDuration:           5*time.Hour + 15*time.Minute,
		LeaseWaitTimeout:             5 * time.Hour,
		SimulationDuration:           24 * time.Hour,
		DevVersions:                  1,
		MeanDuration:                 4 * time.Hour, // Long jobs to ensure overlap
		JobDurationStandardDeviation: 30 * time.Minute,
		DevReleaseIntervalMean:       6 * time.Hour,
		DevReleaseIntervalStdDev:       2 * time.Hour,
		SupportedReleaseIntervalMean: 8 * time.Hour,
		SupportedReleaseIntervalStdDev: 4 * time.Hour,
		EusReleaseIntervalMean:       24 * time.Hour,
		EusReleaseIntervalStdDev:       8 * time.Hour,
	}

	// Two jobs scheduled at the same time (10 AM)
	// Using 10 AM to avoid the cron edge case where jobs at midnight won't run
	cfg.Jobs = []config.Job{
		{
			Name:            "job-1",
			Version:         "dev-1",
			CronSchedule:    "0 10 * * *", // Both at 10 AM
			TriggerType:     config.TriggerTypeCron,
			VersionCategory: config.VersionCategoryDev,
		},
		{
			Name:            "job-2",
			Version:         "dev-1",
			CronSchedule:    "0 10 * * *", // Both at 10 AM
			TriggerType:     config.TriggerTypeCron,
			VersionCategory: config.VersionCategoryDev,
		},
	}

	sim := NewSimulator(cfg)
	err := sim.Run()
	if err != nil {
		t.Fatalf("simulation failed: %v", err)
	}

	events := sim.GetEvents()

	// Count waiting events
	waitingCount := 0
	for _, event := range events {
		if event.Type == EventTypeJobWaiting {
			waitingCount++
		}
	}

	// At least one job should have to wait
	if waitingCount == 0 {
		t.Error("expected at least one job to wait when capacity is exceeded")
	}

	// Verify warnings were generated
	warnings := sim.GetWarnings()
	if len(warnings) == 0 {
		t.Error("expected warnings when jobs have to wait for leases")
	}

	// Peak active leases should not exceed configured max
	timePoints := sim.GetTimePoints()
	for _, tp := range timePoints {
		if tp.ActiveLeases > cfg.MaxActiveLeases {
			t.Errorf("active leases %d exceeded max %d at %s",
				tp.ActiveLeases, cfg.MaxActiveLeases, tp.Time.Format(time.RFC3339))
		}
	}
}

// TestSimulatorLeaseWaitTimeout tests that jobs timeout when waiting too long for a lease
func TestSimulatorLeaseWaitTimeout(t *testing.T) {
	// Create config with 1 lease and 3 concurrent jobs
	// Jobs will have long durations to ensure they overlap
	cfg := &config.Config{
		MaxActiveLeases:              1,                        // Only 1 lease
		JobTimeoutDuration:           10 * time.Hour,           // High execution timeout (won't trigger)
		LeaseWaitTimeout:             5 * time.Hour,            // 5h wait timeout
		SimulationDuration:           24 * time.Hour,           // 24h simulation
		DevVersions:                  1,
		MeanDuration:                 6 * time.Hour,            // Long jobs (6h) to ensure overlap
		JobDurationStandardDeviation: 30 * time.Minute,
		DevReleaseIntervalMean:       6 * time.Hour,
		DevReleaseIntervalStdDev:       2 * time.Hour,
		SupportedReleaseIntervalMean: 8 * time.Hour,
		SupportedReleaseIntervalStdDev: 4 * time.Hour,
		EusReleaseIntervalMean:       24 * time.Hour,
		EusReleaseIntervalStdDev:       8 * time.Hour,
	}

	// Three jobs all scheduled at 10 AM
	cfg.Jobs = []config.Job{
		{
			Name:            "job-1",
			Version:         "dev-1",
			CronSchedule:    "0 10 * * *", // All at 10 AM
			TriggerType:     config.TriggerTypeCron,
			VersionCategory: config.VersionCategoryDev,
		},
		{
			Name:            "job-2",
			Version:         "dev-1",
			CronSchedule:    "0 10 * * *", // All at 10 AM
			TriggerType:     config.TriggerTypeCron,
			VersionCategory: config.VersionCategoryDev,
		},
		{
			Name:            "job-3",
			Version:         "dev-1",
			CronSchedule:    "0 10 * * *", // All at 10 AM
			TriggerType:     config.TriggerTypeCron,
			VersionCategory: config.VersionCategoryDev,
		},
	}

	sim := NewSimulator(cfg)
	err := sim.Run()
	if err != nil {
		t.Fatalf("simulation failed: %v", err)
	}

	events := sim.GetEvents()

	// Count different event types
	waitingCount := 0
	leaseWaitTimeoutCount := 0
	executionTimeoutCount := 0
	acquiredCount := 0

	for _, event := range events {
		switch event.Type {
		case EventTypeJobWaiting:
			waitingCount++
		case EventTypeLeaseWaitTimeout:
			leaseWaitTimeoutCount++
		case EventTypeJobTimeout:
			executionTimeoutCount++
		case EventTypeLeaseAcquired:
			acquiredCount++
		}
	}

	// Verify expectations:
	// - At least 2 jobs should wait (since only 1 lease available)
	if waitingCount < 2 {
		t.Errorf("expected at least 2 waiting events, got %d", waitingCount)
	}

	// - At least 1 job should timeout waiting for a lease
	// With 1 lease, 3 jobs, and 6h job duration, the scenario is:
	//   10:00 - job-1 starts
	//   10:00 - job-2 waits
	//   10:00 - job-3 waits
	//   ~16:00 - job-1 finishes, job-2 starts
	//   15:00 - job-3 has waited 5h and times out
	if leaseWaitTimeoutCount < 1 {
		t.Errorf("expected at least 1 lease wait timeout, got %d", leaseWaitTimeoutCount)
	}

	// - No execution timeouts (jobs complete within 6h, well under 10h limit)
	if executionTimeoutCount > 0 {
		t.Errorf("expected no execution timeouts, got %d", executionTimeoutCount)
	}

	// Verify warnings were generated
	warnings := sim.GetWarnings()
	if len(warnings) == 0 {
		t.Error("expected warnings when jobs timeout waiting for leases")
	}

	// Check that warnings include both waiting events and timeout events
	hasWaitingWarning := false
	hasTimeoutWarning := false
	for _, w := range warnings {
		if w.Type == EventTypeJobWaiting {
			hasWaitingWarning = true
		}
		if w.Type == EventTypeLeaseWaitTimeout || w.Type == EventTypeJobTimeout {
			hasTimeoutWarning = true
		}
	}

	if !hasWaitingWarning {
		t.Error("expected at least one waiting warning")
	}
	if !hasTimeoutWarning {
		t.Error("expected at least one timeout warning")
	}

	// Log summary for visibility
	t.Logf("Summary:")
	t.Logf("  - Leases acquired: %d", acquiredCount)
	t.Logf("  - Jobs waiting: %d", waitingCount)
	t.Logf("  - Lease wait timeouts: %d", leaseWaitTimeoutCount)
	t.Logf("  - Execution timeouts: %d", executionTimeoutCount)
	t.Logf("  - Total warnings: %d", len(warnings))
}

// TestSimulatorDurationCalculation tests that job durations are calculated per instance
func TestSimulatorDurationCalculation(t *testing.T) {
	cfg := &config.Config{
		MaxActiveLeases:              10,
		JobTimeoutDuration:           6 * time.Hour,
		LeaseWaitTimeout:             5 * time.Hour,
		SimulationDuration:           168 * time.Hour, // 1 week
		DevVersions:                  1,
		MeanDuration:                 3 * time.Hour,
		JobDurationStandardDeviation: 1 * time.Hour,
		DevReleaseIntervalMean:       6 * time.Hour,
		DevReleaseIntervalStdDev:       2 * time.Hour,
		SupportedReleaseIntervalMean: 8 * time.Hour,
		SupportedReleaseIntervalStdDev: 4 * time.Hour,
		EusReleaseIntervalMean:       24 * time.Hour,
		EusReleaseIntervalStdDev:       8 * time.Hour,
	}

	// Daily job
	cfg.Jobs = []config.Job{
		{
			Name:            "daily-job",
			Version:         "dev-1",
			CronSchedule:    "0 0 * * *", // Daily at midnight
			TriggerType:     config.TriggerTypeCron,
			VersionCategory: config.VersionCategoryDev,
		},
	}

	sim := NewSimulator(cfg)
	err := sim.Run()
	if err != nil {
		t.Fatalf("simulation failed: %v", err)
	}

	events := sim.GetEvents()

	// Collect all lease acquired events
	var acquireEvents []Event
	for _, event := range events {
		if event.Type == EventTypeLeaseAcquired {
			acquireEvents = append(acquireEvents, event)
		}
	}

	// Should have 7 daily runs in a week
	if len(acquireEvents) != 7 {
		t.Errorf("expected 7 daily job runs in a week, got %d", len(acquireEvents))
	}

	// Find corresponding release events and check durations vary
	durations := []time.Duration{}
	for _, acquire := range acquireEvents {
		// Find the corresponding release event
		for _, event := range events {
			if event.Type == EventTypeLeaseReleased &&
				event.JobInstance == acquire.JobInstance {
				duration := event.Time.Sub(acquire.Time)
				durations = append(durations, duration)
				break
			}
		}
	}

	if len(durations) != len(acquireEvents) {
		t.Errorf("expected %d durations, got %d", len(acquireEvents), len(durations))
	}

	// Verify durations are not all identical (Gaussian distribution should vary)
	// At least one duration should differ from the first
	allSame := true
	if len(durations) > 1 {
		firstDuration := durations[0]
		for i := 1; i < len(durations); i++ {
			if durations[i] != firstDuration {
				allSame = false
				break
			}
		}
	}

	if allSame && len(durations) > 1 {
		t.Error("all job durations are identical, expected variation from Gaussian distribution")
	}

	// Verify durations are positive and reasonable
	for i, d := range durations {
		if d <= 0 {
			t.Errorf("duration %d is non-positive: %v", i, d)
		}
		// Should be roughly around mean (3h) with some variance
		// Allow range of 30min to 6h (mean ± 3 std dev should cover 99.7%)
		if d < 30*time.Minute || d > 6*time.Hour {
			t.Errorf("duration %d seems unreasonable: %v (expected roughly 1h-6h range)", i, d)
		}
	}
}
