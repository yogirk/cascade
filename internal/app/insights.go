package app

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"charm.land/lipgloss/v2"

	bq "github.com/yogirk/cascade/internal/bigquery"
	bqtools "github.com/yogirk/cascade/internal/tools/bigquery"
)

// Styles for insights report rendering.
var (
	insightHeaderStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B9FFF")).Bold(true)
	insightTitleStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#F3F4F6")).Bold(true)
	insightDimStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#4B5563"))
	insightTextStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#D1D5DB"))
	insightLabelStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#9CA3AF"))
	insightValueStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#F3F4F6")).Bold(true)
	insightAccentStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#38BDF8"))
	insightSepStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#374151"))
	insightWarnStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#D97706"))
)

// InsightsReport holds the results of cost analysis queries.
type InsightsReport struct {
	ProjectID string
	Location  string

	// Query costs (last 7 days)
	DailyCosts     []float64 // 7 values, oldest first
	DailyLabels    []string  // date labels
	TotalQueryCost float64
	QueryCostErr   error

	// Top expensive queries (today)
	TopQueries   []TopQueryRow
	TopQueryErr  error

	// Storage summary
	StorageSummary *StorageSummaryData
	StorageErr     error

	// Slot utilization (may be nil if on-demand)
	SlotUtilization []float64 // 7 daily averages
	SlotErr         error
}

// TopQueryRow represents one expensive query.
type TopQueryRow struct {
	UserEmail  string
	SQLPreview string
	CostUSD    float64
	BytesBilled int64
}

// StorageSummaryData holds storage cost breakdown.
type StorageSummaryData struct {
	ActiveBytes   float64
	LongTermBytes float64
	TotalBytes    float64
	ActiveCost    float64   // monthly estimate
	LongTermCost  float64   // monthly estimate
	TotalCost     float64   // monthly estimate
	TopTables     []bqtools.BarChartItem // largest tables
}

// RunInsights executes cost analysis queries and returns a report.
// Queries run in parallel; individual failures are captured per-section.
func RunInsights(ctx context.Context, bqComp *BigQueryComponents, location string) *InsightsReport {
	if bqComp == nil || bqComp.Client == nil {
		return &InsightsReport{QueryCostErr: fmt.Errorf("BigQuery not configured")}
	}

	report := &InsightsReport{
		ProjectID: bqComp.Client.ProjectID(),
		Location:  location,
	}

	var wg sync.WaitGroup
	wg.Add(3) // query costs, top queries + storage run together, slots

	// 1. Daily query cost trend (last 7 days)
	go func() {
		defer wg.Done()
		report.DailyCosts, report.DailyLabels, report.TotalQueryCost, report.QueryCostErr = queryDailyCosts(ctx, bqComp.Client, location)
	}()

	// 2. Top expensive queries today + storage summary
	go func() {
		defer wg.Done()
		report.TopQueries, report.TopQueryErr = queryTopQueries(ctx, bqComp.Client, location)
		report.StorageSummary, report.StorageErr = queryStorageSummary(ctx, bqComp.Client, location)
	}()

	// 3. Slot utilization (graceful failure if on-demand)
	go func() {
		defer wg.Done()
		report.SlotUtilization, report.SlotErr = querySlotUtilization(ctx, bqComp.Client, location)
	}()

	wg.Wait()
	return report
}

func queryDailyCosts(ctx context.Context, client *bq.Client, location string) (costs []float64, labels []string, total float64, err error) {
	sql := fmt.Sprintf(`
		SELECT
			DATE(creation_time) AS day,
			SUM(total_bytes_billed) / POW(1024, 4) * 6.25 AS cost_usd
		FROM `+"`region-%s`.INFORMATION_SCHEMA.JOBS_BY_PROJECT"+`
		WHERE creation_time >= TIMESTAMP_SUB(CURRENT_TIMESTAMP(), INTERVAL 7 DAY)
			AND job_type = 'QUERY'
			AND state = 'DONE'
			AND statement_type != 'SCRIPT'
		GROUP BY day
		ORDER BY day
	`, location)

	headers, rows, _, _, execErr := client.ExecuteQuery(ctx, sql, 7)
	if execErr != nil {
		return nil, nil, 0, execErr
	}

	_ = headers
	for _, row := range rows {
		if len(row) >= 2 {
			labels = append(labels, row[0])
			var cost float64
			fmt.Sscanf(row[1], "%f", &cost)
			costs = append(costs, cost)
			total += cost
		}
	}
	return costs, labels, total, nil
}

func queryTopQueries(ctx context.Context, client *bq.Client, location string) ([]TopQueryRow, error) {
	sql := fmt.Sprintf(`
		SELECT
			user_email,
			SUBSTR(query, 1, 60) AS sql_preview,
			total_bytes_billed / POW(1024, 4) * 6.25 AS cost_usd,
			total_bytes_billed
		FROM `+"`region-%s`.INFORMATION_SCHEMA.JOBS_BY_PROJECT"+`
		WHERE creation_time >= TIMESTAMP_TRUNC(CURRENT_TIMESTAMP(), DAY)
			AND job_type = 'QUERY'
			AND state = 'DONE'
			AND statement_type != 'SCRIPT'
			AND total_bytes_billed > 0
		ORDER BY total_bytes_billed DESC
		LIMIT 5
	`, location)

	_, rows, _, _, err := client.ExecuteQuery(ctx, sql, 5)
	if err != nil {
		return nil, err
	}

	var result []TopQueryRow
	for _, row := range rows {
		if len(row) >= 4 {
			var cost float64
			var bytes int64
			fmt.Sscanf(row[2], "%f", &cost)
			fmt.Sscanf(row[3], "%d", &bytes)
			result = append(result, TopQueryRow{
				UserEmail:   row[0],
				SQLPreview:  row[1],
				CostUSD:     cost,
				BytesBilled: bytes,
			})
		}
	}
	return result, nil
}

func queryStorageSummary(ctx context.Context, client *bq.Client, location string) (*StorageSummaryData, error) {
	sql := fmt.Sprintf(`
		SELECT
			SUM(active_logical_bytes) AS active_bytes,
			SUM(long_term_logical_bytes) AS long_term_bytes,
			SUM(total_logical_bytes) AS total_bytes
		FROM `+"`region-%s`.INFORMATION_SCHEMA.TABLE_STORAGE"+`
	`, location)

	_, rows, _, _, err := client.ExecuteQuery(ctx, sql, 1)
	if err != nil {
		return nil, err
	}

	if len(rows) == 0 || len(rows[0]) < 3 {
		return nil, fmt.Errorf("no storage data returned")
	}

	var active, longTerm, total float64
	fmt.Sscanf(rows[0][0], "%f", &active)
	fmt.Sscanf(rows[0][1], "%f", &longTerm)
	fmt.Sscanf(rows[0][2], "%f", &total)

	// Monthly cost estimates
	activeCost := active / (1024 * 1024 * 1024) * 0.02
	longTermCost := longTerm / (1024 * 1024 * 1024) * 0.01

	// Top tables by size
	topSQL := fmt.Sprintf(`
		SELECT
			CONCAT(table_schema, '.', table_name) AS table_path,
			total_logical_bytes
		FROM `+"`region-%s`.INFORMATION_SCHEMA.TABLE_STORAGE"+`
		ORDER BY total_logical_bytes DESC
		LIMIT 5
	`, location)

	var topTables []bqtools.BarChartItem
	_, topRows, _, _, topErr := client.ExecuteQuery(ctx, topSQL, 5)
	if topErr == nil {
		for _, row := range topRows {
			if len(row) >= 2 {
				var bytes float64
				fmt.Sscanf(row[1], "%f", &bytes)
				topTables = append(topTables, bqtools.BarChartItem{
					Label:          row[0],
					Value:          bytes,
					FormattedValue: bqtools.FormatGB(bytes),
				})
			}
		}
	}

	return &StorageSummaryData{
		ActiveBytes:   active,
		LongTermBytes: longTerm,
		TotalBytes:    total,
		ActiveCost:    activeCost,
		LongTermCost:  longTermCost,
		TotalCost:     activeCost + longTermCost,
		TopTables:     topTables,
	}, nil
}

func querySlotUtilization(ctx context.Context, client *bq.Client, location string) ([]float64, error) {
	sql := fmt.Sprintf(`
		SELECT
			DATE(period_start) AS day,
			AVG(baseline_slots + COALESCE(autoscale.current_slots, 0)) AS avg_slots
		FROM `+"`region-%s`.INFORMATION_SCHEMA.RESERVATIONS_TIMELINE"+`
		WHERE period_start >= TIMESTAMP_SUB(CURRENT_TIMESTAMP(), INTERVAL 7 DAY)
		GROUP BY day
		ORDER BY day
	`, location)

	_, rows, _, _, err := client.ExecuteQuery(ctx, sql, 7)
	if err != nil {
		return nil, err // Expected to fail for on-demand projects
	}

	var utilization []float64
	for _, row := range rows {
		if len(row) >= 2 {
			var slots float64
			fmt.Sscanf(row[1], "%f", &slots)
			utilization = append(utilization, slots)
		}
	}
	return utilization, nil
}

// sectionSep renders a thin separator line.
func sectionSep(width int) string {
	if width <= 0 {
		width = 60
	}
	return "  " + insightSepStyle.Render(strings.Repeat("─", width))
}

// RenderInsightsReport formats the insights report as a styled string for the TUI.
func RenderInsightsReport(report *InsightsReport) (display string, content string) {
	var db, cb strings.Builder

	// Title
	db.WriteString("\n")
	db.WriteString("  " + insightAccentStyle.Render("≋") + " " +
		insightTitleStyle.Render("BigQuery Cost Insights") + "  " +
		insightDimStyle.Render(report.ProjectID+" · "+report.Location) + "\n")
	db.WriteString(sectionSep(60) + "\n\n")
	cb.WriteString(fmt.Sprintf("BigQuery Cost Insights — %s (%s)\n\n", report.ProjectID, report.Location))

	// Section 1: Query Costs
	db.WriteString("  " + insightHeaderStyle.Render("Query Costs") + "  " +
		insightDimStyle.Render("last 7 days") + "\n\n")
	cb.WriteString("Query Costs (last 7 days)\n")
	if report.QueryCostErr != nil {
		db.WriteString("  " + insightWarnStyle.Render("⚠ "+report.QueryCostErr.Error()) + "\n")
		cb.WriteString("  unavailable: " + report.QueryCostErr.Error() + "\n")
	} else if len(report.DailyCosts) > 0 {
		spark := bqtools.RenderSparkline(report.DailyCosts)
		db.WriteString("  " + spark + "\n\n")

		// Stats row
		var total, peak, avg float64
		for _, v := range report.DailyCosts {
			total += v
			if v > peak {
				peak = v
			}
		}
		avg = total / float64(len(report.DailyCosts))

		db.WriteString("  " +
			insightLabelStyle.Render("Total ") + insightValueStyle.Render(bqtools.FormatDollars(total)) + "    " +
			insightLabelStyle.Render("Avg ") + insightTextStyle.Render(bqtools.FormatDollars(avg)+"/day") + "    " +
			insightLabelStyle.Render("Peak ") + insightTextStyle.Render(bqtools.FormatDollars(peak)) + "\n")
		cb.WriteString(fmt.Sprintf("  Total: %s  Avg: %s/day  Peak: %s\n",
			bqtools.FormatDollars(total), bqtools.FormatDollars(avg), bqtools.FormatDollars(peak)))
	} else {
		db.WriteString("  " + insightDimStyle.Render("No query costs in the last 7 days") + "\n")
		cb.WriteString("  No query costs in the last 7 days\n")
	}

	// Section 2: Top Queries
	db.WriteString("\n" + sectionSep(60) + "\n\n")
	db.WriteString("  " + insightHeaderStyle.Render("Top Queries") + "  " +
		insightDimStyle.Render("today by cost") + "\n\n")
	cb.WriteString("\nTop Expensive Queries (today)\n")
	if report.TopQueryErr != nil {
		db.WriteString("  " + insightWarnStyle.Render("⚠ "+report.TopQueryErr.Error()) + "\n")
		cb.WriteString("  unavailable: " + report.TopQueryErr.Error() + "\n")
	} else if len(report.TopQueries) > 0 {
		for i, q := range report.TopQueries {
			// User + cost on one line
			user := q.UserEmail
			if len(user) > 25 {
				user = user[:22] + "..."
			}
			db.WriteString("  " + insightTextStyle.Render(user) + "  " +
				insightValueStyle.Render(bqtools.FormatDollars(q.CostUSD)) + "\n")

			// SQL preview dimmed below
			preview := strings.ReplaceAll(q.SQLPreview, "\n", " ")
			if len(preview) > 55 {
				preview = preview[:52] + "..."
			}
			db.WriteString("  " + insightDimStyle.Render(preview) + "\n")

			if i < len(report.TopQueries)-1 {
				db.WriteString("\n")
			}

			cb.WriteString(fmt.Sprintf("  %s  %s  %s\n", q.UserEmail, bqtools.FormatDollars(q.CostUSD), q.SQLPreview))
		}
	} else {
		db.WriteString("  " + insightDimStyle.Render("No queries today") + "\n")
		cb.WriteString("  No queries today\n")
	}

	// Section 3: Storage
	db.WriteString("\n" + sectionSep(60) + "\n\n")
	db.WriteString("  " + insightHeaderStyle.Render("Storage") + "  " +
		insightDimStyle.Render("monthly estimate") + "\n\n")
	cb.WriteString("\nStorage Summary\n")
	if report.StorageErr != nil {
		db.WriteString("  " + insightWarnStyle.Render("⚠ "+report.StorageErr.Error()) + "\n")
		cb.WriteString("  unavailable: " + report.StorageErr.Error() + "\n")
	} else if report.StorageSummary != nil {
		s := report.StorageSummary

		// Storage breakdown
		db.WriteString("  " +
			insightLabelStyle.Render("Active      ") +
			insightValueStyle.Render(bqtools.FormatGB(s.ActiveBytes)) + "  " +
			insightDimStyle.Render(bqtools.FormatDollars(s.ActiveCost)+"/mo") + "\n")
		db.WriteString("  " +
			insightLabelStyle.Render("Long-term   ") +
			insightValueStyle.Render(bqtools.FormatGB(s.LongTermBytes)) + "  " +
			insightDimStyle.Render(bqtools.FormatDollars(s.LongTermCost)+"/mo") + "\n")
		db.WriteString("  " +
			insightLabelStyle.Render("Total       ") +
			insightTitleStyle.Render(bqtools.FormatGB(s.TotalBytes)) + "  " +
			insightAccentStyle.Render(bqtools.FormatDollars(s.TotalCost)+"/mo") + "\n")

		cb.WriteString(fmt.Sprintf("  Active: %s (%s/mo)\n", bqtools.FormatGB(s.ActiveBytes), bqtools.FormatDollars(s.ActiveCost)))
		cb.WriteString(fmt.Sprintf("  Long-term: %s (%s/mo)\n", bqtools.FormatGB(s.LongTermBytes), bqtools.FormatDollars(s.LongTermCost)))
		cb.WriteString(fmt.Sprintf("  Total: %s (%s/mo)\n", bqtools.FormatGB(s.TotalBytes), bqtools.FormatDollars(s.TotalCost)))

		if len(s.TopTables) > 0 {
			db.WriteString("\n  " + insightLabelStyle.Render("Largest tables") + "\n\n")
			db.WriteString(bqtools.RenderBarChart(s.TopTables, 25) + "\n")
			cb.WriteString("  Largest tables:\n")
			for _, t := range s.TopTables {
				cb.WriteString(fmt.Sprintf("    %s  %s\n", t.Label, t.FormattedValue))
			}
		}
	}

	// Section 4: Slot Utilization (optional)
	if report.SlotErr == nil && len(report.SlotUtilization) > 0 {
		db.WriteString("\n" + sectionSep(60) + "\n\n")
		db.WriteString("  " + insightHeaderStyle.Render("Slot Utilization") + "  " +
			insightDimStyle.Render("last 7 days") + "\n\n")
		db.WriteString("  " + bqtools.RenderSparkline(report.SlotUtilization) + "\n\n")

		var total, peak float64
		for _, v := range report.SlotUtilization {
			total += v
			if v > peak {
				peak = v
			}
		}
		avg := total / float64(len(report.SlotUtilization))
		db.WriteString("  " +
			insightLabelStyle.Render("Avg ") + insightTextStyle.Render(fmt.Sprintf("%.0f slots", avg)) + "    " +
			insightLabelStyle.Render("Peak ") + insightTextStyle.Render(fmt.Sprintf("%.0f slots", peak)) + "\n")

		cb.WriteString("\nSlot Utilization (last 7 days)\n")
		for i, v := range report.SlotUtilization {
			label := ""
			if i < len(report.DailyLabels) {
				label = report.DailyLabels[i]
			}
			cb.WriteString(fmt.Sprintf("  %s: %.0f slots\n", label, v))
		}
	}

	db.WriteString("\n")
	return db.String(), cb.String()
}
