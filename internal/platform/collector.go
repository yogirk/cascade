package platform

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// SignalCollector collects signals from a single GCP service.
type SignalCollector interface {
	// Source returns the signal source identifier.
	Source() SignalSource
	// Collect gathers signals from the service within the given time window.
	// Returns signals found and any error. A nil error with empty signals is valid ("all clear").
	Collect(ctx context.Context, since time.Duration) ([]Signal, error)
}

// PlatformCollector orchestrates concurrent signal collection from multiple sources.
type PlatformCollector struct {
	collectors []SignalCollector
}

// NewPlatformCollector creates a collector with the given signal sources.
func NewPlatformCollector(collectors ...SignalCollector) *PlatformCollector {
	return &PlatformCollector{collectors: collectors}
}

// Collect runs all signal collectors concurrently and returns a MorningReport.
// Each collector runs in its own goroutine with recover() to prevent panics
// from crashing the entire collection.
func (pc *PlatformCollector) Collect(ctx context.Context, since time.Duration) *MorningReport {
	results := make([]SourceResult, len(pc.collectors))
	var wg sync.WaitGroup

	for i, c := range pc.collectors {
		wg.Add(1)
		go func(idx int, collector SignalCollector) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					results[idx] = SourceResult{
						Source: collector.Source(),
						Err:    fmt.Errorf("collector panic: %v", r),
						Note:   fmt.Sprintf("%s: internal error (recovered from panic)", collector.Source()),
					}
				}
			}()

			signals, err := collector.Collect(ctx, since)
			if err != nil {
				results[idx] = SourceResult{
					Source: collector.Source(),
					Err:    err,
					Note:   fmt.Sprintf("%s: %v", collector.Source(), err),
				}
				return
			}
			results[idx] = SourceResult{
				Source:  collector.Source(),
				Signals: signals,
			}
		}(i, c)
	}

	wg.Wait()

	// Aggregate results
	var allSignals []Signal
	var notes []string
	for _, r := range results {
		if r.Note != "" {
			notes = append(notes, r.Note)
		}
		allSignals = append(allSignals, r.Signals...)
	}

	// Check if all sources failed
	if len(allSignals) == 0 && len(notes) == len(pc.collectors) {
		allFailed := true
		for _, r := range results {
			if r.Err == nil {
				allFailed = false
				break
			}
		}
		if allFailed {
			notes = append(notes, "No platform sources available. Check GCP credentials.")
		}
	}

	incidents := Correlate(allSignals)

	return &MorningReport{
		Incidents:   incidents,
		Signals:     allSignals,
		SourceNotes: notes,
		Since:       since,
		CollectedAt: time.Now(),
	}
}
