package types

type DiskSpaceUsageBytesHuman struct {
	Bytes      int64
	Human      string
	LimitHuman string
	Percentage float64
}

type StatisticData struct {
	ObjectStorageData ObjectStorageData
	DiskStorageData   DiskSpaceUsageBytesHuman
}
type ObjectStorageData map[string]map[string]DiskSpaceUsageBytesHuman
