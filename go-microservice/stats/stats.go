/*
Copyright © 2024 Patrick Hermann patrick.hermann@sva.de
*/

// Package stats provides structures for collecting and reporting workflow statistics.
package stats

type WorkflowStats struct {
	Lint struct {
		Duration string   `json:"duration"`
		Findings []string `json:"findings"`
	} `json:"lint"`
	Test struct {
		Duration string `json:"duration"`
		Coverage string `json:"coverage"`
	} `json:"test"`
	SecurityScan struct {
		Duration string   `json:"duration"`
		Findings []string `json:"findings"`
	} `json:"securityScan"`
	TotalDuration string `json:"totalDuration"` // Total duration of the workflow
}
