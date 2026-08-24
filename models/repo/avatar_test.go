// Copyright 2026 The STACKIT Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"os"
	"testing"

	"forgejo.org/models/db"
	"forgejo.org/models/unittest"
	"forgejo.org/modules/setting"
	"forgejo.org/modules/storage"
	"forgejo.org/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// >>> @@@ STACKIT CODE @@@
// User Story 56494
// Repository avatars belong in the repository avatar storage and must be
// written with an explicit content length. See modules/avatarstore.
func TestRepoAvatarGenerate(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	userStore, err := storage.NewLocalStorage(t.Context(), &setting.Storage{Path: t.TempDir()})
	require.NoError(t, err)
	repoStore, err := storage.NewLocalStorage(t.Context(), &setting.Storage{Path: t.TempDir()})
	require.NoError(t, err)
	defer test.MockVariableValue(&storage.Avatars, userStore)()
	defer test.MockVariableValue(&storage.RepoAvatars, repoStore)()

	repo := unittest.AssertExistsAndLoadBean(t, &Repository{ID: 1})
	require.NoError(t, generateRandomAvatar(db.DefaultContext, repo))

	// it goes to the repository avatar storage, not to the user avatar storage
	_, err = storage.Avatars.Stat(repo.CustomAvatarRelativePath())
	require.ErrorIs(t, err, os.ErrNotExist)

	fi, err := storage.RepoAvatars.Stat(repo.CustomAvatarRelativePath())
	require.NoError(t, err)
	assert.Positive(t, fi.Size(), "the avatar was written as an empty file")
}

// <<< @@@ STACKIT CODE @@@
