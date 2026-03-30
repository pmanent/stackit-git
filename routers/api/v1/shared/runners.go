// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package shared

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	actions_model "forgejo.org/models/actions"
	"forgejo.org/models/db"
	"forgejo.org/models/shared/types"
	"forgejo.org/modules/container"
	"forgejo.org/modules/structs"
	"forgejo.org/modules/timeutil"
	"forgejo.org/modules/util"
	"forgejo.org/services/context"
)

// RegistrationToken is a string used to register a runner with a server
// swagger:response RegistrationToken
type RegistrationToken struct {
	Token string `json:"token"`
}

func GetRegistrationToken(ctx *context.APIContext, ownerID, repoID int64) {
	token, err := actions_model.GetLatestRunnerToken(ctx, ownerID, repoID)
	if errors.Is(err, util.ErrNotExist) || (token != nil && !token.IsActive) {
		token, err = actions_model.NewRunnerToken(ctx, ownerID, repoID)
	}
	if err != nil {
		ctx.InternalServerError(err)
		return
	}

	ctx.JSON(http.StatusOK, RegistrationToken{Token: token.Token})
}

func GetActionRunJobs(ctx *context.APIContext, ownerID, repoID int64) {
	labels := strings.Split(ctx.FormTrim("labels"), ",")

	total, err := db.Find[actions_model.ActionRunJob](ctx, &actions_model.FindTaskOptions{
		Status:  []actions_model.Status{actions_model.StatusWaiting, actions_model.StatusRunning},
		OwnerID: ownerID,
		RepoID:  repoID,
	})
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "CountWaitingActionRunJobs", err)
		return
	}

	res := fromRunJobModelToResponse(total, labels)

	ctx.JSON(http.StatusOK, res)
}

func fromRunJobModelToResponse(job []*actions_model.ActionRunJob, labels []string) []*structs.ActionRunJob {
	var res []*structs.ActionRunJob
	for i := range job {
		if job[i].ItRunsOn(labels) {
			res = append(res, &structs.ActionRunJob{
				ID:      job[i].ID,
				RepoID:  job[i].RepoID,
				OwnerID: job[i].OwnerID,
				Name:    job[i].Name,
				Needs:   job[i].Needs,
				RunsOn:  job[i].RunsOn,
				TaskID:  job[i].TaskID,
				Status:  job[i].Status.String(),
			})
		}
	}
	return res
}

// >>> @@@@ STACKIT Code @@@

// GetRunnerConsumption retrieves and calculates the consumption of runners based on
// various filtering criteria provided in the request. It aggregates task data for
// runners that match the specified labels and types within a given time frame.
// The results include metrics such as total processing duration and the number of
// tasks processed by each runner.
//
// Parameters:
//   - ctx: The API context containing the HTTP request. It is used to parse query
//     parameters (runner_labels, runner_types, start_date, end_date) and to send
//     the JSON response.
//
// The function writes a JSON response of type structs.RunnerConsumption to the context.
func GetRunnerConsumption(ctx *context.APIContext) {
	opts := NewGetRunnerConsumptionOptionsFromRequest(ctx)

	if err := opts.Validate(); err != nil {
		ctx.Error(http.StatusBadRequest, "Validation", err)
		return
	}

	findTaskOptions := &actions_model.FindTaskOptions{
		ListOptions: db.ListOptionsAll,
		RunnerIDs:   []int64{},
	}

	findRunnerOptions := actions_model.FindRunnerOptions{
		ListOptions: db.ListOptionsAll,
	}

	if opts.HasLabels() {
		findRunnerOptions.AgentLabels = opts.UniqueRawLabels.Values()
	}
	findTaskOptions.StartedBefore = timeutil.TimeStamp(opts.EndDateTime().Unix())
	findTaskOptions.StoppedAfter = timeutil.TimeStamp(opts.StartDateTime().Unix())

	var validOwnerTypes = make(container.Set[types.OwnerType])
	if opts.HasTypes() {
		for _, rt := range opts.UniqueRawTypes.Values() {
			candidate := types.OwnerType(rt)

			switch candidate {
			case types.OwnerTypeSystemGlobal,
				types.OwnerTypeIndividual,
				types.OwnerTypeRepository,
				types.OwnerTypeOrganization,
				types.OwnerTypeStackit:
				validOwnerTypes.Add(candidate)
			default:
				ctx.Error(http.StatusBadRequest, "InvalidRunnerType", fmt.Errorf("unknown runner type: %s", candidate))
				return
			}
		}
	} else {
		validOwnerTypes.AddMultiple(types.AllOwnerTypes...)
	}

	runnerList, err := db.Find[actions_model.ActionRunner](ctx, findRunnerOptions)
	if err != nil {
		ctx.ServerError("FindRunners", err)
		return
	}

	var runners []*actions_model.ActionRunner
	runners = make([]*actions_model.ActionRunner, 0, len(runnerList))
	runners = append(runners, runnerList...)

	for _, runner := range runners {
		if opts.HasTypes() {
			runnerType := runner.BelongsToOwnerType()

			if validOwnerTypes.Contains(runnerType) {
				findTaskOptions.RunnerIDs = append(findTaskOptions.RunnerIDs, runner.ID)
			}
		} else {
			findTaskOptions.RunnerIDs = append(findTaskOptions.RunnerIDs, runner.ID)
		}
	}

	total, err := db.Find[actions_model.ActionTask](ctx, findTaskOptions)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "CountWaitingActionRunJobs", err)
		return
	}

	res := fromGetRunnerConsumptionToResponse(total, runners)
	res.Meta.FilteredBy.RunnerTypes = container.ToStringSlice(validOwnerTypes.Values())
	res.Meta.FilteredBy.StartDate = opts.StartDate
	res.Meta.FilteredBy.EndDate = opts.EndDate

	ctx.JSON(http.StatusOK, res)
}

func fromGetRunnerConsumptionToResponse(tasks []*actions_model.ActionTask, runners []*actions_model.ActionRunner) *structs.RunnerConsumption {
	var res = &structs.RunnerConsumption{
		Meta: structs.RunnerConsumptionMeta{
			TotalTasksProcessed: len(tasks),
		},
		Data: []structs.RunnerConsumptionItem{},
	}

	groupedTasksMap := make(map[int64][]*actions_model.ActionTask)
	groupedRunnerMap := make(map[int64]*actions_model.ActionRunner)

	for _, r := range runners {
		groupedRunnerMap[r.ID] = r
	}

	for _, t := range tasks {
		groupedTasksMap[t.RunnerID] = append(groupedTasksMap[t.RunnerID], t)
	}

	for runnerId, runnerTasks := range groupedTasksMap {
		var runnerItem structs.RunnerConsumptionItem
		var runnerDuration timeutil.TimeStamp = 0
		var runnerInfo = groupedRunnerMap[runnerId]
		for i := range runnerTasks {
			currentTaskDuration := runnerTasks[i].Stopped - runnerTasks[i].Started
			runnerDuration = currentTaskDuration + runnerDuration
		}
		runnerItem.Metrics = structs.RunnerConsumptionItemMetrics{
			TotalDurationInSeconds: runnerDuration.AsTime().Unix(),
			TotalTasksProcessed:    len(runnerTasks),
		}
		runnerItem.RunnerID = runnerId
		runnerItem.RunnerType = string(runnerInfo.BelongsToOwnerType())
		runnerItem.RunnerLabels = runnerInfo.AgentLabels

		res.Data = append(res.Data, runnerItem)
	}

	return res
}

// NewGetRunnerConsumptionOptionsFromRequest creates a new GetRunnerConsumptionOptions from a request
// It parses query parameters such as runner_labels, runner_types, start_date, and end_date
// from the provided API context to populate the options struct.
//
// Parameters:
//   - ctx: The API context containing the HTTP request from which to extract the query parameters.
//
// Returns:
//
//	A pointer to a structs.GetRunnerConsumptionOptions struct populated with the values
//	from the request's query parameters.
func NewGetRunnerConsumptionOptionsFromRequest(ctx *context.APIContext) *structs.GetRunnerConsumptionOptions {
	opts := &structs.GetRunnerConsumptionOptions{
		RunnerLabels: ctx.Req.FormValue("runner_labels"),
		RunnerTypes:  ctx.Req.FormValue("runner_types"),
		StartDate:    ctx.Req.FormValue("start_date"),
		EndDate:      ctx.Req.FormValue("end_date"),
	}

	if opts.RunnerTypes != "" {
		opts.RawTypes = strings.Split(opts.RunnerTypes, ",")
	}
	if opts.RunnerLabels != "" {
		opts.RawLabels = strings.Split(opts.RunnerLabels, ",")
	}

	opts.UniqueRawLabels = make(container.Set[string])
	opts.UniqueRawTypes = make(container.Set[string])
	for _, rl := range opts.RawLabels {
		opts.UniqueRawLabels.Add(strings.TrimSpace(rl))
	}
	for _, rt := range opts.RawTypes {
		opts.UniqueRawTypes.Add(strings.TrimSpace(rt))
	}
	return opts
}

//>>> @@@@ STACKIT Code @@@
