// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"bytes"
	"context"
	"crypto/md5"
	"fmt"
	"image/png"
	"strings"

	"forgejo.org/models/avatars"
	"forgejo.org/models/db"
	"forgejo.org/modules/avatar"
	"forgejo.org/modules/avatarstore"
	"forgejo.org/modules/log"
	"forgejo.org/modules/setting"
	"forgejo.org/modules/storage"
)

// CustomAvatarRelativePath returns user custom avatar relative path.
func (u *User) CustomAvatarRelativePath() string {
	return u.Avatar
}

// GenerateRandomAvatar generates a random avatar for user.
func GenerateRandomAvatar(ctx context.Context, u *User) error {
	seed := u.Email
	if len(seed) == 0 {
		seed = u.Name
	}

	img, err := avatar.RandomImage([]byte(seed))
	if err != nil {
		return fmt.Errorf("RandomImage: %w", err)
	}

	u.Avatar = avatars.HashEmail(seed)

	// >>> @@@ STACKIT CODE @@@
	// User Story 56494
	// Random avatars are written through avatarstore so the object store always
	// gets an explicit content length. storage.SaveFrom streams through an
	// io.Pipe and passes size -1, which the S3/MinIO backend stores as an object
	// that reads back empty or without a content length at all (Stat then
	// reports size -1 and the object cannot even be seeked). An avatar that is
	// present but has no usable size is one of those, so treat it as missing and
	// write it again instead of keeping it broken forever.
	fi, err := storage.Avatars.Stat(u.CustomAvatarRelativePath())
	if err != nil || fi.Size() <= 0 {
		// If unable to Stat the avatar file (usually it means non-existing), then try to save a new one
		// Don't share the images so that we can delete them easily
		var buf bytes.Buffer
		if err := png.Encode(&buf, img); err != nil {
			return fmt.Errorf("Encode: %w", err)
		}
		if err := avatarstore.StoreAvatar(u.CustomAvatarRelativePath(), buf.Bytes(), img, storage.Avatars); err != nil {
			return fmt.Errorf("failed to save avatar %s: %w", u.CustomAvatarRelativePath(), err)
		}
	}
	// <<< @@@ STACKIT CODE @@@

	if _, err := db.GetEngine(ctx).ID(u.ID).Cols("avatar").Update(u); err != nil {
		return err
	}

	log.Info("New random avatar created: %d", u.ID)
	return nil
}

// AvatarLinkWithSize returns a link to the user's avatar. It may be a relative
// or absolute URL.
func (u *User) AvatarLinkWithSize(ctx context.Context, size int) string {
	if u.IsGhost() || u.ID <= 0 {
		return avatars.DefaultAvatarLink()
	}

	useLocalAvatar := false
	autoGenerateAvatar := false

	disableGravatar := setting.Config().Picture.DisableGravatar.Value(ctx)

	switch {
	case u.UseCustomAvatar:
		useLocalAvatar = true
	case disableGravatar, setting.OfflineMode:
		useLocalAvatar = true
		autoGenerateAvatar = true
	}

	if useLocalAvatar {
		if u.Avatar == "" && autoGenerateAvatar {
			if err := GenerateRandomAvatar(ctx, u); err != nil {
				log.Error("GenerateRandomAvatar: %v", err)
			}
		}
		if u.Avatar == "" {
			return avatars.DefaultAvatarLink()
		}
		return avatars.GenerateUserResizedAvatarLink(u.Avatar, size)
	}
	return avatars.GenerateEmailAvatarFastLink(ctx, u.AvatarEmail, size)
}

// AvatarLink returns the full avatar link with http host
func (u *User) AvatarLink(ctx context.Context) string {
	link := u.AvatarLinkWithSize(ctx, 0)
	if !strings.HasPrefix(link, "//") && !strings.Contains(link, "://") {
		return setting.AppURL + strings.TrimPrefix(link, setting.AppSubURL+"/")
	}
	return link
}

// IsUploadAvatarChanged returns true if the current user's avatar would be changed with the provided data
func (u *User) IsUploadAvatarChanged(data []byte) bool {
	if !u.UseCustomAvatar || len(u.Avatar) == 0 {
		return true
	}
	avatarID := fmt.Sprintf("%x", md5.Sum(fmt.Appendf(nil, "%d-%x", u.ID, md5.Sum(data))))
	return u.Avatar != avatarID
}

// ExistsWithAvatarAtStoragePath returns true if there is a user with this Avatar
func ExistsWithAvatarAtStoragePath(ctx context.Context, storagePath string) (bool, error) {
	// See func (u *User) CustomAvatarRelativePath()
	// u.Avatar is used directly as the storage path - therefore we can check for existence directly using the path
	return db.GetEngine(ctx).Where("`avatar`=?", storagePath).Exist(new(User))
}
