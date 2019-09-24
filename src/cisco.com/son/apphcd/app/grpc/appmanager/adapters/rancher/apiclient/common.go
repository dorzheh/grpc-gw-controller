// Author <dorzheho@cisco.com>

package apiclient

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/rancher/norman/types"
	clusterClient "github.com/rancher/types/client/cluster/v3"
	projectClient "github.com/rancher/types/client/project/v3"
	"github.com/sirupsen/logrus"

	"cisco.com/son/apphcd/api/v1/appmanager"
	appcommon "cisco.com/son/apphcd/app/common"
	appmgrcommon "cisco.com/son/apphcd/app/grpc/appmanager/common"
	"cisco.com/son/apphcd/app/grpc/common/rancher"
	"cisco.com/son/apphcd/app/grpc/common/resourcemgr"
)

// createNamespace creates namespace
func CreateNamespace(mc *rancher.MasterClient, namespace string) error {

	opts := rancher.DefaultListOpts()
	opts.Filters["name"] = namespace

	// Find namespace according to the filter
	namespaces, err := mc.ClusterClient.Namespace.List(opts)
	if err != nil {
		return err
	}

	// If the namespace doesn't exist
	if len(namespaces.Data) == 0 {
		newNamespace := &clusterClient.Namespace{
			Name:      opts.Filters["name"].(string),
			ProjectID: mc.UserConfig.Project,
		}

		// Create the namespace
		logrus.WithFields(logrus.Fields{"namespace": opts.Filters["name"]}).Info("Creating namespace")

		ns, err := mc.ClusterClient.Namespace.Create(newNamespace)
		if err != nil {
			return err
		}

		// Wait till the namespaces will be ready
		startTime := time.Now()
		for {
			if time.Since(startTime)/time.Second > 30 {
				return fmt.Errorf("timed out waiting for new namespace %s", opts.Filters["name"])
			}
			ns, err = mc.ClusterClient.Namespace.ByID(ns.ID)
			if err != nil {
				return err
			}

			if ns.State == "active" {
				break
			}

			time.Sleep(500 * time.Millisecond)
		}

		logrus.WithFields(logrus.Fields{"namespace": opts.Filters["name"], "status": "OK"}).
			Info("Creating namespace")

	} else {
		if namespaces.Data[0].ProjectID != mc.UserConfig.Project {
			return fmt.Errorf("namespace %s already exists", opts.Filters["name"])
		}
	}

	return nil
}

func cleanUpVolume(mc *rancher.MasterClient, opts *types.ListOpts) (pvId string, err error) {
	c, err := mc.ClusterClient.PersistentVolume.List(opts)
	if err != nil {
		return
	}

	if len(c.Data) > 0 {
		// Store PV ID
		pvId = c.Data[0].ID
		// In case we have a volume in state "Released"
		if c.Data[0].Name != "" && c.Data[0].Status.Phase == "Released" {
			// Delete the volume
			if err = deletePersistentVolume(mc, opts); err != nil {
				return
			}
			// Wait till the PV will be removed
			if err = waitForVolumeState(mc, opts, "removed"); err != nil {
				return
			}
		}
	}

	return
}

// deletePersistentVolume removes appropriate persistent volume (PV)
func deletePersistentVolume(mc *rancher.MasterClient, opts *types.ListOpts) error {

	// Lookup persistentVolume resource according to the schema
	c, err := mc.ClusterClient.PersistentVolume.List(opts)
	if c == nil || len(c.Data) == 0 {
		return nil
	}

	if err != nil {
		return err
	}

	// Fetch PV ID
	pvId := c.Data[0].ID

	var pv *clusterClient.PersistentVolume

	// Wait till the PV will be ready
	startTime := time.Now()
	for {
		if time.Since(startTime)/time.Second > 30 {
			return fmt.Errorf("timed out waiting for the volume status Released %s", opts.Filters["name"])
		}
		if pv, err = mc.ClusterClient.PersistentVolume.ByID(pvId); err != nil {
			return err
		}

		if pv.Status.Phase == "Released" || pv.Status.Phase == "Available" {
			break
		}

		time.Sleep(500 * time.Millisecond)
	}

	// Delete PV
	logrus.WithFields(logrus.Fields{"volume": pv.Name}).Info("Deleting persistent volume")
	if err := mc.ClusterClient.PersistentVolume.Delete(pv); err != nil {
		return err
	}

	logrus.WithFields(logrus.Fields{"volume": pv.Name, "status": "OK"}).Info("Deleting persistent volume")

	return nil
}

// deletePersistentVolumeClaim removes appropriate persistent volume claim (PVC)
func deletePersistentVolumeClaim(mc *rancher.MasterClient, opts *types.ListOpts) error {

	// List the PVC collection
	c, err := mc.ProjectClient.PersistentVolumeClaim.List(opts)
	if c == nil || len(c.Data) == 0 {
		return nil
	}

	if err != nil {
		return err
	}

	// Fetch PVC ID
	pvcId := c.Data[0].ID

	if err := mc.ProjectClient.PersistentVolumeClaim.Delete(&c.Data[0]); err != nil {
		return err
	}

	logrus.WithFields(logrus.Fields{"claim": opts.Filters["name"]}).Info("Deleting persistent volume claim")

	startTime := time.Now()
	for {
		if time.Since(startTime)/time.Second > 300 {
			return fmt.Errorf("timed out waiting for deleting persistentVolumeClaim %s", opts.Filters["name"])
		}

		c, err := mc.ProjectClient.PersistentVolumeClaim.ByID(pvcId)
		if c != nil && c.ID == "" {
			break
		}

		if err != nil {
			return err
		}

		time.Sleep(1 * time.Second)
	}

	logrus.WithFields(logrus.Fields{"claim": opts.Filters["name"], "status": "OK"}).Info("Deleting persistent volume claim")

	return nil
}

// getPodsInErrorState checks the state of pods and containers belonging to appropriate application instance
func getPodsInErrorState(mc *rancher.MasterClient, opts *types.ListOpts) (string, error) {

	// Get list of pods
	pods, err := mc.ProjectClient.Pod.List(opts)
	if err != nil {
		return "", err
	}

	// Iterate over pods data
	for _, pod := range pods.Data {
		// Iterate over containers data
		for _, c := range pod.Containers {
			// In case container in transitioning state
			if c.Transitioning == "yes" {
				// Return transitioning message
				return c.TransitioningMessage, nil
			}
		}
	}

	return "", nil
}

// waitForVolume waits until particular volume will be ready
func waitForVolumeState(mc *rancher.MasterClient, opts *types.ListOpts, expectedState string) error {
	logrus.WithFields(logrus.Fields{"volume": opts.Filters["name"]}).Info("Waiting for the volume")

	timeout := time.After(time.Duration(100) * time.Second)
	every := time.Tick(1 * time.Second)

LOOP:
	for {
		select {
		case <-timeout:
			return fmt.Errorf("timed out waiting for PV state %s", expectedState)
		case <-every:
			pv, err := mc.ClusterClient.PersistentVolume.List(opts)
			if len(pv.Data) == 0 {
				if expectedState == "removed" {
					break LOOP
				}
			} else {
				if pv.Data[0].State == expectedState {
					break LOOP
				}
			}

			if err != nil {
				return err
			}
		}
	}

	logrus.WithFields(logrus.Fields{"volume": opts.Filters["name"], "status": "OK"}).Info("Waiting for the volume")

	return nil
}

// Rancher doesn't wait till all pods and containers in the application instance
// are ready hence apiWaitForPodsReadiness implements this functionality
func waitForPodsReadiness(mc *rancher.MasterClient, appInstanceName string) error {

	logrus.WithFields(logrus.Fields{"instance": appInstanceName}).Info("Waiting for pods readiness")

	filter := rancher.DefaultListOpts()
	filter.Filters["name"] = appInstanceName

	c, err := mc.ProjectClient.Workload.List(filter)
	if err != nil {
		return err
	}

	if len(c.Data) > 0 {
		// Start the timer
		startTime := time.Now()
		timeout := time.Duration(0)

		// Add filters
		filter.Filters["workloadId"] = c.Data[0].ID
		filter.Filters["transitioning"] = "yes"
		filter.Filters["state"] = "unavailable"
		delete(filter.Filters,"name")

		for {
			// Get the state of pods and containers
			msg, err := getPodsInErrorState(mc, filter)
			if err != nil {
				return err
			}

			// If no error message , exit the loop
			if msg == "" {
				break
			}

			// In case the message tells that container is being created we do not consider it as an error
			// but an ordinary transition message. In this case set the timeout to 6 minutes. Otherwise set to 10 seconds
			if msg == "ContainerCreating" {
				timeout = 360
			} else {
				timeout = 10
			}

			// If the timer is exceeded , return appropriate error
			if time.Since(startTime)/time.Second > timeout {
				return fmt.Errorf("timed out waiting for pods readiness: %s", msg)
			}

			// Wait and continue in the loop
			time.Sleep(1 * time.Second)
		}

		logrus.WithFields(logrus.Fields{"instance": appInstanceName, "status": "OK"}).Info("Waiting for pods readiness")

	}
	// No error so far
	return nil
}

// getWorkloadInstanceData creates appmanager.Workload proto message
func getWorkloadInstanceData(mc *rancher.MasterClient, item *projectClient.Workload, catalogId string, verbose bool) (*appmanager.Instance, error) {
	ai := &appmanager.Instance{}

	// Application instance name (converted to the Kubernetes format)
	ai.Name = item.Name

	// Application instance ID (not converted to the Kubernetes format)
	ai.Id = appcommon.MapGet(item.Annotations, appmgrcommon.AppInstanceAnnotationId)

	// Application instance version
	ai.Version = appcommon.MapGet(item.Annotations, appmgrcommon.AppInstanceAnnotationVersion)

	// Application instance root group ID
	ai.RootGroupId = appcommon.MapGet(item.Annotations, appmgrcommon.AppInstanceAnnotationRootGroupId)

	// Application instance group ID
	ai.GroupId = appcommon.MapGet(item.Annotations, appmgrcommon.AppInstanceAnnotationGroupId)

	// Application image repo
	ai.ImageRepo = appcommon.MapGet(item.Annotations, appmgrcommon.AppInstanceAnnotationImageRepoName)

	// Application image name
	ai.ImageName = appcommon.MapGet(item.Annotations, appmgrcommon.AppInstanceAnnotationImageName)

	// Application image tag
	ai.ImageTag = appcommon.MapGet(item.Annotations, appmgrcommon.AppInstanceAnnotationImageTag)

	// Switch application type
	switch item.Type {

	// Type cronJobe
	case appmgrcommon.TypeCronJob:
		s := &appmanager.Instance_PeriodicFields{}
		s.PeriodicFields = &appmanager.PeriodicFields{}

		// CronJob scheduler
		s.PeriodicFields.Schedule = item.CronJobConfig.Schedule

		// Last time the job was scheduled
		s.PeriodicFields.LastScheduleTime = item.CronJobStatus.LastScheduleTime

		// Policy telling how many successful jobs need to be preserved in the cluster
		s.PeriodicFields.SuccessfulJobsHistoryLimit = *item.CronJobConfig.SuccessfulJobsHistoryLimit

		// Policy telling how many failed jobs need to be preserved in the cluster
		s.PeriodicFields.FailedJobsHistoryLimit = *item.CronJobConfig.FailedJobsHistoryLimit
		ai.CycleFields = s
		ai.Cycle = appmgrcommon.TypePeriodic

		// Type job
	case appmgrcommon.TypeJob:
		s := &appmanager.Instance_RunOnceFields{}
		s.RunOnceFields = &appmanager.RunOnceFields{}

		// Number of completions
		s.RunOnceFields.Completions = *item.JobConfig.Completions

		// Number of active jobs
		s.RunOnceFields.Active = item.JobStatus.Active

		// Job start time
		s.RunOnceFields.StartTime = item.JobStatus.StartTime

		// Job completion time
		s.RunOnceFields.CompletionTime = item.JobStatus.CompletionTime

		// Succeeded jobs
		s.RunOnceFields.Succeeded = item.JobStatus.Succeeded

		// Failed jobs
		s.RunOnceFields.Failed = item.JobStatus.Failed
		ai.CycleFields = s
		ai.Cycle = appmgrcommon.TypeRunOnce

		// Type daemon
	default:
		ai.Cycle = appmgrcommon.TypeDaemon
	}

	ai.State = item.State
	ai.CreateDate = item.Created

	// Set scale only if supported
	if item.Scale == nil {
		ai.Scale = "-"
	} else {
		ai.Scale = strconv.Itoa(int(*item.Scale))
	}

	ai.Resources = &appmanager.Resources{}
	ai.Namespace = item.NamespaceId

	opts := rancher.DefaultListOpts()
	opts.Filters["name"] = ai.Name + "-pv"
	var err error
	if ai.Resources.PersistentStorage, err  = getStorageCapacity(mc, opts); err != nil {
		return nil, err
	}

	ai.ProjectId = item.ProjectID

	// Print only if "verbose" flag was provided
	if verbose {
		for _, cont := range item.Containers {
			c := &appmanager.Instance_Container{}
			c.State = cont.State
			c.Name = cont.Name
			c.Image = cont.Image
			rcpu := appcommon.MapGet(cont.Resources.Requests, "cpu")
			if rcpu != "" {
				cpu, err := resourcemgr.ParseCpuString(rcpu)
				if err != nil {
					return nil, err
				}
				ai.Resources.Requests.Cpu += cpu
			}

			rmemory := appcommon.MapGet(cont.Resources.Requests, "memory")
			if rmemory != "" {
				memory, err := resourcemgr.ParseMemoryString(rmemory)
				if err != nil {
					return nil, err
				}

				ai.Resources.Requests.Memory += memory
			}

			lcpu := appcommon.MapGet(cont.Resources.Limits, "cpu")
			if lcpu != "" {
				cpu, err := resourcemgr.ParseCpuString(lcpu)
				if err != nil {
					return nil, err
				}
				ai.Resources.Limits.Cpu += cpu
			}

			lmemory := appcommon.MapGet(cont.Resources.Limits, "memory")
			if lmemory != "" {
				memory, err := resourcemgr.ParseMemoryString(lmemory)
				if err != nil {
					return nil, err
				}

				ai.Resources.Limits.Memory += memory
			}

			for _, vol := range cont.VolumeMounts {
				v := &appmanager.Instance_Container_VolumeMount{}
				v.Name = vol.Name
				v.MountPath = vol.MountPath
				v.ReadOnly = vol.ReadOnly
				v.SubPath = vol.SubPath
				c.VolMounts = append(c.VolMounts, v)
			}

			for _, port := range cont.Ports {
				p := &appmanager.Instance_Container_Port{}
				p.Name = port.Name
				p.Proto = port.Protocol
				p.Port = port.ContainerPort
				p.SrcPort = port.SourcePort
				p.DnsName = port.DNSName
				p.HostIp = port.HostIp
				p.Kind = port.Kind
				c.Ports = append(c.Ports, p)
			}
			ai.Containers = append(ai.Containers, c)
		}

		for _, endp := range item.PublicEndpoints {
			ep := &appmanager.Instance_PublicEndpoint{}
			ep.ServiceId = endp.ServiceID
			ep.Port = endp.Port
			ep.Proto = endp.Protocol
			ep.AllNodes = endp.AllNodes
			ep.Hostname = endp.Hostname
			ep.IngressId = endp.IngressID
			ep.NodeId = endp.NodeID
			ep.Path = endp.Path
			ep.PodId = endp.PodID
			for _, a := range endp.Addresses {
				ep.Addresses = append(ep.Addresses, a)
			}
			ai.PublicEndpoints = append(ai.PublicEndpoints, ep)
		}

		// Get the list of available templates
		versions, err := GetTemplateVersions(mc, ai.Name)
		if err != nil {
			return nil, err
		}

		ai.Template = &appmanager.Template{}
		ai.Template.Name = ai.Name
		ai.Template.CatalogId = catalogId
		for _, v := range versions {
			ai.Template.Versions = append(ai.Template.Versions, v.String())
		}
	}

	return ai, nil
}

// getAppInstanceData provides information about particular instance
func getAppInstanceData(mc *rancher.MasterClient, item *projectClient.App, catalogId string, verbose bool) (*appmanager.Instance, error) {
	ai := &appmanager.Instance{}

	// Application instance name (converted to the Kubernetes format)
	ai.Name = item.Name

	// Application instance ID (not converted to the Kubernetes format)
	ai.Id = appcommon.MapGet(item.Annotations, appmgrcommon.AppInstanceAnnotationId)

	// Application instance version
	ai.Version = appcommon.MapGet(item.Annotations, appmgrcommon.AppInstanceAnnotationVersion)

	// Application instance root group ID
	ai.RootGroupId = appcommon.MapGet(item.Annotations, appmgrcommon.AppInstanceAnnotationRootGroupId)

	// Application instance group ID
	ai.GroupId = appcommon.MapGet(item.Annotations, appmgrcommon.AppInstanceAnnotationGroupId)

	// Application image repo
	ai.ImageRepo = appcommon.MapGet(item.Annotations, appmgrcommon.AppInstanceAnnotationImageRepoName)

	// Application image name
	ai.ImageName = appcommon.MapGet(item.Annotations, appmgrcommon.AppInstanceAnnotationImageName)

	// Application image tag
	ai.ImageTag = appcommon.MapGet(item.Annotations, appmgrcommon.AppInstanceAnnotationImageTag)

	ai.Cycle = appcommon.MapGet(item.Annotations, appmgrcommon.AppAnnotationCycle)

	ai.State = "disabled"
	ai.CreateDate = item.Created

	s := appcommon.MapGet(item.Annotations, appmgrcommon.AppInstanceAnnotationPersistentVolumeSize)
	if s != "" {
		size, err := strconv.Atoi(s)
		if err != nil {
			return nil, err
		}

		ai.Resources = &appmanager.Resources{}
		ai.Resources.PersistentStorage = uint32(size)
	}

	if verbose {
		ai.Namespace = item.TargetNamespace

		ai.Template = &appmanager.Template{}
		ai.Template.Name = ai.Name
		ai.Template.CatalogId = catalogId

		// Get the list of available templates
		versions, err := GetTemplateVersions(mc, ai.Name)
		if err != nil {
			return nil, err
		}

		for _, v := range versions {
			ai.Template.Versions = append(ai.Template.Versions, v.String())
		}
	}

	return ai, nil
}

// getAppLogsEndpoint provides an endpoint to Kibana dashboard
func getAppLogsEndpoint(endpoint, appName, rootGroupId string) string {

	appFilter := fmt.Sprintf(appmgrcommon.AppNameFilter, appName, appName, appName)
	if rootGroupId != "" {
		appFilter += "," + fmt.Sprintf(appmgrcommon.RootGroupIdFilter, rootGroupId, rootGroupId, rootGroupId)
	}

	return fmt.Sprintf(appmgrcommon.UrlGenericPart, endpoint, appFilter)
}

// postDeploymentAction applies appropriate actions as soon as application is deployed.
// Currently supported actions are enable or disable applications
func postDeploymentAction(mc *rancher.MasterClient, instanceName, instanceVersion string,
	state appmanager.AppStateAfterDeployment) error {

	if state == appmanager.AppStateAfterDeployment_enabled {
		// Wait for Pods readiness
		if err := waitForPodsReadiness(mc, instanceName); err != nil {
			logrus.Error(err)
			return err
		}

	} else {
		// In case the application instance should be disabled , controller will delete appropriate workload
		// Set default options for workloads
		wopts := rancher.DefaultListOpts()
		// Add a new filter
		wopts.Filters["name"] = instanceName
		// Get a list of workloads
		wc, err := mc.ProjectClient.Workload.List(wopts)
		if err != nil {
			return err
		}

		// If the workload exists
		if len(wc.Data) > 0 {
			if err := disableAppInstance(mc, wc, instanceVersion); err != nil {
				return err
			}
		}
	}

	return nil
}

// disableAppInstance disables a particular application instance
func disableAppInstance(mc *rancher.MasterClient, wc *projectClient.WorkloadCollection, instanceVersion string) error {

	instance := &wc.Data[0]

	logrus.WithFields(logrus.Fields{"instance": instance.Name, "version": instanceVersion}).
		Info("Disabling application")

	// Delete workload
	if err := mc.ProjectClient.Workload.Delete(instance); err != nil {
		return err
	}

	logrus.WithFields(logrus.Fields{"instance": instance.Name, "version": instanceVersion,
		"status": "OK"}).Info("Disabling application")

	return nil
}

// templateVersionIDFromVersionLink returns version ID from API link
func templateVersionIDFromVersionLink(s string) string {
	pieces := strings.Split(s, "/")
	return pieces[len(pieces)-1]
}
