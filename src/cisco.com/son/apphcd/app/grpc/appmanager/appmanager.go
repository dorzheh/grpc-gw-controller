// Author  <dorzheho@cisco.com>

package appmanager

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"

	pb "cisco.com/son/apphcd/api/v1/appmanager"
	"cisco.com/son/apphcd/app/common/mutex"
	appmgrcommon "cisco.com/son/apphcd/app/grpc/appmanager/common"
	"cisco.com/son/apphcd/app/grpc/common/resourcemgr"
)

// AppManager structure
type manager struct {
	// AppManager adapter
	adapter appmgrcommon.AppManagerAdapter
}

// New method instantiates a new AppManager server
func New(adapter appmgrcommon.AppManagerAdapter) pb.AppManagerServer {
	return &manager{adapter}
}

func (mgr *manager) CreateApp(ctx context.Context, req *pb.CreateAppRequest) (*pb.Response, error) {
	logrus.WithFields(logrus.Fields{
		"service": "AppManager",
		"type":    "grpc",
	}).Info("Received CreateAppRequest")

	logrus.Debugf("CreateAppRequest message: %q", req.String())

	appLocker := req.Name + "" + req.RootGroupId

	if mutex.IsLocked(appLocker) {
		return appmgrcommon.GenerateResponse(pb.Status_ERROR, fmt.Sprintf("application %s is locked", req.Name), nil)
	}

	if err := req.Validate(); err != nil {
		return appmgrcommon.GenerateResponse(pb.Status_ERROR, err.Error(), nil)
	}

	if err := resourcemgr.ValidateInputs(req.GetSpec().GetResources().GetLimits().GetCpu(),
		appmgrcommon.AppInstanceDefaultCpuRequestFloat64, req.GetSpec().GetResources().GetLimits().GetMemory(),
		appmgrcommon.AppInstanceDefaultMemoryRequestUint32); err != nil {
		return appmgrcommon.GenerateResponse(pb.Status_ERROR, err.Error(), nil)
	}

	mutex.Lock(appLocker, nil)
	defer mutex.Unlock(appLocker)

	if err := appmgrcommon.ValidateDockerImage(req.GetSpec().GetImage().GetRepo(), req.GetSpec().GetImage().GetTag()); err != nil {
		return appmgrcommon.GenerateResponse(pb.Status_ERROR, err.Error(), nil)
	}

	return mgr.adapter.CreateApp(req)
}

func (mgr *manager) UpgradeApp(ctx context.Context, req *pb.UpgradeAppRequest) (*pb.Response, error) {
	logrus.WithFields(logrus.Fields{
		"service": "AppManager",
		"type":    "grpc",
	}).Info("Received UpgradeAppRequest")

	logrus.Debugf("UpgradeAppRequest message: %q", req.String())

	appLocker := req.Name + "" + req.RootGroupId

	if mutex.IsLocked(appLocker) {
		return appmgrcommon.GenerateResponse(pb.Status_ERROR, fmt.Sprintf("application %s is locked", req.Name), nil)
	}

	if err := req.Validate(); err != nil {
		return appmgrcommon.GenerateResponse(pb.Status_ERROR, err.Error(), nil)
	}

	mutex.Lock(appLocker, nil)
	defer mutex.Unlock(appLocker)

	if err := appmgrcommon.ValidateDockerImage(req.GetSpec().GetImage().GetRepo(), req.GetSpec().GetImage().GetTag()); err != nil {
		return appmgrcommon.GenerateResponse(pb.Status_ERROR, err.Error(), nil)
	}

	return mgr.adapter.UpgradeApp(req)
}

func (mgr *manager) UpdateApp(ctx context.Context, req *pb.UpdateAppRequest) (*pb.Response, error) {
	logrus.WithFields(logrus.Fields{
		"service": "AppManager",
		"type":    "grpc",
	}).Info("Received UpdateAppRequest")

	logrus.Debugf("UpdateAppRequest message: %q", req.String())

	appLocker := req.Name + "" + req.RootGroupId

	if mutex.IsLocked(appLocker) {
		return appmgrcommon.GenerateResponse(pb.Status_ERROR, fmt.Sprintf("application %s is locked", req.Name), nil)
	}

	if err := req.Validate(); err != nil {
		return appmgrcommon.GenerateResponse(pb.Status_ERROR, err.Error(), nil)
	}

	mutex.Lock(appLocker, nil)
	defer mutex.Unlock(appLocker)

	if req.GetSpec() != nil {
		if req.GetSpec().GetImage() != nil {
			if err := req.GetSpec().GetImage().Validate(); err != nil {
				return appmgrcommon.GenerateResponse(pb.Status_ERROR, err.Error(), nil)
			}
		}

		if req.GetSpec().GetPorts() != nil {
			for _, p := range req.GetSpec().GetPorts() {
				if err := p.Validate(); err != nil {
					return appmgrcommon.GenerateResponse(pb.Status_ERROR, err.Error(), nil)
				}
			}
		}
	}

	if req.GetVersion() != "" {
		return appmgrcommon.GenerateResponse(pb.Status_ERROR, "the field version is not supported by the UpdateApp request", nil)
	}

	if err := appmgrcommon.ValidateDockerImage(req.GetSpec().GetImage().GetRepo(), req.GetSpec().GetImage().GetTag()); err != nil {
		return appmgrcommon.GenerateResponse(pb.Status_ERROR, err.Error(), nil)
	}

	return mgr.adapter.UpdateApp(req)
}

func (mgr *manager) EnableDisableApp(ctx context.Context, req *pb.EnableDisableAppRequest) (*pb.Response, error) {
	logrus.WithFields(logrus.Fields{
		"service": "AppManager",
		"type":    "grpc",
	}).Info("Received EnableDisableAppRequest")

	logrus.Debugf("EnableDisableAppRequest message: %q", req.String())

	if mutex.IsLocked(mutex.LockActionEnableDisableApps) {
		return appmgrcommon.GenerateResponse(pb.Status_ERROR, fmt.Sprintf("application %s is locked", req.Name), nil)
	}

	mutex.Lock(mutex.LockActionEnableDisableApps, nil)
	defer mutex.Unlock(mutex.LockActionEnableDisableApps)

	return mgr.adapter.EnableDisableApp(req)
}

func (mgr *manager) DeleteApp(ctx context.Context, req *pb.DeleteAppRequest) (*pb.Response, error) {
	logrus.WithFields(logrus.Fields{
		"service": "AppManager",
		"type":    "grpc",
	}).Info("Received DeleteAppRequest")

	logrus.Debugf("DeleteAppRequest message: %q", req.String())

	appLocker := req.Name + "" + req.RootGroupId

	if mutex.IsLocked(appLocker) {
		return appmgrcommon.GenerateResponse(pb.Status_ERROR, fmt.Sprintf("application %s is locked", req.Name), nil)
	}

	if err := req.Validate(); err != nil {
		return appmgrcommon.GenerateResponse(pb.Status_ERROR, err.Error(), nil)
	}

	mutex.Lock(appLocker, nil)
	defer mutex.Unlock(appLocker)

	return mgr.adapter.DeleteApp(req)
}

func (mgr *manager) DeleteApps(ctx context.Context, req *pb.DeleteAppsRequest) (*pb.Response, error) {
	logrus.WithFields(logrus.Fields{
		"service": "AppManager",
		"type":    "grpc",
	}).Info("Received DeleteAppsRequest")

	logrus.Debugf("DeleteAppsRequest message: %q", req.String())

	if mutex.IsLocked(mutex.LockActionDeleteApps) {
		return appmgrcommon.GenerateResponse(pb.Status_ERROR, "applications are locked", nil)
	}

	if err := req.Validate(); err != nil {
		return appmgrcommon.GenerateResponse(pb.Status_ERROR, err.Error(), nil)
	}

	mutex.Lock(mutex.LockActionDeleteApps, nil)
	defer mutex.Unlock(mutex.LockActionDeleteApps)

	return mgr.adapter.DeleteApps(req)
}

func (mgr *manager) GetApps(ctx context.Context, req *pb.GetAppsRequest) (*pb.Response, error) {
	logrus.WithFields(logrus.Fields{
		"service": "AppManager",
		"type":    "grpc",
	}).Info("Received GetAppsRequest")

	logrus.Debugf("GetAppsRequest message: %q", req.String())

	if err := req.Validate(); err != nil {
		return appmgrcommon.GenerateResponse(pb.Status_ERROR, err.Error(), nil)
	}

	return mgr.adapter.GetApps(req)
}

func (mgr *manager) DeleteAppMetadata(ctx context.Context, req *pb.DeleteAppMetadataRequest) (*pb.Response, error) {
	logrus.WithFields(logrus.Fields{
		"service": "AppManager",
		"type":    "grpc",
	}).Info("Received DeleteAppMetadataRequest")

	logrus.Debugf("DeleteAppMetadataRequest message: %q", req.String())

	if mutex.IsLocked(req.AppName) {
		return appmgrcommon.GenerateResponse(pb.Status_ERROR, fmt.Sprintf("metadata for application %s is locked", req.AppName), nil)
	}

	if err := req.Validate(); err != nil {
		return appmgrcommon.GenerateResponse(pb.Status_ERROR, err.Error(), nil)
	}

	mutex.Lock(req.AppName, nil)
	defer mutex.Unlock(req.AppName)

	return mgr.adapter.DeleteAppMetadata(req)
}
