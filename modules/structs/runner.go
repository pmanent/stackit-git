// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package structs

import (
	"fmt"
	"time"

	"forgejo.org/modules/container"
)

// RegisterRunnerOptions declares the accepted options for registering runners.
// swagger:model
type RegisterRunnerOptions struct {
	// Name of the runner to register. The name of the runner does not have to be unique.
	//
	// required: true
	Name string `json:"name" binding:"Required"`

	// Description of the runner to register.
	//
	// required: false
	Description string `json:"description"`

	// Register as ephemeral runner https://forgejo.org/docs/latest/admin/actions/security/#ephemeral-runner
	//
	// required: false
	Ephemeral bool `json:"ephemeral"`
}

// RegisterRunnerResponse contains the details of the just registered runner.
// swagger:model
type RegisterRunnerResponse struct {
	ID    int64  `json:"id" binding:"Required"`
	UUID  string `json:"uuid" binding:"Required"`
	Token string `json:"token" binding:"Required"`
}

// >>> @@@@ STACKIT Code @@@@

// RunnerConsumption represents a compound of RunnerConsumptionMeta and RunnerConsumptionItem
// swagger:model
type RunnerConsumption struct {
	// meta info
	Meta RunnerConsumptionMeta `json:"meta"`
	// list of items
	Data []RunnerConsumptionItem `json:"data"`
}

// RunnerConsumptionMeta represents a meta info from RunnerConsumption
// swagger:model
type RunnerConsumptionMeta struct {
	// total tasks processed
	TotalTasksProcessed int `json:"total_tasks_processed"`
	// filtered by info
	FilteredBy RunnerConsumptionMetaFilteredBy `json:"filtered_by"`
}

// RunnerConsumptionMetaFilteredBy represents a meta info from RunnerConsumption
// swagger:model
type RunnerConsumptionMetaFilteredBy struct {
	// list of runner types
	RunnerTypes []string `json:"runner_types"`
	// start date
	StartDate string `json:"start_date"`
	// end date
	EndDate string `json:"end_date"`
}

// RunnerConsumptionItem represents a consumption of a runner
// swagger:model
type RunnerConsumptionItem struct {
	// the runner id
	RunnerID int64 `json:"runner_id"`
	// the runner type
	RunnerType string `json:"runner_type"`
	// the runner labels
	RunnerLabels []string `json:"runner_labels"`
	// the metrics of the runner
	Metrics RunnerConsumptionItemMetrics `json:"metrics"`
}

// RunnerConsumptionItemMetrics represents a consumption of a runner
// swagger:model
type RunnerConsumptionItemMetrics struct {
	// total duration in seconds
	TotalDurationInSeconds int64 `json:"total_duration_in_seconds"`
	// the repository id
	TotalTasksProcessed int `json:"total_tasks_processed"`
}

// GetRunnerConsumption represents the options for searching or filtering runner consumption data
// swagger:model
type GetRunnerConsumption struct {
	// A list of runner labels to filter by.
	Labels []string `json:"runner_labels"`
	// A list of runner types to filter by.
	Types []string `json:"runner_types"`
	// The start date for the filter window (RFC3339 format).
	StartDate string `json:"start_date" binding:"Required"`
	// The end date for the filter window (RFC3339 format).
	EndDate string `json:"end_date" binding:"Required"`
}

// GetRunnerConsumptionOptions represents the options for getting runner consumption
type GetRunnerConsumptionOptions struct {
	RunnerLabels    string
	RunnerTypes     string
	StartDate       string
	EndDate         string
	RawTypes        []string
	RawLabels       []string
	UniqueRawLabels container.Set[string]
	UniqueRawTypes  container.Set[string]
}

// Validate validates the options for getting runner consumption.
// It checks if the start and end dates are provided and in the correct format (RFC3339).
// It also ensures that the start date is not later than the end date.
// It returns an error if any validation fails, otherwise nil.
func (o *GetRunnerConsumptionOptions) Validate() error {
	if len(o.StartDate) == 0 {
		return fmt.Errorf("start_date is required")
	}
	startDate, err := time.Parse(time.RFC3339, o.StartDate)
	if err != nil {
		return fmt.Errorf("invalid start_date format, expected RFC3339: %w", err)
	}

	if len(o.EndDate) == 0 {
		return fmt.Errorf("end_date is required")
	}
	endDate, err := time.Parse(time.RFC3339, o.EndDate)
	if err != nil {
		return fmt.Errorf("invalid end_date format, expected RFC3339: %w", err)
	}

	if startDate.Compare(endDate) == 1 {
		return fmt.Errorf("start_date cannot be later than end_date")
	}

	return nil
}

func (o *GetRunnerConsumptionOptions) HasLabels() bool {
	return len(o.UniqueRawLabels.Values()) > 0
}

func (o *GetRunnerConsumptionOptions) HasTypes() bool {
	return len(o.UniqueRawTypes.Values()) > 0
}

func (o *GetRunnerConsumptionOptions) StartDateTime() time.Time {
	time, _ := time.Parse(time.RFC3339, o.StartDate)
	return time
}

func (o *GetRunnerConsumptionOptions) EndDateTime() time.Time {
	time, _ := time.Parse(time.RFC3339, o.EndDate)
	return time
}

// <<< @@@@ STACKIT Code @@@@
