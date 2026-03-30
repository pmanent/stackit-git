// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package admin

import (
	"reflect"
	"testing"

	"forgejo.org/models/unittest"
	"forgejo.org/modules/setting"
	"forgejo.org/services/contexttest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConfigStackitGitFieldNames ensures the StackitGit struct fields exposed to
// the admin/config template use the exact names referenced in the template.
// This catches case-sensitivity mismatches (e.g. OrganizationId vs OrganizationID)
// that cause "can't evaluate field … in type interface {}" template render errors.
func TestConfigStackitGitFieldNames(t *testing.T) {
	unittest.PrepareTestEnv(t)

	// Verify the setting struct itself has the fields with the correct names.
	st := reflect.TypeOf(setting.StackitGit)
	_, hasOrgID := st.FieldByName("OrganizationID")
	require.True(t, hasOrgID, "setting.StackitGit must have field OrganizationID")
	_, hasProjID := st.FieldByName("ProjectID")
	require.True(t, hasProjID, "setting.StackitGit must have field ProjectID")

	// Verify that Config() puts a value in ctx.Data["StackitGit"] whose fields
	// are accessible under the names the template uses.
	ctx, _ := contexttest.MockContext(t, "admin/config")
	Config(ctx)

	raw, ok := ctx.Data["StackitGit"]
	require.True(t, ok, "Config() must set ctx.Data[\"StackitGit\"]")

	v := reflect.ValueOf(raw)
	assert.True(t, v.FieldByName("OrganizationID").IsValid(),
		"StackitGit value must expose OrganizationID (template uses .StackitGit.OrganizationID)")
	assert.True(t, v.FieldByName("ProjectID").IsValid(),
		"StackitGit value must expose ProjectID (template uses .StackitGit.ProjectID)")
}
