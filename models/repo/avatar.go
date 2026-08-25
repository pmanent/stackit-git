// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"bytes"
	"context"
	"fmt"
	"image/png"
	"net/url"
	"strings"

	"forgejo.org/models/db"
	"forgejo.org/modules/avatar"
	"forgejo.org/modules/avatarstore"
	"forgejo.org/modules/log"
	"forgejo.org/modules/setting"
	"forgejo.org/modules/storage"
)

// CustomAvatarRelativePath returns repository custom avatar file path.
func (repo *Repository) CustomAvatarRelativePath() string {
	return repo.Avatar
}

// ExistsWithAvatarAtStoragePath returns true if there is a user with this Avatar
func ExistsWithAvatarAtStoragePath(ctx context.Context, storagePath string) (bool, error) {
	// See func (repo *Repository) CustomAvatarRelativePath()
	// repo.Avatar is used directly as the storage path - therefore we can check for existence directly using the path
	return db.GetEngine(ctx).Where("`avatar`=?", storagePath).Exist(new(Repository))
}

// RelAvatarLink returns a relative link to the repository's avatar.
func (repo *Repository) RelAvatarLink(ctx context.Context) string {
	return repo.relAvatarLink(ctx, 0)
}

// generateRandomAvatar generates a random avatar for repository.
func generateRandomAvatar(ctx context.Context, repo *Repository) error {
	idToString := fmt.Sprintf("%d", repo.ID)

	seed := idToString
	img, err := avatar.RandomImage([]byte(seed))
	if err != nil {
		return fmt.Errorf("RandomImage: %w", err)
	}

	repo.Avatar = idToString

	// >>> @@@ STACKIT CODE @@@
	// User Story 56494
	// Was storage.SaveFrom, which streams through an io.Pipe and passes size -1
	// to the object store; the S3/MinIO backend stores that as an empty object.
	// avatarstore serializes the image first so the write carries an explicit
	// content length, and it precomputes the resized variants at the same time.
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return fmt.Errorf("Encode: %w", err)
	}
	if err := avatarstore.StoreAvatar(repo.CustomAvatarRelativePath(), buf.Bytes(), img, storage.RepoAvatars); err != nil {
		// <<< @@@ STACKIT CODE @@@
		return fmt.Errorf("Failed to create dir %s: %w", repo.CustomAvatarRelativePath(), err)
	}

	log.Info("New random avatar created for repository: %d", repo.ID)

	if _, err := db.GetEngine(ctx).ID(repo.ID).Cols("avatar").NoAutoTime().Update(repo); err != nil {
		return err
	}

	return nil
}

func (repo *Repository) relAvatarLink(ctx context.Context, size int) string {
	// If no avatar - path is empty
	avatarPath := repo.CustomAvatarRelativePath()
	if len(avatarPath) == 0 {
		switch mode := setting.RepoAvatar.Fallback; mode {
		case "image":
			return setting.RepoAvatar.FallbackImage
		case "random":
			if err := generateRandomAvatar(ctx, repo); err != nil {
				log.Error("generateRandomAvatar: %v", err)
			}
		default:
			// default behaviour: do not display avatar
			return ""
		}
	}
	cachedSize := avatar.BestAvatarCachedSize(size)
	if cachedSize == 0 {
		return setting.AppSubURL + "/repo-avatars/" + url.PathEscape(repo.Avatar)
	}
	return fmt.Sprintf("%s/repo-avatars/%s?size=%d", setting.AppSubURL, url.PathEscape(repo.Avatar), cachedSize)
}

// AvatarLink returns a link to the repository's avatar.
func (repo *Repository) AvatarLink(ctx context.Context) string {
	return repo.AvatarLinkWithSize(ctx, 0)
}

// Returns URL to the smallest resized version of the avatar bigger than the supplied size
func (repo *Repository) AvatarLinkWithSize(ctx context.Context, size int) string {
	link := repo.relAvatarLink(ctx, size)
	// we only prepend our AppURL to our known (relative, internal) avatar link to get an absolute URL
	if strings.HasPrefix(link, "/") && !strings.HasPrefix(link, "//") {
		return setting.AppURL + strings.TrimPrefix(link, setting.AppSubURL)[1:]
	}
	// otherwise, return the link as it is
	return link
}
