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

# Run with default config.yaml
./leases

# Run with custom config file
./leases -c path/to/config.yaml

# Show detailed timeline with event limit
./leases -t -l 100

# Run without event summary
./leases --summary=false
```

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
- `root.go`: Cobra CLI implementation with flags for timeline display, event limits, and summary control

### Lease Management Logic

The simulator implements a reservation system for release controller jobs:
- **Reserved leases**: Count of release controller jobs reserves capacity from total max leases
- **Regular jobs**: Can use `maxActiveLeases - activeLeases` capacity
- **Release controller jobs**: Can use `maxActiveLeases - (activeLeases - reservedLeases)` capacity, ensuring they always have reserved slots

Simulation runs in 5-minute time steps, processing:
1. Job starts and lease acquisitions
2. Job completions and lease releases
3. Waiting queue management (FIFO)
4. Timeout detection for both lease waits and job execution

### Event System

Events track all state changes with timestamps and generate warnings when:
- Jobs wait for available leases
- Active leases exceed configured maximum
- Jobs timeout waiting for leases (after `leaseWaitTimeout`)
- Jobs exceed execution timeout (after `jobTimeoutDuration`)

Exit code is 1 if any warnings occurred during simulation.

### Configuration Structure

Critical config fields:
- `maxActiveLeases`: Total concurrent lease limit
- `jobTimeoutDuration`: Max execution time before job timeout warning
- `leaseWaitTimeout`: Max wait time before lease acquisition timeout warning
- `simulationDuration`: Total time to simulate

Jobs require: `name`, `duration`, `triggerType` (`cron` or `release-controller`)
- Cron jobs need: `cronSchedule` (5-field cron expression)
- Release controller jobs: Set `isReleaseController: true`

### Cron Job Instance Generation

Uses `github.com/robfig/cron/v3` parser with all fields enabled (Minute|Hour|Dom|Month|Dow). Iterates through simulation period calling `schedule.Next()` to generate all occurrences.

### Release Controller Job Generation

Simulated with pseudo-random intervals averaging ~10 hours between triggers (starting 2 hours into simulation). Current implementation uses deterministic randomness based on instance count.

## Development Notes

- Simulation starts at current hour truncated (for consistent timeline display)
- Time points for charting sampled every 30 minutes
- No tests currently exist in codebase
- Uses standard Go duration parsing for YAML fields (e.g., `72h`, `6h30m`)
