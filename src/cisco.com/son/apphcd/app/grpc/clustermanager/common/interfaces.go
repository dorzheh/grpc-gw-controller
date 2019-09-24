// Author <dorzheho@cisco.com>

package common

import (
	"k8s.io/api/core/v1"

	"cisco.com/son/apphcd/api/v1/clustermanager"
)

// ClusterManagerAdapter interface
type ClusterManagerAdapter interface {
	GetKubeConfig(request *clustermanager.GetKubeConfigRequest) (*clustermanager.GetKubeConfigResponse, error)
	GetClusterInfo(request *clustermanager.GetClusterInfoRequest, namespace *v1.Namespace) (*clustermanager.Response, error)
	CreateNode(request *clustermanager.CreateNodeRequest) (*clustermanager.Response, error)
	DeleteNode(request *clustermanager.DeleteNodeRequest) (*clustermanager.Response, error)
	UpdateNodeState(request *clustermanager.UpdateNodeStateRequest) (*clustermanager.Response, error)
	SetClusterResourceQuotas(request *clustermanager.SetClusterResourceQuotasRequest) (*clustermanager.Response, error)
	GetClusterResourceQuotas(request *clustermanager.GetClusterResourceQuotasRequest) (*clustermanager.Response, error)
	DeleteClusterResourceQuotas(request *clustermanager.DeleteClusterResourceQuotasRequest) (*clustermanager.Response, error)
}
