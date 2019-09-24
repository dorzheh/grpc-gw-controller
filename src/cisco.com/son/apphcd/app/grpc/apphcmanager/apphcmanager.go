// Author  <dorzheho@cisco.com>

package apphcmanager

import (
	"context"

	"github.com/sirupsen/logrus"

	pb "cisco.com/son/apphcd/api/v1/apphcmanager"
	"cisco.com/son/apphcd/app/grpc/apphcmanager/version"
)

type ApphcManager struct{}

func (mgr *ApphcManager) GetVersion(ctx context.Context, req *pb.GetApphcVersionRequest) (*pb.GetApphcVersionResponse, error) {
	logrus.WithFields(logrus.Fields{"service": "ApphcManager", "type": "grpc"}).Info("Received GetVersionRequest")

	v := version.New()
	resp := &pb.GetApphcVersionResponse{}
	resp.Version = v.Version
	resp.ApiVersion = version.ApiVersion
	resp.GitCommit = v.GitCommit
	resp.GitState = v.GitTreeState

	logrus.WithFields(logrus.Fields{"service": "ApphcManager", "type": "grpc"}).Info("Sending response")

	return resp, nil
}
