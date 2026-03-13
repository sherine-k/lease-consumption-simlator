package config

import (
	"fmt"
	"math/rand"
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

	// Calculate job durations from Gaussian distribution
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := range config.Jobs {
		job := &config.Jobs[i]
		// Generate duration from normal distribution: mean + stddev * N(0,1)
		gaussianValue := rng.NormFloat64()
		durationSeconds := job.MeanDuration.Seconds() + (job.StdDev.Seconds() * gaussianValue)

		// Ensure duration is positive
		if durationSeconds < 0 {
			durationSeconds = job.MeanDuration.Seconds() * 0.1 // Use 10% of mean as minimum
		}

		job.Duration = time.Duration(durationSeconds * float64(time.Second))
	}

	// Validate configuration
	if err := validateConfig(&config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &config, nil
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

	for i, job := range config.Jobs {
		if job.Name == "" {
			return fmt.Errorf("job %d: name is required", i)
		}

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

		if job.TriggerType == TriggerTypeReleaseController {
			job.IsReleaseController = true
		}
	}

	return nil
}
