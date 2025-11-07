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

// GenerateLeaseChart generates an ASCII chart showing lease usage over time
func (g *Generator) GenerateLeaseChart(timePoints []simulation.TimePoint, events []simulation.Event, maxLeases int) string {
	if len(timePoints) == 0 {
		return "No data to display"
	}

	var sb strings.Builder

	// Header
	sb.WriteString("\n")
	sb.WriteString("Lease Usage Over Time\n")
	sb.WriteString(strings.Repeat("=", g.width))
	sb.WriteString("\n\n")

	// Build enhanced time points with timeout information
	type EnhancedTimePoint struct {
		ActiveLeases  int
		WaitingJobs   int
		TimeoutJobs   int
	}

	enhancedPoints := make([]EnhancedTimePoint, len(timePoints))

	for i, tp := range timePoints {
		timeoutCount := 0

		// Count timeout events at this specific time point
		for _, event := range events {
			if event.Time.Equal(tp.Time) && event.Type == simulation.EventTypeJobTimeout {
				timeoutCount++
			}
		}

		enhancedPoints[i] = EnhancedTimePoint{
			ActiveLeases: tp.ActiveLeases,
			WaitingJobs:  tp.WaitingJobs,
			TimeoutJobs:  timeoutCount,
		}
	}

	// Find max waiting/timeout jobs to determine chart height
	maxWaitingAndTimeout := 0
	for _, ep := range enhancedPoints {
		total := ep.WaitingJobs + ep.TimeoutJobs
		if total > maxWaitingAndTimeout {
			maxWaitingAndTimeout = total
		}
	}

	totalRows := maxLeases + maxWaitingAndTimeout

	// Build the chart from top to bottom
	// First draw waiting/timeout rows (if any)
	for row := totalRows; row > maxLeases; row-- {
		// Y-axis label
		sb.WriteString(fmt.Sprintf("%3d |", row))

		// Plot data points across time
		for x := 0; x < len(timePoints) && x < g.width-6; x++ {
			pointIndex := int(float64(x) / float64(g.width-6) * float64(len(timePoints)-1))
			if pointIndex >= len(enhancedPoints) {
				pointIndex = len(enhancedPoints) - 1
			}

			ep := enhancedPoints[pointIndex]
			waitingRow := row - maxLeases

			if waitingRow <= ep.TimeoutJobs {
				// Show timeout
				sb.WriteString("!")
			} else if waitingRow <= ep.TimeoutJobs+ep.WaitingJobs {
				// Show waiting
				sb.WriteString("*")
			} else {
				sb.WriteString(" ")
			}
		}
		sb.WriteString("\n")
	}

	// Separator line between waiting/timeout and lease slots
	if maxWaitingAndTimeout > 0 {
		sb.WriteString("    ")
		sb.WriteString(strings.Repeat("-", g.width-4))
		sb.WriteString("\n")
	}

	// Draw lease slots (maxLeases down to 1)
	for leaseSlot := maxLeases; leaseSlot >= 1; leaseSlot-- {
		// Y-axis label
		sb.WriteString(fmt.Sprintf("%3d |", leaseSlot))

		// Plot data points across time
		for x := 0; x < len(timePoints) && x < g.width-6; x++ {
			pointIndex := int(float64(x) / float64(g.width-6) * float64(len(timePoints)-1))
			if pointIndex >= len(enhancedPoints) {
				pointIndex = len(enhancedPoints) - 1
			}

			ep := enhancedPoints[pointIndex]

			if ep.ActiveLeases >= leaseSlot {
				// This lease slot is active
				sb.WriteString("█")
			} else {
				// This lease slot is free
				sb.WriteString(" ")
			}
		}
		sb.WriteString("\n")
	}

	// X-axis
	sb.WriteString("    +")
	sb.WriteString(strings.Repeat("-", g.width-6))
	sb.WriteString("\n")

	// X-axis labels - show marks every 24 hours
	if len(timePoints) > 0 {
		startTime := timePoints[0].Time
		endTime := timePoints[len(timePoints)-1].Time
		totalDuration := endTime.Sub(startTime)
		chartWidth := g.width - 6

		// Build the label line with day markers
		labelLine := make([]rune, chartWidth)
		for i := range labelLine {
			labelLine[i] = ' '
		}

		// Place markers every 24 hours
		day := 0
		for {
			dayDuration := time.Duration(day) * 24 * time.Hour
			if dayDuration > totalDuration {
				break
			}

			// Calculate position in chart
			position := 0
			if totalDuration > 0 {
				position = int(float64(dayDuration) / float64(totalDuration) * float64(chartWidth))
			}

			// Format day marker
			marker := fmt.Sprintf("%dd", day)

			// Place marker if it fits
			if position+len(marker) <= chartWidth {
				for i, ch := range marker {
					if position+i < chartWidth {
						labelLine[position+i] = ch
					}
				}
			}

			day++
		}

		sb.WriteString("    ")
		sb.WriteString(string(labelLine))
		sb.WriteString("\n")
	}

	// Legend
	sb.WriteString("\n")
	sb.WriteString("Legend:\n")
	sb.WriteString(fmt.Sprintf("  Lease slots (1-%d):\n", maxLeases))
	sb.WriteString("    █ - Active lease\n")
	sb.WriteString("    (space) - Free lease\n")
	if maxWaitingAndTimeout > 0 {
		sb.WriteString(fmt.Sprintf("  Waiting/Timeout rows (>%d):\n", maxLeases))
		sb.WriteString("    * - Job waiting for lease\n")
		sb.WriteString("    ! - Job timed out waiting for lease\n")
	}
	sb.WriteString("\n")

	return sb.String()
}

// GenerateEventSummary generates a summary of events
func (g *Generator) GenerateEventSummary(events []simulation.Event) string {
	var sb strings.Builder

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
	sb.WriteString(fmt.Sprintf("  - Job Timeouts: %d\n", eventsByType[simulation.EventTypeJobTimeout]))
	sb.WriteString(fmt.Sprintf("  - Max Exceeded: %d\n", eventsByType[simulation.EventTypeMaxExceeded]))
	sb.WriteString("\n")

	return sb.String()
}

// GenerateWarnings generates a list of warnings
func (g *Generator) GenerateWarnings(warnings []simulation.Event) string {
	var sb strings.Builder

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

	sb.WriteString("\n")
	sb.WriteString("Detailed Timeline")
	if limit > 0 && limit < len(events) {
		sb.WriteString(fmt.Sprintf(" (showing first %d events)", limit))
	}
	sb.WriteString("\n")
	sb.WriteString(strings.Repeat("=", g.width))
	sb.WriteString("\n\n")

	displayCount := len(events)
	if limit > 0 && limit < displayCount {
		displayCount = limit
	}

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
			typeIcon = "T"
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

// FormatDuration formats a duration in a human-readable way
func FormatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
}
