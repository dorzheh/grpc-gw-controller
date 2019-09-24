// Author  <dorzheho@cisco.com>

package rancher

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	managementClient "github.com/rancher/types/client/management/v3"
	projectClient "github.com/rancher/types/client/project/v3"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"k8s.io/api/core/v1"

	"cisco.com/son/apphcd/api/v1/clustermanager"
	appcommon "cisco.com/son/apphcd/app/common"
	appmgrcommon "cisco.com/son/apphcd/app/grpc/appmanager/common"
	clumgrcommon "cisco.com/son/apphcd/app/grpc/clustermanager/common"
	"cisco.com/son/apphcd/app/grpc/clustermanager/common/lcm"
	grpccommon "cisco.com/son/apphcd/app/grpc/common"
	"cisco.com/son/apphcd/app/grpc/common/rancher"
	"cisco.com/son/apphcd/app/grpc/common/resourcemgr"
)

type rancherCluMgrAdapter struct {
	mc *rancher.MasterClient // Master client
	sc *projectClient.Client // ApphosterServices project client
}

// NewAdapter creates Rancher Cluster manager adapter
func NewAdapter(masterClient *rancher.MasterClient, svcsProjectClient *projectClient.Client) *rancherCluMgrAdapter {
	return &rancherCluMgrAdapter{
		mc: masterClient,
		sc: svcsProjectClient,
	}
}

// GetKubeConfig fetches Kubernetes cluster configuration
func (adapter *rancherCluMgrAdapter) GetKubeConfig(req *clustermanager.GetKubeConfigRequest) (*clustermanager.GetKubeConfigResponse, error) {
	c, err := rancher.ClusterKubeConfig(adapter.mc)
	if err != nil {
		return nil, err
	}

	conf := &clustermanager.GetKubeConfigResponse{}
	conf.Kubeconfig = c

	logrus.WithFields(logrus.Fields{"service": "ClusterManager", "type": "grpc"}).Info("Sending response")

	return conf, nil
}

// GetClusterInfo shows AppHoster Cluster information
func (adapter *rancherCluMgrAdapter) GetClusterInfo(req *clustermanager.GetClusterInfoRequest, ns *v1.Namespace) (*clustermanager.Response, error) {

	col, err := adapter.mc.ManagementClient.Cluster.List(rancher.DefaultListOpts())
	if err != nil {
		return nil, err
	}

	c := col.Data[0]
	body := &clustermanager.GetClusterInfoResponseBody{}
	body.Id = c.ID
	body.ClusterName = c.Name
	body.CreationDate = c.Created
	body.CpuCores = &clustermanager.Cpu{}
	body.CpuCores.Total = c.Allocatable["cpu"]

	if a := appcommon.MapGet(ns.Annotations, clumgrcommon.NamespaceAnnotationApphosterUpgradeStatus); a != "" {
		m := json.RawMessage(a)
		out, err := m.MarshalJSON()
		if err != nil {
			return clumgrcommon.GenerateResponse(clustermanager.Status_ERROR, err.Error(), nil)
		}

		var u lcm.Upgrade
		err = json.Unmarshal(out, &u)
		if err != nil {
			return clumgrcommon.GenerateResponse(clustermanager.Status_ERROR, err.Error(), nil)
		}

		body.LastUpgradeStatus = &clustermanager.GetClusterInfoResponseBody_UpgradeStatus{}
		body.LastUpgradeStatus.LastUpgrade = u.LastUpgrade
		body.LastUpgradeStatus.Status = u.Status
		body.LastUpgradeStatus.ErrorMessage = u.ErrorMessage
		for _, c := range u.Components {
			comp := &clustermanager.GetClusterInfoResponseBody_UpgradeStatus_Component{}
			comp.Name = c.Name
			comp.Status = c.Status
			comp.ErrorMessage = c.ErrorMessage
			body.LastUpgradeStatus.Components = append(body.LastUpgradeStatus.Components, comp)
		}
	}

	monEndpoint, err := rancher.GetEndpoint(adapter.sc, adapter.mc.ManagementClient, grpccommon.ServiceMonitoringNamespace,
		grpccommon.ServiceMonitoringName, viper.GetString(appcommon.EnvApphSvcsUrlExternalIp))
	if err != nil {
		return clumgrcommon.GenerateResponse(clustermanager.Status_ERROR, err.Error(), nil)
	}

	if body.SvcMonitorUrl, err = clumgrcommon.GetServicesMonitorUrl(monEndpoint); err != nil {
		return clumgrcommon.GenerateResponse(clustermanager.Status_ERROR, err.Error(), nil)
	}

	if body.CpuCores.Allocated, err = getAllocatedCpu(c.Requested["cpu"]); err != nil {
		return clumgrcommon.GenerateResponse(clustermanager.Status_ERROR, err.Error(), nil)
	}

	body.Memory, err = getMemory(c.Allocatable["memory"], c.Requested["memory"])
	body.Condition = &clustermanager.Condition{}
	body.Condition.State = clustermanager.State_active
	for _, cond := range c.Conditions {
		if cond.Reason != "" && cond.Status == "True" {
			body.Condition.Status = clustermanager.Condition_ERROR
			body.Condition.Errors = append(body.Condition.Errors, cond.Message)
		}
	}

	var numOfCreatedInstances uint32
	createdInstances := make(map[string]uint32)

	var numOfActiveInstances uint32
	activeInstances := make(map[string]uint32)

	collection, err := adapter.mc.ProjectClient.App.List(rancher.DefaultListOpts())
	if err != nil {
		return clumgrcommon.GenerateResponse(clustermanager.Status_ERROR, err.Error(), nil)
	}

	for _, item := range collection.Data {
		appName := appcommon.MapGet(item.Annotations,appmgrcommon.AppAnnotationBaseName)
		createdInstances[appName]++
		numOfCreatedInstances++
		state := appcommon.MapGet(item.Annotations,appmgrcommon.AppInstanceAnnotationState)
		if state == "1" {
			numOfActiveInstances++
			activeInstances[appName]++
		}
	}

	body.Workloads = &clustermanager.GetClusterInfoResponseBody_Workloads{}
	body.Workloads.NumberOfApps = uint32(len(createdInstances))
	body.Workloads.NumberOfInstances = numOfCreatedInstances
	body.Workloads.NumberOfActiveApps = uint32(len(activeInstances))
	body.Workloads.NumberOfActiveInstances = numOfActiveInstances
	body.Workloads.Apps = []*clustermanager.GetClusterInfoResponseBody_App{}
	for name, numberOfInstances := range createdInstances {
		w := &clustermanager.GetClusterInfoResponseBody_App{}
		w.Name = name
		w.NumberOfInstances = numberOfInstances
		if _, ok := activeInstances[name]; ok {
			w.Active = true
		}
		body.Workloads.Apps = append(body.Workloads.Apps, w)
	}

	nc, err := adapter.mc.ManagementClient.Node.List(rancher.DefaultListOpts())
	if nil != err {
		return clumgrcommon.GenerateResponse(clustermanager.Status_ERROR, err.Error(), nil)
	}

	body.NumberOfNodes = uint32(len(collection.Data))

	body.Nodes = []*clustermanager.GetClusterInfoResponseBody_Node{}
	for _, n := range nc.Data {
		node := &clustermanager.GetClusterInfoResponseBody_Node{}
		if node.MonitorUrl, err = clumgrcommon.GetNodeMonitorUrl(monEndpoint, n.IPAddress); err != nil {
			return clumgrcommon.GenerateResponse(clustermanager.Status_ERROR, err.Error(), nil)
		}
		node.Id = n.ID
		node.Hostname = n.Hostname
		node.Master = n.ControlPlane
		node.Worker = n.Worker
		node.Etcd = n.Etcd
		node.ExternalIp = n.ExternalIPAddress
		node.Memory = &clustermanager.Memory{}
		node.CpuCores = &clustermanager.Cpu{}
		node.CpuCores.Total = n.Allocatable["cpu"]
		nCpu, err := getAllocatedCpu(n.Requested["cpu"])
		if err != nil {
			return clumgrcommon.GenerateResponse(clustermanager.Status_ERROR, err.Error(), nil)
		}

		node.CpuCores.Allocated = nCpu

		node.Condition = &clustermanager.Condition{}
		node.Condition.State = clumgrcommon.GetState(n.State)
		if n.Transitioning == "error" {
			node.Condition.Errors = append(node.Condition.Errors, n.TransitioningMessage)
			node.Condition.Status = clustermanager.Condition_ERROR
			body.Condition.Status = clustermanager.Condition_WARNING
		}

		for _, ncond := range n.Conditions {
			if ncond.Reason != "" && ncond.Status == "True" && ncond.Type != "Ready" {
				node.Condition.Status = clustermanager.Condition_ERROR
				node.Condition.Errors = append(node.Condition.Errors, ncond.Message)
				body.Condition.Status = clustermanager.Condition_WARNING
			}
		}

		node.Memory, err = getMemory(n.Allocatable["memory"], n.Requested["memory"])
		node.LocalStorage = &clustermanager.GetClusterInfoResponseBody_Node_Storage{}

		s := &clustermanager.GetClusterInfoResponseBody_Node_Storage{}

		sCap, err := grpccommon.GetStorageGbInt(n.Capacity["ephemeral-storage"])
		if err != nil {
			return clumgrcommon.GenerateResponse(clustermanager.Status_ERROR, err.Error(), nil)
		}

		s.Total = fmt.Sprintf("%d", sCap)

		sFree, err := grpccommon.GetStorageGbInt(n.Allocatable["ephemeral-storage"])
		if err != nil {
			return clumgrcommon.GenerateResponse(clustermanager.Status_ERROR, err.Error(), nil)
		}

		sFree = sFree / 1024 / 1024

		s.Free = fmt.Sprintf("%d", sFree)
		s.Used = fmt.Sprintf("%d", sCap-sFree)

		node.LocalStorage = s
		body.Nodes = append(body.Nodes, node)
	}

	return clumgrcommon.GenerateResponse(clustermanager.Status_SUCCESS, "Cluster information", body)
}

func (adapter *rancherCluMgrAdapter) CreateNode(req *clustermanager.CreateNodeRequest) (*clustermanager.Response, error) {

	col, err := adapter.mc.ManagementClient.Node.List(rancher.DefaultListOpts())
	if nil != err {
		return clumgrcommon.GenerateResponse(1, err.Error(), nil)
	}

	if len(col.Data) == 0 {
		return clumgrcommon.GenerateResponse(1, "no AppHoster nodes found", nil)
	}

	//for _, n := range col.Data {
	//	if n.
	//}
	//

	//clusterTokenCollection, err := adapter.client.ManagementClient.ClusterRegistrationToken.List(rancher.DefaultListOpts())
	//if nil != err {
	//	return common.GenerateResponse(1, err.Error(), nil)
	//}
	//
	//var clusterToken *managementClient.ClusterRegistrationToken
	//
	//if len(clusterTokenCollection.Data) == 0 {
	//	crt := &managementClient.ClusterRegistrationToken{
	//		ClusterID: clusterCollection.Data[0].ID,
	//	}
	//
	//	clusterToken, err = adapter.client.ManagementClient.ClusterRegistrationToken.Create(crt)
	//	if nil != err {
	//		return common.GenerateResponse(1, err.Error(), nil)
	//	}
	//
	//} else {
	//
	//	clusterToken = &clusterTokenCollection.Data[0]
	//}
	//
	//logrus.Info(clusterToken.Command)
	//
	//var roleFlags string
	//
	//if req.GetMaster() {
	//	roleFlags = " --etcd  --controlplane  --worker"
	//} else {
	//	roleFlags = " --worker"
	//}
	//
	//command := clusterToken.NodeCommand + roleFlags
	//
	//client, err := ssh.ConnectWithPassword("DDDDDDD")+":"+
	//	viper.GetString(common.MdsoLeMmeSshPort), viper.GetString(common.MdsoLeMmeSshUser),
	//	viper.GetString(common.MdsoLeMmeSshPassword))
	//if err != nil {
	//	return
	//}
	//
	//defer client.Close()
	//
	//if _, err := client.Exec(viper.GetString(common.MdsoLeMmeDumpCmd)); err != nil {
	//	logrus.Errorf("[SYNCER] %v", err)
	//	return
	//}
	//
	//fullSearch := filepath.Join(viper.GetString(common.MdsoLeMmeDumpFileRemoteDir), viper.GetString(common.MdsoLeMmeDumpFilePattern))
	//
	//out, err := client.Exec("ls -tr " + fullSearch + "|tail -1")
	//if err != nil {
	//}
	//	strings.Replace(command, "rancher/rancher-agent", viper.GetString(gcommon.EnvApphcPrivateDockerRegistry), -1)

	body := &clustermanager.CreateNodeResponseBody{}
	body.Hostname = req.GetHostname()
	body.Master = req.GetMaster()
	body.Id = "dummyID"
	return clumgrcommon.GenerateResponse(clustermanager.Status_SUCCESS, "The new node was added", body)
}

func (adapter *rancherCluMgrAdapter) DeleteNode(req *clustermanager.DeleteNodeRequest) (*clustermanager.Response, error) {

	node, err := getNodeByHostname(adapter.mc, req.GetHostname())
	if err != nil {
		return clumgrcommon.GenerateResponse(clustermanager.Status_ERROR, err.Error(), nil)
	}

	if err := adapter.mc.ManagementClient.Node.ActionDrain(node, drainOpts()); err != nil {
		return clumgrcommon.GenerateResponse(clustermanager.Status_ERROR, err.Error(), nil)
	}

	if err := waitForDrained(adapter.mc, req.GetHostname()); err != nil {
		return clumgrcommon.GenerateResponse(clustermanager.Status_ERROR, err.Error(), nil)
	}

	if err := adapter.mc.ManagementClient.Node.Delete(node); err != nil {
		return clumgrcommon.GenerateResponse(clustermanager.Status_ERROR, err.Error(), nil)
	}

	body := &clustermanager.DeleteNodeResponseBody{}
	body.Id = node.ID
	body.Hostname = node.Hostname

	return clumgrcommon.GenerateResponse(clustermanager.Status_SUCCESS, "The node was deleted", body)
}

func (adapter *rancherCluMgrAdapter) UpdateNodeState(req *clustermanager.UpdateNodeStateRequest) (*clustermanager.Response, error) {

	node, err := getNodeByHostname(adapter.mc, req.GetHostname())
	if err != nil {
		return clumgrcommon.GenerateResponse(clustermanager.Status_ERROR, err.Error(), nil)
	}

	body := &clustermanager.UpdateNodesStateResponseBody{}

	switch req.GetState() {

	case clustermanager.State_unschedulable:
		if err := adapter.mc.ManagementClient.Node.ActionCordon(node); err != nil {
			return clumgrcommon.GenerateResponse(clustermanager.Status_ERROR, err.Error(), nil)
		}

		body.State = clustermanager.State_unschedulable

	case clustermanager.State_maintenance:
		if err := adapter.mc.ManagementClient.Node.ActionDrain(node, drainOpts()); err != nil {
			return clumgrcommon.GenerateResponse(clustermanager.Status_ERROR, err.Error(), nil)
		}

		if err := waitForDrained(adapter.mc, req.GetHostname()); err != nil {
			return clumgrcommon.GenerateResponse(clustermanager.Status_ERROR, err.Error(), nil)
		}

		body.State = clustermanager.State_maintenance

	case clustermanager.State_active:
		if err := adapter.mc.ManagementClient.Node.ActionUncordon(node); err != nil {
			return clumgrcommon.GenerateResponse(clustermanager.Status_ERROR, err.Error(), nil)
		}

		body.State = clustermanager.State_active
	}

	body.Hostname = node.Hostname
	body.Id = node.ID

	return clumgrcommon.GenerateResponse(clustermanager.Status_SUCCESS, "The node was set to the new state", body)
}

// Implemented in the clustermanager package
func (adapter *rancherCluMgrAdapter) SetClusterResourceQuotas(req *clustermanager.SetClusterResourceQuotasRequest) (*clustermanager.Response, error) {
	return nil, nil
}

// Implemented in the clustermanager package
func (adapter *rancherCluMgrAdapter) GetClusterResourceQuotas(req *clustermanager.GetClusterResourceQuotasRequest) (*clustermanager.Response, error) {
	return nil, nil
}

// Implemented in the clustermanager package 
func (adapter *rancherCluMgrAdapter) DeleteClusterResourceQuotas(req *clustermanager.DeleteClusterResourceQuotasRequest) (*clustermanager.Response, error) {
	return nil, nil
}

func getNodeByHostname(client *rancher.MasterClient, hostName string) (*managementClient.Node, error) {
	opts := rancher.DefaultListOpts()
	opts.Filters["hostname"] = hostName

	collection, err := client.ManagementClient.Node.List(opts)
	if err != nil {
		return nil, err
	}

	if len(collection.Data) == 0 {
		return nil, fmt.Errorf("node %s not found", hostName)
	}

	return &collection.Data[0], nil
}

func getAllocatedCpu(reqCpu string) (string, error) {
	// In case the value represented in milli cores and "m" is the suffix
	if strings.HasSuffix(reqCpu, "m") {
		// Cut last char, convert to int
		reqCoresInt, err := strconv.Atoi(reqCpu[0:(len(reqCpu) - 1)])
		if err != nil {
			return "", err
		}
		// Calculate CPU allocation
		return fmt.Sprintf("%.2f", float32(reqCoresInt)/float32(1000)), nil
	}

	return reqCpu, nil
}

func getMemory(capacityMem, allocatedMem string) (*clustermanager.Memory, error) {

	m := &clustermanager.Memory{}
	total, allocated, free, err := resourcemgr.ParseAllMemoryString(capacityMem, allocatedMem)
	if err != nil {
		return nil, err
	}

	m.Total = fmt.Sprintf("%d", total)
	m.Allocated = fmt.Sprintf("%d", allocated)
	m.Free = fmt.Sprintf("%d", free)
	return m, nil
}

//func getStorageInt(storageInKb string) (int, error) {
//	var storage string
//
//	if strings.HasSuffix(storageInKb, "Ki") {
//		storage = storageInKb[0:(len(storageInKb) - 2)]
//	} else {
//		storage = storageInKb
//	}
//
//	storageInt, err := strconv.Atoi(storage)
//	if err != nil {
//		return 0, err
//	}
//
//	return storageInt / 1024, nil
//}

// drainOpt creates options for drain operation
func drainOpts() *managementClient.NodeDrainInput {
	drainOpts := &managementClient.NodeDrainInput{}
	drainOpts.DeleteLocalData = true
	drainOpts.Force = true
	drainOpts.IgnoreDaemonSets = true
	drainOpts.Timeout = 180

	return drainOpts
}

func waitForDrained(client *rancher.MasterClient, hostName string) error {
	// Start the timer
	startTime := time.Now()
	timeout := time.Duration(0)
	timeout = 180

	for {

		h, err := getNodeByHostname(client, hostName)
		if err != nil {
			return err
		}

		if h.State == clumgrcommon.NodeStateDrained {
			break
		}

		// If the timer is exceeded , return appropriate error
		if time.Since(startTime)/time.Second > timeout {
			return fmt.Errorf("timed out waiting for draining the node %s", hostName)
		}

		// Wait and continue in the loop
		time.Sleep(1 * time.Second)
	}

	return nil
}
