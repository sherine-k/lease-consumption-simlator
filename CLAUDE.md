# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Go CLI tool that simulates CI job lease usage over time to identify resource contention issues. It models two types of CI jobs:
- **Cron-based jobs**: Scheduled at specific times using cron expressions
- **Release controller jobs**: Triggered unpredictably with reserved lease capacity

The simulator generates ASCII timeseries charts and warnings about lease contention, job timeouts, and capacity issues.

## Build and Run Commands

```bash
# Build the binary
go build -o leases .

# Run with default config.yaml (writes to simulation-report.txt)
./leases

# Run with custom config file
./leases -c path/to/config.yaml

# Show detailed timeline with event limit
./leases -t -l 100

# Run without event summary
./leases --summary=false

# Specify custom output file
./leases -c config_ideal.yaml -o ideal-report.txt

# Full output with timeline and custom file
./leases -c config.yaml -t -l 200 -o full-report.txt
```

### Command Flags

- `-c, --config`: Path to configuration file (default: `config.yaml`)
- `-o, --output`: Path to output file for complete report (default: `simulation-report.txt`)
- `-t, --timeline`: Show detailed timeline of events in output file
- `-l, --timeline-limit`: Limit number of timeline events to display (default: 50)
- `-s, --summary`: Show event summary (default: true)

## Architecture

### Core Simulation Flow

1. **Configuration Loading** (`pkg/config/parser.go`): Parses YAML config and validates all settings including job definitions, lease limits, and timeouts
2. **Job Instance Generation** (`pkg/simulation/simulator.go:generateJobInstances`): Creates concrete job instances for the simulation period based on trigger type
3. **Simulation Execution** (`pkg/simulation/simulator.go:simulateLeaseUsage`): Processes job instances chronologically, tracking lease acquisition/release and detecting contention
4. **Visualization** (`pkg/chart/chart.go`): Generates ASCII charts and reports from simulation data

### Key Components

**pkg/config/**
- `types.go`: Defines `Config`, `Job`, `JobInstance`, and `TriggerType` structures
- `parser.go`: YAML config loading with comprehensive validation

**pkg/simulation/**
- `simulator.go`: Core simulation engine with lease management logic
- `events.go`: Event types (`lease-acquired`, `lease-released`, `job-waiting`, `job-timeout`, `max-exceeded`) and time point tracking

**pkg/chart/**
- `chart.go`:  **Visualization** with 3 main sections:
  1. Executive Summary with health status (OK/WARNING/CRITICAL)
  2. Hourly Statistics breakdown (avg/peak/waiting jobs per hour)
  3. Problem Timeline (timeouts and waits only - focuses on issues)

**cmd/**
- `root.go`: Cobra CLI implementation with flags for timeline display, event limits, summary control, and output file path. Displays lease chart to console and writes complete report to file.

### Lease Management Logic

The simulator implements a simple FIFO (First-In-First-Out) lease allocation system:
- All jobs (cron, release controller, and developer sessions) compete equally for leases
- Available capacity: `maxActiveLeases - activeLeases`
- Jobs that cannot acquire a lease immediately are added to a waiting queue
- When a lease is released, the first waiting job acquires it

Simulation runs in 5-minute time steps, processing:
1. Job starts and lease acquisitions (immediate if available, otherwise queued)
2. Job completions and lease releases
3. Waiting queue management (FIFO - first waiting job gets released lease)
4. Timeout detection for both lease waits (`leaseWaitTimeout`) and job execution (`jobTimeoutDuration`)
5. Developer session simulation (if `devLeaseBuffer` configured)

### Event System

Events track all state changes with timestamps and generate warnings when:
- Jobs wait for available leases
- Active leases exceed configured maximum
- Jobs timeout waiting for leases (after `leaseWaitTimeout`)
- Jobs exceed execution timeout (after `jobTimeoutDuration`)

All events are recorded with timestamps, active lease counts, and descriptive messages. Warning events are flagged with `IsWarning: true` and included in the warnings output.

### Configuration Structure

The simulator uses a **template-based configuration format** for OpenShift CI scenarios testing multiple versions:

#### Core Configuration Fields
- `maxActiveLeases`: Total concurrent lease limit
- `jobTimeoutDuration`: Max execution time before job timeout warning
- `leaseWaitTimeout`: Max wait time before lease acquisition timeout warning
- `simulationDuration`: Total time to simulate
- `devVersions`, `supportedVersions`, `eusVersions`: Number of each version type to simulate
- `devLeaseBuffer`: Number of leases reserved for developer testing sessions
- `meanDuration`: Global average job duration (Gaussian distribution mean)
- `jobDurationStdDev`: Global standard deviation for duration variation

#### Quantity of OCP releases being tested by category
Controls how many active versions of OCP are being tested by the simulation: 
- `devVersions`: Number of releases under development being tested
- `supportedVersions`: Number of releases in full suported being tested
- `eusVersions`: Number of releases in Extended support being tested

#### Release Interval Configuration (Optional)
Control how frequently release controller events are simulated per version category using Gaussian distribution:
- `devReleaseIntervalMean`, `devReleaseIntervalStdDev`: Dev version release frequency (default: mean=6h, stddev=2h)
- `supportedReleaseIntervalMean`, `supportedReleaseIntervalStdDev`: Supported version release frequency (default: mean=8h, stddev=4h)
- `eusReleaseIntervalMean`, `eusReleaseIntervalStdDev`: EUS version release frequency (default: mean=24h, stddev=8h)

#### Job Templates
Jobs are templates that get expanded to multiple versions:
- `name`: Job scenario name (expanded to `scenario-version-X` for each version)
- `onReleaseController`: Array of version categories (`[dev]`, `[supported]`, `[dev, supported]`, etc.) where this job triggers on release events in addition to cron schedule

### Cron Job Instance Generation

Uses `github.com/robfig/cron/v3` parser with all fields enabled (Minute|Hour|Dom|Month|Dow). Iterates through simulation period calling `schedule.Next()` to generate all occurrences.

### Release Controller Job Generation

Release controller jobs are simulated with Gaussian-distributed intervals that vary by version category (configurable):
- **Dev versions**: Default mean=6h, stddev=2h (frequent development builds)
- **Supported versions**: Default mean=8h, stddev=4h (regular updates)
- **EUS versions**: Default mean=24h, stddev=8h (infrequent updates)

Each interval between releases is calculated using Gaussian distribution with the configured mean and standard deviation. Jobs are grouped by version, and each version has independent release events. Jobs with `onReleaseController` specified trigger on BOTH:
1. Daily cron schedule (auto-staggered across 24 hours)
2. Simulated release events (Gaussian-distributed intervals based on version category configuration)

### Developer Session Generation

When `devLeaseBuffer` > 0, the simulator generates synthetic developer testing sessions:
- Sessions are randomly distributed across the simulation period
- Targets 40% average utilization of the `devLeaseBuffer` leases
- Duration follows same Gaussian distribution as regular jobs
- Simulates ad-hoc developer usage that competes with CI jobs for leases

### Job Duration Calculation

Job durations are calculated **per-instance** using Gaussian (normal) distribution:
- Each job instance gets a unique duration when generated in the simulator
- Uses `simulation.CalculateGaussianDuration(meanDuration, stdDev)` helper function
- Durations are clamped to minimum 10% of mean to avoid negative or zero values
- Statistical distribution: ~68% within ±1σ, ~95% within ±2σ, ~99.7% within ±3σ
- Expected behavior: ~4% of jobs naturally exceed mean+1.75σ threshold
- Package-level RNG with `sync.Once` ensures thread-safe random generation

## Development Notes

- Simulation starts at last Monday midnight (for consistent timeline display across runs)
- Simulation runs in 5-minute time steps
- Time points for charting sampled every **15 minutes** (tracks peak active leases during interval)
- Job durations calculated **per-instance** using Gaussian distribution in simulator (not parser)
- Template-based configs auto-expand jobs across versions with staggered start times
- Developer sessions generated randomly if `devLeaseBuffer` > 0 (targets 40% utilization)
- Uses standard Go duration parsing for YAML fields (e.g., `72h`, `6h30m`, `5h15m`)

### Test Coverage

**pkg/config/parser_test.go** (94.2% coverage)
- `TestLoadConfig`: 17 test cases covering valid/invalid configs, template expansion, validation
- `TestLoadConfigFileNotFound`: File I/O error handling
- `TestExpandJobTemplates`: Job template expansion logic
- `TestValidateConfig`: Configuration validation rules
- `TestLoadConfigWithRealFiles`: Integration tests with actual config files

**pkg/simulation/simulator_test.go**
- `TestCalculateGaussianDuration`: Validates positive durations and minimum enforcement
- `TestSimulatorBasicScheduling`: 2 jobs at different times, no timeouts expected
- `TestSimulatorLeaseContention`: Insufficient capacity causes waiting events
- `TestSimulatorLeaseWaitTimeout`: 1 lease + 3 concurrent jobs → wait timeouts detected
- `TestSimulatorDurationCalculation`: Verifies per-instance duration variation (7 daily runs show different durations)

### Critical Bug Fixes and Optimizations

**Duration Calculation (per-instance)**
- Moved from parser to simulator: `calculateInstanceDuration()` called for each job instance
- Fixed RNG to be package-level singleton with `sync.Once` for proper randomization
- Each instance now gets unique duration from Gaussian distribution

**Time Point Generation (peak tracking)**
- Fixed to track **peak** active leases during 15-minute sampling interval
- Previous bug: used final value (often 0 after job completion) instead of peak
- Impact: Chart now accurately shows utilization spikes

**Timeout Detection**
- Fixed execution timeout: Calculate `expectedDuration = job.EndTime.Sub(job.StartTime)` instead of using zero `job.Job.Duration`
- Fixed lease wait timeout: Use `WasWaiting` flag to distinguish from execution timeout
- Fixed job EndTime recalculation when jobs wait for leases: preserve original duration

**Lease Assignment**
- Fixed `assignLeaseToWaitingJob()` to preserve original job duration when lease becomes available
- Ensures jobs run for full expected time even if they had to wait

**Visualization**
- New improved visualization focuses on problems (timeouts, waits) rather than just showing all events
- Executive summary provides health status at a glance
- Hourly heatmap uses visual bars to show average and peak usage patterns
- Problem timeline shows only timeout and wait events for easier troubleshooting
