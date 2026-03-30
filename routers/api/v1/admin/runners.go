// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package admin

import (
	"forgejo.org/routers/api/v1/shared"
	"forgejo.org/services/context"
)

// https://docs.github.com/en/rest/actions/self-hosted-runners?apiVersion=2022-11-28#create-a-registration-token-for-an-organization

// GetRegistrationToken returns the token to register global runners
func GetRegistrationToken(ctx *context.APIContext) {
	// swagger:operation GET /admin/runners/registration-token admin adminGetRunnerRegistrationToken
	// ---
	// summary: Get an global actions runner registration token
	// produces:
	// - application/json
	// parameters:
	// responses:
	//   "200":
	//     "$ref": "#/responses/RegistrationToken"

	shared.GetRegistrationToken(ctx, 0, 0)
}

// SearchActionRunJobs return a list of actions jobs filtered by the provided parameters
func SearchActionRunJobs(ctx *context.APIContext) {
	// swagger:operation GET /admin/runners/jobs admin adminSearchRunJobs
	// ---
	// summary: Search action jobs according filter conditions
	// produces:
	// - application/json
	// parameters:
	// - name: labels
	//   in: query
	//   description: a comma separated list of run job labels to search for
	//   type: string
	// responses:
	//   "200":
	//     "$ref": "#/responses/RunJobList"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	shared.GetActionRunJobs(ctx, 0, 0)
}

// >>> @@@@ STACKIT Code @@@

// GetRunnerConsumption returns the consumption for runners.
// The consumption can be filtered by start and end date, runner types and runner labels,
func GetRunnerConsumption(ctx *context.APIContext) {
	// swagger:operation GET /admin/runners/consumption admin adminGetRunnerConsumption
	// ---
	// summary: Get runner consumption
	// produces:
	// - application/json
	// parameters:
	// - name: start_date
	//   in: query
	//   description: |-
	//     Limit results to items with a creation timestamp greater than or equal to this value (inclusive).
	//     Must be in RFC3339 format.
	//   type: string
	//   required: true
	//   example: "2023-01-01T00:00:00Z"
	// - name: end_date
	//   in: query
	//   description: |-
	//     Limit results to items with a creation timestamp less than or equal to this value (inclusive).
	//     Must be in RFC3339 format.
	//   type: string
	//   required: true
	//   example: "2023-01-31T23:59:59Z"
	// - name: runner_types
	//   in: query
	//   description: |-
	//     A comma-separated list of runner types to search for.
	//     Possible values are: `system-global`, `individual`, `repository`, `organization`, `stackit`.
	//   type: string
	// - name: runner_labels
	//   in: query
	//   description: A comma-separated list of runner labels to search for.
	//   type: string
	//   example: "ubuntu,ubuntu-latest"
	// responses:
	//   "200":
	//     description: "A successful response detailing the consumption metrics for runners matching the filter criteria."
	//     schema:
	//       "$ref": "#/definitions/RunnerConsumption"
	//     examples:
	//       application/json: {"meta":{"total_tasks_processed":5,"filtered_by":{"runner_types":["stackit"],"start_date":"2025-01-01T00:00:00Z","end_date":"2025-12-31T23:59:59Z"}},"data":[{"runner_id":1,"runner_type":"stackit","runner_labels":["stackit-ubuntu-20","stackit-docker"],"metrics":{"total_duration_in_seconds":1860,"total_tasks_processed":5}}]}
	//   "400":
	//     description: "Bad Request, e.g. missing required parameters"
	//     schema:
	//       "$ref": "#/definitions/APIError"
	//     examples:
	//       application/json: { "message": "start_date is required", "url": "URL" }
	shared.GetRunnerConsumption(ctx)
}

//>>> @@@@ STACKIT Code @@@
