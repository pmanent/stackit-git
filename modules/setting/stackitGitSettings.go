package setting

import (
	"fmt"

	"forgejo.org/modules/log"

	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	TRANSPARENTORGID               = "00000000-0000-0000-0000-000000000000"
	DefaultLimitDiskStorageSpace   = "200Gi"
	DefaultLimitObjectStorageSpace = "1Ti"
)

var (
	DefaultLimitDiskStorageSpaceQuantity   = resource.MustParse(DefaultLimitDiskStorageSpace)
	DefaultLimitObjectStorageSpaceQuantity = resource.MustParse(DefaultLimitObjectStorageSpace)
)

// StackitGit settings
var StackitGit = struct {
	// Login Page settings
	EnableUserPassSignIn            bool
	OrganizationID                  string
	ProjectID                       string
	AdminPermissions                string
	InstanceID                      string
	AdminProjects                   string
	LimitDiskStorageSpace           string
	LimitDiskStorageSpaceQuantity   resource.Quantity
	LimitDiskStorageSpaceBytes      int64
	LimitObjectStorageSpace         string
	LimitObjectStorageSpaceQuantity resource.Quantity
	LimitObjectStorageSpaceBytes    int64
}{}

func loadStackitGitSettingsFrom(rootCfg ConfigProvider) {
	sec := rootCfg.Section("stackitgitsettings")
	StackitGit.EnableUserPassSignIn = sec.Key("ENABLE_USER_PASS_SIGNIN").MustBool(false)
	StackitGit.OrganizationID = sec.Key("ORGANIZATIONID").MustString("")
	StackitGit.ProjectID = sec.Key("PROJECTID").MustString("")
	StackitGit.AdminPermissions = sec.Key("ADMIN_PERMISSIONS").MustString("")
	StackitGit.InstanceID = sec.Key("INSTANCEID").MustString("")
	StackitGit.LimitDiskStorageSpace = sec.Key("LIMIT_DISK_STORAGE_SPACE").MustString(DefaultLimitDiskStorageSpace)
	StackitGit.LimitObjectStorageSpace = sec.Key("LIMIT_OBJECT_STORAGE_SPACE").MustString(DefaultLimitObjectStorageSpace)

	// Cache storage limits
	var err error
	if StackitGit.LimitDiskStorageSpaceQuantity, StackitGit.LimitDiskStorageSpaceBytes, err = parseStorageLimit(StackitGit.LimitDiskStorageSpace); err != nil {
		log.Error("failed to parse LIMIT_DISK_STORAGE_SPACE quantity: %v. Using default of '%s' instead.", err, DefaultLimitDiskStorageSpace)
		StackitGit.LimitDiskStorageSpaceQuantity = DefaultLimitDiskStorageSpaceQuantity
		StackitGit.LimitDiskStorageSpaceBytes = DefaultLimitDiskStorageSpaceQuantity.Value()
	}
	if StackitGit.LimitObjectStorageSpaceQuantity, StackitGit.LimitObjectStorageSpaceBytes, err = parseStorageLimit(StackitGit.LimitObjectStorageSpace); err != nil {
		log.Error("failed to parse LIMIT_DISK_STORAGE_SPACE quantity: %v. Using default of '%s' instead.", err, DefaultLimitDiskStorageSpace)
		StackitGit.LimitObjectStorageSpaceQuantity = DefaultLimitObjectStorageSpaceQuantity
		StackitGit.LimitObjectStorageSpaceBytes = DefaultLimitObjectStorageSpaceQuantity.Value()
	}
}

func parseStorageLimit(limit string) (resource.Quantity, int64, error) {
	qty, err := resource.ParseQuantity(limit)
	if err != nil {
		return qty, 0, fmt.Errorf("could not parse quantity '%s': %w", limit, err)
	}
	if qty.Format != resource.BinarySI && qty.Format != resource.DecimalSI {
		return qty, 0, fmt.Errorf("invalid quantity format: '%s'", qty.Format)
	}

	return qty, qty.Value(), nil
}
