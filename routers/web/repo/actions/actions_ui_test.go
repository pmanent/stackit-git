// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

	"forgejo.org/models/unittest"
	"forgejo.org/modules/templates"
	"forgejo.org/services/context"
	"forgejo.org/services/contexttest"
	"forgejo.org/services/htmltest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	sidebarSelector         = "#sidebar_section"
	workflowSidebarSelector = "#workflow_sidebar"
	mainSelector            = "#main_section"
	mockedWorkflows         = []map[string]interface{}{
		{
			"ErrMsg": "",
			"Entry": map[string]interface{}{
				"ID":   "019a3707-02f3-776c-9253-e3c997275009",
				"Name": "demo.yml",
			},
		},
		{
			"ErrMsg": "",
			"Entry": map[string]interface{}{
				"ID":   "019a3707-22f3-776c-9253-e3c997275009",
				"Name": "test.yml",
			},
		},
	}
)

func baseTest(t *testing.T, ctx *context.Context) {
	t.Run("Core test", func(t *testing.T) {
		List(ctx)
		rawHtml, err := ctx.RenderToHTML("repo/actions/list", ctx.Data)
		require.NoError(t, err)
		doc := htmltest.NewHTMLParserFromTemplateHTML(t, rawHtml)

		t.Run("Check core HTML elemens exists", func(t *testing.T) {
			doc.AssertElementExists(t, sidebarSelector)
			doc.AssertElementExists(t, workflowSidebarSelector)
			doc.AssertElementExists(t, mainSelector)
		})
	})
}

func noWorkflowsTest(t *testing.T, ctx *context.Context) {
	t.Run("No workflows test", func(t *testing.T) {
		List(ctx)
		rawHtml, err := ctx.RenderToHTML("repo/actions/list", ctx.Data)
		require.NoError(t, err)
		doc := htmltest.NewHTMLParserFromTemplateHTML(t, rawHtml)

		t.Run("No workflows", func(t *testing.T) {
			workflow_main_section := doc.Find(mainSelector)
			workflow_sidebar_title := doc.Find(workflowSidebarSelector).Children().First().Text()
			workflow_sidebar_item := doc.Find(workflowSidebarSelector).Children().Last().Text()
			t.Run("Check sidebar HTML elements", func(t *testing.T) {
				doc.AssertElementCount(t, workflowSidebarSelector, 1)
				doc.AssertElementChildCount(t, workflowSidebarSelector, 2)
				assert.Contains(t, workflow_sidebar_title, "actions.runs.all_workflows")
				assert.Contains(t, workflow_sidebar_item, "actions.runs.workflows.empty")
			})

			t.Run("Check Quick guide is visible", func(t *testing.T) {
				htmltest.AssertElementContains(t, workflow_main_section, "h4", "repo.quick_guide")
			})
		})
	})
}

func noRunnersTest(t *testing.T, ctx *context.Context) {
	t.Run("No runners test", func(t *testing.T) {
		List(ctx)
		rawHtml, err := ctx.RenderToHTML("repo/actions/list", ctx.Data)
		require.NoError(t, err)
		doc := htmltest.NewHTMLParserFromTemplateHTML(t, rawHtml)

		t.Run("Check main HTML elements", func(t *testing.T) {
			workflow_main_section := doc.Find(mainSelector)
			doc.AssertElementCount(t, mainSelector, 1)

			t.Run("Check Enable Stackit Runners is visible", func(t *testing.T) {
				htmltest.AssertElementContains(t, workflow_main_section, "h4", "actions.runs.enable_stackit_runners")
			})
		})
	})
}

func withStackitRunnersNotUsedTest(t *testing.T, ctx *context.Context) {
	t.Run("With stackit runners but not used test", func(t *testing.T) {
		List(ctx)

		ctx.Data["workflows"] = mockedWorkflows
		ctx.Data["HasWorkflows"] = true
		ctx.Data["HasRunners"] = true
		ctx.Data["HasStackitRunner"] = true
		ctx.Data["HasStackitRunnerUsed"] = false

		rawHtml, err := ctx.RenderToHTML("repo/actions/list", ctx.Data)
		require.NoError(t, err)
		doc := htmltest.NewHTMLParserFromTemplateHTML(t, rawHtml)

		t.Run("Check main HTML elements", func(t *testing.T) {
			workflow_main_section := doc.Find(mainSelector)
			doc.AssertElementCount(t, mainSelector, 1)

			t.Run("Check Quick guide is not visible", func(t *testing.T) {
				htmltest.AssertElementNotContains(t, workflow_main_section, "h4", "repo.quick_guide")
			})

			t.Run("Check Enable Stackit Runners is not visible", func(t *testing.T) {
				htmltest.AssertElementNotContains(t, workflow_main_section, "h4", "actions.runs.enable_stackit_runners")
			})

			t.Run("Check start using runners is visible", func(t *testing.T) {
				htmltest.AssertElementContains(t, workflow_main_section, "h4", "start_using_stackit_runners")
			})
		})
	})

}

func withWorkflowsTest(t *testing.T, ctx *context.Context) {
	t.Run("With workflows test", func(t *testing.T) {
		List(ctx)

		ctx.Data["workflows"] = mockedWorkflows
		ctx.Data["HasWorkflows"] = true

		rawHtml, err := ctx.RenderToHTML("repo/actions/list", ctx.Data)

		require.NoError(t, err)
		doc := htmltest.NewHTMLParserFromTemplateHTML(t, rawHtml)

		t.Run("With workflows", func(t *testing.T) {
			workflow_main_section := doc.Find(mainSelector)
			workflow_sidebar_title := doc.Find(workflowSidebarSelector).Children().First().Text()
			workflow_sidebar_item_demo := doc.Find(workflowSidebarSelector).Children().Eq(1).Text()
			workflow_sidebar_item_test := doc.Find(workflowSidebarSelector).Children().Eq(2).Text()
			t.Run("Check sidebar HTML elements", func(t *testing.T) {
				doc.AssertElementCount(t, workflowSidebarSelector, 1)
				doc.AssertElementChildCount(t, workflowSidebarSelector, 3)
				assert.Contains(t, workflow_sidebar_title, "actions.runs.all_workflows")
				assert.Contains(t, workflow_sidebar_item_demo, "demo.yml")
				assert.Contains(t, workflow_sidebar_item_test, "test.yml")
			})

			t.Run("Check Quick guide is not visible", func(t *testing.T) {
				htmltest.AssertElementNotContains(t, workflow_main_section, "h4", "repo.quick_guide")
			})
		})
	})
}

func TestActionsUI(t *testing.T) {
	unittest.PrepareTestEnv(t)

	ctx, _ := contexttest.MockContext(t, "user2/repo1/actions", contexttest.MockContextOption{Render: templates.HTMLRenderer()})
	contexttest.LoadUser(t, ctx, 2)
	contexttest.LoadRepo(t, ctx, 1)
	contexttest.LoadGitRepo(t, ctx)
	contexttest.LoadRepoCommit(t, ctx)

	baseTest(t, ctx)
	noWorkflowsTest(t, ctx)
	withWorkflowsTest(t, ctx)

	noRunnersTest(t, ctx)
	withStackitRunnersNotUsedTest(t, ctx)
}
