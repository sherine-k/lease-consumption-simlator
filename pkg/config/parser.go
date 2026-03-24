package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

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

	// Set default release intervals if not specified
	setDefaultReleaseIntervalsIfEmpty(&config)

	// Expand job templates into full job instances
	if err := expandJobTemplates(&config); err != nil {
		return nil, fmt.Errorf("failed to expand job templates: %w", err)
	}

	// Validate configuration
	if err := validateConfig(&config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &config, nil
}

// setDefaultReleaseIntervalsIfEmpty sets default release interval values if not specified
func setDefaultReleaseIntervalsIfEmpty(config *Config) {
	// Dev defaults: mean=6h, stddev=2h (Gaussian distribution centered at 6h with variation)
	if config.DevReleaseIntervalMean == 0 {
		config.DevReleaseIntervalMean = 6 * time.Hour
	}
	if config.DevReleaseIntervalStdDev == 0 {
		config.DevReleaseIntervalStdDev = 2 * time.Hour
	}

	// Supported defaults: mean=8h, stddev=4h
	if config.SupportedReleaseIntervalMean == 0 {
		config.SupportedReleaseIntervalMean = 8 * time.Hour
	}
	if config.SupportedReleaseIntervalStdDev == 0 {
		config.SupportedReleaseIntervalStdDev = 4 * time.Hour
	}

	// EUS defaults: mean=24h, stddev=8h
	if config.EusReleaseIntervalMean == 0 {
		config.EusReleaseIntervalMean = 24 * time.Hour
	}
	if config.EusReleaseIntervalStdDev == 0 {
		config.EusReleaseIntervalStdDev = 8 * time.Hour
	}
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
	totalCronJobs := len(jobTemplates) * totalVersions

	// Spread jobs evenly across 24 hours using minutes for finer granularity
	minutesPerDay := 24 * 60
	minutesBetweenCronJobs := minutesPerDay / totalCronJobs
	if minutesBetweenCronJobs < 1 {
		minutesBetweenCronJobs = 1 // Minimum 1 minute spacing
	}

	// Global job counter for staggering
	globalJobIndex := 0

	// Expand each template across all versions
	for _, template := range jobTemplates {

		// Expand for dev versions
		for i := 0; i < config.DevVersions; i++ {
			minuteOfDay := (globalJobIndex * minutesBetweenCronJobs) % minutesPerDay
			cronHour := minuteOfDay / 60
			cronMinute := minuteOfDay % 60
			job := expandJobTemplate(template, "dev", i+1, cronHour, cronMinute, VersionCategoryDev)
			config.Jobs = append(config.Jobs, job)
			globalJobIndex++
		}

		// Expand for supported versions
		for i := 0; i < config.SupportedVersions; i++ {
			minuteOfDay := (globalJobIndex * minutesBetweenCronJobs) % minutesPerDay
			cronHour := minuteOfDay / 60
			cronMinute := minuteOfDay % 60
			job := expandJobTemplate(template, "supported", i+1, cronHour, cronMinute, VersionCategorySupported)
			config.Jobs = append(config.Jobs, job)
			globalJobIndex++
		}

		// Expand for EUS versions
		for i := 0; i < config.EusVersions; i++ {
			minuteOfDay := (globalJobIndex * minutesBetweenCronJobs) % minutesPerDay
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

		// Validate release intervals (Gaussian distribution parameters)
		if config.DevReleaseIntervalMean < 0 {
			return fmt.Errorf("devReleaseIntervalMean must be greater than or equal to 0")
		}
		if config.DevReleaseIntervalStdDev < 0 {
			return fmt.Errorf("devReleaseIntervalStdDev must be greater than or equal to 0")
		}

		if config.SupportedReleaseIntervalMean < 0 {
			return fmt.Errorf("supportedReleaseIntervalMean must be greater than or equal to 0")
		}
		if config.SupportedReleaseIntervalStdDev < 0 {
			return fmt.Errorf("supportedReleaseIntervalStdDev must be greater than or equal to 0")
		}

		if config.EusReleaseIntervalMean < 0 {
			return fmt.Errorf("eusReleaseIntervalMean must be greater than or equal to 0")
		}
		if config.EusReleaseIntervalStdDev < 0 {
			return fmt.Errorf("eusReleaseIntervalStdDev must be greater than or equal to 0")
		}
	}

	for i, job := range config.Jobs {
		if job.Name == "" {
			return fmt.Errorf("job %d: name is required", i)
		}

	}

	return nil
}
