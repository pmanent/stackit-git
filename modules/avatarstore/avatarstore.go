// Copyright 2026 the Forgejo authors. All rights reserved.
// SPDX-License-Identifier: MIT

package avatarstore

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/png"
	"os"

	"forgejo.org/modules/avatar"
	"forgejo.org/modules/log"
	"forgejo.org/modules/storage"

	"golang.org/x/image/draw"
)

// >>> @@@ STACKIT CODE @@@
// User Story 56494
// Avatar writes are serialized through an in-memory buffer so the object
// storage always receives a known content length. storage.SaveFrom streams
// through an io.Pipe and passes size -1, which the S3/MinIO backend cannot
// handle reliably for avatars.

// saveSized writes the bytes to the object store with an explicit content length.
func saveSized(imgStore storage.ObjectStorage, path string, data []byte) error {
	_, err := imgStore.Save(path, bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return err
	}
	log.Debug("Saved an image of %d bytes at %s", len(data), path)
	return nil
}

// <<< @@@ STACKIT CODE @@@

// StoreAvatar stores an avatar in an object store and precomputes resized versions of it
func StoreAvatar(avatarPath string, avatarData []byte, avatarImg image.Image, imgStore storage.ObjectStorage) error {
	// >>> @@@ STACKIT CODE @@@ (was storage.SaveFrom, see saveSized)
	if err := saveSized(imgStore, avatarPath, avatarData); err != nil {
		return err
	}
	// <<< @@@ STACKIT CODE @@@
	if avatarImg != nil {
		// pre-compute rescaled versions of the avatar
		return PrecomputeResizedAvatars(imgStore, avatarImg, avatarPath)
	}
	return nil
}

// PrecomputeResizedAvatars computes resized versions of the avatars and stores them in the cache
func PrecomputeResizedAvatars(resizedStore storage.ObjectStorage, avatarImg image.Image, avatarPath string) error {
	for _, size := range avatar.AllowedResizedAvatarSizes {
		rescaled := avatar.Scale(avatarImg, size, size, draw.BiLinear)
		// >>> @@@ STACKIT CODE @@@ (was storage.SaveFrom, see saveSized)
		var buf bytes.Buffer
		if err := png.Encode(&buf, rescaled); err != nil {
			return err
		}
		if err := saveSized(resizedStore, fmt.Sprintf("resized/%d/%s", size, avatarPath), buf.Bytes()); err != nil {
			return err
		}
		// <<< @@@ STACKIT CODE @@@
	}
	return nil
}

// DeleteAvatar removes an avatar file from both the original and resized storage
func DeleteAvatar(avatarPath string, imgStore storage.ObjectStorage) error {
	if err := imgStore.Delete(avatarPath); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("failed to remove %s: %w", avatarPath, err)
		}
		log.Warn("Deleting avatar %s but it doesn't exist", avatarPath)
	}
	for _, size := range avatar.AllowedResizedAvatarSizes {
		err := imgStore.Delete(fmt.Sprintf("resized/%d/%s", size, avatarPath))
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("failed to remove resized avatar resized/%d/%s: %w", size, avatarPath, err)
		}
	}
	return nil
}
