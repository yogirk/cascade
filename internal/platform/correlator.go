package platform

import "sort"

// Correlate groups signals into incidents using union-find on shared resources.
// Signals that share at least one Related resource are grouped together.
// Signals with empty Related form their own singleton incident.
// Incidents are ranked by severity (critical first) → blast radius → recency.
func Correlate(signals []Signal) []Incident {
	if len(signals) == 0 {
		return nil
	}

	// Build resource → signal indices map
	resourceToSignals := make(map[string][]int)
	for i, sig := range signals {
		if len(sig.Related) == 0 {
			// Singleton: use a unique key
			key := singletonKey(i)
			resourceToSignals[key] = append(resourceToSignals[key], i)
		}
		for _, res := range sig.Related {
			resourceToSignals[res] = append(resourceToSignals[res], i)
		}
	}

	// Union-find to group signals sharing resources
	parent := make([]int, len(signals))
	for i := range parent {
		parent[i] = i
	}

	for _, indices := range resourceToSignals {
		if len(indices) <= 1 {
			continue
		}
		root := find(parent, indices[0])
		for _, idx := range indices[1:] {
			union(parent, root, idx)
		}
	}

	// Group signals by root
	groups := make(map[int][]int)
	for i := range signals {
		root := find(parent, i)
		groups[root] = append(groups[root], i)
	}

	// Build incidents
	incidents := make([]Incident, 0, len(groups))
	for _, indices := range groups {
		inc := buildIncident(signals, indices)
		incidents = append(incidents, inc)
	}

	// Rank: severity (critical first) → blast radius (highest) → recency (newest)
	sort.Slice(incidents, func(i, j int) bool {
		if incidents[i].TopSignal.Severity != incidents[j].TopSignal.Severity {
			return incidents[i].TopSignal.Severity > incidents[j].TopSignal.Severity
		}
		if incidents[i].BlastRadius != incidents[j].BlastRadius {
			return incidents[i].BlastRadius > incidents[j].BlastRadius
		}
		return incidents[i].TopSignal.Timestamp.After(incidents[j].TopSignal.Timestamp)
	})

	return incidents
}

func buildIncident(signals []Signal, indices []int) Incident {
	inc := Incident{}
	resourceSet := make(map[string]bool)
	maxBlast := 0

	for _, idx := range indices {
		sig := signals[idx]
		inc.Signals = append(inc.Signals, sig)
		for _, res := range sig.Related {
			resourceSet[res] = true
		}
		if sig.BlastRadius > maxBlast {
			maxBlast = sig.BlastRadius
		}
		if len(inc.Signals) == 1 || sig.Severity > inc.TopSignal.Severity {
			inc.TopSignal = sig
		}
	}

	for res := range resourceSet {
		inc.Resources = append(inc.Resources, res)
	}
	sort.Strings(inc.Resources)
	inc.BlastRadius = maxBlast

	// Generate suggested action based on top signal type
	inc.SuggestedAction = suggestAction(inc.TopSignal)

	return inc
}

func suggestAction(sig Signal) string {
	switch sig.Type {
	case SignalJobFailed:
		return "Investigate the failed job: check error details and upstream dependencies"
	case SignalTableStale:
		return "Check the pipeline that refreshes this table: is the load job running?"
	case SignalObjectMissing:
		return "Verify the upstream process that creates this file is running"
	case SignalLogError:
		return "Review the error logs for this resource"
	case SignalCostSpike:
		return "Review expensive queries: run /insights for cost breakdown"
	case SignalSecurityIssue:
		return "Review security findings: check IAM and access patterns"
	default:
		return "Investigate this issue"
	}
}

func singletonKey(idx int) string {
	return "__singleton__" + string(rune(idx+'0'))
}

// Union-find helpers
func find(parent []int, i int) int {
	for parent[i] != i {
		parent[i] = parent[parent[i]] // path compression
		i = parent[i]
	}
	return i
}

func union(parent []int, a, b int) {
	ra, rb := find(parent, a), find(parent, b)
	if ra != rb {
		parent[rb] = ra
	}
}
