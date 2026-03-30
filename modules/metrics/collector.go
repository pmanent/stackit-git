// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package metrics

import (
	"runtime"

	activities_model "forgejo.org/models/activities"
	"forgejo.org/models/db"
	"forgejo.org/modules/setting"

	"github.com/prometheus/client_golang/prometheus"
)

const namespace = "gitea_"

// Collector implements the prometheus.Collector interface and
// exposes gitea metrics for prometheus
type Collector struct {
	Accesses    *prometheus.Desc
	Attachments *prometheus.Desc
	BuildInfo   *prometheus.Desc
	Comments    *prometheus.Desc
	// >>> @@@ STACKIT CODE @@@
	DiskSpaceUsage                  *prometheus.Desc
	RunJobsDurationThisMonth        *prometheus.Desc
	RunJobsAverageDurationThisMonth *prometheus.Desc
	RunJobsMaxDurationThisMonth     *prometheus.Desc
	RunJobsCountThisMonth           *prometheus.Desc
	ApiCallsCount                   *prometheus.Desc
	ApiCallsAverageLatency          *prometheus.Desc
	ApiCallsMaxLatency              *prometheus.Desc
	ApiCallsMinLatency              *prometheus.Desc
	ApiCalls1xxStat                 *prometheus.Desc
	ApiCalls2xxStat                 *prometheus.Desc
	ApiCalls3xxStat                 *prometheus.Desc
	ApiCalls4xxStat                 *prometheus.Desc
	ApiCalls5xxStat                 *prometheus.Desc
	ApiCallsUnknownStat             *prometheus.Desc
	// >>> @@@ STACKIT CODE @@@
	Follows            *prometheus.Desc
	HookTasks          *prometheus.Desc
	Issues             *prometheus.Desc
	IssuesOpen         *prometheus.Desc
	IssuesClosed       *prometheus.Desc
	IssuesByLabel      *prometheus.Desc
	IssuesByRepository *prometheus.Desc
	Labels             *prometheus.Desc
	LoginSources       *prometheus.Desc
	Milestones         *prometheus.Desc
	Mirrors            *prometheus.Desc
	Oauths             *prometheus.Desc
	Organizations      *prometheus.Desc
	Projects           *prometheus.Desc
	ProjectColumns     *prometheus.Desc
	PublicKeys         *prometheus.Desc
	Releases           *prometheus.Desc
	Repositories       *prometheus.Desc
	Stars              *prometheus.Desc
	Teams              *prometheus.Desc
	UpdateTasks        *prometheus.Desc
	Users              *prometheus.Desc
	Watches            *prometheus.Desc
	Webhooks           *prometheus.Desc
}

// NewCollector returns a new Collector with all prometheus.Desc initialized
func NewCollector() Collector {
	return Collector{
		Accesses: prometheus.NewDesc(
			namespace+"accesses",
			"Number of Accesses",
			nil, nil,
		),
		Attachments: prometheus.NewDesc(
			namespace+"attachments",
			"Number of Attachments",
			nil, nil,
		),
		BuildInfo: prometheus.NewDesc(
			namespace+"build_info",
			"Build information",
			[]string{
				"goarch",
				"goos",
				"goversion",
				"version",
			}, nil,
		),
		Comments: prometheus.NewDesc(
			namespace+"comments",
			"Number of Comments",
			nil, nil,
		),
		// >>> @@@ STACKIT CODE @@@
		DiskSpaceUsage: prometheus.NewDesc(
			namespace+"disk_space_usage",
			"Disk Space Usage",
			nil, nil,
		),
		RunJobsDurationThisMonth: prometheus.NewDesc(
			namespace+"run_jobs_duration_this_month",
			"Run Jobs Duration This Month",
			nil, nil,
		),
		RunJobsAverageDurationThisMonth: prometheus.NewDesc(
			namespace+"run_jobs_average_duration_this_month",
			"Run Jobs Average Duration This Month",
			nil, nil,
		),
		RunJobsMaxDurationThisMonth: prometheus.NewDesc(
			namespace+"run_jobs_max_duration_this_month",
			"Run Jobs Max Duration This Month",
			nil, nil,
		),
		RunJobsCountThisMonth: prometheus.NewDesc(
			namespace+"run_jobs_count_this_month",
			"Run Jobs Count This Month",
			nil, nil,
		),
		ApiCallsCount: prometheus.NewDesc(
			namespace+"api_calls_count",
			"Number of API Calls",
			nil, nil,
		),
		ApiCallsAverageLatency: prometheus.NewDesc(
			namespace+"api_calls_average_latency",
			"Average Latency of API Calls",
			nil, nil,
		),
		ApiCallsMaxLatency: prometheus.NewDesc(
			namespace+"api_calls_max_latency",
			"Maximum Latency of API Calls",
			nil, nil,
		),
		ApiCallsMinLatency: prometheus.NewDesc(
			namespace+"api_calls_min_latency",
			"Minimum Latency of API Calls",
			nil, nil,
		),
		ApiCalls1xxStat: prometheus.NewDesc(
			namespace+"api_calls_1xx_stat",
			"API Calls 1xx Status",
			nil, nil,
		),
		ApiCalls2xxStat: prometheus.NewDesc(
			namespace+"api_calls_2xx_stat",
			"API Calls 2xx Status",
			nil, nil,
		),
		ApiCalls3xxStat: prometheus.NewDesc(
			namespace+"api_calls_3xx_stat",
			"API Calls 3xx Status",
			nil, nil,
		),
		ApiCalls4xxStat: prometheus.NewDesc(
			namespace+"api_calls_4xx_stat",
			"API Calls 4xx Status",
			nil, nil,
		),
		ApiCalls5xxStat: prometheus.NewDesc(
			namespace+"api_calls_5xx_stat",
			"API Calls 5xx Status",
			nil, nil,
		),
		ApiCallsUnknownStat: prometheus.NewDesc(
			namespace+"api_calls_unknown_stat",
			"API Calls Unknown Status",
			nil, nil,
		),
		// >>> @@@ STACKIT CODE @@@
		Follows: prometheus.NewDesc(
			namespace+"follows",
			"Number of Follows",
			nil, nil,
		),
		HookTasks: prometheus.NewDesc(
			namespace+"hooktasks",
			"Number of HookTasks",
			nil, nil,
		),
		Issues: prometheus.NewDesc(
			namespace+"issues",
			"Number of Issues",
			nil, nil,
		),
		IssuesByLabel: prometheus.NewDesc(
			namespace+"issues_by_label",
			"Number of Issues",
			[]string{"label"}, nil,
		),
		IssuesByRepository: prometheus.NewDesc(
			namespace+"issues_by_repository",
			"Number of Issues",
			[]string{"repository"}, nil,
		),
		IssuesOpen: prometheus.NewDesc(
			namespace+"issues_open",
			"Number of open Issues",
			nil, nil,
		),
		IssuesClosed: prometheus.NewDesc(
			namespace+"issues_closed",
			"Number of closed Issues",
			nil, nil,
		),
		Labels: prometheus.NewDesc(
			namespace+"labels",
			"Number of Labels",
			nil, nil,
		),
		LoginSources: prometheus.NewDesc(
			namespace+"loginsources",
			"Number of LoginSources",
			nil, nil,
		),
		Milestones: prometheus.NewDesc(
			namespace+"milestones",
			"Number of Milestones",
			nil, nil,
		),
		Mirrors: prometheus.NewDesc(
			namespace+"mirrors",
			"Number of Mirrors",
			nil, nil,
		),
		Oauths: prometheus.NewDesc(
			namespace+"oauths",
			"Number of Oauths",
			nil, nil,
		),
		Organizations: prometheus.NewDesc(
			namespace+"organizations",
			"Number of Organizations",
			nil, nil,
		),
		Projects: prometheus.NewDesc(
			namespace+"projects",
			"Number of projects",
			nil, nil,
		),
		ProjectColumns: prometheus.NewDesc(
			namespace+"projects_boards", // TODO: change the key name will affect the consume's result history
			"Number of project columns",
			nil, nil,
		),
		PublicKeys: prometheus.NewDesc(
			namespace+"publickeys",
			"Number of PublicKeys",
			nil, nil,
		),
		Releases: prometheus.NewDesc(
			namespace+"releases",
			"Number of Releases",
			nil, nil,
		),
		Repositories: prometheus.NewDesc(
			namespace+"repositories",
			"Number of Repositories",
			nil, nil,
		),
		Stars: prometheus.NewDesc(
			namespace+"stars",
			"Number of Stars",
			nil, nil,
		),
		Teams: prometheus.NewDesc(
			namespace+"teams",
			"Number of Teams",
			nil, nil,
		),
		UpdateTasks: prometheus.NewDesc(
			namespace+"updatetasks",
			"Number of UpdateTasks",
			nil, nil,
		),
		Users: prometheus.NewDesc(
			namespace+"users",
			"Number of Users",
			nil, nil,
		),
		Watches: prometheus.NewDesc(
			namespace+"watches",
			"Number of Watches",
			nil, nil,
		),
		Webhooks: prometheus.NewDesc(
			namespace+"webhooks",
			"Number of Webhooks",
			nil, nil,
		),
	}
}

// Describe returns all possible prometheus.Desc
func (c Collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.Accesses
	ch <- c.Attachments
	ch <- c.BuildInfo
	ch <- c.Comments
	// >>> @@@ STACKIT CODE @@@
	ch <- c.DiskSpaceUsage
	ch <- c.RunJobsDurationThisMonth
	ch <- c.RunJobsAverageDurationThisMonth
	ch <- c.RunJobsMaxDurationThisMonth
	ch <- c.RunJobsCountThisMonth
	ch <- c.ApiCallsCount
	ch <- c.ApiCallsAverageLatency
	ch <- c.ApiCallsMaxLatency
	ch <- c.ApiCallsMinLatency
	ch <- c.ApiCalls1xxStat
	ch <- c.ApiCalls2xxStat
	ch <- c.ApiCalls3xxStat
	ch <- c.ApiCalls4xxStat
	ch <- c.ApiCalls5xxStat
	ch <- c.ApiCallsUnknownStat
	// >>> @@@ STACKIT CODE @@@
	ch <- c.Follows
	ch <- c.HookTasks
	ch <- c.Issues
	ch <- c.IssuesByLabel
	ch <- c.IssuesByRepository
	ch <- c.IssuesOpen
	ch <- c.IssuesClosed
	ch <- c.Labels
	ch <- c.LoginSources
	ch <- c.Milestones
	ch <- c.Mirrors
	ch <- c.Oauths
	ch <- c.Organizations
	ch <- c.Projects
	ch <- c.ProjectColumns
	ch <- c.PublicKeys
	ch <- c.Releases
	ch <- c.Repositories
	ch <- c.Stars
	ch <- c.Teams
	ch <- c.UpdateTasks
	ch <- c.Users
	ch <- c.Watches
	ch <- c.Webhooks
}

// Collect returns the metrics with values
func (c Collector) Collect(ch chan<- prometheus.Metric) {
	stats := activities_model.GetStatistic(db.DefaultContext)

	ch <- prometheus.MustNewConstMetric(
		c.Accesses,
		prometheus.GaugeValue,
		float64(stats.Counter.Access),
	)
	ch <- prometheus.MustNewConstMetric(
		c.Attachments,
		prometheus.GaugeValue,
		float64(stats.Counter.Attachment),
	)
	ch <- prometheus.MustNewConstMetric(
		c.BuildInfo,
		prometheus.GaugeValue,
		1,
		runtime.GOARCH,
		runtime.GOOS,
		runtime.Version(),
		setting.AppVer,
	)
	ch <- prometheus.MustNewConstMetric(
		c.Comments,
		prometheus.GaugeValue,
		float64(stats.Counter.Comment),
	)
	// >>> @@@ STACKIT CODE @@@
	ch <- prometheus.MustNewConstMetric(
		c.DiskSpaceUsage,
		prometheus.GaugeValue,
		float64(stats.Counter.DiskSpaceUsage.Bytes),
	)
	ch <- prometheus.MustNewConstMetric(
		c.RunJobsDurationThisMonth,
		prometheus.GaugeValue,
		float64(stats.Counter.RunJobsDurationThisMonth),
	)
	ch <- prometheus.MustNewConstMetric(
		c.RunJobsAverageDurationThisMonth,
		prometheus.GaugeValue,
		float64(stats.Counter.RunJobsAverageDurationThisMonth),
	)
	ch <- prometheus.MustNewConstMetric(
		c.RunJobsMaxDurationThisMonth,
		prometheus.GaugeValue,
		float64(stats.Counter.RunJobsMaxDurationThisMonth),
	)
	ch <- prometheus.MustNewConstMetric(
		c.RunJobsCountThisMonth,
		prometheus.GaugeValue,
		float64(stats.Counter.RunJobsCountThisMonth),
	)

	ch <- prometheus.MustNewConstMetric(
		c.ApiCallsCount,
		prometheus.GaugeValue,
		float64(stats.Counter.ApiCallsCount),
	)
	ch <- prometheus.MustNewConstMetric(
		c.ApiCallsAverageLatency,
		prometheus.GaugeValue,
		float64(stats.Counter.ApiCallsAverageLatency),
	)
	ch <- prometheus.MustNewConstMetric(
		c.ApiCallsMaxLatency,
		prometheus.GaugeValue,
		float64(stats.Counter.ApiCallsMaxLatency),
	)
	ch <- prometheus.MustNewConstMetric(
		c.ApiCallsMinLatency,
		prometheus.GaugeValue,
		float64(stats.Counter.ApiCallsMinLatency),
	)
	ch <- prometheus.MustNewConstMetric(
		c.ApiCalls1xxStat,
		prometheus.GaugeValue,
		float64(stats.Counter.ApiCalls1xxStat),
	)
	ch <- prometheus.MustNewConstMetric(
		c.ApiCalls2xxStat,
		prometheus.GaugeValue,
		float64(stats.Counter.ApiCalls2xxStat),
	)
	ch <- prometheus.MustNewConstMetric(
		c.ApiCalls3xxStat,
		prometheus.GaugeValue,
		float64(stats.Counter.ApiCalls3xxStat),
	)
	ch <- prometheus.MustNewConstMetric(
		c.ApiCalls4xxStat,
		prometheus.GaugeValue,
		float64(stats.Counter.ApiCalls4xxStat),
	)
	ch <- prometheus.MustNewConstMetric(
		c.ApiCalls5xxStat,
		prometheus.GaugeValue,
		float64(stats.Counter.ApiCalls5xxStat),
	)
	ch <- prometheus.MustNewConstMetric(
		c.ApiCallsUnknownStat,
		prometheus.GaugeValue,
		float64(stats.Counter.ApiCallsUnknownStat),
	)
	// >>> @@@ STACKIT CODE @@@
	ch <- prometheus.MustNewConstMetric(
		c.Follows,
		prometheus.GaugeValue,
		float64(stats.Counter.Follow),
	)
	ch <- prometheus.MustNewConstMetric(
		c.HookTasks,
		prometheus.GaugeValue,
		float64(stats.Counter.HookTask),
	)
	ch <- prometheus.MustNewConstMetric(
		c.Issues,
		prometheus.GaugeValue,
		float64(stats.Counter.Issue),
	)
	for _, il := range stats.Counter.IssueByLabel {
		ch <- prometheus.MustNewConstMetric(
			c.IssuesByLabel,
			prometheus.GaugeValue,
			float64(il.Count),
			il.Label,
		)
	}
	for _, ir := range stats.Counter.IssueByRepository {
		ch <- prometheus.MustNewConstMetric(
			c.IssuesByRepository,
			prometheus.GaugeValue,
			float64(ir.Count),
			ir.OwnerName+"/"+ir.Repository,
		)
	}
	ch <- prometheus.MustNewConstMetric(
		c.IssuesClosed,
		prometheus.GaugeValue,
		float64(stats.Counter.IssueClosed),
	)
	ch <- prometheus.MustNewConstMetric(
		c.IssuesOpen,
		prometheus.GaugeValue,
		float64(stats.Counter.IssueOpen),
	)
	ch <- prometheus.MustNewConstMetric(
		c.Labels,
		prometheus.GaugeValue,
		float64(stats.Counter.Label),
	)
	ch <- prometheus.MustNewConstMetric(
		c.LoginSources,
		prometheus.GaugeValue,
		float64(stats.Counter.AuthSource),
	)
	ch <- prometheus.MustNewConstMetric(
		c.Milestones,
		prometheus.GaugeValue,
		float64(stats.Counter.Milestone),
	)
	ch <- prometheus.MustNewConstMetric(
		c.Mirrors,
		prometheus.GaugeValue,
		float64(stats.Counter.Mirror),
	)
	ch <- prometheus.MustNewConstMetric(
		c.Oauths,
		prometheus.GaugeValue,
		float64(stats.Counter.Oauth),
	)
	ch <- prometheus.MustNewConstMetric(
		c.Organizations,
		prometheus.GaugeValue,
		float64(stats.Counter.Org),
	)
	ch <- prometheus.MustNewConstMetric(
		c.Projects,
		prometheus.GaugeValue,
		float64(stats.Counter.Project),
	)
	ch <- prometheus.MustNewConstMetric(
		c.ProjectColumns,
		prometheus.GaugeValue,
		float64(stats.Counter.ProjectColumn),
	)
	ch <- prometheus.MustNewConstMetric(
		c.PublicKeys,
		prometheus.GaugeValue,
		float64(stats.Counter.PublicKey),
	)
	ch <- prometheus.MustNewConstMetric(
		c.Releases,
		prometheus.GaugeValue,
		float64(stats.Counter.Release),
	)
	ch <- prometheus.MustNewConstMetric(
		c.Repositories,
		prometheus.GaugeValue,
		float64(stats.Counter.Repo),
	)
	ch <- prometheus.MustNewConstMetric(
		c.Stars,
		prometheus.GaugeValue,
		float64(stats.Counter.Star),
	)
	ch <- prometheus.MustNewConstMetric(
		c.Teams,
		prometheus.GaugeValue,
		float64(stats.Counter.Team),
	)
	ch <- prometheus.MustNewConstMetric(
		c.UpdateTasks,
		prometheus.GaugeValue,
		float64(stats.Counter.UpdateTask),
	)
	ch <- prometheus.MustNewConstMetric(
		c.Users,
		prometheus.GaugeValue,
		float64(stats.Counter.User),
	)
	ch <- prometheus.MustNewConstMetric(
		c.Watches,
		prometheus.GaugeValue,
		float64(stats.Counter.Watch),
	)
	ch <- prometheus.MustNewConstMetric(
		c.Webhooks,
		prometheus.GaugeValue,
		float64(stats.Counter.Webhook),
	)
}
