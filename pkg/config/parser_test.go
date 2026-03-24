package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestLoadConfig tests the LoadConfig function with various scenarios
func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name          string
		configContent string
		expectError   bool
		errorContains string
		validate      func(*testing.T, *Config)
	}{
		{
			name: "valid template-based config",
			configContent: `
maxActiveLeases: 50
jobTimeoutDuration: 5h15m
leaseWaitTimeout: 5h
simulationDuration: 168h
devVersions: 1
supportedVersions: 2
eusVersions: 1
devLeaseBuffer: 5
meanDuration: 3h30m
jobDurationStandardDeviation: 1h
devReleaseIntervalMean: 6h
devReleaseIntervalStdDev: 1h
supportedReleaseIntervalMean: 12h
supportedReleaseIntervalStdDev: 4h
eusReleaseIntervalMean: 24h
eusReleaseIntervalStdDev: 8h
jobs:
  - name: e2e-conformance
    onReleaseController: [dev, supported]
  - name: e2e-upgrade
    onReleaseController: [dev]
`,
			expectError: false,
			validate: func(t *testing.T, cfg *Config) {
				if cfg.MaxActiveLeases != 50 {
					t.Errorf("expected MaxActiveLeases=50, got %d", cfg.MaxActiveLeases)
				}
				if cfg.JobTimeoutDuration != 5*time.Hour+15*time.Minute {
					t.Errorf("expected JobTimeoutDuration=5h15m, got %v", cfg.JobTimeoutDuration)
				}
				if cfg.DevVersions != 1 {
					t.Errorf("expected DevVersions=1, got %d", cfg.DevVersions)
				}
				if cfg.SupportedVersions != 2 {
					t.Errorf("expected SupportedVersions=2, got %d", cfg.SupportedVersions)
				}
				if cfg.EusVersions != 1 {
					t.Errorf("expected EusVersions=1, got %d", cfg.EusVersions)
				}
				// Should have 2 job templates * (1 dev + 2 supported + 1 eus) = 8 expanded jobs
				expectedJobs := 2 * (1 + 2 + 1)
				if len(cfg.Jobs) != expectedJobs {
					t.Errorf("expected %d expanded jobs, got %d", expectedJobs, len(cfg.Jobs))
				}
				// Verify job names are expanded
				for i, job := range cfg.Jobs {
					if job.Version == "" {
						t.Errorf("job %d (%s): Version not set", i, job.Name)
					}
				}
				// Check that release controller flags are set correctly
				rcJobCount := 0
				for _, job := range cfg.Jobs {
					if len(job.OnReleaseController) > 0 {
						rcJobCount++
					}
				}
				// e2e-conformance on dev+supported (3) + e2e-upgrade on dev (1) = 4
				if rcJobCount != 4 {
					t.Errorf("expected 4 release controller jobs, got %d", rcJobCount)
				}
			},
		},

		{
			name: "missing maxActiveLeases",
			configContent: `
jobTimeoutDuration: 5h
leaseWaitTimeout: 5h
simulationDuration: 168h
meanDuration: 5h
jobDurationStdDev: 30m
devVersions: 1
jobs:
  - name: "test-job"
    onReleaseController: [dev]

`,
			expectError:   true,
			errorContains: "maxActiveLeases must be greater than 0",
		},
		{
			name: "negative jobTimeoutDuration",
			configContent: `
maxActiveLeases: 10
jobTimeoutDuration: -5h
leaseWaitTimeout: 5h
simulationDuration: 168h
meanDuration: 5h
jobDurationStdDev: 30m
devVersions: 1
jobs:
  - name: "test-job"
    onReleaseController: [dev]
`,
			expectError:   true,
			errorContains: "jobTimeoutDuration must be greater than 0",
		},
		{
			name: "missing leaseWaitTimeout",
			configContent: `
maxActiveLeases: 10
jobTimeoutDuration: 5h
simulationDuration: 168h
meanDuration: 5h
jobDurationStdDev: 30m
devVersions: 1
jobs:
  - name: "test-job"
    onReleaseController: [dev]
`,
			expectError:   true,
			errorContains: "leaseWaitTimeout must be greater than 0",
		},
		{
			name: "missing simulationDuration",
			configContent: `
maxActiveLeases: 10
jobTimeoutDuration: 5h
leaseWaitTimeout: 5h
meanDuration: 5h
jobDurationStdDev: 30m
supportedVersions: 1
jobs:
  - name: "test-job"
    onReleaseController: [supported]
`,
			expectError:   true,
			errorContains: "simulationDuration must be greater than 0",
		},
		{
			name: "no jobs defined",
			configContent: `
maxActiveLeases: 10
jobTimeoutDuration: 5h
leaseWaitTimeout: 5h
simulationDuration: 168h
meanDuration: 5h
jobDurationStdDev: 30m
jobs: []
`,
			expectError:   true,
			errorContains: "failed to expand job templates: at least one job template must be defined",
		},
		{
			name: "template config missing meanDuration",
			configContent: `
maxActiveLeases: 10
jobTimeoutDuration: 5h
leaseWaitTimeout: 5h
simulationDuration: 168h
devVersions: 1
jobs:
  - name: "test-job"
`,
			expectError:   true,
			errorContains: "meanDuration must be greater than 0 for template-based config",
		},

		{
			name: "invalid YAML syntax",
			configContent: `
maxActiveLeases: 10
jobTimeoutDuration: 5h
this is not valid yaml: [
`,
			expectError:   true,
			errorContains: "failed to parse config file",
		},
		{
			name: "template config with no job templates",
			configContent: `
maxActiveLeases: 10
jobTimeoutDuration: 5h
leaseWaitTimeout: 5h
simulationDuration: 168h
devVersions: 1
meanDuration: 3h
jobDurationStandardDeviation: 1h
jobs: []
`,
			expectError:   true,
			errorContains: "at least one job template must be defined",
		},
		{
			name: "template config with default release intervals",
			configContent: `
maxActiveLeases: 10
jobTimeoutDuration: 5h
leaseWaitTimeout: 5h
simulationDuration: 168h
devVersions: 1
meanDuration: 3h
jobDurationStandardDeviation: 1h
jobs:
  - name: test-job
`,
			expectError: false,
			validate: func(t *testing.T, cfg *Config) {
				// Verify defaults were set
				if cfg.DevReleaseIntervalMean != 6*time.Hour {
					t.Errorf("expected DevReleaseIntervalMean=6h, got %v", cfg.DevReleaseIntervalMean)
				}
				if cfg.DevReleaseIntervalStdDev != 2*time.Hour {
					t.Errorf("expected DevReleaseIntervalStdDev=2h, got %v", cfg.DevReleaseIntervalStdDev)
				}
				if cfg.SupportedReleaseIntervalMean != 8*time.Hour {
					t.Errorf("expected SupportedReleaseIntervalMean=8h, got %v", cfg.SupportedReleaseIntervalMean)
				}
				if cfg.SupportedReleaseIntervalStdDev != 4*time.Hour {
					t.Errorf("expected SupportedReleaseIntervalStdDev=4h, got %v", cfg.SupportedReleaseIntervalStdDev)
				}
				if cfg.EusReleaseIntervalMean != 24*time.Hour {
					t.Errorf("expected EusReleaseIntervalMean=24h, got %v", cfg.EusReleaseIntervalMean)
				}
				if cfg.EusReleaseIntervalStdDev != 8*time.Hour {
					t.Errorf("expected EusReleaseIntervalStdDev=8h, got %v", cfg.EusReleaseIntervalStdDev)
				}
			},
		},
		{
			name: "template config with negative mean for dev",
			configContent: `
maxActiveLeases: 10
jobTimeoutDuration: 5h
leaseWaitTimeout: 5h
simulationDuration: 168h
devVersions: 1
meanDuration: 3h
jobDurationStandardDeviation: 1h
devReleaseIntervalMean: -5h
devReleaseIntervalStdDev: 1h
supportedReleaseIntervalMean: 12h
supportedReleaseIntervalStdDev: 4h
eusReleaseIntervalMean: 24h
eusReleaseIntervalStdDev: 8h
jobs:
  - name: test-job
`,
			expectError:   true,
			errorContains: "devReleaseIntervalMean must be greater than or equal to 0",
		},
		{
			name: "template config with negative stddev for supported",
			configContent: `
maxActiveLeases: 10
jobTimeoutDuration: 5h
leaseWaitTimeout: 5h
simulationDuration: 168h
supportedVersions: 1
meanDuration: 3h
jobDurationStandardDeviation: 1h
supportedReleaseIntervalStdDev: -2h
supportedReleaseIntervalMean: 12h
jobs:
  - name: test-job
`,
			expectError:   true,
			errorContains: "supportedReleaseIntervalStdDev must be greater than or equal to 0",
		},
		{
			name: "template config with negative mean for eus",
			configContent: `
maxActiveLeases: 10
jobTimeoutDuration: 5h
leaseWaitTimeout: 5h
simulationDuration: 168h
eusVersions: 1
meanDuration: 3h
jobDurationStandardDeviation: 1h
eusReleaseIntervalMean: -10h
eusReleaseIntervalStdDev: 8h
jobs:
  - name: test-job
`,
			expectError:   true,
			errorContains: "eusReleaseIntervalMean must be greater than or equal to 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a temporary config file
			tmpFile, err := os.CreateTemp("", "config-test-*.yaml")
			if err != nil {
				t.Fatalf("failed to create temp file: %v", err)
			}
			defer os.Remove(tmpFile.Name())

			if _, err := tmpFile.WriteString(tt.configContent); err != nil {
				t.Fatalf("failed to write temp file: %v", err)
			}
			tmpFile.Close()

			// Test LoadConfig
			cfg, err := LoadConfig(tmpFile.Name())

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error containing '%s', got nil", tt.errorContains)
				} else if tt.errorContains != "" && !contains(err.Error(), tt.errorContains) {
					t.Errorf("expected error containing '%s', got '%s'", tt.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				} else if tt.validate != nil {
					tt.validate(t, cfg)
				}
			}
		})
	}
}

// TestLoadConfigFileNotFound tests LoadConfig with non-existent file
func TestLoadConfigFileNotFound(t *testing.T) {
	_, err := LoadConfig("/nonexistent/path/to/config.yaml")
	if err == nil {
		t.Error("expected error for non-existent file, got nil")
	}
	if !contains(err.Error(), "failed to read config file") {
		t.Errorf("expected 'failed to read config file' error, got: %v", err)
	}
}

// TestExpandJobTemplates tests the job template expansion logic
func TestExpandJobTemplates(t *testing.T) {
	config := &Config{
		DevVersions:       1,
		SupportedVersions: 2,
		EusVersions:       1,
		Jobs: []Job{
			{
				Name:                "test-job-1",
				OnReleaseController: []VersionCategory{VersionCategoryDev},
			},
			{
				Name: "test-job-2",
				// No release controller - cron only
			},
		},
	}

	err := expandJobTemplates(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have 2 templates * (1 dev + 2 supported + 1 eus) = 8 jobs
	expectedJobs := 2 * (1 + 2 + 1)
	if len(config.Jobs) != expectedJobs {
		t.Errorf("expected %d expanded jobs, got %d", expectedJobs, len(config.Jobs))
	}

	// Verify version categories are set correctly
	devCount := 0
	supportedCount := 0
	eusCount := 0
	for _, job := range config.Jobs {
		switch job.VersionCategory {
		case VersionCategoryDev:
			devCount++
		case VersionCategorySupported:
			supportedCount++
		case VersionCategoryEus:
			eusCount++
		}
	}

	if devCount != 2 {
		t.Errorf("expected 2 dev jobs, got %d", devCount)
	}
	if supportedCount != 4 {
		t.Errorf("expected 4 supported jobs, got %d", supportedCount)
	}
	if eusCount != 2 {
		t.Errorf("expected 2 eus jobs, got %d", eusCount)
	}

	// Verify release controller flags
	rcCount := 0
	for _, job := range config.Jobs {
		if len(job.OnReleaseController) > 0 {
			rcCount++
		}
	}
	// Only test-job-1 on dev versions (1) should be RC
	if rcCount != 1 {
		t.Errorf("expected 1 release controller job, got %d", rcCount)
	}

	// Verify all jobs have cron schedules
	for i, job := range config.Jobs {
		if job.CronSchedule == "" {
			t.Errorf("job %d (%s): missing cron schedule", i, job.Name)
		}
	}
}

// TestValidateConfig tests the configuration validation logic
func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name          string
		config        *Config
		expectError   bool
		errorContains string
	}{
		{
			name: "valid template config",
			config: &Config{
				MaxActiveLeases:              10,
				JobTimeoutDuration:           5 * time.Hour,
				LeaseWaitTimeout:             5 * time.Hour,
				SimulationDuration:           168 * time.Hour,
				DevVersions:                  1,
				MeanDuration:                 3 * time.Hour,
				JobDurationStandardDeviation: 1 * time.Hour,
				DevReleaseIntervalMean:       6 * time.Hour,
				DevReleaseIntervalStdDev:     1 * time.Hour,
				SupportedReleaseIntervalMean: 12 * time.Hour,
				SupportedReleaseIntervalStdDev: 4 * time.Hour,
				EusReleaseIntervalMean:       24 * time.Hour,
				EusReleaseIntervalStdDev:     8 * time.Hour,
				Jobs: []Job{
					{Name: "test-job", Version: "dev-1"},
				},
			},
			expectError: false,
		},
		{
			name: "valid individual job config",
			config: &Config{
				MaxActiveLeases:              10,
				JobTimeoutDuration:           5 * time.Hour,
				LeaseWaitTimeout:             5 * time.Hour,
				SimulationDuration:           168 * time.Hour,
				MeanDuration:                 5 * time.Hour,
				JobDurationStandardDeviation: 30 * time.Minute,
				Jobs: []Job{
					{
						Name:         "test-job",
						TriggerType:  TriggerTypeCron,
						CronSchedule: "0 0 * * *",
					},
				},
			},
			expectError: false,
		},
		{
			name: "negative stdDev in template config",
			config: &Config{
				MaxActiveLeases:              10,
				JobTimeoutDuration:           5 * time.Hour,
				LeaseWaitTimeout:             5 * time.Hour,
				SimulationDuration:           168 * time.Hour,
				DevVersions:                  1,
				MeanDuration:                 3 * time.Hour,
				JobDurationStandardDeviation: -1 * time.Hour,
				Jobs: []Job{
					{Name: "test-job"},
				},
			},
			expectError:   true,
			errorContains: "jobDurationStandardDeviation must be greater than or equal to 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConfig(tt.config)
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error containing '%s', got nil", tt.errorContains)
				} else if !contains(err.Error(), tt.errorContains) {
					t.Errorf("expected error containing '%s', got '%s'", tt.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

// TestLoadConfigWithRealFiles tests LoadConfig with actual config files in the repo
func TestLoadConfigWithRealFiles(t *testing.T) {
	// This test will only run if the config files exist
	testFiles := []string{
		"../../config_ideal.yaml",
		"../../example_gaussian_config.yaml",
	}

	for _, file := range testFiles {
		t.Run(filepath.Base(file), func(t *testing.T) {
			absPath, err := filepath.Abs(file)
			if err != nil {
				t.Skipf("could not resolve path: %v", err)
			}

			if _, err := os.Stat(absPath); os.IsNotExist(err) {
				t.Skipf("config file not found: %s", absPath)
			}

			cfg, err := LoadConfig(absPath)
			if err != nil {
				t.Errorf("failed to load %s: %v", file, err)
			}
			if cfg == nil {
				t.Errorf("expected non-nil config for %s", file)
			}
			if len(cfg.Jobs) == 0 {
				t.Errorf("expected at least one job in %s", file)
			}
		})
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && stringContains(s, substr)))
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
