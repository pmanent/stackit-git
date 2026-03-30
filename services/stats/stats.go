package stats

import (
	"fmt"
	"github.com/dustin/go-humanize"
	"sync"

	"forgejo.org/modules/base"
	"forgejo.org/modules/setting"
	"forgejo.org/modules/storage"
	"forgejo.org/modules/types"
	"forgejo.org/modules/util/numbers"
)

var (
	cache    sync.Map
	cacheKey = "stats_cache"
)

func GetStats() (types.StatisticData, error) {
	dataInterface, found := cache.Load(cacheKey)
	if found {
		cached := dataInterface.(types.StatisticData)
		return cached, nil
	}
	return types.StatisticData{}, fmt.Errorf("error reading stats from cache")
}

func RefreshStats() {
	var limitDiskStorageSpaceBytes = setting.StackitGit.LimitDiskStorageSpaceBytes
	var limitObjectStorageSpaceBytes = setting.StackitGit.LimitObjectStorageSpaceBytes

	consumedDiskHumanReadable, consumedDiskBytes := storage.GetDiskUsage()
	consumedDiskPercentage := (float64(consumedDiskBytes) / float64(limitDiskStorageSpaceBytes)) * 100.0

	diskSpaceUsage := types.DiskSpaceUsageBytesHuman{
		Bytes:      consumedDiskBytes,
		Human:      consumedDiskHumanReadable,
		LimitHuman: humanize.IBytes(uint64(limitDiskStorageSpaceBytes)),
		Percentage: numbers.RoundUpToTwoSignificantDigits(consumedDiskPercentage),
	}

	bucketSizesBytes, _ := storage.GetMinioDiskUsage()
	minioBucketSizes := make(map[string]map[string]types.DiskSpaceUsageBytesHuman, len(bucketSizesBytes))

	for k, v := range bucketSizesBytes {
		_, exists := minioBucketSizes[k]
		if !exists {
			minioBucketSizes[k] = make(map[string]types.DiskSpaceUsageBytesHuman, len(v))
		}
		for bucketName, bucketValue := range v {
			bucketSizeBytes := int64(bucketValue)
			objectStoragePercentage := (float64(bucketSizeBytes) / float64(limitObjectStorageSpaceBytes)) * 100.0
			minioBucketSizes[k][bucketName] = types.DiskSpaceUsageBytesHuman{
				Bytes:      bucketSizeBytes,
				Human:      base.FileSize(bucketSizeBytes),
				LimitHuman: humanize.IBytes(uint64(limitObjectStorageSpaceBytes)),
				Percentage: numbers.RoundUpToTwoSignificantDigits(objectStoragePercentage),
			}
		}
	}
	objectStorageSpaceUsage := minioBucketSizes

	newStatsData := types.StatisticData{
		ObjectStorageData: objectStorageSpaceUsage,
		DiskStorageData:   diskSpaceUsage,
	}

	cache.Store(cacheKey, newStatsData)
}
