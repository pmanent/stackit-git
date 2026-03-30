package storage

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"forgejo.org/modules/base"
	"forgejo.org/modules/log"
)

// >>> @@@ STACKIT CODE @@@

func isDir(dir string) bool {
	f, e := os.Stat(dir)
	if e != nil {
		return false
	}
	return f.IsDir()
}

func GetDiskUsage() (string, int64) {
	// Implement disk usage calculation
	var du int64

	root, err := os.Getwd()
	if err != nil {
		log.Error("Get Working directory: %v", err)
		return "Error", 0
	}
	if !isDir(root) {
		log.Error("Not a directory: %s", root)
		return "Error", 0
	}

	err = filepath.WalkDir(root, func(_ string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			info, err := entry.Info()
			if err != nil {
				return err
			}
			du += info.Size()
		}
		return nil
	})
	if err != nil {
		log.Error("filepath.WalkDir: %v", err)
		return "Error", 0
	}

	return base.FileSize(du), du
}

func GetMinioDiskUsage() (map[string]map[string]uint64, error) {
	minioDiskUsage := make(map[string]map[string]uint64)

	if len(MinioStorages) == 0 {
		return minioDiskUsage, nil
	}

	for msKey, msValue := range MinioStorages {
		currentMinioStorage, ok := msValue.(*MinioStorage)
		if !ok {
			message := "Could not assert ObjectStorage to *MinioStorage"
			fmt.Println(message)
			return minioDiskUsage, errors.New(message)
		}

		dataUsage, err := currentMinioStorage.admin.DataUsageInfo(context.Background())
		if err != nil {
			return minioDiskUsage, err
		}

		minioDiskUsage[msKey] = dataUsage.BucketSizes
	}

	return minioDiskUsage, nil
}

// >>> @@@ STACKIT CODE @@@
