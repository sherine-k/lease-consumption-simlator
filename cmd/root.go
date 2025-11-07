package cmd

import (
	"fmt"

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
}

func runSimulation(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	fmt.Printf("Loaded configuration from %s\n", configFile)
	fmt.Printf("  - Max Active Leases: %d\n", cfg.MaxActiveLeases)
	fmt.Printf("  - Job Timeout: %s\n", cfg.JobTimeoutDuration)
	fmt.Printf("  - Lease Wait Timeout: %s\n", cfg.LeaseWaitTimeout)
	fmt.Printf("  - Simulation Duration: %s\n", cfg.SimulationDuration)
	fmt.Printf("  - Jobs: %d\n\n", len(cfg.Jobs))

	// Create and run simulator
	sim := simulation.NewSimulator(cfg)
	if err := sim.Run(); err != nil {
		return fmt.Errorf("simulation failed: %w", err)
	}

	// Generate and display chart
	chartGen := chart.NewGenerator()

	timePoints := sim.GetTimePoints()
	events := sim.GetEvents()
	warnings := sim.GetWarnings()

	// Display lease chart
	leaseChart := chartGen.GenerateLeaseChart(timePoints, events, cfg.MaxActiveLeases)
	fmt.Println(leaseChart)

	// Display event summary
	if showEventSummary {
		eventSummary := chartGen.GenerateEventSummary(events)
		fmt.Println(eventSummary)
	}

	// Display warnings
	warningsOutput := chartGen.GenerateWarnings(warnings)
	fmt.Println(warningsOutput)

	// Display detailed timeline if requested
	if showTimeline {
		timeline := chartGen.GenerateDetailedTimeline(events, timelineLimit)
		fmt.Println(timeline)
	}

	return nil
}
