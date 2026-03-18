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
- `chart.go`: ASCII chart generation, event summaries, warnings, and timeline output

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

The simulator supports two configuration formats:

#### Template-Based Format (Primary)
Used for OpenShift CI scenarios with multiple versions:
- `maxActiveLeases`: Total concurrent lease limit
- `jobTimeoutDuration`: Max execution time before job timeout warning
- `leaseWaitTimeout`: Max wait time before lease acquisition timeout warning
- `simulationDuration`: Total time to simulate
- `devVersions`, `supportedVersions`, `eusVersions`: Number of each version type to simulate
- `devLeaseBuffer`: Number of leases reserved for developer testing sessions
- `meanDuration`: Global average job duration (Gaussian distribution mean)
- `jobDurationStdDev` (or `jobDurationStandardDeviation`): Global standard deviation for duration variation

Jobs are templates with:
- `name`: Job scenario name (expanded to multiple versions)
- `onReleaseController`: If true, triggers on both cron schedule AND release events

#### Individual Job Format (Legacy)
For custom scheduling scenarios:
- Jobs require: `name`, `meanDuration`, `stdDev`, `triggerType` (`cron` or `release-controller`)
- Cron jobs need: `cronSchedule` (5-field cron expression)
- Release controller jobs: Set `isReleaseController: true` (automatically set during parsing)

### Cron Job Instance Generation

Uses `github.com/robfig/cron/v3` parser with all fields enabled (Minute|Hour|Dom|Month|Dow). Iterates through simulation period calling `schedule.Next()` to generate all occurrences.

### Release Controller Job Generation

Release controller jobs are simulated with random intervals that vary by version category:
- **Dev versions**: 4-8 hours between releases (frequent development builds)
- **Supported versions**: 4-24 hours between releases (regular updates)
- **EUS versions**: 4 hours to 5 days between releases (infrequent updates)

Jobs are grouped by version, and each version has independent release events. In template-based configs, jobs with `onReleaseController: true` trigger on BOTH:
1. Daily cron schedule (auto-staggered across 24 hours)
2. Simulated release events (random intervals based on version category)

### Developer Session Generation

When `devLeaseBuffer` > 0, the simulator generates synthetic developer testing sessions:
- Sessions are randomly distributed across the simulation period
- Targets 40% average utilization of the `devLeaseBuffer` leases
- Duration follows same Gaussian distribution as regular jobs
- Simulates ad-hoc developer usage that competes with CI jobs for leases

## Development Notes

- Simulation starts at last Monday midnight (for consistent timeline display across runs)
- Simulation runs in 5-minute time steps
- Time points for charting sampled every 30 minutes
- Job durations calculated using Gaussian distribution via `config.CalculateGaussianDuration()` helper
- Template-based configs auto-expand jobs across versions with staggered start times
- Developer sessions generated randomly if `devLeaseBuffer` > 0 (targets 40% utilization)
- No tests currently exist in codebase
- Uses standard Go duration parsing for YAML fields (e.g., `72h`, `6h30m`, `5h15m`)

### Recent Code Optimizations

The codebase has been optimized for maintainability:
- Gaussian duration calculation extracted to shared helper function
- String building uses `strings.Builder` for efficiency
- `IsReleaseController` flag properly set during parsing (not validation)
- TimePoints generation correctly tracks waiting jobs
- Removed unused code and outdated comments
