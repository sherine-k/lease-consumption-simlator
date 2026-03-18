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

The simulator supports two configuration formats: **template-based** (recommended for OpenShift CI scenarios) and **individual job** (for custom scheduling).

### Template-Based Format (Recommended)

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
meanDuration: 3h
jobDurationStdDev: 1h

# Job templates - automatically expanded for each version
jobs:
  - name: e2e-conformance-parallel
    onReleaseController: true
  - name: e2e-upgrade
    onReleaseController: false
  - name: e2e-serial
    onReleaseController: false
```

**How it works:**
- Each job template is expanded into `devVersions + supportedVersions + eusVersions` instances
- Jobs are automatically staggered throughout the day to avoid simultaneous starts
- Jobs with `onReleaseController: true` also trigger on simulated release events
- All jobs use the global `meanDuration` and `jobDurationStdDev` for realistic duration variation

### Individual Job Format

For custom scheduling scenarios, define jobs individually:

```yaml
maxActiveLeases: 10
jobTimeoutDuration: 8h
leaseWaitTimeout: 2h
simulationDuration: 72h

jobs:
  # Cron-based job
  - name: "e2e-aws-nightly"
    meanDuration: 6h
    stdDev: 30m
    triggerType: "cron"
    cronSchedule: "0 2 * * *"  # Daily at 2 AM

  # Release controller job
  - name: "e2e-gcp-release"
    meanDuration: 5h
    stdDev: 45m
    triggerType: "release-controller"
    isReleaseController: true
```

### Configuration Fields

#### Global Settings

- `maxActiveLeases`: Maximum number of concurrent leases allowed
- `jobTimeoutDuration`: Maximum duration for a job to complete before timing out
- `leaseWaitTimeout`: Maximum time a job can wait for an available lease before timing out
- `simulationDuration`: Total duration to simulate (e.g., `72h`, `168h`)

#### Template-Based Format Fields

- `devVersions`: Number of development OpenShift versions to simulate
- `supportedVersions`: Number of actively supported OpenShift versions
- `eusVersions`: Number of Extended Update Support versions
- `devLeaseBuffer`: Number of leases to reserve for developer testing sessions
- `meanDuration`: Average job duration (used for all jobs)
- `jobDurationStdDev`: Standard deviation for job duration (creates realistic variation using Gaussian distribution)

**Job template fields:**
- `name`: Job scenario name (expanded to `name-{version-type}-{number}`)
- `onReleaseController`: If `true`, job triggers both on daily schedule AND on release events

#### Individual Job Format Fields

- `name`: Unique identifier for the job
- `meanDuration`: Average duration for this job
- `stdDev`: Standard deviation for duration (creates realistic variation using Gaussian distribution)
- `triggerType`: Either `cron` or `release-controller`
- `cronSchedule`: Cron expression for scheduled jobs (required if `triggerType` is `cron`)
- `isReleaseController`: Set to `true` for release controller jobs

### Cron Schedule Format

The cron schedule uses the standard 5-field format:

```
* * * * *
│ │ │ │ │
│ │ │ │ └─── Day of week (0-7, both 0 and 7 represent Sunday)
│ │ │ └───── Month (1-12)
│ │ └─────── Day of month (1-31)
│ └───────── Hour (0-23)
└─────────── Minute (0-59)
```

Examples:
- `0 */12 * * *` - Every 12 hours
- `0 0 * * *` - Daily at midnight
- `30 */6 * * *` - Every 6 hours at 30 minutes past the hour

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

The simulator displays the lease chart to the console and writes a complete report to a file (default: `simulation-report.txt`). The report includes:
- Configuration summary
- Lease usage chart
- Event summary (if enabled with `-s`)
- Warnings
- Detailed timeline (if enabled with `-t`)

## Output

The simulator provides several types of output:

### 1. Lease Usage Chart

An ASCII chart showing active leases over time:

```
Active Leases Over Time
================================================================================

 10 |-----------------███----------------------████---------------------███----
  9 |                 ████         █           ████        ██          █████
  8 |     ██          ████        ███          █████       ███ ██      █████
  7 |     ████        ██████   █  ███          ██████      ██████      ██████
  ...

Legend:
  █ - Active leases
  * - Jobs waiting for lease
  ! - Max leases exceeded
  - - Max lease threshold
```

### 2. Event Summary

Statistics about the simulation:

```
Event Summary
================================================================================

Total Events: 167
  - Leases Acquired: 81
  - Leases Released: 81
  - Jobs Waiting: 5
  - Job Timeouts: 0
  - Max Exceeded: 0
```

### 3. Warnings

Details about any issues detected:

```
Warnings
================================================================================

[2025-11-05 02:00:00] Job 'serial-aws-4.18' waiting for lease
[2025-11-06 02:00:00] Job 'serial-aws-4.18' waiting for lease

Total Warnings: 2
```

### 4. Detailed Timeline (Optional)

With `-t` flag, shows a chronological list of events:

```
Detailed Timeline
================================================================================

[08:00:00] + [1/3] Job 'e2e-aws-4.18' acquired lease
[08:30:00] + [2/3] Job 'e2e-gcp-4.18' acquired lease
[14:00:00] - [1/3] Job 'e2e-aws-4.18' completed and released lease
...
```

## Understanding Release Controller Jobs

Release controller jobs are special jobs that:
- Are triggered unpredictably when new builds are available
- Represent critical release validation that should not be blocked by regular periodic jobs

The simulator models these jobs with different trigger frequencies based on version category:
- **Dev versions**: Every 4-8 hours (frequent development builds)
- **Supported versions**: Every 4-24 hours (regular updates)
- **EUS versions**: Every 4 hours to 5 days (infrequent updates)

In the template-based format, jobs with `onReleaseController: true` trigger both:
1. On a daily cron schedule (staggered throughout the day)
2. On simulated release events (random intervals based on version category)

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
│   │   └── types.go
│   ├── simulation/        # Core simulation engine
│   │   ├── events.go
│   │   └── simulator.go
│   └── chart/             # Chart and output generation
│       └── chart.go
├── main.go                # Application entry point
├── config.yaml            # Example configuration
├── go.mod
└── README.md
```

## How It Works

1. **Configuration Loading**: Parses the YAML config and validates all settings
2. **Duration Calculation**: Job durations are calculated using a Gaussian (normal) distribution:
   - Uses `meanDuration` as the average
   - Uses `stdDev` or `jobDurationStdDev` for variation
   - Ensures realistic variation in job execution times
3. **Job Instance Generation**: Creates scheduled job instances based on:
   - Template expansion (for template-based configs)
   - Cron schedules for periodic jobs
   - Simulated random intervals for release controller jobs (frequency depends on version category)
   - Developer testing sessions (if `devLeaseBuffer` > 0)
4. **Simulation**: Processes all job instances chronologically (5-minute time steps):
   - Tracks lease acquisition/release
   - Manages waiting queue (FIFO)
   - Detects resource contention
   - Records timeout events
5. **Visualization**: Generates charts and reports from the simulation data (30-minute sampling for charts)

## Tips for Optimal Configuration

1. **Reserve Capacity**: Ensure `maxActiveLeases` accounts for peak concurrent jobs across all versions
2. **Use Template Format**: For OpenShift CI scenarios, the template-based format automatically handles version expansion and scheduling
3. **Configure Duration Variation**: Use `jobDurationStdDev` to model realistic job duration variation (typically 20-30% of mean)
4. **Developer Buffer**: Set `devLeaseBuffer` to simulate ad-hoc developer testing that competes for leases
5. **Monitor Warnings**: Pay attention to jobs waiting for leases - this indicates resource pressure
6. **Adjust Timeouts**: Set `leaseWaitTimeout` based on your acceptable wait times (should be > meanDuration)
7. **Plan for Peaks**: The chart helps identify peak usage times for capacity planning
8. **Test Different Scenarios**: Compare `config_default.yaml`, `config_projected.yaml`, and `config_ideal.yaml` to understand capacity needs

## License

MIT
