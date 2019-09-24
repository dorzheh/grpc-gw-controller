// Author <dorzheho@cisco.com>

package apiclient

import (
	"fmt"
	"path/filepath"

	clusterClient "github.com/rancher/types/client/cluster/v3"
	projectClient "github.com/rancher/types/client/project/v3"
	"github.com/sirupsen/logrus"

	appcommon "cisco.com/son/apphcd/app/common"
	"cisco.com/son/apphcd/app/grpc/appmanager/common"
	grpccommon "cisco.com/son/apphcd/app/grpc/common"

	"cisco.com/son/apphcd/app/grpc/common/rancher"
)

func CreateSharedStorage(mc *rancher.MasterClient, appName, kubeConvAppName, namespace string, volSize int) error {
	volumeName := kubeConvAppName + "-shared-pv"
	claimName := kubeConvAppName + "-shared-pvc"

	opts := rancher.DefaultListOpts()

	opts.Filters["name"] = claimName
	pvcCol, err := mc.ProjectClient.PersistentVolumeClaim.List(opts)
	if err != nil {
		return err
	}

	opts.Filters["name"] = volumeName
	pvCol, err := mc.ClusterClient.PersistentVolume.List(opts)
	if err != nil {
		return err
	}

	if len(pvCol.Data) > 0 {
		currentSize, err := grpccommon.GetStorageGbInt(pvCol.Data[0].Capacity["storage"])
		if err != nil {
			return err
		}

		if volSize == currentSize {
			logrus.WithFields(logrus.Fields{"volume": volumeName}).Info("Persistent volume already exists")
			if len(pvcCol.Data) > 0 {
				return nil
			}

			return createPersistentVolumeClaim(mc, appName, kubeConvAppName, volumeName, claimName, namespace, volSize)
		}

		opts.Filters["name"] = claimName
		// If appropriate PVC found
		if err := deletePersistentVolumeClaim(mc, opts); err != nil {
			return err
		}

		// Delete persistent volume
		opts.Filters["name"] = volumeName
		if err := deletePersistentVolume(mc, opts); err != nil {
			return err
		}

		// Wait till the PV will be removed
		if err := waitForVolumeState(mc, opts, "removed"); err != nil {
			return err
		}
	}

	logrus.WithFields(logrus.Fields{"volume": volumeName}).Info("Creating persistent volume")

	// Configure volume
	v := &clusterClient.PersistentVolume{}
	v.Annotations = make(map[string]string)
	appcommon.MapAdd(v.Annotations, common.AppAnnotationBaseName, appName)
	v.HostPath = &clusterClient.HostPathVolumeSource{}
	v.HostPath.Path = filepath.Join(appcommon.AppdataPath, appName, "shared")
	v.Capacity = make(map[string]string)
	v.Capacity["storage"] = fmt.Sprintf("%dGi", volSize)
	v.AccessModes = []string{"ReadWriteMany"}
	v.Name = volumeName
	v.PersistentVolumeReclaimPolicy = "Retain"

	if _, err := mc.ClusterClient.PersistentVolume.Create(v); err != nil {
		logrus.Error(err)
		return err
	}

	logrus.WithFields(logrus.Fields{"volume": volumeName, "status": "OK"}).Info("Creating persistent volume")

	if len(pvcCol.Data) == 0 {
		return createPersistentVolumeClaim(mc, appName, kubeConvAppName, volumeName, claimName, namespace, volSize)
	}

	pvc := pvcCol.Data[0]
	currentSize, err := grpccommon.GetStorageGbInt(pvc.Status.Capacity["storage"])
	if err != nil {
		return err
	}

	if currentSize != volSize {
		return createPersistentVolumeClaim(mc, appName, kubeConvAppName, volumeName, claimName, namespace, volSize)
	}

	return nil
}

func DeleteStorage(mc *rancher.MasterClient, volumeName, claimName string) error {
	opts := rancher.DefaultListOpts()
	opts.Filters["name"] = claimName
	// If appropriate PVC found
	if err := deletePersistentVolumeClaim(mc, opts); err != nil {
		return err
	}

	// Delete persistent volume
	opts.Filters["name"] = volumeName
	if err := deletePersistentVolume(mc, opts); err != nil {
		return err
	}

	// Wait till the PV will be removed
	return waitForVolumeState(mc, opts, "removed")
}

// createPersistentVolumeClaim creates new PVC and binds it to appropriate Persistent Volume (PV)
func createPersistentVolumeClaim(mc *rancher.MasterClient, appName, kubeConvAppName, volumeName, claimName, namespace string, volSize int) error {
	newPvc := &projectClient.PersistentVolumeClaim{}
	newPvc.Annotations = make(map[string]string)
	newPvc.Name = claimName
	newPvc.NamespaceId = namespace
	newPvc.AccessModes = []string{"ReadWriteMany"}
	newPvc.Resources = &projectClient.ResourceRequirements{}
	newPvc.Resources.Requests = make(map[string]string)
	newPvc.Resources.Requests["storage"] = fmt.Sprintf("%dGi", volSize)
	newPvc.Selector = &projectClient.LabelSelector{}
	newPvc.Selector.MatchLabels = make(map[string]string)
	newPvc.Selector.MatchLabels["name"] = kubeConvAppName
	newPvc.VolumeID = volumeName

	appcommon.MapAdd(newPvc.Annotations, common.AppAnnotationBaseName, appName)

	logrus.WithFields(logrus.Fields{"claim": newPvc.Name}).Info("Creating persistent volume claim")

	if _, err := mc.ProjectClient.PersistentVolumeClaim.Create(newPvc); err != nil {
		return err
	}

	logrus.WithFields(logrus.Fields{"instance": newPvc.Name, "claim": newPvc.Name, "status": "OK"}).
		Info("Creating persistent volume claim")

	// Wait till the PV and PVC will be in the "Bound" state
	opts := rancher.DefaultListOpts()
	opts.Filters["name"] = volumeName
	return waitForVolumeState(mc, opts, "bound")
}
