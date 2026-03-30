package version

import (
	"testing"

	version "github.com/hashicorp/go-version"
	"github.com/stretchr/testify/require"
)

func Test_STACKITGit_Version(t *testing.T) {
	// Confirm test is working by checking the version string used by codeberg
	// fetched from https://codeberg.org/api/v1/version 2025-07-29
	_, err := version.NewVersion("12.0.1-48-32cf6330+gitea-1.22.0")
	require.NoError(t, err)

	// STACKIT Git Forgejo compatibility using our Forgejo base version (8.0.0)
	_, err = version.NewVersion("8.0.0-stackit-32cf6330+gitea-1.22.0")
	require.NoError(t, err)

	// STACKIT Git version string using our version string (1.8.0 as of 2025-07-29) and setting the variant
	_, err = version.NewVersion("1.8.0-stackit-32cf6330+gitea-1.22.0")
	require.NoError(t, err)

	// STACKIT Git version string using our version string (1.8.0 as of 2025-07-29) and setting the project name
	_, err = version.NewVersion("stackitgit-1.8.0-32cf6330+gitea-1.22.0")
	require.NoError(t, err)

	// STACKIT Git version just using our version without labeling the distribution
	_, err = version.NewVersion("1.8.0-32cf6330+gitea-1.22.0")
	require.NoError(t, err)
}
