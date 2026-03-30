// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"bytes"
	"image"
	"io"
	"strings"
	"testing"

	"forgejo.org/models/db"
	"forgejo.org/models/unittest"
	"forgejo.org/modules/setting"
	"forgejo.org/modules/storage"
	"forgejo.org/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserAvatarLink(t *testing.T) {
	defer test.MockVariableValue(&setting.AppURL, "https://localhost/")()
	defer test.MockVariableValue(&setting.AppSubURL, "")()

	u := &User{ID: 1, Avatar: "avatar.png"}
	link := u.AvatarLink(db.DefaultContext)
	assert.Equal(t, "https://localhost/avatars/avatar.png", link)

	setting.AppURL = "https://localhost/sub-path/"
	setting.AppSubURL = "/sub-path"
	link = u.AvatarLink(db.DefaultContext)
	assert.Equal(t, "https://localhost/sub-path/avatars/avatar.png", link)
}

func TestUserAvatarLinkWithSize(t *testing.T) {
	u := &User{ID: 1, Avatar: "avatar.png"}
	link := u.AvatarLinkWithSize(db.DefaultContext, 12)
	assert.Equal(t, "/avatars/avatar.png?size=64", link)
	link = u.AvatarLinkWithSize(db.DefaultContext, 2048)
	assert.Equal(t, "/avatars/avatar.png", link)
}

func TestUserAvatarGenerate(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	var err error
	tmpDir := t.TempDir()
	storage.Avatars, err = storage.NewLocalStorage(t.Context(), &setting.Storage{Path: tmpDir})
	require.NoError(t, err)

	u := unittest.AssertExistsAndLoadBean(t, &User{ID: 2})

	// there was no avatar, generate a new one
	assert.Empty(t, u.Avatar)
	err = GenerateRandomAvatar(db.DefaultContext, u)
	require.NoError(t, err)
	assert.NotEmpty(t, u.Avatar)

	// make sure the generated one exists
	oldAvatarPath := u.CustomAvatarRelativePath()
	_, err = storage.Avatars.Stat(u.CustomAvatarRelativePath())
	require.NoError(t, err)
	// and try to change its content
	_, err = storage.Avatars.Save(u.CustomAvatarRelativePath(), strings.NewReader("abcd"), 4)
	require.NoError(t, err)

	// try to generate again
	err = GenerateRandomAvatar(db.DefaultContext, u)
	require.NoError(t, err)
	assert.Equal(t, oldAvatarPath, u.CustomAvatarRelativePath())
	f, err := storage.Avatars.Open(u.CustomAvatarRelativePath())
	require.NoError(t, err)
	defer f.Close()
	content, _ := io.ReadAll(f)
	assert.Equal(t, "abcd", string(content))
}

// >>> @@@ STACKIT CODE @@@
// User Story 56494
// Avatar writes must reach the object store with an explicit content length.
// storage.SaveFrom streams through an io.Pipe and passes size -1, which the
// S3/MinIO backend stores as an empty object and which then fails to render.
// See modules/avatarstore.

// sizeRecordingStorage records the size argument of every Save call.
type sizeRecordingStorage struct {
	storage.ObjectStorage
	sizes map[string]int64
}

func (s *sizeRecordingStorage) Save(path string, r io.Reader, size int64) (int64, error) {
	s.sizes[path] = size
	return s.ObjectStorage.Save(path, r, size)
}

func TestUserAvatarGenerateContentLength(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	local, err := storage.NewLocalStorage(t.Context(), &setting.Storage{Path: t.TempDir()})
	require.NoError(t, err)
	recorder := &sizeRecordingStorage{ObjectStorage: local, sizes: map[string]int64{}}
	var store storage.ObjectStorage = recorder
	defer test.MockVariableValue(&storage.Avatars, store)()

	u := unittest.AssertExistsAndLoadBean(t, &User{ID: 2})
	require.NoError(t, GenerateRandomAvatar(db.DefaultContext, u))

	size, ok := recorder.sizes[u.CustomAvatarRelativePath()]
	require.True(t, ok, "the avatar was not written")
	assert.Positive(t, size, "the avatar was written with an unknown content length")
	for path, size := range recorder.sizes {
		assert.NotEqual(t, int64(-1), size, "%s was written with an unknown content length", path)
	}

	// what landed in the store is a usable image, not an empty file
	f, err := storage.Avatars.Open(u.CustomAvatarRelativePath())
	require.NoError(t, err)
	defer f.Close()
	content, err := io.ReadAll(f)
	require.NoError(t, err)
	_, format, err := image.DecodeConfig(bytes.NewReader(content))
	require.NoError(t, err)
	assert.Equal(t, "png", format)
}

func TestUserAvatarGenerateReplacesEmptyAvatar(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	local, err := storage.NewLocalStorage(t.Context(), &setting.Storage{Path: t.TempDir()})
	require.NoError(t, err)
	defer test.MockVariableValue(&storage.Avatars, local)()

	u := unittest.AssertExistsAndLoadBean(t, &User{ID: 2})
	require.NoError(t, GenerateRandomAvatar(db.DefaultContext, u))

	// an avatar an earlier unknown-length write left empty in the object store
	_, err = storage.Avatars.Save(u.CustomAvatarRelativePath(), strings.NewReader(""), 0)
	require.NoError(t, err)

	require.NoError(t, GenerateRandomAvatar(db.DefaultContext, u))

	fi, err := storage.Avatars.Stat(u.CustomAvatarRelativePath())
	require.NoError(t, err)
	assert.Positive(t, fi.Size(), "the empty avatar was not regenerated")
}

// <<< @@@ STACKIT CODE @@@
