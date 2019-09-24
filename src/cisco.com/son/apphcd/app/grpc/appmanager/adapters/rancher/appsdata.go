// Author  <dorzheho@cisco.com>

package rancher

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"cisco.com/son/apphcd/api/v1/appmanager"
	gcommon "cisco.com/son/apphcd/app/common"
	"cisco.com/son/apphcd/app/grpc/appmanager/common"
	"cisco.com/son/apphcd/app/grpc/common/rancher"
)

// Application temporary data
type AppsData struct {
	NewAppInstancesData []*common.AppInstanceData
	RunningAppsData     map[string][]*common.AppInstanceData
	SampleInstance      *common.AppInstanceData
}

// Create temporary data structure
func NewAppsData() *AppsData {
	d := new(AppsData)
	d.RunningAppsData = make(map[string][]*common.AppInstanceData)
	d.NewAppInstancesData = []*common.AppInstanceData{}
	return d
}

// Create temporary data for a new application instance
func (apps *AppsData) AppendNewAppInstances(req common.GenericRequester, appCycle string,
	description string, state appmanager.AppStateAfterDeployment) {
	// Iterate over Group IDs
	for _, gid := range req.GetGroupIds() {
		// Create annotation map
		a := gcommon.MakeMap()

		// Create labels map
		l := gcommon.MakeMap()

		appName := req.GetName()

		// Add "apphc.app.basename" annotation - required for all instances deployed by controller
		a.Add(common.AppAnnotationBaseName, appName)

		var truncAppName string
		if len(appName) > common.AppNameStringLength {
			truncAppName = appName[0 : common.AppNameStringLength-1]
		} else {
			truncAppName = appName
		}

		var truncAppGroupId string
		if len(gid) > common.AppGroupIdLength {
			truncAppGroupId = gid[common.AppGroupIdLength-1:]
		} else {
			truncAppGroupId = gid
		}

		var appInstanceId string
		var kubeConvName string

		if req.GetRootGroupId() == "" {
			// Instance ID consists of the application name and GroupID
			appInstanceId = strings.Join([]string{appName, gid}, "-")
			//
			kubeConvName = strings.ToLower(strings.Join([]string{truncAppName, truncAppGroupId}, "-"))
		} else {
			// Instance ID consists of the application name, RoodGroupID and GroupID
			appInstanceId = strings.Join([]string{appName, req.GetRootGroupId(), gid}, "-")
			// Add annotation "apphc.app.instance.root_group_id"
			a.Add(common.AppInstanceAnnotationRootGroupId, req.GetRootGroupId())
			// Add label "apphc.app.instance.root_group_id"
			l.Add(common.AppLabelRootGroupId, req.GetRootGroupId())
			//
			// Root Group ID
			rootGroupId := req.GetRootGroupId()
			if len(rootGroupId) > common.AppRootGroupIdLength {
				rootGroupId = req.GetRootGroupId()[0 : common.AppRootGroupIdLength-1]
			}
			//
			kubeConvName = strings.ToLower(strings.Join([]string{truncAppName, rootGroupId, truncAppGroupId}, "-"))
		}

		a.Add(common.AppInstanceAnnotationId, appInstanceId)

		kubeConvName = strings.ToLower(strings.Replace(kubeConvName, "_", "-", -1))

		appInstance := &common.AppInstanceData{}
		appInstance.InstanceName = kubeConvName
		appInstance.TargetNamespace = strings.Replace(strings.ToLower(appName),"_","-", -1)
		appInstance.State = state
		appInstance.TemplateAvailable = false
		appInstance.NextAction = common.AppInstanceDataNextActionCreate
		appInstance.InstanceStorageSize = -1

		// In case description is empty - set default
		if description == "" {
			appInstance.Description = fmt.Sprintf("Application %s, instance ID %s", appName, appInstanceId)
		} else {
			appInstance.Description = description
		}

		appInstance.RequestedVersion = req.GetVersion()

		// Since the list consists of the new app instance,
		// Set it as not deployed
		// Add annotation "apphc.app.cycle"
		a.Add(common.AppAnnotationCycle, appCycle)

		// Add annotation "apphc.app.instance.group_id"
		a.Add(common.AppInstanceAnnotationGroupId, gid)

		// Add annotation "apphc.app.instance.version"
		a.Add(common.AppInstanceAnnotationVersion, appInstance.RequestedVersion)

		// Add annotation "apphc.app.instance.id
		a.Add(common.AppInstanceAnnotationId, appInstanceId)

		// Add annotation "apphc.app.instance.template_name"
		a.Add(common.AppInstanceAnnotationTemplateName, kubeConvName)

		// Add annotations
		appInstance.Annotations = a

		// Add label "apphc.app.cycle"
		l.Add(common.AppLabelCycle, appCycle)

		// Add label "apphc.app.instance.group_id"
		l.Add(common.AppLabelGroupId, gid)

		// Add labels
		appInstance.Labels = l

		// Append the app
		apps.NewAppInstancesData = append(apps.NewAppInstancesData, appInstance)
	}
}

// Append running applications
func (apps *AppsData) AppendRunningAppsData(apiClient *rancher.MasterClient,
	req common.GenericRequester, cycle string, state appmanager.AppStateAfterDeployment) error {
	// Fetch data from the server
	collection, err := apiClient.ProjectClient.App.List(rancher.DefaultListOpts())
	if err != nil {
		return err
	}
	// Iterate over the data
	for _, item := range collection.Data {
		// All the application instances deployed by Controller
		// have appropriate annotation - "apphc.app.basename"
		if appName := gcommon.MapGet(item.Annotations, common.AppAnnotationBaseName); appName != "" {
			// Continue if the annotation value is no equal to the name in the request

			if req.GetName() != "" && req.GetName() != appName {
				continue
			}

			// If the cycle field received with request use it for filtering
			if cycle != "" && cycle != gcommon.MapGet(item.Annotations, common.AppAnnotationCycle) {
				continue
			}

			// If root group ID appears in request use it for filtering
			if req.GetRootGroupId() != "" &&
				req.GetRootGroupId() != gcommon.MapGet(item.Annotations, common.AppInstanceAnnotationRootGroupId) {
				continue
			}

			// Find Group ID
			groupId := gcommon.MapGet(item.Annotations, common.AppInstanceAnnotationGroupId)
			if groupId == "" {
				continue
			}

			var newState appmanager.AppStateAfterDeployment

			if state != appmanager.AppStateAfterDeployment_disabled || state != appmanager.AppStateAfterDeployment_enabled {
				if s := gcommon.MapGet(item.Annotations, common.AppInstanceAnnotationState); s != "" {
					r, err := strconv.Atoi(s)
					if err != nil {
						return err
					}
					newState = appmanager.AppStateAfterDeployment(r)
				}
			} else {
				newState = state
			}

			if apps.SampleInstance == nil {
				apps.SampleInstance = &common.AppInstanceData{}
				apps.SampleInstance.InstanceName = item.Name
				apps.SampleInstance.State = newState
				apps.SampleInstance.CurrentVersion = gcommon.MapGet(item.Annotations, common.AppInstanceAnnotationVersion)
				apps.SampleInstance.RequestedVersion = apps.SampleInstance.CurrentVersion
				apps.SampleInstance.Annotations = item.Annotations
				apps.SampleInstance.TargetNamespace = item.TargetNamespace
			}

			found := false
			if len(req.GetGroupIds()) > 0 {
				// if more that a single Group ID provided , iterate over the list
				// and find a particular Group ID
				for _, gid := range req.GetGroupIds() {
					if gid == groupId {
						found = true
						break
					}
				}

				if !found {
					continue
				}
			}

			// Create the new application instance data
			appInstance := &common.AppInstanceData{}

			// Parse external ID
			externalData, err := getExternalID(item.ExternalID)
			if err != nil {
				return err
			}

			size, err := strconv.Atoi(gcommon.MapGet(item.Annotations, common.AppInstanceAnnotationPersistentVolumeSize))
			if err != nil {
				return err
			}

			appInstance.InstanceStorageSize = size
			appInstance.InstanceName = item.Name
			appInstance.State = newState
			appInstance.Description = item.Description
			appInstance.CurrentVersion = externalData["version"]
			appInstance.TemplateAvailable = true
			appInstance.NextAction = common.AppInstanceDataNextActionUpgrade
			appInstance.TargetNamespace = item.TargetNamespace

			if req.GetVersion() == "" || appInstance.CurrentVersion == req.GetVersion() {
				appInstance.RequestedVersion = appInstance.CurrentVersion
				appInstance.NextAction = common.AppInstanceDataNextActionRecreate
			} else {
				appInstance.RequestedVersion = req.GetVersion()
			}

			appInstance.Annotations = item.Annotations
			appInstance.Labels = item.Labels
			apps.RunningAppsData[appName] = append(apps.RunningAppsData[appName], appInstance)
		}
	}

	return nil
}

// Append all running application instances to the temporary data store
func (apps *AppsData) AppendAppsDataToDelete(apiClient *rancher.MasterClient, req common.GenericRequester) error {
	// Fetch data from the server
	collection, err := apiClient.ProjectClient.App.List(rancher.DefaultListOpts())
	if err != nil {
		return err
	}

	var requestedVersion string

	// Iterate over the data
	for _, item := range collection.Data {
		// All the application instances deployed by Controller
		// have appropriate annotation - "apphc.app.basename"
		if appName := gcommon.MapGet(item.Annotations, common.AppAnnotationBaseName); appName != "" {
			if req != nil {
				// Continue if the annotation value is no equal to the name in the request
				if req.GetName() != "" && req.GetName() != appName {
					continue
				}

				// If version appears in request use it for filtering
				if req.GetVersion() != "" &&
					req.GetVersion() != gcommon.MapGet(item.Annotations, common.AppInstanceAnnotationVersion) {
					continue
				}

				// Set the version to the version came with request
				requestedVersion = req.GetVersion()

				// If root group ID appears in request use it for filtering
				if req.GetRootGroupId() != "" &&
					req.GetRootGroupId() != gcommon.MapGet(item.Annotations, common.AppInstanceAnnotationRootGroupId) {
					continue
				}

				// Find Group ID
				groupId := gcommon.MapGet(item.Annotations, common.AppInstanceAnnotationGroupId)
				if groupId == "" {
					continue
				}

				found := false
				if len(req.GetGroupIds()) > 0 {
					// if more that a single Group ID provided , iterate over the list
					// and find a particular Group ID
					for _, gid := range req.GetGroupIds() {
						if gid == groupId {
							found = true
							break
						}
					}
					if !found {
						continue
					}
				}
			}

			// Create the new application instance data
			appInstance := &common.AppInstanceData{}
			appInstance.InstanceName = item.Name
			appInstance.CurrentVersion = gcommon.MapGet(item.Annotations, common.AppInstanceAnnotationVersion)
			appInstance.RequestedVersion = requestedVersion
			appInstance.TemplateAvailable = true
			appInstance.Annotations = item.Annotations
			appInstance.NextAction = common.AppInstanceDataNextActionDelete
			apps.RunningAppsData[appName] = append(apps.RunningAppsData[appName], appInstance)
		}
	}

	return nil
}

// Get list of running applications
func (apps *AppsData) GetRunningAppsData() map[string][]*common.AppInstanceData {
	return apps.RunningAppsData
}

// Get running application
func (apps *AppsData) GetRunningAppData(appName string) []*common.AppInstanceData {
	if appInstances, ok := apps.RunningAppsData[appName]; ok {
		return appInstances
	}
	return nil
}

// Check whether the list containing appropriate data related to the
// running instances is not empty
func (apps *AppsData) RunningAppsDataNotEmpty() bool {
	return len(apps.RunningAppsData) > 0
}

// Check whether the list containing appropriate data related to the
// running instances is empty
func (apps *AppsData) RunningAppsDataEmpty() bool {
	return len(apps.RunningAppsData) == 0
}

// Check whether the list containing appropriate data related to the
// instances that requested to be created is not empty
func (apps *AppsData) NewAppInstancesDataNotEmpty() bool {
	return len(apps.NewAppInstancesData) > 0
}

// Check whether the list containing appropriate data related to the
// instances that requested to be created is empty
func (apps *AppsData) NewAppInstancesDataEmpty() bool {
	return len(apps.NewAppInstancesData) == 0
}

// Get number of new instances
func (apps *AppsData) GetNumberOfNewInstances() int {
	return len(apps.NewAppInstancesData)
}

// getExternalID gives back a map with the keys catalog, template and version
func getExternalID(e string) (map[string]string, error) {
	// Allocate map for the parsed data
	parsed := make(map[string]string)
	u, err := url.Parse(e)
	if err != nil {
		return parsed, err
	}

	// Query by using the URL
	q := u.Query()

	// Construct map from response
	for key, value := range q {
		if len(value) > 0 {
			parsed[key] = value[0]
		}
	}

	return parsed, nil
}
