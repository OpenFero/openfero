package main

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// BenchmarkResult holds parsed benchmark data
type BenchmarkResult struct {
	Name        string  `json:"name"`
	Iterations  int     `json:"iterations"`
	NsPerOp     float64 `json:"ns_per_op"`
	BytesPerOp  int     `json:"bytes_per_op"`
	AllocsPerOp int     `json:"allocs_per_op"`
}

// AggregatedResult holds the averaged results from multiple runs
type AggregatedResult struct {
	Name          string  `json:"name"`
	NsPerOp       float64 `json:"ns_per_op"`
	NsPerOpStdDev float64 `json:"ns_per_op_stddev"`
	BytesPerOp    float64 `json:"bytes_per_op"`
	AllocsPerOp   float64 `json:"allocs_per_op"`
	SampleCount   int     `json:"sample_count"`
}

// ComparisonEntry holds the comparison between old and new versions
type ComparisonEntry struct {
	Name           string  `json:"name"`
	Category       string  `json:"category"`
	OldNsPerOp     float64 `json:"old_ns_per_op"`
	NewNsPerOp     float64 `json:"new_ns_per_op"`
	NsPerOpChange  float64 `json:"ns_per_op_change_pct"`
	OldBytesPerOp  float64 `json:"old_bytes_per_op"`
	NewBytesPerOp  float64 `json:"new_bytes_per_op"`
	BytesChange    float64 `json:"bytes_change_pct"`
	OldAllocsPerOp float64 `json:"old_allocs_per_op"`
	NewAllocsPerOp float64 `json:"new_allocs_per_op"`
	AllocsChange   float64 `json:"allocs_change_pct"`
	Winner         string  `json:"winner"`
}

// Report holds the full comparison report
type Report struct {
	OldLabel string            `json:"old_label"`
	NewLabel string            `json:"new_label"`
	Entries  []ComparisonEntry `json:"entries"`
	Summary  ReportSummary     `json:"summary"`
}

// ReportSummary holds aggregate statistics
type ReportSummary struct {
	TotalBenchmarks     int     `json:"total_benchmarks"`
	OldWins             int     `json:"old_wins"`
	NewWins             int     `json:"new_wins"`
	Ties                int     `json:"ties"`
	AvgNsPerOpChange    float64 `json:"avg_ns_per_op_change_pct"`
	AvgBytesChange      float64 `json:"avg_bytes_change_pct"`
	AvgAllocsChange     float64 `json:"avg_allocs_change_pct"`
	MaxImprovement      float64 `json:"max_improvement_pct"`
	MaxImprovementBench string  `json:"max_improvement_bench"`
	MaxRegression       float64 `json:"max_regression_pct"`
	MaxRegressionBench  string  `json:"max_regression_bench"`
}

// parseBenchmarkFile parses a Go benchmark output file
func parseBenchmarkFile(filename string) (map[string][]BenchmarkResult, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open %s: %w", filename, err)
	}
	defer file.Close()

	// Regex for benchmark lines:
	// BenchmarkName-8   123456   9.87 ns/op   64 B/op   2 allocs/op
	benchRegex := regexp.MustCompile(
		`^(Benchmark\S+?)(?:-\d+)?\s+(\d+)\s+(\d+(?:\.\d+)?)\s+ns/op` +
			`(?:\s+(\d+)\s+B/op)?` +
			`(?:\s+(\d+)\s+allocs/op)?`,
	)

	results := make(map[string][]BenchmarkResult)
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		matches := benchRegex.FindStringSubmatch(line)
		if matches == nil {
			continue
		}

		name := matches[1]
		iterations, _ := strconv.Atoi(matches[2])
		nsPerOp, _ := strconv.ParseFloat(matches[3], 64)

		var bytesPerOp, allocsPerOp int
		if matches[4] != "" {
			bytesPerOp, _ = strconv.Atoi(matches[4])
		}
		if matches[5] != "" {
			allocsPerOp, _ = strconv.Atoi(matches[5])
		}

		results[name] = append(results[name], BenchmarkResult{
			Name:        name,
			Iterations:  iterations,
			NsPerOp:     nsPerOp,
			BytesPerOp:  bytesPerOp,
			AllocsPerOp: allocsPerOp,
		})
	}

	return results, scanner.Err()
}

// aggregateResults averages multiple runs of the same benchmark
func aggregateResults(results map[string][]BenchmarkResult) map[string]AggregatedResult {
	aggregated := make(map[string]AggregatedResult)

	for name, runs := range results {
		if len(runs) == 0 {
			continue
		}

		var totalNs, totalBytes, totalAllocs float64
		for _, r := range runs {
			totalNs += r.NsPerOp
			totalBytes += float64(r.BytesPerOp)
			totalAllocs += float64(r.AllocsPerOp)
		}

		n := float64(len(runs))
		meanNs := totalNs / n

		// Calculate stddev for ns/op
		var sumSquaredDiff float64
		for _, r := range runs {
			diff := r.NsPerOp - meanNs
			sumSquaredDiff += diff * diff
		}
		stddev := math.Sqrt(sumSquaredDiff / n)

		aggregated[name] = AggregatedResult{
			Name:          name,
			NsPerOp:       meanNs,
			NsPerOpStdDev: stddev,
			BytesPerOp:    totalBytes / n,
			AllocsPerOp:   totalAllocs / n,
			SampleCount:   len(runs),
		}
	}

	return aggregated
}

// categorize determines the category of a benchmark based on its name
func categorize(name string) string {
	switch {
	case strings.Contains(name, "MatchesHookMessage"):
		return "Alert Matching"
	case strings.Contains(name, "FindMatchingOperarius"):
		return "Operarius Lookup"
	case strings.Contains(name, "ProcessTemplate"):
		return "Template Processing"
	case strings.Contains(name, "CreateJobFromOperarius"):
		return "Job Creation"
	case strings.Contains(name, "CheckDeduplication"):
		return "Deduplication"
	case strings.Contains(name, "ToJobInfo"):
		return "Data Conversion"
	case strings.Contains(name, "FullAlertProcessing"):
		return "Full Pipeline"
	case strings.Contains(name, "SaveAlert"):
		return "Alert Storage"
	case strings.Contains(name, "GetAlerts"):
		return "Alert Retrieval"
	case strings.Contains(name, "AlertMatchesQuery"):
		return "Alert Search"
	case strings.Contains(name, "MemoryStore"):
		return "Memory Store"
	case strings.Contains(name, "CheckAlertStatus"):
		return "Status Check"
	default:
		return "Other"
	}
}

// pctChange calculates percentage change from old to new
// Negative means improvement (new is faster/smaller)
func pctChange(old, new float64) float64 {
	if old == 0 {
		return 0
	}
	return ((new - old) / old) * 100
}

// compare generates comparison entries between old and new results
func compare(oldResults, newResults map[string]AggregatedResult, oldLabel, newLabel string) Report {
	var entries []ComparisonEntry

	// Find common benchmarks
	for name, oldResult := range oldResults {
		newResult, exists := newResults[name]
		if !exists {
			continue
		}

		nsChange := pctChange(oldResult.NsPerOp, newResult.NsPerOp)
		bytesChange := pctChange(oldResult.BytesPerOp, newResult.BytesPerOp)
		allocsChange := pctChange(oldResult.AllocsPerOp, newResult.AllocsPerOp)

		var winner string
		switch {
		case nsChange < -1.0:
			winner = newLabel
		case nsChange > 1.0:
			winner = oldLabel
		default:
			winner = "tie"
		}

		entries = append(entries, ComparisonEntry{
			Name:           name,
			Category:       categorize(name),
			OldNsPerOp:     oldResult.NsPerOp,
			NewNsPerOp:     newResult.NsPerOp,
			NsPerOpChange:  nsChange,
			OldBytesPerOp:  oldResult.BytesPerOp,
			NewBytesPerOp:  newResult.BytesPerOp,
			BytesChange:    bytesChange,
			OldAllocsPerOp: oldResult.AllocsPerOp,
			NewAllocsPerOp: newResult.AllocsPerOp,
			AllocsChange:   allocsChange,
			Winner:         winner,
		})
	}

	// Sort by impact (largest improvement first)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].NsPerOpChange < entries[j].NsPerOpChange
	})

	// Calculate summary
	summary := calculateSummary(entries, oldLabel, newLabel)

	return Report{
		OldLabel: oldLabel,
		NewLabel: newLabel,
		Entries:  entries,
		Summary:  summary,
	}
}

func calculateSummary(entries []ComparisonEntry, oldLabel, newLabel string) ReportSummary {
	summary := ReportSummary{
		TotalBenchmarks: len(entries),
	}

	var totalNsChange, totalBytesChange, totalAllocsChange float64
	for _, e := range entries {
		totalNsChange += e.NsPerOpChange
		totalBytesChange += e.BytesChange
		totalAllocsChange += e.AllocsChange

		switch e.Winner {
		case oldLabel:
			summary.OldWins++
		case newLabel:
			summary.NewWins++
		default:
			summary.Ties++
		}

		// Track extremes (improvement = negative change)
		if e.NsPerOpChange < summary.MaxImprovement || summary.MaxImprovementBench == "" {
			summary.MaxImprovement = e.NsPerOpChange
			summary.MaxImprovementBench = e.Name
		}
		if e.NsPerOpChange > summary.MaxRegression || summary.MaxRegressionBench == "" {
			summary.MaxRegression = e.NsPerOpChange
			summary.MaxRegressionBench = e.Name
		}
	}

	n := float64(len(entries))
	if n > 0 {
		summary.AvgNsPerOpChange = totalNsChange / n
		summary.AvgBytesChange = totalBytesChange / n
		summary.AvgAllocsChange = totalAllocsChange / n
	}

	return summary
}

// writeJSON writes the report as JSON
func writeJSON(report Report, outputDir string) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outputDir, "comparison.json"), data, 0644)
}

// writeCSV writes the comparison as CSV
func writeCSV(report Report, outputDir string) error {
	file, err := os.Create(filepath.Join(outputDir, "comparison.csv"))
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Header
	_ = writer.Write([]string{
		"Benchmark", "Category",
		report.OldLabel + " ns/op", report.NewLabel + " ns/op", "ns/op Change %",
		report.OldLabel + " B/op", report.NewLabel + " B/op", "B/op Change %",
		report.OldLabel + " allocs/op", report.NewLabel + " allocs/op", "allocs/op Change %",
		"Winner",
	})

	for _, e := range report.Entries {
		_ = writer.Write([]string{
			e.Name, e.Category,
			fmt.Sprintf("%.2f", e.OldNsPerOp), fmt.Sprintf("%.2f", e.NewNsPerOp), fmt.Sprintf("%.2f", e.NsPerOpChange),
			fmt.Sprintf("%.0f", e.OldBytesPerOp), fmt.Sprintf("%.0f", e.NewBytesPerOp), fmt.Sprintf("%.2f", e.BytesChange),
			fmt.Sprintf("%.0f", e.OldAllocsPerOp), fmt.Sprintf("%.0f", e.NewAllocsPerOp), fmt.Sprintf("%.2f", e.AllocsChange),
			e.Winner,
		})
	}

	return nil
}

// formatNs formats nanoseconds into a human-readable string
func formatNs(ns float64) string {
	switch {
	case ns >= 1e9:
		return fmt.Sprintf("%.2fs", ns/1e9)
	case ns >= 1e6:
		return fmt.Sprintf("%.2fms", ns/1e6)
	case ns >= 1e3:
		return fmt.Sprintf("%.2fus", ns/1e3)
	default:
		return fmt.Sprintf("%.2fns", ns)
	}
}

// changeIndicator returns a visual indicator for the change direction
func changeIndicator(pct float64) string {
	switch {
	case pct < -10:
		return "<<< (major improvement)"
	case pct < -5:
		return "<< (good improvement)"
	case pct < -1:
		return "< (minor improvement)"
	case pct > 10:
		return ">>> (major regression)"
	case pct > 5:
		return ">> (notable regression)"
	case pct > 1:
		return "> (minor regression)"
	default:
		return "~ (no significant change)"
	}
}

// writeMarkdown writes the report as a Markdown document
func writeMarkdown(report Report, outputDir string) error {
	file, err := os.Create(filepath.Join(outputDir, "REPORT.md"))
	if err != nil {
		return err
	}
	defer file.Close()

	w := bufio.NewWriter(file)
	defer w.Flush()

	fmt.Fprintf(w, "# Go Benchmark Comparison: %s vs %s\n\n", report.OldLabel, report.NewLabel)

	// Summary
	s := report.Summary
	fmt.Fprintln(w, "## Summary\n")
	fmt.Fprintf(w, "| Metric | Value |\n")
	fmt.Fprintf(w, "|--------|-------|\n")
	fmt.Fprintf(w, "| Total benchmarks | %d |\n", s.TotalBenchmarks)
	fmt.Fprintf(w, "| %s wins | %d |\n", report.NewLabel, s.NewWins)
	fmt.Fprintf(w, "| %s wins | %d |\n", report.OldLabel, s.OldWins)
	fmt.Fprintf(w, "| Ties (< 1%% diff) | %d |\n", s.Ties)
	fmt.Fprintf(w, "| Avg ns/op change | %.2f%% |\n", s.AvgNsPerOpChange)
	fmt.Fprintf(w, "| Avg B/op change | %.2f%% |\n", s.AvgBytesChange)
	fmt.Fprintf(w, "| Avg allocs/op change | %.2f%% |\n", s.AvgAllocsChange)
	fmt.Fprintln(w)

	if s.MaxImprovement < -1 {
		fmt.Fprintf(w, "**Best improvement**: `%s` (%.2f%%)\n\n", s.MaxImprovementBench, s.MaxImprovement)
	}
	if s.MaxRegression > 1 {
		fmt.Fprintf(w, "**Largest regression**: `%s` (+%.2f%%)\n\n", s.MaxRegressionBench, s.MaxRegression)
	}

	// Interpretation
	fmt.Fprintln(w, "## Interpretation\n")
	switch {
	case s.AvgNsPerOpChange < -5:
		fmt.Fprintf(w, "%s shows **significant performance improvements** over %s ", report.NewLabel, report.OldLabel)
		fmt.Fprintf(w, "with an average of %.1f%% faster execution across all benchmarks.\n\n", math.Abs(s.AvgNsPerOpChange))
	case s.AvgNsPerOpChange < -1:
		fmt.Fprintf(w, "%s shows **moderate performance improvements** over %s ", report.NewLabel, report.OldLabel)
		fmt.Fprintf(w, "with an average of %.1f%% faster execution.\n\n", math.Abs(s.AvgNsPerOpChange))
	case s.AvgNsPerOpChange > 5:
		fmt.Fprintf(w, "%s shows **regressions** compared to %s ", report.NewLabel, report.OldLabel)
		fmt.Fprintf(w, "with an average of %.1f%% slower execution. Investigate before upgrading.\n\n", s.AvgNsPerOpChange)
	default:
		fmt.Fprintf(w, "Performance between %s and %s is **comparable** (%.1f%% average change).\n\n", report.OldLabel, report.NewLabel, s.AvgNsPerOpChange)
	}

	// GC impact analysis
	fmt.Fprintln(w, "### GC / Allocation Impact\n")
	if s.AvgBytesChange < -1 || s.AvgAllocsChange < -1 {
		fmt.Fprintln(w, "The new Go version shows **reduced allocations**, which likely benefits from the improved garbage collector.")
		fmt.Fprintf(w, "Average B/op change: %.2f%%, Average allocs/op change: %.2f%%\n\n", s.AvgBytesChange, s.AvgAllocsChange)
	} else if s.AvgBytesChange > 1 || s.AvgAllocsChange > 1 {
		fmt.Fprintln(w, "Allocations have **increased** slightly. The GC improvements may still compensate at runtime.")
		fmt.Fprintf(w, "Average B/op change: +%.2f%%, Average allocs/op change: +%.2f%%\n\n", s.AvgBytesChange, s.AvgAllocsChange)
	} else {
		fmt.Fprintln(w, "Allocation patterns are **unchanged** between versions. GC improvements will benefit throughput and latency under load.\n")
	}

	// Detailed results by category
	fmt.Fprintln(w, "## Detailed Results\n")

	// Group by category
	categories := make(map[string][]ComparisonEntry)
	for _, e := range report.Entries {
		categories[e.Category] = append(categories[e.Category], e)
	}

	// Sort category names
	var catNames []string
	for name := range categories {
		catNames = append(catNames, name)
	}
	sort.Strings(catNames)

	for _, cat := range catNames {
		entries := categories[cat]
		fmt.Fprintf(w, "### %s\n\n", cat)

		fmt.Fprintf(w, "| Benchmark | %s | %s | Change | Indicator |\n", report.OldLabel, report.NewLabel)
		fmt.Fprintln(w, "|-----------|---------|---------|--------|-----------|")

		for _, e := range entries {
			shortName := strings.TrimPrefix(e.Name, "Benchmark")
			fmt.Fprintf(w, "| `%s` | %s | %s | %.2f%% | %s |\n",
				shortName,
				formatNs(e.OldNsPerOp),
				formatNs(e.NewNsPerOp),
				e.NsPerOpChange,
				changeIndicator(e.NsPerOpChange),
			)
		}
		fmt.Fprintln(w)

		// Allocation table for this category
		fmt.Fprintf(w, "**Allocations (%s):**\n\n", cat)
		fmt.Fprintf(w, "| Benchmark | %s B/op | %s B/op | Change | %s allocs | %s allocs | Change |\n",
			report.OldLabel, report.NewLabel, report.OldLabel, report.NewLabel)
		fmt.Fprintln(w, "|-----------|---------|---------|--------|-----------|-----------|--------|")

		for _, e := range entries {
			shortName := strings.TrimPrefix(e.Name, "Benchmark")
			fmt.Fprintf(w, "| `%s` | %.0f | %.0f | %.2f%% | %.0f | %.0f | %.2f%% |\n",
				shortName,
				e.OldBytesPerOp, e.NewBytesPerOp, e.BytesChange,
				e.OldAllocsPerOp, e.NewAllocsPerOp, e.AllocsChange,
			)
		}
		fmt.Fprintln(w)
	}

	// Recommendations
	fmt.Fprintln(w, "## Recommendations\n")

	if s.AvgNsPerOpChange < -1 {
		fmt.Fprintf(w, "1. **Upgrade recommended**: %s provides measurable performance gains\n", report.NewLabel)
	} else if s.AvgNsPerOpChange > 5 {
		fmt.Fprintf(w, "1. **Investigate regressions**: %s shows slower performance in several areas\n", report.NewLabel)
	} else {
		fmt.Fprintf(w, "1. **Upgrade safe**: Performance is comparable between versions\n")
	}

	fmt.Fprintln(w, "2. Run benchmarks under production-like load to validate GC improvements")
	fmt.Fprintln(w, "3. Monitor memory usage and GC pauses in production after upgrade")
	fmt.Fprintln(w, "4. Consider adding profiling (`-cpuprofile`, `-memprofile`) for deeper analysis")
	fmt.Fprintln(w)

	// How to reproduce
	fmt.Fprintln(w, "## Reproduction\n")
	fmt.Fprintln(w, "```bash")
	fmt.Fprintln(w, "# Run benchmarks locally")
	fmt.Fprintf(w, "make benchmark\n\n")
	fmt.Fprintln(w, "# Run in Docker (isolated)")
	fmt.Fprintf(w, "make benchmark-docker\n\n")
	fmt.Fprintln(w, "# Run with custom settings")
	fmt.Fprintf(w, "BENCH_COUNT=10 BENCH_TIME=10s make benchmark-docker\n")
	fmt.Fprintln(w, "```")

	return nil
}

func main() {
	oldFile := flag.String("old", "", "Path to old Go version benchmark results")
	newFile := flag.String("new", "", "Path to new Go version benchmark results")
	outputDir := flag.String("output", ".", "Output directory for reports")
	oldLabel := flag.String("old-label", "Old", "Label for old version")
	newLabel := flag.String("new-label", "New", "Label for new version")
	flag.Parse()

	if *oldFile == "" || *newFile == "" {
		fmt.Fprintln(os.Stderr, "Usage: analyze-benchmarks -old <file> -new <file> [-output <dir>]")
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Parse benchmark files
	fmt.Fprintf(os.Stderr, "Parsing %s...\n", *oldFile)
	oldResults, err := parseBenchmarkFile(*oldFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing old results: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "Parsing %s...\n", *newFile)
	newResults, err := parseBenchmarkFile(*newFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing new results: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "Found %d benchmark names in old, %d in new\n", len(oldResults), len(newResults))

	// Aggregate results
	oldAgg := aggregateResults(oldResults)
	newAgg := aggregateResults(newResults)

	// Generate comparison
	report := compare(oldAgg, newAgg, *oldLabel, *newLabel)

	// Write outputs
	if err := writeJSON(report, *outputDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing JSON: %v\n", err)
	} else {
		fmt.Fprintf(os.Stderr, "Written: %s\n", filepath.Join(*outputDir, "comparison.json"))
	}

	if err := writeCSV(report, *outputDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing CSV: %v\n", err)
	} else {
		fmt.Fprintf(os.Stderr, "Written: %s\n", filepath.Join(*outputDir, "comparison.csv"))
	}

	if err := writeMarkdown(report, *outputDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing Markdown: %v\n", err)
	} else {
		fmt.Fprintf(os.Stderr, "Written: %s\n", filepath.Join(*outputDir, "REPORT.md"))
	}

	// Print summary to stdout
	fmt.Println()
	fmt.Printf("=== %s vs %s ===\n", *oldLabel, *newLabel)
	fmt.Printf("Benchmarks compared: %d\n", report.Summary.TotalBenchmarks)
	fmt.Printf("%s wins: %d\n", *newLabel, report.Summary.NewWins)
	fmt.Printf("%s wins: %d\n", *oldLabel, report.Summary.OldWins)
	fmt.Printf("Ties:       %d\n", report.Summary.Ties)
	fmt.Printf("Avg ns/op change:    %+.2f%%\n", report.Summary.AvgNsPerOpChange)
	fmt.Printf("Avg B/op change:     %+.2f%%\n", report.Summary.AvgBytesChange)
	fmt.Printf("Avg allocs/op change: %+.2f%%\n", report.Summary.AvgAllocsChange)
	fmt.Println()
}
