// Copyright 2024 The Forgejo Authors c/o Codeberg e.V.. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	actions_model "forgejo.org/models/actions"
	auth_model "forgejo.org/models/auth"
	"forgejo.org/models/unittest"
	"forgejo.org/modules/setting"
	api "forgejo.org/modules/structs"
	"forgejo.org/tests"

	"github.com/stretchr/testify/assert"
)

func TestAPISearchActionJobs_GlobalRunner(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	job := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: 393})
	adminUsername := "user1"
	token := getUserToken(t, adminUsername, auth_model.AccessTokenScopeWriteAdmin)

	req := NewRequest(
		t,
		"GET",
		fmt.Sprintf("/api/v1/admin/runners/jobs?labels=%s", "ubuntu-latest"),
	).AddTokenAuth(token)
	res := MakeRequest(t, req, http.StatusOK)

	var jobs []*api.ActionRunJob
	DecodeJSON(t, res, &jobs)

	assert.Len(t, jobs, 1)
	assert.EqualValues(t, job.ID, jobs[0].ID)
}

// >>> @@@@ STACKIT Code @@@
func TestAPIGetRunnerConsumption(t *testing.T) {
	defer tests.AddFixtures("tests/integration/fixtures/TestAPIGetRunnerConsumption/")()
	defer tests.PrepareTestEnv(t)()

	user := "user1"

	t.Run("When no parameters are sended", func(t *testing.T) {
		var fields = map[string]string{}
		resp := makeApiRequest(t, user, fields, http.StatusBadRequest)

		expectedAPIError := api.APIError{
			Message: "start_date is required",
			URL:     setting.API.SwaggerURL,
		}
		var apiError api.APIError
		DecodeJSON(t, resp, &apiError)
		assert.Equal(t, expectedAPIError, apiError)
	})

	t.Run("When only start_date parameter is sended", func(t *testing.T) {
		var fields = map[string]string{"start_date": "2025-11-27T00:00:00Z"}
		resp := makeApiRequest(t, user, fields, http.StatusBadRequest)

		expectedAPIError := api.APIError{
			Message: "end_date is required",
			URL:     setting.API.SwaggerURL,
		}
		var apiError api.APIError
		DecodeJSON(t, resp, &apiError)
		assert.Equal(t, expectedAPIError, apiError)
	})

	t.Run("When start_date and end_date parameters are sended", func(t *testing.T) {
		t.Run("And is filtered by year 2025", func(t *testing.T) {
			var fields = map[string]string{
				"start_date": "2025-01-01T00:00:00Z",
				"end_date":   "2025-12-31T23:59:59Z",
			}
			resp := makeApiRequest(t, user, fields, http.StatusOK)

			var apiRunnerConsumption api.RunnerConsumption
			DecodeJSON(t, resp, &apiRunnerConsumption)

			assert.Equal(t, len(apiRunnerConsumption.Data), int(3))
			assert.Equal(t, apiRunnerConsumption.Meta.TotalTasksProcessed, int(9))
		})

		t.Run("And is filtered by year 1 Month (January 2025)", func(t *testing.T) {
			var fields = map[string]string{
				"start_date": "2025-01-01T00:00:00Z",
				"end_date":   "2025-01-31T23:59:59Z",
			}
			resp := makeApiRequest(t, user, fields, http.StatusOK)

			var apiRunnerConsumption api.RunnerConsumption
			DecodeJSON(t, resp, &apiRunnerConsumption)

			assert.Equal(t, len(apiRunnerConsumption.Data), int(1))
			assert.Equal(t, apiRunnerConsumption.Meta.TotalTasksProcessed, int(2))
		})

		t.Run("And is filtered by 1 year and runner label ubuntu", func(t *testing.T) {
			var fields = map[string]string{
				"start_date":    "2025-01-01T00:00:00Z",
				"end_date":      "2025-12-31T23:59:59Z",
				"runner_labels": "ubuntu",
			}
			resp := makeApiRequest(t, user, fields, http.StatusOK)

			var apiRunnerConsumption api.RunnerConsumption
			DecodeJSON(t, resp, &apiRunnerConsumption)

			assert.Equal(t, len(apiRunnerConsumption.Data), int(2))
			assert.Equal(t, apiRunnerConsumption.Meta.TotalTasksProcessed, int(8))
		})

		t.Run("And is filtered by 1 year and runner type stackit", func(t *testing.T) {
			var fields = map[string]string{
				"start_date":   "2025-01-01T00:00:00Z",
				"end_date":     "2025-12-31T23:59:59Z",
				"runner_types": "stackit",
			}
			resp := makeApiRequest(t, user, fields, http.StatusOK)

			var apiRunnerConsumption api.RunnerConsumption
			DecodeJSON(t, resp, &apiRunnerConsumption)

			assert.Equal(t, len(apiRunnerConsumption.Data), int(1))
			assert.Equal(t, apiRunnerConsumption.Meta.TotalTasksProcessed, int(5))
		})

		t.Run("But end_date is greater than start_date", func(t *testing.T) {
			var fields = map[string]string{
				"start_date": "2025-12-31T23:59:59Z",
				"end_date":   "2025-01-01T00:00:00Z",
			}
			resp := makeApiRequest(t, user, fields, http.StatusBadRequest)

			expectedAPIError := api.APIError{
				Message: "start_date cannot be later than end_date",
				URL:     setting.API.SwaggerURL,
			}
			var apiError api.APIError
			DecodeJSON(t, resp, &apiError)
			assert.Equal(t, expectedAPIError, apiError)
		})

		t.Run("And is filtered by 1 year and runner type is invalid", func(t *testing.T) {
			var fields = map[string]string{
				"start_date":   "2025-01-01T00:00:00Z",
				"end_date":     "2025-12-31T23:59:59Z",
				"runner_types": "lidl-runner",
			}
			resp := makeApiRequest(t, user, fields, http.StatusBadRequest)

			expectedAPIError := api.APIError{
				Message: "unknown runner type: lidl-runner",
				URL:     setting.API.SwaggerURL,
			}
			var apiError api.APIError
			DecodeJSON(t, resp, &apiError)
			assert.Equal(t, expectedAPIError, apiError)
		})
	})
}

func makeApiRequest(t *testing.T, user string, fields map[string]string, expectedHTTPCode int) *httptest.ResponseRecorder {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	for key, value := range fields {
		_ = writer.WriteField(key, value)
	}

	_ = writer.Close()
	req := NewRequestWithBody(t, "GET", "/api/v1/admin/runners/consumption", body).
		SetHeader("Content-Type", writer.FormDataContentType())
	req.SetBasicAuth(user, userPassword)
	resp := MakeRequest(t, req, expectedHTTPCode)
	return resp
}

// >>> @@@@ STACKIT Code @@@
