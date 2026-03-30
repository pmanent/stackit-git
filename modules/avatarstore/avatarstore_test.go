// Copyright 2026 the Forgejo authors. All rights reserved.
// SPDX-License-Identifier: MIT

package avatarstore

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/url"
	"os"
	"testing"

	"forgejo.org/modules/avatar"
	"forgejo.org/modules/storage"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordingStorage records the size argument every Save call receives.
type recordingStorage struct {
	storage.ObjectStorage
	sizes map[string]int64
}

func (s *recordingStorage) Save(path string, r io.Reader, size int64) (int64, error) {
	n, err := io.Copy(io.Discard, r)
	if err != nil {
		return 0, err
	}
	s.sizes[path] = size
	return n, nil
}

func (s *recordingStorage) Delete(string) error              { return nil }
func (s *recordingStorage) Stat(string) (os.FileInfo, error) { return nil, os.ErrNotExist }
func (s *recordingStorage) URL(string, string, url.Values) (*url.URL, error) {
	return nil, nil
}

// >>> @@@ STACKIT CODE @@@
// User Story 56494: avatar writes must reach the object store with an explicit
// content length. storage.SaveFrom streams through an io.Pipe and passes
// size -1, which the S3/MinIO backend cannot handle reliably for avatars.
// This guards the saveSized() indirection in avatarstore.go against being
// reverted to storage.SaveFrom by a future upstream merge.
func TestStoreAvatarUsesKnownContentLength(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.White)
	img.Set(1, 1, color.Black)
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))

	store := &recordingStorage{sizes: map[string]int64{}}
	const avatarPath = "abcdef0123456789"

	require.NoError(t, StoreAvatar(avatarPath, buf.Bytes(), img, store))

	// the original avatar is stored with its real length, not -1
	size, ok := store.sizes[avatarPath]
	require.True(t, ok, "original avatar was not stored")
	assert.Equal(t, int64(buf.Len()), size)

	// and so is every precomputed resized variant
	for _, s := range avatar.AllowedResizedAvatarSizes {
		p := fmt.Sprintf("resized/%d/%s", s, avatarPath)
		size, ok := store.sizes[p]
		require.True(t, ok, "resized avatar %s was not stored", p)
		assert.Positive(t, size, "resized avatar %s stored with unknown size", p)
	}

	for p, size := range store.sizes {
		assert.NotEqual(t, int64(-1), size, "%s was stored with an unknown content length", p)
	}
}

// <<< @@@ STACKIT CODE @@@
