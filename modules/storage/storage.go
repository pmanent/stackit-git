// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/url"
	"os"

	"forgejo.org/modules/log"
	"forgejo.org/modules/setting"
)

// ErrURLNotSupported represents url is not supported
var ErrURLNotSupported = errors.New("url method not supported")

type Type = setting.StorageType

// NewStorageFunc is a function that creates a storage
type NewStorageFunc func(ctx context.Context, cfg *setting.Storage) (ObjectStorage, error)

var (
	storageMap      = map[Type]NewStorageFunc{}
	minioStorageMap = map[string]setting.MinioStorageConfig{}
	MinioStorages   = make(map[string]ObjectStorage)
)

// RegisterStorageType registers a provided storage type with a function to create it
func RegisterStorageType(typ Type, fn func(ctx context.Context, cfg *setting.Storage) (ObjectStorage, error)) {
	storageMap[typ] = fn
}

// Object represents the object on the storage
type Object interface {
	io.ReadCloser
	io.Seeker
	Stat() (os.FileInfo, error)
}

// ObjectStorage represents an object storage to handle a bucket and files
type ObjectStorage interface {
	Open(path string) (Object, error)
	// Save store a object, if size is unknown set -1
	Save(path string, r io.Reader, size int64) (int64, error)
	Stat(path string) (os.FileInfo, error)
	Delete(path string) error
	URL(path, name string, reqParams url.Values) (*url.URL, error)
	IterateObjects(path string, iterator func(path string, obj Object) error) error
}

// Copy copies a file from source ObjectStorage to dest ObjectStorage
func Copy(dstStorage ObjectStorage, dstPath string, srcStorage ObjectStorage, srcPath string) (int64, error) {
	f, err := srcStorage.Open(srcPath)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	size := int64(-1)
	fsinfo, err := f.Stat()
	if err == nil {
		size = fsinfo.Size()
	}

	return dstStorage.Save(dstPath, f, size)
}

// Clean delete all the objects in this storage
func Clean(storage ObjectStorage) error {
	return storage.IterateObjects("", func(path string, obj Object) error {
		_ = obj.Close()
		return storage.Delete(path)
	})
}

// >>> @@@ STACKIT CODE @@@
// User Story 56494
// For small image the process can be serialized

func handleRecover() {
	if r := recover(); r != nil {
		log.Error("recovered from %v", r)
	}
}

func SaveFromDirect(objStorage ObjectStorage, p string, img image.Image) error {
	defer handleRecover()
	var buff bytes.Buffer
	if err := png.Encode(&buff, img); err != nil {
		log.Error("Encode: %v", err)
	}
	log.Debug("Encoded an image of %d bytes", len(buff.Bytes()))
	_, err := objStorage.Save(p, &buff, int64(len(buff.Bytes())))
	log.Debug("Saved an image of %d bytes", len(buff.Bytes()))

	return err
}

// <<< @@@ STACKIT CODE @@@

// SaveFrom saves data to the ObjectStorage with path p from the callback
func SaveFrom(objStorage ObjectStorage, p string, callback func(w io.Writer) error) error {
	pr, pw := io.Pipe()
	defer pr.Close()
	go func() {
		defer pw.Close()
		if err := callback(pw); err != nil {
			_ = pw.CloseWithError(err)
		}
	}()

	_, err := objStorage.Save(p, pr, -1)
	return err
}

var (
	// Attachments represents attachments storage
	Attachments ObjectStorage = UninitializedStorage

	// LFS represents lfs storage
	LFS ObjectStorage = UninitializedStorage

	// Avatars represents user avatars storage
	Avatars ObjectStorage = UninitializedStorage
	// RepoAvatars represents repository avatars storage
	RepoAvatars ObjectStorage = UninitializedStorage

	// RepoArchives represents repository archives storage
	RepoArchives ObjectStorage = UninitializedStorage

	// Packages represents packages storage
	Packages ObjectStorage = UninitializedStorage

	// Actions represents actions storage
	Actions ObjectStorage = UninitializedStorage
	// Actions Artifacts represents actions artifacts storage
	ActionsArtifacts ObjectStorage = UninitializedStorage
)

// Init init the storage
func Init() error {
	minioStorageMap = make(map[string]setting.MinioStorageConfig)
	for _, f := range []func() error{
		initAttachments,
		initAvatars,
		initRepoAvatars,
		initLFS,
		initRepoArchives,
		initPackages,
		initActions,
		initBaseBuckets,
	} {
		if err := f(); err != nil {
			return err
		}
	}

	return nil
}

// NewStorage takes a storage type and some config and returns an ObjectStorage or an error
func NewStorage(typStr Type, cfg *setting.Storage) (ObjectStorage, error) {
	if len(typStr) == 0 {
		typStr = setting.LocalStorageType
	}
	fn, ok := storageMap[typStr]
	if !ok {
		return nil, fmt.Errorf("Unsupported storage type: %s", typStr)
	}

	return fn(context.Background(), cfg)
}

// >>> @@@ STACKIT CODE @@@
func MinioMapper(storage *setting.Storage) {
	if storage.Type == setting.MinioStorageType {
		_, exists := minioStorageMap[storage.MinioConfig.Bucket]
		if !exists {
			minioStorageMap[storage.MinioConfig.Bucket] = storage.MinioConfig
		}
	}
}

// >>> @@@ STACKIT CODE @@@

func initAvatars() (err error) {
	log.Info("Initialising Avatar storage with type: %s", setting.Avatar.Storage.Type)
	Avatars, err = NewStorage(setting.Avatar.Storage.Type, setting.Avatar.Storage)

	// >>> @@@ STACKIT CODE @@@
	MinioMapper(setting.Avatar.Storage)
	// >>> @@@ STACKIT CODE @@@

	return err
}

func initBaseBuckets() (err error) {
	for _, valor := range minioStorageMap {
		bucket, _ := NewStorage(setting.MinioStorageType, &setting.Storage{
			MinioConfig: valor,
			Type:        setting.MinioStorageType,
		})
		MinioStorages[valor.Endpoint] = bucket
	}
	return nil
}

func initAttachments() (err error) {
	if !setting.Attachment.Enabled {
		Attachments = DiscardStorage("Attachment isn't enabled")
		return nil
	}
	log.Info("Initialising Attachment storage with type: %s", setting.Attachment.Storage.Type)
	Attachments, err = NewStorage(setting.Attachment.Storage.Type, setting.Attachment.Storage)
	// >>> @@@ STACKIT CODE @@@
	MinioMapper(setting.Attachment.Storage)
	// >>> @@@ STACKIT CODE @@@

	return err
}

func initLFS() (err error) {
	if !setting.LFS.StartServer {
		LFS = DiscardStorage("LFS isn't enabled")
		return nil
	}
	log.Info("Initialising LFS storage with type: %s", setting.LFS.Storage.Type)
	LFS, err = NewStorage(setting.LFS.Storage.Type, setting.LFS.Storage)

	// >>> @@@ STACKIT CODE @@@
	MinioMapper(setting.LFS.Storage)
	// >>> @@@ STACKIT CODE @@@

	return err
}

func initRepoAvatars() (err error) {
	log.Info("Initialising Repository Avatar storage with type: %s", setting.RepoAvatar.Storage.Type)
	RepoAvatars, err = NewStorage(setting.RepoAvatar.Storage.Type, setting.RepoAvatar.Storage)
	// >>> @@@ STACKIT CODE @@@
	MinioMapper(setting.RepoAvatar.Storage)
	// >>> @@@ STACKIT CODE @@@

	return err
}

func initRepoArchives() (err error) {
	log.Info("Initialising Repository Archive storage with type: %s", setting.RepoArchive.Storage.Type)
	RepoArchives, err = NewStorage(setting.RepoArchive.Storage.Type, setting.RepoArchive.Storage)

	// >>> @@@ STACKIT CODE @@@
	MinioMapper(setting.RepoArchive.Storage)
	// >>> @@@ STACKIT CODE @@@

	return err
}

func initPackages() (err error) {
	if !setting.Packages.Enabled {
		Packages = DiscardStorage("Packages isn't enabled")
		return nil
	}
	log.Info("Initialising Packages storage with type: %s", setting.Packages.Storage.Type)
	Packages, err = NewStorage(setting.Packages.Storage.Type, setting.Packages.Storage)

	// >>> @@@ STACKIT CODE @@@
	MinioMapper(setting.Packages.Storage)
	// >>> @@@ STACKIT CODE @@@

	return err
}

func initActions() (err error) {
	if !setting.Actions.Enabled {
		Actions = DiscardStorage("Actions isn't enabled")
		ActionsArtifacts = DiscardStorage("ActionsArtifacts isn't enabled")
		return nil
	}
	log.Info("Initialising Actions storage with type: %s", setting.Actions.LogStorage.Type)
	if Actions, err = NewStorage(setting.Actions.LogStorage.Type, setting.Actions.LogStorage); err != nil {
		return err
	}
	log.Info("Initialising ActionsArtifacts storage with type: %s", setting.Actions.ArtifactStorage.Type)
	ActionsArtifacts, err = NewStorage(setting.Actions.ArtifactStorage.Type, setting.Actions.ArtifactStorage)

	// >>> @@@ STACKIT CODE @@@
	MinioMapper(setting.Actions.ArtifactStorage)
	// >>> @@@ STACKIT CODE @@@
	return err
}
