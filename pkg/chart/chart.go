package chart

import (
	"fmt"
	"strings"
	"time"

	"github.com/sherine-k/leases/pkg/simulation"
)

const (
	chartWidth  = 80
	chartHeight = 20
)

// Generator generates ASCII charts
type Generator struct {
	width  int
	height int
}

// NewGenerator creates a new chart generator
func NewGenerator() *Generator {
	return &Generator{
		width:  chartWidth,
		height: chartHeight,
	}
}

// GenerateImprovedReport generates a comprehensive report focused on lease pressure and issues
func (g *Generator) GenerateImprovedReport(timePoints []simulation.TimePoint, events []simulation.Event, maxLeases int, simulationStart, simulationEnd time.Time) string {
	var sb strings.Builder
	sb.Grow(10000) // Pre-allocate for large output

	// Calculate statistics
	stats := calculateStatistics(timePoints, events, maxLeases)

	// Header
	sb.WriteString("\n")
	sb.WriteString("LEASE SIMULATION ANALYSIS REPORT\n")
	sb.WriteString(strings.Repeat("=", 100))
	sb.WriteString("\n\n")

	// Executive Summary
	sb.WriteString(generateExecutiveSummary(stats, simulationStart, simulationEnd))

	// Hourly Breakdown
	sb.WriteString(generateHourlyBreakdown(timePoints, events, maxLeases, simulationStart))

	// Problem Timeline (focused on issues)
	sb.WriteString(generateProblemTimeline(events))

	return sb.String()
}

// SimulationStats holds calculated statistics
type SimulationStats struct {
	AvgLeaseUsage        float64
	PeakLeaseUsage       int
	UtilizationPct       float64
	ExecutionTimeouts    int  // Jobs that ran too long
	LeaseWaitTimeouts    int  // Jobs that waited too long for a lease
	TotalWaiting         int
	MaxWaiting           int
	PeakUtilization      float64
	TimeoutPeriods       []TimePeriod
	HighPressurePeriods  []TimePeriod
}

// TimePeriod represents a time range
type TimePeriod struct {
	Start time.Time
	End   time.Time
	Count int
}

func calculateStatistics(timePoints []simulation.TimePoint, events []simulation.Event, maxLeases int) SimulationStats {
	stats := SimulationStats{}

	if len(timePoints) == 0 {
		return stats
	}

	totalUsage := 0
	maxWaiting := 0

	for _, tp := range timePoints {
		totalUsage += tp.ActiveLeases
		if tp.ActiveLeases > stats.PeakLeaseUsage {
			stats.PeakLeaseUsage = tp.ActiveLeases
		}
		if tp.WaitingJobs > maxWaiting {
			maxWaiting = tp.WaitingJobs
		}
	}

	stats.AvgLeaseUsage = float64(totalUsage) / float64(len(timePoints))
	stats.UtilizationPct = (stats.AvgLeaseUsage / float64(maxLeases)) * 100
	stats.PeakUtilization = (float64(stats.PeakLeaseUsage) / float64(maxLeases)) * 100
	stats.MaxWaiting = maxWaiting

	// Count event types
	for _, event := range events {
		switch event.Type {
		case simulation.EventTypeJobTimeout:
			stats.ExecutionTimeouts++
		case simulation.EventTypeLeaseWaitTimeout:
			stats.LeaseWaitTimeouts++
		case simulation.EventTypeJobWaiting:
			stats.TotalWaiting++
		}
	}

	// Identify problem periods
	stats.TimeoutPeriods = findTimeoutPeriods(events)
	stats.HighPressurePeriods = findHighPressurePeriods(timePoints, maxLeases)

	return stats
}

func findTimeoutPeriods(events []simulation.Event) []TimePeriod {
	periods := []TimePeriod{}
	var currentPeriod *TimePeriod

	for _, event := range events {
		if event.Type == simulation.EventTypeJobTimeout || event.Type == simulation.EventTypeLeaseWaitTimeout {
			if currentPeriod == nil {
				currentPeriod = &TimePeriod{
					Start: event.Time,
					End:   event.Time,
					Count: 1,
				}
			} else if event.Time.Sub(currentPeriod.End) < 1*time.Hour {
				// Extend current period
				currentPeriod.End = event.Time
				currentPeriod.Count++
			} else {
				// Start new period
				periods = append(periods, *currentPeriod)
				currentPeriod = &TimePeriod{
					Start: event.Time,
					End:   event.Time,
					Count: 1,
				}
			}
		}
	}

	if currentPeriod != nil {
		periods = append(periods, *currentPeriod)
	}

	return periods
}

func findHighPressurePeriods(timePoints []simulation.TimePoint, maxLeases int) []TimePeriod {
	periods := []TimePeriod{}
	var currentPeriod *TimePeriod
	threshold := float64(maxLeases) * 0.90 // 90% utilization

	for _, tp := range timePoints {
		if float64(tp.ActiveLeases) >= threshold || tp.WaitingJobs > 0 {
			if currentPeriod == nil {
				currentPeriod = &TimePeriod{
					Start: tp.Time,
					End:   tp.Time,
					Count: 1,
				}
			} else if tp.Time.Sub(currentPeriod.End) < 30*time.Minute {
				currentPeriod.End = tp.Time
				currentPeriod.Count++
			} else {
				periods = append(periods, *currentPeriod)
				currentPeriod = &TimePeriod{
					Start: tp.Time,
					End:   tp.Time,
					Count: 1,
				}
			}
		}
	}

	if currentPeriod != nil {
		periods = append(periods, *currentPeriod)
	}

	return periods
}

func generateExecutiveSummary(stats SimulationStats, start, end time.Time) string {
	var sb strings.Builder

	duration := end.Sub(start)

	sb.WriteString("EXECUTIVE SUMMARY\n")
	sb.WriteString(strings.Repeat("-", 100))
	sb.WriteString("\n\n")

	sb.WriteString(fmt.Sprintf("Simulation Period: %s to %s (%.0f hours)\n\n",
		start.Format("2006-01-02 15:04"),
		end.Format("2006-01-02 15:04"),
		duration.Hours()))

	sb.WriteString("Lease Utilization:\n")
	sb.WriteString(fmt.Sprintf("  • Average Usage:    %.1f leases (%.1f%% utilization)\n",
		stats.AvgLeaseUsage, stats.UtilizationPct))
	sb.WriteString(fmt.Sprintf("  • Peak Usage:       %d leases (%.1f%% utilization)\n",
		stats.PeakLeaseUsage, stats.PeakUtilization))
	sb.WriteString("\n")

	// Status indicator
	status := "✓ HEALTHY"
	if stats.ExecutionTimeouts > 0 || stats.LeaseWaitTimeouts > 0 {
		status = "✗ CRITICAL - Timeouts detected"
	} else if stats.MaxWaiting > 0 {
		status = "⚠ WARNING - Jobs waiting for leases"
	} else if stats.PeakUtilization > 90 {
		status = "⚠ WARNING - High utilization (>90%)"
	}

	sb.WriteString(fmt.Sprintf("Status: %s\n\n", status))

	sb.WriteString("Problem Summary:\n")
	sb.WriteString(fmt.Sprintf("  • Execution Timeouts:     %d (jobs ran too long)\n", stats.ExecutionTimeouts))
	sb.WriteString(fmt.Sprintf("  • Lease Wait Timeouts:    %d (waited too long for lease)\n", stats.LeaseWaitTimeouts))
	sb.WriteString(fmt.Sprintf("  • Jobs Waiting:           %d events\n", stats.TotalWaiting))
	sb.WriteString(fmt.Sprintf("  • Max Queue Depth:        %d concurrent waiting jobs\n", stats.MaxWaiting))

	if len(stats.TimeoutPeriods) > 0 {
		sb.WriteString(fmt.Sprintf("  • Timeout Periods:  %d distinct periods\n", len(stats.TimeoutPeriods)))
	}
	if len(stats.HighPressurePeriods) > 0 {
		sb.WriteString(fmt.Sprintf("  • High Pressure:    %d periods with >90%% utilization\n", len(stats.HighPressurePeriods)))
	}

	sb.WriteString("\n\n")

	return sb.String()
}

func generateProblemTimeline(events []simulation.Event) string {
	var sb strings.Builder

	sb.WriteString("PROBLEM TIMELINE\n")
	sb.WriteString(strings.Repeat("-", 100))
	sb.WriteString("\n\n")

	// Extract problem events
	problems := []simulation.Event{}
	for _, event := range events {
		if event.Type == simulation.EventTypeJobTimeout ||
			event.Type == simulation.EventTypeLeaseWaitTimeout ||
			event.Type == simulation.EventTypeJobWaiting ||
			event.Type == simulation.EventTypeMaxExceeded {
			problems = append(problems, event)
		}
	}

	if len(problems) == 0 {
		sb.WriteString("✓ No timeouts, queue waits, or capacity issues detected!\n\n")
		return sb.String()
	}

	// Show first 50 problems
	displayCount := len(problems)
	if displayCount > 50 {
		displayCount = 50
	}

	sb.WriteString(fmt.Sprintf("%-20s %-10s %-12s %s\n", "Timestamp", "Type", "Active/Max", "Details"))
	sb.WriteString(strings.Repeat("-", 100))
	sb.WriteString("\n")

	for i := 0; i < displayCount; i++ {
		event := problems[i]

		typeStr := ""
		switch event.Type {
		case simulation.EventTypeJobTimeout:
			typeStr = "EXEC_TIMEOUT"
		case simulation.EventTypeLeaseWaitTimeout:
			typeStr = "WAIT_TIMEOUT"
		case simulation.EventTypeJobWaiting:
			typeStr = "WAITING"
		case simulation.EventTypeMaxExceeded:
			typeStr = "EXCEEDED"
		}

		sb.WriteString(fmt.Sprintf("%-20s %-10s %-12s %s\n",
			event.Time.Format("2006-01-02 15:04:05"),
			typeStr,
			fmt.Sprintf("%d", event.ActiveLeases),
			truncateString(event.Message, 60)))
	}

	if len(problems) > displayCount {
		sb.WriteString(fmt.Sprintf("\n... and %d more problem events (see detailed timeline for all)\n", len(problems)-displayCount))
	}

	sb.WriteString("\n\n")

	return sb.String()
}

func generateHourlyBreakdown(timePoints []simulation.TimePoint, events []simulation.Event, maxLeases int, start time.Time) string {
	var sb strings.Builder

	sb.WriteString("HOURLY STATISTICS\n")
	sb.WriteString(strings.Repeat("-", 100))
	sb.WriteString("\n\n")

	if len(timePoints) == 0 {
		sb.WriteString("No data available\n\n")
		return sb.String()
	}

	// Find max hour
	lastPoint := timePoints[len(timePoints)-1]
	maxHour := int(lastPoint.Time.Sub(start).Hours()) + 1

	// Aggregate by hour
	type HourStats struct {
		avgUtil        float64
		peakUtil       int
		execTimeouts   int
		waitTimeouts   int
		waiting        int
		samples        int
	}

	hourStats := make([]HourStats, maxHour)

	// Process time points
	for _, tp := range timePoints {
		hour := int(tp.Time.Sub(start).Hours())
		if hour >= 0 && hour < maxHour {
			hourStats[hour].avgUtil += float64(tp.ActiveLeases)
			if tp.ActiveLeases > hourStats[hour].peakUtil {
				hourStats[hour].peakUtil = tp.ActiveLeases
			}
			hourStats[hour].samples++
		}
	}

	// Process events
	for _, event := range events {
		hour := int(event.Time.Sub(start).Hours())
		if hour >= 0 && hour < maxHour {
			if event.Type == simulation.EventTypeJobTimeout {
				hourStats[hour].execTimeouts++
			} else if event.Type == simulation.EventTypeLeaseWaitTimeout {
				hourStats[hour].waitTimeouts++
			} else if event.Type == simulation.EventTypeJobWaiting {
				hourStats[hour].waiting++
			}
		}
	}

	// Calculate averages
	for i := range hourStats {
		if hourStats[i].samples > 0 {
			hourStats[i].avgUtil /= float64(hourStats[i].samples)
		}
	}

	// Print table
	sb.WriteString(fmt.Sprintf("%-12s %-15s %-15s %-12s %-12s %-12s\n",
		"Time Range", "Avg Util %", "Peak Util %", "Exec T/O", "Wait T/O", "Waiting"))
	sb.WriteString(strings.Repeat("-", 100))
	sb.WriteString("\n")

	for hour, stats := range hourStats {
		if stats.samples == 0 {
			continue
		}

		day := hour / 24
		hourOfDay := hour % 24

		avgPct := (stats.avgUtil / float64(maxLeases)) * 100
		peakPct := (float64(stats.peakUtil) / float64(maxLeases)) * 100

		timeRange := fmt.Sprintf("D%d %02d:00", day, hourOfDay)

		sb.WriteString(fmt.Sprintf("%-12s %-15s %-15s %-12d %-12d %-12d\n",
			timeRange,
			fmt.Sprintf("%.1f%%", avgPct),
			fmt.Sprintf("%.1f%%", peakPct),
			stats.execTimeouts,
			stats.waitTimeouts,
			stats.waiting))
	}

	sb.WriteString("\n\n")

	return sb.String()
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// GenerateEventSummary generates a summary of events
func (g *Generator) GenerateEventSummary(events []simulation.Event) string {
	var sb strings.Builder
	sb.Grow(500) // Small summary, pre-allocate modest buffer

	sb.WriteString("\n")
	sb.WriteString("Event Summary\n")
	sb.WriteString(strings.Repeat("=", g.width))
	sb.WriteString("\n\n")

	// Group events by type
	eventsByType := make(map[simulation.EventType]int)
	for _, event := range events {
		eventsByType[event.Type]++
	}

	sb.WriteString(fmt.Sprintf("Total Events: %d\n", len(events)))
	sb.WriteString(fmt.Sprintf("  - Leases Acquired: %d\n", eventsByType[simulation.EventTypeLeaseAcquired]))
	sb.WriteString(fmt.Sprintf("  - Leases Released: %d\n", eventsByType[simulation.EventTypeLeaseReleased]))
	sb.WriteString(fmt.Sprintf("  - Jobs Waiting: %d\n", eventsByType[simulation.EventTypeJobWaiting]))
	sb.WriteString(fmt.Sprintf("  - Execution Timeouts: %d\n", eventsByType[simulation.EventTypeJobTimeout]))
	sb.WriteString(fmt.Sprintf("  - Lease Wait Timeouts: %d\n", eventsByType[simulation.EventTypeLeaseWaitTimeout]))
	sb.WriteString(fmt.Sprintf("  - Max Exceeded: %d\n", eventsByType[simulation.EventTypeMaxExceeded]))
	sb.WriteString("\n")

	return sb.String()
}

// GenerateWarnings generates a list of warnings
func (g *Generator) GenerateWarnings(warnings []simulation.Event) string {
	var sb strings.Builder
	// Estimate: ~100 chars per warning + header
	sb.Grow(len(warnings)*100 + 200)

	sb.WriteString("\n")
	sb.WriteString("Warnings\n")
	sb.WriteString(strings.Repeat("=", g.width))
	sb.WriteString("\n\n")

	if len(warnings) == 0 {
		sb.WriteString("No warnings!\n")
		return sb.String()
	}

	for _, warning := range warnings {
		timestamp := warning.Time.Format("2006-01-02 15:04:05")
		sb.WriteString(fmt.Sprintf("[%s] %s\n", timestamp, warning.Message))
	}

	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("Total Warnings: %d\n", len(warnings)))
	sb.WriteString("\n")

	return sb.String()
}

// GenerateDetailedTimeline generates a detailed timeline of events
func (g *Generator) GenerateDetailedTimeline(events []simulation.Event, limit int) string {
	var sb strings.Builder
	displayCount := len(events)
	if limit > 0 && limit < displayCount {
		displayCount = limit
	}
	// Estimate: ~100 chars per event line + header
	sb.Grow(displayCount*100 + 200)

	sb.WriteString("\n")
	sb.WriteString("Detailed Timeline")
	if limit > 0 && limit < len(events) {
		sb.WriteString(fmt.Sprintf(" (showing first %d events)", limit))
	}
	sb.WriteString("\n")
	sb.WriteString(strings.Repeat("=", g.width))
	sb.WriteString("\n\n")

	for i := 0; i < displayCount; i++ {
		event := events[i]
		timestamp := event.Time.Format("15:04:05")

		typeIcon := " "
		switch event.Type {
		case simulation.EventTypeLeaseAcquired:
			typeIcon = "+"
		case simulation.EventTypeLeaseReleased:
			typeIcon = "-"
		case simulation.EventTypeJobWaiting:
			typeIcon = "W"
		case simulation.EventTypeJobTimeout:
			typeIcon = "E"  // Execution timeout
		case simulation.EventTypeLeaseWaitTimeout:
			typeIcon = "T"  // Lease wait timeout
		case simulation.EventTypeMaxExceeded:
			typeIcon = "!"
		}

		sb.WriteString(fmt.Sprintf("[%s] %s [%d] %s\n",
			timestamp,
			typeIcon,
			event.ActiveLeases,
			event.Message))
	}

	if limit > 0 && limit < len(events) {
		sb.WriteString(fmt.Sprintf("\n... and %d more events\n", len(events)-limit))
	}

	sb.WriteString("\n")

	return sb.String()
}
