// Author  <dorzheho@cisco.com>

package clustermanager

import (
	"context"
	"strings"

	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"cisco.com/son/apphcd/api/v1/clustermanager"
	pb "cisco.com/son/apphcd/api/v1/clustermanager"
	"cisco.com/son/apphcd/app/common/mutex"
	clumgrcommon "cisco.com/son/apphcd/app/grpc/clustermanager/common"
	"cisco.com/son/apphcd/app/grpc/clustermanager/common/lcm"
	grpccommon "cisco.com/son/apphcd/app/grpc/common"
	"cisco.com/son/apphcd/app/grpc/common/resourcemgr"
)

const (
	errorUpgradeNotAvailable      = "Process exited with status 1"
	errorUpgradeMultipleArtifacts = "Process exited with status 2"
)

const cmdIsUpgradAvailable = "apph-upgrade.sh -o available"

type manager struct {
	adapter clumgrcommon.ClusterManagerAdapter
	kc      *kubernetes.Clientset
}

func New(adapter clumgrcommon.ClusterManagerAdapter, clientset *kubernetes.Clientset) pb.ClusterManagerServer {
	return &manager{adapter: adapter, kc: clientset}
}

func (mgr *manager) GetKubeConfig(ctx context.Context, req *pb.GetKubeConfigRequest) (*pb.GetKubeConfigResponse, error) {
	logrus.WithFields(logrus.Fields{
		"service": "ClusterManager",
		"type":    "grpc",
	}).Info("Received GetKubeConfigRequest")

	return mgr.adapter.GetKubeConfig(req)
}

func (mgr *manager) UpgradeCluster(ctx context.Context, req *pb.UpgradeClusterRequest) (*pb.Response, error) {
	logrus.WithFields(logrus.Fields{
		"service": "ClusterManager",
		"type":    "grpc",
	}).Info("Received UpgradeClusterRequest")

	if mutex.IsLocked(mutex.LockActionUpgradeCluster) {
		return clumgrcommon.GenerateResponse(clustermanager.Status_IN_PROGRESS, "AppHoster cluster upgrade in progress", nil)
	}

	client, err := grpccommon.CreateSshClient()
	if err != nil {
		return clumgrcommon.GenerateResponse(clustermanager.Status_ERROR, err.Error(), nil)
	}

	// Create session
	s, err := client.NewSession()
	if err != nil {
		return clumgrcommon.GenerateResponse(clustermanager.Status_ERROR, err.Error(), nil)
	}

	defer s.Close()

	err = s.Run(cmdIsUpgradAvailable)
	if err != nil {
		if strings.Contains(errorUpgradeNotAvailable, err.Error()) {
			return clumgrcommon.GenerateResponse(clustermanager.Status_ERROR, "no upgrade available", nil)
		}

		if strings.Contains(errorUpgradeMultipleArtifacts, err.Error()) {
			return clumgrcommon.GenerateResponse(clustermanager.Status_ERROR, "found multiple upgrade resources", nil)
		}

		return clumgrcommon.GenerateResponse(clustermanager.Status_ERROR, "internal error", nil)
	}

	logrus.Debug("Upgrade available")

	ns, err := mgr.kc.CoreV1().Namespaces().Get("default", metav1.GetOptions{})
	if err != nil {
		return clumgrcommon.GenerateResponse(clustermanager.Status_ERROR, err.Error(), nil)
	}
	value, _ := clumgrcommon.GenerateResponse(clustermanager.Status_IN_PROGRESS, "AppHoster cluster upgrade in progress", nil)
	mutex.Lock(mutex.LockActionUpgradeCluster, value)

	go lcm.UpgradeCluster(client, mgr.kc, ns)
	return value, nil
}

func (mgr *manager) GetClusterInfo(ctx context.Context, req *pb.GetClusterInfoRequest) (*pb.Response, error) {
	logrus.WithFields(logrus.Fields{
		"service": "ClusterManager",
		"type":    "grpc",
	}).Info("Received GetClusterInfoRequest")

	if mutex.IsLocked(mutex.LockActionUpgradeCluster) {
		return clumgrcommon.GenerateResponse(clustermanager.Status_IN_PROGRESS, "AppHoster cluster upgrade in progress", nil)
	}

	ns, err := mgr.kc.CoreV1().Namespaces().Get("default", metav1.GetOptions{})
	if err != nil {
		return clumgrcommon.GenerateResponse(clustermanager.Status_ERROR, err.Error(), nil)
	}
	return mgr.adapter.GetClusterInfo(req, ns)
}

func (mgr *manager) CreateNode(ctx context.Context, req *pb.CreateNodeRequest) (*pb.Response, error) {
	logrus.WithFields(logrus.Fields{
		"service": "ClusterManager",
		"type":    "grpc",
	}).Info("Received CreateNodeRequest")

	if mutex.IsLocked(mutex.LockActionAny) {
		return clumgrcommon.GenerateResponse(clustermanager.Status_IN_PROGRESS, "AppHoster cluster is locked", nil)
	}

	mutex.Lock(mutex.LockActionCreateNode, nil)
	defer mutex.Unlock(mutex.LockActionCreateNode)

	return mgr.adapter.CreateNode(req)
}

func (mgr *manager) DeleteNode(ctx context.Context, req *pb.DeleteNodeRequest) (*pb.Response, error) {
	logrus.WithFields(logrus.Fields{
		"service": "ClusterManager",
		"type":    "grpc",
	}).Info("Received DeleteNodeRequest")

	if mutex.IsLocked(mutex.LockActionAny) {
		return clumgrcommon.GenerateResponse(clustermanager.Status_IN_PROGRESS, "AppHoster cluster is locked", nil)
	}

	mutex.Lock(mutex.LockActionDeleteNode, nil)
	defer mutex.Unlock(mutex.LockActionDeleteNode)

	return mgr.adapter.DeleteNode(req)
}

func (mgr *manager) UpdateNodeState(ctx context.Context, req *pb.UpdateNodeStateRequest) (*pb.Response, error) {
	logrus.WithFields(logrus.Fields{
		"service": "ClusterManager",
		"type":    "grpc",
	}).Info("Received UpdateNodeStateRequest")

	if mutex.IsLocked(mutex.LockActionAny) {
		return clumgrcommon.GenerateResponse(clustermanager.Status_IN_PROGRESS, "AppHoster cluster is locked", nil)
	}

	mutex.Lock(mutex.LockActionUpdateNodeState, nil)
	defer mutex.Unlock(mutex.LockActionUpdateNodeState)

	return mgr.adapter.UpdateNodeState(req)
}

func (mgr *manager) SetClusterResourceQuotas(ctx context.Context, req *pb.SetClusterResourceQuotasRequest) (*pb.Response, error) {
	logrus.WithFields(logrus.Fields{
		"service": "ClusterManager",
		"type":    "grpc",
	}).Info("Received SetClusterResourceQuotas")

	if err := req.Validate(); err != nil {
		return clumgrcommon.GenerateResponse(clustermanager.Status_ERROR, err.Error(), nil)
	}

	rbody := &pb.SetGetClusterResourceQuotasResponseBody{}

	for _, q := range req.Quotas {
		if err := resourcemgr.CreateUpdateResourceQuotas(mgr.kc, q.Namespace, q.Cpu, q.Memory); err != nil {
			return clumgrcommon.GenerateResponse(clustermanager.Status_ERROR, err.Error(), nil)
		}

		rbody.Quotas = append(rbody.Quotas, q)
	}

	return clumgrcommon.GenerateResponse(clustermanager.Status_SUCCESS, "Successfully applied resource quotas", rbody)
}

func (mgr *manager) GetClusterResourceQuotas(ctx context.Context, req *pb.GetClusterResourceQuotasRequest) (*pb.Response, error) {
	logrus.WithFields(logrus.Fields{
		"service": "ClusterManager",
		"type":    "grpc",
	}).Info("Received GetClusterResourceQuotas")

	if err := req.Validate(); err != nil {
		return clumgrcommon.GenerateResponse(clustermanager.Status_ERROR, err.Error(), nil)
	}

	rbody := &pb.SetGetClusterResourceQuotasResponseBody{}

	for _, ns := range req.Namespaces {
		cpu, memory, err := resourcemgr.GetResourceQuotas(mgr.kc, ns)
		if err != nil {
			return clumgrcommon.GenerateResponse(clustermanager.Status_ERROR, err.Error(), nil)
		}

		q := &pb.Quota{}
		q.Namespace = ns
		q.Cpu = cpu
		q.Memory = memory
		rbody.Quotas = append(rbody.Quotas, q)
	}

	return clumgrcommon.GenerateResponse(clustermanager.Status_SUCCESS, "List of quotas", rbody)
}

func (mgr *manager) DeleteClusterResourceQuotas(ctx context.Context, req *pb.DeleteClusterResourceQuotasRequest) (*pb.Response, error) {
	logrus.WithFields(logrus.Fields{
		"service": "ClusterManager",
		"type":    "grpc",
	}).Info("Received DeleteClusterResourceQuotas")

	if err := resourcemgr.DeleteResourceQuotas(mgr.kc, req.Namespace); err != nil {
		return clumgrcommon.GenerateResponse(clustermanager.Status_ERROR, err.Error(), nil)
	}

	return clumgrcommon.GenerateResponse(clustermanager.Status_SUCCESS,
		"Quotas were removed successfully from the namespace "+req.Namespace, nil)
}
