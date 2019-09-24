// Author  <dorzheho@cisco.com>

package common

import (
	"cisco.com/son/apphcd/api/v1/appmanager"
)

// AppManagerAdapter interface
type AppManagerAdapter interface {
	CreateApp(request *appmanager.CreateAppRequest) (*appmanager.Response, error)
	UpgradeApp(request *appmanager.UpgradeAppRequest) (*appmanager.Response, error)
	UpdateApp(request *appmanager.UpdateAppRequest) (*appmanager.Response, error)
	DeleteApp(request *appmanager.DeleteAppRequest) (*appmanager.Response, error)
	DeleteApps(*appmanager.DeleteAppsRequest) (*appmanager.Response, error)
	GetApps(request *appmanager.GetAppsRequest) (*appmanager.Response, error)
	DeleteAppMetadata(request *appmanager.DeleteAppMetadataRequest) (*appmanager.Response, error)
	EnableDisableApp(request *appmanager.EnableDisableAppRequest) (*appmanager.Response, error)
}

// CreateUpgradeRequester interface
type CreateUpgradeUpdateRequester interface {
	GetName() string
	GetAppState() appmanager.AppStateAfterDeployment
	GetVersion() string
	GetFromCatalog() bool
	GetDescription() string
	GetCycle() string
	GetCyclePeriodicAttr() *appmanager.CyclePeriodicReqAttr
	GetAppConfigs() map[string]string
	GetRootGroupId() string
	GetGroupIds() []string
	GetEnvVars() map[string]string
	GetSecrets() map[string]string
	GetLabels() map[string]string
	GetAnnotations() map[string]string
	GetSharedStorage() uint32
	GetSpec() *appmanager.Spec
}

// GenericRequester interface
type GenericRequester interface {
	GetName() string
	GetVersion() string
	GetRootGroupId() string
	GetGroupIds() []string
}

type QuotaRequester interface {
	CreateQuota() error
	UpdateQuota() error
	DeleteQuota() error
}
