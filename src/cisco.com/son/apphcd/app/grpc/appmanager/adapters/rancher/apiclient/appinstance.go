// Author  <dorzheho@cisco.com>

package apiclient

import (
	"fmt"
	"github.com/rancher/norman/types"
	"math"
	"net/url"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"time"

	gover "github.com/hashicorp/go-version"
	clusterClient "github.com/rancher/types/client/cluster/v3"
	projectClient "github.com/rancher/types/client/project/v3"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"k8s.io/client-go/kubernetes"

	"cisco.com/son/apphcd/api/v1/appmanager"
	appcommon "cisco.com/son/apphcd/app/common"
	appmgrcommon "cisco.com/son/apphcd/app/grpc/appmanager/common"
	grpccommon "cisco.com/son/apphcd/app/grpc/common"
	"cisco.com/son/apphcd/app/grpc/common/rancher"
	"cisco.com/son/apphcd/app/grpc/common/resourcemgr"
)

// ApiCreateApp deploys a new application instance
func CreateAppInstance(mc *rancher.MasterClient, data *appmgrcommon.AppInstanceData, version string) error {
	// Set the template name
	templateName := data.Annotations.Get(appmgrcommon.AppInstanceAnnotationTemplateName)

	logrus.WithFields(logrus.Fields{"instance": data.InstanceName,
		"resourceType": "template", "templateName": templateName}).
		Debug("Looking up for resource")

	// Find the resource
	resource, err := rancher.Lookup(mc, templateName, "template")
	if err != nil {
		return err
	}

	logrus.WithFields(logrus.Fields{"instance": data.InstanceName,
		"resource.ID": resource.ID}).Debug("Searching template by resource ID")

	// Lookup the template by ID
	template, err := mc.ManagementClient.Template.ByID(resource.ID)
	if err != nil {
		return err
	}

	// Get template ID for a particular version
	logrus.WithFields(logrus.Fields{"instance": data.InstanceName,
		"resource.ID": resource.ID}).Debug("Searching template by version")

	templateVersionID := templateVersionIDFromVersionLink(template.VersionLinks[template.DefaultVersion])
	if version != "" {
		if link, ok := template.VersionLinks[version]; ok {
			templateVersionID = templateVersionIDFromVersionLink(link)
		} else {
			return fmt.Errorf("version %s for template %s is invalid", version, templateName)
		}
	}

	// Get template version
	logrus.WithFields(logrus.Fields{"instance": data.InstanceName,
		"ID": resource.ID}).Debug("Searching template by ID")
	templateVersion, err := mc.ManagementClient.TemplateVersion.ByID(templateVersionID)
	if err != nil {
		return err
	}
	opts := rancher.DefaultListOpts()

	// Create persistent volume if requested
	if data.InstanceStorageSize > 0 {
		volumeName := data.InstanceName + "-pv"
		opts.Filters["name"] = volumeName

		pvId, err := cleanUpVolume(mc, opts)
		if err != nil {
			return err
		}

		if pvId == "" {
			logrus.WithFields(logrus.Fields{"instance": data.InstanceName, "volume": volumeName}).Info("Creating persistent volume for instance")

			// Configure the volume
			newPv := &clusterClient.PersistentVolume{}
			newPv.Annotations = data.Annotations
			newPv.Labels = data.Labels
			newPv.HostPath = &clusterClient.HostPathVolumeSource{}
			newPv.HostPath.Path = filepath.Join(appcommon.AppdataPath, data.Annotations.Get(appmgrcommon.AppAnnotationBaseName), data.InstanceName)
			newPv.Capacity = make(map[string]string)
			newPv.Capacity["storage"] = fmt.Sprintf("%dGi", data.InstanceStorageSize)
			newPv.AccessModes = []string{"ReadWriteOnce"}
			newPv.Name = volumeName
			newPv.PersistentVolumeReclaimPolicy = "Retain"

			if _, err := mc.ClusterClient.PersistentVolume.Create(newPv); err != nil {
				return err
			}

			logrus.WithFields(logrus.Fields{"instance": newPv.Name, "volume": newPv.Name, "status": "OK"}).
				Info("Creating persistent volume for instance")

		}

		// Find appropriate PVC
		claimName := data.InstanceName + "-pvc"
		opts.Filters["name"] = claimName
		pvc, err := mc.ProjectClient.PersistentVolumeClaim.List(opts)
		if err != nil {
			return err
		}

		// If no PVC found , create a new one
		if len(pvc.Data) == 0 {
			newPvc := &projectClient.PersistentVolumeClaim{}
			newPvc.Labels = data.Labels
			newPvc.Annotations = data.Annotations
			newPvc.Name = data.InstanceName + "-pvc"
			newPvc.NamespaceId = data.TargetNamespace
			newPvc.AccessModes = []string{"ReadWriteOnce"}
			newPvc.Resources = &projectClient.ResourceRequirements{}
			newPvc.Resources.Requests = make(map[string]string)
			newPvc.Resources.Requests["storage"] = fmt.Sprintf("%dGi", data.InstanceStorageSize)
			newPvc.Selector = &projectClient.LabelSelector{}
			newPvc.Selector.MatchLabels = make(map[string]string)
			newPvc.Selector.MatchLabels["name"] = data.InstanceName
			newPvc.VolumeID = volumeName

			logrus.WithFields(logrus.Fields{"instance": newPvc.Name, "claim": newPvc.Name}).
				Info("Creating persistent volume claim")

			if _, err := mc.ProjectClient.PersistentVolumeClaim.Create(newPvc); err != nil {
				return err
			}

			logrus.WithFields(logrus.Fields{"instance": newPvc.Name, "claim": newPvc.Name, "status": "OK"}).
				Info("Creating persistent volume claim")

			// Wait till the PV and PVC will be in the "Bound" state
			opts.Filters["name"] = volumeName
			if err := waitForVolumeState(mc, opts, "bound"); err != nil {
				return err
			}
		}
	}

	// Add application instance state
	appcommon.MapAdd(data.Annotations, appmgrcommon.AppInstanceAnnotationState, fmt.Sprintf("%d", data.State))

	// Construct application data
	app := &projectClient.App{
		Name:            data.InstanceName,
		Description:     data.Description,
		Labels:          data.Labels,
		Annotations:     data.Annotations,
		TargetNamespace: data.TargetNamespace,
		ExternalID:      templateVersion.ExternalID,
		Notes:           "",
	}

	logrus.WithFields(logrus.Fields{"instance": data.InstanceName, "version": version}).Info("Creating application")

	// Create the application instance
	createdApp, err := mc.ProjectClient.App.Create(app)
	if err != nil {
		return err
	}

	// Wait for the app's notes to be populated so we can print them
	var timeout int
	for len(createdApp.Notes) == 0 {
		if timeout == 60 {
			return fmt.Errorf("timed out waiting for application")
		}
		timeout++
		time.Sleep(2 * time.Second)
		createdApp, err = mc.ProjectClient.App.ByID(createdApp.ID)
		if err != nil {
			return err
		}
	}

	if err := postDeploymentAction(mc, data.InstanceName, version, data.State); err != nil {
		return err
	}

	logrus.WithFields(logrus.Fields{"instance": data.InstanceName, "version": version,
		"status": "OK"}).Info("Creating application")

	return nil
}

// ApiUpgradeApp upgrades appropriate application instance
func UpgradeAppInstance(mc *rancher.MasterClient, data *appmgrcommon.AppInstanceData) error {
	// Find the resource
	resource, err := rancher.Lookup(mc, data.InstanceName, "app")
	if err != nil {
		return err
	}

	// Find application by ID
	app, err := mc.ProjectClient.App.ByID(resource.ID)
	if err != nil {
		return err
	}

	// Parse external ID
	u, err := url.Parse(app.ExternalID)
	if err != nil {
		return err
	}

	// Parse query and return appropriate value
	q := u.Query()

	// Set version for the new application
	q.Set("version", data.RequestedVersion)

	// Configure filter
	filter := rancher.DefaultListOpts()
	filter.Filters["externalId"] = "catalog://?" + q.Encode()

	// Get the template
	template, err := mc.ManagementClient.TemplateVersion.List(filter)
	if err != nil {
		return err
	}

	// In case metadata is unavailable
	if len(template.Data) == 0 {
		return fmt.Errorf("version %s not valid for app", data.RequestedVersion)
	}

	// Create configuration for the upgrade
	config := &projectClient.AppUpgradeConfig{
		ExternalID: template.Data[0].ExternalID,
	}

	// Set labels
	app.Labels = data.Labels

	// Set annotations
	app.Annotations = data.Annotations

	// Add application instance state
	appcommon.MapAdd(app.Annotations, appmgrcommon.AppInstanceAnnotationState, fmt.Sprintf("%d", data.State))

	// Update application object
	if app, err = mc.ProjectClient.App.Replace(app); err != nil {
		return err
	}

	logrus.WithFields(logrus.Fields{"instance": data.InstanceName, "version": data.RequestedVersion}).
		Info("Upgrading application")

	// Upgrade application instance
	if err := mc.ProjectClient.App.ActionUpgrade(app, config); err != nil {
		return err
	}

	logrus.WithFields(logrus.Fields{"instance": data.InstanceName, "version": data.RequestedVersion,
		"status": "OK"}).Info("Upgrading application")

	// If persistent volume requested
	if data.InstanceStorageSize > 0 {
		// Wait till Persistent Volume and its Claim will be available and bound
		opts := rancher.DefaultListOpts()
		opts.Filters["name"] = data.InstanceName + "-pv"
		if err := waitForVolumeState(mc, opts, "bound"); err != nil {
			return err
		}
	}

	if err := postDeploymentAction(mc, data.InstanceName,
		appcommon.MapGet(data.Annotations, appmgrcommon.AppInstanceAnnotationVersion), data.State); err != nil {
		return err
	}

	return nil
}

// ApiDeleteApp deletes appropriate application instance
func DeleteAppInstance(mc *rancher.MasterClient, data *appmgrcommon.AppInstanceData, version string) error {

	wResource, _ := rancher.Lookup(mc, data.InstanceName, "workload")

	if wResource != nil {
		appInstance, err := mc.ProjectClient.Workload.ByID(wResource.ID)
		if err != nil {
			return err
		}

		if err := mc.ProjectClient.Workload.Delete(appInstance); err != nil {
			return err
		}
	}

	// Find the resource
	resource, err := rancher.Lookup(mc, data.InstanceName, "app")
	if resource == nil {
		return nil
	}

	if err != nil {
		return err
	}

	// Lookup application instance by resource ID
	appInstance, err := mc.ProjectClient.App.ByID(resource.ID)
	if err != nil {
		return err
	}

	// Delete application instance
	logrus.WithFields(logrus.Fields{"instance": data.InstanceName,
		"version": version}).Info("Deleting application")

	if err := mc.ProjectClient.App.Delete(appInstance); err != nil {
		return err
	}

	startTime := time.Now()
	for {
		if time.Since(startTime)/time.Second > 30 {
			return fmt.Errorf("timed out waiting for deleting application instance %s-%s", data.InstanceName, version)
		}
		a, _ := mc.ProjectClient.App.ByID(resource.ID)
		if a.ID == "" {
			break
		}

		time.Sleep(500 * time.Millisecond)
	}

	logrus.WithFields(logrus.Fields{"instance": data.InstanceName, "version": version, "status": "OK"}).Info("Deleting application")

	if data.DeleteInstanceStorage {
		volumeName := data.InstanceName + "-pv"
		claimName := data.InstanceName + "-pvc"
		if err := DeleteStorage(mc, volumeName, claimName); err != nil {
			return err
		}
	}

	return nil
}

// ApiGetApps shows information about running applications and their instances
func GetApps(mc *rancher.MasterClient, pc *projectClient.Client, kc *kubernetes.Clientset, req appmgrcommon.GenericRequester,
	appCycle, catalogId string, verbose bool) (*appmanager.AppsInfo, error) {

	// Identify application type
	appType, _ := appmgrcommon.AppCycleToAppType(appCycle)

	// Get workloads
	apps, err := mc.ProjectClient.App.List(rancher.DefaultListOpts())
	if nil != err {
		return nil, err
	}

	// Create empty body
	body := &appmanager.AppsInfo{}
	body.Apps = make(map[string]*appmanager.AppInfo)

	var monEndpoint string
	var logEndpoint string
	if len(apps.Data) > 0 {
		// Discover monitoring endpoint
		monEndpoint, err = rancher.GetEndpoint(pc, mc.ManagementClient, grpccommon.ServiceMonitoringNamespace,
			grpccommon.ServiceMonitoringName, viper.GetString(appcommon.EnvApphSvcsUrlExternalIp))
		if err != nil {
			return nil, err
		}

		// Discover logging endpoint
		logEndpoint, err = rancher.GetEndpoint(pc, mc.ManagementClient, grpccommon.ServiceLoggingNamespace,
			grpccommon.ServiceLoggingeName, viper.GetString(appcommon.EnvApphSvcsUrlExternalIp))
		if err != nil {
			return nil, err
		}
	}

	limits := &appmanager.Resources_Limits{}
	var requests *appmanager.Resources_Requests

	// Iterate over workload data
	for _, item := range apps.Data {
		if appName := appcommon.MapGet(item.Annotations, appmgrcommon.AppAnnotationBaseName); appName != "" {
			// Continue if the annotation value is no equal to the name in the request
			if req.GetName() != "" && req.GetName() != appName {
				continue
			}

			if req.GetRootGroupId() != "" && req.GetRootGroupId() != appcommon.MapGet(item.Annotations,
				appmgrcommon.AppInstanceAnnotationRootGroupId) {
				continue
			}

			// Find Group ID
			appAnnotationGroupId := appcommon.MapGet(item.Annotations, appmgrcommon.AppInstanceAnnotationGroupId)
			if appAnnotationGroupId == "" {
				continue
			}

			// If application type appears in request , filter according to the type
			appAnnotationsCycle := appcommon.MapGet(item.Annotations, appmgrcommon.AppAnnotationCycle)
			if appType != "" && appType != appAnnotationsCycle {
				continue
			}

			found := false
			if len(req.GetGroupIds()) > 0 {
				// if more that a single Group ID provided , iterate over the list
				// and find a particular Group ID
				for _, gid := range req.GetGroupIds() {
					if gid == appAnnotationGroupId {
						found = true
						break
					}
				}
				if !found {
					continue
				}
			}

			// If version comes in request , get application instance version
			version := appcommon.MapGet(item.Annotations, appmgrcommon.AppInstanceAnnotationVersion)
			if req.GetVersion() != "" && req.GetVersion() != version {
				continue
			}

			if body.Apps[appName] == nil {
				limits.Cpu, limits.Memory, err = resourcemgr.GetResourcesLimit(kc, item.TargetNamespace)
				if err != nil {
					return nil, err
				}

				if limits != nil {
					requests = &appmanager.Resources_Requests{}
					requests.Cpu, err = strconv.ParseFloat(appmgrcommon.AppInstanceDefaultCpuRequest, 64)
					if err != nil {
						return nil, err
					}

					requests.Memory, err = resourcemgr.ParseMemoryString(appmgrcommon.AppInstanceDefaultMemoryRequest)
					if err != nil {
						return nil, err
					}
				}

				ai := &appmanager.AppInfo{}
				if appAnnotationsCycle == appmgrcommon.TypePeriodic {
					if sched := appcommon.MapGet(item.Annotations, appmgrcommon.AppAnnotationSchedule); sched != "" {
						f, err := appmgrcommon.CronStringToCyclePeriodicRespAttr(appcommon.MapGet(item.Annotations, appmgrcommon.AppAnnotationSchedule))
						if err != nil {
							return nil, err
						}

						ai.CyclePeriodicFields = f
					}
				}

				ai.TotalResources = &appmanager.Resources{}
				ai.TotalResources.Requests = &appmanager.Resources_Requests{}
				ai.TotalResources.Limits = &appmanager.Resources_Limits{}

				opts := rancher.DefaultListOpts()
				opts.Filters["name"] = item.TargetNamespace + "-shared-pv"
				if ai.SharedStorage, err = getStorageCapacity(mc, opts); err != nil {
					return nil, err
				}

				if ai.MonitorUrl, err = appmgrcommon.GetAppMonitorUrl(monEndpoint, appName, appAnnotationsCycle); err != nil {
					return nil, err
				}
				ai.LogsUrl = getAppLogsEndpoint(logEndpoint, appName, req.GetRootGroupId())
				body.Apps[appName] = ai

			}

			d := &appmanager.Instance{}
			d.Resources = &appmanager.Resources{}

			opts := rancher.DefaultListOpts()
			opts.Filters["name"] = item.Name
			w, err := mc.ProjectClient.Workload.List(opts)

			if len(w.Data) > 0 {
				// Gather instances for a particular application
				if d, err = getWorkloadInstanceData(mc, &w.Data[0], catalogId, verbose); err != nil {
					return nil, err
				}
			} else if d, err = getAppInstanceData(mc, &item, catalogId, verbose); err != nil {
				return nil, err
			}

			if limits != nil {
				d.Resources.Limits = limits
				d.Resources.Requests = requests
				body.Apps[appName].TotalResources.Requests.Cpu += d.Resources.Requests.Cpu
				body.Apps[appName].TotalResources.Requests.Memory += d.Resources.Requests.Memory
				body.Apps[appName].TotalResources.Limits.Cpu += d.Resources.Limits.Cpu
				body.Apps[appName].TotalResources.Limits.Memory += d.Resources.Limits.Memory
				body.Apps[appName].TotalResources.PersistentStorage += d.Resources.PersistentStorage
				body.Apps[appName].TotalResources.Requests.Cpu = math.Round(body.Apps[appName].TotalResources.Requests.Cpu*100) / 100
				body.Apps[appName].TotalResources.Limits.Cpu = math.Round(body.Apps[appName].TotalResources.Limits.Cpu*100) / 100
			}
			body.Apps[appName].Instances = append(body.Apps[appName].Instances, d)
		}
	}

	// Return body
	return body, nil
}

// Enable or disable application
func EnableDisableApp(mc *rancher.MasterClient, req appmgrcommon.GenericRequester,
	state appmanager.AppStateAfterDeployment) (*appmanager.AppsActivation, error) {

	opts := rancher.DefaultListOpts()

	// Fetch data from the server
	collection, err := mc.ProjectClient.App.List(opts)
	if err != nil {
		return nil, err
	}

	// Create empty body
	body := &appmanager.AppsActivation{}
	body.Apps = make(map[string]*appmanager.AffectedAppInstances)

	var wg sync.WaitGroup
	var errors []error

	// Iterate over collection data
	for _, instance := range collection.Data {
		// All the application instances deployed by Controller
		// have appropriate annotation - "apphc.app.basename"
		if appName := appcommon.MapGet(instance.Annotations, appmgrcommon.AppAnnotationBaseName); appName != "" {
			var version string
			if req != nil {
				// Continue if the annotation value is no equal to the name in the request
				if req.GetName() != "" && req.GetName() != appName {
					continue
				}

				// If version appears in request use it for filtering
				version = appcommon.MapGet(instance.Annotations, appmgrcommon.AppInstanceAnnotationVersion)
				if req.GetVersion() != "" && req.GetVersion() != version {
					continue
				}

				// If root group ID appears in request use it for filtering
				if req.GetRootGroupId() != "" &&
					req.GetRootGroupId() != appcommon.MapGet(instance.Annotations, appmgrcommon.AppInstanceAnnotationRootGroupId) {
					continue
				}

				// Find Group ID
				groupId := appcommon.MapGet(instance.Annotations, appmgrcommon.AppInstanceAnnotationGroupId)
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

			// Add a new filter
			opts.Filters["name"] = instance.Name
			// Get a list of workloads
			wc, err := mc.ProjectClient.Workload.List(opts)
			if err != nil {
				return nil, err
			}

			// Add application instance state
			appcommon.MapAdd(instance.Annotations, appmgrcommon.AppInstanceAnnotationState, fmt.Sprintf("%d", state))

			// Disable workload
			if state == appmanager.AppStateAfterDeployment_disabled {
				// Update application object
				if _, err = mc.ProjectClient.App.Replace(&instance); err != nil {
					return nil, err
				}

				// If the workload exists
				if len(wc.Data) > 0 {
					if err := disableAppInstance(mc, wc, version); err != nil {
						return nil, err
					}
				}

				// Enable workload
			} else {
				// No workload found
				if len(wc.Data) == 0 {
					// Create temporary data
					tmpApp := &appmgrcommon.AppInstanceData{}
					tmpApp.InstanceName = instance.Name
					tmpApp.Description = instance.Description
					tmpApp.Annotations = instance.Annotations
					tmpApp.Labels = instance.Labels
					tmpApp.TargetNamespace = instance.TargetNamespace
					tmpApp.RequestedVersion = appcommon.MapGet(instance.Annotations, appmgrcommon.AppInstanceAnnotationVersion)

					cycle := appcommon.MapGet(instance.Annotations, appmgrcommon.AppAnnotationCycle)
					// Delete the application
					if err := DeleteAppInstance(mc, tmpApp, version); err != nil {
						return nil, err
					}

					// If the application type is not job
					if cycle != appmgrcommon.TypeRunOnce {
						// Use existing metadata in catalog in order to create the application
						wg.Add(1)
						go func(tmpApp *appmgrcommon.AppInstanceData) {
							defer wg.Done()
							tmpApp.State = appmanager.AppStateAfterDeployment_enabled
							if err := CreateAppInstance(mc, tmpApp, tmpApp.RequestedVersion); err != nil {
								errors = append(errors, err)
							}
						}(tmpApp)

						time.Sleep(time.Second * 1)
					}
				}
			}

			wg.Wait()

			if len(errors) > 0 {
				return nil, errors[0]
			}

			ai := &appmanager.AffectedAppInstance{}

			// Application instance name (converted to the Kubernetes format)
			ai.Name = instance.Name

			// Application instance ID (not converted to the Kubernetes format)
			ai.Id = appcommon.MapGet(instance.Annotations, appmgrcommon.AppInstanceAnnotationId)

			// Application instance version
			ai.Version = appcommon.MapGet(instance.Annotations, appmgrcommon.AppInstanceAnnotationVersion)

			// Application instance root group ID
			ai.RootGroupId = appcommon.MapGet(instance.Annotations, appmgrcommon.AppInstanceAnnotationRootGroupId)

			// Application instance group ID
			ai.GroupId = appcommon.MapGet(instance.Annotations, appmgrcommon.AppInstanceAnnotationGroupId)

			// Application instance state

			if body.Apps[appName] == nil {
				body.Apps[appName] = &appmanager.AffectedAppInstances{}
			}

			body.Apps[appName].Instances = append(body.Apps[appName].Instances, ai)
		}
	}

	return body, nil
}

// RefreshCatalog updates catalog indexes
func RefreshCatalog(mc *rancher.MasterClient, catalogId string) error {
	logrus.WithFields(logrus.Fields{"resourceType": "catalog",
		"catalogId": catalogId}).Debug("Looking up for the catalog resource")

	// Lookup catalog according to its name
	resource, err := rancher.Lookup(mc, catalogId, "catalog")
	if err != nil {
		return err
	}

	logrus.WithFields(logrus.Fields{"resource.ID": resource.ID}).Debug("Looking for the catalog by ID")

	// Find catalog resource according to the resource ID
	catalog, err := mc.ManagementClient.Catalog.ByID(resource.ID)
	if err != nil {
		return err
	}

	logrus.WithFields(logrus.Fields{"catalog": catalog}).Debug("Refreshing catalog")

	// Refresh the catalog
	return mc.ManagementClient.Catalog.ActionRefresh(catalog)
}

// TemplateAvailable figures out whether required template available
func TemplateAvailable(mc *rancher.MasterClient, templateName, appInstanceVersion, catalogId string) (bool, error) {
	// Construct filter from the default options
	filter := rancher.DefaultListOpts()

	logrus.WithFields(logrus.Fields{"templateName": templateName}).Debug("Looking up for the template resource")

	// Lookup resource according to its name and type
	resource, err := rancher.Lookup(mc, templateName, "template")
	if resource == nil {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	// Add more filters
	filter.Filters["name"] = templateName
	filter.Filters["catalogId"] = catalogId

	// Find collection
	collection, err := mc.ManagementClient.Template.List(filter)
	if err != nil {
		return false, err
	}

	// Iterate over templates data
	for _, item := range collection.Data {
		if _, ok := item.VersionLinks[appInstanceVersion]; ok {
			return true, nil
		}
	}

	return false, nil
}

// GetTemplateVersions gets versions of a particular metadata
func GetTemplateVersions(mc *rancher.MasterClient, appInstanceName string) ([]*gover.Version, error) {
	// Lookup the resource
	resource, err := rancher.Lookup(mc, appInstanceName, "template")
	if err != nil {
		return nil, err
	}
	// Find template according
	template, err := mc.ManagementClient.Template.ByID(resource.ID)
	if err != nil {
		return nil, err
	}

	var versions []*gover.Version
	for key := range template.VersionLinks {
		v, err := gover.NewVersion(key)
		if err != nil {
			return nil, err
		}
		versions = append(versions, v)
	}

	sort.Sort(gover.Collection(versions))
	return versions, nil
}

// DeleteNamespace deletes namespace
func DeleteNamespace(mc *rancher.MasterClient, namespace string) error {

	opts := rancher.DefaultListOpts()
	opts.Filters["name"] = namespace

	c, err := mc.ClusterClient.Namespace.List(opts)
	if len(c.Data) == 0 {
		return nil
	}

	if err != nil {
		return err
	}

	// Fetch namespace ID
	nsId := c.Data[0].ID

	logrus.WithFields(logrus.Fields{"namespace": opts.Filters["name"]}).Info("Deleting namespace")

	// Create the namespace
	if err := mc.ClusterClient.Namespace.Delete(&c.Data[0]); err != nil {
		return err
	}

	// Wait till the namespaces will be ready
	startTime := time.Now()
	for {
		if time.Since(startTime)/time.Second > 50 {
			return fmt.Errorf("timed out waiting for deleting namespace %s", opts.Filters["name"])
		}
		ns, _ := mc.ClusterClient.Namespace.ByID(nsId)
		if ns.ID == "" {
			break
		}

		time.Sleep(1 * time.Second)
	}

	logrus.WithFields(logrus.Fields{"namespace": opts.Filters["name"], "status": "OK"}).Info("Deleting namespace")

	return nil
}

// CheckAvailableResources checks whether amount of free resources in the cluster satisfies request
func CheckAvailableResources(mc *rancher.MasterClient, numberOfInstances int, limits *appmanager.Spec_Resources_Limits) error {

	logrus.Info("Verifying resources availability")
	opts := rancher.DefaultListOpts()

	col, err := mc.ManagementClient.Cluster.List(opts)
	if err != nil {
		return err
	}

	if len(col.Data) == 0 {
		return fmt.Errorf("no cluster found")
	}

	c := col.Data[0]

	if limits.GetCpu() > 0 {
		available, err := resourcemgr.ParseCpuString(c.Allocatable["cpu"])
		if err != nil {
			return err
		}

		requested := limits.GetCpu() * float64(numberOfInstances)
		if requested > available {
			return fmt.Errorf("number of requested CPUs (%.2f) exceeds number of available CPUs (%.2f)", requested, available)
		}
	}

	if limits.GetMemory() > 0 {
		_, _, free, err := resourcemgr.ParseAllMemoryString(c.Allocatable["memory"], c.Requested["memory"])
		if err != nil {
			return nil
		}

		requested := limits.GetMemory() * uint32(numberOfInstances)

		if requested >= free {
			return fmt.Errorf("requested memory (%d MiB) exceeds available memory (%d MiB)", requested, free)
		}
	}

	logrus.WithFields(logrus.Fields{"status": "OK"}).Info("Verifying resources availability")

	return nil
}

func getStorageCapacity(mc *rancher.MasterClient, opts *types.ListOpts) (uint32, error) {

	capacity := uint32(0)
	v, err := mc.ClusterClient.PersistentVolume.List(opts)
	if err != nil {
		return capacity, err
	}

	if len(v.Data) > 0 {
		c, err := grpccommon.GetStorageGbInt(v.Data[0].Capacity["storage"])
		if err != nil {
			return capacity, err
		}

		capacity = uint32(c)
	}

	return capacity, nil
}
