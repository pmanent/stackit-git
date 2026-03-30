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
	"forgejo.org/modules/optional"
	"forgejo.org/modules/structs"
	"forgejo.org/modules/timeutil"
	"forgejo.org/modules/util"
	"forgejo.org/modules/web"
	"forgejo.org/routers/api/v1/utils"
	"forgejo.org/services/context"
	"forgejo.org/services/convert"

	gouuid "github.com/google/uuid"
)

// RegistrationToken is a string used to register a runner with a server
type RegistrationToken struct {
	Token string `json:"token"`
}

func GetRegistrationToken(ctx *context.APIContext, ownerID, repoID int64) {
	optOwnerID := optional.None[int64]()
	if ownerID != 0 {
		optOwnerID = optional.Some(ownerID)
	}
	optRepoID := optional.None[int64]()
	if repoID != 0 {
		optRepoID = optional.Some(repoID)
	}

	token, err := actions_model.GetLatestRunnerToken(ctx, optOwnerID, optRepoID)
	if errors.Is(err, util.ErrNotExist) || (token != nil && !token.IsActive) {
		token, err = actions_model.NewRunnerToken(ctx, optOwnerID, optRepoID)
	}
	if err != nil {
		ctx.InternalServerError(err)
		return
	}

	ctx.JSON(http.StatusOK, RegistrationToken{Token: token.Token})
}

func GetActionRunJobs(ctx *context.APIContext, ownerID, repoID int64) {
	labels := []string{}
	if len(ctx.Req.Form["labels"]) > 0 {
		labels = strings.Split(ctx.FormTrim("labels"), ",")
	}

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
		if len(labels) == 0 || labels[0] == "" && len(job[i].RunsOn) == 0 || job[i].ItRunsOn(labels) {
			res = append(res, convert.ToActionRunJob(job[i]))
		}
	}
	return res
}

// ListRunners lists runners for api route validated ownerID and repoID
// ownerID == 0 and repoID == 0 means all runners including global runners, does not appear in sql where clause
// ownerID == 0 and repoID != 0 means all runners for the given repo
// ownerID != 0 and repoID == 0 means all runners for the given user/org
// ownerID != 0 and repoID != 0 undefined behavior
// Access rights are checked at the API route level
func ListRunners(ctx *context.APIContext, ownerID, repoID int64) {
	if ownerID != 0 && repoID != 0 {
		ctx.Error(http.StatusUnprocessableEntity, "", fmt.Errorf("ownerID and repoID should not be both set: %d and %d", ownerID, repoID))
		return
	}

	listOptions := utils.GetListOptions(ctx)
	runners, total, err := db.FindAndCount[actions_model.ActionRunner](ctx, &actions_model.FindRunnerOptions{
		OwnerID:     ownerID,
		RepoID:      repoID,
		ListOptions: listOptions,
		WithVisible: ctx.FormBool("visible"),
	})
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "FindCountRunners", map[string]string{})
		return
	}

	runnerList := make([]structs.ActionRunner, len(runners))
	for i, runner := range runners {
		actionRunner, err := convert.ToActionRunner(runner)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "ToActionRunner", err)
			return
		}
		runnerList[i] = actionRunner
	}

	ctx.SetLinkHeader(int(total), listOptions.PageSize)
	ctx.SetTotalCountHeader(total)
	ctx.JSON(http.StatusOK, &runnerList)
}

// GetRunner get the runner for api route validated ownerID and repoID
// ownerID == 0 and repoID == 0 means any runner including global runners
// ownerID == 0 and repoID != 0 means any runner for the given repo
// ownerID != 0 and repoID == 0 means any runner for the given user/org
// ownerID != 0 and repoID != 0 undefined behavior
// Access rights are checked at the API route level
func GetRunner(ctx *context.APIContext, ownerID, repoID, runnerID int64) {
	if ownerID != 0 && repoID != 0 {
		ctx.Error(http.StatusUnprocessableEntity, "", fmt.Errorf("ownerID and repoID should not be both set: %d and %d", ownerID, repoID))
		return
	}
	runner, err := actions_model.GetVisibleRunnerByID(ctx, runnerID, ownerID, repoID)
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			ctx.Error(http.StatusNotFound, "GetRunnerNotFound", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetRunnerFailed", err)
		}
		return
	}

	actionRunner, err := convert.ToActionRunner(runner)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "ToActionRunner", err)
		return
	}
	ctx.JSON(http.StatusOK, actionRunner)
}

func RegisterRunner(ctx *context.APIContext, ownerID, repoID int64) {
	if ownerID != 0 && repoID != 0 {
		ctx.Error(http.StatusUnprocessableEntity, "RegisterRunner", fmt.Errorf("ownerID '%d' and repoID '%d' cannot be set simultaneously", ownerID, repoID))
		return
	}

	options := web.GetForm(ctx).(*structs.RegisterRunnerOptions)
	runner := &actions_model.ActionRunner{
		UUID:        gouuid.NewString(),
		Name:        options.Name,
		OwnerID:     ownerID,
		RepoID:      repoID,
		Description: options.Description,
		Ephemeral:   options.Ephemeral,
	}
	runner.GenerateToken()
	if err := actions_model.CreateRunner(ctx, runner); err != nil {
		ctx.Error(http.StatusInternalServerError, "CreateRunner", err)
		return
	}

	response := &structs.RegisterRunnerResponse{
		ID:    runner.ID,
		UUID:  runner.UUID,
		Token: runner.Token,
	}
	ctx.JSON(http.StatusCreated, response)
}

// DeleteRunner deletes the runner for api route validated ownerID and repoID
// ownerID == 0 and repoID == 0 means any runner including global runners
// ownerID == 0 and repoID != 0 means any runner for the given repo
// ownerID != 0 and repoID == 0 means any runner for the given user/org
// ownerID != 0 and repoID != 0 undefined behavior
// Access rights are checked at the API route level
func DeleteRunner(ctx *context.APIContext, ownerID, repoID, runnerID int64) {
	if ownerID != 0 && repoID != 0 {
		ctx.Error(http.StatusUnprocessableEntity, "", fmt.Errorf("ownerID and repoID should not be both set: %d and %d", ownerID, repoID))
		return
	}
	runner, err := actions_model.GetVisibleRunnerByID(ctx, runnerID, ownerID, repoID)
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			ctx.Error(http.StatusNotFound, "DeleteRunnerNotFound", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "DeleteRunnerFailed", err)
		}
		return
	}
	if !runner.Editable(ownerID, repoID) {
		ctx.Error(http.StatusNotFound, "EditRunner", "No permission to delete this runner")
		return
	}

	err = actions_model.DeleteRunner(ctx, runner)
	if err != nil {
		ctx.InternalServerError(err)
		return
	}
	ctx.Status(http.StatusNoContent)
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

	findRunnerOptions := actions_model.FindRunnerOptions{
		ListOptions: db.ListOptionsAll,
	}

	if opts.HasLabels() {
		findRunnerOptions.AgentLabels = opts.UniqueRawLabels.Values()
	}

	validOwnerTypes := make(container.Set[types.OwnerType])
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

	startedBefore := timeutil.TimeStamp(opts.EndDateTime().Unix())
	stoppedAfterUnix := timeutil.TimeStamp(opts.StartDateTime().Unix())

	// Collect all tasks for selected runners
	var allTasks []*actions_model.ActionTask
	for _, runner := range runners {
		if opts.HasTypes() {
			runnerType := runner.BelongsToOwnerType()
			if !validOwnerTypes.Contains(runnerType) {
				continue
			}
		}

		tasks, taskErr := db.Find[actions_model.ActionTask](ctx, &actions_model.FindTaskOptions{
			ListOptions:   db.ListOptionsAll,
			RunnerID:      runner.ID,
			StartedBefore: startedBefore,
		})
		if taskErr != nil {
			ctx.Error(http.StatusInternalServerError, "FindRunnerTasks", taskErr)
			return
		}
		// Filter tasks that stopped after the start time
		for _, t := range tasks {
			if t.Stopped >= stoppedAfterUnix {
				allTasks = append(allTasks, t)
			}
		}
	}

	res := fromGetRunnerConsumptionToResponse(allTasks, runners)
	res.Meta.FilteredBy.RunnerTypes = container.ToStringSlice(validOwnerTypes.Values())
	res.Meta.FilteredBy.StartDate = opts.StartDate
	res.Meta.FilteredBy.EndDate = opts.EndDate

	ctx.JSON(http.StatusOK, res)
}

func fromGetRunnerConsumptionToResponse(tasks []*actions_model.ActionTask, runners []*actions_model.ActionRunner) *structs.RunnerConsumption {
	res := &structs.RunnerConsumption{
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

	for runnerID, runnerTasks := range groupedTasksMap {
		var runnerItem structs.RunnerConsumptionItem
		var runnerDuration timeutil.TimeStamp
		runnerInfo := groupedRunnerMap[runnerID]
		for i := range runnerTasks {
			currentTaskDuration := runnerTasks[i].Stopped - runnerTasks[i].Started
			runnerDuration = currentTaskDuration + runnerDuration
		}
		runnerItem.Metrics = structs.RunnerConsumptionItemMetrics{
			TotalDurationInSeconds: runnerDuration.AsTime().Unix(),
			TotalTasksProcessed:    len(runnerTasks),
		}
		runnerItem.RunnerID = runnerID
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

// >>> @@@@ STACKIT Code @@@
