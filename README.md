# CI Job Lease Simulator

A Go CLI tool that simulates CI job lease usage over time and generates visual charts to help identify potential resource contention issues.

## Features

- Simulates CI job execution based on cron schedules or release controller triggers
- Tracks lease acquisition and release over time
- Generates ASCII timeseries charts showing active leases vs time
- Detects and warns about:
  - Jobs waiting for available leases
  - Max active leases being exceeded
  - Job timeouts
- Supports two types of jobs:
  - **Cron-based jobs**: Scheduled at specific times using cron expressions
  - **Release controller jobs**: Triggered unpredictably with reserved lease capacity

## Installation

```bash
# Clone the repository
git clone https://github.com/sherine-k/leases.git
cd leases

# Build the binary
go build -o leases .

# Run the simulator
./leases -c config.yaml
```

## Configuration

The simulator uses a **template-based configuration format** designed for OpenShift CI scenarios running tests for multiple versions of OpenShift.

### Configuration Format

This format automatically generates job instances across multiple OpenShift versions:

```yaml
# Maximum number of concurrent active leases
maxActiveLeases: 20

# Maximum time to wait for a job to complete before timing out
jobTimeoutDuration: 5h15m

# Maximum time a job can wait for a lease before timing out
leaseWaitTimeout: 5h

# Duration of the simulation
simulationDuration: 168h

# Number of OpenShift development releases
devVersions: 1

# Number of actively supported OpenShift versions
supportedVersions: 5

# Number of EUS (Extended Update Support) releases
eusVersions: 5

# Number of leases reserved as buffer for developer testing
devLeaseBuffer: 0

# Job duration parameters (Gaussian distribution)
# Each job instance gets a unique duration based on these parameters
meanDuration: 3h
jobDurationStandardDeviation: 1h

# Optional: Configure release controller trigger frequencies per version category
# Uses Gaussian distribution (mean ± stddev)
# (defaults shown below)
devReleaseIntervalMean: 6h
devReleaseIntervalStdDev: 2h
supportedReleaseIntervalMean: 8h
supportedReleaseIntervalStdDev: 4h
eusReleaseIntervalMean: 24h
eusReleaseIntervalStdDev: 8h

# Job templates - automatically expanded for each version
jobs:
  - name: e2e-conformance-parallel
    onReleaseController: [dev, supported]  # Triggers on releases for these version categories
  - name: e2e-upgrade
    # No onReleaseController - cron-only job
  - name: e2e-serial
    onReleaseController: [dev]  # Triggers only on dev releases
```

**How it works:**
- Each job template is expanded into `devVersions + supportedVersions + eusVersions` instances
- Jobs are automatically staggered throughout the day to avoid simultaneous starts
- Jobs with `onReleaseController: [dev]` (or other categories) trigger on both:
  1. Daily cron schedule (auto-staggered)
  2. Simulated release events (random intervals based on version category)
- All jobs use the global `meanDuration` and `jobDurationStdDev` for realistic duration variation
- **Each job instance gets a unique duration** calculated using Gaussian (normal) distribution

### Configuration Fields

#### Core Settings

- `maxActiveLeases`: Maximum number of concurrent leases allowed
- `jobTimeoutDuration`: Maximum duration for a job to complete before timing out
- `leaseWaitTimeout`: Maximum time a job can wait for an available lease before timing out
- `simulationDuration`: Total duration to simulate (e.g., `72h`, `168h`)
- `devVersions`: Number of development OpenShift versions to simulate
- `supportedVersions`: Number of actively supported OpenShift versions
- `eusVersions`: Number of Extended Update Support versions
- `devLeaseBuffer`: Number of leases to reserve for developer testing sessions

#### Job Duration Parameters

- `meanDuration`: Average job duration (applied globally to all jobs)
- `jobDurationStdDev`: Standard deviation for duration variation
  - Creates realistic variation using Gaussian (normal) distribution
  - Each job instance gets a unique duration calculated at generation time
  - ~68% of jobs within ±1σ, ~95% within ±2σ, ~99.7% within ±3σ

#### Release Controller Intervals (Optional)

Control how frequently release events are simulated per version category using Gaussian distribution:
- `devReleaseIntervalMean`, `devReleaseIntervalStdDev`: Dev version release frequency (default: mean=6h, stddev=2h)
- `supportedReleaseIntervalMean`, `supportedReleaseIntervalStdDev`: Supported version release frequency (default: mean=8h, stddev=4h)
- `eusReleaseIntervalMean`, `eusReleaseIntervalStdDev`: EUS version release frequency (default: mean=24h, stddev=8h)
- Each interval between releases is calculated using Gaussian distribution, providing realistic variation

#### Quantity of OCP releases being tested by category
Controls how many active versions of OCP are being tested by the simulation: 
- `devVersions`: Number of releases under development being tested
- `supportedVersions`: Number of releases in full suported being tested
- `eusVersions`: Number of releases in Extended support being tested

#### Job Template Fields

- `name`: Job scenario name (expanded to `name-{version-type}-{number}`)
- `onReleaseController`: Array of version categories where job triggers on releases (e.g., `[dev]`, `[supported]`, `[dev, supported, eus]`)
  - Jobs trigger on BOTH daily cron schedule AND release events for specified categories
  - Omit this field for cron-only jobs


## Usage

### Basic Usage

```bash
# Run with default config.yaml
./leases

# Specify a custom config file
./leases -c path/to/config.yaml
```

### Command-Line Options

```bash
./leases [flags]

Flags:
  -c, --config string        Path to configuration file (default "config.yaml")
  -h, --help                 Help for leases
  -o, --output string        Path to output file for complete report (default "simulation-report.txt")
  -s, --summary              Show event summary (default true)
  -t, --timeline             Show detailed timeline of events
  -l, --timeline-limit int   Limit number of timeline events to display (default 50)
```

### Examples

```bash
# Show detailed timeline with first 100 events
./leases -t -l 100

# Run without event summary
./leases --summary=false

# Full output with timeline, custom output file
./leases -c config.yaml -t -l 200 -o report.txt

# Use a specific config
./leases -c config_ideal.yaml
```

### Output Files

The simulator displays results to the console and writes a complete report to a file (default: `simulation-report.txt`). The report includes:
- Configuration summary
- Executive summary with health status
- Hourly statistics breakdown
- Problem timeline (timeouts and waits)
- Detailed event timeline (if enabled with `-t`)

## Output

The simulator provides comprehensive visualization focusing on identifying problems:

### 1. Executive Summary

Health status and key metrics at a glance:

```
================================================================================
EXECUTIVE SUMMARY
================================================================================

Simulation Period: Mon 03/24 00:00 - Tue 03/25 00:00 (24h0m0s)
Peak Lease Usage: 98% (49/50 leases)
Total Jobs Executed: 156 jobs
Average Utilization: 92%

Health Status: CRITICAL
  - 121 execution timeouts detected
  - 169 lease wait timeouts detected
  - 0 jobs waiting for leases

Peak Congestion: Mon 03/24 16:00 - 49 active leases (98%)
```

### 2. Hourly Statistics

Breakdown of lease usage and problems by hour:

```
================================================================================
HOURLY STATISTICS
================================================================================

Hour       Avg    Peak   Waiting   Problems
00:00-01:00  45.0   47      0      -
01:00-02:00  46.2   48      0      -
14:00-15:00  48.5   49      3      7 timeouts, 12 waits
15:00-16:00  47.8   49      5      15 timeouts, 18 waits
...
```

### 3. Problem Timeline

Focused view of issues (timeouts and waits only):

```
================================================================================
PROBLEM TIMELINE
================================================================================

Mon 03/24 06:15 - Job 'e2e-conformance-dev-1' TIMEOUT (execution exceeded 5h15m0s)
Mon 03/24 14:30 - Job 'e2e-upgrade-supported-2' WAITING for lease
Mon 03/24 19:45 - Job 'e2e-serial-eus-3' TIMEOUT (waited 5h0m0s for lease)
...

Total Problems: 290 (121 execution timeouts, 169 lease wait timeouts)
```

### 4. Detailed Timeline (Optional)

With `-t` flag, shows all events chronologically:

```
Detailed Timeline
================================================================================

[08:00:00] + [1/50] Job 'e2e-aws-4.18' acquired lease
[08:30:00] + [2/50] Job 'e2e-gcp-4.18' acquired lease
[14:00:00] - [1/50] Job 'e2e-aws-4.18' completed and released lease
...
```

## Understanding Release Controller Jobs

Release controller jobs are triggered when new builds are available. The simulator models these jobs with configurable trigger frequencies per version category using Gaussian distribution:
- **Dev versions**: Default mean=6h, stddev=2h (frequent development builds)
- **Supported versions**: Default mean=8h, stddev=4h (regular updates)
- **EUS versions**: Default mean=24h, stddev=8h (infrequent updates)

Jobs with `onReleaseController: [dev]` (or other categories) trigger on BOTH:
1. Daily cron schedule (auto-staggered across the day)
2. Simulated release events (Gaussian-distributed intervals based on configured mean and standard deviation for that category)

You can customize release frequencies using the `*ReleaseIntervalMean` and `*ReleaseIntervalStdDev` configuration fields for each version category.

## Exit Codes

- `0`: Simulation completed successfully with no warnings
- `1`: Simulation completed but warnings were detected

## Project Structure

```
.
├── cmd/                    # CLI command implementation
│   └── root.go
├── pkg/
│   ├── config/            # Configuration parsing and types
│   │   ├── parser.go
│   │   ├── parser_test.go
│   │   └── types.go
│   ├── simulation/        # Core simulation engine
│   │   ├── events.go
│   │   ├── simulator.go
│   │   └── simulator_test.go
│   └── chart/             # Visualization and output generation
│       ├── chart.go       # Legacy visualization
│       └── chart_new.go   # Improved visualization (default)
├── main.go                # Application entry point
├── config.yaml            # Example configuration
├── go.mod
└── README.md
```

## How It Works

1. **Configuration Loading**: Parses the YAML config and validates all settings
2. **Job Instance Generation**: Creates scheduled job instances based on:
   - Template expansion across all configured versions
   - Cron schedules for periodic jobs (auto-staggered)
   - Simulated random intervals for release controller jobs (based on configured release intervals)
   - Developer testing sessions (if `devLeaseBuffer` > 0)
3. **Per-Instance Duration Calculation**: Each job instance gets a unique duration:
   - Calculated when the instance is generated using Gaussian (normal) distribution
   - Uses `meanDuration` as the average and `jobDurationStandardDeviation` for variation
   - Ensures realistic variation across job executions
   - Statistical distribution: ~68% within ±1σ, ~95% within ±2σ, ~99.7% within ±3σ
4. **Simulation**: Processes all job instances chronologically (5-minute time steps):
   - Tracks lease acquisition/release
   - Manages waiting queue (FIFO - first come, first served)
   - Detects resource contention and timeout conditions
   - Records all events with timestamps
5. **Visualization**: Generates charts and reports from simulation data:
   - Time points sampled every **15 minutes** (tracks peak active leases during interval)
   - Executive summary with health status
   - Hourly statistics breakdown
   - Problem timeline focusing on timeouts and waits

## Tips for Optimal Configuration

1. **Reserve Capacity**: Ensure `maxActiveLeases` accounts for peak concurrent jobs across all versions
2. **Configure Duration Variation**: Use `jobDurationStdDev` to model realistic job duration variation (typically 20-30% of mean)
   - Each job instance will get a unique duration from the Gaussian distribution
   - ~4% of jobs naturally exceed mean+1.75σ (this is expected behavior)
3. **Developer Buffer**: Set `devLeaseBuffer` to simulate ad-hoc developer testing that competes for leases
4. **Monitor Problems**: The Problem Timeline focuses on timeouts and waits - these indicate resource pressure
5. **Adjust Timeouts**: Set `leaseWaitTimeout` based on your acceptable wait times (should be > meanDuration)
6. **Configure Release Intervals**: Adjust `*ReleaseIntervalMean` and `*ReleaseIntervalStdDev` to match your actual release cadence per version category (uses Gaussian distribution like job durations)
7. **Plan for Peaks**: Use Hourly Statistics to identify peak usage times for capacity planning
8. **Test Different Scenarios**: Create configs with different capacity and version counts to understand resource needs
9. **Understand Statistics**: With Gaussian distribution, some jobs will naturally take longer - look for patterns, not individual outliers

## Testing

The project includes comprehensive unit tests:

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run specific test package
go test ./pkg/config
go test ./pkg/simulation
```

### Test Coverage

**pkg/config/parser_test.go** (94.2% coverage)
- Configuration loading and validation
- Template expansion logic
- Error handling for invalid configs
- Integration tests with real config files

**pkg/simulation/simulator_test.go**
- Gaussian duration calculation
- Basic job scheduling (cron-based)
- Lease contention scenarios
- Lease wait timeout detection
- Per-instance duration variation

Tests validate critical functionality including:
- Per-instance duration calculation using Gaussian distribution
- Peak lease tracking during sampling intervals
- Timeout detection (both execution and lease wait timeouts)
- FIFO lease assignment from waiting queue

## License

MIT
