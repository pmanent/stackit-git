// Copyright 2026 The STACKIT Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package web

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"forgejo.org/modules/storage"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// >>> @@@ STACKIT CODE @@@
// User Story 56494

const brokenAvatarPath = "146f1430db49566a8e16068bee6ebfae01e1d9e7a7d96775dc11cd2598ea3c10"

var errStorageRead = errors.New("get object: checksum mismatch")

type stubFileInfo struct {
	size int64
}

func (f stubFileInfo) Name() string       { return brokenAvatarPath }
func (f stubFileInfo) Size() int64        { return f.size }
func (f stubFileInfo) Mode() os.FileMode  { return os.ModePerm }
func (f stubFileInfo) ModTime() time.Time { return time.Unix(0, 0) }
func (f stubFileInfo) IsDir() bool        { return false }
func (f stubFileInfo) Sys() any           { return nil }

// stubObject mimics minio's *Object closely enough for this handler: the first
// Read is what actually issues the GET, and a failed Read is remembered so that
// every later Seek returns the same error instead of seeking (api-get-object.go
// stores it in prevErr and Seek returns it). That is the shape that makes
// http.ServeContent report "seeker can't seek".
type stubObject struct {
	readErr error
	prevErr error
	reads   int
}

func (o *stubObject) Read(p []byte) (int, error) {
	o.reads++
	if o.readErr != nil {
		o.prevErr = o.readErr
		return 0, o.readErr
	}
	return 0, io.EOF
}

func (o *stubObject) Seek(int64, int) (int64, error) {
	if o.prevErr != nil {
		return 0, o.prevErr
	}
	return 0, nil
}

func (o *stubObject) Close() error               { return nil }
func (o *stubObject) Stat() (os.FileInfo, error) { return stubFileInfo{-1}, nil }

type stubStorage struct {
	storage.ObjectStorage
	size    int64
	readErr error
	obj     *stubObject
}

func (s *stubStorage) Stat(string) (os.FileInfo, error) {
	return stubFileInfo{s.size}, nil
}

func (s *stubStorage) Open(string) (storage.Object, error) {
	s.obj = &stubObject{readErr: s.readErr}
	return s.obj, nil
}

// TestResizingHandlerUnusableSize covers an object whose backend reports no
// content length at all -- it can never be served, so it must not be attempted.
func TestResizingHandlerUnusableSize(t *testing.T) {
	for _, url := range []string{
		"/avatars/" + brokenAvatarPath + "?size=64",
		"/avatars/" + brokenAvatarPath,
	} {
		t.Run(url, func(t *testing.T) {
			store := &stubStorage{size: -1}
			w := httptest.NewRecorder()
			resizingHandler("avatars", store, []int{64})(w, httptest.NewRequest("GET", url, nil))

			assert.Equal(t, http.StatusNotFound, w.Code)
			assert.NotContains(t, w.Body.String(), "seeker can't seek")
			assert.Nil(t, store.obj, "a sizeless object must never be opened")
		})
	}
}

// TestResizingHandlerStorageReadFailure is the production case: HEAD reports a
// healthy 38.2 KB object, so every Stat-based check passes, but the GET fails.
// The handler must surface that as its own error rather than let ServeContent
// swallow it and report an opaque "seeker can't seek".
func TestResizingHandlerStorageReadFailure(t *testing.T) {
	for _, url := range []string{
		"/avatars/" + brokenAvatarPath + "?size=64",
		"/avatars/" + brokenAvatarPath,
	} {
		t.Run(url, func(t *testing.T) {
			store := &stubStorage{size: 39117, readErr: errStorageRead}
			w := httptest.NewRecorder()
			resizingHandler("avatars", store, []int{64})(w, httptest.NewRequest("GET", url, nil))

			assert.Equal(t, http.StatusInternalServerError, w.Code)
			assert.NotContains(t, w.Body.String(), "seeker can't seek")
			assert.Contains(t, w.Body.String(), "Error whilst reading avatars")
			require.NotNil(t, store.obj)
			assert.Positive(t, store.obj.reads, "the handler must read the object itself")
		})
	}
}

// <<< @@@ STACKIT CODE @@@
