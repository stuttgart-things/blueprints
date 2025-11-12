/*
Copyright © 2024 Patrick Hermann patrick.hermann@sva.de
*/

// Package stats provides structures for collecting and reporting workflow statistics.
package stats

type WorkflowStats struct {
	TotalDuration string    `json:"total_duration"`
	HasFailures   bool      `json:"has_failures"`
	Lint          LintStats `json:"lint,omitempty"`
	Test          TestStats `json:"test,omitempty"`
}

type LintStats struct {
	Duration string   `json:"duration"`
	Failed   bool     `json:"failed"`
	Error    string   `json:"error,omitempty"`
	Findings []string `json:"findings,omitempty"`
}

type TestStats struct {
	Duration string `json:"duration"`
	Failed   bool   `json:"failed"`
	Error    string `json:"error,omitempty"`
	Output   string `json:"output,omitempty"`
	Coverage string `json:"coverage,omitempty"`
}
