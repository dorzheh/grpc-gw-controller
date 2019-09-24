// Author  <dorzheho@cisco.com>

package common

import (
	"cisco.com/son/apphcd/api/v1/appmanager"
	appcommon "cisco.com/son/apphcd/app/common"
)

type Image struct {
	Repository string // Image repository name
	Name       string // Image name
	Tag        string // Image tag
}

type Port struct {
	Name   string // Port label
	Number uint32 // Port number
	Proto  string // Protocol ("TCP" or "UDP")
}

// AppInstanceData is the data belonging to appropriate application instance
type AppInstanceData struct {
	InstanceName          string                             // Application instance name
	State                 appmanager.AppStateAfterDeployment // Indicates whether application must be enabled after creation/upgrade/update
	TargetNamespace       string                             // Target namespace
	Description           string                             // Description
	CurrentVersion        string                             // Application instance current version
	RequestedVersion      string                             // Version came as a request attribute
	NextAction            AppInstanceDataNextAction          // Is the instance deployed
	TemplateAvailable     bool                               // Is template available
	CyclePeriodicSched    string                             // Cycle Periodic attributes (if applicable)
	Annotations           appcommon.Map                         // Annotations
	Labels                appcommon.Map                         // Labels
	Image                 *Image                             // Docker image fields
	AppConfigs            appcommon.Map                         // Application configuration
	EnvVars               appcommon.Map                         // Application environment variables
	Secrets               appcommon.Map                         // A list of Secret objects
	Ports                 []*Port                            // Ports
	InstanceStorageSize   int                                // Instance storage size in GiB
	SharedStorageEnabled  bool                               // Indicates whether application shared storage is enabled or not
	DeleteInstanceStorage bool                               // Indicates whether the instance storage should be deleted
}
