package config

import (
	"fmt"
	"math/rand"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// CalculateGaussianDuration calculates a duration from a Gaussian (normal) distribution
// given a mean duration and standard deviation. Ensures the result is always positive.
func CalculateGaussianDuration(rng *rand.Rand, meanDuration, stdDev time.Duration) time.Duration {
	// Generate duration from normal distribution: mean + stddev * N(0,1)
	gaussianValue := rng.NormFloat64()
	durationSeconds := meanDuration.Seconds() + (stdDev.Seconds() * gaussianValue)

	// Ensure duration is positive (use 10% of mean as minimum)
	if durationSeconds < 0 {
		durationSeconds = meanDuration.Seconds() * 0.1
	}

	return time.Duration(durationSeconds * float64(time.Second))
}

// LoadConfig loads and parses the configuration file
func LoadConfig(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Handle alternate field name for jobDurationStdDev -> jobDurationStandardDeviation
	var rawConfig map[string]interface{}
	if err := yaml.Unmarshal(data, &rawConfig); err == nil {
		if stdDev, ok := rawConfig["jobDurationStdDev"]; ok && config.JobDurationStandardDeviation == 0 {
			// Parse the duration from the alternate field name
			if stdDevStr, ok := stdDev.(string); ok {
				if duration, err := time.ParseDuration(stdDevStr); err == nil {
					config.JobDurationStandardDeviation = duration
				}
			}
		}
	}

	// Detect if using template-based format
	isTemplateFormat := config.DevVersions > 0 || config.SupportedVersions > 0 || config.EusVersions > 0

	if isTemplateFormat {
		// Expand job templates into full job instances
		if err := expandJobTemplates(&config); err != nil {
			return nil, fmt.Errorf("failed to expand job templates: %w", err)
		}
	}

	// Calculate job durations from Gaussian distribution
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := range config.Jobs {
		job := &config.Jobs[i]

		// Determine which mean/stddev to use
		meanDuration := job.MeanDuration
		stdDev := job.StdDev

		// For template-based configs, use global values if job-specific not set
		if isTemplateFormat && meanDuration == 0 {
			meanDuration = config.MeanDuration
			stdDev = config.JobDurationStandardDeviation
		}

		job.Duration = CalculateGaussianDuration(rng, meanDuration, stdDev)

		// Set IsReleaseController flag based on TriggerType for non-template configs
		if !isTemplateFormat && job.TriggerType == TriggerTypeReleaseController {
			job.IsReleaseController = true
		}
	}

	// Validate configuration
	if err := validateConfig(&config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &config, nil
}

// expandJobTemplates expands job templates into full job instances
func expandJobTemplates(config *Config) error {
	if len(config.Jobs) == 0 {
		return fmt.Errorf("at least one job template must be defined")
	}

	// Save the job templates
	jobTemplates := config.Jobs
	config.Jobs = []Job{}

	// Calculate total number of jobs to distribute evenly across the day
	totalVersions := config.DevVersions + config.SupportedVersions + config.EusVersions
	totalJobs := len(jobTemplates) * totalVersions

	// Spread jobs evenly across 24 hours using minutes for finer granularity
	minutesPerDay := 24 * 60
	minutesBetweenJobs := minutesPerDay / totalJobs
	if minutesBetweenJobs < 1 {
		minutesBetweenJobs = 1 // Minimum 1 minute spacing
	}

	// Global job counter for staggering
	globalJobIndex := 0

	// Expand each template across all versions
	for _, template := range jobTemplates {
		versionIndex := 0

		// Expand for dev versions
		for i := 0; i < config.DevVersions; i++ {
			versionIndex++
			minuteOfDay := (globalJobIndex * minutesBetweenJobs) % minutesPerDay
			cronHour := minuteOfDay / 60
			cronMinute := minuteOfDay % 60
			job := expandJobTemplate(template, "dev", versionIndex, cronHour, cronMinute, VersionCategoryDev)
			config.Jobs = append(config.Jobs, job)
			globalJobIndex++
		}

		// Expand for supported versions
		for i := 0; i < config.SupportedVersions; i++ {
			versionIndex++
			minuteOfDay := (globalJobIndex * minutesBetweenJobs) % minutesPerDay
			cronHour := minuteOfDay / 60
			cronMinute := minuteOfDay % 60
			job := expandJobTemplate(template, "supported", i+1, cronHour, cronMinute, VersionCategorySupported)
			config.Jobs = append(config.Jobs, job)
			globalJobIndex++
		}

		// Expand for EUS versions
		for i := 0; i < config.EusVersions; i++ {
			versionIndex++
			minuteOfDay := (globalJobIndex * minutesBetweenJobs) % minutesPerDay
			cronHour := minuteOfDay / 60
			cronMinute := minuteOfDay % 60
			job := expandJobTemplate(template, "eus", i+1, cronHour, cronMinute, VersionCategoryEus)
			config.Jobs = append(config.Jobs, job)
			globalJobIndex++
		}
	}

	return nil
}

// expandJobTemplate creates a job instance from a template
func expandJobTemplate(template Job, versionType string, versionNum int, cronHour int, cronMinute int, category VersionCategory) Job {
	versionName := fmt.Sprintf("%s-%d", versionType, versionNum)

	job := Job{
		Name:            fmt.Sprintf("%s-%s", template.Name, versionName),
		Version:         versionName,
		Scenario:        template.Name,
		PayloadType:     versionType,
		VersionCategory: category,
	}

	// Set cron schedule (daily at specified hour and minute)
	job.CronSchedule = fmt.Sprintf("%d %d * * *", cronMinute, cronHour)
	job.TriggerType = TriggerTypeCron

	// Check if job should also trigger on release controller for this category
	shouldTriggerOnRC := false
	for _, rcCategory := range template.OnReleaseController {
		if rcCategory == category {
			shouldTriggerOnRC = true
			break
		}
	}

	if shouldTriggerOnRC {
		job.TriggerType = TriggerTypeReleaseController
		job.IsReleaseController = true
		job.OnReleaseController = template.OnReleaseController
	}

	return job
}

// validateConfig validates the configuration
func validateConfig(config *Config) error {
	if config.MaxActiveLeases <= 0 {
		return fmt.Errorf("maxActiveLeases must be greater than 0")
	}

	if config.JobTimeoutDuration <= 0 {
		return fmt.Errorf("jobTimeoutDuration must be greater than 0")
	}

	if config.LeaseWaitTimeout <= 0 {
		return fmt.Errorf("leaseWaitTimeout must be greater than 0")
	}

	if config.SimulationDuration <= 0 {
		return fmt.Errorf("simulationDuration must be greater than 0")
	}

	if len(config.Jobs) == 0 {
		return fmt.Errorf("at least one job must be defined")
	}

	// Check if template-based format
	isTemplateFormat := config.DevVersions > 0 || config.SupportedVersions > 0 || config.EusVersions > 0

	// Validate template-based format
	if isTemplateFormat {
		if config.MeanDuration <= 0 {
			return fmt.Errorf("meanDuration must be greater than 0 for template-based config")
		}
		if config.JobDurationStandardDeviation < 0 {
			return fmt.Errorf("jobDurationStandardDeviation must be greater than or equal to 0")
		}
	}

	for i, job := range config.Jobs {
		if job.Name == "" {
			return fmt.Errorf("job %d: name is required", i)
		}

		// For non-template format, validate job-specific fields
		if !isTemplateFormat {
			if job.MeanDuration <= 0 {
				return fmt.Errorf("job %s: meanDuration must be greater than 0", job.Name)
			}

			if job.StdDev < 0 {
				return fmt.Errorf("job %s: stdDev must be greater than or equal to 0", job.Name)
			}

			if job.TriggerType != TriggerTypeCron && job.TriggerType != TriggerTypeReleaseController {
				return fmt.Errorf("job %s: triggerType must be either 'cron' or 'release-controller'", job.Name)
			}

			if job.TriggerType == TriggerTypeCron && job.CronSchedule == "" {
				return fmt.Errorf("job %s: cronSchedule is required for cron-type jobs", job.Name)
			}
		}
	}

	return nil
}
