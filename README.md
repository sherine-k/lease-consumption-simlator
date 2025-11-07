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

The simulator is configured via a YAML file. Here's an example:

```yaml
# Maximum number of concurrent active leases
maxActiveLeases: 10

# Maximum time to wait for a job to complete before timing out
jobTimeoutDuration: 8h

# Maximum time a job can wait for a lease before timing out
leaseWaitTimeout: 2h

# Duration of the simulation
simulationDuration: 72h

# List of CI jobs
jobs:
  # Cron-based job example
  - name: "e2e-aws-4.18"
    version: "4.18"
    scenario: "e2e-test"
    payloadType: "aws"
    duration: 6h
    triggerType: "cron"
    cronSchedule: "0 */12 * * *"  # Every 12 hours

  # Release controller job example
  - name: "release-aws-4.19"
    version: "4.19"
    scenario: "release"
    payloadType: "aws"
    duration: 6h
    triggerType: "release-controller"
    isReleaseController: true
```

### Configuration Fields

#### Global Settings

- `maxActiveLeases`: Maximum number of concurrent leases allowed
- `jobTimeoutDuration`: Maximum duration for a job to complete
- `leaseWaitTimeout`: Maximum time a job can wait for an available lease
- `simulationDuration`: Total duration to simulate (e.g., `72h`, `7d`)

#### Job Fields

- `name`: Unique identifier for the job
- `version`: Version of the software being tested
- `scenario`: Type of test scenario (e.g., `e2e-test`, `upgrade`, `conformance`)
- `payloadType`: Platform type (e.g., `aws`, `gcp`, `azure`, `metal`)
- `duration`: How long the job takes to run
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

# Full output with timeline
./leases -c config.yaml -t -l 200
```

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
- Have "reserved" lease capacity to ensure they can run
- Are considered critical and should not be blocked by regular periodic jobs

The simulator accounts for these by:
1. Reserving lease capacity equal to the number of release controller jobs
2. Allowing these jobs to acquire leases even when regular capacity is full
3. Simulating random trigger times (approximately every 8-14 hours)

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
2. **Job Instance Generation**: Creates scheduled job instances based on:
   - Cron schedules for periodic jobs
   - Simulated random intervals for release controller jobs
3. **Simulation**: Processes all job instances chronologically:
   - Tracks lease acquisition/release
   - Detects resource contention
   - Records warnings and events
4. **Visualization**: Generates charts and reports from the simulation data

## Tips for Optimal Configuration

1. **Reserve Capacity**: Ensure `maxActiveLeases` accounts for release controller jobs plus regular jobs
2. **Stagger Schedules**: Offset cron schedules to avoid many jobs starting simultaneously
3. **Monitor Warnings**: Pay attention to jobs waiting for leases - this indicates resource pressure
4. **Adjust Timeouts**: Set `leaseWaitTimeout` based on your acceptable wait times
5. **Plan for Peaks**: The chart helps identify peak usage times for capacity planning

## License

MIT
