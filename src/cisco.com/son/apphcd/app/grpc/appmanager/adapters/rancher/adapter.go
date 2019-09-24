// Author  <dorzheho@cisco.com>

package rancher

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	projectClient "github.com/rancher/types/client/project/v3"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"golang.org/x/sync/errgroup"
	"k8s.io/client-go/kubernetes"

	"cisco.com/son/apphcd/api/v1/appmanager"
	appcommon "cisco.com/son/apphcd/app/common"
	"cisco.com/son/apphcd/app/grpc/appmanager/adapters/rancher/apiclient"
	appmgrcommon "cisco.com/son/apphcd/app/grpc/appmanager/common"
	"cisco.com/son/apphcd/app/grpc/appmanager/common/chartutils"
	"cisco.com/son/apphcd/app/grpc/common/rancher"
	"cisco.com/son/apphcd/app/grpc/common/resourcemgr"
	"cisco.com/son/apphcd/app/grpc/common/syncer"
)

const (
	recreateErrorPrefix = "cannot recreate the application instance"
	timeOutErrorPrefix  = "timed out waiting for pods readiness"
)

type rancherAppMgrAdapter struct {
	mc *rancher.MasterClient
	pc *projectClient.Client
	kc *kubernetes.Clientset
}

// Initialize a new Rancher Adapter:
// - parse Application Controller config file
// - create cache
// - create a new commander
func NewAdapter(mc *rancher.MasterClient, pc *projectClient.Client, kubeClient *kubernetes.Clientset) (*rancherAppMgrAdapter, error) {
	// Set cache path
	path, err := os.Stat(viper.GetString(appcommon.EnvApphcCachePath))
	if os.IsNotExist(err) {
		if err := os.MkdirAll(viper.GetString(appcommon.EnvApphcCachePath), os.FileMode(0755)); err != nil {
			return nil, err
		}
	} else if !path.IsDir() {
		return nil, fmt.Errorf("path %s exists but is not directory", path.Name())
	}

	sysCatalogErrCh := make(chan error)
	appsCatalogErrCh := make(chan error)

	go func(sysCatalogErrCh chan error) {
		targetDir := filepath.Join(viper.GetString(appcommon.EnvApphcCachePath), appmgrcommon.CatalogTemplatesRepo)
		if err := os.RemoveAll(targetDir); err != nil {
			sysCatalogErrCh <- err
		}
		sysCatalogErrCh <- syncer.Clone(viper.GetString(rancher.EnvApphcAdaptersRancherCatalogProto),
			appmgrcommon.CatalogUser, viper.GetString(rancher.EnvApphcAdaptersRancherCatalogPassword),
			viper.GetString(appcommon.EnvApphcGitServerEndpoint), appmgrcommon.CatalogTemplatesRepo,
			targetDir, viper.GetString(rancher.EnvApphcAdaptersRancherTemplatesCatalogBranch))

	}(sysCatalogErrCh)

	go func(appsCatalogErrCh chan error) {
		targetDir := filepath.Join(viper.GetString(appcommon.EnvApphcCachePath), appmgrcommon.CatalogAppsRepo)
		if err := os.RemoveAll(targetDir); err != nil {
			appsCatalogErrCh <- err
		}

		appsCatalogErrCh <- syncer.Clone(viper.GetString(rancher.EnvApphcAdaptersRancherCatalogProto),
			appmgrcommon.CatalogUser, viper.GetString(rancher.EnvApphcAdaptersRancherCatalogPassword),
			viper.GetString(appcommon.EnvApphcGitServerEndpoint),
			appmgrcommon.CatalogAppsRepo, targetDir,
			viper.GetString(rancher.EnvApphcAdaptersRancherAppsCatalogBranch))

	}(appsCatalogErrCh)

	select {
	case err := <-sysCatalogErrCh:
		if err != nil {
			return nil, err
		}
	case err := <-appsCatalogErrCh:
		if err != nil {
			return nil, err
		}
	}

	// Return Rancher AppManager adapter
	return &rancherAppMgrAdapter{mc: mc, pc: pc, kc: kubeClient}, nil
}

// Create an application and related instances
func (adapter *rancherAppMgrAdapter) CreateApp(req *appmanager.CreateAppRequest) (*appmanager.Response, error) {
	// Synchronize cache
	if err := syncCache(); err != nil {
		return appmgrcommon.GenerateResponse(appmanager.Status_ERROR, err.Error(), nil)
	}

	// Refresh catalog
	if err := apiclient.RefreshCatalog(adapter.mc, viper.GetString(rancher.EnvApphcAdaptersRancherAppsCatalogName)); err != nil {
		return appmgrcommon.GenerateResponse(appmanager.Status_ERROR, err.Error(), nil)
	}

	// Construct Application temporary data
	apps := NewAppsData()
	if err := apps.AppendRunningAppsData(adapter.mc, req, req.Cycle, req.AppState); err != nil {
		return appmgrcommon.GenerateResponse(appmanager.Status_ERROR, err.Error(), nil)
	}

	// Identify application type
	chartType, err := appmgrcommon.AppTypeToAppCycle(req.Cycle)
	if err != nil {
		return appmgrcommon.GenerateResponse(appmanager.Status_ERROR, err.Error(), nil)
	}

	// Create the temporary data for the new applications
	apps.AppendNewAppInstances(req, chartType, req.Description, req.AppState)

	// Check if the requested application name and version already exist
	if apps.NewAppInstancesDataEmpty() {
		return appmgrcommon.GenerateResponse(appmanager.Status_NOT_FOUND, "Nothing to deploy", nil)
	}

	namespace := apps.NewAppInstancesData[0].TargetNamespace
	if err := apiclient.CreateNamespace(adapter.mc, namespace); err != nil {
		return appmgrcommon.GenerateResponse(appmanager.Status_ERROR, err.Error(), nil)
	}

	// If shared storage requested
	if req.SharedStorage > 0 {
		kubeConvAppName := namespace
		if err := apiclient.CreateSharedStorage(adapter.mc, req.Name, kubeConvAppName, namespace, int(req.SharedStorage)); err != nil {
			return appmgrcommon.GenerateResponse(appmanager.Status_ERROR, err.Error(), nil)
		}
	}

	// Check if the requested application instances already exist
	runAppInstances := apps.GetRunningAppData(req.Name)

	if runAppInstances != nil {
		// Check if the requested application already exists
		for _, newAppInstance := range apps.NewAppInstancesData {
			for _, runAppInstance := range runAppInstances {
				// We do not allow to spin up multiple versions of a particular application instance
				if newAppInstance.InstanceName == runAppInstance.InstanceName /*&& n.Version == r.Version*/ {
					newAppInstance.NextAction = appmgrcommon.AppInstanceDataNextActionNone
					newAppInstance.TemplateAvailable = true
					continue
				}

				// Check if appropriate template available
				value, err := apiclient.TemplateAvailable(adapter.mc, appcommon.MapGet(newAppInstance.Annotations,
					appmgrcommon.AppInstanceAnnotationTemplateName), newAppInstance.RequestedVersion,
					viper.GetString(rancher.EnvApphcAdaptersRancherAppsCatalogName))
				if err != nil {
					return appmgrcommon.GenerateResponse(appmanager.Status_ERROR, err.Error(), nil)
				}
				newAppInstance.TemplateAvailable = value
			}
		}
	}

	limits := req.GetSpec().GetResources().GetLimits()

	if limits != nil {
		if err := apiclient.CheckAvailableResources(adapter.mc, apps.GetNumberOfNewInstances(), req.Spec.Resources.Limits); err != nil {
			return appmgrcommon.GenerateResponse(appmanager.Status_ERROR, err.Error(), nil)
		}
	}

	// if from_catalog property is not set or set to "false"
	// create a new chart for the application instance
	if !req.FromCatalog {
		createdEntries := 0
		for _, newAppInstance := range apps.NewAppInstancesData {
			if !newAppInstance.TemplateAvailable {
				chartTemplate := filepath.Join(viper.GetString(appcommon.EnvApphcCachePath), appmgrcommon.CatalogNameTemplates, chartType)
				dstChart := filepath.Join(viper.GetString(appcommon.EnvApphcCachePath), appmgrcommon.CatalogAppsRepo,
					req.Name, appcommon.MapGet(newAppInstance.Annotations, appmgrcommon.AppInstanceAnnotationTemplateName),
					newAppInstance.RequestedVersion)
				if err := chartutils.SetChartData(newAppInstance, req, dstChart, nil); err != nil {
					return appmgrcommon.GenerateResponse(appmanager.Status_ERROR, err.Error(), nil)
				}
				if err := chartutils.CreateChart(chartTemplate, dstChart, chartType, req.Name, newAppInstance); err != nil {
					return appmgrcommon.GenerateResponse(appmanager.Status_ERROR, err.Error(), nil)
				}
				createdEntries++
			}
		}

		// Update the chart store only if necessary
		if createdEntries > 0 {
			if err := syncCatalog(adapter.mc, apps.NewAppInstancesData, req.Description); err != nil {
				return appmgrcommon.GenerateResponse(appmanager.Status_ERROR, err.Error(), nil)
			}
		}
	}

	if limits == nil {
		if err := resourcemgr.DeleteLimitRange(adapter.kc, namespace); err != nil {
			return appmgrcommon.GenerateResponse(appmanager.Status_ERROR, err.Error(), nil)
		}

	} else {

		if err := resourcemgr.CreateUpdateLimitRange(adapter.kc, namespace, req.GetSpec().GetResources().GetLimits().GetCpu(),
			req.GetSpec().GetResources().GetLimits().GetMemory(), appmgrcommon.AppInstanceDefaultCpuRequest,
			appmgrcommon.AppInstanceDefaultMemoryRequest); err != nil {
			return appmgrcommon.GenerateResponse(appmanager.Status_ERROR, err.Error(), nil)
		}
	}

	// Create application instance
	doneList, err := createUpgradeApps(adapter.mc, apps.NewAppInstancesData, viper.GetString(rancher.EnvApphcAdaptersRancherAppsCatalogName))
	if err != nil {
		return appmgrcommon.GenerateResponse(appmanager.Status_ERROR, err.Error(), nil)
	}

	// If nothing is added print appropriate message
	if len(doneList) == 0 {
		err := fmt.Errorf("application %s already deployed", req.Name)
		return appmgrcommon.GenerateResponse(appmanager.Status_UNCHANGED, err.Error(), nil)
	}

	var msg string
	if req.GetAppState() == appmanager.AppStateAfterDeployment_enabled {
		msg = "Application deployed successfully"
	} else {
		msg = "Application deployed successfully but disabled"
	}
	// Generate response
	return appmgrcommon.GenerateResponse(appmanager.Status_SUCCESS, msg, &appmanager.App{Name: req.Name,
		Cycle: req.Cycle, Instances: doneList})
}

// Upgrade running application instance
func (adapter *rancherAppMgrAdapter) UpgradeApp(req *appmanager.UpgradeAppRequest) (*appmanager.Response, error) {
	namespace := strings.Replace(strings.ToLower(req.Name),"_","-", -1)
	if err := apiclient.CreateNamespace(adapter.mc, namespace); err != nil {
		return appmgrcommon.GenerateResponse(appmanager.Status_ERROR, err.Error(), nil)
	}

	return adapter.updateUpgradeApps(req, false)
}

// Update running application instance
func (adapter *rancherAppMgrAdapter) UpdateApp(req *appmanager.UpdateAppRequest) (*appmanager.Response, error) {
	return adapter.updateUpgradeApps(req, true)
}

// DeleteApp deletes appropriate Application instance
func (adapter *rancherAppMgrAdapter) DeleteApp(req *appmanager.DeleteAppRequest) (*appmanager.Response, error) {
	// Synchronize cache
	if err := syncCache(); err != nil {
		return appmgrcommon.GenerateResponse(appmanager.Status_ERROR, err.Error(), nil)
	}

	// Refresh catalog
	if err := apiclient.RefreshCatalog(adapter.mc, viper.GetString(rancher.EnvApphcAdaptersRancherAppsCatalogName)); err != nil {
		return appmgrcommon.GenerateResponse(appmanager.Status_ERROR, err.Error(), nil)
	}

	// Create New Apps temporary data
	apps := NewAppsData()

	// Add running applications data
	if err := apps.AppendAppsDataToDelete(adapter.mc, req); err != nil {
		return appmgrcommon.GenerateResponse(appmanager.Status_ERROR, err.Error(), nil)
	}

	// Check if the requested application name and version already exist
	existingData := apps.GetRunningAppData(req.Name)
	if existingData == nil {
		return appmgrcommon.GenerateResponse(appmanager.Status_NOT_FOUND, "Nothing to delete", nil)
	}

	// Set Application type
	appCycle := existingData[0].Annotations.Get(appmgrcommon.AppAnnotationCycle)

	// Set path to repository
	repoPath := filepath.Join(viper.GetString(appcommon.EnvApphcCachePath), appmgrcommon.CatalogAppsRepo)

	// Decide if application chart should be deleted
	// In case global parameter is true or request enforces to delete the chart
	// Controller will delete the application chart
	purge := false
	if req.Purge || viper.GetBool(appcommon.EnvApphcPurgeAppMetadata) {
		purge = true
	}

	// Delete the app and/or related instances
	doneList, err := deleteApps(adapter.mc, existingData, viper.GetString(rancher.EnvApphcAdaptersRancherAppsCatalogName),
		req.Name, repoPath, purge)
	if err != nil {
		return appmgrcommon.GenerateResponse(appmanager.Status_ERROR, err.Error(), nil)
	}

	// If purge flag is set, delete application chart
	if purge {
		if err := syncer.Push(repoPath, "Delete application charts"); err != nil {
			return appmgrcommon.GenerateResponse(appmanager.Status_ERROR, err.Error(), nil)
		}
		if err := apiclient.RefreshCatalog(adapter.mc, viper.GetString(rancher.EnvApphcAdaptersRancherAppsCatalogName)); err != nil {
			return appmgrcommon.GenerateResponse(appmanager.Status_ERROR, err.Error(), nil)
		}
	}

	if len(doneList) == len(existingData) {
		if err := apiclient.DeleteStorage(adapter.mc, req.Name+"-shared-pv", req.Name+"-shared-pvc"); err != nil {
			return appmgrcommon.GenerateResponse(appmanager.Status_ERROR, err.Error(), nil)
		}

		if err := apiclient.DeleteNamespace(adapter.mc, existingData[0].TargetNamespace); err != nil {
			return appmgrcommon.GenerateResponse(appmanager.Status_ERROR, err.Error(), nil)
		}
	}

	// Generate response
	return appmgrcommon.GenerateResponse(appmanager.Status_SUCCESS, "Application instances successfully deleted",
		&appmanager.App{Name: req.Name, Cycle: appCycle, Instances: doneList})
}

// DeleteApps removes all instances of all applications created by controller
func (adapter *rancherAppMgrAdapter) DeleteApps(req *appmanager.DeleteAppsRequest) (*appmanager.Response, error) {
	// Synchronize cache
	if err := syncCache(); err != nil {
		return appmgrcommon.GenerateResponse(appmanager.Status_ERROR, err.Error(), nil)
	}

	// Refresh catalog
	if err := apiclient.RefreshCatalog(adapter.mc, viper.GetString(rancher.EnvApphcAdaptersRancherAppsCatalogName)); err != nil {
		return appmgrcommon.GenerateResponse(appmanager.Status_ERROR, err.Error(), nil)
	}

	// Construct Application temporary data
	apps := NewAppsData()
	if err := apps.AppendAppsDataToDelete(adapter.mc, nil); err != nil {
		return appmgrcommon.GenerateResponse(appmanager.Status_ERROR, err.Error(), nil)
	}

	// cannot find running applications
	if apps.RunningAppsDataEmpty() {
		return appmgrcommon.GenerateResponse(appmanager.Status_NOT_FOUND, "Nothing to delete", nil)
	}

	// Set path to the application repository
	repoPath := filepath.Join(viper.GetString(appcommon.EnvApphcCachePath), appmgrcommon.CatalogAppsRepo)

	doneApps := &appmanager.Apps{}

	// Decide if application chart should be deleted
	// In case global parameter is true or request enforces to delete the chart
	// Controller will delete the application chart
	purge := false
	if req.Purge || viper.GetBool(appcommon.EnvApphcPurgeAppMetadata) {
		purge = true
	}

	var err error
	// Iterate over running applications
	for appName, existingData := range apps.RunningAppsData {
		app := &appmanager.App{}
		app.Name = appName
		app.Cycle = existingData[0].Annotations.Get(appmgrcommon.AppAnnotationCycle)
		// Delete the app and/or related instances
		app.Instances, err = deleteApps(adapter.mc, existingData, viper.GetString(rancher.EnvApphcAdaptersRancherAppsCatalogName),
			appName, repoPath, purge)
		if err != nil {
			return appmgrcommon.GenerateResponse(appmanager.Status_ERROR, err.Error(), nil)
		}

		if len(app.Instances) == len(existingData) {
			if err := apiclient.DeleteStorage(adapter.mc, appName+"-shared-pv", appName+"-shared-pvc"); err != nil {
				return appmgrcommon.GenerateResponse(appmanager.Status_ERROR, err.Error(), nil)
			}

			if err := apiclient.DeleteNamespace(adapter.mc, existingData[0].TargetNamespace); err != nil {
				return appmgrcommon.GenerateResponse(appmanager.Status_ERROR, err.Error(), nil)
			}
		}

		// Add the app to the list
		doneApps.Apps = append(doneApps.Apps, app)
	}

	// If purge flag is set, delete application chart
	if purge {
		if err := syncer.Push(repoPath, "Delete application charts"); err != nil {
			return appmgrcommon.GenerateResponse(appmanager.Status_ERROR, err.Error(), nil)
		}
	}

	// Refresh catalog
	if err := apiclient.RefreshCatalog(adapter.mc, viper.GetString(rancher.EnvApphcAdaptersRancherAppsCatalogName)); err != nil {
		return appmgrcommon.GenerateResponse(appmanager.Status_ERROR, err.Error(), nil)
	}

	// Generate response message
	return appmgrcommon.GenerateResponse(appmanager.Status_SUCCESS, "Applications successfully deleted", doneApps)
}

// GetApps fetches information about running application instances
func (adapter *rancherAppMgrAdapter) GetApps(req *appmanager.GetAppsRequest) (*appmanager.Response, error) {
	wList, err := apiclient.GetApps(adapter.mc, adapter.pc, adapter.kc, req, req.Cycle,
		viper.GetString(rancher.EnvApphcAdaptersRancherAppsCatalogName), req.Verbose)
	if err != nil {
		return appmgrcommon.GenerateResponse(appmanager.Status_ERROR, err.Error(), nil)
	}

	// There is neither workload nor application in the cluster
	if len(wList.Apps) == 0 {
		return appmgrcommon.GenerateResponse(appmanager.Status_NOT_FOUND, "no application found", nil)
	}

	// Generate a response
	return appmgrcommon.GenerateResponse(appmanager.Status_SUCCESS, "Running applications", wList)
}

// DeleteAppMetadata deletes metadata for appropriate application instance
func (adapter *rancherAppMgrAdapter) DeleteAppMetadata(req *appmanager.DeleteAppMetadataRequest) (*appmanager.Response, error) {
	// Synchronize cache
	if err := syncCache(); err != nil {
		return appmgrcommon.GenerateResponse(appmanager.Status_ERROR, err.Error(), nil)
	}

	// Set path to the applications metadata cache
	repoPath := filepath.Join(viper.GetString(appcommon.EnvApphcCachePath), appmgrcommon.CatalogAppsRepo)

	// Set path to the appropriate application metadata cache
	appChartRootDir := filepath.Join(repoPath, req.AppName)

	appTmplts := &appmanager.AppTemplates{}
	appTmplts.AppName = req.AppName

	// In case request doesn't contain grpup_ids,
	// we assume that need to remove metadata for
	// all instances of a particular application
	if len(req.GroupIds) == 0 {
		// Get instances names
		appInstances, err := appmgrcommon.GetSubDirs(appChartRootDir)
		if err != nil {
			return appmgrcommon.GenerateResponse(appmanager.Status_ERROR, err.Error(), nil)
		}
		// Iterate over application instances
		for _, appInstance := range appInstances {
			// Remove metadata for appropriate application instance
			t, err := removeTemplateData(adapter.mc, appChartRootDir,
				viper.GetString(rancher.EnvApphcAdaptersRancherAppsCatalogName), appInstance, req.Version)
			if err != nil {
				return appmgrcommon.GenerateResponse(appmanager.Status_ERROR, err.Error(), nil)
			}
			// Add metadata information
			appTmplts.Templates = append(appTmplts.Templates, t)
		}

		// If application instance version is not sent with request
		if req.GetVersion() == "" {
			// Delete entire application
			if err := chartutils.DeleteChart(appChartRootDir, "all", "all"); err != nil {
				return appmgrcommon.GenerateResponse(appmanager.Status_ERROR, err.Error(), nil)
			}
		}

	} else {
		// In case group_ids data has come with request
		var name string
		for _, gid := range req.GroupIds {
			// Convert to the K8S comply format
			if req.RootGroupId == "" {
				name = req.AppName + "-" + gid
			} else {
				name = req.AppName + "-" + req.RootGroupId + "-" + gid
			}

			chartName := strings.ToLower(strings.Replace(name, "_", "-", -1))
			// Delete metadata
			t, err := removeTemplateData(adapter.mc, appChartRootDir,
				viper.GetString(rancher.EnvApphcAdaptersRancherAppsCatalogName), chartName, req.Version)
			if err != nil {
				return appmgrcommon.GenerateResponse(appmanager.Status_ERROR, err.Error(), nil)
			}
			// Add metadata information
			appTmplts.Templates = append(appTmplts.Templates, t)
		}
	}

	// If cannot find templates for appropriate application instance
	if len(appTmplts.Templates) == 0 {
		return appmgrcommon.GenerateResponse(appmanager.Status_NOT_FOUND, "Nothing to delete", nil)
	}

	// Update chart store
	if err := syncer.Push(repoPath, "Delete application charts"); err != nil {
		return appmgrcommon.GenerateResponse(appmanager.Status_ERROR, err.Error(), nil)
	}

	// Refresh catalog
	if err := apiclient.RefreshCatalog(adapter.mc, viper.GetString(rancher.EnvApphcAdaptersRancherAppsCatalogName)); err != nil {
		return appmgrcommon.GenerateResponse(appmanager.Status_ERROR, err.Error(), nil)
	}

	// Generate response
	return appmgrcommon.GenerateResponse(appmanager.Status_SUCCESS, "Application metadata successfully deleted", appTmplts)
}

func (adapter *rancherAppMgrAdapter) updateUpgradeApps(req appmgrcommon.CreateUpgradeUpdateRequester, reuseValues bool) (*appmanager.Response, error) {
	// Synchronize cache
	if err := syncCache(); err != nil {
		return appmgrcommon.GenerateResponse(appmanager.Status_ERROR, err.Error(), nil)
	}

	// Refresh catalog
	if err := apiclient.RefreshCatalog(adapter.mc, viper.GetString(rancher.EnvApphcAdaptersRancherAppsCatalogName)); err != nil {
		return appmgrcommon.GenerateResponse(appmanager.Status_ERROR, err.Error(), nil)
	}

	// Construct Application temporary data
	apps := NewAppsData()
	if err := apps.AppendRunningAppsData(adapter.mc, req, req.GetCycle(), req.GetAppState()); err != nil {
		return appmgrcommon.GenerateResponse(appmanager.Status_ERROR, err.Error(), nil)
	}

	// Identify application type
	chartType, err := appmgrcommon.AppTypeToAppCycle(req.GetCycle())
	if err != nil {
		return appmgrcommon.GenerateResponse(appmanager.Status_ERROR, err.Error(), nil)
	}

	// Create the temporary data for the new applications
	apps.AppendNewAppInstances(req, chartType, req.GetDescription(), req.GetAppState())

	// Check for instances
	existingData := apps.GetRunningAppData(req.GetName())

	// If we don't have aby information in request and
	// there are no running applications - return with appropriate response
	if apps.NewAppInstancesDataEmpty() && existingData == nil {
		return appmgrcommon.GenerateResponse(appmanager.Status_NOT_FOUND, "Nothing to update/upgrade", nil)
	}

	namespace := strings.Replace(strings.ToLower(req.GetName()),"_","-", -1)
	if err := apiclient.CreateNamespace(adapter.mc, namespace); err != nil {
		return appmgrcommon.GenerateResponse(appmanager.Status_ERROR, err.Error(), nil)
	}

	var bkpAppInstances []*appmgrcommon.AppInstanceData

	// In case nothing has come with request (except the application name and version)
	// check for running instances related to the application
	if apps.NewAppInstancesDataEmpty() {
		for _, existingAppInstance := range existingData {
			apps.NewAppInstancesData = append(apps.NewAppInstancesData, existingAppInstance)
			bkpAppInstance, err := getBackupData(existingAppInstance)
			if err != nil {
				return appmgrcommon.GenerateResponse(appmanager.Status_ERROR, err.Error(), nil)
			}

			bkpAppInstances = append(bkpAppInstances, bkpAppInstance)
		}
		// If we have running app instances as well as the list of apps that came with request is not empty
	} else {
		// Iterate over the new data
		for _, newAppInstance := range apps.NewAppInstancesData {
			// Iterate over existing data
			for _, existingAppInstance := range existingData {
				// If new application instance name equal to
				// the running application instance name
				if newAppInstance.InstanceName == existingAppInstance.InstanceName {
					// if the upgrade policy is set to recreate or
					// the new application instance version equal to
					// the running application instance version
					if viper.GetBool(appcommon.EnvApphcAppsUpgradePolicyRecreate) ||
						existingAppInstance.RequestedVersion == existingAppInstance.CurrentVersion {
						newAppInstance.NextAction = appmgrcommon.AppInstanceDataNextActionRecreate
					} else {
						// Otherwise set to upgrade
						newAppInstance.NextAction = appmgrcommon.AppInstanceDataNextActionUpgrade
					}

					newAppInstance.CurrentVersion = appcommon.MapGet(existingAppInstance.Annotations, appmgrcommon.AppInstanceAnnotationVersion)
					bkpAppInstance, err := getBackupData(existingAppInstance)
					if err != nil {
						return appmgrcommon.GenerateResponse(appmanager.Status_ERROR, err.Error(), nil)
					}

					bkpAppInstances = append(bkpAppInstances, bkpAppInstance)
				}

				if req.GetVersion() == "" {
					newAppInstance.RequestedVersion = existingAppInstance.RequestedVersion
				}

				// Set new application instance storage size
				// to existing application instance storage size
				newAppInstance.InstanceStorageSize = existingAppInstance.InstanceStorageSize
			}
		}
	}

	limits := req.GetSpec().GetResources().GetLimits()
	if limits == nil && reuseValues {
		cpu, memory, err := resourcemgr.GetResourcesLimit(adapter.kc, req.GetName())
		if err != nil {
			return appmgrcommon.GenerateResponse(appmanager.Status_ERROR, err.Error(), nil)
		}

		limits = &appmanager.Spec_Resources_Limits{}
		limits.Cpu = cpu
		limits.Memory = memory
	}

	if limits != nil {
		if err := apiclient.CheckAvailableResources(adapter.mc, apps.GetNumberOfNewInstances(), limits); err != nil {
			return appmgrcommon.GenerateResponse(appmanager.Status_ERROR, err.Error(), nil)
		}
	}

	id, err := uuid.NewRandom()
	if err != nil {
		return appmgrcommon.GenerateResponse(appmanager.Status_ERROR, err.Error(), nil)
	}

	bkpRootDir := "backup-" + id.String()

	defer os.RemoveAll(filepath.Join(viper.GetString(appcommon.EnvApphcCachePath), bkpRootDir))

	//needToRestore := false
	chartApp := filepath.Join(viper.GetString(appcommon.EnvApphcCachePath), appmgrcommon.CatalogAppsRepo, req.GetName())

	// if from_catalog property is not set or set to "false"
	// create a new chart for the application instance
	if !req.GetFromCatalog() {
		chartTemplate := filepath.Join(viper.GetString(appcommon.EnvApphcCachePath),
			appmgrcommon.CatalogTemplatesRepo, chartType)

		var lastGoodKnownMetadataDir string
		var cfg map[string]interface{}
		var state string
		if reuseValues {
			if len(existingData) > 0 {
				lastGoodKnownMetadataDir = filepath.Join(chartApp, appcommon.MapGet(existingData[0].Annotations,
					appmgrcommon.AppInstanceAnnotationTemplateName), existingData[0].CurrentVersion)
				state = appcommon.MapGet(existingData[0].Annotations, appmgrcommon.AppInstanceAnnotationState)
			} else if apps.SampleInstance != nil {
				lastGoodKnownMetadataDir = filepath.Join(chartApp, appcommon.MapGet(apps.SampleInstance.Annotations,
					appmgrcommon.AppInstanceAnnotationTemplateName), apps.SampleInstance.CurrentVersion)
				state = appcommon.MapGet(apps.SampleInstance.Annotations, appmgrcommon.AppInstanceAnnotationState)
			} else {
				return appmgrcommon.GenerateResponse(appmanager.Status_NOT_FOUND, "Nothing to update/upgrade", nil)
			}

			if cfg, err = chartutils.ParseLastGoodConfig(filepath.Join(lastGoodKnownMetadataDir, "values.yaml")); err != nil {
				return appmgrcommon.GenerateResponse(appmanager.Status_ERROR, err.Error(), nil)
			}

			if state != "" {
				a := cfg["annotations"]
				a.(map[interface{}]interface{})[appmgrcommon.AppInstanceAnnotationState] = state
			}
		}

		// Create charts for the new application instances
		for _, newAppInstance := range apps.NewAppInstancesData {
			if err := chartutils.SetChartData(newAppInstance, req, lastGoodKnownMetadataDir, cfg); err != nil {
				return appmgrcommon.GenerateResponse(appmanager.Status_ERROR, err.Error(), nil)
			}

			// Backup old chart
			bkpAppDir := filepath.Join(viper.GetString(appcommon.EnvApphcCachePath), bkpRootDir,
				req.GetName(), appcommon.MapGet(newAppInstance.Annotations, appmgrcommon.AppInstanceAnnotationTemplateName))

			_ = os.RemoveAll(bkpAppDir)

			if err := os.MkdirAll(bkpAppDir, 0755); err != nil {
				return appmgrcommon.GenerateResponse(appmanager.Status_ERROR, err.Error(), nil)
			}

			srcBkpChart := filepath.Join(chartApp, appcommon.MapGet(newAppInstance.Annotations,
				appmgrcommon.AppInstanceAnnotationTemplateName), newAppInstance.CurrentVersion)

			if _, err := os.Stat(srcBkpChart); !os.IsNotExist(err) {
				trgtBkpChart := filepath.Join(bkpAppDir, newAppInstance.CurrentVersion)

				// Copy template for appropriate chart type
				if err := chartutils.CopyChart(srcBkpChart, trgtBkpChart, newAppInstance.InstanceName,
					newAppInstance.CurrentVersion); err != nil {
					return appmgrcommon.GenerateResponse(appmanager.Status_ERROR, err.Error(), nil)
				}
			}

			// Set destination chart
			dstChart := filepath.Join(chartApp, appcommon.MapGet(newAppInstance.Annotations,
				appmgrcommon.AppInstanceAnnotationTemplateName), newAppInstance.RequestedVersion)

			//	newAppInstance.RequestedChartPath = dstChart
			// Remove the chart
			if err := chartutils.DeleteChart(dstChart, newAppInstance.InstanceName, newAppInstance.RequestedVersion); err != nil {
				return appmgrcommon.GenerateResponse(appmanager.Status_ERROR, err.Error(), nil)
			}

			// Create updated chart
			if err := chartutils.CreateChart(chartTemplate, dstChart, chartType, req.GetName(), newAppInstance); err != nil {
				return appmgrcommon.GenerateResponse(appmanager.Status_ERROR, err.Error(), nil)
			}
		}

		if err := syncCatalog(adapter.mc, apps.NewAppInstancesData, req.GetDescription()); err != nil {
			return appmgrcommon.GenerateResponse(appmanager.Status_ERROR, err.Error(), nil)
		}
	}

	if limits == nil && !reuseValues {
		if err := resourcemgr.DeleteLimitRange(adapter.kc, req.GetName()); err != nil {
			return appmgrcommon.GenerateResponse(appmanager.Status_ERROR, err.Error(), nil)
		}

	} else {
		if err := resourcemgr.CreateUpdateLimitRange(adapter.kc, namespace, limits.Cpu, limits.Memory,
			appmgrcommon.AppInstanceDefaultCpuRequest, appmgrcommon.AppInstanceDefaultMemoryRequest); err != nil {
			return appmgrcommon.GenerateResponse(appmanager.Status_ERROR, err.Error(), nil)
		}
	}

	//// If shared storage requested
	kubeConvAppName := apps.NewAppInstancesData[0].TargetNamespace
	if req.GetSharedStorage() > 0 {
		if err := apiclient.CreateSharedStorage(adapter.mc, req.GetName(), kubeConvAppName,
			apps.NewAppInstancesData[0].TargetNamespace, int(req.GetSharedStorage())); err != nil {
			return appmgrcommon.GenerateResponse(appmanager.Status_ERROR, err.Error(), nil)
		}

	} else {

		if err := apiclient.DeleteStorage(adapter.mc, kubeConvAppName+"-shared-pv", kubeConvAppName+"-shared-pvc"); err != nil {
			return appmgrcommon.GenerateResponse(appmanager.Status_ERROR, err.Error(), nil)
		}
	}

	// Create or upgrade instances
	doneList, err := createUpgradeApps(adapter.mc, apps.NewAppInstancesData, viper.GetString(rancher.EnvApphcAdaptersRancherAppsCatalogName))

	if err != nil {
		if !req.GetFromCatalog() && len(bkpAppInstances) > 0 &&
			(strings.HasPrefix(err.Error(), recreateErrorPrefix) || strings.Contains(err.Error(), timeOutErrorPrefix)) {
			for _, appInstance := range bkpAppInstances {
				logrus.WithFields(logrus.Fields{"instance": appInstance.InstanceName,
					"current_version": appInstance.CurrentVersion,
					"target_version":  appInstance.RequestedVersion}).Info("Rolling back the application instance")

				// Backup old chart
				bkpAppDir := filepath.Join(viper.GetString(appcommon.EnvApphcCachePath), bkpRootDir,
					req.GetName(), appcommon.MapGet(appInstance.Annotations, appmgrcommon.AppInstanceAnnotationTemplateName))

				chartToRestoreSrc := filepath.Join(bkpAppDir, appInstance.RequestedVersion)
				chartToRestoreDst := filepath.Join(chartApp, appcommon.MapGet(appInstance.Annotations,
					appmgrcommon.AppInstanceAnnotationTemplateName), appInstance.RequestedVersion)

				// Remove the chart
				if err := chartutils.DeleteChart(chartToRestoreDst, appInstance.InstanceName, appInstance.RequestedVersion); err != nil {
					return appmgrcommon.GenerateResponse(appmanager.Status_ERROR, err.Error(), nil)
				}

				if err := chartutils.CopyChart(chartToRestoreSrc, chartToRestoreDst, appInstance.InstanceName,
					appInstance.RequestedVersion); err != nil {
					return appmgrcommon.GenerateResponse(appmanager.Status_ERROR, err.Error(), nil)
				}
			}

			if err := syncCatalog(adapter.mc, bkpAppInstances, ""); err != nil {
				return appmgrcommon.GenerateResponse(appmanager.Status_ERROR, err.Error(), nil)
			}

		} else {
			return appmgrcommon.GenerateResponse(appmanager.Status_ERROR, err.Error(), nil)
		}

		doneList, err = createUpgradeApps(adapter.mc, bkpAppInstances,
			viper.GetString(rancher.EnvApphcAdaptersRancherAppsCatalogName))
		if err == nil {
			return appmgrcommon.GenerateResponse(appmanager.Status_ERROR, "upgrade failed, the application was rolled back to previous state",
				&appmanager.App{Name: req.GetName(), Cycle: req.GetCycle(), Instances: doneList})
		}

		errMsg := fmt.Sprintf("upgrade and rollback failed: %s", err.Error())
		return appmgrcommon.GenerateResponse(appmanager.Status_ERROR, errMsg, nil)
	}

	var msg string
	//if enableApps {
	msg = "Application updated/upgraded successfully"
	//} else {
	//	msg = "Application updated/upgraded successfully but disabled"
	//}

	// Generate response
	return appmgrcommon.GenerateResponse(appmanager.Status_SUCCESS, msg, &appmanager.App{Name: req.GetName(),
		Cycle: req.GetCycle(), Instances: doneList})
}

func (adapter *rancherAppMgrAdapter) EnableDisableApp(req *appmanager.EnableDisableAppRequest) (*appmanager.Response, error) {

	var state appmanager.AppStateAfterDeployment

	if req.GetDisable() {
		state = appmanager.AppStateAfterDeployment_disabled
	} else {
		state = appmanager.AppStateAfterDeployment_enabled
	}

	apps, err := apiclient.EnableDisableApp(adapter.mc, req, state)
	if err != nil {
		return appmgrcommon.GenerateResponse(appmanager.Status_ERROR, err.Error(), nil)
	}

	var msg string
	if len(apps.Apps) == 0 {
		return appmgrcommon.GenerateResponse(appmanager.Status_NOT_FOUND, "no application found", apps)
	}

	if req.GetDisable() {
		msg = "Application(s) disabled successfully"
	} else {
		msg = "Application(s) enabled successfully"
	}

	return appmgrcommon.GenerateResponse(appmanager.Status_SUCCESS, msg, apps)
}

// removeTemplateData removes application instance metadata
func removeTemplateData(mc *rancher.MasterClient, appChartRootDir, catalogId, instanceName,
	version string) (*appmanager.Template, error) {
	// Get template versions
	versions, err := apiclient.GetTemplateVersions(mc, instanceName)
	if err != nil {
		return nil, err
	}

	t := &appmanager.Template{}
	// Remove all versions if the "version" field is empty
	if version == "" {
		for _, v := range versions {
			t.Versions = append(t.Versions, v.String())
		}
		// Remove explicit version
		if err := chartutils.DeleteChart(filepath.Join(appChartRootDir, instanceName), instanceName, "all"); err != nil {
			return nil, err
		}

	} else {
		// Otherwise iterate over available metadata versions
		for _, v := range versions {
			// If appropriate version found
			if version == v.String() {
				// Remove explicit version
				if err := chartutils.DeleteChart(filepath.Join(appChartRootDir, instanceName, version), instanceName, version); err != nil {
					return nil, err
				}
				// Add information
				t.Versions = append(t.Versions, version)
				break
			}
		}
	}

	// Metadata not found
	if len(t.Versions) == 0 {
		return nil, fmt.Errorf("no metadata found for the instance %s version %s", instanceName, version)
	}

	t.CatalogId = catalogId
	t.Name = instanceName
	return t, nil
}

// waitForCatalogEntry waits until appropriate template will be ready in catalog
func waitForCatalogEntry(mc *rancher.MasterClient, apps []*appmgrcommon.AppInstanceData, appsCatalogId string) error {

	logrus.WithFields(logrus.Fields{"catalog": appsCatalogId}).Info("Synchronizing catalog")

	foundApps := make(map[string]bool)

	for i := 0; i < 20; i++ {
		for _, app := range apps {
			//if !app.TemplateAvailable {
			// Send a request to the catalog
			available, err := apiclient.TemplateAvailable(mc, appcommon.MapGet(app.Annotations,
				appmgrcommon.AppInstanceAnnotationTemplateName),
				app.Annotations.Get(appmgrcommon.AppInstanceAnnotationVersion),
				appsCatalogId)

			if err != nil {
				return err
			}

			// If metadata available
			if available {
				// Set the flag
				app.TemplateAvailable = true
				// Remove from the list
				delete(foundApps, app.InstanceName)
			} else {
				foundApps[app.InstanceName] = false
			}
		}

		// If all templates available
		if len(foundApps) == 0 {
			break
		}

		time.Sleep(1 * time.Second)
	}

	// In case not all metadata available after appropriate amount of time
	if len(foundApps) > 0 {
		return fmt.Errorf("not all application instances available in catalog")
	}

	logrus.WithFields(logrus.Fields{"catalog": appsCatalogId, "status": "OK"}).Info("Synchronizing catalog")

	return nil
}

// generateProtoData generates application properties that will be part of response
func generateProtoData(app *appmgrcommon.AppInstanceData, catalogId string) *appmanager.AppInstance {
	a := &appmanager.AppInstance{}

	// Application instance name
	a.Name = app.InstanceName

	// Application ID: application name + RootGroupID + GroupID fetched from request
	a.Id = app.Annotations.Get(appmgrcommon.AppInstanceAnnotationId)

	// Application instance version
	a.Version = app.RequestedVersion

	// Root Group ID
	a.RootGroupId = app.Annotations.Get(appmgrcommon.AppInstanceAnnotationRootGroupId)

	// Group ID
	a.GroupId = app.Annotations.Get(appmgrcommon.AppInstanceAnnotationGroupId)

	// If catalog ID available
	if catalogId != "" {
		a.Template = &appmanager.Template{}

		// Instance chart name
		a.Template.Name = app.InstanceName

		// Available versions for a particular metadata
		a.Template.Versions = append(a.Template.Versions, app.Annotations.Get(appmgrcommon.AppInstanceAnnotationVersion))

		// Catalog ID
		a.Template.CatalogId = catalogId
	}
	return a
}

func getBackupData(existingAppInstance *appmgrcommon.AppInstanceData) (*appmgrcommon.AppInstanceData, error) {
	// Collect data for backup
	bkp := &appmgrcommon.AppInstanceData{}
	bkp.InstanceName = existingAppInstance.InstanceName
	bkp.Annotations = existingAppInstance.Annotations
	bkp.TemplateAvailable = existingAppInstance.TemplateAvailable
	bkp.Labels = existingAppInstance.Labels
	bkp.CurrentVersion = existingAppInstance.RequestedVersion
	bkp.RequestedVersion = existingAppInstance.CurrentVersion
	bkp.TargetNamespace = existingAppInstance.TargetNamespace

	if viper.GetBool(appcommon.EnvApphcAppsUpgradePolicyRecreate) ||
		existingAppInstance.RequestedVersion == existingAppInstance.CurrentVersion {
		bkp.NextAction = appmgrcommon.AppInstanceDataNextActionRecreate
	} else {
		bkp.NextAction = appmgrcommon.AppInstanceDataNextActionUpgrade
	}

	var err error
	bkp.InstanceStorageSize, err = strconv.Atoi(appcommon.MapGet(existingAppInstance.Annotations, appmgrcommon.AppInstanceAnnotationPersistentVolumeSize))
	if err != nil {
		return nil, err
	}

	bkp.DeleteInstanceStorage = true

	//data.RequestedVersion
	s := appcommon.MapGet(existingAppInstance.Annotations, appmgrcommon.AppInstanceAnnotationState)
	i, err := strconv.Atoi(s)
	if err != nil {
		return nil, err
	}

	bkp.State = appmanager.AppStateAfterDeployment(i)

	return bkp, nil
}

// SyncCache synchronizes local chart cache against remote repository
func syncCache() error {

	// Instantiate the group
	var g errgroup.Group

	// Sync templates cache
	g.Go(func() error {
		repoPath := filepath.Join(viper.GetString(appcommon.EnvApphcCachePath), appmgrcommon.CatalogTemplatesRepo)
		logrus.WithFields(logrus.Fields{"path": repoPath}).Info("Synchronizing cache")
		return syncer.Pull(repoPath, viper.GetString(rancher.EnvApphcAdaptersRancherTemplatesCatalogBranch))
	})

	// Sync applications metadata cache
	g.Go(func() error {
		repoPath := filepath.Join(viper.GetString(appcommon.EnvApphcCachePath), appmgrcommon.CatalogAppsRepo)
		logrus.WithFields(logrus.Fields{"path": repoPath}).Info("Synchronizing cache")
		return syncer.Pull(repoPath, viper.GetString(rancher.EnvApphcAdaptersRancherAppsCatalogBranch))
	})

	// Wait for completion
	return g.Wait()
}

// Synchronize catalog
func syncCatalog(mc *rancher.MasterClient, apps []*appmgrcommon.AppInstanceData, description string) error {
	// Update the chart store
	if err := syncer.Push(filepath.Join(viper.GetString(appcommon.EnvApphcCachePath),
		appmgrcommon.CatalogAppsRepo), description); err != nil {
		return err
	}

	// Refresh catalog
	if err := apiclient.RefreshCatalog(mc, viper.GetString(rancher.EnvApphcAdaptersRancherAppsCatalogName)); err != nil {
		return err
	}

	// Wait until catalog will be updated with the new stuff
	return waitForCatalogEntry(mc, apps, viper.GetString(rancher.EnvApphcAdaptersRancherAppsCatalogName))
}
