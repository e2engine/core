package model

import (
	"time"
)

type TestSuiteExecution struct {
	ID         string          `json:"id" yaml:"id"`
	StartedAt  time.Time       `json:"started_at" yaml:"started_at"`
	FinishedAt time.Time       `json:"finished_at" yaml:"finished_at"`
	Status     ExecutionStatus `json:"status" yaml:"status"`

	EnvironmentID   string `json:"environment_id" yaml:"environment_id"`
	EnvironmentName string `json:"environment_name,omitempty" yaml:"environment_name,omitempty"`

	TestSuiteID   string `json:"testsuite_id" yaml:"testsuite_id"`
	TestSuiteName string `json:"testsuite_name,omitempty" yaml:"testsuite_name,omitempty"`

	TestsCount int             `json:"tests_count" yaml:"tests_count"`
	Tests      []TestExecution `json:"tests,omitempty" yaml:"tests,omitempty"`

	Summary *TestSuiteExecutionSummary `json:"summary,omitempty" yaml:"summary,omitempty"`
}

type TestSuiteExecutionSummary struct {
	Error string `json:"error,omitempty" yaml:"error,omitempty"`

	Total  int `json:"total" yaml:"total"`
	Passed int `json:"passed" yaml:"passed"`
	Failed int `json:"failed" yaml:"failed"`
	Errors int `json:"errors" yaml:"errors"`
}

// TODO: Add setters.

type TestSuiteExecutionsPage = Page[TestSuiteExecution]
