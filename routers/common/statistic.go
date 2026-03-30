package common

import (
	"fmt"
	"strings"

	activities_model "forgejo.org/models/activities"
	"forgejo.org/services/context"
)

func getMaxUsageSize(ctx *context.Context) {
	dataStats, _ := activities_model.GetBucketStatistic(ctx)
	diskSpaceUsage, objectStorageSpaceUsage := dataStats.DiskStorageData, dataStats.ObjectStorageData
	currentDiskPercentage := diskSpaceUsage.Percentage
	var maxPercentage float64 = 0

	for _, v1 := range objectStorageSpaceUsage {
		for _, v := range v1 {
			if maxPercentage <= v.Percentage {
				maxPercentage = v.Percentage
			}
		}
	}
	if currentDiskPercentage > maxPercentage {
		maxPercentage = currentDiskPercentage
	}
	ctx.Data["UsedPercentage"] = maxPercentage
	ctx.Data["UsedPercentageValue"] = fmt.Sprintf("%v %%", maxPercentage)
}

func getIfHasAlert(ctx *context.Context) {
	currentUsedPercentage := ctx.Data["UsedPercentage"].(float64)
	ctx.Data["HasAlert"] = currentUsedPercentage > 95
}

func getIfHasWarning(ctx *context.Context) {
	currentUsedPercentage := ctx.Data["UsedPercentage"].(float64)
	ctx.Data["HasWarning"] = currentUsedPercentage > 90
}

func GetStatistics(ctx *context.Context) {
	if strings.HasPrefix(ctx.Req.URL.Path, "/api") || strings.HasPrefix(ctx.Req.URL.Path, "/.well-known") {
		return
	}

	if !ctx.IsSigned {
		return
	}

	accept := ctx.Req.Header.Get("Accept")
	if !strings.Contains(accept, "html") {
		return
	}

	getMaxUsageSize(ctx)
	getIfHasAlert(ctx)
	getIfHasWarning(ctx)
}
