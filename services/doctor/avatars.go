// Copyright 2026 The STACKIT Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package doctor

import (
	"context"
	"errors"
	"os"

	"forgejo.org/models/db"
	repo_model "forgejo.org/models/repo"
	user_model "forgejo.org/models/user"
	"forgejo.org/modules/log"
	"forgejo.org/modules/storage"
	repo_service "forgejo.org/services/repository"

	"xorm.io/builder"
)

// >>> @@@ STACKIT CODE @@@
// User Story 56494
// Avatars written before the known-content-length fix landed in
// modules/avatarstore ended up in the object store as empty objects. They are
// never rewritten on their own: the avatar column still points at them, so
// nothing regenerates them and every request for them fails. This check finds
// them and, with --fix, regenerates the ones that can be regenerated.

// avatarObjectIsUsable reports whether an avatar path holds a non-empty object.
func avatarObjectIsUsable(store storage.ObjectStorage, path string) (bool, error) {
	fi, err := store.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	// a size of 0 -- or of -1, when the backend reports no content length at
	// all -- is an object an unknown-length write left behind
	return fi.Size() > 0, nil
}

func checkEmptyAvatars(ctx context.Context, logger log.Logger, autofix bool) error {
	brokenUsers, fixedUsers, uploadedUsers := 0, 0, 0

	if err := db.Iterate(ctx, builder.Neq{"avatar": ""}, func(ctx context.Context, u *user_model.User) error {
		usable, err := avatarObjectIsUsable(storage.Avatars, u.CustomAvatarRelativePath())
		if err != nil {
			return err
		}
		if usable {
			return nil
		}
		brokenUsers++

		if u.UseCustomAvatar {
			// the uploaded image is gone, only the user can replace it
			uploadedUsers++
			logger.Warn("User %q has an empty uploaded avatar at %q, it has to be uploaded again", u.Name, u.CustomAvatarRelativePath())
			return nil
		}
		if !autofix {
			logger.Warn("User %q has an empty generated avatar at %q", u.Name, u.CustomAvatarRelativePath())
			return nil
		}
		if err := user_model.GenerateRandomAvatar(ctx, u); err != nil {
			logger.Error("Unable to regenerate the avatar of user %q: %v", u.Name, err)
			return nil
		}
		fixedUsers++
		return nil
	}); err != nil {
		logger.Error("Error whilst iterating over the user avatars: %v", err)
		return err
	}

	brokenRepos, fixedRepos := 0, 0

	if err := db.Iterate(ctx, builder.Neq{"avatar": ""}, func(ctx context.Context, r *repo_model.Repository) error {
		usable, err := avatarObjectIsUsable(storage.RepoAvatars, r.CustomAvatarRelativePath())
		if err != nil {
			return err
		}
		if usable {
			return nil
		}
		brokenRepos++

		if !autofix {
			logger.Warn("Repository %q has an empty avatar at %q", r.FullName(), r.CustomAvatarRelativePath())
			return nil
		}
		// the avatar column is the only thing pointing at the empty object, and
		// a repository avatar is either uploaded again or generated on demand
		if err := repo_service.DeleteAvatar(ctx, r); err != nil {
			logger.Error("Unable to drop the empty avatar of repository %q: %v", r.FullName(), err)
			return nil
		}
		fixedRepos++
		return nil
	}); err != nil {
		logger.Error("Error whilst iterating over the repository avatars: %v", err)
		return err
	}

	if brokenUsers == 0 && brokenRepos == 0 {
		logger.Info("No empty avatar found in storage")
		return nil
	}

	if autofix {
		logger.Info("Regenerated %d of %d empty user avatars and dropped %d of %d empty repository avatars", fixedUsers, brokenUsers, fixedRepos, brokenRepos)
		if uploadedUsers > 0 {
			logger.Warn("%d users have to upload their avatar again", uploadedUsers)
		}
		return nil
	}
	logger.Warn("Found %d empty user avatars and %d empty repository avatars, run with --fix to repair them", brokenUsers, brokenRepos)
	return nil
}

func init() {
	Register(&Check{
		Title:                      "Check if there are empty avatars in storage",
		Name:                       "storage-avatars-empty",
		IsDefault:                  false,
		Run:                        checkEmptyAvatars,
		AbortIfFailed:              false,
		SkipDatabaseInitialization: false,
		// storage.Avatars and storage.RepoAvatars are read directly, so the
		// object store has to be brought up before the check runs
		InitStorage: true,
		Priority:    1,
	})
}

// <<< @@@ STACKIT CODE @@@
