package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/sherine-k/leases/pkg/chart"
	"github.com/sherine-k/leases/pkg/config"
	"github.com/sherine-k/leases/pkg/simulation"
	"github.com/spf13/cobra"
)

var (
	configFile       string
	showTimeline     bool
	timelineLimit    int
	showEventSummary bool
	outputFile       string
)

var rootCmd = &cobra.Command{
	Use:   "leases",
	Short: "CI Job Lease Simulator",
	Long: `A CLI tool that simulates CI job lease usage over time.

This tool reads a configuration file containing CI jobs with their schedules,
simulates their execution, and generates a visual chart showing lease usage
over time along with warnings for potential issues.`,
	RunE: runSimulation,
}

// Execute runs the root command
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.Flags().StringVarP(&configFile, "config", "c", "config.yaml", "Path to configuration file")
	rootCmd.Flags().BoolVarP(&showTimeline, "timeline", "t", false, "Show detailed timeline of events")
	rootCmd.Flags().IntVarP(&timelineLimit, "timeline-limit", "l", 50, "Limit number of timeline events to display")
	rootCmd.Flags().BoolVarP(&showEventSummary, "summary", "s", true, "Show event summary")
	rootCmd.Flags().StringVarP(&outputFile, "output", "o", "simulation-report.txt", "Path to output file for event summary and warnings")
}

func runSimulation(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Build configuration summary
	configSummary := fmt.Sprintf("Loaded configuration from %s\n", configFile)
	configSummary += fmt.Sprintf("  - Max Active Leases: %d\n", cfg.MaxActiveLeases)
	configSummary += fmt.Sprintf("  - Job Timeout: %s\n", cfg.JobTimeoutDuration)
	configSummary += fmt.Sprintf("  - Lease Wait Timeout: %s\n", cfg.LeaseWaitTimeout)
	configSummary += fmt.Sprintf("  - Simulation Duration: %s\n", cfg.SimulationDuration)
	configSummary += fmt.Sprintf("  - Jobs: %d\n\n", len(cfg.Jobs))

	// Display config summary to console
	fmt.Print(configSummary)

	// Create and run simulator
	sim := simulation.NewSimulator(cfg)
	if err := sim.Run(); err != nil {
		return fmt.Errorf("simulation failed: %w", err)
	}

	// Generate outputs
	chartGen := chart.NewGenerator()

	timePoints := sim.GetTimePoints()
	events := sim.GetEvents()
	warnings := sim.GetWarnings()

	leaseChart := chartGen.GenerateLeaseChart(timePoints, events, cfg.MaxActiveLeases)

	// Display lease chart to console
	fmt.Println(leaseChart)

	// Build complete output for file
	var fileContent strings.Builder
	fileContent.WriteString(configSummary)
	fileContent.WriteString(leaseChart)

	if showEventSummary {
		eventSummary := chartGen.GenerateEventSummary(events)
		fileContent.WriteString(eventSummary)
	}

	warningsOutput := chartGen.GenerateWarnings(warnings)
	fileContent.WriteString(warningsOutput)

	if showTimeline {
		timeline := chartGen.GenerateDetailedTimeline(events, timelineLimit)
		fileContent.WriteString(timeline)
	}

	// Write complete output to file
	if err := os.WriteFile(outputFile, []byte(fileContent.String()), 0644); err != nil {
		return fmt.Errorf("failed to write output file: %w", err)
	}

	fmt.Printf("Complete simulation report written to: %s\n", outputFile)

	return nil
}
