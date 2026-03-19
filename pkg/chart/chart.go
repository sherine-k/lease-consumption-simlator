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
	// Pre-allocate buffer to reduce allocations (rough estimate: 100 chars per row * totalRows)
	estimatedSize := (maxLeases + 10) * 100
	sb.Grow(estimatedSize)

	// Header
	sb.WriteString("\n")
	sb.WriteString("Lease Usage Over Time\n")
	sb.WriteString(strings.Repeat("=", g.width))
	sb.WriteString("\n\n")

	// Build enhanced time points with timeout information
	type EnhancedTimePoint struct {
		ActiveLeases  int
		WaitingJobs   int
		HasTimeout    bool
	}

	enhancedPoints := make([]EnhancedTimePoint, len(timePoints))

	for i, tp := range timePoints {
		hasTimeout := false

		// Check for timeout events within the window of this time point
		// Look ahead to next time point (or end of simulation)
		windowEnd := tp.Time.Add(15 * time.Minute)
		if i+1 < len(timePoints) {
			windowEnd = timePoints[i+1].Time
		}

		for _, event := range events {
			if event.Type == simulation.EventTypeJobTimeout &&
				(event.Time.Equal(tp.Time) || (event.Time.After(tp.Time) && event.Time.Before(windowEnd))) {
				hasTimeout = true
				break
			}
		}

		enhancedPoints[i] = EnhancedTimePoint{
			ActiveLeases: tp.ActiveLeases,
			WaitingJobs:  tp.WaitingJobs,
			HasTimeout:   hasTimeout,
		}
	}

	// Find max waiting jobs to determine chart height
	// Check what will actually be visible when plotted at chart width
	maxWaiting := 0
	hasAnyWaiting := false

	for x := 0; x < len(timePoints) && x < g.width-6; x++ {
		pointIndex := int(float64(x) / float64(g.width-6) * float64(len(timePoints)-1))
		if pointIndex >= len(enhancedPoints) {
			pointIndex = len(enhancedPoints) - 1
		}

		ep := enhancedPoints[pointIndex]
		if ep.WaitingJobs > maxWaiting {
			maxWaiting = ep.WaitingJobs
		}
		if ep.WaitingJobs > 0 {
			hasAnyWaiting = true
		}
	}

	// Only include waiting rows if there are actually waiting jobs visible
	totalRows := maxLeases
	if hasAnyWaiting {
		totalRows = maxLeases + maxWaiting
	}

	// Build the chart from top to bottom
	// Only draw waiting rows if there are actually waiting jobs to show
	if hasAnyWaiting {
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

			if waitingRow <= ep.WaitingJobs {
				// Show waiting
				sb.WriteString("W")
			} else {
				sb.WriteString(" ")
			}
			}
			sb.WriteString("\n")
		}

		// Separator line between waiting and lease slots
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
	if maxWaiting > 0 {
		sb.WriteString(fmt.Sprintf("  Waiting rows (>%d):\n", maxLeases))
		sb.WriteString("    W - Job waiting for lease\n")
	}
	sb.WriteString("\n")

	return sb.String()
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
	sb.WriteString(fmt.Sprintf("  - Job Timeouts: %d\n", eventsByType[simulation.EventTypeJobTimeout]))
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
